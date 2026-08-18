package app

import (
	"testing"
	contracts "wails-app/internal/contracts"
)

func TestRemoteProviderCapabilities(t *testing.T) {
	for _, p := range []contracts.RemoteProtocol{contracts.ProtocolSMB, contracts.ProtocolS3, contracts.ProtocolGDrive, contracts.ProtocolOneDrive} {
		provider, err := remoteProviderFor(p)
		if err != nil {
			t.Fatal(err)
		}
		c := provider.Descriptor().Capabilities
		if p == contracts.ProtocolSMB && (!c.Rename || !c.AtomicMove || c.Pagination || c.Trash) {
			t.Fatalf("bad smb: %+v", c)
		}
		if p == contracts.ProtocolS3 && (!c.Pagination || !c.RandomAccess || !c.RangeRead || c.Rename) {
			t.Fatalf("bad s3: %+v", c)
		}
		if (p == contracts.ProtocolGDrive || p == contracts.ProtocolOneDrive) && (!c.Pagination || c.RandomAccess || c.RangeRead || c.Rename) {
			t.Fatalf("bad cloud: %+v", c)
		}
	}
}
