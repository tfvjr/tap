package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/tfvjr/tap/internal/discovery"
	"github.com/tfvjr/tap/internal/model"
)

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List all dev processes",
	Long:  "List all running dev processes, grouped by project, in a table format.",
	RunE:  lsRun,
}

func init() {
	rootCmd.AddCommand(lsCmd)
}

func lsRun(cmd *cobra.Command, args []string) error {
	snap, err := discovery.TakeSnapshot(!noDocker, true)
	if err != nil {
		return fmt.Errorf("snapshot: %w", err)
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(snap)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "PROJECT\tPID\tNAME\tPORTS\tCPU%\tMEMORY\tUPTIME")

	for _, proj := range snap.Projects {
		for _, proc := range proj.Processes {
			printProcessRow(w, proj.Name, &proc)
		}
	}
	for _, proc := range snap.Unattributed {
		printProcessRow(w, "-", &proc)
	}

	return w.Flush()
}

func printProcessRow(w *tabwriter.Writer, project string, p *model.DevProcess) {
	pid := "-"
	if p.PID != nil {
		pid = fmt.Sprintf("%d", *p.PID)
	}

	name := p.Name
	if verbose && p.Command != "" {
		name = p.Command
	}

	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%.1f%%\t%.1f MB\t%s\n",
		project,
		pid,
		name,
		formatPorts(p.Ports),
		p.CPUPercent,
		p.MemoryMB(),
		formatUptime(p.Uptime()),
	)
}

func formatPorts(ports []model.PortBinding) string {
	if len(ports) == 0 {
		return ""
	}
	parts := make([]string, len(ports))
	for i, pb := range ports {
		parts[i] = fmt.Sprintf(":%d", pb.Port)
	}
	return strings.Join(parts, ", ")
}

func formatUptime(d time.Duration) string {
	if d < 0 {
		d = 0
	}

	totalSeconds := int(d.Seconds())
	days := totalSeconds / 86400
	hours := (totalSeconds % 86400) / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh", days, hours)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, minutes)
	case minutes > 0:
		return fmt.Sprintf("%dm", minutes)
	default:
		return fmt.Sprintf("%ds", seconds)
	}
}
