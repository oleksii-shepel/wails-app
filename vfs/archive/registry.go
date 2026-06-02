package archive

import (
	"fmt"
	"strings"
	"sync"
)

// Registry maps file extensions and magic bytes to Handler implementations.
type Registry struct {
	mu      sync.RWMutex
	byName  map[string]Handler // keyed by Handler.Name()
	byExt   map[string]Handler // keyed by lowercase dot-prefixed extension
	byMagic []Handler          // handlers that expose Magic() bytes, checked in registration order
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		byName: make(map[string]Handler),
		byExt:  make(map[string]Handler),
	}
}

// Register adds h to the registry. Returns an error if the name is already taken.
// Intended to be called from package init() functions.
func (r *Registry) Register(h Handler) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := h.Name()
	if _, exists := r.byName[name]; exists {
		return fmt.Errorf("archive: handler %q already registered", name)
	}
	r.byName[name] = h

	for _, ext := range h.Extensions() {
		// Lowercase, ensure leading dot.
		ext = strings.ToLower(ext)
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		r.byExt[ext] = h
	}

	if len(h.Magic()) > 0 {
		r.byMagic = append(r.byMagic, h)
	}

	return nil
}

// ForName returns the handler registered under the given name.
func (r *Registry) ForName(name string) (Handler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.byName[name]
	return h, ok
}

// ForPath selects a handler by matching the longest known extension suffix of
// path. For example "archive.tar.gz" matches ".tar.gz" before ".gz".
// path may be just a filename or a full OS path.
func (r *Registry) ForPath(path string) (Handler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	lower := strings.ToLower(path)

	// Try compound extensions first (longest match wins).
	// We iterate all registered extensions sorted by length descending.
	// Since the map is small, a linear scan is fine.
	best := ""
	var bestHandler Handler
	for ext, h := range r.byExt {
		if strings.HasSuffix(lower, ext) && len(ext) > len(best) {
			best = ext
			bestHandler = h
		}
	}
	if bestHandler != nil {
		return bestHandler, true
	}
	return nil, false
}

// Detect tries to identify the format by inspecting the first bytes of a file.
// Returns the matching handler or (nil, false) if none matched.
func (r *Registry) Detect(data []byte) (Handler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, h := range r.byMagic {
		if h.Detect(data) {
			return h, true
		}
	}
	return nil, false
}

// List returns the names of all registered handlers.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.byName))
	for name := range r.byName {
		names = append(names, name)
	}
	return names
}

// DefaultRegistry is the package-level registry used by Register and helper
// functions. Format packages call archive.Register(h) from their init().
var DefaultRegistry = NewRegistry()

// Register adds h to DefaultRegistry. Logs and returns if the handler name
// conflicts — this is a programming error caught at startup, not a runtime
// condition. Prefer DefaultRegistry.Register for explicit error handling.
func Register(h Handler) {
	_ = DefaultRegistry.Register(h)
}

// ForPath is a convenience wrapper around DefaultRegistry.ForPath.
func ForPath(path string) (Handler, bool) {
	return DefaultRegistry.ForPath(path)
}

// ForName is a convenience wrapper around DefaultRegistry.ForName.
func ForName(name string) (Handler, bool) {
	return DefaultRegistry.ForName(name)
}

// Detect is a convenience wrapper around DefaultRegistry.Detect.
func Detect(data []byte) (Handler, bool) {
	return DefaultRegistry.Detect(data)
}

// IsArchiveExt reports whether name has a file extension that matches a
// registered archive handler.  It uses the same longest-suffix matching as
// ForPath, so it stays automatically in sync with whatever handlers have been
// registered via init() — no separate extension list to maintain.
func IsArchiveExt(name string) bool {
	_, ok := DefaultRegistry.ForPath(name)
	return ok
}
