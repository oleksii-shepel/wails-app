package sax

import (
	"testing"

	"office/pkg/ooxml/drawing"
	ir "office/pkg/presentation/ir"
)

func TestClass9ExplicitZeroBodyRotationOverridesInheritedRotation(t *testing.T) {
	master := &ir.Shape{TextBodyProps: drawing.TextBodyProperties{Rot: 5400000, HasRot: true}}
	slide := &ir.Shape{}
	applyTextBodyProps(slide, TextBodyProperties{Rot: 0, HasRot: true})
	inheritTextBodyFacts(slide, master)
	if !slide.TextBodyProps.HasRot || slide.TextBodyProps.Rot != 0 {
		t.Fatalf("explicit zero body rotation lost to inheritance: %+v", slide.TextBodyProps)
	}
}

func TestClass9OmittedBodyRotationInheritsAuthoredRotation(t *testing.T) {
	master := &ir.Shape{TextBodyProps: drawing.TextBodyProperties{Rot: 5400000, HasRot: true}}
	slide := &ir.Shape{}
	applyTextBodyProps(slide, TextBodyProperties{})
	inheritTextBodyFacts(slide, master)
	if !slide.TextBodyProps.HasRot || slide.TextBodyProps.Rot != 5400000 {
		t.Fatalf("omitted body rotation did not inherit: %+v", slide.TextBodyProps)
	}
}

func TestClass9ExplicitFalseAnchorCtrOverridesInheritedTrue(t *testing.T) {
	master := &ir.Shape{TextBodyProps: drawing.TextBodyProperties{AnchorCtr: true, HasAnchorCtr: true}}
	slide := &ir.Shape{}
	applyTextBodyProps(slide, TextBodyProperties{AnchorCtr: false, HasAnchorCtr: true})
	inheritTextBodyFacts(slide, master)
	if !slide.TextBodyProps.HasAnchorCtr || slide.TextBodyProps.AnchorCtr {
		t.Fatalf("explicit anchorCtr=false lost to inheritance: %+v", slide.TextBodyProps)
	}
}

func TestClass9OmittedAnchorCtrInheritsAuthoredTrue(t *testing.T) {
	master := &ir.Shape{TextBodyProps: drawing.TextBodyProperties{AnchorCtr: true, HasAnchorCtr: true}}
	slide := &ir.Shape{}
	applyTextBodyProps(slide, TextBodyProperties{})
	inheritTextBodyFacts(slide, master)
	if !slide.TextBodyProps.HasAnchorCtr || !slide.TextBodyProps.AnchorCtr {
		t.Fatalf("omitted anchorCtr did not inherit: %+v", slide.TextBodyProps)
	}
}
