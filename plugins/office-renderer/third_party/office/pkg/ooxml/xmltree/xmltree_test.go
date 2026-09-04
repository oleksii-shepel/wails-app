package xmltree_test

import (
	"testing"

	"office/pkg/ooxml/xmltree"
)

const sampleXML = `<root>
  <a>hello</a>
  <b attr="val">
    <c>world</c>
    <c>again</c>
  </b>
  <a>second</a>
</root>`

func mustParse(t *testing.T, data string) *xmltree.Node {
	t.Helper()
	n, err := xmltree.Parse([]byte(data))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return n
}

func TestParse_RootName(t *testing.T) {
	n := mustParse(t, sampleXML)
	if n.Name.Local != "root" {
		t.Errorf("root name = %q, want %q", n.Name.Local, "root")
	}
}

func TestParse_EmptyInput(t *testing.T) {
	_, err := xmltree.Parse([]byte(""))
	if err == nil {
		t.Error("expected error for empty input, got nil")
	}
}

func TestChild_Found(t *testing.T) {
	n := mustParse(t, sampleXML)
	b := n.Child("b")
	if b == nil {
		t.Fatal("Child(\"b\") returned nil")
	}
	if b.Name.Local != "b" {
		t.Errorf("got %q, want %q", b.Name.Local, "b")
	}
}

func TestChild_Missing(t *testing.T) {
	n := mustParse(t, sampleXML)
	if got := n.Child("z"); got != nil {
		t.Errorf("Child(\"z\") = %v, want nil", got)
	}
}

func TestChild_Nil(t *testing.T) {
	var n *xmltree.Node
	if got := n.Child("x"); got != nil {
		t.Errorf("nil.Child = %v, want nil", got)
	}
}

func TestChild_FirstOnly(t *testing.T) {
	// Should return first <a>, not the second.
	n := mustParse(t, sampleXML)
	a := n.Child("a")
	if a == nil {
		t.Fatal("Child(\"a\") returned nil")
	}
	if a.Text != "hello" {
		t.Errorf("first <a> text = %q, want %q", a.Text, "hello")
	}
}

func TestChildrenNamed(t *testing.T) {
	n := mustParse(t, sampleXML)
	as := n.ChildrenNamed("a")
	if len(as) != 2 {
		t.Fatalf("ChildrenNamed(\"a\") = %d results, want 2", len(as))
	}
	if as[0].Text != "hello" || as[1].Text != "second" {
		t.Errorf("texts = %q, %q; want \"hello\", \"second\"", as[0].Text, as[1].Text)
	}
}

func TestChildrenNamed_NilReceiver(t *testing.T) {
	var n *xmltree.Node
	if got := n.ChildrenNamed("a"); got != nil {
		t.Errorf("nil.ChildrenNamed = %v, want nil", got)
	}
}

func TestFindAll(t *testing.T) {
	n := mustParse(t, sampleXML)
	// <c> is nested inside <b>, not a direct child of root.
	cs := n.FindAll("c")
	if len(cs) != 2 {
		t.Fatalf("FindAll(\"c\") = %d results, want 2", len(cs))
	}
}

func TestFindAll_NilReceiver(t *testing.T) {
	var n *xmltree.Node
	if got := n.FindAll("a"); got != nil {
		t.Errorf("nil.FindAll = %v, want nil", got)
	}
}

func TestAttrValue_Found(t *testing.T) {
	n := mustParse(t, sampleXML)
	b := n.Child("b")
	v, ok := b.AttrValue("attr")
	if !ok {
		t.Fatal("AttrValue(\"attr\"): ok = false")
	}
	if v != "val" {
		t.Errorf("attr value = %q, want %q", v, "val")
	}
}

func TestAttrValue_Missing(t *testing.T) {
	n := mustParse(t, sampleXML)
	_, ok := n.AttrValue("nope")
	if ok {
		t.Error("AttrValue(\"nope\"): ok = true, want false")
	}
}

func TestAttrValue_NilReceiver(t *testing.T) {
	var n *xmltree.Node
	_, ok := n.AttrValue("x")
	if ok {
		t.Error("nil.AttrValue: ok = true, want false")
	}
}

func TestAllText_Leaf(t *testing.T) {
	n := mustParse(t, "<x>foo</x>")
	if got := n.AllText(); got != "foo" {
		t.Errorf("AllText = %q, want %q", got, "foo")
	}
}

func TestAllText_Nested(t *testing.T) {
	n := mustParse(t, "<root><a>hello</a><b>world</b></root>")
	got := n.AllText()
	if got != "helloworld" {
		t.Errorf("AllText = %q, want %q", got, "helloworld")
	}
}

func TestAllText_NilReceiver(t *testing.T) {
	var n *xmltree.Node
	if got := n.AllText(); got != "" {
		t.Errorf("nil.AllText = %q, want empty", got)
	}
}
