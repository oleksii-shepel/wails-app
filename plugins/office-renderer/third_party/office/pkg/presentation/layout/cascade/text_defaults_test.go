package cascade

import (
	"testing"

	"office/pkg/presentation/ir"
	"office/pkg/presentation/parsers/ooxml/parts"
)

func TestEffectiveTextDefaultsUsesSpecificFirstCascade(t *testing.T) {
	shapeSize, layoutSize, masterSize := 2400, 2000, 1800
	shape := &parts.TextBody{ListStyle: &parts.DMLTextListStyle{Lvl1PPr: &parts.DMLTextStyleLevel{DefRPr: &parts.DMLRunPr{FontSize: shapeSize}}}}
	layout := &parts.TextBody{ListStyle: &parts.DMLTextListStyle{Lvl1PPr: &parts.DMLTextStyleLevel{DefRPr: &parts.DMLRunPr{FontSize: layoutSize}}}}
	master := &parts.TextBody{ListStyle: &parts.DMLTextListStyle{Lvl1PPr: &parts.DMLTextStyleLevel{DefRPr: &parts.DMLRunPr{FontSize: masterSize}}}}
	got := EffectiveTextDefaults(shape, layout, master, nil, nil, ir.PlaceholderBody, 0, nil, nil, nil, &ir.Theme{MinorFont: "Body"})
	if got.FontSize != shapeSize {
		t.Fatalf("font size = %d, want shape %d", got.FontSize, shapeSize)
	}
	if got.FontFamily != "Body" {
		t.Fatalf("font family = %q, want theme fallback Body", got.FontFamily)
	}
}

func TestMergeEndParagraphRunDefaultsOverridesInheritedCharacters(t *testing.T) {
	inheritedSize, endSize := 1800, 2200
	inheritedBold, endBold := false, true
	got := MergeEndParagraphRunDefaults(TextDefaults{FontSize: inheritedSize, Bold: &inheritedBold}, &parts.DMLRunPr{FontSize: endSize, Bold: &endBold}, nil, nil)
	if got.FontSize != endSize || got.Bold == nil || !*got.Bold {
		t.Fatalf("endParaRPr defaults not applied: %+v", got)
	}
}

func TestEffectiveTextDefaultsUsesPresentationDefaultTextStyleAsFinalFallback(t *testing.T) {
	presentation := &parts.DMLTextListStyle{Lvl1PPr: &parts.DMLTextStyleLevel{DefRPr: &parts.DMLRunPr{
		FontSize: 2600,
		Latin:    &parts.FontRef{Typeface: "Source Code Pro"},
	}}}
	got := EffectiveTextDefaults(&parts.TextBody{}, nil, nil, nil, presentation, ir.PlaceholderTitle, 0, nil, nil, nil, &ir.Theme{MajorFont: "Arial"})
	if got.FontSize != 2600 || got.FontFamily != "Source Code Pro" {
		t.Fatalf("presentation fallback = size %d family %q, want 2600 Source Code Pro", got.FontSize, got.FontFamily)
	}
}

func TestEffectiveTextDefaultsKeepsMasterAndShapeAheadOfPresentationDefault(t *testing.T) {
	shape := &parts.TextBody{ListStyle: &parts.DMLTextListStyle{Lvl1PPr: &parts.DMLTextStyleLevel{DefRPr: &parts.DMLRunPr{FontSize: 3200}}}}
	master := &parts.SlideMaster{TxStyles: &parts.MasterTxStyles{TitleStyle: &parts.DMLTextListStyle{Lvl1PPr: &parts.DMLTextStyleLevel{DefRPr: &parts.DMLRunPr{Latin: &parts.FontRef{Typeface: "Master Face"}}}}}}
	presentation := &parts.DMLTextListStyle{Lvl1PPr: &parts.DMLTextStyleLevel{DefRPr: &parts.DMLRunPr{FontSize: 1800, Latin: &parts.FontRef{Typeface: "Presentation Face"}}}}
	got := EffectiveTextDefaults(shape, nil, nil, master, presentation, ir.PlaceholderTitle, 0, nil, nil, nil, &ir.Theme{})
	if got.FontSize != 3200 || got.FontFamily != "Master Face" {
		t.Fatalf("precedence = size %d family %q, want shape size 3200 and master family", got.FontSize, got.FontFamily)
	}
}
