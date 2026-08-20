//go:build windows

package runtime

import "testing"

func TestInitVFSMountsRegistry(t *testing.T) {
	vfs, err := InitVFS()
	if err != nil {
		t.Fatalf("InitVFS() error = %v", err)
	}
	if !vfs.IsMounted(registryMountPoint) {
		t.Fatalf("expected %s to be mounted", registryMountPoint)
	}
}
