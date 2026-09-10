package layout

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestFloatBoxPlacementIsAuthoritativeAcrossDrawingKinds(t *testing.T) {
	cases := []struct {
		name string
		box  FloatBox
	}{
		{"image", FloatBox{Image: &wordir.Image{WrapType: "square"}}},
		{"textbox", FloatBox{TextBox: &wordir.TextBox{WrapType: "square"}}},
		{"shape", FloatBox{Shape: &wordir.Shape{WrapType: "square"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			placement, ok := tc.box.DrawingPlacement()
			if !ok {
				t.Fatal("FloatBox drawing placement missing")
			}
			if !placement.Floating {
				t.Fatal("resolved FloatBox was downgraded to inline placement")
			}
			if !placement.HasWrapExclusion() {
				t.Fatal("resolved square-wrapped FloatBox lost its exclusion")
			}
		})
	}
}

func TestResolvedDrawingFloatIsPayloadIndependent(t *testing.T) {
	const emu = int64(12700)
	geom := PageGeometry{
		Width: 600, Height: 800,
		MarginLeft: 72, MarginRight: 72,
		MarginTop: 72, MarginBottom: 72,
		ColumnCount: 1, ColumnWidthPt: 456,
	}
	placementImage := &wordir.Image{
		Floating: true, Width: 100 * emu, Height: 50 * emu,
		X: 10 * emu, Y: 20 * emu, XRelative: "page", YRelative: "page",
		ZIndex: 7, BehindDoc: true,
	}
	placementTextBox := &wordir.TextBox{
		Floating: true, Width: 100 * emu, Height: 50 * emu,
		X: 10 * emu, Y: 20 * emu, XRelative: "page", YRelative: "page",
		ZIndex: 7, BehindDoc: true,
	}
	placementShape := &wordir.Shape{
		Floating: true, Width: 100 * emu, Height: 50 * emu,
		X: 10 * emu, Y: 20 * emu, XRelative: "page", YRelative: "page",
		ZIndex: 7, BehindDoc: true,
	}

	var want FloatBox
	for i, drawing := range []wordir.Drawing{placementImage, placementTextBox, placementShape} {
		got, ok := resolveDrawingFloat(drawing, geom, 0, 100, 14)
		if !ok {
			t.Fatalf("drawing %d did not resolve", i)
		}
		if i == 0 {
			want = got
			continue
		}
		if got.X != want.X || got.Y != want.Y || got.Width != want.Width || got.Height != want.Height ||
			got.ZIndex != want.ZIndex || got.BehindDoc != want.BehindDoc {
			t.Fatalf("drawing %d geometry = %+v, want %+v", i, got, want)
		}
	}
}

func TestResolvedFloatBehindDocOverridesPayloadWrap(t *testing.T) {
	fl := FloatBox{
		Image: &wordir.Image{WrapType: "square"},
		X:     10, Y: 20, Width: 40, Height: 20,
		BehindDoc: true,
	}
	if _, ok := drawingExclusionForFloat(fl); ok {
		t.Fatal("resolved behind-text FloatBox produced a line exclusion")
	}
}
