package sax

import (
	"testing"

	"office/pkg/ooxml/drawing"
	"office/pkg/presentation/ir"
)

func TestIRBuilderPublishesCustomGeometryWithoutSyntaxSentinel(t *testing.T) {
	b := newIRBuilder()
	b.slide = &ir.Slide{}
	if err := b.HandleEvent(Event{Type: EventStartShape, ShapeID: 7, ShapeKind: "shape"}); err != nil {
		t.Fatal(err)
	}
	geom := &drawing.Geometry{Type: "custom", Paths: []drawing.Path{{Commands: []drawing.PathCommand{{Type: "M"}, {Type: "L"}}}}}
	if err := b.HandleEvent(Event{Type: EventGeometry, Geometry: "custom", GeometryData: geom}); err != nil {
		t.Fatal(err)
	}
	if err := b.HandleEvent(Event{Type: EventEndShape, ShapeID: 7}); err != nil {
		t.Fatal(err)
	}
	if len(b.slide.Shapes) != 1 {
		t.Fatalf("shapes = %d, want 1", len(b.slide.Shapes))
	}
	got := b.slide.Shapes[0]
	if got.Geometry != "" || got.GeometryData == nil || got.GeometryData.Type != "custom" {
		t.Fatalf("canonical custom geometry = Geometry %q Data %#v", got.Geometry, got.GeometryData)
	}
}
