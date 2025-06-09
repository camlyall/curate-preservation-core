package preservation

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	transferservice "github.com/penwern/curate-preservation-core/common/proto/a3m/gen/go/a3m/api/transferservice/v1beta1"
	"github.com/penwern/curate-preservation-core/internal/a3mclient"
	"github.com/penwern/curate-preservation-core/internal/atom"
	"github.com/penwern/curate-preservation-core/internal/cells"
	"github.com/penwern/curate-preservation-core/internal/processor"
	"github.com/penwern/curate-preservation-core/pkg/config"
	"github.com/penwern/curate-preservation-core/pkg/logger"
	"github.com/penwern/curate-preservation-core/pkg/utils"
	"github.com/pydio/cells-sdk-go/v4/models"
)

var (
	preservationTagNamespace     = "usermeta-preservation-status"
	dipTagNamespace              = "usermeta-dip-status"
	atomSlugTagNamespace         = "usermeta-atom-slug"
	preservationTagStarting      = "🟢 Starting..."
	preservationTagDownloading   = "🌐 Downloading..."
	preservationTagPreprocessing = "🗂️ Preprocessing..."
	preservationTagPackaging     = "📦 Packaging..."
	preservationTagExtracting    = "🗃️ Extracting..."
	preservationTagCompressing   = "🗃️ Compressing..."
	preservationTagWaiting       = "⏳ Waiting..."
	preservationTagUploading     = "🌐 Uploading..."
	preservationTagCompleted     = "🔒 Preserved"
	preservationTagFailed        = "❌ Failed"
	preservationTagDipFailed     = "❌ DIP Failed"
	dipTagWaiting                = "⏳ Waiting..."
	dipTagStarting               = preservationTagStarting
	dipTagMigrating              = "📨 Migrating..."
	dipTagDepositing             = "🌐 Depositing..."
	dipTagCompleted              = "🖼️ Deposited"
	dipTagFailed                 = preservationTagFailed
)

// TagUpdaters holds functions to update various tag namespaces
type TagUpdaters struct {
	Preservation func(context.Context, string) error
	Dip          func(context.Context, string) error
	AtomSlug     func(context.Context, string) error
}

// Preserver is the service for the preservation process
type Preserver struct {
	a3mClient   a3mclient.ClientInterface
	cellsClient cells.ClientInterface
	envConfig   *config.Config
}

// NewPreserver creates a new preservation service.
// Initializes the Cells and A3M clients.
// Panics if the clients cannot be created.
func NewPreserver(ctx context.Context, cfg *config.Config) *Preserver {
	a3mClient, err := a3mclient.NewClient(cfg.A3M.Address)
	if err != nil {
		panic(fmt.Errorf("a3m client error: %w", err))
	}
	return NewPreserverWithA3MClient(ctx, cfg, a3mClient)
}

func NewPreserverWithA3MClient(ctx context.Context, cfg *config.Config, a3mClient a3mclient.ClientInterface) *Preserver {
	cellsClient, err := cells.NewClient(ctx, cfg.Cells.CecPath, cfg.Cells.Address, cfg.Cells.AdminToken)
	if err != nil {
		logger.Fatal("cells client error: %v", err)
	}
	return &Preserver{
		a3mClient:   a3mClient,
		cellsClient: cellsClient,
		envConfig:   cfg,
	}
}

func (p *Preserver) Close() {
	logger.Debug("Closing Clients")
	p.cellsClient.Close()
	p.a3mClient.Close()
}

