package app

import (
	"testing"

	pluginpackage "wails-app/internal/pluginpackage"
)

func TestRequiresStartupPluginHostPrivilege(t *testing.T) {
	t.Run("matches enabled elevated archive plugin", func(t *testing.T) {
		packages := []pluginpackage.DiscoveredPackage{
			{
				Kind:  "archive",
				Valid: true,
				Manifest: pluginpackage.Manifest{
					ID:      "elevated.plugin",
					Version: "1.0.0",
					Backend: &pluginpackage.BackendEntry{Privilege: "elevated"},
				},
			},
		}
		states := []pluginpackage.PackageState{
			{PluginID: "elevated.plugin", Enabled: true},
		}

		if !requiresStartupPluginHostPrivilege(packages, states) {
			t.Fatal("expected enabled elevated archive plugin to require startup elevation")
		}
	})

	t.Run("ignores disabled or non-elevated plugins", func(t *testing.T) {
		packages := []pluginpackage.DiscoveredPackage{
			{
				Kind:  "archive",
				Valid: true,
				Manifest: pluginpackage.Manifest{
					ID:      "normal.plugin",
					Version: "1.0.0",
					Backend: &pluginpackage.BackendEntry{Privilege: "normal"},
				},
			},
			{
				Kind:  "directory",
				Valid: true,
				Manifest: pluginpackage.Manifest{
					ID:      "elevated.directory",
					Version: "1.0.0",
					Backend: &pluginpackage.BackendEntry{Privilege: "elevated"},
				},
			},
			{
				Kind:  "archive",
				Valid: true,
				Manifest: pluginpackage.Manifest{
					ID:      "disabled.elevated",
					Version: "1.0.0",
					Backend: &pluginpackage.BackendEntry{Privilege: "elevated"},
				},
			},
		}
		states := []pluginpackage.PackageState{
			{PluginID: "normal.plugin", Enabled: true},
			{PluginID: "elevated.directory", Enabled: true},
			{PluginID: "disabled.elevated", Enabled: false},
		}

		if requiresStartupPluginHostPrivilege(packages, states) {
			t.Fatal("did not expect startup elevation without an enabled elevated archive plugin")
		}
	})
}
