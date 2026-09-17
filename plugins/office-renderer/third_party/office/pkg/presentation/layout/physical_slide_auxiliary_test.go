package layout

import (
	"testing"

	"office/pkg/presentation/ir"
)

func TestMaterializePhysicalSlideOwnsAuxiliarySurfaces(t *testing.T) {
	note := &ir.Shape{Kind: ir.ShapeText, Bounds: ir.Rect{W: 1000, H: 1000}}
	slide := &ir.Slide{
		Notes:    &ir.NotesSlide{BgColor: "FFF2CC", Shapes: []*ir.Shape{note}},
		Comments: []*ir.Comment{{AuthorName: "Ada", Initials: "AL", DateTime: "2026-09-12T18:00:00Z", Text: "Review", X: 11, Y: 22}},
	}
	got := MaterializePhysicalSlide(slide)
	if got.Auxiliary.Notes == nil || got.Auxiliary.Notes.BackgroundColor != "FFF2CC" || len(got.Auxiliary.Notes.Content) != 1 {
		t.Fatalf("notes auxiliary plan = %+v", got.Auxiliary.Notes)
	}
	if len(got.Auxiliary.Comments) != 1 {
		t.Fatalf("comments auxiliary plan = %+v", got.Auxiliary.Comments)
	}
	comment := got.Auxiliary.Comments[0]
	if comment.AuthorName != "Ada" || comment.Initials != "AL" || comment.Text != "Review" || comment.AnchorXEMU != 11 || comment.AnchorYEMU != 22 {
		t.Fatalf("comment auxiliary plan = %+v", comment)
	}
}
