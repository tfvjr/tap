package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tfvjr/tap/internal/model"
	"github.com/tfvjr/tap/internal/store"
)

// textResult is a shorthand for returning a text CallToolResult.
func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: text},
		},
	}
}

// --- tap_snapshot ---

type snapshotArgs struct {
	Project string `json:"project" jsonschema:"Filter to a specific project name"`
	Curated *bool  `json:"curated" jsonschema:"Apply curated filtering (default true) - hides non-dev system processes"`
}

func handleSnapshot(s *store.Store) mcp.ToolHandlerFor[snapshotArgs, any] {
	return func(ctx context.Context, request *mcp.CallToolRequest, args snapshotArgs) (*mcp.CallToolResult, any, error) {
		curated := true
		if args.Curated != nil {
			curated = *args.Curated
		}

		snap, err := s.LatestSnapshot(curated)
		if err != nil {
			return nil, nil, fmt.Errorf("query snapshot: %w", err)
		}
		if snap == nil {
			return textResult("No snapshot data available yet. The collector may still be starting."), nil, nil
		}

		if args.Project != "" {
			snap = filterSnapshotByProject(snap, args.Project)
		}

		data, _ := json.MarshalIndent(snap, "", "  ")
		return textResult(string(data)), nil, nil
	}
}

// --- tap_history ---

type historyArgs struct {
	Project string  `json:"project" jsonschema:"Filter to a specific project"`
	Since   string  `json:"since" jsonschema:"Time range, e.g. '30m', '1h', '24h' (default '1h')"`
	Limit   float64 `json:"limit" jsonschema:"Maximum number of entries to return (default 50)"`
}

func handleHistory(s *store.Store) mcp.ToolHandlerFor[historyArgs, any] {
	return func(ctx context.Context, request *mcp.CallToolRequest, args historyArgs) (*mcp.CallToolResult, any, error) {
		since := 1 * time.Hour
		if args.Since != "" {
			if d, err := time.ParseDuration(args.Since); err == nil {
				since = d
			}
		}

		limit := 50
		if args.Limit > 0 {
			limit = int(args.Limit)
		}

		summaries, err := s.SnapshotSummaries(args.Project, since, limit)
		if err != nil {
			return nil, nil, fmt.Errorf("query history: %w", err)
		}

		if len(summaries) == 0 {
			return textResult("No history data available yet."), nil, nil
		}

		data, _ := json.MarshalIndent(summaries, "", "  ")
		return textResult(string(data)), nil, nil
	}
}

// --- tap_logs ---

type logsArgs struct {
	Project string  `json:"project" jsonschema:"Filter by project name"`
	Stream  string  `json:"stream" jsonschema:"Filter by stream: 'stdout' or 'stderr'"`
	Since   string  `json:"since" jsonschema:"Time range, e.g. '5m', '1h'"`
	Limit   float64 `json:"limit" jsonschema:"Maximum lines to return (default 100)"`
	Search  string  `json:"search" jsonschema:"Search for lines containing this text"`
}

func handleLogs(s *store.Store) mcp.ToolHandlerFor[logsArgs, any] {
	return func(ctx context.Context, request *mcp.CallToolRequest, args logsArgs) (*mcp.CallToolResult, any, error) {
		var sinceTime time.Time
		if args.Since != "" {
			if d, err := time.ParseDuration(args.Since); err == nil {
				sinceTime = time.Now().Add(-d)
			}
		}

		limit := 100
		if args.Limit > 0 {
			limit = int(args.Limit)
		}

		var lines []store.LogLine
		var err error

		if args.Project != "" {
			lines, err = s.QueryLogsByProject(args.Project, sinceTime, limit)
		} else {
			lines, err = s.QueryLogs("", args.Stream, sinceTime, limit)
		}
		if err != nil {
			return nil, nil, fmt.Errorf("query logs: %w", err)
		}

		if args.Search != "" {
			var filtered []store.LogLine
			for _, l := range lines {
				if strings.Contains(l.Line, args.Search) {
					filtered = append(filtered, l)
				}
			}
			lines = filtered
		}

		if len(lines) == 0 {
			return textResult("No logs found. Use `tap run <command>` to capture output."), nil, nil
		}

		data, _ := json.MarshalIndent(lines, "", "  ")
		return textResult(string(data)), nil, nil
	}
}

// --- tap_projects ---

type projectsArgs struct{}

