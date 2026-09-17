package layout

import (
	"office/pkg/ooxml/drawing"
	"office/pkg/presentation/ir"
	"testing"
)

func TestPhysicalTablePlanOwnsGridMergeAndCellLayout(t *testing.T) {
	shape := &ir.Shape{Kind: ir.ShapeTable, Table: &ir.Table{
		ColumnWidthsEMU: []int64{12700, 25400},
		Rows:            []*ir.TableRow{{HeightEMU: 25400, Cells: []*ir.TableCell{{GridSpan: 2, RowSpan: 3, TextAnchor: "ctr", MarginLeftEMU: 12700, MarginTopEMU: 25400}}}},
	}}
	plan := ResolvePhysicalTablePlan(shape)
	if plan == nil || len(plan.ColumnWidthsPercent) != 2 || len(plan.Rows) != 1 {
		t.Fatalf("table plan = %#v", plan)
	}
	if plan.ColumnWidthsPercent[0] < 33.3 || plan.ColumnWidthsPercent[0] > 33.4 {
		t.Fatalf("first column = %v", plan.ColumnWidthsPercent[0])
	}
	if plan.Rows[0].HeightPt != 2 {
		t.Fatalf("row height = %v", plan.Rows[0].HeightPt)
	}
	cell := plan.Rows[0].Cells[0]
	if cell.ColumnSpan != 2 || cell.RowSpan != 3 || cell.VerticalAlign != "middle" {
		t.Fatalf("cell plan = %#v", cell)
	}
	if cell.PaddingLeftPt != 1 || cell.PaddingTopPt != 2 {
		t.Fatalf("cell padding = %#v", cell)
	}
}

func TestPhysicalChartPlanOwnsEmbeddedChartArtifact(t *testing.T) {
	shape := &ir.Shape{Kind: ir.ShapeChart, Name: "Revenue", ImageMIME: "image/svg+xml", ImageData: []byte("<svg/>")}
	plan := ResolvePhysicalChartPlan(shape)
	if plan == nil || plan.MIME != "image/svg+xml" || plan.Alt != "Revenue" || string(plan.Data) != "<svg/>" {
		t.Fatalf("chart plan = %#v", plan)
	}
	shape.ImageData[0] = 'X'
	if string(plan.Data) != "<svg/>" {
		t.Fatal("physical chart plan aliases canonical image bytes")
	}
}

func TestPhysicalTableBorderUsesSharedStrokeSemantics(t *testing.T) {
	shape := &ir.Shape{Kind: ir.ShapeTable, Table: &ir.Table{ColumnWidthsEMU: []int64{1270000}, Rows: []*ir.TableRow{{HeightEMU: 127000, Cells: []*ir.TableCell{{
		BorderLeft: &drawing.LineStyle{Width: 2, Color: "336699", Alpha: 0.4, Dash: drawing.DashPatternDash},
	}}}}}}
	plan := ResolvePhysicalTablePlan(shape)
	if len(plan.Edges) != 1 {
		t.Fatalf("physical edge count = %d, want 1", len(plan.Edges))
	}
	stroke := plan.Edges[0].Stroke
	if stroke.WidthPt != 2 || stroke.Color != "336699" || stroke.Alpha != 0.4 || stroke.CSSStyle != "dashed" {
		t.Fatalf("physical table border lost DrawingML stroke semantics: %#v", stroke)
	}
	if stroke.DashArray == "" {
		t.Fatal("physical table border must retain materialized dash metrics")
	}
}

func TestPhysicalTableCellOwnsSpannedContentHeightAndVerticalPlacement(t *testing.T) {
	shape := &ir.Shape{Kind: ir.ShapeTable, Table: &ir.Table{
		ColumnWidthsEMU: []int64{1270000},
		Rows: []*ir.TableRow{
			{HeightEMU: 254000, Cells: []*ir.TableCell{{RowSpan: 2, TextAnchor: "b", MarginTopEMU: 12700, MarginBottomEMU: 25400}}},
			{HeightEMU: 381000, Cells: []*ir.TableCell{{VMerge: true}}},
		},
	}}
	cell := ResolvePhysicalTablePlan(shape).Rows[0].Cells[0]
	if cell.CellHeightPt != 50 {
		t.Fatalf("spanned cell height = %v, want 50pt", cell.CellHeightPt)
	}
	if cell.ContentHeightPt != 47 {
		t.Fatalf("cell content height = %v, want 47pt", cell.ContentHeightPt)
	}
	if cell.JustifyContent != "flex-end" || !cell.ClipContent {
		t.Fatalf("physical cell placement = %#v", cell)
	}
}

