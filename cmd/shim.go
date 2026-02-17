package cmd

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	"github.com/tfvjr/tap/internal/capture"
	"github.com/tfvjr/tap/internal/discovery"
	"github.com/tfvjr/tap/internal/shim"
	"github.com/tfvjr/tap/internal/store"
)

var shimCmd = &cobra.Command{
	Use:    "_shim <command> [args...]",
	Short:  "Shim entry point for transparent output capture (internal)",
	Hidden: true,
	// Skip PersistentPreRunE — no daemon check, no snapshot wait.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Handle --help manually since flag parsing is disabled.
		for _, a := range args {
			if a == "--help" || a == "-h" {
				return cmd.Help()
			}
		}

		if len(args) == 0 {
			return fmt.Errorf("_shim requires a command name")
		}

		name := args[0]
		childArgs := args[1:]

		shimsDir := shim.ShimsDir(dataDir)

		// Resolve the real binary (PATH minus shims dir).
		realBin, err := shim.ResolveReal(name, shimsDir)
		if err != nil {
			// Fallback: try to run the command directly.
			// This handles the case where resolve fails but the command
			// might still work (e.g. built-in shell command).
			return execFallback(name, childArgs)
		}

		// Try to open the store and capture output.
		s, err := store.Open(dataDir)
		if err != nil {
			// Store failed — run the real binary without capture.
			return execDirect(realBin, childArgs)
		}
		defer s.Close()

		// Detect project from CWD.
		var project string
		cwd, _ := os.Getwd()
		if root, ok := discovery.DetectProjectRoot(cwd); ok {
			project = discovery.InferProjectName(root)
		}

		// Generate unique tap_id.
		tapID := fmt.Sprintf("shim_%s_%d_%04x", name, time.Now().Unix(), rand.Intn(0xFFFF))

		// Build the full command: real binary path + child args.
		fullCmd := append([]string{realBin}, childArgs...)

		exitCode, err := capture.RunAndCapture(context.Background(), fullCmd, tapID, project, s)
		if err != nil {
			return err
		}
		if exitCode != 0 {
			os.Exit(exitCode)
		}
		return nil
	},
}

// execDirect runs the real binary without capture, forwarding stdin/stdout/stderr.
// Used as fallback when store can't be opened.
func execDirect(bin string, args []string) error {
	c := exec.Command(bin, args...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	if err := c.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return err
	}
	return nil
}

// execFallback tries to run a command by name when ResolveReal fails.
// On Windows, tries cmd /c; on Unix, tries the command directly.
func execFallback(name string, args []string) error {
	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		cmdArgs := append([]string{"/c", name}, args...)
		c = exec.Command("cmd", cmdArgs...)
	} else {
		c = exec.Command(name, args...)
	}

	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	if err := c.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return err
	}
	return nil
}

func init() {
	rootCmd.AddCommand(shimCmd)
}
