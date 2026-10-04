package stream

import (
	"testing"

	wordir "office/pkg/word/ir"
	wordparts "office/pkg/word/parsers/ooxml/parts"
	wordsax "office/pkg/word/parsers/ooxml/sax"
)

func TestPopulateAnchoredImagePreservesFullAnchorSemantics(t *testing.T) {
	anchor := &wordparts.DrawingAnchor{
		RelativeHeight: 42,
		BehindDoc:      1,
		DistT:          12700,
		DistB:          25400,
		DistL:          38100,
		DistR:          50800,
		Locked:         true,
		LayoutInCell:   true,
		AllowOverlap:   true,
		SimplePosAttr:  true,
		SimplePos:      &wordparts.SimplePos{X: 111000, Y: 222000},
		EffectExtent:   &wordparts.EffectExtent{L: 12700, T: 25400, R: 38100, B: 50800},
		SizeRelH:       &wordparts.SizeRel{RelativeFrom: "margin", Pct: 80000},
		SizeRelV:       &wordparts.SizeRel{RelativeFrom: "page", Pct: 120000},
		PositionH:      &wordparts.DrawingPosition{RelativeFrom: "page", PosOffset: 333000},
		PositionV:      &wordparts.DrawingPosition{RelativeFrom: "paragraph", PosOffset: 444000},
		WrapThrough: &wordparts.DrawingWrapSided{
			DrawingWrapDistances: wordparts.DrawingWrapDistances{DistT: 6350, DistB: 12700, DistL: 25400},
			WrapText:             "largest",
			Polygon: &wordparts.WrapPolygon{
				Start:  wordparts.WrapPolygonPt{X: 0, Y: 0},
				LineTo: []wordparts.WrapPolygonPt{{X: 1270000, Y: 0}, {X: 1270000, Y: 635000}},
			},
		},
	}
	raw := wordsax.Event{DrawingModel: &wordparts.Drawing{Anchor: anchor}}
	img := &wordir.Image{}

	populateAnchoredImage(img, raw)

	if !img.Floating || img.ZIndex != 42 || !img.BehindDoc || img.X != 333000 || img.Y != 444000 || img.XRelative != "page" || img.YRelative != "paragraph" {
		t.Fatalf("basic anchor = %#v", img)
	}
	if img.WrapType != "through" || img.WrapSide != "largest" {
		t.Fatalf("wrap mode = %q/%q", img.WrapType, img.WrapSide)
	}
	if img.DistTop != 0.5 || img.DistBottom != 1 || img.DistLeft != 2 || img.DistRight != 4 {
		t.Fatalf("wrap distances = %v/%v/%v/%v", img.DistTop, img.DistBottom, img.DistLeft, img.DistRight)
	}
	if len(img.WrapPolygon) != 3 || img.WrapPolygon[1] != [2]float64{100, 0} || img.WrapPolygon[2] != [2]float64{100, 50} {
		t.Fatalf("wrap polygon = %#v", img.WrapPolygon)
	}
	if !img.SimplePosAttr || img.SimplePosX != 111000 || img.SimplePosY != 222000 || !img.Locked || !img.LayoutInCell || !img.AllowOverlap {
		t.Fatalf("advanced flags = %#v", img)
	}
	if img.EffectExtentL != 12700 || img.EffectExtentT != 25400 || img.EffectExtentR != 38100 || img.EffectExtentB != 50800 {
		t.Fatalf("effect extent = %d/%d/%d/%d", img.EffectExtentL, img.EffectExtentT, img.EffectExtentR, img.EffectExtentB)
	}
	if img.SizeRelHPct != 80000 || img.SizeRelHFrom != "margin" || img.SizeRelVPct != 120000 || img.SizeRelVFrom != "page" {
		t.Fatalf("relative size = %d/%q %d/%q", img.SizeRelHPct, img.SizeRelHFrom, img.SizeRelVPct, img.SizeRelVFrom)
	}
}

func TestPopulateAnchoredImageScalesNormalizedWrapPolygon(t *testing.T) {
	anchor := &wordparts.DrawingAnchor{WrapTight: &wordparts.DrawingWrapSided{
		Polygon: &wordparts.WrapPolygon{
			Start:  wordparts.WrapPolygonPt{X: 0, Y: 0},
			LineTo: []wordparts.WrapPolygonPt{{X: 21600, Y: 0}, {X: 21600, Y: 21600}, {X: 0, Y: 21600}},
		},
	}}
	img := &wordir.Image{Width: 1828800, Height: 2743200}
	populateAnchoredImage(img, wordsax.Event{DrawingModel: &wordparts.Drawing{Anchor: anchor}})
	if len(img.WrapPolygon) != 4 {
		t.Fatalf("polygon points = %d, want 4", len(img.WrapPolygon))
	}
	if img.WrapPolygon[1] != [2]float64{144, 0} || img.WrapPolygon[2] != [2]float64{144, 216} {
		t.Fatalf("normalized wrap polygon = %#v, want 144pt by 216pt", img.WrapPolygon)
	}
}