func TestPhysicalTableSharedEdgeIsResolvedOnce(t *testing.T) {
	shape := &ir.Shape{Kind: ir.ShapeTable, Table: &ir.Table{ColumnWidthsEMU: []int64{1270000, 1270000}, Rows: []*ir.TableRow{{HeightEMU: 127000, Cells: []*ir.TableCell{
		{BorderRight: &drawing.LineStyle{Width: 1, Color: "111111"}},
		{BorderLeft: &drawing.LineStyle{Width: 3, Color: "336699", Dash: drawing.DashPatternDash}},
	}}}}}
	plan := ResolvePhysicalTablePlan(shape)
	if len(plan.Edges) != 1 {
		t.Fatalf("shared edge count = %d, want 1", len(plan.Edges))
	}
	e := plan.Edges[0]
	if e.X1Pct != 50 || e.X2Pct != 50 || e.Stroke.WidthPt != 3 || e.Stroke.Color != "336699" || e.Stroke.CSSStyle != "dashed" {
		t.Fatalf("shared edge winner = %#v", e)
	}
}

func TestPhysicalTableHorizontalSharedEdgeIsResolvedOnce(t *testing.T) {
	shape := &ir.Shape{Kind: ir.ShapeTable, Table: &ir.Table{ColumnWidthsEMU: []int64{1270000}, Rows: []*ir.TableRow{
		{HeightEMU: 127000, Cells: []*ir.TableCell{{BorderBottom: &drawing.LineStyle{Width: 2, Color: "AA0000"}}}},
		{HeightEMU: 127000, Cells: []*ir.TableCell{{BorderTop: &drawing.LineStyle{Width: 1, Color: "00AA00"}}}},
	}}}
	plan := ResolvePhysicalTablePlan(shape)
	if len(plan.Edges) != 1 {
		t.Fatalf("horizontal shared edge count = %d, want 1", len(plan.Edges))
	}
	e := plan.Edges[0]
	if e.Y1Pct != 50 || e.Y2Pct != 50 || e.Stroke.WidthPt != 2 || e.Stroke.Color != "AA0000" {
		t.Fatalf("horizontal shared edge = %#v", e)
	}
}

func TestPhysicalTableMergedCellEdgesResolvePerGridSegment(t *testing.T) {
	shape := &ir.Shape{Kind: ir.ShapeTable, Table: &ir.Table{
		ColumnWidthsEMU: []int64{1270000, 1270000, 1270000},
		Rows: []*ir.TableRow{{HeightEMU: 127000, Cells: []*ir.TableCell{
			{GridSpan: 2, BorderRight: &drawing.LineStyle{Width: 2, Color: "111111"}}, {HMerge: true},
			{BorderLeft: &drawing.LineStyle{Width: 3, Color: "222222"}},
		}}},
	}}
	plan := ResolvePhysicalTablePlan(shape)
	if len(plan.Edges) != 1 {
		t.Fatalf("merged shared segment count = %d, want 1: %#v", len(plan.Edges), plan.Edges)
	}
	e := plan.Edges[0]
	if e.X1Pct < 66.6 || e.X1Pct > 66.7 || e.X2Pct != e.X1Pct || e.Stroke.WidthPt != 3 || e.Stroke.Color != "222222" {
		t.Fatalf("merged shared segment winner = %#v", e)
	}
}

func TestPhysicalChartPlanOwnsFrameTransformAndEffects(t *testing.T) {
	shape := &ir.Shape{
		Kind: ir.ShapeChart, Name: "Revenue", Bounds: ir.Rect{W: 914400, H: 457200},
		ImageMIME: "image/svg+xml", ImageData: []byte("<svg/>"), RotationDeg: 30, FlipH: true,
		Effects: drawing.EffectList{Effects: []drawing.Effect{drawing.Shadow{Color: "112233", Alpha: 0.5, Blur: 2}}},
	}
	plan := ResolvePhysicalChartPlan(shape)
	if plan == nil {
		t.Fatal("chart plan = nil")
	}
	if plan.WidthPt != 72 || plan.HeightPt != 36 || plan.ObjectFit != "fill" || !plan.ClipContent {
		t.Fatalf("chart frame = %#v", plan)
	}
	if plan.Transform.RotationDeg != 30 || !plan.Transform.FlipH {
		t.Fatalf("chart transform = %#v", plan.Transform)
	}
	if !plan.Effects.HasEffects {
		t.Fatalf("chart effects = %#v", plan.Effects)
	}
}

func TestPhysicalChartPlanOwnsSlideSpaceFrameBounds(t *testing.T) {
	shape := &ir.Shape{
		Kind:      ir.ShapeChart,
		Bounds:    ir.Rect{X: 12700, Y: 25400, W: 1270000, H: 635000},
		ImageMIME: "image/svg+xml", ImageData: []byte("<svg/>"),
	}
	plan := ResolvePhysicalChartPlan(shape)
	if plan == nil {
		t.Fatal("chart plan = nil")
	}
	if plan.FrameBounds.LeftPt != 1 || plan.FrameBounds.TopPt != 2 || plan.FrameBounds.WidthPt != 100 || plan.FrameBounds.HeightPt != 50 {
		t.Fatalf("chart frame bounds = %+v, want left=1 top=2 width=100 height=50", plan.FrameBounds)
	}
}

