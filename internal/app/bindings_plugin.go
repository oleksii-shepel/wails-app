package app

import (
	"context"

	"wails-app/internal/plugin"
)

// ListPlugins returns metadata for all registered plugins.
func (a *App) ListPlugins() []plugin.Info {
	if a.State == nil || a.State.PluginRegistry == nil {
		return nil
	}
	return a.State.PluginRegistry.List()
}

// GetPlugin returns metadata for a single plugin by ID.
func (a *App) GetPlugin(id string) *plugin.Info {
	if a.State == nil || a.State.PluginRegistry == nil {
		return nil
	}
	p := a.State.PluginRegistry.Get(id)
	if p == nil {
		return nil
	}
	info := p.Info()
	return &info
}

// InvokePluginCommand executes a command exposed by a plugin.
func (a *App) InvokePluginCommand(pluginID, commandID string, paths []string) error {
	if a.State == nil || a.State.PluginRegistry == nil {
		return nil
	}
	ctx := context.Background()
	return a.State.PluginRegistry.InvokeCommand(ctx, pluginID, commandID, paths)
}
