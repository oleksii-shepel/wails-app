package contentprobe

import (
	"testing"
)

func TestNormalizeEncoding(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"UTF-8", "utf8"},
		{"utf-8", "utf8"},
		{"ISO-8859-1", "iso-8859-1"},
		{"WINDOWS-1252", "windows-1252"},
		{"windows1252", "windows-1252"},
		{"iso8859-1", "iso-8859-1"},
		{"", ""},
		{"unknown", "unknown"},
	}
	for _, tc := range tests {
		got := normalizeEncoding(tc.input)
		if got != tc.want {
			t.Fatalf("normalizeEncoding(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestDetectMime(t *testing.T) {
	tests := []struct {
		data []byte
		want string
	}{
		{[]byte("PK\x03\x04"), "application/zip"},
		{[]byte("hello"), "text/plain"},
		{[]byte{0x00, 0x01, 0x02}, "application/octet-stream"},
	}
	for _, tc := range tests {
		got := detectMime(tc.data)
		if got != tc.want {
			t.Fatalf("detectMime(%q) = %q, want %q", tc.data, got, tc.want)
		}
	}
}

func TestDetectArchiveMime(t *testing.T) {
	tests := []struct {
		data []byte
		want string
	}{
		{[]byte("PK\x03\x04"), "application/zip"},
		{[]byte("Rar!\x1a\x07\x01\x00"), "application/vnd.rar"},
		{[]byte("Rar!\x1a\x07\x00"), "application/vnd.rar"},
		{[]byte("7z\xbc\xaf\x27\x1c"), "application/x-7z-compressed"},
		{[]byte{0x1f, 0x8b}, "application/gzip"},
		{[]byte("BZh"), "application/x-bzip2"},
		{[]byte{0xfd, 0x37, 0x7a, 0x58, 0x5a, 0x00}, "application/x-xz"},
		{func() []byte { b := make([]byte, 512); copy(b, []byte("ustar")); return b }(), "application/x-tar"},
		{[]byte("hello"), ""},
	}
	for _, tc := range tests {
		got := detectArchiveMime(tc.data)
		if got != tc.want {
			t.Fatalf("detectArchiveMime(%q) = %q, want %q", tc.data, got, tc.want)
		}
	}
}

func TestIsArchiveMime(t *testing.T) {
	if !isArchiveMime("application/zip") {
		t.Fatal("expected zip to be archive mime")
	}
	if !isArchiveMime("application/vnd.rar") {
		t.Fatal("expected rar to be archive mime")
	}
	if isArchiveMime("text/plain") {
		t.Fatal("expected text not to be archive mime")
	}
}

func TestLooksLikeArchive(t *testing.T) {
	if !looksLikeArchive([]byte("PK\x03\x04")) {
		t.Fatal("expected ZIP to look like archive")
	}
	if looksLikeArchive([]byte("hello world")) {
		t.Fatal("expected text not to look like archive")
	}
}

func TestLooksLikeTar(t *testing.T) {
	data := make([]byte, 512)
	copy(data[257:], []byte("ustar"))
	if !looksLikeTar(data) {
		t.Fatal("expected ustar header to look like tar")
	}
	if looksLikeTar([]byte("short")) {
		t.Fatal("expected short data not to look like tar")
	}
}

func TestHasPrefix(t *testing.T) {
	if !hasPrefix([]byte("hello"), []byte("he")) {
		t.Fatal("expected prefix match")
	}
	if hasPrefix([]byte("hi"), []byte("hello")) {
		t.Fatal("expected no prefix match")
	}
}

func TestNormalizeMime(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"text/plain; charset=utf-8", "text/plain"},
		{"application/json", "application/json"},
		{"", ""},
	}
	for _, tc := range tests {
		got := normalizeMime(tc.input)
		if got != tc.want {
			t.Fatalf("normalizeMime(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestIsTextMime(t *testing.T) {
	if !isTextMime("text/plain") {
		t.Fatal("expected text/plain to be text")
	}
	if !isTextMime("application/json") {
		t.Fatal("expected json to be text")
	}
	if !isTextMime("text/html") {
		t.Fatal("expected html to be text")
	}
	if isTextMime("image/png") {
		t.Fatal("expected png not to be text")
	}
}

func TestLooksLikeText(t *testing.T) {
	if !looksLikeText([]byte("hello world")) {
		t.Fatal("expected ASCII to look like text")
	}
	if looksLikeText([]byte{0x00, 0x01, 0x02, 0x03}) {
		t.Fatal("expected binary not to look like text")
	}
	// Tab, CR, LF are allowed
	if !looksLikeText([]byte("line1\tline2\r\n")) {
		t.Fatal("expected tabs and newlines to be text")
	}
}

func TestLooksLikeMPEGTS(t *testing.T) {
	data := make([]byte, 188*3)
	data[0] = 0x47
	data[188] = 0x47
	data[188*2] = 0x47
	if !looksLikeMPEGTS(data) {
		t.Fatal("expected valid MPEGTS sync bytes")
	}
	if looksLikeMPEGTS([]byte("not mpegts")) {
		t.Fatal("expected non-MPEGTS")
	}
}

func TestLooksLikeUTF16LE(t *testing.T) {
	// "A" in UTF-16LE is 0x41 0x00, repeated
	data := []byte{0x41, 0x00, 0x42, 0x00, 0x43, 0x00}
	if !looksLikeUTF16LE(data) {
		t.Fatal("expected UTF-16LE pattern")
	}
	if looksLikeUTF16LE([]byte("ABC")) {
		t.Fatal("expected ASCII not to look like UTF-16LE")
	}
}

func TestLooksLikeUTF16BE(t *testing.T) {
	// "A" in UTF-16BE is 0x00 0x41, repeated
	data := []byte{0x00, 0x41, 0x00, 0x42, 0x00, 0x43}
	if !looksLikeUTF16BE(data) {
		t.Fatal("expected UTF-16BE pattern")
	}
	if looksLikeUTF16BE([]byte("ABC")) {
		t.Fatal("expected ASCII not to look like UTF-16BE")
	}
}

func TestHasCP1252OnlyBytes(t *testing.T) {
	// Smart quote is CP1252-only
	if !hasCP1252OnlyBytes([]byte{0x92}) {
		t.Fatal("expected smart quote to be CP1252-only")
	}
	if hasCP1252OnlyBytes([]byte("hello")) {
		t.Fatal("expected ASCII not to have CP1252-only bytes")
	}
}

func TestDecodedTextScore(t *testing.T) {
	score := decodedTextScore("hello world")
	if score <= 0 {
		t.Fatal("expected positive score for ASCII text")
	}
	bad := decodedTextScore("\x00\x01\x02\x03")
	if bad != 0 {
		t.Fatalf("expected 0 score for garbage, got %f", bad)
	}
}

func TestStripBOM(t *testing.T) {
	utf8BOM := []byte{0xef, 0xbb, 0xbf, 0x68, 0x69}
	got := stripBOM(utf8BOM)
	if string(got) != "hi" {
		t.Fatalf("expected 'hi', got %q", string(got))
	}
	noBOM := []byte("hello")
	if string(stripBOM(noBOM)) != "hello" {
		t.Fatal("expected unchanged without BOM")
	}
}
