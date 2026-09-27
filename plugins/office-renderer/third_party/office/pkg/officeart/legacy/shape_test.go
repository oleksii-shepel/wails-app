package legacy

import (
	"encoding/binary"
	"testing"

	"office/pkg/officeart/ir"
	"office/pkg/officeart/legacy/escher"
)

func TestShapeFromEscherBuildsTextBox(t *testing.T) {
	node := &escher.ShapeNode{
		ShapeID:          77,
		ShapeType:        1,
		HasClientTextbox: true,
		Properties: []escher.Property{
			{ID: escher.PropRotation, Value: 30 * 65536},
			{ID: escher.PropTextLeft, Value: 25400},
			{ID: escher.PropWrapText, Value: 0},
		},
	}
	got := ShapeFromEscher(node)
	if got.Kind != ir.ShapeTextBox || got.Text == nil {
		t.Fatalf("shape = %#v", got)
	}
	if got.Transform.RotationDeg != 30 {
		t.Fatalf("rotation = %g", got.Transform.RotationDeg)
	}
	if got.Text.Insets.Left != 25400 || got.Text.Wrap != ir.TextWrapSquare {
		t.Fatalf("text = %#v", got.Text)
	}
}

func TestShapeFromEscherPromotesGeometryTextIntoSharedIR(t *testing.T) {
	node := &escher.ShapeNode{
		ShapeID:   90,
		ShapeType: 1,
		Properties: []escher.Property{
			{ID: escher.PropGTextUnicode, IsComplex: true, Complex: geometryTextUTF16("Shared WordArt")},
			{ID: escher.PropGTextFont, IsComplex: true, Complex: geometryTextUTF16("Arial Black")},
			{ID: escher.PropGTextSize, Value: 30 << 16},
			{ID: escher.PropGTextAlign, Value: 1},
		},
	}
	got := ShapeFromEscher(node)
	if got == nil || got.Text == nil || len(got.Text.Paragraphs) != 1 {
		t.Fatalf("shape text = %#v", got)
	}
	p := got.Text.Paragraphs[0]
	if p.Align != "center" || len(p.Runs) != 1 {
		t.Fatalf("paragraph = %+v", p)
	}
	run := p.Runs[0]
	if run.Text != "Shared WordArt" || run.FontFamily != "Arial Black" || run.FontSizePt != 30 {
		t.Fatalf("run = %+v", run)
	}
}

func TestShapeFromEscherKeepsGeometryTextAsAutoShape(t *testing.T) {
	node := &escher.ShapeNode{
		ShapeID:   91,
		ShapeType: 1,
		Properties: []escher.Property{
			{ID: escher.PropGTextUnicode, IsComplex: true, Complex: geometryTextUTF16("WordArt")},
		},
	}
	got := ShapeFromEscher(node)
	if got.Kind == ir.ShapeTextBox {
		t.Fatalf("GeoText auto-shape incorrectly became textbox: %+v", got)
	}
	if got.Text == nil || got.Text.Paragraphs[0].Runs[0].Text != "WordArt" {
		t.Fatalf("GeoText missing: %+v", got)
	}
}

func TestShapeFromEscherTextBoxFillColorWithoutFilledFlagStaysTransparent(t *testing.T) {
	node := &escher.ShapeNode{ShapeID: 7, HasClientTextbox: true, Properties: []escher.Property{{ID: escher.PropFillColor, Value: 0}}}
	shape := ShapeFromEscher(node)
	if shape.Fill.Kind != ir.FillNone {
		t.Fatalf("fill kind = %v, want none for implicit textbox fill", shape.Fill.Kind)
	}
}

