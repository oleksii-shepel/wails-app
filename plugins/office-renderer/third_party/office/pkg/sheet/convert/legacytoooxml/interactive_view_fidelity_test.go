package legacytoooxml

import (
	sheetir "office/pkg/sheet/ir"
	sheetparts "office/pkg/sheet/parsers/ooxml/parts"
	"testing"
)

func TestPhase3PaneAndValidationMetadataMapsBackToOOXML(t *testing.T) {
	c := &converter{}
	sheet := &sheetir.Sheet{
		PaneState: "split", SplitX: 1234.5, SplitY: 678.25, TopLeftCell: "D9", ActivePane: "bottomRight",
		PaneSelections:  []sheetir.PaneSelection{{Pane: "bottomRight", ActiveCell: "F12", ActiveCellID: 2, SqRef: "F12:H14"}},
		DataValidations: []*sheetir.DataValidation{{SqRef: "A1:A9", Type: "whole", Operator: "between", ErrorStyle: "warning", IMEMode: "hiragana", Formula1: "1", Formula2: "9"}},
	}
	ws := &sheetparts.Worksheet{}
	c.mapPanes(sheet, ws)
	c.mapDataValidations(sheet, ws)
	if len(ws.SheetViews) != 1 || ws.SheetViews[0].Pane == nil {
		t.Fatalf("sheet views = %#v", ws.SheetViews)
	}
	p := ws.SheetViews[0].Pane
	if p.State != "split" || p.XSplit != 1234.5 || p.YSplit != 678.25 || p.TopLeftCell != "D9" || p.ActivePane != "bottomRight" {
		t.Fatalf("pane = %#v", p)
	}
	if len(ws.SheetViews[0].Selections) != 1 || ws.SheetViews[0].Selections[0].SqRef != "F12:H14" || ws.SheetViews[0].Selections[0].ActiveCellID != 2 {
		t.Fatalf("selections = %#v", ws.SheetViews[0].Selections)
	}
	if len(ws.DataValidations) != 1 || ws.DataValidations[0].ErrorStyle != "warning" || ws.DataValidations[0].IMEMode != "hiragana" {
		t.Fatalf("validations = %#v", ws.DataValidations)
	}
}
