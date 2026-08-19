package archive

import (
	"testing"
)

func TestIsZipMagic(t *testing.T) {
	if IsZipMagic(nil) {
		t.Fatal("expected false for nil")
	}
	if IsZipMagic([]byte{0x50, 0x4b}) {
		t.Fatal("expected false for too-short data")
	}
	// Valid ZIP local file header signature
	if !IsZipMagic([]byte{0x50, 0x4b, 0x03, 0x04}) {
		t.Fatal("expected true for valid ZIP magic")
	}
	// Empty archive signature
	if !IsZipMagic([]byte{0x50, 0x4b, 0x05, 0x06}) {
		t.Fatal("expected true for empty ZIP magic")
	}
	if IsZipMagic([]byte{0x00, 0x00, 0x00, 0x00}) {
		t.Fatal("expected false for non-ZIP data")
	}
}

func TestIsRarArchiveName(t *testing.T) {
	if IsRarArchiveName("") {
		t.Fatal("expected false for empty")
	}
	if !IsRarArchiveName("file.rar") {
		t.Fatal("expected true for .rar")
	}
	if !IsRarArchiveName("FILE.RAR") {
		t.Fatal("expected true for .RAR")
	}
	if IsRarArchiveName("file.zip") {
		t.Fatal("expected false for .zip")
	}
}

func TestIsZipArchiveName(t *testing.T) {
	if isZipArchiveName("") {
		t.Fatal("expected false for empty")
	}
	if !isZipArchiveName("archive.zip") {
		t.Fatal("expected true for .zip")
	}
	if isZipArchiveName("archive.rar") {
		t.Fatal("expected false for .rar")
	}
}

func TestUsesStdlibArchiveHandling(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"archive.tar", true},
		{"archive.tar.gz", true},
		{"archive.tgz", true},
		{"archive.tar.bz2", true},
		{"archive.tar.xz", true},
		{"file.gz", true},
		{"file.bz2", true},
		{"file.xz", true},
		{"archive.zip", true},
		{"archive.rar", false},
		{"archive.7z", false},
	}
	for _, tc := range tests {
		got := usesStdlibArchiveHandling(tc.name)
		if got != tc.want {
			t.Fatalf("usesStdlibArchiveHandling(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestNeedsArchiveReader(t *testing.T) {
	if needsArchiveReader("") {
		t.Fatal("expected false for empty")
	}
	if !needsArchiveReader("file.rar") {
		t.Fatal("expected true for .rar (needs external reader)")
	}
	if needsArchiveReader("file.tar") {
		t.Fatal("expected false for .tar (uses stdlib)")
	}
}

func TestNormalizeArchiveEntryPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"folder/file.txt", "folder/file.txt"},
		{"\\folder\\file.txt", "folder/file.txt"},
		{"/folder/file.txt", "folder/file.txt"},
		{"/", ""},
	}
	for _, tc := range tests {
		got := normalizeArchiveEntryPath(tc.input)
		if got != tc.want {
			t.Fatalf("normalizeArchiveEntryPath(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestMatchesEntries(t *testing.T) {
	entries := []string{"docs", "src/main.go"}
	if !matchesEntries("docs/readme.md", entries) {
		t.Fatal("expected match for child of docs")
	}
	if !matchesEntries("docs", entries) {
		t.Fatal("expected exact match")
	}
	if matchesEntries("other/file.txt", entries) {
		t.Fatal("expected no match")
	}
	if !matchesEntries("src/main.go", entries) {
		t.Fatal("expected exact match for nested path")
	}
	if matchesEntries("src", entries) {
		t.Fatal("expected no match for parent prefix")
	}
}

func TestDetectArchiveExt(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"archive.tar.gz", ".tar.gz"},
		{"archive.tar.bz2", ".tar.bz2"},
		{"archive.tar.xz", ".tar.xz"},
		{"archive.tgz", ".tgz"},
		{"archive.tbz2", ".tbz2"},
		{"archive.zip", ".zip"},
		{"archive.rar", ".rar"},
		{"archive.7z", ".7z"},
		{"noext", ""},
	}
	for _, tc := range tests {
		got := detectArchiveExt(tc.name)
		if got != tc.want {
			t.Fatalf("detectArchiveExt(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}