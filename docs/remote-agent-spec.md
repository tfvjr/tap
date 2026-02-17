# Tap Remote Agent — Implementation Specification

## One-Liner

Self-replicating remote agent over SSH — one command copies tap to a server, captures console output from Docker/journald/shims, streams it back with sequence-based sync, and exposes everything to AI tools via MCP.

---

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Capture model | Shims (interactive) + Docker logs + journald | Shims cover interactive use; Docker/journald cover production services already running |
| Disconnect handling | Remote SQLite + sequence-based catch-up | No data loss during SSH drops; simple append-only protocol |
| Project model | Always separate, tagged with source | `my-app@api.myserver.com` — never merged with local, avoids confusion |
| Install method | Auto-copy binary over SSH | Matches zero-config identity; devs already scp binaries to servers |
| Sync scope | Logs + process snapshots | Full visibility — `tap ls` and `tap logs` both work for remote |
| Resource budget | Same as local (~30MB RAM, 5s polling) | Consistent behavior, predictable resource usage |
| Multi-user | Shared agent via Unix socket, SSH is auth | If you can SSH in, you can read the stream; no new identity layer |

---

## User Experience

### Connect to a Server

```bash
tap connect user@api.myserver.com
```

Output:
```
tap: copying tap binary to api.myserver.com...
tap: starting agent on api.myserver.com...
tap: connected — capturing output
```

One command. Uses existing SSH keys. No config files, no API keys, no accounts.

### Connect to Multiple Servers

```bash
tap connect user@api.myapp.com
tap connect user@worker.myapp.com
tap connect deploy@staging.myapp.com
```

Each connection is independent. All data flows into the same local SQLite.

### View Remote Logs

```bash
tap logs api.myapp.com              # all output from api server
tap logs api.myapp.com --since 5m   # last 5 minutes
tap logs api.myapp.com -f           # follow mode
tap logs --since 1h                 # all sources (local + all remotes)
```

### View Remote Processes

```bash
tap ls api.myapp.com                # processes on api server
tap ls                              # local only (default, no change)
tap ls --all                        # local + all remotes
```

### List Connections

```bash
tap connections
```

Output:
```
  HOST                    STATUS        SINCE       LAST SYNC
  api.myapp.com           connected     2h ago      3s ago
  worker.myapp.com        connected     2h ago      3s ago
  staging.myapp.com       reconnecting  1d ago      15m ago (SSH dropped)
```

### Disconnect

```bash
tap disconnect api.myapp.com            # stop streaming, leave agent running
tap disconnect api.myapp.com --clean    # also remove tap binary + data from remote
```

---

## Architecture

### Overview

```
LOCAL MACHINE                              REMOTE SERVER
─────────────                              ─────────────
tap connect user@host
    │
    ├── scp tap binary ──────────────────► ~/.tap/bin/tap
    │
    ├── ssh: tap _agent ─────────────────► tap _agent starts
    │                                          │
    │                                          ├── Process discovery (gopsutil)
    │                                          ├── Docker logs (Docker API)
    │                                          ├── journald (journalctl)
    │                                          ├── PATH shims (interactive)
    │                                          │
    │                                          ▼
    │                                      Remote SQLite (~/.tap/tap.db)
    │                                          │
    │   SSH stdout (NDJSON)                    │
    │◄─────────────────────────────────────────┘
    │
    ▼
Local SQLite
(logs + snapshots tagged with source)
    │
    ├──► tap logs api.myserver.com
    ├──► tap ls api.myserver.com
    └──► MCP tools (source parameter)
```

### Remote Side: `tap _agent`

Hidden command. Runs on the remote server as a persistent background process.

```
tap _agent
```

**Three capture sources running concurrently:**

#### 1. Process Discovery (existing code)

Reuses `discovery.TakeSnapshot()` — same 5-second polling as local collector. Writes snapshots to remote SQLite.

#### 2. Docker Log Reader

```go
type DockerLogReader struct {
    client *docker.Client
    store  *store.Store
}
```

