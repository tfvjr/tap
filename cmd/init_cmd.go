package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
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
	if data, err := os.ReadFile(tapFile); err == nil {
		name := inferNameFromTOML(string(data))
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

// inferNameFromTOML does a quick extraction of the project name from .tap.toml content.
func inferNameFromTOML(data string) string {
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "name") {
			// Extract the quoted value: name = "value"
			if start := strings.IndexByte(line, '"'); start != -1 {
				if end := strings.IndexByte(line[start+1:], '"'); end != -1 {
					return line[start+1 : start+1+end]
				}
			}
		}
	}
	return "<unknown>"
}
