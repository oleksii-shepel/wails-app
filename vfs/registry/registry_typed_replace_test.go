//go:build windows

package registry

import (
	"context"
	"testing"

	"golang.org/x/sys/windows/registry"
)

func TestWriteRegistryValueReplacesType(t *testing.T) {
	const keyPath = `Software\ReassemblyTypedValueTest`
	const valueName = "Value"

	key, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		keyPath,
		registry.ALL_ACCESS,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		key.Close()
		_ = registry.DeleteKey(registry.CURRENT_USER, keyPath)
	}()

	if err := key.SetStringValue(valueName, "42"); err != nil {
		t.Fatal(err)
	}

	fs := &FS{}
	path := "HKEY_CURRENT_USER/Software/ReassemblyTypedValueTest/Value"
	if err := fs.WriteRegistryValue(
		context.Background(),
		path,
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
	const keyPath = `Software\ReassemblyTypedValueRollbackTest`
	const valueName = "Value"

	key, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		keyPath,
		registry.ALL_ACCESS,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		key.Close()
		_ = registry.DeleteKey(registry.CURRENT_USER, keyPath)
	}()

	if err := key.SetStringValue(valueName, "original"); err != nil {
		t.Fatal(err)
	}

	fs := &FS{}
	path := "HKEY_CURRENT_USER/Software/ReassemblyTypedValueRollbackTest/Value"
	err = fs.WriteRegistryValue(
		context.Background(),
		path,
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
