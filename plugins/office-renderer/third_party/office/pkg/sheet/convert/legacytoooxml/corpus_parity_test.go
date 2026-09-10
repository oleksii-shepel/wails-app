package legacytoooxml

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"office/pkg/ooxml/opc"
	sheetir "office/pkg/sheet/ir"
	"office/pkg/sheet/parsers/legacy"
	sheetparts "office/pkg/sheet/parsers/ooxml/parts"
	sheetresolve "office/pkg/sheet/parsers/ooxml/resolve"
)

// TestLegacyCorpusSemanticRoundTrip is the corpus/parity closure gate for the
// legacy XLS conversion path. It deliberately compares canonical semantics,
// not generated OOXML relationship IDs, part names, or XML ordering.
func TestLegacyCorpusSemanticRoundTrip(t *testing.T) {
	fixturesDir := legacyCorpusPath(t, "testdata", "xls")
	entries, err := os.ReadDir(fixturesDir)
	if err != nil {
		t.Fatalf("read XLS corpus: %v", err)
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".xls") {
			continue
		}
		count++
		t.Run(entry.Name(), func(t *testing.T) {
			before, err := legacy.ParseXls(filepath.Join(fixturesDir, entry.Name()))
			if err != nil {
				if isEncryptedLegacyCorpusWorkbook(err) {
					t.Skip("encrypted workbook requires password")
				}
				t.Fatalf("parse XLS: %v", err)
			}

			after := legacyRoundTripWorkbook(t, before)
			assertLegacyWorkbookSemanticParity(t, before, after)
		})
	}
	if count == 0 {
		t.Fatal("no XLS corpus fixtures found")
	}
}

func legacyRoundTripWorkbook(t *testing.T, before *sheetir.Workbook) *sheetir.Workbook {
	t.Helper()
	pkg, err := Convert(before)
	if err != nil {
		t.Fatalf("convert legacy workbook: %v", err)
	}
	var buf bytes.Buffer
	if err := sheetparts.WritePackage(pkg, &buf); err != nil {
		t.Fatalf("write XLSX package: %v", err)
	}
	op, err := opc.OpenReaderAt(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("open converted XLSX: %v", err)
	}
	parsed, err := sheetparts.LoadPackage(op)
	if err != nil {
		t.Fatalf("load converted XLSX: %v", err)
	}
	after, err := sheetresolve.ResolvePackage(parsed)
	if err != nil {
		t.Fatalf("resolve converted XLSX: %v", err)
	}
	return after
}

func assertLegacyWorkbookSemanticParity(t *testing.T, before, after *sheetir.Workbook) {
	t.Helper()
	if len(before.Sheets) != len(after.Sheets) {
		t.Fatalf("sheet count changed: %d -> %d", len(before.Sheets), len(after.Sheets))
	}
	if len(before.DefinedNames) != len(after.DefinedNames) {
		t.Fatalf("defined-name count changed: %d -> %d", len(before.DefinedNames), len(after.DefinedNames))
	}
	if len(before.Connections) != len(after.Connections) {
		t.Fatalf("connection count changed: %d -> %d", len(before.Connections), len(after.Connections))
	}
	if len(before.ExternalLinks) != len(after.ExternalLinks) {
		t.Fatalf("external-link count changed: %d -> %d", len(before.ExternalLinks), len(after.ExternalLinks))
	}

	for i := range before.Sheets {
		want, got := before.Sheets[i], after.Sheets[i]
		if want.Name != got.Name || want.Hidden != got.Hidden || want.Visibility != got.Visibility {
			t.Fatalf("sheet %d identity/visibility changed: (%q,%t,%q) -> (%q,%t,%q)", i, want.Name, want.Hidden, want.Visibility, got.Name, got.Hidden, got.Visibility)
		}
		assertLegacyCellParity(t, want, got)
		assertLegacySheetFeatureParity(t, want, got)
	}
}

