package layout

import (
    "testing"
    ir "office/pkg/presentation/ir"
)

func TestRunCharacterSpacingParticipatesInMeasuredWidth(t *testing.T) {
    plain := &ir.Run{Text: "ABCD", FontSize: 2000, FontFamily: "Arial"}
    spaced := &ir.Run{Text: "ABCD", FontSize: 2000, FontFamily: "Arial", CharSpacing: 100}
    a := measureRunTextWidthPt(plain.Text, plain, 20)
    b := measureRunTextWidthPt(spaced.Text, spaced, 20)
    if d := b-a; d < 2.99 || d > 3.01 { t.Fatalf("spacing delta = %.3fpt, want 3pt", d) }
}

func TestCharacterSpacingSurvivesWrapTokenBoundaries(t *testing.T) {
    run := &ir.Run{Text: "AB CD", FontSize: 2000, FontFamily: "Arial", CharSpacing: 100}
    p := &ir.Paragraph{Runs: []*ir.Run{run}}
    metrics := ParagraphMetrics{AvailableWidthPt: 500, LineHeightScale: 1}
    got := layoutParagraphLinesWithMarker(p, metrics, 1, "", 0, "", 0)
    if len(got.Lines) != 1 { t.Fatalf("lines = %d, want 1", len(got.Lines)) }
    want := measureRunTextWidthPt(run.Text, run, 20)
    if d := got.Lines[0].WidthPt - want; d < -.001 || d > .001 {
        t.Fatalf("tokenized width = %.3fpt, want %.3fpt", got.Lines[0].WidthPt, want)
    }
}

func TestCharacterSpacingSurvivesCharacterByCharacterWrappingPath(t *testing.T) {
    run := &ir.Run{Text: "ABCD", FontSize: 2000, FontFamily: "Arial", CharSpacing: 100}
    p := &ir.Paragraph{Runs: []*ir.Run{run}}
    // Force the oversized-token path. The 50pt limit legitimately wraps ABCD,
    // so verify spacing is preserved inside each resulting physical line rather
    // than asserting that the authored token must remain on one line.
    metrics := ParagraphMetrics{AvailableWidthPt: 50, LineHeightScale: 1}
    got := layoutParagraphLinesWithMarker(p, metrics, 1, "", 0, "", 0)
    if len(got.Lines) != 2 { t.Fatalf("lines = %d, want 2", len(got.Lines)) }
    for i, line := range got.Lines {
        text := ""
        for _, frag := range line.Fragments {
            text += frag.Text
        }
        want := measureRunTextWidthPt(text, run, 20)
        if d := line.WidthPt - want; d < -.001 || d > .001 {
            t.Fatalf("line %d character-path width = %.3fpt, want %.3fpt for %q", i, line.WidthPt, want, text)
        }
    }
}
