package legacy

import (
	"office/pkg/officeart/ir"
	"office/pkg/officeart/legacy/escher"
)

const emuPerPoint = 12700.0

// TextProperties is the shared interpretation of OfficeArt text-body FOPT
// properties. Presence flags are retained because an explicit zero value is
// semantically different from an absent property.
type TextProperties struct {
	HasLeft, HasTop, HasRight, HasBottom bool
	Insets                               ir.Insets

	HasWrap bool
	Wrap    ir.TextWrap

	HasAnchor    bool
	Anchor       ir.TextAnchor
	AnchorCenter bool

	HasVert bool
	Vert    ir.TextVertical

	HasAutoFit bool
	AutoFit    ir.AutoFit
}

func DecodeTextProperties(props []escher.Property) TextProperties {
	var out TextProperties
	for _, prop := range props {
		if prop.IsComplex || prop.IsBlip {
			continue
		}
		switch prop.ID {
		case escher.PropTextLeft:
			out.HasLeft = true
			out.Insets.Left = int64(int32(prop.Value))
		case escher.PropTextTop:
			out.HasTop = true
			out.Insets.Top = int64(int32(prop.Value))
		case escher.PropTextRight:
			out.HasRight = true
			out.Insets.Right = int64(int32(prop.Value))
		case escher.PropTextBottom:
			out.HasBottom = true
			out.Insets.Bottom = int64(int32(prop.Value))
		case escher.PropWrapText:
			out.HasWrap = true
			// OfficeArt TEXT__WRAPTEXT/MSOWRAPMODE uses 0 for square wrapping,
			// 1 for wrap-by-points and 2 for no-wrap. Values beyond the classic
			// set are wrapping modes as well; only the explicit NONE value disables
			// wrapping. Treating 0 as no-wrap made ordinary Word textboxes render
			// as a single clipped line instead of preserving Word's line breaks.
			if prop.Value == 2 {
				out.Wrap = ir.TextWrapNone
			} else {
				out.Wrap = ir.TextWrapSquare
			}
		case escher.PropAnchorText:
			out.HasAnchor = true
			switch prop.Value & 0xF {
			case 1, 4, 7:
				out.Anchor = ir.TextAnchorCenter
			case 2, 5, 8:
				out.Anchor = ir.TextAnchorBottom
			default:
				out.Anchor = ir.TextAnchorTop
			}
			out.AnchorCenter = prop.Value >= 3 && prop.Value <= 5
		case escher.PropTextFlow:
			out.HasVert = true
			out.Vert = TextVerticalFromFlow(prop.Value)
		case escher.PropTextBooleans:
			fit := prop.Value & 1
			use := (prop.Value >> 16) & 1
			if use == 1 {
				out.HasAutoFit = true
				if fit == 1 {
					out.AutoFit = ir.AutoFitShape
				} else {
					out.AutoFit = ir.AutoFitNone
				}
			}
		}
	}
	return out
}

func TextVerticalFromFlow(raw uint32) ir.TextVertical {
	// [MS-ODRAW] MSOTXFL:
	// 0 HorzN, 1 TtoBA, 2 BtoT, 3 TtoBN, 4 HorzA, 5 VertN.
	// GeometryText has its own WordArt vertical flag; txflTextFlow=3 is not it.
	switch raw & 0xF {
	case 1, 3, 5:
		return ir.TextVerticalRL
	case 2:
		return ir.TextVertical270
	default:
		return ir.TextHorizontal
	}
}

// PointsFromEMU exists for adapters that still store legacy text padding in
// points while migration is underway.
func PointsFromEMU(v int64) float64 { return float64(v) / emuPerPoint }
