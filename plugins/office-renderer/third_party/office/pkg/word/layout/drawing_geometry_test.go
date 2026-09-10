package layout

import (
	"math"
	"testing"

	wordir "office/pkg/word/ir"
)

func TestFloatBoxDrawingUsesCanonicalPayload(t *testing.T) {
	image := &wordir.Image{Floating: true}
	textbox := &wordir.TextBox{Floating: true}
	shape := &wordir.Shape{Floating: true}

	cases := []struct {
		name string
		fl   FloatBox
		kind string
	}{
		{"image", FloatBox{Image: image}, "image"},
		{"textbox", FloatBox{TextBox: textbox}, "textbox"},
		{"shape", FloatBox{Shape: shape}, "shape"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			drawing := tc.fl.Drawing()
			if drawing == nil || drawing.DrawingKind() != tc.kind {
				t.Fatalf("Drawing = %#v, want %s", drawing, tc.kind)
			}
		})
	}
	if (FloatBox{}).Drawing() != nil {
		t.Fatal("empty float unexpectedly returned drawing")
	}
}

func TestDrawingExclusionIsPayloadIndependent(t *testing.T) {
	const emu = int64(12700)
	polygon := [][2]float64{{0, 0}, {40, 0}, {40, 20}, {0, 20}}
	makeFloat := func(kind string) FloatBox {
		fl := FloatBox{X: 100, Y: 200, Width: 40, Height: 20}
		switch kind {
		case "image":
			fl.Image = &wordir.Image{Floating: true, Width: 40 * emu, Height: 20 * emu, WrapType: "tight", WrapPolygon: polygon, DistLeft: 3, DistRight: 5, RotationDeg: 15}
		case "textbox":
			fl.TextBox = &wordir.TextBox{Floating: true, Width: 40 * emu, Height: 20 * emu, WrapType: "tight", WrapPolygon: polygon, DistLeft: 3, DistRight: 5, RotationDeg: 15}
		case "shape":
			fl.Shape = &wordir.Shape{Floating: true, Width: 40 * emu, Height: 20 * emu, WrapType: "tight", WrapPolygon: polygon, DistLeft: 3, DistRight: 5, RotationDeg: 15}
		}
		return fl
	}

	want, ok := drawingExclusionForFloat(makeFloat("image"))
	if !ok {
		t.Fatal("image exclusion missing")
	}
	for _, kind := range []string{"textbox", "shape"} {
		got, ok := drawingExclusionForFloat(makeFloat(kind))
		if !ok {
			t.Fatalf("%s exclusion missing", kind)
		}
		if math.Abs(got.left-want.left) > 1e-9 || math.Abs(got.right-want.right) > 1e-9 ||
			math.Abs(got.top-want.top) > 1e-9 || math.Abs(got.bottom-want.bottom) > 1e-9 || len(got.polygon) != len(want.polygon) {
			t.Fatalf("%s exclusion = %+v, want %+v", kind, got, want)
		}
	}
}

func TestDrawingExclusionHonorsBehindDoc(t *testing.T) {
	fl := FloatBox{
		Image: &wordir.Image{Floating: true, Width: 12700, Height: 12700, WrapType: "square", BehindDoc: true},
		X:     10, Y: 20, Width: 1, Height: 1,
	}
	if _, ok := drawingExclusionForFloat(fl); ok {
		t.Fatal("behind-text drawing produced line exclusion")
	}
}

func TestDrawingTopAndBottomUsesResolvedHeight(t *testing.T) {
	const emu = int64(12700)
	fl := FloatBox{
		Shape: &wordir.Shape{
			Floating: true, Height: 10 * emu, WrapType: "topAndBottom",
			DistTop: 2, DistBottom: 3, EffectExtentT: emu, EffectExtentB: 2 * emu,
		},
		Height: 25,
	}
	if !drawingIsTopAndBottom(fl) {
		t.Fatal("top-and-bottom drawing not recognized")
	}
	if got, want := drawingOuterHeightPt(fl), 33.0; math.Abs(got-want) > 1e-9 {
		t.Fatalf("outer height = %v, want %v", got, want)
	}
}
