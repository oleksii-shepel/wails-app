package layout

import (
	"strings"
	"math"
	"testing"

	"office/pkg/ooxml/drawing"
	"office/pkg/presentation/ir"
)

func TestParagraphVisualPlanOwnsMarkerTabsAndSpacing(t *testing.T) {
	p := &ir.Paragraph{Level: 2, AutoNum: true, AutoNumType: "romanUcPeriod", AutoNumStart: 4, BulletColor: "336699", BulletFontFamily: "Aptos", BulletSizePct: 80000, DefaultTabSizeEMU: 457200, HasLineSpacing: true, LineSpacingPct: 120000, Runs: []*ir.Run{{Text: "A\tB", FontSize: 2000}}}
	plan := ResolveParagraphVisualPlan(p, 300, 1, 1)
	if plan.Marker.Kind != "auto" || plan.Marker.NumberingType != "romanUcPeriod" || plan.Marker.Start != 4 {
		t.Fatalf("marker plan = %+v", plan.Marker)
	}
	if plan.Marker.FontSizePt != 16 || plan.Marker.Color != "336699" || plan.Marker.FontFamily != "Aptos" {
		t.Fatalf("marker paint = %+v", plan.Marker)
	}
	if !plan.HasTabs || plan.TabSizePt != 36 {
		t.Fatalf("tab plan = %+v", plan)
	}
	if !plan.HasLineSpacing || plan.Metrics.LineHeightScale <= 1 {
		t.Fatalf("line spacing = %+v", plan.Metrics)
	}
}

func TestRunVisualPlanOwnsTextPaintAndEffects(t *testing.T) {
	r := &ir.Run{Text: "Paint", Bold: true, Underline: true, FontSize: 1800, FontFamily: "Aptos", Color: "112233", Highlight: "FFFF00", Effects: drawing.EffectList{Effects: []drawing.Effect{drawing.Glow{Color: "FF0000", Radius: 3}}}}
	plan := ResolveRunVisualPlan(r, .5)
	if !plan.Bold || !plan.Underline || plan.FontSizePt != 9 || plan.FontFamily != "Aptos" || plan.Color != "112233" {
		t.Fatalf("run plan = %+v", plan)
	}
	if !plan.Effects.HasGlow || len(plan.Effects.Glows) != 1 {
		t.Fatalf("run effects = %+v", plan.Effects)
	}
}

func TestParagraphVisualPlanOwnsPhysicalLineMaterialization(t *testing.T) {
	p := &ir.Paragraph{Runs: []*ir.Run{{Text: "alpha beta", FontSize: 1200}}}
	plan := ResolveParagraphVisualPlan(p, 35, 1, 1)
	if !plan.LinesMeasured || len(plan.Lines) < 2 {
		t.Fatalf("paragraph physical lines = measured:%v lines:%d, want layout-owned wrapping", plan.LinesMeasured, len(plan.Lines))
	}
}

func TestPhysicalTabsHonorCenterRightAndDecimalAlignment(t *testing.T) {
	font := 1200
	cases := []struct {
		name       string
		align      string
		text       string
		wantTarget func(fieldWidth, prefixWidth float64) float64
	}{
		{"center", "ctr", "AB", func(w, _ float64) float64 { return 72 - w/2 }},
		{"right", "r", "AB", func(w, _ float64) float64 { return 72 - w }},
		{"decimal", "dec", "12.34", func(_, prefix float64) float64 { return 72 - prefix }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &ir.Paragraph{TabStops: []ir.TabStop{{Pos: 72 * 12700, Algn: tc.align}}, Runs: []*ir.Run{{Text: "X\t" + tc.text, FontSize: font, FontFamily: "Arial"}}}
			plan := ResolveParagraphVisualPlan(p, 300, 1, 1)
			if len(plan.Lines) != 1 || len(plan.Lines[0].Fragments) < 3 {
				t.Fatalf("lines = %+v", plan.Lines)
			}
			line := plan.Lines[0]
			xWidth := measureRunTextWidthPt("X", p.Runs[0], 12)
			fieldWidth := measureRunTextWidthPt(tc.text, p.Runs[0], 12)
			prefix := fieldWidth
			if tc.align == "dec" {
				prefix = measureRunTextWidthPt("12", p.Runs[0], 12)
			}
			wantTab := tc.wantTarget(fieldWidth, prefix) - xWidth
			var gotTab float64
			for _, f := range line.Fragments {
				if f.Text == "\t" {
					gotTab = f.WidthPt
				}
			}
			if math.Abs(gotTab-wantTab) > .01 {
				t.Fatalf("tab advance = %.3f, want %.3f", gotTab, wantTab)
			}
		})
	}
}


