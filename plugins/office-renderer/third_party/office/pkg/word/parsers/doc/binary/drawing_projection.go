package docbinary

import (
	officeartlegacy "office/pkg/officeart/legacy"
	"office/pkg/ooxml/units"
	wordir "office/pkg/word/ir"
)

// AnchorPlacement is the normalized Word IR placement derived from FSPA.
type AnchorPlacement struct {
	X, Y          int64
	Width, Height int64
	XRelative     string
	YRelative     string
	WrapType      string
	WrapSide      string
	BehindDoc     bool
}

// PlacementFromAnchor projects signed FSPA twip geometry and positioning flags
// into the coordinate vocabulary shared by Word IR images, shapes and textboxes.
func PlacementFromAnchor(anchor FloatingAnchor) (AnchorPlacement, bool) {
	rect, ok := officeartlegacy.NormalizeRect(int64(anchor.Left), int64(anchor.Top), int64(anchor.Right), int64(anchor.Bottom))
	if !ok { return AnchorPlacement{}, false }
	wrapType := WrapTypeFromSpaFlags(anchor.Flags)
	return AnchorPlacement{
		X: units.TwipsToEMU(rect.X), Y: units.TwipsToEMU(rect.Y),
		Width: units.TwipsToEMU(rect.W), Height: units.TwipsToEMU(rect.H),
		XRelative: AnchorRelative(anchor, false), YRelative: AnchorRelative(anchor, true),
		WrapType: wrapType, WrapSide: WrapSideFromSpaFlags(anchor.Flags),
		BehindDoc: wrapType == "none" && anchor.BelowText,
	}, true
}

// AnchorRelative implements the FSPA origin rule, including fRcaSimple.
func AnchorRelative(anchor FloatingAnchor, vertical bool) string {
	if anchor.RcaSimple || anchor.Flags&0x2000 != 0 { return "page" }
	if vertical {
		switch anchor.BY { case 0: return "margin"; case 1: return "page"; case 2: return "paragraph"; default: return "page" }
	}
	switch anchor.BX { case 0: return "margin"; case 1: return "page"; case 2: return "column"; default: return "page" }
}

func WrapTypeFromSpaFlags(flags uint16) string {
	switch (flags >> 5) & 0x0F {
	case 0, 2: return "square"
	case 1: return "topAndBottom"
	case 3: return "none"
	case 4: return "tight"
	case 5: return "through"
	default: return "square"
	}
}

func WrapSideFromSpaFlags(flags uint16) string {
	switch (flags >> 9) & 0x0F { case 1: return "left"; case 2: return "right"; case 3: return "largest"; default: return "" }
}

// UsesAnchorLineFlow identifies the Word binary SHAPE-field convention where
// FSPA x/y are local offsets in an anchor-line sequence.
func UsesAnchorLineFlow(anchor FloatingAnchor) bool {
	return !anchor.RcaSimple && anchor.BX == 2 && anchor.BY == 2 && anchor.Wrap == 3
}

// ApplyAnchorToImage applies only DOC placement semantics; OfficeArt visual
// properties remain a separate projection layer.
func ApplyAnchorToImage(image *wordir.Image, anchor FloatingAnchor) bool {
	if image == nil { return false }
	p, ok := PlacementFromAnchor(anchor); if !ok { return false }
	image.Floating=true; image.XRelative=p.XRelative; image.YRelative=p.YRelative
	image.X=p.X; image.Y=p.Y; image.Width=p.Width; image.Height=p.Height
	image.WrapType=p.WrapType; image.WrapSide=p.WrapSide; image.BehindDoc=p.BehindDoc
	return true
}

func ApplyAnchorToShape(shape *wordir.Shape, anchor FloatingAnchor) bool {
	if shape == nil { return false }
	p, ok := PlacementFromAnchor(anchor); if !ok { return false }
	shape.LegacyShapeID=anchor.Shape; shape.LegacyFlowAnchor=UsesAnchorLineFlow(anchor)
	if shape.LegacyFlowAnchor { shape.LegacyFlowOrder=anchor.CP }
	shape.Floating=true; shape.XRelative=p.XRelative; shape.YRelative=p.YRelative
	shape.X=p.X; shape.Y=p.Y; shape.Width=p.Width; shape.Height=p.Height
	shape.WrapType=p.WrapType; shape.WrapSide=p.WrapSide; shape.BehindDoc=p.BehindDoc
	return true
}

func ApplyAnchorToTextBox(tb *wordir.TextBox, anchor FloatingAnchor) bool {
	if tb == nil { return false }
	p, ok := PlacementFromAnchor(anchor); if !ok { return false }
	tb.LegacyShapeID=anchor.Shape; tb.Floating=true; tb.XRelative=p.XRelative; tb.YRelative=p.YRelative
	tb.X=p.X; tb.Y=p.Y; tb.Width=p.Width; tb.Height=p.Height
	tb.WrapType=p.WrapType; tb.WrapSide=p.WrapSide; tb.BehindDoc=p.BehindDoc
	return true
}
