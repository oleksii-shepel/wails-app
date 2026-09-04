package legacy

import (
	"encoding/binary"
	"strings"
	"unicode/utf16"

	"office/pkg/officeart/legacy/escher"
)

// GeometryText is the common OfficeArt GeoText / WordArt payload.
type GeometryText struct {
	Present  bool
	Text     string
	Font     string
	Align    uint32
	Size     int32
	Spacing  int32
	Vertical bool
	Bold     bool
	Italic   bool
}

func DecodeGeometryText(props []escher.Property) GeometryText {
	var out GeometryText
	for _, prop := range props {
		switch prop.ID {
		case escher.PropGTextUnicode:
			if prop.IsComplex {
				out.Text = decodeGeometryTextUTF16(prop.Complex)
				if out.Text != "" {
					out.Present = true
				}
			}
		case escher.PropGTextFont:
			if prop.IsComplex {
				out.Font = decodeGeometryTextUTF16(prop.Complex)
			}
		case escher.PropGTextAlign:
			if !prop.IsComplex && !prop.IsBlip {
				out.Align = prop.Value
			}
		case escher.PropGTextSize:
			if !prop.IsComplex && !prop.IsBlip {
				out.Size = int32(prop.Value)
			}
		case escher.PropGTextSpacing:
			if !prop.IsComplex && !prop.IsBlip {
				out.Spacing = int32(prop.Value)
			}
		case escher.PropGTextBooleans:
			if prop.IsComplex || prop.IsBlip {
				continue
			}
			values := prop.Value & 0xffff
			use := prop.Value >> 16
			if use&(1<<1) != 0 && values&(1<<1) != 0 {
				out.Present = true
			}
			if use&(1<<2) != 0 {
				out.Vertical = values&(1<<2) != 0
			}
			if use&(1<<10) != 0 {
				out.Bold = values&(1<<10) != 0
			}
			if use&(1<<11) != 0 {
				out.Italic = values&(1<<11) != 0
			}
		}
	}
	if out.Text != "" {
		out.Present = true
	}
	return out
}

func decodeGeometryTextUTF16(data []byte) string {
	if len(data) < 2 {
		return ""
	}
	units := make([]uint16, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		u := binary.LittleEndian.Uint16(data[i : i+2])
		if u == 0 {
			break
		}
		units = append(units, u)
	}
	return strings.TrimSpace(string(utf16.Decode(units)))
}
