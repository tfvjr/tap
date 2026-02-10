package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tap-dev/tap/internal/discovery"
	"github.com/tap-dev/tap/internal/model"
)

var killCmd = &cobra.Command{
	Use:   "kill <pid or :port>",
	Short: "Kill a process by PID or by :port",
	Long:  "Stop a process or container. Pass a numeric PID to kill directly, or :port to find and kill whatever is listening on that port.",
	Args:  cobra.ExactArgs(1),
	RunE:  killRun,
}

func init() {
	rootCmd.AddCommand(killCmd)
}

func killRun(cmd *cobra.Command, args []string) error {
	target := strings.TrimSpace(args[0])

	if strings.HasPrefix(target, ":") {
		return killByPort(target[1:])
	}
	return killByPID(target)
}

// killByPort finds the process listening on the given port and kills it.
func killByPort(portStr string) error {
	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return fmt.Errorf("invalid port number %q: %w", portStr, err)
	}

	snap, err := discovery.TakeSnapshot(!noDocker)
	if err != nil {
		return fmt.Errorf("taking snapshot: %w", err)
	}

	// Collect every process from the snapshot (projects + unattributed).
	allProcs := collectAllProcesses(snap)

	for _, proc := range allProcs {
		if !processListensOnPort(proc, uint16(port)) {
			continue
		}

		// Found a match — decide how to kill it.
		switch proc.Kind {
		case model.ProcessDocker, model.ProcessPodman:
			return killContainer(proc, uint16(port))
		default:
			return killNativeProcess(proc, uint16(port))
		}
	}

	return fmt.Errorf("no process found listening on port :%d", port)
}

// killByPID kills the process with the given PID.
func killByPID(pidStr string) error {
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return fmt.Errorf("invalid PID %q: %w", pidStr, err)
	}

	p, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("finding process %d: %w", pid, err)
	}

	if err := p.Kill(); err != nil {
		return fmt.Errorf("killing process %d: %w", pid, err)
	}

	fmt.Fprintf(os.Stdout, "Killed process %d\n", pid)
	return nil
}

// killNativeProcess kills a native OS process referenced by a DevProcess.
func killNativeProcess(proc model.DevProcess, port uint16) error {
	if proc.PID == nil {
		return fmt.Errorf("process %q on port :%d has no PID", proc.Name, port)
	}

	pid := int(*proc.PID)
	p, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("finding process %d: %w", pid, err)
	}

	if err := p.Kill(); err != nil {
		return fmt.Errorf("killing process %d (%s): %w", pid, proc.Name, err)
	}

	fmt.Fprintf(os.Stdout, "Killed process %d (%s) on port :%d\n", pid, proc.Name, port)
	return nil
}

// killContainer stops a Docker/Podman container.
func killContainer(proc model.DevProcess, port uint16) error {
	if proc.ContainerInfo == nil || proc.ContainerInfo.ContainerID == "" {
		return fmt.Errorf("container %q on port :%d has no container ID", proc.Name, port)
	}

	containerID := proc.ContainerInfo.ContainerID

	runtime := "docker"
	if proc.Kind == model.ProcessPodman {
		runtime = "podman"
	}

	out, err := exec.Command(runtime, "stop", containerID).CombinedOutput()
	if err != nil {
		return fmt.Errorf("stopping container %s: %s: %w", containerID, strings.TrimSpace(string(out)), err)
	}

	fmt.Fprintf(os.Stdout, "Killed container %s (%s) on port :%d\n", containerID[:12], proc.Name, port)
	return nil
}

// collectAllProcesses gathers every DevProcess from the snapshot.
func collectAllProcesses(snap *model.Snapshot) []model.DevProcess {
	var all []model.DevProcess
	for _, proj := range snap.Projects {
		all = append(all, proj.Processes...)
	}
	all = append(all, snap.Unattributed...)
	return all
}

// processListensOnPort reports whether the process is listening on the given port.
func processListensOnPort(proc model.DevProcess, port uint16) bool {
	for _, pb := range proc.Ports {
		if pb.Port == port {
			return true
		}
	}
	// Also check container port mappings (host-side port).
	if proc.ContainerInfo != nil {
		for _, pm := range proc.ContainerInfo.PortMappings {
			if pm.HostPort == port {
				return true
			}
		}
	}
	return false
}
