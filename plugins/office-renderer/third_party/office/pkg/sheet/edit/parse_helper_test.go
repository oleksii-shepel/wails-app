package edit_test

import (
	"testing"

	"office/pkg/sheet"
	sheetir "office/pkg/sheet/ir"
)

func resolveWorkbook(t *testing.T, data []byte) *sheet.Workbook {
	t.Helper()
	wb, err := sheet.OpenBytes(data)
	if err != nil {
		t.Fatalf("parse workbook: %v", err)
	}
	return wb
}

func findCell(t *testing.T, sheet *sheetir.Sheet, row, col int) *sheetir.Cell {
	t.Helper()
	for _, r := range sheet.Rows {
		if r.Index != row {
			continue
		}
		for _, c := range r.Cells {
			if c.Col == col {
				return c
			}
		}
	}
	t.Fatalf("cell %d,%d not found", row, col)
	return nil
}
