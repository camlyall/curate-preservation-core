// Provides functionality for making HTTP request to instances if Pydio Cells API
// TODO: Make requests directly to Cells gRPC endpoints? Note: HTPP requests simply forward to gRPC

package cells

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/penwern/curate-preservation-core/pkg/logger"
	"github.com/penwern/curate-preservation-core/pkg/utils"
	"github.com/pydio/cells-sdk-go/v4/client"
	"github.com/pydio/cells-sdk-go/v4/models"
)

// Client represents a Cells client.
type Client struct {
	address             string                          // Cells http(s) address. Remove in favour of adminClient?
	adminClient         *AdminClient                    // Cells admin client
	cecPath             string                          // Path to cec binary. Only used for cec binary
	httpClient          *utils.HTTPClient               // User for generating and revoking tokens
	workspaceCollection *models.RestWorkspaceCollection // Cells workspace collection. For parsing template paths
}

// AdminClient represents a Cells admin client.
type AdminClient struct {
	client *client.PydioCellsRestAPI
	token  string
}

// UserClient represents a Cells user client.
type UserClient struct {
	client    *client.PydioCellsRestAPI
	UserData  *models.IdmUser
	userToken string
}

// ClientInterface defines the interface for the Cells client.
type ClientInterface interface {
	Close()
	DownloadNode(ctx context.Context, userClient UserClient, cellsSrc, dest string) (string, error)
	GetNodeCollection(ctx context.Context, absNodePath string) (*models.RestNodesCollection, error)
	GetNodeStats(ctx context.Context, absNodePath string) (*models.TreeReadNodeResponse, error)
	NewUserClient(ctx context.Context, username string, insecure bool) (UserClient, error)
	ResolveCellsPath(userClient UserClient, cellsPath string) (string, error)   // e.g. personal-files/file -> personal/username/file
	UnresolveCellsPath(userClient UserClient, cellsPath string) (string, error) // e.g. personal/username/file -> personal-files/file
	UpdateTag(ctx context.Context, userClient UserClient, nodeUUID, namespace, content string) error
	UploadNode(ctx context.Context, userClient UserClient, src, cellsDest string) (string, error)
}

// NewClient creates a new Cells client for managing Cells related tasks.
func NewClient(ctx context.Context, cecPath, address, adminToken string, insecure bool) (*Client, error) {
	// We can use a short http timeout. This is only used for token gen.
	httpClient := utils.NewHTTPClient(5*time.Second, insecure)

	url, err := url.Parse(address)
	if err != nil {
		return nil, fmt.Errorf("error parsing address: %v", err)
	}
	address = url.Scheme + "://" + url.Host

	adminClient, err := newSDKClient(url.Scheme, url.Host, "/a", insecure, adminToken)
	if err != nil {
		return nil, fmt.Errorf("error creating admin client: %v", err)
	}

	client := &Client{
		cecPath:    cecPath,
		address:    address,
		httpClient: httpClient,
		adminClient: &AdminClient{
			client: adminClient,
			token:  adminToken,
		},
	}

	client.workspaceCollection, err = client.getWorkspaceCollection(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting workspace collection: %v", err)
	}
	logger.Debug("Client created: {address: %s, cecPath: %s}", address, cecPath)
	return client, nil
}

// NewUserClient creates a new Cells user client.
func (c *Client) NewUserClient(ctx context.Context, username string, insecure bool) (UserClient, error) {
	url, err := url.Parse(c.address)
	if err != nil {
		return UserClient{}, fmt.Errorf("error parsing address: %v", err)
	}

	userToken, err := c.generateUserToken(ctx, username, 30*time.Minute)
	if err != nil {
		return UserClient{}, fmt.Errorf("error generating user token: %v", err)
	}
	if userToken == "" {
		return UserClient{}, fmt.Errorf("user token is empty")
	}

	userSDKClient, err := newSDKClient(url.Scheme, url.Host, "/a", insecure, userToken)
	if err != nil {
		return UserClient{}, fmt.Errorf("error creating user client: %v", err)
	}

	userData, err := c.getUserData(ctx, username)
	if err != nil {
		return UserClient{}, fmt.Errorf("error retrieving user data: %v", err)
	}
	if userData == nil {
		return UserClient{}, fmt.Errorf("user data is nil for username: %s", username)
	}

	user := UserClient{
		client:    userSDKClient,
		userToken: userToken,
		UserData:  userData,
	}
	logger.Debug("User client created: {username: %s, login: %s}", username, userData.Login)
	return user, nil
}

