package ir

import "testing"

func TestDrawingPlacementIsSharedAcrossPayloadKinds(t *testing.T) {
	polygon := [][2]float64{{1, 2}, {3, 4}}
	image := &Image{Floating: true, X: 10, Y: 20, Width: 30, Height: 40, WrapType: "tight", WrapPolygon: polygon, RotationDeg: 17, BehindDoc: true}
	textbox := &TextBox{Floating: true, X: 10, Y: 20, Width: 30, Height: 40, WrapType: "tight", WrapPolygon: polygon, RotationDeg: 17, BehindDoc: true}
	shape := &Shape{Floating: true, X: 10, Y: 20, Width: 30, Height: 40, WrapType: "tight", WrapPolygon: polygon, RotationDeg: 17, BehindDoc: true}

	for _, drawing := range []Drawing{image, textbox, shape} {
		placement := drawing.DrawingPlacement()
		if !placement.Floating || placement.X != 10 || placement.Y != 20 || placement.Width != 30 || placement.Height != 40 {
			t.Fatalf("%s placement = %+v", drawing.DrawingKind(), placement)
		}
		if placement.WrapType != "tight" || len(placement.WrapPolygon) != 2 || placement.RotationDeg != 17 || !placement.BehindDoc {
			t.Fatalf("%s drawing semantics = %+v", drawing.DrawingKind(), placement)
		}
	}
}

func TestDrawingPlacementSimplePositionIsCanonical(t *testing.T) {
	placement := (&Image{X: 10, Y: 20, SimplePosAttr: true, SimplePosX: 100, SimplePosY: 200}).DrawingPlacement()
	if got := placement.EffectiveX(); got != 100 {
		t.Fatalf("EffectiveX = %d, want 100", got)
	}
	if got := placement.EffectiveY(); got != 200 {
		t.Fatalf("EffectiveY = %d, want 200", got)
	}
}

func TestDrawingPlacementWrapExclusion(t *testing.T) {
	cases := []struct {
		name string
		p    DrawingPlacement
		want bool
	}{
		{"inline", DrawingPlacement{WrapType: "square"}, false},
		{"behind", DrawingPlacement{Floating: true, BehindDoc: true, WrapType: "square"}, false},
		{"none", DrawingPlacement{Floating: true, WrapType: "none"}, false},
		{"square", DrawingPlacement{Floating: true, WrapType: "square"}, true},
		{"tight", DrawingPlacement{Floating: true, WrapType: "tight"}, true},
		{"through", DrawingPlacement{Floating: true, WrapType: "through"}, true},
		{"topBottom", DrawingPlacement{Floating: true, WrapType: "topAndBottom"}, true},
		{"legacyDefault", DrawingPlacement{Floating: true}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.p.HasWrapExclusion(); got != tc.want {
				t.Fatalf("HasWrapExclusion = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDrawingFromRun(t *testing.T) {
	image := &Image{}
	textbox := &TextBox{}
	shape := &Shape{}
	cases := []struct {
		run  *Run
		kind string
	}{
		{&Run{Image: image}, "image"},
		{&Run{TextBox: textbox}, "textbox"},
		{&Run{Shape: shape}, "shape"},
	}
	for _, tc := range cases {
		drawing := DrawingFromRun(tc.run)
		if drawing == nil || drawing.DrawingKind() != tc.kind {
			t.Fatalf("DrawingFromRun = %#v, want %s", drawing, tc.kind)
		}
	}
	if DrawingFromRun(nil) != nil || DrawingFromRun(&Run{}) != nil {
		t.Fatal("empty run unexpectedly produced drawing")
	}
}

func TestDrawingPlacementDoesNotAliasWrapPolygon(t *testing.T) {
	image := &Image{WrapPolygon: [][2]float64{{1, 2}}}
	placement := image.DrawingPlacement()
	placement.WrapPolygon[0][0] = 99
	if image.WrapPolygon[0][0] != 1 {
		t.Fatalf("placement polygon aliases source: %+v", image.WrapPolygon)
	}
}

func TestDrawingPlacementApplyToTextBoxCopiesCanonicalState(t *testing.T) {
	shape := &Shape{
		Floating: true, X: 10, Y: 20, Width: 30, Height: 40,
		XRelative: "page", YRelative: "paragraph", XAlign: "center", YAlign: "bottom",
		ZIndex: 9, BehindDoc: true,
		WrapType: "tight", WrapSide: "largest", WrapPolygon: [][2]float64{{1, 2}, {3, 4}},
		DistTop: 1, DistBottom: 2, DistLeft: 3, DistRight: 4,
		SimplePosAttr: true, SimplePosX: 50, SimplePosY: 60,
		EffectExtentL: 7, EffectExtentT: 8, EffectExtentR: 9, EffectExtentB: 10,
		Locked: true, LayoutInCell: true, AllowOverlap: true,
		SizeRelHPct: 50000, SizeRelHFrom: "margin", SizeRelVPct: 25000, SizeRelVFrom: "page",
		RotationDeg: 17, FlipH: true, FlipV: true,
	}
	tb := &TextBox{PaddingLeft: 11, FillColor: "ABCDEF"}
	shape.DrawingPlacement().ApplyToTextBox(tb)

	got := tb.DrawingPlacement()
	want := shape.DrawingPlacement()
	if got.Floating != want.Floating || got.X != want.X || got.Y != want.Y || got.Width != want.Width || got.Height != want.Height ||
		got.XRelative != want.XRelative || got.YRelative != want.YRelative || got.XAlign != want.XAlign || got.YAlign != want.YAlign ||
		got.ZIndex != want.ZIndex || got.BehindDoc != want.BehindDoc || got.WrapType != want.WrapType || got.WrapSide != want.WrapSide ||
		got.SimplePosAttr != want.SimplePosAttr || got.SimplePosX != want.SimplePosX || got.SimplePosY != want.SimplePosY ||
		got.EffectExtentL != want.EffectExtentL || got.EffectExtentT != want.EffectExtentT || got.EffectExtentR != want.EffectExtentR || got.EffectExtentB != want.EffectExtentB ||
		got.Locked != want.Locked || got.LayoutInCell != want.LayoutInCell || got.AllowOverlap != want.AllowOverlap ||
		got.SizeRelHPct != want.SizeRelHPct || got.SizeRelHFrom != want.SizeRelHFrom || got.SizeRelVPct != want.SizeRelVPct || got.SizeRelVFrom != want.SizeRelVFrom ||
		got.RotationDeg != want.RotationDeg || got.FlipH != want.FlipH || got.FlipV != want.FlipV || len(got.WrapPolygon) != len(want.WrapPolygon) {
		t.Fatalf("textbox placement = %+v, want %+v", got, want)
	}
	if tb.PaddingLeft != 11 || tb.FillColor != "ABCDEF" {
		t.Fatalf("visual textbox state changed: %+v", tb)
	}
	got.WrapPolygon[0][0] = 99
	if shape.WrapPolygon[0][0] != 1 || tb.WrapPolygon[0][0] != 1 {
		t.Fatal("projected placement aliases source polygon")
	}
}
