package cells

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/penwern/curate-preservation-core/pkg/logger"
)

// DownloadNode downloads a node from Cells to a local directory using the CEC binary
// Returns the output of the command
func cecDownloadNode(ctx context.Context, cecPath, address, username, token, cellsSrc, dest string) ([]byte, error) {
	logger.Debug("Downloading {cecPath: %s, address: %s, username: %s, cellsSrc: %s, dest: %s}", cecPath, address, username, cellsSrc, dest)
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
	logger.Debug("Uploading {cecPath: %s, address: %s, username: %s, src: %s, cellsDest: %s}", cecPath, address, username, src, cellsDest)
	cmd := exec.CommandContext(ctx, cecPath, "scp", "-n", "--url", address, "--skip-verify", "--login", username, "--token", token, src, fmt.Sprintf("cells://%s/", cellsDest))
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("CEC upload node: %v", err)
		return output, fmt.Errorf("CEC upload error: %v, Output: %s", err, string(output))
	}
	return output, nil
}
