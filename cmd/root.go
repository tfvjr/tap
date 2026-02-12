package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/tfvjr/tap/internal/daemon"
	"github.com/tfvjr/tap/internal/store"
	"github.com/tfvjr/tap/internal/tui"
)

var (
	jsonOutput bool
	verbose    bool
	dataDir    string
	appStore   *store.Store
)

var rootCmd = &cobra.Command{
	Use:   "tap",
	Short: "Diagnostic dev process manager",
	Long:  "Tap gives developers instant visibility and diagnostics for every port, process, and container running on their dev machine, organized by project.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// _collect handles its own lifecycle — skip auto-start.
		if cmd.Name() == "_collect" {
			return nil
		}

		justStarted, err := daemon.EnsureRunning(dataDir)
		if err != nil {
			return fmt.Errorf("ensuring collector: %w", err)
		}

		appStore, err = store.Open(dataDir)
		if err != nil {
			return fmt.Errorf("opening store: %w", err)
		}

		// Wait for first snapshot if the collector was just started.
		if justStarted {
			deadline := time.Now().Add(3 * time.Second)
			for time.Now().Before(deadline) {
				if has, _ := appStore.HasData(); has {
					break
				}
				time.Sleep(200 * time.Millisecond)
			}
			if has, _ := appStore.HasData(); !has {
				fmt.Fprintln(os.Stderr, "tap: waiting for first snapshot...")
				for {
					if has, _ := appStore.HasData(); has {
						break
					}
					time.Sleep(500 * time.Millisecond)
				}
			}

			// First-run notice.
			if v, _ := appStore.GetMeta("collector_started"); v == "" {
				fmt.Fprintf(os.Stderr, "tap: started background collector (%s)\n", appStore.DBPath())
				appStore.SetMeta("collector_started", "1")
			}
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// If --json flag is set, fall back to ls output for scripting.
		if jsonOutput {
			return lsRun(cmd, args)
		}
		// Default: launch TUI dashboard.
		m := tui.NewModel(appStore)
		p := tea.NewProgram(m, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if appStore != nil {
			return appStore.Close()
		}
		return nil
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	home, _ := os.UserHomeDir()
	defaultDir := filepath.Join(home, ".tap")

	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Show full command lines and additional details")
	rootCmd.PersistentFlags().StringVar(&dataDir, "data-dir", defaultDir, "Data directory for tap")
}

func exitError(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
	os.Exit(1)
}
