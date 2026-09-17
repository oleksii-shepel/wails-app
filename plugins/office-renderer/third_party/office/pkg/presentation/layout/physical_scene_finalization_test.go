package layout

import (
	"testing"

	"office/pkg/presentation/ir"
)

func TestMaterializePhysicalSlideOwnsRendererSceneIdentityAndBehaviorSummary(t *testing.T) {
	slide := &ir.Slide{Index: 4, SectionName: "Results", Shapes: []*ir.Shape{{ID: 7, Kind: ir.ShapeText, Bounds: ir.Rect{W: 127000, H: 63500}, AnimStep: 2, ActionLink: "https://example.test"}}}
	plan := MaterializePhysicalSlide(slide)
	if plan.Index != 4 || plan.SectionName != "Results" {
		t.Fatalf("physical scene identity = (%d, %q), want (4, Results)", plan.Index, plan.SectionName)
	}
	if plan.Playback.MaxAnimationStep != 2 || !plan.Playback.HasInteractiveBehavior {
		t.Fatalf("physical playback summary = %+v, want animation step and interactive behavior", plan.Playback)
	}
	if len(plan.Content) != 1 {
		t.Fatalf("physical scene content = %d, want 1", len(plan.Content))
	}
}
