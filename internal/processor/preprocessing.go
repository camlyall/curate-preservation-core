// Preprocessor for preparing packages downloaded from Cells for submission to A3M

package processor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/penwern/preservation-go/pkg/utils"
)

// PreprocessPackage prepares a package for preservation submission and returns the path to the preprocessed package path.
func PreprocessPackage(packagePath string, preprocessingDir string) (string, error) {

	packageName := filepath.Base(strings.TrimSuffix(packagePath, filepath.Ext(packagePath)))

	// Create transfer package directory
	transferPath := preprocessingDir + "/" + packageName
	if err := os.Mkdir(transferPath, 0755); err != nil {
		return "", fmt.Errorf("error creating transfer directory: %w", err)
	}

	// Create data subdirectory
	dataDir := transferPath + "/data"
	if err := os.Mkdir(dataDir, 0755); err != nil {
		return "", fmt.Errorf("error creating data directory: %w", err)
	}

	// Get file info
	fileInfo, err := os.Stat(packagePath)
	if err != nil {
		return "", fmt.Errorf("error checking path: %w", err)
	}

	if fileInfo.Mode().IsRegular() && utils.IsZipFile(packagePath) {
		// If it's a ZIP file, extract it
		_, err := utils.ExtractZip(packagePath, filepath.Join(dataDir, packageName))
		if err != nil {
			return "", fmt.Errorf("error extracting zip: %w", err)
		}
	} else if fileInfo.Mode().IsRegular() {
		// If it's a regular file, move it
		err := os.Rename(packagePath, filepath.Join(dataDir, filepath.Base(packagePath)))
		if err != nil {
			return "", fmt.Errorf("error moving file: %w", err)
		}
	} else if fileInfo.IsDir() {
		// If it's a directory, move it
		err := os.Rename(packagePath, filepath.Join(dataDir, filepath.Base(packagePath)))
		if err != nil {
			return "", fmt.Errorf("error moving directory: %w", err)
		}
	} else {
		return "", fmt.Errorf("file type not supported: %s", packagePath)
	}

	// Create metadata subdirectory
	metadataDir := transferPath + "/metadata"
	if err = os.Mkdir(metadataDir, 0755); err != nil {
		return "", err
	}
	// TODO: Write dc and isadg json metadata

	// TODO: Write premis xml

	return transferPath, nil
}
