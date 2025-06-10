// Package atom provides a client for interacting with Atom, specifically for migrating packages and depositing DIPs.
package atom

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/penwern/curate-preservation-core/pkg/config"
	"github.com/penwern/curate-preservation-core/pkg/logger"
	"github.com/penwern/curate-preservation-core/pkg/utils"
)

// Client represents an Atom client.
type Client struct {
	httpClient *utils.HTTPClient
	config     *config.AtomConfig
}

// ClientInterface defines the interface for the Atom client.
type ClientInterface interface {
	Close()
	MigratePackage()
	DepositDip()
}

// NewClient creates a new Atom client.
func NewClient(config *config.AtomConfig) (*Client, error) {

	// Validate the config
	if config == nil {
		return nil, fmt.Errorf("atom config cannot be nil")
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid atom config: %w", err)
	}

	// We can use a short http timeout. This is only used for sending DIP deposit requests.
	httpClient := utils.NewHTTPClient(5*time.Second, true)

	return &Client{
		httpClient: httpClient,
		config:     config,
	}, nil
}

// Close closes the Atom client.
func (c *Client) Close() {
	c.httpClient.Close()
}

// MigratePackage migrates a package to Atom using rsync
func (c *Client) MigratePackage(ctx context.Context, dipPath string) error {
	if err := utils.RsyncFile(ctx, dipPath, c.config.RsyncTarget, strings.Split(c.config.RsyncCommand, " ")); err != nil {
		return fmt.Errorf("error during rsync: %w", err)
	}
	return nil
}

// DepositDip deposits a DIP to Atom using the Sword Depostit API endpoint.
func (c *Client) DepositDip(ctx context.Context, slug, dipName string) error {
	depositURL := fmt.Sprintf("%s/sword/deposit/%s", c.config.Host, slug)
	encodedString := fmt.Sprintf("file:///%s", url.QueryEscape(dipName))

	auth := fmt.Sprintf("%s:%s", c.config.LoginEmail, c.config.LoginPassword)
	token := utils.Base64Encode(auth)

	headers := map[string]string{
		"Authorization":    "Basic " + token,
		"Content-Location": encodedString,
		"X-Packaging":      "http://purl.org/net/sword-types/METSArchivematicaDIP",
		"X-No-Op":          "false",
		"User-Agent":       "curate",
		"Content-Type":     "application/zip",
	}
	resp, err := c.httpClient.DoRequest(ctx, "POST", depositURL, nil, headers)
	if err != nil {
		return fmt.Errorf("error during deposit: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Error("Failed to close response body: %v", err)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to deposit DIP: %s", resp.Status)
	}
	return nil
}
