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
	addr    string
	cleanup bool
	serve   bool

	// Pydio Cells
	cells_archiveDir string
	cells_paths      []string
	cells_username   string

	// Preservations Config
	compressAip                                      bool
	a3m_assignUuidsToDirectories                     bool
	a3m_examineContents                              bool
	a3m_generateTransferStructureReport              bool
	a3m_documentEmptyDirectories                     bool
	a3m_extractPackages                              bool
	a3m_deletePackagesAfterExtraction                bool
	a3m_identifyTransfer                             bool
	a3m_identifySubmissionAndMetadata                bool
	a3m_identifyBeforeNormalization                  bool
	a3m_normalize                                    bool
	a3m_transcribeFiles                              bool
	a3m_performPolicyChecksOnOriginals               bool
	a3m_performPolicyChecksOnPreservationDerivatives bool
	a3m_performPolicyChecksOnAccessDerivatives       bool
	a3m_thumbnailModeStr                             string
	a3m_thumbnailMode                                transferservice.ProcessingConfig_ThumbnailMode
	a3m_aipCompressionLevel                          int32
	a3m_aipCompressionAlgorithm                      transferservice.ProcessingConfig_AIPCompressionAlgorithm

	// AtoM Config
	atom_host          string
	atom_apiKey        string
	atom_loginEmail    string
	atom_loginPassword string
	atom_rsyncTarget   string
	atom_rsyncCommand  string
	atom_slug          string
)

