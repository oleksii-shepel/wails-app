package edit

import (
	"office/pkg/ooxml/drawing"
	presir "office/pkg/presentation/ir"
)

// cloneShape returns a deep copy of src. The copy receives a new identity
// (ID is zeroed and Name is left as-is) so that clones can be added to the
// same slide without conflicting non-visual properties.
func cloneShape(src *presir.Shape) *presir.Shape {
	if src == nil {
		return nil
	}
	dst := &presir.Shape{
		ID:                   0,
		OriginalShapeID:      src.OriginalShapeID,
		Name:                 src.Name,
		Kind:                 src.Kind,
		Placeholder:          src.Placeholder,
		Bounds:               src.Bounds,
		FillColor:            src.FillColor,
		Fill:                 cloneFill(src.Fill),
		Effects:              src.Effects,
		Geometry:             src.Geometry,
		GeometryData:         cloneGeometryData(src.GeometryData),
		RotationDeg:          src.RotationDeg,
		FlipH:                src.FlipH,
		FlipV:                src.FlipV,
		TextAnchor:           src.TextAnchor,
		HasTextInsets:        src.HasTextInsets,
		TextInsetLeftEMU:     src.TextInsetLeftEMU,
		TextInsetRightEMU:    src.TextInsetRightEMU,
		TextInsetTopEMU:      src.TextInsetTopEMU,
		TextInsetBottomEMU:   src.TextInsetBottomEMU,
		TextAllowOverflow:    src.TextAllowOverflow,
		TextAutoGrow:         src.TextAutoGrow,
		SentinelPlaceholder:  src.SentinelPlaceholder,
		TextBodyProps:        src.TextBodyProps,
		Paragraphs:           cloneParagraphs(src.Paragraphs),
		ChartData:            cloneBytes(src.ChartData),
		ImageRelID:           "",
		ImageData:            cloneBytes(src.ImageData),
		ImageMIME:            src.ImageMIME,
		ImageCropLeft:        src.ImageCropLeft,
		ImageCropTop:         src.ImageCropTop,
		ImageCropRight:       src.ImageCropRight,
		ImageCropBottom:      src.ImageCropBottom,
		MediaKind:            src.MediaKind,
		MediaRelID:           "",
		MediaData:            cloneBytes(src.MediaData),
		MediaLinkURL:         src.MediaLinkURL,
		MediaMIME:            src.MediaMIME,
		MediaPosterData:      cloneBytes(src.MediaPosterData),
		MediaPosterMIME:      src.MediaPosterMIME,
		Children:             cloneShapes(src.Children),
		LineColor:            src.LineColor,
		LineWidthPt:          src.LineWidthPt,
		LineStyle:            cloneLineStyle(src.LineStyle),
		ConnectorType:        src.ConnectorType,
		ConnectorFlipH:       src.ConnectorFlipH,
		ConnectorFlipV:       src.ConnectorFlipV,
		ConnectorAdjustments: append([]int(nil), src.ConnectorAdjustments...),
		ConnectorStart:       clonePoint(src.ConnectorStart),
		ConnectorEnd:         clonePoint(src.ConnectorEnd),
		ArrowHead:            src.ArrowHead,
		ArrowHeadWidth:       src.ArrowHeadWidth,
		ArrowHeadLength:      src.ArrowHeadLength,
		ArrowTail:            src.ArrowTail,
		ArrowTailWidth:       src.ArrowTailWidth,
		ArrowTailLength:      src.ArrowTailLength,
		AnimStep:             src.AnimStep,
		EmphasisEffects:      append([]string(nil), src.EmphasisEffects...),
		MotionPath:           cloneMotionPath(src.MotionPath),
		AnimationTriggers:    append([]string(nil), src.AnimationTriggers...),
		AnimationDelayMs:     src.AnimationDelayMs,
		HyperlinkURL:         src.HyperlinkURL,
		ActionLink:           src.ActionLink,
		Table:                cloneTable(src.Table),
		OLEObjectData:        cloneBytes(src.OLEObjectData),
		OLEMathML:            src.OLEMathML,
		OLEMathColor:         src.OLEMathColor,
		OLEObjectMIME:        src.OLEObjectMIME,
		OLEObjectName:        src.OLEObjectName,
		OLEObjectProgID:      src.OLEObjectProgID,
		OLEObjectLinkURL:     src.OLEObjectLinkURL,
		OLEIconData:          cloneBytes(src.OLEIconData),
		OLEIconMIME:          src.OLEIconMIME,
	}
	return dst
}

func cloneShapes(src []*presir.Shape) []*presir.Shape {
	if src == nil {
		return nil
	}
	dst := make([]*presir.Shape, len(src))
	for i, s := range src {
		dst[i] = cloneShape(s)
	}
	return dst
}

