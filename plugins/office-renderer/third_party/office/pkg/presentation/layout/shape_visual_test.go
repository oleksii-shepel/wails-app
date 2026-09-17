package layout

import (
	"reflect"
	"testing"

	"office/pkg/ooxml/drawing"

	"office/pkg/presentation/ir"
)

func TestResolveShapeVisualPlanOwnsPresetGeometryAndPhysicalBounds(t *testing.T) {
	shape := &ir.Shape{Bounds: ir.Rect{W: 914400, H: 457200}, Geometry: "ellipse", GeometryAdjustments: []float64{25000}}
	plan := ResolveShapeVisualPlan(shape)
	near(t, "shape width", plan.WidthPt, 72)
	near(t, "shape height", plan.HeightPt, 36)
	if !plan.UseVector || plan.Geometry.Preset != "ellipse" || len(plan.Primitives) == 0 {
		t.Fatalf("invalid visual plan: %+v", plan)
	}
}

func TestResolveShapeVisualPlanKeepsRectOnCSSPath(t *testing.T) {
	shape := &ir.Shape{Bounds: ir.Rect{W: 914400, H: 457200}, Geometry: "rect"}
	plan := ResolveShapeVisualPlan(shape)
	if plan.UseVector {
		t.Fatalf("ordinary rect unexpectedly requires vector materialization")
	}
	if plan.Geometry.Preset != "rect" {
		t.Fatalf("preset = %q, want rect", plan.Geometry.Preset)
	}
}

func TestResolveShapeVisualPlanUsesVectorPathForNonCSSFill(t *testing.T) {
	shape := &ir.Shape{
		Bounds:   ir.Rect{W: 914400, H: 457200},
		Geometry: "rect",
		Fill:     drawing.GradientFill{},
	}
	plan := ResolveShapeVisualPlan(shape)
	if !plan.UseVector {
		t.Fatal("gradient-filled rect must use vector materialization")
	}
}

func TestResolveShapeVisualPlanOwnsClipAndEffectClassification(t *testing.T) {
	shape := &ir.Shape{
		Bounds:   ir.Rect{W: 914400, H: 457200},
		Geometry: "ellipse",
		Effects:  drawing.EffectList{Effects: []drawing.Effect{drawing.ThreeD{}}},
	}
	plan := ResolveShapeVisualPlan(shape)
	if !plan.ClipContent {
		t.Fatal("ellipse content must be clipped to canonical geometry")
	}
	if !plan.HasEffects || !plan.HasThreeD {
		t.Fatalf("effect classification = %+v, want effects and 3-D", plan)
	}
}

func TestResolveShapeVisualPlanClassifiesSVGFillDefinition(t *testing.T) {
	shape := &ir.Shape{Bounds: ir.Rect{W: 914400, H: 457200}, Fill: drawing.PatternFill{}}
	plan := ResolveShapeVisualPlan(shape)
	if !plan.FillNeedsDefinition || !plan.UseVector {
		t.Fatalf("pattern fill classification = %+v, want SVG definition and vector path", plan)
	}
}

func TestResolveStrokeVisualPlanOwnsDashCapJoinAndCompound(t *testing.T) {
	style := &drawing.LineStyle{
		Width: 2.5, Color: "336699", Dash: drawing.DashPatternDashDot,
		Cap: drawing.LineCapRound, Join: drawing.LineJoinBevel,
		Compound: true, CompoundType: "dbl",
	}
	plan := ResolveStrokeVisualPlan(style, "000000", 1)
	if plan.Color != "336699" || plan.WidthPt != 2.5 {
		t.Fatalf("stroke paint = (%q, %.2f), want (336699, 2.5)", plan.Color, plan.WidthPt)
	}
	if plan.CSSStyle != "double" || plan.SVGLineCap != "round" || plan.SVGLineJoin != "bevel" {
		t.Fatalf("stroke materialization = %+v", plan)
	}
	if plan.DashArray != "7.5, 7.5, 2.5, 7.5" {
		t.Fatalf("dash array = %q", plan.DashArray)
	}
}

func TestResolveShapeVisualPlanPromotesArrowStrokeToVector(t *testing.T) {
	shape := &ir.Shape{Geometry: "rect", LineStyle: &drawing.LineStyle{Width: 1, Color: "000000", TailEnd: &drawing.Arrowhead{}}}
	plan := ResolveShapeVisualPlan(shape)
	if !plan.Stroke.HasArrowheads || !plan.Stroke.NeedsVector || !plan.UseVector {
		t.Fatalf("arrow stroke must use vector materialization: %+v", plan.Stroke)
	}
}

