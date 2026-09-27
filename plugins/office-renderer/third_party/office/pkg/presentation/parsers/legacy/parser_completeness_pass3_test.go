package legacy

import (
	"testing"

	officeartescher "office/pkg/officeart/legacy/escher"
)

func TestParserCompletenessProjectionCarriesOwnershipLedger(t *testing.T) {
	scene := legacyPptVisualScene{}
	scene.dispatch.containers.containers = []legacyPptShapeContainer{{
		ShapeID: 77,
		Node:    &officeartescher.ShapeNode{ShapeID: 77},
	}}
	projection := scene.completeProjection(legacySlideText{}, nil, nil, nil, nil, nil, nil)
	if len(projection.Ownership) != 1 {
		t.Fatalf("ownership entries = %d, want 1", len(projection.Ownership))
	}
	gaps := projection.unclaimed()
	if len(gaps) != 1 || gaps[0].ShapeID != 77 {
		t.Fatalf("projection must retain dropped-container gap: %+v", gaps)
	}
}

func TestParserCompletenessBackgroundIsNotCanonicalShape(t *testing.T) {
	const backgroundFlag uint32 = 0x00000400
	scene := legacyPptVisualScene{}
	scene.dispatch.containers.containers = []legacyPptShapeContainer{{
		ShapeID: 88,
		Node:    &officeartescher.ShapeNode{ShapeID: 88, Flags: backgroundFlag},
	}}
	projection := scene.completeProjection(legacySlideText{}, nil, nil, nil, nil, nil, nil)
	if len(projection.Ownership) != 1 || !projection.Ownership[0].Claimed || projection.Ownership[0].Reason != "background paint" {
		t.Fatalf("background ownership = %+v", projection.Ownership)
	}
}
