package cmd

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/tfvjr/tap/internal/tui"
)

var dashCmd = &cobra.Command{
	Use:   "dash",
	Short: "Launch interactive TUI dashboard",
	Long:  "Launch the interactive terminal UI with project grouping, diagnostics, and keyboard controls.",
	RunE:  dashRun,
}

func init() {
	rootCmd.AddCommand(dashCmd)
}

func dashRun(cmd *cobra.Command, args []string) error {
	m := tui.NewModel(noDocker)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	return nil
}
