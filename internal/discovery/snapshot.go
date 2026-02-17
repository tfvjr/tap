package discovery

import (
	"fmt"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/tfvjr/tap/internal/model"
)

// devProcessNames lists known developer tool process names (lowercase, without
// extension). A process whose name matches one of these is considered
// dev-relevant even if it has no open ports and no project attribution.
var devProcessNames = map[string]bool{
	// JavaScript / TypeScript runtimes
	"node": true, "node.exe": true,
	"bun": true, "bun.exe": true,
	"deno": true, "deno.exe": true,

	// JS package managers & tools
	"npm": true, "npm.exe": true, "npx": true, "npx.exe": true,
	"yarn": true, "yarn.exe": true, "pnpm": true, "pnpm.exe": true,
	"nodemon": true, "ts-node": true, "tsx": true,

	// JS build tools
	"webpack": true, "vite": true, "esbuild": true,
	"next": true, "nuxt": true, "turbo": true, "turbo.exe": true,

	// Python runtimes & tools
	"python": true, "python.exe": true, "python3": true, "python3.exe": true,
	"uv": true, "uv.exe": true,
	"uvicorn": true, "gunicorn": true, "flask": true, "django": true,
	"pipenv": true, "poetry": true, "poetry.exe": true,
	"celery": true, "pytest": true, "pytest.exe": true,

	// Ruby
	"ruby": true, "ruby.exe": true,
	"bundler": true, "rails": true, "puma": true, "sidekiq": true,
	"jekyll": true,

	// Go
	"go": true, "go.exe": true,
	"gopls": true, "gopls.exe": true,
	"air": true, "air.exe": true,

	// Rust
	"cargo": true, "cargo.exe": true, "rustc": true, "rustc.exe": true,

	// Java / JVM
	"java": true, "java.exe": true, "javac": true, "javac.exe": true,
	"gradle": true, "gradle.exe": true, "mvn": true, "mvn.exe": true,

	// PHP
	"php": true, "php.exe": true, "composer": true, "artisan": true,

	// .NET
	"dotnet": true, "dotnet.exe": true,

	// Elixir
	"elixir": true, "mix": true, "iex": true,

	// Databases
	"postgres": true, "postgres.exe": true, "pg_isready": true,
	"mysqld": true, "mysqld.exe": true, "mysql": true,
	"redis-server": true, "redis-server.exe": true,
	"mongod": true, "mongod.exe": true, "mongos": true,

	// Web servers / reverse proxies
	"nginx": true, "nginx.exe": true, "caddy": true, "caddy.exe": true,

	// Static site generators
	"hugo": true, "hugo.exe": true,

	// Container tools
	"docker-compose": true, "podman": true,
}

// isKnownDevName reports whether the process name matches a known dev tool.
func isKnownDevName(p *model.DevProcess) bool {
	return devProcessNames[strings.ToLower(p.Name)]
}

// IsKnownDevName reports whether the given name matches a known dev tool.
// Exported for use by the store package's curated filtering.
func IsKnownDevName(name string) bool {
	return devProcessNames[strings.ToLower(name)]
}

// isDevCandidate returns true if a process might be dev-relevant (first pass).
// Keeps known dev names, containers, and anything with open ports.
func isDevCandidate(p *model.DevProcess) bool {
	if p.Kind != model.ProcessNative {
		return true
	}
	if isKnownDevName(p) {
		return true
	}
	return len(p.Ports) > 0
}

// filterDevCandidates is the first-pass filter run before attribution.
func filterDevCandidates(procs []model.DevProcess) []model.DevProcess {
	filtered := make([]model.DevProcess, 0, len(procs)/4)
	for i := range procs {
		if isDevCandidate(&procs[i]) {
			filtered = append(filtered, procs[i])
		}
	}
	return filtered
}

// filterUnattributed is the second-pass filter run after attribution.
// It removes unattributed processes that only qualified because they had
// open ports but are not known dev tools and not containers.
func filterUnattributed(procs []model.DevProcess) []model.DevProcess {
	filtered := make([]model.DevProcess, 0, len(procs))
	for i := range procs {
		if procs[i].Kind != model.ProcessNative || isKnownDevName(&procs[i]) {
			filtered = append(filtered, procs[i])
		}
	}
	return filtered
}

// TakeSnapshot captures the full system state by discovering processes, ports,
// and (optionally) containers, attributing them to projects, and collecting
// system-level resource statistics. When curated is true, the unattributed
// list is filtered to only known dev tools (browsing mode). When false, all
// port-listening processes are kept (search mode for tap port).
func TakeSnapshot(includeDocker bool, curated bool) (*model.Snapshot, error) {
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

	// 5. First-pass filter: keep known dev names, containers, and port listeners.
	processes = filterDevCandidates(processes)

	// 6. Attribute processes to projects and separate the unattributed ones.
	projectMap, unattributed := AttributeProcesses(processes)

	// 7. Second-pass filter (curated mode): remove unattributed processes that
	// only qualified because they had ports but aren't known dev tools.
	if curated {
		unattributed = filterUnattributed(unattributed)
	}

	projects := make([]model.Project, 0, len(projectMap))
	for _, proj := range projectMap {
		projects = append(projects, *proj)
	}

	// 8. Collect system-level resource statistics.
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

	// 9. Assemble and return the snapshot.
	snapshot := &model.Snapshot{
		Timestamp:         time.Now(),
		Projects:          projects,
		Unattributed:      unattributed,
		SystemCPU:         systemCPU,
		SystemMemoryTotal: memTotal,
		SystemMemoryUsed:  memUsed,
	}

	// 10. Run health analysis (stale, orphan, resource warnings, parent chains).
	AnalyzeHealth(snapshot)

	return snapshot, nil
}
