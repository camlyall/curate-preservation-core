// Provides functionality for making HTTP request to instances if Pydio Cells API
// TODO: Make requests directly to Cells gRPC endpoints? Note: HTPP requests simply forward to gRPC

package cells

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Client struct {
	CECPath    string
	Address    string
	User       string
	UserToken  string
	AdminToken string
}

type NodeData struct {
	Uuid      string `json:"Uuid"`
	Path      string `json:"Path"`
	MetaStore struct {
		Premis string `json:"usermeta-premis-data"`
	} `json:"MetaStore"`
}

type NodeCollection struct {
	Parent   NodeData   `json:"Parent"`
	Children []NodeData `json:"Children"`
}

// NewClient creates a new Cells client for managing Cells related tasks
func NewClient(cecPath, cellsAddress, cellsUserName, cellsAdminToken string) (*Client, error) {
	cellsUserToken, err := getUserToken(cellsAddress, cellsAdminToken, cellsUserName)
	if err != nil {
		return nil, fmt.Errorf("error getting user token: %v", err)
	}
	return &Client{
		CECPath:    cecPath,
		Address:    cellsAddress,
		User:       cellsUserName,
		UserToken:  cellsUserToken,
		AdminToken: cellsAdminToken,
	}, nil
}

// getUserToken gets user specific authentication token using the admin token
func getUserToken(cellsAddress, adminToken, userName string) (string, error) {
	url := fmt.Sprintf("%s/a/auth/token/impersonate", cellsAddress)

	type Payload struct {
		Label     string `json:"Label"`
		UserLogin string `json:"UserLogin"`
		ExpiresAt int64  `json:"ExpiresAt"`
	}

	payloadData := Payload{
		Label:     "Preservation Token",
		UserLogin: userName,
		ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
	}

	// Marshal the payload into JSON
	payload, err := json.Marshal(payloadData)
	if err != nil {
		return "", err
	}

	// Create a new POST request with the JSON payload.
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token gen request gave status code: %d", resp.StatusCode)
	}

	// Read the response body.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	type Response struct {
		AccessToken string `json:"AccessToken"`
	}
	reponseData := Response{}

	err = json.Unmarshal(body, &reponseData)
	if err != nil {
		return "", fmt.Errorf("error unmarshalling response: %w", err)
	}

	return string(reponseData.AccessToken), nil
}

// constructAdminCellsPath constructs the path to the admin area of the cells server
func (c *Client) ConstructAdminWorkspaceRoot(workspaceRoot string) (string, error) {

	url := fmt.Sprintf("%s/a/workspace", c.Address)

	type Payload struct {
		Queries []struct {
			Scope string `json:"scope"`
		} `json:"Queries"`
	}

	payloadData := Payload{
		Queries: []struct {
			Scope string `json:"scope"`
		}{
			{
				Scope: "ADMIN",
			},
		},
	}

	payload, err := json.Marshal(payloadData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.AdminToken)
	req.Header.Set("Content-Type", "application/json")

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get workspace: %d %s", resp.StatusCode, string(body))
	}

	type Workspace struct {
		Slug      string `json:"Slug"`
		RootNodes map[string]struct {
			MetaStore struct {
				Resolution string `json:"resolution"`
			} `json:"MetaStore"`
		} `json:"RootNodes"`
	}

	type Response struct {
		Workspaces []Workspace `json:"Workspaces"`
	}

	var response Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal response body: %w", err)
	}
	if len(response.Workspaces) == 0 {
		return "", fmt.Errorf("no workspaces found")
	}

	var workspace Workspace
	for _, w := range response.Workspaces {
		if w.Slug == workspaceRoot {
			workspace = w
			break
		}
	}

	if workspace.RootNodes == nil {
		return "", fmt.Errorf("workspace root not found")
	}

	var resolution string
	for root, rootNode := range workspace.RootNodes {
		// Ignore if the root node is a DATASOURCE
		if strings.HasPrefix(root, "DATASOURCE") {
			continue
		}
		resolution = rootNode.MetaStore.Resolution
	}

	if resolution == "" {
		return workspaceRoot, nil
	}

	// Parse resolution
	// Example: // Comments\nPath = DataSources.personal + \"/\" + User.Name;
	// Returns DataSources.personal + \"/\" + User.Name;
	re := regexp.MustCompile(`Path\s*=\s*(.*?)\s*;`)
	matches := re.FindStringSubmatch(resolution)

	if len(matches) < 2 {
		return "", fmt.Errorf("failed to parse resolution")
	}
	resolutionPath := matches[1]

	adminPath := strings.Replace(resolutionPath, " ", "", -1)
	replacer := strings.NewReplacer(
		"DataSources.", "",
		"+\"/\"+", "/",
		"User.Name", c.User,
	)
	adminPath = replacer.Replace(adminPath)
	return adminPath, nil
}

