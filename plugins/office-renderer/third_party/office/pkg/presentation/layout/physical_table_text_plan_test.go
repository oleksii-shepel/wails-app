package layout

import (
	"math"
	"testing"

	"office/pkg/ooxml/drawing"
	ir "office/pkg/presentation/ir"
)

func TestPhysicalTableCellOwnsParagraphLinePlans(t *testing.T) {
	shape := &ir.Shape{Table: &ir.Table{
		ColumnWidthsEMU: []int64{12700 * 100},
		Rows: []*ir.TableRow{{Cells: []*ir.TableCell{{
			MarginLeftEMU: 12700 * 5, MarginRightEMU: 12700 * 5,
			Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{Text: "alpha beta gamma", FontSize: 1200}}}},
		}}}},
	}}
	plan := ResolvePhysicalTablePlan(shape)
	if plan == nil || len(plan.Rows) != 1 || len(plan.Rows[0].Cells) != 1 {
		t.Fatal("missing physical table cell")
	}
	cell := plan.Rows[0].Cells[0]
	if len(cell.ParagraphPlans) != 1 || !cell.ParagraphPlans[0].LinesMeasured {
		t.Fatal("table cell text was not physically materialized")
	}
	if got := cell.ParagraphPlans[0].Metrics.AvailableWidthPt; got < 89.9 || got > 90.1 {
		t.Fatalf("cell paragraph width = %.2f, want 90pt", got)
	}
}

func TestPhysicalTableCellNumberingUsesExactMarkerBeforeWrap(t *testing.T) {
	shape := &ir.Shape{Table: &ir.Table{
		ColumnWidthsEMU: []int64{12700 * 80},
		Rows: []*ir.TableRow{{Cells: []*ir.TableCell{{Paragraphs: []*ir.Paragraph{
			{AutoNum: true, AutoNumStart: 9, AutoNumType: "arabicPeriod", Runs: []*ir.Run{{Text: "nine", FontSize: 1200}}},
			{AutoNum: true, AutoNumStart: 9, AutoNumType: "arabicPeriod", Runs: []*ir.Run{{Text: "ten", FontSize: 1200}}},
		}}}}},
	}}
	cell := ResolvePhysicalTablePlan(shape).Rows[0].Cells[0]
	if len(cell.ParagraphPlans) != 2 {
		t.Fatal("missing paragraph plans")
	}
	if cell.ParagraphPlans[0].Marker.Text != "9." || cell.ParagraphPlans[1].Marker.Text != "10." {
		t.Fatalf("markers = %q, %q", cell.ParagraphPlans[0].Marker.Text, cell.ParagraphPlans[1].Marker.Text)
	}
}

func TestPhysicalTableEdgesUseAuthoredFrameHeightCoordinateSystem(t *testing.T) {
	shape := &ir.Shape{
		Kind:   ir.ShapeTable,
		Bounds: ir.Rect{W: 12700 * 432, H: 12700 * 216},
		Table: &ir.Table{
			ColumnWidthsEMU: []int64{12700 * 216, 12700 * 216},
			Rows: []*ir.TableRow{
				{HeightEMU: 12700 * 54, Cells: []*ir.TableCell{{GridSpan: 1, RowSpan: 1, BorderBottom: &drawing.LineStyle{Width: 1, Color: "CC6600"}}, {GridSpan: 1, RowSpan: 1, BorderBottom: &drawing.LineStyle{Width: 1, Color: "CC6600"}}}},
				{HeightEMU: 12700 * 54, Cells: []*ir.TableCell{{GridSpan: 1, RowSpan: 1}, {GridSpan: 1, RowSpan: 1}}},
				{HeightEMU: 12700 * 54, Cells: []*ir.TableCell{{GridSpan: 1, RowSpan: 1}, {GridSpan: 1, RowSpan: 1}}},
			},
		},
	}
	plan := ResolvePhysicalTablePlan(shape)
	if plan == nil {
		t.Fatal("table plan = nil")
	}
	found := false
	for _, edge := range plan.Edges {
		if edge.Stroke.Color == "CC6600" && edge.Y1Pct == edge.Y2Pct {
			found = true
			if math.Abs(edge.Y1Pct-25) > 0.001 {
				t.Fatalf("first-row edge y = %.4f%%, want 25%% of 216pt frame", edge.Y1Pct)
			}
		}
	}
	if !found {
		t.Fatal("first-row bottom edge not materialized")
	}
}
