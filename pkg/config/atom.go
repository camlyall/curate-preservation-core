package config

import "fmt"

type AtomConfig struct {
	Host          string
	ApiKey        string
	LoginEmail    string
	LoginPassword string
	RsyncTarget   string
	RsyncCommand  string
	Slug          string
}

func DefaultAtomConfig() *AtomConfig {
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
	if a.Host == "" {
		return fmt.Errorf("atom host is required")
	}
	if a.ApiKey == "" {
		return fmt.Errorf("atom API key is required")
	}
	if a.LoginEmail == "" {
		return fmt.Errorf("atom login email is required")
	}
	if a.LoginPassword == "" {
		return fmt.Errorf("atom login password is required")
	}
	if a.RsyncTarget == "" {
		return fmt.Errorf("rsync target is required")
	}
	// TODO: Sanitize and validate rsync command
	// if a.RsyncCommand == "" {
	// 	return fmt.Errorf("rsync command is required")
	// }
	if a.Slug == "" {
		return fmt.Errorf("slug is required")
	}
	return nil
}