var RootCmd = &cobra.Command{
	Use:   "ca4m",
	Short: "Cells A4M",
	Long: `Cells A4M (A3M + DIP Generation)

Integrates with Pydio Cells and A3M to provide functionality to Cells for preserving packages.
If the --serve flag is provided, the tool will start a HTTP server.
Otherwise, the tool can be used in the CLI to preserve packages by providing the --path and --username flags.
Environment configuration is loaded from the environment variables.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Create a root context
		ctx := context.Background()

		startTime := time.Now()

		cfg, err := config.Load()
		if err != nil {
			logger.Fatal("Error loading configuration:\n%v", err)
		}

		// Initialize the logger
		logger.Initialize(cfg.LogLevel)
		// Only log the execution time once the logger is initialized
		defer func() {
			logger.Debug("Execution time: %vs", time.Since(startTime).Seconds())
		}()

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
				AssignUuidsToDirectories:                     a3m_assignUuidsToDirectories,
				ExamineContents:                              a3m_examineContents,
				GenerateTransferStructureReport:              a3m_generateTransferStructureReport,
				DocumentEmptyDirectories:                     a3m_documentEmptyDirectories,
				ExtractPackages:                              a3m_extractPackages,
				DeletePackagesAfterExtraction:                a3m_deletePackagesAfterExtraction,
				IdentifyTransfer:                             a3m_identifyTransfer,
				IdentifySubmissionAndMetadata:                a3m_identifySubmissionAndMetadata,
				IdentifyBeforeNormalization:                  a3m_identifyBeforeNormalization,
				Normalize:                                    a3m_normalize,
				TranscribeFiles:                              a3m_transcribeFiles,
				PerformPolicyChecksOnOriginals:               a3m_performPolicyChecksOnOriginals,
				PerformPolicyChecksOnPreservationDerivatives: a3m_performPolicyChecksOnPreservationDerivatives,
				PerformPolicyChecksOnAccessDerivatives:       a3m_performPolicyChecksOnAccessDerivatives,
				ThumbnailMode:                                a3m_thumbnailMode,
				AipCompressionLevel:                          a3m_aipCompressionLevel,
				AipCompressionAlgorithm:                      a3m_aipCompressionAlgorithm,
			},
			AtomConfig: &config.AtomConfig{
				Host:          atom_host,
				ApiKey:        atom_apiKey,
				LoginEmail:    atom_loginEmail,
				LoginPassword: atom_loginPassword,
				RsyncTarget:   atom_rsyncTarget,
				RsyncCommand:  atom_rsyncCommand,
				Slug:          atom_slug,
			},
		}

		svcArgs := internal.ServiceArgs{
			CellsArchiveDir: cells_archiveDir,
			CellsPaths:      cells_paths,
			CellsUsername:   cells_username,
			Cleanup:         cleanup,
			PreservationCfg: &preservationCfg,
		}

		if err := svc.RunArgs(ctx, &svcArgs); err != nil {
			logger.Debug("Error running preservation: %v", err)
		}
	},
}

func init() {
	cobra.OnInitialize(config.Init)

	defaultPreservationCfg := config.DefaultPreservationConfig()

	RootCmd.Flags().BoolVar(&serve, "serve", false, "Start HTTP server")
	RootCmd.Flags().StringVar(&addr, "addr", ":6905", "HTTP listen address (with --serve)")
	RootCmd.Flags().BoolVar(&cleanup, "cleanup", true, "Cleanup after run")

	// Cells
	RootCmd.Flags().StringSliceVarP(&cells_paths, "cells-path", "p", nil, "Cells paths to preserve. can provide multiple.")
	RootCmd.Flags().StringVarP(&cells_username, "cells-username", "u", "", "Cells username (required)")
	RootCmd.Flags().StringVarP(&cells_archiveDir, "cells-archive-dir", "a", "common-files", "Cells archive directory")

	// Preservation
	RootCmd.Flags().BoolVar(&compressAip, "compress-aip", defaultPreservationCfg.CompressAip, "Compress AIP")
	// A3M
	RootCmd.Flags().BoolVar(&a3m_assignUuidsToDirectories, "a3m-assign-uuids-to-directories", defaultPreservationCfg.A3mConfig.AssignUuidsToDirectories, "Assign UUIDs to directories")
	RootCmd.Flags().BoolVar(&a3m_examineContents, "a3m-examine-contents", defaultPreservationCfg.A3mConfig.ExamineContents, "Examine contents")
	RootCmd.Flags().BoolVar(&a3m_generateTransferStructureReport, "a3m-generate-transfer-struct-report", defaultPreservationCfg.A3mConfig.GenerateTransferStructureReport, "Generate transfer struct report")
	RootCmd.Flags().BoolVar(&a3m_documentEmptyDirectories, "a3m-document-empty-directories", defaultPreservationCfg.A3mConfig.DocumentEmptyDirectories, "Document empty directories")
	RootCmd.Flags().BoolVar(&a3m_extractPackages, "a3m-extract-packages", defaultPreservationCfg.A3mConfig.ExtractPackages, "Extract packages")
	RootCmd.Flags().BoolVar(&a3m_deletePackagesAfterExtraction, "a3m-delete-packages-after-extraction", defaultPreservationCfg.A3mConfig.DeletePackagesAfterExtraction, "Delete packages after extraction")
	RootCmd.Flags().BoolVar(&a3m_identifyTransfer, "a3m-identify-transfer", defaultPreservationCfg.A3mConfig.IdentifyTransfer, "Identify transfer")
	RootCmd.Flags().BoolVar(&a3m_identifySubmissionAndMetadata, "a3m-identify-submission-and-metadata", defaultPreservationCfg.A3mConfig.IdentifySubmissionAndMetadata, "Identify submission and metadata")
	RootCmd.Flags().BoolVar(&a3m_identifyBeforeNormalization, "a3m-identify-before-normalization", defaultPreservationCfg.A3mConfig.IdentifyBeforeNormalization, "Identify before normalization")
	RootCmd.Flags().BoolVar(&a3m_normalize, "a3m-normalize", defaultPreservationCfg.A3mConfig.Normalize, "Normalize")
	RootCmd.Flags().BoolVar(&a3m_transcribeFiles, "a3m-transcribe-files", defaultPreservationCfg.A3mConfig.TranscribeFiles, "Transcribe files")
	RootCmd.Flags().BoolVar(&a3m_performPolicyChecksOnOriginals, "a3m-perform-policy-checks-on-originals", defaultPreservationCfg.A3mConfig.PerformPolicyChecksOnOriginals, "Perform policy checks on originals")
	RootCmd.Flags().BoolVar(&a3m_performPolicyChecksOnPreservationDerivatives, "a3m-perform-policy-checks-on-preservation-derivatives", defaultPreservationCfg.A3mConfig.PerformPolicyChecksOnPreservationDerivatives, "Perform policy checks on preservation derivatives")
	RootCmd.Flags().BoolVar(&a3m_performPolicyChecksOnAccessDerivatives, "a3m-perform-policy-checks-on-access-derivatives", defaultPreservationCfg.A3mConfig.PerformPolicyChecksOnAccessDerivatives, "Perform policy checks on access derivatives")
	RootCmd.Flags().StringVar(&a3m_thumbnailModeStr, "a3m-thumbnail-mode", defaultPreservationCfg.A3mConfig.ThumbnailMode.String(), "Thumbnail mode (generate, generate_non_default, do_not_generate)")
	// AtoM Config
	RootCmd.Flags().StringVar(&atom_host, "atom-host", defaultPreservationCfg.AtomConfig.Host, "AtoM host")
	RootCmd.Flags().StringVar(&atom_apiKey, "atom-api-key", defaultPreservationCfg.AtomConfig.ApiKey, "AtoM API key")
	RootCmd.Flags().StringVar(&atom_loginEmail, "atom-login-email", defaultPreservationCfg.AtomConfig.LoginEmail, "AtoM login email")
	RootCmd.Flags().StringVar(&atom_loginPassword, "atom-login-password", defaultPreservationCfg.AtomConfig.LoginPassword, "AtoM login password")
	RootCmd.Flags().StringVar(&atom_rsyncTarget, "atom-rsync-target", defaultPreservationCfg.AtomConfig.RsyncTarget, "AtoM rsync target")
	RootCmd.Flags().StringVar(&atom_rsyncCommand, "atom-rsync-command", defaultPreservationCfg.AtomConfig.RsyncCommand, "AtoM rsync command")
	RootCmd.Flags().StringVar(&atom_slug, "atom-slug", defaultPreservationCfg.AtomConfig.Slug, "AtoM digital object slug")

	// TODO: Compression variables should be set at the processing config level (not a3m) as a3m compression is invisible to the user
	a3m_aipCompressionLevel = 1
	a3m_aipCompressionAlgorithm = transferservice.ProcessingConfig_AIP_COMPRESSION_ALGORITHM_S7_COPY

	// Convert thumbnail mode string to enum
	cobra.OnInitialize(func() {
		switch a3m_thumbnailModeStr {
		case "generate":
			a3m_thumbnailMode = transferservice.ProcessingConfig_THUMBNAIL_MODE_GENERATE
		case "generate_non_default":
			a3m_thumbnailMode = transferservice.ProcessingConfig_THUMBNAIL_MODE_GENERATE_NON_DEFAULT
		case "do_not_generate":
			a3m_thumbnailMode = transferservice.ProcessingConfig_THUMBNAIL_MODE_DO_NOT_GENERATE
		default:
			a3m_thumbnailMode = transferservice.ProcessingConfig_THUMBNAIL_MODE_UNSPECIFIED
		}
	})

	// Conditionally mark flags as required
	RootCmd.PreRun = func(cmd *cobra.Command, args []string) {
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
