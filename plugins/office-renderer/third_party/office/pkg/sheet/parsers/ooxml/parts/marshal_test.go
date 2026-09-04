package parts

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"
)

func encodeElement(v interface{}, name string) ([]byte, error) {
	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	start := xml.StartElement{Name: xml.Name{Local: name}}
	if err := enc.EncodeElement(v, start); err != nil {
		return nil, err
	}
	if err := enc.Flush(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func TestMarshalCellSharedString(t *testing.T) {
	c := Cell{Ref: "A1", Type: "s", Value: "42"}
	b, err := encodeElement(c, "c")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, `<c r="A1" t="s">`) {
		t.Fatalf("expected opening tag, got %s", s)
	}
	if !strings.Contains(s, `<v>42</v>`) {
		t.Fatalf("expected <v>42</v>, got %s", s)
	}
	if strings.Contains(s, `<is>`) {
		t.Fatalf("unexpected <is> in shared string cell: %s", s)
	}
}

func TestMarshalCellNumber(t *testing.T) {
	c := Cell{Ref: "B2", Value: "3.14"}
	b, err := encodeElement(c, "c")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, `<c r="B2">`) {
		t.Fatalf("expected opening tag, got %s", s)
	}
	if !strings.Contains(s, `<v>3.14</v>`) {
		t.Fatalf("expected <v>3.14</v>, got %s", s)
	}
}

func TestMarshalCellInlineString(t *testing.T) {
	c := Cell{Ref: "C3", Type: "inlineStr", InlineStr: &RichText{PlainText: "hello"}}
	b, err := encodeElement(c, "c")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, `<c r="C3" t="inlineStr">`) {
		t.Fatalf("expected opening tag, got %s", s)
	}
	if !strings.Contains(s, `<is>`) {
		t.Fatalf("expected <is>, got %s", s)
	}
	if strings.Contains(s, `<v>`) {
		t.Fatalf("unexpected <v> in inlineStr cell: %s", s)
	}
}

func TestMarshalCellFormula(t *testing.T) {
	c := Cell{Ref: "D4", Formula: &Formula{Text: "=SUM(A1:A3)"}, Value: "6"}
	b, err := encodeElement(c, "c")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, `<f>=SUM(A1:A3)</f>`) {
		t.Fatalf("expected formula, got %s", s)
	}
	if !strings.Contains(s, `<v>6</v>`) {
		t.Fatalf("expected cached value, got %s", s)
	}
}

func TestMarshalRichTextPlain(t *testing.T) {
	rt := RichText{PlainText: "hello"}
	b, err := encodeElement(rt, "si")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, `<si><t>hello</t></si>`) {
		t.Fatalf("expected plain text, got %s", s)
	}
	if strings.Contains(s, `<r>`) {
		t.Fatalf("unexpected <r> in plain text: %s", s)
	}
}

func TestMarshalRichTextRuns(t *testing.T) {
	rt := RichText{
		Runs: []RTRun{
			{Text: "bold", Properties: &RTProps{Bold: &struct{}{}}},
			{Text: " normal"},
		},
	}
	b, err := encodeElement(rt, "si")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	if strings.Contains(s, `<t>bold normal</t>`) {
		t.Fatalf("expected runs, not bare <t>: %s", s)
	}
	if !strings.Contains(s, `<r>`) {
		t.Fatalf("expected <r> runs, got %s", s)
	}
	if !strings.Contains(s, `<t>bold</t>`) {
		t.Fatalf("expected first run text, got %s", s)
	}
	if !strings.Contains(s, `<t xml:space="preserve"> normal</t>`) {
		t.Fatalf("expected second run text with leading space, got %s", s)
	}
	if !strings.Contains(s, `<rPr><b></b></rPr>`) {
		t.Fatalf("expected bold property, got %s", s)
	}
}

func TestMarshalSharedStrings(t *testing.T) {
	ss := SharedStrings{
		Count:       2,
		UniqueCount: 2,
		Items: []RichText{
			{PlainText: "hello"},
			{Runs: []RTRun{{Text: "world"}}},
		},
	}
	b, err := xml.Marshal(ss)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, `<sst count="2" uniqueCount="2">`) {
		t.Fatalf("expected sst header, got %s", s)
	}
	if !strings.Contains(s, `<si><t>hello</t></si>`) {
		t.Fatalf("expected first si, got %s", s)
	}
	if !strings.Contains(s, `<si><r><t>world</t></r></si>`) {
		t.Fatalf("expected second si with run, got %s", s)
	}
}
