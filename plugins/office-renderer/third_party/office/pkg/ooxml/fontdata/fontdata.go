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
