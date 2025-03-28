package cmd

import (
	"context"
	"log"
	"time"

	"github.com/penwern/preservation-go/internal"
	"github.com/penwern/preservation-go/pkg/config"
	"github.com/penwern/preservation-go/pkg/logger"
	"github.com/spf13/cobra"
)

var (
	serve      bool
	addr       string
	cleanup    bool
	paths      []string
	username   string
	archiveDir string
)

var RootCmd = &cobra.Command{
	Use:   "Penwern Preservation Tools",
	Short: "Pydio Cells Preservation Tool",
	Long: `Pydio Cells Preservation Tool.
Integrates with Pydio Cells and A3M to provide functionality to Pydio Cells for preserving packages.
If the --serve flag is provided, the tool will start an HTTP server.
Otherwise, the tool can be used in the CLI to preserve packages by providing the --path and --username flags.
Environment configuration is loaded from the environment variables.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Create a root context
		ctx := context.Background()

		startTime := time.Now()

		cfg, err := config.Load()
		if err != nil {
			log.Fatalf("Error loading configuration: %v\n", err)
		}

		// Initialize the logger
		logger.Initialize(cfg.LogLevel)
		// Only log the execution time if the logger is initialized
		defer func() {
			logger.Debug("Execution time: %v\n", time.Since(startTime))
		}()

		svc, err := internal.NewService(ctx, cfg)
		if err != nil {
			logger.Fatal("Error creating service: %v\n", err)
		}
		defer svc.Close()

		// Handle serve mode
		if serve {
			logger.Info("Starting HTTP server on %s\n", addr)
			log.Fatal(internal.Serve(svc, addr))
		}

		srcArgs := internal.ServiceArgs{
			Username:   username,
			Paths:      paths,
			Cleanup:    cleanup,
			ArchiveDir: archiveDir,
		}
		if err := svc.RunArgs(ctx, &srcArgs); err != nil {
			logger.Fatal("Error running preservation: %v\n", err)
		}
	},
}

func init() {
	RootCmd.Flags().BoolVar(&serve, "serve", false, "start HTTP server")
	RootCmd.Flags().StringVar(&addr, "addr", ":6905", "HTTP listen address")
	RootCmd.Flags().StringSliceVarP(&paths, "path", "p", nil, "cells paths to preserve")
	RootCmd.Flags().StringVarP(&username, "username", "u", "", "cells username (required)")
	RootCmd.Flags().BoolVar(&cleanup, "cleanup", true, "cleanup after run")
	RootCmd.Flags().StringVarP(&archiveDir, "archive-dir", "a", "common-files", "cells archive directory")

	// Conditionally mark flags as required
	RootCmd.PreRun = func(cmd *cobra.Command, args []string) {
		if !serve {
			if err := cmd.MarkFlagRequired("username"); err != nil {
				log.Fatalf("Error marking username as required: %v", err)
			}
			if err := cmd.MarkFlagRequired("path"); err != nil {
				log.Fatalf("Error marking path as required: %v", err)
			}
		}
	}
}
