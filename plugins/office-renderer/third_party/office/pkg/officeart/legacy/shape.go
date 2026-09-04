package legacy

import (
	"office/pkg/officeart/ir"
	"office/pkg/officeart/legacy/escher"
)

// ShapeFromEscher creates the shared, format-neutral portion of a shape.
// Application adapters still provide anchors, text story ownership and images.
func ShapeFromEscher(node *escher.ShapeNode) *ir.Shape {
	if node == nil {
		return nil
	}

	shape := &ir.Shape{
		ID:   node.ShapeID,
		Kind: ir.ShapeAuto,
		Geometry: ir.Geometry{
			Kind:   ir.GeometryPreset,
			Preset: ShapeTypeToPreset(node.ShapeType),
		},
		Source: ir.SourceRef{
			Record:  escher.RecordSpContainer,
			ShapeID: node.ShapeID,
		},
	}
	if node.HasClientTextbox {
		shape.Kind = ir.ShapeTextBox
		shape.Text = &ir.TextBody{}
	}

	visual := DecodeVisualProperties(node.Properties, nil)
	if visual.HasRotation {
		shape.Transform.RotationDeg = visual.RotationDeg
	}
	if visual.HasFill && !visual.FillEnabled {
		shape.Fill.Kind = ir.FillNone
	} else if visual.HasFillColor && visual.FillColor != "" {
		shape.Fill.Kind = ir.FillSolid
		shape.Fill.Color = visual.FillColor
		shape.Fill.Alpha = 1
	}
	if visual.HasLine && !visual.LineEnabled {
		shape.Line.Visible = false
	} else {
		if visual.HasLineColor && visual.LineColor != "" {
			shape.Line.Visible = true
			shape.Line.Color = visual.LineColor
			shape.Line.Alpha = 1
		}
		if visual.HasLineWidth {
			shape.Line.Visible = true
			shape.Line.WidthPt = float64(visual.LineWidthEMU) / 12700.0
		}
		if visual.HasLineDash {
			shape.Line.Dash = visual.LineDash
		}
	}
	if visual.HasCrop {
		if shape.Image == nil {
			shape.Image = &ir.Image{}
		}
		shape.Image.Crop = visual.Crop
	}

	text := visual.Text
	geometryText := DecodeGeometryText(node.Properties)
	if shape.Text != nil || geometryText.Present || text.HasLeft || text.HasTop || text.HasRight || text.HasBottom ||
		text.HasWrap || text.HasAnchor || text.HasVert || text.HasAutoFit {
		if shape.Text == nil {
			shape.Text = &ir.TextBody{}
		}
		if text.HasLeft {
			shape.Text.Insets.Left = text.Insets.Left
		}
		if text.HasTop {
			shape.Text.Insets.Top = text.Insets.Top
		}
		if text.HasRight {
			shape.Text.Insets.Right = text.Insets.Right
		}
		if text.HasBottom {
			shape.Text.Insets.Bottom = text.Insets.Bottom
		}
		if text.HasWrap {
			shape.Text.Wrap = text.Wrap
		}
		if text.HasAnchor {
			shape.Text.Anchor = text.Anchor
			shape.Text.AnchorCenter = text.AnchorCenter
		}
		if text.HasVert {
			shape.Text.Vert = text.Vert
		}
		if text.HasAutoFit {
			shape.Text.AutoFit = text.AutoFit
		}
	}

	if geometryText.Present && geometryText.Text != "" {
		if shape.Text == nil {
			shape.Text = &ir.TextBody{}
		}
		if geometryText.Vertical {
			shape.Text.Vert = ir.TextWordArtVertical
		}
		shape.Text.Paragraphs = append(shape.Text.Paragraphs, ir.Paragraph{
			Align: legacyGeometryTextAlign(geometryText.Align),
			Runs: []ir.TextRun{{
				Text:       geometryText.Text,
				FontFamily: geometryText.Font,
				FontSizePt: legacyGeometryTextSizePt(geometryText.Size),
				Bold:       geometryText.Bold,
				Italic:     geometryText.Italic,
			}},
		})
	}

	for _, child := range node.Children {
		if converted := ShapeFromEscher(child); converted != nil {
			shape.Children = append(shape.Children, converted)
		}
	}
	if len(shape.Children) > 0 {
		shape.Kind = ir.ShapeGroup
	}
	return shape
}

func legacyGeometryTextSizePt(raw int32) float64 {
	if raw <= 0 {
		return 0
	}
	return float64(raw) / 65536.0
}

func legacyGeometryTextAlign(raw uint32) string {
	switch raw {
	case 1:
		return "center"
	case 2:
		return "right"
	case 3:
		return "justify"
	default:
		return "left"
	}
}