func (p *Preserver) Run(ctx context.Context, pcfg *config.PreservationConfig, userClient cells.UserClient, cellsPackagePath string, cleanUp, pathResolved bool) error {

	var (
		err            error
		nodeCollection *models.RestNodesCollection
		tagUpdaters    *TagUpdaters
	)

	///////////////////////////////////////////////////////////////////
	//						Pre-requisites							 //
	///////////////////////////////////////////////////////////////////

	// Unresolve resolved paths
	// TODO: We imeditately resolve it again in gatherNodeEnvironment...
	if pathResolved {
		cellsPackagePath, err = p.cellsClient.UnresolveCellsPath(userClient, cellsPackagePath)
		if err != nil {
			return fmt.Errorf("error unresolving cells path: %w", err)
		}
		logger.Info("Unresolved Cells Path: %s", cellsPackagePath)
	}

	// Gather the node environment
	nodeCollection, tagUpdaters, err = p.gatherNodeEnvironment(ctx, userClient, cellsPackagePath)
	if err != nil {
		return fmt.Errorf("error gathering node environment: %w", err)
	}
	// Ensure the preservation tags are updated on failure
	processingDip := false
	defer func() {
		if err != nil {
			if !processingDip {
				// Update the preservation tag on failure
				errMsg := fmt.Sprintf("%s: %s", preservationTagFailed, utils.TruncateError(err.Error(), 100))
				if updateErr := tagUpdaters.Preservation(ctx, errMsg); updateErr != nil {
					logger.Error("error updating Preservation tag on failure: %v", updateErr)
				}
			} else {
				// Update the atom tag on failure
				dipErrMsg := fmt.Sprintf("%s: %s", dipTagFailed, utils.TruncateError(err.Error(), 100))
				if updateErr := tagUpdaters.Dip(ctx, dipErrMsg); updateErr != nil {
					logger.Error("error updating AtoM tag on failure: %v", updateErr)
				}
				if updateErr := tagUpdaters.Preservation(ctx, preservationTagDipFailed); updateErr != nil {
					logger.Error("error updating Preservation tag on failure: %v", updateErr)
				}
			}
		}
	}()

	atomSlug := nodeCollection.Parent.MetaStore[atomSlugTagNamespace]
	// If no slug is present on the new, use the slug from the args
	// TODO: Override the slug from the config?
	if atomSlug == "" || atomSlug == "\"\"" {
		atomSlug = pcfg.AtomConfig.Slug
	}

	logger.Debug("Atom Slug: %q", atomSlug)

	///////////////////////////////////////////////////////////////////
	//						Start Processing						 //
	///////////////////////////////////////////////////////////////////

	// Tag Package: Starting
	if err = tagUpdaters.Preservation(ctx, preservationTagStarting); err != nil {
		return fmt.Errorf("error updating Preservation tag: %w", err)
	}

	// If the DIP tag exists, update it to "Waiting..." if a slug exists else clear it.
	if atomSlug == "" || atomSlug == "\"\"" {
		if nodeCollection.Parent.MetaStore[dipTagNamespace] != "" {
			if err = tagUpdaters.Dip(ctx, ""); err != nil {
				return fmt.Errorf("error updating AtoM tag: %w", err)
			}
		}
	} else {
		if err = tagUpdaters.Dip(ctx, dipTagWaiting); err != nil {
			return fmt.Errorf("error updating AtoM tag: %w", err)
		}
	}

	// Create unique processing directory
	var processingDir string
	processingDir, err = utils.MakeUniqueDir(ctx, p.envConfig.ProcessingBaseDir)
	if err != nil {
		return fmt.Errorf("failed to create processing directory: %w", err)
	}
	logger.Info("Created processing dir: %s", processingDir)
	// Clean up the processing directory
	defer func() {
		if cleanUp && processingDir != "" {
			logger.Info("Cleaning up.")
			if removeErr := os.RemoveAll(processingDir); removeErr != nil {
				logger.Error("Error deleting processing directory: %v", removeErr)
			}
			logger.Debug("Deleted processing dir: %s", processingDir)
		}
	}()

	///////////////////////////////////////////////////////////////////
	//					 Download Cells Package						 //
	///////////////////////////////////////////////////////////////////

	// Tag Package: Downloading
	if err = tagUpdaters.Preservation(ctx, preservationTagDownloading); err != nil {
		return fmt.Errorf("error updating Preservation tag: %w", err)
	}
	logger.Info("Downloading package: %s", cellsPackagePath)
	var downloadedPath string
	downloadedPath, err = p.downloadPackage(ctx, userClient, processingDir, cellsPackagePath)
	if err != nil {
		return fmt.Errorf("error downloading package: %v", err)
	}

	///////////////////////////////////////////////////////////////////
	//						 Preprocessing							 //
	///////////////////////////////////////////////////////////////////

	// Tag Package: Preprocessing
	if err = tagUpdaters.Preservation(ctx, preservationTagPreprocessing); err != nil {
		return fmt.Errorf("error updating Preservation tag: %w", err)
	}
	// Preprocess package. Don't use retry as we move/extract the package in the first step
	logger.Info("Preprocessing package: %s", cellsPackagePath)
	var transferPath string
	transferPath, err = p.preprocessPackage(ctx, processingDir, downloadedPath, nodeCollection, userClient.UserData)
	if err != nil {
		return fmt.Errorf("error preprocessing package: %w", err)
	}

	///////////////////////////////////////////////////////////////////
	//						 Submit to A3M							 //
	///////////////////////////////////////////////////////////////////

	// Tag Package: Preserving
	if err = tagUpdaters.Preservation(ctx, preservationTagPackaging); err != nil {
		return fmt.Errorf("error updating Preservation tag: %w", err)
	}

	a3mStartTime := time.Now()
	// Submit package to A3M
	logger.Info("Submitting package to A3M: %s", utils.RelPath(p.envConfig.ProcessingBaseDir, transferPath))
	transferName := transferNameFromPath(transferPath)
	var aipUuid string
	aipUuid, err = p.submitPackage(ctx, transferPath, transferName, pcfg.A3mConfig)
	if err != nil {
		return fmt.Errorf("failed to submit package: %w (path: %s)", err, transferPath)
	}
	var a3mAipPath string
	a3mAipPath, err = getA3mAipPath(p.envConfig.A3M.CompletedDir, transferName, aipUuid)
	if err != nil {
		return fmt.Errorf("error getting A3M AIP path: %v", err)
	}
	a3mFinishTime := time.Since(a3mStartTime).Seconds()
	defer func() {
		// Clean up the A3M AIP
		if cleanUp && a3mAipPath != "" {
			if removeErr := os.RemoveAll(a3mAipPath); removeErr != nil {
				logger.Error("Error deleting A3M AIP: %v", removeErr)
			} else {
				logger.Debug("Deleted A3M AIP: %s", a3mAipPath)
			}
		}
		logger.Debug("A3M Execution time: %vs", a3mFinishTime)
	}()
	logger.Info("Generated A3M AIP: %s", utils.RelPath(p.envConfig.ProcessingBaseDir, a3mAipPath))

	///////////////////////////////////////////////////////////////////
	//						 Postprocessing							 //
	///////////////////////////////////////////////////////////////////

	// Tag Package: Extracting
	if err = tagUpdaters.Preservation(ctx, preservationTagExtracting); err != nil {
		return fmt.Errorf("error updating Preservation tag: %w", err)
	}
	// Create AIP Directory
	processingAipDir := filepath.Join(processingDir, "aip")
	if err = utils.CreateDir(processingAipDir); err != nil {
		return fmt.Errorf("failed to create AIP directory: %w", err)
	}
	// Post-process package
	logger.Info("Postprocessing A3M AIP: %s", utils.RelPath(p.envConfig.ProcessingBaseDir, a3mAipPath))
	var aipPath string
	aipPath, err = p.postprocessPackage(ctx, processingAipDir, a3mAipPath)
	if err != nil {
		return fmt.Errorf("error postprocessing package: %w", err)
	}
	logger.Info("Postprocessed AIP: %s", utils.RelPath(p.envConfig.ProcessingBaseDir, aipPath))
	if pcfg.CompressAip {
		// Tag Package: Compressing
		if err = tagUpdaters.Preservation(ctx, preservationTagCompressing); err != nil {
			return fmt.Errorf("error updating Preservation tag: %w", err)
		}
		// Compress AIP
		logger.Info("Compressing AIP: %s", utils.RelPath(p.envConfig.ProcessingBaseDir, aipPath))
		aipPath, err = p.compressPackage(ctx, processingAipDir, aipPath)
		if err != nil {
			return fmt.Errorf("error compressing AIP: %w", err)
		}
		logger.Info("Compressed AIP %s", utils.RelPath(p.envConfig.ProcessingBaseDir, aipPath))
	}

	///////////////////////////////////////////////////////////////////
	//						 DIP Submission							 //
	///////////////////////////////////////////////////////////////////

	producingDip := atomSlug != "" && atomSlug != "\"\""

	if !producingDip {
		logger.Debug("No AtoM slug found. Skipping DIP submission.")
	} else {
		processingDip = true

		// Tag Package: Starting DIP Processing
		if err = tagUpdaters.Dip(ctx, dipTagStarting); err != nil {
			return fmt.Errorf("error updating AtoM tag: %w", err)
		}

		// Tag Package: Waiting
		if err = tagUpdaters.Preservation(ctx, preservationTagWaiting); err != nil {
			return fmt.Errorf("error updating Preservation tag: %w", err)
		}

		// Create AtoM Client
		var atomClient *atom.Client
		atomClient, err = atom.NewClient(pcfg.AtomConfig)
		if err != nil {
			return fmt.Errorf("error creating AtoM client: %w", err)
		}
		defer atomClient.Close()

		// Ensure DIP exists where expected
		logger.Debug("Searching for DIP: %s", aipUuid)
		var a3mDipPath string
		a3mDipPath, err = getA3mDipPath(p.envConfig.A3M.DipsDir, aipUuid)
		if err != nil {
			return fmt.Errorf("error getting A3M DIP path: %v", err)
		}
		defer func() {
			// Clean up the A3M AIP
			if cleanUp && a3mDipPath != "" {
				if removeErr := os.RemoveAll(a3mDipPath); removeErr != nil {
					logger.Error("Error deleting A3M DIP: %v", removeErr)
				} else {
					logger.Debug("Deleted A3M DIP: %s", a3mDipPath)
				}
			}
		}()
		logger.Debug("Found A3M DIP: %s", a3mDipPath)

		logger.Info("Migrating DIP: %s", utils.RelPath(p.envConfig.ProcessingBaseDir, a3mDipPath))

		// Tag Package: Migrating to AtoM Server
		if err = tagUpdaters.Dip(ctx, dipTagMigrating); err != nil {
			return fmt.Errorf("error updating AtoM tag: %w", err)
		}

		// Migrate DIP to AtoM server
		if err = atomClient.MigratePackage(ctx, a3mDipPath); err != nil {
			return fmt.Errorf("error migrating DIP to AtoM: %w", err)
		}

		// Tag Package: Depositing
		if err = tagUpdaters.Dip(ctx, dipTagDepositing); err != nil {
			return fmt.Errorf("error updating AtoM tag: %w", err)
		}

		// Deposit DIP to AtoM
		if err = atomClient.DepositDip(ctx, atomSlug, filepath.Base(a3mDipPath)); err != nil {
			return fmt.Errorf("error depositing DIP to AtoM: %w", err)
		}

		// Tag Package: Preserved
		if err = tagUpdaters.Preservation(ctx, dipTagCompleted); err != nil {
			return fmt.Errorf("error updating Preservation tag: %w", err)
		}

		processingDip = false
	}

	///////////////////////////////////////////////////////////////////
	//						 Uploading AIP							 //
	///////////////////////////////////////////////////////////////////

	// Tag Package: Uploading
	if err = tagUpdaters.Preservation(ctx, preservationTagUploading); err != nil {
		return fmt.Errorf("error updating Preservation tag: %w", err)
	}
	// Upload Node
	logger.Info("Uploading AIP: %s", utils.RelPath(p.envConfig.ProcessingBaseDir, aipPath))
	var cellsUploadPath string
	cellsUploadPath, err = p.uploadPackage(ctx, userClient, aipPath)
	if err != nil {
		return fmt.Errorf("error uploading AIP: %w", err)
	}
	logger.Info("Uploaded AIP %s", cellsUploadPath)

	// Verify the AIP is located in the upload destination
	var resolvedUploadPath string
	resolvedUploadPath, err = p.cellsClient.ResolveCellsPath(userClient, cellsUploadPath)
	if err != nil {
		return fmt.Errorf("error resolving upload path: %w", err)
	}
	_, err = p.getNodeStats(ctx, resolvedUploadPath)
	if err != nil {
		return fmt.Errorf("error getting node stats: %w", err)
	}
	logger.Info("Verified AIP in Cells: %s", resolvedUploadPath)

	if producingDip {
		// Tag Atom-Slug with Atom Slug
		if err = tagUpdaters.AtomSlug(ctx, atomSlug); err != nil {
			return fmt.Errorf("error updating Atom Slug tag: %w", err)
		}
	}

	// Tag Package: Preserved
	if err = tagUpdaters.Preservation(ctx, preservationTagCompleted); err != nil {
		return fmt.Errorf("error updating Preservation tag: %w", err)
	}

	// Stops preservation tag from being updated on failure after this point
	// preservationComplete = true
	logger.Info("Preservation successful: %s", filepath.Base(aipPath))

	return nil
}

