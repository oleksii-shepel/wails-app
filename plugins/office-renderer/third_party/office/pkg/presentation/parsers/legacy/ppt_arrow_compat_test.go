package legacy

import (
	"testing"

	officeartescher "office/pkg/officeart/legacy/escher"
)

func TestParseLegacyPptFOPTPropertiesTransformPreservesPPTArrowEnums(t *testing.T) {
	props := []officeartescher.Property{
		{ID: officeartescher.PropLineStartArrow, Value: 2},
		{ID: officeartescher.PropLineStartArrowWidth, Value: 3},
		{ID: officeartescher.PropLineStartArrowLength, Value: 1},
		{ID: officeartescher.PropLineEndArrow, Value: 4},
		{ID: officeartescher.PropLineEndArrowWidth, Value: 1},
		{ID: officeartescher.PropLineEndArrowLength, Value: 3},
	}

	var got legacyPptShapeTransform
	parseLegacyPptFOPTPropertiesTransform(props, nil, &got)

	if got.ArrowTail != "triangle" || got.ArrowTailWidth != "lg" || got.ArrowTailLength != "sm" {
		t.Fatalf("tail arrow = (%q,%q,%q), want (triangle,lg,sm)",
			got.ArrowTail, got.ArrowTailWidth, got.ArrowTailLength)
	}
	if got.ArrowHead != "stealth" || got.ArrowHeadWidth != "sm" || got.ArrowHeadLength != "lg" {
		t.Fatalf("head arrow = (%q,%q,%q), want (stealth,sm,lg)",
			got.ArrowHead, got.ArrowHeadWidth, got.ArrowHeadLength)
	}
}

func TestParseLegacyPptFOPTPropertiesTransformKeepsSharedNonArrowVisuals(t *testing.T) {
	props := []officeartescher.Property{
		{ID: officeartescher.PropLineWidth, Value: 25400},
		{ID: officeartescher.PropLineDashing, Value: 7},
		{ID: officeartescher.PropLineStartArrow, Value: 2},
	}

	var got legacyPptShapeTransform
	parseLegacyPptFOPTPropertiesTransform(props, nil, &got)

	if got.LineWidthEMU != 25400 {
		t.Fatalf("LineWidthEMU = %d, want 25400", got.LineWidthEMU)
	}
	if got.ArrowTail != "triangle" {
		t.Fatalf("ArrowTail = %q, want triangle", got.ArrowTail)
	}
}
