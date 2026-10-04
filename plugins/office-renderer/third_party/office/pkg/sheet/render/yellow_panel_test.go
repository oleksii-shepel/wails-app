package render

import (
	"regexp"
	"strings"
	"testing"

	"office/pkg/ooxml/drawing"
	"office/pkg/sheet/parsers/ooxml/resolve"
)

func TestYellowInstructionPanelDoesNotRenderShadow(t *testing.T) {
	wb := &resolve.Workbook{Sheets: []*resolve.Sheet{{
		Name: "FY months",
		Drawings: []*resolve.Drawing{{
			Name:     "How to create this rule",
			Type:     resolve.DrawingTypeShape,
			Position: resolve.DrawingPosition{Anchor: resolve.AnchorAbsolute, Width: 149 * 9525, Height: 17 * 9525},
			Shape:    &resolve.DrawingShape{Geometry: "rect", FillColor: "FFFFCC", Text: "How to create this rule"},
		}},
	}}}

	html, err := HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	panel := regexp.MustCompile(`title="How to create this rule"[^>]*>(?:<[^>]+>)*`).FindString(html)
	if panel == "" {
		t.Fatalf("yellow instructional panel missing from output")
	}
	if strings.Contains(panel, "drop-shadow(") || strings.Contains(panel, "box-shadow:") {
		t.Fatalf("yellow instructional panel unexpectedly has a shadow:\n%s", html)
	}
}

func TestDrawingBesideTableWithoutEffectDoesNotRenderShadow(t *testing.T) {
	wb := &resolve.Workbook{Sheets: []*resolve.Sheet{{
		Name: "Sheet1",
		Rows: []*resolve.Row{{Index: 1, Cells: []*resolve.Cell{{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellString, StrValue: "Table"}}}},
		Drawings: []*resolve.Drawing{{
			Name:     "Side panel",
			Type:     resolve.DrawingTypeShape,
			Position: resolve.DrawingPosition{Anchor: resolve.AnchorAbsolute, X: 80 * 9525, Width: 120 * 9525, Height: 40 * 9525},
			Shape:    &resolve.DrawingShape{Geometry: "rect", FillColor: "C6D9F1", Text: "Side panel"},
		}},
	}}}

	html, err := HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	panel := regexp.MustCompile(`title="Side panel"[^>]*>(?:<[^>]+>)*`).FindString(html)
	if panel == "" {
		t.Fatalf("side panel missing from output")
	}
	if strings.Contains(panel, "drop-shadow(") || strings.Contains(panel, "box-shadow:") {
		t.Fatalf("side drawing gained a shadow without one in its source:\n%s", html)
	}
}

func TestExamplePanelImageRendersShadow(t *testing.T) {
	wb := &resolve.Workbook{Sheets: []*resolve.Sheet{{
		Name: "Products1",
		Drawings: []*resolve.Drawing{{
			Name:     "Picture 2",
			Type:     resolve.DrawingTypeImage,
			Position: resolve.DrawingPosition{Anchor: resolve.AnchorAbsolute, Width: 500 * 9525, Height: 600 * 9525},
			Image: &resolve.DrawingImage{
				ContentType: "image/png", Data: []byte{137, 80, 78, 71},
				Effects: drawing.EffectList{Effects: []drawing.Effect{
					drawing.Shadow{Color: "000000", Alpha: 0.4, Blur: 5},
				}},
			},
		}},
	}}}

	html, err := HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(html, `title="Picture 2"`) || !strings.Contains(html, "filter:drop-shadow(0.0pt 0.0pt 5.0pt rgba(0, 0, 0, 0.40))") {
		t.Fatalf("example panel image shadow missing:\n%s", html)
	}
}