func (c *Client) DownloadNode(cellsSrc, dest string) (string, error) {
	cmd := exec.Command(c.CECPath, "scp", "-n", "--url", c.Address, "--skip-verify", "--login", c.User, "--token", c.UserToken, fmt.Sprintf("cells://%s/", cellsSrc), dest)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to execute command: %s err: %w, output: %s", cmd, err, output)
	}
	// fmt.Printf("Download Output: %s", output)
	return filepath.Join(dest, filepath.Base(cellsSrc)), nil
}

func (c *Client) UploadNode(src, cellsDest string) (string, error) {
	cmd := exec.Command(c.CECPath, "scp", "-n", "--url", c.Address, "--skip-verify", "--login", c.User, "--token", c.UserToken, src, fmt.Sprintf("cells://%s/", cellsDest))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to execute command: %w, output: %s", err, output)
	}
	// fmt.Printf("Upload Output: %s", output)
	return filepath.Join(cellsDest, filepath.Base(src)), nil
}

func (c *Client) GetNodeCollection(nodePath string) (NodeCollection, error) {

	nodeRoot := strings.Split(nodePath, "/")[0]
	adminNodeRoot, err := c.ConstructAdminWorkspaceRoot(nodeRoot)
	if err != nil {
		return NodeCollection{}, fmt.Errorf("error constructing admin workspace root: %w", err)
	}
	adminNodePath := strings.Replace(nodePath, nodeRoot, adminNodeRoot, 1)

	url := fmt.Sprintf("%s/a/tree/admin/list", c.Address)

	type Payload struct {
		Node struct {
			Path string `json:"Path"`
		} `json:"Node"`
		Recursive bool `json:"Recursive"`
	}

	payloadData := Payload{
		Node: struct {
			Path string `json:"Path"`
		}{
			Path: adminNodePath,
		},
		Recursive: true,
	}

	payload, err := json.Marshal(payloadData)
	if err != nil {
		return NodeCollection{}, fmt.Errorf("failed to marshal payload: %w", err)
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return NodeCollection{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.AdminToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return NodeCollection{}, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return NodeCollection{}, fmt.Errorf("failed to read response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return NodeCollection{}, fmt.Errorf("failed to get node collection: %d %s", resp.StatusCode, string(body))
	}
	// fmt.Printf("Node collection body: %s\n", string(body))
	var nodeCollection NodeCollection
	err = json.Unmarshal(body, &nodeCollection)
	if err != nil {
		return NodeCollection{}, fmt.Errorf("failed to unmarshal response body: %w", err)
	}
	if nodeCollection.Parent.Path == "" {
		return NodeCollection{}, fmt.Errorf("node collection is empty")
	}
	return nodeCollection, nil
}

func (c *Client) UpdateTag(nodeUuid, namespace, content string) error {
	url := fmt.Sprintf("%s/a/user-meta/update", c.Address)

	type MetaData struct {
		JsonValue string `json:"JsonValue"`
		Namespace string `json:"Namespace"`
		NodeUuid  string `json:"NodeUuid"`
	}
	type Payload struct {
		MetaDatas []MetaData `json:"MetaDatas"`
		Operation string     `json:"Operation"`
	}

	payloadData := Payload{
		MetaDatas: []MetaData{
			{
				JsonValue: fmt.Sprintf("\"%s\"", content),
				Namespace: namespace,
				NodeUuid:  nodeUuid,
			},
		},
		Operation: "PUT",
	}

	// Marshal the payload into JSON
	payload, err := json.Marshal(payloadData)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create a new POST request with the JSON payload.
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.UserToken)
	req.Header.Set("Content-Type", "application/json")

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}
		return fmt.Errorf("request gave status code: %d %s", resp.StatusCode, string(body))
	}
	return nil
}
