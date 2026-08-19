package platform

import (
	"path/filepath"
	"strings"
)

func ToSlash(path string) string {
	return filepath.ToSlash(path)
}

func CleanPath(path string) string {
	return ToSlash(filepath.Clean(path))
}

func Normalize(path string) string {
	return strings.Trim(strings.ReplaceAll(path, "\\", "/"), "/")
}

func JoinNormalized(base, child string) string {
	base = Normalize(base)
	child = Normalize(child)
	if base == "" {
		return child
	}
	return base + "/" + child
}

func Join(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	return ToSlash(filepath.Join(parts...))
}

func ParentPath(path string) string {
	parent := filepath.Dir(path)
	if parent == path {
		return ToSlash(path)
	}
	return ToSlash(parent)
}

func SplitPath(path string) (dir, base, name, ext string) {
	dir, base = filepath.Split(path)
	ext = filepath.Ext(base)
	name = strings.TrimSuffix(base, ext)
	return CleanPath(dir), base, name, ext
}

func Breadcrumbs(path string) []map[string]string {
	path = filepath.Clean(path)

	type segment struct {
		label string
		path  string
	}
	segments := make([]segment, 0)
	current := path
	for {
		parent := filepath.Dir(current)
		isRoot := parent == current

		label := filepath.Base(current)
		if isRoot {
			label = current
		}

		segments = append(segments, segment{label: label, path: current})
		if isRoot {
			break
		}
		current = parent
	}

	for left, right := 0, len(segments)-1; left < right; left, right = left+1, right-1 {
		segments[left], segments[right] = segments[right], segments[left]
	}

	crumbs := make([]map[string]string, len(segments))
	for index, seg := range segments {
		crumbs[index] = map[string]string{
			"label": seg.label,
			"path":  ToSlash(seg.path),
		}
	}
	return crumbs
}