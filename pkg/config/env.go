package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/penwern/preservation-go/pkg/utils"
)

var defaultConfig = struct {
	A3mCompletedDir       string
	CecPath               string
	CellsAddress          string
	A3mAddress            string
	CellsArchiveWorkspace string
}{
	A3mCompletedDir:       "/home/a3m/.local/share/a3m/share/completed",
	CecPath:               "/usr/bin/cec",
	CellsAddress:          "https://localhost:8080",
	A3mAddress:            "localhost:7000",
	CellsArchiveWorkspace: "common-files",
}

// Config holds the configuration for the application.
type Config struct {
	ProcessingBaseDir     string // Base directory for processing. Required
	A3mAddress            string // gRPC address.
	A3mCompletedDir       string // Directory of completed A3M AIPs.
	CellsCecPath          string // Path to cec binary.
	CellsAddress          string // HTTP address of Cells.
	CellsArchiveWorkspace string // Cells path to upload the AIP. Overwritten by input if set.
	CellsAdminToken       string // Cells admin personal access token. Overwritten by input if set.
}

// loadEnvWithDefault loads an environment variable or returns a default value if not set.
func loadEnvWithDefault(envVar, defaultValue string) string {
	value, ok := os.LookupEnv(envVar)
	if !ok || value == "" {
		log.Printf("%s environment variable is not set. Defaulting to %q\n", envVar, defaultValue)
		return defaultValue
	}
	return value
}

// Load loads and validates configuration from environment variables.
func Load() (*Config, error) {

	// Load the .env file if not in production - doesn't override existing env vars
	if os.Getenv("GO_ENV") != "production" {
		if err := godotenv.Load(); err != nil {
			log.Printf("No .env file found: %v\n", err)
		} else {
			log.Printf("Loaded configuration from .env file\n")
		}
	}

	processingDir := os.Getenv("PROCESSING_BASE_DIR")
	if processingDir == "" {
		return nil, fmt.Errorf("missing required environment variable: PROCESSING_BASE_DIR")
	}
	absProcessingDir, err := utils.ValidateDirectory(processingDir)
	if err != nil {
		return nil, fmt.Errorf("invalid processing directory: %w", err)
	}

	a3mCompletedDir := loadEnvWithDefault("A3M_COMPLETED_DIR", defaultConfig.A3mCompletedDir)
	absA3mCompletedDir, err := utils.ValidateDirectory(a3mCompletedDir)
	if err != nil {
		return nil, fmt.Errorf("invalid A3M completed directory: %w", err)
	}

	cecPath := loadEnvWithDefault("CELLS_CEC_PATH", defaultConfig.CecPath)
	absCecPath, err := utils.ValidateExecutable(cecPath)
	if err != nil {
		return nil, fmt.Errorf("invalid cec binary path: %w", err)
	}

	cellsAddress := loadEnvWithDefault("CELLS_ENDPOINT", defaultConfig.CellsAddress)
	if err := utils.CheckHTTPConnection(cellsAddress); err != nil {
		return nil, fmt.Errorf("error connecting to Cells at %q: %w", cellsAddress, err)
	}

	a3mAddress := loadEnvWithDefault("A3M_ADDRESS", defaultConfig.A3mAddress)
	if err := utils.CheckGRPCConnection(a3mAddress); err != nil {
		return nil, fmt.Errorf("error connecting to A3M at %q: %w", a3mAddress, err)
	}

	cellsArchiveWorkspace := loadEnvWithDefault("CELLS_ARCHIVE_WORKSPACE", defaultConfig.CellsArchiveWorkspace)

	cellsAdminToken := os.Getenv("CELLS_ADMIN_TOKEN")
	if cellsAdminToken == "" {
		log.Printf("CELLS_ADMIN_TOKEN not set in environment. Expecting it to be provided as input.")
	}

	return &Config{
		ProcessingBaseDir:     absProcessingDir,
		A3mAddress:            a3mAddress,
		A3mCompletedDir:       absA3mCompletedDir,
		CellsCecPath:          absCecPath,
		CellsAddress:          cellsAddress,
		CellsArchiveWorkspace: cellsArchiveWorkspace,
		CellsAdminToken:       cellsAdminToken,
	}, nil
}
