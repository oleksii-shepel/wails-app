package edit

import "testing"

func TestDocumentCreateList(t *testing.T) {
	d := NewDocument()
	list := d.CreateList()
	if list == nil {
		t.Fatal("expected list")
	}
	if list.NumID() == 0 {
		t.Fatal("expected non-zero NumID")
	}

	p1 := list.AddItem()
	if p1 == nil {
		t.Fatal("expected paragraph")
	}
	if !p1.IsListItem() {
		t.Fatal("expected list item")
	}
	if p1.NumID() != list.NumID() {
		t.Fatalf("numID = %d, want %d", p1.NumID(), list.NumID())
	}
	if p1.ListLevel() != 0 {
		t.Fatalf("level = %d, want 0", p1.ListLevel())
	}

	p2 := list.AddItemLevel(1)
	if p2.ListLevel() != 1 {
		t.Fatalf("level = %d, want 1", p2.ListLevel())
	}

	if list.Count() != 2 {
		t.Fatalf("count = %d, want 2", list.Count())
	}
}

func TestListSetLevelStyle(t *testing.T) {
	d := NewDocument()
	list := d.CreateList()
	list.SetLevelStyle(0, "decimal", "%1.")

	p := list.AddItem()
	p.SetText("item")

	pkg, err := d.ToOOXML()
	if err != nil {
		t.Fatalf("ToOOXML: %v", err)
	}
	if pkg.Numbering == nil {
		t.Fatal("expected numbering part")
	}
	found := false
	for _, num := range pkg.Numbering.Nums {
		if num.ID == list.NumID() {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected numbering instance")
	}
}

func TestParagraphSetList(t *testing.T) {
	d := NewDocument()
	list1 := d.CreateList()
	list2 := d.CreateList()

	p := d.AddParagraph()
	p.SetList(list1, 0)
	if p.NumID() != list1.NumID() {
		t.Fatalf("numID = %d, want %d", p.NumID(), list1.NumID())
	}

	p.SetList(list2, 2)
	if p.NumID() != list2.NumID() {
		t.Fatalf("numID = %d, want %d", p.NumID(), list2.NumID())
	}
	if p.ListLevel() != 2 {
		t.Fatalf("level = %d, want 2", p.ListLevel())
	}

	p.SetList(nil, 0)
	if p.IsListItem() {
		t.Fatal("expected not list item")
	}
}

func TestDocumentMultipleLists(t *testing.T) {
	d := NewDocument()
	list1 := d.CreateList()
	list2 := d.CreateList()

	if list1.NumID() == list2.NumID() {
		t.Fatal("expected distinct NumIDs")
	}

	list1.AddItem().SetText("a")
	list2.AddItem().SetText("b")

	if list1.Count() != 1 || list2.Count() != 1 {
		t.Fatalf("counts = %d, %d, want 1, 1", list1.Count(), list2.Count())
	}
}

func TestListParagraphsOrder(t *testing.T) {
	d := NewDocument()
	list := d.CreateList()
	p1 := list.AddItem()
	p1.SetText("first")
	d.AddParagraph().SetText("not in list")
	p2 := list.AddItem()
	p2.SetText("second")

	paras := list.Paragraphs()
	if len(paras) != 2 {
		t.Fatalf("len = %d, want 2", len(paras))
	}
	if paras[0].Text() != "first" {
		t.Fatalf("para[0] = %q", paras[0].Text())
	}
	if paras[1].Text() != "second" {
		t.Fatalf("para[1] = %q", paras[1].Text())
	}
}