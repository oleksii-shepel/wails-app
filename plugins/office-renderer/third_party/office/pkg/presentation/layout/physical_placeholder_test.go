package layout

import (
	"office/pkg/presentation/ir"
	"testing"
)

func TestPhysicalVisualOwnsPlaceholderPresentationRole(t *testing.T) {
	shape := &ir.Shape{Placeholder: ir.PlaceholderTitle}
	got := ResolvePhysicalPlaceholderClass(shape)
	if got != "ph-title" {
		t.Fatalf("title placeholder class = %q", got)
	}
}

func TestPresentationVisualDefaultsTextColor(t *testing.T) {
	got := ResolvePresentationVisualPlan(nil)
	if got.DefaultTextColor != "000000" {
		t.Fatalf("default text color = %q", got.DefaultTextColor)
	}
}
