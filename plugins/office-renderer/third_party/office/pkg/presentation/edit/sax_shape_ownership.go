package edit

import (
	"fmt"

	pasir "office/pkg/presentation/ir"
)

// transferSAXShapeOwnership replaces composer-owned core visual nodes with the
// authored SAX nodes while retaining only explicit feature enrichments. Feature payloads that have
// not moved to SAX yet (image bytes, DrawingML rich paint/effects, animation,
// actions, media/OLE/table/chart data, etc.) remain supplied by ComposePackage.
//
// Composer ordering remains the merge skeleton while feature promotion is hybrid,
// but matched core nodes and nested group structure are now SAX-owned objects.
func transferSAXShapeOwnership(saxSlides []*pasir.Slide, resolved *pasir.Presentation) error {
	if resolved == nil || len(saxSlides) != len(resolved.Slides) {
		return fmt.Errorf("cannot transfer SAX shape ownership across different slide sets")
	}
	for i, saxSlide := range saxSlides {
		if saxSlide == nil || resolved.Slides[i] == nil {
			return fmt.Errorf("slide %d is nil during SAX ownership transfer", i)
		}
		authored := make(map[int]*pasir.Shape)
		collectSAXOwnedShapes(saxSlide.Shapes, authored)
		matched := make(map[int]bool)
		resolved.Slides[i].Shapes = takeOverSAXShapeTree(resolved.Slides[i].Shapes, authored, matched)
		// A SAX shape may legitimately be absent from the final authored-shape
		// family because the mature feature resolver promotes it to media/OLE.
		// All ordinary visual nodes that can be matched are now the SAX nodes
		// themselves rather than composer nodes with SAX fields overlaid.
	}
	return nil
}

func saxOwnsShape(kind pasir.ShapeKind) bool {
	switch kind {
	case pasir.ShapeText, pasir.ShapeImage, pasir.ShapeGroup, pasir.ShapeConnector,
		pasir.ShapeTable, pasir.ShapeChart, pasir.ShapeSmartArt, pasir.ShapeMedia, pasir.ShapeOleObject:
		return true
	default:
		return false
	}
}

func collectSAXOwnedShapes(shapes []*pasir.Shape, out map[int]*pasir.Shape) {
	for _, shape := range shapes {
		if shape == nil {
			continue
		}
		if saxOwnsShape(shape.Kind) && shape.ID != 0 {
			out[shape.ID] = shape
		}
		collectSAXOwnedShapes(shape.Children, out)
	}
}

func takeOverSAXShapeTree(composed []*pasir.Shape, authored map[int]*pasir.Shape, matched map[int]bool) []*pasir.Shape {
	out := make([]*pasir.Shape, 0, len(composed))
	for _, old := range composed {
		if old == nil {
			out = append(out, nil)
			continue
		}
		if src := authored[old.ID]; src != nil && saxOwnsShape(old.Kind) && compatibleSAXShapeKind(src.Kind, old.Kind) {
			// The SAX node owns authored structure and semantics. Composer-only
			// enrichments are copied onto it explicitly; the old node is discarded.
			if src.Kind == pasir.ShapeGroup {
				src.Children = takeOverSAXShapeTree(old.Children, authored, matched)
			}
			enrichSAXShapeFromComposer(src, old)
			matched[src.ID] = true
			out = append(out, src)
			continue
		}
		old.Children = takeOverSAXShapeTree(old.Children, authored, matched)
		out = append(out, old)
	}
	return out
}

