package drawingxml

import (
	"encoding/xml"
	"errors"
	"io"
	"math"
	"testing"

	"office/pkg/ooxml/drawing"
)

func TestPresetGeometryDrawingPresetGeometryOrdersNamedAdjustments(t *testing.T) {
	geom := (&PresetGeometry{
		Prst: "rightArrow",
		AvLst: []GeometryGuide{
			{Name: "adj2", Fmla: "val 10000"},
			{Name: "adj1", Fmla: "val 60000"},
		},
	}).DrawingPresetGeometry()
	if geom == nil {
		t.Fatal("DrawingPresetGeometry returned nil")
	}
	if len(geom.Adjustments) != 2 {
		t.Fatalf("adjustment count = %d, want 2", len(geom.Adjustments))
	}
	if geom.Adjustments[0] != 0.6 || geom.Adjustments[1] != 0.1 {
		t.Fatalf("adjustments = %#v, want [0.6 0.1]", geom.Adjustments)
	}
}

func TestPresetGeometryTiebreakSameOrder(t *testing.T) {
	// adj and adj1 both map to order 0, triggering the tiebreak
	geom := (&PresetGeometry{
		Prst: "roundRect",
		AvLst: []GeometryGuide{
			{Name: "adj", Fmla: "val 30000"},
			{Name: "adj1", Fmla: "val 50000"},
		},
	}).DrawingPresetGeometry()
	if geom == nil {
		t.Fatal("DrawingPresetGeometry returned nil")
	}
	if len(geom.Adjustments) != 2 {
		t.Fatalf("adjustment count = %d, want 2", len(geom.Adjustments))
	}
}

func TestPresetGeometryPreservesAdjustHandles(t *testing.T) {
	geom := (&PresetGeometry{
		Prst: "roundRect",
		AhLst: []AdjustHandle{{
			XMLName: xml.Name{Local: "ahXY"},
			GDRefX:  "adj1",
			MinX:    "0",
			MaxX:    "50000",
			Pos:     AdjustHandlePos{X: "adj1", Y: "0"},
		}},
		AhPolar: []AdjustHandle{{
			XMLName:  xml.Name{Local: "ahPolar"},
			GDRefR:   "adj2",
			GDRefAng: "adj3",
			Pos:      AdjustHandlePos{X: "adj2", Y: "adj3"},
		}},
	}).DrawingPresetGeometry()
	if geom == nil {
		t.Fatal("DrawingPresetGeometry returned nil")
	}
	if len(geom.Handles) != 2 {
		t.Fatalf("handle count = %d, want 2", len(geom.Handles))
	}
	if geom.Handles[0].Kind != "ahXY" || geom.Handles[0].GDRefX != "adj1" || geom.Handles[0].PosX != "adj1" {
		t.Fatalf("first handle = %+v", geom.Handles[0])
	}
	if geom.Handles[1].Kind != "ahPolar" || geom.Handles[1].GDRefR != "adj2" || geom.Handles[1].GDRefAng != "adj3" {
		t.Fatalf("second handle = %+v", geom.Handles[1])
	}
}

func TestCustomGeometryPreservesAdjustHandles(t *testing.T) {
	geom, diags := (&CustomGeometry{
		AhLst: []AdjustHandle{{
			XMLName: xml.Name{Local: "ahXY"},
			GDRefX:  "adj1",
			Pos:     AdjustHandlePos{X: "adj1", Y: "0"},
		}},
		Paths: []CustomPath{{
			W: 100000,
			H: 100000,
			Commands: []CustomPathCommand{
				{Type: "M", Points: []FormulaPoint{{X: "0", Y: "0"}}},
				{Type: "L", Points: []FormulaPoint{{X: "100000", Y: "100000"}}},
			},
		}},
	}).DrawingGeometryWithDiagnostics()
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
	if geom == nil {
		t.Fatal("DrawingGeometryWithDiagnostics returned nil")
	}
	if len(geom.Handles) != 1 {
		t.Fatalf("handle count = %d, want 1", len(geom.Handles))
	}
	if geom.Handles[0].Kind != "ahXY" || geom.Handles[0].PosX != "adj1" {
		t.Fatalf("handle = %+v", geom.Handles[0])
	}
}

func TestSolidFillToDrawingSolidFillSupportsAdditionalColorSources(t *testing.T) {
	tests := []struct {
		name string
		fill *SolidFill
		want string
	}{
		{
			name: "sysClr lastClr",
			fill: &SolidFill{SysClr: &SysClr{LastClr: "112233"}},
			want: "112233",
		},
		{
			name: "sysClr fallback map",
			fill: &SolidFill{SysClr: &SysClr{Val: "windowText"}},
			want: "000000",
		},
		{
			name: "prstClr common name",
			fill: &SolidFill{PrstClr: &PrstClr{Val: "red"}},
			want: "FF0000",
		},
		{
			name: "prstClr lowercase alias",
			fill: &SolidFill{PrstClr: &PrstClr{Val: "ltGray"}},
			want: "D3D3D3",
		},
		{
			name: "prstClr dark alias",
			fill: &SolidFill{PrstClr: &PrstClr{Val: "dkBlue"}},
			want: "00008B",
		},
		{
			name: "scrgbClr percentage rgb",
			fill: &SolidFill{ScrGBClr: &ScrGBClr{R: 100000, G: 50000, B: 0}},
			want: "FF8000",
		},
		{
			name: "hslClr pure green",
			fill: &SolidFill{HSLClr: &HSLClr{Hue: 7200000, Sat: 100000, Lum: 50000}},
			want: "00FF00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fill.ToDrawingSolidFill()
			if got == nil {
				t.Fatal("ToDrawingSolidFill returned nil")
			}
			if got.Color != tt.want {
				t.Fatalf("color = %q, want %q", got.Color, tt.want)
			}
		})
	}
}

func TestAdditionalColorSourcesPreserveTransforms(t *testing.T) {
	fill := (&SolidFill{
		SysClr: &SysClr{
			LastClr: "336699",
			colorTransforms: colorTransforms{
				AlphaMod: &ColorTransform{Val: 50000},
				Tint:     &ColorTransform{Val: 25000},
			},
		},
	}).ToDrawingSolidFill()
	if fill == nil {
		t.Fatal("ToDrawingSolidFill returned nil")
	}
	if fill.Color != "336699" {
		t.Fatalf("base color = %q, want 336699", fill.Color)
	}
	if fill.AlphaMod != 0.5 || !fill.HasTint || fill.Tint != 0.25 {
		t.Fatalf("transforms = %+v", fill)
	}
}

