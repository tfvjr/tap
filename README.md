# tap

Replaces `lsof` + `ps` + `docker ps` with one project-aware command. Background collector keeps a persistent history in SQLite, captures console output, and exposes everything to AI tools via MCP.

```
$ tap
 ▼ my-saas-app                    3 services    CPU 13.6%    630 MB    2h
   ● :3000  node (next dev)                     CPU 12.3%    420 MB    2h
   ● :5432  postgres (docker: pg-main)          CPU  1.1%    180 MB    2h
   ● :6379  redis (docker: redis-cache)         CPU  0.2%     30 MB    2h

 ▼ side-project                   2 services    CPU  3.5%    240 MB   45m
   ● :8080  go (air)                            CPU  3.4%     90 MB   45m
   ● :5433  postgres (docker: pg-side)          CPU  0.1%    150 MB   45m

 ▼ unattributed
   ⚠ :8443  node                  ⚠ 3d uptime   CPU  5.0%    200 MB    3d
```

That `node` on `:3000`, the `postgres` on `:5432`, and the `redis` on `:6379` all belong to your SaaS app — not your side project. Tap figures that out by walking the filesystem for project markers (`package.json`, `go.mod`, `Cargo.toml`, etc.) and groups everything together.

Press Enter on any process and you get the full story:

```
 ● Port 3000 — node                                              ⚠ STALE
─────────────────────────────────────────────────────────────────────────────

 Process
   PID:       4521
   Command:   /usr/local/bin/node .next/server.js
   Uptime:    3d 4h 12m
   Memory:    284.2 MB

 Parent Chain
   └─ npm run dev (PID 4518)         ✗ DEAD
      └─ bash (PID 4510)             ✓ alive
         └─ tmux (PID 1200)          ✓ alive

 Diagnostics
   ⚠ Running for 3+ days
   ⚠ Parent process (npm run dev) is dead — probably orphaned
```

The parent that started it is dead. It's been running for 3 days. You probably forgot about it. Hit `k` to kill it.

## Install

```
go install github.com/tfvjr/tap@latest
```

Or `git clone && go build`. Works on Windows, macOS, Linux.

## Usage

```bash
tap                     # TUI dashboard
tap ls                  # plain table (for pipes/scripts)
tap port 3000           # what's on this port?
tap ports               # everything listening
tap kill :3000          # kill it
tap stop my-app         # stop a whole project
tap clean               # find stale stuff, offer to remove it
tap doctor              # what's eating my machine?
tap export              # shareable snapshot
tap init                # register current dir as a project
tap logs                # view captured output
tap history             # snapshot timeline
tap mcp                 # start MCP server for AI tools
tap teardown            # remove shims and restore shell config
```

`--json` for scripting. `--verbose` for full command lines.

## Background Collector

On first run, tap starts a background collector that snapshots your system every 5 seconds and stores results in `~/.tap/tap.db` (SQLite with WAL mode). All commands read from the database instead of scanning live — this makes them instant and enables history.

The collector auto-starts when you run any tap command. Check its status with `tap doctor`.

## Console Capture

On first run, tap creates lightweight shims in `~/.tap/shims/` for common dev tools (`node`, `npm`, `python`, `go`, `cargo`, etc.) and prepends them to your PATH. Every dev command you run is transparently intercepted — output goes to your terminal normally AND gets stored in the database. No workflow changes needed.

Review captured output:

```bash
tap logs                    # all captured output
tap logs my-project         # filter by project
tap logs --since 5m         # last 5 minutes
tap logs --stream stderr    # just errors
tap logs -f                 # follow mode (like tail -f)
```

To remove shims and restore your shell config, run `tap teardown`.

## History

```bash
tap history                 # snapshot timeline
tap history my-project      # for a specific project
tap history --since 24h     # last 24 hours
```

## MCP Server

Tap exposes its data to AI tools via the [Model Context Protocol](https://modelcontextprotocol.io/):

```bash
claude mcp add -s user tap -- tap mcp
```

This gives Claude Code 6 tools:

| Tool | Parameters | Description |
|------|-----------|-------------|
| `tap_snapshot` | `project`, `curated` (bool, default true) | Latest system snapshot with all running dev processes, grouped by project |
| `tap_history` | `project`, `since` (e.g. "30m", "1h"), `limit` (default 50) | Historical snapshot summaries showing process counts, CPU, and memory over time |
| `tap_logs` | `project`, `stream` ("stdout"/"stderr"), `since`, `limit` (default 100), `search` | Captured console output with text search support |
| `tap_projects` | _(none)_ | List all known projects currently being tracked |
| `tap_health` | `project` | Health diagnostics — flags stale, orphaned, high-memory, and high-CPU processes |
| `tap_processes` | `project`, `port`, `name` | Process list with filtering by project, port, or name |

## `tap doctor`

```
$ tap doctor
System: CPU 22%  MEM 27.6/31.7 GB (87%)

Background Collector: running (PID 12345)
Database: 1200 snapshots, 50 system stats, 340 log lines
DB size: 2.1 MB (~/.tap/tap.db)

Resource usage by project:
  PROJECT       SERVICES  CPU    MEMORY
  my-saas-app   3         13.6%  630 MB
  side-project  2         3.5%   240 MB

Found 2 issue(s):
  ⚠ node.exe (PID 15332) has been running for 3d 2h
    → tap kill 15332
  ⚠ System memory is at 87%
    → tap ls  — check which processes are consuming memory
```

## How it decides what to show

The dashboard only shows dev stuff — it hides system noise. `tap port` and `tap ports` show everything regardless, because when you're asking "what's on port 9090" you want the answer.

Processes get flagged: ⚠ if they've been running >24h, if their parent process is dead, or if they're eating >1GB memory or >50% CPU.

## Building

Go 1.24+.

```bash
go build -o tap .
```

## License

MIT
