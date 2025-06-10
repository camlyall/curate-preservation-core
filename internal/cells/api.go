// Package cells for interacting with Pydio Cells API
// Package cells provides functions to interact with the Pydio Cells API for user token management.
// It includes functions to generate and revoke user tokens, and to update user metadata.
package cells

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/penwern/curate-preservation-core/pkg/logger"
	"github.com/penwern/curate-preservation-core/pkg/utils"
)

// apiGenerateUserToken generates a user token for the given user
// https://pydio.com/en/docs/developer-guide/post-aauthtokenimpersonate
func apiGenerateUserToken(ctx context.Context, client *utils.HTTPClient, address, username, adminToken string, timeout time.Duration) (string, error) {
	url := fmt.Sprintf("%s/a/auth/token/impersonate", address)

	body := map[string]any{
		"Label":     "Preservation Token",
		"UserLogin": username,
		"ExpiresAt": time.Now().Add(timeout).Unix(),
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("error marshalling body: %w", err)
	}

	headers := map[string]string{
		"Accept":        "application/json",
		"Authorization": "Bearer " + adminToken,
		"Content-Type":  "application/json",
	}

	resp, err := client.DoRequest(ctx, "POST", url, bytes.NewBuffer(jsonBody), headers)
	if err != nil {
		return "", fmt.Errorf("error requesting user token: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Error("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("error reading response body: %w", err)
		}
		return "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var response struct {
		AccessToken string `json:"AccessToken"`
	}
	if err := utils.ParseResponse(resp, &response); err != nil {
		return "", fmt.Errorf("error parsing response: %w", err)
	}

	return response.AccessToken, nil
}

// apiRevokeUserToken revoke a user token from the Cells server.
// https://pydio.com/en/docs/developer-guide/delete-aauthtokenstokenid
// TODO: This deosn't work because the token UUID is required, not the token itself.
// Need to get the UUID but observing difference before and after generating.
// But it is an imperfect solution.
// func apiRevokeUserToken(ctx context.Context, client *utils.HttpClient, address, token, userToken string) error {
// 	url := fmt.Sprintf("%s/a/auth/tokens/%s", address, userToken)
// 	headers := map[string]string{
// 		"Authorization": "Bearer " + token,
// 		"Content-Type":  "application/json",
// 	}
// 	resp, err := client.DoRequest(ctx, "DELETE", url, nil, headers)
// 	if err != nil {
// 		return fmt.Errorf("error deleting user token: %w", err)
// 	}
// 	defer resp.Body.Close()
// 	if resp.StatusCode != http.StatusOK {
// 		body, _ := io.ReadAll(resp.Body)
// 		return fmt.Errorf("unexpected status %d %s", resp.StatusCode, string(body))
// 	}
// 	var response struct {
// 		Success bool `json:"success"`
// 	}
// 	if err := utils.ParseResponse(resp, &response); err != nil {
// 		return fmt.Errorf("error parsing response: %w", err)
// 	}
// 	if !response.Success {
// 		return fmt.Errorf("failed to delete user token")
// 	}
// 	return nil
// }

// Support for both premis metadatas until usermeta-premis-data is phased out
// type NodeData struct {
// 	Uuid      string            `json:"Uuid"`
// 	Path      string            `json:"Path"`
// 	MetaStore map[string]string `json:"MetaStore"`
// }

// type NodeCollection struct {
// 	Parent   NodeData   `json:"Parent"`
// 	Children []NodeData `json:"Children"`
// }

// type Workspace struct {
// 	Slug      string `json:"Slug"`
// 	RootNodes map[string]struct {
// 		MetaStore struct {
// 			Resolution string `json:"resolution"`
// 		} `json:"MetaStore"`
// 	} `json:"RootNodes"`
// }

// Pydio Cells: /definitions/restWorkspaceCollection
// type WorkspaceCollection struct {
// 	Workspaces []Workspace `json:"Workspaces"`
// }

// Pydio Cells: /definitions/restUsersCollection
// type UserData struct {
// 	Uuid       string `json:"Uuid"`
// 	GroupPath  string `json:"GroupPath"`
// 	Attributes struct {
// 		DisplayName string `json:"displayName"`
// 	}
// 	Login string `json:"Login"`
// }

// updateTag updates a user metadata tag for a node
// https://pydio.com/en/docs/developer-guide/put-auser-metaupdate
// func updateTag(ctx context.Context, client *utils.HttpClient, address, token, nodeUuid, namespace, content string) error {
// 	url := fmt.Sprintf("%s/a/user-meta/update", address)
// 	body := map[string]any{
// 		"MetaDatas": []map[string]any{
// 			{
// 				"NodeUuid":  nodeUuid,
// 				"Namespace": namespace,
// 				"JsonValue": fmt.Sprintf("\"%s\"", content),
// 			},
// 		},
// 		"Operation": "PUT",
// 	}
// 	jsonBody, err := json.Marshal(body)
// 	if err != nil {
// 		return fmt.Errorf("error marshalling body: %w", err)
// 	}
// 	headers := map[string]string{
// 		"Authorization": "Bearer " + token,
// 		"Content-Type":  "application/json",
// 	}
// 	resp, err := client.DoRequest(ctx, "PUT", url, bytes.NewBuffer(jsonBody), headers)
// 	if err != nil {
// 		return fmt.Errorf("error updating tag: %w", err)
// 	}
// 	defer resp.Body.Close()
// 	if resp.StatusCode != http.StatusOK {
// 		body, _ := io.ReadAll(resp.Body)
// 		return fmt.Errorf("unexpected status %d %s", resp.StatusCode, string(body))
// 	}
// 	return nil
// }

