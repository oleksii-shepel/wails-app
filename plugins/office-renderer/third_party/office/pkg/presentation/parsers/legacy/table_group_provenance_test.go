package legacy

import (
	"testing"

	presentationir "office/pkg/presentation/ir"
)

func TestDuplicateProjectedTableCanBeRejectedByGroupOwnedSiblingText(t *testing.T) {
	paragraphs := func(text string) []*presentationir.Paragraph {
		return []*presentationir.Paragraph{{Runs: []*presentationir.Run{{Text: text}}}}
	}
	table := &presentationir.Shape{Kind: presentationir.ShapeTable, Table: &presentationir.Table{Rows: []*presentationir.TableRow{
		{Cells: []*presentationir.TableCell{{Paragraphs: paragraphs("Space Segment")}, {Paragraphs: paragraphs("Control Segment")}}},
		{Cells: []*presentationir.TableCell{{Paragraphs: paragraphs("User Segment")}}},
	}}}
	groups := map[int][]*presentationir.Shape{7: {{Kind: presentationir.ShapeGroup, Children: []*presentationir.Shape{
		{Kind: presentationir.ShapeText, Paragraphs: paragraphs("Space Segment")},
		{Kind: presentationir.ShapeText, Paragraphs: paragraphs("Control Segment")},
		{Kind: presentationir.ShapeText, Paragraphs: paragraphs("User Segment")},
	}}}}
	projected := map[int][]*presentationir.Shape{7: {table}}
	filtered := filterLegacyDuplicateProjectedTables(groups, projected)
	if len(filtered[7]) != 0 {
		t.Fatalf("group-owned sibling text left %d duplicate table(s), want 0", len(filtered[7]))
	}
}
