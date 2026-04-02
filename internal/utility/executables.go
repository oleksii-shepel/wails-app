package utility

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var (
	bundledToolsDirsOnce sync.Once
	bundledToolsDirs     []string
)

// ResolveExecutable returns an executable from the bundled tools directory if
// present, then falls back to PATH or absolute candidates.
func ResolveExecutable(candidates ...string) (string, bool) {
	for _, candidate := range candidates {
		if resolved, ok := resolveBundledCandidate(candidate); ok {
			return resolved, true
		}
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if resolved, err := exec.LookPath(candidate); err == nil {
			return resolved, true
		}
		if filepath.IsAbs(candidate) {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate, true
			}
		}
	}

	return "", false
}

func resolveBundledCandidate(candidate string) (string, bool) {
	trimmed := strings.TrimSpace(candidate)
	if trimmed == "" {
		return "", false
	}

	base := filepath.Base(trimmed)
	names := uniqueExecutableNames(base)
	for _, root := range bundledToolsRoots() {
		for _, name := range names {
			if resolved, ok := findExecutableUnderRoot(root, name); ok {
				return resolved, true
			}
		}
	}

	return "", false
}

func uniqueExecutableNames(name string) []string {
	if name == "" {
		return nil
	}

	seen := map[string]struct{}{}
	result := []string{}
	appendName := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}

	appendName(name)
	if runtime.GOOS == "windows" && filepath.Ext(name) == "" {
		appendName(name + ".exe")
	}
	return result
}

func bundledToolsRoots() []string {
	bundledToolsDirsOnce.Do(func() {
		seen := map[string]struct{}{}
		appendRoot := func(root string) {
			root = strings.TrimSpace(root)
			if root == "" {
				return
			}
			abs, err := filepath.Abs(root)
			if err != nil {
				return
			}
			info, err := os.Stat(abs)
			if err != nil || !info.IsDir() {
				return
			}
			key := strings.ToLower(abs)
			if _, ok := seen[key]; ok {
				return
			}
			seen[key] = struct{}{}
			bundledToolsDirs = append(bundledToolsDirs, abs)
		}

		for _, envName := range []string{"WAILS_TOOLS_DIR", "WAILS_EXTERNAL_TOOLS_DIR"} {
			appendRoot(os.Getenv(envName))
		}

		if exePath, err := os.Executable(); err == nil {
			exeDir := filepath.Dir(exePath)
			appendRoot(filepath.Join(exeDir, "tools"))
			appendRoot(filepath.Join(exeDir, "portable-tools"))
			appendRoot(filepath.Join(exeDir, "..", "tools"))
		}

		if cwd, err := os.Getwd(); err == nil {
			appendRoot(filepath.Join(cwd, "tools"))
			appendRoot(filepath.Join(cwd, "portable-tools"))
			appendRoot(filepath.Join(cwd, "..", "tools"))
			appendRoot(filepath.Join(cwd, "..", "portable-tools"))
		}
	})

	return bundledToolsDirs
}

func findExecutableUnderRoot(root string, fileName string) (string, bool) {
	direct := filepath.Join(root, fileName)
	if info, err := os.Stat(direct); err == nil && !info.IsDir() {
		return direct, true
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return "", false
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		candidate := filepath.Join(root, entry.Name(), fileName)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, true
		}
	}

	return "", false
}
