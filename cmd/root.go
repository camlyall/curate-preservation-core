// Package cmd provides the root command for the Cells A4M CLI tool.
// It integrates with Pydio Cells and A3M to provide functionality for preserving packages.
// It can run in CLI mode to preserve packages or in server mode to provide a HTTP API.
// It loads configuration from environment variables and command line flags.
// It also provides a version command to display the current build information.
package cmd

import (
	"context"
	"time"

	transferservice "github.com/penwern/curate-preservation-core/common/proto/a3m/gen/go/a3m/api/transferservice/v1beta1"
	"github.com/penwern/curate-preservation-core/internal"
	"github.com/penwern/curate-preservation-core/pkg/config"
	"github.com/penwern/curate-preservation-core/pkg/logger"
	"github.com/spf13/cobra"
)

var (
	addr             string
	cleanup          bool
	serve            bool
	allowInsecureTLS bool

	// Pydio Cells
	cellsArchiveDir string
	cellsPaths      []string
	cellsUsername   string

	// Preservations Config
	compressAip                                     bool
	a3mAssignUuidsToDirectories                     bool
	a3mExamineContents                              bool
	a3mGenerateTransferStructureReport              bool
	a3mDocumentEmptyDirectories                     bool
	a3mExtractPackages                              bool
	a3mDeletePackagesAfterExtraction                bool
	a3mIdentifyTransfer                             bool
	a3mIdentifySubmissionAndMetadata                bool
	a3mIdentifyBeforeNormalization                  bool
	a3mNormalize                                    bool
	a3mTranscribeFiles                              bool
	a3mPerformPolicyChecksOnOriginals               bool
	a3mPerformPolicyChecksOnPreservationDerivatives bool
	a3mPerformPolicyChecksOnAccessDerivatives       bool
	a3mThumbnailModeStr                             string
	a3mThumbnailMode                                transferservice.ProcessingConfig_ThumbnailMode
	a3mAipCompressionLevel                          int32
	a3mAipCompressionAlgorithm                      transferservice.ProcessingConfig_AIPCompressionAlgorithm

	// AtoM Config
	atomHost          string
	atomAPIKey        string
	atomLoginEmail    string
	atomLoginPassword string
	atomRsyncTarget   string
	atomRsyncCommand  string
	atomSlug          string
)

// RootCmd is the root command for the preservation service.
var RootCmd = &cobra.Command{
	Use:   "ca4m",
	Short: "Cells A4M",
	Long: `Cells A4M (A3M + DIP Generation)

Integrates with Pydio Cells and A3M to provide functionality to Cells for preserving packages.
If the --serve flag is provided, the tool will start a HTTP server.
Otherwise, the tool can be used in the CLI to preserve packages by providing the --path and --username flags.
Environment configuration is loaded from the environment variables.`,
	Run: func(_ *cobra.Command, _ []string) {
		// Create a root context
		ctx := context.Background()

		startTime := time.Now()

		cfg, err := config.Load()
		if err != nil {
			logger.Fatal("Error loading configuration:\n%v", err)
		}

		// Initialize the logger
		logger.Initialize(cfg.LogLevel, cfg.LogFilePath)
		// Only log the execution time once the logger is initialized
		defer func() {
			logger.Debug("Execution time: %vs", time.Since(startTime).Seconds())
		}()

		// Override config with CLI flag if provided
		if allowInsecureTLS {
			cfg.AllowInsecureTLS = allowInsecureTLS
		}
		if cleanup {
			cfg.Cleanup = cleanup
		}

		// Create CLI AtoM config from flags
		cliAtomConfig := &config.AtomConfig{
			Host:          atomHost,
			APIKey:        atomAPIKey,
			LoginEmail:    atomLoginEmail,
			LoginPassword: atomLoginPassword,
			RsyncTarget:   atomRsyncTarget,
			RsyncCommand:  atomRsyncCommand,
			Slug:          atomSlug,
		}

		// Get final AtoM config with proper priority
		finalAtomConfig, err := config.GetAtomConfig(cfg, cliAtomConfig)
		if err != nil {
			logger.Fatal("Failed to load AtoM configuration: %v", err)
		}

		svc, err := internal.NewService(ctx, cfg)
		if err != nil {
			logger.Fatal("Error creating service: %v", err)
		}
		defer svc.Close()

		// Handle serve mode
		if serve {
			logger.Info("Starting HTTP server on %s", addr)
			if err := internal.Serve(svc, addr); err != nil {
				logger.Fatal("Error starting HTTP server: %v", err)
			}
			return
		}

		preservationCfg := config.PreservationConfig{
			CompressAip: compressAip,
			A3mConfig: &transferservice.ProcessingConfig{
				AssignUuidsToDirectories:                     a3mAssignUuidsToDirectories,
				ExamineContents:                              a3mExamineContents,
				GenerateTransferStructureReport:              a3mGenerateTransferStructureReport,
				DocumentEmptyDirectories:                     a3mDocumentEmptyDirectories,
				ExtractPackages:                              a3mExtractPackages,
				DeletePackagesAfterExtraction:                a3mDeletePackagesAfterExtraction,
				IdentifyTransfer:                             a3mIdentifyTransfer,
				IdentifySubmissionAndMetadata:                a3mIdentifySubmissionAndMetadata,
				IdentifyBeforeNormalization:                  a3mIdentifyBeforeNormalization,
				Normalize:                                    a3mNormalize,
				TranscribeFiles:                              a3mTranscribeFiles,
				PerformPolicyChecksOnOriginals:               a3mPerformPolicyChecksOnOriginals,
				PerformPolicyChecksOnPreservationDerivatives: a3mPerformPolicyChecksOnPreservationDerivatives,
				PerformPolicyChecksOnAccessDerivatives:       a3mPerformPolicyChecksOnAccessDerivatives,
				ThumbnailMode:                                a3mThumbnailMode,
				AipCompressionLevel:                          a3mAipCompressionLevel,
				AipCompressionAlgorithm:                      a3mAipCompressionAlgorithm,
			},
		}

		svcArgs := internal.ServiceArgs{
			AllowInsecureTLS: allowInsecureTLS,
			CellsArchiveDir:  cellsArchiveDir,
			CellsPaths:       cellsPaths,
			CellsUsername:    cellsUsername,
			Cleanup:          cleanup,
			PreservationCfg:  &preservationCfg,
			AtomCfg:          finalAtomConfig,
		}

		if err := svc.RunArgs(ctx, &svcArgs); err != nil {
			logger.Debug("Error running preservation: %v", err)
		}
	},
}

