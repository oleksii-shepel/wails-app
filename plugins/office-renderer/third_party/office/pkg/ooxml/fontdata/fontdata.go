package fontdata

import (
	"encoding/binary"
	"strings"
)

// Source is a browser-consumable font asset.
type Source struct {
	MIME   string
	Format string
	Data   []byte
}

// EmbeddedSources returns CSS font sources for the supplied embedded font data.
// It prefers browser-friendly sfnt payloads and appends the original EOT blob
// as a fallback when necessary.
func EmbeddedSources(data []byte, contentType, family string) []Source {
	if len(data) == 0 {
		return nil
	}

	if mime, format := detectRawSource(data); mime != "" && format != "" {
		return []Source{{MIME: mime, Format: format, Data: data}}
	}

	if decoded, ok := decodeEOT(data, family); ok {
		if mime, format := detectRawSource(decoded); mime != "" && format != "" {
			sources := []Source{{MIME: mime, Format: format, Data: decoded}}
			if emime, eformat := detectEOT(data, contentType); emime != "" && eformat != "" {
				// Keep the original EOT wrapper as a final fallback for older browsers.
				sources = append(sources, Source{MIME: emime, Format: eformat, Data: data})
			}
			return sources
		}
	}

	if mime, format := detectEOT(data, contentType); mime != "" && format != "" {
		return []Source{{MIME: mime, Format: format, Data: data}}
	}
	return nil
}

func detectRawSource(data []byte) (string, string) {
	if len(data) >= 4 {
		switch string(data[:4]) {
		case "wOFF":
			return "font/woff", "woff"
		case "wOF2":
			return "font/woff2", "woff2"
		case "OTTO":
			return "font/opentype", "opentype"
		case "true":
			return "font/truetype", "truetype"
		}
		if binary.BigEndian.Uint32(data[:4]) == 0x00010000 {
			return "font/truetype", "truetype"
		}
	}
	return "", ""
}

func detectEOT(data []byte, contentType string) (string, string) {
	if looksLikeEOT(data) || strings.EqualFold(strings.TrimSpace(contentType), "application/vnd.ms-fontobject") {
		return "application/vnd.ms-fontobject", "embedded-opentype"
	}
	return "", ""
}

func looksLikeEOT(data []byte) bool {
	if len(data) < 36 {
		return false
	}
	version := binary.LittleEndian.Uint32(data[8:12])
	if version != 0x00010000 && version != 0x00020001 && version != 0x00020002 {
		return false
	}
	return binary.LittleEndian.Uint16(data[34:36]) == 0x504C
}

// fontKeyBytes converts a w:fontKey GUID ("{XXXXXXXX-XXXX-...}") into the
// 16-byte ODTTF obfuscation key. ECMA-376 Part 1 §17.8.1 stores the key as the
// GUID's hex digits in reverse byte order.
func fontKeyBytes(fontKey string) []byte {
	hex := make([]byte, 0, 32)
	for _, r := range fontKey {
		switch {
		case r >= '0' && r <= '9':
			hex = append(hex, byte(r))
		case r >= 'a' && r <= 'f', r >= 'A' && r <= 'F':
			hex = append(hex, byte(r))
		}
	}
	if len(hex) != 32 {
		return nil
	}
	key := make([]byte, 16)
	for i := 0; i < 16; i++ {
		hi, okHi := hexNibble(hex[2*i])
		lo, okLo := hexNibble(hex[2*i+1])
		if !okHi || !okLo {
			return nil
		}
		// Reverse the byte order: the last pair is the first key byte.
		key[15-i] = hi<<4 | lo
	}
	return key
}

func hexNibble(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

// Deobfuscate undoes Word's embedded-font (ODTTF) obfuscation: the first 32
// bytes are XORed with the 16-byte font key repeated twice. Data without a
// usable key is returned unchanged so callers can pass parts of any provenance.
func Deobfuscate(data []byte, fontKey string) []byte {
	if len(data) == 0 {
		return data
	}
	key := fontKeyBytes(fontKey)
	if len(key) == 0 {
		return data
	}
	out := append([]byte(nil), data...)
	limit := 32
	if len(out) < limit {
		limit = len(out)
	}
	for i := 0; i < limit; i++ {
		out[i] ^= key[i%16]
	}
	return out
}

// EmbeddedSourcesForKey resolves browser/measurement sources for an embedded
// font part, de-obfuscating it first when a Word font key is present.
func EmbeddedSourcesForKey(data []byte, contentType, family, fontKey string) []Source {
	return EmbeddedSources(Deobfuscate(data, fontKey), contentType, family)
}