// Close closes the Cells client.
func (c *Client) Close() {
	if c.httpClient != nil {
		c.httpClient.Close()
	}
	// if err := c.removeUserToken(ctx, c.userClient.userToken); err != nil {
	// 	log.Printf("error removing user token: %v", err)
	// }
	// log.Println("Removed user token")
}

///////////////////////////////////////////////////////////////////
//						  Cells API 							 //
///////////////////////////////////////////////////////////////////

// UpdateTag updates a tag for a node.
// Cells SDK.
func (c *Client) UpdateTag(ctx context.Context, userClient UserClient, nodeUUID, namespace, content string) error {
	err := utils.WithRetry(func() error {
		return sdkUpdateUserMeta(ctx, *userClient.client, nodeUUID, namespace, content)
	})
	return err
}

// GetNodeCollection gets a collection of nodes from a given path.
// It requires the absolute, fully qualified node path.
// Admin Task. Cells SDK.
func (c *Client) GetNodeCollection(ctx context.Context, absNodePath string) (*models.RestNodesCollection, error) {
	var result *models.RestNodesCollection
	err := utils.WithRetry(func() error {
		var err error
		result, err = sdkGetNodeCollection(ctx, *c.adminClient.client, absNodePath)
		return err
	})
	// If 404 log node not found
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			logger.Debug("Node not found: %s", absNodePath)
			return nil, fmt.Errorf("node not found: %s", absNodePath)
		}
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("node collection is nil for path: %s", absNodePath)
	}
	if result.Parent == nil {
		return nil, fmt.Errorf("parent node is nil for path: %s", absNodePath)
	}
	return result, nil
}

// GetNodeStats gets the stats of a node from a given path.
// It requires the absolute, fully qualified node path.
// Admin Task. Cells SDK.
func (c *Client) GetNodeStats(ctx context.Context, absNodePath string) (*models.TreeReadNodeResponse, error) {
	var result *models.TreeReadNodeResponse
	err := utils.WithRetry(func() error {
		var err error
		result, err = sdkGetNodeStats(ctx, *c.adminClient.client, absNodePath)
		return err
	})
	return result, err
}

// GetWorkspaceCollection get the collection of Pydio Cells workspaces.
// Admin not required. Used as User generated after execution. Cells SDK.
func (c *Client) getWorkspaceCollection(ctx context.Context) (*models.RestWorkspaceCollection, error) {
	var result *models.RestWorkspaceCollection
	err := utils.WithRetry(func() error {
		var err error
		result, err = sdkGetWorkspaceCollection(ctx, *c.adminClient.client)
		return err
	})
	return result, err
}

// GetUserData get the user data for the user.
// Admin not required. Used as User generated after execution. Cells SDK.
func (c *Client) getUserData(ctx context.Context, username string) (*models.IdmUser, error) {
	var result *models.IdmUser
	err := utils.WithRetry(func() error {
		var err error
		result, err = sdkGetUserData(ctx, *c.adminClient.client, username)
		return err
	})
	return result, err
}

// generateUserToken generates a user token for a given user.
// Admin Task. Cells API. Returns the user token.
func (c *Client) generateUserToken(ctx context.Context, user string, duration time.Duration) (string, error) {
	var result string
	err := utils.WithRetry(func() error {
		var err error
		result, err = apiGenerateUserToken(ctx, c.httpClient, c.address, user, c.adminClient.token, duration)
		return err
	})
	return result, err
}

