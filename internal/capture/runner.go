package capture

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tfvjr/tap/internal/store"
)

// RunAndCapture spawns a child process, tees its stdout/stderr to the
// terminal, and writes each line to the store. It returns the child's
// exit code.
func RunAndCapture(ctx context.Context, command []string, tapID, project string, s *store.Store) (int, error) {
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 1, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return 1, fmt.Errorf("stderr pipe: %w", err)
	}

	cmd.Stdin = os.Stdin

	if err := cmd.Start(); err != nil {
		return 1, fmt.Errorf("start command: %w", err)
	}

	var wg sync.WaitGroup
	var seq atomic.Int64
	cmdStr := strings.Join(command, " ")

	capture := func(r io.Reader, stream string, w *os.File) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Fprintln(w, line)
			n := seq.Add(1)
			s.InsertLogLine(time.Now(), tapID, project, cmdStr, stream, line, n)
		}
	}

	wg.Add(2)
	go capture(stdout, "stdout", os.Stdout)
	go capture(stderr, "stderr", os.Stderr)

	// Forward signals to child process.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		for sig := range sigCh {
			cmd.Process.Signal(sig)
		}
	}()

	wg.Wait()
	err = cmd.Wait()
	signal.Stop(sigCh)
	close(sigCh)

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return 1, err
	}
	return 0, nil
}
