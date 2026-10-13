package edit

import (
	"testing"

	presir "office/pkg/presentation/ir"
)

func TestSlideFindShape(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	s.AddTextBox(0, 0, 1000, 1000).SetName("Alpha")
	s.AddTextBox(0, 0, 1000, 1000).SetName("Beta")

	found := s.FindShape("Alpha")
	if found == nil || found.Name() != "Alpha" {
		t.Fatalf("expected to find Alpha, got %v", found)
	}
	if s.FindShape("Gamma") != nil {
		t.Fatal("expected no Gamma")
	}
}

func TestSlideFindShapeRecursive(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	grp := s.AddGroup(0, 0, 1000, 1000)
	child := grp.AddTextBox(0, 0, 100, 100)
	child.SetName("Nested")

	if s.FindShape("Nested") != nil {
		t.Fatal("expected direct lookup to miss nested shape")
	}
	found := s.FindShapeRecursive("Nested")
	if found == nil || found.Name() != "Nested" {
		t.Fatalf("expected recursive find to locate Nested, got %v", found)
	}
}

func TestSlideFindShapeByID(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	shape := s.AddTextBox(0, 0, 1000, 1000)
	shape.SetName("WithID")
	shape.SetID(42)

	found := s.FindShapeByID(42)
	if found == nil || found.Name() != "WithID" {
		t.Fatalf("expected find by ID, got %v", found)
	}
	if s.FindShapeByID(99) != nil {
		t.Fatal("expected no shape with ID 99")
	}
}

func TestSlideFindShapesByKind(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	s.AddTextBox(0, 0, 1000, 1000)
	s.AddImage(minimalPNG(), "image/png", 0, 0, 1000, 1000)
	s.AddTable(1, 1, 0, 0, 1000, 1000)

	images := s.FindShapesByKind(presir.ShapeImage)
	if len(images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(images))
	}
	tables := s.FindShapesByKindRecursive(presir.ShapeTable)
	if len(tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(tables))
	}
}

func TestSlideRemoveShape(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	a := s.AddTextBox(0, 0, 1000, 1000)
	a.SetName("A")
	b := s.AddTextBox(0, 0, 1000, 1000)
	b.SetName("B")

	if !s.RemoveShape(a) {
		t.Fatal("expected RemoveShape to return true")
	}
	if len(s.Shapes()) != 1 {
		t.Fatalf("expected 1 shape after remove, got %d", len(s.Shapes()))
	}
	if s.FindShape("A") != nil {
		t.Fatal("expected A to be removed")
	}
	if s.RemoveShape(a) {
		t.Fatal("expected double remove to return false")
	}
}

func TestSlideRemoveShapeByName(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	s.AddTextBox(0, 0, 1000, 1000).SetName("A")

	if !s.RemoveShapeByName("A") {
		t.Fatal("expected removal")
	}
	if s.RemoveShapeByName("A") {
		t.Fatal("expected no second removal")
	}
}

func TestSlideClearShapes(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	s.AddTextBox(0, 0, 1000, 1000)
	s.AddImage(minimalPNG(), "image/png", 0, 0, 1000, 1000)
	s.ClearShapes()
	if len(s.Shapes()) != 0 {
		t.Fatalf("expected 0 shapes, got %d", len(s.Shapes()))
	}
}

func TestGroupRemoveAndFindShape(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	grp := s.AddGroup(0, 0, 1000, 1000)
	a := grp.AddTextBox(0, 0, 100, 100)
	a.SetName("ChildA")
	b := grp.AddTextBox(0, 0, 100, 100)
	b.SetName("ChildB")

	if grp.FindShape("ChildA") == nil {
		t.Fatal("expected ChildA")
	}
	if !grp.RemoveShape(a) {
		t.Fatal("expected remove ChildA")
	}
	if grp.FindShape("ChildA") != nil {
		t.Fatal("expected ChildA gone")
	}
	if len(grp.Shapes()) != 1 {
		t.Fatalf("expected 1 child, got %d", len(grp.Shapes()))
	}
}

func TestShapeOrdering(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	a := s.AddTextBox(0, 0, 1000, 1000)
	a.SetName("A")
	b := s.AddTextBox(0, 0, 1000, 1000)
	b.SetName("B")
	c := s.AddTextBox(0, 0, 1000, 1000)
	c.SetName("C")

	s.BringToFront(a)
	if s.Shapes()[2].Name() != "A" {
		t.Fatalf("expected A at front, got %s", s.Shapes()[2].Name())
	}

	s.SendToBack(a)
	if s.Shapes()[0].Name() != "A" {
		t.Fatalf("expected A at back, got %s", s.Shapes()[0].Name())
	}

	s.MoveShape(1, c)
	if s.Shapes()[1].Name() != "C" {
		t.Fatalf("expected C at index 1, got %s", s.Shapes()[1].Name())
	}
}

