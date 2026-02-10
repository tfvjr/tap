# Tap — Project Specification

## One-Liner

A CLI tool that replaces `lsof` + `ps` + `docker ps` + `netstat` with a single, project-aware command for managing everything running on your dev machine.

---

## Problem Statement

Developers constantly deal with:

- **"Port 3000 is already in use"** — by what? `lsof -i :3000` gives a PID but not *which project* started it or when.
- **Zombie processes** — you started a dev server 3 days ago and forgot. It's eating CPU and RAM.
- **Orphaned containers** — `docker ps` shows 8 containers but you have no idea which project they belong to or if they're still needed.
- **Port collision** — two projects both default to port 5432 for Postgres. Which one is running?
- **Resource drain** — your laptop fan is screaming but you don't know which dev process is responsible.
- **"Works on my machine" debugging** — teammates can't reproduce your setup because nobody knows what's actually running.

Today, developers cobble together `lsof`, `ps aux | grep`, `docker ps`, `netstat`, and Activity Monitor to answer these questions. There is no unified tool that understands the developer's world.

---

## Why Not Just Use lsof?

`lsof` tells you a PID is listening on port 3000. Tap tells you:

- **Which project** owns that port (your SaaS app, not your side project)
- **What else** that project is running (its database, cache, worker processes)
- **How much** CPU and memory the whole project stack is consuming
- **One command** to kill everything for that project, or clean up zombie processes you forgot about
- **Native processes and Docker containers** in the same view — `lsof` doesn't know about containers, `docker ps` doesn't know about native processes

The difference is context. `lsof` gives you raw data. Tap gives you answers.

---

## Product Vision

Tap is a single CLI command that acts as a **project-aware resource dashboard** for your development machine. It automatically discovers running processes, maps them to projects, shows which ports are bound, which containers are alive, and how much CPU/memory everything is consuming — all in one view.

Think: **`htop` meets `docker ps` meets a project-aware process manager.**

---

## Target User

- Full-stack developers working on 1-5 projects simultaneously
- Developers running microservices locally (multiple services per project)
- Anyone who has ever typed `kill -9` on a PID they found via `lsof` and hoped for the best

---

## Current State (v0.1 — Phase 1 Complete)

### What's Built

Phase 1 is implemented and working on Windows, macOS, and Linux:

- **Process discovery** — enumerates all running processes via gopsutil, collects PID, name, command line, CWD, CPU%, memory, uptime
- **Port mapping** — cross-platform port-to-PID mapping via gopsutil's net.Connections
- **Docker container discovery** — connects to the Docker daemon, lists containers with port mappings, images, names. Gracefully handles Docker being unavailable.
- **Project attribution** — walks each process's CWD up to find project markers (package.json, go.mod, Cargo.toml, etc.), groups processes by project root
- **Dev-process filtering** — hides system processes, only shows dev tools and processes with open ports
- **CLI commands**: `tap ls`, `tap ports`, `tap port <PORT>`, `tap kill <PID or :PORT>`
- **JSON output** — `--json` flag on all commands for scripting
- **Docker skip** — `--no-docker` flag for faster output when you don't need container info

### Known Limitations

- **Project attribution relies on CWD** — if a dev server changes its working directory after start, or is launched from a shell in a different directory, it won't be attributed to the right project. This is the biggest accuracy gap.
- **No process tree walking** — a `node` process spawned by `npm run dev` in project X might have its own CWD that doesn't match the project root. Walking the parent chain would improve attribution.
- **Docker container stats** — CPU and memory for containers are reported as 0 because the Docker stats API is a streaming call and too slow for a snapshot. Needs a caching layer.
- **No config file** — the ignore list and project directories are hardcoded.

---

## Core Features

### 1. Project Discovery & Registration

Tap automatically detects projects and will support manual registration.

**Auto-detection heuristics (implemented):**
- Walk the process tree and look for processes whose `cwd` is inside a directory containing project markers: `package.json`, `Cargo.toml`, `go.mod`, `pyproject.toml`, `Gemfile`, `.git`, `docker-compose.yml`, `docker-compose.yaml`, `Procfile`, `Makefile`, `.tap.toml`, `pom.xml`, `build.gradle`, `composer.json`, `mix.exs`, `CMakeLists.txt`
- Group processes by their project root directory
- Infer a project name from the `name` field in `package.json` or the directory basename

