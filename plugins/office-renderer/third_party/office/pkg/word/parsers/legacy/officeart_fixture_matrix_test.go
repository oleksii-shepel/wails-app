package legacy

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	dml "office/pkg/ooxml/drawing"
	wordir "office/pkg/word/ir"
	"office/pkg/word/render"
)

type officeArtFixtureExpectation struct {
	name              string
	reference         string
	minShapes         int
	minTextBoxes      int
	requireStroke     bool
	requireArrow      bool
	requireShadow     bool
	requirePadding    bool
	requireVertical   bool
	rotationDeg       float64
	requireGroup      bool
	requireTextPath   bool
	requireRenderText string
}

func TestLegacyOfficeArtFixtureMatrix(t *testing.T) {
	fixtures := []officeArtFixtureExpectation{
		{name: "legacy-officeart-line-solid.doc", reference: "legacy-officeart-line-solid.reference.pdf", minShapes: 1, requireStroke: true},
		{name: "legacy-officeart-arrows.doc", reference: "legacy-officeart-arrows.reference.pdf", minShapes: 1, requireArrow: true},
		{name: "legacy-officeart-shadow.doc", reference: "legacy-officeart-shadow.reference.pdf", minShapes: 1, requireShadow: true},
		{name: "legacy-officeart-textbox-padding.doc", reference: "legacy-officeart-textbox-padding.reference.pdf", minTextBoxes: 1, requirePadding: true, requireRenderText: "Padded textbox"},
		{name: "legacy-officeart-textbox-vertical.doc", reference: "legacy-officeart-textbox-vertical.reference.pdf", minTextBoxes: 1, requireVertical: true, requireRenderText: "Vertical textbox"},
		{name: "legacy-officeart-rotation.doc", reference: "legacy-officeart-rotation.reference.pdf", minShapes: 1, rotationDeg: 27},
		{name: "legacy-officeart-overlap-zorder.doc", reference: "legacy-officeart-overlap-zorder.reference.pdf", minShapes: 2},
		{name: "legacy-officeart-group.doc", reference: "legacy-officeart-group.reference.pdf", minShapes: 3, requireGroup: true},
		{name: "legacy-officeart-autoshapes.doc", reference: "legacy-officeart-autoshapes.reference.pdf", minShapes: 2},
		// Complex OfficeArt fixtures are now semantic regression contracts rather
		// than parse-only smoke tests.
		{name: "legacy-textboxes-advanced.doc", reference: "legacy-textboxes-advanced.reference.docx", minTextBoxes: 5, requirePadding: true, requireVertical: true},
		{name: "legacy-arrows-shadows.doc", reference: "legacy-arrows-shadows.reference.docx", minShapes: 1, requireArrow: true, requireShadow: true},
		{name: "legacy-officeart-mixed.doc", reference: "legacy-officeart-mixed.reference.docx", minShapes: 1},
		{name: "legacy-wordart-fontwork.doc", reference: "legacy-wordart-fontwork.reference.docx", minShapes: 1, requireTextPath: true},
	}

	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(fixture.name, func(t *testing.T) {
			fixtureRoot := filepath.Join("..", "..", "..", "..", "testdata", "doc")
			path := filepath.Join(fixtureRoot, fixture.name)
			referencePath := filepath.Join(fixtureRoot, "officeart-reference", fixture.reference)
			if _, err := os.Stat(referencePath); err != nil {
				t.Fatalf("reference %s: %v", fixture.reference, err)
			}
			doc, err := ParseDoc(path)
			if err != nil {
				t.Fatalf("ParseDoc(%s): %v", fixture.name, err)
			}
			if doc == nil {
				t.Fatalf("ParseDoc(%s): nil document", fixture.name)
			}

			shapes := collectOfficeArtFixtureShapes(doc)
			textBoxes := collectOfficeArtFixtureTextBoxes(doc)
			if len(shapes) < fixture.minShapes {
				t.Fatalf("%s: shapes=%d, want >=%d", fixture.name, len(shapes), fixture.minShapes)
			}
			if len(textBoxes) < fixture.minTextBoxes {
				t.Fatalf("%s: textboxes=%d, want >=%d", fixture.name, len(textBoxes), fixture.minTextBoxes)
			}

			if fixture.requireStroke && !officeArtFixtureHasStroke(shapes) {
				t.Fatalf("%s: expected a shape with explicit stroke", fixture.name)
			}
			if fixture.requireArrow && !officeArtFixtureHasArrow(shapes) {
				t.Fatalf("%s: expected projected OfficeArt arrowhead", fixture.name)
			}
			if fixture.requireShadow && !officeArtFixtureHasShadow(shapes) {
				t.Fatalf("%s: expected projected OfficeArt shadow", fixture.name)
			}
			if fixture.requirePadding && !officeArtFixtureHasPadding(textBoxes) {
				t.Fatalf("%s: expected non-zero textbox insets", fixture.name)
			}
			if fixture.requireVertical && !officeArtFixtureHasVerticalText(textBoxes) {
				t.Fatalf("%s: expected vertical textbox semantics", fixture.name)
			}
			if fixture.rotationDeg != 0 && !officeArtFixtureHasRotation(shapes, fixture.rotationDeg) {
				t.Fatalf("%s: expected shape rotation near %.1f degrees", fixture.name, fixture.rotationDeg)
			}
			if fixture.requireGroup && !officeArtFixtureHasGroup(shapes) {
				t.Fatalf("%s: expected group shape with children", fixture.name)
			}
			if fixture.requireTextPath && !officeArtFixtureHasTextPath(shapes) {
				t.Fatalf("%s: expected legacy WordArt/TextPath semantics", fixture.name)
			}

			html, err := render.HTML(doc)
			if err != nil {
				t.Fatalf("HTML(%s): %v", fixture.name, err)
			}
			if !strings.Contains(html, "<!DOCTYPE html>") {
				t.Fatalf("HTML(%s): missing HTML envelope", fixture.name)
			}
			if fixture.requireRenderText != "" && !strings.Contains(html, fixture.requireRenderText) {
				t.Fatalf("HTML(%s): missing %q", fixture.name, fixture.requireRenderText)
			}
		})
	}
}

