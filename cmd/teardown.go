package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tfvjr/tap/internal/shim"
)

var teardownCmd = &cobra.Command{
	Use:   "teardown",
	Short: "Remove shims and restore shell config",
	Long:  "Remove the PATH shims that tap uses for automatic console capture, and clean up shell config modifications.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := shim.RemoveShims(dataDir); err != nil {
			return fmt.Errorf("teardown: %w", err)
		}
		fmt.Println("Removed shims and cleaned shell config.")
		fmt.Println("Restart your terminal to complete the removal.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(teardownCmd)
}
