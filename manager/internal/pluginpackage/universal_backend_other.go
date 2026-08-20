//go:build !windows

package pluginpackage

import "fmt"

func startBackendForPrivilege(entry, privilege string) (*BackendProcess, error) {
	switch privilege {
	case "", "normal":
		return startInheritedBackend(entry)
	case "elevated":
		return nil, fmt.Errorf("elevated plugin backends are not implemented on this platform")
	default:
		return nil, fmt.Errorf("unsupported backend privilege %q", privilege)
	}
}
