package cascade

import (
	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/drawingxml"
	"office/pkg/presentation/ir"
	"office/pkg/presentation/parsers/ooxml/parts"
	"testing"
)

func TestShapeStyleFontFamilyFollowsShapeLayoutMasterOrder(t *testing.T) {
	theme := &ir.Theme{MajorFont: "Major", MinorFont: "Minor"}
	layout := &parts.Shape{Style: &parts.ShapeStyle{FontRef: &parts.ShapeFontRef{Idx: "minor"}}}
	master := &parts.Shape{Style: &parts.ShapeStyle{FontRef: &parts.ShapeFontRef{Idx: "major"}}}
	if got := ShapeStyleFontFamily(nil, layout, master, theme); got != "Minor" {
		t.Fatalf("font family = %q, want Minor", got)
	}
}

func TestShapeFillRefUsesBackgroundMatrixFor1001Indexes(t *testing.T) {
	theme := &parts.Theme{}
	theme.Elements.FmtScheme.BGFillStyleList = []parts.ThemeFormatFillStyle{{SolidFill: &parts.ColorRef{Val: "112233"}}}
	fill := ShapeFillRef(&parts.ShapeStyleRef{Idx: 1001}, theme, nil)
	if fill == nil {
		t.Fatal("background fill ref resolved nil")
	}
}

func TestShapeStyleFontColorFollowsShapeLayoutMasterOrder(t *testing.T) {
	shape := &parts.Shape{Style: &parts.ShapeStyle{FontRef: &parts.ShapeFontRef{SchemeClr: &drawingxml.SchemeClr{Val: "accent1"}}}}
	layout := &parts.Shape{Style: &parts.ShapeStyle{FontRef: &parts.ShapeFontRef{Color: &parts.ColorRef{Val: "ABCDEF"}}}}
	colors := map[string]string{"accent1": "FFFFFF"}
	if got := ShapeStyleFontColor(shape, layout, nil, colors, nil, nil); got != "FFFFFF" {
		t.Fatalf("font color = %q, want FFFFFF", got)
	}
}

func TestConnectorStyleLineUsesThemeLineRef(t *testing.T) {
	theme := &parts.Theme{}
	theme.Elements.FmtScheme.LineStyleList = []parts.ConnectorLine{{SolidFill: &drawingxml.SolidFill{SchemeClr: &drawingxml.SchemeClr{Val: "phClr"}}}}
	cx := &parts.Connector{Style: &parts.ShapeStyle{LnRef: &parts.ShapeStyleRef{Idx: 1, Color: &parts.ColorRef{Val: "336699"}}}}
	line := ConnectorStyleLine(cx, theme, nil)
	if line == nil || line.Color != "336699" {
		t.Fatalf("connector line = %+v, want color 336699", line)
	}
}

func TestEffectiveConnectorLineKeepsThemePaintForPartialLocalLine(t *testing.T) {
	theme := &parts.Theme{}
	theme.Elements.FmtScheme.LineStyleList = []parts.ConnectorLine{{
		W:         25400,
		SolidFill: &drawingxml.SolidFill{SchemeClr: &drawingxml.SchemeClr{Val: "phClr"}},
		PrstDash:  &parts.PresetDash{Val: "dash"},
	}}
	cx := &parts.Connector{
		Style: &parts.ShapeStyle{LnRef: &parts.ShapeStyleRef{Idx: 1, Color: &parts.ColorRef{Val: "336699"}}},
	}
	cx.SpPr.Line = &parts.ConnectorLine{W: 38100, HeadEnd: &parts.LineEnd{Type: "triangle"}}

	line := EffectiveConnectorLine(cx, theme, nil)
	if line == nil {
		t.Fatal("effective connector line = nil")
	}
	if line.Color != "336699" {
		t.Fatalf("effective connector color = %q, want theme-ref 336699", line.Color)
	}
	if line.Width != 3 {
		t.Fatalf("effective connector width = %v, want local 3pt", line.Width)
	}
	if line.Dash != drawing.DashPatternDash {
		t.Fatalf("effective connector dash = %v, want inherited dash", line.Dash)
	}
}
