package legacy

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestParsePapxGrpprlCapturesTabStops(t *testing.T) {
	grpprl := []byte{
		0x15, 0xC6, // sprmPChgTabs
		0x00,       // no deletions
		0x02,       // add two tabs
		0xD0, 0x02, // 720 twips
		0xA0, 0x05, // 1440 twips
		0x00, // left
		0x02, // right
	}

	got := parsePapxGrpprl(grpprl)
	if len(got.TabStops) != 2 {
		t.Fatalf("len(TabStops) = %d, want 2", len(got.TabStops))
	}
	if got.TabStops[0] != (wordir.TabStop{Pos: 720, Algn: "left"}) {
		t.Fatalf("TabStops[0] = %+v, want left tab at 720", got.TabStops[0])
	}
	if got.TabStops[1] != (wordir.TabStop{Pos: 1440, Algn: "right"}) {
		t.Fatalf("TabStops[1] = %+v, want right tab at 1440", got.TabStops[1])
	}
}

func TestParsePreWord97PapxGrpprlCapturesTabStops(t *testing.T) {
	grpprl := []byte{
		23, // ChgTabs
		8,  // payload length
		0x00,
		0x02,
		0xD0, 0x02,
		0xA0, 0x05,
		0x01, // center
		0x03, // decimal
	}

	got, _, _, _ := parsePreWord97PapxGrpprl(grpprl)
	if len(got.TabStops) != 2 {
		t.Fatalf("len(TabStops) = %d, want 2", len(got.TabStops))
	}
	if got.TabStops[0] != (wordir.TabStop{Pos: 720, Algn: "center"}) {
		t.Fatalf("TabStops[0] = %+v, want center tab at 720", got.TabStops[0])
	}
	if got.TabStops[1] != (wordir.TabStop{Pos: 1440, Algn: "decimal"}) {
		t.Fatalf("TabStops[1] = %+v, want decimal tab at 1440", got.TabStops[1])
	}
}

func TestApplyPreWord97StyleInheritedParagraphFormattingAppliesStyleTabStops(t *testing.T) {
	istd := 7
	runs := []docRun{{
		Text:       "x\a",
		ParaFormat: wordir.ParagraphFormat{BinaryStyleIndex: &istd},
	}}
	stylesheet := &legacyStyleSheet{
		paragraphByIndex: map[int]wordir.Style{
			istd: {
				ID: "legacy-istd-7",
				Format: wordir.ParagraphFormat{
					TabStops: []wordir.TabStop{{Pos: 720, Algn: "left"}},
				},
			},
		},
	}

	applyPreWord97StyleInheritedParagraphFormatting(runs, stylesheet)

	if len(runs[0].ParaFormat.TabStops) != 1 {
		t.Fatalf("len(TabStops) = %d, want 1", len(runs[0].ParaFormat.TabStops))
	}
	if runs[0].ParaFormat.TabStops[0] != (wordir.TabStop{Pos: 720, Algn: "left"}) {
		t.Fatalf("TabStops[0] = %+v, want inherited left tab at 720", runs[0].ParaFormat.TabStops[0])
	}
}