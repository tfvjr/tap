package discovery

import (
	"fmt"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/tap-dev/tap/internal/model"
)

// devProcessNames lists known developer tool process names (lowercase, without
// extension). A process whose name matches one of these is considered
// dev-relevant even if it has no open ports and no project attribution.
var devProcessNames = map[string]bool{
	"node": true, "node.exe": true,
	"python": true, "python.exe": true, "python3": true, "python3.exe": true,
	"ruby": true, "ruby.exe": true,
	"go": true, "go.exe": true,
	"java": true, "java.exe": true, "javac": true, "javac.exe": true,
	"cargo": true, "cargo.exe": true, "rustc": true, "rustc.exe": true,
	"webpack": true, "vite": true, "esbuild": true,
	"postgres": true, "postgres.exe": true, "pg_isready": true,
	"mysqld": true, "mysqld.exe": true, "mysql": true,
	"redis-server": true, "redis-server.exe": true,
	"mongod": true, "mongod.exe": true, "mongos": true,
	"nginx": true, "nginx.exe": true, "caddy": true, "caddy.exe": true,
	"npm": true, "npm.exe": true, "npx": true, "npx.exe": true,
	"yarn": true, "yarn.exe": true, "pnpm": true, "pnpm.exe": true,
	"bun": true, "bun.exe": true, "deno": true, "deno.exe": true,
	"php": true, "php.exe": true, "dotnet": true, "dotnet.exe": true,
	"uvicorn": true, "gunicorn": true, "flask": true, "django": true,
	"next": true, "nuxt": true, "hugo": true, "hugo.exe": true,
	"jekyll": true, "air": true, "air.exe": true,
	"nodemon": true, "ts-node": true, "tsx": true,
	"docker-compose": true, "podman": true,
	"gopls": true, "gopls.exe": true,
}

// isDevProcess returns true if the process is considered dev-relevant:
// it has open ports, is a container, or has a known dev tool name.
func isDevProcess(p *model.DevProcess) bool {
	if len(p.Ports) > 0 {
		return true
	}
	if p.Kind != model.ProcessNative {
		return true
	}
	name := strings.ToLower(p.Name)
	return devProcessNames[name]
}

// filterDevProcesses returns only dev-relevant processes from the input.
func filterDevProcesses(procs []model.DevProcess) []model.DevProcess {
	filtered := make([]model.DevProcess, 0, len(procs)/4)
	for i := range procs {
		if isDevProcess(&procs[i]) {
			filtered = append(filtered, procs[i])
		}
	}
	return filtered
}

// TakeSnapshot captures the full system state by discovering processes, ports,
// and (optionally) containers, attributing them to projects, and collecting
// system-level resource statistics. The returned Snapshot is a point-in-time
// view suitable for display or serialisation.
func TakeSnapshot(includeDocker bool) (*model.Snapshot, error) {
	// 1. Discover all native (OS-level) processes.
	processes, err := DiscoverProcesses()
	if err != nil {
		return nil, fmt.Errorf("discover processes: %w", err)
	}

	// 2. Build a PID-to-ports mapping.
	portMap, err := DiscoverPorts()
	if err != nil {
		return nil, fmt.Errorf("discover ports: %w", err)
	}

	// 3. Merge port bindings into the corresponding processes.
	for i := range processes {
		if processes[i].PID == nil {
			continue
		}
		if ports, ok := portMap[*processes[i].PID]; ok {
			processes[i].Ports = ports
		}
	}

	// 4. Optionally discover Docker containers and append them.
	if includeDocker {
		containers, err := DiscoverContainers()
		if err != nil {
			// Container discovery is best-effort; log but do not abort.
			// In a future iteration this could be surfaced as a warning.
			_ = err
		} else {
			processes = append(processes, containers...)
		}
	}

	// 5. Filter to dev-relevant processes only.
	processes = filterDevProcesses(processes)

	// 6. Attribute processes to projects and separate the unattributed ones.
	projectMap, unattributed := AttributeProcesses(processes)

	projects := make([]model.Project, 0, len(projectMap))
	for _, proj := range projectMap {
		projects = append(projects, *proj)
	}

	// 6. Collect system-level resource statistics.
	systemCPU := 0.0
	cpuPercents, err := cpu.Percent(0, false)
	if err == nil && len(cpuPercents) > 0 {
		systemCPU = cpuPercents[0]
	}

	var memTotal, memUsed uint64
	vmStat, err := mem.VirtualMemory()
	if err == nil && vmStat != nil {
		memTotal = vmStat.Total
		memUsed = vmStat.Used
	}

	// 7. Assemble and return the snapshot.
	snapshot := &model.Snapshot{
		Timestamp:         time.Now(),
		Projects:          projects,
		Unattributed:      unattributed,
		SystemCPU:         systemCPU,
		SystemMemoryTotal: memTotal,
		SystemMemoryUsed:  memUsed,
	}

	return snapshot, nil
}
