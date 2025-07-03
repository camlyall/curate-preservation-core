// Package a3mclient provides a client for interacting with the A3M Transfer Service.
package a3mclient

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	transferservice "github.com/penwern/curate-preservation-core/common/proto/a3m/gen/go/a3m/api/transferservice/v1beta1"
	"github.com/penwern/curate-preservation-core/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client wraps the gRPC connection and provides package submission methods.
type Client struct {
	address string
	client  transferservice.TransferServiceClient
	conn    *grpc.ClientConn

	// Rate limiting
	activeRequests   sync.Map      // Map of active package IDs
	processingTokens chan struct{} // Semaphore for limiting concurrent processing
	opt              ClientOptions
}

// ClientOptions represents the options for the A3M client.
type ClientOptions struct {
	MaxActiveProcessing int           // Maximum number of concurrent packages in processing state
	PollInterval        time.Duration // Time between status polls
}

// ClientInterface defines the interface for the A3M client.
type ClientInterface interface {
	Close()
	SubmitPackage(ctx context.Context, path, name string, config *transferservice.ProcessingConfig) (string, *transferservice.ReadResponse, error)
	GetActiveProcessingCount() int
}

// NewClient creates a new client instance with default options.
func NewClient(address string) (*Client, error) {
	return NewClientWithOptions(address, ClientOptions{
		MaxActiveProcessing: 1,
		PollInterval:        1 * time.Second,
	})
}

// NewClientWithOptions creates a new client instance with custom options.
func NewClientWithOptions(address string, options ClientOptions) (*Client, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	conn, err := grpc.NewClient(address, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to a3m server at %q: %w", address, err)
	}

	maxActive := options.MaxActiveProcessing
	if maxActive <= 0 {
		maxActive = 1
	}
	pollingInterval := options.PollInterval
	if pollingInterval <= 0 {
		pollingInterval = 1 * time.Second
	}

	client := transferservice.NewTransferServiceClient(conn)
	return &Client{
		address: address,
		client:  client,
		conn:    conn,
		opt: ClientOptions{
			MaxActiveProcessing: maxActive,
			PollInterval:        pollingInterval,
		},

		processingTokens: make(chan struct{}, maxActive),
	}, nil
}

// GetActiveProcessingCount returns the number of packages currently being processed
func (c *Client) GetActiveProcessingCount() int {
	count := 0
	c.activeRequests.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}