func handleProjects(s *store.Store) mcp.ToolHandlerFor[projectsArgs, any] {
	return func(ctx context.Context, request *mcp.CallToolRequest, args projectsArgs) (*mcp.CallToolResult, any, error) {
		projects, err := s.ProjectList()
		if err != nil {
			return nil, nil, fmt.Errorf("query projects: %w", err)
		}

		if len(projects) == 0 {
			return textResult("No projects found. Start some dev processes to see them here."), nil, nil
		}

		data, _ := json.MarshalIndent(projects, "", "  ")
		return textResult(string(data)), nil, nil
	}
}

// --- tap_health ---

type healthArgs struct {
	Project string `json:"project" jsonschema:"Filter to a specific project"`
}

func handleHealth(s *store.Store) mcp.ToolHandlerFor[healthArgs, any] {
	return func(ctx context.Context, request *mcp.CallToolRequest, args healthArgs) (*mcp.CallToolResult, any, error) {
		snap, err := s.LatestSnapshot(true)
		if err != nil {
			return nil, nil, fmt.Errorf("query snapshot: %w", err)
		}
		if snap == nil {
			return textResult("No snapshot data available yet."), nil, nil
		}

		if args.Project != "" {
			snap = filterSnapshotByProject(snap, args.Project)
		}

		type healthIssue struct {
			Name    string   `json:"name"`
			PID     *int32   `json:"pid,omitempty"`
			Project string   `json:"project,omitempty"`
			Flags   []string `json:"flags"`
		}

		var issues []healthIssue
		collectIssues := func(procs []model.DevProcess) {
			for _, proc := range procs {
				if proc.Health != nil && !proc.Health.IsHealthy() {
					flags := make([]string, len(proc.Health.Flags))
					for i, f := range proc.Health.Flags {
						flags[i] = string(f)
					}
					issues = append(issues, healthIssue{
						Name:    proc.Name,
						PID:     proc.PID,
						Project: proc.Project,
						Flags:   flags,
					})
				}
			}
		}

		for _, proj := range snap.Projects {
			collectIssues(proj.Processes)
		}
		collectIssues(snap.Unattributed)

		if len(issues) == 0 {
			return textResult("All processes are healthy. No issues detected."), nil, nil
		}

		data, _ := json.MarshalIndent(issues, "", "  ")
		return textResult(string(data)), nil, nil
	}
}

// --- tap_processes ---

type processesArgs struct {
	Project string  `json:"project" jsonschema:"Filter by project name"`
	Port    float64 `json:"port" jsonschema:"Find process on this port"`
	Name    string  `json:"name" jsonschema:"Filter by process name"`
}

func handleProcesses(s *store.Store) mcp.ToolHandlerFor[processesArgs, any] {
	return func(ctx context.Context, request *mcp.CallToolRequest, args processesArgs) (*mcp.CallToolResult, any, error) {
		if args.Port > 0 {
			procs, err := s.ProcessesByPort(uint16(args.Port))
			if err != nil {
				return nil, nil, fmt.Errorf("query by port: %w", err)
			}
			if len(procs) == 0 {
				return textResult(fmt.Sprintf("No process found on port %d.", int(args.Port))), nil, nil
			}
			data, _ := json.MarshalIndent(procs, "", "  ")
			return textResult(string(data)), nil, nil
		}

		snap, err := s.LatestSnapshot(true)
		if err != nil {
			return nil, nil, fmt.Errorf("query snapshot: %w", err)
		}
		if snap == nil {
			return textResult("No snapshot data available yet."), nil, nil
		}

		if args.Project != "" {
			snap = filterSnapshotByProject(snap, args.Project)
		}

		var procs []model.DevProcess
		for _, proj := range snap.Projects {
			for _, proc := range proj.Processes {
				if args.Name != "" && !strings.EqualFold(proc.Name, args.Name) {
					continue
				}
				procs = append(procs, proc)
			}
		}
		for _, proc := range snap.Unattributed {
			if args.Name != "" && !strings.EqualFold(proc.Name, args.Name) {
				continue
			}
			procs = append(procs, proc)
		}

		if len(procs) == 0 {
			msg := "No processes found"
			if args.Project != "" {
				msg += " for project " + args.Project
			}
			if args.Name != "" {
				msg += " with name " + args.Name
			}
			return textResult(msg + "."), nil, nil
		}

		data, _ := json.MarshalIndent(procs, "", "  ")
		return textResult(string(data)), nil, nil
	}
}

// --- helpers ---

func filterSnapshotByProject(snap *model.Snapshot, project string) *model.Snapshot {
	var filtered []model.Project
	for _, proj := range snap.Projects {
		if strings.EqualFold(proj.Name, project) {
			filtered = append(filtered, proj)
		}
	}
	snap.Projects = filtered
	snap.Unattributed = nil
	return snap
}