func assertLegacyCellParity(t *testing.T, want, got *sheetir.Sheet) {
	t.Helper()
	wantCells := canonicalCells(want)
	gotCells := canonicalCells(got)
	if len(wantCells) != len(gotCells) {
		t.Fatalf("sheet %q cell count changed: %d -> %d", want.Name, len(wantCells), len(gotCells))
	}
	for ref, wc := range wantCells {
		gc, ok := gotCells[ref]
		if !ok {
			t.Fatalf("sheet %q lost cell %s", want.Name, ref)
		}
		if !roundTripCellKindsEquivalent(wc.Kind, gc.Kind) || xmlCanonicalCellText(wc) != xmlCanonicalCellText(gc) || wc.Formula != gc.Formula || math.Abs(wc.NumValue-gc.NumValue) > 1e-9 {
			t.Fatalf("sheet %q cell %s changed: kind=%v str=%q num=%v formula=%q -> kind=%v str=%q num=%v formula=%q", want.Name, ref, wc.Kind, wc.StrValue, wc.NumValue, wc.Formula, gc.Kind, gc.StrValue, gc.NumValue, gc.Formula)
		}
	}
}

func assertLegacySheetFeatureParity(t *testing.T, want, got *sheetir.Sheet) {
	t.Helper()
	checks := []struct {
		name string
		want int
		got  int
	}{
		{"merged cells", len(want.MergedCells), len(got.MergedCells)},
		{"data validations", len(want.DataValidations), len(got.DataValidations)},
		{"conditional-format rules", len(want.CFRules), len(got.CFRules)},
		{"hyperlinks", len(want.Hyperlinks), len(got.Hyperlinks)},
		{"drawings", len(want.Drawings), len(got.Drawings)},
		{"form controls", len(want.FormControls), len(got.FormControls)},
		{"tables", len(want.Tables), len(got.Tables)},
		{"pivot tables", len(want.PivotTables), len(got.PivotTables)},
		{"row page breaks", len(want.RowPageBreaks), len(got.RowPageBreaks)},
		{"column page breaks", len(want.ColPageBreaks), len(got.ColPageBreaks)},
	}
	for _, check := range checks {
		if check.want != check.got {
			t.Fatalf("sheet %q %s count changed: %d -> %d", want.Name, check.name, check.want, check.got)
		}
	}
	if want.PaneState != got.PaneState || want.TopLeftCell != got.TopLeftCell {
		t.Fatalf("sheet %q pane/view semantics changed: state/top-left (%q,%q) -> (%q,%q)", want.Name, want.PaneState, want.TopLeftCell, got.PaneState, got.TopLeftCell)
	}
	switch want.PaneState {
	case "frozen", "frozenSplit":
		if want.FrozenRows != got.FrozenRows || want.FrozenCols != got.FrozenCols {
			t.Fatalf("sheet %q frozen pane semantics changed: rows/cols (%d,%d) -> (%d,%d)", want.Name, want.FrozenRows, want.FrozenCols, got.FrozenRows, got.FrozenCols)
		}
	case "split":
		// OOXML stores split panes in xSplit/ySplit units. SplitRows/SplitCols are
		// a legacy convenience projection and are re-derived by the OOXML resolver.
		if math.Abs(want.SplitX-got.SplitX) > 1e-9 || math.Abs(want.SplitY-got.SplitY) > 1e-9 {
			t.Fatalf("sheet %q split pane semantics changed: x/y (%v,%v) -> (%v,%v)", want.Name, want.SplitX, want.SplitY, got.SplitX, got.SplitY)
		}
	}
	if (want.PageSetup == nil) != (got.PageSetup == nil) {
		t.Fatalf("sheet %q page setup presence changed", want.Name)
	}
	if (want.HeaderFooter == nil) != (got.HeaderFooter == nil) {
		t.Fatalf("sheet %q header/footer presence changed", want.Name)
	}
	assertLegacyHeaderFooterParity(t, want, got)
	assertLegacyHyperlinkParity(t, want, got)
	assertLegacyTableParity(t, want, got)
	assertLegacyPivotParity(t, want, got)
	assertLegacyFilterParity(t, want, got)
	if want.PrintArea != got.PrintArea || want.PrintTitleRows != got.PrintTitleRows || want.PrintTitleCols != got.PrintTitleCols {
		t.Fatalf("sheet %q print range/title semantics changed", want.Name)
	}
}

