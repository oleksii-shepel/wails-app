package edit

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestFontTableNames(t *testing.T) {
	d := NewDocument()
	ft := d.FontTable()
	if len(ft.Names()) != 0 {
		t.Fatal("expected empty names")
	}

	d.IR.FontFamilies = map[string]wordir.FontFamilyInfo{
		"Arial":           {Family: "swiss"},
		"Times New Roman": {Family: "roman"},
	}

	names := ft.Names()
	if len(names) != 2 {
		t.Fatalf("len(names) = %d, want 2", len(names))
	}
	if names[0] != "Arial" {
		t.Fatalf("names[0] = %q, want Arial", names[0])
	}
	if names[1] != "Times New Roman" {
		t.Fatalf("names[1] = %q, want Times New Roman", names[1])
	}
}

func TestFontTableGet(t *testing.T) {
	d := NewDocument()
	ft := d.FontTable()

	_, ok := ft.Get("Arial")
	if ok {
		t.Fatal("expected not found")
	}

	d.IR.FontFamilies = map[string]wordir.FontFamilyInfo{
		"Arial": {Family: "swiss", Pitch: "variable"},
	}

	info, ok := ft.Get("Arial")
	if !ok {
		t.Fatal("expected found")
	}
	if info.Family != "swiss" {
		t.Fatalf("family = %q, want swiss", info.Family)
	}
	if info.Pitch != "variable" {
		t.Fatalf("pitch = %q, want variable", info.Pitch)
	}
}

func TestFontTableAdd(t *testing.T) {
	d := NewDocument()
	ft := d.FontTable()
	ft.Add("Courier New", wordir.FontFamilyInfo{Family: "modern", Pitch: "fixed"})

	if ft.Count() != 1 {
		t.Fatalf("count = %d, want 1", ft.Count())
	}
	info, ok := ft.Get("Courier New")
	if !ok {
		t.Fatal("expected found")
	}
	if info.Family != "modern" {
		t.Fatalf("family = %q, want modern", info.Family)
	}
}

func TestFontTableRemove(t *testing.T) {
	d := NewDocument()
	ft := d.FontTable()
	ft.Add("Arial", wordir.FontFamilyInfo{Family: "swiss"})
	ft.Remove("Arial")
	if ft.Count() != 0 {
		t.Fatalf("count = %d, want 0", ft.Count())
	}
}

func TestFontTableClear(t *testing.T) {
	d := NewDocument()
	ft := d.FontTable()
	ft.Add("Arial", wordir.FontFamilyInfo{})
	ft.Add("Times New Roman", wordir.FontFamilyInfo{})
	ft.Clear()
	if ft.Count() != 0 {
		t.Fatalf("count = %d, want 0", ft.Count())
	}
}

func TestFontTableNilSafety(t *testing.T) {
	var ft *FontTable
	if ft.Count() != 0 {
		t.Fatal("expected 0")
	}
	if ft.Names() != nil {
		t.Fatal("expected nil")
	}
	ft.Add("x", wordir.FontFamilyInfo{})
	ft.Remove("x")
	ft.Clear()
}

func TestFontTableRoundTrip(t *testing.T) {
	d := NewDocument()
	ft := d.FontTable()
	ft.Add("Arial", wordir.FontFamilyInfo{Family: "swiss", Pitch: "variable", Charset: "00", Panose1: "020B0604020202020204"})
	ft.Add("Times New Roman", wordir.FontFamilyInfo{Family: "roman", Pitch: "variable"})

	pkg, err := d.ToOOXML()
	if err != nil {
		t.Fatalf("ToOOXML: %v", err)
	}
	if pkg.FontTable == nil {
		t.Fatal("expected fontTable in package")
	}
	if len(pkg.FontTable.Fonts) < 2 {
		t.Fatalf("fonts = %d, want >= 2", len(pkg.FontTable.Fonts))
	}
}