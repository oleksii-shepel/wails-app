package vfs

import "testing"

func TestPublicAliasToVFSPreservesS3AliasPath(t *testing.T) {
	got, ok := publicAliasToVFS("s3://minio-dev/backups/2026", s3Scheme, s3VFSRoot)
	if !ok {
		t.Fatalf("publicAliasToVFS(s3) did not match")
	}

	want := "/remote/s3/minio-dev/backups/2026"
	if got != want {
		t.Fatalf("publicAliasToVFS(s3) mismatch\nwant: %q\n got: %q", want, got)
	}
}

func TestPublicAliasToVFSPreservesCloudProviderAliasPath(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		scheme string
		root   string
		want   string
	}{
		{name: "onedrive", path: "onedrive://personal/docs", scheme: oneDriveScheme, root: oneDriveVFSRoot, want: "/remote/onedrive/personal/docs"},
		{name: "gdrive", path: "gdrive://team/designs", scheme: gDriveScheme, root: gDriveVFSRoot, want: "/remote/gdrive/team/designs"},
		{name: "dropbox", path: "dropbox://workspace/contracts", scheme: dropboxScheme, root: dropboxVFSRoot, want: "/remote/dropbox/workspace/contracts"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := publicAliasToVFS(tt.path, tt.scheme, tt.root)
			if !ok {
				t.Fatalf("publicAliasToVFS(%s) did not match", tt.name)
			}
			if got != tt.want {
				t.Fatalf("publicAliasToVFS(%s) mismatch\nwant: %q\n got: %q", tt.name, tt.want, got)
			}
		})
	}
}