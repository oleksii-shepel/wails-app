package sheet

import (
	"bytes"
	"testing"

	sheetedit "office/pkg/sheet/edit"
)

func TestWalkBytesFacadeEmitsSpreadsheetEvents(t *testing.T) {
	data := makeXLSXFixture(t)

	var sawText bool
	var sawFormula bool
	var sawHyperlink bool
	var sawComment bool
	var sawHiddenSheet bool
	err := WalkBytes(data, EventHandlerFunc(func(event Event) error {
		switch {
		case event.Type == EventCell && event.CellRef == "A1" && event.Text == "sheet event text":
			sawText = true
		case event.Type == EventCell && event.CellRef == "A2" && event.Formula == "SUM(B1:B1)" && event.Text == "42":
			sawFormula = true
		case event.Type == EventCell && event.CellRef == "B1" && event.HyperlinkTarget == "https://example.com":
			sawHyperlink = true
		case event.Type == EventComment && event.CellRef == "C3" && event.CommentText == "sheet comment":
			sawComment = true
		case event.Type == EventStartSheet && event.SheetName == "Hidden":
			sawHiddenSheet = true
		}
		return nil
	}), EventOptions{IncludeComments: true, IncludeHiddenSheets: true})
	if err != nil {
		t.Fatalf("WalkBytes: %v", err)
	}
	if !sawText {
		t.Fatal("expected spreadsheet event facade to emit A1 text")
	}
	if !sawFormula {
		t.Fatal("expected spreadsheet event facade to emit formula metadata")
	}
	if !sawHyperlink {
		t.Fatal("expected spreadsheet event facade to resolve hyperlink target")
	}
	if !sawComment {
		t.Fatal("expected spreadsheet event facade to emit standalone comments")
	}
	if !sawHiddenSheet {
		t.Fatal("expected spreadsheet event facade to include hidden sheet when requested")
	}
}

func makeXLSXFixture(t *testing.T) []byte {
	t.Helper()

	wb := sheetedit.NewWorkbook()
	visible, err := wb.CreateSheet("Visible")
	if err != nil {
		t.Fatalf("CreateSheet Visible: %v", err)
	}
	hidden, err := wb.CreateSheet("Hidden")
	if err != nil {
		t.Fatalf("CreateSheet Hidden: %v", err)
	}
	hidden.SetHidden(true)

	cell, err := visible.CreateCell(1, 1)
	if err != nil {
		t.Fatalf("CreateCell A1: %v", err)
	}
	cell.SetString("sheet event text")

	cell, err = visible.CreateCell(1, 2)
	if err != nil {
		t.Fatalf("CreateCell B1: %v", err)
	}
	cell.SetString("linked text")
	cell.SetHyperlink("https://example.com")

	cell, err = visible.CreateCell(2, 1)
	if err != nil {
		t.Fatalf("CreateCell A2: %v", err)
	}
	cell.SetFormula("SUM(B1:B1)", "42")

	if err := visible.SetComment("C3", "sheet comment"); err != nil {
		t.Fatalf("SetComment C3: %v", err)
	}

	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}
	return buf.Bytes()
}
