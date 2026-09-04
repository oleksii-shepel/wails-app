package render_test

import (
	"strings"
	"testing"

	"office/pkg/sheet/parsers/ooxml/resolve"
	"office/pkg/sheet/render"
)

func TestHTMLRendersSemanticSmartArtNodes(t *testing.T) {
	wb := &resolve.Workbook{Sheets: []*resolve.Sheet{{
		Name: "Sheet1",
		Rows: []*resolve.Row{{Index: 1, Cells: []*resolve.Cell{{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellString, StrValue: "diagram"}}}},
		Drawings: []*resolve.Drawing{{
			Name:     "Org",
			Type:     resolve.DrawingTypeSmartArt,
			Position: resolve.DrawingPosition{Anchor: resolve.AnchorAbsolute, Width: 500 * 9525, Height: 300 * 9525},
			SmartArt: &resolve.DrawingSmartArt{
				DiagramType:   "hierarchy",
				SemanticDirty: true,
				Nodes: []*resolve.SmartArtNode{
					{ID: "pt1", Type: "root", Text: "Chief Executive", FillColor: "4472C4", TextColor: "FFFFFF", Children: []string{"pt2"}},
					{ID: "pt2", Type: "node", Text: "Engineering", FillColor: "70AD47", TextColor: "FFFFFF"},
				},
				Connections: []resolve.SmartArtConnection{{SrcID: "pt1", DestID: "pt2"}},
			},
		}},
	}}}

	html, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(html, `drawing--smartart-semantic`) {
		t.Fatalf("expected semantic SmartArt wrapper: %s", html)
	}
	if !strings.Contains(html, "Chief Executive") || !strings.Contains(html, "Engineering") {
		t.Fatalf("expected SmartArt node text: %s", html)
	}
	if !strings.Contains(html, `drawing--connector`) {
		t.Fatalf("expected SmartArt connector: %s", html)
	}
}