func TestResolveShapeVisualPlanClassifiesOverflowEffects(t *testing.T) {
	shape := &ir.Shape{Effects: drawing.EffectList{Effects: []drawing.Effect{
		drawing.Glow{Radius: 4}, drawing.Reflection{Distance: 2}, drawing.SoftEdge{Radius: 1},
	}}}
	plan := ResolveShapeVisualPlan(shape)
	if !plan.Effects.HasGlow || !plan.Effects.HasReflection || !plan.Effects.HasSoftEdge || !plan.Effects.NeedsOverflow {
		t.Fatalf("effect plan = %+v", plan.Effects)
	}
}

func TestResolveEffectCompositionPlanPreservesAuthoredTransformOrder(t *testing.T) {
	plan := ResolveEffectCompositionPlan(drawing.EffectList{Effects: []drawing.Effect{
		drawing.RelativeOffset{X: 0.1, Y: -0.2},
		drawing.TransformEffect{ScaleX: 1.25, ScaleY: 0.75, SkewX: 5},
	}})
	if len(plan.Transforms) != 2 {
		t.Fatalf("transform steps = %d, want 2", len(plan.Transforms))
	}
	if plan.Transforms[0].Kind != "offset" || plan.Transforms[1].Kind != "transform" {
		t.Fatalf("transform order = %#v", plan.Transforms)
	}
	near(t, "offset x", plan.Transforms[0].X, 0.1)
	near(t, "scale x", plan.Transforms[1].ScaleX, 1.25)
	near(t, "scale y", plan.Transforms[1].ScaleY, 0.75)
}

func TestResolveEffectCompositionPlanOwnsBlendReflectionAndThreeD(t *testing.T) {
	plan := ResolveEffectCompositionPlan(drawing.EffectList{Effects: []drawing.Effect{
		drawing.BlendMode{Mode: "mult"},
		drawing.Reflection{Distance: 4, Direction: 90},
		drawing.ThreeD{CameraPreset: "perspectiveFront", RotationY: 20},
	}})
	if plan.BlendMode != "mult" {
		t.Fatalf("blend mode = %q, want mult", plan.BlendMode)
	}
	if plan.Reflection == nil || plan.Reflection.DistancePt != 4 {
		t.Fatalf("reflection = %#v", plan.Reflection)
	}
	if plan.ThreeD == nil || plan.ThreeD.RotationYDeg != 20 {
		t.Fatalf("3-D = %#v", plan.ThreeD)
	}
}

func TestResolveFillVisualPlanOwnsVectorTransparencyAndTileSemantics(t *testing.T) {
	fill := drawing.PictureFill{Tile: true, Alpha: 0.5}
	plan := ResolveFillVisualPlan(fill, "ABCDEF", &ir.PictureTile{Sx: 50000, Sy: 75000})
	if plan.Kind != drawing.FillTypePicture || !plan.NeedsDefinition || !plan.HasTransparency {
		t.Fatalf("picture fill plan = %+v", plan)
	}
	if plan.PictureTile == nil || plan.PictureTile.Sx != 50000 || plan.PictureTile.Sy != 75000 {
		t.Fatalf("picture tile semantics lost: %+v", plan.PictureTile)
	}
}

func TestResolveShapeVisualPlanOwnsGeometryClipAndEffectOverlays(t *testing.T) {
	shape := &ir.Shape{
		Geometry: "ellipse",
		Bounds:   ir.Rect{W: 914400, H: 457200},
		Effects: drawing.EffectList{Effects: []drawing.Effect{
			drawing.FillOverlay{Fill: drawing.GradientFill{Stops: []drawing.GradientStop{{Position: 0, Color: "FF0000"}, {Position: 1, Color: "0000FF"}}}, BlendMode: "mult"},
		}},
	}
	plan := ResolveShapeVisualPlan(shape)
	if plan.Geometry.ClipKind != "ellipse" || !plan.ClipContent {
		t.Fatalf("ellipse clip ownership lost: %+v", plan.Geometry)
	}
	if len(plan.Effects.Overlays) != 1 || !plan.Effects.Overlays[0].NeedsDefinition || plan.Effects.Overlays[0].BlendMode != "mult" {
		t.Fatalf("effect overlay ownership lost: %+v", plan.Effects.Overlays)
	}
}

func TestResolveStrokeVisualPlanCarriesCanonicalStrokePaint(t *testing.T) {
	fill := drawing.GradientFill{Stops: []drawing.GradientStop{{Position: 0, Color: "111111"}, {Position: 1, Color: "EEEEEE"}}}
	style := &drawing.LineStyle{Width: 2.5, Color: "123456", Alpha: 0.4, Fill: fill, CompoundType: "tri"}
	plan := ResolveStrokeVisualPlan(style, "", 0)
	if plan.Fill == nil || plan.Alpha != 0.4 || !plan.NeedsVector || !plan.Compound {
		t.Fatalf("stroke paint ownership lost: %+v", plan)
	}
}

