package contentprobe

import (
	"bytes"
	"testing"
)

func TestProbeBytesDetectsMPEGTS(t *testing.T) {
	sample := make([]byte, 188*4)
	for i := 0; i < 4; i++ {
		sample[i*188] = 0x47
	}

	probe := ProbeBytes(sample)
	if probe.MimeType != "video/mp2t" {
		t.Fatalf("expected video/mp2t, got %q", probe.MimeType)
	}
	if probe.IsText {
		t.Fatalf("expected MPEG-TS probe to be binary")
	}
}

func TestProbeBytesDetectsZipArchive(t *testing.T) {
	sample := append([]byte("PK\x03\x04"), make([]byte, 32)...)

	probe := ProbeBytes(sample)
	if !probe.IsArchive {
		t.Fatalf("expected zip probe to be archive")
	}
	if probe.MimeType != "application/zip" {
		t.Fatalf("expected application/zip, got %q", probe.MimeType)
	}
}

func TestDecodeUTF16LE(t *testing.T) {
	data := append([]byte{0xFF, 0xFE}, []byte("h\x00i\x00")...)
	out, err := Decode(data, "utf16le")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if out != "hi" {
		t.Fatalf("expected hi, got %q", out)
	}
}

func TestProbeReaderDetectsTextEncoding(t *testing.T) {
	data := []byte("hello, world\n")
	_, probe, err := ProbeReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("probe failed: %v", err)
	}
	if !probe.IsText {
		t.Fatalf("expected text probe")
	}
	if probe.Encoding != "utf8" {
		t.Fatalf("expected utf8, got %q", probe.Encoding)
	}
}

func TestDetectEncodingUTF16LEWithoutBOM(t *testing.T) {
	data := []byte("h\x00e\x00l\x00l\x00o\x00 \x00w\x00o\x00r\x00l\x00d\x00")
	if got := DetectEncoding(data); got != "utf16le" {
		t.Fatalf("expected utf16le, got %q", got)
	}
}

func TestDetectEncodingPrefersWindows1252ForSmartQuotes(t *testing.T) {
	data := []byte{0x93, 'H', 'e', 'l', 'l', 'o', 0x94}
	if got := DetectEncoding(data); got != "windows-1252" {
		t.Fatalf("expected windows-1252, got %q", got)
	}
}

func TestDetectEncodingReturnsISO88591ForLatinTextWithoutCP1252OnlyBytes(t *testing.T) {
	data := []byte("caf\xe9 avec cr\xe8me")
	if got := DetectEncoding(data); got != "iso-8859-1" {
		t.Fatalf("expected iso-8859-1, got %q", got)
	}
}
