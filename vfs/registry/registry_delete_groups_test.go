//go:build windows

package registry

import (
	"context"
	"testing"

	"golang.org/x/sys/windows/registry"
)

func TestRemoveDeletesNestedRegistryKeyTree(t *testing.T) {
	const rootPath = `Software\\ReassemblyDeleteTest`

	root, _, err := registry.CreateKey(registry.CURRENT_USER, rootPath, registry.ALL_ACCESS)
	if err != nil {
		t.Fatal(err)
	}
	child, _, err := registry.CreateKey(root, `Child`, registry.ALL_ACCESS)
	if err != nil {
		root.Close()
		t.Fatal(err)
	}
	if err := child.SetStringValue("Value", "x"); err != nil {
		child.Close()
		root.Close()
		t.Fatal(err)
	}
	child.Close()
	root.Close()

	fs := &FS{}
	if err := fs.Remove(context.Background(), "HKEY_CURRENT_USER/Software/ReassemblyDeleteTest"); err != nil {
		t.Fatal(err)
	}

	if _, err := registry.OpenKey(registry.CURRENT_USER, rootPath, registry.READ); err == nil {
		t.Fatal("registry key tree still exists after Remove")
	}
}
