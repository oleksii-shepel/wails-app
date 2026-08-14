package app

import (
	"fmt"

	pluginpackage "wails-app/internal/pluginpackage"
)

const pluginPackageAppName = "wails-file-manager"

func pluginPackageStateStore() (*pluginpackage.StateStore, error) {
	return pluginpackage.NewStateStore(pluginPackageAppName)
}

// GetPluginDirectory returns the trusted per-user directory scanned for plugin packages.
// Discovery is metadata-only; no code is executed from this directory.
func (a *App) GetPluginDirectory() (string, error) {
	return pluginpackage.DefaultDirectory(pluginPackageAppName)
}

// DiscoverPluginPackages scans immediate child directories containing plugin.json.
// It validates package metadata and returns diagnostics without loading executable code.
// Valid packages are reconciled with the separate persisted state store so observational
// metadata such as lastSeenVersion stays current without changing user trust decisions.
func (a *App) DiscoverPluginPackages() ([]pluginpackage.DiscoveredPackage, error) {
	root, err := a.GetPluginDirectory()
	if err != nil {
		return nil, err
	}
	packages, err := pluginpackage.Discover(root)
	if err != nil {
		return nil, err
	}
	store, err := pluginPackageStateStore()
	if err != nil {
		return nil, err
	}
	if err := store.Reconcile(packages); err != nil {
		return nil, err
	}
	return packages, nil
}

// GetPluginPackageStates returns persisted user decisions and runtime health.
func (a *App) GetPluginPackageStates() ([]pluginpackage.PackageState, error) {
	store, err := pluginPackageStateStore()
	if err != nil {
		return nil, err
	}
	return store.List()
}

// GetPluginPackageState returns persisted state for one plugin. Unknown plugins
// receive conservative defaults without being persisted until changed/reconciled.
func (a *App) GetPluginPackageState(pluginID string) (pluginpackage.PackageState, error) {
	store, err := pluginPackageStateStore()
	if err != nil {
		return pluginpackage.PackageState{}, err
	}
	return store.Get(pluginID)
}

// UpdatePluginPackageState persists explicit user decisions: enablement, trust,
// granted permissions, and clearing an existing quarantine.
func (a *App) UpdatePluginPackageState(pluginID string, patch pluginpackage.PackageStatePatch) (pluginpackage.PackageState, error) {
	pkg, err := a.findDiscoveredPluginPackage(pluginID)
	if err != nil {
		return pluginpackage.PackageState{}, err
	}
	if patch.GrantedPermissions != nil {
		if err := pluginpackage.ValidateGrantedPermissions(pkg.Manifest, *patch.GrantedPermissions); err != nil {
			return pluginpackage.PackageState{}, err
		}
	}
	if patch.Trusted != nil && *patch.Trusted {
		version := pkg.Manifest.Version
		patch.TrustedVersion = &version
	}

	store, err := pluginPackageStateStore()
	if err != nil {
		return pluginpackage.PackageState{}, err
	}
	return store.Update(pluginID, patch)
}

func (a *App) findDiscoveredPluginPackage(pluginID string) (pluginpackage.DiscoveredPackage, error) {
	root, err := a.GetPluginDirectory()
	if err != nil {
		return pluginpackage.DiscoveredPackage{}, err
	}
	packages, err := pluginpackage.Discover(root)
	if err != nil {
		return pluginpackage.DiscoveredPackage{}, err
	}
	for _, pkg := range packages {
		if pkg.Manifest.ID != pluginID {
			continue
		}
		if !pkg.Valid {
			return pluginpackage.DiscoveredPackage{}, fmt.Errorf("plugin %s is not a valid discovered package", pluginID)
		}
		return pkg, nil
	}
	return pluginpackage.DiscoveredPackage{}, fmt.Errorf("plugin %s is not installed", pluginID)
}

// QuarantinePluginPackage prevents automatic activation until explicitly cleared.
func (a *App) QuarantinePluginPackage(pluginID, reason string) (pluginpackage.PackageState, error) {
	store, err := pluginPackageStateStore()
	if err != nil {
		return pluginpackage.PackageState{}, err
	}
	return store.Quarantine(pluginID, reason)
}

// RecordPluginActivation records activation health independently from package discovery.
func (a *App) RecordPluginActivation(pluginID string, success bool, activationError string) (pluginpackage.PackageState, error) {
	store, err := pluginPackageStateStore()
	if err != nil {
		return pluginpackage.PackageState{}, err
	}
	return store.RecordActivation(pluginID, success, activationError)
}
