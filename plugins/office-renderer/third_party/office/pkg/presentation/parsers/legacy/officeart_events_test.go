package legacy

import "testing"

func TestWalkOfficeArtEventsEmitsShapeContainersOnce(t *testing.T) {
	sp := buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F,
		buildOfficeArtRecord(0, legacyOfficeArtSp, 0x0002, []byte{1, 2, 3, 4, 5, 6, 7, 8}))
	group := buildOfficeArtRecord(0, legacyOfficeArtSpgrContainer, 0x000F, sp)
	var starts, ends int
	if err := WalkOfficeArtEvents(group, OfficeArtConsumerFunc(func(e OfficeArtEvent) error {
		if e.ShapeStart {
			starts++
			if len(e.Payload) == 0 {
				t.Fatal("shape start lost container payload")
			}
		}
		if e.ShapeEnd {
			ends++
		}
		return nil
	})); err != nil {
		t.Fatal(err)
	}
	if starts != 1 || ends != 1 {
		t.Fatalf("shape events = %d/%d, want 1/1", starts, ends)
	}
}
