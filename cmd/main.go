package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	transferservice "github.com/penwern/preservation-go/gen/go/a3m/api/transferservice/v1beta1"
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

func LoadConfigDir(dir string) (string, error) {
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

func LoadConfigExecutable(path string) (string, error) {
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
func LoadConfig() (*Config, error) {
	processingDir, ok := os.LookupEnv("PROCESSING_BASE_DIR")
	if !ok || processingDir == "" {
		return nil, fmt.Errorf("PROCESSING_BASE_DIR environment variable is not set")
	}
	absProcessingBaseDir, err := LoadConfigDir(processingDir)
	if err != nil {
		return nil, err
	}

	a3mCompletedDir, ok := os.LookupEnv("A3M_COMPLETED_DIR")
	if !ok || a3mCompletedDir == "" {
		fmt.Printf("A3M_COMPLETED_DIR environment variable is not set. Defaulting to /home/a3m/.local/share/a3m/share/completed\n")
		a3mCompletedDir = "/home/a3m/.local/share/a3m/share/completed"
	}
	absA3mCompletedDir, err := LoadConfigDir(a3mCompletedDir)
	if err != nil {
		return nil, err
	}

	cecPath, ok := os.LookupEnv("CELLS_CEC_PATH")
	if !ok || cecPath == "" {
		fmt.Printf("CELLS_CEC_PATH environment variable is not set. Defaulting to /usr/bin/cec\n")
		cecPath = "/usr/bin/cec"
	}
	absCecPath, err := LoadConfigExecutable(cecPath)
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
	const fakeDownload = false
	const deleteProcessingDir = true

	// Inputs
	const cellsPackagePath = "personal-files/test_dir"
	const cellsUserName = "admin"
	const cellsAdminToken = "TOKEN"

	// Load the .env file if not in production
	if os.Getenv("GO_ENV") != "production" {
		if err := godotenv.Load(); err != nil {
			log.Printf("No .env file found or failed to load: %v", err)
		}
	}

	// Load and validate environment configuration
	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// Load and validate the processing configuration
	// TODO: Populate from config args
	pcfg := ProcessingConfig{
		CompressAip: false,
	}

	// Create Cells client for user
	cellsClient, err := cells.NewClient(cfg.CellsCecPath, cfg.CellsAddress, cellsUserName, cellsAdminToken)
	if err != nil {
		log.Fatalf("Error creating Cells client: %v", err)
	}

	nodeCollection, err := cellsClient.GetNodeCollection(cellsPackagePath)
	if err != nil {
		log.Fatalf("Error getting node collection: %v", err)
	}
	fmt.Printf("Number of children: %d\n", len(nodeCollection.Children))

	nodeUuid := nodeCollection.Parent.Uuid

	// Create unique processing directory
	processingDir := makeNewProcessingDirectory(cfg.ProcessingBaseDir)
	fmt.Printf("Created Processing directory \t%s\n", processingDir)

	// Create the download directory
	cellsDownloadDir := filepath.Join(processingDir, "cells_download")
	if err = os.Mkdir(cellsDownloadDir, 0755); err != nil {
		log.Fatalf("Failed to create download directory: %v", err)
	}

	err = cellsClient.UpdateTag(nodeUuid, "usermeta-a3m-progress", "Processing...")
	if err != nil {
		log.Fatalf("Error updating Cells tag: %v", err)
	}

	// Download the package
	var downloadedPackagePath string
	if fakeDownload {
		fmt.Println("Mimicing downloading the package...")
		// Fake file
		dummyFilePath := filepath.Join(cellsDownloadDir, "dummy_file.txt")
		file, err := os.Create(dummyFilePath)
		if err != nil {
			log.Fatalf("Failed to create dummy file: %v", err)
		}
		defer file.Close()
		// Fake file content
		dummyContent := "This is a dummy file representing downloaded content."
		_, err = file.WriteString(dummyContent)
		if err != nil {
			log.Fatalf("Failed to write to dummy file: %v", err)
		}
		downloadedPackagePath = dummyFilePath
	} else {
		downloadedPackagePath, err = cellsClient.DownloadNode(cellsPackagePath, cellsDownloadDir)
		if err != nil {
			log.Fatalf("Error downloading package: %v", err)
		}
	}
	fmt.Printf("Downloaded package to \t\t%s\n", utils.RelPath(cfg.ProcessingBaseDir, downloadedPackagePath))

	// Create the a3m transfer directory
	a3mTransferDir := filepath.Join(processingDir, "a3m_transfer")
	if err = os.Mkdir(a3mTransferDir, 0755); err != nil {
		log.Fatalf("Failed to create a3m transfer directory: %v", err)
	}

	err = cellsClient.UpdateTag(nodeUuid, "usermeta-a3m-progress", "Preparing...")
	if err != nil {
		log.Fatalf("Error updating Cells tag: %v", err)
	}

	// Preprocess package
	fmt.Printf("Preprocessing package \t\t%s\n", utils.RelPath(cfg.ProcessingBaseDir, downloadedPackagePath))
	transferPath, err := processor.PreprocessPackage(downloadedPackagePath, a3mTransferDir)
	if err != nil {
		log.Fatalf("Error preprocessing package: %v", err)
	}

	err = cellsClient.UpdateTag(nodeUuid, "usermeta-a3m-progress", "Submitting...")
	if err != nil {
		log.Fatalf("Error updating Cells tag: %v", err)
	}

	// Execute a3m transfer
	fmt.Printf("Submitting A3M Transfer \t%s\n", utils.RelPath(cfg.ProcessingBaseDir, transferPath))
	a3mClient, err := a3mclient.NewClient(cfg.A3mAddress)
	if err != nil {
		log.Fatalf("Error creating a3m client: %v", err)
	}
	defer a3mClient.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	aipUuid, _, err := a3mClient.SubmitPackage(ctx, transferPath, filepath.Base(transferPath), nil)
	if err != nil {
		log.Fatalf("Submission failed: %v", err)
	}
	a3mAipPath := getA3mAipPath(cfg.A3mCompletedDir, filepath.Base(transferPath), aipUuid)
	fmt.Printf("A3M AIP generated at \t\t%s\n", utils.RelPath(cfg.A3mCompletedDir, a3mAipPath))

	// Create the aip directory
	processingAipDir := filepath.Join(processingDir, "aip")
	if err = os.Mkdir(processingAipDir, 0755); err != nil {
		log.Fatalf("Failed to create a3m transfer directory: %v", err)
	}

	err = cellsClient.UpdateTag(nodeUuid, "usermeta-a3m-progress", "Extracting...")
	if err != nil {
		log.Fatalf("Error updating Cells tag: %v", err)
	}

	aipPath, err := utils.ExtractArchive(a3mAipPath, processingAipDir)
	if err != nil {
		log.Fatalf("Error extracting AIP: %v", err)
	}
	fmt.Printf("Extracted AIP to \t\t%s\n", utils.RelPath(cfg.ProcessingBaseDir, aipPath))

	// Compress AIP
	if pcfg.CompressAip {

		err = cellsClient.UpdateTag(nodeUuid, "usermeta-a3m-progress", "Compressing...")
		if err != nil {
			log.Fatalf("Error updating Cells tag: %v", err)
		}

		archiveAipPath := filepath.Join(processingAipDir, fmt.Sprintf("%s.zip", filepath.Base(aipPath)))
		err = utils.CompressToZip(aipPath, archiveAipPath)
		if err != nil {
			log.Fatalf("Error compressing AIP: %v", err)
		}
		fmt.Printf("Compressed AIP to \t\t\t%s\n", utils.RelPath(cfg.ProcessingBaseDir, archiveAipPath))
		aipPath = archiveAipPath
	}

	// Upload AIP
	err = cellsClient.UpdateTag(nodeUuid, "usermeta-a3m-progress", "Uploading...")
	if err != nil {
		log.Fatalf("Error updating Cells tag: %v", err)
	}
	cellsUploadPath, err := cellsClient.UploadNode(aipPath, cfg.CellsArchiveWorkspace)
	if err != nil {
		log.Fatalf("Error uploading AIP: %v", err)
	}
	fmt.Printf("Uploaded AIP to \t\t%s\n", cellsUploadPath)

	err = cellsClient.UpdateTag(nodeUuid, "usermeta-a3m-progress", "🔒 Preserved")
	if err != nil {
		log.Fatalf("Error updating Cells tag: %v", err)
	}

	// Delete the processing directory
	if deleteProcessingDir {
		err = os.RemoveAll(processingDir)
		if err != nil {
			log.Fatalf("Error deleting processing directory: %v", err)
		}
	}
}

// Generates a new directory name that doesn't exist in the baseDir
func makeNewProcessingDirectory(baseDir string) string {
	var newDirPath string
	for {
		newDirName := uuid.New().String()
		newDirPath = filepath.Join(baseDir, newDirName)
		// Ensure the directory doesn't already exist
		if _, err := os.Stat(newDirPath); os.IsNotExist(err) {
			break
		}
	}
	err := os.Mkdir(newDirPath, 0755)
	if err != nil {
		log.Fatalf("Failed to create directory %q: %v", newDirPath, err)
	}
	return newDirPath
}

// Construct the path of the A3M Generated AIP and ensures it exists
func getA3mAipPath(a3mCompletedDir string, packageName string, packageUUID string) string {
	sanitiasedPackageName := strings.ReplaceAll(packageName, " ", "")
	expectedAIPPath := filepath.Join(a3mCompletedDir, sanitiasedPackageName+"-"+packageUUID+".7z")
	if _, err := os.Stat(expectedAIPPath); os.IsNotExist(err) {
		log.Fatalf("A3M AIP not found: %v", err)
	}
	return expectedAIPPath
}
