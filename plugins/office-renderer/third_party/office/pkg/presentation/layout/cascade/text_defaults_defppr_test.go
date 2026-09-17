package cascade

import (
	"office/pkg/presentation/parsers/ooxml/parts"
	"testing"
)

func intp(v int) *int { return &v }

func TestTextDefaultsFromListStyleMergesDefPPrIntoLevel(t *testing.T) {
	style := &parts.DMLTextListStyle{
		DefPPr:  &parts.DMLTextStyleLevel{DefRPr: &parts.DMLRunPr{FontSize: 5400, Latin: &parts.FontRef{Typeface: "Aptos Narrow"}}},
		Lvl1PPr: &parts.DMLTextStyleLevel{Algn: "ctr", MarL: intp(1200)},
	}
	got := TextDefaultsFromListStyle(style, 0, nil, nil)
	if got.FontSize != 5400 {
		t.Fatalf("font size = %d, want inherited defPPr 5400", got.FontSize)
	}
	if got.FontFamily != "Aptos Narrow" {
		t.Fatalf("font family = %q, want inherited defPPr family", got.FontFamily)
	}
	if got.Align != "ctr" {
		t.Fatalf("align = %q, want level override ctr", got.Align)
	}
	if got.MarginLeftEMU == nil || *got.MarginLeftEMU != 1200 {
		t.Fatalf("level margin not preserved: %#v", got.MarginLeftEMU)
	}
}

func TestTextDefaultsFromListStyleLevelRunPropertiesOverrideDefPPr(t *testing.T) {
	style := &parts.DMLTextListStyle{
		DefPPr:  &parts.DMLTextStyleLevel{DefRPr: &parts.DMLRunPr{FontSize: 5400, Latin: &parts.FontRef{Typeface: "Base Face"}}},
		Lvl1PPr: &parts.DMLTextStyleLevel{DefRPr: &parts.DMLRunPr{FontSize: 3200}},
	}
	got := TextDefaultsFromListStyle(style, 0, nil, nil)
	if got.FontSize != 3200 {
		t.Fatalf("font size = %d, want level 3200", got.FontSize)
	}
	if got.FontFamily != "Base Face" {
		t.Fatalf("font family = %q, want defPPr fallback", got.FontFamily)
	}
}
