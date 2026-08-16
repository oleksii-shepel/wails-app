package app

import (
	"testing"

	contracts "wails-app/internal/contracts"
)

func TestRemoteProviderDescriptorsExposeConnectionSemantics(t *testing.T) {
	providers := remoteProviderDescriptors()
	if len(providers) < 5 {
		t.Fatalf("expected built-in providers, got %d", len(providers))
	}

	byProtocol := make(map[contracts.RemoteProtocol]contracts.RemoteProviderDescriptor, len(providers))
	for _, provider := range providers {
		byProtocol[provider.Protocol] = provider
	}

	for _, protocol := range []contracts.RemoteProtocol{
		contracts.ProtocolGDrive,
		contracts.ProtocolOneDrive,
		contracts.ProtocolS3,
	} {
		if _, ok := byProtocol[protocol]; !ok {
			t.Fatalf("missing provider descriptor for %s", protocol)
		}
	}

	if got := byProtocol[contracts.ProtocolGDrive]; got.Auth != "oauth-or-token" || !got.ReadOnly || got.Hierarchy != "id-backed" {
		t.Fatalf("unexpected Google Drive descriptor: %#v", got)
	}
	if got := byProtocol[contracts.ProtocolOneDrive]; got.Auth != "oauth-or-token" || !got.ReadOnly || got.Hierarchy != "id-backed" {
		t.Fatalf("unexpected OneDrive descriptor: %#v", got)
	}
	if got := byProtocol[contracts.ProtocolS3]; got.Auth != "access-key" || !got.ReadOnly || got.Hierarchy != "object-prefix" {
		t.Fatalf("unexpected S3 descriptor: %#v", got)
	}
}

func TestRemoteProviderUsesNonPOSIXS3MountIdentity(t *testing.T) {
	provider, err := remoteProviderFor(contracts.ProtocolS3)
	if err != nil {
		t.Fatal(err)
	}
	cfg := contracts.RemoteConfig{
		Protocol: contracts.ProtocolS3,
		Name:     "archive-store",
		Bucket:   "company-archive",
		Prefix:   "2026/reports",
	}
	if got, want := provider.MountPath(cfg), "/remote/s3/archive-store"; got != want {
		t.Fatalf("mount path mismatch: got %q want %q", got, want)
	}
	if got, want := provider.DisplayName(cfg), "archive-store (company-archive)"; got != want {
		t.Fatalf("display name mismatch: got %q want %q", got, want)
	}
	if provider.Descriptor().Hierarchy != "object-prefix" {
		t.Fatalf("S3 must advertise object-prefix hierarchy, got %q", provider.Descriptor().Hierarchy)
	}
}

func TestUnsupportedRemoteProviderFailsBeforeConnection(t *testing.T) {
	if _, err := remoteProviderFor(contracts.RemoteProtocol("unknown")); err == nil {
		t.Fatal("expected unsupported provider error")
	}
}
