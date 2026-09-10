package resolve

import (
	"testing"

	"office/pkg/sheet/parsers/ooxml/parts"
)

func TestStyleEntriesPreservePatternFillAndExtendedHorizontalAlignment(t *testing.T) {
	theme := map[int]string{4: "112233"}
	indexed := map[int]string{}
	themeIndex := 4
	ss := &parts.StyleSheet{
		Fills: []parts.Fill{
			{}, {},
			{PatternFill: &parts.PatternFill{
				PatternType: "darkGrid",
				FgColor:     &parts.ColorVal{RGB: "FF445566"},
				BgColor:     &parts.ColorVal{Theme: &themeIndex},
			}},
		},
		CellXfs: []parts.Xf{{FillID: 2, Alignment: &parts.Alignment{Horizontal: "distributed"}}},
	}

	got := buildStyleEntries(ss, ss.CellXfs, theme, indexed)
	if len(got) != 1 {
		t.Fatalf("style entries = %d, want 1", len(got))
	}
	if got[0].fillPattern != "darkGrid" || got[0].bgColor != "445566" || got[0].fillPatternBgColor != "112233" {
		t.Fatalf("pattern fill = (%q,%q,%q), want (darkGrid,445566,112233)", got[0].fillPattern, got[0].bgColor, got[0].fillPatternBgColor)
	}
	if got[0].hAlign != "distributed" {
		t.Fatalf("horizontal alignment = %q, want distributed", got[0].hAlign)
	}

	style := styleEntryToCellStyle(got[0])
	if style.FillPattern != "darkGrid" || style.FillPatternBgColor != "112233" {
		t.Fatalf("canonical fill = %#v", style)
	}
}

func TestResolveRichTextRunsUsesCanonicalThemeAndIndexedColors(t *testing.T) {
	themeIndex := 4
	indexedIndex := 7
	rt := &parts.RichText{Runs: []parts.RTRun{
		{Text: "theme", Properties: &parts.RTProps{Color: &parts.ColorVal{Theme: &themeIndex}}},
		{Text: "indexed", Properties: &parts.RTProps{Color: &parts.ColorVal{Indexed: &indexedIndex}}},
	}}
	runs := resolveRichTextRuns(rt, map[int]string{4: "112233"}, map[int]string{7: "445566"})
	if len(runs) != 2 {
		t.Fatalf("rich text runs = %d, want 2", len(runs))
	}
	if runs[0].FontColor != "112233" || runs[1].FontColor != "445566" {
		t.Fatalf("rich text colors = (%q,%q), want (112233,445566)", runs[0].FontColor, runs[1].FontColor)
	}
}

func TestBuildStyleTableInheritsReferencedCellStyleXF(t *testing.T) {
	ss := &parts.StyleSheet{
		Fonts: []parts.Font{
			{Name: &parts.StringVal{Val: "Calibri"}, Size: &parts.FloatVal{Val: 11}},
			{Bold: &struct{}{}, Name: &parts.StringVal{Val: "Aptos"}, Size: &parts.FloatVal{Val: 14}},
		},
		Fills:        []parts.Fill{{}, {}, {PatternFill: &parts.PatternFill{PatternType: "solid", FgColor: &parts.ColorVal{RGB: "FFFF00"}}}},
		CellStyleXfs: []parts.Xf{{FontID: 1, FillID: 2, Alignment: &parts.Alignment{Horizontal: "center", WrapText: true}}},
		CellXfs:      []parts.Xf{{XfID: 0}},
	}

	styles := buildStyleTable(ss, nil, nil)
	if len(styles) != 1 {
		t.Fatalf("styles = %d, want 1", len(styles))
	}
	got := styleEntryToCellStyle(styles[0])
	if !got.Bold || got.FontFamily != "Aptos" || got.FontSizePt != 14 {
		t.Fatalf("inherited font = %+v", got)
	}
	if got.BgColor != "FFFF00" || got.HAlign != "center" || !got.WrapText {
		t.Fatalf("inherited fill/alignment = %+v", got)
	}
}

