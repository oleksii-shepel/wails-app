package edit_test

import (
	"bytes"
	"strings"
	"testing"

	"office/pkg/ooxml/opc"
	"office/pkg/sheet/edit"
	sheetir "office/pkg/sheet/ir"
)

func TestSmartArtSemanticEditRoundTrip(t *testing.T) {
	wb := edit.NewWorkbook()
	sheet, err := wb.CreateSheet("Data")
	if err != nil {
		t.Fatalf("CreateSheet: %v", err)
	}
	art, err := sheet.AddSmartArt("Org", "hierarchy", sheetir.DrawingPosition{
		Anchor:   sheetir.AnchorOneCell,
		FromCell: sheetir.DrawingCellAnchor{Row: 2, Col: 2},
		Width:    4_000_000,
		Height:   2_500_000,
	})
	if err != nil {
		t.Fatalf("AddSmartArt: %v", err)
	}
	root := art.AddRoot("CEO")
	root.SetFillColor("4472C4")
	root.SetTextColor("FFFFFF")
	child := root.AddChild("Engineering")
	child.SetType("assistant")
	if len(art.Nodes()) != 2 {
		t.Fatalf("nodes = %d, want 2", len(art.Nodes()))
	}

	var first bytes.Buffer
	if err := wb.SaveAsXLSX(&first); err != nil {
		t.Fatalf("SaveAsXLSX first: %v", err)
	}

	opened, err := edit.OpenXLSX(bytes.NewReader(first.Bytes()), int64(first.Len()))
	if err != nil {
		t.Fatalf("OpenXLSX: %v", err)
	}
	openedSheet, err := opened.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet: %v", err)
	}
	arts := openedSheet.SmartArts()
	if len(arts) != 1 {
		t.Fatalf("SmartArts = %d, want 1", len(arts))
	}
	loaded := arts[0]
	if got := len(loaded.Nodes()); got != 2 {
		t.Fatalf("loaded nodes = %d, want 2", got)
	}
	loadedRoot := loaded.NodeByID(root.ID())
	if loadedRoot == nil || loadedRoot.Text() != "CEO" {
		t.Fatalf("loaded root = %#v", loadedRoot)
	}
	loadedRoot.SetText("Chief Executive")
	loadedRoot.AddChild("Finance")

	var second bytes.Buffer
	if err := opened.SaveAsXLSX(&second); err != nil {
		t.Fatalf("SaveAsXLSX second: %v", err)
	}
	resolved := resolveWorkbook(t, second.Bytes())
	if len(resolved.Sheets) != 1 || len(resolved.Sheets[0].Drawings) != 1 {
		t.Fatalf("round-trip drawings = %#v", resolved.Sheets)
	}
	gotArt := resolved.Sheets[0].Drawings[0].SmartArt
	if gotArt == nil {
		t.Fatal("expected SmartArt")
	}
	if len(gotArt.Nodes) != 3 {
		t.Fatalf("nodes = %d, want 3", len(gotArt.Nodes))
	}
	var rootText string
	for _, n := range gotArt.Nodes {
		if n != nil && n.ID == root.ID() {
			rootText = n.Text
		}
	}
	if rootText != "Chief Executive" {
		t.Fatalf("root text = %q", rootText)
	}
	if len(gotArt.Connections) != 2 {
		t.Fatalf("connections = %d, want 2", len(gotArt.Connections))
	}
	opcPkg, err := opc.OpenBytes(second.Bytes())
	if err != nil {
		t.Fatalf("open saved package: %v", err)
	}
	if relsDoc, ok, err := opcPkg.Relationships("xl/diagrams/data1.xml"); err != nil {
		t.Fatalf("data model relationships: %v", err)
	} else if ok {
		for _, rel := range relsDoc.Relationships {
			if strings.Contains(strings.ToLower(rel.Type), "diagramdrawing") {
				t.Fatalf("stale generated drawing relationship survived semantic edit: %#v", rel)
			}
		}
	}
}

func TestSmartArtRemoveNodeCleansConnections(t *testing.T) {
	wb := edit.NewWorkbook()
	sheet, _ := wb.CreateSheet("Data")
	art, err := sheet.AddSmartArt("Process", "process", sheetir.DrawingPosition{Anchor: sheetir.AnchorAbsolute, Width: 1_000_000, Height: 1_000_000})
	if err != nil {
		t.Fatalf("AddSmartArt: %v", err)
	}
	root := art.AddRoot("Start")
	child := root.AddChild("Middle")
	child.AddChild("End")
	if !art.RemoveNode(child) {
		t.Fatal("RemoveNode returned false")
	}
	if len(art.Nodes()) != 2 {
		t.Fatalf("nodes = %d, want 2", len(art.Nodes()))
	}
	if art.NodeByID(child.ID()) != nil {
		t.Fatal("removed node still present")
	}
}
