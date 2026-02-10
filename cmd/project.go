package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/tfvjr/tap/internal/discovery"
	"github.com/tfvjr/tap/internal/model"
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

	snap, err := discovery.TakeSnapshot(!noDocker, true)
	if err != nil {
		return fmt.Errorf("snapshot: %w", err)
	}

	var found *model.Project
	for i := range snap.Projects {
		if strings.EqualFold(snap.Projects[i].Name, name) {
			found = &snap.Projects[i]
			break
		}
	}

	if found == nil {
		fmt.Fprintf(os.Stderr, "Project %q not found.\n", name)
		if len(snap.Projects) > 0 {
			fmt.Fprintln(os.Stderr, "Available projects:")
			for _, p := range snap.Projects {
				fmt.Fprintf(os.Stderr, "  - %s\n", p.Name)
			}
		}
		os.Exit(1)
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(found)
	}

	fmt.Printf("Project: %s (%s)\n\n", found.Name, found.Path)

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "PID\tNAME\tPORTS\tCPU%\tMEMORY\tUPTIME")

	for _, proc := range found.Processes {
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
