package drawing

import (
	"office/pkg/officeart/legacy"
	"office/pkg/ooxml/units"
	docbinary "office/pkg/word/parsers/doc/binary"
)

// Placement is the Word-specific projection of an FSPA anchor into the
// normalized Word drawing coordinate model. OfficeArt owns shape appearance;
// DOC owns where that shape participates in the document layout.
type Placement struct {
	X, Y          int64
	Width, Height int64
	XRelative     string
	YRelative     string
	WrapType      string
	WrapSide      string
	BehindDoc     bool
}

// PlacementFromAnchor converts signed FSPA twips and decoded SPA flags into
// the canonical drawing placement used by images, shapes and text boxes.
func PlacementFromAnchor(anchor docbinary.FloatingAnchor) (Placement, bool) {
	rect, ok := legacy.NormalizeRect(int64(anchor.Left), int64(anchor.Top), int64(anchor.Right), int64(anchor.Bottom))
	if !ok {
		return Placement{}, false
	}
	wrapType := WrapType(anchor)
	return Placement{
		X: units.TwipsToEMU(rect.X), Y: units.TwipsToEMU(rect.Y),
		Width: units.TwipsToEMU(rect.W), Height: units.TwipsToEMU(rect.H),
		XRelative: Relative(anchor, false), YRelative: Relative(anchor, true),
		WrapType: wrapType, WrapSide: WrapSide(anchor),
		BehindDoc: wrapType == "none" && anchor.BelowText,
	}, true
}

// Relative implements the FSPA origin rule. fRcaSimple overrides bx/by and
// makes the complete rectangle page-relative.
func Relative(anchor docbinary.FloatingAnchor, vertical bool) string {
	if anchor.RcaSimple || anchor.Flags&0x2000 != 0 {
		return "page"
	}
	if vertical {
		switch anchor.BY {
		case 0: return "margin"
		case 1: return "page"
		case 2: return "paragraph"
		default: return "page"
		}
	}
	switch anchor.BX {
	case 0: return "margin"
	case 1: return "page"
	case 2: return "column"
	default: return "page"
	}
}

// WrapType maps the FSPA wrapping mode to Word IR terminology.
func WrapType(anchor docbinary.FloatingAnchor) string {
	wrap := anchor.Wrap
	if wrap == 0 && anchor.Flags != 0 { wrap = uint8((anchor.Flags >> 5) & 0x0f) }
	switch wrap {
	case 0: return "square"
	case 1: return "tight"
	case 2: return "through"
	case 3: return "none"
	case 4: return "topAndBottom"
	default: return "square"
	}
}

// WrapSide maps the FSPA wrap-side field to Word IR terminology.
func WrapSide(anchor docbinary.FloatingAnchor) string {
	side := anchor.WrapSide
	if side == 0 && anchor.Flags != 0 { side = uint8((anchor.Flags >> 9) & 0x0f) }
	switch side {
	case 1: return "left"
	case 2: return "right"
	case 3: return "largest"
	default: return "both"
	}
}
