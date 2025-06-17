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
	Host          string `json:"host,omitempty" validate:"url" comment:"AtoM host URL"`
	APIKey        string `json:"api_key,omitempty" comment:"AtoM API key for authentication"`
	LoginEmail    string `json:"login_email,omitempty" validate:"email" comment:"AtoM login email"`
	LoginPassword string `json:"login_password,omitempty" comment:"AtoM login password"`
	RsyncTarget   string `json:"rsync_target,omitempty" comment:"Rsync target for DIP transfer"`
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

// Save saves the configuration to a file with file locking
func (a *AtomConfig) Save(path string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	// Create a temporary file in the same directory
	tempFile := path + ".tmp"
	file, err := os.OpenFile(filepath.Clean(tempFile), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("creating temporary file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			logger.Error("Failed to close temporary file: %v", closeErr)
		}
		if removeErr := os.Remove(tempFile); removeErr != nil && !os.IsNotExist(removeErr) {
			logger.Error("Failed to remove temporary file: %v", removeErr)
		}
	}()

	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	// Write to temporary file
	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	// Close the file before renaming
	if err := file.Close(); err != nil {
		return fmt.Errorf("closing temporary file: %w", err)
	}

	// Atomically rename the temporary file to the target file
	if err := os.Rename(tempFile, path); err != nil {
		return fmt.Errorf("renaming temporary file: %w", err)
	}

	return nil
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
	if a.Slug == "" {
		a.Slug = fileConfig.Slug
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
