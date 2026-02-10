# Tap — Project Specification

## One-Liner

A diagnostic CLI that tells you *why* port 3000 is stuck, which project owns it, and what else that project is running — then lets you fix it.

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

**Tap's angle:** The diagnostic card. Not just "what's on port 3000" but "here's the full story — the process, its parent chain, the project, the resources, and whether it's healthy or a zombie." Visualized in a TUI that you'd actually screenshot and share.

The tools that succeed in dev tooling are either daily drivers or visually compelling enough to spread. Tap aims for the latter: the kind of output you'd paste in a PR, tweet, or teammate DM.

---

## Target User

- Full-stack developers running 2-5 projects simultaneously
- Developers who hit port conflicts weekly and want a faster diagnostic workflow
- Anyone who has killed a PID from `lsof` and hoped for the best

---

## Core Concept: The Diagnostic Card

The killer feature is `tap :3000` — a single command that produces a diagnostic card:

```
Port 3000 — OCCUPIED

├─ node (PID 4521) — running 3d 4h — ⚠ STALE (uptime > 24h)
│  command:  next dev
│  spawned by: npm run dev (PID 4518) — DEAD
│  project: ~/code/my-saas-app (Next.js)
│  memory: 284 MB (RSS)
│  cpu: 12.3%
│
└─ [k] kill  [s] stop project  [q] quit
```

This tells you everything:
- **What** is on the port (node, running Next.js dev server)
- **Who started it** (npm run dev — and that parent is dead, so this is semi-orphaned)
- **Which project** it belongs to
- **Whether it's healthy** (running 3 days with a dead parent = probably forgotten)
- **What to do about it** (kill it, or stop the whole project)

---

## Current State (v0.2 — TUI + Diagnostics Complete)

### What's Built

Discovery engine, CLI commands, health analysis, and TUI dashboard — working on Windows, macOS, and Linux:

**Discovery & Attribution:**
- Process discovery via gopsutil (PID, PPID, name, cmdline, CWD, CPU%, memory, uptime)
- Cross-platform port-to-PID mapping via gopsutil net.Connections
- Docker container discovery via Docker Engine API (graceful fallback when unavailable)
- Project attribution by walking CWD up to project markers, with parent chain fallback (up to 10 levels)
- Two-pass dev filtering: first pass keeps known dev names + containers + port listeners; second pass (curated mode) drops unattributed processes that aren't known dev tools
- Broad ecosystem support: JS/TS, Python, Rust, Go, Ruby, Java, PHP, .NET, Elixir, C++

**Health Analysis:**
- Stale detection (uptime > 24 hours)
- Orphan detection (parent PID is dead, verified against OS process table)
- Resource warnings (memory > 1GB, CPU > 50%)
- Parent chain walking with liveness checks for diagnostic detail view

**TUI Dashboard (bubbletea + lipgloss):**
- Dashboard view with project grouping, health indicators, system stats header
- Process detail view with diagnostic card (process info, parent chain tree, sibling services, health diagnostics)
- Help overlay
- Kill/stop with confirmation dialogs
- 2-second auto-refresh with cursor preservation
- j/k and arrow key navigation, Enter to drill down, Esc to go back

**Browsing vs Searching:**
- Browsing (`tap`, `tap ls`, `tap project`) — curated view, hides non-dev port listeners
- Searching (`tap port`, `tap ports`, `tap kill`) — unfiltered, always shows everything on a port

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
- `tap init` — create `.tap.toml` to register a project
- `tap export [NAME]` — shareable snapshot with services and system info
- `--json`, `--no-docker`, `--verbose` flags on all commands

### Known Limitations

- CWD-based attribution isn't perfect — processes that change their working directory after start may be misattributed
- Docker container CPU/memory shows as 0 (Docker stats API is streaming, needs caching)
- Docker containers are opaque for diagnostics — no parent chain or orphan detection inside containers
- No config file yet — dev process names and thresholds are hardcoded

---

## Phase 2: TUI Dashboard (Done)

