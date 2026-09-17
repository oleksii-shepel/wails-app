package legacy

import (
    "testing"

    presentationir "office/pkg/presentation/ir"
)

func TestMSPPTPartialTitleRunInheritsMissingTitleMetrics(t *testing.T) {
    styles := legacyPptMasterTextStyles{
        pptTextTypeTitle: {0: {run: stpaCFRun{FontFamily: "Arial"}}},
    }
    got := legacyPptMasterTextLevelsForType(styles, pptTextTypeTitle)
    if got[0].run.FontFamily != "Arial" {
        t.Fatalf("font family = %q, want Arial", got[0].run.FontFamily)
    }
    if got[0].run.FontSize != 4400 {
        t.Fatalf("font size = %d, want 4400", got[0].run.FontSize)
    }
}

func TestMSPPTSubtitlePlaceholderCentersWhenLocalAlignmentIsOmitted(t *testing.T) {
    shape := &presentationir.Shape{Kind: presentationir.ShapeText, Placeholder: presentationir.PlaceholderSubtitle, Paragraphs: []*presentationir.Paragraph{{Runs: []*presentationir.Run{{Text: "subtitle"}}}}}
    styles := legacyPptMasterTextStyles{pptTextTypeCenterBody: {0: {paragraph: stpaPFEntry{Align: "l"}, run: stpaCFRun{FontSize: 3200, FontFamily: "Arial"}}}}
    applyLegacyPptMasterTextStylesToShape(shape, styles, pptTextTypeCenterBody)
    if got := shape.Paragraphs[0].Align; got != "ctr" { t.Fatalf("subtitle alignment = %q, want ctr", got) }
    if got := shape.Paragraphs[0].Runs[0].FontSize; got != 3200 { t.Fatalf("subtitle font size = %d, want 3200", got) }
}

func TestMSPPTMasterPlaceholderKindsAreRecognized(t *testing.T) {
    tests := []struct { id byte; want presentationir.PlaceholderKind }{{0x01,presentationir.PlaceholderTitle},{0x02,presentationir.PlaceholderBody},{0x03,presentationir.PlaceholderTitle},{0x04,presentationir.PlaceholderSubtitle}}
    for _, tt := range tests {
        payload := []byte{0,0,0,0,tt.id,0,0,0}
        got, ok := parseLegacyPptPlaceholderAtom(payload)
        if !ok || got != tt.want { t.Fatalf("placeholder %#x = (%v,%v), want (%v,true)",tt.id,got,ok,tt.want) }
    }
}
