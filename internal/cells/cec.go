package cells

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/penwern/curate-preservation-core/pkg/logger"
)

// sanitizePathArg sanitizes a path argument to prevent command injection
func sanitizePathArg(path string) string {
	// Clean the path and remove any shell metacharacters
	cleaned := filepath.Clean(path)
	// Remove potentially dangerous characters
	cleaned = strings.ReplaceAll(cleaned, ";", "")
	cleaned = strings.ReplaceAll(cleaned, "&", "")
	cleaned = strings.ReplaceAll(cleaned, "|", "")
	cleaned = strings.ReplaceAll(cleaned, "`", "")
	cleaned = strings.ReplaceAll(cleaned, "$", "")
	return cleaned
}

// sanitizeStringArg sanitizes a string argument to prevent command injection
func sanitizeStringArg(arg string) string {
	cleaned := strings.ReplaceAll(arg, ";", "")
	cleaned = strings.ReplaceAll(cleaned, "&", "")
	cleaned = strings.ReplaceAll(cleaned, "|", "")
	cleaned = strings.ReplaceAll(cleaned, "`", "")
	cleaned = strings.ReplaceAll(cleaned, "$", "")
	cleaned = strings.ReplaceAll(cleaned, "\"", "")
	cleaned = strings.ReplaceAll(cleaned, "'", "")
	if arg != cleaned {
		logger.Warn("Sanitized argument from %s to %s", arg, cleaned)
	}
	return cleaned
}

// DownloadNode downloads a node from Cells to a local directory using the CEC binary
// Returns the output of the command
func cecDownloadNode(ctx context.Context, cecPath, address, username, token, cellsSrc, dest string) ([]byte, error) {
	// Sanitize inputs to prevent command injection
	cecPath = sanitizePathArg(cecPath)
	address = sanitizeStringArg(address)
	username = sanitizeStringArg(username)
	cellsSrc = sanitizePathArg(cellsSrc)
	dest = sanitizePathArg(dest)

	logger.Debug("Downloading {cecPath: %s, address: %s, username: %s, cellsSrc: %s, dest: %s}", cecPath, address, username, cellsSrc, dest)
	// #nosec G204 -- input arguments are sanitized above
	cmd := exec.CommandContext(ctx, cecPath, "scp", "-n", "--url", address, "--skip-verify", "--login", username, "--token", token, fmt.Sprintf("cells://%s/", cellsSrc), dest)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("CEC download node: %v", err)
		return output, fmt.Errorf("CEC download error: %v, Output: %s", err, string(output))
	}
	return output, nil
}

// UploadNode uploads a node from a local directory to Cells using the CEC binary
// Returns the output of the command
func cecUploadNode(ctx context.Context, cecPath, address, username, token, src, cellsDest string) ([]byte, error) {
	// Sanitize inputs to prevent command injection
	cecPath = sanitizePathArg(cecPath)
	address = sanitizeStringArg(address)
	username = sanitizeStringArg(username)
	src = sanitizePathArg(src)
	cellsDest = sanitizePathArg(cellsDest)

	logger.Debug("Uploading {cecPath: %s, address: %s, username: %s, src: %s, cellsDest: %s}", cecPath, address, username, src, cellsDest)
	// #nosec G204 -- input arguments are sanitized above
	cmd := exec.CommandContext(ctx, cecPath, "scp", "-n", "--url", address, "--skip-verify", "--login", username, "--token", token, src, fmt.Sprintf("cells://%s/", cellsDest))
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("CEC upload node: %v", err)
		return output, fmt.Errorf("CEC upload error: %v, Output: %s", err, string(output))
	}
	return output, nil
}
