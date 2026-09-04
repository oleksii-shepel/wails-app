package edit_test

import (
	"bytes"
	"testing"

	"office/pkg/sheet/edit"
	sheetir "office/pkg/sheet/ir"
)

func TestWorkbookConnectionEditRoundTrip(t *testing.T) {
	wb := edit.NewWorkbook()
	if _, err := wb.CreateSheet("Data"); err != nil {
		t.Fatal(err)
	}
	connection, err := wb.SetConnection(sheetir.WorkbookConnection{
		Name:          "Warehouse",
		Kind:          "dbPr",
		Description:   "analytics",
		RefreshOnLoad: true,
		Connection:    "Provider=Example",
		Command:       "select * from sales",
		CommandType:   "2",
	})
	if err != nil {
		t.Fatalf("SetConnection: %v", err)
	}
	if connection.ID != 1 {
		t.Fatalf("id = %d, want 1", connection.ID)
	}
	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}
	roundTrip := resolveWorkbook(t, buf.Bytes())
	if len(roundTrip.Connections) != 1 || roundTrip.Connections[0].Name != "Warehouse" || !roundTrip.Connections[0].RefreshOnLoad {
		t.Fatalf("connections = %#v", roundTrip.Connections)
	}
	if !wb.RemoveConnection(connection.ID, "") || len(wb.Connections()) != 0 {
		t.Fatal("expected connection removal")
	}
}
