//go:build darwin

package platform

// JoinMountNamespace is a no-op on Darwin because mount namespaces are Linux-specific.
func JoinMountNamespace(nsPath string) error {
	return nil
}
