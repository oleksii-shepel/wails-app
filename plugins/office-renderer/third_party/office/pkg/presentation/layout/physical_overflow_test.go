package layout

import (
	"office/pkg/ooxml/drawing"
	"office/pkg/presentation/ir"
	"testing"
)

func TestPhysicalOverflowMaterializesOuterEffects(t *testing.T) {
	shape := &ir.Shape{Bounds: ir.Rect{W: 12700 * 100, H: 12700 * 50}}
	effects := EffectVisualPlan{
		HasReflection: true,
		Shadows:       []ShadowVisualPlan{{BlurPt: 4, OffsetXPt: 10, OffsetYPt: -3}},
		Glows:         []GlowVisualPlan{{RadiusPt: 6}},
		Blurs:         []BlurVisualPlan{{RadiusPt: 5, Grow: true}},
	}
	got := resolvePhysicalOverflowPlan(shape, effects)
	if got.LeftPt != 6 || got.TopPt != 8 || got.RightPt != 18 || got.BottomPt != 50 {
		t.Fatalf("overflow = %+v, want left=6 top=8 right=18 bottom=50", got)
	}
}

func TestPhysicalOverflowIgnoresInnerAndNonGrowingEffects(t *testing.T) {
	effects := EffectVisualPlan{
		Shadows:   []ShadowVisualPlan{{Inner: true, BlurPt: 20, OffsetXPt: 30}},
		Blurs:     []BlurVisualPlan{{RadiusPt: 15, Grow: false}},
		SoftEdges: []SoftEdgeVisualPlan{{RadiusPt: 12}},
	}
	if got := resolvePhysicalOverflowPlan(&ir.Shape{}, effects); !got.Empty() {
		t.Fatalf("non-expanding effects produced overflow: %+v", got)
	}
}

func TestPhysicalVisualCarriesEffectOverflow(t *testing.T) {
	shape := &ir.Shape{Kind: ir.ShapeText, Bounds: ir.Rect{W: 12700 * 20, H: 12700 * 10}, Effects: drawing.EffectList{Effects: []drawing.Effect{drawing.Glow{Radius: 7}}}}
	items := MaterializePhysicalVisualTree([]*ir.Shape{shape})
	if len(items) != 1 {
		t.Fatalf("physical visuals = %d", len(items))
	}
	if items[0].Drawing.Paint.Overflow.LeftPt != 7 || items[0].Drawing.Paint.Overflow.RightPt != 7 || !items[0].Drawing.Paint.OverflowVisible {
		t.Fatalf("physical overflow not materialized: %+v", items[0])
	}
}
