// Package config provides configuration structures and validation for the AtoM service.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/go-playground/validator/v10"
	"github.com/penwern/curate-preservation-core/pkg/logger"
)

// AtomConfig holds the configuration for the AtoM service.
type AtomConfig struct {
	Host          string `json:"host,omitempty" validate:"required,url" comment:"AtoM host URL"`
	APIKey        string `json:"api_key,omitempty" validate:"required" comment:"AtoM API key for authentication"`
	LoginEmail    string `json:"login_email,omitempty" validate:"required,email" comment:"AtoM login email"`
	LoginPassword string `json:"login_password,omitempty" validate:"required" comment:"AtoM login password"`
	RsyncTarget   string `json:"rsync_target,omitempty" validate:"required" comment:"Rsync target for DIP transfer"`
	RsyncCommand  string `json:"rsync_command,omitempty" comment:"Custom rsync command (optional)"`
	Slug          string `json:"slug,omitempty" validate:"required" comment:"AtoM digital object slug"`

	mu sync.RWMutex // Mutex for thread-safe access
}

// DefaultAtomConfig returns a default configuration for the AtoM service.
func DefaultAtomConfig() *AtomConfig {
	return &AtomConfig{
		Host:          "",
		APIKey:        "",
		LoginEmail:    "",
		LoginPassword: "",
		RsyncTarget:   "",
		RsyncCommand:  "",
		Slug:          "",
	}
}

// Validate validates the AtomConfig.
func (a *AtomConfig) Validate() error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	validate := validator.New()
	return validate.Struct(a)
}

// LoadAtomConfig loads the configuration from a file
func LoadAtomConfig(path string) (*AtomConfig, error) {
	// Read file
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			// Return default config if file doesn't exist
			return DefaultAtomConfig(), nil
		}
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	// Unmarshal config
	var config AtomConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	return &config, nil
}

// MergeWithFile merges the current config with values from a file
func (a *AtomConfig) MergeWithFile(path string) error {
	fileConfig, err := LoadAtomConfig(path)
	if err != nil {
		return err
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// Only update fields that are empty in the current config
	if a.Host == "" {
		a.Host = fileConfig.Host
	}
	if a.APIKey == "" {
		a.APIKey = fileConfig.APIKey
	}
	if a.LoginEmail == "" {
		a.LoginEmail = fileConfig.LoginEmail
	}
	if a.LoginPassword == "" {
		a.LoginPassword = fileConfig.LoginPassword
	}
	if a.RsyncTarget == "" {
		a.RsyncTarget = fileConfig.RsyncTarget
	}
	if a.RsyncCommand == "" {
		a.RsyncCommand = fileConfig.RsyncCommand
	}

	return nil
}

// GetAtomConfig loads the AtoM configuration with proper priority:
// 1. CLI flags (highest priority)
// 2. Config file
// 3. Environment variables (lowest priority)
func GetAtomConfig(cfg *Config, cliConfig *AtomConfig) (*AtomConfig, error) {
	// Start with CLI config if provided
	var atomConfig *AtomConfig
	if cliConfig != nil {
		atomConfig = cliConfig
	} else {
		atomConfig = DefaultAtomConfig()
	}

	// Try to load from file
	if err := atomConfig.MergeWithFile(cfg.Atom.ConfigPath); err != nil {
		logger.Warn("Failed to load AtoM config from file: %v", err)
	}

	return atomConfig, nil
}
