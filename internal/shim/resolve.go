package shim

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ResolveReal finds the real binary for name by searching PATH with the
// shims directory removed. This prevents infinite recursion when a shim
// calls back into tap _shim.
func ResolveReal(name, shimsDir string) (string, error) {
	shimsDir = filepath.Clean(shimsDir)

	pathEnv := os.Getenv("PATH")
	dirs := strings.Split(pathEnv, string(os.PathListSeparator))

	var filtered []string
	for _, d := range dirs {
		if filepath.Clean(d) == shimsDir {
			continue
		}
		filtered = append(filtered, d)
	}

	if runtime.GOOS == "windows" {
		return resolveWindows(name, filtered)
	}
	return resolveUnix(name, filtered)
}

// resolveUnix searches for an executable file named name in the given dirs.
func resolveUnix(name string, dirs []string) (string, error) {
	for _, dir := range dirs {
		p := filepath.Join(dir, name)
		if isExecutable(p) {
			return p, nil
		}
	}
	return "", fmt.Errorf("shim: %q not found in PATH (excluding shims dir)", name)
}

// resolveWindows searches for name with common Windows extensions.
func resolveWindows(name string, dirs []string) (string, error) {
	// If name already has an extension, try it directly first.
	if ext := filepath.Ext(name); ext != "" {
		for _, dir := range dirs {
			p := filepath.Join(dir, name)
			if fileExists(p) {
				return p, nil
			}
		}
	}

	// Try PATHEXT extensions (and the bare name).
	exts := windowsExts()
	for _, dir := range dirs {
		// Try bare name first (handles names like "node.exe" passed directly).
		p := filepath.Join(dir, name)
		if fileExists(p) {
			return p, nil
		}
		for _, ext := range exts {
			p := filepath.Join(dir, name+ext)
			if fileExists(p) {
				return p, nil
			}
		}
	}
	return "", fmt.Errorf("shim: %q not found in PATH (excluding shims dir)", name)
}

// windowsExts returns the list of executable extensions from PATHEXT,
// falling back to a sensible default.
func windowsExts() []string {
	pathext := os.Getenv("PATHEXT")
	if pathext == "" {
		pathext = ".COM;.EXE;.BAT;.CMD"
	}
	return strings.Split(strings.ToLower(pathext), ";")
}

// isExecutable checks if a file exists and is executable (Unix).
func isExecutable(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !fi.IsDir() && fi.Mode()&0111 != 0
}

// fileExists checks if a file exists and is not a directory.
func fileExists(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !fi.IsDir()
}
