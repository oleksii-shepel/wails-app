package legacy

import (
	"encoding/binary"
	"strings"
	"unicode/utf16"

	"office/pkg/officeart/legacy/escher"
)

// GeometryText is the common OfficeArt GeoText / WordArt payload.
//
// Size and Spacing preserve the authored 16.16 fixed-point values. Align keeps
// the OfficeArt enumeration value losslessly; application adapters can decide
// how closely their target renderer can reproduce the alignment mode.
type GeometryText struct {
	Present       bool
	Text          string
	Font          string
	Align         uint32
	Size          int32
	Spacing       int32
	ReverseRows   bool
	Vertical      bool
	Kerning       bool
	Tight         bool
	Stretch       bool
	ShrinkFit     bool
	BestFit       bool
	Normalize     bool
	DxMeasure     bool
	Bold          bool
	Italic        bool
	Underline     bool
	Shadow        bool
	SmallCaps     bool
	Strikethrough bool
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
			// MS-ODRAW 2.3.22.10 pairs a "use" bit with its value bit: the low
			// 16 bits carry the fUse* flags, the high 16 the values themselves.
			use := uint16(prop.Value)
			values := uint16(prop.Value >> 16)
			get := func(bit uint) (bool, bool) {
				mask := uint16(1 << bit)
				if use&mask == 0 {
					return false, false
				}
				return values&mask != 0, true
			}
			if value, authored := get(0); authored {
				out.ReverseRows = value
			}
			if value, authored := get(1); authored && value {
				out.Present = true
			}
			if value, authored := get(2); authored {
				out.Vertical = value
			}
			if value, authored := get(3); authored {
				out.Kerning = value
			}
			if value, authored := get(4); authored {
				out.Tight = value
			}
			if value, authored := get(5); authored {
				out.Stretch = value
			}
			if value, authored := get(6); authored {
				out.ShrinkFit = value
			}
			if value, authored := get(7); authored {
				out.BestFit = value
			}
			if value, authored := get(8); authored {
				out.Normalize = value
			}
			if value, authored := get(9); authored {
				out.DxMeasure = value
			}
			if value, authored := get(10); authored {
				out.Bold = value
			}
			if value, authored := get(11); authored {
				out.Italic = value
			}
			if value, authored := get(12); authored {
				out.Underline = value
			}
			if value, authored := get(13); authored {
				out.Shadow = value
			}
			if value, authored := get(14); authored {
				out.SmallCaps = value
			}
			if value, authored := get(15); authored {
				out.Strikethrough = value
			}
		}
	}
	if out.Text != "" {
		out.Present = true
	}
	return out
}

// WordArtWarpName normalizes the OfficeArt text-effect shape range into stable
// semantic names. Non-WordArt shape types return an empty string.
func WordArtWarpName(shapeType uint16) string {
	// [MS-ODRAW] has two text-effect ranges:
	//   24..31  : TextSimple .. TextOnRing
	//   136..175: TextPlainText .. TextCanDown
	if shapeType >= 24 && shapeType <= 31 {
		names := [...]string{
			"textSimple", "textOctagon", "textHexagon", "textCurve",
			"textWave", "textRing", "textOnCurve", "textOnRing",
		}
		return names[shapeType-24]
	}
	if shapeType >= 136 && shapeType <= 175 {
		names := [...]string{
			"textPlain", "textStop", "textTriangle", "textTriangleInverted",
			"textChevron", "textChevronInverted", "textRingInside", "textRingOutside",
			"textArchUpCurve", "textArchDownCurve", "textCircleCurve", "textButtonCurve",
			"textArchUpPour", "textArchDownPour", "textCirclePour", "textButtonPour",
			"textCurveUp", "textCurveDown", "textCascadeUp", "textCascadeDown",
			"textWave1", "textWave2", "textWave3", "textWave4",
			"textInflate", "textDeflate", "textInflateBottom", "textDeflateBottom",
			"textInflateTop", "textDeflateTop", "textDeflateInflate", "textDeflateInflateDeflate",
			"textFadeRight", "textFadeLeft", "textFadeUp", "textFadeDown",
			"textSlantUp", "textSlantDown", "textCanUp", "textCanDown",
		}
		return names[shapeType-136]
	}
	return ""
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
