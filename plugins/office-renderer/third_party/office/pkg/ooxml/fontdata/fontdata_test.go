package fontdata

import (
	"archive/zip"
	"io"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDecodeEOTSample(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only decoder")
	}
	path := filepath.Join("..", "..", "..", "testdata", "pptx", "sample-2.pptx")
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer zr.Close()
	var data []byte
	for _, f := range zr.File {
		if f.Name == "ppt/fonts/Oswald-regular.fntdata" {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open font part: %v", err)
			}
			defer rc.Close()
			data, err = io.ReadAll(rc)
			if err != nil {
				t.Fatalf("read font part: %v", err)
			}
			break
		}
	}
	if len(data) == 0 {
		t.Fatal("missing font bytes")
	}
	out, ok := decodeEOT(data, "Oswald")
	t.Logf("decoded=%v size=%d", ok, len(out))
	if ok {
		if len(out) >= 4 {
			t.Logf("head=% x", out[:4])
		}
	}
}

func TestDetectRawSource(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		wantMIME string
		wantFmt  string
	}{
		{"woff", []byte("wOFF extra data"), "font/woff", "woff"},
		{"woff2", []byte("wOF2 extra data"), "font/woff2", "woff2"},
		{"opentype", []byte("OTTO extra data"), "font/opentype", "opentype"},
		{"truetype magic", []byte("true extra data"), "font/truetype", "truetype"},
		{"truetype hex", []byte{0x00, 0x01, 0x00, 0x00, 0x00}, "font/truetype", "truetype"},
		{"empty", nil, "", ""},
		{"too short", []byte("abc"), "", ""},
		{"unknown", []byte("abcd"), "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mime, fmt := detectRawSource(tt.data)
			if mime != tt.wantMIME || fmt != tt.wantFmt {
				t.Errorf("detectRawSource(%x) = (%q, %q), want (%q, %q)", tt.data, mime, fmt, tt.wantMIME, tt.wantFmt)
			}
		})
	}
}

func eotHeader(v0, v1, v2, v3 byte) []byte {
	// EOT header format (all little-endian):
	//   offset 0-7:  reserved (usually 0)
	//   offset 8-11: version (uint32 LE): 0x00010000 | 0x00020001 | 0x00020002
	//   offset 34-35: signature (uint16 LE): 0x504C
	b := make([]byte, 36)
	b[8], b[9], b[10], b[11] = v0, v1, v2, v3
	b[34], b[35] = 0x4C, 0x50 // LE for 0x504C
	return b
}

func TestDetectEOT(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		contentType string
		wantMIME    string
		wantFmt     string
	}{
		{"valid eot header v1", eotHeader(0x00, 0x00, 0x01, 0x00), "", "application/vnd.ms-fontobject", "embedded-opentype"},
		{"valid eot header v2", eotHeader(0x01, 0x00, 0x02, 0x00), "", "application/vnd.ms-fontobject", "embedded-opentype"},
		{"eot content type", []byte("not eot"), "application/vnd.ms-fontobject", "application/vnd.ms-fontobject", "embedded-opentype"},
		{"no match", []byte("not eot"), "image/png", "", ""},
		{"too short", []byte("short"), "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mime, fmt := detectEOT(tt.data, tt.contentType)
			if mime != tt.wantMIME || fmt != tt.wantFmt {
				t.Errorf("detectEOT(%x, %q) = (%q, %q), want (%q, %q)", tt.data, tt.contentType, mime, fmt, tt.wantMIME, tt.wantFmt)
			}
		})
	}
}

