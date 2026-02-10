package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/tfvjr/tap/internal/discovery"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new Tap project in the current directory",
	Long:  "Register the current directory as a Tap project by creating a .tap.toml configuration file.",
	RunE:  initRun,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func initRun(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	tapFile := filepath.Join(cwd, ".tap.toml")

	// If .tap.toml already exists, inform the user and exit.
	if _, err := os.Stat(tapFile); err == nil {
		name := discovery.InferProjectName(cwd)
		fmt.Fprintf(cmd.OutOrStdout(), "Project already initialized: %s\n", name)
		return nil
	}

	// Infer project name from the directory name.
	name := filepath.Base(cwd)

	content := fmt.Sprintf("[project]\nname = %q\n", name)

	if err := os.WriteFile(tapFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing .tap.toml: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Initialized project '%s' in %s\n", name, cwd)
	fmt.Fprintln(cmd.OutOrStdout(), "Created .tap.toml")
	return nil
}

