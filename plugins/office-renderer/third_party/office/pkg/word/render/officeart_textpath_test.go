package render

import (
	"bytes"
	"strings"
	"testing"

	"office/pkg/word/parsers/ooxml/resolve"
)

func TestWordArtPathDataCoversCurvesWavesAndVerticalText(t *testing.T) {
	for _, tc := range []struct {
		warp     string
		vertical bool
		want     string
	}{
		{"textArchUpCurve", false, "Q 50 5"},
		{"textArchDownCurve", false, "Q 50 95"},
		{"textWave1", false, "C 18 18"},
		{"textSlantUp", false, "L 95 22"},
		{"textPlain", false, "L 95 50"},
		{"textWave1", true, "M 50 5 L 50 95"},
	} {
		got := wordArtPathData(tc.warp, tc.vertical)
		if !strings.Contains(got, tc.want) {
			t.Fatalf("wordArtPathData(%q, %v) = %q, want fragment %q", tc.warp, tc.vertical, got, tc.want)
		}
	}
}

func TestRenderWordArtUsesSVGTextPathAndSemanticFormatting(t *testing.T) {
	r := &htmlRenderer{}
	shape := &resolve.Shape{
		Kind:        "rect",
		FillColor:   "336699",
		StrokeColor: "112233",
		TextPath: &resolve.TextPath{
			On:            "t",
			String:        "Legacy WordArt",
			Font:          "Arial Black",
			SizePt:        30,
			Align:         2,
			Spacing:       1.1,
			Warp:          "textArchUpCurve",
			Bold:          true,
			Italic:        true,
			Underline:     true,
			Strikethrough: true,
			SmallCaps:     true,
			Shadow:        true,
			Kerning:       true,
			Stretch:       true,
		},
	}
	var out bytes.Buffer
	if !r.renderWordArtShapeWithStyleAttrs(&out, shape, []string{"position:absolute", "left:10pt"}, 180, 72) {
		t.Fatal("WordArt renderer declined a valid geometry-text shape")
	}
	html := out.String()
	for _, want := range []string{
		`class="shape-wordart-host"`,
		`class="shape-wordart"`,
		`<textPath`,
		`Q 50 5 95 78`,
		`Legacy WordArt`,
		`fill="#336699"`,
		`stroke="#112233"`,
		`font-family:'Arial Black'`,
		`font-size:30.00pt`,
		`font-weight:bold`,
		`font-style:italic`,
		`font-variant:small-caps`,
		`text-decoration:underline line-through`,
		`font-kerning:normal`,
		`letter-spacing:0.1000em`,
		`startOffset="5%"`,
		`textLength="90"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("WordArt HTML missing %q: %s", want, html)
		}
	}
}

func TestRenderShapeRoutesGeometryTextBeforeRectFallback(t *testing.T) {
	r := &htmlRenderer{}
	shape := &resolve.Shape{
		Kind:      "rect",
		TextPath:  &resolve.TextPath{On: "t", String: "No rectangle", Warp: "textWave1"},
		FillColor: "FF0000",
	}
	var out bytes.Buffer
	r.renderShapeWithStyleAttrs(&out, shape, []string{"position:relative"}, 100, 40)
	html := out.String()
	if !strings.Contains(html, `class="shape-wordart"`) {
		t.Fatalf("WordArt SVG missing: %s", html)
	}
	if strings.Contains(html, `class="shape-rect"`) {
		t.Fatalf("WordArt fell through to rectangular shape rendering: %s", html)
	}
}
