package legacy

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestMinLegacyTableCols(t *testing.T) {
	tests := []struct {
		in   int
		want int
	}{
		{0, 0},
		{1, 0},
		{2, 1},
		{3, 1},
		{4, 2},
		{5, 2},
		{6, 5},
		{7, 6},
		{10, 6},
	}
	for _, tt := range tests {
		got := minLegacyTableCols(tt.in)
		if got != tt.want {
			t.Errorf("minLegacyTableCols(%d) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestLineSliceIsCellMark(t *testing.T) {
	if lineSliceIsCellMark([]docRun{{ParaFormat: wordir.ParagraphFormat{IsTableCellTrailingParagraph: true}}}) != true {
		t.Error("expected true for cell mark slice")
	}
	if lineSliceIsCellMark([]docRun{{Text: "hello"}}) != false {
		t.Error("expected false for non-cell-mark slice")
	}
	if lineSliceIsCellMark(nil) != false {
		t.Error("expected false for nil slice")
	}
}

func TestLineSliceIsExplicitBlankParagraph(t *testing.T) {
	if lineSliceIsExplicitBlankParagraph([]docRun{{Text: ""}}) != true {
		t.Error("expected true for blank paragraph")
	}
	if lineSliceIsExplicitBlankParagraph([]docRun{{Text: "x"}}) != false {
		t.Error("expected false for non-blank paragraph")
	}
	if lineSliceIsExplicitBlankParagraph(nil) != false {
		t.Error("expected false for nil slice")
	}
	if lineSliceIsExplicitBlankParagraph([]docRun{{Kind: "run", Text: ""}}) != false {
		t.Error("expected false when kind is set")
	}
}

func TestParseHyperlinkFieldInstr(t *testing.T) {
	tests := []struct {
		instr string
		want  string
	}{
		{`HYPERLINK "https://example.com"`, "https://example.com"},
		{`HYPERLINK https://example.com`, "https://example.com"},
		{`HYPERLINK https://example.com \l anchor`, "https://example.com"},
		{`HYPERLINK`, ""},
		{`NOTHYPERLINK`, ""},
	}
	for _, tt := range tests {
		got := parseHyperlinkFieldInstr(tt.instr)
		if got != tt.want {
			t.Errorf("parseHyperlinkFieldInstr(%q) = %q, want %q", tt.instr, got, tt.want)
		}
	}
}

func TestDecodePieceText(t *testing.T) {
	// compressed ASCII
	wordStream := []byte("Hello World")
	got := decodePieceText(wordStream, 0, 5, true)
	if got != "Hello" {
		t.Errorf("decodePieceText compressed = %q, want Hello", got)
	}
	// UTF-16LE
	wordStream = []byte{0x48, 0x00, 0x65, 0x00, 0x6C, 0x00, 0x6C, 0x00, 0x6F, 0x00}
	got = decodePieceText(wordStream, 0, 5, false)
	if got != "Hello" {
		t.Errorf("decodePieceText UTF-16LE = %q, want Hello", got)
	}
	// out of bounds
	got = decodePieceText(wordStream, 100, 5, true)
	if got != "" {
		t.Errorf("decodePieceText out of bounds = %q, want empty", got)
	}
	// negative char count
	got = decodePieceText(wordStream, 0, -1, true)
	if got != "" {
		t.Errorf("decodePieceText negative count = %q, want empty", got)
	}
}

func TestLegacyTableRowPropsClearHMerge(t *testing.T) {
	p := &legacyTableRowProps{hMerge: map[int]byte{0: 1, 1: 2, 2: 3}}
	p.clearHMerge(0, 2)
	if len(p.hMerge) != 1 {
		t.Errorf("expected 1 entry after clearHMerge, got %d", len(p.hMerge))
	}
	if _, ok := p.hMerge[2]; !ok {
		t.Error("expected entry 2 to remain")
	}
	// nil receiver
	var nilP *legacyTableRowProps
	nilP.clearHMerge(0, 2) // should not panic
	// nil map
	p = &legacyTableRowProps{}
	p.clearHMerge(0, 2) // should not panic
	// invalid range
	p = &legacyTableRowProps{hMerge: map[int]byte{0: 1}}
	p.clearHMerge(2, 1)
	if len(p.hMerge) != 1 {
		t.Error("expected no change when lim <= first")
	}
}
func TestDocRunsToIRRunsConvertsLiteralTabsToSemanticTabs(t *testing.T) {
	runs := docRunsToIRRunsImpl([]docRun{{Text: "Column1\tColumn2\tColumn3"}}, wordir.RunFormat{}, map[*docField]bool{})
	if len(runs) != 5 {
		t.Fatalf("runs = %#v, want 5 text/tab runs", runs)
	}
	if runs[0].Text != "Column1" || runs[1].Kind != "tab" || runs[2].Text != "Column2" || runs[3].Kind != "tab" || runs[4].Text != "Column3" {
		t.Fatalf("runs = %#v, want text/tab/text/tab/text", runs)
	}
}

func TestProcessLegacyObjectAnchorsDistinguishesPictureDataFromOLE(t *testing.T) {
	image := &wordir.Image{MIMEType: "image/png"}
	picture := processLegacyObjectAnchors([]docRun{{Text: "\x01", Data: true}}, []*wordir.Image{image})
	if len(picture) != 1 || picture[0].Kind != "image" || picture[0].Image != image {
		t.Fatalf("picture anchor = %#v, want image", picture)
	}
	ole := processLegacyObjectAnchors([]docRun{{Text: "\x01", Spec: true, Obj: true}}, []*wordir.Image{image})
	if len(ole) != 1 || ole[0].Kind != "olePlaceholder" {
		t.Fatalf("OLE anchor = %#v, want OLE placeholder", ole)
	}
}
