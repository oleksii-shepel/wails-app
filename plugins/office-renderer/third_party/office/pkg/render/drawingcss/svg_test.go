package drawingcss_test

import (
	"strings"
	"testing"

	"office/pkg/ooxml/drawing"
	drawingcss "office/pkg/render/drawingcss"
)

func TestPatternSVGDefinitionCoverage(t *testing.T) {
	types := []drawing.PatternType{
		drawing.PatternTypeSolidDiamond,
		drawing.PatternTypeDarkHorizontal,
		drawing.PatternTypeDarkVertical,
		drawing.PatternTypeDarkUpDiagonal,
		drawing.PatternTypeDarkDownDiagonal,
		drawing.PatternTypeSmallGrid,
		drawing.PatternTypeLargeGrid,
		drawing.PatternTypeDashedHorizontal,
		drawing.PatternTypeDashedVertical,
		drawing.PatternTypeLightHorizontal,
		drawing.PatternTypeLightVertical,
		drawing.PatternTypeLightUpDiagonal,
		drawing.PatternTypeLightDownDiagonal,
		drawing.PatternTypeOpenDiamond,
		drawing.PatternTypeLargeCheckerBoard,
		drawing.PatternTypeSmallCheckerBoard,
		drawing.PatternTypeDiagonalBrick,
		drawing.PatternTypeHorizontalBrick,
		drawing.PatternTypeCross,
		drawing.PatternTypeDiagonalCross,
		drawing.PatternTypeTrellis,
		drawing.PatternTypeDottedGrid,
		drawing.PatternTypeDottedDiamond,
		drawing.PatternTypeWave,
		drawing.PatternTypeZigZag,
		drawing.PatternTypeNarrowDownwardDiagonal,
		drawing.PatternTypeWeaveBraidNarrow,
		drawing.PatternTypeWideDownwardDiagonal,
		drawing.PatternTypeWeaveBraidBig,
		drawing.PatternTypeDashedUpDiagonal,
		drawing.PatternTypeDashedDownDiagonal,
		drawing.PatternTypeDivot,
		drawing.PatternTypeShingle,
		drawing.PatternTypeSphere,
		drawing.PatternTypeSmallCheckerCenter,
		drawing.PatternTypeLargeConfetti,
		drawing.PatternTypeSmallConfetti,
		drawing.PatternTypeSolidFill,
		drawing.PatternTypePercent5,
		drawing.PatternTypePercent50,
	}
	for _, pt := range types {
		fill := drawing.PatternFill{
			PatternType: pt,
			Foreground:  "4472C4",
			Background:  "FFFFFF",
			ForeAlpha:   1.0,
			BackAlpha:   1.0,
		}
		svg := drawingcss.SVGDefinition(fill, "test-pat")
		if svg == "" {
			t.Errorf("PatternType %q produced empty SVG definition", pt)
			continue
		}
		if !strings.Contains(svg, `id="test-pat"`) {
			t.Errorf("PatternType %q SVG missing id attribute: %s", pt, svg)
		}
		if !strings.Contains(svg, "</pattern>") {
			t.Errorf("PatternType %q SVG missing closing pattern tag: %s", pt, svg)
		}
	}
}

func TestPatternSVGNeedsSVGDefinitionTrue(t *testing.T) {
	fill := drawing.PatternFill{PatternType: drawing.PatternTypeDarkHorizontal, Foreground: "FF0000", Background: "FFFFFF"}
	if !drawingcss.NeedsSVGDefinition(fill) {
		t.Error("expected NeedsSVGDefinition to return true for PatternFill")
	}
}

func TestPatternSVGShortAliasesWork(t *testing.T) {
	aliases := []string{
		"dkhorz", "dkvert", "dkdndiag", "dkupdiag",
		"smgrid", "lggrid", "dashhorz", "dashvert",
		"lthorz", "ltvert", "ltdndiag", "ltupdiag",
	}
	for _, alias := range aliases {
		fill := drawing.PatternFill{
			PatternType: drawing.PatternType(alias),
			Foreground:  "000000",
			Background:  "FFFFFF",
			ForeAlpha:   1.0,
			BackAlpha:   1.0,
		}
		svg := drawingcss.SVGDefinition(fill, "alias-test")
		if svg == "" {
			t.Errorf("alias %q produced empty SVG definition", alias)
		}
	}
}