func (p *Preserver) NewUserClient(ctx context.Context, username string) (cells.UserClient, error) {
	return p.cellsClient.NewUserClient(ctx, username)
}

// createTagUpdater creates a tag update function for a given namespace
func (p *Preserver) createTagUpdater(userClient cells.UserClient, parentNodeUuid, namespace string) func(context.Context, string) error {
	return func(ctx context.Context, status string) error {
		return utils.Retry(3, 2*time.Second, func() error {
			logger.Debug("Tagging: {tag: %s, status: %s, node: %s}", namespace, status, parentNodeUuid)
			return p.cellsClient.UpdateTag(ctx, userClient, parentNodeUuid, namespace, status)
		}, utils.IsTransientError)
	}
}

// Gather the node environment. Returns the node collection and tag updaters.
func (p *Preserver) gatherNodeEnvironment(ctx context.Context, userClient cells.UserClient, cellsPackagePath string) (*models.RestNodesCollection, *TagUpdaters, error) {
	// Get the resolved cells path, parsing cells template path if necessary
	resolvedCellsPackagePath, err := p.cellsClient.ResolveCellsPath(userClient, cellsPackagePath)
	if err != nil {
		return nil, nil, fmt.Errorf("error resolving cells path: %w", err)
	}

	// Collect the package node data
	nodeCollection, err := p.cellsClient.GetNodeCollection(ctx, resolvedCellsPackagePath)
	if err != nil {
		return nil, nil, fmt.Errorf("error getting node collection: %w", err)
	}

	// Add defensive checks for nil values
	if nodeCollection == nil {
		return nil, nil, fmt.Errorf("node collection is nil for path: %s", cellsPackagePath)
	}
	if nodeCollection.Parent == nil {
		return nil, nil, fmt.Errorf("parent node is nil for path: %s", cellsPackagePath)
	}
	if nodeCollection.Parent.MetaStore == nil {
		nodeCollection.Parent.MetaStore = make(map[string]string)
	}

	// Set the parent node uuid
	parentNodeUuid := nodeCollection.Parent.UUID

	// Check for old tag namespaces
	if nodeCollection.Parent.MetaStore["usermeta-a3m-progress"] != "" {
		logger.Warn("Using old usermeta-a3m-progress preservation tag - this will be removed in the future")
		preservationTagNamespace = "usermeta-a3m-progress"
	}

	if nodeCollection.Parent.MetaStore["usermeta-dip-progress"] != "" {
		logger.Warn("Using old usermeta-dip-progress dip tag - this will be removed in the future")
		dipTagNamespace = "usermeta-dip-progress"
	}

	// Create tag updaters
	tagUpdaters := &TagUpdaters{
		Preservation: p.createTagUpdater(userClient, parentNodeUuid, preservationTagNamespace),
		Dip:          p.createTagUpdater(userClient, parentNodeUuid, dipTagNamespace),
		AtomSlug:     p.createTagUpdater(userClient, parentNodeUuid, atomSlugTagNamespace),
	}

	return nodeCollection, tagUpdaters, nil
}