- Lists all running containers via Docker API (already implemented in `discovery/docker.go`)
- Calls `ContainerLogs()` with `Follow: true` on each container
- Attributes container to project via `com.docker.compose.project.working_dir` label (already implemented)
- Writes each line to `logs` table with:
  - `tap_id`: `docker_<container_id>_<short_hash>`
  - `project`: inferred from Docker Compose label or container name
  - `command`: container image + command
  - `stream`: `stdout` or `stderr` (Docker multiplexes these)
- Watches for new/stopped containers, starts/stops readers accordingly

#### 3. Journald Reader

```go
type JournaldReader struct {
    store *store.Store
}
```

- Runs `journalctl --output=json --follow --since=now` (or since last checkpoint)
- Parses JSON output for `_SYSTEMD_UNIT`, `MESSAGE`, `PRIORITY` fields
- Filters to user services (excludes kernel, audit, etc.)
- Attributes to project by matching unit name to known project patterns or CWD
- Writes each line to `logs` table with:
  - `tap_id`: `journal_<unit_name>_<boot_id>`
  - `project`: inferred from unit name or working directory
  - `command`: systemd unit ExecStart value
  - `stream`: `stdout` or `stderr` based on `PRIORITY` (0-3 → stderr, 4+ → stdout)
- Falls back gracefully if journald is not available (e.g. Alpine, Docker-only hosts)

#### 4. PATH Shims (optional)

Same shim mechanism as local tap. Only captures commands typed interactively by users SSH'd into the server. Already implemented — `_agent` calls `shim.EnsureShims()` on startup.

#### Agent Lifecycle

- Detaches as background process (same mechanism as `_collect` via `daemon/detach_*.go`)
- PID file at `~/.tap/agent.pid`
- Listens on Unix socket at `~/.tap/agent.sock` for consumers
- Multiple SSH connections read from the same socket
- Stays alive after SSH disconnect
- Liveness checked via PID file on subsequent `tap connect`

#### Agent Stream Protocol

The agent streams newline-delimited JSON (NDJSON) over the Unix socket:

```json
{"type":"log","seq":1042,"ts":"2026-02-11T15:04:05Z","tap_id":"docker_abc123_f7e2","project":"my-app","cmd":"node server.js","stream":"stderr","line":"Error: ECONNREFUSED 127.0.0.1:5432"}
{"type":"snapshot","seq":1043,"ts":"2026-02-11T15:04:05Z","processes":[...],"system":{"cpu":45.2,"mem_total":8589934592,"mem_used":3221225472}}
{"type":"heartbeat","seq":1044,"ts":"2026-02-11T15:04:10Z","uptime":"4h32m","version":"0.1.0"}
```

Each message has a monotonically increasing `seq` number. This is the basis for catch-up sync.

### Local Side: Connection Manager

Runs as a background goroutine set, managed by the daemon process.

#### SSH Tunnel

```go
type Connection struct {
    Host       string
    User       string
    Status     string    // connected, reconnecting, disconnected
    LastSeq    int64     // last seq received from this source
    LastSync   time.Time
    SSHClient  *ssh.Client
}
```

- Maintains persistent SSH connection per remote host
- Reads NDJSON from remote agent's stdout
- Inserts into local SQLite with `source` tag
- Tracks `LastSeq` per source for catch-up

#### Catch-Up on Reconnect

When SSH connection is re-established after a drop:

1. Local tap sends: `{"type":"resume","last_seq":1042}`
2. Remote agent queries its SQLite: `SELECT * FROM logs WHERE seq > 1042 ORDER BY seq ASC`
3. Streams all missed records, then switches to live streaming
4. Local tap inserts the backfill, then continues as normal

No complex sync protocol. Just "give me everything after this number." The remote SQLite is the source of truth for gap recovery.

#### Reconnect Logic

```
Connection drops
    → Wait 1s, retry
    → Wait 2s, retry
    → Wait 4s, retry
    → ...
    → Cap at 60s
    → Keep retrying indefinitely
    → On success: run catch-up, resume live streaming
```

#### Connection Persistence

Active connections are stored in local SQLite so they survive `tap` restarts:

