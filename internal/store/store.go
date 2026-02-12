package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Store wraps a SQLite database for tap's persistent state.
type Store struct {
	db     *sql.DB
	dbPath string
}

// Open opens (or creates) the tap database in the given data directory.
// It enables WAL mode and initializes the schema.
func Open(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	dbPath := filepath.Join(dataDir, "tap.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Enable WAL mode for concurrent reads.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable WAL: %w", err)
	}

	// Set busy timeout for concurrent access.
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}

	s := &Store{db: db, dbPath: dbPath}
	if err := s.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return s, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// DBPath returns the path to the database file.
func (s *Store) DBPath() string {
	return s.dbPath
}

// DB returns the underlying *sql.DB for advanced queries.
func (s *Store) DB() *sql.DB {
	return s.db
}

// HasData returns true if there is at least one snapshot row.
func (s *Store) HasData() (bool, error) {
	var exists int
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM snapshots LIMIT 1)").Scan(&exists)
	return exists == 1, err
}

// GetMeta retrieves a value from the meta key-value table.
func (s *Store) GetMeta(key string) (string, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM meta WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// SetMeta sets a value in the meta key-value table.
func (s *Store) SetMeta(key, value string) error {
	_, err := s.db.Exec("INSERT OR REPLACE INTO meta (key, value) VALUES (?, ?)", key, value)
	return err
}

// RowCounts returns the number of rows in the main tables.
func (s *Store) RowCounts() (snapshots, systemStats, logs int, err error) {
	s.db.QueryRow("SELECT COUNT(*) FROM snapshots").Scan(&snapshots)
	s.db.QueryRow("SELECT COUNT(*) FROM system_stats").Scan(&systemStats)
	s.db.QueryRow("SELECT COUNT(*) FROM logs").Scan(&logs)
	return
}

func (s *Store) initSchema() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS snapshots (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			taken_at       TEXT    NOT NULL,
			pid            INTEGER,
			ppid           INTEGER,
			name           TEXT    NOT NULL,
			command        TEXT,
			project        TEXT,
			project_path   TEXT,
			kind           TEXT    NOT NULL,
			ports_json     TEXT,
			cpu_percent    REAL,
			memory_bytes   INTEGER,
			start_time     TEXT,
			health_json    TEXT,
			container_json TEXT
		);

		CREATE INDEX IF NOT EXISTS idx_snapshots_taken_at ON snapshots(taken_at);
		CREATE INDEX IF NOT EXISTS idx_snapshots_project  ON snapshots(project);
		CREATE INDEX IF NOT EXISTS idx_snapshots_pid      ON snapshots(pid);
		CREATE INDEX IF NOT EXISTS idx_snapshots_name     ON snapshots(name);

		CREATE TABLE IF NOT EXISTS system_stats (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			taken_at    TEXT    NOT NULL,
			cpu_percent REAL,
			mem_total   INTEGER,
			mem_used    INTEGER
		);

		CREATE INDEX IF NOT EXISTS idx_system_stats_taken_at ON system_stats(taken_at);

		CREATE TABLE IF NOT EXISTS logs (
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp TEXT    NOT NULL,
			tap_id    TEXT    NOT NULL,
			project   TEXT,
			command   TEXT    NOT NULL,
			stream    TEXT    NOT NULL,
			line      TEXT    NOT NULL,
			seq       INTEGER NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_logs_tap_id    ON logs(tap_id);
		CREATE INDEX IF NOT EXISTS idx_logs_project   ON logs(project);
		CREATE INDEX IF NOT EXISTS idx_logs_timestamp ON logs(timestamp);

		CREATE TABLE IF NOT EXISTS meta (
			key   TEXT PRIMARY KEY,
			value TEXT
		);
	`)
	return err
}
