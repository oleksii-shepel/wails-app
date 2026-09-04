package edit

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestLatentStylesNames(t *testing.T) {
	d := NewDocument()
	ls := d.LatentStyles()
	if len(ls.Names()) != 0 {
		t.Fatal("expected empty")
	}

	d.IR.LatentStyles = &wordir.LatentStyles{
		Exceptions: []wordir.LatentStyle{
			{Name: "Heading1"},
			{Name: "Normal"},
		},
	}
	names := ls.Names()
	if len(names) != 2 {
		t.Fatalf("len = %d, want 2", len(names))
	}
	if names[0] != "Heading1" || names[1] != "Normal" {
		t.Fatalf("names = %v", names)
	}
}

func TestLatentStylesGet(t *testing.T) {
	d := NewDocument()
	ls := d.LatentStyles()
	_, ok := ls.Get("Heading1")
	if ok {
		t.Fatal("expected not found")
	}

	d.IR.LatentStyles = &wordir.LatentStyles{
		Exceptions: []wordir.LatentStyle{
			{Name: "Heading1", UIPriority: 1, Locked: true},
		},
	}
	exc, ok := ls.Get("Heading1")
	if !ok {
		t.Fatal("expected found")
	}
	if exc.UIPriority != 1 {
		t.Fatalf("priority = %d, want 1", exc.UIPriority)
	}
	if !exc.Locked {
		t.Fatal("expected locked")
	}
}

func TestLatentStylesAdd(t *testing.T) {
	d := NewDocument()
	ls := d.LatentStyles()
	ls.Add(wordir.LatentStyle{Name: "Heading1", UIPriority: 1})
	if ls.Count() != 1 {
		t.Fatalf("count = %d, want 1", ls.Count())
	}
	// Replace
	ls.Add(wordir.LatentStyle{Name: "Heading1", UIPriority: 2})
	exc, _ := ls.Get("Heading1")
	if exc.UIPriority != 2 {
		t.Fatalf("priority = %d, want 2", exc.UIPriority)
	}
	if ls.Count() != 1 {
		t.Fatalf("count = %d, want 1", ls.Count())
	}
}

func TestLatentStylesRemove(t *testing.T) {
	d := NewDocument()
	ls := d.LatentStyles()
	ls.Add(wordir.LatentStyle{Name: "A"})
	ls.Add(wordir.LatentStyle{Name: "B"})
	ls.Remove("A")
	if ls.Count() != 1 {
		t.Fatalf("count = %d, want 1", ls.Count())
	}
}

func TestLatentStylesClear(t *testing.T) {
	d := NewDocument()
	ls := d.LatentStyles()
	ls.Add(wordir.LatentStyle{Name: "A"})
	ls.Clear()
	if ls.Count() != 0 {
		t.Fatalf("count = %d, want 0", ls.Count())
	}
}

func TestLatentStylesNilSafety(t *testing.T) {
	var ls *LatentStyles
	if ls.Count() != 0 {
		t.Fatal("expected 0")
	}
	if ls.Names() != nil {
		t.Fatal("expected nil")
	}
	ls.Add(wordir.LatentStyle{Name: "x"})
	ls.Remove("x")
	ls.Clear()
}

func TestLatentStylesRoundTrip(t *testing.T) {
	d := NewDocument()
	ls := d.LatentStyles()
	ls.Add(wordir.LatentStyle{Name: "Heading1", UIPriority: 1, Locked: true, SemiHidden: true})
	ls.Add(wordir.LatentStyle{Name: "Normal", QFormat: true})

	pkg, err := d.ToOOXML()
	if err != nil {
		t.Fatalf("ToOOXML: %v", err)
	}
	if pkg.Styles == nil || pkg.Styles.LatentStyles == nil {
		t.Fatal("expected latentStyles in package")
	}
	if len(pkg.Styles.LatentStyles.Exceptions) != 2 {
		t.Fatalf("exceptions = %d, want 2", len(pkg.Styles.LatentStyles.Exceptions))
	}
}