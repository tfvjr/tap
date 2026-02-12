package daemon

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"time"

	"github.com/tfvjr/tap/internal/discovery"
	"github.com/tfvjr/tap/internal/store"
)

// RunCollector runs the background collector loop. It takes snapshots every
// 5 seconds, persists them to SQLite, and prunes old data.
func RunCollector(dataDir string) error {
	s, err := store.Open(dataDir)
	if err != nil {
		return err
	}
	defer s.Close()

	// Write PID file.
	pidFile := filepath.Join(dataDir, "tap.pid")
	os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0644)
	defer os.Remove(pidFile)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		<-sigCh
		cancel()
	}()

	// First poll immediately.
	poll(s)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			poll(s)
		}
	}
}

func poll(s *store.Store) {
	snap, err := discovery.TakeSnapshot(true, false)
	if err != nil {
		return // best-effort
	}
	s.PersistSnapshot(snap)
	s.Prune(24 * time.Hour)
}
