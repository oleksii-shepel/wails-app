package layout

import (
	"math"
	"testing"

	"office/pkg/ooxml/drawing"
	"office/pkg/presentation/ir"
)

func TestResolveImageVisualPlanOwnsCropFillAndCounterRotation(t *testing.T) {
	shape := &ir.Shape{
		ImageCropLeft: .10, ImageCropTop: .20, ImageCropRight: .15, ImageCropBottom: .05,
		ImageFillLeft: .05, ImageFillTop: .10, ImageFillRight: .20, ImageFillBottom: .15,
		RotationDeg: 30, FlipH: true,
		ImageRotWithShapeSet: true, ImageRotWithShape: false,
	}
	got := ResolveImageVisualPlan(shape)
	close := func(name string, got, want float64) {
		t.Helper()
		if math.Abs(got-want) > .0001 {
			t.Fatalf("%s = %v, want %v", name, got, want)
		}
	}
	close("frame left", got.FrameLeft, 5)
	close("frame top", got.FrameTop, 10)
	close("frame right", got.FrameRight, 20)
	close("frame bottom", got.FrameBottom, 15)
	close("image width", got.ImageWidth, 100.0/.75)
	close("image height", got.ImageHeight, 100.0/.75)
	close("image left", got.ImageLeft, -100.0*.10/.75)
	close("image top", got.ImageTop, -100.0*.20/.75)
	close("counter rotation", got.CounterRotationDeg, 30)
	if !got.Composition.Transform.FlipH || got.Composition.Transform.FlipV {
		t.Fatalf("transform flips = %+v", got.Composition.Transform)
	}
}

func TestResolveImageVisualPlanRejectsEmptyVisibleSourceOrDestination(t *testing.T) {
	if !ResolveImageVisualPlan(&ir.Shape{ImageCropLeft: .6, ImageCropRight: .4}).ImageHidden {
		t.Fatal("empty source crop must be hidden")
	}
	if !ResolveImageVisualPlan(&ir.Shape{ImageFillTop: .7, ImageFillBottom: .3}).FrameHidden {
		t.Fatal("empty destination fill rect must be hidden")
	}
}

func TestResolveImageVisualPlanPreservesExpandedSignedSourceRect(t *testing.T) {
	shape := &ir.Shape{
		ImageCropLeft: -.10, ImageCropTop: -.20, ImageCropRight: -.15, ImageCropBottom: -.05,
	}
	got := ResolveImageVisualPlan(shape)
	close := func(name string, got, want float64) {
		t.Helper()
		if math.Abs(got-want) > .0001 {
			t.Fatalf("%s = %v, want %v", name, got, want)
		}
	}
	// A negative relative rectangle expands the source selection. The image
	// therefore occupies less than 100% of the destination frame; clamping the
	// visible extent back to 1 would incorrectly turn this into an uncropped image.
	close("image width", got.ImageWidth, 100.0/1.25)
	close("image height", got.ImageHeight, 100.0/1.25)
	close("image left", got.ImageLeft, 100.0*.10/1.25)
	close("image top", got.ImageTop, 100.0*.20/1.25)
	if got.SourceRect.Left != -.10 || got.SourceRect.Top != -.20 || got.SourceRect.Right != -.15 || got.SourceRect.Bottom != -.05 {
		t.Fatalf("source rect = %+v", got.SourceRect)
	}
}

func TestResolveImageVisualPlanOwnsPictureMetadata(t *testing.T) {
	shape := &ir.Shape{ImageDPI: 220, ImageRotWithShapeSet: true, ImageRotWithShape: false}
	got := ResolveImageVisualPlan(shape)
	if got.DPI != 220 || got.RotWithShape {
		t.Fatalf("picture metadata = dpi %d rotWithShape %v", got.DPI, got.RotWithShape)
	}
}

