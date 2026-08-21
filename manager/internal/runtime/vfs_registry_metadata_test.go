package runtime

import (
	"io/fs"
	"testing"
	"time"

	vfslib "wails-app/vfs"
)

type registryMetadataStub struct{}

func (registryMetadataStub) Name() string       { return "ValueName" }
func (registryMetadataStub) Size() int64        { return 12 }
func (registryMetadataStub) Mode() fs.FileMode  { return 0 }
func (registryMetadataStub) ModTime() time.Time { return time.Unix(123, 0) }
func (registryMetadataStub) IsDir() bool        { return false }
func (registryMetadataStub) Sys() any           { return nil }
func (registryMetadataStub) Path() string       { return "HKEY_CURRENT_USER/Software/ValueName" }
func (registryMetadataStub) RealPath() string   { return "" }
func (registryMetadataStub) Volume() vfslib.Volume {
	return nil
}
func (registryMetadataStub) IsArchive() bool             { return false }
func (registryMetadataStub) IsHidden() bool              { return false }
func (registryMetadataStub) IsSymlink() bool             { return false }
func (registryMetadataStub) IsNested() bool              { return false }
func (registryMetadataStub) MountChain() []string        { return nil }
func (registryMetadataStub) RegistryKind() string        { return "Value" }
func (registryMetadataStub) RegistryValueType() string   { return "REG_SZ" }
func (registryMetadataStub) RegistryPreview() string     { return "hello world" }

func TestConvertVFSInfoIncludesRegistryMetadata(t *testing.T) {
	file := convertVFSInfo(registryMetadataStub{})

	if file.RegistryKind != "Value" {
		t.Fatalf("expected registry kind to be preserved, got %q", file.RegistryKind)
	}
	if file.RegistryValueType != "REG_SZ" {
		t.Fatalf("expected registry value type to be preserved, got %q", file.RegistryValueType)
	}
	if file.RegistryPreview != "hello world" {
		t.Fatalf("expected registry preview to be preserved, got %q", file.RegistryPreview)
	}
}