func TestLooksLikeEOT(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"version 0x00010000", eotHeader(0x00, 0x00, 0x01, 0x00), true},
		{"version 0x00020001", eotHeader(0x01, 0x00, 0x02, 0x00), true},
		{"version 0x00020002", eotHeader(0x02, 0x00, 0x02, 0x00), true},
		{"bad version", func() []byte {
			b := eotHeader(0x00, 0x00, 0x01, 0x00)
			b[8], b[9], b[10], b[11] = 0x01, 0x00, 0x00, 0x00
			return b
		}(), false},
		{"bad signature", func() []byte {
			b := eotHeader(0x00, 0x00, 0x01, 0x00)
			b[34], b[35] = 0x00, 0x00
			return b
		}(), false},
		{"too short", []byte("short"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := looksLikeEOT(tt.data); got != tt.want {
				t.Errorf("looksLikeEOT() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEmbeddedSources(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		contentType string
		family      string
		wantCount   int
		wantMIME    string
	}{
		{"nil data", nil, "", "", 0, ""},
		{"empty data", []byte{}, "", "", 0, ""},
		{"woff data", []byte("wOFF payload"), "", "", 1, "font/woff"},
		{"truetype data", []byte{0x00, 0x01, 0x00, 0x00, 'a', 'b', 'c'}, "", "", 1, "font/truetype"},
		{"opentype data", []byte("OTTO payload"), "", "", 1, "font/opentype"},
		{"unknown data", []byte("unknown font format"), "", "", 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sources := EmbeddedSources(tt.data, tt.contentType, tt.family)
			if len(sources) != tt.wantCount {
				t.Fatalf("EmbeddedSources() = %d sources, want %d", len(sources), tt.wantCount)
			}
			if tt.wantCount > 0 && sources[0].MIME != tt.wantMIME {
				t.Errorf("source[0].MIME = %q, want %q", sources[0].MIME, tt.wantMIME)
			}
		})
	}
}

func TestEmbeddedSources_EOTFallback(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("EOT decoding is windows-only")
	}
	// Build a minimal valid EOT header so EOT detection works.
	data := eotHeader(0x00, 0x00, 0x01, 0x00)

	sources := EmbeddedSources(data, "", "TestFamily")
	// On Windows, EOT decode will fail (the data is a header-only stub),
	// so it should fall back to the raw EOT detection.
	if len(sources) > 0 {
		if sources[0].MIME != "application/vnd.ms-fontobject" {
			t.Errorf("expected EOT fallback MIME, got %q", sources[0].MIME)
		}
	}
}

func TestEmbeddedSources_EOTContentType(t *testing.T) {
	// Even for data that doesn't look like EOT, the content type should be
	// detected when it matches the EOT content type.
	sources := EmbeddedSources([]byte("not really eot"), "application/vnd.ms-fontobject", "")
	if len(sources) != 1 {
		t.Fatalf("expected 1 source from content-type match, got %d", len(sources))
	}
	if sources[0].MIME != "application/vnd.ms-fontobject" {
		t.Errorf("expected EOT MIME, got %q", sources[0].MIME)
	}
	if sources[0].Format != "embedded-opentype" {
		t.Errorf("expected embedded-opentype format, got %q", sources[0].Format)
	}
}

func TestLoadEmbeddedFont_NilChecks(t *testing.T) {
	// Ensure nil-safe behavior when nil package or nil byID map.
	if got := LoadEmbeddedFont(nil, "", "MyFont", StyleRefs{}, nil); got != nil {
		t.Error("expected nil for nil package")
	}
	if got := LoadEmbeddedFont(nil, "", "", StyleRefs{}, nil); got != nil {
		t.Error("expected nil for empty family")
	}
}

func TestEmbeddedFontData_EmptyFamily(t *testing.T) {
	if ef := LoadEmbeddedFont(nil, "", "  ", StyleRefs{}, nil); ef != nil {
		t.Error("expected nil for whitespace-only family")
	}
	if ef := LoadEmbeddedFont(nil, "", "", StyleRefs{}, nil); ef != nil {
		t.Error("expected nil for empty family")
	}
}

func TestEmbeddedSources_TTFDualSource(t *testing.T) {
	// When the raw data looks like TTF and also has EOT structure, TTF detection wins.
	ttfHeader := []byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00}
	// Pad to 36 bytes with EOT signature
	for len(ttfHeader) < 36 {
		ttfHeader = append(ttfHeader, 0)
	}
	ttfHeader[34], ttfHeader[35] = 0x4C, 0x50

	sources := EmbeddedSources(ttfHeader, "", "Test")
	if len(sources) == 0 {
		t.Fatal("expected at least TTF source")
	}
	if sources[0].MIME != "font/truetype" {
		t.Errorf("source[0].MIME = %q, want font/truetype", sources[0].MIME)
	}
}

func TestEmbeddedSources_CSSFamilyInSource(t *testing.T) {
	// Verify that the returned sources contain the original data.
	data := []byte("wOFF actual font data")
	sources := EmbeddedSources(data, "", "")
	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}
	if string(sources[0].Data) != string(data) {
		t.Error("source data does not match input data")
	}
}
