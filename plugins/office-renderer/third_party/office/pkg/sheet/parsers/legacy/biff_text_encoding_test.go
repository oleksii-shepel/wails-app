package legacy

import (
	"encoding/binary"
	"testing"
	"unicode/utf8"
)

// TestDecodeBiffBytesDecodesCompressedUnicodeUnderUnicodeCodepage covers the
// BIFF8 default CODEPAGE 1200. Compressed strings hold one Unicode code point
// per byte, so a payload like "Constituci\xf3n" must be decoded to UTF-8 rather
// than copied verbatim into the HTML output.
func TestDecodeBiffBytesDecodesCompressedUnicodeUnderUnicodeCodepage(t *testing.T) {
	defer func(prev int) { activeBiffCodepage = prev }(activeBiffCodepage)
	activeBiffCodepage = 1200

	got := decodeBiffBytes([]byte{'C', 'o', 'n', 's', 't', 'i', 't', 'u', 'c', 'i', 0xF3, 'n'})
	if got != "Constitución" {
		t.Fatalf("decodeBiffBytes = %q, want %q", got, "Constitución")
	}
	if !utf8.ValidString(got) {
		t.Fatalf("decodeBiffBytes produced invalid UTF-8: %q", got)
	}
}

// TestDecodeBiffBytesRawDecodesLatin1WithoutCodepage covers workbooks that never
// carry a CODEPAGE record: single-byte BIFF text is still Latin-1, never raw
// bytes.
func TestDecodeBiffBytesRawDecodesLatin1WithoutCodepage(t *testing.T) {
	defer func(prev int) { activeBiffCodepage = prev }(activeBiffCodepage)
	activeBiffCodepage = 0

	got := decodeBiffBytesRaw([]byte{'M', 0xE9, 'x', 'i', 'c', 'o'})
	if got != "México" {
		t.Fatalf("decodeBiffBytesRaw = %q, want %q", got, "México")
	}
	if !utf8.ValidString(got) {
		t.Fatalf("decodeBiffBytesRaw produced invalid UTF-8: %q", got)
	}
}

// TestParseSharedStringsDecodesCompressedLatin1Text pins the SST path that feeds
// cell text: a compressed shared string with 0xF3 must come back as ó.
func TestParseSharedStringsDecodesCompressedLatin1Text(t *testing.T) {
	defer func(prev int) { activeBiffCodepage = prev }(activeBiffCodepage)
	activeBiffCodepage = 1200

	text := []byte("Constituci\xf3n")
	payload := make([]byte, 8+2+1+len(text))
	binary.LittleEndian.PutUint32(payload[0:4], 1)
	binary.LittleEndian.PutUint32(payload[4:8], 1) // unique string count
	binary.LittleEndian.PutUint16(payload[8:10], uint16(len(text)))
	payload[10] = 0x00 // fHighByte: compressed (one byte per character)
	copy(payload[11:], text)

	got := parseSharedStrings(payload, nil, [56]string{})
	if len(got) != 1 {
		t.Fatalf("shared strings = %d, want 1", len(got))
	}
	if got[0].Text != "Constitución" {
		t.Fatalf("shared string text = %q, want %q", got[0].Text, "Constitución")
	}
}
