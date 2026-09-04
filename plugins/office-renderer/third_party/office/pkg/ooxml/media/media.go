package media

import (
	"sort"
	"strings"

	"office/pkg/ooxml/opc"
)

// Part is a binary payload stored in an OOXML package.
type Part struct {
	Name        string
	ContentType string
	Data        []byte
}

// Registry indexes media parts by normalized part name.
type Registry struct {
	parts map[string]Part
}

// FromPackage discovers media parts from a loaded OPC package.
func FromPackage(pkg *opc.Package) *Registry {
	registry := &Registry{parts: make(map[string]Part)}
	if pkg == nil {
		return registry
	}

	for _, part := range pkg.Parts() {
		if !isMediaPart(part.ContentType, part.Name) {
			continue
		}
		registry.parts[part.Name] = Part{
			Name:        part.Name,
			ContentType: part.ContentType,
			Data:        part.Data,
		}
	}

	return registry
}

// Part returns a media part by normalized name.
func (r *Registry) Part(name string) (Part, bool) {
	if r == nil {
		return Part{}, false
	}
	part, ok := r.parts[normalizePartName(name)]
	return part, ok
}

// Parts returns all discovered media parts sorted by name.
func (r *Registry) Parts() []Part {
	if r == nil {
		return nil
	}

	names := make([]string, 0, len(r.parts))
	for name := range r.parts {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]Part, 0, len(names))
	for _, name := range names {
		out = append(out, r.parts[name])
	}
	return out
}

func isMediaPart(contentType, name string) bool {
	if strings.HasPrefix(contentType, "image/") || strings.HasPrefix(contentType, "audio/") || strings.HasPrefix(contentType, "video/") {
		return true
	}

	name = normalizePartName(name)
	return strings.Contains(name, "/media/")
}

func normalizePartName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = strings.TrimPrefix(name, "./")
	name = strings.TrimPrefix(name, "/")
	name = strings.TrimSpace(name)
	return strings.ToLower(name)
}
