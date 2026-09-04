package edit

import (
	"bytes"
	"strings"
	"testing"
)

func TestAddSmartArtAndRoundTrip(t *testing.T) {
	p := NewPresentation()
	slide := p.AddSlide("blank")
	art := NewSmartArt("hierarchy", 1000000, 500000, 8000000, 4000000)
	root := art.AddRoot("CEO")
	root.AddChild("Engineering")
	root.AddChild("Sales")
	slide.AddSmartArt(art)

	if len(slide.ir.SmartArt) != 1 {
		t.Fatalf("got %d smart art objects, want 1", len(slide.ir.SmartArt))
	}
	if slide.ir.SmartArt[0].Type != "hierarchy" {
		t.Errorf("type = %q, want hierarchy", slide.ir.SmartArt[0].Type)
	}
	if len(slide.ir.SmartArt[0].Nodes) != 3 {
		t.Fatalf("got %d nodes, want 3", len(slide.ir.SmartArt[0].Nodes))
	}
	if len(slide.ir.SmartArt[0].Connections) != 2 {
		t.Fatalf("got %d connections, want 2", len(slide.ir.SmartArt[0].Connections))
	}

	var buf bytes.Buffer
	if err := p.SaveAsPPTX(&buf); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := LoadFromPPTX(&buf)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if loaded.SlideCount() != 1 {
		t.Fatalf("got %d slides, want 1", loaded.SlideCount())
	}
	ls := loaded.Slide(0)
	if ls == nil {
		t.Fatal("loaded slide nil")
	}
	var texts []string
	for _, sh := range ls.Shapes() {
		texts = append(texts, sh.Text())
	}
	joined := strings.Join(texts, "|")
	for _, want := range []string{"CEO", "Engineering", "Sales"} {
		if !strings.Contains(joined, want) {
			t.Errorf("round-trip output missing %q: %s", want, joined)
		}
	}
}

func TestSmartArtNodeHelpers(t *testing.T) {
	art := NewSmartArt("process", 0, 0, 1000000, 1000000)
	if art.Type() != "process" {
		t.Errorf("type = %q, want process", art.Type())
	}
	a := art.AddNode(nil, "A")
	b := a.AddChild("B")
	c := art.AddNode(a, "C")
	if a.Type() != "root" {
		t.Errorf("root type = %q, want root", a.Type())
	}
	if b.Type() != "node" {
		t.Errorf("child type = %q, want node", b.Type())
	}
	if c.Text() != "C" {
		t.Errorf("c text = %q, want C", c.Text())
	}
	if len(art.ir.Connections) != 2 {
		t.Fatalf("connections = %d, want 2", len(art.ir.Connections))
	}
}

func TestSmartArtEditingHelpers(t *testing.T) {
	art := NewSmartArt("hierarchy", 0, 0, 1000000, 1000000)
	art.SetLayout("orgChart")
	art.SetQuickStyle("simpleFill")
	art.SetColorStyle("accent1")
	art.SetColors("#112233", "#445566", "#FFFFFF")
	root := art.AddRoot("Root")
	child := root.AddChild("Child")
	other := root.AddChild("Other")
	child.SetFillColor("#ABCDEF")
	child.SetTextColor("#102030")

	if art.Layout() != "orgChart" {
		t.Fatalf("layout = %q, want orgChart", art.Layout())
	}
	if got := art.NodeByID(child.ID()); got == nil || got.Text() != "Child" {
		t.Fatalf("NodeByID returned %+v", got)
	}
	if !art.MoveNode(2, 1) {
		t.Fatal("MoveNode returned false")
	}
	if art.Nodes()[1].ID() != other.ID() {
		t.Fatalf("node order after move = %s, want %s", art.Nodes()[1].ID(), other.ID())
	}
	if !art.RemoveNode(child) {
		t.Fatal("RemoveNode returned false")
	}
	if art.NodeByID(child.ID()) != nil {
		t.Fatal("removed node still found")
	}
	for _, cxn := range art.ir.Connections {
		if cxn.SrcID == child.ID() || cxn.DestID == child.ID() {
			t.Fatalf("connection still references removed node: %+v", cxn)
		}
	}
}

func TestLoadFromPPTXExposesEditableSmartArt(t *testing.T) {
	p := NewPresentation()
	slide := p.AddSlide("blank")
	art := NewSmartArt("hierarchy", 1000000, 500000, 8000000, 4000000)
	root := art.AddRoot("CEO")
	root.AddChild("Engineering")
	slide.AddSmartArt(art)

	var buf bytes.Buffer
	if err := p.SaveAsPPTX(&buf); err != nil {
		t.Fatalf("SaveAsPPTX: %v", err)
	}
	loaded, err := LoadFromPPTX(&buf)
	if err != nil {
		t.Fatalf("LoadFromPPTX: %v", err)
	}
	loadedSlide := loaded.Slide(0)
	if loadedSlide == nil {
		t.Fatal("loaded slide nil")
	}
	smartArts := loadedSlide.SmartArts()
	if len(smartArts) != 1 {
		t.Fatalf("SmartArts = %d, want 1", len(smartArts))
	}
	if len(smartArts[0].Nodes()) != 2 {
		t.Fatalf("nodes = %d, want 2", len(smartArts[0].Nodes()))
	}
	if node := smartArts[0].NodeByID("pt1"); node == nil || node.Text() != "CEO" {
		t.Fatalf("loaded root = %+v", node)
	}
}