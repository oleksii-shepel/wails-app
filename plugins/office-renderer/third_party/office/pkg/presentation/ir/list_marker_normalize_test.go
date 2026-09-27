package ir

import "testing"

func TestNormalizeDuplicateBulletGlyphShapesFindsTextInsideSiblingGroup(t *testing.T) {
	text := &Shape{Kind: ShapeText, Bounds: Rect{X: 96, Y: 864, W: 1824, H: 3168}, Paragraphs: []*Paragraph{{
		BulletChar: "•", BulletColor: "FFFF00", Runs: []*Run{{Text: "GPS Block II", Color: "FFFF00"}},
	}}}
	inner := &Shape{Kind: ShapeGroup, Bounds: Rect{X: 48, Y: 864, W: 1872, H: 3168}, Children: []*Shape{text}}
	dot := &Shape{Geometry: "ellipse", FillColor: "FFFF00", Bounds: Rect{X: 100, Y: 1916, W: 48, H: 48}}
	outer := &Shape{Kind: ShapeGroup, Children: []*Shape{inner, dot}}
	p := &Presentation{Slides: []*Slide{{Shapes: []*Shape{outer}}}}

	NormalizeDuplicateBulletGlyphShapes(p)
	if got := len(outer.Children); got != 1 || outer.Children[0] != inner {
		t.Fatalf("duplicate nested bullet ellipse survived: %#v", outer.Children)
	}
}

func TestNormalizeDuplicateBulletGlyphShapesUsesFollowTextColor(t *testing.T) {
	text := &Shape{Kind: ShapeText, Bounds: Rect{X: 457200, Y: 1371600, W: 8458200, H: 5029200}, Paragraphs: []*Paragraph{{
		BulletChar: "•", BulletColor: "FFFFFF", BulletColorExplicit: false,
		Runs: []*Run{{Text: "Tools for SQM Analysis", Color: "B2B2B2"}},
	}}}
	dot := &Shape{Geometry: "ellipse", FillColor: "B2B2B2", Bounds: Rect{X: 463550, Y: 5356225, W: 88900, H: 85725}}
	p := &Presentation{Slides: []*Slide{{Shapes: []*Shape{text, dot}}}}

	NormalizeDuplicateBulletGlyphShapes(p)
	if got := len(p.Slides[0].Shapes); got != 1 || p.Slides[0].Shapes[0] != text {
		t.Fatalf("duplicate OOXML bullet ellipse survived: %#v", p.Slides[0].Shapes)
	}
	if got := text.Paragraphs[0].BulletColor; got != "B2B2B2" {
		t.Fatalf("follow-text bullet color = %q, want B2B2B2", got)
	}
}

func TestNormalizeDuplicateBulletGlyphShapesRemovesEllipseTextShapeWithEmptyParagraph(t *testing.T) {
	text := &Shape{Kind: ShapeText, Bounds: Rect{X: 96, Y: 864, W: 1824, H: 3168}, Paragraphs: []*Paragraph{{
		BulletChar: "•", BulletColor: "FFFFFF", BulletColorExplicit: false,
		Runs: []*Run{{Text: "GPS Block II", Color: "FFFF00"}},
	}}}
	dot := &Shape{Kind: ShapeText, Geometry: "ellipse", FillColor: "FFFF00", Bounds: Rect{X: 100, Y: 1916, W: 48, H: 48}, Paragraphs: []*Paragraph{{}}}
	p := &Presentation{Slides: []*Slide{{Shapes: []*Shape{text, dot}}}}

	NormalizeDuplicateBulletGlyphShapes(p)
	if got := len(p.Slides[0].Shapes); got != 1 || p.Slides[0].Shapes[0] != text {
		t.Fatalf("duplicate bullet ellipse text shape survived: %#v", p.Slides[0].Shapes)
	}
	if got := text.Paragraphs[0].BulletColor; got != "FFFF00" {
		t.Fatalf("follow-text bullet color = %q, want FFFF00", got)
	}
}

func TestNormalizeDuplicateBulletGlyphShapesPreservesDifferentAuthoredBulletColor(t *testing.T) {
	text := &Shape{Kind: ShapeText, Bounds: Rect{X: 100, Y: 100, W: 2000, H: 3000}, Paragraphs: []*Paragraph{{
		BulletChar: "•", BulletColor: "FFFFFF", BulletColorExplicit: true,
		Runs: []*Run{{Text: "Item", Color: "B2B2B2"}},
	}}}
	dot := &Shape{Geometry: "ellipse", FillColor: "B2B2B2", Bounds: Rect{X: 104, Y: 1100, W: 50, H: 48}}
	p := &Presentation{Slides: []*Slide{{Shapes: []*Shape{text, dot}}}}

	NormalizeDuplicateBulletGlyphShapes(p)
	if got := len(p.Slides[0].Shapes); got != 2 {
		t.Fatalf("authored independent-color ellipse removed: got %d shapes", got)
	}
}

func TestNormalizeLegacyListMarkerGlyphsCanonicalizesWingdingsStorageCodes(t *testing.T) {
	p := &Presentation{Slides: []*Slide{{Shapes: []*Shape{{Paragraphs: []*Paragraph{
		{BulletChar: "w", BulletFontFamily: "Wingdings", Runs: []*Run{{Text: "Level 0", FontFamily: "Palatino"}}},
		{BulletChar: "n", BulletFontFamily: "Wingdings", Runs: []*Run{{Text: "Level 1", FontFamily: "Palatino"}}},
	}}}}}}
	NormalizeLegacyListMarkerGlyphs(p)
	got0 := p.Slides[0].Shapes[0].Paragraphs[0]
	got1 := p.Slides[0].Shapes[0].Paragraphs[1]
	if got0.BulletChar != "◆" || got1.BulletChar != "▪" {
		t.Fatalf("canonical markers = %q/%q, want diamond/square", got0.BulletChar, got1.BulletChar)
	}
	if got0.BulletFontFamily != "" || !got0.BulletFontTx || got1.BulletFontFamily != "" || !got1.BulletFontTx {
		t.Fatalf("legacy symbol font ownership leaked after Unicode normalization: %#v %#v", got0, got1)
	}
}

func TestNormalizeLegacyListMarkerGlyphsPreservesOrdinaryUnicodeMarker(t *testing.T) {
	para := &Paragraph{BulletChar: "•", BulletFontFamily: "Aptos"}
	p := &Presentation{Slides: []*Slide{{Shapes: []*Shape{{Paragraphs: []*Paragraph{para}}}}}}
	NormalizeLegacyListMarkerGlyphs(p)
	if para.BulletChar != "•" || para.BulletFontFamily != "Aptos" || para.BulletFontTx {
		t.Fatalf("ordinary Unicode marker changed: %#v", para)
	}
}
