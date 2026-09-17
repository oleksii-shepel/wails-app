package layout

import (
	"math"
	"testing"

	"office/pkg/ooxml/drawing"
	"office/pkg/presentation/ir"
)

func TestPhysicalPaintPipelineClosesPictureEffectsCropAndAncestorClip(t *testing.T) {
	group := &ir.Shape{
		Kind: ir.ShapeGroup, Geometry: "ellipse",
		GroupTransform: &ir.VisualTransform{
			Offset: ir.Point{X: 12700, Y: 12700}, Extent: ir.VisualSize{W: 1270000, H: 1270000},
			ChildOffset: ir.Point{}, ChildExtent: ir.VisualSize{W: 1270000, H: 1270000},
		},
		Children: []*ir.Shape{{
			Kind: ir.ShapeImage, Geometry: "ellipse", Bounds: ir.Rect{X: -127000, Y: -127000, W: 1524000, H: 1524000},
			RotationDeg: 30, ImageCropLeft: .1, ImageCropRight: .2,
			ImageRotWithShapeSet: true, ImageRotWithShape: false,
			Effects: drawing.EffectList{Effects: []drawing.Effect{
				drawing.Shadow{Color: "000000", Alpha: .4, Blur: 6, Distance: 8, Angle: 0},
				drawing.Glow{Color: "00A0FF", Alpha: .5, Radius: 4},
			}},
		}},
	}
	got := MaterializePhysicalVisualTree([]*ir.Shape{group})
	if len(got) != 1 {
		t.Fatalf("physical visuals = %d, want 1", len(got))
	}
	item := got[0]
	if item.Image == nil {
		t.Fatal("picture physical plan missing")
	}
	if item.Image.ClipKind != "ellipse" || item.Image.CounterRotationDeg != -30 {
		t.Fatalf("image plan = %+v", item.Image)
	}
	if len(item.Drawing.Paint.AncestorClips) != 1 || !item.Drawing.Paint.PaintClipped {
		t.Fatalf("paint clip composition = %+v", item.Drawing.Paint)
	}
	if !item.Drawing.Visual.Effects.HasShadow || !item.Drawing.Visual.Effects.HasGlow || !item.Drawing.Paint.OverflowVisible {
		t.Fatalf("effect paint = %+v", item.Drawing.Paint)
	}
	if item.Drawing.Paint.Bounds.WidthPt <= item.Drawing.Visual.WidthPt {
		t.Fatalf("effect bounds did not expand: %+v", item.Drawing.Paint.Bounds)
	}
}

func TestEffectVisualPlanOwnsRendererNeutralReflectionAndThreeD(t *testing.T) {
	plan := ResolveEffectVisualPlan(drawing.EffectList{Effects: []drawing.Effect{
		drawing.Reflection{Direction: 90, Distance: 7, Alpha: .3, StartAlpha: 50000, EndAlpha: 10000, StartPosition: 10000, EndPosition: 90000},
		drawing.ThreeD{CameraFOV: 45, CameraZoom: 1.2, RotationX: 10, RotationY: 20, RotationZ: 30, Material: "matte"},
	}})
	if plan.Composition.Reflection == nil || plan.Composition.ThreeD == nil {
		t.Fatalf("composition = %+v", plan.Composition)
	}
	r := plan.Composition.Reflection
	if r.DirectionDeg != 90 || r.DistancePt != 7 || math.Abs(r.StartAlpha-.5) > 1e-9 || r.StartPosition != 10 {
		t.Fatalf("reflection = %+v", r)
	}
	d := plan.Composition.ThreeD
	if d.RotationXDeg != 10 || d.RotationYDeg != 20 || d.RotationZDeg != 30 || d.Material != "matte" {
		t.Fatalf("3d = %+v", d)
	}
}

func TestThreeDVisualPlanOwnsVectorExtrusionAndLighting(t *testing.T) {
	plan := ResolveEffectCompositionPlan(drawing.EffectList{Effects: []drawing.Effect{
		drawing.ThreeD{
			CameraPreset: "perspectiveTopLeft", RotationX: -15, RotationY: 10,
			ExtrusionDepth: 6, ExtrusionColor: "595959", ContourWidth: 1,
			ContourColor: "FFFFFF", Material: "metal", LightRig: "threePt", LightDirection: "tl",
		},
	}})
	if plan.ThreeD == nil {
		t.Fatal("3-D physical plan = nil")
	}
	threeD := plan.ThreeD
	if threeD.ExtrusionDepthPt != 6 || threeD.ExtrusionColor != "595959" || threeD.ContourWidthPt != 1 {
		t.Fatalf("3-D extrusion/contour = %#v", threeD)
	}
	if len(threeD.LightingOverlays) != 2 {
		t.Fatalf("3-D lighting overlays = %d, want 2", len(threeD.LightingOverlays))
	}
	if threeD.LightingOverlays[0].BlendMode != "screen" || threeD.LightingOverlays[1].BlendMode != "mult" {
		t.Fatalf("3-D lighting blend modes = %#v", threeD.LightingOverlays)
	}
}


func TestClass14EffectFamiliesConvergeOnOnePhysicalVisualPlan(t *testing.T) {
	effects := drawing.EffectList{Effects: []drawing.Effect{
		drawing.Shadow{Color: "112233", Alpha: .4, Blur: 3, Distance: 5, Angle: 30},
		drawing.InnerShadow{Color: "223344", Alpha: .5, Blur: 2, Distance: 2, Angle: 90},
		drawing.Glow{Color: "334455", Alpha: .6, Radius: 4},
		drawing.SoftEdge{Radius: 1.5},
		drawing.Blur{Radius: 2, Grow: true},
		drawing.Reflection{Distance: 6, Blur: 2, ScaleY: .5},
	}}
	plan := ResolveEffectVisualPlan(effects)
	if !plan.HasShadow || !plan.HasInnerShadow || !plan.HasGlow ||
		!plan.HasSoftEdge || !plan.HasBlur || !plan.HasReflection {
		t.Fatalf("effect family ownership incomplete: %+v", plan)
	}
	if len(plan.Shadows) != 2 || len(plan.Glows) != 1 ||
		len(plan.SoftEdges) != 1 || len(plan.Blurs) != 1 {
		t.Fatalf("physical effect payloads incomplete: %+v", plan)
	}
	if plan.Composition.Reflection == nil {
		t.Fatal("reflection did not enter physical composition")
	}
	if !plan.NeedsOverflow {
		t.Fatal("outer effects must request physical paint overflow")
	}
}

func TestClass14InnerShadowAndSoftEdgeDoNotExpandOuterPaintBounds(t *testing.T) {
	shape := &ir.Shape{
		Kind: ir.ShapeText,
		Bounds: ir.Rect{W: 1270000, H: 635000},
		Effects: drawing.EffectList{Effects: []drawing.Effect{
			drawing.InnerShadow{Blur: 8, Distance: 6},
			drawing.SoftEdge{Radius: 12},
		}},
	}
	visual := ResolveShapeVisualPlan(shape)
	if visual.Effects.NeedsOverflow {
		t.Fatalf("inward-only effects requested outer overflow: %+v", visual.Effects)
	}
}
