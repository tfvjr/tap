package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tfvjr/tap/internal/store"
)

// Serve creates the MCP server, registers all tap tools, and serves over stdio.
func Serve(s *store.Store) error {
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "tap",
		Version: "1.0.0",
	}, nil)

	// tap_snapshot — latest system state
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "tap_snapshot",
		Description: "Get the latest system snapshot showing all running dev processes, grouped by project, with CPU/memory stats",
	}, handleSnapshot(s))

	// tap_history — historical snapshots
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "tap_history",
		Description: "Get historical snapshot summaries showing process counts, CPU, and memory over time",
	}, handleHistory(s))

	// tap_logs — console output
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "tap_logs",
		Description: "Get console output captured by 'tap run'. Returns stdout/stderr lines with timestamps.",
	}, handleLogs(s))

	// tap_projects — list known projects
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "tap_projects",
		Description: "List all known projects currently being tracked",
	}, handleProjects(s))

	// tap_health — health diagnostics
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "tap_health",
		Description: "Get health diagnostics for processes — flags stale, orphaned, high-memory, and high-CPU processes",
	}, handleHealth(s))

	// tap_processes — process list
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "tap_processes",
		Description: "List running dev processes with details. Can filter by project, port, or name.",
	}, handleProcesses(s))

	return srv.Run(context.Background(), &mcp.StdioTransport{})
}
