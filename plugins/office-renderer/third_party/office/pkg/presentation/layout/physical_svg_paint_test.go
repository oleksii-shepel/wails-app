package layout

import (
	"math"
	"testing"

	"office/pkg/ooxml/drawing"
	"office/pkg/presentation/ir"
)

func TestPhysicalSVGPaintOpensViewportForOuterEffects(t *testing.T) {
	shape := &ir.Shape{Kind: ir.ShapeText, Bounds: ir.Rect{W: 1270000, H: 635000}, Effects: drawing.EffectList{Effects: []drawing.Effect{drawing.Glow{Radius: 10}}}}
	got := MaterializePhysicalVisualTree([]*ir.Shape{shape})
	if len(got) != 1 {
		t.Fatalf("physical visuals = %d, want 1", len(got))
	}
	svg := got[0].Drawing.Paint.SVG
	if !svg.OverflowVisible {
		t.Fatal("outer effect must open the SVG viewport")
	}
	if math.Abs(svg.FilterXPercent+10) > 0.001 || math.Abs(svg.FilterYPercent+20) > 0.001 || math.Abs(svg.FilterWPercent-120) > 0.001 || math.Abs(svg.FilterHPercent-140) > 0.001 {
		t.Fatalf("svg paint = %+v, want -10%% -20%% 120%% 140%%", svg)
	}
}

func TestPhysicalSVGPaintKeepsOrdinaryViewportCanonical(t *testing.T) {
	shape := &ir.Shape{Kind: ir.ShapeText, Bounds: ir.Rect{W: 1270000, H: 635000}}
	got := MaterializePhysicalVisualTree([]*ir.Shape{shape})
	svg := got[0].Drawing.Paint.SVG
	if svg.OverflowVisible || svg.FilterXPercent != 0 || svg.FilterYPercent != 0 || svg.FilterWPercent != 100 || svg.FilterHPercent != 100 {
		t.Fatalf("ordinary svg paint = %+v", svg)
	}
}

func TestPhysicalSVGPaintOpensViewportForStrokeOverhang(t *testing.T) {
	shape := &ir.Shape{
		Kind:        ir.ShapeText,
		Geometry:    "rect",
		Bounds:      ir.Rect{W: 1270000, H: 635000},
		LineColor:   "000000",
		LineWidthPt: 3,
		LineStyle:   &drawing.LineStyle{Width: 3, Color: "000000", Cap: drawing.LineCapRound, Join: drawing.LineJoinRound},
	}
	got := MaterializePhysicalVisualTree([]*ir.Shape{shape})
	if len(got) != 1 {
		t.Fatalf("physical visuals = %d, want 1", len(got))
	}
	if !got[0].Drawing.Paint.OverflowVisible {
		t.Fatalf("stroke overhang must open outer paint overflow: %+v", got[0].Drawing.Paint)
	}
	svg := got[0].Drawing.Paint.SVG
	if !svg.OverflowVisible {
		t.Fatalf("stroke overhang must open svg viewport: %+v", svg)
	}
	if svg.FilterXPercent != 0 || svg.FilterYPercent != 0 || svg.FilterWPercent != 100 || svg.FilterHPercent != 100 {
		t.Fatalf("stroke-only svg paint should not synthesize filter expansion: %+v", svg)
	}
}
