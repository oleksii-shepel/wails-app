package edit

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestTableCellContentControl(t *testing.T) {
	d := NewDocument()
	tbl := d.AddTable()
	row := tbl.AddRow()
	cell := row.AddCell()

	cc := &wordir.ContentControl{ID: 1, Tag: "cell-tag", Type: "richText"}
	cell.SetContentControl(cc)

	got := cell.ContentControl()
	if got == nil {
		t.Fatal("expected content control")
	}
	if got.ID != 1 || got.Tag != "cell-tag" {
		t.Fatalf("cc = %+v", got)
	}

	// Remove
	cell.SetContentControl(nil)
	if cell.ContentControl() != nil {
		t.Fatal("expected nil after removal")
	}
}