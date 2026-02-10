# tap

See everything running on your dev machine, organized by project.

Tap groups your dev processes, ports, and Docker containers by project — so you know that the `node` on `:3000`, the `postgres` on `:5432`, and the `redis` on `:6379` all belong to your SaaS app, not your side project.

```
$ tap
PROJECT       PID    NAME       PORTS          CPU%   MEMORY     UPTIME
my-saas-app   8821   node       :3000          12.3%  420.5 MB   2h 15m
my-saas-app   9102   postgres   :5432          1.1%   180.2 MB   2h 15m
my-saas-app   9103   redis      :6379          0.2%   30.1 MB    2h 15m
side-project  11200  go         :8080          3.4%   90.3 MB    45m
side-project  11201  postgres   :5433          0.1%   150.0 MB   45m
-             15332  node       :8443          5.0%   200.1 MB   3d 2h
```

## Why not just use lsof?

`lsof -i :3000` tells you PID 8821 is on port 3000. Then you run `ps aux | grep 8821` to figure out what it is. Then maybe `docker ps` to check containers. Then `kill -9 8821` and hope for the best.

Tap gives you the full picture in one command: which project owns the port, what else that project is running, and how much resources it's all consuming. Kill by port, kill by project, or just see what's going on.

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
# List all dev processes, grouped by project
tap
tap ls

# What's on port 3000?
tap port 3000

# List all ports in use
tap ports

# Kill whatever's on port 3000
tap kill :3000

# Kill by PID
tap kill 8821

# JSON output (for scripting)
tap ls --json

# Skip Docker discovery (faster)
tap ls --no-docker

# Show full command lines
tap ls --verbose
```

## How it works

Tap scans your system and builds a snapshot:

1. **Process discovery** — enumerates all running processes, collects PID, name, command line, working directory, CPU%, memory, uptime
2. **Port mapping** — finds which processes are listening on which TCP ports
3. **Docker discovery** — lists running containers with their port mappings, images, and names
4. **Dev filtering** — hides system processes, keeps dev tools (node, python, go, postgres, redis, etc.) and anything with an open port
5. **Project attribution** — walks each process's working directory up the filesystem tree looking for project markers (`package.json`, `go.mod`, `Cargo.toml`, `pyproject.toml`, `.git`, `docker-compose.yml`, etc.) and groups processes by project root

## Commands

| Command | Description |
|---------|-------------|
| `tap` / `tap ls` | List all dev processes in a table |
| `tap ports` | List all ports in use, sorted by port number |
| `tap port <PORT>` | Show what's running on a specific port |
| `tap kill <PID>` | Kill a process by PID |
| `tap kill :<PORT>` | Kill whatever's listening on a port |

### Flags

| Flag | Description |
|------|-------------|
| `--json` | Output as JSON |
| `--no-docker` | Skip Docker/Podman container discovery |
| `--verbose` | Show full command lines |

## Platform support

Tap works natively on **Windows**, **macOS**, and **Linux**. Process and port discovery use [gopsutil](https://github.com/shirou/gopsutil) for cross-platform support. Docker discovery uses the Docker Engine API.

## Roadmap

- [x] Process discovery + port mapping
- [x] Project attribution via working directory heuristics
- [x] `tap ls`, `tap ports`, `tap port`, `tap kill`
- [x] Docker container discovery
- [x] JSON output
- [ ] Interactive TUI dashboard (bubbletea)
- [ ] `tap stop <project>` — stop all processes for a project
- [ ] `tap clean` — find and kill orphaned dev processes
- [ ] `tap init` — manually register a project
- [ ] `tap up` / `tap down` — start/stop a full project stack from `.tap.toml`
- [ ] Port conflict prevention
- [ ] Background watchdog with notifications

## Building from source

Requires Go 1.23+ (toolchain auto-downloads 1.24 if needed).

```bash
# Development build
go build -o tap .

# Release build (smaller binary, ~7.6MB)
go build -ldflags="-s -w" -o tap .

# Run tests
go test ./...
```

## Architecture

```
tap/
├── main.go                    # Entry point
├── cmd/                       # CLI commands (cobra)
│   ├── root.go                # Root command + global flags
│   ├── ls.go                  # tap ls
│   ├── ports.go               # tap ports
│   ├── port.go                # tap port <PORT>
│   └── kill.go                # tap kill
└── internal/
    ├── model/                 # Data types (DevProcess, Project, Snapshot)
    └── discovery/             # System scanning
        ├── processes.go       # Process enumeration (gopsutil)
        ├── ports.go           # Port-to-PID mapping (gopsutil)
        ├── docker.go          # Docker container discovery
        ├── projects.go        # Project root detection + attribution
        └── snapshot.go        # Aggregator + dev filter
```

## License

MIT
