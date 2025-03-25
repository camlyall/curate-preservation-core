package preservation

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/penwern/preservation-go/internal/a3mclient"
	"github.com/penwern/preservation-go/internal/cells"
	"github.com/penwern/preservation-go/internal/processor"
	"github.com/penwern/preservation-go/pkg/config"
	"github.com/penwern/preservation-go/pkg/utils"
	"github.com/pydio/cells-sdk-go/v4/models"
)

type Service struct {
	a3mClient   a3mclient.ClientInterface
	cellsClient cells.ClientInterface
	config      *config.Config
}

func NewService(ctx context.Context, cfg *config.Config) *Service {
	cellsClient, err := cells.NewClient(ctx, cfg.CellsCecPath, cfg.CellsAddress, cfg.CellsAdminToken)
	if err != nil {
		panic(fmt.Errorf("cells client error: %w", err))
	}
	a3mClient, err := a3mclient.NewClient(cfg.A3mAddress)
	if err != nil {
		panic(fmt.Errorf("a3m client error: %w", err))
	}
	return &Service{
		a3mClient:   a3mClient,
		cellsClient: cellsClient,
		config:      cfg,
	}
}

func (s *Service) Close(ctx context.Context) {
	log.Printf("Closing Clients\n")
	s.cellsClient.Close(ctx)
	s.a3mClient.Close()
}

