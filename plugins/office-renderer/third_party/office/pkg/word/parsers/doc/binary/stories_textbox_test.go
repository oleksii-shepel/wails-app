package docbinary

import "testing"

func TestTextboxStoryBase(t *testing.T) {
	counts := StoryCounts{Text: 100, Ftn: 10, Hdr: 20, Mcr: 1, Atn: 5, Edn: 6, Txbx: 30, HdrTxbx: 40}
	if got := TextboxStoryBase(counts, false); got != 142 {
		t.Fatalf("main textbox base=%d, want 142", got)
	}
	if got := TextboxStoryBase(counts, true); got != 172 {
		t.Fatalf("header textbox base=%d, want 172", got)
	}
}