func assertLegacyHeaderFooterParity(t *testing.T, want, got *sheetir.Sheet) {
	t.Helper()
	if want.HeaderFooter == nil {
		return
	}
	wh, gh := want.HeaderFooter, got.HeaderFooter
	if wh.OddHeader != gh.OddHeader || wh.OddFooter != gh.OddFooter ||
		wh.EvenHeader != gh.EvenHeader || wh.EvenFooter != gh.EvenFooter ||
		wh.FirstHeader != gh.FirstHeader || wh.FirstFooter != gh.FirstFooter ||
		wh.DifferentOddEven != gh.DifferentOddEven || wh.DifferentFirst != gh.DifferentFirst ||
		wh.ScaleWithDoc != gh.ScaleWithDoc || wh.AlignWithMargins != gh.AlignWithMargins {
		t.Fatalf("sheet %q header/footer semantics changed across XLS->XLSX conversion: %#v -> %#v", want.Name, wh, gh)
	}
}

func assertLegacyHyperlinkParity(t *testing.T, want, got *sheetir.Sheet) {
	t.Helper()
	if len(want.Hyperlinks) != len(got.Hyperlinks) {
		return // count mismatch is reported by the parent gate
	}
	for i := range want.Hyperlinks {
		w, g := want.Hyperlinks[i], got.Hyperlinks[i]
		if w == nil || g == nil || w.Ref != g.Ref || w.URL != g.URL || w.Display != g.Display || w.Tooltip != g.Tooltip {
			t.Fatalf("sheet %q hyperlink %d semantics changed: %#v -> %#v", want.Name, i, w, g)
		}
	}
}

func assertLegacyTableParity(t *testing.T, want, got *sheetir.Sheet) {
	t.Helper()
	if len(want.Tables) != len(got.Tables) {
		return
	}
	for i := range want.Tables {
		w, g := want.Tables[i], got.Tables[i]
		if w == nil || g == nil {
			t.Fatalf("sheet %q table %d became nil", want.Name, i)
		}
		if w.Name != g.Name || w.DisplayName != g.DisplayName || w.Ref != g.Ref ||
			w.HeaderRowCount != g.HeaderRowCount || w.TotalsRowCount != g.TotalsRowCount ||
			w.TotalsRowShown != g.TotalsRowShown || w.AutoFilterRef != g.AutoFilterRef {
			t.Fatalf("sheet %q table %d semantics changed: %#v -> %#v", want.Name, i, w, g)
		}
		if len(w.Columns) != len(g.Columns) {
			t.Fatalf("sheet %q table %q column count changed: %d -> %d", want.Name, w.Name, len(w.Columns), len(g.Columns))
		}
		for col := range w.Columns {
			if !reflect.DeepEqual(w.Columns[col], g.Columns[col]) {
				t.Fatalf("sheet %q table %q column %d changed: %#v -> %#v", want.Name, w.Name, col, w.Columns[col], g.Columns[col])
			}
		}
	}
}

func assertLegacyPivotParity(t *testing.T, want, got *sheetir.Sheet) {
	t.Helper()
	if len(want.PivotTables) != len(got.PivotTables) {
		return
	}
	for i := range want.PivotTables {
		w, g := want.PivotTables[i], got.PivotTables[i]
		if w == nil || g == nil {
			t.Fatalf("sheet %q pivot %d became nil", want.Name, i)
		}
		if w.Name != g.Name || w.LocationRef != g.LocationRef || w.DataCaption != g.DataCaption ||
			w.SourceSheet != g.SourceSheet || w.SourceRef != g.SourceRef || w.SourceName != g.SourceName ||
			w.RowGrandTotals != g.RowGrandTotals || w.ColGrandTotals != g.ColGrandTotals ||
			w.ShowHeaders != g.ShowHeaders || !reflect.DeepEqual(w.RowFields, g.RowFields) ||
			!reflect.DeepEqual(w.ColFields, g.ColFields) || !reflect.DeepEqual(w.PageFields, g.PageFields) ||
			!reflect.DeepEqual(w.DataFields, g.DataFields) || !reflect.DeepEqual(w.Fields, g.Fields) ||
			!reflect.DeepEqual(w.PageFieldSelections, g.PageFieldSelections) {
			t.Fatalf("sheet %q pivot %d semantics changed:\nwant=%+v\ngot =%+v\nwant fields=%s\ngot fields =%s\nwant data=%s\ngot data =%s",
				want.Name, i, w, g, pivotFieldDebug(w.Fields), pivotFieldDebug(g.Fields), pivotDataFieldDebug(w.DataFields), pivotDataFieldDebug(g.DataFields))
		}
	}
}

