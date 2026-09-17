package event

import (
	"reflect"
	"testing"

	"office/pkg/presentation/ir"
)

func TestEmitPresentationPreservesTreeOrderAndDepth(t *testing.T) {
	leaf1 := &ir.Shape{ID: 11, Kind: ir.ShapeText}
	leaf2 := &ir.Shape{ID: 12, Kind: ir.ShapeImage}
	group := &ir.Shape{ID: 10, Kind: ir.ShapeGroup, Children: []*ir.Shape{leaf1, leaf2}}
	p := &ir.Presentation{Slides: []*ir.Slide{{Shapes: []*ir.Shape{group}}}}
	var got []string
	err := EmitPresentation("ppt", p, ConsumerFunc(func(e Event) error {
		got = append(got, string(e.Type)+":"+itoa(e.Depth))
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"startPresentation:0", "startSlide:0", "startGroup:0", "shape:1", "shape:1", "endGroup:0", "endSlide:0", "endPresentation:0"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	if v == 1 {
		return "1"
	}
	return "?"
}
