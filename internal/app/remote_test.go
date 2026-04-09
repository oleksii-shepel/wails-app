package app

import (
	"testing"

	contracts "wails-app/internal/contracts"
)

func TestRemoteMountPathUsesCloudProviderRoots(t *testing.T) {
	tests := []struct {
		name string
		cfg  contracts.RemoteConfig
		want string
	}{
		{
			name: "gdrive",
			cfg: contracts.RemoteConfig{
				Protocol: contracts.ProtocolGDrive,
				Name:     "team-drive",
			},
			want: "/remote/gdrive/team-drive",
		},
		{
			name: "dropbox",
			cfg: contracts.RemoteConfig{
				Protocol: contracts.ProtocolDropbox,
				Name:     "shared-workspace",
			},
			want: "/remote/dropbox/shared-workspace",
		},
		{
			name: "onedrive",
			cfg: contracts.RemoteConfig{
				Protocol: contracts.ProtocolOneDrive,
				Name:     "personal",
			},
			want: "/remote/onedrive/personal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := remoteMountPath(tt.cfg)
			if got != tt.want {
				t.Fatalf("remoteMountPath() mismatch\nwant: %q\n got: %q", tt.want, got)
			}
		})
	}
}

func TestRemoteDisplayNameUsesProviderDefaults(t *testing.T) {
	tests := []struct {
		name string
		cfg  contracts.RemoteConfig
		want string
	}{
		{name: "gdrive", cfg: contracts.RemoteConfig{Protocol: contracts.ProtocolGDrive}, want: "Google Drive"},
		{name: "dropbox", cfg: contracts.RemoteConfig{Protocol: contracts.ProtocolDropbox}, want: "Dropbox"},
		{name: "onedrive", cfg: contracts.RemoteConfig{Protocol: contracts.ProtocolOneDrive}, want: "OneDrive"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := remoteDisplayName(tt.cfg)
			if got != tt.want {
				t.Fatalf("remoteDisplayName() mismatch\nwant: %q\n got: %q", tt.want, got)
			}
		})
	}
}
