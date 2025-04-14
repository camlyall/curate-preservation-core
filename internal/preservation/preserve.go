package preservation

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/penwern/preservation-go/internal/a3mclient"
	"github.com/penwern/preservation-go/internal/cells"
	"github.com/penwern/preservation-go/internal/processor"
	"github.com/penwern/preservation-go/pkg/config"
	"github.com/penwern/preservation-go/pkg/logger"
	"github.com/penwern/preservation-go/pkg/utils"
	"github.com/pydio/cells-sdk-go/v4/models"
)

// Preserver is the service for the preservation process
type Preserver struct {
	a3mClient   a3mclient.ClientInterface
	cellsClient cells.ClientInterface
	config      *config.Config
}

// NewPreserver creates a new preservation service.
// Initializes the Cells and A3M clients.
// Panics if the clients cannot be created.
func NewPreserver(ctx context.Context, cfg *config.Config) *Preserver {
	a3mClient, err := a3mclient.NewClient(cfg.A3mAddress)
	if err != nil {
		panic(fmt.Errorf("a3m client error: %w", err))
	}
	return NewPreserverWithA3MClient(ctx, cfg, a3mClient)
}

func NewPreserverWithA3MClient(ctx context.Context, cfg *config.Config, a3mClient a3mclient.ClientInterface) *Preserver {
	cellsClient, err := cells.NewClient(ctx, cfg.CellsCecPath, cfg.CellsAddress, cfg.CellsAdminToken)
	if err != nil {
		panic(fmt.Errorf("cells client error: %w", err))
	}
	return &Preserver{
		a3mClient:   a3mClient,
		cellsClient: cellsClient,
		config:      cfg,
	}
}

func (s *Preserver) Close() {
	logger.Debug("Closing Clients")
	s.cellsClient.Close()
	s.a3mClient.Close()
}

func (s *Preserver) Run(ctx context.Context, pcfg *config.PreservationConfig, userClient cells.UserClient, cellsPackagePath string, cleanUp, pathResolved bool) error {

	///////////////////////////////////////////////////////////////////
	//						Pre-requisites							 //
	///////////////////////////////////////////////////////////////////

	// Unresolve resolved paths
	if pathResolved {
		var err error
		cellsPackagePath, err = s.cellsClient.UnresolveCellsPath(userClient, cellsPackagePath)
		if err != nil {
			return fmt.Errorf("error unresolving cells path: %w", err)
		}
		logger.Info("Unresolved Cells Path: %s", cellsPackagePath)
	}

	// Gather the node environment
	nodeCollection, updateTag, err := s.gatherNodeEnvironment(ctx, userClient, cellsPackagePath)
	if err != nil {
		return fmt.Errorf("error gathering node environment: %w", err)
	}
	// Ensure the tag is updated on failure
	defer func() {
		if err != nil {
			if updateErr := updateTag("❌ Failed"); updateErr != nil {
				logger.Error("Error updating Cells tag on failure: %v", updateErr)
			}
		}
	}()

	///////////////////////////////////////////////////////////////////
	//						Start Processing						 //
	///////////////////////////////////////////////////////////////////

	// Tag Package: Starting
	if err = updateTag("🟢 Starting..."); err != nil {
		return fmt.Errorf("error updating Cells tag: %w", err)
	}
	// Create unique processing directory
	processingDir, err := utils.MakeUniqueDir(ctx, s.config.ProcessingBaseDir)
	if err != nil {
		return fmt.Errorf("failed to create processing directory: %w", err)
	}
	logger.Info("Created processing dir: %s", processingDir)
	// Clean up the processing directory
	defer func() {
		if cleanUp && processingDir != "" {
			if err := os.RemoveAll(processingDir); err != nil {
				logger.Error("Error deleting processing directory: %v", err)
			}
			logger.Debug("Deleted processing dir: %s", processingDir)
		}
	}()

	///////////////////////////////////////////////////////////////////
	//					 Download Cells Package						 //
	///////////////////////////////////////////////////////////////////

	// Tag Package: Downloading
	if err = updateTag("🌐 Downloading..."); err != nil {
		return fmt.Errorf("error updating Cells tag: %w", err)
	}
	downloadedPath, err := s.downloadPackage(ctx, userClient, processingDir, cellsPackagePath)
	if err != nil {
		return fmt.Errorf("error downloading package: %v", err)
	}

	///////////////////////////////////////////////////////////////////
	//						 Preprocessing							 //
	///////////////////////////////////////////////////////////////////

	// Tag Package: Preprocessing
	if err = updateTag("🗂️ Preprocessing..."); err != nil {
		return fmt.Errorf("error updating Cells tag: %w", err)
	}
	// Preprocess package. Don't use retry as we move/extract the package in the first step
	transferPath, err := s.preprocessPackage(ctx, processingDir, downloadedPath, nodeCollection, userClient.UserData)
	if err != nil {
		return fmt.Errorf("error preprocessing package: %w", err)
	}

	///////////////////////////////////////////////////////////////////
	//						 Submit to A3M							 //
	///////////////////////////////////////////////////////////////////

	// Tag Package: Preserving
	if err = updateTag("📦 Packaging..."); err != nil {
		return fmt.Errorf("error updating Cells tag: %w", err)
	}
	a3mAipPath, err := s.submitPackage(ctx, transferPath)
	if err != nil {
		return fmt.Errorf("failed to submit package: %w (path: %s)", err, transferPath)
	}
	defer func() {
		if cleanUp && a3mAipPath != "" {
			if err := os.Remove(a3mAipPath); err != nil {
				logger.Error("Error deleting A3M AIP: %v", err)
			}
			logger.Debug("Deleted A3M AIP: %s", a3mAipPath)
		}
	}()

	///////////////////////////////////////////////////////////////////
	//						 Postprocessing							 //
	///////////////////////////////////////////////////////////////////

	// Tag Package: Extracting
	if err = updateTag("🗃️ Extracting..."); err != nil {
		return fmt.Errorf("error updating Cells tag: %w", err)
	}
	// Create AIP Directory
	processingAipDir := filepath.Join(processingDir, "aip")
	if err := utils.CreateDir(processingAipDir); err != nil {
		return fmt.Errorf("failed to create AIP directory: %w", err)
	}
	// Post-process package
	aipPath, err := s.postprocessPackage(ctx, processingAipDir, a3mAipPath)
	if err != nil {
		return fmt.Errorf("error postprocessing package: %w", err)
	}
	if pcfg.CompressAip {
		// Tag Package: Compressing
		if err = updateTag("🗃️ Compressing..."); err != nil {
			return fmt.Errorf("error updating Cells tag: %w", err)
		}
		// Compress AIP
		aipPath, err = s.compressPackage(ctx, processingAipDir, aipPath)
		if err != nil {
			return fmt.Errorf("error compressing AIP: %w", err)
		}
	}

	///////////////////////////////////////////////////////////////////
	//						 Uploading AIP							 //
	///////////////////////////////////////////////////////////////////

	// Tag Package: Uploading
	if err = updateTag("🌐 Uploading..."); err != nil {
		return fmt.Errorf("error updating Cells tag: %w", err)
	}
	// Upload Node
	s.uploadPackage(ctx, userClient, aipPath)
	// Tag Package: Preserved
	if err = updateTag("🔒 Preserved"); err != nil {
		return fmt.Errorf("error updating Cells tag: %w", err)
	}

	logger.Info("Preservation successful: %s", filepath.Base(aipPath))
	if s.config.LogLevel == "ERROR" {
		logger.Error("Preservation successful: %s", filepath.Base(aipPath))
	}
	if cleanUp {
		logger.Info("Cleaning up.")
	}
	return nil
}

