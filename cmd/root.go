package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	jsonOutput bool
	noDocker   bool
	verbose    bool
)

var rootCmd = &cobra.Command{
	Use:   "tap",
	Short: "Dev machine resource manager",
	Long:  "Tap gives developers instant visibility and control over every port, process, container, and resource running on their dev machine.",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Default behavior: show ls output (TUI comes in Phase 2)
		return lsRun(cmd, args)
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	rootCmd.PersistentFlags().BoolVar(&noDocker, "no-docker", false, "Skip Docker/Podman discovery")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Show full command lines and additional details")
}

func exitError(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
	os.Exit(1)
}
