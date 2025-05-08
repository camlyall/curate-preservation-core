package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/penwern/preservation-go/pkg/logger"
	"github.com/penwern/preservation-go/pkg/utils"
)

const envPrefix = "CA4M_"

var defaultConfig = struct {
	A3mAddress            string
	A3mCompletedDir       string
	A3mDipsDir            string
	CecPath               string
	CellsAddress          string
	CellsArchiveWorkspace string
}{
	A3mAddress:            "localhost:7000",
	A3mCompletedDir:       "/home/a3m/.local/share/a3m/share/completed",
	A3mDipsDir:            "/home/a3m/.local/share/a3m/share/dips",
	CecPath:               "/usr/local/bin/cec",
	CellsAddress:          "https://localhost:8080",
	CellsArchiveWorkspace: "common-files",
}

// EnvConfig holds the configuration for the application.
type EnvConfig struct {
	A3mAddress            string // gRPC address.
	A3mCompletedDir       string // Directory of completed A3M AIPs.
	A3mDipsDir            string // Directory of completed A3M DIPs.
	CellsAddress          string // HTTP address of Cells.
	CellsAdminToken       string // Cells admin personal access token. Overwritten by input if set.
	CellsArchiveWorkspace string // Cells path to upload the AIP. Overwritten by input if set. TODO: Override in input args
	CellsCecPath          string // Path to cec binary.
	LogLevel              string // Log level.
	ProcessingBaseDir     string // Base directory for processing. Required
}

// loadEnvWithDefault loads an environment variable with prefix or returns a default value if not set.
func loadEnvWithDefault(envVar, defaultValue string, log bool) string {
	prefixedVar := envPrefix + envVar
	value, ok := os.LookupEnv(prefixedVar)
	if !ok || value == "" {
		if log {
			logger.Debug("%s environment variable is not set. Defaulting to %q", prefixedVar, defaultValue)
		}
		return defaultValue
	}
	return value
}

// Load loads and validates configuration from environment variables.
func Load() (*EnvConfig, error) {

	// Load the .env file if not in production - doesn't override existing env vars
	if os.Getenv("GO_ENV") != "production" {
		if err := godotenv.Load(); err != nil {
			logger.Warn("No .env file found: %v", err)
		} else {
			logger.Debug("Loaded configuration from .env file")
		}
	}

	processingDir := os.Getenv(envPrefix + "PROCESSING_BASE_DIR")
	if processingDir == "" {
		return nil, fmt.Errorf("missing required environment variable: %sPROCESSING_BASE_DIR", envPrefix)
	}
	absProcessingDir, err := utils.ValidateDirectory(processingDir)
	if err != nil {
		return nil, fmt.Errorf("invalid processing directory: %w", err)
	}

	a3mCompletedDir := loadEnvWithDefault("A3M_COMPLETED_DIR", defaultConfig.A3mCompletedDir, true)
	absA3mCompletedDir, err := utils.ValidateDirectory(a3mCompletedDir)
	if err != nil {
		return nil, fmt.Errorf("invalid A3M completed directory: %w", err)
	}

	a3mDipsDir := loadEnvWithDefault("A3M_DIPS_DIR", defaultConfig.A3mDipsDir, true)
	absA3mDipsDir, err := utils.ValidateDirectory(a3mDipsDir)
	if err != nil {
		return nil, fmt.Errorf("invalid A3M completed directory: %w", err)
	}

	cecPath := loadEnvWithDefault("CELLS_CEC_PATH", defaultConfig.CecPath, true)
	absCecPath, err := utils.ValidateExecutable(cecPath)
	if err != nil {
		return nil, fmt.Errorf("invalid cec binary path: %w", err)
	}

	logLevel := loadEnvWithDefault("LOG_LEVEL", "info", false)
	if !utils.ValidateLogLevel(logLevel) {
		return nil, fmt.Errorf("invalid log level: %s not in: %v", logLevel, utils.ValidLogLevels)
	}

	cellsAdminToken := os.Getenv(envPrefix + "CELLS_ADMIN_TOKEN")
	if cellsAdminToken == "" {
		return nil, fmt.Errorf("missing required environment variable: %sCELLS_ADMIN_TOKEN", envPrefix)
	}

	cellsAddress := loadEnvWithDefault("CELLS_ADDRESS", defaultConfig.CellsAddress, true)
	a3mAddress := loadEnvWithDefault("A3M_ADDRESS", defaultConfig.A3mAddress, true)
	cellsArchiveWorkspace := loadEnvWithDefault("CELLS_ARCHIVE_WORKSPACE", defaultConfig.CellsArchiveWorkspace, true)

	return &EnvConfig{
		A3mAddress:            a3mAddress,
		A3mCompletedDir:       absA3mCompletedDir,
		A3mDipsDir:            absA3mDipsDir,
		CellsAddress:          cellsAddress,
		CellsAdminToken:       cellsAdminToken,
		CellsArchiveWorkspace: cellsArchiveWorkspace,
		CellsCecPath:          absCecPath,
		LogLevel:              logLevel,
		ProcessingBaseDir:     absProcessingDir,
	}, nil
}
