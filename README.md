# tap

Replaces `lsof` + `ps` + `docker ps` with one project-aware command.

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

Or `git clone && go build`. Works on Windows, macOS, Linux. Binaries on the [releases page](https://github.com/tfvjr/tap/releases).

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
```

`--json` for scripting. `--no-docker` to skip containers. `--verbose` for full command lines.

## `tap doctor`

```
$ tap doctor
System: CPU 22%  MEM 27.6/31.7 GB (87%)

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
go build -o tap .                       # dev
go build -ldflags="-s -w" -o tap .      # release (~8.4MB)
```

## License

MIT