func (s *Service) Run(ctx context.Context, pcfg *config.PreservationConfig, cellsUserName, cellsPackagePath string, cleanUp bool) error {

	///////////////////////////////////////////////////////////////////
	//						Pre-requisites							 //
	///////////////////////////////////////////////////////////////////

	// Create the Cells User Client. Attaches to root Client
	s.cellsClient.NewUserClient(ctx, cellsUserName)

	nodeCollection, updateTag, err := s.gatherNodeEnvironment(ctx, cellsPackagePath)
	if err != nil {
		return fmt.Errorf("error gathering node environment: %w", err)
	}
	// Ensure the tag is updated on failure
	defer func() {
		if err != nil {
			if updateErr := updateTag("❌ Failed"); updateErr != nil {
				log.Printf("Error updating Cells tag on failure: %v\n", updateErr)
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
	log.Printf("Created Processing directory: \t%s\n", processingDir)
	// Clean up the processing directory
	defer func() {
		if cleanUp && processingDir != "" {
			if err := os.RemoveAll(processingDir); err != nil {
				log.Printf("Error deleting processing directory: %v\n", err)
			}
			log.Printf("Deleted processing directory: \t%s\n", processingDir)
		}
	}()

	///////////////////////////////////////////////////////////////////
	//					 Download Cells Package						 //
	///////////////////////////////////////////////////////////////////

	// Tag Package: Downloading
	if err = updateTag("🌐 Downloading..."); err != nil {
		return fmt.Errorf("error updating Cells tag: %w", err)
	}
	// Download the package
	downloadedPath, err := s.downloadPackage(ctx, processingDir, cellsPackagePath)
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
	// Preprocess package
	transferPath, err := s.preprocessPackage(ctx, processingDir, downloadedPath, nodeCollection)
	if err != nil {
		return fmt.Errorf("error preprocessing package: %w", err)
	}

	///////////////////////////////////////////////////////////////////
	//						 Submit to A3M							 //
	///////////////////////////////////////////////////////////////////

	// Tag Package: Preserving
	if err = updateTag("📦 Preserving..."); err != nil {
		return fmt.Errorf("error updating Cells tag: %w", err)
	}
	// Submit package to A3M
	a3mAipPath, err := s.submitPackage(ctx, transferPath)
	if err != nil {
		return fmt.Errorf("failed to submit package: %w (path: %s)", err, transferPath)
	}
	// Clean up the processing directory
	defer func() {
		if cleanUp && a3mAipPath != "" {
			if err := os.Remove(a3mAipPath); err != nil {
				log.Printf("Error deleting A3M AIP: %v\n", err)
			}
			log.Printf("Deleted A3M AIP: \t\t\t%s\n", a3mAipPath)
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
	s.uploadPackage(ctx, aipPath)
	// Tag Package: Preserved
	if err = updateTag("🔒 Preserved"); err != nil {
		return fmt.Errorf("error updating Cells tag: %w", err)
	}

	log.Printf("Preservation successful: \t\t%s\n", filepath.Base(aipPath))
	return nil
}

func (s *Service) gatherNodeEnvironment(ctx context.Context, cellsPackagePath string) (*models.RestNodesCollection, func(string) error, error) {

	// Get the resolved cells path, parsing cells template path if necessary
	resolvedCellsPackagePath, err := s.cellsClient.ResolveCellsPath(cellsPackagePath)
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

	// Define the pre-configured update tag function
	updateTag := func(status string) error {
		return s.cellsClient.UpdateTag(ctx, parentNodeUuid, preservationProgressTag, status)
	}

	return nodeCollection, updateTag, nil
}

// Download package
func (s *Service) downloadPackage(ctx context.Context, processingDir, packagePath string) (string, error) {
	log.Printf("Downloading package: \t\t%s\n", packagePath)
	downloadDir := filepath.Join(processingDir, "cells_download")
	if err := utils.CreateDir(downloadDir); err != nil {
		return "", fmt.Errorf("failed to create download directory: %w", err)
	}
	downloadedPath, err := s.cellsClient.DownloadNode(ctx, packagePath, downloadDir)
	if err != nil {
		return "", fmt.Errorf("download error: %w", err)
	}
	return downloadedPath, nil
}

// Preprocess package
func (s *Service) preprocessPackage(ctx context.Context, processingDir, packagePath string, nodeCollection *models.RestNodesCollection) (string, error) {
	log.Printf("Preprocessing package: \t\t%s\n", utils.RelPath(s.config.ProcessingBaseDir, packagePath))
	// Create the a3m transfer directory
	a3mTransferDir := filepath.Join(processingDir, "a3m_transfer")
	if err := utils.CreateDir(a3mTransferDir); err != nil {
		return "", fmt.Errorf("failed to create a3m transfer directory: %w", err)
	}
	// Preprocess package
	transferPath, err := processor.PreprocessPackage(ctx, packagePath, a3mTransferDir, nodeCollection, s.cellsClient.GetUserClientUserData())
	if err != nil {
		return "", fmt.Errorf("error preprocessing package: %w", err)
	}
	return transferPath, nil
}

// Submit package to A3M
func (s *Service) submitPackage(ctx context.Context, transferPath string) (string, error) {
	log.Printf("Submitting A3M Transfer: \t\t%s\n", utils.RelPath(s.config.ProcessingBaseDir, transferPath))
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	aipUuid, _, err := s.a3mClient.SubmitPackage(ctx, transferPath, filepath.Base(transferPath), nil)
	if err != nil {
		return "", fmt.Errorf("submission failed: %v", err)
	}
	// Find the a3m AIP
	a3mAipPath, err := getA3mAipPath(s.config.A3mCompletedDir, filepath.Base(transferPath), aipUuid)
	if err != nil {
		return "", fmt.Errorf("error getting A3M AIP path: %v", err)
	}
	log.Printf("A3M AIP generated at: \t\t%s\n", a3mAipPath)
	return a3mAipPath, nil
}

// Post-processes the AIP. Compresses if configured.
func (s *Service) postprocessPackage(ctx context.Context, processingAipDir, a3mAipPath string) (string, error) {
	// Extract AIP
	aipPath, err := utils.ExtractArchive(ctx, a3mAipPath, processingAipDir)
	if err != nil {
		return "", fmt.Errorf("error extracting AIP: %w", err)
	}
	log.Printf("Extracted AIP to: \t\t\t%s\n", utils.RelPath(s.config.ProcessingBaseDir, aipPath))
	return aipPath, nil
}

// Convert the AIP to a ZIP archive.
func (s *Service) compressPackage(ctx context.Context, processingAipDir, aipPath string) (string, error) {
	archiveAipPath := filepath.Join(processingAipDir, fmt.Sprintf("%s.zip", filepath.Base(aipPath)))
	err := utils.CompressToZip(ctx, aipPath, archiveAipPath)
	if err != nil {
		return "", fmt.Errorf("error compressing AIP: %w", err)
	}
	log.Printf("Compressed AIP to: \t\t\t%s\n", utils.RelPath(s.config.ProcessingBaseDir, archiveAipPath))
	return archiveAipPath, nil
}

// Uploads the AIP to Cells
func (s *Service) uploadPackage(ctx context.Context, aipPath string) error {
	cellsUploadPath, err := s.cellsClient.UploadNode(ctx, aipPath, s.config.CellsArchiveWorkspace)
	if err != nil {
		return fmt.Errorf("error uploading AIP: %w", err)
	}
	log.Printf("Uploaded AIP to: \t\t\t%s\n", cellsUploadPath)
	return nil
}

// Construct the path of the A3M Generated AIP and ensures it exists
// TODO: Consider AIP Compression Algorithm
func getA3mAipPath(a3mCompletedDir string, packageName string, packageUUID string) (string, error) {
	sanitisedPackageName := strings.ReplaceAll(packageName, " ", "")
	expectedAIPPath := filepath.Join(a3mCompletedDir, sanitisedPackageName+"-"+packageUUID+".7z")
	if _, err := os.Stat(expectedAIPPath); os.IsNotExist(err) {
		log.Printf("A3M AIP not found: %v\n", err)
		return "", err
	}
	return expectedAIPPath, nil
}
