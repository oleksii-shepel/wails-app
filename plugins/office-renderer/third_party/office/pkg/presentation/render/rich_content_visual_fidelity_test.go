package render

import (
	"os"
	"strings"
	"testing"

	"office/pkg/ooxml/drawing"
	"office/pkg/presentation/ir"
)

func TestRichContentVisualFidelityDoesNotReintroduceBrowserLayoutGuesses(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	tableStart := strings.Index(s, "func renderTableShape(")
	tableEnd := strings.Index(s[tableStart:], "\nfunc renderTextShape(")
	table := s[tableStart : tableStart+tableEnd]
	if strings.Contains(table, "min-height:") {
		t.Fatal("physical PowerPoint table frame must not become browser-content-driven min-height")
	}
	if !strings.Contains(table, "physicalEffectColor(stroke.Color, stroke.Alpha)") {
		t.Fatal("table border serialization must preserve physical stroke alpha")
	}

	smartStart := strings.Index(s, "func renderSmartArtShape(")
	smartEnd := strings.Index(s[smartStart:], "\n// renderSmartArtVectorShape")
	smart := s[smartStart : smartStart+smartEnd]
	if strings.Contains(smart, `} else {\n\t\t\ttextStyles = append(textStyles, "text-align:center")`) {
		t.Fatal("SmartArt renderer must not force non-anchor-centered content to centered text")
	}
}

func TestLegacyUnnamedConnectorDoesNotSerializeAsEmptySVG(t *testing.T) {
	connector := &ir.Shape{
		Kind: ir.ShapeConnector, Bounds: ir.Rect{X: 914400, Y: 914400, W: 1828800, H: 914400}, LineColor: "FFFF00", LineWidthPt: 1.5,
	}
	var out strings.Builder
	renderVisualShapes(&out, []*ir.Shape{connector}, 720, 540, &htmlRenderContext{})
	html := out.String()
	if !strings.Contains(html, `<line `) {
		t.Fatalf("legacy unnamed connector serialized without physical line geometry: %s", html)
	}
	if strings.Contains(html, `xmlns="http://www.w3.org/2000/svg"></svg>`) {
		t.Fatalf("legacy unnamed connector emitted an empty SVG layer: %s", html)
	}
}

func TestPhysicalTableRendererOwnsCellClipAndVerticalPlacement(t *testing.T) {
	shape := &ir.Shape{Kind: ir.ShapeTable, Bounds: ir.Rect{W: 1270000, H: 254000}, Table: &ir.Table{
		ColumnWidthsEMU: []int64{1270000},
		Rows:            []*ir.TableRow{{HeightEMU: 254000, Cells: []*ir.TableCell{{TextAnchor: "ctr", Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{Text: "Cell"}}}}}}}},
	}}
	var out strings.Builder
	renderVisualShapes(&out, []*ir.Shape{shape}, 720, 540, &htmlRenderContext{})
	html := out.String()
	if !strings.Contains(html, `class="ppt-table__cell-content"`) || !strings.Contains(html, "justify-content:center") || !strings.Contains(html, "overflow:hidden") {
		t.Fatalf("table cell did not serialize physical content box: %s", html)
	}
	if strings.Contains(html, "overflow-wrap:anywhere") {
		t.Fatalf("table renderer reintroduced browser word splitting: %s", html)
	}
}

func TestPresentationTableCSSDoesNotForceTransparentCellsWhite(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if strings.Contains(s, `.ppt-table { width: 100%; height: 100%; border-collapse: collapse; table-layout: fixed; background: #fff; }`) {
		t.Fatal("presentation table CSS must not force transparent table surfaces to white")
	}
}

func TestPhysicalTableCSSDoesNotDelegateSharedEdgeConflictsToBrowser(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if strings.Contains(s, "border-collapse: collapse") {
		t.Fatal("physical table renderer must not delegate shared-edge conflict resolution to browser border-collapse")
	}
	if !strings.Contains(s, "border-collapse: separate; border-spacing: 0") {
		t.Fatal("physical table renderer must serialize zero-spacing independently-owned cell edges")
	}
}

func TestPhysicalTableRendererSerializesMergedGridEdgeLayer(t *testing.T) {
	shape := &ir.Shape{Kind: ir.ShapeTable, Bounds: ir.Rect{W: 3810000, H: 127000}, Table: &ir.Table{
		ColumnWidthsEMU: []int64{1270000, 1270000, 1270000},
		Rows: []*ir.TableRow{{HeightEMU: 127000, Cells: []*ir.TableCell{
			{GridSpan: 2, BorderRight: &drawing.LineStyle{Width: 2, Color: "111111"}}, {HMerge: true},
			{BorderLeft: &drawing.LineStyle{Width: 3, Color: "222222"}},
		}}},
	}}
	var out strings.Builder
	renderVisualShapes(&out, []*ir.Shape{shape}, 720, 540, &htmlRenderContext{})
	html := out.String()
	if !strings.Contains(html, `class="ppt-table__edge-layer"`) || !strings.Contains(html, `ppt-table__edge--v`) {
		t.Fatalf("merged table did not serialize physical grid edge layer: %s", html)
	}
	if !strings.Contains(html, "border-left:3.00pt solid rgba(34,34,34,1.0000)") {
		t.Fatalf("merged shared edge winner not serialized physically: %s", html)
	}
	if strings.Contains(html, "border-right:2.00pt") {
		t.Fatalf("losing merged-cell edge leaked back into cell CSS: %s", html)
	}
}
