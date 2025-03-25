package a3mclient

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	transferservice "github.com/penwern/preservation-go/common/proto/a3m/gen/go/a3m/api/transferservice/v1beta1"
)

// Client wraps the gRPC connection and provides package submission methods.
type Client struct {
	address string
	client  transferservice.TransferServiceClient
	conn    *grpc.ClientConn
	mu      sync.Mutex // Mutex to ensure thread-safe operations
}

type ClientInterface interface {
	Close()
	SubmitPackage(ctx context.Context, url, name string, config *transferservice.ProcessingConfig) (string, *transferservice.ReadResponse, error)
}

// NewClient creates a new client instance.
func NewClient(address string) (*Client, error) {
	options := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	conn, err := grpc.NewClient(address, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to a3m server at %q: %w", address, err)
	}

	client := transferservice.NewTransferServiceClient(conn)
	return &Client{
		address: address,
		client:  client,
		conn:    conn,
	}, nil
}

// Close shuts down the underlying gRPC connection.
func (c *Client) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

// SubmitPackage submits a package (given by its URI) with a name and configuration.
// It polls the server until processing is complete (or fails) and returns the AIP UUID and final response.
func (c *Client) SubmitPackage(ctx context.Context, url, name string, config *transferservice.ProcessingConfig) (string, *transferservice.ReadResponse, error) {
	c.mu.Lock() // Lock the mutex to ensure thread-safe access
	defer c.mu.Unlock()

	submitReq := &transferservice.SubmitRequest{
		Name:   name,
		Url:    url,
		Config: config,
	}
	submitResp, err := c.client.Submit(ctx, submitReq)
	if err != nil {
		return "", nil, fmt.Errorf("failed to submit package: %v", err)
	}

	// Poll for completion.
	for {
		readReq := &transferservice.ReadRequest{Id: submitResp.Id}
		readResp, err := c.client.Read(ctx, readReq)
		if err != nil {
			return "", nil, fmt.Errorf("error reading status: %v", err)
		}
		status := readResp.Status
		if status == transferservice.PackageStatus_PACKAGE_STATUS_COMPLETE {
			return submitResp.Id, readResp, nil
		} else if status == transferservice.PackageStatus_PACKAGE_STATUS_FAILED ||
			status == transferservice.PackageStatus_PACKAGE_STATUS_REJECTED {
			failedJobs := c.collectFailedJobs(ctx, readResp.Jobs)
			return "", nil, fmt.Errorf("error processing package (status: %s). Failed jobs: %v",
				transferservice.PackageStatus_name[int32(status)], failedJobs)
		}
		// Wait before polling again.
		time.Sleep(3 * time.Second)
	}
}

// collectFailedJobs gathers details on failed jobs.
func (c *Client) collectFailedJobs(ctx context.Context, jobs []*transferservice.Job) []map[string]any {
	var failedJobsInfo []map[string]any
	for _, job := range jobs {
		if job.Status != transferservice.Job_STATUS_FAILED {
			continue
		}
		jobInfo := map[string]any{
			"job_name": job.Name,
			"job_id":   job.Id,
			"link_id":  job.LinkId,
		}
		listReq := &transferservice.ListTasksRequest{JobId: job.Id}
		listResp, err := c.client.ListTasks(ctx, listReq)
		if err != nil {
			fmt.Printf("Failed to retrieve tasks for job %s: %v", job.Id, err)
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
