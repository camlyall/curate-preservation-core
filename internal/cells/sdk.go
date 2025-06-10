package cells

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	httptransport "github.com/go-openapi/runtime/client"

	"github.com/pydio/cells-sdk-go/v4/client"
	"github.com/pydio/cells-sdk-go/v4/client/admin_tree_service"
	"github.com/pydio/cells-sdk-go/v4/client/user_meta_service"
	"github.com/pydio/cells-sdk-go/v4/client/user_service"
	"github.com/pydio/cells-sdk-go/v4/client/workspace_service"
	"github.com/pydio/cells-sdk-go/v4/models"
)

func newSDKClient(scheme, host, basePath string, insecure bool, pat string) (*client.PydioCellsRestAPI, error) {
	cfg := client.DefaultTransportConfig().
		WithHost(host).
		WithBasePath(basePath).
		WithSchemes([]string{scheme})

	cellsClient := client.NewHTTPClientWithConfig(nil, cfg)
	transport, ok := cellsClient.Transport.(*httptransport.Runtime)
	if !ok {
		return nil, fmt.Errorf("unexpected transport type from cells-sdk-go")
	}

	transport.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure},
	}
	auth := httptransport.BearerToken(pat)
	transport.DefaultAuthentication = auth
	return client.New(transport, nil), nil
}

func sdkUpdateUserMeta(ctx context.Context, client client.PydioCellsRestAPI, nodeUUID, namespace, content string) error {
	updateParams := user_meta_service.NewUpdateUserMetaParamsWithContext(ctx)
	updateParams.Body = &models.IdmUpdateUserMetaRequest{
		MetaDatas: []*models.IdmUserMeta{
			{
				NodeUUID:  nodeUUID,
				Namespace: namespace,
				JSONValue: fmt.Sprintf("\"%s\"", content),
			},
		},
		Operation: models.UpdateUserMetaRequestUserMetaOpPUT.Pointer(),
	}

	updateUserMetaOK, err := client.UserMetaService.UpdateUserMeta(updateParams)
	if err != nil {
		return fmt.Errorf("error updating metadata {namespace: %s, content %s, nodeUuid: %s} : %v", namespace, content, nodeUUID, err)
	}
	if updateUserMetaOK == nil || updateUserMetaOK.GetPayload() == nil {
		return fmt.Errorf("no payload returned when updating metadata {namespace: %s, content %s, nodeUuid: %s}", namespace, content, nodeUUID)
	}
	return nil
}

func sdkGetNodeCollection(ctx context.Context, client client.PydioCellsRestAPI, nodePath string) (*models.RestNodesCollection, error) {
	nodeParams := admin_tree_service.NewListAdminTreeParamsWithContext(ctx)
	nodeParams.Body = &models.TreeListNodesRequest{
		Node: &models.TreeNode{
			Path: nodePath,
		},
		Recursive: true,
	}
	nodeCollectionOk, err := client.AdminTreeService.ListAdminTree(nodeParams)
	if err != nil {
		return nil, fmt.Errorf("error getting node collection: %v", err)
	}
	return nodeCollectionOk.GetPayload(), nil
}

func sdkGetNodeStats(ctx context.Context, client client.PydioCellsRestAPI, nodePath string) (*models.TreeReadNodeResponse, error) {
	nodeParams := admin_tree_service.NewStatAdminTreeParamsWithContext(ctx)
	nodeParams.Body = &models.TreeReadNodeRequest{
		Node: &models.TreeNode{
			Path: nodePath,
		},
	}
	nodeStatsOk, err := client.AdminTreeService.StatAdminTree(nodeParams)
	if err != nil {
		return nil, fmt.Errorf("error getting node stats: %v", err)
	}
	return nodeStatsOk.GetPayload(), nil
}

func sdkGetWorkspaceCollection(ctx context.Context, client client.PydioCellsRestAPI) (*models.RestWorkspaceCollection, error) {
	workspaceParams := workspace_service.NewSearchWorkspacesParamsWithContext(ctx)
	workspaceParams.Body = &models.RestSearchWorkspaceRequest{
		Queries: []*models.IdmWorkspaceSingleQuery{
			{
				Scope: models.IdmWorkspaceScopeADMIN.Pointer(),
			},
		},
	}
	workspacesOk, err := client.WorkspaceService.SearchWorkspaces(workspaceParams)
	if err != nil {
		return nil, fmt.Errorf("error getting workspaces: %v", err)
	}
	return workspacesOk.GetPayload(), nil
}

// Not required due to passing in input from Cells
func sdkGetUserData(ctx context.Context, client client.PydioCellsRestAPI, user string) (*models.IdmUser, error) {
	userParams := user_service.NewGetUserParamsWithContext(ctx)
	userParams.Login = user
	userOk, err := client.UserService.GetUser(userParams)
	if err != nil {
		return nil, fmt.Errorf("error getting user data: %v", err)
	}
	return userOk.GetPayload(), nil
}

// TODO: Implement sdkUploadNode function
// func sdkUploadNode() {}

// TODO: Implement sdkDownloadNode function
// func sdkDownloadNode() {}
