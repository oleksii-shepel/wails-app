package legacy

import (
	"testing"

	officeartir "office/pkg/officeart/ir"
	presentationir "office/pkg/presentation/ir"
)

func TestPass31OfficeArtCustomPathPaintFlagsSurvivePresentationBridge(t *testing.T) {
	shape := &presentationir.Shape{Kind: presentationir.ShapeOther, Bounds: presentationir.Rect{W: 1000, H: 1000}}
	shared := &officeartir.Shape{Geometry: officeartir.Geometry{
		Kind: officeartir.GeometryCustom,
		Paths: []officeartir.Path{
			{Stroke: true, Fill: false, Commands: []officeartir.PathCommand{{Op: "M", Points: []officeartir.PointF{{X: 0, Y: 0}}}, {Op: "L", Points: []officeartir.PointF{{X: 1, Y: 1}}}}},
			{Stroke: false, Fill: true, Commands: []officeartir.PathCommand{{Op: "M", Points: []officeartir.PointF{{X: 0, Y: 0}}}, {Op: "L", Points: []officeartir.PointF{{X: 1, Y: 0}}}, {Op: "Z"}}},
		},
	}}
	applyLegacyPptOfficeArtGeometry(shape, shared)
	if shape.GeometryData == nil || len(shape.GeometryData.Paths) != 2 {
		t.Fatalf("custom geometry paths = %#v", shape.GeometryData)
	}
	if got := shape.GeometryData.Paths[0]; !got.PaintFlagsSet || got.FillEnabled || !got.StrokeEnabled {
		t.Fatalf("stroke-only path flags = set:%v fill:%v stroke:%v", got.PaintFlagsSet, got.FillEnabled, got.StrokeEnabled)
	}
	if got := shape.GeometryData.Paths[1]; !got.PaintFlagsSet || !got.FillEnabled || got.StrokeEnabled {
		t.Fatalf("fill-only path flags = set:%v fill:%v stroke:%v", got.PaintFlagsSet, got.FillEnabled, got.StrokeEnabled)
	}
}

func TestPass31LegacyBulletWithoutAuthoredColorFollowsEffectiveTextColor(t *testing.T) {
	paragraph := &presentationir.Paragraph{
		BulletSpecified: true,
		BulletChar:      "•",
		Runs:            []*presentationir.Run{{Text: "white", Color: "FFFFFF"}},
	}
	master := legacyPptMasterTextLevel{paragraph: stpaPFEntry{BulletColor: "000000", HasBulletColor: false}}
	got := mergeLegacyPptMasterParagraph(paragraph, master, true)
	if got.BulletColor != "FFFFFF" || got.BulletColorExplicit {
		t.Fatalf("bullet color = %q explicit=%v, want effective text FFFFFF and inherited provenance", got.BulletColor, got.BulletColorExplicit)
	}
}

func TestPass31LegacyAuthoredMasterBulletColorStillWins(t *testing.T) {
	paragraph := &presentationir.Paragraph{
		BulletSpecified: true,
		BulletChar:      "•",
		Runs:            []*presentationir.Run{{Text: "white", Color: "FFFFFF"}},
	}
	master := legacyPptMasterTextLevel{paragraph: stpaPFEntry{BulletColor: "112233", HasBulletColor: true}}
	got := mergeLegacyPptMasterParagraph(paragraph, master, true)
	if got.BulletColor != "112233" || !got.BulletColorExplicit {
		t.Fatalf("bullet color = %q explicit=%v, want authored master 112233", got.BulletColor, got.BulletColorExplicit)
	}
}