func TestResolveGeometryVisualPlanOwnsEllipseContentConstraints(t *testing.T) {
	shape := &ir.Shape{Geometry: "ellipse", GeometryAdjustments: []float64{12000}}
	plan := ResolveGeometryVisualPlan(shape)
	if plan.Preset != "ellipse" || plan.ClipKind != "ellipse" {
		t.Fatalf("ellipse geometry plan = %+v", plan)
	}
	near(t, "ellipse top inset ratio", plan.ContentInsetTop, 0.16)
	near(t, "ellipse right inset ratio", plan.ContentInsetRight, 0.18)
	near(t, "ellipse bottom inset ratio", plan.ContentInsetBottom, 0.16)
	near(t, "ellipse left inset ratio", plan.ContentInsetLeft, 0.18)
}

func TestResolveGeometryVisualPlanKeepsRectContentUnconstrained(t *testing.T) {
	plan := ResolveGeometryVisualPlan(&ir.Shape{Geometry: "rect"})
	if plan.ClipKind != "" || plan.ContentInsetTop != 0 || plan.ContentInsetRight != 0 || plan.ContentInsetBottom != 0 || plan.ContentInsetLeft != 0 {
		t.Fatalf("rect geometry unexpectedly constrains content: %+v", plan)
	}
}

func TestResolveConnectorVisualPlanOwnsPhysicalGeometryAndStroke(t *testing.T) {
	shape := &ir.Shape{
		Bounds:        ir.Rect{X: 914400, Y: 457200, W: 1828800, H: 914400},
		ConnectorType: "bentConnector3", ConnectorAdjustments: []int{25000},
		ConnectorFlipH: true, RotationDeg: 15, LineColor: "123456", LineWidthPt: 2.5,
		ArrowHead: "triangle", ArrowTail: "diamond",
	}
	plan := ResolveConnectorVisualPlan(shape)
	near(t, "connector x", plan.BoundsXPt, 72)
	near(t, "connector y", plan.BoundsYPt, 36)
	near(t, "connector width", plan.BoundsWPt, 144)
	near(t, "connector height", plan.BoundsHPt, 72)
	if plan.Composition.Transform.RotationDeg != 15 || plan.Type != "bentConnector3" || !plan.FlipH {
		t.Fatalf("connector geometry plan = %+v", plan)
	}
	if plan.Stroke.Color != "123456" || plan.Stroke.WidthPt != 2.5 {
		t.Fatalf("connector stroke = %+v", plan.Stroke)
	}
	if plan.ArrowHead != "triangle" || plan.ArrowTail != "diamond" {
		t.Fatalf("connector endpoints = head=%q tail=%q", plan.ArrowHead, plan.ArrowTail)
	}
}

func TestResolveConnectorVisualPlanCopiesAdjustments(t *testing.T) {
	shape := &ir.Shape{ConnectorAdjustments: []int{10000, 20000}}
	plan := ResolveConnectorVisualPlan(shape)
	shape.ConnectorAdjustments[0] = 99999
	if plan.Adjustments[0] != 10000 {
		t.Fatalf("connector plan aliases canonical adjustments: %v", plan.Adjustments)
	}
}

func TestResolveConnectorVisualPlanMaterializesLegacyUnnamedConnector(t *testing.T) {
	shape := &ir.Shape{
		Bounds:    ir.Rect{X: 914400, Y: 457200, W: 1828800, H: 914400},
		LineColor: "FFFF00", LineWidthPt: 1.5,
	}
	plan := ResolveConnectorVisualPlan(shape)
	if plan.Type != "straightConnector1" {
		t.Fatalf("unnamed legacy connector type = %q, want straightConnector1", plan.Type)
	}
	if len(plan.Primitives) == 0 {
		t.Fatal("unnamed legacy connector materialized an empty physical SVG plan")
	}
}

func TestResolveShapeVisualPlanOwnsVisibleMaterializationState(t *testing.T) {
	filled := ResolveShapeVisualPlan(&ir.Shape{Fill: drawing.SolidFill{Color: "ABCDEF"}})
	if !filled.HasVisibleFill || filled.HasVisibleStroke {
		t.Fatalf("filled shape visibility = %+v", filled)
	}

	noFill := ResolveShapeVisualPlan(&ir.Shape{Fill: drawing.NoFill{}})
	if noFill.HasVisibleFill {
		t.Fatalf("noFill unexpectedly visible: %+v", noFill.Fill)
	}

	stroked := ResolveShapeVisualPlan(&ir.Shape{LineColor: "123456", LineWidthPt: 1.5})
	if !stroked.HasVisibleStroke {
		t.Fatalf("stroke visibility lost: %+v", stroked.Stroke)
	}
}

