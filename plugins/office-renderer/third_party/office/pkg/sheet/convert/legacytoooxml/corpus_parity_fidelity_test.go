package legacytoooxml

import (
	"testing"

	sheetir "office/pkg/sheet/ir"
	sheetparts "office/pkg/sheet/parsers/ooxml/parts"
)

func TestConvertCellFallsBackToFullStringWhenLegacyRichTextRunsArePartial(t *testing.T) {
	c := &converter{
		pkg: &sheetparts.Package{SharedStrings: &sheetparts.SharedStrings{}},
		stringIndex: map[string]int{},
	}
	cell := &sheetir.Cell{
		Row:      5,
		Col:      2,
		Ref:      "B5",
		Kind:     sheetir.CellString,
		StrValue: "Full canonical cell text; Worksheet column.",
		RichTextRuns: []*sheetir.RichTextRun{
			{Text: "Worksheet column.", Bold: true},
		},
	}

	got := c.convertCell(cell, nil)
	if got.Type != "s" {
		t.Fatalf("cell type = %q, want shared string fallback", got.Type)
	}
	if len(c.pkg.SharedStrings.Items) != 1 || c.pkg.SharedStrings.Items[0].PlainText != cell.StrValue {
		t.Fatalf("shared string = %#v, want full canonical value %q", c.pkg.SharedStrings.Items, cell.StrValue)
	}
}
