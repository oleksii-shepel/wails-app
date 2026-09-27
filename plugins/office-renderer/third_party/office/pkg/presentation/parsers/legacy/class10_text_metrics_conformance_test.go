package legacy

import (
	"encoding/binary"
	"testing"
)

func TestClass10LegacyCFExceptionPreservesExactBaselineDisplacement(t *testing.T) {
	payload := make([]byte, 6)
	binary.LittleEndian.PutUint32(payload[:4], 1<<12)
	binary.LittleEndian.PutUint16(payload[4:], uint16(int16ToBits(-25)))
	run, pos, ok := parseCFExceptionWithTheme(payload, 0, nil, nil)
	if !ok || pos != len(payload) {
		t.Fatalf("parse = ok:%v pos:%d, want true/%d", ok, pos, len(payload))
	}
	if run.BaselineShiftPct != -25 || !run.Subscript || run.Superscript {
		t.Fatalf("baseline = %.1f super:%v sub:%v, want -25/false/true", run.BaselineShiftPct, run.Superscript, run.Subscript)
	}
}

func TestClass10LegacyCFExceptionDecodesScriptSpecificTypefaceRefs(t *testing.T) {
	const masks = uint32((1 << 14) | (1 << 15) | (1 << 16))
	payload := make([]byte, 10)
	binary.LittleEndian.PutUint32(payload[:4], masks)
	binary.LittleEndian.PutUint16(payload[4:6], 2)  // oldEATypeface
	binary.LittleEndian.PutUint16(payload[6:8], 1)  // ansiTypeface
	binary.LittleEndian.PutUint16(payload[8:10], 3) // symbolTypeface
	fonts := map[uint16]string{1: "Arial", 2: "MS Gothic", 3: "Wingdings"}
	run, pos, ok := parseCFExceptionWithFonts(payload, 0, fonts)
	if !ok || pos != len(payload) {
		t.Fatalf("parse = ok:%v pos:%d, want true/%d", ok, pos, len(payload))
	}
	if run.AnsiFontFamily != "Arial" || run.EAFontFamily != "MS Gothic" || run.SymbolFontFamily != "Wingdings" {
		t.Fatalf("font refs = ansi:%q ea:%q symbol:%q", run.AnsiFontFamily, run.EAFontFamily, run.SymbolFontFamily)
	}
}

func TestClass10ScriptSpecificTypefaceParticipatesInCanonicalFontSelection(t *testing.T) {
	run := stpaCFRun{FontFamily: "Times New Roman", AnsiFontFamily: "Arial", EAFontFamily: "MS Gothic", SymbolFontFamily: "Wingdings"}
	cases := []struct{ text, want string }{
		{"Legacy text", "Arial"},
		{"日本語", "MS Gothic"},
		{"\ue000", "Wingdings"},
		{"Latin 日本語", "Times New Roman"}, // mixed-script run keeps generic authored typeface
	}
	for _, tc := range cases {
		if got := legacyPptCFRunFontFamily(run, tc.text); got != tc.want {
			t.Fatalf("font(%q) = %q, want %q", tc.text, got, tc.want)
		}
	}
}

func int16ToBits(v int16) uint16 { return uint16(v) }

func TestClass10SingleCFRunSplitsAtScriptTypefaceBoundaries(t *testing.T) {
	cf := stpaCFRun{
		CharCount:        5,
		FontFamily:       "Calibri",
		AnsiFontFamily:   "Arial",
		EAFontFamily:     "MS Gothic",
		SymbolFontFamily: "Wingdings",
		FontSize:         2000,
	}
	runs := buildRunsFromCF("A 日\ue000", []stpaCFRun{cf}, 0, 5)
	if len(runs) != 3 {
		t.Fatalf("runs = %d, want 3: %#v", len(runs), runs)
	}
	wantText := []string{"A ", "日", "\ue000"}
	wantFont := []string{"Arial", "MS Gothic", "Wingdings"}
	for i := range runs {
		if runs[i].Text != wantText[i] || runs[i].FontFamily != wantFont[i] || runs[i].FontSize != 2000 {
			t.Fatalf("run %d = text %q font %q size %d, want %q/%q/2000", i, runs[i].Text, runs[i].FontFamily, runs[i].FontSize, wantText[i], wantFont[i])
		}
	}
}

func TestClass10CFRunBoundaryDoesNotLeakMetricsAcrossRuns(t *testing.T) {
	bold := true
	cfRuns := []stpaCFRun{
		{CharCount: 2, FontFamily: "Arial", FontSize: 1800, Bold: &bold},
		{CharCount: 2, FontFamily: "Times New Roman", FontSize: 2400},
	}
	runs := buildRunsFromCF("abcd", cfRuns, 0, 4)
	if len(runs) != 2 {
		t.Fatalf("runs = %d, want 2", len(runs))
	}
	if runs[0].Text != "ab" || runs[0].FontFamily != "Arial" || runs[0].FontSize != 1800 || !runs[0].BoldSet || !runs[0].Bold {
		t.Fatalf("first run = %#v", runs[0])
	}
	if runs[1].Text != "cd" || runs[1].FontFamily != "Times New Roman" || runs[1].FontSize != 2400 || runs[1].BoldSet || runs[1].Bold {
		t.Fatalf("second run leaked metrics = %#v", runs[1])
	}
}

func TestClass10NeutralCharactersStayWithSurroundingScriptMetricRun(t *testing.T) {
	cf := stpaCFRun{AnsiFontFamily: "Arial", EAFontFamily: "MS Gothic"}
	segments := legacyPptSplitCFRunByTypeface(cf, "(ABC) 日本")
	if len(segments) != 2 {
		t.Fatalf("segments = %#v, want two metric runs", segments)
	}
	if segments[0].Text != "(ABC) " || segments[0].FontFamily != "Arial" {
		t.Fatalf("latin segment = %#v", segments[0])
	}
	if segments[1].Text != "日本" || segments[1].FontFamily != "MS Gothic" {
		t.Fatalf("EA segment = %#v", segments[1])
	}
}

func TestClass10MixedLegacySymbolGlyphDoesNotOwnFollowingANSIText(t *testing.T) {
	cf := stpaCFRun{CharCount: 36, FontFamily: "Symbol", SymbolFontFamily: "Symbol", FontSize: 2800}
	runs := buildRunsFromCF("\uf0bc and its frequency-domain equivalent", []stpaCFRun{cf}, 0, 36)
	if len(runs) != 2 {
		t.Fatalf("runs = %#v, want symbol glyph + ANSI segment", runs)
	}
	if runs[0].Text != "\uf0bc " || runs[0].FontFamily != "Symbol" {
		t.Fatalf("symbol segment = %#v", runs[0])
	}
	if runs[1].Text != "and its frequency-domain equivalent" || runs[1].FontFamily != "" {
		t.Fatalf("ANSI segment = %#v, want unowned font for master cascade", runs[1])
	}
}
