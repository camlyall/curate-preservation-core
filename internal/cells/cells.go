// Provides functionality for making HTTP request to instances if Pydio Cells API
// TODO: Make requests directly to Cells gRPC endpoints? Note: HTPP requests simply forward to gRPC

package cells

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/penwern/preservation-go/pkg/utils"
)

type Client struct {
	cecPath string
	address string
	// username            string
	userToken           string
	adminToken          string
	httpClient          *utils.HttpClient
	workspaceCollection WorkspaceCollection
	UserData            UserData
}

// NewClient creates a new Cells client for managing Cells related tasks.
func NewClient(ctx context.Context, cecPath, address, username, adminToken string) (*Client, error) {
	// We can use a short http timeout because upload and download are handled by the CEC binary.
	httpClient := utils.NewHttpClient(10*time.Second, true)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	client := &Client{
		cecPath:    cecPath,
		address:    address,
		adminToken: adminToken,
		httpClient: httpClient,
	}

	userToken, err := client.getUserToken(ctx, username, 30*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("error generating user token: %v", err)
	}
	if userToken == "" {
		return nil, fmt.Errorf("user token is empty")
	}
	client.userToken = userToken

	client.UserData, err = client.GetUserData(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("error getting user data: %v", err)
	}

	client.workspaceCollection, err = client.GetWorkspaceCollection(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting workspace collection: %v", err)
	}

	return client, nil
}

func (c *Client) Close(ctx context.Context) {
	if c.httpClient != nil {
		c.httpClient.Close()
	}
	if err := c.removeUserToken(ctx); err != nil {
		fmt.Printf("error removing user token: %v", err)
	}
	fmt.Println("Removed user token")
}

///////////////////////////////////////////////////////////////////
//						  Cells API 							 //
///////////////////////////////////////////////////////////////////

// UpdateTag updates a tag for a node.
func (c *Client) UpdateTag(ctx context.Context, nodeUuid, namespace, content string) error {
	return updateTag(ctx, c.httpClient, c.address, c.userToken, nodeUuid, namespace, content)
}

// GetNodeCollection gets a collection of nodes from a given path.
// The given path is represented by Parent and its child nodes are represented by []Children
// It requires the absolute, fully qualified node path.
// Admin Task.
func (c *Client) GetNodeCollection(ctx context.Context, absNodePath string) (NodeCollection, error) {
	return getNodeCollection(ctx, c.httpClient, c.address, c.adminToken, absNodePath)
}

// GetWorkspaceCollection get the collection of Pydio Cells workspaces.
func (c *Client) GetWorkspaceCollection(ctx context.Context) (WorkspaceCollection, error) {
	return getWorkspaceCollection(ctx, c.httpClient, c.address, c.userToken)
}

// GetUserData get the user data for the user.
func (c *Client) GetUserData(ctx context.Context, username string) (UserData, error) {
	return getUserData(ctx, c.httpClient, c.address, c.userToken, username)
}

// generateUserToken generates a user token for a given user.
// Admin Task.
func (c *Client) getUserToken(ctx context.Context, user string, duration time.Duration) (string, error) {
	return generateUserToken(ctx, c.httpClient, c.address, user, c.adminToken, duration)
}

// removeUserToken removes a user token from the Cells server.
// Admin Task.
func (c *Client) removeUserToken(ctx context.Context) error {
	return revokeUserToken(ctx, c.httpClient, c.address, c.adminToken, c.userToken)
}

///////////////////////////////////////////////////////////////////
//						  Cells CEC 							 //
///////////////////////////////////////////////////////////////////

// DownloadNode downloads a node from Cells to a local directory using the CEC binary.
// Returns the path of the downloaded node.
func (c *Client) DownloadNode(ctx context.Context, cellsSrc, dest string) (string, error) {
	if _, err := downloadNode(ctx, c.cecPath, c.address, c.UserData.Login, c.userToken, cellsSrc, dest); err != nil {
		return "", fmt.Errorf("error downloading node: %w", err)
	}
	return filepath.Join(dest, filepath.Base(cellsSrc)), nil

}

// UploadNode uploads a node from a local directory to Cells using the CEC binary.
// Returns the path of the uploaded node.
// TODO: Confirm if the upload path is correct (coz duplication)
func (c *Client) UploadNode(ctx context.Context, src, cellsDest string) (string, error) {
	if _, err := uploadNode(ctx, c.cecPath, c.address, c.UserData.Login, c.userToken, src, cellsDest); err != nil {
		return "", fmt.Errorf("error uploading node: %w", err)
	}
	return filepath.Join(cellsDest, filepath.Base(src)), nil
}

///////////////////////////////////////////////////////////////////
//						  	Utils								 //
///////////////////////////////////////////////////////////////////

// ResolveCellsPath resolves a cells path to an absolute path.
// The path is resolved by replacing the workspace root with the resolved workspace root.
func (c *Client) ResolveCellsPath(cellsPath string) (string, error) {

	// Get the workspace from the cells path
	pathParts := strings.Split(cellsPath, "/")
	if len(pathParts) == 0 {
		return "", fmt.Errorf("invalid cells path: %s", cellsPath)
	}
	workspaceRoot := pathParts[0]

	// Find the workspace in the workspace collection with the same slug
	var workspace Workspace
	for _, w := range c.workspaceCollection.Workspaces {
		if w.Slug == workspaceRoot {
			workspace = w
			break
		}
	}

	// Error if workspace not found
	if workspace.RootNodes == nil {
		return "", fmt.Errorf("workspace not found: %s", workspaceRoot)
	}

	// Find the resolution for the workspace if it uses a template path (i.e. not a DATASOURCE root)
	var resolution string
	for root, rootNode := range workspace.RootNodes {
		// Ignore if the root node is a DATASOURCE, i.e. not a template path
		if !strings.HasPrefix(root, "DATASOURCE") {
			resolution = rootNode.MetaStore.Resolution
			break
		}
	}
	// If no resolution is found, return the cells path becuase it doesn't use a template path
	if resolution == "" {
		return cellsPath, nil
	}

	// Parse resolution
	resolvedWorkspaceRoot, err := parseWorkspaceResolution(resolution, c.UserData.Login, c.UserData.GroupPath)
	if err != nil {
		return "", fmt.Errorf("error parsing workspace resolution: %w", err)
	}

	// Construct the resolved path
	resolvedPath := strings.Replace(cellsPath, workspaceRoot, resolvedWorkspaceRoot, 1)

	return resolvedPath, nil
}

// parseWorkspaceResolution parses the resolution of a workspace to get the full path.
// The resolutions is a string of the template path assigned to a workspace to.
// TODO: Add more details about the resolution format
// https://pydio.com/en/docs/cells/v4/ent-shard-template-path
func parseWorkspaceResolution(resolution, username, groupname string) (string, error) {
	re := regexp.MustCompile(`Path\s*=\s*(.*?)\s*;`)
	matches := re.FindStringSubmatch(resolution)

	if len(matches) < 2 {
		return "", fmt.Errorf("failed to parse resolution")
	}

	resolutionPath := matches[1]

	// Construct the admin path
	// DataSources.personal + \"/\" + User.Name
	// Becomes: personal/username
	adminPath := strings.Replace(resolutionPath, " ", "", -1)
	replacer := strings.NewReplacer(
		"DataSources.", "",
		"+\"/\"+", "/",
		"User.Name", username,
		"User.Group", groupname,
	)
	adminPath = replacer.Replace(adminPath)
	return adminPath, nil
}
