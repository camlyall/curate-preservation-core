package utils

import "path/filepath"

// RelPath returns the relative path from the base directory to the given path.
// On error the input path is returned.
func RelPath(baseDir string, path string) string {
	relPath, err := filepath.Rel(baseDir, path)
	if err != nil {
		return path
	}
	return relPath
}