func pivotFieldDebug(fields []*sheetir.PivotField) string {
	var out strings.Builder
	for i, field := range fields {
		if i > 0 {
			out.WriteString(" | ")
		}
		if field == nil {
			out.WriteString("<nil>")
			continue
		}
		fmt.Fprintf(&out, "%d:{Name:%q Axis:%q DataField:%t Compact:%t Outline:%t SubtotalTop:%t ShowAll:%t Items:[",
			i, field.Name, field.Axis, field.DataField, field.Compact, field.Outline, field.SubtotalTop, field.ShowAll)
		for j, item := range field.Items {
			if j > 0 {
				out.WriteString(", ")
			}
			if item == nil {
				out.WriteString("<nil>")
				continue
			}
			fmt.Fprintf(&out, "{X:%d Type:%q Name:%q Hidden:%t HideDetail:%t}",
				item.CacheIndex, item.Type, item.Name, item.Hidden, item.HideDetail)
		}
		out.WriteString("]}")
	}
	return out.String()
}

func pivotDataFieldDebug(fields []*sheetir.PivotDataField) string {
	var out strings.Builder
	for i, field := range fields {
		if i > 0 {
			out.WriteString(" | ")
		}
		if field == nil {
			out.WriteString("<nil>")
			continue
		}
		fmt.Fprintf(&out, "%d:{Name:%q Source:%q Subtotal:%q NumFmtID:%d}",
			i, field.Name, field.SourceFieldName, field.Subtotal, field.NumFmtID)
	}
	return out.String()
}

func assertLegacyFilterParity(t *testing.T, want, got *sheetir.Sheet) {
	t.Helper()
	if want.AutoFilterRef != got.AutoFilterRef {
		t.Fatalf("sheet %q AutoFilterRef changed: %q -> %q", want.Name, want.AutoFilterRef, got.AutoFilterRef)
	}
	if !reflect.DeepEqual(want.FilterColumns, got.FilterColumns) {
		t.Fatalf("sheet %q filter-column semantics changed: %#v -> %#v", want.Name, want.FilterColumns, got.FilterColumns)
	}
}

func roundTripCellKindsEquivalent(want, got sheetir.CellKind) bool {
	if want == got {
		return true
	}
	// OOXML stores dates as numeric cells plus a date number format. The OOXML
	// resolver therefore projects a legacy CellDate back as CellNumber.
	return want == sheetir.CellDate && got == sheetir.CellNumber
}

func xmlCanonicalCellText(cell *sheetir.Cell) string {
	// CellDate.StrValue is a legacy convenience projection of the numeric serial.
	// OOXML carries the serial and number format, not a separate ISO date string.
	if cell.Kind == sheetir.CellDate || cell.Kind == sheetir.CellNumber {
		return ""
	}
	return xmlCanonicalText(cell.StrValue)
}

func xmlCanonicalText(s string) string {
	// encoding/xml replaces each malformed UTF-8 byte with U+FFFD. Do not use
	// strings.ToValidUTF8 here: it collapses a consecutive run of invalid bytes
	// into a single replacement rune, which does not match encoding/xml.
	// Ranging the original string yields RuneError once per malformed byte.
	var b strings.Builder
	for _, r := range s {
		if isXML10Rune(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune('\uFFFD')
		}
	}
	return b.String()
}

func isXML10Rune(r rune) bool {
	return r == '\t' || r == '\n' || r == '\r' ||
		(r >= 0x20 && r <= 0xD7FF) ||
		(r >= 0xE000 && r <= 0xFFFD) ||
		(r >= 0x10000 && r <= 0x10FFFF)
}

func canonicalCells(sheet *sheetir.Sheet) map[string]*sheetir.Cell {
	out := make(map[string]*sheetir.Cell)
	for _, row := range sheet.Rows {
		for _, cell := range row.Cells {
			out[cell.Ref] = cell
		}
	}
	return out
}

func isEncryptedLegacyCorpusWorkbook(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "encrypted workbook") && strings.Contains(message, "password")
}

func legacyCorpusPath(t *testing.T, elems ...string) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", ".."))
	return filepath.Join(append([]string{root}, elems...)...)
}
