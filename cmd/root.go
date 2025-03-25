package cmd

import (
	"context"
	"log"
	"os"

	"github.com/penwern/preservation-go/internal"
	"github.com/spf13/cobra"
)

var (
	serve    bool
	addr     string
	cleanup  bool
	paths    []string
	username string
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
		svc, err := internal.NewService(ctx)
		if err != nil {
			log.Printf("Error creating service: %v\n", err)
			os.Exit(1)
		}
		defer svc.Close(ctx)

		// Handle serve mode
		if serve {
			log.Printf("Starting HTTP server on %s\n", addr)
			log.Fatal(internal.Serve(svc, addr))
		}

		// Delegate the Run logic to the service
		if err := svc.Run(ctx, username, paths, cleanup); err != nil {
			log.Fatalf("Error running preservation: %v\n", err)
		}

		log.Println("Preservation process completed successfully")
	},
}

func init() {
	RootCmd.Flags().BoolVar(&serve, "serve", false, "start HTTP server")
	RootCmd.Flags().StringVar(&addr, "addr", ":6905", "HTTP listen address")
	RootCmd.Flags().StringSliceVarP(&paths, "path", "p", nil, "paths to preserve")
	RootCmd.Flags().StringVarP(&username, "username", "u", "", "username (required)")
	RootCmd.Flags().BoolVar(&cleanup, "cleanup", true, "cleanup after run")

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
