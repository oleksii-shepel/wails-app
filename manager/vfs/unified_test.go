package vfs

import "testing"

func TestPathrefToVFSConvertsAliasSchemes(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "s3", path: "s3://minio-dev/backups/2026", want: "/remote/s3/minio-dev/backups/2026"},
		{name: "s3 uppercase scheme", path: "S3://minio-dev/backups", want: "/remote/s3/minio-dev/backups"},
		{name: "s3 bare host", path: "s3://minio-dev", want: "/remote/s3/minio-dev"},
		{name: "registry", path: "registry://HKEY_CURRENT_USER/Environment", want: "/registry/HKEY_CURRENT_USER/Environment"},
		{name: "registry uppercase scheme", path: "REGISTRY://HKLM/SOFTWARE", want: "/registry/HKLM/SOFTWARE"},
		{name: "onedrive", path: "onedrive://personal/docs", want: "/remote/onedrive/personal/docs"},
		{name: "gdrive", path: "gdrive://team/designs", want: "/remote/gdrive/team/designs"},
		{name: "dropbox", path: "dropbox://workspace/contracts", want: "/remote/dropbox/workspace/contracts"},
		{name: "onedrive uppercase scheme", path: "OneDrive://personal", want: "/remote/onedrive/personal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pathrefToVFS(tt.path); got != tt.want {
				t.Fatalf("pathrefToVFS(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestPathrefToVFSConvertsDriveScheme(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "drive root", path: "filesystem://c", want: "/c"},
		{name: "drive path", path: "filesystem://c/Users/oleks", want: "/c/Users/oleks"},
		{name: "drive path uppercase scheme", path: "FILESYSTEM://d/Data", want: "/d/Data"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pathrefToVFS(tt.path); got != tt.want {
				t.Fatalf("pathrefToVFS(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestPathrefToVFSKeepsPlainPaths(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "native drive path", path: "C:/Users", want: "C:/Users"},
		{name: "unix path", path: "/var/log", want: "/var/log"},
		{name: "http alias", path: "https://example.com/page", want: "https://example.com/page"},
		{name: "search alias", path: "search://?path=/src&pattern=main", want: "search://?path=/src&pattern=main"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pathrefToVFS(tt.path); got != tt.want {
				t.Fatalf("pathrefToVFS(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
