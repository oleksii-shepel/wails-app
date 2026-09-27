package cascade

import (
	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/drawingxml"
	"office/pkg/ooxml/media"
	"office/pkg/presentation/ir"
	"office/pkg/presentation/parsers/ooxml/parts"
)

// EffectiveShapeTransform resolves the shape transform across the
// slide -> layout -> master inheritance chain.  Composition owns this choice;
// renderers and OOXML projection code should only consume the selected value.
func EffectiveShapeTransform(shape, layoutShape, masterShape *parts.Shape) *parts.Transform2D {
	if shape != nil && shape.SpPr.Xfrm != nil {
		return shape.SpPr.Xfrm
	}
	if layoutShape != nil && layoutShape.SpPr.Xfrm != nil {
		return layoutShape.SpPr.Xfrm
	}
	if masterShape != nil && masterShape.SpPr.Xfrm != nil {
		return masterShape.SpPr.Xfrm
	}
	return nil
}

// RectFromTransform projects DrawingML transform geometry into canonical IR.
func RectFromTransform(xfrm *parts.Transform2D) ir.Rect {
	if xfrm == nil {
		return ir.Rect{}
	}
	return ir.Rect{X: xfrm.Off.X, Y: xfrm.Off.Y, W: xfrm.Ext.CX, H: xfrm.Ext.CY}
}

// RotationDegrees returns the canonical clockwise rotation in degrees.
func RotationDegrees(xfrm *parts.Transform2D) float64 {
	if xfrm == nil || xfrm.Rot == 0 {
		return 0
	}
	return ir.CanonicalRotationDeg(float64(xfrm.Rot) / 60000.0)
}

// EffectiveShapeFill resolves explicit shape fill inheritance and materializes
// relationship-backed picture fills at the same composition boundary.
func EffectiveShapeFill(shape, layoutShape, masterShape *parts.Shape,
	slideColors, layoutColors, masterColors map[string]string,
	slideMedia, layoutMedia, masterMedia map[string]media.Part) drawing.Fill {
	if shape != nil {
		if fill := shape.SpPr.DrawingFill(slideColors); fill != nil {
			return MaterializeFill(fill, slideMedia)
		}
	}
	if layoutShape != nil {
		if fill := layoutShape.SpPr.DrawingFill(layoutColors); fill != nil {
			return MaterializeFill(fill, layoutMedia)
		}
	}
	if masterShape != nil {
		if fill := masterShape.SpPr.DrawingFill(masterColors); fill != nil {
			return MaterializeFill(fill, masterMedia)
		}
	}
	return nil
}

// EffectiveShapeLine resolves explicit line inheritance.  An explicit no-fill
// line remains authoritative and therefore must not fall through to ancestors.
func EffectiveShapeLine(shape, layoutShape, masterShape *parts.Shape,
	slideColors, layoutColors, masterColors map[string]string) *drawing.LineStyle {
	if shape != nil && shape.SpPr.Line != nil {
		return shape.SpPr.Line.DrawingLineStyle(slideColors)
	}
	if layoutShape != nil && layoutShape.SpPr.Line != nil {
		return layoutShape.SpPr.Line.DrawingLineStyle(layoutColors)
	}
	if masterShape != nil && masterShape.SpPr.Line != nil {
		return masterShape.SpPr.Line.DrawingLineStyle(masterColors)
	}
	return nil
}

// EffectiveShapeEffects resolves the first explicit effect list in the
// slide -> layout -> master chain.
func EffectiveShapeEffects(shape, layoutShape, masterShape *parts.Shape,
	slideColors, layoutColors, masterColors map[string]string) drawing.EffectList {
	if shape != nil {
		if effects := shape.SpPr.DrawingEffectList(slideColors); len(effects.Effects) > 0 {
			return effects
		}
	}
	if layoutShape != nil {
		if effects := layoutShape.SpPr.DrawingEffectList(layoutColors); len(effects.Effects) > 0 {
			return effects
		}
	}
	if masterShape != nil {
		if effects := masterShape.SpPr.DrawingEffectList(masterColors); len(effects.Effects) > 0 {
			return effects
		}
	}
	return drawing.EffectList{}
}