// Get the node stats. Uses the Cells Client.
func (p *Preserver) getNodeStats(ctx context.Context, cellsPackagePath string) (*models.TreeReadNodeResponse, error) {
	logger.Debug("Getting node stats: %s", cellsPackagePath)
	nodeStats, err := p.cellsClient.GetNodeStats(ctx, cellsPackagePath)
	if err != nil {
		return nil, fmt.Errorf("error getting node stats: %w", err)
	}
	if nodeStats == nil {
		return nil, fmt.Errorf("node stats is nil")
	}
	if nodeStats.Node == nil {
		return nil, fmt.Errorf("node stats node is nil")
	}
	return nodeStats, nil
}

// Download package. Uses the Cells Client. Retries on transient errors.
func (p *Preserver) downloadPackage(ctx context.Context, userClient cells.UserClient, processingDir, packagePath string) (string, error) {
	downloadDir := filepath.Join(processingDir, "cells_download")
	if err := utils.CreateDir(downloadDir); err != nil {
		return "", fmt.Errorf("failed to create download directory: %w", err)
	}
	// TODO: I don't think retry will work here because the download is executed using CEC binary, so doesn't produce a transient error.
	var downloadedPath string
	err := utils.Retry(3, 2*time.Second, func() error {
		var downloadErr error
		downloadedPath, downloadErr = p.cellsClient.DownloadNode(ctx, userClient, packagePath, downloadDir)
		return downloadErr
	}, utils.IsTransientError)
	if err != nil {
		return "", fmt.Errorf("download error: %w", err)
	}
	return downloadedPath, nil
}

