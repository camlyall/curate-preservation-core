package config

import (
	"github.com/go-playground/validator/v10"
)

// AtomConfig holds the configuration for the AtoM service.
type AtomConfig struct {
	Host          string `json:"host,omitempty" validate:"url" comment:"AtoM host URL"`
	ApiKey        string `json:"api_key,omitempty" comment:"AtoM API key for authentication"`
	LoginEmail    string `json:"login_email,omitempty" validate:"email" comment:"AtoM login email"`
	LoginPassword string `json:"login_password,omitempty" comment:"AtoM login password"`
	RsyncTarget   string `json:"rsync_target,omitempty" comment:"Rsync target for DIP transfer"`
	RsyncCommand  string `json:"rsync_command,omitempty" comment:"Custom rsync command (optional)"`
	Slug          string `json:"slug,omitempty" validate:"required" comment:"AtoM digital object slug"`
}

// defaultAtomConfig returns a default configuration for the AtoM service.
func defaultAtomConfig() *AtomConfig {
	return &AtomConfig{
		Host:          "",
		ApiKey:        "",
		LoginEmail:    "",
		LoginPassword: "",
		RsyncTarget:   "",
		RsyncCommand:  "",
		Slug:          "",
	}
}

func (a *AtomConfig) Validate() error {
	validate := validator.New()
	return validate.Struct(a)
}
