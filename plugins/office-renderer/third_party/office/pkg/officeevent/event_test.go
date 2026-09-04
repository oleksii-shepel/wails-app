package officeevent

import (
	"bytes"
	"testing"

	presedit "office/pkg/presentation/edit"
	sheetedit "office/pkg/sheet/edit"
	"office/pkg/word"
)

func TestWalkBytesDispatchesWordEvents(t *testing.T) {
	data := makeDOCXFixture(t, "office word event")
	var sawText bool
	err := WalkBytes("fixture.docx", data, HandlerFunc(func(event Event) error {
		if event.Format == FormatWord && event.Word != nil && event.Word.Text == "office word event" {
			sawText = true
		}
		return nil
	}), Options{})
	if err != nil {
		t.Fatalf("WalkBytes: %v", err)
	}
	if !sawText {
		t.Fatal("expected shared office event dispatcher to emit wrapped word text")
	}
}

func TestWalkBytesDispatchesPresentationEvents(t *testing.T) {
	data := makePPTXFixture(t, "office presentation event")
	var sawText bool
	err := WalkBytes("fixture.pptx", data, HandlerFunc(func(event Event) error {
		if event.Format == FormatPresentation && event.Presentation != nil && event.Presentation.Text == "office presentation event" {
			sawText = true
		}
		return nil
	}), Options{})
	if err != nil {
		t.Fatalf("WalkBytes: %v", err)
	}
	if !sawText {
		t.Fatal("expected shared office event dispatcher to emit wrapped presentation text")
	}
}

func TestWalkBytesDispatchesSpreadsheetEvents(t *testing.T) {
	data := makeXLSXFixture(t)
	var sawText bool
	err := WalkBytes("fixture.xlsx", data, HandlerFunc(func(event Event) error {
		if event.Format == FormatSpreadsheet && event.Spreadsheet != nil && event.Spreadsheet.CellRef == "A1" && event.Spreadsheet.Text == "office sheet event" {
			sawText = true
		}
		return nil
	}), Options{})
	if err != nil {
		t.Fatalf("WalkBytes: %v", err)
	}
	if !sawText {
		t.Fatal("expected shared office event dispatcher to emit wrapped spreadsheet cell text")
	}
}

func makeDOCXFixture(t *testing.T, text string) []byte {
	t.Helper()
	editor := word.New()
	p := editor.AddParagraph()
	p.AddRun().SetText(text)
	var buf bytes.Buffer
	if err := editor.SaveAsDOCX(&buf); err != nil {
		t.Fatalf("SaveAsDOCX: %v", err)
	}
	return buf.Bytes()
}

func makePPTXFixture(t *testing.T, text string) []byte {
	t.Helper()
	p := presedit.NewPresentation()
	s := p.AddBlankSlide()
	s.AddTextBox(100, 100, 3000, 1200).SetText(text)
	var buf bytes.Buffer
	if err := p.SaveAsPPTX(&buf); err != nil {
		t.Fatalf("SaveAsPPTX: %v", err)
	}
	return buf.Bytes()
}

func makeXLSXFixture(t *testing.T) []byte {
	t.Helper()
	wb := sheetedit.NewWorkbook()
	sheet, err := wb.CreateSheet("Data")
	if err != nil {
		t.Fatalf("CreateSheet Data: %v", err)
	}
	cell, err := sheet.CreateCell(1, 1)
	if err != nil {
		t.Fatalf("CreateCell A1: %v", err)
	}
	cell.SetString("office sheet event")
	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}
	return buf.Bytes()
}