func TestPhysicalMarkerFallsBackToEffectiveParagraphTextFont(t *testing.T) {
	p := &ir.Paragraph{
		BulletChar: "•",
		Runs: []*ir.Run{{Text: "Body", FontFamily: "Aptos", FontSize: 2000, Color: "336699"}},
	}
	plan := ResolveParagraphVisualPlan(p, 300, 1, 1)
	if plan.Marker.FontFamily != "Aptos" {
		t.Fatalf("marker font = %q, want effective text font Aptos", plan.Marker.FontFamily)
	}
	if plan.Marker.Color != "336699" {
		t.Fatalf("marker color = %q, want effective text color", plan.Marker.Color)
	}
}

func TestPhysicalMarkerPreservesExplicitSymbolFont(t *testing.T) {
	p := &ir.Paragraph{
		BulletChar: "\uf0a7", BulletFontFamily: "Wingdings",
		Runs: []*ir.Run{{Text: "Body", FontFamily: "Arial", FontSize: 1800}},
	}
	plan := ResolveParagraphVisualPlan(p, 300, 1, 1)
	if plan.Marker.FontFamily != "Wingdings" {
		t.Fatalf("explicit symbol marker font = %q, want Wingdings", plan.Marker.FontFamily)
	}
}

func TestPhysicalMarkerPercentageSizeWinsIfBothRepresentationsReachLayout(t *testing.T) {
	p := &ir.Paragraph{
		BulletChar: "•", BulletSize: 3000, BulletSizePct: 125000,
		Runs: []*ir.Run{{Text: "Body", FontFamily: "Arial", FontSize: 2000}},
	}
	plan := ResolveParagraphVisualPlan(p, 300, 1, 1)
	if plan.Marker.FontSizePt != 25 {
		t.Fatalf("marker size = %.3fpt, want 25pt from 125%% effective text size", plan.Marker.FontSizePt)
	}
}


func TestPhysicalTabsSkipStopsBehindCurrentPenAndUseNextAuthoredStop(t *testing.T) {
	p := &ir.Paragraph{
		TabStops: []ir.TabStop{{Pos: 36 * 12700, Algn: "l"}, {Pos: 72 * 12700, Algn: "l"}},
		Runs: []*ir.Run{{Text: "XXXXXXXX\tA", FontSize: 1200, FontFamily: "Arial"}},
	}
	plan := ResolveParagraphVisualPlan(p, 300, 1, 1)
	if len(plan.Lines) != 1 {
		t.Fatalf("lines = %d", len(plan.Lines))
	}
	var before, tab float64
	for _, f := range plan.Lines[0].Fragments {
		if f.Text == "\t" {
			tab = f.WidthPt
			break
		}
		before += f.WidthPt
	}
	if before >= 36 && before < 72 && math.Abs((before+tab)-72) > .01 {
		t.Fatalf("tab target = %.3f, want next authored stop 72pt", before+tab)
	}
}

func TestHangingIndentKeepsMarkerInsideAuthoredMarkerRegion(t *testing.T) {
	p := &ir.Paragraph{
		HasMarginLeft: true, MarginLeftEMU: 36 * 12700,
		HasTextIndent: true, TextIndentEMU: -30 * 12700,
		BulletChar: "•",
		Runs: []*ir.Run{{Text: "Body", FontFamily: "Arial", FontSize: 1800}},
	}
	plan := ResolveParagraphVisualPlan(p, 300, 1, 1)
	if math.Abs(plan.Marker.OffsetPt-(-30)) > .01 {
		t.Fatalf("marker offset = %.3f, want authored -30pt", plan.Marker.OffsetPt)
	}
	if plan.Metrics.MarginLeftPt != 36 || plan.Metrics.TextIndentPt != -30 {
		t.Fatalf("paragraph metrics = %+v", plan.Metrics)
	}
}


func TestClass12CalculatedWrapNeverManufacturesEmptyLineForOversizeGlyph(t *testing.T) {
	p := &ir.Paragraph{Runs: []*ir.Run{{Text: "WW", FontFamily: "Arial", FontSize: 7200}}}
	// Keep the line above CanLayoutParagraphLines' half-em safety floor while
	// still narrower than one 72pt Arial W.
	plan := ResolveParagraphVisualPlan(p, 40, 1, 1)
	if len(plan.Lines) != 2 {
		t.Fatalf("lines = %d, want one physical line per oversize glyph", len(plan.Lines))
	}
	for i, line := range plan.Lines {
		if len(line.Fragments) == 0 {
			t.Fatalf("line %d is empty; wrapping manufactured a source line", i)
		}
	}
}

