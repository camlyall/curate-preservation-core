package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	transferservice "github.com/penwern/preservation-go/common/proto/a3m/gen/go/a3m/api/transferservice/v1beta1"
	"github.com/penwern/preservation-go/internal/a3mclient"
	"github.com/penwern/preservation-go/internal/cells"
	"github.com/penwern/preservation-go/internal/processor"
	"github.com/penwern/preservation-go/pkg/utils"
)

type Config struct {
	ProcessingBaseDir     string // Base directory for processing (required)
	A3mAddress            string // gRPC address (default: localhost:7000)
	A3mCompletedDir       string // Directory of A3M completed AIPs (default: /home/a3m/.local/share/a3m/share/completed)
	CellsCecPath          string // Path to cec binary (default: /usr/bin/cec)
	CellsAddress          string // HTTP address of Cells (default: https://localhost:8080)
	CellsArchiveWorkspace string // Workspace for Cells archive (default: common-files)
	// MySQLAddress      string // e.g. penwern:password@tcp(localhost:3306)/preservation
	// MongoAddress      string // e.g. mongodb://localhost:27017
}

// User stored variables
type ProcessingConfig struct {
	// ID                     uint16 // Unused
	// Name                   string // Unused
	// Description            string // Unused
	// ProcessType            string // eark or standard
	// ImageNormalizationTiff bool // Unused yet?
	// CellsEndpoint string // e.g. http://localhost:8080
	CompressAip bool
	A3mConfig   transferservice.ProcessingConfig
}

// Load and validate the configuration directory
func loadConfigDir(dir string) (string, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %v", err)
	}
	info, err := os.Stat(absDir)
	if err != nil {
		return "", fmt.Errorf("processing directory %q does not exist: %v", absDir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("processing path %q is not a directory", absDir)
	}
	return absDir, nil
}

// Load and validate the configuration executable
func loadConfigExecutable(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %v", err)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("cec path %q does not exist: %v", absPath, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("cec path %q is a directory, expected an executable", absPath)
	}
	if info.Mode().Perm()&0111 == 0 {
		return "", fmt.Errorf("cec path %q is not executable", absPath)
	}
	return absPath, nil
}

// Loads and validates configuration from environment variables
func loadConfig() (*Config, error) {
	processingDir, ok := os.LookupEnv("PROCESSING_BASE_DIR")
	if !ok || processingDir == "" {
		return nil, fmt.Errorf("PROCESSING_BASE_DIR environment variable is not set")
	}
	absProcessingBaseDir, err := loadConfigDir(processingDir)
	if err != nil {
		return nil, err
	}

	a3mCompletedDir, ok := os.LookupEnv("A3M_COMPLETED_DIR")
	if !ok || a3mCompletedDir == "" {
		fmt.Printf("A3M_COMPLETED_DIR environment variable is not set. Defaulting to /home/a3m/.local/share/a3m/share/completed\n")
		a3mCompletedDir = "/home/a3m/.local/share/a3m/share/completed"
	}
	absA3mCompletedDir, err := loadConfigDir(a3mCompletedDir)
	if err != nil {
		return nil, err
	}

	cecPath, ok := os.LookupEnv("CELLS_CEC_PATH")
	if !ok || cecPath == "" {
		fmt.Printf("CELLS_CEC_PATH environment variable is not set. Defaulting to /usr/bin/cec\n")
		cecPath = "/usr/bin/cec"
	}
	absCecPath, err := loadConfigExecutable(cecPath)
	if err != nil {
		return nil, err
	}

	cellsAddress, ok := os.LookupEnv("CELLS_ENDPOINT")
	if !ok || cellsAddress == "" {
		fmt.Printf("CELLS_ENDPOINT environment variable is not set. Defaulting to https://localhost:8080\n")
		cellsAddress = "https://localhost:8080"
	}
	err = utils.CheckHTTPConnection(cellsAddress)
	if err != nil {
		return nil, fmt.Errorf("error connecting to Cells: %v", err)
	}

	a3mAddress, ok := os.LookupEnv("A3M_ADDRESS")
	if !ok || a3mAddress == "" {
		fmt.Printf("A3M_ADDRESS environment variable is not set. Defaulting to localhost:7000\n")
		a3mAddress = "localhost:7000"
	}
	err = utils.CheckGRPCConnection(a3mAddress)
	if err != nil {
		return nil, fmt.Errorf("error connecting to A3M: %v", err)
	}

	cellsArchiveWorkspace, ok := os.LookupEnv("CELLS_ARCHIVE_WORKSPACE")
	if !ok || cellsArchiveWorkspace == "" {
		fmt.Printf("CELLS_ARCHIVE_WORKSPACE environment variable is not set. Defaulting to common-files\n")
		cellsArchiveWorkspace = "common-files"
	}
	// TODO: Validate Cells Path

	return &Config{
		ProcessingBaseDir:     absProcessingBaseDir,
		A3mAddress:            a3mAddress,
		A3mCompletedDir:       absA3mCompletedDir,
		CellsCecPath:          absCecPath,
		CellsAddress:          cellsAddress,
		CellsArchiveWorkspace: cellsArchiveWorkspace,
	}, nil
}