```sql
CREATE TABLE connections (
    host       TEXT PRIMARY KEY,
    user       TEXT NOT NULL,
    status     TEXT NOT NULL DEFAULT 'disconnected',
    last_seq   INTEGER NOT NULL DEFAULT 0,
    last_sync  TEXT,
    added_at   TEXT NOT NULL
);
```

On daemon startup, the connection manager reads this table and re-establishes all active connections.

---

## Schema Changes

### `logs` table — add `source` column

```sql
ALTER TABLE logs ADD COLUMN source TEXT NOT NULL DEFAULT 'local';
CREATE INDEX IF NOT EXISTS idx_logs_source ON logs(source);
```

Values: `local`, `api.myserver.com`, `worker.myserver.com`, etc.

### `snapshots` table — add `source` column

```sql
ALTER TABLE snapshots ADD COLUMN source TEXT NOT NULL DEFAULT 'local';
CREATE INDEX IF NOT EXISTS idx_snapshots_source ON snapshots(source);
```

### `connections` table — new

```sql
CREATE TABLE IF NOT EXISTS connections (
    host       TEXT PRIMARY KEY,
    user       TEXT NOT NULL,
    status     TEXT NOT NULL DEFAULT 'disconnected',
    last_seq   INTEGER NOT NULL DEFAULT 0,
    last_sync  TEXT,
    added_at   TEXT NOT NULL
);
```

### Migration

Schema changes applied via `initSchema()` in `store.go` using `CREATE TABLE IF NOT EXISTS` and `ALTER TABLE ... ADD COLUMN` with error suppression for idempotency (SQLite errors on duplicate column add — catch and ignore).

---

## Binary Distribution

The tap binary is a single static Go binary (no CGo, pure Go SQLite). This enables the self-replicating pattern.

### Cross-Compilation

Build platform variants as part of the release process:

```
tap-linux-amd64
tap-linux-arm64
tap-darwin-amd64
tap-darwin-arm64
```

### Remote Platform Detection

```bash
ssh user@host "uname -s && uname -m"
# → Linux x86_64
```

Map to the correct binary:

| uname -s | uname -m | Binary |
|----------|----------|--------|
| Linux | x86_64 | tap-linux-amd64 |
| Linux | aarch64 | tap-linux-arm64 |
| Darwin | x86_64 | tap-darwin-amd64 |
| Darwin | arm64 | tap-darwin-arm64 |

### Binary Storage

Cross-compiled binaries stored locally at `~/.tap/bin/`:

```
~/.tap/bin/tap-linux-amd64
~/.tap/bin/tap-linux-arm64
~/.tap/bin/tap-darwin-amd64
~/.tap/bin/tap-darwin-arm64
```

Built on first `tap connect` if not present (`go build` with `GOOS`/`GOARCH`), or downloaded from GitHub releases.

### Remote Installation Path

```
~/.tap/bin/tap          # the binary
~/.tap/tap.db           # agent's SQLite (for catch-up sync)
~/.tap/agent.pid        # agent PID file
~/.tap/agent.sock       # Unix socket for multi-consumer
~/.tap/shims/           # PATH shims (if enabled)
```

---

## New Commands

### `tap connect user@host`

```go
var connectCmd = &cobra.Command{
    Use:   "connect user@host",
    Short: "Connect to a remote server and start capturing",
}
```

**Flow:**
1. Parse `user@host` (support `user@host:port` for non-standard SSH ports)
2. Establish SSH connection using system SSH agent / keys (`~/.ssh/id_*`)
3. Detect remote platform: `uname -s && uname -m`
4. Check if tap exists on remote: `~/.tap/bin/tap --version`
5. If missing or outdated: `scp` the matching binary
6. Check if agent is running: `cat ~/.tap/agent.pid` + liveness check
7. If not running: start `tap _agent` in detached mode
8. Open stream from agent socket
9. Send `{"type":"resume","last_seq":<n>}` for catch-up
10. Insert connection record in local SQLite
11. Start background reader goroutine
12. Print confirmation

### `tap disconnect host [--clean]`

