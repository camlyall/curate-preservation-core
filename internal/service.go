package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/penwern/curate-preservation-core/internal/a3mclient"
	"github.com/penwern/curate-preservation-core/internal/preservation"
	"github.com/penwern/curate-preservation-core/pkg/config"
	"github.com/penwern/curate-preservation-core/pkg/logger"
)

// Service is the root service for the preservation tool.
type Service struct {
	cfg *config.Config
	svc *preservation.Preserver
}

// ServiceArgs holds the arguments for the root service.
type ServiceArgs struct {
	AllowInsecureTLS bool                       `json:"allowInsecureTLS"`
	CellsArchiveDir  string                     `json:"archiveDir"`
	CellsNodes       []NodeAlias                `json:"nodes"` // Support for passing nodes directly from flows
	CellsPaths       []string                   `json:"paths"`
	CellsUsername    string                     `json:"username"`
	Cleanup          bool                       `json:"cleanup"`
	PathsResolved    bool                       `json:"pathsResolved"`
	PreservationCfg  *config.PreservationConfig `json:"preservationCfg"`
}

// NodeAlias represents a cells node.
// Using this node alias until I find a proper way to serialize Node input into Cells SDK models.TreeNode
// Currently the SDK models.TreeNode is not directly serializable.
type NodeAlias struct {
	Path string `json:"path"`
	UUID string `json:"uuid"`
}

// NewService creates a new preservation service.
func NewService(ctx context.Context, cfg *config.Config) (*Service, error) {
	// Create a3m client with concurrency control
	a3mOptions := a3mclient.ClientOptions{
		MaxActiveProcessing: 1, // Currently only support 1 package at a time ;(
		PollInterval:        1 * time.Second,
	}
	a3mClient, err := a3mclient.NewClientWithOptions(cfg.A3M.Address, a3mOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create a3m client: %w", err)
	}

	s := &Service{
		svc: preservation.NewPreserverWithA3MClient(ctx, cfg, a3mClient),
		cfg: cfg,
	}
	return s, nil
}

// Close closes the preservation service.
func (s *Service) Close() {
	s.svc.Close()
}

// RunArgs runs the preservation service with the given arguments.
func (s *Service) RunArgs(ctx context.Context, args *ServiceArgs) error {
	return s.Run(ctx, args.CellsUsername, args.CellsPaths, args.Cleanup, args.PathsResolved, args.PreservationCfg)
}

// Run runs the preservation service.
func (s *Service) Run(ctx context.Context, username string, paths []string, cleanup, pathsResolved bool, presConfig *config.PreservationConfig) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(paths))

	if s.cfg.LogLevel == "debug" {
		// Pretty print the configuration
		jsonCfg, err := json.MarshalIndent(s.cfg, "", "  ")
		if err != nil {
			logger.Error("Error marshalling configuration: %v", err)
		}
		logger.Debug("Service Configuration:\n%s", string(jsonCfg))

		jsonPresCfg, err := json.MarshalIndent(presConfig, "", "  ")
		if err != nil {
			logger.Error("Error marshalling preservation configuration: %v", err)
		}
		logger.Debug("Preservation Configuration:\n%s", string(jsonPresCfg))
	}

	// Number of concurrent operations
	maxWorkers := 10
	semaphore := make(chan struct{}, maxWorkers)

	const maxRetries = 1

	// Create a user client per submission
	userClient, err := s.svc.NewUserClient(ctx, username)
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
				if err := s.svc.Run(ctx, presConfig, userClient, path, cleanup, pathsResolved); err != nil {
					logger.Error("Error running preservation for package '%s' (attempt %d/%d): %v", path, i+1, maxRetries, err)
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

	if err := <-errChan; err != nil {
		return fmt.Errorf("preservation process completed with errors")
	}

	return nil
}