func init() {
	cobra.OnInitialize(config.Init)

	defaultPreservationCfg := config.DefaultPreservationConfig()
	defaultAtomCfg := config.DefaultAtomConfig()

	// Add version command
	RootCmd.AddCommand(versionCmd)

	RootCmd.Flags().BoolVar(&serve, "serve", false, "Start HTTP server")
	RootCmd.Flags().StringVar(&addr, "addr", ":6905", "HTTP listen address (with --serve)")
	RootCmd.Flags().BoolVar(&cleanup, "cleanup", true, "Cleanup after run")
	RootCmd.Flags().BoolVar(&allowInsecureTLS, "allow-insecure-tls", false, "Allow insecure TLS connections (for testing only)")

	// Cells
	RootCmd.Flags().StringSliceVarP(&cellsPaths, "cells-path", "p", nil, "Cells paths to preserve. can provide multiple.")
	RootCmd.Flags().StringVarP(&cellsUsername, "cells-username", "u", "", "Cells username (required)")
	RootCmd.Flags().StringVarP(&cellsArchiveDir, "cells-archive-dir", "a", "common-files", "Cells archive directory")

	// Preservation
	RootCmd.Flags().BoolVar(&compressAip, "compress-aip", defaultPreservationCfg.CompressAip, "Compress AIP")
	// A3M
	RootCmd.Flags().BoolVar(&a3mAssignUuidsToDirectories, "a3m-assign-uuids-to-directories", defaultPreservationCfg.A3mConfig.AssignUuidsToDirectories, "Assign UUIDs to directories")
	RootCmd.Flags().BoolVar(&a3mExamineContents, "a3m-examine-contents", defaultPreservationCfg.A3mConfig.ExamineContents, "Examine contents")
	RootCmd.Flags().BoolVar(&a3mGenerateTransferStructureReport, "a3m-generate-transfer-struct-report", defaultPreservationCfg.A3mConfig.GenerateTransferStructureReport, "Generate transfer struct report")
	RootCmd.Flags().BoolVar(&a3mDocumentEmptyDirectories, "a3m-document-empty-directories", defaultPreservationCfg.A3mConfig.DocumentEmptyDirectories, "Document empty directories")
	RootCmd.Flags().BoolVar(&a3mExtractPackages, "a3m-extract-packages", defaultPreservationCfg.A3mConfig.ExtractPackages, "Extract packages")
	RootCmd.Flags().BoolVar(&a3mDeletePackagesAfterExtraction, "a3m-delete-packages-after-extraction", defaultPreservationCfg.A3mConfig.DeletePackagesAfterExtraction, "Delete packages after extraction")
	RootCmd.Flags().BoolVar(&a3mIdentifyTransfer, "a3m-identify-transfer", defaultPreservationCfg.A3mConfig.IdentifyTransfer, "Identify transfer")
	RootCmd.Flags().BoolVar(&a3mIdentifySubmissionAndMetadata, "a3m-identify-submission-and-metadata", defaultPreservationCfg.A3mConfig.IdentifySubmissionAndMetadata, "Identify submission and metadata")
	RootCmd.Flags().BoolVar(&a3mIdentifyBeforeNormalization, "a3m-identify-before-normalization", defaultPreservationCfg.A3mConfig.IdentifyBeforeNormalization, "Identify before normalization")
	RootCmd.Flags().BoolVar(&a3mNormalize, "a3m-normalize", defaultPreservationCfg.A3mConfig.Normalize, "Normalize")
	RootCmd.Flags().BoolVar(&a3mTranscribeFiles, "a3m-transcribe-files", defaultPreservationCfg.A3mConfig.TranscribeFiles, "Transcribe files")
	RootCmd.Flags().BoolVar(&a3mPerformPolicyChecksOnOriginals, "a3m-perform-policy-checks-on-originals", defaultPreservationCfg.A3mConfig.PerformPolicyChecksOnOriginals, "Perform policy checks on originals")
	RootCmd.Flags().BoolVar(&a3mPerformPolicyChecksOnPreservationDerivatives, "a3m-perform-policy-checks-on-preservation-derivatives", defaultPreservationCfg.A3mConfig.PerformPolicyChecksOnPreservationDerivatives, "Perform policy checks on preservation derivatives")
	RootCmd.Flags().BoolVar(&a3mPerformPolicyChecksOnAccessDerivatives, "a3m-perform-policy-checks-on-access-derivatives", defaultPreservationCfg.A3mConfig.PerformPolicyChecksOnAccessDerivatives, "Perform policy checks on access derivatives")
	RootCmd.Flags().StringVar(&a3mThumbnailModeStr, "a3m-thumbnail-mode", defaultPreservationCfg.A3mConfig.ThumbnailMode.String(), "Thumbnail mode (generate, generate_non_default, do_not_generate)")

	// AtoM Config
	RootCmd.Flags().StringVar(&atomHost, "atom-host", defaultAtomCfg.Host, "AtoM host")
	RootCmd.Flags().StringVar(&atomAPIKey, "atom-api-key", defaultAtomCfg.APIKey, "AtoM API key")
	RootCmd.Flags().StringVar(&atomLoginEmail, "atom-login-email", defaultAtomCfg.LoginEmail, "AtoM login email")
	RootCmd.Flags().StringVar(&atomLoginPassword, "atom-login-password", defaultAtomCfg.LoginPassword, "AtoM login password")
	RootCmd.Flags().StringVar(&atomRsyncTarget, "atom-rsync-target", defaultAtomCfg.RsyncTarget, "AtoM rsync target")
	RootCmd.Flags().StringVar(&atomRsyncCommand, "atom-rsync-command", defaultAtomCfg.RsyncCommand, "AtoM rsync command")
	RootCmd.Flags().StringVar(&atomSlug, "atom-slug", defaultAtomCfg.Slug, "AtoM digital object slug")

	// TODO: Compression variables should be set at the processing config level (not a3m) as a3m compression is invisible to the user
	a3mAipCompressionLevel = 1
	a3mAipCompressionAlgorithm = transferservice.ProcessingConfig_AIP_COMPRESSION_ALGORITHM_S7_COPY

	// Convert thumbnail mode string to enum
	cobra.OnInitialize(func() {
		switch a3mThumbnailModeStr {
		case "generate":
			a3mThumbnailMode = transferservice.ProcessingConfig_THUMBNAIL_MODE_GENERATE
		case "generate_non_default":
			a3mThumbnailMode = transferservice.ProcessingConfig_THUMBNAIL_MODE_GENERATE_NON_DEFAULT
		case "do_not_generate":
			a3mThumbnailMode = transferservice.ProcessingConfig_THUMBNAIL_MODE_DO_NOT_GENERATE
		default:
			a3mThumbnailMode = transferservice.ProcessingConfig_THUMBNAIL_MODE_UNSPECIFIED
		}
	})

	// Conditionally mark flags as required
	RootCmd.PreRun = func(cmd *cobra.Command, _ []string) {
		if !serve {
			if err := cmd.MarkFlagRequired("cells-username"); err != nil {
				logger.Fatal("Error marking username as required: %v", err)
			}
			if err := cmd.MarkFlagRequired("cells-path"); err != nil {
				logger.Fatal("Error marking path as required: %v", err)
			}
		}
	}
}
