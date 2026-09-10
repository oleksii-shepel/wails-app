package sax_test

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"office/pkg/ooxml/opc"
	sheetir "office/pkg/sheet/ir"
	"office/pkg/sheet/parsers/ooxml/parts"
	"office/pkg/sheet/parsers/ooxml/resolve"
	"office/pkg/sheet/parsers/ooxml/sax"
)

type streamedCore struct {
	sheets []streamedSheet
}

type streamedSheet struct {
	name   string
	hidden bool
	rows   map[int]streamedRow
}

type streamedRow struct {
	height       float64
	hidden       bool
	outlineLevel int
	collapsed    bool
	cells        map[string]streamedCell
}

type streamedCell struct {
	ref       string
	cellType  string
	formula   string
	rawValue  string
	text      string
	hyperlink string
}

func TestXlsxCorpusStreamingCoreParity(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", "..", "..", "testdata", "xlsx"))
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Skipf("xlsx corpus unavailable: %v", err)
	}

	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".xlsx") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		name := name
		t.Run(strings.TrimSuffix(name, filepath.Ext(name)), func(t *testing.T) {
			path := filepath.Join(root, name)
			want := loadCompatibilityWorkbook(t, path)
			got := streamCore(t, path)
			assertCoreParity(t, want, got)
		})
	}
}

func loadCompatibilityWorkbook(t *testing.T, path string) *sheetir.Workbook {
	t.Helper()
	pkg, err := opc.Open(path)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	parsed, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load compatibility parts: %v", err)
	}
	wb, err := resolve.ResolvePackage(parsed)
	if err != nil {
		t.Fatalf("resolve compatibility workbook: %v", err)
	}
	return wb
}

func streamCore(t *testing.T, path string) streamedCore {
	t.Helper()
	var out streamedCore
	sheetByIndex := map[int]*streamedSheet{}

	err := sax.WalkPath(path, sax.HandlerFunc(func(event sax.Event) error {
		switch event.Type {
		case sax.EventStartSheet:
			for len(out.sheets) <= event.SheetIndex {
				out.sheets = append(out.sheets, streamedSheet{})
			}
			sheet := &out.sheets[event.SheetIndex]
			sheet.name = event.SheetName
			sheet.hidden = event.Hidden
			sheet.rows = map[int]streamedRow{}
			sheetByIndex[event.SheetIndex] = sheet

		case sax.EventStartRow:
			sheet := sheetByIndex[event.SheetIndex]
			if sheet == nil {
				return fmt.Errorf("row before sheet %d", event.SheetIndex)
			}
			sheet.rows[event.RowIndex] = streamedRow{
				height:       event.RowHeight,
				hidden:       event.RowHidden,
				outlineLevel: event.RowOutlineLevel,
				collapsed:    event.RowCollapsed,
				cells:        map[string]streamedCell{},
			}

		case sax.EventCell:
			sheet := sheetByIndex[event.SheetIndex]
			if sheet == nil {
				return fmt.Errorf("cell before sheet %d", event.SheetIndex)
			}
			row := sheet.rows[event.RowIndex]
			if row.cells == nil {
				row.cells = map[string]streamedCell{}
			}
			link := event.HyperlinkTarget
			if link == "" && event.HyperlinkLocation != "" {
				link = "#" + event.HyperlinkLocation
			}
			row.cells[strings.ToUpper(event.CellRef)] = streamedCell{
				ref:       strings.ToUpper(event.CellRef),
				cellType:  event.CellType,
				formula:   event.Formula,
				rawValue:  event.RawValue,
				text:      event.Text,
				hyperlink: link,
			}
			sheet.rows[event.RowIndex] = row
		}
		return nil
	}), sax.Options{IncludeHiddenSheets: true, IncludeComments: true})
	if err != nil {
		t.Fatalf("stream workbook: %v", err)
	}
	return out
}

func assertCoreParity(t *testing.T, want *sheetir.Workbook, got streamedCore) {
	t.Helper()
	if len(want.Sheets) != len(got.sheets) {
		t.Fatalf("sheet count = %d, streaming = %d", len(want.Sheets), len(got.sheets))
	}
	for i, ws := range want.Sheets {
		gs := got.sheets[i]
		if ws.Name != gs.name || ws.Hidden != gs.hidden {
			t.Fatalf("sheet %d identity = (%q,%v), streaming = (%q,%v)", i, ws.Name, ws.Hidden, gs.name, gs.hidden)
		}

		wantRows := map[int]*sheetir.Row{}
		for _, row := range ws.Rows {
			wantRows[row.Index] = row
		}
		for rowIndex, wr := range wantRows {
			gr, ok := gs.rows[rowIndex]
			if !ok {
				t.Fatalf("sheet %q row %d missing from streaming core", ws.Name, rowIndex)
			}
			if !sameOptionalFloat(wr.HeightPt, gr.height) || wr.Hidden != gr.hidden ||
				wr.OutlineLevel != gr.outlineLevel || wr.Collapsed != gr.collapsed {
				t.Fatalf("sheet %q row %d geometry changed: height=%v hidden=%v outline=%d collapsed=%v -> height=%v hidden=%v outline=%d collapsed=%v",
					ws.Name, rowIndex, wr.HeightPt, wr.Hidden, wr.OutlineLevel, wr.Collapsed,
					gr.height, gr.hidden, gr.outlineLevel, gr.collapsed)
			}
			for _, wc := range wr.Cells {
				gc, ok := gr.cells[strings.ToUpper(wc.Ref)]
				if !ok {
					t.Fatalf("sheet %q cell %s missing from streaming core", ws.Name, wc.Ref)
				}
				assertCellCoreParity(t, ws.Name, wc, gc)
			}
		}
	}
}

func assertCellCoreParity(t *testing.T, sheet string, want *sheetir.Cell, got streamedCell) {
	t.Helper()
	if want.Formula != got.formula {
		t.Fatalf("sheet %q cell %s formula = %q, streaming = %q", sheet, want.Ref, want.Formula, got.formula)
	}
	if want.HyperlinkURL != got.hyperlink {
		t.Fatalf("sheet %q cell %s hyperlink = %q, streaming = %q", sheet, want.Ref, want.HyperlinkURL, got.hyperlink)
	}

	switch want.Kind {
	case sheetir.CellString, sheetir.CellError, sheetir.CellFormula:
		if want.StrValue != got.text {
			t.Fatalf("sheet %q cell %s text = %q, streaming = %q (type=%q raw=%q)", sheet, want.Ref, want.StrValue, got.text, got.cellType, got.rawValue)
		}
	case sheetir.CellBool:
		wantRaw := "0"
		if want.NumValue != 0 {
			wantRaw = "1"
		}
		if strings.TrimSpace(got.rawValue) != wantRaw {
			t.Fatalf("sheet %q cell %s bool = %s, streaming raw = %q", sheet, want.Ref, wantRaw, got.rawValue)
		}
	case sheetir.CellNumber, sheetir.CellDate:
		value, err := strconv.ParseFloat(strings.TrimSpace(got.rawValue), 64)
		if err != nil {
			t.Fatalf("sheet %q cell %s numeric streaming raw %q: %v", sheet, want.Ref, got.rawValue, err)
		}
		if math.Abs(want.NumValue-value) > 1e-9 {
			t.Fatalf("sheet %q cell %s number = %v, streaming = %v", sheet, want.Ref, want.NumValue, value)
		}
	}
}

func sameOptionalFloat(a, b float64) bool {
	if a == 0 || b == 0 {
		return a == b
	}
	return math.Abs(a-b) < 1e-9
}
