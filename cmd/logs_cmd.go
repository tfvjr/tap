package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/tfvjr/tap/internal/store"
)

var (
	logsSince  string
	logsStream string
	logsFollow bool
	logsLimit  int
)

var logsCmd = &cobra.Command{
	Use:   "logs [project]",
	Short: "Show captured console output",
	Long:  "Display output captured by `tap run`. Filter by project, time range, or stream.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  logsRun,
}

func init() {
	logsCmd.Flags().StringVar(&logsSince, "since", "", "Show logs since duration (e.g. 5m, 1h)")
	logsCmd.Flags().StringVar(&logsStream, "stream", "", "Filter by stream: stdout or stderr")
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Follow log output (like tail -f)")
	logsCmd.Flags().IntVarP(&logsLimit, "limit", "n", 100, "Maximum number of lines to show")
	rootCmd.AddCommand(logsCmd)
}

func logsRun(cmd *cobra.Command, args []string) error {
	var project string
	if len(args) > 0 {
		project = args[0]
	}

	var since time.Time
	if logsSince != "" {
		d, err := time.ParseDuration(logsSince)
		if err != nil {
			return fmt.Errorf("invalid --since duration %q: %w", logsSince, err)
		}
		since = time.Now().Add(-d)
	}

	if logsFollow {
		return followLogs(project)
	}

	var lines []store.LogLine
	var err error

	if project != "" {
		lines, err = appStore.QueryLogsByProject(project, since, logsLimit)
	} else {
		lines, err = appStore.QueryLogs("", logsStream, since, logsLimit)
	}
	if err != nil {
		return fmt.Errorf("query logs: %w", err)
	}

	if len(lines) == 0 {
		fmt.Println("No logs found. Use `tap run <command>` to capture output.")
		return nil
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(lines)
	}

	for _, l := range lines {
		ts := l.Timestamp.Format("15:04:05")
		streamTag := ""
		if logsStream == "" {
			if l.Stream == "stderr" {
				streamTag = " [err]"
			}
		}
		fmt.Printf("%s%s  %s\n", ts, streamTag, l.Line)
	}

	return nil
}

func followLogs(project string) error {
	lastSeq, _ := appStore.MaxLogSeq()

	fmt.Fprintln(os.Stderr, "Following logs... (Ctrl+C to stop)")

	for {
		lines, err := appStore.QueryLogsAfter(lastSeq, project, logsStream)
		if err != nil {
			return err
		}

		for _, l := range lines {
			ts := l.Timestamp.Format("15:04:05")
			streamTag := ""
			if l.Stream == "stderr" {
				streamTag = " [err]"
			}
			fmt.Printf("%s%s  %s\n", ts, streamTag, l.Line)
			if l.Seq > lastSeq {
				lastSeq = l.Seq
			}
		}

		time.Sleep(500 * time.Millisecond)
	}
}
