package field

import "testing"

func TestReferenceFieldsKeepBookmarkInAnchorOnly(t *testing.T) {
	for _, instr := range []string{
		`REF "Bookmark Name" \h`,
		`PAGEREF "Bookmark Name" \h`,
	} {
		got := ParseInstruction(instr)
		if got.Anchor != "Bookmark Name" {
			t.Fatalf("ParseInstruction(%q).Anchor = %q", instr, got.Anchor)
		}
		if got.Arg != "" {
			t.Fatalf("ParseInstruction(%q).Arg = %q, want empty; bookmark ownership is Anchor", instr, got.Arg)
		}
	}
}