func TestResolveStrokeVisualPlanScalesDashMetricsWithPhysicalWidth(t *testing.T) {
	thin := ResolveStrokeVisualPlan(&drawing.LineStyle{Width: 1, Dash: drawing.DashPatternDot}, "000000", 1)
	thick := ResolveStrokeVisualPlan(&drawing.LineStyle{Width: 3, Dash: drawing.DashPatternDot}, "000000", 1)
	if thin.DashArray != "1, 3" {
		t.Fatalf("thin dot dash = %q, want 1, 3", thin.DashArray)
	}
	if thick.DashArray != "3, 9" {
		t.Fatalf("thick dot dash = %q, want 3, 9", thick.DashArray)
	}
}

func TestResolveStrokeVisualPlanPreservesMiterAndFlatCap(t *testing.T) {
	plan := ResolveStrokeVisualPlan(&drawing.LineStyle{
		Width: 2, Cap: drawing.LineCapFlat, Join: drawing.LineJoinMiter, MiterLimit: 7,
	}, "112233", 1)
	if plan.SVGLineCap != "butt" || plan.SVGLineJoin != "miter" || plan.MiterLimit != 7 {
		t.Fatalf("physical cap/join = %+v", plan)
	}
}

func TestResolveShapeVisualPlanPromotesPreciseStrokeSemanticsToVector(t *testing.T) {
	cases := []struct {
		name  string
		style *drawing.LineStyle
	}{
		{"dash", &drawing.LineStyle{Width: 2, Color: "123456", Dash: drawing.DashPatternDashDot}},
		{"round-cap", &drawing.LineStyle{Width: 2, Color: "123456", Cap: drawing.LineCapRound}},
		{"alpha", &drawing.LineStyle{Width: 2, Color: "123456", Alpha: 0.35}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan := ResolveShapeVisualPlan(&ir.Shape{Geometry: "rect", Bounds: ir.Rect{W: 914400, H: 457200}, LineStyle: tc.style})
			if !plan.UseVector || !plan.Stroke.NeedsVector {
				t.Fatalf("precise stroke must use physical vector materialization: %+v", plan.Stroke)
			}
		})
	}
}

func TestClass6StrokePlanDistinguishesTransparentFromUnspecifiedAlpha(t *testing.T) {
	transparent := ResolveStrokeVisualPlan(&drawing.LineStyle{Width: 2, Color: "123456", Alpha: 0}, "", 0)
	if !transparent.AlphaSet || transparent.Alpha != 0 || !transparent.NeedsVector {
		t.Fatalf("transparent stroke plan = %+v", transparent)
	}
	fallback := ResolveStrokeVisualPlan(nil, "123456", 2)
	if fallback.AlphaSet {
		t.Fatalf("fallback stroke unexpectedly owns alpha: %+v", fallback)
	}
}

func TestClass6StrokePlanOwnsCompoundLineTopology(t *testing.T) {
	cases := []struct {
		kind string
		want []float64
	}{
		{"dbl", []float64{1.0 / 3.0, 1.0 / 3.0}},
		{"thickThin", []float64{2.0 / 3.0, 1.0 / 3.0}},
		{"thinThick", []float64{1.0 / 3.0, 2.0 / 3.0}},
		{"tri", []float64{1.0 / 3.0, 1.0 / 3.0, 1.0 / 3.0}},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			plan := ResolveStrokeVisualPlan(&drawing.LineStyle{Width: 3, CompoundType: tc.kind}, "000000", 1)
			if !reflect.DeepEqual(plan.CompoundWidths, tc.want) {
				t.Fatalf("compound widths = %#v, want %#v", plan.CompoundWidths, tc.want)
			}
		})
	}
}

func TestClass6StrokePlanPreservesAuthoredCompoundWidths(t *testing.T) {
	style := &drawing.LineStyle{Width: 4, CompoundType: "tri", CompoundWidths: []float64{0.2, 0.5, 0.3}}
	plan := ResolveStrokeVisualPlan(style, "000000", 1)
	if !reflect.DeepEqual(plan.CompoundWidths, []float64{0.2, 0.5, 0.3}) {
		t.Fatalf("compound widths = %#v", plan.CompoundWidths)
	}
	style.CompoundWidths[0] = 0.9
	if plan.CompoundWidths[0] != 0.2 {
		t.Fatalf("physical plan aliases canonical style: %#v", plan.CompoundWidths)
	}
}
