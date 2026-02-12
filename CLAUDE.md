# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Is

Tap is a developer process observatory — replaces `lsof` + `ps` + `docker ps` with one project-aware command. A background collector snapshots the system every 5 seconds into SQLite, captures console output via PATH shims, and exposes everything to AI tools via MCP.

## Build & Run

```bash
go build -o tap .       # build (requires Go 1.24+)
go vet ./...            # lint
go test ./...           # test
./tap                   # run TUI dashboard
./tap mcp               # start MCP server (stdio)
```

No CGo required — uses `modernc.org/sqlite` (pure Go SQLite).

## Architecture

**Entry point:** `main.go` → `cmd.Execute()` (Cobra CLI)

**Internal packages** (`internal/`):

| Package | Purpose |
|---------|---------|
| `model` | Data structures: `DevProcess`, `ProcessHealth`, `PortBinding`, `ContainerInfo`, `Project`, `Snapshot` |
| `discovery` | Process enumeration (gopsutil), port mapping, Docker containers, project attribution via CWD + marker files, health analysis |
| `store` | SQLite persistence (`~/.tap/tap.db`), WAL mode, tables: `snapshots`, `system_stats`, `logs`, `meta` |
| `daemon` | Background collector lifecycle — 5s poll loop, auto-start via `daemon.EnsureRunning()`, platform-specific detachment (`detach_windows.go` / `detach_unix.go`) |
| `capture` | Console output capture — `RunAndCapture` tees stdout/stderr to terminal + SQLite |
| `shim` | PATH shimming — installs shims in `~/.tap/shims/` for dev tools (node, npm, python, go, cargo, etc.), resolves real binaries |
| `mcp` | MCP server — 6 tools (tap_snapshot, tap_history, tap_logs, tap_projects, tap_health, tap_processes) over stdio |
| `tui` | Terminal UI — bubbletea + lipgloss dashboard with project grouping and process detail view |

**CLI commands** (`cmd/`): Each file is one Cobra command. `root.go` opens the store in `PersistentPreRunE` and auto-starts the collector. Hidden commands `_collect` and `_shim` are used internally by the daemon and shim system.

**Data flow:**
```
gopsutil (processes + ports) + Docker API
  → Project attribution (CWD walking + marker files)
  → Dev filtering (first pass: known names + port listeners; curated: drop unattributed)
  → Health analysis (stale >24h, orphaned parent, high mem/cpu)
  → SQLite (collector writes every 5s)
  → TUI / CLI / MCP (all read from DB, never live-query)
```

## Key Patterns

- **Global store:** `appStore` (*store.Store) is opened in root `PersistentPreRunE`, closed in `PersistentPostRunE`. All commands access it.
- **Auto-start:** Every command (except `_collect`, `_shim`) calls `daemon.EnsureRunning()` to start the background collector if needed.
- **Platform handling:** Windows-specific code uses build tags (`detach_windows.go` / `detach_unix.go`). Path normalization and `DETACHED_PROCESS` flag for background processes on Windows.
- **Graceful degradation:** Docker errors, capture failures, and missing data never crash — the tool always shows what it can.
- **Output flags:** `--json` for machine-readable output, `--verbose` for full details, consistent across commands.
- **Data dir:** Configurable via `--data-dir` flag, defaults to `~/.tap/`.
