//go:build windows

package registry

import (
	"context"
	"crypto/rand"
	"testing"

	"golang.org/x/sys/windows/registry"
)

func createTypedValueTestKey(t *testing.T) (registry.Key, string) {
	t.Helper()
	name := "ReassemblyTypedValueTest-" + rand.Text()
	keyPath := `Software\` + name
	key, _, err := registry.CreateKey(registry.CURRENT_USER, keyPath, registry.ALL_ACCESS)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		key.Close()
		if err := registry.DeleteKey(registry.CURRENT_USER, keyPath); err != nil {
			t.Errorf("delete test key: %v", err)
		}
	})
	return key, "HKEY_CURRENT_USER/Software/" + name
}

func TestWriteRegistryValueReplacesType(t *testing.T) {
	const valueName = "Value"
	key, path := createTypedValueTestKey(t)

	if err := key.SetStringValue(valueName, "42"); err != nil {
		t.Fatal(err)
	}

	fs := &FS{}
	if err := fs.WriteRegistryValue(
		context.Background(),
		path+"/"+valueName,
		"REG_DWORD",
		"42",
	); err != nil {
		t.Fatal(err)
	}

	got, valueType, err := key.GetIntegerValue(valueName)
	if err != nil {
		t.Fatal(err)
	}
	if valueType != registry.DWORD {
		t.Fatalf("type = %d, want REG_DWORD", valueType)
	}
	if got != 42 {
		t.Fatalf("value = %d, want 42", got)
	}
}

func TestWriteRegistryValueRollsBackOnFailedTypeReplacement(t *testing.T) {
	const valueName = "Value"
	key, path := createTypedValueTestKey(t)

	if err := key.SetStringValue(valueName, "original"); err != nil {
		t.Fatal(err)
	}

	fs := &FS{}
	err := fs.WriteRegistryValue(
		context.Background(),
		path+"/"+valueName,
		"REG_DWORD",
		"not-a-number",
	)
	if err == nil {
		t.Fatal("expected type replacement to fail")
	}

	got, valueType, readErr := key.GetStringValue(valueName)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if valueType != registry.SZ {
		t.Fatalf("type = %d, want REG_SZ after rollback", valueType)
	}
	if got != "original" {
		t.Fatalf("value = %q, want original after rollback", got)
	}
}