```go
var disconnectCmd = &cobra.Command{
    Use:   "disconnect host",
    Short: "Stop streaming from a remote server",
}
```

**Flow:**
1. Close SSH connection
2. Update connection status to `disconnected` in local SQLite
3. If `--clean`: SSH in, run `tap _agent --stop`, then `rm -rf ~/.tap/` on remote

### `tap connections`

```go
var connectionsCmd = &cobra.Command{
    Use:   "connections",
    Short: "List remote connections and their status",
}
```

**Output:** Table showing host, status, time since connected, last sync time.

### `tap _agent`

```go
var agentCmd = &cobra.Command{
    Use:    "_agent",
    Hidden: true,
    Short:  "Remote capture agent (internal)",
}
```

**Flags:**
- `--stop` — gracefully stop a running agent
- `--no-shims` — skip PATH shim installation
- `--no-docker` — skip Docker log reading
- `--no-journal` — skip journald reading

---

## New Files

```
cmd/
    connect.go                  # tap connect user@host
    disconnect.go               # tap disconnect host [--clean]
    connections.go              # tap connections
    agent.go                    # tap _agent (hidden)

internal/
    remote/
        ssh.go                  # SSH connection, platform detection, binary copy
        tunnel.go               # Connection manager, reconnect logic, catch-up
        stream.go               # NDJSON parser, local SQLite inserter

    agent/
        agent.go                # Agent main loop: start readers, manage socket
        docker_reader.go        # Docker container log reader (Docker API)
        journal_reader.go       # journald reader (journalctl --output=json)
        socket.go               # Unix socket server for multi-consumer streaming
```

## Modified Files

| File | Change |
|------|--------|
| `internal/store/store.go` | Add `source` column migration, `connections` table |
| `internal/store/logs.go` | Add `source` param to `InsertLogLine`, all query functions |
| `internal/store/query.go` | Add `source` filter to snapshot queries |
| `internal/store/write.go` | Add `source` param to `PersistSnapshot` |
| `cmd/logs_cmd.go` | Add `--source` flag, pass to queries |
| `cmd/ls.go` | Add source argument and `--all` flag |
| `cmd/root.go` | Start connection manager in PersistentPreRunE (reconnect active connections) |
| `internal/mcp/tools.go` | Add `source` parameter to all tool handlers |
| `internal/mcp/server.go` | Register `tap_connections` tool |
| `internal/capture/runner.go` | Add `source` param to `RunAndCapture` (default `"local"`) |
| `cmd/shim.go` | Pass `source: "local"` to capture calls |

---

## MCP Integration

### Updated Tools

All existing tools gain an optional `source` parameter:

| Tool | New Parameter | Behavior |
|------|--------------|----------|
| `tap_snapshot` | `source` (string) | Filter to specific remote host. Default: local only. |
| `tap_logs` | `source` (string) | Filter to specific remote host. Omit for all sources. |
| `tap_processes` | `source` (string) | Filter to specific remote host. Default: local only. |
| `tap_history` | `source` (string) | Filter to specific remote host. Default: local only. |
| `tap_health` | `source` (string) | Filter to specific remote host. Default: local only. |

### New Tool

| Tool | Parameters | Description |
|------|-----------|-------------|
| `tap_connections` | _(none)_ | List active remote connections, status, last sync time |

### Example AI Interaction

> **User:** My API server is throwing 500 errors
> **Claude:** calls `tap_connections` — sees `api.myapp.com` connected
> **Claude:** calls `tap_logs` with `source: "api.myapp.com"`, `stream: "stderr"`, `since: "10m"`
> **Claude:** "Your Express server is crashing with `TypeError: Cannot read property 'id' of undefined` at `routes/users.js:47`. The request handler isn't null-checking the user object from the database query."

---

## Multi-User Behavior

### Scenario: Three developers connect to the same server

1. **Dev A** runs `tap connect deploy@api.myapp.com`
   - Binary copied to remote (first time)
   - Agent started, PID file + Unix socket created
   - Dev A's local tap reads from socket via SSH

