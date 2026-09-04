package resolve

import (
	"testing"

	"office/pkg/ooxml/drawing"
	"office/pkg/sheet/parsers/ooxml/parts"
)

func TestResolveShapeCarriesDrawingEffects(t *testing.T) {
	shape := resolveShape(&parts.XDRShape{
		SpPr: parts.XDRSpPr{
			EffectLst: &parts.XDREffectList{
				OuterShadow: &parts.XDROuterShadowEffect{
					BlurRad: 38100,
					Dist:    25400,
					Dir:     2700000,
					SchemeClr: &parts.XDREffectSchemeColor{
						Val:   "accent1",
						Alpha: &parts.XDREffectAlpha{Val: 50000},
					},
				},
				Glow: &parts.XDRGlowEffect{
					Rad:     50800,
					SrgbClr: &parts.XDREffectColorRef{Val: "FF0000"},
				},
				SoftEdge: &parts.XDRSoftEdgeEffect{Rad: 12700},
				Reflection: &parts.XDRReflectionEffect{
					BlurRad: 25400,
					Dist:    38100,
					Dir:     5400000,
					StA:     25000,
				},
			},
		},
	}, nil, map[string]string{"accent1": "673AB7"})

	if shape == nil {
		t.Fatal("resolveShape returned nil")
	}
	if len(shape.Effects.Effects) != 4 {
		t.Fatalf("effect count = %d, want 4", len(shape.Effects.Effects))
	}

	shadow, ok := shape.Effects.Effects[0].(drawing.Shadow)
	if !ok {
		t.Fatalf("shadow type = %T, want drawing.Shadow", shape.Effects.Effects[0])
	}
	if shadow.Color != "673AB7" || shadow.Alpha != 0.5 || shadow.Blur != 3.0 || shadow.Distance != 2.0 || shadow.Angle != 45.0 {
		t.Fatalf("unexpected shadow = %#v", shadow)
	}

	glow, ok := shape.Effects.Effects[1].(drawing.Glow)
	if !ok {
		t.Fatalf("glow type = %T, want drawing.Glow", shape.Effects.Effects[1])
	}
	if glow.Color != "FF0000" || glow.Radius != 4.0 {
		t.Fatalf("unexpected glow = %#v", glow)
	}

	softEdge, ok := shape.Effects.Effects[2].(drawing.SoftEdge)
	if !ok {
		t.Fatalf("soft edge type = %T, want drawing.SoftEdge", shape.Effects.Effects[2])
	}
	if softEdge.Radius != 1.0 {
		t.Fatalf("unexpected soft edge = %#v", softEdge)
	}

	reflection, ok := shape.Effects.Effects[3].(drawing.Reflection)
	if !ok {
		t.Fatalf("reflection type = %T, want drawing.Reflection", shape.Effects.Effects[3])
	}
	if reflection.Alpha != 0.25 || reflection.Blur != 2.0 || reflection.Distance != 3.0 || reflection.Direction != 90.0 {
		t.Fatalf("unexpected reflection = %#v", reflection)
	}
}