func TestShapeFromEscherUsesOfficeArtDefaultLineWidth(t *testing.T) {
	node := &escher.ShapeNode{Properties: []escher.Property{{ID: escher.PropLineColor, Value: 0x00000000}}}
	shape := ShapeFromEscher(node)
	if !shape.Line.Visible {
		t.Fatal("line with authored color must remain visible")
	}
	if shape.Line.WidthPt != 0.75 {
		t.Fatalf("implicit OfficeArt line width = %gpt, want 0.75pt", shape.Line.WidthPt)
	}

	node.Properties = append(node.Properties, escher.Property{ID: escher.PropLineWidth, Value: 25400})
	shape = ShapeFromEscher(node)
	if shape.Line.WidthPt != 2 {
		t.Fatalf("authored OfficeArt line width = %gpt, want 2pt", shape.Line.WidthPt)
	}
}

func TestShapeFromEscherProjectsLegacyGradientFill(t *testing.T) {
	node := &escher.ShapeNode{Properties: []escher.Property{
		{ID: escher.PropFillType, Value: 4},
		{ID: escher.PropFillColor, Value: 0x000000FF},
		{ID: escher.PropFillBackColor, Value: 0x00FF0000},
		{ID: escher.PropFillAngle, Value: uint32(45 << 16)},
	}}
	got := ShapeFromEscher(node)
	if got.Fill.Kind != ir.FillGradient || got.Fill.Gradient == nil {
		t.Fatalf("fill = %#v, want gradient", got.Fill)
	}
	if got.Fill.Gradient.Type != "linear" || got.Fill.Gradient.Angle != 45 {
		t.Fatalf("gradient = %#v, want linear 45deg", got.Fill.Gradient)
	}
	if len(got.Fill.Gradient.Stops) != 2 || got.Fill.Gradient.Stops[0].Color != "FF0000" || got.Fill.Gradient.Stops[1].Color != "0000FF" {
		t.Fatalf("stops = %#v", got.Fill.Gradient.Stops)
	}
}

func TestShapeFromEscherRequiresExplicitShadowBoolean(t *testing.T) {
	base := []escher.Property{
		{ID: escher.PropShadowColor, Value: 0x00000000},
		{ID: escher.PropShadowOffsetX, Value: 25400},
		{ID: escher.PropShadowOffsetY, Value: 25400},
	}
	if got := ShapeFromEscher(&escher.ShapeNode{Properties: base}); got.Effects.Shadow != nil {
		t.Fatalf("shadow = %#v, want nil without fShadow", got.Effects.Shadow)
	}
	props := append(append([]escher.Property(nil), base...), escher.Property{ID: escher.PropShadowBooleans, Value: (1 << 30) | (1 << 14)})
	got := ShapeFromEscher(&escher.ShapeNode{Properties: props})
	if got.Effects.Shadow == nil || got.Effects.Shadow.Distance == 0 {
		t.Fatalf("shadow = %#v, want authored shadow", got.Effects.Shadow)
	}
}

func TestShapeFromEscherPreservesLegacyGradientFocusAndCenterRect(t *testing.T) {
	props := []escher.Property{
		{ID: escher.PropFillType, Value: 7},
		{ID: escher.PropFillColor, Value: 0x000000FF},
		{ID: escher.PropFillBackColor, Value: 0x00FF0000},
		{ID: escher.PropFillFocus, Value: 0xFFFFFFCE},
		{ID: escher.PropFillToLeft, Value: 32768},
		{ID: escher.PropFillToTop, Value: 32768},
		{ID: escher.PropFillToRight, Value: 32768},
		{ID: escher.PropFillToBottom, Value: 32768},
	}
	got := ShapeFromEscher(&escher.ShapeNode{Properties: props})
	if got.Fill.Gradient == nil || got.Fill.Gradient.Type != "linear" {
		t.Fatalf("gradient = %#v", got.Fill.Gradient)
	}
	stops := got.Fill.Gradient.Stops
	if len(stops) != 3 || stops[0].Color != "0000FF" || stops[1].Color != "FF0000" || stops[1].Position != .5 || stops[2].Color != "0000FF" {
		t.Fatalf("focus stops = %#v", stops)
	}
	g := got.Fill.Gradient
	if !g.HasFillTo || g.FillToLeft != .5 || g.FillToTop != .5 || g.FillToRight != .5 || g.FillToBottom != .5 {
		t.Fatalf("fill-to = %#v", g)
	}
}

