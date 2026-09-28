//go:build !bindings

package main

import internalapp "wails-app/internal/app"

// maybeRelaunchElevatedForStartup runs only in an actual Manager process.
// Wails generates bindings by compiling and executing this package with the
// `bindings` tag, which must never cause a UAC prompt.
func maybeRelaunchElevatedForStartup() (bool, error) {
	return internalapp.MaybeRelaunchElevatedForStartup()
}
