package legacy

import (
	"testing"

	officeartir "office/pkg/officeart/ir"
	"office/pkg/ooxml/drawing"
	presentationir "office/pkg/presentation/ir"
)

func TestPass8EffectiveSurfaceProjectsLineEndsForAnyOfficeArtShape(t *testing.T) {
	shape := &presentationir.Shape{}
	shared := &officeartir.Shape{Line: officeartir.Line{
		Visible:    true,
		Color:      "112233",
		Alpha:      1,
		StartArrow: officeartir.Arrow{Type: "triangle", Width: "lg", Length: "sm"},
		EndArrow:   officeartir.Arrow{Type: "stealth", Width: "sm", Length: "lg"},
	}}

	applyLegacyPptOfficeArtSurface(shape, shared, legacyPptShapeTransform{})

	if shape.ArrowHead != "triangle" || shape.ArrowHeadWidth != "lg" || shape.ArrowHeadLength != "sm" {
		t.Fatalf("head/start arrow = (%q,%q,%q), want (triangle,lg,sm)", shape.ArrowHead, shape.ArrowHeadWidth, shape.ArrowHeadLength)
	}
	if shape.ArrowTail != "stealth" || shape.ArrowTailWidth != "sm" || shape.ArrowTailLength != "lg" {
		t.Fatalf("tail/end arrow = (%q,%q,%q), want (stealth,sm,lg)", shape.ArrowTail, shape.ArrowTailWidth, shape.ArrowTailLength)
	}
}

func TestPass8PPTArrowCompatibilityOverridesSharedLineEnds(t *testing.T) {
	shape := &presentationir.Shape{}
	shared := &officeartir.Shape{Line: officeartir.Line{
		Visible:    true,
		Color:      "112233",
		Alpha:      1,
		StartArrow: officeartir.Arrow{Type: "arrow", Width: "med", Length: "med"},
		EndArrow:   officeartir.Arrow{Type: "arrow", Width: "med", Length: "med"},
	}}
	compat := legacyPptShapeTransform{
		ArrowTail:       "triangle",
		ArrowTailWidth:  "lg",
		ArrowTailLength: "sm",
		ArrowHead:       "stealth",
		ArrowHeadWidth:  "sm",
		ArrowHeadLength: "lg",
	}

	applyLegacyPptOfficeArtSurface(shape, shared, compat)

	if shape.ArrowTail != "triangle" || shape.ArrowHead != "stealth" {
		t.Fatalf("PPT compatibility did not override shared arrow mapping: tail=%q head=%q", shape.ArrowTail, shape.ArrowHead)
	}
}

func TestPass8OfficeArtSurfaceProjectsAllCanonicalEffects(t *testing.T) {
	shape := &presentationir.Shape{}
	shared := &officeartir.Shape{Effects: officeartir.Effects{
		Shadow: &officeartir.Shadow{Color: "112233", Alpha: .4, BlurRadius: 2, Distance: 3, AngleDeg: 45, Inner: true},
		Glow:   &officeartir.Glow{Color: "445566", Alpha: .5, RadiusPt: 4},
		Blur:   &officeartir.Blur{RadiusPt: 5},
	}}

	applyLegacyPptOfficeArtSurface(shape, shared, legacyPptShapeTransform{})

	if len(shape.Effects.Effects) != 3 {
		t.Fatalf("effects = %#v, want inner shadow + glow + blur", shape.Effects.Effects)
	}
	if _, ok := shape.Effects.Effects[0].(drawing.InnerShadow); !ok {
		t.Fatalf("effect[0] = %T, want drawing.InnerShadow", shape.Effects.Effects[0])
	}
	if _, ok := shape.Effects.Effects[1].(drawing.Glow); !ok {
		t.Fatalf("effect[1] = %T, want drawing.Glow", shape.Effects.Effects[1])
	}
	if _, ok := shape.Effects.Effects[2].(drawing.Blur); !ok {
		t.Fatalf("effect[2] = %T, want drawing.Blur", shape.Effects.Effects[2])
	}
}
