package cmd

import (
	"fmt"
	"runtime"

	"github.com/penwern/curate-preservation-core/pkg/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  `Display version, build time, and commit information for the Curate Preservation System.`,
	Run: func(_ *cobra.Command, _ []string) {
		//nolint:forbidigo // Version command needs to output directly to stdout
		fmt.Printf("Curate Preservation System\n")
		//nolint:forbidigo // Version command needs to output directly to stdout
		fmt.Printf("Version:    %s\n", version.Version())
		//nolint:forbidigo // Version command needs to output directly to stdout
		fmt.Printf("Git Commit: %s\n", version.Commit())
		//nolint:forbidigo // Version command needs to output directly to stdout
		fmt.Printf("Build Date: %s\n", version.BuildTime())
		//nolint:forbidigo // Version command needs to output directly to stdout
		fmt.Printf("Go Version: %s\n", runtime.Version())
		//nolint:forbidigo // Version command needs to output directly to stdout
		fmt.Printf("OS/Arch:    %s/%s\n", runtime.GOOS, runtime.GOARCH)
	},
}
