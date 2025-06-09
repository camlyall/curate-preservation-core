package main

import (
	"os"

	"github.com/penwern/curate-preservation-core/cmd"
	"github.com/penwern/curate-preservation-core/pkg/logger"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		logger.Error("Error executing command: %v", err)
		os.Exit(1)
	}
}
