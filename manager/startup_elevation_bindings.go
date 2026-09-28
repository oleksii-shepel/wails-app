//go:build bindings

package main

// Wails executes a temporary wailsbindings.exe to generate frontend bindings.
// It is a build-time tool rather than an app launch, so skip the Manager's
// startup-elevation policy and never request UAC from that process.
func maybeRelaunchElevatedForStartup() (bool, error) {
	return false, nil
}
