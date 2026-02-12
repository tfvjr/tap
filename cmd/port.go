package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tfvjr/tap/internal/model"
)

var portCmd = &cobra.Command{
	Use:   "port <PORT>",
	Short: "Show what is running on a specific port",
	Long:  "Looks up which dev process is bound to the given port and prints a summary.",
	Args:  cobra.ExactArgs(1),
	RunE:  portRun,
}

func init() {
	rootCmd.AddCommand(portCmd)
}

func portRun(cmd *cobra.Command, args []string) error {
	raw := strings.TrimPrefix(args[0], ":")
	portNum, err := strconv.ParseUint(raw, 10, 16)
	if err != nil {
		return fmt.Errorf("invalid port number %q: %w", args[0], err)
	}
	targetPort := uint16(portNum)

	procs, err := appStore.ProcessesByPort(targetPort)
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}

	if len(procs) == 0 {
		fmt.Fprintf(os.Stderr, "Nothing found on port %d\n", targetPort)
		os.Exit(1)
	}

	proc := &procs[0]

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(proc)
	}

	pidStr := "-"
	if proc.PID != nil {
		pidStr = strconv.Itoa(int(*proc.PID))
	}

	project := proc.Project
	if project == "" {
		project = "(none)"
	}

	fmt.Printf("PID %s | %s (%s) | project: %s | running %s | CPU %.1f%%\n",
		pidStr,
		proc.Name,
		proc.Command,
		project,
		formatUptime(proc.Uptime()),
		proc.CPUPercent,
	)

	return nil
}

// findProcessByPort searches all processes in the snapshot for one that has a
// PortBinding matching the target port. Returns nil if none is found.
func findProcessByPort(snap *model.Snapshot, target uint16) *model.DevProcess {
	for i := range snap.Projects {
		for j := range snap.Projects[i].Processes {
			p := &snap.Projects[i].Processes[j]
			for _, pb := range p.Ports {
				if pb.Port == target {
					return p
				}
			}
		}
	}
	for i := range snap.Unattributed {
		p := &snap.Unattributed[i]
		for _, pb := range p.Ports {
			if pb.Port == target {
				return p
			}
		}
	}
	return nil
}
