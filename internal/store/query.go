package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/tfvjr/tap/internal/discovery"
	"github.com/tfvjr/tap/internal/model"
)

// LatestSnapshot reconstructs a Snapshot from the most recent poll cycle's rows.
// If curated is true, unattributed processes are filtered to known dev tools only.
func (s *Store) LatestSnapshot(curated bool) (*model.Snapshot, error) {
	var takenAt string
	err := s.db.QueryRow("SELECT taken_at FROM snapshots ORDER BY id DESC LIMIT 1").Scan(&takenAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return s.snapshotAt(takenAt, curated)
}

// SnapshotsBetween returns snapshots taken between from and to.
func (s *Store) SnapshotsBetween(from, to time.Time) ([]model.Snapshot, error) {
	fromStr := from.Format(time.RFC3339Nano)
	toStr := to.Format(time.RFC3339Nano)

	rows, err := s.db.Query(
		"SELECT DISTINCT taken_at FROM snapshots WHERE taken_at >= ? AND taken_at <= ? ORDER BY taken_at",
		fromStr, toStr,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var takenAts []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		takenAts = append(takenAts, t)
	}

	var snaps []model.Snapshot
	for _, t := range takenAts {
		snap, err := s.snapshotAt(t, false)
		if err != nil {
			return nil, err
		}
		if snap != nil {
			snaps = append(snaps, *snap)
		}
	}

	return snaps, nil
}

// ProcessHistory returns processes matching name/project within a time range.
func (s *Store) ProcessHistory(name, project string, since time.Duration) ([]model.DevProcess, error) {
	cutoff := time.Now().Add(-since).Format(time.RFC3339Nano)

	query := `SELECT pid, ppid, name, command, project, project_path, kind,
		ports_json, cpu_percent, memory_bytes, start_time, health_json, container_json
		FROM snapshots WHERE taken_at > ?`
	args := []interface{}{cutoff}

	if name != "" {
		query += " AND name = ?"
		args = append(args, name)
	}
	if project != "" {
		query += " AND project = ?"
		args = append(args, project)
	}

	query += " ORDER BY taken_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var procs []model.DevProcess
	for rows.Next() {
		proc, err := scanProcess(rows)
		if err != nil {
			return nil, err
		}
		procs = append(procs, proc)
	}

	return procs, nil
}

// ProjectList returns the names of distinct projects seen in the latest snapshot.
func (s *Store) ProjectList() ([]string, error) {
	var takenAt string
	err := s.db.QueryRow("SELECT taken_at FROM snapshots ORDER BY id DESC LIMIT 1").Scan(&takenAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	rows, err := s.db.Query(
		"SELECT DISTINCT project FROM snapshots WHERE taken_at = ? AND project != '' ORDER BY project",
		takenAt,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		projects = append(projects, name)
	}

	return projects, nil
}

// ProcessesByPort returns processes from the latest snapshot that are listening
// on the given port.
func (s *Store) ProcessesByPort(port uint16) ([]model.DevProcess, error) {
	snap, err := s.LatestSnapshot(false)
	if err != nil || snap == nil {
		return nil, err
	}

	var result []model.DevProcess
	for _, proj := range snap.Projects {
		for _, proc := range proj.Processes {
			if processHasPort(proc, port) {
				result = append(result, proc)
			}
		}
	}
	for _, proc := range snap.Unattributed {
		if processHasPort(proc, port) {
			result = append(result, proc)
		}
	}

	return result, nil
}

// SnapshotSummary is a compact summary of a snapshot for history display.
type SnapshotSummary struct {
	TakenAt      time.Time `json:"taken_at"`
	ProcessCount int       `json:"process_count"`
	ProjectCount int       `json:"project_count"`
	SystemCPU    float64   `json:"system_cpu"`
	MemUsedPct   float64   `json:"memory_used_percent"`
	MemUsedMB    float64   `json:"memory_used_mb"`
}

// SnapshotSummaries returns compact summaries of recent snapshots.
func (s *Store) SnapshotSummaries(project string, since time.Duration, limit int) ([]SnapshotSummary, error) {
	cutoff := time.Now().Add(-since).Format(time.RFC3339Nano)

	query := `
		SELECT s.taken_at,
			COUNT(*) as proc_count,
			COUNT(DISTINCT CASE WHEN s.project != '' THEN s.project END) as proj_count,
			COALESCE(ss.cpu_percent, 0),
			COALESCE(ss.mem_total, 0),
			COALESCE(ss.mem_used, 0)
		FROM snapshots s
		LEFT JOIN system_stats ss ON s.taken_at = ss.taken_at
		WHERE s.taken_at > ?
	`
	args := []interface{}{cutoff}

	if project != "" {
		query += " AND s.project = ?"
		args = append(args, project)
	}

	query += " GROUP BY s.taken_at ORDER BY s.taken_at DESC"
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []SnapshotSummary
	for rows.Next() {
		var takenAtStr string
		var sm SnapshotSummary
		var memTotal, memUsed int64

		if err := rows.Scan(&takenAtStr, &sm.ProcessCount, &sm.ProjectCount, &sm.SystemCPU, &memTotal, &memUsed); err != nil {
			return nil, err
		}

		sm.TakenAt, _ = time.Parse(time.RFC3339Nano, takenAtStr)
		if memTotal > 0 {
			sm.MemUsedPct = float64(memUsed) / float64(memTotal) * 100
		}
		sm.MemUsedMB = float64(memUsed) / (1024 * 1024)
		summaries = append(summaries, sm)
	}

	return summaries, nil
}

func (s *Store) snapshotAt(takenAt string, curated bool) (*model.Snapshot, error) {
	rows, err := s.db.Query(`
		SELECT pid, ppid, name, command, project, project_path, kind,
			ports_json, cpu_percent, memory_bytes, start_time, health_json, container_json
		FROM snapshots WHERE taken_at = ?
	`, takenAt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	projectMap := make(map[string]*model.Project)
	var unattributed []model.DevProcess

	for rows.Next() {
		proc, err := scanProcess(rows)
		if err != nil {
			return nil, err
		}

		if proc.Project != "" {
			key := strings.ToLower(proc.ProjectPath)
			if key == "" {
				key = strings.ToLower(proc.Project)
			}
			proj, exists := projectMap[key]
			if !exists {
				proj = &model.Project{
					Name: proc.Project,
					Path: proc.ProjectPath,
				}
				projectMap[key] = proj
			}
			proj.Processes = append(proj.Processes, proc)
		} else {
			unattributed = append(unattributed, proc)
		}
	}

	if curated {
		unattributed = filterUnattributedProcs(unattributed)
	}

	projects := make([]model.Project, 0, len(projectMap))
	for _, proj := range projectMap {
		proj.Recalculate()
		projects = append(projects, *proj)
	}

	ts, _ := time.Parse(time.RFC3339Nano, takenAt)
	snap := &model.Snapshot{
		Timestamp:    ts,
		Projects:     projects,
		Unattributed: unattributed,
	}

	// Load system stats.
	var cpuPct sql.NullFloat64
	var memTotal, memUsed sql.NullInt64
	err = s.db.QueryRow(
		"SELECT cpu_percent, mem_total, mem_used FROM system_stats WHERE taken_at = ?", takenAt,
	).Scan(&cpuPct, &memTotal, &memUsed)
	if err == nil {
		snap.SystemCPU = cpuPct.Float64
		snap.SystemMemoryTotal = uint64(memTotal.Int64)
		snap.SystemMemoryUsed = uint64(memUsed.Int64)
	}

	return snap, nil
}

func scanProcess(rows *sql.Rows) (model.DevProcess, error) {
	var proc model.DevProcess
	var pid sql.NullInt32
	var ppid int32
	var name, kind string
	var command, project, projectPath sql.NullString
	var portsJSON, startTimeStr, healthJSON, containerJSON sql.NullString
	var cpuPct float64
	var memBytes int64

	err := rows.Scan(&pid, &ppid, &name, &command, &project, &projectPath, &kind,
		&portsJSON, &cpuPct, &memBytes, &startTimeStr, &healthJSON, &containerJSON)
	if err != nil {
		return proc, err
	}

	if pid.Valid {
		v := pid.Int32
		proc.PID = &v
	}
	proc.PPID = ppid
	proc.Name = name
	proc.Command = command.String
	proc.Project = project.String
	proc.ProjectPath = projectPath.String
	proc.Kind = model.ProcessKind(kind)
	proc.CPUPercent = cpuPct
	proc.MemoryBytes = uint64(memBytes)

	if startTimeStr.Valid && startTimeStr.String != "" {
		proc.StartTime, _ = time.Parse(time.RFC3339Nano, startTimeStr.String)
	}

	if portsJSON.Valid && portsJSON.String != "" {
		json.Unmarshal([]byte(portsJSON.String), &proc.Ports)
	}

	if healthJSON.Valid && healthJSON.String != "" {
		var health model.ProcessHealth
		if json.Unmarshal([]byte(healthJSON.String), &health) == nil {
			proc.Health = &health
		}
	}

	if containerJSON.Valid && containerJSON.String != "" {
		var ci model.ContainerInfo
		if json.Unmarshal([]byte(containerJSON.String), &ci) == nil {
			proc.ContainerInfo = &ci
		}
	}

	return proc, nil
}

// filterUnattributedProcs filters unattributed processes to keep only containers
// and known dev tools. Mirrors the curated logic from discovery.
func filterUnattributedProcs(procs []model.DevProcess) []model.DevProcess {
	filtered := make([]model.DevProcess, 0, len(procs))
	for i := range procs {
		if procs[i].Kind != model.ProcessNative || discovery.IsKnownDevName(procs[i].Name) {
			filtered = append(filtered, procs[i])
		}
	}
	return filtered
}

func processHasPort(proc model.DevProcess, port uint16) bool {
	for _, pb := range proc.Ports {
		if pb.Port == port {
			return true
		}
	}
	if proc.ContainerInfo != nil {
		for _, pm := range proc.ContainerInfo.PortMappings {
			if pm.HostPort == port {
				return true
			}
		}
	}
	return false
}