// removeUserToken removes a user token from the Cells server.
// Admin Task. Removed because it doesn't work. Need the token UUID
// func (c *Client) removeUserToken(ctx context.Context, userToken string) error {
// 	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
// 	defer cancel()
// 	return apiRevokeUserToken(ctx, c.httpClient, c.address, c.adminToken, userToken)
// }

///////////////////////////////////////////////////////////////////
//						  Cells CEC 							 //
///////////////////////////////////////////////////////////////////

// DownloadNode downloads a node from Cells to a local directory using the CEC binary.
// Returns the path of the downloaded node.
func (c *Client) DownloadNode(ctx context.Context, userClient UserClient, cellsSrc, dest string) (string, error) {
	if _, err := cecDownloadNode(ctx, c.cecPath, c.address, userClient.UserData.Login, userClient.userToken, cellsSrc, dest); err != nil {
		return "", fmt.Errorf("error downloading node: %w", err)
	}
	return filepath.Join(dest, filepath.Base(cellsSrc)), nil
}

// UploadNode uploads a node from a local directory to Cells using the CEC binary.
// Returns the path of the uploaded node.
// TODO: Confirm if the upload path is correct (coz duplication)
func (c *Client) UploadNode(ctx context.Context, userClient UserClient, src, cellsDest string) (string, error) {
	if _, err := cecUploadNode(ctx, c.cecPath, c.address, userClient.UserData.Login, userClient.userToken, src, cellsDest); err != nil {
		return "", fmt.Errorf("error uploading node: %w", err)
	}
	return filepath.Join(cellsDest, filepath.Base(src)), nil
}

///////////////////////////////////////////////////////////////////
//						  	Utils								 //
///////////////////////////////////////////////////////////////////

// ResolveCellsPath resolves a cells path to an absolute path.
// The path is resolved by replacing the workspace root with the resolved workspace root.
func (c *Client) ResolveCellsPath(userClient UserClient, cellsPath string) (string, error) {
	// Get the workspace from the cells path
	pathParts := strings.Split(cellsPath, "/")
	if len(pathParts) == 0 {
		return "", fmt.Errorf("invalid cells path: %s", cellsPath)
	}
	workspaceRoot := pathParts[0]

	// Find the workspace in the workspace collection with the same slug
	var workspace *models.IdmWorkspace
	for _, w := range c.workspaceCollection.Workspaces {
		if w.Slug == workspaceRoot {
			workspace = w
			break
		}
	}

	// Error if workspace collection is empty
	if workspace == nil {
		return "", fmt.Errorf("workspace not found: %s", workspaceRoot)
	}

	// Error if workspace has no root nodes
	if workspace.RootNodes == nil {
		return "", fmt.Errorf("workspace has no root nodes: %s", workspaceRoot)
	}

	logger.Debug("Selected workspace: %s", workspace.Slug)

	// Find the resolution for the workspace if it uses a template path (i.e. not a DATASOURCE root)
	// Store the datasource path for later use if a resolution is not found
	var resolution string
	var datasource string
	// TODO: The case of multiple root nodes required further testing. We will search for a resolution and fall back to the datasource if not found.
	for root, rootNode := range workspace.RootNodes {
		// Ignore if the root node is a DATASOURCE, a.k.a. not a templated workspace
		if !strings.HasPrefix(root, "DATASOURCE") {
			resolution = rootNode.MetaStore["resolution"]
			break
		}
		datasource = strings.TrimSuffix(rootNode.Path, "/")
	}
	// If no resolution is found, return the cells path because it doesn't use a template path
	// We must fall back to the datasource path.
	if resolution == "" {
		if datasource == "" {
			return "", fmt.Errorf("no resolution or datasource found for workspace: %s", workspaceRoot)
		}
		logger.Debug("No resolution found for cells path: %s. Falling back to datasource: %s", cellsPath, datasource)
		resolvedPath := strings.Replace(cellsPath, workspaceRoot, datasource, 1)

		// Verify the resolved path exists
		_, err := c.GetNodeStats(context.Background(), resolvedPath)
		if err != nil {
			return "", fmt.Errorf("resolved path does not exist: %s (%w)", resolvedPath, err)
		}

		return resolvedPath, nil
	}

	// Parse resolution
	resolutionPath, err := parseWorkspaceResolution(resolution)
	if err != nil {
		return "", fmt.Errorf("error parsing resolution: %w", err)
	}
	resolvedWorkspaceRoot, err := resolveResolution(resolutionPath, userClient.UserData.Login, userClient.UserData.GroupPath)
	if err != nil {
		return "", fmt.Errorf("error parsing workspace resolution: %w", err)
	}

	// Construct the resolved path
	resolvedPath := strings.Replace(cellsPath, workspaceRoot, resolvedWorkspaceRoot, 1)

	return resolvedPath, nil
}

