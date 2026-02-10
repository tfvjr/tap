package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/tap-dev/tap/internal/tui"
)

var (
	jsonOutput bool
	noDocker   bool
	verbose    bool
)

var rootCmd = &cobra.Command{
	Use:   "tap",
	Short: "Diagnostic dev process manager",
	Long:  "Tap gives developers instant visibility and diagnostics for every port, process, and container running on their dev machine, organized by project.",
	RunE: func(cmd *cobra.Command, args []string) error {
		// If --json flag is set, fall back to ls output for scripting.
		if jsonOutput {
			return lsRun(cmd, args)
		}
		// Default: launch TUI dashboard.
		m := tui.NewModel(noDocker)
		p := tea.NewProgram(m, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}
		return nil
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
