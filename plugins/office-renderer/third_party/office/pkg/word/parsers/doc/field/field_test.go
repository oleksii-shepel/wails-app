package field

import "testing"

func TestParseInstructionSharedBinaryGrammar(t *testing.T) {
	tests := []struct {
		input  string
		kind   string
		arg    string
		target string
		anchor string
	}{
		{`HYPERLINK "https://example.com/report"`, "HYPERLINK", "", "https://example.com/report", ""},
		{`REF "Bookmark Name" \\h`, "REF", "", "", "Bookmark Name"},
		{`PAGEREF Section_2`, "PAGEREF", "", "", "Section_2"},
		{`DOCPROPERTY "Last Saved By"`, "DOCPROPERTY", "Last Saved By", "", ""},
		{`DOCVARIABLE BuildNumber`, "DOCVARIABLE", "BuildNumber", "", ""},
		{`FILLIN "Prompt text" "Default value"`, "FILLIN", "Default value", "", ""},
		{`SEQ Figure \\* ARABIC`, "SEQ", "", "", ""},
	}
	for _, tt := range tests {
		got := ParseInstruction(tt.input)
		if got.Kind != tt.kind || got.Arg != tt.arg || got.Target != tt.target || got.Anchor != tt.anchor {
			t.Fatalf("ParseInstruction(%q) = %#v", tt.input, got)
		}
		if got.Instr != tt.input {
			t.Fatalf("ParseInstruction(%q).Instr = %q", tt.input, got.Instr)
		}
	}
}

func TestParseInstructionPreservesUnknownKind(t *testing.T) {
	got := ParseInstruction(`CUSTOMFIELD "value"`)
	if got.Kind != "CUSTOMFIELD" || got.Instr != `CUSTOMFIELD "value"` {
		t.Fatalf("unknown field = %#v", got)
	}
}
