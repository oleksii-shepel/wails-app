package legacy

import (
	"testing"

	officeartescher "office/pkg/officeart/legacy/escher"
)

func TestParseLegacyPptFOPTPropertiesTransformUsesSharedOfficeArtArrowEnums(t *testing.T) {
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

	if got.ArrowHead != "stealth" || got.ArrowHeadWidth != "med" || got.ArrowHeadLength != "med" {
		t.Fatalf("head/start arrow = (%q,%q,%q), want (stealth,med,med)",
			got.ArrowHead, got.ArrowHeadWidth, got.ArrowHeadLength)
	}
	if got.ArrowTail != "oval" || got.ArrowTailWidth != "med" || got.ArrowTailLength != "med" {
		t.Fatalf("tail/end arrow = (%q,%q,%q), want (oval,med,med)",
			got.ArrowTail, got.ArrowTailWidth, got.ArrowTailLength)
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
	if got.ArrowHead != "stealth" {
		t.Fatalf("ArrowHead = %q, want stealth", got.ArrowHead)
	}
}
