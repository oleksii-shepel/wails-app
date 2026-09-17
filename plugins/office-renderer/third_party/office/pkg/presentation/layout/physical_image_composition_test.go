package layout

import (
	"math"
	"testing"

	"office/pkg/presentation/ir"
)

func TestPhysicalImagePlanCarriesSourceCropAndDestinationFrame(t *testing.T) {
	shape := &ir.Shape{
		Kind:          ir.ShapeImage,
		Bounds:        ir.Rect{X: 12700, Y: 25400, W: 1270000, H: 635000},
		ImageCropLeft: .10, ImageCropTop: .20, ImageCropRight: .25, ImageCropBottom: .05,
		ImageFillLeft: .10, ImageFillTop: .20, ImageFillRight: .30, ImageFillBottom: .10,
	}
	got := ResolveImageVisualPlan(shape)
	if got.SourceRect.Left != .10 || got.SourceRect.Top != .20 || got.SourceRect.Right != .25 || got.SourceRect.Bottom != .05 {
		t.Fatalf("source crop = %+v", got.SourceRect)
	}
	// 100pt x 50pt authored bounds -> 60pt x 35pt fillRect frame at 11pt,12pt.
	if math.Abs(got.FrameBounds.LeftPt-11) > 1e-6 || math.Abs(got.FrameBounds.TopPt-12) > 1e-6 ||
		math.Abs(got.FrameBounds.WidthPt-60) > 1e-6 || math.Abs(got.FrameBounds.HeightPt-35) > 1e-6 {
		t.Fatalf("frame bounds = %+v, want left=11 top=12 width=60 height=35", got.FrameBounds)
	}
}

func TestPhysicalImageFrameBoundsIncludeFinalRotation(t *testing.T) {
	shape := &ir.Shape{Kind: ir.ShapeImage, Bounds: ir.Rect{W: 1270000, H: 635000}, RotationDeg: 90}
	got := ResolveImageVisualPlan(shape)
	if math.Abs(got.FrameBounds.WidthPt-50) > 1e-6 || math.Abs(got.FrameBounds.HeightPt-100) > 1e-6 {
		t.Fatalf("rotated frame bounds = %+v, want width=50 height=100", got.FrameBounds)
	}
}

func TestPhysicalImagePlanCombinesCropShapeClipAndCounterRotation(t *testing.T) {
	shape := &ir.Shape{
		Kind: ir.ShapeImage, Geometry: "ellipse", Bounds: ir.Rect{W: 1270000, H: 1270000},
		ImageCropLeft: .1, ImageCropRight: .2, RotationDeg: 30,
		ImageRotWithShapeSet: true, ImageRotWithShape: false,
	}
	got := ResolveImageVisualPlan(shape)
	if got.ClipKind != "ellipse" || got.ClipPath == "" {
		t.Fatalf("clip = %q %q", got.ClipKind, got.ClipPath)
	}
	if got.CounterRotationDeg != -30 {
		t.Fatalf("counter rotation = %v, want -30", got.CounterRotationDeg)
	}
	if math.Abs(got.ImageWidth-(100.0/.7)) > 1e-6 {
		t.Fatalf("cropped image width = %v", got.ImageWidth)
	}
}

func TestPhysicalImageFillRectRotatesAroundPictureCenter(t *testing.T) {
	shape := &ir.Shape{
		Kind:          ir.ShapeImage,
		Bounds:        ir.Rect{W: 1270000, H: 635000}, // 100pt x 50pt
		ImageFillLeft: .10, ImageFillTop: .20, ImageFillRight: .30, ImageFillBottom: .10,
		RotationDeg: 90,
	}
	got := ResolveImageVisualPlan(shape)
	// The 10,10..70,45pt fillRect rotates around the authored picture center
	// (50,25), producing 30,-15..65,45pt. Rotating around the fillRect's own
	// center would incorrectly produce 22.5,-2.5..57.5,57.5pt.
	if math.Abs(got.FrameBounds.LeftPt-30) > 1e-6 || math.Abs(got.FrameBounds.TopPt+15) > 1e-6 ||
		math.Abs(got.FrameBounds.WidthPt-35) > 1e-6 || math.Abs(got.FrameBounds.HeightPt-60) > 1e-6 {
		t.Fatalf("rotated asymmetric fillRect bounds = %+v, want left=30 top=-15 width=35 height=60", got.FrameBounds)
	}
}