func (s *Preserver) GetUserClient(ctx context.Context, username string) (cells.UserClient, error) {
	return s.cellsClient.NewUserClient(ctx, username)
}

// Gather the node environment. Returns the node collection and the update tag function.
func (s *Preserver) gatherNodeEnvironment(ctx context.Context, userClient cells.UserClient, cellsPackagePath string) (*models.RestNodesCollection, func(string) error, error) {

	// Get the resolved cells path, parsing cells template path if necessary
	resolvedCellsPackagePath, err := s.cellsClient.ResolveCellsPath(userClient, cellsPackagePath)
	if err != nil {
		return nil, nil, fmt.Errorf("error resolving cells path: %w", err)
	}

	// Collect the package node data
	nodeCollection, err := s.cellsClient.GetNodeCollection(ctx, resolvedCellsPackagePath)
	if err != nil {
		return nil, nil, fmt.Errorf("error getting node collection: %w", err)
	}

	// Set the parent node uuid
	parentNodeUuid := nodeCollection.Parent.UUID

	// Decide which cells usermeta namespace to use for preservation tagging.
	// Cells doesn't add namespace to the node until it is used.
	// Which makes it difficult to determine which namespace to use.
	// If the old namespace is present on the node, use that.
	// Otherwise, use the new namespace. We assume it is present.
	var preservationProgressTag string
	if nodeCollection.Parent.MetaStore["usermeta-a3m-progress"] != "" {
		preservationProgressTag = "usermeta-a3m-progress"
	} else {
		preservationProgressTag = "usermeta-preservation-status"
	}

	// Define the pre-configured update tag function using retry
	updateTag := func(status string) error {
		return utils.Retry(3, 2*time.Second, func() error {
			logger.Debug("Tagging: {node: %s, tag: %s, status: %s}", parentNodeUuid, preservationProgressTag, status)
			return s.cellsClient.UpdateTag(ctx, userClient, parentNodeUuid, preservationProgressTag, status)
		}, utils.IsTransientError)
	}

	return nodeCollection, updateTag, nil
}

