package legacy

import (
	dml "office/pkg/ooxml/drawing"
	wordir "office/pkg/word/ir"
	docbinary "office/pkg/word/parsers/doc/binary"
)

// These helpers exist only for writer compatibility assertions. Production DOC
// projection is canonical in pkg/word/parsers/doc/binary and doc/officeart.
func legacyCloneDrawingImage(template *wordir.Image, anchor legacyFloatingDrawingAnchor, shapeProps map[uint32]legacyOfficeArtShapeProps) *wordir.Image {
	if template == nil {
		return nil
	}
	cloned := *template
	if !docbinary.ApplyAnchorToImage(&cloned, anchor) {
		return nil
	}
	if props, ok := shapeProps[anchor.Shape]; ok {
		if props.HasRotation {
			cloned.RotationDeg = props.Rotation
		}
		cloned.FlipH = props.FlipH
		cloned.FlipV = props.FlipV
		cloned.CropTop = props.CropTop
		cloned.CropBottom = props.CropBottom
		cloned.CropLeft = props.CropLeft
		cloned.CropRight = props.CropRight
		cloned.Brightness = props.Brightness
		cloned.Contrast = props.Contrast
		cloned.Grayscale = props.Grayscale
		cloned.BiLevel = props.BiLevel
	}
	return &cloned
}

func legacyCloneDrawingShape(anchor legacyFloatingDrawingAnchor, shapeProps map[uint32]legacyOfficeArtShapeProps) *wordir.Shape {
	shape := &wordir.Shape{Kind: "rect", LegacyShapeID: anchor.Shape}
	if props, ok := shapeProps[anchor.Shape]; ok {
		if props.GeometryData != nil {
			shape.Kind = "custom"
			shape.GeometryData = props.GeometryData
		} else if preset := legacyOfficeArtShapeTypeToGeometry(props.ShapeType); preset != "" {
			shape.Kind = preset
			geometry := &dml.PresetGeometry{Name: preset}
			for _, adjust := range props.adjustments {
				if adjust != nil {
					geometry.Adjustments = append(geometry.Adjustments, float64(*adjust))
				}
			}
			shape.Geometry = geometry
		}
		if props.HasRotation {
			shape.RotationDeg = props.Rotation
		}
		shape.FlipH = props.FlipH
		shape.FlipV = props.FlipV
		shape.FillColor = props.FillColor
		shape.StrokeColor = props.StrokeColor
		shape.StrokeWidth = props.StrokeWidth
	}
	if !docbinary.ApplyAnchorToShape(shape, anchor) {
		return nil
	}
	return shape
}
