package cmd

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/tfvjr/tap/internal/capture"
	"github.com/tfvjr/tap/internal/discovery"
)

var runCmd = &cobra.Command{
	Use:   "run <command> [args...]",
	Short: "Run a command and capture its output",
	Long:  "Execute a command, tee its stdout/stderr to the terminal, and store every line in the tap database for later querying with `tap logs`.",
	Args:  cobra.MinimumNArgs(1),
	// Disable Cobra's flag parsing after "run" so flags like --port are
	// forwarded to the child process.
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Since DisableFlagParsing is true, we need to handle --help ourselves.
		for _, a := range args {
			if a == "--help" || a == "-h" {
				return cmd.Help()
			}
		}

		// Strip a leading "--" separator if present.
		if len(args) > 0 && args[0] == "--" {
			args = args[1:]
		}
		if len(args) == 0 {
			return fmt.Errorf("no command specified; usage: tap run <command> [args...]")
		}

		// Detect project from CWD.
		var project string
		cwd, _ := os.Getwd()
		if root, ok := discovery.DetectProjectRoot(cwd); ok {
			project = discovery.InferProjectName(root)
		}

		// Generate a unique tap_id for this run session.
		tapID := fmt.Sprintf("run_%d_%04x", time.Now().Unix(), rand.Intn(0xFFFF))

		exitCode, err := capture.RunAndCapture(context.Background(), args, tapID, project, appStore)
		if err != nil {
			return err
		}
		if exitCode != 0 {
			os.Exit(exitCode)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
