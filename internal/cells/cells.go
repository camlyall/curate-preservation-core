// Provides functionality for making HTTP request to instances if Pydio Cells API
// TODO: Make requests directly to Cells gRPC endpoints? Note: HTPP requests simply forward to gRPC

package cells

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/penwern/preservation-go/pkg/utils"
	"github.com/pydio/cells-sdk-go/v4/client"
	"github.com/pydio/cells-sdk-go/v4/models"
)

type Client struct {
	address             string                          // Cells http(s) address. Remove in favour of adminClient?
	adminClient         *AdminClient                    // Cells admin client
	cecPath             string                          // Path to cec binary. Only used for cec binary
	httpClient          *utils.HttpClient               // User for generating and revoking tokens
	userClient          *UserClient                     // Cells user client
	workspaceCollection *models.RestWorkspaceCollection // Cells workspace collection. For parsing template paths
}

type AdminClient struct {
	client *client.PydioCellsRestAPI
	token  string
}

type UserClient struct {
	client    *client.PydioCellsRestAPI
	UserData  *models.IdmUser
	userToken string
}

type ClientInterface interface {
	Close(ctx context.Context)
	DownloadNode(ctx context.Context, cellsSrc, dest string) (string, error)
	GetNodeCollection(ctx context.Context, absNodePath string) (*models.RestNodesCollection, error)
	GetUserClientUserData() *models.IdmUser
	NewUserClient(ctx context.Context, username string) error
	ResolveCellsPath(cellsPath string) (string, error)
	UpdateTag(ctx context.Context, nodeUuid, namespace, content string) error
	UploadNode(ctx context.Context, src, cellsDest string) (string, error)
}

// NewClient creates a new Cells client for managing Cells related tasks.
func NewClient(ctx context.Context, cecPath, address, adminToken string) (*Client, error) {
	// We can use a short http timeout. This is only used for token gen.
	httpClient := utils.NewHttpClient(5*time.Second, true)

	url, err := url.Parse(address)
	if err != nil {
		return nil, fmt.Errorf("error parsing address: %v", err)
	}
	address = url.Scheme + "://" + url.Host

	adminClient := newSDKClient(url.Scheme, url.Host, "/a", true, adminToken)

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

	return client, nil
}

func (c *Client) NewUserClient(ctx context.Context, username string) error {

	// Check if user client already exists. Temp for testing.
	if c.userClient != nil {
		return fmt.Errorf("user client already exists")
	}

	url, err := url.Parse(c.address)
	if err != nil {
		return fmt.Errorf("error parsing address: %v", err)
	}

	userToken, err := c.generateUserToken(ctx, username, 30*time.Minute)
	if err != nil {
		return fmt.Errorf("error generating user token: %v", err)
	}
	if userToken == "" {
		return fmt.Errorf("user token is empty")
	}

	userSDKClient := newSDKClient(url.Scheme, url.Host, "/a", true, userToken)

	userData, err := c.getUserData(ctx, username)
	if err != nil {
		return fmt.Errorf("error retrieving user data: %v", err)
	}

	user := UserClient{
		client:    userSDKClient,
		userToken: userToken,
		UserData:  userData,
	}

	c.userClient = &user
	return nil
}

func (c *Client) Close(ctx context.Context) {
	if c.httpClient != nil {
		c.httpClient.Close()
	}
	// if err := c.removeUserToken(ctx, c.userClient.userToken); err != nil {
	// 	log.Printf("error removing user token: %v", err)
	// }
	// log.Println("Removed user token")
}

func (c *Client) GetUserClientUserData() *models.IdmUser {
	return c.userClient.UserData
}

///////////////////////////////////////////////////////////////////
//						  Cells API 							 //
///////////////////////////////////////////////////////////////////

// UpdateTag updates a tag for a node.
// Cells SDK.
func (c *Client) UpdateTag(ctx context.Context, nodeUuid, namespace, content string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return sdkUpdateUserMeta(ctx, *c.userClient.client, nodeUuid, namespace, content)
}

// GetNodeCollection gets a collection of nodes from a given path.
// It requires the absolute, fully qualified node path.
// Admin Task. Cells SDK.
func (c *Client) GetNodeCollection(ctx context.Context, absNodePath string) (*models.RestNodesCollection, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return sdkGetNodeCollection(ctx, *c.adminClient.client, absNodePath)
}

// GetWorkspaceCollection get the collection of Pydio Cells workspaces.
// Admin not required. Used as User generated after execution. Cells SDK.
func (c *Client) getWorkspaceCollection(ctx context.Context) (*models.RestWorkspaceCollection, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return sdkGetWorkspaceCollection(ctx, *c.adminClient.client)
}

// GetUserData get the user data for the user.
// Admin not required. Used as User generated after execution. Cells SDK.
func (c *Client) getUserData(ctx context.Context, username string) (*models.IdmUser, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return sdkGetUserData(ctx, *c.adminClient.client, username)
}

// generateUserToken generates a user token for a given user.
// Admin Task. Cells API. Returns the user token.
func (c *Client) generateUserToken(ctx context.Context, user string, duration time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return apiGenerateUserToken(ctx, c.httpClient, c.address, user, c.adminClient.token, duration)
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
func (c *Client) DownloadNode(ctx context.Context, cellsSrc, dest string) (string, error) {
	if _, err := cecDownloadNode(ctx, c.cecPath, c.address, c.userClient.UserData.Login, c.userClient.userToken, cellsSrc, dest); err != nil {
		return "", fmt.Errorf("error downloading node: %w", err)
	}
	return filepath.Join(dest, filepath.Base(cellsSrc)), nil

}

// UploadNode uploads a node from a local directory to Cells using the CEC binary.
// Returns the path of the uploaded node.
// TODO: Confirm if the upload path is correct (coz duplication)
func (c *Client) UploadNode(ctx context.Context, src, cellsDest string) (string, error) {
	if _, err := cecUploadNode(ctx, c.cecPath, c.address, c.userClient.UserData.Login, c.userClient.userToken, src, cellsDest); err != nil {
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

	// Error if workspace not found
	if workspace.RootNodes == nil {
		return "", fmt.Errorf("workspace has no root nodes: %s", workspaceRoot)
	}

	// Find the resolution for the workspace if it uses a template path (i.e. not a DATASOURCE root)
	var resolution string
	for root, rootNode := range workspace.RootNodes {
		// Ignore if the root node is a DATASOURCE, i.e. not a template path
		if !strings.HasPrefix(root, "DATASOURCE") {
			resolution = rootNode.MetaStore["resolution"]
			break
		}
	}
	// If no resolution is found, return the cells path becuase it doesn't use a template path
	if resolution == "" {
		log.Printf("No resolution found for cells path: %s", cellsPath)
		return cellsPath, nil
	}

	// Parse resolution
	resolvedWorkspaceRoot, err := parseWorkspaceResolution(resolution, c.userClient.UserData.Login, c.userClient.UserData.GroupPath)
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