**Manual registration (planned):**
- `tap init` in a project directory to explicitly register it
- Creates a `.tap.toml` file or entry in `~/.config/tap/projects.toml`

### 2. Process Discovery & Port Mapping

**Core capability (implemented):** Enumerate all running processes on the machine that are relevant to development, and map them to projects and ports.

**What gets detected:**
- Processes listening on TCP ports (via gopsutil `net.Connections` — cross-platform)
- Docker/Podman containers and which ports they expose
- Known dev tool processes: node, python, ruby, go, java, cargo, rustc, webpack, vite, esbuild, postgres, mysql, redis, mongo, nginx, caddy, npm, yarn, pnpm, bun, deno, php, dotnet, uvicorn, gunicorn, gopls, and more
- Any process with an open listening port (even if not in the known list)

**Output per process:**
- PID
- Process name / command
- Port(s) bound
- Project it belongs to (or unattributed)
- CPU % and memory usage
- How long it's been running (uptime)
- Whether it's a container (and container name/ID/image if so)

### 3. Interactive TUI Dashboard (Phase 2 — Next)

The primary interface will be a terminal UI (TUI) launched via `tap` or `tap dash`.

**Layout:**
```
╔══════════════════════════════════════════════════════════════════╗
║  tap                                     CPU: 34%  MEM: 61%  ║
╠══════════════════════════════════════════════════════════════════╣
║                                                                  ║
║  ▼ my-saas-app           (3 processes, 2 containers)             ║
║    ● :3000  node (next dev)              CPU 12%  MEM 420MB  2h  ║
║    ● :5432  postgres (docker: pg-main)   CPU  1%  MEM 180MB  2h  ║
║    ● :6379  redis (docker: redis-cache)  CPU  0%  MEM  30MB  2h  ║
║                                                                  ║
║  ▼ side-project           (2 processes)                          ║
║    ● :8080  go (air)                     CPU  3%  MEM  90MB 45m  ║
║    ● :5433  postgres (docker: pg-side)   CPU  0%  MEM 150MB 45m  ║
║                                                                  ║
║  ▼ unattributed           (1 process)                            ║
║    ● :8443  node                         CPU  5%  MEM 200MB  3d  ║
║                                                                  ║
╠══════════════════════════════════════════════════════════════════╣
║  [k]ill  [r]estart  [s]top project  [a]ttribute  [f]ilter  [q]uit║
╚══════════════════════════════════════════════════════════════════╝
```

**Interactions:**
- Arrow keys / j/k to navigate
- `k` to kill a selected process (with confirmation)
- `s` to stop all processes for a selected project
- `r` to restart a selected process
- `a` to manually attribute an unattributed process to a project
- `f` to filter by project, port range, or resource usage
- `Enter` to expand a process and see full command, environment, network connections
- `?` for help

### 4. Quick CLI Commands (Non-Interactive)

```bash
# Show everything (table format, not TUI)
tap ls

# What's on port 3000?
tap port 3000
# Output: PID 12345 | node (next dev) | project: my-saas-app | running 2h | CPU 12%

# What's running for a specific project?
tap project my-saas-app

# Kill everything for a project
tap stop my-saas-app

# Kill whatever is on a specific port
tap kill :3000

# Show all ports in use
tap ports

# Show resource hogs
tap top

# Clean up — find and kill orphaned dev processes and containers
tap clean

# JSON output for scripting
tap ls --json
```

### 5. Container Awareness

Tap natively understands Docker and Podman:

- List running containers alongside native processes (implemented)
- Map container port bindings (e.g., container port 5432 mapped to host port 5433) (implemented)
- Show container names, images, and status (implemented)
- Allow stopping/removing containers through `tap kill` (implemented)
- Detect docker-compose projects and group their containers together (planned)
- Show stopped containers that are still taking up disk space (planned)

---

## Extended Features (v0.2+)

### 6. Project Profiles / Start Commands

Allow projects to define their full dev stack in `.tap.toml`:

```toml
[project]
name = "my-saas-app"

[[services]]
name = "api"
cmd = "npm run dev"
port = 3000
cwd = "./api"

[[services]]
name = "worker"
cmd = "npm run worker"
cwd = "./worker"

[[services]]
name = "postgres"
type = "docker"
image = "postgres:16"
port = 5432
env = { POSTGRES_PASSWORD = "dev" }

[[services]]
name = "redis"
type = "docker"
image = "redis:7"
port = 6379
```

