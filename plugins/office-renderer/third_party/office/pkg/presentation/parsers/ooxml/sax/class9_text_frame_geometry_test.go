package sax

import (
	"testing"

	ir "office/pkg/presentation/ir"
)

func TestClass9PartialBodyPrInsetsMaterializeCompletePhysicalFrame(t *testing.T) {
	shape := &ir.Shape{}
	applyTextBodyProps(shape, TextBodyProperties{
		HasInsetL: true,
		InsetL:    0, // explicit zero must survive
	})

	if !shape.HasTextInsets {
		t.Fatal("partial bodyPr inset should materialize canonical text-frame insets")
	}
	if shape.TextInsetLeftEMU != 0 {
		t.Fatalf("left inset = %d, want authored zero", shape.TextInsetLeftEMU)
	}
	if shape.TextInsetRightEMU != 91440 || shape.TextInsetTopEMU != 45720 || shape.TextInsetBottomEMU != 45720 {
		t.Fatalf("resolved insets = L%d T%d R%d B%d, want L0 T45720 R91440 B45720",
			shape.TextInsetLeftEMU, shape.TextInsetTopEMU, shape.TextInsetRightEMU, shape.TextInsetBottomEMU)
	}
	p := shape.TextBodyProps
	if p.InsetL != 0 || p.InsetR != 91440 || p.InsetT != 45720 || p.InsetB != 45720 {
		t.Fatalf("bodyPr insets not canonicalized with frame: %+v", p)
	}
}

func TestClass9PartialBodyPrInsetsPreserveExplicitZeroOnEveryEdge(t *testing.T) {
	cases := []struct {
		name  string
		props TextBodyProperties
		check func(*ir.Shape) int64
	}{
		{"left", TextBodyProperties{HasInsetL: true, InsetL: 0}, func(s *ir.Shape) int64 { return s.TextInsetLeftEMU }},
		{"right", TextBodyProperties{HasInsetR: true, InsetR: 0}, func(s *ir.Shape) int64 { return s.TextInsetRightEMU }},
		{"top", TextBodyProperties{HasInsetT: true, InsetT: 0}, func(s *ir.Shape) int64 { return s.TextInsetTopEMU }},
		{"bottom", TextBodyProperties{HasInsetB: true, InsetB: 0}, func(s *ir.Shape) int64 { return s.TextInsetBottomEMU }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			shape := &ir.Shape{}
			applyTextBodyProps(shape, tc.props)
			if got := tc.check(shape); got != 0 {
				t.Fatalf("authored zero inset = %d, want 0", got)
			}
		})
	}
}