// Preprocess package. Uses preproces module. Constructs the a3m tranfer package. Writes DC and Premis Metadata.
func (p *Preserver) preprocessPackage(ctx context.Context, processingDir, packagePath string, nodeCollection *models.RestNodesCollection, userData *models.IdmUser) (string, error) {
	// Create the a3m transfer directory
	a3mTransferDir := filepath.Join(processingDir, "a3m_transfer")
	if err := utils.CreateDir(a3mTransferDir); err != nil {
		return "", fmt.Errorf("failed to create a3m transfer directory: %w", err)
	}
	// Preprocess package
	transferPath, err := processor.PreprocessPackage(ctx, packagePath, a3mTransferDir, nodeCollection, userData, p.envConfig.Premis.Organization)
	if err != nil {
		return "", fmt.Errorf("error preprocessing package: %w", err)
	}
	return transferPath, nil
}

// Submit package to A3M. Submits the package to A3M and returns the path of the generated AIP.
// The generated AIP is expected to be in the configured A3M Completed directory.
// Will retry submission on transient errors.
func (p *Preserver) submitPackage(ctx context.Context, transferPath, transferName string, config *transferservice.ProcessingConfig) (string, error) {
	var aipUuid string
	// Submit package to A3M with retry
	if err := utils.Retry(3, 2*time.Second, func() error {
		logger.Debug("Queing A3M Transfer: %s", utils.RelPath(p.envConfig.ProcessingBaseDir, transferPath))
		ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
		defer cancel()
		var submitErr error
		aipUuid, _, submitErr = p.a3mClient.SubmitPackage(ctx, transferPath, transferName, config)
		return submitErr
	}, utils.IsTransientError); err != nil {
		return "", fmt.Errorf("submission failed: %v", err)
	}
	return aipUuid, nil
}

