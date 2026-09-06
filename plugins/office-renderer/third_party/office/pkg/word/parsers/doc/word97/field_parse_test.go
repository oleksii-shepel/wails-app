package word97

import "testing"

func TestParseFieldInstructionDisambiguatesKinds(t *testing.T) {
	cases := map[string]string{
		"PAGEREF Section1": "PAGEREF",
		"PAGE": "PAGE",
		"NUMPAGES": "NUMPAGES",
		"STYLEREF Heading1": "STYLEREF",
		"CREATEDATE": "CREATEDATE",
		"REF Bookmark": "REF",
		"HYPERLINK \"https://example.com/report\"": "HYPERLINK",
	}
	for input, want := range cases {
		if got := ParseFieldInstruction(input).Kind; got != want {
			t.Fatalf("ParseFieldInstruction(%q).Kind = %q, want %q", input, got, want)
		}
	}
}

func TestParseFieldInstructionArguments(t *testing.T) {
	ref := ParseFieldInstruction(`REF "Bookmark Name"`)
	if ref.Anchor != "Bookmark Name" { t.Fatalf("REF anchor = %q", ref.Anchor) }
	style := ParseFieldInstruction(`STYLEREF "Heading 1"`)
	if style.Arg != "Heading 1" { t.Fatalf("STYLEREF arg = %q", style.Arg) }
	fill := ParseFieldInstruction(`FILLIN "Enter your name" "John Doe"`)
	if fill.Arg != "John Doe" { t.Fatalf("FILLIN default = %q", fill.Arg) }
	link := ParseFieldInstruction(`HYPERLINK "https://example.com/report"`)
	if link.Target != "https://example.com/report" { t.Fatalf("HYPERLINK target = %q", link.Target) }
}