func TestClass12AuthoredBreakSurvivesCalculatedWrapping(t *testing.T) {
	p := &ir.Paragraph{Runs: []*ir.Run{{Text: "A\nB", FontFamily: "Arial", FontSize: 1200}}}
	plan := ResolveParagraphVisualPlan(p, 300, 1, 1)
	if len(plan.Lines) != 2 || plan.Lines[0].BreakAfter != "authored" {
		t.Fatalf("authored break ownership lost: %+v", plan.Lines)
	}
}

func TestClass12NoWrapPreservesAuthoredBreakButSuppressesCalculatedWrap(t *testing.T) {
	p := &ir.Paragraph{Runs: []*ir.Run{{Text: "alpha beta\nomega", FontFamily: "Arial", FontSize: 2400}}}
	base := ResolveParagraphVisualPlan(p, 20, 1, 1)
	got := ResolveNoWrapParagraphVisualPlan(p, base, 1)
	if len(got.Lines) != 2 {
		t.Fatalf("no-wrap lines = %d, want exactly two authored lines", len(got.Lines))
	}
	if got.Lines[0].BreakAfter != "authored" {
		t.Fatalf("first break = %q, want authored", got.Lines[0].BreakAfter)
	}
}


func TestClass12MultiRunWrapPreservesRunOwnershipAcrossBoundary(t *testing.T) {
	r1 := &ir.Run{Text: "AAAA ", FontFamily: "Arial", FontSize: 1800}
	r2 := &ir.Run{Text: "BBBB", FontFamily: "Times New Roman", FontSize: 1800, Italic: true}
	p := &ir.Paragraph{Runs: []*ir.Run{r1, r2}}
	plan := ResolveParagraphVisualPlan(p, 55, 1, 1)
	if len(plan.Lines) < 2 {
		t.Fatalf("lines = %d, want calculated wrap", len(plan.Lines))
	}
	foundSecondRun := false
	for _, f := range plan.Lines[1].Fragments {
		if f.Run == r2 && strings.Contains(f.Text, "B") {
			foundSecondRun = true
		}
	}
	if !foundSecondRun {
		t.Fatalf("second physical line lost second-run ownership: %+v", plan.Lines[1].Fragments)
	}
}

func TestClass12WhitespaceWithinAuthoredLineIsPreserved(t *testing.T) {
	p := &ir.Paragraph{Runs: []*ir.Run{{Text: "alpha  beta", FontFamily: "Arial", FontSize: 1200}}}
	plan := ResolveParagraphVisualPlan(p, 300, 1, 1)
	var got string
	for _, f := range plan.Lines[0].Fragments {
		got += f.Text
	}
	if got != "alpha  beta" {
		t.Fatalf("physical text = %q, want authored double space", got)
	}
}

func TestClass12TabAtWrapBoundaryMovesToNextPhysicalLine(t *testing.T) {
	p := &ir.Paragraph{
		DefaultTabSizeEMU: 36 * 12700,
		Runs: []*ir.Run{{Text: "XXXXXXXX\tA", FontFamily: "Arial", FontSize: 1200}},
	}
	plan := ResolveParagraphVisualPlan(p, 60, 1, 1)
	if len(plan.Lines) < 2 {
		t.Fatalf("lines = %d, want calculated wrapping", len(plan.Lines))
	}
	found := false
	for _, line := range plan.Lines {
		for i, f := range line.Fragments {
			if f.Text == "\t" {
				found = true
				hasField := false
				for _, after := range line.Fragments[i+1:] {
					if strings.Contains(after.Text, "A") {
						hasField = true
						break
					}
				}
				if !hasField {
					t.Fatalf("wrapped tab lost ownership of following field: %+v", line.Fragments)
				}
			}
		}
	}
	if !found {
		t.Fatalf("no physical tab fragment after wrapping: %+v", plan.Lines)
	}
}

func TestClass12ContinuationLineRecoversFirstLineIndentWidth(t *testing.T) {
	p := &ir.Paragraph{
		HasTextIndent: true, TextIndentEMU: 36 * 12700,
		Runs: []*ir.Run{{Text: "AAAA BBBB CCCC", FontFamily: "Arial", FontSize: 1200}},
	}
	plan := ResolveParagraphVisualPlan(p, 100, 1, 1)
	if plan.Metrics.FirstLineAvailableWidthPt >= plan.Metrics.AvailableWidthPt {
		t.Fatalf("first line width %.2f must be narrower than continuation %.2f",
			plan.Metrics.FirstLineAvailableWidthPt, plan.Metrics.AvailableWidthPt)
	}
	if len(plan.Lines) < 2 {
		t.Fatalf("expected wrapping with first-line indent: %+v", plan.Lines)
	}
}
