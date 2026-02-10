# tap

Diagnostic dev process manager. See what's running, which project owns it, and whether it's healthy.

Tap groups your dev processes, ports, and Docker containers by project — and flags problems like stale processes, orphaned children, and resource hogs. One command to diagnose, one keystroke to fix.

```
$ tap
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

 ↑↓ navigate  enter expand  k kill  s stop project  ? help  q quit
```

Press Enter on any process for a full diagnostic card — parent chain, health flags, sibling services, and quick actions.

## Why not just use lsof?

`lsof -i :3000` gives you a PID. Then `ps aux | grep` to figure out what it is. Then `kill -9` and hope for the best.

Tap tells you the full story: which project owns it, who spawned it, whether the parent is dead (orphaned process), how long it's been running, and what else that project has going. Then lets you kill it, or stop the whole project, with one keystroke.

## Install

```bash
# From source
go install github.com/tap-dev/tap@latest

# Or build locally
git clone https://github.com/tap-dev/tap.git
cd tap
go build -o tap .
```

Prebuilt binaries for Windows, macOS, and Linux are available on the [releases page](https://github.com/tap-dev/tap/releases).

## Usage

```bash
# Launch TUI dashboard (default)
tap

# Non-interactive table output
tap ls

# What's on port 3000? (full diagnostic)
tap port 3000

# List all ports in use
tap ports

# Kill whatever's on port 3000
tap kill :3000

# Kill by PID
tap kill 8821

# Show all processes for a specific project
tap project my-saas-app

# Stop everything for a project
tap stop my-saas-app

# Clean up stale processes and stopped containers
tap clean

# Export a shareable snapshot (services, toolchain versions, OS)
tap export
tap export my-saas-app

# Register current directory as a project
tap init

# JSON output (for scripting)
tap ls --json

# Skip Docker discovery (faster)
tap --no-docker
```

## How it works

Tap scans your system and builds a diagnostic snapshot:

1. **Process discovery** — enumerates all running processes via gopsutil (PID, name, command line, working directory, CPU%, memory, uptime)
2. **Port mapping** — finds which processes are listening on which TCP ports
3. **Docker discovery** — lists running containers with their port mappings, images, and names
4. **Dev filtering** — two-pass filter: first keeps dev tools and port listeners, then removes non-dev processes that aren't attributed to any project
5. **Project attribution** — walks each process's working directory up the filesystem tree looking for project markers, and groups processes by project root. Falls back to parent process chain walking.
6. **Health analysis** — flags stale processes (>24h uptime), orphaned processes (parent is dead), high memory (>1GB), and high CPU (>50%). Walks the parent chain for diagnostic detail.

### Browsing vs searching

The dashboard (`tap`, `tap ls`) is **curated** — it only shows dev-relevant processes and hides system noise. When you **search** (`tap port`, `tap ports`, `tap kill`), you get the complete unfiltered picture. If something is on a port, those commands will always find it.

### Health indicators

| Indicator | Condition |
|-----------|-----------|
| ● HEALTHY | No issues detected |
| ⚠ STALE | Running longer than 24 hours |
| ⚠ ORPHAN | Parent process is dead |
| ⚠ HIGH MEM | Memory usage exceeds 1 GB |
| ⚠ HIGH CPU | CPU usage exceeds 50% |

### Supported ecosystems

Tap recognizes project markers and runtimes for:

- **JavaScript/TypeScript** — package.json, yarn.lock, pnpm-lock.yaml, bun.lockb, node, bun, deno, npm, yarn, pnpm, vite, webpack, turbo, nodemon
- **Python** — pyproject.toml, Pipfile, poetry.lock, uv.lock, requirements.txt, setup.py, python, uv, uvicorn, gunicorn, celery, pipenv, poetry
- **Go** — go.mod, go, gopls, air
- **Rust** — Cargo.toml, Cargo.lock, cargo, rustc
- **Ruby** — Gemfile, Gemfile.lock, ruby, bundler, rails, puma, sidekiq
- **Java/JVM** — pom.xml, build.gradle, java, gradle, mvn
- **PHP** — composer.json, composer.lock, php, artisan
- **.NET** — *.csproj, *.sln, global.json, dotnet
- **Elixir** — mix.exs, elixir, mix, iex
- **C/C++** — CMakeLists.txt
- **Databases** — postgres, mysql, redis-server, mongod
- **Docker** — docker-compose.yml, Dockerfile, containers via Docker API

## Commands

| Command | Description |
|---------|-------------|
| `tap` | Launch interactive TUI dashboard |
| `tap ls` | List all dev processes (non-interactive table) |
| `tap dash` | Launch TUI dashboard (explicit) |
| `tap ports` | List all ports in use, sorted by port number |
| `tap port <PORT>` | Diagnostic lookup for a specific port |
| `tap project <NAME>` | Show all processes for a specific project |
| `tap kill <PID>` | Kill a process by PID |
| `tap kill :<PORT>` | Kill whatever's listening on a port |
| `tap stop <NAME>` | Stop all processes for a project (with confirmation) |
| `tap clean` | Find stale processes and stopped containers, offer to remove |
| `tap init` | Register current directory as a project (creates `.tap.toml`) |
| `tap export [NAME]` | Export a shareable snapshot with services and system info |

### TUI keyboard shortcuts

| Key | Action |
|-----|--------|
| `↑`/`↓` or `j`/`k` | Navigate processes |
| `Enter` | Expand process detail (diagnostic card) |
| `Esc` | Back to dashboard |
| `k` | Kill selected process |
| `s` | Stop all processes for selected project |
| `r` | Force refresh |
| `?` | Help |
| `q` | Quit |

### Flags

| Flag | Description |
|------|-------------|
| `--json` | Output as JSON (falls back to non-interactive) |
| `--no-docker` | Skip Docker/Podman container discovery |
| `--verbose` | Show full command lines |

## Platform support

Tap works natively on **Windows**, **macOS**, and **Linux**. Process and port discovery use [gopsutil](https://github.com/shirou/gopsutil) for cross-platform support. Docker discovery uses the Docker Engine API. TUI uses [bubbletea](https://github.com/charmbracelet/bubbletea).

## Roadmap

- [x] Process discovery + port mapping
- [x] Project attribution via working directory heuristics
- [x] Process tree walking (parent chain) for better attribution
- [x] `tap ls`, `tap ports`, `tap port`, `tap kill`
- [x] `tap project`, `tap stop`, `tap clean`, `tap init`, `tap export`
- [x] Docker container discovery
- [x] JSON output
- [x] Broad ecosystem support (JS, Python, Go, Rust, Ruby, Java, PHP, .NET, Elixir, C++)
- [x] Interactive TUI dashboard (bubbletea + lipgloss)
- [x] Health diagnostics (stale, orphan, high memory, high CPU)
- [x] Parent chain visualization in detail view
- [x] Two-pass dev filtering (curated browsing, unfiltered search)
- [ ] Global config file (`~/.config/tap/config.toml`)
- [ ] `tap up` / `tap down` — start/stop a full project stack from `.tap.toml`
- [ ] Port conflict prevention
- [ ] Background watchdog with notifications

## Building from source

Requires Go 1.24+.

```bash
# Development build
go build -o tap .

# Release build (smaller binary, ~8.4MB)
go build -ldflags="-s -w" -o tap .

# Run tests
go test ./...
```

## Architecture

```
tap/
├── main.go                    # Entry point
├── cmd/                       # CLI commands (cobra)
│   ├── root.go                # Root command + global flags + TUI launch
│   ├── dash.go                # tap dash (explicit TUI entry)
│   ├── ls.go                  # tap ls (non-interactive table)
│   ├── ports.go               # tap ports
│   ├── port.go                # tap port <PORT>
│   ├── kill.go                # tap kill
│   ├── project.go             # tap project <NAME>
│   ├── stop.go                # tap stop <NAME>
│   ├── clean.go               # tap clean
│   ├── init_cmd.go            # tap init
│   └── export.go              # tap export
└── internal/
    ├── model/                 # Data types (DevProcess, Project, Snapshot, ProcessHealth)
    ├── discovery/             # System scanning
    │   ├── processes.go       # Process enumeration (gopsutil)
    │   ├── ports.go           # Port-to-PID mapping (gopsutil)
    │   ├── docker.go          # Docker container discovery
    │   ├── projects.go        # Project root detection + parent chain attribution
    │   ├── snapshot.go        # Aggregator + two-pass dev filter
    │   └── health.go          # Health analysis (stale, orphan, resources)
    └── tui/                   # Interactive TUI (bubbletea)
        ├── app.go             # Main model + update loop
        ├── dashboard.go       # Dashboard view (project list)
        ├── detail.go          # Process detail / diagnostic card
        ├── keys.go            # Key bindings
        └── styles.go          # lipgloss styles
```

## License

MIT
