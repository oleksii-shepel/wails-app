package stream_test

import (
	"bytes"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"office/pkg/ooxml/opc"
	sheetir "office/pkg/sheet/ir"
	sheetparts "office/pkg/sheet/parsers/ooxml/parts"
	sheetresolve "office/pkg/sheet/parsers/ooxml/resolve"
	sheetstream "office/pkg/sheet/stream"
)

func TestStreamingWriterRoundTrip(t *testing.T) {
	var out bytes.Buffer
	wb, err := sheetstream.New(&out, sheetstream.Options{RowWindowSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	styleID := wb.AddStyle(sheetir.CellStyle{Bold: true, FontColor: "FF0000", BgColor: "FFFF00", NumFmtCode: "0.00", HAlign: "center"})
	sheet, err := wb.AddSheet("Data")
	if err != nil {
		t.Fatal(err)
	}
	if err := sheet.SetColumnWidth(1, 22); err != nil {
		t.Fatal(err)
	}
	if err := sheet.FreezePanes(1, 1); err != nil {
		t.Fatal(err)
	}
	if err := sheet.SetAutoFilter("A1:C3"); err != nil {
		t.Fatal(err)
	}
	if err := sheet.MergeCells("D1:E1"); err != nil {
		t.Fatal(err)
	}

	r1, err := sheet.CreateRow(1)
	if err != nil {
		t.Fatal(err)
	}
	if err := r1.SetCell(1, sheetstream.Cell{Value: "Name", StyleID: styleID, Comment: "header note", CommentAuthor: "tester"}); err != nil {
		t.Fatal(err)
	}
	if err := r1.SetCell(2, sheetstream.Cell{Value: "URL", Hyperlink: "https://example.com"}); err != nil {
		t.Fatal(err)
	}
	if err := r1.SetCell(3, sheetstream.Cell{Value: "Total"}); err != nil {
		t.Fatal(err)
	}
	if err := r1.SetCell(4, sheetstream.Cell{Value: "Merged"}); err != nil {
		t.Fatal(err)
	}

	r2, _ := sheet.CreateRow(2)
	_ = r2.SetCell(1, sheetstream.Cell{Value: "Alpha"})
	_ = r2.SetCell(2, sheetstream.Cell{Value: true})
	_ = r2.SetCell(3, sheetstream.Cell{Formula: "SUM(1,2)", Cached: 3.0, StyleID: styleID})

	r3, _ := sheet.CreateRow(3)
	_ = r3.SetCell(1, sheetstream.Cell{Value: "Beta"})
	_ = r3.SetCell(2, sheetstream.Cell{Value: 12.5})
	if err := r1.SetHidden(true); !errors.Is(err, sheetstream.ErrRowFlushed) {
		t.Fatalf("mutating flushed row error = %v", err)
	}

	second, err := wb.AddSheet("Second")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := second.AppendRow("hello", 42); err != nil {
		t.Fatal(err)
	}
	if err := wb.Close(); err != nil {
		t.Fatal(err)
	}

	got := resolveBytes(t, out.Bytes())
	if len(got.Sheets) != 2 {
		t.Fatalf("sheets=%d", len(got.Sheets))
	}
	data := got.Sheets[0]
	if data.Name != "Data" {
		t.Fatalf("name=%q", data.Name)
	}
	if data.FrozenRows != 1 || data.FrozenCols != 1 {
		t.Fatalf("freeze=%d,%d", data.FrozenRows, data.FrozenCols)
	}
	if data.AutoFilterRef != "A1:C3" {
		t.Fatalf("filter=%q", data.AutoFilterRef)
	}
	if len(data.MergedCells) != 1 {
		t.Fatalf("merges=%d", len(data.MergedCells))
	}
	c := findCell(t, data, 1, 1)
	if c.StrValue != "Name" || !c.Style.Bold || c.Style.BgColor == "" {
		t.Fatalf("A1=%#v style=%#v", c, c.Style)
	}
	if data.Comments["A1"] != "header note" {
		t.Fatalf("comment=%q", data.Comments["A1"])
	}
	if findCell(t, data, 1, 2).HyperlinkURL != "https://example.com" {
		t.Fatalf("hyperlink=%q", findCell(t, data, 1, 2).HyperlinkURL)
	}
	f := findCell(t, data, 2, 3)
	if f.Formula != "SUM(1,2)" || f.NumValue != 3 {
		t.Fatalf("formula cell=%#v", f)
	}
}

func TestStreamingWriterWindowAndExplicitFlush(t *testing.T) {
	var out bytes.Buffer
	wb, _ := sheetstream.New(&out, sheetstream.Options{RowWindowSize: 3})
	s, _ := wb.AddSheet("Rows")
	rows := make([]*sheetstream.Row, 0, 5)
	for i := 1; i <= 5; i++ {
		r, _ := s.CreateRow(i)
		_ = r.SetCell(1, sheetstream.Cell{Value: i})
		rows = append(rows, r)
	}
	if err := rows[0].SetCell(2, sheetstream.Cell{Value: "late"}); !errors.Is(err, sheetstream.ErrRowFlushed) {
		t.Fatalf("old row error=%v", err)
	}
	if err := rows[2].SetCell(2, sheetstream.Cell{Value: "still mutable"}); err != nil {
		t.Fatal(err)
	}
	if err := s.FlushRows(1); err != nil {
		t.Fatal(err)
	}
	if err := rows[2].SetCell(3, sheetstream.Cell{Value: "too late"}); !errors.Is(err, sheetstream.ErrRowFlushed) {
		t.Fatalf("flush error=%v", err)
	}
	if err := rows[4].SetCell(2, sheetstream.Cell{Value: "kept"}); err != nil {
		t.Fatal(err)
	}
	if err := wb.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestStreamingWriterUsesInlineStringsAndCleansTempFiles(t *testing.T) {
	var out bytes.Buffer
	wb, _ := sheetstream.New(&out, sheetstream.Options{RowWindowSize: 1})
	s, _ := wb.AddSheet("Data")
	for i := 0; i < 200; i++ {
		_, _ = s.AppendRow(strings.Repeat("x", 100))
	}
	temp := wb.TempFiles()
	if len(temp) == 0 {
		t.Fatal("expected temp files")
	}
	if err := wb.Close(); err != nil {
		t.Fatal(err)
	}
	for _, name := range temp {
		if _, err := osStat(name); err == nil {
			t.Fatalf("temp file still exists: %s", name)
		}
	}
	if bytes.Contains(out.Bytes(), []byte("sharedStrings.xml")) {
		t.Fatal("streaming output unexpectedly contains sharedStrings.xml")
	}
}

func TestStreamingWriterLargeDatasetMemoryBound(t *testing.T) {
	if testing.Short() {
		t.Skip("large streaming regression")
	}
	var out bytes.Buffer
	wb, _ := sheetstream.New(&out, sheetstream.Options{RowWindowSize: 8})
	s, _ := wb.AddSheet("Large")
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	const rows = 20000
	for r := 1; r <= rows; r++ {
		row, err := s.CreateRow(r)
		if err != nil {
			t.Fatal(err)
		}
		for c := 1; c <= 6; c++ {
			if err := row.SetCell(c, sheetstream.Cell{Value: fmt.Sprintf("r%d-c%d", r, c)}); err != nil {
				t.Fatal(err)
			}
		}
	}
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	liveGrowth := int64(after.Alloc) - int64(before.Alloc)
	if liveGrowth > 32<<20 {
		t.Fatalf("live heap grew by %d bytes for %d streamed rows", liveGrowth, rows)
	}
	if err := wb.Close(); err != nil {
		t.Fatal(err)
	}
	if out.Len() == 0 {
		t.Fatal("empty xlsx")
	}
}

func BenchmarkStreamingWriter100kRows(b *testing.B) {
	for n := 0; n < b.N; n++ {
		var out bytes.Buffer
		wb, _ := sheetstream.New(&out, sheetstream.Options{RowWindowSize: 32})
		s, _ := wb.AddSheet("Data")
		for r := 1; r <= 100000; r++ {
			row, _ := s.CreateRow(r)
			_ = row.SetCell(1, sheetstream.Cell{Value: r})
			_ = row.SetCell(2, sheetstream.Cell{Value: "streamed"})
		}
		_ = wb.Close()
	}
}

func resolveBytes(t *testing.T, data []byte) *sheetir.Workbook {
	t.Helper()
	pkg, err := opc.OpenReaderAt(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	sp, err := sheetparts.LoadPackage(pkg)
	if err != nil {
		t.Fatal(err)
	}
	wb, err := sheetresolve.ResolvePackage(sp)
	if err != nil {
		t.Fatal(err)
	}
	return wb
}
func findCell(t *testing.T, s *sheetir.Sheet, row, col int) *sheetir.Cell {
	t.Helper()
	for _, r := range s.Rows {
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
func osStat(name string) (any, error) { return statFile(name) }