The TUI is the primary interface. It's what makes tap visual enough to share.

### Main View — Project Dashboard

```
 tap                                              CPU: 34%  MEM: 8.2/16 GB
─────────────────────────────────────────────────────────────────────────────

 ▼ my-saas-app                    3 services    CPU 13.6%    630 MB    2h
   ● :3000  node (next dev)                     CPU 12.3%    420 MB    2h
   ● :5432  postgres (docker: pg-main)          CPU  1.1%    180 MB    2h
   ● :6379  redis (docker: redis-cache)         CPU  0.2%     30 MB    2h

 ▼ side-project                   2 services    CPU  3.5%    240 MB   45m
   ● :8080  go (air)                            CPU  3.4%     90 MB   45m
   ● :5433  postgres (docker: pg-side)          CPU  0.1%    150 MB   45m

 ▼ unattributed                   1 process
   ⚠ :8443  node                  ⚠ 3d uptime   CPU  5.0%    200 MB    3d

─────────────────────────────────────────────────────────────────────────────
 ↑↓ navigate  enter expand  k kill  s stop project  f filter  ? help  q quit
```

### Detail View — Process Diagnostic (Enter on a process)

```
 ● Port 3000 — node                                              ⚠ STALE
─────────────────────────────────────────────────────────────────────────────

 Process
   PID:       4521
   Name:      node
   Command:   /usr/local/bin/node .next/server.js
   Uptime:    3d 4h 12m
   CPU:       12.3%
   Memory:    284.2 MB (RSS)

 Parent Chain
   └─ npm run dev (PID 4518)         ✗ DEAD
      └─ bash (PID 4510)             ✓ alive
         └─ tmux (PID 1200)          ✓ alive

 Project
   Name:      my-saas-app
   Path:      ~/code/my-saas-app
   Marker:    package.json
   Other services:
     ● :5432  postgres    180 MB  2h
     ● :6379  redis        30 MB  2h

 Diagnostics
   ⚠ Process has been running for 3+ days
   ⚠ Parent process (npm run dev, PID 4518) is dead — this may be orphaned
   ● Listening on 0.0.0.0:3000 (tcp)

─────────────────────────────────────────────────────────────────────────────
 k kill  s stop project  esc back  q quit
```

### Health Indicators

Tap flags potential problems automatically:

| Indicator | Condition | Display |
|-----------|-----------|---------|
| ⚠ STALE | Uptime > 24 hours | Yellow warning |
| ⚠ ORPHAN | Parent process is dead | Yellow warning |
| ⚠ HIGH MEM | Memory > 1 GB | Yellow warning |
| ⚠ HIGH CPU | CPU > 50% sustained | Yellow warning |
| ● HEALTHY | None of the above | Green dot |

### Keyboard Controls

| Key | Action |
|-----|--------|
| `↑`/`↓` or `j`/`k` | Navigate processes |
| `Enter` | Expand process detail / diagnostic card |
| `Esc` | Back to main view |
| `k` | Kill selected process (with confirmation) |
| `s` | Stop all processes for selected project |
| `f` | Filter by project name |
| `/` | Search by name or port |
| `r` | Force refresh |
| `?` | Help overlay |
| `q` | Quit |

### Real-time Refresh

- Dashboard auto-refreshes every 2 seconds
- Cursor position preserved across refreshes
- Stale/orphan detection runs on each refresh

---

## CLI Interface

```
tap — Diagnostic dev process manager

USAGE:
    tap [COMMAND]

COMMANDS:
    (no command)    Launch TUI dashboard
    ls              List all dev processes (table, non-interactive)
    ports           List all ports in use
    port <PORT>     Diagnostic card for a specific port (non-interactive)
    kill <TARGET>   Kill a process by PID or :PORT
    project <NAME>  Show processes for a project
    stop <NAME>     Stop all processes for a project
    clean           Find stale processes and stopped containers
    init            Register current directory as a project
    export [NAME]   Export state for sharing

OPTIONS:
    --json          Output as JSON (for ls, ports, port commands)
    --no-docker     Skip Docker/Podman discovery
    --verbose       Show full command lines
    -h, --help      Show help
    -V, --version   Show version
```

