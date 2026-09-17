package sax

import (
	"math"

	"office/pkg/ooxml/drawing"
	"office/pkg/presentation/ir"
	"testing"
)

func TestBackgroundStyleRefUsesThemeBackgroundFillAndLayoutColorMap(t *testing.T) {
	b := newIRBuilder()
	b.slide = &ir.Slide{Index: 1}
	b.themeColors["dk1"] = "000000"
	b.layoutColorMap = map[string]string{"bg1": "dk1"}
	b.activeColorScope = "layout"
	b.themeFillStyles[1002] = ThemeFillStyle{Kind: "gradient", Gradient: GradientFill{
		Path: "circle", HasFillToRect: true, FillToLeft: 65000, FillToBottom: 98000,
		Stops: []GradientStop{{Position: 0, Color: VisualColor{Kind: "schemeClr", Value: "phClr"}}, {Position: 100000, Color: VisualColor{Kind: "schemeClr", Value: "phClr"}}},
	}}
	if err := b.HandleEvent(Event{Type: EventBackgroundStyleRef, SlideIndex: 1, Scope: "layout", StyleIndex: 1002, Color: VisualColor{Kind: "schemeClr", Value: "bg1"}}); err != nil {
		t.Fatal(err)
	}
	g, ok := b.slide.BgFill.(drawing.GradientFill)
	if !ok {
		t.Fatalf("background fill = %T, want drawing.GradientFill", b.slide.BgFill)
	}
	if g.GradientType != drawing.GradientTypePath {
		t.Fatalf("gradient type = %q, want path", g.GradientType)
	}
	if g.FillToRect == nil || math.Abs(g.FillToRect.X-.65) > 1e-9 || math.Abs(g.FillToRect.Y) > 1e-9 || math.Abs(g.FillToRect.Width-.35) > 1e-9 || math.Abs(g.FillToRect.Height-.02) > 1e-9 {
		t.Fatalf("fillToRect = %#v", g.FillToRect)
	}
}

func TestInheritedMasterTextColorResolvesThroughEffectiveLayoutColorMap(t *testing.T) {
	b := newIRBuilder()
	b.themeColors["lt1"] = "FFFFFF"
	b.layoutColorMap = map[string]string{"tx1": "lt1"}
	b.activeColorScope = "layout"
	r := &ir.Run{}
	b.inheritRunStyle(r, RunProperties{Color: VisualColor{Kind: "schemeClr", Value: "tx1"}})
	if r.Color != "FFFFFF" {
		t.Fatalf("inherited color = %q, want FFFFFF", r.Color)
	}
}

func TestThemeBackgroundPhClrPreservesPerStopTransforms(t *testing.T) {
	b := newIRBuilder()
	b.themeColors["dk1"] = "000000"
	b.slideColorMap = map[string]string{"bg1": "dk1"}
	b.activeColorScope = "slide"
	b.slide = &ir.Slide{}
	b.themeFillStyles[1002] = ThemeFillStyle{Kind: "gradient", Gradient: GradientFill{Stops: []GradientStop{
		{Position: 0, Color: VisualColor{Kind: "schemeClr", Value: "phClr", Tint: 55000, HasTint: true, SatMod: 300000, HasSatMod: true}},
		{Position: 100000, Color: VisualColor{Kind: "schemeClr", Value: "phClr", Shade: 65000, HasShade: true, SatMod: 300000, HasSatMod: true}},
	}}}
	if err := b.HandleEvent(Event{Type: EventBackgroundStyleRef, StyleIndex: 1002, Color: VisualColor{Kind: "schemeClr", Value: "bg1"}}); err != nil {
		t.Fatal(err)
	}
	g, ok := b.slide.BgFill.(drawing.GradientFill)
	if !ok || len(g.Stops) != 2 {
		t.Fatalf("background fill = %#v", b.slide.BgFill)
	}
	if g.Stops[0].Color == g.Stops[1].Color {
		t.Fatalf("phClr transforms collapsed gradient stops to %q", g.Stops[0].Color)
	}
	if g.Stops[0].Color == "000000" {
		t.Fatalf("tinted phClr stop remained untransformed black")
	}
}

func TestDirectBackgroundPictureUsesOwningPartPayloadAndStretchRect(t *testing.T) {
	b := newIRBuilder()
	b.slide = &ir.Slide{Index: 0}
	b.partPayloads["ppt/slides/slide1.xml"] = map[string]relationshipPayload{
		"rId2": {Data: []byte{1, 2, 3}, ContentType: "image/jpeg"},
	}
	if err := b.HandleEvent(Event{
		Type: EventBackgroundPicture, PartName: "ppt/slides/slide1.xml", SlideIndex: 0, Scope: "slide", RelID: "rId2",
		PictureFillLeft: -18000, PictureFillRight: -18000,
	}); err != nil {
		t.Fatal(err)
	}
	fill, ok := b.slide.BgFill.(drawing.PictureFill)
	if !ok {
		t.Fatalf("background fill = %T, want drawing.PictureFill", b.slide.BgFill)
	}
	if fill.ContentType != "image/jpeg" || len(fill.ImageData) != 3 || !fill.Stretch {
		t.Fatalf("picture fill = %#v", fill)
	}
	if fill.FillRect == nil || math.Abs(fill.FillRect.X-(-.18)) > 1e-9 || math.Abs(fill.FillRect.Width-1.36) > 1e-9 {
		t.Fatalf("fillRect = %#v, want x=-.18 width=1.36", fill.FillRect)
	}
}