func TestLegacyGradientFocusPositiveMirrorsForegroundAtEnds(t *testing.T) {
	stops := legacyGradientStops("112233", "AABBCC", VisualProperties{HasFillFocus: true, FillFocus: 50})
	if len(stops) != 3 || stops[0].Color != "112233" || stops[1].Color != "AABBCC" || stops[1].Position != .5 || stops[2].Color != "112233" {
		t.Fatalf("stops = %#v", stops)
	}
}

func TestLegacyGradientFocusPositiveWithNegativeAngleMirrorsBackgroundAtEnds(t *testing.T) {
	stops := legacyGradientStops("112233", "AABBCC", VisualProperties{HasFillFocus: true, FillFocus: 50, HasFillAngle: true, FillAngleDeg: -210})
	if len(stops) != 3 || stops[0].Color != "AABBCC" || stops[1].Color != "112233" || stops[1].Position != .5 || stops[2].Color != "AABBCC" {
		t.Fatalf("stops = %#v", stops)
	}
}

func TestLegacyGradientShadeColorsOverrideEndpointFallback(t *testing.T) {
	complex := make([]byte, 22)
	binary.LittleEndian.PutUint16(complex[0:2], 2)
	binary.LittleEndian.PutUint16(complex[2:4], 2)
	binary.LittleEndian.PutUint16(complex[4:6], 8)
	binary.LittleEndian.PutUint32(complex[6:10], 0x00332211)
	binary.LittleEndian.PutUint32(complex[10:14], 0)
	binary.LittleEndian.PutUint32(complex[14:18], 0x00CCBBAA)
	binary.LittleEndian.PutUint32(complex[18:22], 0x00010000)
	props := []escher.Property{{ID: escher.PropFillShadeColors, IsComplex: true, Complex: complex}}
	got := legacyGradientShadeStops(props, nil)
	if len(got) != 2 || got[0].Color != "112233" || got[0].Position != 0 || got[1].Color != "AABBCC" || got[1].Position != 1 {
		t.Fatalf("shade stops = %#v", got)
	}
}

func TestLegacyGradientTypePreservesOfficeArtShadeGeometry(t *testing.T) {
	cases := []struct {
		fillType uint32
		want     string
	}{
		{4, "linear"}, // msofillShade
		{5, "radial"}, // msofillShadeCenter
		{6, "path"},   // msofillShadeShape
		{7, "linear"}, // msofillShadeScale is directional, not radial
		{8, "linear"}, // msofillShadeTitle is host interpreted; PPT uses directional shade
	}
	for _, tc := range cases {
		if got := legacyGradientType(tc.fillType); got != tc.want {
			t.Fatalf("fillType %d gradient type = %q, want %q", tc.fillType, got, tc.want)
		}
	}
}

func TestShapeFromEscherShadeScaleRemainsDirectional(t *testing.T) {
	node := &escher.ShapeNode{Properties: []escher.Property{
		{ID: escher.PropFillType, Value: 7},
		{ID: escher.PropFillColor, Value: 0x000000FF},
		{ID: escher.PropFillBackColor, Value: 0x00FF0000},
		{ID: escher.PropFillAngle, Value: uint32(45 << 16)},
	}}
	got := ShapeFromEscher(node)
	if got.Fill.Gradient == nil || got.Fill.Gradient.Type != "linear" {
		t.Fatalf("shadeScale gradient = %#v, want directional linear gradient", got.Fill.Gradient)
	}
}

func TestContextualOfficeArtColorResolverResolvesFillColorLightenReference(t *testing.T) {
	props := []escher.Property{
		{ID: escher.PropFillColor, Value: 0x000000FF}, // red
	}
	resolve := contextualOfficeArtColorResolver(props, ColorRefToHex)
	// fSysIndex, base=0xF0 (fill color), transform=2 (lighten), amount=0x80.
	got := resolve(0x108002F0)
	if got != "FF7F7F" {
		t.Fatalf("resolved color = %q, want FF7F7F", got)
	}
}

