package discovery

import (
	"time"

	"github.com/shirou/gopsutil/v3/process"
	"github.com/tap-dev/tap/internal/model"
)

// DiscoverProcesses enumerates all running processes on the system and returns
// them as a slice of model.DevProcess. Processes that cannot be inspected (for
// example due to insufficient permissions) are silently skipped. Individual
// field-level errors (CPU, memory, etc.) are handled gracefully by falling back
// to zero values rather than aborting.
func DiscoverProcesses() ([]model.DevProcess, error) {
	procs, err := process.Processes()
	if err != nil {
		return nil, err
	}

	result := make([]model.DevProcess, 0, len(procs))

	for _, p := range procs {
		// Name is the minimum requirement — skip the process if we cannot
		// read it (usually EPERM / access denied).
		name, err := p.Name()
		if err != nil {
			continue
		}

		// Full command line (e.g. "node server.js --port 3000").
		cmdline, _ := p.Cmdline()

		// Working directory of the process.
		cwd, _ := p.Cwd()

		// CPU usage as a percentage (0-100+). A single-sample call may
		// return 0 on the first invocation; callers that need accurate
		// readings should sample twice with a short interval.
		cpuPct, _ := p.CPUPercent()

		// Resident set size (RSS) in bytes.
		var memBytes uint64
		memInfo, err := p.MemoryInfo()
		if err == nil && memInfo != nil {
			memBytes = memInfo.RSS
		}

		// Process creation time as milliseconds since epoch.
		var startTime time.Time
		createMs, err := p.CreateTime()
		if err == nil {
			startTime = time.UnixMilli(createMs)
		}

		pid := p.Pid

		var ppid int32
		ppidVal, err := p.Ppid()
		if err == nil {
			ppid = ppidVal
		}

		dp := model.DevProcess{
			PID:         &pid,
			PPID:        ppid,
			Name:        name,
			Command:     cmdline,
			Ports:       nil, // Port mapping is handled separately.
			CPUPercent:  cpuPct,
			MemoryBytes: memBytes,
			StartTime:   startTime,
			Kind:        model.ProcessNative,
			Project:     "",
			ProjectPath: cwd,
		}

		result = append(result, dp)
	}

	return result, nil
}
