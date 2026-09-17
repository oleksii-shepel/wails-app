package layout

import (
	ir "office/pkg/presentation/ir"
	"reflect"
	"testing"
)

func TestSpecializedPhysicalPlansShareCompositionContract(t *testing.T) {
	types := []reflect.Type{
		reflect.TypeOf(PhysicalSmartArtPlan{}),
		reflect.TypeOf(PhysicalTablePlan{}),
		reflect.TypeOf(PhysicalMediaPlan{}),
		reflect.TypeOf(PhysicalOLEPlan{}),
		reflect.TypeOf(PhysicalChartPlan{}),
		reflect.TypeOf(TextVisualPlan{}),
	}
	for _, typ := range types {
		field, ok := typ.FieldByName("PhysicalCompositionPlan")
		if !ok || !field.Anonymous || field.Type != reflect.TypeOf(PhysicalCompositionPlan{}) {
			t.Fatalf("%s must embed PhysicalCompositionPlan", typ.Name())
		}
		for _, name := range []string{"FrameBounds", "Paint", "Transform", "Effects"} {
			resolved, ok := typ.FieldByName(name)
			if !ok {
				t.Fatalf("%s does not expose composition field %s", typ.Name(), name)
			}
			if len(resolved.Index) < 2 {
				t.Fatalf("%s retained direct composition field %s", typ.Name(), name)
			}
		}
	}
}

func TestPhysicalCompositionResolverOwnsCommonOuterState(t *testing.T) {
	plan := resolvePhysicalCompositionPlan(nil)
	if !plan.FrameBounds.Empty() || plan.Transform != (PhysicalTransformPlan{}) || plan.Effects.HasEffects || plan.Paint.ZIndex != 0 {
		t.Fatalf("nil composition = %+v", plan)
	}
}


func TestClass16PhysicalTableOwnsMergedTopologyAndCellPaint(t *testing.T) {
	shape := &ir.Shape{
		Kind: ir.ShapeTable, Bounds: ir.Rect{W: 2540000, H: 1270000},
		Table: &ir.Table{
			ColumnWidthsEMU: []int64{1270000, 1270000},
			Rows: []*ir.TableRow{
				{HeightEMU: 635000, Cells: []*ir.TableCell{
					{GridSpan: 2, FillColor: "112233"},
					{HMerge: true},
				}},
				{HeightEMU: 635000, Cells: []*ir.TableCell{{}, {}}},
			},
		},
	}
	plan := ResolvePhysicalTablePlan(shape)
	if plan == nil || len(plan.Rows) != 2 || len(plan.Rows[0].Cells) != 2 {
		t.Fatalf("physical table topology missing: %+v", plan)
	}
	if plan.Rows[0].Cells[0].ColumnSpan != 2 || !plan.Rows[0].Cells[1].Skip {
		t.Fatalf("merged topology lost: %+v", plan.Rows[0].Cells)
	}
	if len(plan.ColumnWidthsPercent) != 2 || plan.ColumnWidthsPercent[0] != 50 || plan.ColumnWidthsPercent[1] != 50 {
		t.Fatalf("column geometry not materialized: %+v", plan.ColumnWidthsPercent)
	}
}

func TestClass16PhysicalChartCopiesArtifactPayload(t *testing.T) {
	shape := &ir.Shape{
		Kind: ir.ShapeChart, Bounds: ir.Rect{W: 1270000, H: 635000},
		ImageMIME: "image/png", ImageData: []byte{1,2,3}, Name: "chart",
	}
	plan := ResolvePhysicalChartPlan(shape)
	if plan == nil || plan.MIME != "image/png" || len(plan.Data) != 3 {
		t.Fatalf("chart artifact missing: %+v", plan)
	}
	shape.ImageData[0] = 9
	if plan.Data[0] != 1 {
		t.Fatal("physical chart aliases canonical artifact bytes")
	}
}