// MaterializeFill binds an already-resolved picture fill to its package media.
func MaterializeFill(fill drawing.Fill, mediaParts map[string]media.Part) drawing.Fill {
	if fill == nil || len(mediaParts) == 0 {
		return fill
	}
	switch f := fill.(type) {
	case drawing.PictureFill:
		if f.EmbedID == "" {
			return f
		}
		if part, ok := mediaParts[f.EmbedID]; ok {
			f.ImageData = part.Data
			f.ContentType = part.ContentType
		}
		return f
	case *drawing.PictureFill:
		if f == nil || f.EmbedID == "" {
			return fill
		}
		if part, ok := mediaParts[f.EmbedID]; ok {
			clone := *f
			clone.ImageData = part.Data
			clone.ContentType = part.ContentType
			return &clone
		}
	}
	return fill
}

// EffectiveShapePictureTile follows the same fill-inheritance decision as
// EffectiveShapeFill and projects tile metadata into canonical IR.
func EffectiveShapePictureTile(shape, layoutShape, masterShape *parts.Shape,
	slideColors, layoutColors, masterColors map[string]string) *ir.PictureTile {
	candidates := []struct {
		shape  *parts.Shape
		colors map[string]string
	}{
		{shape, slideColors}, {layoutShape, layoutColors}, {masterShape, masterColors},
	}
	for _, candidate := range candidates {
		if candidate.shape == nil {
			continue
		}
		if fill := candidate.shape.SpPr.DrawingFill(candidate.colors); fill != nil {
			if candidate.shape.SpPr.BlipFill != nil {
				return pictureTile(candidate.shape.SpPr.BlipFill.Tile)
			}
			return nil
		}
	}
	return nil
}

func pictureTile(tile *drawingxml.Tile) *ir.PictureTile {
	if tile == nil {
		return nil
	}
	return &ir.PictureTile{Tx: int64(tile.Tx), Ty: int64(tile.Ty), Sx: int(tile.Sx), Sy: int(tile.Sy), Flip: tile.Flip, Alignment: tile.Algn}
}

// CanonicalTransform projects DrawingML transform syntax into the format-neutral
// visual-composition transform. This is the OOXML adapter boundary: composition
// itself does not depend on PresentationML parts.
func CanonicalTransform(xfrm *parts.Transform2D) ir.VisualTransform {
	if xfrm == nil {
		return ir.VisualTransform{}
	}
	return ir.VisualTransform{
		Offset:      ir.Point{X: xfrm.Off.X, Y: xfrm.Off.Y},
		Extent:      ir.VisualSize{W: xfrm.Ext.CX, H: xfrm.Ext.CY},
		ChildOffset: ir.Point{X: xfrm.ChOff.X, Y: xfrm.ChOff.Y},
		ChildExtent: ir.VisualSize{W: xfrm.ChExt.CX, H: xfrm.ChExt.CY},
		Rotation:    RotationDegrees(xfrm),
		FlipH:       xfrm.FlipH,
		FlipV:       xfrm.FlipV,
	}
}

// GroupChildRect is retained as the OOXML-facing compatibility adapter. The
// coordinate-system semantics now belong to presentation/ir.
func GroupChildRect(child ir.Rect, group *parts.Transform2D) ir.Rect {
	if group == nil {
		return child
	}
	return ir.MapVisualRect(child, CanonicalTransform(group))
}

// GroupChildPoint is retained as the OOXML-facing compatibility adapter.
func GroupChildPoint(point *ir.Point, group *parts.Transform2D) *ir.Point {
	if point == nil || group == nil {
		return point
	}
	return ir.MapVisualPoint(point, CanonicalTransform(group))
}

// ApplyGroupTransform adapts DrawingML xfrm into canonical presentation IR. The
// actual group semantics are shared with legacy PPT and future parsers.
func ApplyGroupTransform(shape *ir.Shape, group *parts.Transform2D) {
	if shape == nil || group == nil {
		return
	}
	ir.ApplyVisualTransform(shape, CanonicalTransform(group))
}
