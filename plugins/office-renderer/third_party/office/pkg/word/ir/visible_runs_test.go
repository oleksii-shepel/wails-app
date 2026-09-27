package ir

import "testing"

func TestCanonicalVisibleRunsRestoresWhitespaceWithoutMovingFormatting(t *testing.T) {
	hidden := true
	p := &Paragraph{Text: "documents.  However, help the reader’s eye", Runs: []Run{
		{Kind: "run", Text: "documents."},
		{Kind: "run", Text: "  ", Format: RunFormat{Vanish: &hidden}},
		{Kind: "run", Text: "How"},
		{Kind: "run", Text: " ever, help the"},
		{Kind: "run", Text: "reader’s eye"},
	}}
	runs := CanonicalVisibleRuns(p)
	got := ""
	for _, run := range runs {
		if !runHidden(run) {
			got += run.Text
		}
	}
	if got != p.Text {
		t.Fatalf("visible text = %q, want %q", got, p.Text)
	}
	if runs[1].Text != "  " {
		t.Fatalf("hidden run changed to %q", runs[1].Text)
	}
}

func TestCanonicalVisibleRunsKeepsTabsSemantic(t *testing.T) {
	p := &Paragraph{Text: "Column1\tColumn2\tColumn3", Runs: []Run{
		{Kind: "run", Text: "Column1"}, {Kind: "tab"}, {Kind: "run", Text: "Column2"}, {Kind: "tab"}, {Kind: "run", Text: "Column3"},
	}}
	runs := CanonicalVisibleRuns(p)
	if runs[1].Kind != "tab" || runs[3].Kind != "tab" {
		t.Fatalf("semantic tabs were lost: %#v", runs)
	}
}

func TestCanonicalVisibleRunsDoesNotReconcileDifferentCharacters(t *testing.T) {
	p := &Paragraph{Text: "Alpha Beta", Runs: []Run{{Kind: "run", Text: "Alpha Zeta"}}}
	runs := CanonicalVisibleRuns(p)
	if got := runs[0].Text; got != "Alpha Zeta" {
		t.Fatalf("text = %q, want original run text", got)
	}
}

func TestCanonicalVisibleRunsRepairsExactMissingLegacyPrefix(t *testing.T) {
	p := &Paragraph{Text: "Legacy DOC fixture", Runs: []Run{{Text: "gacy DOC fixture"}}}
	got := CanonicalVisibleRuns(p)
	if len(got) != 1 || got[0].Text != "Legacy DOC fixture" {
		t.Fatalf("visible runs = %#v", got)
	}
}

func TestCanonicalVisibleRunsDoesNotRepairArbitraryMismatch(t *testing.T) {
	p := &Paragraph{Text: "Expected text", Runs: []Run{{Text: "Different text"}}}
	got := CanonicalVisibleRuns(p)
	if len(got) != 1 || got[0].Text != "Different text" {
		t.Fatalf("arbitrary mismatch was rewritten: %#v", got)
	}
}
