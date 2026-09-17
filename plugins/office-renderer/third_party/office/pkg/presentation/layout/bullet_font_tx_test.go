package layout

import (
	"office/pkg/presentation/ir"
	"testing"
)

func TestBulletFontTxUsesParagraphTextFontInsteadOfInheritedWingdings(t *testing.T) {
	p := &ir.Paragraph{
		AutoNum: true, AutoNumType: "arabicPeriod", AutoNumStart: 5,
		BulletFontFamily: "Wingdings 3", BulletFontTx: true,
		Runs: []*ir.Run{{Text: "If a straight line", FontFamily: "Lucida Sans Unicode", FontSize: 2800}},
	}
	plan := ResolveParagraphVisualPlan(p, 600, 1, 1)
	if plan.Marker.FontFamily != "Lucida Sans Unicode" {
		t.Fatalf("marker font = %q, want paragraph text font", plan.Marker.FontFamily)
	}
}
