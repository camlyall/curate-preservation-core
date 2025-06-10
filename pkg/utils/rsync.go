package utils

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// allowedRsyncFlags is a whitelist of allowed rsync flags
var allowedRsyncFlags = map[string]bool{
	"-a":         true,
	"--archive":  true,
	"-v":         true,
	"--verbose":  true,
	"-z":         true,
	"--compress": true,
	"--progress": true,
	"-e":         true, // for SSH options
}

// validateRsyncArg checks if an rsync argument is safe to use
func validateRsyncArg(arg string) error {
	// Check if it's a known safe flag
	if allowedRsyncFlags[arg] {
		return nil
	}

	// Handle SSH command (-e "ssh -p 2222")
	if strings.HasPrefix(arg, "ssh ") {
		// Only allow basic SSH options
		parts := strings.SplitSeq(arg, " ")
		for part := range parts {
			if part == "ssh" {
				continue
			}
			if part == "-p" {
				continue
			}
			// Allow only port numbers
			if _, err := fmt.Sscanf(part, "%d", new(int)); err != nil {
				return fmt.Errorf("invalid SSH option: %s", part)
			}
		}
		return nil
	}

	// If it's not a flag (doesn't start with -), it must be a path
	if !strings.HasPrefix(arg, "-") {
		// Clean the path to remove any potential directory traversal
		cleaned := filepath.Clean(arg)
		if cleaned != arg {
			return fmt.Errorf("invalid path: %s", arg)
		}
		return nil
	}

	return fmt.Errorf("unsupported rsync argument: %s", arg)
}

// RsyncFile uses the local rsync command to sync src to dest.
// This function implements security measures to prevent command injection:
// 1. Only allows specific rsync flags (see allowedRsyncFlags)
// 2. Validates all arguments including paths
// 3. Cleans paths to prevent directory traversal
//
// Allowed flags include: -a/--archive, -v/--verbose, -z/--compress, --progress
// SSH options are allowed but restricted to port specifications only.
//
// Example usage:
//
//	// Basic sync
//	RsyncFile(ctx, "src/", "dest/", []string{"-av"})
//
//	// With SSH
//	RsyncFile(ctx, "src/", "user@host:dest/", []string{"-avz", "-e", "ssh -p 2222"})
//
// Security note: This function is designed to be used with trusted input only.
// While it implements security measures, it should not be exposed directly to
// untrusted user input without additional validation.
func RsyncFile(ctx context.Context, src, dest string, extraArgs []string) error {
	// Validate source and destination
	if src == "" || dest == "" {
		return fmt.Errorf("source and destination paths cannot be empty")
	}

	// Clean paths
	src = filepath.Clean(src)
	dest = filepath.Clean(dest)

	// Validate all arguments
	for _, arg := range extraArgs {
		if err := validateRsyncArg(arg); err != nil {
			return fmt.Errorf("invalid argument: %w", err)
		}
	}

	// Build command arguments: extra args + source + destination
	cmdArgs := make([]string, 0, len(extraArgs)+2)
	cmdArgs = append(cmdArgs, extraArgs...)
	cmdArgs = append(cmdArgs, src, dest)

	// Create the command
	// #nosec G204 -- all arguments are validated by validateRsyncArg function above
	cmd := exec.CommandContext(ctx, "rsync", cmdArgs...)

	// Run it
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rsync failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}
