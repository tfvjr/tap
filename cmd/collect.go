package cmd

import (
	"github.com/spf13/cobra"
	"github.com/tfvjr/tap/internal/daemon"
)

var collectCmd = &cobra.Command{
	Use:    "_collect",
	Short:  "Run the background collector (internal)",
	Hidden: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil // Skip auto-start logic — we ARE the collector.
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return daemon.RunCollector(dataDir)
	},
}

func init() {
	rootCmd.AddCommand(collectCmd)
}