func TestContextualOfficeArtColorResolverResolvesFillColorDarkenReference(t *testing.T) {
	props := []escher.Property{{ID: escher.PropFillColor, Value: 0x00FF0000}} // blue
	resolve := contextualOfficeArtColorResolver(props, ColorRefToHex)
	got := resolve(0x108001F0)
	if got != "000080" {
		t.Fatalf("resolved color = %q, want 000080", got)
	}
}

func TestLegacyPatternFillPreservesBackgroundInsteadOfCollapsingToBlack(t *testing.T) {
	node := &escher.ShapeNode{Properties: []escher.Property{
		{ID: escher.PropFillType, Value: 1},               // msofillPattern
		{ID: escher.PropFillColor, Value: 0x00000000},     // black foreground
		{ID: escher.PropFillBackColor, Value: 0x0000FFFF}, // yellow COLORREF (BGR storage)
	}}
	shape := ShapeFromEscher(node)
	if shape.Fill.Kind != ir.FillPattern || shape.Fill.Pattern == nil {
		t.Fatalf("pattern fill = %#v, want canonical pattern", shape.Fill)
	}
	if got := shape.Fill.Pattern.Foreground; got != "000000" {
		t.Fatalf("foreground = %q, want 000000", got)
	}
	if got := shape.Fill.Pattern.Background; got != "FFFF00" {
		t.Fatalf("background = %q, want FFFF00", got)
	}
	if got := shape.Fill.Color; got != "FFFF00" {
		t.Fatalf("representative fill color = %q, want FFFF00", got)
	}
}

func TestLegacyGradientShadeVariantIsPreserved(t *testing.T) {
	cases := []struct {
		fillType uint32
		want     string
	}{
		{4, "shade"},
		{5, "shadeCenter"},
		{6, "shadeShape"},
		{7, "shadeScale"},
		{8, "shadeTitle"},
	}
	for _, tc := range cases {
		node := &escher.ShapeNode{Properties: []escher.Property{
			{ID: escher.PropFillType, Value: tc.fillType},
			{ID: escher.PropFillColor, Value: 0x000000FF},
			{ID: escher.PropFillBackColor, Value: 0x00FF0000},
		}}
		shape := ShapeFromEscher(node)
		if shape.Fill.Gradient == nil {
			t.Fatalf("fillType %d: gradient missing", tc.fillType)
		}
		if got := shape.Fill.Gradient.LegacyShadeType; got != tc.want {
			t.Fatalf("fillType %d: LegacyShadeType = %q, want %q", tc.fillType, got, tc.want)
		}
	}
}

func TestClass14ExplicitShadowFalseIsAuthoritative(t *testing.T) {
	props := []escher.Property{
		{ID: escher.PropShadowColor, Value: 0x00000000},
		{ID: escher.PropShadowOffsetX, Value: 25400},
		{ID: escher.PropShadowOffsetY, Value: 12700},
		{ID: escher.PropShadowBooleans, Value: (1 << 30)}, // use fShadow, value false
	}
	got := ShapeFromEscher(&escher.ShapeNode{Properties: props})
	if got.Effects.Shadow != nil {
		t.Fatalf("explicit fShadow=false produced shadow: %#v", got.Effects.Shadow)
	}
}

func TestClass14LegacyShadowPreservesTransparentAlphaAndPhysicalGeometry(t *testing.T) {
	props := []escher.Property{
		{ID: escher.PropShadowColor, Value: 0x000000FF},
		{ID: escher.PropShadowOpacity, Value: 0},
		{ID: escher.PropShadowOffsetX, Value: 25400},
		{ID: escher.PropShadowOffsetY, Value: 12700},
		{ID: escher.PropShadowSoftness, Value: 38100},
		{ID: escher.PropShadowBooleans, Value: (1 << 30) | (1 << 14)},
	}
	got := ShapeFromEscher(&escher.ShapeNode{Properties: props})
	sh := got.Effects.Shadow
	if sh == nil {
		t.Fatal("authored shadow missing")
	}
	if sh.Alpha != 0 {
		t.Fatalf("transparent shadow alpha = %g, want 0", sh.Alpha)
	}
	if sh.BlurRadius != 3 {
		t.Fatalf("shadow softness = %gpt, want 3pt", sh.BlurRadius)
	}
	if sh.Distance <= 0 {
		t.Fatalf("shadow distance = %g, want positive physical distance", sh.Distance)
	}
}

