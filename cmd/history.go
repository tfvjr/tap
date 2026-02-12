package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var (
	historySince string
	historyLimit int
)

var historyCmd = &cobra.Command{
	Use:   "history [project]",
	Short: "Show snapshot history",
	Long:  "Display a timeline of system snapshots showing process counts, CPU/memory trends, and project activity.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  historyRun,
}

func init() {
	historyCmd.Flags().StringVar(&historySince, "since", "1h", "Time range (e.g. 30m, 1h, 24h)")
	historyCmd.Flags().IntVarP(&historyLimit, "limit", "n", 50, "Maximum number of entries")
	rootCmd.AddCommand(historyCmd)
}

func historyRun(cmd *cobra.Command, args []string) error {
	var project string
	if len(args) > 0 {
		project = args[0]
	}

	since, err := time.ParseDuration(historySince)
	if err != nil {
		return fmt.Errorf("invalid --since duration %q: %w", historySince, err)
	}

	summaries, err := appStore.SnapshotSummaries(project, since, historyLimit)
	if err != nil {
		return fmt.Errorf("query history: %w", err)
	}

	if len(summaries) == 0 {
		fmt.Println("No history data yet. The collector stores snapshots every 5 seconds.")
		return nil
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(summaries)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "TIME\tPROCESSES\tPROJECTS\tCPU%\tMEMORY")

	for _, s := range summaries {
		ts := s.TakenAt.Format("15:04:05")
		memStr := fmt.Sprintf("%.0f MB", s.MemUsedMB)
		if s.MemUsedMB > 1024 {
			memStr = fmt.Sprintf("%.1f GB", s.MemUsedMB/1024)
		}
		fmt.Fprintf(w, "%s\t%d\t%d\t%.0f%%\t%s\n",
			ts, s.ProcessCount, s.ProjectCount, s.SystemCPU, memStr)
	}

	return w.Flush()
}