func collectOfficeArtFixtureShapes(doc *wordir.Document) []*wordir.Shape {
	var out []*wordir.Shape
	var walkShape func(*wordir.Shape)
	walkShape = func(shape *wordir.Shape) {
		if shape == nil {
			return
		}
		out = append(out, shape)
		for i := range shape.Children {
			walkShape(&shape.Children[i])
		}
	}
	var walkBlocks func([]wordir.Block)
	walkBlocks = func(blocks []wordir.Block) {
		for i := range blocks {
			block := &blocks[i]
			switch block.Kind {
			case "paragraph":
				if block.Paragraph == nil {
					continue
				}
				for ri := range block.Paragraph.Runs {
					run := &block.Paragraph.Runs[ri]
					walkShape(run.Shape)
					if run.TextBox != nil {
						walkBlocks(run.TextBox.Blocks)
					}
				}
			case "table":
				if block.Table == nil {
					continue
				}
				for _, row := range block.Table.Rows {
					for _, cell := range row.Cells {
						walkBlocks(cell.Blocks)
					}
				}
			}
		}
	}
	if doc != nil {
		walkBlocks(doc.Blocks)
	}
	return out
}

func collectOfficeArtFixtureTextBoxes(doc *wordir.Document) []*wordir.TextBox {
	var out []*wordir.TextBox
	for _, shape := range collectOfficeArtFixtureShapes(doc) {
		if shape.TextBox != nil {
			out = append(out, shape.TextBox)
		}
	}
	if doc == nil {
		return out
	}
	for pi := range doc.Paragraphs {
		for ri := range doc.Paragraphs[pi].Runs {
			run := &doc.Paragraphs[pi].Runs[ri]
			if run.Kind == "textbox" && run.TextBox != nil {
				out = append(out, run.TextBox)
			}
		}
	}
	return out
}

func officeArtFixtureHasStroke(shapes []*wordir.Shape) bool {
	for _, shape := range shapes {
		if shape != nil && shape.StrokeColor != "" && shape.StrokeWidth > 0 {
			return true
		}
	}
	return false
}

func officeArtFixtureHasArrow(shapes []*wordir.Shape) bool {
	for _, shape := range shapes {
		if shape == nil || shape.LineStyle == nil {
			continue
		}
		line := shape.LineStyle
		if (line.HeadEnd != nil && line.HeadEnd.Type != dml.ArrowheadNone) ||
			(line.TailEnd != nil && line.TailEnd.Type != dml.ArrowheadNone) {
			return true
		}
	}
	return false
}

func officeArtFixtureHasShadow(shapes []*wordir.Shape) bool {
	for _, shape := range shapes {
		if shape == nil {
			continue
		}
		for _, effect := range shape.Effects.Effects {
			if effect != nil && effect.Type() == dml.EffectTypeShadow {
				return true
			}
		}
	}
	return false
}

func officeArtFixtureHasPadding(boxes []*wordir.TextBox) bool {
	for _, box := range boxes {
		if box != nil && (box.PaddingLeft > 0 || box.PaddingRight > 0 || box.PaddingTop > 0 || box.PaddingBottom > 0) {
			return true
		}
	}
	return false
}

func officeArtFixtureHasVerticalText(boxes []*wordir.TextBox) bool {
	for _, box := range boxes {
		if box != nil && box.Vert != "" {
			return true
		}
	}
	return false
}

func officeArtFixtureHasRotation(shapes []*wordir.Shape, want float64) bool {
	for _, shape := range shapes {
		if shape != nil && math.Abs(shape.RotationDeg-want) <= 1 {
			return true
		}
	}
	return false
}

func officeArtFixtureHasGroup(shapes []*wordir.Shape) bool {
	for _, shape := range shapes {
		if shape != nil && shape.Kind == "group" && len(shape.Children) >= 2 {
			return true
		}
	}
	return false
}

func officeArtFixtureHasTextPath(shapes []*wordir.Shape) bool {
	for _, shape := range shapes {
		if shape != nil && shape.TextPath != nil &&
			shape.TextPath.String != "" &&
			(shape.TextPath.On == "t" || shape.TextPath.On == "true" || shape.TextPath.On == "1") {
			return true
		}
	}
	return false
}
