package field

import "testing"

func TestParseInstructionCoversLegacyBinaryFieldFamilies(t *testing.T) {
	tests := []struct {
		instr  string
		kind   string
		arg    string
		target string
		anchor string
	}{
		{`PAGE`, "PAGE", "", "", ""},
		{`NUMPAGES`, "NUMPAGES", "", "", ""},
		{`NUMCHARS`, "NUMCHARS", "", "", ""},
		{`NUMWORDS`, "NUMWORDS", "", "", ""},
		{`AUTHOR`, "AUTHOR", "", "", ""},
		{`CREATEDATE`, "CREATEDATE", "", "", ""},
		{`SAVEDATE`, "SAVEDATE", "", "", ""},
		{`PRINTDATE`, "PRINTDATE", "", "", ""},
		{`DATE`, "DATE", "", "", ""},
		{`TIME`, "TIME", "", "", ""},
		{`TITLE`, "TITLE", "", "", ""},
		{`SUBJECT`, "SUBJECT", "", "", ""},
		{`FORMTEXT`, "FORMTEXT", "", "", ""},
		{`FORMCHECKBOX`, "FORMCHECKBOX", "", "", ""},
		{`FORMDROPDOWN`, "FORMDROPDOWN", "", "", ""},
		{`REF "Bookmark Name"`, "REF", "", "", "Bookmark Name"},
		{`PAGEREF target`, "PAGEREF", "", "", "target"},
		{`STYLEREF "Heading 1"`, "STYLEREF", "Heading 1", "", ""},
		{`DOCPROPERTY Title`, "DOCPROPERTY", "Title", "", ""},
		{`DOCVARIABLE Build`, "DOCVARIABLE", "Build", "", ""},
		{`SEQ Figure`, "SEQ", "", "", ""},
		{`ASK Name`, "ASK", "Name", "", ""},
		{`MERGEFIELD Customer`, "MERGEFIELD", "Customer", "", ""},
		{`AUTOTEXT Signature`, "AUTOTEXT", "Signature", "", ""},
		{`SET Bookmark "value"`, "SET", "Bookmark", "", ""},
	}
	for _, tt := range tests {
		got := ParseInstruction(tt.instr)
		if got.Kind != tt.kind || got.Arg != tt.arg || got.Target != tt.target || got.Anchor != tt.anchor {
			t.Errorf("ParseInstruction(%q) = %#v", tt.instr, got)
		}
	}
}

func TestParseInstructionHyperlinkSwitches(t *testing.T) {
	tests := []struct {
		instr  string
		target string
		anchor string
	}{
		{`HYPERLINK "https://example.com/report"`, "https://example.com/report", ""},
		{`HYPERLINK \l "Section 1"`, "#Section 1", "Section 1"},
		{`HYPERLINK "https://example.com/report" \l "Section 1"`, "https://example.com/report#Section 1", "Section 1"},
		{`HYPERLINK "https://example.com" \o "tooltip"`, "https://example.com", ""},
	}
	for _, tt := range tests {
		got := ParseInstruction(tt.instr)
		if got.Kind != "HYPERLINK" || got.Target != tt.target || got.Anchor != tt.anchor {
			t.Errorf("ParseInstruction(%q) = %#v", tt.instr, got)
		}
	}
}

func TestParseInstructionPreservesComplexArguments(t *testing.T) {
	for _, instr := range []string{
		`QUOTE "literal value"`,
		`EMBED Excel.Sheet.12`,
		`LINK Excel.Sheet.12 "book.xls"`,
		`IF 1 = 1 "yes" "no"`,
		`COMPARE 1 = 1`,
	} {
		got := ParseInstruction(instr)
		if got.Arg != instr || got.Instr != instr {
			t.Fatalf("ParseInstruction(%q) = %#v, want raw semantic argument preserved", instr, got)
		}
	}
}

func TestParseInstructionUnknownIsLossless(t *testing.T) {
	got := ParseInstruction(`PRIVATEFIELD "opaque value"`)
	if got.Kind != "PRIVATEFIELD" || got.Instr != `PRIVATEFIELD "opaque value"` {
		t.Fatalf("unknown instruction = %#v", got)
	}
}