// Post-processes the AIP. Extracts the AIP.
func (p *Preserver) postprocessPackage(ctx context.Context, processingAipDir, a3mAipPath string) (string, error) {
	// Extract AIP
	aipPath, err := utils.ExtractArchive(ctx, a3mAipPath, processingAipDir)
	if err != nil {
		return "", fmt.Errorf("error extracting AIP: %w", err)
	}
	logger.Debug("Extracted AIP: %s", utils.RelPath(p.envConfig.ProcessingBaseDir, aipPath))
	return aipPath, nil
}

// Convert the AIP to a ZIP archive.
func (p *Preserver) compressPackage(ctx context.Context, processingAipDir, aipPath string) (string, error) {
	archiveAipPath := filepath.Join(processingAipDir, fmt.Sprintf("%s.zip", filepath.Base(aipPath)))
	err := utils.CompressToZip(ctx, aipPath, archiveAipPath)
	if err != nil {
		return "", fmt.Errorf("error compressing AIP: %w", err)
	}
	return archiveAipPath, nil
}

// Uploads the AIP to Cells
func (p *Preserver) uploadPackage(ctx context.Context, userClient cells.UserClient, aipPath string) (string, error) {
	return p.cellsClient.UploadNode(ctx, userClient, aipPath, p.envConfig.Cells.ArchiveWorkspace)
}

// Construct the path of the A3M Generated AIP and ensures it exists
// TODO: Consider AIP Compression Algorithm
func getA3mAipPath(a3mCompletedDir string, packageName string, packageUUID string) (string, error) {
	expectedAIPPath := filepath.Join(a3mCompletedDir, packageName+"-"+packageUUID+".7z")
	if _, err := os.Stat(expectedAIPPath); os.IsNotExist(err) {
		logger.Error("A3M AIP not found: %v", err)
		return "", err
	}
	return expectedAIPPath, nil
}

// Construct the path of the A3M Generated DIP and ensures it exists
func getA3mDipPath(a3mDipsDir string, packageUUID string) (string, error) {
	expectedDIPPath := filepath.Join(a3mDipsDir, packageUUID)
	if _, err := os.Stat(expectedDIPPath); os.IsNotExist(err) {
		logger.Error("A3M DIP not found: %v", err)
		return "", err
	}
	return expectedDIPPath, nil
}

func transferNameFromPath(path string) string {
	// Get the base name of the path
	baseName := filepath.Base(path)
	// Remove the extension
	ext := filepath.Ext(baseName)
	name := strings.TrimSuffix(baseName, ext)
	// Sanitize the name
	name = sanitizeTransferName(name)
	return name
}

func sanitizeTransferName(name string) string {
	// Remove whitespace and special characters
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, ":", "_")
	name = strings.ReplaceAll(name, "*", "_")
	name = strings.ReplaceAll(name, "?", "_")
	name = strings.ReplaceAll(name, "\"", "_")
	name = strings.ReplaceAll(name, "<", "_")
	name = strings.ReplaceAll(name, ">", "_")
	return name
}