// SubmitPackage submits a package (given by its URI) with a name and configuration.
// It polls the server until processing is complete (or fails) and returns the AIP UUID and final response.
// This implementation will block if there are already maxActiveProcessing packages being processed.
func (c *Client) SubmitPackage(ctx context.Context, path, name string, config *transferservice.ProcessingConfig) (string, *transferservice.ReadResponse, error) {
	// Acquire processing token (will block if too many packages are processing)
	select {
	case c.processingTokens <- struct{}{}:
		// Token acquired
	case <-ctx.Done():
		return "", nil, fmt.Errorf("context cancelled while waiting for processing slot: %w", ctx.Err())
	}

	// Sanitize name
	// Remove whitespace and special characters
	name = strings.ReplaceAll(name, " ", "_")

	// Ensure token is released when done
	defer func() {
		<-c.processingTokens
	}()
	submitReq := &transferservice.SubmitRequest{
		Name:   name,
		Url:    path,
		Config: config,
	}
	logger.Debug("A3M Submission Request: %+v", submitReq)

	submitResp, err := c.client.Submit(ctx, submitReq)
	logger.Debug("A3M Submission Response: %v", submitResp)
	if err != nil {
		return "", nil, fmt.Errorf("failed to submit package: %w", err)
	}
	logger.Debug("Submitted package %q with ID %q", name, submitResp.Id)

	// Track this as an active request
	c.activeRequests.Store(submitResp.Id, struct{}{})
	defer c.activeRequests.Delete(submitResp.Id)

	// Poll for completion
	for {
		logger.Debug("Polling package %q (ID: %q)", name, submitResp.Id)
		select {
		case <-ctx.Done():
			return "", nil, fmt.Errorf("context cancelled during package processing: %w", ctx.Err())
		case <-time.After(c.opt.PollInterval):
			// Continue with polling
		}

		readReq := &transferservice.ReadRequest{Id: submitResp.Id}
		readResp, err := c.client.Read(ctx, readReq)
		if err != nil {
			return "", nil, fmt.Errorf("error reading status for package %q (ID: %q): %w", name, submitResp.Id, err)
		}

		status := readResp.Status
		switch status {
		case transferservice.PackageStatus_PACKAGE_STATUS_UNSPECIFIED:
			return "", nil, fmt.Errorf("package %q (ID: %q) has an unspecified status", name, submitResp.Id)
		case transferservice.PackageStatus_PACKAGE_STATUS_PROCESSING:
			logger.Debug("Package %q (ID: %q) is still processing", name, submitResp.Id)
			continue
		case transferservice.PackageStatus_PACKAGE_STATUS_COMPLETE:
			failedJobs := c.collectFailedJobs(ctx, readResp.Jobs)
			if len(failedJobs) > 0 {
				logger.Debug("Package %q (ID: %q) completed with failed jobs: %v", name, submitResp.Id, failedJobs)
			}
			return submitResp.Id, readResp, nil
		case transferservice.PackageStatus_PACKAGE_STATUS_FAILED:
			logger.Debug("Package %q (ID: %q) failed", name, submitResp.Id)
			failedJobs := c.collectFailedJobs(ctx, readResp.Jobs)
			return "", nil, fmt.Errorf("error processing package (status: %s). Failed jobs: %v",
				transferservice.PackageStatus_name[int32(status)], failedJobs)
		case transferservice.PackageStatus_PACKAGE_STATUS_REJECTED:
			logger.Debug("Package %q (ID: %q) rejected", name, submitResp.Id)
			failedJobs := c.collectFailedJobs(ctx, readResp.Jobs)
			return "", nil, fmt.Errorf("error processing package (status: %s). Failed jobs: %v",
				transferservice.PackageStatus_name[int32(status)], failedJobs)
		default:
			return "", nil, fmt.Errorf("unknown status %q for package %q (ID: %q)", status, name, submitResp.Id)
		}
	}
}

// Close shuts down the underlying gRPC connection.
func (c *Client) Close() {
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			logger.Error("Failed to close connection: %v", err)
		}
	}
}

// collectFailedJobs gathers details on failed jobs.
func (c *Client) collectFailedJobs(ctx context.Context, jobs []*transferservice.Job) []map[string]any {
	failedJobsInfo := make([]map[string]any, 0, len(jobs))
	for _, job := range jobs {
		if job.Status != transferservice.Job_STATUS_FAILED {
			continue
		}

		// Check if context is cancelled
		if ctx.Err() != nil {
			return append(failedJobsInfo, map[string]any{
				"error": "context cancelled while collecting job information",
			})
		}

		jobInfo := map[string]any{
			"job_name": job.Name,
			"job_id":   job.Id,
			"link_id":  job.LinkId,
		}

		// Add timeout for task listing
		taskCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		listReq := &transferservice.ListTasksRequest{JobId: job.Id}
		listResp, err := c.client.ListTasks(taskCtx, listReq)
		cancel()

		if err != nil {
			jobInfo["tasks_error"] = err.Error()
			jobInfo["tasks"] = nil
		} else {
			var tasks []map[string]any
			for _, task := range listResp.Tasks {
				tasks = append(tasks, map[string]any{
					"task_id":   task.Id,
					"execution": task.Execution,
					"arguments": task.Arguments,
					"stdout":    task.Stdout,
					"stderr":    task.Stderr,
				})
			}
			jobInfo["tasks"] = tasks
		}
		failedJobsInfo = append(failedJobsInfo, jobInfo)
	}
	return failedJobsInfo
}
