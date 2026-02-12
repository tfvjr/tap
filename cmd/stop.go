package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tfvjr/tap/internal/model"
)

var stopCmd = &cobra.Command{
	Use:   "stop <project-name>",
	Short: "Stop all processes for a project",
	Long:  "Stop all running processes (native and containerized) that belong to the given project.",
	Args:  cobra.ExactArgs(1),
	RunE:  stopRun,
}

func init() {
	rootCmd.AddCommand(stopCmd)
}

func stopRun(cmd *cobra.Command, args []string) error {
	name := args[0]

	snap, err := appStore.LatestSnapshot(true)
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}
	if snap == nil {
		return fmt.Errorf("no data yet")
	}

	// Find the project (case-insensitive match).
	var matched *model.Project
	for i := range snap.Projects {
		if strings.EqualFold(snap.Projects[i].Name, name) {
			matched = &snap.Projects[i]
			break
		}
	}

	if matched == nil {
		fmt.Fprintf(os.Stderr, "No project found matching '%s'\n", name)
		if len(snap.Projects) > 0 {
			fmt.Fprintln(os.Stderr, "Available projects:")
			for _, p := range snap.Projects {
				fmt.Fprintf(os.Stderr, "  - %s\n", p.Name)
			}
		}
		os.Exit(1)
	}

	if len(matched.Processes) == 0 {
		fmt.Printf("Project '%s' has no running processes.\n", matched.Name)
		return nil
	}

	// --json: output what would be stopped as JSON without killing anything.
	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(matched)
	}

	// Show what will be killed and ask for confirmation.
	fmt.Printf("The following processes will be stopped for project '%s':\n\n", matched.Name)
	for _, proc := range matched.Processes {
		pid := "-"
		if proc.PID != nil {
			pid = fmt.Sprintf("%d", *proc.PID)
		}
		ports := formatPorts(proc.Ports)
		if ports == "" {
			ports = "(none)"
		}
		fmt.Printf("  PID %-8s  %-20s  Ports: %s\n", pid, proc.Name, ports)
	}

	fmt.Printf("\nStop %d process(es)? [y/N] ", len(matched.Processes))

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	answer := strings.TrimSpace(scanner.Text())
	if !strings.EqualFold(answer, "y") {
		fmt.Println("Aborted.")
		return nil
	}

	// Stop each process.
	stopped := 0
	for _, proc := range matched.Processes {
		if err := stopProcess(proc); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: could not stop %s (PID %s): %v\n", proc.Name, formatPID(proc.PID), err)
			continue
		}
		stopped++
	}

	fmt.Printf("Stopped %d processes for project %s\n", stopped, matched.Name)
	return nil
}

// stopProcess kills a native process or stops a container.
func stopProcess(proc model.DevProcess) error {
	switch proc.Kind {
	case model.ProcessDocker, model.ProcessPodman:
		if proc.ContainerInfo == nil {
			return fmt.Errorf("no container info available")
		}
		runtime := "docker"
		if proc.Kind == model.ProcessPodman {
			runtime = "podman"
		}
		out, err := exec.Command(runtime, "stop", proc.ContainerInfo.ContainerID).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s stop: %s: %w", runtime, strings.TrimSpace(string(out)), err)
		}
		return nil
	default:
		// Native process.
		if proc.PID == nil {
			return fmt.Errorf("no PID available")
		}
		p, err := os.FindProcess(int(*proc.PID))
		if err != nil {
			return fmt.Errorf("find process %d: %w", *proc.PID, err)
		}
		return p.Kill()
	}
}

// formatPID returns the PID as a string or "-" if nil.
func formatPID(pid *int32) string {
	if pid == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *pid)
}