func TestResolveImageVisualPlanCounterRotationRespectsReflectedHandedness(t *testing.T) {
	oneFlip := ResolveImageVisualPlan(&ir.Shape{RotationDeg: 30, FlipH: true, ImageRotWithShapeSet: true, ImageRotWithShape: false})
	if oneFlip.CounterRotationDeg != 30 {
		t.Fatalf("single-reflection counter rotation = %v, want 30", oneFlip.CounterRotationDeg)
	}
	twoFlips := ResolveImageVisualPlan(&ir.Shape{RotationDeg: 30, FlipH: true, FlipV: true, ImageRotWithShapeSet: true, ImageRotWithShape: false})
	if twoFlips.CounterRotationDeg != -30 {
		t.Fatalf("double-reflection counter rotation = %v, want -30", twoFlips.CounterRotationDeg)
	}
}

func TestResolveImageVisualPlanCounterRotationUsesAuthoredPictureCenter(t *testing.T) {
	shape := &ir.Shape{
		RotationDeg: 30, ImageRotWithShapeSet: true, ImageRotWithShape: false,
		ImageCropLeft: .10, ImageCropTop: .20, ImageCropRight: .25, ImageCropBottom: .05,
		ImageFillLeft: .05, ImageFillTop: .10, ImageFillRight: .20, ImageFillBottom: .15,
	}
	got := ResolveImageVisualPlan(shape)
	if math.Abs(got.CounterRotationOriginX-49.0) > .0001 || math.Abs(got.CounterRotationOriginY-60.0) > .0001 {
		t.Fatalf("counter-rotation origin = %.4f%% %.4f%%, want 49%% 60%%", got.CounterRotationOriginX, got.CounterRotationOriginY)
	}
}

func TestResolveImageVisualPlanOwnsShapeEffectPaintOrder(t *testing.T) {
	shape := &ir.Shape{
		Geometry: "ellipse",
		Effects: drawing.EffectList{Effects: []drawing.Effect{
			drawing.Shadow{Color: "112233", Alpha: .5, Blur: 3, Distance: 4},
			drawing.InnerShadow{Color: "445566", Alpha: .75, Blur: 2, Distance: 1},
			drawing.Glow{Color: "778899", Alpha: .4, Radius: 5},
		}},
	}
	got := ResolveImageVisualPlan(shape)
	if got.ClipPath == "" {
		t.Fatal("ellipse picture must own geometry mask")
	}
	if !got.Composition.Effects.HasShadow || !got.Composition.Effects.HasInnerShadow || !got.Composition.Effects.HasGlow {
		t.Fatalf("picture shape effects = %+v", got.Composition.Effects)
	}
}


func TestClass15ImageCropAndFillRectAreMaterializedTogether(t *testing.T) {
	shape := &ir.Shape{
		Kind: ir.ShapeImage, Bounds: ir.Rect{W: 2540000, H: 1270000},
		ImageCropLeft: .1, ImageCropTop: .2, ImageCropRight: .1, ImageCropBottom: .2,
		ImageFillLeft: .05, ImageFillTop: .1, ImageFillRight: .05, ImageFillBottom: .1,
	}
	plan := ResolveImageVisualPlan(shape)
	if plan.ImageHidden || plan.FrameHidden {
		t.Fatalf("valid crop/fill unexpectedly hidden: %+v", plan)
	}
	if plan.ImageWidth <= 100 || plan.ImageHeight <= 100 || plan.ImageLeft >= 0 || plan.ImageTop >= 0 {
		t.Fatalf("crop not materialized into physical image geometry: %+v", plan)
	}
	if plan.FrameLeft != 5 || plan.FrameTop != 10 || plan.FrameRight != 5 || plan.FrameBottom != 10 {
		t.Fatalf("fillRect frame lost: %+v", plan)
	}
}

func TestClass15RotWithShapeFalseCounterRotatesPixelsNotFrame(t *testing.T) {
	shape := &ir.Shape{
		Kind: ir.ShapeImage, Bounds: ir.Rect{W: 1270000, H: 635000},
		RotationDeg: 30, ImageRotWithShapeSet: true, ImageRotWithShape: false,
	}
	plan := ResolveImageVisualPlan(shape)
	if plan.CounterRotationDeg == 0 {
		t.Fatalf("rotWithShape=false did not materialize pixel counter-rotation: %+v", plan)
	}
	if plan.Composition.Transform.RotationDeg != 30 {
		t.Fatalf("picture frame rotation changed: %+v", plan.Composition.Transform)
	}
}
