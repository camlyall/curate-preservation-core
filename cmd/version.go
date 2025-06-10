package cmd

import (
	"fmt"

	"github.com/penwern/curate-preservation-core/pkg/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  `Display version, build time, and commit information for the Curate Preservation System.`,
	Run: func(_ *cobra.Command, _ []string) {
		//nolint:forbidigo // Version command needs to output directly to stdout
		fmt.Printf("Version:    %s\n", version.Version())
		//nolint:forbidigo // Version command needs to output directly to stdout
		fmt.Printf("Commit:     %s\n", version.Commit())
		//nolint:forbidigo // Version command needs to output directly to stdout
		fmt.Printf("Built:      %s\n", version.BuildTime())
	},
}
