package internal

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/penwern/preservation-go/internal/preservation"
	"github.com/penwern/preservation-go/pkg/config"
)

type Service struct {
	svc *preservation.Service
	cfg *config.Config
}

func NewService(ctx context.Context) (*Service, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	s := &Service{svc: preservation.NewService(ctx, cfg), cfg: cfg}
	return s, nil
}

func (s *Service) Close(ctx context.Context) {
	s.svc.Close(ctx)
}

func (s *Service) Run(ctx context.Context, username string, paths []string, cleanup bool) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(paths))

	// Number of concurrent operations
	maxWorkers := 3
	semaphore := make(chan struct{}, maxWorkers)

	for _, packagePath := range paths {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			semaphore <- struct{}{}
			if err := s.svc.Run(ctx, &config.PreservationConfig{CompressAip: false}, username, path, cleanup); err != nil {
				errChan <- err
			}
			<-semaphore
		}(packagePath)
	}

	wg.Wait()
	close(errChan)

	// Collect errors
	var hasErrors bool
	for err := range errChan {
		log.Printf("Error running preservation for a package: %v\n", err)
		hasErrors = true
	}

	if hasErrors {
		return fmt.Errorf("preservation process completed with errors")
	}

	return nil
}
