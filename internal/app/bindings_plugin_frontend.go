package app

import (
	"fmt"

	pluginpackage "wails-app/internal/pluginpackage"
)

// ReadPluginFrontendModule returns the constrained declarative frontend module
// for a discovered package. It never evaluates code and never accepts arbitrary paths.
func (a *App) ReadPluginFrontendModule(pluginID string) (pluginpackage.FrontendModule, error) {
	root, err := a.GetPluginDirectory()
	if err != nil {
		return pluginpackage.FrontendModule{}, err
	}
	packages, err := pluginpackage.Discover(root)
	if err != nil {
		return pluginpackage.FrontendModule{}, err
	}
	for _, pkg := range packages {
		if pkg.Manifest.ID == pluginID {
			return pluginpackage.ReadFrontendModule(root, pkg)
		}
	}
	return pluginpackage.FrontendModule{}, fmt.Errorf("plugin %s is not installed", pluginID)
}
