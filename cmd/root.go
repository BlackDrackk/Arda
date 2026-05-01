package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

const version = "0.1.0"

var rootCmd = &cobra.Command{
	Use:     "arda",
	Version: version,
	Short:   "Arda — Podman-powered container manager",
	Long:    `Arda is a fast, rootless container manager built on Podman, configured via ~/.arda/config.toml.`,
	// No PersistentPreRun: commands output only what matters.
}

// Execute is the entry point called by main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
