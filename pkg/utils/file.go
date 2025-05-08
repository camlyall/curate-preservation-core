package utils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/penwern/preservation-go/pkg/logger"
)

// RelPath returns the relative path from the base directory to the given path.
// On error the input path is returned.
func RelPath(baseDir string, path string) string {
	relPath, err := filepath.Rel(baseDir, path)
	if err != nil {
		logger.Error("failed to get relative path: %v\n", err)
		return path
	}
	return relPath
}

func CreateDir(path string) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", path, err)
	}
	return nil
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
	err = CreateDir(uniqueDirPath)
	if err != nil {
		return "", err
	}
	return uniqueDirPath, nil
}

// validateExecutable validates and returns the absolute path of an executable.
func ValidateExecutable(path string) (string, error) {
	absPath, info, err := ValidateFile(path)
	if err != nil {
		return "", err
	}
	if info.Mode().Perm()&0111 == 0 {
		return "", fmt.Errorf("path %q is not executable", absPath)
	}
	return absPath, nil
}

func ValidateFile(path string) (string, os.FileInfo, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get absolute path for file %q: %v", path, err)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return "", nil, fmt.Errorf("file %q does not exist: %v", absPath, err)
	}
	if info.IsDir() {
		return "", nil, fmt.Errorf("path %q is a directory, expected a file", absPath)
	}
	return absPath, info, nil
}

func ValidateDirectory(path string) (string, error) {
	absDir, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for directory %q: %v", path, err)
	}
	info, err := os.Stat(absDir)
	if err != nil {
		return "", fmt.Errorf("directory %q does not exist: %v", absDir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path %q is not a directory", absDir)
	}
	return absDir, nil
}
