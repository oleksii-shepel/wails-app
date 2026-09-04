package edit_test

import (
	"bytes"
	"testing"

	"office/pkg/sheet/edit"
	sheetir "office/pkg/sheet/ir"
)

func TestGenericVMLObjectEditRoundTrip(t *testing.T) {
	wb := edit.NewWorkbook()
	sheet, err := wb.CreateSheet("VML")
	if err != nil {
		t.Fatal(err)
	}
	obj, err := sheet.AddVMLObject("CustomShape", "#custom", "position:absolute;width:90pt;height:30pt")
	if err != nil {
		t.Fatal(err)
	}
	obj.FillColor = "#ffee00"
	obj.StrokeColor = "#223344"
	obj.Text = "Editable VML"
	obj.Anchor = "1, 0, 1, 0, 4, 0, 3, 0"
	if err := sheet.SetVMLObject(obj); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatal(err)
	}
	reopened, err := edit.OpenXLSX(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	rs, err := reopened.Sheet(0)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := rs.VMLObject("CustomShape")
	if !ok {
		t.Fatal("VML object missing after round-trip")
	}
	want := &sheetir.VMLObject{ID: "CustomShape", Type: "#custom", Style: "position:absolute;width:90pt;height:30pt", FillColor: "#ffee00", StrokeColor: "#223344", Text: "Editable VML", Anchor: "1, 0, 1, 0, 4, 0, 3, 0"}
	if *got != *want {
		t.Fatalf("got %#v want %#v", got, want)
	}
	if !rs.RemoveVMLObject("CustomShape") {
		t.Fatal("remove VML object")
	}
	var second bytes.Buffer
	if err := reopened.SaveAsXLSX(&second); err != nil {
		t.Fatal(err)
	}
	reopened2, err := edit.OpenXLSX(bytes.NewReader(second.Bytes()), int64(second.Len()))
	if err != nil {
		t.Fatal(err)
	}
	rs2, _ := reopened2.Sheet(0)
	if _, ok := rs2.VMLObject("CustomShape"); ok {
		t.Fatal("removed VML object returned")
	}
}