func TestClass17SlidePaintOrderPreservesContentDecorationInterleaving(t *testing.T) {
	a := &ir.Shape{Kind: ir.ShapeText, Name: "content", Bounds: ir.Rect{W: 127000, H: 127000},
		Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{Text: "A"}}}}}
	footer := &ir.Shape{Kind: ir.ShapeText, Name: "footer", Bounds: ir.Rect{W: 127000, H: 127000},
		Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{Text: "F"}}}}}
	number := &ir.Shape{Kind: ir.ShapeText, Name: "number", Bounds: ir.Rect{W: 127000, H: 127000},
		Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{Text: "1"}}}}}
	slide := &ir.Slide{Shapes: []*ir.Shape{a}, Footer: footer, SlideNumber: number}
	plan := MaterializePhysicalSlide(slide)
	if len(plan.PaintOrder) != 3 {
		t.Fatalf("paint order len=%d, want 3", len(plan.PaintOrder))
	}
	if plan.PaintOrder[0].SourceShape != a || plan.PaintOrder[1].SourceShape != footer ||
		plan.PaintOrder[2].SourceShape != number {
		t.Fatalf("slide paint order regrouped semantic categories: %+v", plan.PaintOrder)
	}
	for i, item := range plan.PaintOrder {
		if item.Drawing == nil || item.Drawing.Paint.ZIndex != i+1 {
			t.Fatalf("paint %d z-index=%v, want %d", i, item.Drawing, i+1)
		}
	}
}

func TestClass17FlattenedGroupChildrenRetainDepthFirstAuthoredPaintOrder(t *testing.T) {
	child1 := &ir.Shape{Kind: ir.ShapeText, Name: "g1", Bounds: ir.Rect{W: 127000, H: 127000},
		Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{Text: "1"}}}}}
	child2 := &ir.Shape{Kind: ir.ShapeText, Name: "g2", Bounds: ir.Rect{W: 127000, H: 127000},
		Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{Text: "2"}}}}}
	group := &ir.Shape{Kind: ir.ShapeGroup, Name: "g", Bounds: ir.Rect{W: 254000, H: 254000}, Children: []*ir.Shape{child1, child2}}
	after := &ir.Shape{Kind: ir.ShapeText, Name: "after", Bounds: ir.Rect{W: 127000, H: 127000},
		Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{Text: "3"}}}}}
	got := MaterializePhysicalVisualTree([]*ir.Shape{group, after})
	if len(got) != 3 || got[0].SourceShape != child1 || got[1].SourceShape != child2 || got[2].SourceShape != after {
		t.Fatalf("flattening changed authored paint order: %+v", got)
	}
	for i := range got {
		if got[i].Drawing.Paint.ZIndex != i+1 {
			t.Fatalf("flattened z-index[%d]=%d, want %d", i, got[i].Drawing.Paint.ZIndex, i+1)
		}
	}
}


func TestClass18SuppressedLeavesDoNotConsumePhysicalZIndex(t *testing.T) {
	empty := &ir.Shape{Kind: ir.ShapeText, Placeholder: ir.PlaceholderTitle,
		Bounds: ir.Rect{W: 127000, H: 127000}}
	visible := &ir.Shape{Kind: ir.ShapeText, Name: "visible",
		Bounds: ir.Rect{W: 127000, H: 127000},
		Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{Text: "visible"}}}}}
	got := MaterializePhysicalVisualTree([]*ir.Shape{empty, visible})
	if len(got) != 1 {
		t.Fatalf("physical visuals=%d, want 1", len(got))
	}
	if got[0].Drawing == nil || got[0].Drawing.Paint.ZIndex != 1 {
		t.Fatalf("first surviving visual z-index=%v, want 1", got[0].Drawing)
	}
}


func TestClass19PhysicalVisualOwnsRenderClassification(t *testing.T) {
	shape := &ir.Shape{
		Kind: ir.ShapeText,
		Bounds: ir.Rect{W: 127000, H: 127000},
		Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{Text: "physical dispatch"}}}},
	}
	got := MaterializePhysicalVisualTree([]*ir.Shape{shape})
	if len(got) != 1 {
		t.Fatalf("physical visuals=%d, want 1", len(got))
	}
	if got[0].Kind != ir.ShapeText {
		t.Fatalf("physical kind=%v, want text", got[0].Kind)
	}
}
