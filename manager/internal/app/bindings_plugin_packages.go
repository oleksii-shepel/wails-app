package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	internalelevation "wails-app/internal/elevation"

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

// DiscoverPluginPackages scans immediate child directories and .zip archives.
// ZIP discovery is metadata-only; archives are not extracted until policy allows installation.
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

func (a *App) GetPluginPackageStates() ([]pluginpackage.PackageState, error) {
	store, err := pluginPackageStateStore()
	if err != nil {
		return nil, err
	}
	return store.List()
}

func (a *App) GetPluginPackageState(pluginID string) (pluginpackage.PackageState, error) {
	store, err := pluginPackageStateStore()
	if err != nil {
		return pluginpackage.PackageState{}, err
	}
	return store.Get(pluginID)
}

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

func (a *App) QuarantinePluginPackage(pluginID, reason string) (pluginpackage.PackageState, error) {
	store, err := pluginPackageStateStore()
	if err != nil {
		return pluginpackage.PackageState{}, err
	}
	return store.Quarantine(pluginID, reason)
}

func (a *App) RecordPluginActivation(pluginID string, success bool, activationError string) (pluginpackage.PackageState, error) {
	store, err := pluginPackageStateStore()
	if err != nil {
		return pluginpackage.PackageState{}, err
	}
	return store.RecordActivation(pluginID, success, activationError)
}

// PrepareUniversalPluginPackage validates and extracts an archive into the per-user cache.
// Callers should only invoke this after the existing trust/permission policy admits the package.
func (a *App) PrepareUniversalPluginPackage(pluginID string) (pluginpackage.PreparedPackage, error) {
	pkg, err := a.findDiscoveredPluginPackage(pluginID)
	if err != nil {
		return pluginpackage.PreparedPackage{}, err
	}
	if pkg.Kind != "archive" {
		return pluginpackage.PreparedPackage{}, fmt.Errorf("plugin %s is not an archive package", pluginID)
	}
	return pluginpackage.PrepareUniversalArchive(pluginPackageAppName, pkg)
}

// ReadUniversalPluginFrontendEntry returns only the manifest-declared frontend entry.
func (a *App) ReadUniversalPluginFrontendEntry(pluginID string) (string, error) {
	pkg, err := a.PrepareUniversalPluginPackage(pluginID)
	if err != nil {
		return "", err
	}
	if pkg.Manifest.Frontend == nil {
		return "", fmt.Errorf("plugin %s has no frontend entry", pluginID)
	}
	path := filepath.Join(pkg.CachePath, filepath.FromSlash(pkg.Manifest.Frontend.Entry))
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("frontend entry is not a regular file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// EnsureUniversalPluginHostPrivilege is called when the user explicitly enables
// a universal plugin. The enabled state is persisted before this call.
func (a *App) EnsureUniversalPluginHostPrivilege(pluginID string) (bool, error) {
	pkg, err := a.PrepareUniversalPluginPackage(pluginID)
	if err != nil {
		return false, err
	}
	if pkg.Manifest.Backend == nil || pkg.Manifest.Backend.Privilege != "elevated" {
		return false, nil
	}
	return a.relaunchElevatedAndQuit()
}

// EnsureStartupPluginHostPrivilege checks persisted plugin policy at startup
// and relaunches Manager elevated when an enabled universal plugin requires it.
func (a *App) EnsureStartupPluginHostPrivilege() (bool, error) {
	root, err := a.GetPluginDirectory()
	if err != nil {
		return false, err
	}
	packages, err := pluginpackage.Discover(root)
	if err != nil {
		return false, err
	}
	store, err := pluginPackageStateStore()
	if err != nil {
		return false, err
	}
	if err := store.Reconcile(packages); err != nil {
		return false, err
	}
	states, err := store.List()
	if err != nil {
		return false, err
	}
	if !requiresStartupPluginHostPrivilege(packages, states) {
		return false, nil
	}
	return a.relaunchElevatedAndQuit()
}

func (a *App) CallUniversalPluginBackend(pluginID, method, paramsJSON string) (interface{}, error) {
	pkg, err := a.PrepareUniversalPluginPackage(pluginID)
	if err != nil {
		return nil, err
	}

	raw := json.RawMessage(paramsJSON)
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	if !json.Valid(raw) {
		return nil, fmt.Errorf("backend params are not valid JSON")
	}

	result, err := pluginpackage.DefaultBackendManager.Call(pkg, method, raw)
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, nil
	}
	var value interface{}
	if err := json.Unmarshal(result, &value); err != nil {
		return nil, fmt.Errorf("backend returned invalid JSON result: %w", err)
	}
	return value, nil
}

func (a *App) StopUniversalPluginBackend(pluginID string) error {
	return pluginpackage.DefaultBackendManager.Stop(pluginID)
}

func (a *App) relaunchElevatedAndQuit() (bool, error) {
	if internalelevation.IsProcessElevated() {
		return false, nil
	}
	if err := internalelevation.RelaunchCurrentProcessElevated(); err != nil {
		return false, err
	}

	if a.State != nil && a.State.Ctx != nil {
		ctx := a.State.Ctx
		go func() {
			time.Sleep(250 * time.Millisecond)
			wailsruntime.Quit(ctx)
		}()
	}
	return true, nil
}

func requiresStartupPluginHostPrivilege(
	packages []pluginpackage.DiscoveredPackage,
	states []pluginpackage.PackageState,
) bool {
	enabled := make(map[string]bool, len(states))
	for _, state := range states {
		enabled[state.PluginID] = state.Enabled
	}
	for _, pkg := range packages {
		if !pkg.Valid || pkg.Kind != "archive" {
			continue
		}
		if pkg.Manifest.Backend == nil || pkg.Manifest.Backend.Privilege != "elevated" {
			continue
		}
		if enabled[pkg.Manifest.ID] {
			return true
		}
	}
	return false
}
