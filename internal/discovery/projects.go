package discovery

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/tfvjr/tap/internal/model"
)

// normalizePath returns a canonical path suitable for use as a map key.
// On Windows, paths are lowercased to avoid case-sensitivity mismatches.
func normalizePath(p string) string {
	p = filepath.Clean(p)
	if runtime.GOOS == "windows" {
		p = strings.ToLower(p)
	}
	return p
}

// projectMarkers lists filenames and directory names whose presence indicates
// a project root directory. The order does not matter — the first directory
// (walking upward) that contains any of these wins.
var projectMarkers = []string{
	// Version control
	".git",

	// JavaScript / TypeScript
	"package.json",
	"package-lock.json",
	"yarn.lock",
	"pnpm-lock.yaml",
	"bun.lockb",

	// Rust
	"Cargo.toml",
	"Cargo.lock",

	// Go
	"go.mod",

	// Python
	"pyproject.toml",
	"Pipfile",
	"Pipfile.lock",
	"poetry.lock",
	"uv.lock",
	"setup.py",
	"requirements.txt",

	// Ruby
	"Gemfile",
	"Gemfile.lock",

	// Java / JVM
	"pom.xml",
	"build.gradle",
	"build.gradle.kts",

	// PHP
	"composer.json",
	"composer.lock",

	// Elixir
	"mix.exs",

	// C / C++
	"CMakeLists.txt",

	// .NET (Global.json is a fixed name; .csproj/.sln are handled by glob check)
	"global.json",

	// Docker / DevOps
	"docker-compose.yml",
	"docker-compose.yaml",
	"Dockerfile",
	"Procfile",

	// General
	"Makefile",
	".tap.toml",
}

// projectGlobMarkers are file patterns that indicate a project root but can't
// be checked with a fixed filename (e.g. .NET projects use *.csproj, *.sln).
var projectGlobMarkers = []string{
	"*.csproj",
	"*.sln",
	"*.fsproj",
}

// DetectProjectRoot walks up from dir looking for a directory that contains
// one of the known project marker files. It returns the project root path and
// true when found, or ("", false) when the filesystem root is reached without
// finding a marker.
func DetectProjectRoot(dir string) (string, bool) {
	dir = filepath.Clean(dir)

	for {
		for _, marker := range projectMarkers {
			path := filepath.Join(dir, marker)
			if _, err := os.Stat(path); err == nil {
				return dir, true
			}
		}

		for _, pattern := range projectGlobMarkers {
			matches, _ := filepath.Glob(filepath.Join(dir, pattern))
			if len(matches) > 0 {
				return dir, true
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// packageJSON is a minimal struct used to extract the "name" field from a
// package.json file.
type packageJSON struct {
	Name string `json:"name"`
}

// InferProjectName derives a human-friendly project name from a project root
// directory. It first attempts to read the "name" field from package.json.
// If that fails for any reason it falls back to the directory basename.
func InferProjectName(projectRoot string) string {
	pkgPath := filepath.Join(projectRoot, "package.json")
	data, err := os.ReadFile(pkgPath)
	if err == nil {
		var pkg packageJSON
		if err := json.Unmarshal(data, &pkg); err == nil && pkg.Name != "" {
			return pkg.Name
		}
	}

	return filepath.Base(projectRoot)
}

// AttributeProcesses groups a slice of DevProcesses by project. For every
// process whose ProjectPath (working directory) is non-empty, it calls
// DetectProjectRoot to find the owning project. If a process's own CWD
// doesn't match a project, it walks the parent process chain (via PPID)
// and tries each parent's CWD. Processes that cannot be attributed after
// exhausting the chain are returned in the unattributed slice.
//
// The returned projects map is keyed by the normalized project root path.
func AttributeProcesses(processes []model.DevProcess) (projects map[string]*model.Project, unattributed []model.DevProcess) {
	projects = make(map[string]*model.Project)

	// Build a PID -> CWD lookup from all processes for parent chain walking.
	cwdByPID := make(map[int32]string)
	ppidByPID := make(map[int32]int32)
	for i := range processes {
		if processes[i].PID != nil {
			pid := *processes[i].PID
			if processes[i].ProjectPath != "" {
				cwdByPID[pid] = processes[i].ProjectPath
			}
			ppidByPID[pid] = processes[i].PPID
		}
	}

	for i := range processes {
		proc := &processes[i]

		root, found := findProjectRoot(proc, cwdByPID, ppidByPID)
		if !found {
			unattributed = append(unattributed, *proc)
			continue
		}

		proc.ProjectPath = root
		proc.Project = InferProjectName(root)

		key := normalizePath(root)
		proj, exists := projects[key]
		if !exists {
			proj = &model.Project{
				Name: proc.Project,
				Path: root,
			}
			projects[key] = proj
		}

		proj.Processes = append(proj.Processes, *proc)
	}

	for _, proj := range projects {
		proj.Recalculate()
	}

	return projects, unattributed
}

// findProjectRoot tries to find a project root for a process. It first checks
// the process's own CWD, then walks up the parent chain (up to 10 levels)
// trying each parent's CWD.
func findProjectRoot(proc *model.DevProcess, cwdByPID map[int32]string, ppidByPID map[int32]int32) (string, bool) {
	// Try the process's own CWD first.
	if proc.ProjectPath != "" {
		if root, ok := DetectProjectRoot(proc.ProjectPath); ok {
			return root, true
		}
	}

	// Walk the parent chain.
	if proc.PID == nil {
		return "", false
	}

	visited := make(map[int32]bool)
	pid := *proc.PID

	for depth := 0; depth < 10; depth++ {
		ppid, ok := ppidByPID[pid]
		if !ok || ppid == 0 || ppid == pid || visited[ppid] {
			break
		}
		visited[ppid] = true

		if cwd, ok := cwdByPID[ppid]; ok && cwd != "" {
			if root, ok := DetectProjectRoot(cwd); ok {
				return root, true
			}
		}

		pid = ppid
	}

	return "", false
}
