package archive

import (
	"fmt"
	"sync"
)

// Registry manages archive handlers
type Registry struct {
	mu       sync.RWMutex
	handlers map[string]Handler // by name
	byExt    map[string]Handler // by extension
	byMagic  []Handler          // for magic detection
}

func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[string]Handler),
		byExt:    make(map[string]Handler),
	}
}

func (r *Registry) Register(h Handler) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := h.Name()
	if _, exists := r.handlers[name]; exists {
		return fmt.Errorf("handler %q already registered", name)
	}

	r.handlers[name] = h

	// Register by extension
	for _, ext := range h.Extensions() {
		r.byExt[ext] = h
	}

	// Register for magic detection if supported
	if len(h.Magic()) > 0 {
		r.byMagic = append(r.byMagic, h)
	}

	return nil
}

func (r *Registry) GetByName(name string) (Handler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.handlers[name]
	return h, ok
}

func (r *Registry) GetByPath(path string) (Handler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Try extension matching (longest first)
	// ".tar.gz" should match before ".gz"
	for i := len(path); i > 0; i-- {
		if h, ok := r.byExt[path[:i]]; ok {
			return h, true
		}
	}
	return nil, false
}

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

func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.handlers))
	for name := range r.handlers {
		names = append(names, name)
	}
	return names
}

// Global registry instance
var DefaultRegistry = NewRegistry()

func Register(h Handler) error {
	return DefaultRegistry.Register(h)
}