func TestPictureSVGDefinitionAppliesBlipEffects(t *testing.T) {
	fill := drawing.PictureFill{
		ContentType: "image/png",
		ImageData:   []byte{0x89, 0x50, 0x4E, 0x47},
		Alpha:       0.8,
		BlipEffects: []drawing.Effect{
			drawing.Grayscale{},
			drawing.AlphaMod{Amount: 50000},
		},
	}

	svg := drawingcss.SVGDefinition(fill, "picture-fill")
	if !strings.Contains(svg, `<pattern id="picture-fill"`) {
		t.Fatalf("expected picture fill pattern in SVG: %s", svg)
	}
	if !strings.Contains(svg, `style="filter:grayscale(100%)"`) {
		t.Fatalf("expected blip filter style on picture fill image: %s", svg)
	}
	if !strings.Contains(svg, `opacity="0.4000"`) {
		t.Fatalf("expected fill alpha and alphaMod opacity to be combined: %s", svg)
	}
}

func TestPictureSVGDefinitionTileMode(t *testing.T) {
	fill := drawing.PictureFill{
		ContentType: "image/png",
		ImageData:   []byte("invalid-png-data"),
		Tile:        true,
		TileScaleX:  200000,
		TileScaleY:  150000,
		TileOffsetX: 12700,
		TileOffsetY: 25400,
		TileFlip:    "x",
	}

	svg := drawingcss.SVGDefinition(fill, "tile-fill")
	if !strings.Contains(svg, `<pattern id="tile-fill"`) {
		t.Fatalf("expected tile pattern in SVG: %s", svg)
	}
	if !strings.Contains(svg, `patternUnits="userSpaceOnUse"`) {
		t.Fatalf("expected userSpaceOnUse for tile pattern: %s", svg)
	}
	// Default image size is 100x100 when data cannot be decoded
	if !strings.Contains(svg, `width="200.0000"`) {
		t.Fatalf("expected width scaled by 200%% (100*2): %s", svg)
	}
	if !strings.Contains(svg, `height="150.0000"`) {
		t.Fatalf("expected height scaled by 150%% (100*1.5): %s", svg)
	}
	if !strings.Contains(svg, `x="1.0000"`) {
		t.Fatalf("expected x offset 12700 EMU = 1 pt: %s", svg)
	}
	if !strings.Contains(svg, `y="2.0000"`) {
		t.Fatalf("expected y offset 25400 EMU = 2 pt: %s", svg)
	}
	if !strings.Contains(svg, `patternTransform="translate(200.0000, 0) scale(-1, 1)"`) {
		t.Fatalf("expected x-flip transform: %s", svg)
	}
}

func TestPictureSVGDefinitionTileModeNoImageData(t *testing.T) {
	fill := drawing.PictureFill{
		ImageURL:    "http://example.com/img.png",
		Tile:        true,
		TileScaleX:  100000,
		TileScaleY:  100000,
		TileOffsetX: 0,
		TileOffsetY: 0,
	}

	svg := drawingcss.SVGDefinition(fill, "tile-fill2")
	if !strings.Contains(svg, `<pattern id="tile-fill2"`) {
		t.Fatalf("expected tile pattern in SVG: %s", svg)
	}
	if !strings.Contains(svg, `patternUnits="userSpaceOnUse"`) {
		t.Fatalf("expected userSpaceOnUse for tile pattern: %s", svg)
	}
	if !strings.Contains(svg, `width="100.0000"`) {
		t.Fatalf("expected default width 100 when no image data: %s", svg)
	}
	if !strings.Contains(svg, `height="100.0000"`) {
		t.Fatalf("expected default height 100 when no image data: %s", svg)
	}
}
