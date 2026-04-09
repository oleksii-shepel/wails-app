package pathref

import (
	"os"
	"path/filepath"
	"testing"
)

func TestToNativePathDriveRootUsesPlatformSeparator(t *testing.T) {
	got := ToNativePath("filesystem://c")
	want := filepath.FromSlash("C:/")
	if got != want {
		t.Fatalf("ToNativePath(drive root) mismatch\nwant: %q\n got: %q", want, got)
	}
}

func TestToNativePathDriveChildUsesPlatformSeparator(t *testing.T) {
	got := ToNativePath("filesystem://c/New Folder")
	want := filepath.FromSlash("C:/New Folder")
	if got != want {
		t.Fatalf("ToNativePath(drive child) mismatch\nwant: %q\n got: %q", want, got)
	}
}

func TestToVFSAddsDefaultSMBPort(t *testing.T) {
	got := ToVFS("smb://fileserver/share/docs")
	want := "/remote/smb/fileserver:445/share/docs"
	if got != want {
		t.Fatalf("ToVFS(smb) mismatch\nwant: %q\n got: %q", want, got)
	}
}

func TestToVFSKeepsS3AliasSegments(t *testing.T) {
	got := ToVFS("s3://minio-dev/photos/2026")
	want := "/remote/s3/minio-dev/photos/2026"
	if got != want {
		t.Fatalf("ToVFS(s3) mismatch\nwant: %q\n got: %q", want, got)
	}
}

func TestToVFSMountsCloudProviderAliases(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "gdrive", path: "gdrive://team-drive/designs", want: "/remote/gdrive/team-drive/designs"},
		{name: "dropbox", path: "dropbox://workspace/contracts", want: "/remote/dropbox/workspace/contracts"},
		{name: "onedrive", path: "onedrive://personal/docs", want: "/remote/onedrive/personal/docs"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToVFS(tt.path)
			if got != tt.want {
				t.Fatalf("ToVFS(cloud alias) mismatch\nwant: %q\n got: %q", tt.want, got)
			}
		})
	}
}

func TestNormalizeKeepsAliasSchemes(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "s3", path: "s3://minio-dev/photos/2026", want: "s3://minio-dev/photos/2026"},
		{name: "smb", path: "smb://fileserver/share/docs", want: "smb://fileserver/share/docs"},
		{name: "http", path: "https://example.com/assets/main.js", want: "https://example.com/assets/main.js"},
		{name: "onedrive", path: "onedrive://Projects/2026", want: "onedrive://Projects/2026"},
		{name: "gdrive", path: "gdrive://Shared/team", want: "gdrive://Shared/team"},
		{name: "dropbox", path: "dropbox://Photos/raw", want: "dropbox://Photos/raw"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Normalize(tt.path)
			if got != tt.want {
				t.Fatalf("Normalize(alias) mismatch\nwant: %q\n got: %q", tt.want, got)
			}
		})
	}
}

func TestToNativePathLeavesAliasesUntouched(t *testing.T) {
	tests := []string{
		"s3://minio-dev/photos/2026",
		"smb://fileserver/share/docs",
		"https://example.com/assets/main.js",
	}

	for _, input := range tests {
		if got := ToNativePath(input); got != input {
			t.Fatalf("ToNativePath(alias) should be unchanged\nwant: %q\n got: %q", input, got)
		}
	}
}

func TestToNativePathResolvesLocalCloudAliases(t *testing.T) {
	tempHome := t.TempDir()
	cloudRoot := filepath.Join(tempHome, "OneDrive")
	if err := os.MkdirAll(cloudRoot, 0o755); err != nil {
		t.Fatalf("mkdir cloud root: %v", err)
	}

	prevHomeFn := userHomeDirFn
	prevExistsFn := pathExistsFn
	userHomeDirFn = func() (string, error) { return tempHome, nil }
	pathExistsFn = func(path string) bool {
		info, err := os.Stat(path)
		return err == nil && info.IsDir()
	}
	t.Cleanup(func() {
		userHomeDirFn = prevHomeFn
		pathExistsFn = prevExistsFn
	})

	got := ToNativePath("onedrive://Projects/2026")
	want := filepath.Join(cloudRoot, filepath.FromSlash("Projects/2026"))
	if got != want {
		t.Fatalf("ToNativePath(cloud alias) mismatch\nwant: %q\n got: %q", want, got)
	}
}

func TestLocalCloudAliasFromNativePathUsesInstalledRoot(t *testing.T) {
	tempHome := t.TempDir()
	cloudRoot := filepath.Join(tempHome, "Dropbox")
	if err := os.MkdirAll(filepath.Join(cloudRoot, "Photos"), 0o755); err != nil {
		t.Fatalf("mkdir cloud child: %v", err)
	}

	prevHomeFn := userHomeDirFn
	prevExistsFn := pathExistsFn
	userHomeDirFn = func() (string, error) { return tempHome, nil }
	pathExistsFn = func(path string) bool {
		info, err := os.Stat(path)
		return err == nil && info.IsDir()
	}
	t.Cleanup(func() {
		userHomeDirFn = prevHomeFn
		pathExistsFn = prevExistsFn
	})

	got, ok := LocalCloudAliasFromNativePath(filepath.Join(cloudRoot, "Photos"))
	if !ok {
		t.Fatalf("LocalCloudAliasFromNativePath did not match")
	}

	want := "dropbox://Photos"
	if got != want {
		t.Fatalf("LocalCloudAliasFromNativePath mismatch\nwant: %q\n got: %q", want, got)
	}
}

func TestParentKeepsHTTPAliasesNavigable(t *testing.T) {
	got, ok := Parent("https://example.com/app/assets/main.js")
	if !ok {
		t.Fatalf("Parent(http alias) did not match")
	}

	want := "https://example.com/app/assets"
	if got != want {
		t.Fatalf("Parent(http alias) mismatch\nwant: %q\n got: %q", want, got)
	}
}

func TestIsWebRecognizesHTTPAliases(t *testing.T) {
	if !IsWeb("http://example.com") {
		t.Fatalf("expected http alias to be recognized as web")
	}
	if !IsWeb("https://example.com") {
		t.Fatalf("expected https alias to be recognized as web")
	}
	if IsWeb("smb://example.com/share") {
		t.Fatalf("did not expect smb alias to be recognized as web")
	}
}