func TestShapeClone(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	shape := s.AddTextBox(0, 0, 1000, 1000)
	shape.SetName("Original")
	shape.SetFillSolid("FF0000")
	shape.SetRotation(45)
	para := shape.AddParagraph()
	para.AddRun("hello").SetBold(true)

	clone := shape.Clone()
	if clone == nil {
		t.Fatal("expected clone")
	}
	if clone.ir == shape.ir {
		t.Fatal("clone must not share IR pointer")
	}
	if clone.Name() != "Original" {
		t.Fatalf("name = %q", clone.Name())
	}
	if clone.FillColor() != "FF0000" || clone.Rotation() != 45 {
		t.Fatal("clone lost properties")
	}
	if clone.Text() != "hello" {
		t.Fatalf("clone text = %q", clone.Text())
	}

	// Mutating clone must not affect original.
	clone.SetFillSolid("00FF00")
	clone.AddParagraph().AddRun(" world")
	if shape.FillColor() == "00FF00" {
		t.Fatal("original was mutated by clone edit")
	}
	if shape.Text() != "hello" {
		t.Fatalf("original text mutated: %q", shape.Text())
	}
}

func TestSlideDuplicateShape(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	shape := s.AddTextBox(0, 0, 1000, 1000)
	shape.SetName("Original")
	shape.SetText("data")

	clone := s.DuplicateShape(shape)
	if clone == nil {
		t.Fatal("expected duplicate")
	}
	if clone.ir == shape.ir {
		t.Fatal("duplicate must not share IR pointer")
	}
	if clone.Name() == "Original" {
		t.Fatalf("expected unique name, got %q", clone.Name())
	}
	if clone.Text() != "data" {
		t.Fatalf("duplicate text = %q", clone.Text())
	}
	if len(s.Shapes()) != 2 {
		t.Fatalf("expected 2 shapes, got %d", len(s.Shapes()))
	}
}

func TestPresentationDuplicateSlide(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	s.SetName("Original")
	shape := s.AddTextBox(0, 0, 1000, 1000)
	shape.SetText("hello")

	clone := p.DuplicateSlide(0)
	if clone == nil {
		t.Fatal("expected duplicated slide")
	}
	if p.SlideCount() != 2 {
		t.Fatalf("expected 2 slides, got %d", p.SlideCount())
	}
	if clone.Index() != 1 {
		t.Fatalf("expected clone at index 1, got %d", clone.Index())
	}
	if clone.Name() != "Original" {
		t.Fatalf("name = %q", clone.Name())
	}
	if clone.Shapes()[0].Text() != "hello" {
		t.Fatalf("clone shape text = %q", clone.Shapes()[0].Text())
	}

	// Ensure deep copy.
	clone.Shapes()[0].SetText("modified")
	if p.Slide(0).Shapes()[0].Text() != "hello" {
		t.Fatal("original slide was mutated")
	}
}

func TestShapeID(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	shape := s.AddTextBox(0, 0, 1000, 1000)
	shape.SetID(123)
	if shape.ID() != 123 {
		t.Fatalf("id = %d", shape.ID())
	}
}

func TestCloneSlide(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	s.SetName("Source")
	s.AddTextBox(0, 0, 1000, 1000).SetText("content")

	clonedIR := s.CloneSlide()
	if clonedIR == nil {
		t.Fatal("expected cloned IR")
	}
	if clonedIR == s.ir {
		t.Fatal("cloned IR must be a new pointer")
	}
	if clonedIR.SectionName != "Source" || len(clonedIR.Shapes) != 1 {
		t.Fatal("cloned slide content mismatch")
	}
}

func TestUniqueShapeName(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	s.AddTextBox(0, 0, 1000, 1000).SetName("Box")
	s.AddTextBox(0, 0, 1000, 1000).SetName("Box Copy 1")

	third := s.AddTextBox(0, 0, 1000, 1000)
	third.SetName(uniqueShapeName(s.ir.Shapes, "Box"))
	if third.Name() != "Box Copy 2" {
		t.Fatalf("expected Box Copy 2, got %q", third.Name())
	}
}

func TestGroupOrdering(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	grp := s.AddGroup(0, 0, 1000, 1000)
	a := grp.AddTextBox(0, 0, 100, 100)
	a.SetName("A")
	b := grp.AddTextBox(0, 0, 100, 100)
	b.SetName("B")

	grp.BringToFront(a)
	if grp.Shapes()[1].Name() != "A" {
		t.Fatalf("expected A at front, got %s", grp.Shapes()[1].Name())
	}
	grp.SendToBack(a)
	if grp.Shapes()[0].Name() != "A" {
		t.Fatalf("expected A at back, got %s", grp.Shapes()[0].Name())
	}
}
