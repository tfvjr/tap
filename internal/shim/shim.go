package shim

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// shimmedCommands is the curated list of commands that get shim wrappers.
var shimmedCommands = []string{
	// JS/TS
	"node", "npm", "npx", "yarn", "pnpm", "bun", "deno",
	// Python
	"python", "python3", "uv", "uvicorn", "gunicorn", "flask",
	// Go
	"go",
	// Rust
	"cargo",
	// Ruby
	"ruby", "bundle", "rails",
	// Java
	"java", "gradle", "mvn",
	// PHP
	"php", "composer",
	// .NET
	"dotnet",
	// Elixir
	"mix", "elixir",
	// Other
	"hugo", "docker-compose",
}

const markerStart = "# tap-shims-start"
const markerEnd = "# tap-shims-end"

// ShimsDir returns the path to the shims directory inside dataDir.
func ShimsDir(dataDir string) string {
	return filepath.Join(dataDir, "shims")
}

// EnsureShims creates shim scripts and modifies the shell config if the
// shims directory does not already exist. It is idempotent — if the shims
// directory exists, it returns immediately.
func EnsureShims(dataDir string) (firstRun bool, err error) {
	dir := ShimsDir(dataDir)

	// Already set up — nothing to do.
	if _, err := os.Stat(dir); err == nil {
		return false, nil
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return false, fmt.Errorf("create shims dir: %w", err)
	}

	// Write shim scripts.
	for _, name := range shimmedCommands {
		if err := writeShim(dir, name); err != nil {
			return false, fmt.Errorf("write shim %s: %w", name, err)
		}
	}

	// Modify shell config to prepend shims dir to PATH.
	if err := addShimsToShellConfig(dir); err != nil {
		// Non-fatal — shims exist, user can add PATH manually.
		fmt.Fprintf(os.Stderr, "tap: could not update shell config: %v\n", err)
		fmt.Fprintf(os.Stderr, "tap: add %s to the front of your PATH manually\n", dir)
	}

	return true, nil
}

// writeShim creates a platform-specific shim script for the given command.
func writeShim(shimsDir, name string) error {
	if runtime.GOOS == "windows" {
		return writeShimWindows(shimsDir, name)
	}
	return writeShimUnix(shimsDir, name)
}

// writeShimUnix writes a shell script shim (no extension, executable).
func writeShimUnix(shimsDir, name string) error {
	content := fmt.Sprintf("#!/bin/sh\nexec tap _shim %s \"$@\"\n", name)
	path := filepath.Join(shimsDir, name)
	return os.WriteFile(path, []byte(content), 0755)
}

// writeShimWindows writes a .cmd batch file shim.
func writeShimWindows(shimsDir, name string) error {
	content := fmt.Sprintf("@echo off\r\ntap _shim %s %%*\r\n", name)
	path := filepath.Join(shimsDir, name+".cmd")
	return os.WriteFile(path, []byte(content), 0644)
}

// addShimsToShellConfig detects the user's shell and appends the PATH
// modification wrapped in marker comments.
func addShimsToShellConfig(shimsDir string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	if runtime.GOOS == "windows" {
		return addShimsToShellConfigWindows(home, shimsDir)
	}
	return addShimsToShellConfigUnix(home, shimsDir)
}

// addShimsToShellConfigUnix handles bash, zsh, and fish.
func addShimsToShellConfigUnix(home, shimsDir string) error {
	shell := os.Getenv("SHELL")

	type shellConfig struct {
		path string
		line string
	}

	var configs []shellConfig

	switch {
	case strings.Contains(shell, "fish"):
		fishConfig := filepath.Join(home, ".config", "fish", "config.fish")
		configs = append(configs, shellConfig{
			path: fishConfig,
			line: fmt.Sprintf("fish_add_path -p %s", shimsDir),
		})
	case strings.Contains(shell, "zsh"):
		configs = append(configs, shellConfig{
			path: filepath.Join(home, ".zshrc"),
			line: fmt.Sprintf(`export PATH="$HOME/.tap/shims:$PATH"`),
		})
	default:
		// Default to bash. Also try to cover cases where both .bashrc and
		// .zshrc might exist.
		configs = append(configs, shellConfig{
			path: filepath.Join(home, ".bashrc"),
			line: fmt.Sprintf(`export PATH="$HOME/.tap/shims:$PATH"`),
		})
	}

	// Also add to .zshrc if it exists and we didn't already target it.
	if !strings.Contains(shell, "zsh") && !strings.Contains(shell, "fish") {
		zshrc := filepath.Join(home, ".zshrc")
		if _, err := os.Stat(zshrc); err == nil {
			configs = append(configs, shellConfig{
				path: zshrc,
				line: fmt.Sprintf(`export PATH="$HOME/.tap/shims:$PATH"`),
			})
		}
	}

	var lastErr error
	for _, cfg := range configs {
		if err := appendToShellConfig(cfg.path, cfg.line); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// addShimsToShellConfigWindows modifies the PowerShell profile.
func addShimsToShellConfigWindows(home, shimsDir string) error {
	// PowerShell profile paths.
	profiles := []string{
		filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1"),
		filepath.Join(home, "Documents", "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1"),
	}

	line := `$env:Path = "$HOME\.tap\shims;" + $env:Path`

	var lastErr error
	wrote := false
	for _, profile := range profiles {
		dir := filepath.Dir(profile)
		if _, err := os.Stat(dir); err == nil {
			if err := appendToShellConfig(profile, line); err != nil {
				lastErr = err
			} else {
				wrote = true
			}
		}
	}

	if !wrote && lastErr == nil {
		// Create the first profile path.
		dir := filepath.Dir(profiles[0])
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		return appendToShellConfig(profiles[0], line)
	}

	return lastErr
}

// appendToShellConfig appends a line wrapped in marker comments to a config
// file. It is idempotent — if the markers already exist, it does nothing.
func appendToShellConfig(path, line string) error {
	// Read existing content (file may not exist yet).
	existing, _ := os.ReadFile(path)
	content := string(existing)

	// Already present — idempotent.
	if strings.Contains(content, markerStart) {
		return nil
	}

	block := fmt.Sprintf("\n%s\n%s\n%s\n", markerStart, line, markerEnd)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(block)
	return err
}

// RemoveShims removes the shims directory and cleans up shell config files.
// Used by `tap teardown`.
func RemoveShims(dataDir string) error {
	dir := ShimsDir(dataDir)

	// Remove shims directory.
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("remove shims dir: %w", err)
	}

	// Clean shell config files.
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	configFiles := []string{
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".zshrc"),
		filepath.Join(home, ".config", "fish", "config.fish"),
		filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1"),
		filepath.Join(home, "Documents", "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1"),
	}

	for _, path := range configFiles {
		removeMarkerBlock(path)
	}

	return nil
}

// removeMarkerBlock removes the lines between (and including) the marker
// comments from a file.
func removeMarkerBlock(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	content := string(data)
	startIdx := strings.Index(content, markerStart)
	if startIdx == -1 {
		return
	}

	endIdx := strings.Index(content, markerEnd)
	if endIdx == -1 {
		return
	}
	endIdx += len(markerEnd)

	// Include trailing newline if present.
	if endIdx < len(content) && content[endIdx] == '\n' {
		endIdx++
	}

	// Include leading newline if present.
	if startIdx > 0 && content[startIdx-1] == '\n' {
		startIdx--
	}

	cleaned := content[:startIdx] + content[endIdx:]
	os.WriteFile(path, []byte(cleaned), 0644)
}