// Download package. Uses the Cells Client. Retries on transient errors.
func (s *Preserver) downloadPackage(ctx context.Context, userClient cells.UserClient, processingDir, packagePath string) (string, error) {
	logger.Info("Downloading package: %s", packagePath)
	downloadDir := filepath.Join(processingDir, "cells_download")
	if err := utils.CreateDir(downloadDir); err != nil {
		return "", fmt.Errorf("failed to create download directory: %w", err)
	}
	// TODO: I don't think retry will work here because the download is executed using CEC binary, so doesn't produce a transient error.
	var downloadedPath string
	err := utils.Retry(3, 2*time.Second, func() error {
		var downloadErr error
		downloadedPath, downloadErr = s.cellsClient.DownloadNode(ctx, userClient, packagePath, downloadDir)
		return downloadErr
	}, utils.IsTransientError)
	if err != nil {
		return "", fmt.Errorf("download error: %w", err)
	}
	return downloadedPath, nil
}

// Preprocess package. Uses preproces module. Constructs the a3m tranfer package. Writes DC and Premis Metadata.
func (s *Preserver) preprocessPackage(ctx context.Context, processingDir, packagePath string, nodeCollection *models.RestNodesCollection, userData *models.IdmUser) (string, error) {
	logger.Info("Preprocessing package: %s", utils.RelPath(s.config.ProcessingBaseDir, packagePath))
	// Create the a3m transfer directory
	a3mTransferDir := filepath.Join(processingDir, "a3m_transfer")
	if err := utils.CreateDir(a3mTransferDir); err != nil {
		return "", fmt.Errorf("failed to create a3m transfer directory: %w", err)
	}
	// Preprocess package
	transferPath, err := processor.PreprocessPackage(ctx, packagePath, a3mTransferDir, nodeCollection, userData)
	if err != nil {
		return "", fmt.Errorf("error preprocessing package: %w", err)
	}
	return transferPath, nil
}

// Submit package to A3M. Submits the package to A3M and returns the path of the generated AIP.
// The generated AIP is expected to be in the configured A3M Completed directory.
// Will retry submission on transient errors.
func (s *Preserver) submitPackage(ctx context.Context, transferPath string) (string, error) {
	transferName := strings.ReplaceAll(filepath.Base(transferPath), " ", "")
	var aipUuid string
	// Submit package to A3M with retry
	err := utils.Retry(3, 2*time.Second, func() error {
		logger.Info("Queing A3M Transfer: %s", utils.RelPath(s.config.ProcessingBaseDir, transferPath))
		ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
		defer cancel()
		var submitErr error
		aipUuid, _, submitErr = s.a3mClient.SubmitPackage(ctx, transferPath, transferName, nil)
		return submitErr
	}, utils.IsTransientError)
	if err != nil {
		return "", fmt.Errorf("submission failed: %v", err)
	}
	a3mAipPath, err := getA3mAipPath(s.config.A3mCompletedDir, transferName, aipUuid)
	if err != nil {
		return "", fmt.Errorf("error getting A3M AIP path: %v", err)
	}
	logger.Info("Generated A3M AIP: %s", a3mAipPath)
	return a3mAipPath, nil
}

// Post-processes the AIP. Extracts the AIP.
func (s *Preserver) postprocessPackage(ctx context.Context, processingAipDir, a3mAipPath string) (string, error) {
	// Extract AIP
	aipPath, err := utils.ExtractArchive(ctx, a3mAipPath, processingAipDir)
	if err != nil {
		return "", fmt.Errorf("error extracting AIP: %w", err)
	}
	logger.Info("Extracted AIP %s", utils.RelPath(s.config.ProcessingBaseDir, aipPath))
	return aipPath, nil
}

// Convert the AIP to a ZIP archive.
func (s *Preserver) compressPackage(ctx context.Context, processingAipDir, aipPath string) (string, error) {
	archiveAipPath := filepath.Join(processingAipDir, fmt.Sprintf("%s.zip", filepath.Base(aipPath)))
	err := utils.CompressToZip(ctx, aipPath, archiveAipPath)
	if err != nil {
		return "", fmt.Errorf("error compressing AIP: %w", err)
	}
	logger.Info("Compressed AIP %s", utils.RelPath(s.config.ProcessingBaseDir, archiveAipPath))
	return archiveAipPath, nil
}

// Uploads the AIP to Cells
func (s *Preserver) uploadPackage(ctx context.Context, userClient cells.UserClient, aipPath string) error {
	cellsUploadPath, err := s.cellsClient.UploadNode(ctx, userClient, aipPath, s.config.CellsArchiveWorkspace)
	if err != nil {
		return fmt.Errorf("error uploading AIP: %w", err)
	}
	logger.Info("Uploaded AIP %s", cellsUploadPath)
	return nil
}

// Construct the path of the A3M Generated AIP and ensures it exists
// TODO: Consider AIP Compression Algorithm
func getA3mAipPath(a3mCompletedDir string, packageName string, packageUUID string) (string, error) {
	// sanitisedPackageName := strings.ReplaceAll(packageName, " ", "")
	expectedAIPPath := filepath.Join(a3mCompletedDir, packageName+"-"+packageUUID+".7z")
	if _, err := os.Stat(expectedAIPPath); os.IsNotExist(err) {
		logger.Error("A3M AIP not found: %v", err)
		return "", err
	}
	return expectedAIPPath, nil
}
