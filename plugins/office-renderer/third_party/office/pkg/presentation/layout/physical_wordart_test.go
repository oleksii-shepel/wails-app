package layout

import (
	"testing"

	officeartir "office/pkg/officeart/ir"
	"office/pkg/presentation/ir"
)

func TestPhysicalTextPlanOwnsOfficeArtGeometryText(t *testing.T) {
	shape := &ir.Shape{
		OfficeArt: &officeartir.Shape{GeometryText: &officeartir.GeometryText{
			Text: "WordArt", FontFamily: "Arial Black", FontSizePt: 28,
			Spacing: .12, Align: 1, Warp: "archUp", Vertical: true,
			Bold: true, Italic: true, Underline: true, Shadow: true, SmallCaps: true,
		}},
		Bounds:     ir.Rect{W: 914400 * 4, H: 914400},
		Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{Text: "WordArt"}}}},
	}
	items := MaterializePhysicalVisualTree([]*ir.Shape{shape})
	if len(items) != 1 || items[0].Text == nil || !items[0].Text.Geometry.Enabled {
		t.Fatalf("physical WordArt plan missing: %+v", items)
	}
	got := items[0].Text.Geometry
	if got.Text != "WordArt" || got.FontFamily != "Arial Black" || got.FontSizePt != 28 || got.Warp != "archUp" {
		t.Fatalf("geometry text identity lost: %+v", got)
	}
	if got.Align != "center" || !got.Vertical || !got.Bold || !got.Italic || !got.Underline || !got.Shadow || !got.SmallCaps {
		t.Fatalf("geometry text paint semantics lost: %+v", got)
	}
}
