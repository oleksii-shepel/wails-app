package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	pluginpackage "wails-app/internal/pluginpackage"
)

func (a *App) findUniversalPluginPackage(pluginID string) (pluginpackage.UniversalPackage, error) {
	root, err := a.GetPluginDirectory()
	if err != nil {
		return pluginpackage.UniversalPackage{}, err
	}
	packages, err := pluginpackage.DiscoverUniversalArchives(root)
	if err != nil {
		return pluginpackage.UniversalPackage{}, err
	}
	for _, pkg := range packages {
		if pkg.Manifest.ID != pluginID {
			continue
		}
		if !pkg.Valid {
			return pluginpackage.UniversalPackage{}, fmt.Errorf(
				"plugin %s is not a valid universal package: %v",
				pluginID, pkg.Errors,
			)
		}
		return pkg, nil
	}
	return pluginpackage.UniversalPackage{}, fmt.Errorf("universal plugin %s is not installed", pluginID)
}

func (a *App) PrepareUniversalPluginPackage(pluginID string) (pluginpackage.UniversalPackage, error) {
	pkg, err := a.findUniversalPluginPackage(pluginID)
	if err != nil {
		return pluginpackage.UniversalPackage{}, err
	}
	return pluginpackage.PrepareUniversalArchive(pluginPackageAppName, pkg)
}

func (a *App) ReadUniversalPluginFrontendEntry(pluginID string) (string, error) {
	pkg, err := a.PrepareUniversalPluginPackage(pluginID)
	if err != nil {
		return "", err
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

func (a *App) CallUniversalPluginBackend(pluginID, method string, params interface{}) (interface{}, error) {
	pkg, err := a.PrepareUniversalPluginPackage(pluginID)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(params)
	if err != nil {
		return nil, err
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
