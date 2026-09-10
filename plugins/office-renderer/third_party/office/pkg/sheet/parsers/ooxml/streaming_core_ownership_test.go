package ooxml

import (
	"os"
	"strings"
	"testing"

	sheetir "office/pkg/sheet/ir"
	"office/pkg/sheet/parsers/ooxml/sax"
)

func TestApplyStreamingCellCoreOwnsValueFormulaAndHyperlink(t *testing.T) {
	cell := &sheetir.Cell{
		Row:          9,
		Col:          9,
		Ref:          "I9",
		Kind:         sheetir.CellString,
		StrValue:     "compatibility value",
		Formula:      "OLD()",
		HyperlinkURL: "https://old.invalid/",
		Style:        sheetir.CellStyle{Bold: true},
	}

	applyStreamingCellCore(cell, sax.Event{
		RowIndex:        2,
		ColIndex:        3,
		CellRef:         "C2",
		CellType:        "str",
		Formula:         `CONCAT("stream","ed")`,
		Text:            "streamed",
		HyperlinkTarget: "https://example.test/",
		StyleIndex:      1,
	}, []sheetir.CellStyle{{}, {Italic: true}})

	if cell.Row != 2 || cell.Col != 3 || cell.Ref != "C2" {
		t.Fatalf("coordinates = (%d,%d,%q), want streamed C2", cell.Row, cell.Col, cell.Ref)
	}
	if cell.Kind != sheetir.CellFormula || cell.StrValue != "streamed" || cell.Formula != `CONCAT("stream","ed")` {
		t.Fatalf("streamed cell core not authoritative: %#v", cell)
	}
	if cell.HyperlinkURL != "https://example.test/" {
		t.Fatalf("hyperlink = %q, want streamed target", cell.HyperlinkURL)
	}
	if cell.Style.Bold || !cell.Style.Italic {
		t.Fatalf("cell style = %#v, want streamed style index 1", cell.Style)
	}
}

func TestOOXMLParserAppliesStreamingCoreAfterMetadataResolution(t *testing.T) {
	data, err := osReadParseSource()
	if err != nil {
		t.Fatal(err)
	}
	resolveAt := strings.Index(data, "resolve.ResolveMetadataPackage(parsed)")
	streamAt := strings.Index(data, "applyStreamingWorksheetCore(r, size, wb, styles, parsed.Styles, parsed.Theme)")
	if resolveAt < 0 || streamAt < 0 || streamAt <= resolveAt {
		t.Fatal("OOXML parser must apply SAX worksheet core after package metadata resolution")
	}
}

func osReadParseSource() (string, error) {
	data, err := os.ReadFile("parse.go")
	return string(data), err
}
