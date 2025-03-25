package cells

import (
	"context"
	"fmt"
	"os/exec"
)

// DownloadNode downloads a node from Cells to a local directory using the CEC binary
// Returns the output of the command
func cecDownloadNode(ctx context.Context, cecPath, address, username, token, cellsSrc, dest string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, cecPath, "scp", "-n", "--url", address, "--skip-verify", "--login", username, "--token", token, fmt.Sprintf("cells://%s/", cellsSrc), dest)
	return cmd.CombinedOutput()
}

// UploadNode uploads a node from a local directory to Cells using the CEC binary
// Returns the output of the command
func cecUploadNode(ctx context.Context, cecPath, address, username, token, src, cellsDest string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, cecPath, "scp", "-n", "--url", address, "--skip-verify", "--login", username, "--token", token, src, fmt.Sprintf("cells://%s/", cellsDest))
	return cmd.CombinedOutput()
}