// UnresolveCellsPath unresolves a cells path to a workspace path.
// The path is unresovled by replacing the resolved workspace root with the workspace root.
func (c *Client) UnresolveCellsPath(userClient UserClient, cellsPath string) (string, error) {
	// Get the workspace from the cells path
	pathParts := strings.Split(cellsPath, "/")
	if len(pathParts) == 0 {
		return "", fmt.Errorf("invalid cells path: %s", cellsPath)
	}
	datasource := pathParts[0]

	// Find the workspace in the workspace collection that uses the datasource
	var resolutions []string
	for _, w := range c.workspaceCollection.Workspaces {
		for root, rootNode := range w.RootNodes {
			if !strings.HasPrefix(root, "DATASOURCE") {
				resolutionPath, err := parseWorkspaceResolution(rootNode.MetaStore["resolution"])
				if err != nil {
					return "", fmt.Errorf("error parsing resolution: %w", err)
				}
				// Check if the resolution path references the datasource
				if strings.Contains(resolutionPath, "DataSources."+datasource) {
					resolutions = append(resolutions, resolutionPath)
					// Parse resolution
					resolvedWorkspaceRoot, err := resolveResolution(resolutionPath, userClient.UserData.Login, userClient.UserData.GroupPath)
					if err != nil {
						return "", fmt.Errorf("error parsing workspace resolution: %w", err)
					}
					if strings.HasPrefix(cellsPath, resolvedWorkspaceRoot) {
						unresolvedPath := strings.Replace(cellsPath, resolvedWorkspaceRoot, w.Slug, 1)
						return unresolvedPath, nil
					}
					logger.Debug("Resolved path does not match cells path: %s != %s", resolvedWorkspaceRoot, cellsPath)
				}
			}
		}
	}
	logger.Error("No resolution found for cells path: %s {UserLogin: %s, UserGroup: %s}", cellsPath, userClient.UserData.Login, userClient.UserData.GroupPath)
	logger.Debug("Possible Resolutions: %v", resolutions)
	return cellsPath, nil
}

// parseWorkspaceResolution parses the resolution of a workspace to get the full path.
// The resolutions is a string of the template path assigned to a workspace to.
// TODO: Add more details about the resolution format
// https://pydio.com/en/docs/cells/v4/ent-shard-template-path
func parseWorkspaceResolution(resolution string) (string, error) {
	re := regexp.MustCompile(`Path\s*=\s*(.*?)\s*;`)
	matches := re.FindStringSubmatch(resolution)

	if len(matches) < 2 {
		return "", fmt.Errorf("failed to parse resolution")
	}

	return matches[1], nil
}

func resolveResolution(resolutionPath, username, groupname string) (string, error) {
	// Construct the admin path
	// DataSources.personal + \"/\" + User.Name
	// Becomes: personal/username
	adminPath := strings.ReplaceAll(resolutionPath, " ", "")
	replacer := strings.NewReplacer(
		"DataSources.", "",
		"+\"/\"+", "/",
		"User.Name", username,
		"User.Group", groupname,
	)
	adminPath = replacer.Replace(adminPath)
	return adminPath, nil
}
