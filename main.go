package main

import (
	"context"
	"log"
	"os"
	"sync"

	"github.com/penwern/preservation-go/internal/preservation"
	"github.com/penwern/preservation-go/pkg/config"
	"github.com/spf13/cobra"
)

func main() {
	var cleanup bool
	var paths []string
	var username string

	// Define the root command
	var rootCmd = &cobra.Command{
		Use:   "Penwern Preservation",
		Short: "Pydio Cells Preservation tool",
		Run: func(cmd *cobra.Command, args []string) {
			// Load and validate environment configuration
			cfg, err := config.Load()
			if err != nil {
				log.Printf("Configuration error: %v\n", err)
				os.Exit(1)
			}

			// Create a root context
			ctx := context.Background()
			preservationService := preservation.NewService(ctx, cfg)
			defer preservationService.Close(ctx)

			// Load and validate the processing configuration
			// TODO: Populate from database
			pcfg := config.PreservationConfig{
				CompressAip: false,
			}

			var wg sync.WaitGroup
			errChan := make(chan error, len(paths))

			// Allow limiting the number of concurrent operations
			maxWorkers := 3
			semaphore := make(chan struct{}, maxWorkers)

			for _, packagePath := range paths {
				wg.Add(1)
				go func(path string) {
					defer wg.Done()
					semaphore <- struct{}{}
					if err := preservationService.Run(ctx, &pcfg, username, path, cleanup); err != nil {
						errChan <- err
					}
					<-semaphore
				}(packagePath)
			}

			wg.Wait()
			close(errChan)

			// Return a non-zero exit code on failure
			hasErrors := false
			for err := range errChan {
				log.Printf("Error running preservation for a package: %v\n", err)
				hasErrors = true
			}

			if hasErrors {
				log.Println("Preservation process completed with errors")
				os.Exit(1)
			}

			log.Println("Preservation process completed successfully")
		},
	}

	// Define flags
	rootCmd.Flags().BoolVarP(&cleanup, "cleanup", "c", true, "enable or disable cleanup after processing")
	rootCmd.Flags().StringArrayVarP(&paths, "path", "p", []string{}, "path to the user relative Pydio Cells package for preservation (can be specified multiple times)")
	rootCmd.Flags().StringVarP(&username, "username", "u", "", "username for the preservation process (required)")

	// Mark required flags
	rootCmd.MarkFlagRequired("username")
	rootCmd.MarkFlagRequired("path")

	// Execute the root command
	if err := rootCmd.Execute(); err != nil {
		log.Printf("Error executing command: %v\n", err)
		os.Exit(1)
	}
}
