package discovery

import (
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v3/process"
	"github.com/tap-dev/tap/internal/model"
)

const (
	staleThreshold  = 24 * time.Hour
	highMemBytes    = 1 << 30 // 1 GB
	highCPUPercent  = 50.0
	maxParentDepth  = 10
)

// AnalyzeHealth enriches each process in the snapshot with diagnostic health
// flags (stale, orphan, high memory, high CPU) and a parent chain trace.
func AnalyzeHealth(snap *model.Snapshot) {
	// Build a set of all known PIDs for orphan detection.
	alive := buildAlivePIDSet(snap)

	for i := range snap.Projects {
		for j := range snap.Projects[i].Processes {
			analyzeProcess(&snap.Projects[i].Processes[j], alive)
		}
	}
	for i := range snap.Unattributed {
		analyzeProcess(&snap.Unattributed[i], alive)
	}
}

// buildAlivePIDSet collects every PID in the snapshot into a set.
func buildAlivePIDSet(snap *model.Snapshot) map[int32]bool {
	pids := make(map[int32]bool)
	for _, proj := range snap.Projects {
		for _, proc := range proj.Processes {
			if proc.PID != nil {
				pids[*proc.PID] = true
			}
		}
	}
	for _, proc := range snap.Unattributed {
		if proc.PID != nil {
			pids[*proc.PID] = true
		}
	}
	return pids
}

func analyzeProcess(proc *model.DevProcess, alive map[int32]bool) {
	health := &model.ProcessHealth{}

	// Stale: running longer than 24 hours.
	if !proc.StartTime.IsZero() && time.Since(proc.StartTime) > staleThreshold {
		health.Flags = append(health.Flags, model.HealthStale)
	}

	// Orphan: parent PID is not in the snapshot's process table.
	// We also check the OS-level process table as a second opinion.
	if proc.PPID > 1 && !alive[proc.PPID] {
		if !processExistsOS(proc.PPID) {
			health.Flags = append(health.Flags, model.HealthOrphan)
		}
	}

	// High memory: > 1 GB RSS.
	if proc.MemoryBytes > highMemBytes {
		health.Flags = append(health.Flags, model.HealthHighMem)
	}

	// High CPU: > 50%.
	if proc.CPUPercent > highCPUPercent {
		health.Flags = append(health.Flags, model.HealthHighCPU)
	}

	// Walk the parent chain for the diagnostic detail view.
	health.ParentChain = walkParentChain(proc.PPID)

	proc.Health = health
}

// walkParentChain traces the parent process chain from the given PPID,
// checking whether each ancestor is still alive.
func walkParentChain(ppid int32) []model.ParentInfo {
	if ppid <= 1 {
		return nil
	}

	var chain []model.ParentInfo
	visited := make(map[int32]bool)
	pid := ppid

	for i := 0; i < maxParentDepth; i++ {
		if pid <= 1 || visited[pid] {
			break
		}
		visited[pid] = true

		info := model.ParentInfo{PID: pid}

		p, err := process.NewProcess(pid)
		if err != nil {
			// Process truly doesn't exist.
			info.Name = "unknown"
			info.Alive = false
			chain = append(chain, info)
			break
		}

		// Process exists. Name may be unreadable due to permissions
		// (common on Windows for system processes) — that's not death.
		info.Alive = true
		name, err := p.Name()
		if err != nil {
			info.Name = fmt.Sprintf("pid:%d", pid)
		} else {
			info.Name = name
		}

		chain = append(chain, info)

		nextPPID, err := p.Ppid()
		if err != nil || nextPPID <= 1 || nextPPID == pid {
			break
		}
		pid = nextPPID
	}

	return chain
}

// processExistsOS checks whether a PID is present in the OS process table.
// It does not require reading the process name, which may fail on Windows
// for system processes due to insufficient permissions.
func processExistsOS(pid int32) bool {
	exists, err := process.PidExists(pid)
	if err != nil {
		return false
	}
	return exists
}
