package legacy

import (
	"testing"

	officeartescher "office/pkg/officeart/legacy/escher"
	presentationir "office/pkg/presentation/ir"
)

func TestParserCompletenessConnectorFamilyClaimsBeforeGenericFallback(t *testing.T) {
	connector := &officeartescher.ShapeNode{
		ShapeID:      601,
		ShapeType:    20,
		ClientAnchor: pass5ClientAnchor(10, 20, 300, 40),
	}
	scene := legacyPptVisualScene{dispatch: legacyPptSlideVisualDispatch{containers: legacyPptShapeContainerScene{
		containers: []legacyPptShapeContainer{{ShapeID: 601, Node: connector}},
	}}}

	projection := scene.completeProjection(legacySlideText{}, nil, nil, nil, nil, nil, nil)
	var matches []*presentationir.Shape
	for _, shape := range projection.Shapes {
		if shape != nil && shape.OriginalShapeID == 601 {
			matches = append(matches, shape)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("shape 601 projections = %d, want exactly one family owner", len(matches))
	}
	if matches[0].Kind != presentationir.ShapeConnector {
		t.Fatalf("shape 601 kind = %d, want connector", matches[0].Kind)
	}
	if gaps := projection.unclaimed(); len(gaps) != 0 {
		t.Fatalf("connector family left ownership gaps: %+v", gaps)
	}
}

func TestParserCompletenessGenericFallbackStillOwnsOrdinaryAutoShape(t *testing.T) {
	ordinary := &officeartescher.ShapeNode{
		ShapeID:      602,
		ShapeType:    1,
		ClientAnchor: pass5ClientAnchor(40, 50, 200, 100),
	}
	scene := legacyPptVisualScene{dispatch: legacyPptSlideVisualDispatch{containers: legacyPptShapeContainerScene{
		containers: []legacyPptShapeContainer{{ShapeID: 602, Node: ordinary}},
	}}}

	projection := scene.completeProjection(legacySlideText{}, nil, nil, nil, nil, nil, nil)
	if len(projection.Shapes) != 1 || projection.Shapes[0].OriginalShapeID != 602 {
		t.Fatalf("ordinary AutoShape projection = %+v, want canonical fallback owner", projection.Shapes)
	}
	if gaps := projection.unclaimed(); len(gaps) != 0 {
		t.Fatalf("ordinary AutoShape left ownership gaps: %+v", gaps)
	}
}