func TestPass8ShapeFromEscherPromotesLineEndsIntoSharedIR(t *testing.T) {
	node := &escher.ShapeNode{Properties: []escher.Property{
		{ID: escher.PropLineStartArrow, Value: 1},
		{ID: escher.PropLineStartArrowWidth, Value: 2},
		{ID: escher.PropLineStartArrowLength, Value: 0},
		{ID: escher.PropLineEndArrow, Value: 3},
		{ID: escher.PropLineEndArrowWidth, Value: 0},
		{ID: escher.PropLineEndArrowLength, Value: 2},
	}}
	got := ShapeFromEscher(node)
	if !got.Line.Visible {
		t.Fatal("line with authored endpoints must remain visible")
	}
	if got.Line.StartArrow.Type == "" || got.Line.EndArrow.Type == "" {
		t.Fatalf("line ends were not promoted: start=%+v end=%+v", got.Line.StartArrow, got.Line.EndArrow)
	}
	if got.Line.StartArrow.Width == "" || got.Line.StartArrow.Length == "" || got.Line.EndArrow.Width == "" || got.Line.EndArrow.Length == "" {
		t.Fatalf("line end sizing was not promoted: start=%+v end=%+v", got.Line.StartArrow, got.Line.EndArrow)
	}
}

func TestShapeFromEscherProjectsSolidFillOpacityAndKeepsEarlierFilledBoolean(t *testing.T) {
	node := &escher.ShapeNode{ShapeID: 42485, ShapeType: 1, Properties: []escher.Property{
		{ID: escher.PropFillColor, Value: 0x00FF0000}, // blue COLORREF
		{ID: escher.PropFillOpacity, Value: 0x00004CCD},
		{ID: escher.PropFillBooleans, Value: (1 << 20) | (1 << 4)},
		{ID: escher.PropFillBooleans, Value: 0x00600000},
	}}
	got := ShapeFromEscher(node)
	if got.Fill.Kind != ir.FillSolid || got.Fill.Color != "0000FF" {
		t.Fatalf("fill = %#v, want blue solid", got.Fill)
	}
	if got.Fill.Alpha < 0.299 || got.Fill.Alpha > 0.301 {
		t.Fatalf("fill alpha = %v, want about 0.30", got.Fill.Alpha)
	}
}

func TestShapeFromEscherDefaultsClientTextboxGeometryToRect(t *testing.T) {
	shape := ShapeFromEscher(&escher.ShapeNode{ShapeID: 7, HasClientTextbox: true})
	if shape == nil || shape.Geometry.Kind != ir.GeometryPreset || shape.Geometry.Preset != "rect" {
		t.Fatalf("client textbox geometry = %#v, want preset rect", shape)
	}
}

func TestShapeFromEscherPreservesPresetGeometryAdjustments(t *testing.T) {
	node := &escher.ShapeNode{
		ShapeID:   7,
		ShapeType: 2, // msosptRoundRectangle
		Properties: []escher.Property{
			{ID: escher.PropAdjust1, Value: 1000},
			{ID: escher.PropAdjust2, Value: 2000},
		},
	}
	shape := ShapeFromEscher(node)
	if shape == nil || shape.Geometry.Preset != "roundRect" {
		t.Fatalf("shape geometry=%#v", shape)
	}
	if len(shape.Geometry.Adjustments) != 2 || shape.Geometry.Adjustments[0] != 1000 || shape.Geometry.Adjustments[1] != 2000 {
		t.Fatalf("preset adjustments=%v", shape.Geometry.Adjustments)
	}
}