2. **Dev B** runs `tap connect deploy@api.myapp.com`
   - Binary already exists (skip copy)
   - Agent already running (detected via PID file)
   - Dev B's local tap reads from the same socket via SSH

3. **Dev C** runs `tap connect deploy@api.myapp.com`
   - Same as Dev B

Each developer has their own SSH connection, their own local SQLite, their own `LastSeq` tracking. The remote agent is shared. SSH credentials are the access control — no tap-level auth needed.

### Disconnect Behavior

- `tap disconnect` by one user doesn't affect others
- Agent stays running as long as any consumer is connected
- `tap disconnect --clean` should warn if other consumers are active

---

## Safety & Edge Cases

| Concern | Mitigation |
|---------|------------|
| SSH key not configured | Fail with clear error: "SSH auth failed — add your key to the remote host" |
| Remote is different OS/arch | Detect via `uname`, copy matching binary |
| Agent already running | Detect via PID file, connect to existing socket |
| SSH connection drops | Auto-reconnect with exponential backoff (1s → 60s cap) |
| Remote server reboots | Agent dies; next reconnect detects dead PID, restarts agent |
| Agent crash on remote | PID file stale; next connect detects, restarts |
| Docker not installed on remote | Docker reader skips gracefully, logs warning |
| journald not available | Journal reader skips gracefully (Alpine, containers) |
| Binary version mismatch | Check `tap --version` on remote, warn + offer update if mismatched |
| Disk space on remote | Remote SQLite has same 24-hour retention + prune as local |
| Bandwidth | NDJSON is compact; snapshots every 5s are small (~1KB each); log lines are variable |
| `--clean` with other users connected | Warn: "Agent has N other consumers — disconnect anyway?" |
| Firewall blocks SSH | Same port/access the developer already uses to manage the server |
| Remote tap in PATH conflicts | Agent binary lives in `~/.tap/bin/`, not in system PATH |

---

## Implementation Order

### Phase 1: Basic Remote Streaming

**Goal:** `tap connect` works, logs stream back, `tap logs <host>` shows them.

1. Add `source` column to `logs` and `snapshots` tables (migration in `store.go`)
2. Update all query functions to accept/filter by `source`
3. Implement `tap _agent` — process discovery + Docker log reader + NDJSON streaming to stdout
4. Implement `internal/remote/ssh.go` — SSH connect, platform detect, binary copy
5. Implement `tap connect` — full flow: SSH → install → start agent → read stream → insert locally
6. Implement `tap disconnect`
7. Implement `tap connections`
8. Add `--source` flag to `tap logs` and `tap ls`

### Phase 2: Resilience

**Goal:** Connections survive drops, no data loss.

9. Add remote SQLite to `_agent` (writes locally on server)
10. Implement sequence-based catch-up protocol (resume message → backfill → live)
11. Implement reconnect logic with exponential backoff
12. Store connections in local SQLite, auto-reconnect on daemon startup
13. Add `--clean` flag to `tap disconnect`

### Phase 3: Full Capture

**Goal:** Capture from all server log sources.

14. Implement journald reader (`journalctl --output=json --follow`)
15. Add Docker container start/stop watching (new containers get readers automatically)
16. Install PATH shims on remote for interactive capture
17. Add `--no-shims`, `--no-docker`, `--no-journal` flags to `_agent`

### Phase 4: MCP + Polish

**Goal:** AI tools can access remote data.

18. Add `source` parameter to all MCP tool handlers
19. Add `tap_connections` MCP tool
20. Version checking on connect (warn on mismatch, offer update)
21. `tap doctor` shows remote connection health

---

## What This Is Not

- **Not a log aggregator** — no indexing, no dashboards, no retention policies beyond 24h
- **Not a monitoring platform** — no alerts, no metrics, no SLOs, no paging
- **Not a deployment tool** — doesn't deploy code, restart services, or manage infrastructure
- **Not a SaaS** — no accounts, no cloud relay, no infrastructure to maintain or pay for

It's `tail -f` that works across machines, stores history, and feeds into your AI coding assistant. Nothing more.