func TestBuildStyleTableApplyFontCanResetBaseFontToZeroEntry(t *testing.T) {
	ss := &parts.StyleSheet{
		Fonts: []parts.Font{
			{Name: &parts.StringVal{Val: "Calibri"}, Size: &parts.FloatVal{Val: 11}},
			{Bold: &struct{}{}, Name: &parts.StringVal{Val: "Aptos"}, Size: &parts.FloatVal{Val: 14}},
		},
		CellStyleXfs: []parts.Xf{{FontID: 1}},
		CellXfs:      []parts.Xf{{XfID: 0, FontID: 0, ApplyFont: true}},
	}

	got := styleEntryToCellStyle(buildStyleTable(ss, nil, nil)[0])
	if got.Bold || got.FontFamily != "Calibri" || got.FontSizePt != 11 {
		t.Fatalf("applyFont reset = %+v, want font zero entry", got)
	}
}

func TestBuildStyleTableDirectComponentIDsApplyWithoutFlags(t *testing.T) {
	ss := &parts.StyleSheet{
		Fonts: []parts.Font{
			{Name: &parts.StringVal{Val: "Calibri"}},
			{Italic: &struct{}{}, Name: &parts.StringVal{Val: "Arial"}},
		},
		CellStyleXfs: []parts.Xf{{FontID: 0}},
		CellXfs:      []parts.Xf{{XfID: 0, FontID: 1}},
	}

	got := styleEntryToCellStyle(buildStyleTable(ss, nil, nil)[0])
	if !got.Italic || got.FontFamily != "Arial" {
		t.Fatalf("direct font fallback = %+v", got)
	}
}

func TestStyleEntriesPreserveUnderlineVariantAndFontVerticalAlignment(t *testing.T) {
	ss := &parts.StyleSheet{
		Fonts: []parts.Font{{
			Underline: &parts.StringVal{Val: "doubleAccounting"},
			VertAlign: &parts.StringVal{Val: "superscript"},
		}},
		CellXfs: []parts.Xf{{FontID: 0}},
	}
	got := buildStyleEntries(ss, ss.CellXfs, nil, nil)
	if len(got) != 1 {
		t.Fatalf("style count = %d, want 1", len(got))
	}
	if !got[0].underline || got[0].underlineType != "doubleAccounting" {
		t.Fatalf("underline = (%v, %q), want (true, doubleAccounting)", got[0].underline, got[0].underlineType)
	}
	if got[0].fontVertAlign != "superscript" {
		t.Fatalf("font vertical alignment = %q, want superscript", got[0].fontVertAlign)
	}
}

func TestStyleEntriesPreserveDiagonalAndInsideBorders(t *testing.T) {
	ss := &parts.StyleSheet{
		Borders: []parts.Border{{
			Diagonal:   &parts.BorderSide{Style: "medium", Color: &parts.ColorVal{RGB: "FF112233"}},
			Vertical:   &parts.BorderSide{Style: "dashed", Color: &parts.ColorVal{RGB: "FF445566"}},
			Horizontal: &parts.BorderSide{Style: "dotted", Color: &parts.ColorVal{RGB: "FF778899"}},
			DiagonalUp: true, DiagonalDown: true,
		}},
		CellXfs: []parts.Xf{{BorderID: 0}},
	}
	got := styleEntryToCellStyle(buildStyleEntries(ss, ss.CellXfs, nil, nil)[0])
	if got.BorderDiagonal.Style != "medium" || got.BorderDiagonal.Color != "112233" || !got.BorderDiagonalUp || !got.BorderDiagonalDown {
		t.Fatalf("diagonal border = %+v up=%v down=%v", got.BorderDiagonal, got.BorderDiagonalUp, got.BorderDiagonalDown)
	}
	if got.BorderVertical.Style != "dashed" || got.BorderHorizontal.Style != "dotted" {
		t.Fatalf("inside borders = vertical %+v horizontal %+v", got.BorderVertical, got.BorderHorizontal)
	}
}
