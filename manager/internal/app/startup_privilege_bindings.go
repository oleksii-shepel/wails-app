//go:build bindings

package app

// MaybeRelaunchElevatedForStartup is disabled when Wails executes its
// temporary wailsbindings.exe. Binding generation is a build-time operation,
// not an app launch, and must never request UAC.
func MaybeRelaunchElevatedForStartup() (bool, error) {
	return false, nil
}
