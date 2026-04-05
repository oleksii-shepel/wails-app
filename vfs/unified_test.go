package vfs

import "testing"

func TestPublicAliasToVFSAddsDefaultFTPPort(t *testing.T) {
	got, ok := publicAliasToVFS("ftp://samples.ffmpeg.org/pub", ftpScheme, ftpVFSRoot)
	if !ok {
		t.Fatalf("publicAliasToVFS(ftp) did not match")
	}

	want := "/remote/ftp/samples.ffmpeg.org:21/pub"
	if got != want {
		t.Fatalf("publicAliasToVFS(ftp) mismatch\nwant: %q\n got: %q", want, got)
	}
}

func TestPublicAliasToVFSPreservesExplicitFTPPortAtRoot(t *testing.T) {
	got, ok := publicAliasToVFS("ftp://samples.ffmpeg.org:21", ftpScheme, ftpVFSRoot)
	if !ok {
		t.Fatalf("publicAliasToVFS(ftp explicit port) did not match")
	}

	want := "/remote/ftp/samples.ffmpeg.org:21"
	if got != want {
		t.Fatalf("publicAliasToVFS(ftp explicit port) mismatch\nwant: %q\n got: %q", want, got)
	}
}
