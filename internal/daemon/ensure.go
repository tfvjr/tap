package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v3/process"
)

// EnsureRunning checks whether the background collector is running and starts
// it if not. Returns true if the collector was just started (first run).
func EnsureRunning(dataDir string) (justStarted bool, err error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return false, fmt.Errorf("create data dir: %w", err)
	}

	pidFile := filepath.Join(dataDir, "tap.pid")

	// Check if already running.
	if data, err := os.ReadFile(pidFile); err == nil {
		pidStr := strings.TrimSpace(string(data))
		if pid, err := strconv.Atoi(pidStr); err == nil {
			exists, _ := process.PidExists(int32(pid))
			if exists {
				return false, nil
			}
		}
		// Stale PID file — remove it.
		os.Remove(pidFile)
	}

	// Start collector as a detached background process.
	exe, err := os.Executable()
	if err != nil {
		return false, fmt.Errorf("find executable: %w", err)
	}

	cmd := exec.Command(exe, "_collect", "--data-dir", dataDir)
	cmd.SysProcAttr = sysProcAttr()
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return false, fmt.Errorf("start collector: %w", err)
	}

	// Write PID file.
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(cmd.Process.Pid)), 0644); err != nil {
		return false, fmt.Errorf("write PID file: %w", err)
	}

	// Detach from child — don't wait for it.
	cmd.Process.Release()

	return true, nil
}

// CollectorPID returns the PID of the running collector, or 0 if not running.
func CollectorPID(dataDir string) int {
	pidFile := filepath.Join(dataDir, "tap.pid")
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	exists, _ := process.PidExists(int32(pid))
	if !exists {
		return 0
	}
	return pid
}