func TestMaterializedChartPlanOwnsPaintComposition(t *testing.T) {
	shape := &ir.Shape{
		Kind:      ir.ShapeChart,
		Bounds:    ir.Rect{X: 12700, Y: 25400, W: 1270000, H: 635000},
		ImageMIME: "image/svg+xml", ImageData: []byte("<svg/>"),
		Effects: drawing.EffectList{Effects: []drawing.Effect{drawing.Shadow{Color: "112233", Alpha: .5, Blur: 2}}},
	}
	visuals := MaterializePhysicalVisualTree([]*ir.Shape{shape})
	if len(visuals) != 1 || visuals[0].Chart == nil {
		t.Fatalf("materialized chart = %#v", visuals)
	}
	chart := visuals[0].Chart
	if chart.Paint.ZIndex != 1 || !chart.Paint.OverflowVisible {
		t.Fatalf("chart paint composition = %+v", chart.Paint)
	}
	if chart.Paint.Bounds.WidthPt <= chart.FrameBounds.WidthPt || chart.Paint.Bounds.HeightPt <= chart.FrameBounds.HeightPt {
		t.Fatalf("effect paint bounds = %+v, frame = %+v", chart.Paint.Bounds, chart.FrameBounds)
	}
}

func TestPhysicalTablePlanOwnsSlideComposition(t *testing.T) {
	shape := &ir.Shape{
		Kind:        ir.ShapeTable,
		Bounds:      ir.Rect{X: 12700, Y: 25400, W: 1270000, H: 635000},
		RotationDeg: 17,
		FlipH:       true,
		Table:       &ir.Table{Rows: []*ir.TableRow{{Cells: []*ir.TableCell{{}}}}},
	}
	plan := ResolvePhysicalTablePlan(shape)
	if plan == nil {
		t.Fatal("physical table plan = nil")
	}
	if plan.FrameBounds.LeftPt != 1 || plan.FrameBounds.TopPt != 2 || plan.FrameBounds.WidthPt != 100 || plan.FrameBounds.HeightPt != 50 {
		t.Fatalf("table frame bounds = %+v", plan.FrameBounds)
	}
	if plan.Transform.RotationDeg != 17 || !plan.Transform.FlipH {
		t.Fatalf("table transform = %+v", plan.Transform)
	}
}

func TestMaterializedPhysicalTableCarriesPaintPlan(t *testing.T) {
	shape := &ir.Shape{Kind: ir.ShapeTable, Bounds: ir.Rect{W: 1270000, H: 635000}, Table: &ir.Table{Rows: []*ir.TableRow{{Cells: []*ir.TableCell{{}}}}}}
	visuals := MaterializePhysicalVisualTree([]*ir.Shape{shape})
	if len(visuals) != 1 || visuals[0].Table == nil {
		t.Fatal("table was not materialized")
	}
	if visuals[0].Table.Paint.ZIndex != visuals[0].Drawing.Paint.ZIndex || visuals[0].Table.Paint.ZIndex == 0 {
		t.Fatalf("table paint plan = %+v, visual paint = %+v", visuals[0].Table.Paint, visuals[0].Drawing.Paint)
	}
}

func TestPhysicalSmartArtPlanOwnsSlideComposition(t *testing.T) {
	shape := &ir.Shape{
		Kind:        ir.ShapeSmartArt,
		Bounds:      ir.Rect{X: 12700, Y: 25400, W: 1270000, H: 635000},
		RotationDeg: 23,
		FlipV:       true,
		Effects:     drawing.EffectList{Effects: []drawing.Effect{drawing.Shadow{Color: "334455", Alpha: .5, Blur: 2}}},
	}
	plan := ResolvePhysicalSmartArtPlan(shape)
	if plan == nil {
		t.Fatal("physical SmartArt plan = nil")
	}
	if plan.FrameBounds.LeftPt != 1 || plan.FrameBounds.TopPt != 2 || plan.FrameBounds.WidthPt != 100 || plan.FrameBounds.HeightPt != 50 {
		t.Fatalf("SmartArt frame bounds = %+v", plan.FrameBounds)
	}
	if plan.Transform.RotationDeg != 23 || !plan.Transform.FlipV {
		t.Fatalf("SmartArt transform = %+v", plan.Transform)
	}
	if !plan.Effects.HasEffects {
		t.Fatalf("SmartArt effects = %+v", plan.Effects)
	}
}

func TestMaterializedPhysicalSmartArtCarriesPaintPlan(t *testing.T) {
	shape := &ir.Shape{
		Kind:    ir.ShapeSmartArt,
		Bounds:  ir.Rect{W: 1270000, H: 635000},
		Effects: drawing.EffectList{Effects: []drawing.Effect{drawing.Shadow{Color: "112233", Alpha: .5, Blur: 2}}},
	}
	visuals := MaterializePhysicalVisualTree([]*ir.Shape{shape})
	if len(visuals) != 1 || visuals[0].SmartArt == nil {
		t.Fatal("SmartArt was not materialized")
	}
	plan := visuals[0].SmartArt
	if plan.Paint.ZIndex != visuals[0].Drawing.Paint.ZIndex || plan.Paint.ZIndex == 0 {
		t.Fatalf("SmartArt paint plan = %+v, visual paint = %+v", plan.Paint, visuals[0].Drawing.Paint)
	}
	if !plan.Paint.OverflowVisible {
		t.Fatalf("SmartArt effect overflow was not materialized: %+v", plan.Paint)
	}
}