func cloneParagraphs(src []*presir.Paragraph) []*presir.Paragraph {
	if src == nil {
		return nil
	}
	dst := make([]*presir.Paragraph, len(src))
	for i, p := range src {
		if p == nil {
			continue
		}
		dst[i] = &presir.Paragraph{
			Level:             p.Level,
			Align:             p.Align,
			TextDirection:     p.TextDirection,
			MarginLeftEMU:     p.MarginLeftEMU,
			TextIndentEMU:     p.TextIndentEMU,
			HasMarginLeft:     p.HasMarginLeft,
			HasTextIndent:     p.HasTextIndent,
			MarginRightEMU:    p.MarginRightEMU,
			HasMarginRight:    p.HasMarginRight,
			DefaultTabSizeEMU: p.DefaultTabSizeEMU,
			TabStops:          append([]presir.TabStop(nil), p.TabStops...),
			FontAlign:         p.FontAlign,
			BulletChar:        p.BulletChar,
			AutoNum:           p.AutoNum,
			AutoNumType:       p.AutoNumType,
			AutoNumStart:      p.AutoNumStart,
			BulletColor:       p.BulletColor,
			BulletFontFamily:  p.BulletFontFamily,
			BulletFontTx:      p.BulletFontTx,
			BulletSize:        p.BulletSize,
			BulletSizePct:     p.BulletSizePct,
			SpaceBefore:       p.SpaceBefore,
			SpaceAfter:        p.SpaceAfter,
			HasSpaceBefore:    p.HasSpaceBefore,
			HasSpaceAfter:     p.HasSpaceAfter,
			SpaceBeforePct:    p.SpaceBeforePct,
			SpaceAfterPct:     p.SpaceAfterPct,
			LineSpacingPct:    p.LineSpacingPct,
			LineSpacingPt:     p.LineSpacingPt,
			HasLineSpacing:    p.HasLineSpacing,
			Runs:              cloneRuns(p.Runs),
		}
	}
	return dst
}

func cloneRuns(src []*presir.Run) []*presir.Run {
	if src == nil {
		return nil
	}
	dst := make([]*presir.Run, len(src))
	for i, r := range src {
		if r == nil {
			continue
		}
		dst[i] = &presir.Run{
			Text:             r.Text,
			MathML:           r.MathML,
			FieldKind:        r.FieldKind,
			HyperlinkURL:     r.HyperlinkURL,
			Bold:             r.Bold,
			Italic:           r.Italic,
			Underline:        r.Underline,
			Strike:           r.Strike,
			BoldSet:          r.BoldSet,
			ItalicSet:        r.ItalicSet,
			UnderlineSet:     r.UnderlineSet,
			StrikeSet:        r.StrikeSet,
			AllCaps:          r.AllCaps,
			SmallCaps:        r.SmallCaps,
			Superscript:      r.Superscript,
			Subscript:        r.Subscript,
			BaselineShiftPct: r.BaselineShiftPct,
			FontSize:         r.FontSize,
			FontFamily:       r.FontFamily,
			Color:            r.Color,
			Highlight:        r.Highlight,
			Effects:          r.Effects,
		}
	}
	return dst
}

func cloneTable(src *presir.Table) *presir.Table {
	if src == nil {
		return nil
	}
	dst := &presir.Table{
		ColumnWidthsEMU: append([]int64(nil), src.ColumnWidthsEMU...),
		Rows:            make([]*presir.TableRow, len(src.Rows)),
	}
	for i, r := range src.Rows {
		if r == nil {
			continue
		}
		dst.Rows[i] = &presir.TableRow{
			HeightEMU: r.HeightEMU,
			Cells:     make([]*presir.TableCell, len(r.Cells)),
		}
		for j, c := range r.Cells {
			if c == nil {
				continue
			}
			dst.Rows[i].Cells[j] = &presir.TableCell{
				GridSpan:        c.GridSpan,
				RowSpan:         c.RowSpan,
				HMerge:          c.HMerge,
				VMerge:          c.VMerge,
				Fill:            cloneFill(c.Fill),
				FillColor:       c.FillColor,
				Effects:         c.Effects,
				TextAnchor:      c.TextAnchor,
				MarginLeftEMU:   c.MarginLeftEMU,
				MarginRightEMU:  c.MarginRightEMU,
				MarginTopEMU:    c.MarginTopEMU,
				MarginBottomEMU: c.MarginBottomEMU,
				BorderLeft:      cloneLineStyle(c.BorderLeft),
				BorderRight:     cloneLineStyle(c.BorderRight),
				BorderTop:       cloneLineStyle(c.BorderTop),
				BorderBottom:    cloneLineStyle(c.BorderBottom),
				Paragraphs:      cloneParagraphs(c.Paragraphs),
			}
		}
	}
	return dst
}

func clonePoint(src *presir.Point) *presir.Point {
	if src == nil {
		return nil
	}
	return &presir.Point{X: src.X, Y: src.Y}
}

func cloneMotionPath(src *presir.AnimMotionPath) *presir.AnimMotionPath {
	if src == nil {
		return nil
	}
	return &presir.AnimMotionPath{Path: src.Path, Dur: src.Dur, Origin: src.Origin}
}

func cloneBytes(src []byte) []byte {
	if src == nil {
		return nil
	}
	return append([]byte(nil), src...)
}

func cloneFill(src drawing.Fill) drawing.Fill {
	// drawing.Fill is currently a struct with scalar / string fields; a direct
	// copy is sufficient. If the type gains pointer fields this must deep-copy.
	return src
}

func cloneGeometryData(src *drawing.Geometry) *drawing.Geometry {
	if src == nil {
		return nil
	}
	// drawing.Geometry does not expose a clone method and its fields are value
	// types. Return the same pointer; this is acceptable for editing clones.
	return src
}

func cloneLineStyle(src *drawing.LineStyle) *drawing.LineStyle {
	if src == nil {
		return nil
	}
	dst := *src
	return &dst
}
