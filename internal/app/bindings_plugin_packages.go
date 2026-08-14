package app

import pluginpackage "wails-app/internal/pluginpackage"

const pluginPackageAppName = "wails-file-manager"

// GetPluginDirectory returns the trusted per-user directory scanned for plugin packages.
// Discovery is metadata-only; no code is executed from this directory.
func (a *App) GetPluginDirectory() (string, error) {
	return pluginpackage.DefaultDirectory(pluginPackageAppName)
}

// DiscoverPluginPackages scans immediate child directories containing plugin.json.
// It validates package metadata and returns diagnostics without loading executable code.
func (a *App) DiscoverPluginPackages() ([]pluginpackage.DiscoveredPackage, error) {
	root, err := a.GetPluginDirectory()
	if err != nil {
		return nil, err
	}
	return pluginpackage.Discover(root)
}
