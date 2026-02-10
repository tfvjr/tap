package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tap-dev/tap/internal/discovery"
	"github.com/tap-dev/tap/internal/model"
)

var exportCmd = &cobra.Command{
	Use:   "export [project]",
	Short: "Export a snapshot of running services",
	Long:  "Generate a human-readable snapshot of what's running, useful for sharing with teammates to debug \"works on my machine\" issues.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  exportRun,
}

func init() {
	rootCmd.AddCommand(exportCmd)
}

func exportRun(cmd *cobra.Command, args []string) error {
	snap, err := discovery.TakeSnapshot(!noDocker)
	if err != nil {
		return fmt.Errorf("snapshot: %w", err)
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(snap)
	}

	projects := snap.Projects
	if len(args) == 1 {
		name := args[0]
		found := false
		for _, p := range snap.Projects {
			if p.Name == name {
				projects = []model.Project{p}
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("project %q not found", name)
		}
	}

	for _, proj := range projects {
		fmt.Printf("Project: %s\n", proj.Name)
		fmt.Printf("Path: %s\n", proj.Path)
		fmt.Println("Services:")
		for _, proc := range proj.Processes {
			printExportService(&proc)
		}
		fmt.Println()
	}

	// System info
	usedGB := float64(snap.SystemMemoryUsed) / (1024 * 1024 * 1024)
	totalGB := float64(snap.SystemMemoryTotal) / (1024 * 1024 * 1024)
	fmt.Println("System:")
	fmt.Printf("  CPU: %.1f%%\n", snap.SystemCPU)
	fmt.Printf("  Memory: %.1f/%.1f GB\n", usedGB, totalGB)
	fmt.Println()

	// Toolchain detection
	toolchain := detectToolchain()
	if len(toolchain) > 0 {
		fmt.Println("Toolchain:")
		for _, t := range toolchain {
			fmt.Printf("  %s: %s\n", t.name, t.version)
		}
		fmt.Println()
	}

	// OS info
	fmt.Printf("OS: %s/%s\n", runtime.GOOS, runtime.GOARCH)

	return nil
}

func printExportService(p *model.DevProcess) {
	ports := formatPorts(p.Ports)
	if ports == "" {
		ports = "no ports"
	}

	if p.ContainerInfo != nil {
		fmt.Printf("  - %s on %s (docker: %s, %s)\n",
			p.Name, ports,
			p.ContainerInfo.ContainerName,
			p.ContainerInfo.Image,
		)
	} else {
		pid := "-"
		if p.PID != nil {
			pid = fmt.Sprintf("%d", *p.PID)
		}
		fmt.Printf("  - %s on %s (PID %s, %.1f%%, %.1f MB, running %s)\n",
			p.Name, ports,
			pid,
			p.CPUPercent,
			p.MemoryMB(),
			formatUptime(p.Uptime()),
		)
	}
}

type toolVersion struct {
	name    string
	version string
}

func detectToolchain() []toolVersion {
	var tools []toolVersion

	// Node
	if v := runToolVersion("node", "--version"); v != "" {
		tools = append(tools, toolVersion{"Node", v})
	}

	// Go
	if v := runToolVersion("go", "version"); v != "" {
		// "go version go1.23.4 windows/amd64" -> "go1.23.4"
		parts := strings.Fields(v)
		for _, p := range parts {
			if strings.HasPrefix(p, "go1") || strings.HasPrefix(p, "go2") {
				v = p
				break
			}
		}
		tools = append(tools, toolVersion{"Go", v})
	}

	// Python
	if v := runToolVersion("python3", "--version"); v != "" {
		tools = append(tools, toolVersion{"Python", strings.TrimPrefix(v, "Python ")})
	} else if v := runToolVersion("python", "--version"); v != "" {
		tools = append(tools, toolVersion{"Python", strings.TrimPrefix(v, "Python ")})
	}

	// Docker
	if v := runToolVersion("docker", "--version"); v != "" {
		// "Docker version 27.4.0, build ..." -> "27.4.0"
		v = strings.TrimPrefix(v, "Docker version ")
		if idx := strings.Index(v, ","); idx != -1 {
			v = v[:idx]
		}
		tools = append(tools, toolVersion{"Docker", v})
	}

	return tools
}

func runToolVersion(name string, arg string) string {
	out, err := exec.Command(name, arg).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
