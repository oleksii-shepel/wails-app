package resolve

import (
	"office/pkg/sheet/parsers/ooxml/parts"
	"testing"
)

func TestPhase3PaneAndValidationMetadataResolveLosslessly(t *testing.T) {
	ws := &parts.Worksheet{
		SheetViews: []parts.SheetView{{
			WorkbookViewID: 0,
			Pane:           &parts.Pane{XSplit: 1234.5, YSplit: 678.25, TopLeftCell: "D9", ActivePane: "bottomRight", State: "split"},
			Selections:     []parts.Selection{{Pane: "bottomRight", ActiveCell: "F12", ActiveCellID: 2, SqRef: "F12:H14"}},
		}},
		DataValidations: []parts.DataValidation{{SqRef: "A1:A9", Type: "whole", Operator: "between", ErrorStyle: "warning", IMEMode: "hiragana", Formula1: "1", Formula2: "9"}},
	}
	got := resolveWorksheet(&parts.WorksheetEntry{Name: "Sheet1", Sheet: ws}, nil, nil, nil, nil, nil, nil, nil, nil)
	if got.PaneState != "split" || got.SplitX != 1234.5 || got.SplitY != 678.25 || got.TopLeftCell != "D9" || got.ActivePane != "bottomRight" {
		t.Fatalf("pane metadata = %#v", got)
	}
	if len(got.PaneSelections) != 1 || got.PaneSelections[0].ActiveCell != "F12" || got.PaneSelections[0].SqRef != "F12:H14" || got.PaneSelections[0].ActiveCellID != 2 {
		t.Fatalf("selection metadata = %#v", got.PaneSelections)
	}
	if len(got.DataValidations) != 1 || got.DataValidations[0].ErrorStyle != "warning" || got.DataValidations[0].IMEMode != "hiragana" {
		t.Fatalf("validation metadata = %#v", got.DataValidations)
	}
}