func TestSystemColorFallbackMap(t *testing.T) {
	tests := map[string]string{
		"windowText": "000000",
		"btnFace":    "FFFFFF",
		"highlight":  "3399FF",
		"infobk":     "FFFFE1",
	}
	for name, want := range tests {
		if got := systemColorHex(name); got != want {
			t.Fatalf("systemColorHex(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestGradientAndPatternAdditionalColorSources(t *testing.T) {
	gradient := (&GradientFill{
		StopList: []GradientStop{
			{Pos: 0, HSLClr: &HSLClr{Hue: 0, Sat: 100000, Lum: 50000}},
			{Pos: 100000, ScrGBClr: &ScrGBClr{R: 0, G: 0, B: 100000}},
		},
	}).ToDrawingGradientFill()
	if gradient == nil || len(gradient.Stops) != 2 {
		t.Fatalf("gradient = %+v, want 2 stops", gradient)
	}
	if gradient.Stops[0].Color != "FF0000" || gradient.Stops[1].Color != "0000FF" {
		t.Fatalf("gradient colors = %+v", gradient.Stops)
	}

	pattern := (&PatternFill{
		Prst:  "pct10",
		FgClr: &ColorChoice{PrstClr: &PrstClr{Val: "yellow"}},
		BgClr: &ColorChoice{SysClr: &SysClr{LastClr: "010203"}},
	}).ToDrawingPatternFill()
	if pattern == nil {
		t.Fatal("ToDrawingPatternFill returned nil")
	}
	if pattern.Foreground != "FFFF00" || pattern.Background != "010203" {
		t.Fatalf("pattern = %+v", pattern)
	}
}

func TestSharedResolversApplyThemeColorsAndTransforms(t *testing.T) {
	solid := ResolveSolidFill(&SolidFill{
		SchemeClr: &SchemeClr{
			Val:      "accent1",
			AlphaMod: &ColorTransform{Val: 50000},
			Tint:     &ColorTransform{Val: 25000},
		},
	}, map[string]string{"accent1": "204060"})
	if solid == nil {
		t.Fatal("ResolveSolidFill returned nil")
	}
	if solid.Color == "" || solid.Color == "204060" {
		t.Fatalf("solid.Color = %q, want transformed resolved color", solid.Color)
	}
	if solid.ThemeColor != "accent1" || solid.ResolvedAlpha() != 0.5 {
		t.Fatalf("solid = %+v", solid)
	}

	gradient := ResolveGradientFill(&GradientFill{
		StopList: []GradientStop{
			{
				Pos: 0,
				SchemeClr: &SchemeClr{
					Val:      "accent2",
					AlphaMod: &ColorTransform{Val: 25000},
					Shade:    &ColorTransform{Val: 50000},
				},
			},
		},
	}, map[string]string{"accent2": "00FF00"})
	if gradient == nil || len(gradient.Stops) != 1 {
		t.Fatalf("gradient = %+v, want 1 stop", gradient)
	}
	if gradient.Stops[0].Color == "" || gradient.Stops[0].Color == "accent2" || gradient.Stops[0].Color == "00FF00" {
		t.Fatalf("gradient stop = %+v, want resolved transformed color", gradient.Stops[0])
	}
	if gradient.Stops[0].Alpha != 0.25 {
		t.Fatalf("gradient alpha = %v, want 0.25", gradient.Stops[0].Alpha)
	}

	pattern := ResolvePatternFill(&PatternFill{
		Prst:  "pct10",
		FgClr: &ColorChoice{SchemeClr: &SchemeClr{Val: "accent3", AlphaMod: &ColorTransform{Val: 60000}}},
		BgClr: &ColorChoice{SRGBClr: &SRGBClr{Val: "010203"}},
	}, map[string]string{"accent3": "FF0000"})
	if pattern == nil {
		t.Fatal("ResolvePatternFill returned nil")
	}
	if pattern.Foreground != "FF0000" || pattern.ForeAlpha != 0.6 || pattern.Background != "010203" {
		t.Fatalf("pattern = %+v", pattern)
	}
}

func TestResolveBlipFillAndLineStyle(t *testing.T) {
	picture := ResolveBlipFill(&BlipFill{
		Blip:    &BlipRef{Embed: "rId7"},
		SrcRect: &SrcRect{L: 10000, T: 5000, R: 20000, B: 15000},
		Stretch: &Stretch{},
	})
	if picture == nil {
		t.Fatal("ResolveBlipFill returned nil")
	}
	if picture.EmbedID != "rId7" || !picture.Stretch || picture.Tile {
		t.Fatalf("picture = %+v", picture)
	}
	if picture.Crop == nil || !closeFloat(picture.Crop.X, 0.1) || !closeFloat(picture.Crop.Y, 0.05) || !closeFloat(picture.Crop.Width, 0.7) || !closeFloat(picture.Crop.Height, 0.8) {
		t.Fatalf("crop = %+v", picture.Crop)
	}

	// Tile mode should preserve parameters
	tiled := ResolveBlipFill(&BlipFill{
		Blip: &BlipRef{Embed: "rId8"},
		Tile: &Tile{Tx: 12700, Ty: 25400, Sx: 150000, Sy: 75000, Flip: "x", Algn: "ctr"},
	})
	if tiled == nil {
		t.Fatal("ResolveBlipFill returned nil for tile")
	}
	if !tiled.Tile || tiled.Stretch {
		t.Fatalf("expected tile=true stretch=false, got tile=%v stretch=%v", tiled.Tile, tiled.Stretch)
	}
	if tiled.TileOffsetX != 12700 {
		t.Fatalf("TileOffsetX = %d, want 12700", tiled.TileOffsetX)
	}
	if tiled.TileOffsetY != 25400 {
		t.Fatalf("TileOffsetY = %d, want 25400", tiled.TileOffsetY)
	}
	if tiled.TileScaleX != 150000 {
		t.Fatalf("TileScaleX = %d, want 150000", tiled.TileScaleX)
	}
	if tiled.TileScaleY != 75000 {
		t.Fatalf("TileScaleY = %d, want 75000", tiled.TileScaleY)
	}
	if tiled.TileFlip != "x" {
		t.Fatalf("TileFlip = %q, want x", tiled.TileFlip)
	}
	if tiled.TileAlign != "ctr" {
		t.Fatalf("TileAlign = %q, want ctr", tiled.TileAlign)
	}

	line := ResolveLineStyle(&Line{
		W:         25400,
		Cap:       "round",
		Cmpd:      "dbl",
		SolidFill: &SolidFill{SchemeClr: &SchemeClr{Val: "accent4"}},
		PrstDash:  &PresetDash{Val: "dash"},
		Round:     &LineRound{},
	}, map[string]string{"accent4": "123456"})
	if line == nil {
		t.Fatal("ResolveLineStyle returned nil")
	}
	if line.Width != 2.0 || line.Color != "123456" || line.Cap != drawing.LineCapRound || line.Join != drawing.LineJoinRound || line.Dash != drawing.DashPatternDash || !line.Compound {
		t.Fatalf("line = %+v", line)
	}
}

func closeFloat(got, want float64) bool {
	return math.Abs(got-want) < 1e-9
}

func TestCustomArcCommandInvalidFormula(t *testing.T) {
	_, _, err := CustomArcCommand(
		CustomPathCommand{Type: "A", Args: []string{"bad", "25", "0", "10800000"}},
		drawing.Point{X: 100, Y: 100},
		drawing.GuideContext{Width: 200, Height: 100},
		200,
		100,
	)
	if err == nil {
		t.Fatal("CustomArcCommand error = nil, want invalid formula error")
	}
}

func TestEvaluateCustomPointWithLiteral(t *testing.T) {
	pt, err := EvaluateCustomPoint(FormulaPoint{X: "0", Y: "0"}, drawing.GuideContext{Width: 100, Height: 100})
	if err != nil {
		t.Fatalf("EvaluateCustomPoint error = %v", err)
	}
	if pt.X != 0 || pt.Y != 0 {
		t.Fatalf("point = %+v, want (0,0)", pt)
	}
}

func TestEvaluateCustomValueWithLiteral(t *testing.T) {
	v, err := EvaluateCustomValue("50000", drawing.GuideContext{Width: 100, Height: 100})
	if err != nil {
		t.Fatalf("EvaluateCustomValue error = %v", err)
	}
	if v != 50000 {
		t.Fatalf("value = %v, want 50000", v)
	}
}

func TestCustomPathDrawingPath(t *testing.T) {
	path := CustomPath{
		W:    200,
		H:    100,
		Fill: "none",
		Commands: []CustomPathCommand{
			{Type: "M", Points: []FormulaPoint{{X: "0", Y: "0"}}},
			{Type: "L", Points: []FormulaPoint{{X: "wd/2", Y: "ht"}}},
			{Type: "Z"},
		},
	}
	dp, err := path.DrawingPath([]drawing.Guide{})
	if err != nil {
		t.Fatalf("DrawingPath error = %v", err)
	}
	if len(dp.Commands) != 3 {
		t.Fatalf("command count = %d, want 3", len(dp.Commands))
	}
}

func TestPresetColorHexMapping(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"black", "000000"},
		{"white", "FFFFFF"},
		{"red", "FF0000"},
		{"green", "008000"},
		{"blue", "0000FF"},
		{"yellow", "FFFF00"},
		{"cyan", "00FFFF"},
		{"magenta", "FF00FF"},
		{"gray", "808080"},
		{"grey", "808080"},
		{"lightgray", "D3D3D3"},
		{"darkgray", "A9A9A9"},
		{"navy", "00008B"},
		{"teal", "008B8B"},
		{"orange", "FFA500"},
		{"brown", "A52A2A"},
		{"purple", "800080"},
		{"pink", "FFC0CB"},
		{"unknown", ""},
		{"  BlUe  ", "0000FF"},
	}
	for _, tt := range tests {
		got := presetColorHex(tt.name)
		if got != tt.want {
			t.Errorf("presetColorHex(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestSystemColorHexMapping(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"window", "000000"},
		{"windowtext", "000000"},
		{"windowframe", "808080"},
		{"background", "FFFFFF"},
		{"highlight", "3399FF"},
		{"highlighttext", "FFFFFF"},
		{"graytext", "808080"},
		{"hotlight", "0000FF"},
		{"infobk", "FFFFE1"},
		{"infotext", "000000"},
		{"unknown", ""},
	}
	for _, tt := range tests {
		got := systemColorHex(tt.name)
		if got != tt.want {
			t.Errorf("systemColorHex(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestPrstClrToDrawingSolidFill(t *testing.T) {
	c := &PrstClr{Val: "red"}
	fill := c.toDrawingSolidFill()
	if fill == nil || fill.Color != "FF0000" || fill.Alpha != 1.0 {
		t.Fatalf("PrstClr(red) = %+v", fill)
	}
}

func TestSysClrToDrawingSolidFill(t *testing.T) {
	c := &SysClr{Val: "window", LastClr: "ABCDEF"}
	fill := c.toDrawingSolidFill()
	if fill == nil || fill.Color != "ABCDEF" {
		t.Fatalf("SysClr with LastClr = %+v", fill)
	}

	c2 := &SysClr{Val: "highlight"}
	fill2 := c2.toDrawingSolidFill()
	if fill2 == nil || fill2.Color != "3399FF" {
		t.Fatalf("SysClr without LastClr = %+v", fill2)
	}
}

func TestScrGBClrToDrawingSolidFill(t *testing.T) {
	c := &ScrGBClr{R: 100000, G: 0, B: 0}
	fill := c.toDrawingSolidFill()
	if fill == nil || fill.Color != "FF0000" {
		t.Fatalf("ScrGBClr = %+v", fill)
	}
}

func TestHSLClrToDrawingSolidFill(t *testing.T) {
	c := &HSLClr{Hue: 2160000, Sat: 240000, Lum: 120000} // pure red in HSL (0°, 100%, 50%)
	fill := c.toDrawingSolidFill()
	if fill == nil || fill.Color == "" {
		t.Fatalf("HSLClr = %+v", fill)
	}
}

func TestGradientStopToDrawingSolidFillVariants(t *testing.T) {
	gs := &GradientStop{PrstClr: &PrstClr{Val: "blue"}}
	fill := gs.toDrawingSolidFill()
	if fill == nil || fill.Color != "0000FF" {
		t.Fatalf("GradientStop PrstClr = %+v", fill)
	}

	gs2 := &GradientStop{SysClr: &SysClr{Val: "window"}}
	fill2 := gs2.toDrawingSolidFill()
	if fill2 == nil || fill2.Color != "000000" {
		t.Fatalf("GradientStop SysClr = %+v", fill2)
	}

	gs3 := &GradientStop{ScrGBClr: &ScrGBClr{R: 0, G: 100000, B: 0}}
	fill3 := gs3.toDrawingSolidFill()
	if fill3 == nil || fill3.Color != "00FF00" {
		t.Fatalf("GradientStop ScrGBClr = %+v", fill3)
	}
}

func TestColorChoiceToDrawingSolidFillVariants(t *testing.T) {
	cc := &ColorChoice{PrstClr: &PrstClr{Val: "green"}}
	fill := cc.toDrawingSolidFill()
	if fill == nil || fill.Color != "008000" {
		t.Fatalf("ColorChoice PrstClr = %+v", fill)
	}

	cc2 := &ColorChoice{SysClr: &SysClr{Val: "highlight"}}
	fill2 := cc2.toDrawingSolidFill()
	if fill2 == nil || fill2.Color != "3399FF" {
		t.Fatalf("ColorChoice SysClr = %+v", fill2)
	}

	cc3 := &ColorChoice{ScrGBClr: &ScrGBClr{R: 0, G: 0, B: 100000}}
	fill3 := cc3.toDrawingSolidFill()
	if fill3 == nil || fill3.Color != "0000FF" {
		t.Fatalf("ColorChoice ScrGBClr = %+v", fill3)
	}

	cc4 := &ColorChoice{HSLClr: &HSLClr{Hue: 0, Sat: 240000, Lum: 120000}}
	fill4 := cc4.toDrawingSolidFill()
	if fill4 == nil || fill4.Color == "" {
		t.Fatalf("ColorChoice HSLClr = %+v", fill4)
	}
}

func TestToDrawingGradientFillWithPathGradient(t *testing.T) {
	gf := &GradientFill{
		Path: &PathGradient{Path: "circle"},
		StopList: []GradientStop{
			{Pos: 0, SRGBClr: &SRGBClr{Val: "FF0000"}},
			{Pos: 100000, SRGBClr: &SRGBClr{Val: "0000FF"}},
		},
	}
	result := gf.ToDrawingGradientFill()
	if result == nil {
		t.Fatal("ToDrawingGradientFill returned nil")
	}
	if result.GradientType != drawing.GradientTypeRadial {
		t.Errorf("GradientType = %v, want radial", result.GradientType)
	}
	if len(result.Stops) != 2 {
		t.Fatalf("stops = %d, want 2", len(result.Stops))
	}
}

func TestToDrawingLineStyleWithGradFill(t *testing.T) {
	l := &Line{
		W:        12700,
		GradFill: &GradientFill{StopList: []GradientStop{{Pos: 0, SRGBClr: &SRGBClr{Val: "00FF00"}}}},
		PrstDash: &PresetDash{Val: "dot"},
		HeadEnd:  &LineEnd{Type: "triangle", W: "med", Len: "med"},
		TailEnd:  &LineTailEnd{Type: "arrow", W: "lg", Len: "lg"},
		Bevel:    &LineBevel{},
		Cap:      "sq",
	}
	ls := l.ToDrawingLineStyle()
	if ls == nil {
		t.Fatal("ToDrawingLineStyle returned nil")
	}
	if ls.Color != "00FF00" {
		t.Errorf("Color = %q, want 00FF00", ls.Color)
	}
	if ls.Dash != drawing.DashPatternDot {
		t.Errorf("Dash = %q, want dot", ls.Dash)
	}
	if ls.HeadEnd == nil || ls.HeadEnd.Type != drawing.ArrowheadTriangle {
		t.Errorf("HeadEnd = %+v", ls.HeadEnd)
	}
	if ls.TailEnd == nil || ls.TailEnd.Type != drawing.ArrowheadArrow {
		t.Errorf("TailEnd = %+v", ls.TailEnd)
	}
	if ls.Join != drawing.LineJoinBevel {
		t.Errorf("Join = %q, want bevel", ls.Join)
	}
	if ls.Cap != drawing.LineCapSquare {
		t.Errorf("Cap = %q, want square", ls.Cap)
	}
}

func TestToDrawingLineStyleWithCustDashAndMiter(t *testing.T) {
	l := &Line{
		W:         25400,
		SolidFill: &SolidFill{SRGBClr: &SRGBClr{Val: "000000"}},
		CustDash:  &CustomDash{Ds: []DashStop{{D: 10000, Sp: 5000}}},
		Miter:     &LineMiter{Lim: 288000},
	}
	ls := l.ToDrawingLineStyle()
	if ls == nil {
		t.Fatal("ToDrawingLineStyle returned nil")
	}
	if ls.Dash != drawing.DashPatternDash {
		t.Errorf("Dash = %q, want dash from custom dash", ls.Dash)
	}
	if ls.Join != drawing.LineJoinMiter {
		t.Errorf("Join = %q, want miter", ls.Join)
	}
	if ls.MiterLimit != 22.677165354330707 {
		t.Errorf("MiterLimit = %v, want 22.677...", ls.MiterLimit)
	}
}

func TestResolveLineStyleWithGradFill(t *testing.T) {
	l := &Line{
		W: 12700,
		GradFill: &GradientFill{
			StopList: []GradientStop{
				{Pos: 0, SchemeClr: &SchemeClr{Val: "accent1"}},
			},
		},
	}
	ls := ResolveLineStyle(l, map[string]string{"accent1": "FF0000"})
	if ls == nil {
		t.Fatal("ResolveLineStyle returned nil")
	}
	if ls.Color != "FF0000" {
		t.Errorf("Color = %q, want FF0000", ls.Color)
	}
}

func TestResolveLineStyleWithPattFill(t *testing.T) {
	l := &Line{
		W: 12700,
		PattFill: &PatternFill{
			Prst:  "pct10",
			FgClr: &ColorChoice{SRGBClr: &SRGBClr{Val: "00FF00"}},
			BgClr: &ColorChoice{SRGBClr: &SRGBClr{Val: "000000"}},
		},
	}
	ls := ResolveLineStyle(l, nil)
	if ls == nil {
		t.Fatal("ResolveLineStyle returned nil")
	}
	if ls.Color != "00FF00" {
		t.Errorf("Color = %q, want 00FF00", ls.Color)
	}
}

func TestResolveLineStyleWithNoFill(t *testing.T) {
	l := &Line{NoFill: &struct{}{}}
	ls := ResolveLineStyle(l, nil)
	if ls != nil {
		t.Fatalf("expected nil for no-fill line, got %+v", ls)
	}
}

func TestClampBlipCropPct(t *testing.T) {
	if got := clampBlipCropPct(0); got != 0 {
		t.Errorf("clampBlipCropPct(0) = %v, want 0", got)
	}
	if got := clampBlipCropPct(-1000); got != 0 {
		t.Errorf("clampBlipCropPct(-1000) = %v, want 0", got)
	}
	if got := clampBlipCropPct(50000); got != 0.5 {
		t.Errorf("clampBlipCropPct(50000) = %v, want 0.5", got)
	}
	if got := clampBlipCropPct(100000); got != 1.0 {
		t.Errorf("clampBlipCropPct(100000) = %v, want 1.0", got)
	}
}

func TestClampPercent100000(t *testing.T) {
	if got := clampPercent100000(-1); got != 0 {
		t.Errorf("clampPercent100000(-1) = %v, want 0", got)
	}
	if got := clampPercent100000(50000); got != 50000 {
		t.Errorf("clampPercent100000(50000) = %v, want 50000", got)
	}
	if got := clampPercent100000(200000); got != 100000 {
		t.Errorf("clampPercent100000(200000) = %v, want 100000", got)
	}
}

func TestPercentToByte(t *testing.T) {
	if got := percentToByte(0); got != 0 {
		t.Errorf("percentToByte(0) = %v, want 0", got)
	}
	if got := percentToByte(100000); got != 255 {
		t.Errorf("percentToByte(100000) = %v, want 255", got)
	}
	if got := percentToByte(50000); got != 128 {
		t.Errorf("percentToByte(50000) = %v, want 128", got)
	}
}

func TestColorTransformUnmarshalXML(t *testing.T) {
	input := `<lumMod val="95000"/>`
	var ct ColorTransform
	if err := xml.Unmarshal([]byte(input), &ct); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if ct.XMLName.Local != "lumMod" {
		t.Errorf("XMLName = %q, want lumMod", ct.XMLName.Local)
	}
	if ct.Val != 95000 {
		t.Errorf("Val = %v, want 95000", ct.Val)
	}
}

func TestAdjustHandleUnmarshalXML(t *testing.T) {
	input := `<ahXY gdRefX="adj1" minX="0" maxX="50000"><pos x="adj1" y="0"/></ahXY>`
	var h AdjustHandle
	if err := xml.Unmarshal([]byte(input), &h); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if h.XMLName.Local != "ahXY" {
		t.Errorf("XMLName = %q, want ahXY", h.XMLName.Local)
	}
	if h.GDRefX != "adj1" || h.MinX != "0" || h.MaxX != "50000" {
		t.Errorf("attrs = %+v", h)
	}
	if h.Pos.X != "adj1" || h.Pos.Y != "0" {
		t.Errorf("pos = %+v", h.Pos)
	}
}

func TestCustomPathUnmarshalXML(t *testing.T) {
	input := `<path w="100000" h="100000" fill="none">
		<moveTo><pt x="0" y="0"/></moveTo>
		<lnTo><pt x="100000" y="100000"/></lnTo>
		<close/>
	</path>`
	var p CustomPath
	if err := xml.Unmarshal([]byte(input), &p); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if p.W != 100000 || p.H != 100000 || p.Fill != "none" {
		t.Errorf("attrs = %+v", p)
	}
	if len(p.Commands) != 3 {
		t.Fatalf("commands = %d, want 3", len(p.Commands))
	}
	if p.Commands[0].Type != "M" || p.Commands[1].Type != "L" || p.Commands[2].Type != "Z" {
		t.Errorf("command types = %v", []string{p.Commands[0].Type, p.Commands[1].Type, p.Commands[2].Type})
	}
}

func TestCustomPathUnmarshalXMLWithBezier(t *testing.T) {
	input := `<path w="100000" h="100000">
		<moveTo><pt x="0" y="0"/></moveTo>
		<cubicBezTo><pt x="0" y="50000"/><pt x="50000" y="100000"/><pt x="100000" y="100000"/></cubicBezTo>
		<quadBezTo><pt x="50000" y="50000"/><pt x="0" y="0"/></quadBezTo>
		<arcTo wR="50000" hR="50000" stAng="0" swAng="10800000"/>
	</path>`
	var p CustomPath
	if err := xml.Unmarshal([]byte(input), &p); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(p.Commands) != 4 {
		t.Fatalf("commands = %d, want 4", len(p.Commands))
	}
	if p.Commands[1].Type != "C" || len(p.Commands[1].Points) != 3 {
		t.Errorf("cubicBezTo = %+v", p.Commands[1])
	}
	if p.Commands[2].Type != "Q" || len(p.Commands[2].Points) != 2 {
		t.Errorf("quadBezTo = %+v", p.Commands[2])
	}
	if p.Commands[3].Type != "A" || len(p.Commands[3].Args) != 4 {
		t.Errorf("arcTo = %+v", p.Commands[3])
	}
}

func TestPresetGeometryDrawingGeometry(t *testing.T) {
	pg := &PresetGeometry{
		Prst: "roundRect",
		AvLst: []GeometryGuide{
			{Name: "adj1", Fmla: "val 50000"},
		},
		AhLst: []AdjustHandle{{
			XMLName: xml.Name{Local: "ahXY"},
			GDRefX:  "adj1",
			Pos:     AdjustHandlePos{X: "adj1", Y: "0"},
		}},
	}
	geom := pg.DrawingGeometry()
	if geom == nil {
		t.Fatal("DrawingGeometry returned nil")
	}
	if geom.Type != drawing.GeometryTypePreset {
		t.Errorf("Type = %q, want preset", geom.Type)
	}
	if geom.Name != "roundRect" {
		t.Errorf("Name = %q, want roundRect", geom.Name)
	}
	if len(geom.Guides) != 1 {
		t.Errorf("guides = %d, want 1", len(geom.Guides))
	}
	if len(geom.Handles) != 1 {
		t.Errorf("handles = %d, want 1", len(geom.Handles))
	}
}

func TestPresetGeometryDrawingGeometryNil(t *testing.T) {
	var pg *PresetGeometry
	if geom := pg.DrawingGeometry(); geom != nil {
		t.Fatalf("expected nil, got %+v", geom)
	}
}

func TestCustomGeometryDrawingGeometry(t *testing.T) {
	cg := &CustomGeometry{
		AvLst: []GeometryGuide{{Name: "adj1", Fmla: "val 50000"}},
		Paths: []CustomPath{{
			W:    100000,
			H:    100000,
			Fill: "none",
			Commands: []CustomPathCommand{
				{Type: "M", Points: []FormulaPoint{{X: "0", Y: "0"}}},
				{Type: "L", Points: []FormulaPoint{{X: "100000", Y: "100000"}}},
				{Type: "Z"},
			},
		}},
	}
	geom := cg.DrawingGeometry()
	if geom == nil {
		t.Fatal("DrawingGeometry returned nil")
	}
	if geom.Type != drawing.GeometryTypeCustom {
		t.Errorf("Type = %q, want custom", geom.Type)
	}
	if len(geom.Paths) != 1 {
		t.Errorf("paths = %d, want 1", len(geom.Paths))
	}
}

func TestCustomGeometryDrawingGeometryNil(t *testing.T) {
	var cg *CustomGeometry
	if geom := cg.DrawingGeometry(); geom != nil {
		t.Fatalf("expected nil, got %+v", geom)
	}
}

func TestCustomGeometryDrawingGeometryWithDiagnostics(t *testing.T) {
	cg := &CustomGeometry{
		Paths: []CustomPath{{
			W:        100000,
			H:        100000,
			Commands: []CustomPathCommand{{Type: "M", Points: []FormulaPoint{{X: "invalid_formula", Y: "0"}}}},
		}},
	}
	geom, diags := cg.DrawingGeometryWithDiagnostics()
	if geom == nil {
		t.Fatal("DrawingGeometryWithDiagnostics returned nil")
	}
	if len(diags) == 0 {
		t.Fatal("expected diagnostics for invalid formula")
	}
	if len(geom.Paths) != 0 {
		t.Errorf("paths = %d, want 0 (bad path dropped)", len(geom.Paths))
	}
}

func TestToDrawingSolidFillAllPaths(t *testing.T) {
	// nil SolidFill
	var sf *SolidFill
	if got := sf.ToDrawingSolidFill(); got != nil {
		t.Errorf("nil.ToDrawingSolidFill = %+v, want nil", got)
	}

	// Empty SolidFill (no color) returns nil
	sf2 := &SolidFill{}
	if got := sf2.ToDrawingSolidFill(); got != nil {
		t.Errorf("empty SolidFill = %+v, want nil", got)
	}

	// SRGBClr
	sf3 := &SolidFill{SRGBClr: &SRGBClr{Val: "FF0000"}}
	if got := sf3.ToDrawingSolidFill(); got == nil || got.Color != "FF0000" {
		t.Errorf("srgb = %+v", got)
	}

	// SchemeClr
	sf4 := &SolidFill{SchemeClr: &SchemeClr{Val: "accent1"}}
	if got := sf4.ToDrawingSolidFill(); got == nil || got.ThemeColor != "accent1" {
		t.Errorf("scheme = %+v", got)
	}
}

func TestResolveDrawingSolidFillEmptyColor(t *testing.T) {
	fill := &drawing.SolidFill{Color: ""}
	resolved := resolveDrawingSolidFill(fill, map[string]string{"accent1": "FF0000"})
	if resolved == nil {
		t.Fatal("expected non-nil")
	}
	if resolved.Color != "" {
		t.Errorf("Color = %q, want empty", resolved.Color)
	}
}

func TestResolveDrawingSolidFillThemeResolution(t *testing.T) {
	fill := &drawing.SolidFill{ThemeColor: "accent1"}
	resolved := resolveDrawingSolidFill(fill, map[string]string{"accent1": "4472C4"})
	if resolved == nil {
		t.Fatal("expected non-nil")
	}
	if resolved.Color != "4472C4" {
		t.Errorf("Color = %q, want 4472C4", resolved.Color)
	}
}

func TestResolveSolidFillNil(t *testing.T) {
	if got := ResolveSolidFill(nil, nil); got != nil {
		t.Errorf("ResolveSolidFill(nil) = %+v, want nil", got)
	}
}

func TestResolveGradientFillNil(t *testing.T) {
	if got := ResolveGradientFill(nil, nil); got != nil {
		t.Errorf("ResolveGradientFill(nil) = %+v, want nil", got)
	}
}

func TestResolvePatternFillNil(t *testing.T) {
	if got := ResolvePatternFill(nil, nil); got != nil {
		t.Errorf("ResolvePatternFill(nil) = %+v, want nil", got)
	}
}

func TestResolveBlipFillNil(t *testing.T) {
	if got := ResolveBlipFill(nil); got != nil {
		t.Errorf("ResolveBlipFill(nil) = %+v, want nil", got)
	}
}

func TestResolveLineStyleNil(t *testing.T) {
	if got := ResolveLineStyle(nil, nil); got != nil {
		t.Errorf("ResolveLineStyle(nil) = %+v, want nil", got)
	}
}

func TestToDrawingLineStyleNil(t *testing.T) {
	if got := (*Line)(nil).ToDrawingLineStyle(); got != nil {
		t.Errorf("nil.ToDrawingLineStyle = %+v, want nil", got)
	}
}

func TestToDrawingGradientFillNil(t *testing.T) {
	if got := (*GradientFill)(nil).ToDrawingGradientFill(); got != nil {
		t.Errorf("nil.ToDrawingGradientFill = %+v, want nil", got)
	}
}

func TestToDrawingPatternFillNil(t *testing.T) {
	if got := (*PatternFill)(nil).ToDrawingPatternFill(); got != nil {
		t.Errorf("nil.ToDrawingPatternFill = %+v, want nil", got)
	}
}

func TestDrawingPresetGeometryNil(t *testing.T) {
	if got := (*PresetGeometry)(nil).DrawingPresetGeometry(); got != nil {
		t.Errorf("nil.DrawingPresetGeometry = %+v, want nil", got)
	}
}

func TestOrderedPresetAdjustments(t *testing.T) {
	// Test fallback ordering when no named adjustments
	guides := []GeometryGuide{
		{Name: "adj1", Fmla: "val 10000"},
		{Name: "adj2", Fmla: "val 20000"},
	}
	pg := &PresetGeometry{Prst: "rightArrow", AvLst: guides}
	geom := pg.DrawingPresetGeometry()
	if len(geom.Adjustments) != 2 {
		t.Fatalf("adjustments = %d, want 2", len(geom.Adjustments))
	}
	if geom.Adjustments[0] != 0.1 || geom.Adjustments[1] != 0.2 {
		t.Errorf("adjustments = %v, want [0.1 0.2]", geom.Adjustments)
	}
}

func TestToDrawingGradientFillWithRotateWithShape(t *testing.T) {
	trueStr := "1"
	gf := &GradientFill{
		RotateWithShape: &trueStr,
		StopList:        []GradientStop{{Pos: 0, SRGBClr: &SRGBClr{Val: "000000"}}},
	}
	result := gf.ToDrawingGradientFill()
	if result == nil {
		t.Fatal("ToDrawingGradientFill returned nil")
	}
	if !result.RotateWithShape {
		t.Error("expected RotateWithShape = true")
	}
}

func TestToDrawingGradientFillWithPathType(t *testing.T) {
	gf := &GradientFill{
		Path: &PathGradient{Path: "rect"},
	}
	result := gf.ToDrawingGradientFill()
	if result.GradientType != drawing.GradientTypeRadial {
		t.Errorf("GradientType = %v, want radial", result.GradientType)
	}
}

func TestToDrawingGradientFillWithUnknownPath(t *testing.T) {
	gf := &GradientFill{
		Path: &PathGradient{Path: "triangle"},
	}
	result := gf.ToDrawingGradientFill()
	if result.GradientType != drawing.GradientTypePath {
		t.Errorf("GradientType = %v, want path", result.GradientType)
	}
}

func TestNormalizePatternTypeAllCases(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"pct5", "percent5"},
		{"pct10", "percent10"},
		{"pct20", "percent20"},
		{"pct25", "percent25"},
		{"pct30", "percent30"},
		{"pct40", "percent40"},
		{"pct50", "percent50"},
		{"pct60", "percent60"},
		{"pct70", "percent70"},
		{"pct75", "percent75"},
		{"pct80", "percent80"},
		{"pct90", "percent90"},
		{"dkHorz", "darkHorizontal"},
		{"dkVert", "darkVertical"},
		{"dkDnDiag", "darkDownDiagonal"},
		{"dkUpDiag", "darkUpDiagonal"},
		{"smGrid", "smallGrid"},
		{"lgGrid", "largeGrid"},
		{"dotGrid", "dottedGrid"},
		{"dashHorz", "dashedHorizontal"},
		{"dashVert", "dashedVertical"},
		{"narHorz", "lightHorizontal"},
		{"narVert", "lightVertical"},
		{"dashDnDiag", "dashedDownDiagonal"},
		{"dashUpDiag", "dashedUpDiagonal"},
		{"lgConfetti", "largeConfetti"},
		{"smConfetti", "smallConfetti"},
		{"horzBrick", "horizontalBrick"},
		{"diagBrick", "diagonalBrick"},
		{"dotDmnd", "dottedDiamond"},
		{"smCheck", "smallCheckerBoard"},
		{"lgCheck", "largeCheckerBoard"},
		{"solidDmnd", "solidDiamond"},
		{"openDmnd", "openDiamond"},
		{"diagCross", "diagonalCross"},
		{"unknown", "unknown"},
	}
	for _, tt := range tests {
		if got := normalizePatternType(tt.in); got != tt.want {
			t.Errorf("normalizePatternType(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestNilToDrawingSolidFills(t *testing.T) {
	var nilSRGB *SRGBClr
	if got := nilSRGB.toDrawingSolidFill(); got != nil {
		t.Errorf("nil SRGBClr = %+v", got)
	}
	var nilScheme *SchemeClr
	if got := nilScheme.toDrawingSolidFill(); got != nil {
		t.Errorf("nil SchemeClr = %+v", got)
	}
	var nilSys *SysClr
	if got := nilSys.toDrawingSolidFill(); got != nil {
		t.Errorf("nil SysClr = %+v", got)
	}
	var nilPrst *PrstClr
	if got := nilPrst.toDrawingSolidFill(); got != nil {
		t.Errorf("nil PrstClr = %+v", got)
	}
	var nilScr *ScrGBClr
	if got := nilScr.toDrawingSolidFill(); got != nil {
		t.Errorf("nil ScrGBClr = %+v", got)
	}
	var nilHSL *HSLClr
	if got := nilHSL.toDrawingSolidFill(); got != nil {
		t.Errorf("nil HSLClr = %+v", got)
	}
	var nilGS *GradientStop
	if got := nilGS.toDrawingSolidFill(); got != nil {
		t.Errorf("nil GradientStop = %+v", got)
	}
	var nilCC *ColorChoice
	if got := nilCC.toDrawingSolidFill(); got != nil {
		t.Errorf("nil ColorChoice = %+v", got)
	}
}

func TestToDrawingSolidFillNilColor(t *testing.T) {
	gs := &GradientStop{}
	if got := gs.toDrawingSolidFill(); got != nil {
		t.Errorf("empty GradientStop = %+v", got)
	}
	cc := &ColorChoice{}
	if got := cc.toDrawingSolidFill(); got != nil {
		t.Errorf("empty ColorChoice = %+v", got)
	}
}

func TestApplyColorTransformsAllBranches(t *testing.T) {
	fill := &drawing.SolidFill{}
	ct := func(v int) *ColorTransform { return &ColorTransform{Val: v} }
	applyColorTransforms(fill,
		ct(50000),  // alpha
		ct(50000),  // alphaMod
		ct(10000),  // alphaOff
		ct(80000),  // shade
		ct(20000),  // tint
		ct(255000), // red
		ct(50000),  // redMod
		ct(10000),  // redOff
		ct(255000), // green
		ct(50000),  // greenMod
		ct(10000),  // greenOff
		ct(255000), // blue
		ct(50000),  // blueMod
		ct(10000),  // blueOff
		ct(180000), // hue
		ct(100000), // sat
		ct(50000),  // lum
		ct(50000),  // lumMod
		ct(10000),  // lumOff
		ct(50000),  // satMod
		ct(10000),  // satOff
		ct(50000),  // hueMod
		ct(10000),  // hueOff
		true, true, true, true, true,
	)
	if fill.Alpha != 0.5 || fill.AlphaMod != 0.5 || fill.AlphaOff != 0.1 {
		t.Errorf("alpha transforms = %+v", fill)
	}
	if !fill.HasShade || !fill.HasTint {
		t.Errorf("shade/tint flags not set")
	}
	if !fill.HasRed || !fill.HasRedMod || !fill.HasRedOff {
		t.Errorf("red flags not set")
	}
	if !fill.HasGreen || !fill.HasGreenMod || !fill.HasGreenOff {
		t.Errorf("green flags not set")
	}
	if !fill.HasBlue || !fill.HasBlueMod || !fill.HasBlueOff {
		t.Errorf("blue flags not set")
	}
	if !fill.HasHue || !fill.HasSat || !fill.HasLum {
		t.Errorf("hsl flags not set")
	}
	if fill.Gray && fill.Comp && fill.Inv && fill.Gamma && fill.InvGamma {
		// ok
	} else {
		t.Errorf("bool flags not set: gray=%v comp=%v inv=%v gamma=%v invGamma=%v", fill.Gray, fill.Comp, fill.Inv, fill.Gamma, fill.InvGamma)
	}
}

func TestApplyColorTransformsNilFill(t *testing.T) {
	// Should not panic
	applyColorTransforms(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, false, false, false, false, false)
}

func TestApplySRGBAndSchemeTransformsNilGuards(t *testing.T) {
	applySRGBTransforms(nil, &SRGBClr{Val: "FF0000"})
	applySRGBTransforms(&drawing.SolidFill{}, nil)
	applySchemeTransforms(nil, &SchemeClr{Val: "accent1"})
	applySchemeTransforms(&drawing.SolidFill{}, nil)
}

func TestResolveColorChoiceFillNil(t *testing.T) {
	if got := resolveColorChoiceFill(nil, nil); got != nil {
		t.Errorf("nil choice = %+v", got)
	}
}

func TestResolveDrawingSolidFillNil(t *testing.T) {
	if got := resolveDrawingSolidFill(nil, map[string]string{"a": "b"}); got != nil {
		t.Errorf("nil fill = %+v", got)
	}
}

func TestCustomArcCommandAllBranches(t *testing.T) {
	ctx := drawing.GuideContext{Width: 100, Height: 100}
	current := drawing.Point{X: 50, Y: 0}

	// Too few args
	cmd := CustomPathCommand{Type: "A", Args: []string{"50000"}}
	pc, next, err := CustomArcCommand(cmd, current, ctx, 100, 100)
	if err != nil || pc.Type != "" || next != current {
		t.Errorf("few args = %+v, %+v, %v", pc, next, err)
	}

	// Valid args
	cmd = CustomPathCommand{Type: "A", Args: []string{"50000", "50000", "0", "10800000"}}
	pc, next, err = CustomArcCommand(cmd, current, ctx, 100, 100)
	if err != nil || pc.Type != "A" {
		t.Errorf("valid arc = %+v, %v", pc, err)
	}

	// hR error
	cmd = CustomPathCommand{Type: "A", Args: []string{"50000", "bad", "0", "10800000"}}
	_, _, err = CustomArcCommand(cmd, current, ctx, 100, 100)
	if err == nil {
		t.Error("expected hR error")
	}

	// stAng error
	cmd = CustomPathCommand{Type: "A", Args: []string{"50000", "50000", "bad", "10800000"}}
	_, _, err = CustomArcCommand(cmd, current, ctx, 100, 100)
	if err == nil {
		t.Error("expected stAng error")
	}

	// swAng error
	cmd = CustomPathCommand{Type: "A", Args: []string{"50000", "50000", "0", "bad"}}
	_, _, err = CustomArcCommand(cmd, current, ctx, 100, 100)
	if err == nil {
		t.Error("expected swAng error")
	}

	// large arc (>180 deg)
	cmd = CustomPathCommand{Type: "A", Args: []string{"50000", "50000", "0", "20000000"}}
	pc, _, err = CustomArcCommand(cmd, current, ctx, 100, 100)
	if err != nil || len(pc.Args) < 5 || pc.Args[3] != 1 {
		t.Errorf("large arc args = %+v", pc.Args)
	}

	// negative sweep
	cmd = CustomPathCommand{Type: "A", Args: []string{"50000", "50000", "0", "-10800000"}}
	pc, _, err = CustomArcCommand(cmd, current, ctx, 100, 100)
	if err != nil || len(pc.Args) < 5 || pc.Args[4] != 0 {
		t.Errorf("negative sweep args = %+v", pc.Args)
	}
}

func TestCustomPathUnmarshalXMLPolyLineToAndUnknown(t *testing.T) {
	input := `<path w="100000" h="100000">
		<polyLineTo><pt x="0" y="0"/><pt x="50000" y="50000"/></polyLineTo>
		<unknown/>
	</path>`
	var p CustomPath
	if err := xml.Unmarshal([]byte(input), &p); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	// polyLineTo expands to multiple L commands
	polyCount := 0
	for _, c := range p.Commands {
		if c.Type == "L" {
			polyCount++
		}
	}
	if polyCount != 2 {
		t.Errorf("polyLineTo produced %d L commands, want 2", polyCount)
	}
}

func TestFormatHexRGBFromHSLNegativeHue(t *testing.T) {
	got := formatHexRGBFromHSL(-60000, 100000, 50000)
	if got == "" {
		t.Error("expected non-empty hex from negative hue")
	}
}

func TestHSLToRGBGray(t *testing.T) {
	r, g, b := hslToRGB(0, 0, 0.5)
	if r != 128 || g != 128 || b != 128 {
		t.Errorf("hslToRGB gray = (%d,%d,%d), want (128,128,128)", r, g, b)
	}
}

func TestHSLToRGBLightHigh(t *testing.T) {
	r, g, b := hslToRGB(0, 1.0, 0.75)
	if r == 0 || g == 0 || b == 0 {
		t.Errorf("hslToRGB light>=0.5 = (%d,%d,%d)", r, g, b)
	}
}

func TestHueToRGBWrapAndMidRange(t *testing.T) {
	// t > 1 wrap-around
	got := hueToRGB(0, 1, 1.2)
	if got < 0 || got > 1 {
		t.Errorf("hueToRGB(1.2) = %v, want in [0,1]", got)
	}
	// t in [1/2, 2/3)
	got2 := hueToRGB(0, 1, 0.6)
	if got2 < 0 || got2 > 1 {
		t.Errorf("hueToRGB(0.6) = %v, want in [0,1]", got2)
	}
}

func TestResolveLineStylePatternFallback(t *testing.T) {
	l := &Line{
		W: 12700,
		PattFill: &PatternFill{
			Prst:  "pct10",
			FgClr: nil,
			BgClr: &ColorChoice{SRGBClr: &SRGBClr{Val: "000000"}},
		},
	}
	ls := ResolveLineStyle(l, nil)
	if ls == nil {
		t.Fatal("expected non-nil")
	}
	if ls.Color != "000000" {
		t.Errorf("Color = %q, want 000000", ls.Color)
	}
}

func TestResolveGradientFillNilResolvedStop(t *testing.T) {
	gf := &GradientFill{
		StopList: []GradientStop{
			{Pos: 0}, // no color source
		},
	}
	result := ResolveGradientFill(gf, nil)
	if result == nil {
		t.Fatal("expected non-nil")
	}
	if len(result.Stops) != 1 {
		t.Fatalf("stops = %d, want 1", len(result.Stops))
	}
	if result.Stops[0].Color != "" || result.Stops[0].Alpha != 1.0 {
		t.Errorf("stop = %+v", result.Stops[0])
	}
}

func TestDrawingPathZeroSizeAndArc(t *testing.T) {
	p := CustomPath{
		W: 0,
		H: 0,
		Commands: []CustomPathCommand{
			{Type: "M", Points: []FormulaPoint{{X: "0", Y: "0"}}},
			{Type: "A", Args: []string{"50000", "50000", "0", "10800000"}},
			{Type: "Z"},
		},
	}
	dp, err := p.DrawingPath(nil)
	if err != nil {
		t.Fatalf("DrawingPath error: %v", err)
	}
	if len(dp.Commands) != 3 {
		t.Errorf("commands = %d, want 3", len(dp.Commands))
	}
}

func TestResolveBlipFillNegativeCrop(t *testing.T) {
	bf := &BlipFill{
		Blip:    &BlipRef{Embed: "rId1"},
		SrcRect: &SrcRect{L: 60000, T: 0, R: 60000, B: 0},
	}
	result := ResolveBlipFill(bf)
	if result == nil {
		t.Fatal("expected non-nil")
	}
	if result.Crop != nil {
		if result.Crop.Width < 0 || result.Crop.Height < 0 {
			t.Errorf("negative crop = %+v", result.Crop)
		}
	}
}

func TestToDrawingPatternFillBgClrTheme(t *testing.T) {
	pf := &PatternFill{
		Prst:  "pct10",
		FgClr: &ColorChoice{SRGBClr: &SRGBClr{Val: "FF0000"}},
		BgClr: &ColorChoice{SchemeClr: &SchemeClr{Val: "accent1"}},
	}
	result := pf.ToDrawingPatternFill()
	if result == nil {
		t.Fatal("expected non-nil")
	}
	// When Color is empty but ThemeColor is set, ToDrawingPatternFill falls back to ThemeColor
	if result.Background != "accent1" {
		t.Errorf("Background = %q, want accent1", result.Background)
	}
}

func TestResolvePatternFillNilColorChoice(t *testing.T) {
	pf := &PatternFill{
		Prst:  "pct10",
		FgClr: nil,
		BgClr: nil,
	}
	result := ResolvePatternFill(pf, nil)
	if result == nil {
		t.Fatal("expected non-nil")
	}
	if result.Foreground != "" || result.Background != "" {
		t.Errorf("result = %+v", result)
	}
}

func TestPresetColorHexRemainingAliases(t *testing.T) {
	tests := []struct {
		name, want string
	}{
		{"ltblue", "ADD8E6"},
		{"lightblue", "ADD8E6"},
		{"dkblue", "00008B"},
		{"darkblue", "00008B"},
		{"navy", "00008B"},
		{"ltgreen", "90EE90"},
		{"lightgreen", "90EE90"},
		{"dkgreen", "006400"},
		{"darkgreen", "006400"},
		{"ltcyan", "E0FFFF"},
		{"lightcyan", "E0FFFF"},
		{"dkcyan", "008B8B"},
		{"darkcyan", "008B8B"},
		{"teal", "008B8B"},
		{"ltyellow", "FFFFE0"},
		{"lightyellow", "FFFFE0"},
		{"dkyellow", "808000"},
		{"darkyellow", "808000"},
		{"olive", "808000"},
		{"orange", "FFA500"},
		{"brown", "A52A2A"},
		{"purple", "800080"},
		{"pink", "FFC0CB"},
	}
	for _, tt := range tests {
		got := presetColorHex(tt.name)
		if got != tt.want {
			t.Errorf("presetColorHex(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestOrderedPresetAdjustmentsEdgeCases(t *testing.T) {
	// Non-val formula skipped
	pg := &PresetGeometry{AvLst: []GeometryGuide{{Name: "adj1", Fmla: "add 1 2"}}}
	geom := pg.DrawingPresetGeometry()
	if len(geom.Adjustments) != 0 {
		t.Errorf("non-val formula adjustments = %d, want 0", len(geom.Adjustments))
	}

	// Parse error skipped
	pg2 := &PresetGeometry{AvLst: []GeometryGuide{{Name: "adj1", Fmla: "val bad"}}}
	geom2 := pg2.DrawingPresetGeometry()
	if len(geom2.Adjustments) != 0 {
		t.Errorf("parse error adjustments = %d, want 0", len(geom2.Adjustments))
	}

	// Fallback (no recognized order)
	pg3 := &PresetGeometry{AvLst: []GeometryGuide{{Name: "foo", Fmla: "val 50000"}}}
	geom3 := pg3.DrawingPresetGeometry()
	if len(geom3.Adjustments) != 1 || geom3.Adjustments[0] != 0.5 {
		t.Errorf("fallback adjustments = %v", geom3.Adjustments)
	}
}

func TestPresetAdjustmentOrderEdgeCases(t *testing.T) {
	if _, ok := presetAdjustmentOrder("foo"); ok {
		t.Error("expected false for foo")
	}
	if _, ok := presetAdjustmentOrder("adj0"); ok {
		t.Error("expected false for adj0")
	}
}

func TestDrawingGuidesFromGeometryGuidesEdgeCases(t *testing.T) {
	// Non-val formula - still creates guide with Formula set
	guides := DrawingGuidesFromGeometryGuides([]GeometryGuide{{Name: "g1", Fmla: "add 1 2"}})
	if len(guides) != 1 || guides[0].Formula != "add 1 2" {
		t.Errorf("non-val guide = %+v", guides)
	}

	// Parse error - still creates guide with Formula set
	guides2 := DrawingGuidesFromGeometryGuides([]GeometryGuide{{Name: "g1", Fmla: "val bad"}})
	if len(guides2) != 1 || guides2[0].Formula != "val bad" {
		t.Errorf("parse error guide = %+v", guides2)
	}
}

func TestEvaluateCustomPointYError(t *testing.T) {
	_, err := EvaluateCustomPoint(FormulaPoint{X: "0", Y: "bad"}, drawing.GuideContext{})
	if err == nil {
		t.Error("expected error for bad Y")
	}
}

func TestEvaluateCustomValueEmpty(t *testing.T) {
	v, err := EvaluateCustomValue("", drawing.GuideContext{})
	if err != nil || v != 0 {
		t.Errorf("empty = %v, %v, want 0, nil", v, err)
	}
}

func TestToDrawingGradientFillWithLinear(t *testing.T) {
	gf := &GradientFill{
		Linear: &LinearGradient{Angle: 4500000},
		StopList: []GradientStop{
			{Pos: 0, SRGBClr: &SRGBClr{Val: "000000"}},
		},
	}
	result := gf.ToDrawingGradientFill()
	if result == nil {
		t.Fatal("expected non-nil")
	}
	if result.Angle != 75.0 {
		t.Errorf("Angle = %v, want 75.0", result.Angle)
	}
}

func TestResolvePatternFillNilFgBg(t *testing.T) {
	pf := &PatternFill{
		Prst:  "pct10",
		FgClr: &ColorChoice{SchemeClr: &SchemeClr{Val: "accent1"}},
		BgClr: &ColorChoice{SchemeClr: &SchemeClr{Val: "accent2"}},
	}
	result := ResolvePatternFill(pf, map[string]string{"accent1": "", "accent2": ""})
	if result == nil {
		t.Fatal("expected non-nil")
	}
}

func TestCustomPathUnmarshalXMLDefaults(t *testing.T) {
	input := `<path><moveTo><pt x="0" y="0"/></moveTo></path>`
	var p CustomPath
	if err := xml.Unmarshal([]byte(input), &p); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if p.W != 0 || p.H != 0 {
		t.Errorf("defaults = (%v,%v), want (0,0)", p.W, p.H)
	}
}

func TestPresetGeometryDrawingGeometryWithNilGuides(t *testing.T) {
	pg := &PresetGeometry{Prst: "rect"}
	geom := pg.DrawingGeometry()
	if geom == nil {
		t.Fatal("expected non-nil")
	}
	if len(geom.Guides) != 0 || len(geom.Handles) != 0 {
		t.Errorf("guides=%d handles=%d, want 0,0", len(geom.Guides), len(geom.Handles))
	}
}

func TestHSLToRGBDark(t *testing.T) {
	// l < 0.5 branch: HSL(0, 1.0, 0.25) = dark red ~ (128, 0, 0)
	r, g, b := hslToRGB(0, 1.0, 0.25)
	if r != 128 || g != 0 || b != 0 {
		t.Errorf("hslToRGB dark = (%d,%d,%d), want (128,0,0)", r, g, b)
	}
}

func TestResolveBlipFillNegativeHeight(t *testing.T) {
	bf := &BlipFill{
		Blip:    &BlipRef{Embed: "rId1"},
		SrcRect: &SrcRect{L: 0, T: 60000, R: 0, B: 60000},
	}
	result := ResolveBlipFill(bf)
	if result == nil {
		t.Fatal("expected non-nil")
	}
	if result.Crop == nil {
		t.Fatal("expected crop")
	}
	if result.Crop.Height != 0 {
		t.Errorf("height = %v, want 0", result.Crop.Height)
	}
}

func TestToDrawingLineStyleWithPattern(t *testing.T) {
	// Pattern with foreground
	l := &Line{
		W:        12700,
		PattFill: &PatternFill{Prst: "pct10", FgClr: &ColorChoice{SRGBClr: &SRGBClr{Val: "00FF00"}}},
	}
	ls := l.ToDrawingLineStyle()
	if ls == nil {
		t.Fatal("expected non-nil")
	}
	if ls.Color != "00FF00" {
		t.Errorf("Color = %q, want 00FF00", ls.Color)
	}

	// Pattern with empty foreground (should not set color)
	l2 := &Line{
		W:        12700,
		PattFill: &PatternFill{Prst: "pct10", FgClr: &ColorChoice{}},
	}
	ls2 := l2.ToDrawingLineStyle()
	if ls2 == nil {
		t.Fatal("expected non-nil")
	}
	if ls2.Color != "" {
		t.Errorf("Color = %q, want empty", ls2.Color)
	}
}

func TestResolveLineStylePatternEmptyForeground(t *testing.T) {
	l := &Line{
		W: 12700,
		PattFill: &PatternFill{
			Prst:  "pct10",
			FgClr: &ColorChoice{}, // nil resolved -> foreground empty
			BgClr: &ColorChoice{SRGBClr: &SRGBClr{Val: "000000"}},
		},
	}
	ls := ResolveLineStyle(l, nil)
	if ls == nil {
		t.Fatal("expected non-nil")
	}
	if ls.Color != "000000" {
		t.Errorf("Color = %q, want 000000 (fallback to background)", ls.Color)
	}
}

func TestResolveGradientFillNilResult(t *testing.T) {
	gf := &GradientFill{
		StopList: []GradientStop{
			{Pos: 0, SRGBClr: &SRGBClr{Val: "FF0000"}},
		},
	}
	result := ResolveGradientFill(gf, nil)
	if result == nil {
		t.Fatal("expected non-nil")
	}
	if len(result.Stops) != 1 {
		t.Fatalf("stops = %d, want 1", len(result.Stops))
	}
	if result.Stops[0].Color != "FF0000" {
		t.Errorf("Color = %q, want FF0000", result.Stops[0].Color)
	}
}

func TestResolvePatternFillNilResolved(t *testing.T) {
	pf := &PatternFill{
		Prst:  "pct10",
		FgClr: &ColorChoice{}, // all nil -> resolveColorChoiceFill returns nil
		BgClr: &ColorChoice{}, // all nil -> resolveColorChoiceFill returns nil
	}
	result := ResolvePatternFill(pf, nil)
	if result == nil {
		t.Fatal("expected non-nil")
	}
	if result.Foreground != "" || result.Background != "" {
		t.Errorf("result = %+v", result)
	}
}

func TestOrderedPresetAdjustmentsTiebreak(t *testing.T) {
	pg := &PresetGeometry{AvLst: []GeometryGuide{
		{Name: "adj2", Fmla: "val 20000"},
		{Name: "adj1", Fmla: "val 10000"},
	}}
	geom := pg.DrawingPresetGeometry()
	if len(geom.Adjustments) != 2 {
		t.Fatalf("adjustments = %d, want 2", len(geom.Adjustments))
	}
	if geom.Adjustments[0] != 0.1 || geom.Adjustments[1] != 0.2 {
		t.Errorf("adjustments = %v, want [0.1 0.2]", geom.Adjustments)
	}
}

func TestDrawingPathInvalidGuides(t *testing.T) {
	p := CustomPath{
		W: 100000,
		H: 100000,
		Commands: []CustomPathCommand{
			{Type: "M", Points: []FormulaPoint{{X: "0", Y: "0"}}},
		},
	}
	// Guides with invalid formula cause EvaluateGuides to fail, but DrawingPath catches it
	guides := []drawing.Guide{{Name: "bad", Formula: "invalid"}}
	dp, err := p.DrawingPath(guides)
	if err != nil {
		t.Fatalf("DrawingPath error: %v", err)
	}
	if len(dp.Commands) != 1 {
		t.Errorf("commands = %d, want 1", len(dp.Commands))
	}
}

func TestResolveGradientFillMoreStopsThanResult(t *testing.T) {
	gf := &GradientFill{
		StopList: []GradientStop{
			{Pos: 0, SRGBClr: &SRGBClr{Val: "FF0000"}},
			{Pos: 50000, SRGBClr: &SRGBClr{Val: "00FF00"}},
			{Pos: 100000, SRGBClr: &SRGBClr{Val: "0000FF"}},
		},
	}
	result := ResolveGradientFill(gf, nil)
	if result == nil || len(result.Stops) != 3 {
		t.Fatalf("stops = %d, want 3", len(result.Stops))
	}
}

func TestToDrawingLineStyleWithLumMod(t *testing.T) {
	l := &Line{
		W: 12700,
		SolidFill: &SolidFill{
			SRGBClr: &SRGBClr{
				Val:    "FF0000",
				LumMod: &ColorTransform{Val: 50000},
			},
		},
	}
	ls := l.ToDrawingLineStyle()
	if ls == nil {
		t.Fatal("expected non-nil")
	}
	if ls.LumMod != 0.5 {
		t.Errorf("LumMod = %v, want 0.5", ls.LumMod)
	}
}

func TestDrawingPathInvalidPoint(t *testing.T) {
	cp := &CustomPath{
		W: 100, H: 100,
		Commands: []CustomPathCommand{
			{Type: "M", Points: []FormulaPoint{{X: "invalid"}}},
		},
	}
	_, err := cp.DrawingPath(nil)
	if err == nil {
		t.Error("expected error for invalid point coordinate")
	}
}

func TestDrawingPathInvalidArc(t *testing.T) {
	cp := &CustomPath{
		W: 100, H: 100,
		Commands: []CustomPathCommand{
			{Type: "M", Points: []FormulaPoint{{X: "0", Y: "0"}}},
			{Type: "A", Args: []string{"invalid", "10", "0", "0", "0"}},
		},
	}
	_, err := cp.DrawingPath(nil)
	if err == nil {
		t.Error("expected error for invalid arc args")
	}
}

// errorReader is an io.Reader that returns a fixed error after a set number of bytes.
type errorReader struct {
	data     []byte
	pos      int
	err      error
	errAfter int // return error after reading this many bytes (-1 = immediately after data exhausted)
}

func (r *errorReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		if r.errAfter >= 0 && r.pos >= r.errAfter {
			return 0, r.err
		}
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	if r.errAfter >= 0 && r.pos >= r.errAfter {
		return n, r.err
	}
	return n, nil
}

func TestColorTransformUnmarshalXMLError(t *testing.T) {
	dec := xml.NewDecoder(&errorReader{data: []byte(""), err: errors.New("boom"), errAfter: 0})
	var ct ColorTransform
	err := ct.UnmarshalXML(dec, xml.StartElement{Name: xml.Name{Local: "lumMod"}})
	if err == nil {
		t.Error("expected error from broken decoder")
	}
}

func TestCustomPathUnmarshalXMLTokenError(t *testing.T) {
	dec := xml.NewDecoder(&errorReader{data: []byte(""), err: errors.New("boom"), errAfter: 0})
	var p CustomPath
	err := p.UnmarshalXML(dec, xml.StartElement{Name: xml.Name{Local: "path"}})
	if err == nil {
		t.Error("expected error from broken decoder Token()")
	}
}

func TestCustomPathUnmarshalXMLDecodeElementError(t *testing.T) {
	// Enough XML for the decoder to parse the start element, then fail on content.
	dec := xml.NewDecoder(&errorReader{
		data:     []byte("<moveTo><pt x=\"0\" y=\"0\"/>"),
		err:      errors.New("boom"),
		errAfter: len("<moveTo><pt x=\"0\" y=\"0\"/>"),
	})
	var p CustomPath
	err := p.UnmarshalXML(dec, xml.StartElement{Name: xml.Name{Local: "path"}})
	if err == nil {
		t.Error("expected error from broken decoder DecodeElement()")
	}
}

func TestCustomPathUnmarshalXMLPolyLineToDecodeElementError(t *testing.T) {
	dec := xml.NewDecoder(&errorReader{
		data:     []byte("<polyLineTo><pt x=\"0\" y=\"0\"/>"),
		err:      errors.New("boom"),
		errAfter: len("<polyLineTo><pt x=\"0\" y=\"0\"/>"),
	})
	var p CustomPath
	err := p.UnmarshalXML(dec, xml.StartElement{Name: xml.Name{Local: "path"}})
	if err == nil {
		t.Error("expected error from broken decoder DecodeElement() for polyLineTo")
	}
}

func TestCustomPathUnmarshalXMLCubicBezToDecodeElementError(t *testing.T) {
	dec := xml.NewDecoder(&errorReader{
		data:     []byte("<cubicBezTo><pt x=\"0\" y=\"0\"/>"),
		err:      errors.New("boom"),
		errAfter: len("<cubicBezTo><pt x=\"0\" y=\"0\"/>"),
	})
	var p CustomPath
	err := p.UnmarshalXML(dec, xml.StartElement{Name: xml.Name{Local: "path"}})
	if err == nil {
		t.Error("expected error from broken decoder DecodeElement() for cubicBezTo")
	}
}

func TestCustomPathUnmarshalXMLQuadBezToDecodeElementError(t *testing.T) {
	dec := xml.NewDecoder(&errorReader{
		data:     []byte("<quadBezTo><pt x=\"0\" y=\"0\"/>"),
		err:      errors.New("boom"),
		errAfter: len("<quadBezTo><pt x=\"0\" y=\"0\"/>"),
	})
	var p CustomPath
	err := p.UnmarshalXML(dec, xml.StartElement{Name: xml.Name{Local: "path"}})
	if err == nil {
		t.Error("expected error from broken decoder DecodeElement() for quadBezTo")
	}
}

func TestCustomPathUnmarshalXMLArcToSkipError(t *testing.T) {
	dec := xml.NewDecoder(&errorReader{
		data:     []byte("<arcTo wR=\"1\" hR=\"1\" stAng=\"0\" swAng=\"90\">"),
		err:      errors.New("boom"),
		errAfter: len("<arcTo wR=\"1\" hR=\"1\" stAng=\"0\" swAng=\"90\">"),
	})
	var p CustomPath
	err := p.UnmarshalXML(dec, xml.StartElement{Name: xml.Name{Local: "path"}})
	if err == nil {
		t.Error("expected error from broken decoder Skip() for arcTo")
	}
}

func TestCustomPathUnmarshalXMLCloseSkipError(t *testing.T) {
	dec := xml.NewDecoder(&errorReader{
		data:     []byte("<close>"),
		err:      errors.New("boom"),
		errAfter: len("<close>"),
	})
	var p CustomPath
	err := p.UnmarshalXML(dec, xml.StartElement{Name: xml.Name{Local: "path"}})
	if err == nil {
		t.Error("expected error from broken decoder Skip() for close")
	}
}

func TestCustomPathUnmarshalXMLDefaultSkipError(t *testing.T) {
	dec := xml.NewDecoder(&errorReader{
		data:     []byte("<unknown>"),
		err:      errors.New("boom"),
		errAfter: len("<unknown>"),
	})
	var p CustomPath
	err := p.UnmarshalXML(dec, xml.StartElement{Name: xml.Name{Local: "path"}})
	if err == nil {
		t.Error("expected error from broken decoder Skip() for default")
	}
}

func TestAdjustHandleUnmarshalXMLError(t *testing.T) {
	dec := xml.NewDecoder(&errorReader{data: []byte(""), err: errors.New("boom"), errAfter: 0})
	var h AdjustHandle
	err := h.UnmarshalXML(dec, xml.StartElement{Name: xml.Name{Local: "ahXY"}})
	if err == nil {
		t.Error("expected error from broken decoder DecodeElement()")
	}
}

func TestCustomPathUnmarshalXMLEmptyEOF(t *testing.T) {
	// An empty reader causes Token() to return io.EOF, which should return nil.
	dec := xml.NewDecoder(&errorReader{data: []byte(""), err: io.EOF, errAfter: -1})
	var p CustomPath
	err := p.UnmarshalXML(dec, xml.StartElement{Name: xml.Name{Local: "path"}})
	if err != nil {
		t.Errorf("expected nil for io.EOF, got %v", err)
	}
}

func TestCustomPathUnmarshalXMLLnToDecodeElementError(t *testing.T) {
	dec := xml.NewDecoder(&errorReader{
		data:     []byte("<lnTo><pt x=\"0\" y=\"0\"/>"),
		err:      errors.New("boom"),
		errAfter: len("<lnTo><pt x=\"0\" y=\"0\"/>"),
	})
	var p CustomPath
	err := p.UnmarshalXML(dec, xml.StartElement{Name: xml.Name{Local: "path"}})
	if err == nil {
		t.Error("expected error from broken decoder DecodeElement() for lnTo")
	}
}

func TestResolveBlipFillPreservesStretchFillRect(t *testing.T) {
	bf := &BlipFill{
		Blip:    &BlipRef{Embed: "rId1"},
		Stretch: &Stretch{FillRect: &FillRect{L: 10000, T: 20000, R: 30000, B: 10000}},
	}
	got := ResolveBlipFill(bf)
	if got == nil || got.FillRect == nil {
		t.Fatalf("expected fill rect, got %#v", got)
	}
	if math.Abs(got.FillRect.X-0.1) > 1e-9 || math.Abs(got.FillRect.Y-0.2) > 1e-9 || math.Abs(got.FillRect.Width-0.6) > 1e-9 || math.Abs(got.FillRect.Height-0.7) > 1e-9 {
		t.Fatalf("fill rect = %#v", got.FillRect)
	}
}

func TestResolveBlipFillClampsDegenerateStretchFillRect(t *testing.T) {
	bf := &BlipFill{Blip: &BlipRef{Embed: "rId1"}, Stretch: &Stretch{FillRect: &FillRect{L: 70000, R: 70000}}}
	got := ResolveBlipFill(bf)
	if got == nil || got.FillRect == nil || got.FillRect.Width != 0 {
		t.Fatalf("degenerate fill rect = %#v", got)
	}
}
