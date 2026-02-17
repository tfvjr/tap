# Tap — Project Specification

## One-Liner

A persistent dev process observatory — background collector, SQLite history, console capture, MCP server, and a TUI that tells you what's running, what's broken, and what you forgot about.

---

## Problem Statement

Every developer hits "port 3000 is already in use" a few times a week. The current workflow is:

```bash
lsof -i :3000           # → PID 8821 (what is that?)
ps aux | grep 8821      # → node (which node? I have 3 projects)
kill -9 8821            # → hope for the best
```

This is 3 commands, no context, and no confidence. You don't know:
- **Which project** started that process
- **Whether it's a zombie** (parent died, process is orphaned)
- **What else** that project is running (its database, its cache, its worker)
- **How long** it's been sitting there eating RAM
- **Whether killing it is safe** or if it'll take down something else

Tap replaces this with one command that gives you the full diagnostic picture and lets you act on it.

---

## What Makes Tap Different

There are several tools in this space. Here's how tap fits:

| Tool | Stars | What it does | What it doesn't do |
|------|-------|--------------|--------------------|
| [fkill](https://github.com/sindresorhus/fkill-cli) | 8k | Interactive process killing | No project awareness, no diagnostics |
| [port-kill](https://github.com/treadiehq/port-kill) | 1.9k | Port killing + service orchestration + YAML config | Port-centric, not process-tree aware, Node/Rust |
| [portfinder](https://github.com/doganarif/portfinder) | 34 | Go, project awareness, TUI | Minimal traction, basic implementation |
| [portik](https://github.com/pratik-anurag/portik) | new | "Why is this port stuck" diagnostics | No project grouping, no TUI dashboard |
| `lsof` / `netstat` | — | Raw port/PID lookup | Zero context, zero DX |

**Tap's angle:** Not just "what's on port 3000" but "here's the full story — the process, its parent chain, the project, the resources, and whether it's healthy or a zombie." Plus persistent history, console capture, and AI tool integration via MCP.

---

## Target User

- Full-stack developers running 2-5 projects simultaneously
- Developers who hit port conflicts weekly and want a faster diagnostic workflow
- Teams using Claude Code or other AI tools that benefit from real-time dev environment visibility

---

## Architecture

### Background Collector

The collector is the single source of truth. It's the only component that calls the discovery engine.

- Auto-starts on any `tap` command via `PersistentPreRunE`
- Runs as a detached background process (`tap _collect`, hidden command)
- Polls every 5 seconds: `discovery.TakeSnapshot()` → SQLite
- PID file at `~/.tap/collector.pid` with liveness checks via `gopsutil`
- Prunes data older than 24 hours on each cycle
- Platform-specific detachment: `DETACHED_PROCESS` flag on Windows, `Setsid` on Unix

### Storage Layer

SQLite database at `~/.tap/tap.db` with WAL mode for concurrent reads.

**Tables:**
- `snapshots` — process snapshots (pid, name, command, ports, project, cpu, memory, etc.)
- `system_stats` — system-level CPU/memory per snapshot
- `logs` — captured console output (timestamp, project, command, stream, line)
- `meta` — key-value metadata (collector state, etc.)

**Design decisions:**
- Store unfiltered, filter on read (curated vs unfiltered is a query-time concern)
- Pure Go SQLite driver (`modernc.org/sqlite`) — no CGo, works on Windows
- All commands read from the DB, never from live discovery

### Console Capture (PATH Shimming)

Tap uses PATH shimming (same pattern as rbenv, pyenv, nvm) for transparent console capture. On first run, lightweight wrapper scripts for common dev tools (`node`, `npm`, `python`, `go`, `cargo`, etc.) are placed in `~/.tap/shims/` and prepended to PATH via shell config modification.

When a user runs e.g. `npm start`, the shim intercepts the call and invokes `tap _shim npm start`. The `_shim` command:
1. Opens SQLite directly (skips daemon check — ~5ms overhead)
2. Finds the real `npm` binary (searches PATH minus `~/.tap/shims/`)
3. Spawns the real binary with piped stdout/stderr
4. Tees output: terminal + `store.InsertLogLine()`
5. Forwards signals, returns the child's exit code

If anything fails (store error, resolve error), `_shim` falls back to running the real binary directly without capture — the command must never break.

`tap teardown` removes shims and restores shell config.

### MCP Server

`tap mcp` starts a Model Context Protocol server over stdio using the official Go SDK (`github.com/modelcontextprotocol/go-sdk`). Exposes 6 tools:

| Tool | Parameters | Description |
|------|-----------|-------------|
| `tap_snapshot` | `project` (string), `curated` (bool, default true) | Latest system snapshot with all running dev processes, grouped by project, with CPU/memory stats |
| `tap_history` | `project` (string), `since` (string, e.g. "30m", "1h", default "1h"), `limit` (number, default 50) | Historical snapshot summaries showing process counts, CPU, and memory over time |
| `tap_logs` | `project` (string), `stream` (string, "stdout"/"stderr"), `since` (string), `limit` (number, default 100), `search` (string) | Captured console output with text search support |
| `tap_projects` | _(none)_ | List all known projects currently being tracked |
| `tap_health` | `project` (string) | Health diagnostics — flags stale, orphaned, high-memory, and high-CPU processes |
| `tap_processes` | `project` (string), `port` (number), `name` (string) | Process list with filtering by project, port, or name |

All parameters are optional. Tool handlers use typed input structs with automatic JSON schema generation via the Go SDK's generic `AddTool` function.

---

## Current State

### What's Built

**Discovery & Attribution:**
- Process discovery via gopsutil (PID, PPID, name, cmdline, CWD, CPU%, memory, uptime)
- Cross-platform port-to-PID mapping via gopsutil net.Connections
- Docker container discovery via Docker Engine API (graceful fallback when unavailable)
- Docker Compose containers attributed to projects via `com.docker.compose.project.working_dir` label
- Project attribution by walking CWD up to project markers, with parent chain fallback (up to 10 levels)
- Two-pass dev filtering: first pass keeps known dev names + containers + port listeners; second pass (curated mode) drops unattributed processes that aren't known dev tools
- Broad ecosystem support: JS/TS, Python, Rust, Go, Ruby, Java, PHP, .NET, Elixir, C++

**Health Analysis:**
- Stale detection (uptime > 24 hours)
- Orphan detection (parent PID is dead, verified against OS process table)
- Resource warnings (memory > 1GB, CPU > 50%)
- Parent chain walking with liveness checks for diagnostic detail view

**Persistent Storage:**
- SQLite with WAL mode for concurrent reads
- Background collector polling every 5 seconds
- Auto-start on any tap command
- 24-hour data retention with automatic pruning
- Automatic console output capture via PATH shimming

**TUI Dashboard (bubbletea + lipgloss):**
- Dashboard view with project grouping, health indicators, system stats header
- Process detail view with diagnostic card (process info, parent chain tree, sibling services, health diagnostics)
- Help overlay
- Kill/stop with confirmation dialogs
- 2-second auto-refresh with cursor preservation
- j/k and arrow key navigation, Enter to drill down, Esc to go back

**MCP Server:**
- Official Go SDK (`github.com/modelcontextprotocol/go-sdk`)
- 6 tools with typed input schemas
- Stdio transport for Claude Code integration

**CLI Commands:**
- `tap` — launches TUI dashboard (default)
- `tap dash` — explicit TUI launch
- `tap ls` — non-interactive table view
- `tap ports` — all ports sorted by number (unfiltered)
- `tap port <PORT>` — single port diagnostic (unfiltered)
- `tap kill <PID or :PORT>` — kill by PID or port (unfiltered)
- `tap project <NAME>` — filtered project view
- `tap stop <NAME>` — stop all processes for a project
- `tap clean` — find stale processes (>24h) and stopped containers
- `tap doctor` — resource report with collector status
- `tap init` — create `.tap.toml` to register a project
- `tap export [NAME]` — shareable snapshot
- `tap logs [project]` — view captured output (--since, --stream, --follow, --limit)
- `tap teardown` — remove PATH shims and restore shell config
- `tap history [project]` — snapshot timeline (--since, --limit)
- `tap mcp` — start MCP server over stdio
- `--json`, `--verbose` flags on all commands
- `--data-dir` to override default `~/.tap/` location

**Browsing vs Searching:**
- Browsing (`tap`, `tap ls`, `tap project`) — curated view, hides non-dev port listeners
- Searching (`tap port`, `tap ports`, `tap kill`) — unfiltered, always shows everything on a port

### Known Limitations

- CWD-based attribution isn't perfect — processes that change their working directory after start may be misattributed
- Docker container CPU/memory shows as 0 (Docker stats API is streaming, needs caching)
- Docker containers are opaque for diagnostics — no parent chain or orphan detection inside containers

---

## Data Flow

```
Process Table (gopsutil)
    │
    ▼
Port Mapper (gopsutil net.Connections)
    │
    ▼
First-Pass Filter (known dev names + containers + port listeners)
    │
    ▼
Docker Client (container list, port mappings)
    │
    ▼
Project Mapper (CWD → markers → project root, parent chain fallback)
    │
    ▼
Second-Pass Filter (curated mode: drop non-dev unattributed port listeners)
    │
    ▼
Health Analyzer (stale, orphan, high memory, high CPU, parent chain)
    │
    ▼
Snapshot (unified model)
    │
    ▼
Background Collector (only writer)
    │
    ▼
SQLite (tap.db, WAL mode)
    │
    ├──► TUI Dashboard (bubbletea)        ← reads from DB
    ├──► CLI Commands (tabwriter)         ← reads from DB
    ├──► MCP Server (stdio)               ← reads from DB
    └──► JSON Output (encoding/json)      ← reads from DB

PATH Shims (~/.tap/shims/)
    │
    ▼
tap _shim <command> [args...]             ← intercepts dev commands
    │
    ▼
SQLite (tap.db, WAL mode)                 ← writes captured output
```

---

## Technical Details

### Language

**Go** — fast startup, single static binary, natural Docker integration, excellent TUI ecosystem (Charm's bubbletea), cross-platform process enumeration (gopsutil).

### Key Dependencies

| Concern | Package |
|---------|---------|
| Process enumeration | `github.com/shirou/gopsutil/v3` |
| Docker API | `github.com/docker/docker` v28.x |
| CLI framework | `github.com/spf13/cobra` |
| TUI framework | `github.com/charmbracelet/bubbletea` |
| TUI styling | `github.com/charmbracelet/lipgloss` |
| TUI components | `github.com/charmbracelet/bubbles` |
| SQLite | `modernc.org/sqlite` (pure Go, no CGo) |
| MCP server | `github.com/modelcontextprotocol/go-sdk` |
| Table output | `text/tabwriter` (stdlib) |

### Platform Support

- Windows (native, not just WSL2)
- macOS (Apple Silicon + Intel)
- Linux (x86_64 + arm64)

Windows-specific: path normalization lowercases for dedup, preserves originals for display. Process detachment uses `DETACHED_PROCESS` creation flag.

---

## File Structure

```
tap/
├── go.mod
├── go.sum
├── main.go
├── cmd/
│   ├── root.go                     # Root command + global flags + auto-start
│   ├── ls.go                       # tap ls
│   ├── ports.go                    # tap ports
│   ├── port.go                     # tap port <PORT>
│   ├── kill.go                     # tap kill <TARGET>
│   ├── project.go                  # tap project <NAME>
│   ├── stop.go                     # tap stop <NAME>
│   ├── clean.go                    # tap clean
│   ├── init_cmd.go                 # tap init
│   ├── export.go                   # tap export
│   ├── dash.go                     # tap dash
│   ├── doctor.go                   # tap doctor
│   ├── collect.go                  # tap _collect (hidden, background)
│   ├── shim.go                     # tap _shim (hidden, capture via PATH shims)
│   ├── teardown.go                 # tap teardown
│   ├── logs_cmd.go                 # tap logs
│   ├── history.go                  # tap history
│   └── mcp.go                     # tap mcp
├── internal/
│   ├── discovery/
│   │   ├── processes.go            # Process enumeration via gopsutil
│   │   ├── ports.go                # Port-to-PID mapping
│   │   ├── docker.go               # Docker container discovery
│   │   ├── projects.go             # Project detection + attribution
│   │   ├── snapshot.go             # Aggregator + dev filter + system stats
│   │   └── health.go               # Health analysis
│   ├── model/
│   │   ├── process.go              # DevProcess, PortBinding, ContainerInfo
│   │   ├── project.go              # Project
│   │   └── snapshot.go             # Snapshot
│   ├── store/
│   │   ├── store.go                # SQLite setup, schema, WAL mode
│   │   ├── write.go                # PersistSnapshot, Prune
│   │   ├── query.go                # LatestSnapshot, ProcessesByPort, etc.
│   │   └── logs.go                 # Log line insert/query/prune
│   ├── daemon/
│   │   ├── collector.go            # Background collector poll loop
│   │   ├── ensure.go               # Auto-start logic + PID file
│   │   ├── detach_windows.go       # Windows process detachment
│   │   └── detach_unix.go          # Unix process detachment
│   ├── capture/
│   │   └── runner.go               # RunAndCapture for console capture
│   ├── shim/
│   │   ├── shim.go                 # EnsureShims, RemoveShims, shell config
│   │   └── resolve.go              # ResolveReal — find binary excluding shims
│   ├── mcp/
│   │   ├── server.go               # MCP server setup + stdio transport
│   │   └── tools.go                # Tool handlers
│   └── tui/
│       ├── app.go                  # Main bubbletea model
│       ├── dashboard.go            # Dashboard view
│       ├── detail.go               # Process detail view
│       ├── keys.go                 # Key bindings
│       └── styles.go               # lipgloss styles
├── tap-spec.md
├── README.md
└── LICENSE
```

---

## Data Model

```go
type DevProcess struct {
    PID           *int32
    PPID          int32
    Name          string
    Command       string
    Ports         []PortBinding
    Project       string
    ProjectPath   string
    CPUPercent    float64
    MemoryBytes   uint64
    StartTime     time.Time
    Kind          ProcessKind    // native | docker | podman
    ContainerInfo *ContainerInfo
    Health        *ProcessHealth
}

type ProcessHealth struct {
    Flags       []HealthFlag
    ParentChain []ParentInfo
}

type HealthFlag string  // "stale", "orphan", "high_mem", "high_cpu"
```

---

## Roadmap

### Done
- Process discovery + port mapping (Windows + macOS + Linux)
- Project attribution via CWD + parent chain walking
- Docker container discovery
- Health analysis (stale, orphan, resource warnings)
- TUI dashboard with bubbletea + lipgloss
- All CLI commands (ls, ports, port, kill, stop, clean, doctor, init, export, project)
- Background collector with SQLite persistence
- Automatic console capture (PATH shimming) and log viewing (`tap logs`)
- Snapshot history (`tap history`)
- MCP server for AI tool integration

### Future
- Restart loop detection (process killed but respawned by parent)
- Port conflict warnings ("port 3000 is already taken by project X")
- Config file (`~/.config/tap/config.toml`)
- `tap export` polish (`--clipboard`, richer output)
- Homebrew / Scoop / GitHub Releases

---

## Non-Goals

- **Not a process supervisor** — doesn't keep processes alive (not PM2/systemd)
- **Not a container orchestrator** — shows Docker state, doesn't replace docker-compose
- **Not a system monitor** — only dev processes, not htop

---

## Success Metrics

- `tap` (TUI) renders in **< 1 second** on first launch
- `tap ls` responds in **< 100ms** (reads from DB, no live scan)
- Binary size **< 15MB**
- Diagnostic card accurately identifies stale/orphan processes
- MCP tools respond in **< 200ms**
