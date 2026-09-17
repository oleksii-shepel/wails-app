package resolve

import (
	"os"
	"strings"
	"testing"
)

func TestResolveEntryPointRemainsCompositionOrchestration(t *testing.T) {
	data, err := os.ReadFile("resolve.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, forbidden := range []string{
		"func resolveSmartArt(",
		"func resolveTableFrame(",
		"func resolveParagraph(",
		"func resolveRun(",
		"func resolveNotesSlide(",
		"func resolvePicture(",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("resolve.go regained domain implementation %q; keep the entry point orchestration-only", forbidden)
		}
	}
	if lines := strings.Count(source, "\n") + 1; lines > 500 {
		t.Fatalf("resolve.go grew to %d lines; composition orchestration must stay bounded", lines)
	}
}

func TestVisualInheritanceOwnedByLayoutCascade(t *testing.T) {
	data, err := os.ReadFile("visual_composition.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, forbidden := range []string{
		"func effectiveShapeXfrm(",
		"func effectiveShapeFill(",
		"func effectiveShapeLine(",
		"func effectiveShapeEffectList(",
		"func materializeShapeFill(",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("visual resolver retained layout-owned implementation %q", forbidden)
		}
	}
	for _, required := range []string{
		"cascade.EffectiveShapeTransform(",
		"cascade.EffectiveShapeFill(",
		"cascade.EffectiveShapeLine(",
		"cascade.EffectiveShapeEffects(",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("visual resolver must consume layout cascade %q", required)
		}
	}
}

func TestGroupCoordinateCompositionOwnedByLayoutCascade(t *testing.T) {
	for _, name := range []string{"text_notes.go", "visual_composition.go"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		for _, forbidden := range []string{"func applyGroupTransformToChildBounds", "func applyGroupTransformToPoint", "func applyGroupTransformToShape"} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s retained layout-owned group coordinate helper %q", name, forbidden)
			}
		}
	}
}

func TestBackgroundCompositionLivesInLayoutCascade(t *testing.T) {
	data, err := os.ReadFile("visual_composition.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, forbidden := range []string{"func resolveBgColor(", "func resolveBackgroundFill(", "func resolveThemeFormatFillStyle("} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("resolver retained background composition surface %q", forbidden)
		}
	}
}

func TestShapeStyleMatrixCompositionOwnedByLayoutCascade(t *testing.T) {
	data, err := os.ReadFile("visual_composition.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, forbidden := range []string{
		"func resolveShapeFillRef(", "func resolveShapeLineRef(", "func resolveShapeEffectRef(",
		"func resolveShapeFontRef(", "func resolveShapeStyleFill(", "func resolveShapeStyleLine(",
		"func resolveShapeStyleEffect(", "func resolveShapeStyleFontRef(",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("resolver retained layout-owned shape style matrix surface %q", forbidden)
		}
	}
	for _, required := range []string{"cascade.ShapeStyleFill(", "cascade.ShapeStyleLine(", "cascade.ShapeStyleEffects(", "cascade.ShapeStyleFontFamily("} {
		if !strings.Contains(source, required) {
			t.Fatalf("visual resolver must consume layout style cascade %q", required)
		}
	}
}

func TestTableVisualCascadeOwnedByLayout(t *testing.T) {
	data, err := os.ReadFile("visual_composition.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, forbidden := range []string{
		"func effectiveTableStyle(",
		"func tableColumnCount(",
		"func resolveTableCellFill(",
		"func resolveTableBorderStyle(",
		"func resolveTableLineRefStyle(",
		"func resolveTableFillRef(",
		"func resolveTableEffectRef(",
		"func resolveTableSchemeColor(",
		"func resolveTableCellStyleDefaults(",
		"func mergeResolvedTableCellStyle(",
		"func resolveTableTextDefaults(",
		"func parseOnOffStyle(",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("OOXML resolver retained table visual cascade ownership %q", forbidden)
		}
	}
}

func TestSmartArtThemeCompositionOwnedByLayoutCascade(t *testing.T) {
	visualData, err := os.ReadFile("visual_composition.go")
	if err != nil {
		t.Fatal(err)
	}
	visual := string(visualData)
	for _, forbidden := range []string{"func buildSmartArtFormatScheme(", "func usesPlaceholderFill("} {
		if strings.Contains(visual, forbidden) {
			t.Fatalf("visual resolver retained SmartArt theme composition %q", forbidden)
		}
	}
	resourceData, err := os.ReadFile("smartart_resources.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(resourceData), "cascade.SmartArtFormatScheme(") {
		t.Fatal("SmartArt resolver must consume layout-owned theme format projection")
	}
}

func TestTextBodyCompositionOwnedByLayoutCascade(t *testing.T) {
	data, err := os.ReadFile("visual_composition.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, forbidden := range []string{"type textBodyStyle struct", "func resolveTextBodyStyle(", "func applyTextBodyStyle("} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("resolver retained layout-owned text body composition %q", forbidden)
		}
	}
	if !strings.Contains(source, "cascade.EffectiveTextBodyStyle(") {
		t.Fatal("visual resolver must consume layout-owned text body cascade")
	}
}

func TestTextDefaultCascadeOwnedByLayoutCascade(t *testing.T) {
	data, err := os.ReadFile("text_notes.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, forbidden := range []string{
		"type textDefaults struct", "func resolveTextDefaults(", "func mergeTextDefaults(",
		"func defaultsFromMaster(", "func defaultsFromListStyle(", "func mergeEndParaRPrDefaults(",
		"func fallbackThemeFont(", "func resolveThemeTypeface(",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("OOXML resolver retained text-default cascade ownership %q", forbidden)
		}
	}
	if !strings.Contains(source, "cascade.EffectiveTextDefaults(") {
		t.Fatal("text resolver must consume layout-owned effective text defaults")
	}
}

func TestParagraphAndRunLocalCascadeOwnedByLayout(t *testing.T) {
	data, err := os.ReadFile("text_notes.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, forbidden := range []string{
		"if ap.Properties.MarL != nil",
		"if props.Bold != nil",
		"if props.Italic != nil",
		"cascade.RunColor(props",
		"cascade.RunFontFamily(props",
		"cascade.RunHighlight(props",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("OOXML resolver retained paragraph/run-local cascade ownership %q", forbidden)
		}
	}
	for _, required := range []string{"cascade.ParagraphFromLocal(", "cascade.RunFromLocal("} {
		if !strings.Contains(source, required) {
			t.Fatalf("text resolver must consume layout-owned local text cascade %q", required)
		}
	}
}
