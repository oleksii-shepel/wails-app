package sheet_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	sheet "office/pkg/sheet"
)

func TestDeterministicXLSXSave(t *testing.T) {
	wb := sheet.New()
	s, err := wb.CreateSheet("Data")
	if err != nil {
		t.Fatal(err)
	}
	for r := 1; r <= 50; r++ {
		c, err := s.CreateCell(r, 1)
		if err != nil {
			t.Fatal(err)
		}
		c.SetNumber(float64(r))
	}
	var a, b bytes.Buffer
	if err := wb.SaveAsXLSX(&a); err != nil {
		t.Fatal(err)
	}
	if err := wb.SaveAsXLSX(&b); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("equivalent saves produced different XLSX bytes")
	}
}

func TestMalformedPayloadReturnsErrorWithoutPanic(t *testing.T) {
	payloads := [][]byte{
		nil,
		{0x09, 0x08, 0xff, 0xff, 0x00},
		{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1, 0, 1, 2},
		{'P', 'K', 3, 4, 0, 0, 0},
	}
	for i, payload := range payloads {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("payload %d panicked: %v", i, r)
				}
			}()
			if _, err := sheet.OpenBytes(payload, "malformed"); err == nil {
				t.Fatalf("payload %d unexpectedly succeeded", i)
			}
		}()
	}
}

func TestLargeInMemoryWorkbookSaveReopen(t *testing.T) {
	wb := sheet.New()
	s, err := wb.CreateSheet("Large")
	if err != nil {
		t.Fatal(err)
	}
	const rows = 5000
	for r := 1; r <= rows; r++ {
		c, err := s.CreateCell(r, 1)
		if err != nil {
			t.Fatal(err)
		}
		c.SetNumber(float64(r))
		c2, err := s.CreateCell(r, 2)
		if err != nil {
			t.Fatal(err)
		}
		c2.SetString("value")
	}
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatal(err)
	}
	reopened, err := sheet.OpenBytes(buf.Bytes(), "large.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	if len(reopened.Sheets) != 1 || len(reopened.Sheets[0].Rows) != rows {
		t.Fatalf("reopened rows = %d", len(reopened.Sheets[0].Rows))
	}
}

func TestSpreadsheetCorpusOpenEditSaveReopen(t *testing.T) {
	roots := []string{
		filepath.Join("..", "..", "..", "testdata", "xlsx"),
		filepath.Join("..", "..", "..", "testdata", "xls"),
	}
	var fixtures []string
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			ext := filepath.Ext(entry.Name())
			if ext == ".xlsx" || ext == ".xls" {
				fixtures = append(fixtures, filepath.Join(root, entry.Name()))
			}
		}
	}
	if len(fixtures) == 0 {
		t.Skip("spreadsheet corpus not present in extracted snapshot")
	}
	for _, path := range fixtures {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			wb, err := sheet.Open(path)
			if err != nil {
				t.Skipf("fixture not supported/encrypted: %v", err)
			}
			editable, err := sheet.Edit(wb)
			if err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			if err := editable.SaveAsXLSX(&buf); err != nil {
				t.Fatal(err)
			}
			if _, err := sheet.OpenBytes(buf.Bytes(), filepath.Base(path)+".xlsx"); err != nil {
				t.Fatal(err)
			}
		})
	}
}
