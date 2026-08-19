// Package plugin defines the extension points and registry for the file manager.
//
// Plugins are host-driven: the file manager defines narrow interfaces, and
// plugins register implementations at startup. There is no dynamic code loading
// (DLLs, WASM, etc.) in the initial design — plugins are either compiled into
// the binary or loaded from a trusted plugin directory.
//
// Lifecycle
//
//  1. Backend starts.
//  2. Plugin descriptors are discovered (built-in or from plugin directory).
//  3. Plugin.Init(config) is called for each plugin.
//  4. Plugin registers contributions via the Registry.
//  5. Host uses registered contributions during normal operation.
//  6. On shutdown, Plugin.Shutdown() is called.
package plugin

import (
	"context"
	"fmt"
)

// Info describes a plugin for discovery and diagnostics.
type Info struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description,omitempty"`
	Author      string            `json:"author,omitempty"`
	Schemes     []string          `json:"schemes,omitempty"`
	Extensions  []string          `json:"extensions,omitempty"`
	Commands    []CommandInfo     `json:"commands,omitempty"`
	Config      map[string]string `json:"config,omitempty"`
}

// CommandInfo describes a command exposed by a plugin.
type CommandInfo struct {
	ID        string   `json:"id"`
	Label     string   `json:"label"`
	Icon      string   `json:"icon,omitempty"`
	Hotkey    string   `json:"hotkey,omitempty"`
	AppliesTo []string `json:"appliesTo,omitempty"` // file types / extensions
}

// Plugin is the top-level interface every plugin must implement.
type Plugin interface {
	Info() Info
	Init(config map[string]any) error
	Shutdown() error
}

// Registry is the central place where plugins register their contributions.
// It is safe for concurrent use.
type Registry struct {
	plugins  map[string]Plugin
	vfs      map[string]VFSPlugin
	hooks    []OperationHook
	previews []PreviewPlugin
	commands map[string]CommandPlugin
}

// NewRegistry creates an empty plugin registry.
func NewRegistry() *Registry {
	return &Registry{
		plugins:  make(map[string]Plugin),
		vfs:      make(map[string]VFSPlugin),
		commands: make(map[string]CommandPlugin),
	}
}

// Register adds a plugin and all of its contributed interfaces to the registry.
func (r *Registry) Register(p Plugin) error {
	info := p.Info()
	if info.ID == "" {
		return fmt.Errorf("plugin ID is required")
	}
	if _, exists := r.plugins[info.ID]; exists {
		return fmt.Errorf("plugin %q already registered", info.ID)
	}
	r.plugins[info.ID] = p

	// Discover contributed interfaces.
	if vp, ok := p.(VFSPlugin); ok {
		r.vfs[info.ID] = vp
	}
	if h, ok := p.(OperationHook); ok {
		r.hooks = append(r.hooks, h)
	}
	if pp, ok := p.(PreviewPlugin); ok {
		r.previews = append(r.previews, pp)
	}
	if cp, ok := p.(CommandPlugin); ok {
		r.commands[info.ID] = cp
	}

	return nil
}

// List returns metadata for all registered plugins.
func (r *Registry) List() []Info {
	out := make([]Info, 0, len(r.plugins))
	for _, p := range r.plugins {
		out = append(out, p.Info())
	}
	return out
}

// Get returns the plugin with the given ID, or nil if not found.
func (r *Registry) Get(id string) Plugin {
	return r.plugins[id]
}

// Shutdown calls Shutdown on all registered plugins.
func (r *Registry) Shutdown() {
	for _, p := range r.plugins {
		_ = p.Shutdown()
	}
}

// VFSPlugins returns all registered VFS provider plugins.
func (r *Registry) VFSPlugins() map[string]VFSPlugin {
	// Return a shallow copy to prevent external mutation.
	out := make(map[string]VFSPlugin, len(r.vfs))
	for k, v := range r.vfs {
		out[k] = v
	}
	return out
}

// OperationHooks returns all registered operation hooks.
func (r *Registry) OperationHooks() []OperationHook {
	out := make([]OperationHook, len(r.hooks))
	copy(out, r.hooks)
	return out
}

// PreviewPlugins returns all registered preview plugins.
func (r *Registry) PreviewPlugins() []PreviewPlugin {
	out := make([]PreviewPlugin, len(r.previews))
	copy(out, r.previews)
	return out
}

// CommandPlugins returns all registered command plugins.
func (r *Registry) CommandPlugins() map[string]CommandPlugin {
	out := make(map[string]CommandPlugin, len(r.commands))
	for k, v := range r.commands {
		out[k] = v
	}
	return out
}

// InvokeCommand executes a command from a plugin.
func (r *Registry) InvokeCommand(ctx context.Context, pluginID, commandID string, paths []string) error {
	p, ok := r.commands[pluginID]
	if !ok {
		return fmt.Errorf("command plugin %q not found", pluginID)
	}
	if !p.IsApplicable(commandID, paths) {
		return fmt.Errorf("plugin command %q is not applicable to the current selection", commandID)
	}
	return p.Execute(ctx, commandID, paths)
}