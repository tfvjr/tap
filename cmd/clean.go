package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tfvjr/tap/internal/model"
)

// stoppedContainer holds metadata about a Docker container in the "exited" state.
type stoppedContainer struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Image  string `json:"image"`
	Status string `json:"status"`
}

// cleanReport is the JSON-serialisable output of `tap clean --json`.
type cleanReport struct {
	OrphanedProcesses  []orphanedProcess  `json:"orphaned_processes"`
	StoppedContainers  []stoppedContainer `json:"stopped_containers"`
}

// orphanedProcess is the JSON-serialisable representation of a long-running dev process.
type orphanedProcess struct {
	PID     *int32             `json:"pid,omitempty"`
	Name    string             `json:"name"`
	Project string             `json:"project,omitempty"`
	Uptime  string             `json:"uptime"`
	Ports   []model.PortBinding `json:"ports"`
}

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Find and remove orphaned processes and stopped containers",
	Long:  "Discovers dev processes running for more than 24 hours and stopped Docker containers, then offers to clean them up.",
	RunE:  cleanRun,
}

func init() {
	rootCmd.AddCommand(cleanCmd)
}

func cleanRun(cmd *cobra.Command, args []string) error {
	snap, err := appStore.LatestSnapshot(true)
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}
	if snap == nil {
		fmt.Println("No data yet. The collector is still starting.")
		return nil
	}

	// Find orphaned processes (running > 24 hours).
	allProcs := collectAllProcesses(snap)
	threshold := 24 * time.Hour

	var orphaned []model.DevProcess
	for i := range allProcs {
		if allProcs[i].Uptime() > threshold {
			orphaned = append(orphaned, allProcs[i])
		}
	}

	// Find stopped Docker containers.
	stopped := discoverStoppedContainers()

	if len(orphaned) == 0 && len(stopped) == 0 {
		fmt.Println("Nothing to clean up.")
		return nil
	}

	report := buildCleanReport(orphaned, stopped)

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}

	if len(orphaned) > 0 {
		fmt.Printf("Found %d long-running processes:\n", len(orphaned))
		for _, proc := range orphaned {
			pid := "-"
			if proc.PID != nil {
				pid = fmt.Sprintf("%d", *proc.PID)
			}
			project := proc.Project
			if project == "" {
				project = "-"
			}
			fmt.Printf("  PID %s  %s  project=%s  uptime=%s  ports=%s\n",
				pid,
				proc.Name,
				project,
				formatUptime(proc.Uptime()),
				formatPorts(proc.Ports),
			)
		}
	}

	if len(stopped) > 0 {
		fmt.Printf("Found %d stopped containers:\n", len(stopped))
		for _, c := range stopped {
			fmt.Printf("  %s  %s  image=%s  %s\n", c.ID, c.Name, c.Image, c.Status)
		}
	}

	fmt.Print("\nRemove all? [y/N] ")
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return nil
	}
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
	if answer != "y" && answer != "yes" {
		fmt.Println("Aborted.")
		return nil
	}

	killedProcs := 0
	for _, proc := range orphaned {
		if proc.PID == nil {
			continue
		}
		pid := int(*proc.PID)
		p, err := os.FindProcess(pid)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Could not find process %d: %v\n", pid, err)
			continue
		}
		if err := p.Kill(); err != nil {
			fmt.Fprintf(os.Stderr, "  Could not kill process %d (%s): %v\n", pid, proc.Name, err)
			continue
		}
		killedProcs++
	}

	removedContainers := 0
	for _, c := range stopped {
		out, err := exec.Command("docker", "rm", c.ID).CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Could not remove container %s: %s\n", c.ID, strings.TrimSpace(string(out)))
			continue
		}
		removedContainers++
	}

	fmt.Printf("Cleaned up %d processes and %d containers.\n", killedProcs, removedContainers)
	return nil
}

// discoverStoppedContainers shells out to `docker ps -a` to find exited containers.
func discoverStoppedContainers() []stoppedContainer {
	out, err := exec.Command(
		"docker", "ps", "-a",
		"--filter", "status=exited",
		"--format", "{{.ID}}\t{{.Names}}\t{{.Image}}\t{{.Status}}",
	).Output()
	if err != nil {
		return nil
	}

	var containers []stoppedContainer
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) < 4 {
			continue
		}
		containers = append(containers, stoppedContainer{
			ID:     parts[0],
			Name:   parts[1],
			Image:  parts[2],
			Status: parts[3],
		})
	}
	return containers
}

// buildCleanReport converts the discovered orphaned processes and stopped
// containers into a cleanReport suitable for JSON serialisation.
func buildCleanReport(orphaned []model.DevProcess, stopped []stoppedContainer) cleanReport {
	procs := make([]orphanedProcess, 0, len(orphaned))
	for _, p := range orphaned {
		project := p.Project
		procs = append(procs, orphanedProcess{
			PID:     p.PID,
			Name:    p.Name,
			Project: project,
			Uptime:  formatUptime(p.Uptime()),
			Ports:   p.Ports,
		})
	}
	return cleanReport{
		OrphanedProcesses: procs,
		StoppedContainers: stopped,
	}
}
