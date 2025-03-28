package internal

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/penwern/preservation-go/internal/a3mclient"
	"github.com/penwern/preservation-go/internal/preservation"
	"github.com/penwern/preservation-go/pkg/config"
	"github.com/penwern/preservation-go/pkg/logger"
)

// Service is the root service for the preservation tool.
type Service struct {
	svc *preservation.Preserver
	cfg *config.Config
}

// ServiceArgs holds the arguments for the root service.
type ServiceArgs struct {
	Username      string      `json:"username"`
	Paths         []string    `json:"paths"`
	Nodes         []NodeAlias `json:"nodes"` // Support for passing nodes directly from flows
	PathsResolved bool
	Cleanup       bool   `json:"cleanup"`
	ArchiveDir    string `json:"archiveDir"`
}

// Using this node alias until I find a proper way to serialize Node input into Cells SDK models.TreeNode
// Currently the SDK models.TreeNode is not directly serializable.
type NodeAlias struct {
	Path string `json:"path"`
	Uuid string `json:"uuid"`
}

func NewService(ctx context.Context, cfg *config.Config) (*Service, error) {

	// Create a3m client with concurrency control
	a3mOptions := a3mclient.ClientOptions{
		MaxActiveProcessing: 1, // Currently only support 1 package at a time ;(
		PollInterval:        1 * time.Second,
	}
	a3mClient, err := a3mclient.NewClientWithOptions(cfg.A3mAddress, a3mOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create a3m client: %w", err)
	}

	s := &Service{
		svc: preservation.NewPreserverWithA3MClient(ctx, cfg, a3mClient),
		cfg: cfg,
	}
	return s, nil
}

func (s *Service) Close() {
	s.svc.Close()
}

func (s *Service) RunArgs(ctx context.Context, args *ServiceArgs) error {
	return s.Run(ctx, args.Username, args.ArchiveDir, args.Paths, args.Cleanup, args.PathsResolved)
}

func (s *Service) Run(ctx context.Context, username, archiveDir string, paths []string, cleanup, pathsResolved bool) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(paths))

	// Number of concurrent operations
	maxWorkers := 10
	semaphore := make(chan struct{}, maxWorkers)

	const maxRetries = 2

	userClient, err := s.svc.GetUserClient(ctx, username)
	if err != nil {
		return fmt.Errorf("failed to get user client: %w", err)
	}

	for _, packagePath := range paths {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			for i := range maxRetries {
				if err := s.svc.Run(ctx, &config.PreservationConfig{CompressAip: false}, userClient, path, cleanup, pathsResolved); err != nil {
					logger.Error("Error running preservation for package %s (attempt %d/%d): %v", path, i+1, maxRetries, err)
					if i+1 == maxRetries {
						errChan <- err
					}
				} else {
					// Success, exit retry loop
					break
				}
			}
		}(packagePath)
	}

	wg.Wait()
	close(errChan)

	// Collect errors
	var hasErrors bool
	for err := range errChan {
		logger.Error("Error running preservation for a package: %v\n", err)
		hasErrors = true
	}

	if hasErrors {
		return fmt.Errorf("preservation process completed with errors")
	}

	if s.cfg.LogLevel == "ERROR" {
		logger.Error("Preservation process completed successfully")
	}

	return nil
}
