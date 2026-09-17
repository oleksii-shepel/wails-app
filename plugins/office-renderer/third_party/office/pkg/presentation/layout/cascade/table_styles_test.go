package cascade

import (
	"office/pkg/ooxml/drawing"
	"office/pkg/presentation/parsers/ooxml/parts"
	"testing"
)

func TestEffectiveTableStyleUsesEmbeddedStyleFirst(t *testing.T) {
	embedded := &parts.DrawingTableStyle{StyleID: "embedded"}
	table := &parts.PresentationTable{Props: &parts.PresentationTableProps{TableStyle: embedded, TableStyleID: "listed"}}
	styles := &parts.TableStyleList{Default: "listed", Styles: []parts.DrawingTableStyle{{StyleID: "listed"}}}
	if got := EffectiveTableStyle(table, styles); got != embedded {
		t.Fatalf("embedded table style lost precedence: %#v", got)
	}
}

func TestEffectiveTableStyleFallsBackToDefaultStyleListEntry(t *testing.T) {
	table := &parts.PresentationTable{Props: &parts.PresentationTableProps{}}
	styles := &parts.TableStyleList{Default: "Default-Style", Styles: []parts.DrawingTableStyle{{StyleID: "default-style"}}}
	if got := EffectiveTableStyle(table, styles); got != &styles.Styles[0] {
		t.Fatalf("default table style = %#v, want style-list entry", got)
	}
}

func TestTableColumnCountAccountsForGridSpan(t *testing.T) {
	table := &parts.PresentationTable{Rows: []parts.PresentationTableRow{{Cells: []parts.PresentationTableCell{{GridSpan: 2}, {GridSpan: 1}}}}}
	if got := TableColumnCount(table); got != 3 {
		t.Fatalf("column count = %d, want 3", got)
	}
}

func TestMergeTableTextDefaultsPreservesHigherPriorityValues(t *testing.T) {
	bold := true
	italic := true
	dst := TableTextDefaults{Bold: &bold, Color: "112233"}
	src := TableTextDefaults{Italic: &italic, FontFamily: "Aptos", Color: "AABBCC"}
	got := MergeTableTextDefaults(dst, src)
	if got.Bold == nil || !*got.Bold || got.Italic == nil || !*got.Italic {
		t.Fatalf("merged emphasis = %+v", got)
	}
	if got.Color != "112233" {
		t.Fatalf("higher-priority color = %q, want 112233", got.Color)
	}
	if got.FontFamily != "Aptos" {
		t.Fatalf("inherited font = %q, want Aptos", got.FontFamily)
	}
}

func TestTableSolidFillColorPreservesGradientRepresentativeColor(t *testing.T) {
	fill := drawing.GradientFill{Stops: []drawing.GradientStop{{Color: "336699"}, {Color: "FFFFFF"}}}
	if got := tableSolidFillColor(fill); got != "336699" {
		t.Fatalf("gradient representative color = %q, want 336699", got)
	}
}