Then: `tap up` starts the entire stack. `tap down` stops it cleanly.

Key differences from docker-compose / Procfile:
- Native processes and containers are managed together
- Port conflict detection before starting
- Resource monitoring built in
- Cross-project awareness (won't start if another project is using port 5432)

### 7. Port Conflict Prevention

Before starting a service, check if the port is already in use and tell the user exactly what's occupying it:

```
$ tap up
⚠ Port 5432 is already in use by:
  postgres (docker: pg-side) — project: side-project — running 45m

  Options:
  [1] Stop the conflicting process and continue
  [2] Use alternative port 5433
  [3] Abort
```

### 8. Process Notifications / Watchdog

Optional background daemon (`tap watch`) that:
- Alerts when a dev process crashes
- Alerts when a process has been running for more than N hours (configurable)
- Alerts when total dev process memory exceeds a threshold
- Sends notifications via system notifications

### 9. Team Sharing

`tap export` generates a snapshot of what's running that can be shared:
```
$ tap export
Project: my-saas-app
Services:
  - node (next dev) on :3000
  - postgres:16 on :5432
  - redis:7 on :6379
Node: v20.11.0
Docker: 24.0.7
OS: macOS 14.2 arm64
```

---

## Technical Architecture

### Language

**Go** — for the following reasons:
- Fast startup time (~10-20ms, feels instant for a CLI)
- Single static binary with zero dependencies — trivial cross-compilation
- The container ecosystem (Docker, Kubernetes, containerd) is written in Go — Docker integration is most natural here
- Excellent TUI ecosystem: Charm's `bubbletea` + `lipgloss` + `bubbles`
- `gopsutil` provides robust cross-platform process/CPU/memory enumeration
- Strong concurrency model (goroutines) for parallel process/container/port scanning

### Key Dependencies

| Concern | Package | Notes |
|---------|---------|-------|
| Process enumeration | `github.com/shirou/gopsutil/v3` | Cross-platform process/CPU/memory/network info |
| Docker API | `github.com/docker/docker` | Official Docker Engine API client (v28.x) |
| CLI framework | `github.com/spf13/cobra` | Industry standard Go CLI framework |
| Table output | `text/tabwriter` (stdlib) | Tab-aligned table formatting |
| Config files | `github.com/BurntSushi/toml` | TOML parsing for .tap.toml and global config |
| Colored output | `github.com/fatih/color` | Colored terminal output for non-TUI commands |
| TUI framework | `github.com/charmbracelet/bubbletea` | For Phase 2 TUI dashboard |
| TUI styling | `github.com/charmbracelet/lipgloss` | For Phase 2 TUI dashboard |

### Platform Support

**All primary targets:**
- Windows (native, not just WSL2)
- macOS (Apple Silicon + Intel)
- Linux (x86_64 + arm64)

Process and port discovery use `gopsutil` which abstracts platform differences. Docker discovery uses the Docker Engine API which works the same everywhere.

**Windows-specific notes:**
- Path normalization handles case-insensitive filesystem (paths are lowercased for deduplication, originals preserved for display)
- Process enumeration via WMI (handled by gopsutil)
- Port mapping via Windows API (handled by gopsutil)

### Data Flow

```
┌──────────────────┐
│   Process Table   │  (gopsutil — PIDs, names, CPU, memory, cwd)
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Port Mapper      │  (gopsutil net.Connections — which PIDs are listening)
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Dev Filter       │  (keep only dev-relevant processes)
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Docker Client    │  (docker/client — container list, port mappings)
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Project Mapper   │  (maps PIDs to project directories via cwd + markers)
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Snapshot         │  (unified model with system stats)
└────────┬─────────┘
         │
         ├──► CLI Table Output (text/tabwriter)
         ├──► JSON Output (encoding/json)
         └──► TUI Dashboard (bubbletea — Phase 2)
```

### Refresh Rate

- TUI dashboard: refresh every 2 seconds (configurable)
- Non-interactive commands: single snapshot, then exit
- Watchdog daemon: configurable poll interval (default 30 seconds)

---

## File Structure

```
tap/
├── go.mod
├── go.sum
├── main.go                         # Entry point
├── cmd/
│   ├── root.go                     # Root cobra command + global flags
│   ├── ls.go                       # tap ls (+ default command)
│   ├── ports.go                    # tap ports
│   ├── port.go                     # tap port <PORT>
│   ├── kill.go                     # tap kill <TARGET>
│   ├── stop.go                     # tap stop <project>          [planned]
│   ├── clean.go                    # tap clean                   [planned]
│   ├── init.go                     # tap init                    [planned]
│   └── export.go                   # tap export                  [planned]
├── internal/
│   ├── discovery/
│   │   ├── processes.go            # Process enumeration via gopsutil
│   │   ├── ports.go                # Port-to-PID mapping via gopsutil
│   │   ├── docker.go               # Docker container discovery
│   │   ├── projects.go             # Project detection + attribution
│   │   └── snapshot.go             # Aggregator + dev filter + system stats
│   ├── model/
│   │   ├── process.go              # DevProcess, PortBinding, ContainerInfo
│   │   ├── project.go              # Project
│   │   └── snapshot.go             # Snapshot
│   └── tui/                        # [Phase 2]
│       ├── app.go
│       ├── keys.go
│       └── styles.go
├── tap-spec.md
├── README.md
├── LICENSE
└── Makefile                        # [planned]
```

---

## Data Model

```go
type DevProcess struct {
    PID           *int32         `json:"pid,omitempty"`
    Name          string         `json:"name"`
    Command       string         `json:"command"`
    Ports         []PortBinding  `json:"ports"`
    Project       string         `json:"project,omitempty"`
    ProjectPath   string         `json:"project_path,omitempty"`
    CPUPercent    float64        `json:"cpu_percent"`
    MemoryBytes   uint64         `json:"memory_bytes"`
    StartTime     time.Time      `json:"start_time"`
    Kind          ProcessKind    `json:"kind"`
    ContainerInfo *ContainerInfo `json:"container_info,omitempty"`
}

type PortBinding struct {
    Port     uint16 `json:"port"`
    Protocol string `json:"protocol"`
    Address  string `json:"address"`
}

type ContainerInfo struct {
    ContainerID   string        `json:"container_id"`
    ContainerName string        `json:"container_name"`
    Image         string        `json:"image"`
    Status        string        `json:"status"`
    PortMappings  []PortMapping `json:"port_mappings"`
}

type PortMapping struct {
    HostPort      uint16 `json:"host_port"`
    ContainerPort uint16 `json:"container_port"`
}

type ProcessKind string

const (
    ProcessNative ProcessKind = "native"
    ProcessDocker ProcessKind = "docker"
    ProcessPodman ProcessKind = "podman"
)

type Project struct {
    Name        string       `json:"name"`
    Path        string       `json:"path"`
    Processes   []DevProcess `json:"processes"`
    TotalCPU    float64      `json:"total_cpu"`
    TotalMemory uint64       `json:"total_memory"`
}

type Snapshot struct {
    Timestamp         time.Time    `json:"timestamp"`
    Projects          []Project    `json:"projects"`
    Unattributed      []DevProcess `json:"unattributed"`
    SystemCPU         float64      `json:"system_cpu"`
    SystemMemoryTotal uint64       `json:"system_memory_total"`
    SystemMemoryUsed  uint64       `json:"system_memory_used"`
}
```

---

## CLI Interface

```
tap — Dev machine resource manager

USAGE:
    tap [COMMAND]

COMMANDS:
    (no command)    List all dev processes (same as tap ls)
    ls              List all dev processes (table format)
    ports           List all ports in use by dev processes
    port <PORT>     Show what's running on a specific port
    kill <TARGET>   Kill a process by PID or :PORT
    project <NAME>  Show processes for a specific project       [planned]
    stop <NAME>     Stop all processes for a project            [planned]
    clean           Find and remove orphaned processes           [planned]
    init            Register current directory as a project      [planned]
    export          Export current state for sharing             [planned]
    dash            Launch interactive TUI dashboard             [Phase 2]
    up              Start project stack (requires .tap.toml)    [v0.2]
    down            Stop project stack                          [v0.2]
    watch           Start background watchdog daemon            [v0.2]

OPTIONS:
    --json          Output as JSON (for ls, ports, port commands)
    --no-docker     Skip Docker/Podman discovery
    --verbose       Show full command lines and additional details
    -h, --help      Show help
    -V, --version   Show version
```

---

## Configuration

Global config at `~/.config/tap/config.toml` (planned):

```toml
# Directories to scan for projects (used for attribution heuristics)
project_dirs = ["~/code", "~/work", "~/projects"]

# Processes to always ignore
ignore_processes = ["Spotlight", "mds_stores", "WindowServer"]

# TUI refresh interval in seconds
refresh_interval = 2

# Port ranges to monitor (default: all)
# port_range = [1024, 65535]

# Docker socket path (auto-detected if not set)
# docker_socket = "/var/run/docker.sock"

# Watchdog settings
[watchdog]
max_uptime_hours = 24
max_memory_mb = 4096
notify = true
```

---

## Build & Distribution

### Build
```bash
# Development build
go build -o tap .

# Release build (smaller binary)
go build -ldflags="-s -w" -o tap .
```

### Installation methods (in order of priority)
1. **Go install**: `go install github.com/tap-dev/tap@latest`
2. **GitHub releases**: Prebuilt binaries for Windows, macOS, and Linux
3. **Homebrew** (macOS + Linux): `brew install tap-dev/tap/tap`
4. **Scoop** (Windows): `scoop install tap`

### CI/CD
- GitHub Actions
- Build matrix: Windows (x86_64) + macOS (arm64 + x86_64) + Linux (x86_64 + arm64)
- Run tests on each platform
- GoReleaser for automated cross-compilation and release packaging

---

## Success Metrics

- `tap ls` renders a full snapshot in **< 500ms** (currently met)
- Binary size **< 10MB** (currently 7.6MB stripped)
- Memory footprint of tap itself **< 20MB** (currently ~15MB)
- Port-to-project attribution is **> 80% accurate** for projects where the dev server CWD matches the project root

---

## Roadmap

### Phase 1 — Core CLI (Done)
1. ~~Process discovery + port mapping (Windows + macOS + Linux)~~
2. ~~Project attribution via CWD heuristics~~
3. ~~`tap ls` — non-interactive table output~~
4. ~~`tap port <PORT>` — single port lookup~~
5. ~~`tap ports` — all ports table~~
6. ~~`tap kill <target>` — kill by PID or port~~
7. ~~Basic Docker container discovery~~
8. ~~JSON output mode~~
9. ~~Dev-process filtering~~

### Phase 2 — TUI Dashboard
10. Interactive TUI dashboard with bubbletea + lipgloss
11. Process grouping by project with expand/collapse
12. Keyboard navigation and actions (kill, stop project)
13. Real-time refresh (2-second polling)
14. Process detail view on Enter

### Phase 3 — Quality of Life
15. `tap stop <project>` — stop all project processes
16. `tap clean` — orphan cleanup (long-running processes, stopped containers)
17. `tap init` — manual project registration
18. `tap export` — state export for sharing
19. Global config file (`~/.config/tap/config.toml`)
20. Process tree walking for better project attribution
21. Configurable ignore lists

### Phase 4 — Stack Management (v0.2)
22. `.tap.toml` project profiles
23. `tap up` / `tap down` — project stack management
24. Port conflict prevention and resolution
25. Background watchdog daemon with desktop notifications

---

## Non-Goals

- **Not a process supervisor** — Tap monitors and manages, but it doesn't keep processes alive. It's not systemd or PM2.
- **Not a container orchestrator** — it shows you what Docker is doing, but it doesn't replace docker-compose for complex service topologies.
- **Not a system monitor** — it only cares about dev-related processes. It's not htop.
- **Not an IDE plugin** — it's a standalone CLI/TUI tool.

---

## Competitive Landscape

| Tool | What it does | How Tap differs |
|------|-------------|---------------------|
| `lsof -i` | Lists open ports/files | No project awareness, raw output, no filtering |
| `htop` / `btop` | System process monitor | Shows everything, no project grouping, no Docker |
| `docker ps` | Lists containers | Only containers, no native processes |
| `lazydocker` | Docker TUI | Docker-only, no native process awareness |
| Overmind / Foreman | Process manager | Only manages processes it started, no system-wide view |
| Portmaster | Network monitor | Security-focused, not dev-focused |

**Tap's unique value: it's the only tool that gives you a unified, project-organized view of everything running on your dev machine — native processes and containers together — and lets you act on it.**
