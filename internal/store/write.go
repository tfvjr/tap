package store

import (
	"encoding/json"
	"time"

	"github.com/tfvjr/tap/internal/model"
)

// PersistSnapshot writes all processes and system stats from a snapshot
// into the database in a single transaction.
func (s *Store) PersistSnapshot(snap *model.Snapshot) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	takenAt := snap.Timestamp.Format(time.RFC3339Nano)

	stmt, err := tx.Prepare(`
		INSERT INTO snapshots (taken_at, pid, ppid, name, command, project, project_path, kind, ports_json, cpu_percent, memory_bytes, start_time, health_json, container_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	// Insert all processes from projects and unattributed.
	var allProcs []model.DevProcess
	for _, proj := range snap.Projects {
		allProcs = append(allProcs, proj.Processes...)
	}
	allProcs = append(allProcs, snap.Unattributed...)

	for _, proc := range allProcs {
		portsJSON, _ := json.Marshal(proc.Ports)
		healthJSON, _ := json.Marshal(proc.Health)

		var containerJSON []byte
		if proc.ContainerInfo != nil {
			containerJSON, _ = json.Marshal(proc.ContainerInfo)
		}

		var startTime string
		if !proc.StartTime.IsZero() {
			startTime = proc.StartTime.Format(time.RFC3339Nano)
		}

		var containerStr *string
		if containerJSON != nil {
			s := string(containerJSON)
			containerStr = &s
		}

		_, err := stmt.Exec(
			takenAt,
			proc.PID,
			proc.PPID,
			proc.Name,
			proc.Command,
			proc.Project,
			proc.ProjectPath,
			string(proc.Kind),
			string(portsJSON),
			proc.CPUPercent,
			int64(proc.MemoryBytes),
			startTime,
			string(healthJSON),
			containerStr,
		)
		if err != nil {
			return err
		}
	}

	// Insert system stats.
	_, err = tx.Exec(`
		INSERT INTO system_stats (taken_at, cpu_percent, mem_total, mem_used)
		VALUES (?, ?, ?, ?)
	`, takenAt, snap.SystemCPU, int64(snap.SystemMemoryTotal), int64(snap.SystemMemoryUsed))
	if err != nil {
		return err
	}

	return tx.Commit()
}

// Prune deletes snapshot and system_stats rows older than the given duration.
func (s *Store) Prune(olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan).Format(time.RFC3339Nano)
	if _, err := s.db.Exec("DELETE FROM snapshots WHERE taken_at < ?", cutoff); err != nil {
		return err
	}
	_, err := s.db.Exec("DELETE FROM system_stats WHERE taken_at < ?", cutoff)
	return err
}
