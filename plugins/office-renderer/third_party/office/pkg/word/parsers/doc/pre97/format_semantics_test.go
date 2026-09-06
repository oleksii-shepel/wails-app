package pre97

import "testing"

func TestDecodeCHPXGrpprl(t *testing.T) {
	format := DecodeCHPXGrpprl([]byte{
		85, 1,
		86, 1,
		93, 1, 0,
		94, 3,
		99, 24, 0,
		98, 6,
		104, 1,
	}, []string{"Times New Roman", "Arial"})
	if format.Bold == nil || !*format.Bold || format.Italic == nil || !*format.Italic {
		t.Fatalf("bold/italic not decoded: %#v", format)
	}
	if format.FontASCII != "Arial" || format.FontHAnsi != "Arial" {
		t.Fatalf("font = %q/%q, want Arial", format.FontASCII, format.FontHAnsi)
	}
	if format.Underline != "double" || format.FontSize == nil || *format.FontSize != 24 || format.Color != "FF0000" || format.VertAlignVal != "superscript" {
		t.Fatalf("character semantics = %#v", format)
	}
}

func TestDecodePAPXGrpprl(t *testing.T) {
	grpprl := []byte{
		5, 2,
		17, 0xD0, 0x02,
		19, 0xD0, 0xFD,
		13, 2,
		12, 12, 4, 0, 0, 0, 0, 0, 0, 0, 0, 0, 3, 0,
		24, 1,
		25, 1,
	}
	format, meta := DecodePAPXGrpprl(grpprl)
	if format.Alignment != "right" || format.IndentLeft == nil || *format.IndentLeft != 720 {
		t.Fatalf("paragraph geometry = %#v", format)
	}
	if format.IndentHanging == nil || *format.IndentHanging != 560 {
		t.Fatalf("hanging = %#v", format.IndentHanging)
	}
	if format.BinaryListLevel == nil || *format.BinaryListLevel != 2 || format.BinaryListFormat == nil || *format.BinaryListFormat != 4 {
		t.Fatalf("list semantics = %#v", format)
	}
	if meta.ListStart == nil || *meta.ListStart != 3 || !meta.InTable || !meta.RowMark || !format.InTable {
		t.Fatalf("metadata = %#v format=%#v", meta, format)
	}
}

func TestDecodePAPXGrpprlTabs(t *testing.T) {
	// ChgTabs: operand begins with cb=8, then zero deletes and two additions.
	format, _ := DecodePAPXGrpprl([]byte{23, 8, 0, 2, 0xD0, 0x02, 0xA0, 0x05, 0x08, 0x02})
	if len(format.TabStops) != 2 {
		t.Fatalf("tabs = %#v", format.TabStops)
	}
	if format.TabStops[0].Pos != 720 || format.TabStops[0].Algn != "left" || format.TabStops[0].Leader != "dot" {
		t.Fatalf("first tab = %#v", format.TabStops[0])
	}
	if format.TabStops[1].Pos != 1440 || format.TabStops[1].Algn != "right" {
		t.Fatalf("second tab = %#v", format.TabStops[1])
	}
}
