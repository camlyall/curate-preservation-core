package utils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// RelPath returns the relative path from the base directory to the given path.
// On error the input path is returned.
func RelPath(baseDir string, path string) string {
	relPath, err := filepath.Rel(baseDir, path)
	if err != nil {
		fmt.Printf("failed to get relative path: %v\n", err)
		return path
	}
	return relPath
}

// MakeUniqueDir creates a new directory in the provided base directory with a unique name using UUID.
// It returns the path to the new directory.
func MakeUniqueDir(ctx context.Context, baseDirPath string) (string, error) {
	// Ensure baseDirPath exists and is a directory
	info, err := os.Stat(baseDirPath)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("base directory %q does not exist", baseDirPath)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("base directory %q is not a directory", baseDirPath)
	}
	if !filepath.IsAbs(baseDirPath) {
		return "", fmt.Errorf("base directory %q is not an absolute path", baseDirPath)
	}
	var uniqueDirPath string
	for range 5 {
		uid := uuid.New().String()
		if uid == "" {
			return "", fmt.Errorf("failed to generate UUID")
		}
		testUniqueDirPath := filepath.Join(baseDirPath, uid)
		// Ensure the directory doesn't already exist
		if _, err := os.Stat(testUniqueDirPath); os.IsNotExist(err) {
			uniqueDirPath = testUniqueDirPath
			break
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
	}
	if uniqueDirPath == "" {
		return "", fmt.Errorf("failed to generate unique directory name")
	}
	err = os.Mkdir(uniqueDirPath, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", uniqueDirPath, err)
	}
	return uniqueDirPath, nil
}
