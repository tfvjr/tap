package store

import (
	"database/sql"
	"time"
)

// LogLine represents a single line of captured console output.
type LogLine struct {
	Timestamp time.Time `json:"timestamp"`
	TapID     string    `json:"tap_id"`
	Project   string    `json:"project,omitempty"`
	Command   string    `json:"command"`
	Stream    string    `json:"stream"`
	Line      string    `json:"line"`
	Seq       int64     `json:"seq"`
}

// InsertLogLine inserts a single log line captured by tap run.
func (s *Store) InsertLogLine(timestamp time.Time, tapID, project, command, stream, line string, seq int64) error {
	_, err := s.db.Exec(`
		INSERT INTO logs (timestamp, tap_id, project, command, stream, line, seq)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, timestamp.Format(time.RFC3339Nano), tapID, project, command, stream, line, seq)
	return err
}

// QueryLogs returns log lines matching the given filters.
func (s *Store) QueryLogs(tapID, stream string, since time.Time, limit int) ([]LogLine, error) {
	query := "SELECT timestamp, tap_id, project, command, stream, line, seq FROM logs WHERE 1=1"
	var args []interface{}

	if tapID != "" {
		query += " AND tap_id = ?"
		args = append(args, tapID)
	}
	if stream != "" {
		query += " AND stream = ?"
		args = append(args, stream)
	}
	if !since.IsZero() {
		query += " AND timestamp > ?"
		args = append(args, since.Format(time.RFC3339Nano))
	}

	query += " ORDER BY timestamp ASC, seq ASC"
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	return s.queryLogRows(query, args...)
}

// QueryLogsByProject returns log lines for a specific project.
func (s *Store) QueryLogsByProject(project string, since time.Time, limit int) ([]LogLine, error) {
	query := "SELECT timestamp, tap_id, project, command, stream, line, seq FROM logs WHERE project = ?"
	args := []interface{}{project}

	if !since.IsZero() {
		query += " AND timestamp > ?"
		args = append(args, since.Format(time.RFC3339Nano))
	}

	query += " ORDER BY timestamp ASC, seq ASC"
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	return s.queryLogRows(query, args...)
}

// QueryLogsAfter returns log lines after the given seq for follow mode.
func (s *Store) QueryLogsAfter(lastSeq int64, project, stream string) ([]LogLine, error) {
	query := "SELECT timestamp, tap_id, project, command, stream, line, seq FROM logs WHERE seq > ?"
	args := []interface{}{lastSeq}

	if project != "" {
		query += " AND project = ?"
		args = append(args, project)
	}
	if stream != "" {
		query += " AND stream = ?"
		args = append(args, stream)
	}

	query += " ORDER BY seq ASC"
	return s.queryLogRows(query, args...)
}

// MaxLogSeq returns the highest seq number in the logs table.
func (s *Store) MaxLogSeq() (int64, error) {
	var seq sql.NullInt64
	err := s.db.QueryRow("SELECT MAX(seq) FROM logs").Scan(&seq)
	if err != nil || !seq.Valid {
		return 0, err
	}
	return seq.Int64, nil
}

// PruneLogs keeps only the last maxLines lines per tap_id.
func (s *Store) PruneLogs(tapID string, maxLines int) error {
	_, err := s.db.Exec(`
		DELETE FROM logs WHERE tap_id = ? AND id NOT IN (
			SELECT id FROM logs WHERE tap_id = ? ORDER BY seq DESC LIMIT ?
		)
	`, tapID, tapID, maxLines)
	return err
}

func (s *Store) queryLogRows(query string, args ...interface{}) ([]LogLine, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lines []LogLine
	for rows.Next() {
		var l LogLine
		var ts string
		if err := rows.Scan(&ts, &l.TapID, &l.Project, &l.Command, &l.Stream, &l.Line, &l.Seq); err != nil {
			return nil, err
		}
		l.Timestamp, _ = time.Parse(time.RFC3339Nano, ts)
		lines = append(lines, l)
	}

	return lines, nil
}