// getWorkspaces returns the workspaces for the given address.
// Returns a WorkspaceCollection.
// https://pydio.com/en/docs/developer-guide/post-aworkspace
// func getWorkspaceCollection(ctx context.Context, client *utils.HttpClient, address, adminToken string) (WorkspaceCollection, error) {
// 	url := fmt.Sprintf("%s/a/workspace", address)
// 	body := map[string]any{
// 		"Queries": []struct {
// 			Scope string `json:"scope"`
// 		}{
// 			{
// 				Scope: "ADMIN",
// 			},
// 		},
// 	}
// 	jsonBody, err := json.Marshal(body)
// 	if err != nil {
// 		return WorkspaceCollection{}, fmt.Errorf("error marshalling body: %w", err)
// 	}
// 	headers := map[string]string{
// 		"Accept":        "application/json",
// 		"Authorization": "Bearer " + adminToken,
// 		"Content-Type":  "application/json",
// 	}
// 	resp, err := client.DoRequest(ctx, "POST", url, bytes.NewBuffer(jsonBody), headers)
// 	if err != nil {
// 		return WorkspaceCollection{}, fmt.Errorf("error getting workspace collection: %w", err)
// 	}
// 	defer resp.Body.Close()
// 	if resp.StatusCode != http.StatusOK {
// 		body, _ := io.ReadAll(resp.Body)
// 		return WorkspaceCollection{}, fmt.Errorf("unexpected status %d %s", resp.StatusCode, string(body))
// 	}
// 	var workspaceCollection WorkspaceCollection
// 	if err := utils.ParseResponse(resp, &workspaceCollection); err != nil {
// 		return WorkspaceCollection{}, fmt.Errorf("error parsing response: %w", err)
// 	}
// 	if len(workspaceCollection.Workspaces) == 0 {
// 		return WorkspaceCollection{}, fmt.Errorf("no workspaces found")
// 	}
// 	return workspaceCollection, nil
// }

// getNodeCollection gets node data for a given path. Including it's children.
// https://pydio.com/en/docs/developer-guide/post-atreeadminlist
// func getNodeCollection(ctx context.Context, client *utils.HttpClient, address, adminToken, nodePath string) (NodeCollection, error) {
// 	url := fmt.Sprintf("%s/a/tree/admin/list", address)
// 	body := map[string]any{
// 		"Node": map[string]string{
// 			"Path": nodePath,
// 		},
// 		"Recursive": true,
// 	}
// 	jsonBody, err := json.Marshal(body)
// 	if err != nil {
// 		return NodeCollection{}, fmt.Errorf("error marshalling body: %w", err)
// 	}
// 	headers := map[string]string{
// 		"Accept":        "application/json",
// 		"Authorization": "Bearer " + adminToken,
// 		"Content-Type":  "application/json",
// 	}
// 	resp, err := client.DoRequest(ctx, "POST", url, bytes.NewBuffer(jsonBody), headers)
// 	if err != nil {
// 		return NodeCollection{}, fmt.Errorf("error getting node collection: %w", err)
// 	}
// 	defer resp.Body.Close()
// 	if resp.StatusCode != http.StatusOK {
// 		body, _ := io.ReadAll(resp.Body)
// 		return NodeCollection{}, fmt.Errorf("unexpected status %d %s", resp.StatusCode, string(body))
// 	}
// 	var nodeCollection NodeCollection
// 	if err := utils.ParseResponse(resp, &nodeCollection); err != nil {
// 		return NodeCollection{}, fmt.Errorf("error parsing response: %w", err)
// 	}
// 	return nodeCollection, nil
// }

// apiGetUserData gets the user data for a given user.
// https://pydio.com/en/docs/developer-guide/post-auser
// func apiGetUserData(ctx context.Context, client *utils.HttpClient, address, token, user string) (UserData, error) {
// 	url := fmt.Sprintf("%s/a/user", address)
// 	body := map[string]any{
// 		"Limit": 1,
// 		"Queries": []map[string]any{
// 			{
// 				"Login": user,
// 			},
// 		},
// 	}
// 	jsonBody, err := json.Marshal(body)
// 	if err != nil {
// 		return UserData{}, fmt.Errorf("error marshalling body: %w", err)
// 	}
// 	headers := map[string]string{
// 		"Accept":        "application/json",
// 		"Authorization": "Bearer " + token,
// 		"Content-Type":  "application/json",
// 	}
// 	resp, err := client.DoRequest(ctx, "POST", url, bytes.NewBuffer(jsonBody), headers)
// 	if err != nil {
// 		return UserData{}, fmt.Errorf("error getting user data: %w", err)
// 	}
// 	defer resp.Body.Close()
// 	if resp.StatusCode != http.StatusOK {
// 		body, _ := io.ReadAll(resp.Body)
// 		return UserData{}, fmt.Errorf("unexpected status %d %s", resp.StatusCode, string(body))
// 	}
// 	var response struct {
// 		Users []UserData `json:"Users"`
// 	}
// 	if err := utils.ParseResponse(resp, &response); err != nil {
// 		return UserData{}, fmt.Errorf("error parsing response: %w", err)
// 	}
// 	userData := response.Users[0]
// 	if userData.Login == "" {
// 		return UserData{}, fmt.Errorf("user %s not found", user)
// 	}
// 	return userData, nil
// }
