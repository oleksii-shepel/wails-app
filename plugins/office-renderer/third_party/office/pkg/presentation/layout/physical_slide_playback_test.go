package layout

import (
	"office/pkg/presentation/ir"
	"testing"
)

func TestMaterializePhysicalSlideOwnsPlaybackComposition(t *testing.T) {
	shape := &ir.Shape{ID: 7, AnimStep: 3}
	slide := &ir.Slide{
		Shapes:            []*ir.Shape{shape},
		Transition:        &ir.SlideTransition{Type: "push", Spd: "slow", AdvClick: false, AdvTmMs: 2500},
		AnimationTimeline: []ir.AnimationEvent{{ShapeID: 7, Step: 3, Effect: "appear"}},
	}
	got := MaterializePhysicalSlide(slide)
	if got.Playback.TransitionType != "push" || got.Playback.TransitionSpeed != "slow" || got.Playback.AdvanceClick || got.Playback.AdvanceTimeMs != 2500 {
		t.Fatalf("transition playback not materialized: %+v", got.Playback)
	}
	if got.Playback.MaxAnimationStep != 3 || !got.Playback.HasAnimationTimeline() {
		t.Fatalf("animation playback not materialized: %+v", got.Playback)
	}
	if _, ok := got.Playback.AnimationTargetShapeIDs[7]; !ok {
		t.Fatalf("animation target identity not materialized: %+v", got.Playback.AnimationTargetShapeIDs)
	}
}

func TestMaterializePhysicalSlidePreservesSemanticSourceIdentity(t *testing.T) {
	content := &ir.Shape{ID: 1, Kind: ir.ShapeText, Bounds: ir.Rect{W: 127000, H: 63500}}
	number := &ir.Shape{ID: 2, Kind: ir.ShapeText, Bounds: ir.Rect{W: 127000, H: 63500}}
	got := MaterializePhysicalSlide(&ir.Slide{Shapes: []*ir.Shape{content}, SlideNumber: number})
	if len(got.Content) != 1 || got.Content[0].SourceShape != content {
		t.Fatalf("content semantic identity lost: %+v", got.Content)
	}
	if got.SlideNumber == nil || got.SlideNumber.SourceShape != number {
		t.Fatalf("slide-number semantic identity lost: %+v", got.SlideNumber)
	}
}
