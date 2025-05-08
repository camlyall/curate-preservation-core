package utils

import (
	"context"
	"fmt"
	"os/exec"
)

// RsyncFile uses the local rsync command to sync src to dest.
// extraArgs is any additional flags you want to pass to rsync,
// for example:
//
//	[]string{"-avz", "-e", "ssh -p 2222"}
//
// or
//
//	[]string{"--progress"}
//
// Be sure rsync is installed and on your PATH.
func RsyncFile(ctx context.Context, src, dest string, extraArgs []string) error {
	// Base args: source and destination
	args := append(extraArgs, src, dest)

	// Create the command
	cmd := exec.CommandContext(ctx, "rsync", args...)

	// Run it
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rsync failed: %w", err)
	}
	return nil
}
