package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
	"github.com/penwern/curate-preservation-core/pkg/logger"
	"github.com/spf13/viper"
)

const (
	envPrefix = "CA4M"
)

// Config holds the configuration for the preservation service.
type Config struct {
	A3M struct {
		Address      string `mapstructure:"address" validate:"hostname_port" comment:"A3M gRPC address"`
		CompletedDir string `mapstructure:"completed_dir" validate:"dir" comment:"A3M completed directory"`
		DipsDir      string `mapstructure:"dips_dir" validate:"dir" comment:"A3M dips directory"`
	} `mapstructure:"a3m"`

	Cells struct {
		Address          string `mapstructure:"address" validate:"http_url" comment:"Cells address"`
		AdminToken       string `mapstructure:"admin_token" validate:"required" comment:"Cells admin token"`
		ArchiveWorkspace string `mapstructure:"archive_workspace" comment:"Cells archive workspace"`
		CecPath          string `mapstructure:"cec_path" validate:"file" comment:"Cells cec binary path"`
	} `mapstructure:"cells"`

	Premis struct {
		Organization string `mapstructure:"organization" comment:"Premis Agent Organization"`
	}

	AllowInsecureTLS  bool   `mapstructure:"allow_insecure_tls" comment:"Allow insecure TLS connections"`
	LogLevel          string `mapstructure:"log_level" validate:"oneof=debug info warn error fatal panic" comment:"Log level"`
	ProcessingBaseDir string `mapstructure:"processing_base_dir" validate:"dir" comment:"Base directory for processing"`
}

// Init initializes Viper configuration
func Init() {
	viper.SetEnvPrefix(envPrefix)
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	// Set default values
	setDefaults()
}

// setDefaults sets the default values for the configuration
func setDefaults() {
	viper.SetDefault("a3m.address", "localhost:7000")
	viper.SetDefault("a3m.completed_dir", "/home/a3m/.local/share/a3m/share/completed")
	viper.SetDefault("a3m.dips_dir", "/home/a3m/.local/share/a3m/share/dips")

	viper.SetDefault("cells.address", "https://localhost:8080")
	viper.SetDefault("cells.admin_token", "")
	viper.SetDefault("cells.archive_workspace", "common-files")
	viper.SetDefault("cells.cec_path", "/usr/local/bin/cec")

	viper.SetDefault("premis.organization", "")

	viper.SetDefault("allow_insecure_tls", false)
	viper.SetDefault("log_level", "info")
	viper.SetDefault("processing_base_dir", "/tmp/preservation")
}

// Load loads the configuration from the environment variables and .env file
func Load() (*Config, error) {
	// Load .env if it exists
	if _, err := os.Stat(".env"); err == nil {
		logger.Info("Loading .env file")
		if err := godotenv.Load(); err != nil {
			return nil, fmt.Errorf("error loading .env file: %w", err)
		}
	}

	// Unmarshal the configuration
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshalling configuration: %v", err)
	}

	// Validate the configuration
	if err := validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// validate validates the configuration
func validate(cfg *Config) error {
	validate := validator.New()
	return validate.Struct(cfg)
}
