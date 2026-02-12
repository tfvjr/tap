package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var projectCmd = &cobra.Command{
	Use:   "project <name>",
	Short: "Show all processes for a specific project",
	Long:  "Show all running dev processes that belong to the given project.",
	Args:  cobra.ExactArgs(1),
	RunE:  projectRun,
}

func init() {
	rootCmd.AddCommand(projectCmd)
}

func projectRun(cmd *cobra.Command, args []string) error {
	name := args[0]

	snap, err := appStore.LatestSnapshot(true)
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}
	if snap == nil {
		return fmt.Errorf("no data yet")
	}

	for i := range snap.Projects {
		if strings.EqualFold(snap.Projects[i].Name, name) {
			proj := &snap.Projects[i]

			if jsonOutput {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(proj)
			}

			fmt.Printf("Project: %s (%s)\n\n", proj.Name, proj.Path)

			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "PID\tNAME\tPORTS\tCPU%\tMEMORY\tUPTIME")

			for _, proc := range proj.Processes {
				pid := "-"
				if proc.PID != nil {
					pid = fmt.Sprintf("%d", *proc.PID)
				}

				procName := proc.Name
				if verbose && proc.Command != "" {
					procName = proc.Command
				}

				fmt.Fprintf(w, "%s\t%s\t%s\t%.1f%%\t%.1f MB\t%s\n",
					pid,
					procName,
					formatPorts(proc.Ports),
					proc.CPUPercent,
					proc.MemoryMB(),
					formatUptime(proc.Uptime()),
				)
			}

			return w.Flush()
		}
	}

	fmt.Fprintf(os.Stderr, "Project %q not found.\n", name)
	if len(snap.Projects) > 0 {
		fmt.Fprintln(os.Stderr, "Available projects:")
		for _, p := range snap.Projects {
			fmt.Fprintf(os.Stderr, "  - %s\n", p.Name)
		}
	}
	os.Exit(1)
	return nil
}
