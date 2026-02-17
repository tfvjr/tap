package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/tfvjr/tap/internal/daemon"
	"github.com/tfvjr/tap/internal/model"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose what's eating your machine",
	Long:  "Ranks projects and processes by resource usage, flags problems, and suggests fixes.",
	RunE:  doctorRun,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

type finding struct {
	Severity string // "warning" or "info"
	Message  string
	Action   string
}

func doctorRun(cmd *cobra.Command, args []string) error {
	snap, err := appStore.LatestSnapshot(true)
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}
	if snap == nil {
		fmt.Println("No data yet. The collector is still starting.")
		return nil
	}

	if jsonOutput {
		return doctorJSON(snap)
	}

	findings := diagnose(snap)

	// System overview
	memUsedGB := float64(snap.SystemMemoryUsed) / (1024 * 1024 * 1024)
	memTotalGB := float64(snap.SystemMemoryTotal) / (1024 * 1024 * 1024)
	memPct := 0.0
	if snap.SystemMemoryTotal > 0 {
		memPct = float64(snap.SystemMemoryUsed) / float64(snap.SystemMemoryTotal) * 100
	}

	fmt.Printf("System: CPU %.0f%%  MEM %.1f/%.1f GB (%.0f%%)\n\n", snap.SystemCPU, memUsedGB, memTotalGB, memPct)

	// Background Collector section
	collectorPID := daemon.CollectorPID(dataDir)
	if collectorPID > 0 {
		fmt.Printf("Background Collector: running (PID %d)\n", collectorPID)
	} else {
		fmt.Println("Background Collector: not running")
	}

	snapRows, sysRows, logRows, _ := appStore.RowCounts()
	fmt.Printf("Database: %d snapshots, %d system stats, %d log lines\n", snapRows, sysRows, logRows)

	// DB file size
	if fi, err := os.Stat(appStore.DBPath()); err == nil {
		sizeMB := float64(fi.Size()) / (1024 * 1024)
		fmt.Printf("DB size: %.1f MB (%s)\n", sizeMB, appStore.DBPath())
	}
	fmt.Println()

	// Resource ranking by project
	if len(snap.Projects) > 0 {
		fmt.Println("Resource usage by project:")
		fmt.Println()

		sorted := make([]model.Project, len(snap.Projects))
		copy(sorted, snap.Projects)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].TotalMemory > sorted[j].TotalMemory
		})

		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "  PROJECT\tSERVICES\tCPU\tMEMORY")
		for _, proj := range sorted {
			memMB := float64(proj.TotalMemory) / (1024 * 1024)
			fmt.Fprintf(w, "  %s\t%d\t%.1f%%\t%.0f MB\n",
				proj.Name, len(proj.Processes), proj.TotalCPU, memMB)
		}
		w.Flush()
		fmt.Println()
	}

	// Findings
	if len(findings) == 0 {
		fmt.Println("No issues found. Your dev environment looks healthy.")
		return nil
	}

	fmt.Printf("Found %d issue(s):\n\n", len(findings))
	for i, f := range findings {
		icon := "!"
		if f.Severity == "info" {
			icon = "*"
		}
		fmt.Printf("  %s %s\n", icon, f.Message)
		if f.Action != "" {
			fmt.Printf("    > %s\n", f.Action)
		}
		if i < len(findings)-1 {
			fmt.Println()
		}
	}
	fmt.Println()

	return nil
}

