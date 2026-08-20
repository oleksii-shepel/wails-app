package app

import (
	internalelevation "wails-app/internal/elevation"
	pluginpackage "wails-app/internal/pluginpackage"
	internalworkspace "wails-app/internal/workspace"
)

func MaybeRelaunchElevatedForStartup() (bool, error) {
	if internalelevation.IsProcessElevated() {
		return false, nil
	}
	if !startupRequiresElevation() {
		return false, nil
	}
	if err := internalelevation.RelaunchCurrentProcessElevated(); err != nil {
		return false, err
	}
	return true, nil
}

func startupRequiresElevation() bool {
	return startupPluginsRequireElevation() || internalworkspace.StoredStateRequiresElevation(pluginPackageAppName)
}

func startupPluginsRequireElevation() bool {
	root, err := pluginpackage.DefaultDirectory(pluginPackageAppName)
	if err != nil {
		return false
	}
	packages, err := pluginpackage.Discover(root)
	if err != nil {
		return false
	}
	store, err := pluginPackageStateStore()
	if err != nil {
		return false
	}
	if err := store.Reconcile(packages); err != nil {
		return false
	}
	states, err := store.List()
	if err != nil {
		return false
	}
	return requiresStartupPluginHostPrivilege(packages, states)
}