The bare `tap` command launches the TUI. `tap ls` is the non-interactive fallback for scripts and pipes.

---

## Technical Architecture

### Language

**Go** — fast startup (~10-20ms), single static binary, natural Docker integration, excellent TUI ecosystem (Charm's bubbletea), cross-platform process enumeration (gopsutil).

### Key Dependencies

| Concern | Package | Status |
|---------|---------|--------|
| Process enumeration | `github.com/shirou/gopsutil/v3` | In use |
| Docker API | `github.com/docker/docker` v28.x | In use |
| CLI framework | `github.com/spf13/cobra` | In use |
| Table output | `text/tabwriter` (stdlib) | In use |
| TUI framework | `github.com/charmbracelet/bubbletea` | Phase 2 |
| TUI styling | `github.com/charmbracelet/lipgloss` | Phase 2 |
| TUI components | `github.com/charmbracelet/bubbles` | Phase 2 |

### Platform Support

- Windows (native, not just WSL2)
- macOS (Apple Silicon + Intel)
- Linux (x86_64 + arm64)

Windows-specific: path normalization lowercases for dedup, preserves originals for display.

### Data Flow

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
    ├──► TUI Dashboard (bubbletea)        ← primary interface (curated)
    ├──► CLI Table Output (tabwriter)      ← non-interactive fallback (curated)
    ├──► Port/Search Commands              ← unfiltered (skips second pass)
    └──► JSON Output (encoding/json)       ← scripting
```

### Health Analysis

The health analyzer runs as part of snapshot creation so both the TUI and CLI benefit:

- **Stale detection:** uptime > 24 hours → flag
- **Orphan detection:** check if PPID exists in the snapshot and the OS process table. If parent is dead, flag as orphaned.
- **Resource warnings:** memory > 1GB or CPU > 50% → flag
- **Parent chain:** walk the PPID chain via gopsutil, check liveness of each ancestor for the diagnostic detail view

### Two-Pass Dev Filtering

The dashboard needs to be clean (no system noise), but search commands need to be complete (find anything on a port). This is solved with two filter passes:

1. **First pass (before attribution):** Keep known dev tool names, containers, and any process with open ports. This is a performance gate — most system processes have no ports and get dropped early.
2. **Second pass (after attribution, curated mode only):** Drop unattributed processes that only passed the first filter because they had ports, but are not known dev tools and not containers.

Browsing commands (`tap`, `tap ls`, `tap project`) use curated mode. Search commands (`tap port`, `tap ports`, `tap kill`) skip the second pass.

---

## File Structure

```
tap/
├── go.mod
├── go.sum
├── main.go                         # Entry point
├── cmd/
│   ├── root.go                     # Root cobra command + global flags
│   ├── ls.go                       # tap ls (non-interactive table)
│   ├── ports.go                    # tap ports
│   ├── port.go                     # tap port <PORT>
│   ├── kill.go                     # tap kill <TARGET>
│   ├── project.go                  # tap project <NAME>
│   ├── stop.go                     # tap stop <project>
│   ├── clean.go                    # tap clean
│   ├── init_cmd.go                 # tap init
│   ├── export.go                   # tap export
│   └── dash.go                     # tap dash (TUI entry point)├── internal/
│   ├── discovery/
│   │   ├── processes.go            # Process enumeration via gopsutil
│   │   ├── ports.go                # Port-to-PID mapping
│   │   ├── docker.go               # Docker container discovery
│   │   ├── projects.go             # Project detection + attribution
│   │   ├── snapshot.go             # Aggregator + dev filter + system stats
│   │   └── health.go               # Health analysis (stale, orphan, resources)│   ├── model/
│   │   ├── process.go              # DevProcess, PortBinding, ContainerInfo
│   │   ├── project.go              # Project
│   │   └── snapshot.go             # Snapshot
│   └── tui/                        # TUI dashboard│       ├── app.go                  # Main bubbletea model + update loop
│       ├── dashboard.go            # Dashboard view (project list)
│       ├── detail.go               # Process detail / diagnostic card view
│       ├── keys.go                 # Key bindings
│       └── styles.go               # lipgloss styles + colors
├── tap-spec.md
├── README.md
└── LICENSE
```

---

## Data Model

Current model (unchanged):

```go
type DevProcess struct {
    PID           *int32         // nil for stopped containers
    PPID          int32          // Parent PID
    Name          string         // "node", "postgres"
    Command       string         // Full command line
    Ports         []PortBinding
    Project       string         // Attributed project name
    ProjectPath   string         // Project root directory
    CPUPercent    float64
    MemoryBytes   uint64
    StartTime     time.Time
    Kind          ProcessKind    // native | docker | podman
    ContainerInfo *ContainerInfo
}
```

New additions for health analysis:

```go
type HealthFlag string

const (
    HealthOK      HealthFlag = "ok"
    HealthStale   HealthFlag = "stale"    // uptime > 24h
    HealthOrphan  HealthFlag = "orphan"   // parent PID is dead
    HealthHighMem HealthFlag = "high_mem" // > 1GB RSS
    HealthHighCPU HealthFlag = "high_cpu" // > 50% CPU
)

type ProcessHealth struct {
    Flags       []HealthFlag    // Active health flags
    ParentChain []ParentInfo    // Walked parent chain for diagnostics
}

type ParentInfo struct {
    PID   int32
    Name  string
    Alive bool
}
```

These get attached to `DevProcess` as a `Health *ProcessHealth` field.

---

## Roadmap

### Phase 1 — Core CLI (Done)
- ~~Process discovery + port mapping (Windows + macOS + Linux)~~
- ~~Project attribution via CWD + parent chain walking~~
- ~~`tap ls`, `tap ports`, `tap port`, `tap kill`~~
- ~~Docker container discovery~~
- ~~JSON output, dev-process filtering~~

### Phase 3 — Quality of Life (Done)
- ~~`tap stop`, `tap clean`, `tap init`, `tap export`, `tap project`~~
- ~~Broad ecosystem support (10+ languages)~~
- ~~Parent chain attribution (PPID walking)~~

### Phase 2 — TUI Dashboard + Diagnostics (Done)
- ~~Health analysis engine (stale, orphan, resource warnings)~~
- ~~TUI dashboard with bubbletea + lipgloss~~
- ~~Project grouping with expand/collapse~~
- ~~Process detail view (diagnostic card)~~
- ~~Parent chain visualization with liveness checks~~
- ~~Keyboard navigation and actions (kill, stop, confirm dialogs)~~
- ~~Real-time 2-second refresh~~
- ~~`tap` (bare command) launches TUI~~
- ~~Two-pass dev filtering (curated browsing vs unfiltered search)~~

### Phase 4 — Polish & Distribution (Future)
- Global config file (`~/.config/tap/config.toml`)
- Configurable thresholds (stale hours, memory warning)
- `.tap.toml` project profiles
- `tap up` / `tap down` — project stack management
- Port conflict prevention
- Homebrew / Scoop / GitHub Releases

---

## Non-Goals

- **Not a process supervisor** — doesn't keep processes alive (not PM2/systemd)
- **Not a container orchestrator** — shows Docker state, doesn't replace docker-compose
- **Not a system monitor** — only dev processes, not htop
- **Not a service orchestrator** — port-kill already does YAML-based service management. Tap focuses on diagnostics and visibility, not orchestration.

---

## Success Metrics

- `tap` (TUI) renders in **< 1 second** on first launch
- `tap ls` snapshot in **< 500ms**
- Binary size **< 15MB** (with TUI deps)
- Diagnostic card accurately identifies stale/orphan processes
- Parent chain correctly traces 3+ levels
