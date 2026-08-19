//go:build windows

package registry

import (
	"fmt"
	"testing"

	"golang.org/x/sys/windows/registry"
)

func TestRegistryIntegerPreviewPreservesCapacity(t *testing.T) {
	tests := []struct {
		name      string
		value     uint64
		valueType uint32
		want      string
	}{
		{"dword", 1, registry.DWORD, "1 (0x00000001)"},
		{"qword", 1, registry.QWORD, "1 (0x0000000000000001)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			switch tt.valueType {
			case registry.DWORD, registry.DWORD_BIG_ENDIAN:
				got = fmt.Sprintf("%d (0x%08X)", uint32(tt.value))
			case registry.QWORD:
				got = fmt.Sprintf("%d (0x%016X)", tt.value)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
