package memory

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"wails-app/internal/pathref"
	internalplatform "wails-app/internal/platform"
)

type PathResolver struct {
	store          *Store
	configuredRoot func() string
}

func NewPathResolver(appName string, configuredRoot func() string) (*PathResolver, error) {
	store, err := NewStore(appName)
	if err != nil {
		return nil, err
	}
	if err := internalplatform.EnsureHiddenPath(store.RootPath()); err != nil {
		return nil, err
	}
	return &PathResolver{store: store, configuredRoot: configuredRoot}, nil
}

func normalizeRoot(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	return filepath.Clean(filepath.FromSlash(trimmed))
}

func pathKey(path string) string {
	cleaned := filepath.Clean(filepath.FromSlash(strings.TrimSpace(path)))
	if runtime.GOOS == "windows" {
		cleaned = strings.ToLower(cleaned)
	}
	return strings.TrimRight(cleaned, `\\/`)
}

func (r *PathResolver) currentRoot() string {
	if r.configuredRoot != nil {
		if root := normalizeRoot(r.configuredRoot()); root != "" {
			return root
		}
	}
	return filepath.Clean(r.store.DataPath())
}

func (r *PathResolver) joinUnderRoot(root string, child string) string {
	if child == "" {
		return filepath.Clean(root)
	}
	return filepath.Clean(filepath.Join(root, filepath.FromSlash(child)))
}

func (r *PathResolver) containsPath(root string, candidate string) bool {
	rootKey := pathKey(root)
	candidateKey := pathKey(candidate)
	if candidateKey == rootKey {
		return true
	}
	sep := string(filepath.Separator)
	return strings.HasPrefix(candidateKey, rootKey+sep)
}

func (r *PathResolver) relativeToRoot(root string, target string) (string, bool) {
	rootKey := pathKey(root)
	targetKey := pathKey(target)
	if targetKey == rootKey {
		return "", true
	}
	sep := string(filepath.Separator)
	if !strings.HasPrefix(targetKey, rootKey+sep) {
		return "", false
	}
	rel := strings.TrimPrefix(filepath.Clean(filepath.FromSlash(target)), filepath.Clean(filepath.FromSlash(root)))
	rel = strings.TrimLeft(rel, `\\/`)
	return filepath.ToSlash(rel), true
}

func (r *PathResolver) ResolvePublicPath(path string) (string, bool, error) {
	normalized := pathref.Normalize(path)
	if !pathref.IsMemory(normalized) {
		return normalized, false, nil
	}

	root := r.currentRoot()
	rest := strings.TrimPrefix(normalized, pathref.MemoryScheme)
	if rest == "" {
		return filepath.ToSlash(root), true, nil
	}

	target := r.joinUnderRoot(root, strings.TrimPrefix(rest, "/"))
	if !r.containsPath(root, target) {
		return "", true, fmt.Errorf("memory path escapes configured root")
	}
	return filepath.ToSlash(target), true, nil
}

func (r *PathResolver) ToPublicPath(path string) string {
	root := r.currentRoot()
	target := filepath.Clean(filepath.FromSlash(path))
	rel, ok := r.relativeToRoot(root, target)
	if !ok {
		return filepath.ToSlash(path)
	}
	if rel == "" {
		return pathref.MemoryScheme
	}
	return pathref.MemoryScheme + filepath.ToSlash(rel)
}

func (r *PathResolver) BackingRoot() string {
	return filepath.ToSlash(r.currentRoot())
}