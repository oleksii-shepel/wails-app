package layout

import (
	"testing"

	"office/pkg/ooxml/drawing"
	"office/pkg/presentation/ir"
)

func TestImageVisualPlanMaterializesBlipEffects(t *testing.T) {
	shape := &ir.Shape{ImageEffects: []drawing.Effect{
		drawing.BiLevel{Threshold: 42000},
		drawing.Duotone{Color1: "112233", Color2: "AABBCC"},
		drawing.AlphaOutset{Radius: 3.5},
		drawing.ClrChange{From: "FF0000", To: "00FF00", FromAlpha: .5, ToAlpha: .75, UseAlpha: true},
	}}
	plan := ResolveImageVisualPlan(shape)
	if len(plan.Effects) != 4 {
		t.Fatalf("effects=%d, want 4", len(plan.Effects))
	}
	if plan.Effects[0].Type != string(drawing.EffectTypeBiLevel) || plan.Effects[0].Int1 != 42000 {
		t.Fatalf("bilevel=%+v", plan.Effects[0])
	}
	if plan.Effects[1].Color1 != "112233" || plan.Effects[1].Color2 != "AABBCC" {
		t.Fatalf("duotone=%+v", plan.Effects[1])
	}
	if plan.Effects[2].Float1 != 3.5 {
		t.Fatalf("alpha outset=%+v", plan.Effects[2])
	}
	if !plan.Effects[3].Bool1 || plan.Effects[3].Float2 != .75 {
		t.Fatalf("color change=%+v", plan.Effects[3])
	}
}

func TestImageEffectPlanNormalizesPointerAndValueForms(t *testing.T) {
	threshold := &drawing.BiLevel{Threshold: 17000}
	plans := ResolveImageEffectVisualPlans([]drawing.Effect{threshold, drawing.AlphaCeiling{}, &drawing.AlphaFloor{}})
	if len(plans) != 3 {
		t.Fatalf("effects=%d, want 3", len(plans))
	}
	if plans[0].Int1 != 17000 || plans[1].Type != string(drawing.EffectTypeAlphaCeiling) || plans[2].Type != string(drawing.EffectTypeAlphaFloor) {
		t.Fatalf("plans=%+v", plans)
	}
}