func diagnose(snap *model.Snapshot) []finding {
	var findings []finding

	// Check all processes for health flags.
	for _, proj := range snap.Projects {
		for _, proc := range proj.Processes {
			findings = append(findings, diagnoseProcess(&proc, proj.Name)...)
		}

		// Project-level: high total memory
		totalMB := float64(proj.TotalMemory) / (1024 * 1024)
		if totalMB > 2048 {
			findings = append(findings, finding{
				Severity: "warning",
				Message:  fmt.Sprintf("%s is using %.0f MB total across %d services", proj.Name, totalMB, len(proj.Processes)),
				Action:   fmt.Sprintf("tap project %s  -- review what's running", proj.Name),
			})
		}
	}

	for _, proc := range snap.Unattributed {
		findings = append(findings, diagnoseProcess(&proc, "unattributed")...)
	}

	// System-level checks
	if snap.SystemCPU > 80 {
		findings = append(findings, finding{
			Severity: "warning",
			Message:  fmt.Sprintf("System CPU is at %.0f%%", snap.SystemCPU),
			Action:   "tap ls  -- check which processes are consuming CPU",
		})
	}

	memPct := 0.0
	if snap.SystemMemoryTotal > 0 {
		memPct = float64(snap.SystemMemoryUsed) / float64(snap.SystemMemoryTotal) * 100
	}
	if memPct > 85 {
		findings = append(findings, finding{
			Severity: "warning",
			Message:  fmt.Sprintf("System memory is at %.0f%%", memPct),
			Action:   "tap ls  -- check which processes are consuming memory",
		})
	}

	// Sort: warnings first, then info
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Severity == findings[j].Severity {
			return false
		}
		return findings[i].Severity == "warning"
	})

	return findings
}

func diagnoseProcess(proc *model.DevProcess, project string) []finding {
	if proc.Health == nil || proc.Health.IsHealthy() {
		return nil
	}

	var findings []finding
	name := proc.Name
	if proc.PID != nil {
		name = fmt.Sprintf("%s (PID %d)", proc.Name, *proc.PID)
	}

	for _, flag := range proc.Health.Flags {
		switch flag {
		case model.HealthStale:
			f := finding{
				Severity: "warning",
				Message:  fmt.Sprintf("%s has been running for %s", name, formatUptime(proc.Uptime())),
			}
			if project != "unattributed" {
				f.Action = fmt.Sprintf("tap stop %s  -- or  tap kill %d", project, safeDerefPID(proc.PID))
			} else {
				f.Action = fmt.Sprintf("tap kill %d", safeDerefPID(proc.PID))
			}
			findings = append(findings, f)

		case model.HealthOrphan:
			parentDesc := "unknown parent"
			if len(proc.Health.ParentChain) > 0 {
				p := proc.Health.ParentChain[0]
				parentDesc = fmt.Sprintf("parent %s (PID %d) is dead", p.Name, p.PID)
			}
			findings = append(findings, finding{
				Severity: "warning",
				Message:  fmt.Sprintf("%s may be orphaned -- %s", name, parentDesc),
				Action:   fmt.Sprintf("tap kill %d", safeDerefPID(proc.PID)),
			})

		case model.HealthHighMem:
			findings = append(findings, finding{
				Severity: "warning",
				Message:  fmt.Sprintf("%s is using %.0f MB", name, proc.MemoryMB()),
				Action:   fmt.Sprintf("tap kill %d  -- or restart the service", safeDerefPID(proc.PID)),
			})

		case model.HealthHighCPU:
			findings = append(findings, finding{
				Severity: "warning",
				Message:  fmt.Sprintf("%s is using %.1f%% CPU", name, proc.CPUPercent),
				Action:   fmt.Sprintf("tap kill %d  -- or restart the service", safeDerefPID(proc.PID)),
			})
		}
	}

	return findings
}

func safeDerefPID(pid *int32) int32 {
	if pid == nil {
		return 0
	}
	return *pid
}

func doctorJSON(snap *model.Snapshot) error {
	type jsonFinding struct {
		Severity string `json:"severity"`
		Message  string `json:"message"`
		Action   string `json:"action,omitempty"`
	}
	type jsonReport struct {
		SystemCPU    float64       `json:"system_cpu"`
		SystemMemPct float64       `json:"system_memory_percent"`
		Projects     []model.Project `json:"projects"`
		Findings     []jsonFinding `json:"findings"`
	}

	findings := diagnose(snap)
	jf := make([]jsonFinding, len(findings))
	for i, f := range findings {
		jf[i] = jsonFinding{Severity: f.Severity, Message: f.Message, Action: f.Action}
	}

	memPct := 0.0
	if snap.SystemMemoryTotal > 0 {
		memPct = float64(snap.SystemMemoryUsed) / float64(snap.SystemMemoryTotal) * 100
	}

	report := jsonReport{
		SystemCPU:    snap.SystemCPU,
		SystemMemPct: memPct,
		Projects:     snap.Projects,
		Findings:     jf,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}
