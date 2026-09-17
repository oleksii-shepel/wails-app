package legacy

import (
	"testing"

	officeartescher "office/pkg/officeart/legacy/escher"
)

func TestLegacyXLSOfficeArtPaletteIndexUsesWorkbookPalette(t *testing.T) {
	catalog := legacyStyleCatalog{Palette: defaultBIFFPalette}
	catalog.Palette[0] = "123456" // BIFF palette index 0x08.
	resolve := legacyXLSOfficeArtColorResolver(catalog)

	if got := resolve(0x01000008); got != "123456" {
		t.Fatalf("palette-index COLORREF = %q, want 123456", got)
	}
}

func TestLegacyXLSOfficeArtPaletteRGBPreservesAuthoredRGB(t *testing.T) {
	resolve := legacyXLSOfficeArtColorResolver()
	if got := resolve(0x02332211); got != "112233" {
		t.Fatalf("palette-RGB COLORREF = %q, want 112233", got)
	}
}

func TestLegacyXLSOfficeArtShapeColorsUseWorkbookPaletteContext(t *testing.T) {
	catalog := legacyStyleCatalog{Palette: defaultBIFFPalette}
	catalog.Palette[0] = "A1B2C3"
	catalog.Palette[1] = "D4E5F6"

	shape := &legacyWorksheetShape{}
	parseLegacyWorksheetShapePropertiesParsed([]officeartescher.Property{
		{ID: officeartescher.PropFillColor, Value: 0x01000008},
		{ID: officeartescher.PropLineColor, Value: 0x01000009},
	}, shape, legacyXLSOfficeArtColorResolver(catalog))

	if shape.fillColor != "A1B2C3" {
		t.Fatalf("fill = %q, want A1B2C3", shape.fillColor)
	}
	if shape.lineColor != "D4E5F6" {
		t.Fatalf("line = %q, want D4E5F6", shape.lineColor)
	}
}