func main() {

	const deleteProcessingDir = true

	// Inputs
	// const cellsPackagePath = "personal-files/test_dir"
	const cellsPackagePath = "personal-files/england-tower-bridge.jpg"
	const cellsUserName = "cameron"

	///////////////////////////////////////////////////////////////////
	//						  Configuration							 //
	///////////////////////////////////////////////////////////////////

	// Load the .env file if not in production
	if os.Getenv("GO_ENV") != "production" {
		if err := godotenv.Load(); err != nil {
			fmt.Printf("No .env file found or failed to load: %v\n", err)
		}
	}
	cellsAdminToken := os.Getenv("CELLS_ADMIN_TOKEN")
	if cellsAdminToken == "" {
		fmt.Println("CELLS_ADMIN_TOKEN environment variable is not set")
		return
	}
	// Load and validate environment configuration
	cfg, err := loadConfig()
	if err != nil {
		fmt.Printf("Configuration error: %v\n", err)
		return
	}

	// Load and validate the processing configuration
	// TODO: Populate from config args
	pcfg := ProcessingConfig{
		CompressAip: false,
	}

	// Create a root context
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	///////////////////////////////////////////////////////////////////
	//						  Cells Cient							 //
	///////////////////////////////////////////////////////////////////

	// Create Cells client
	cellsClient, err := cells.NewClient(ctx, cfg.CellsCecPath, cfg.CellsAddress, cellsUserName, cellsAdminToken)
	if err != nil {
		fmt.Printf("Error creating Cells client: %v\n", err)
		return
	}
	defer cellsClient.Close(ctx)

	resolvedCellsPackagePath, err := cellsClient.ResolveCellsPath(cellsPackagePath)
	if err != nil {
		fmt.Printf("Error resolving cells path: %v\n", err)
		return
	}

	fmt.Printf("Resolved Cells Path: %s\n", resolvedCellsPackagePath)

	// Collect the package node data
	nodeCollection, err := cellsClient.GetNodeCollection(ctx, resolvedCellsPackagePath)
	if err != nil {
		fmt.Printf("Error getting node collection: %v\n", err)
		return
	}
	fmt.Printf("Number of children: %d\n", len(nodeCollection.Children))

	nodeUuid := nodeCollection.Parent.Uuid

	// TODO: Decide which usermeta namespace to use.
	// Might convert a3m-progress to preservation-status but want to allow compatibility with existing tags
	preservationProgressTag := "usermeta-a3m-progress"

	// Ensure the tag is updated on failure
	defer func() {
		if err != nil {
			updateErr := cellsClient.UpdateTag(ctx, nodeUuid, preservationProgressTag, "Failed")
			if updateErr != nil {
				fmt.Printf("Error updating Cells tag on failure: %v\n", updateErr)
			}
		}
	}()

	///////////////////////////////////////////////////////////////////
	//						Start Processing						 //
	///////////////////////////////////////////////////////////////////

	err = cellsClient.UpdateTag(ctx, nodeUuid, preservationProgressTag, "Starting...")
	if err != nil {
		fmt.Printf("Error updating Cells tag: %v\n", err)
		return
	}

	// Create unique processing directory
	processingDir, err := utils.MakeUniqueDir(ctx, cfg.ProcessingBaseDir)
	if err != nil {
		fmt.Printf("Failed to create processing directory: %v\n", err)
		return
	}
	fmt.Printf("Created Processing directory \t%s\n", processingDir)

	// Clean up the processing directory
	defer func() {
		if deleteProcessingDir && processingDir != "" {
			if err := os.RemoveAll(processingDir); err != nil {
				fmt.Printf("Error deleting processing directory: %v\n", err)
			}
			fmt.Printf("Deleted processing directory: \t%s\n", processingDir)
		}
	}()

	///////////////////////////////////////////////////////////////////
	//					 Download Cells Package						 //
	///////////////////////////////////////////////////////////////////

	err = cellsClient.UpdateTag(ctx, nodeUuid, preservationProgressTag, "🌐 Downloading...")
	if err != nil {
		fmt.Printf("Error updating Cells tag: %v\n", err)
		return
	}

	// Create the download directory
	cellsDownloadDir := filepath.Join(processingDir, "cells_download")
	if err = os.Mkdir(cellsDownloadDir, 0755); err != nil {
		fmt.Printf("Failed to create download directory: %v\n", err)
		return
	}

	// Download the package
	downloadedPackagePath, err := cellsClient.DownloadNode(ctx, cellsPackagePath, cellsDownloadDir)
	if err != nil {
		fmt.Printf("Error downloading package: %v\n", err)
		return
	}
	fmt.Printf("Downloaded package to \t\t%s\n", utils.RelPath(cfg.ProcessingBaseDir, downloadedPackagePath))

	///////////////////////////////////////////////////////////////////
	//						 Preprocessing							 //
	///////////////////////////////////////////////////////////////////

	err = cellsClient.UpdateTag(ctx, nodeUuid, preservationProgressTag, "🗂️ Preprocessing...")
	if err != nil {
		fmt.Printf("Error updating Cells tag: %v\n", err)
		return
	}

	// Create the a3m transfer directory
	a3mTransferDir := filepath.Join(processingDir, "a3m_transfer")
	if err = os.Mkdir(a3mTransferDir, 0755); err != nil {
		fmt.Printf("Failed to create a3m transfer directory: %v\n", err)
		return
	}

	// Preprocess package
	fmt.Printf("Preprocessing package \t\t%s\n", utils.RelPath(cfg.ProcessingBaseDir, downloadedPackagePath))
	transferPath, err := processor.PreprocessPackage(ctx, downloadedPackagePath, a3mTransferDir, nodeCollection, cellsClient.UserData)
	if err != nil {
		fmt.Printf("Error preprocessing package: %v\n", err)
		return
	}

	///////////////////////////////////////////////////////////////////
	//						 Submit to A3M							 //
	///////////////////////////////////////////////////////////////////

	err = cellsClient.UpdateTag(ctx, nodeUuid, preservationProgressTag, "📦 Processing...")
	if err != nil {
		fmt.Printf("Error updating Cells tag: %v\n", err)
		return
	}

	// Execute a3m transfer
	fmt.Printf("Submitting A3M Transfer \t%s\n", utils.RelPath(cfg.ProcessingBaseDir, transferPath))
	a3mClient, err := a3mclient.NewClient(cfg.A3mAddress)
	if err != nil {
		fmt.Printf("Error creating a3m client: %v\n", err)
		return
	}
	defer a3mClient.Close()
	ctx, cancel = context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	aipUuid, _, err := a3mClient.SubmitPackage(ctx, transferPath, filepath.Base(transferPath), nil)
	if err != nil {
		fmt.Printf("Submission failed: %v\n", err)
		return
	}

	// Find the a3m AIP
	a3mAipPath, err := getA3mAipPath(cfg.A3mCompletedDir, filepath.Base(transferPath), aipUuid)
	if err != nil {
		fmt.Printf("Error getting A3M AIP path: %v\n", err)
		return
	}
	a3mClient.Close()
	fmt.Printf("A3M AIP generated at \t\t%s\n", a3mAipPath)
	defer func() {
		if deleteProcessingDir && a3mAipPath != "" {
			if err := os.Remove(a3mAipPath); err != nil {
				fmt.Printf("Error deleting A3M AIP: %v\n", err)
			}
			fmt.Printf("Deleted A3M AIP: \t\t%s\n", a3mAipPath)
		}
	}()

	///////////////////////////////////////////////////////////////////
	//						 Postprocessing							 //
	///////////////////////////////////////////////////////////////////

	err = cellsClient.UpdateTag(ctx, nodeUuid, preservationProgressTag, "🗃️ Extracting...")
	if err != nil {
		fmt.Printf("Error updating Cells tag: %v\n", err)
		return
	}

	// Create the aip directory
	processingAipDir := filepath.Join(processingDir, "aip")
	if err = os.Mkdir(processingAipDir, 0755); err != nil {
		fmt.Printf("Failed to create a3m transfer directory: %v\n", err)
		return
	}

	// Extract AIP
	aipPath, err := utils.ExtractArchive(a3mAipPath, processingAipDir)
	if err != nil {
		fmt.Printf("Error extracting AIP: %v\n", err)
		return
	}
	fmt.Printf("Extracted AIP to \t\t%s\n", utils.RelPath(cfg.ProcessingBaseDir, aipPath))

	// Compress AIP
	if pcfg.CompressAip {

		err = cellsClient.UpdateTag(ctx, nodeUuid, preservationProgressTag, "🗃️ Compressing...")
		if err != nil {
			fmt.Printf("Error updating Cells tag: %v\n", err)
			return
		}

		archiveAipPath := filepath.Join(processingAipDir, fmt.Sprintf("%s.zip", filepath.Base(aipPath)))
		err = utils.CompressToZip(aipPath, archiveAipPath)
		if err != nil {
			fmt.Printf("Error compressing AIP: %v\n", err)
			return
		}
		fmt.Printf("Compressed AIP to \t\t\t%s\n", utils.RelPath(cfg.ProcessingBaseDir, archiveAipPath))
		aipPath = archiveAipPath
	}

	///////////////////////////////////////////////////////////////////
	//						 Uploading AIP							 //
	///////////////////////////////////////////////////////////////////

	// Upload AIP
	err = cellsClient.UpdateTag(ctx, nodeUuid, preservationProgressTag, "🌐 Uploading...")
	if err != nil {
		fmt.Printf("Error updating Cells tag: %v\n", err)
		return
	}
	cellsUploadPath, err := cellsClient.UploadNode(ctx, aipPath, cfg.CellsArchiveWorkspace)
	if err != nil {
		fmt.Printf("Error uploading AIP: %v\n", err)
		return
	}
	fmt.Printf("Uploaded AIP to \t\t%s\n", cellsUploadPath)

	err = cellsClient.UpdateTag(ctx, nodeUuid, preservationProgressTag, "🔒 Preserved")
	if err != nil {
		fmt.Printf("Error updating Cells tag: %v\n", err)
		return
	}
}

// Construct the path of the A3M Generated AIP and ensures it exists
// TODO: Consider AIP Compression Algorithm
func getA3mAipPath(a3mCompletedDir string, packageName string, packageUUID string) (string, error) {
	sanitisedPackageName := strings.ReplaceAll(packageName, " ", "")
	expectedAIPPath := filepath.Join(a3mCompletedDir, sanitisedPackageName+"-"+packageUUID+".7z")
	if _, err := os.Stat(expectedAIPPath); os.IsNotExist(err) {
		fmt.Printf("A3M AIP not found: %v\n", err)
		return "", err
	}
	return expectedAIPPath, nil
}