func TestShapeFromEscherMaterializesOfficeArtLineDefaults(t *testing.T) {
	node := &escher.ShapeNode{Properties: []escher.Property{
		{ID: escher.PropLineStartArrow, Value: 1},
		{ID: escher.PropLineEndArrow, Value: 2},
	}}
	got := ShapeFromEscher(node)
	if !got.Line.Visible {
		t.Fatal("line with arrows must be visible")
	}
	if got.Line.WidthPt != 0.75 || got.Line.Join != "round" || got.Line.Cap != "flat" {
		t.Fatalf("line defaults = width %g join %q cap %q", got.Line.WidthPt, got.Line.Join, got.Line.Cap)
	}
	if got.Line.StartArrow.Width != "med" || got.Line.StartArrow.Length != "med" || got.Line.EndArrow.Width != "med" || got.Line.EndArrow.Length != "med" {
		t.Fatalf("arrow defaults = start %+v end %+v", got.Line.StartArrow, got.Line.EndArrow)
	}
}

func TestShapeFromEscherPreservesAuthoredOfficeArtJoinAndCap(t *testing.T) {
	node := &escher.ShapeNode{Properties: []escher.Property{
		{ID: escher.PropLineWidth, Value: 12700},
		{ID: escher.PropLineJoinStyle, Value: 1},
		{ID: escher.PropLineEndCapStyle, Value: 1},
	}}
	got := ShapeFromEscher(node)
	if got.Line.Join != "miter" || got.Line.Cap != "square" {
		t.Fatalf("authored line style = join %q cap %q", got.Line.Join, got.Line.Cap)
	}
}

func TestShapeFromEscherCanonicalizesImplicitVisibleLinePaint(t *testing.T) {
	node := &escher.ShapeNode{ShapeType: 20, Properties: []escher.Property{{ID: escher.PropLineWidth, Value: 3175}}}
	got := ShapeFromEscher(node)
	if got.Fill.Kind != ir.FillNone {
		t.Fatalf("line fill = %q, want canonical no-fill", got.Fill.Kind)
	}
	if !got.Line.Visible || got.Line.Color != "000000" || got.Line.Alpha != 1 || got.Line.WidthPt != 0.25 {
		t.Fatalf("implicit line paint = %#v, want opaque black 0.25pt", got.Line)
	}
}

func TestShapeFromEscherPreservesExplicitTransparentLine(t *testing.T) {
	node := &escher.ShapeNode{ShapeType: 20, Properties: []escher.Property{
		{ID: escher.PropLineWidth, Value: 12700},
		{ID: escher.PropLineOpacity, Value: 0},
	}}
	got := ShapeFromEscher(node)
	if !got.Line.Visible || got.Line.Alpha != 0 {
		t.Fatalf("explicit transparent line = %#v, want visible semantic line with alpha 0", got.Line)
	}
}

func TestShapeFromEscherAreaGeometryUsesOfficeArtDefaultWhiteFill(t *testing.T) {
	got := ShapeFromEscher(&escher.ShapeNode{ShapeType: 3}) // ellipse
	if got.Fill.Kind != ir.FillSolid || got.Fill.Color != "FFFFFF" || got.Fill.Alpha != 1 {
		t.Fatalf("default area fill = %#v, want opaque OfficeArt white", got.Fill)
	}
}

func TestShapeFromEscherExplicitNoFillOverridesOfficeArtDefault(t *testing.T) {
	got := ShapeFromEscher(&escher.ShapeNode{ShapeType: 3, Properties: []escher.Property{{
		ID: escher.PropFillBooleans, Value: uint32(1 << (16 + 4)),
	}}})
	if got.Fill.Kind != ir.FillNone {
		t.Fatalf("explicit no-fill = %#v, want none", got.Fill)
	}
}

func TestShapeFromEscherLineDoesNotAcquireOfficeArtDefaultAreaFill(t *testing.T) {
	got := ShapeFromEscher(&escher.ShapeNode{ShapeType: 20})
	if got.Fill.Kind != ir.FillNone {
		t.Fatalf("line fill = %#v, want none", got.Fill)
	}
}
