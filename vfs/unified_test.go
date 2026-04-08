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
