package legacy

import (
	"strings"
	"testing"

	officeartescher "office/pkg/officeart/legacy/escher"
	"office/pkg/ooxml/drawing"
	"office/pkg/presentation/ir"
)

func TestParserCompletenessOwnershipAuditClassifiesEverySpContainer(t *testing.T) {
	const backgroundFlag uint32 = 0x00000400
	scene := legacyPptVisualScene{}
	scene.dispatch.containers.containers = []legacyPptShapeContainer{
		{ShapeID: 10, Node: &officeartescher.ShapeNode{ShapeID: 10}},
		{ShapeID: 20, Node: &officeartescher.ShapeNode{ShapeID: 20, Flags: backgroundFlag}},
		{ShapeID: 30, IsGroupRoot: true, Node: &officeartescher.ShapeNode{ShapeID: 30}},
	}
	audit := scene.ownershipAudit([]*ir.Shape{{OriginalShapeID: 10}})
	if len(audit) != 3 {
		t.Fatalf("audit entries = %d, want 3", len(audit))
	}
	for _, entry := range audit {
		if !entry.Claimed {
			t.Fatalf("ShapeID %d unexpectedly unclaimed: %s", entry.ShapeID, entry.Reason)
		}
	}
	if audit[0].Reason != "canonical IR" || audit[1].Reason != "background paint" || audit[2].Reason != "structural: group root" {
		t.Fatalf("unexpected ownership audit: %+v", audit)
	}
}

func TestParserCompletenessOwnershipAuditExposesDroppedContainer(t *testing.T) {
	scene := legacyPptVisualScene{}
	scene.dispatch.containers.containers = []legacyPptShapeContainer{{ShapeID: 77, Node: &officeartescher.ShapeNode{ShapeID: 77}}}
	audit := scene.ownershipAudit(nil)
	if len(audit) != 1 || audit[0].Claimed || !strings.HasPrefix(audit[0].Reason, "unclaimed:") {
		t.Fatalf("dropped SpContainer must remain visible to completeness audit: %+v", audit)
	}
}

func TestMasterPictureBackgroundRespectsFollowMasterBackground(t *testing.T) {
	fill := drawing.PictureFill{ImageData: []byte{1, 2, 3}, ContentType: "image/png", Stretch: true, Alpha: 1}
	slides := []legacySlideText{{PersistID: 1, MasterPersistID: 9, HasSlideFlags: true, FollowMasterBackground: true}}
	pres := buildPresentationWithMasterBackgroundPaint(slides, nil, nil, legacyPptHeaderFooterInfo{}, "x.ppt", nil, nil, nil, "", nil, map[int]drawing.Fill{9: fill}, legacyPptDefaultCanvas())
	if len(pres.Slides) != 1 || pres.Slides[0].BgFill == nil {
		t.Fatal("master picture background was not inherited")
	}

	slides[0].FollowMasterBackground = false
	pres = buildPresentationWithMasterBackgroundPaint(slides, nil, nil, legacyPptHeaderFooterInfo{}, "x.ppt", nil, nil, nil, "", nil, map[int]drawing.Fill{9: fill}, legacyPptDefaultCanvas())
	if len(pres.Slides) != 1 || pres.Slides[0].BgFill != nil {
		t.Fatal("master picture background inherited despite fMasterBackground=false")
	}
}
