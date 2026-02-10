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

# Show all processes for a specific project
tap project my-saas-app

# Stop everything for a project
tap stop my-saas-app

# Clean up orphaned processes and stopped containers
tap clean

# Export a shareable snapshot (services, toolchain versions, OS)
tap export
tap export my-saas-app

# Register current directory as a project
tap init

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
4. **Dev filtering** — hides system processes, keeps dev tools and anything with an open port
5. **Project attribution** — walks each process's working directory up the filesystem tree looking for project markers, and groups processes by project root. If a process's own CWD doesn't match, walks the parent process chain.

### Supported ecosystems

Tap recognizes project markers and runtimes for:

- **JavaScript/TypeScript** — package.json, package-lock.json, yarn.lock, pnpm-lock.yaml, bun.lockb, node, bun, deno, npm, yarn, pnpm, vite, webpack, turbo, nodemon
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
| `tap` / `tap ls` | List all dev processes in a table |
| `tap ports` | List all ports in use, sorted by port number |
| `tap port <PORT>` | Show what's running on a specific port |
| `tap project <NAME>` | Show all processes for a specific project |
| `tap kill <PID>` | Kill a process by PID |
| `tap kill :<PORT>` | Kill whatever's listening on a port |
| `tap stop <NAME>` | Stop all processes for a project (with confirmation) |
| `tap clean` | Find long-running processes and stopped containers, offer to remove |
| `tap init` | Register current directory as a project (creates `.tap.toml`) |
| `tap export [NAME]` | Export a shareable snapshot with services, system info, toolchain versions |

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
- [x] Process tree walking (parent chain) for better attribution
- [x] `tap ls`, `tap ports`, `tap port`, `tap kill`
- [x] `tap project`, `tap stop`, `tap clean`, `tap init`, `tap export`
- [x] Docker container discovery
- [x] JSON output
- [x] Broad ecosystem support (JS, Python, Go, Rust, Ruby, Java, PHP, .NET, Elixir, C++)
- [ ] Interactive TUI dashboard (bubbletea)
- [ ] Global config file (`~/.config/tap/config.toml`)
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
│   ├── ls.go                  # tap ls (default command)
│   ├── ports.go               # tap ports
│   ├── port.go                # tap port <PORT>
│   ├── kill.go                # tap kill
│   ├── project.go             # tap project <NAME>
│   ├── stop.go                # tap stop <NAME>
│   ├── clean.go               # tap clean
│   ├── init_cmd.go            # tap init
│   └── export.go              # tap export
└── internal/
    ├── model/                 # Data types (DevProcess, Project, Snapshot)
    └── discovery/             # System scanning
        ├── processes.go       # Process enumeration (gopsutil)
        ├── ports.go           # Port-to-PID mapping (gopsutil)
        ├── docker.go          # Docker container discovery
        ├── projects.go        # Project root detection + parent chain attribution
        └── snapshot.go        # Aggregator + dev filter
```

## License

MIT