// enrichSAXShapeFromComposer is the temporary compatibility membrane around
// the remaining compatibility island. Authored core geometry, transforms, paint,
// text, hierarchy, actions, and placeholder semantics must never flow backward
// from the composer.
func enrichSAXShapeFromComposer(dst, old *pasir.Shape) {
	dst.OfficeArt = old.OfficeArt
	dst.OriginalShapeID = old.OriginalShapeID
	dst.GeometryAdjustments = append([]float64(nil), old.GeometryAdjustments...)
	if len(dst.ImageData) == 0 {
		dst.ImageData = append([]byte(nil), old.ImageData...)
		dst.ImageMIME = old.ImageMIME
	}
	dst.ImageDPI = old.ImageDPI
	dst.ImageRotWithShape = old.ImageRotWithShape
	dst.ImageRotWithShapeSet = old.ImageRotWithShapeSet
	if dst.Kind == pasir.ShapeConnector && dst.ConnectorType == "" {
		dst.ConnectorType = dst.Geometry
	}
	dst.ConnectorFlipH, dst.ConnectorFlipV = dst.FlipH, dst.FlipV
	dst.ConnectorAdjustments = append([]int(nil), old.ConnectorAdjustments...)
	dst.ConnectorStart, dst.ConnectorEnd = old.ConnectorStart, old.ConnectorEnd
	dst.ConnectorPath = append([]pasir.Point(nil), old.ConnectorPath...)
	dst.ArrowHead, dst.ArrowHeadWidth, dst.ArrowHeadLength = old.ArrowHead, old.ArrowHeadWidth, old.ArrowHeadLength
	dst.ArrowTail, dst.ArrowTailWidth, dst.ArrowTailLength = old.ArrowTail, old.ArrowTailWidth, old.ArrowTailLength
	dst.AnimStep = old.AnimStep
	dst.EmphasisEffects = append([]string(nil), old.EmphasisEffects...)
	dst.MotionPath = old.MotionPath
	dst.AnimationTriggers = append([]string(nil), old.AnimationTriggers...)
	dst.AnimationDelayMs = old.AnimationDelayMs
	// Complex-object payloads are package-level enrichments. SAX owns the frame,
	// source order and authored table structure; the composer temporarily supplies
	// binary/secondary-part payloads until those part decoders move behind SAX.
	switch dst.Kind {
	case pasir.ShapeChart:
		if len(dst.ChartData) == 0 {
			dst.ChartData = append([]byte(nil), old.ChartData...)
		}
	case pasir.ShapeTable:
		enrichSAXTable(dst.Table, old.Table)
	case pasir.ShapeMedia:
		if len(dst.MediaData) == 0 {
			dst.MediaData = append([]byte(nil), old.MediaData...)
		}
		dst.MediaLinkURL = old.MediaLinkURL
		if dst.MediaMIME == "" {
			dst.MediaMIME = old.MediaMIME
		}
		dst.MediaPosterData = append([]byte(nil), old.MediaPosterData...)
		dst.MediaPosterMIME = old.MediaPosterMIME
		if dst.MediaRelID == "" {
			dst.MediaRelID = old.MediaRelID
		}
	case pasir.ShapeOleObject:
		// BIG18: OLE payload, metadata, linked target and fallback icon are all
		// authored by the SAX/package relationship stream. Nothing flows back
		// from the compatibility resolver for this shape family.
	case pasir.ShapeSmartArt:
		// SmartArt expansion is projected directly from package diagram parts.
		// The compatibility resolver must not enrich SAX SmartArt shapes.
	}
	if dst.Kind == pasir.ShapeText {
		dst.Paragraphs = cloneSAXParagraphsWithEnrichment(dst.Paragraphs, old.Paragraphs)
	}
}

func compatibleSAXShapeKind(src, dst pasir.ShapeKind) bool {
	return src == dst
}

func cloneSAXParagraphsWithEnrichment(src, old []*pasir.Paragraph) []*pasir.Paragraph {
	out := make([]*pasir.Paragraph, 0, len(src))
	for pi, p := range src {
		if p == nil {
			out = append(out, nil)
			continue
		}
		cp := *p
		cp.TabStops = append([]pasir.TabStop(nil), p.TabStops...)
		cp.Runs = make([]*pasir.Run, 0, len(p.Runs))
		for ri, r := range p.Runs {
			if r == nil {
				cp.Runs = append(cp.Runs, nil)
				continue
			}
			cr := *r
			if pi < len(old) && old[pi] != nil && ri < len(old[pi].Runs) && old[pi].Runs[ri] != nil {
				or := old[pi].Runs[ri]
				cr.MathML = or.MathML
				cr.Highlight = or.Highlight
				cr.Effects = or.Effects
			}
			cp.Runs = append(cp.Runs, &cr)
		}
		out = append(out, &cp)
	}
	return out
}

func enrichSAXTable(dst, old *pasir.Table) {
	if dst == nil || old == nil {
		return
	}
	for ri := range dst.Rows {
		if ri >= len(old.Rows) || dst.Rows[ri] == nil || old.Rows[ri] == nil {
			continue
		}
		for ci := range dst.Rows[ri].Cells {
			if ci >= len(old.Rows[ri].Cells) || dst.Rows[ri].Cells[ci] == nil || old.Rows[ri].Cells[ci] == nil {
				continue
			}
			d, o := dst.Rows[ri].Cells[ci], old.Rows[ri].Cells[ci]
			d.RowSpan, d.HMerge, d.VMerge = o.RowSpan, o.HMerge, o.VMerge
			// Direct cell paint, borders, margins and anchoring are SAX-owned.
			// Composer remains only for unresolved table-style inheritance/effects.
			d.Effects = o.Effects
			// Text is SAX-owned; retain only run enrichments not decoded by SAX yet.
			d.Paragraphs = cloneSAXParagraphsWithEnrichment(d.Paragraphs, o.Paragraphs)
		}
	}
}