func TestPass31LegacyCutAnimationDuplicateBecomesAnimationOnlyOverlay(t *testing.T) {
	base := &presentationir.Shape{
		Kind:       presentationir.ShapeText,
		Bounds:     presentationir.Rect{X: 100, Y: 200, W: 1000, H: 800},
		Paragraphs: []*presentationir.Paragraph{{Runs: []*presentationir.Run{{Text: "same text"}}}},
	}
	overlayText := &presentationir.Shape{
		Kind:       presentationir.ShapeText,
		Paragraphs: []*presentationir.Paragraph{{Runs: []*presentationir.Run{{Text: "same text"}}}},
	}
	overlay := &presentationir.Shape{
		Kind:              presentationir.ShapeGroup,
		Bounds:            presentationir.Rect{X: 90, Y: 200, W: 1030, H: 800},
		AnimStep:          1,
		EmphasisEffects:   []string{"cut"},
		AnimationTriggers: []string{"onClick"},
		Children:          []*presentationir.Shape{overlayText},
	}
	markLegacyPptAnimationStateOverlays([]*presentationir.Shape{base, overlay})
	if !overlay.AnimationOnly || !overlayText.AnimationOnly {
		t.Fatalf("overlay flags = group:%v child:%v, want both animation-only", overlay.AnimationOnly, overlayText.AnimationOnly)
	}
}

func TestPass101StaticCutStatePreservesAnimatedTextInCanonicalIR(t *testing.T) {
	base := &presentationir.Shape{
		Kind: presentationir.ShapeText, Bounds: presentationir.Rect{X: 100, Y: 200, W: 1000, H: 800},
		Paragraphs: []*presentationir.Paragraph{{Runs: []*presentationir.Run{{Text: "base text"}}}},
	}
	overlayText := &presentationir.Shape{
		Kind:       presentationir.ShapeText,
		Paragraphs: []*presentationir.Paragraph{{Runs: []*presentationir.Run{{Text: "base text"}}}},
	}
	frame := &presentationir.Shape{
		Kind: presentationir.ShapeOther, FillColor: "FFFF00",
		OfficeArt: &officeartir.Shape{Fill: officeartir.Fill{Kind: officeartir.FillSolid, Color: "FFFF00", Alpha: 1}},
	}
	overlay := &presentationir.Shape{
		Kind: presentationir.ShapeGroup, Bounds: presentationir.Rect{X: 90, Y: 200, W: 1030, H: 800},
		AnimStep: 1, EmphasisEffects: []string{"cut"}, Children: []*presentationir.Shape{overlayText, frame},
	}
	markLegacyPptAnimationStateOverlays([]*presentationir.Shape{base, overlay})
	if len(overlay.Children) != 2 || overlay.Children[0] != overlayText {
		t.Fatalf("overlay children = %d, want authored animated text preserved", len(overlay.Children))
	}
	if !overlayText.AnimationOnly {
		t.Fatal("animated state text must be retained as AnimationOnly")
	}
	if got := legacyPptNormalizedShapeText(base); got != "base text" {
		t.Fatalf("base text = %q", got)
	}
}

func TestPass101InvisibleEmptyTextboxIsNotCanonicalVisual(t *testing.T) {
	empty := &officeartir.Shape{Fill: officeartir.Fill{Kind: officeartir.FillNone}, Text: &officeartir.TextBody{}}
	if !legacyPptOfficeArtShapeIsEmptyTextbox(empty) {
		t.Fatal("empty unpainted textbox must be structural")
	}
	painted := &officeartir.Shape{Fill: officeartir.Fill{Kind: officeartir.FillSolid, Color: "FFFF00", Alpha: 1}, Text: &officeartir.TextBody{}}
	if legacyPptOfficeArtShapeIsEmptyTextbox(painted) {
		t.Fatal("painted empty textbox is authored visual content")
	}
	withText := &officeartir.Shape{Fill: officeartir.Fill{Kind: officeartir.FillNone}, Text: &officeartir.TextBody{Paragraphs: []officeartir.Paragraph{{Runs: []officeartir.TextRun{{Text: "x"}}}}}}
	if legacyPptOfficeArtShapeIsEmptyTextbox(withText) {
		t.Fatal("textbox with semantic text is authored content")
	}
}
