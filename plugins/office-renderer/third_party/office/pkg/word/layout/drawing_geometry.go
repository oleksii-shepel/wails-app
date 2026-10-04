package layout

import (
	"strings"

	wordir "office/pkg/word/ir"
)

func drawingFromRun(run *wordir.Run) wordir.Drawing {
	if run == nil {
		return nil
	}
	if run.Image != nil {
		return run.Image
	}
	if run.TextBox != nil {
		return run.TextBox
	}
	if run.Shape != nil {
		return run.Shape
	}
	return nil
}

func drawingRunPlacement(run *wordir.Run) (wordir.DrawingPlacement, bool) {
	drawing := drawingFromRun(run)
	if drawing == nil {
		return wordir.DrawingPlacement{}, false
	}
	return drawing.DrawingPlacement(), true
}

func drawingRunIsFloating(run *wordir.Run) bool {
	placement, ok := drawingRunPlacement(run)
	return ok && placement.Floating
}
func drawingRunIsTopAndBottom(run *wordir.Run) bool {
	placement, ok := drawingRunPlacement(run)
	return ok && placement.Floating && strings.EqualFold(placement.WrapType, "topAndBottom")
}

func paragraphContainsOnlyDrawingForLayout(p *wordir.Paragraph, target wordir.Drawing) bool {
	if p == nil || target == nil {
		return false
	}
	found := false
	for i := range p.Runs {
		run := &p.Runs[i]
		drawing := drawingFromRun(run)
		switch {
		case drawing != nil && sameDrawing(drawing, target):
			found = true
		case drawing != nil:
			return false
		case strings.TrimSpace(run.Text) != "":
			return false
		case run.Kind == "hyperlink":
			return false
		}
	}
	return found
}

func paragraphBlockDrawingEligibleForLayout(p *wordir.Paragraph, target wordir.Drawing) bool {
	if paragraphContainsOnlyDrawingForLayout(p, target) {
		return true
	}
	if p == nil || target == nil {
		return false
	}
	targetIndex := -1
	for i := range p.Runs {
		if sameDrawing(drawingFromRun(&p.Runs[i]), target) {
			targetIndex = i
			break
		}
	}
	if targetIndex < 0 {
		return false
	}
	for i := 0; i < targetIndex; i++ {
		run := &p.Runs[i]
		if strings.TrimSpace(run.Text) != "" || run.Kind == "hyperlink" {
			return false
		}
		if drawing := drawingFromRun(run); drawing != nil && !drawingRunIsTopAndBottom(run) {
			return false
		}
	}
	return true
}

func paragraphContainsOnlyLeadingBlockDrawingForLayout(p *wordir.Paragraph) bool {
	if p == nil {
		return false
	}
	for i := range p.Runs {
		run := &p.Runs[i]
		if drawing := drawingFromRun(run); drawing != nil {
			return drawingRunIsTopAndBottom(run) && paragraphContainsOnlyDrawingForLayout(p, drawing)
		}
		if strings.TrimSpace(run.Text) != "" || run.Kind == "hyperlink" {
			return false
		}
	}
	return false
}

func paragraphHasFloatingDrawing(p *wordir.Paragraph) bool {
	if p == nil {
		return false
	}
	for i := range p.Runs {
		if drawingRunIsFloating(&p.Runs[i]) {
			return true
		}
	}
	return false
}

func sameDrawing(a, b wordir.Drawing) bool {
	if a == nil || b == nil {
		return false
	}
	switch av := a.(type) {
	case *wordir.Image:
		bv, ok := b.(*wordir.Image)
		return ok && av == bv
	case *wordir.TextBox:
		bv, ok := b.(*wordir.TextBox)
		return ok && av == bv
	case *wordir.Shape:
		bv, ok := b.(*wordir.Shape)
		return ok && av == bv
	default:
		return false
	}
}

func drawingIsLeadingBlockFloat(p *wordir.Paragraph, fl FloatBox) bool {
	drawing := fl.Drawing()
	return drawing != nil && drawingIsTopAndBottom(fl) && paragraphBlockDrawingEligibleForLayout(p, drawing)
}

func resolveDrawingFloat(drawing wordir.Drawing, geom PageGeometry, column int, paraPageY, paraHeight float64) (FloatBox, bool) {
	if drawing == nil {
		return FloatBox{}, false
	}
	placement := drawing.DrawingPlacement()
	if !placement.Floating {
		return FloatBox{}, false
	}
	widthPt := emuToPoints(placement.Width)
	heightPt := emuToPoints(placement.Height)
	if placement.SizeRelHPct > 0 {
		widthPt = relativeObjectSizePt(placement.SizeRelHPct, placement.SizeRelHFrom, true, geom, column)
	}
	if placement.SizeRelVPct > 0 {
		heightPt = relativeObjectSizePt(placement.SizeRelVPct, placement.SizeRelVFrom, false, geom, column)
	}
	return FloatBox{X: resolveX(placement.X, placement.XRelative, placement.XAlign, geom, column, widthPt), Y: resolveY(placement.Y, placement.YRelative, placement.YAlign, geom, paraPageY, paraHeight, heightPt), Width: widthPt, Height: heightPt, ZIndex: placement.ZIndex, BehindDoc: placement.BehindDoc}, true
}

func attachDrawingPayload(fl FloatBox, drawing wordir.Drawing) FloatBox {
	switch d := drawing.(type) {
	case *wordir.Image:
		fl.Image = d
	case *wordir.TextBox:
		fl.TextBox = d
	case *wordir.Shape:
		fl.Shape = d
	}
	return fl
}

func resolvedDrawingFloatFromRun(run *wordir.Run, geom PageGeometry, column int, paraPageY, paraHeight float64) (FloatBox, bool) {
	drawing := drawingFromRun(run)
	fl, ok := resolveDrawingFloat(drawing, geom, column, paraPageY, paraHeight)
	if !ok {
		return FloatBox{}, false
	}
	return attachDrawingPayload(fl, drawing), true
}

func paragraphLeadingBlockDrawingHeightPt(p *wordir.Paragraph, geom PageGeometry, column int, paraPageY, paraHeight float64) float64 {
	if p == nil {
		return 0
	}
	height := 0.0
	for i := range p.Runs {
		run := &p.Runs[i]
		drawing := drawingFromRun(run)
		if drawing != nil {
			fl, ok := resolvedDrawingFloatFromRun(run, geom, column, paraPageY, paraHeight)
			if ok && drawingIsLeadingBlockFloat(p, fl) {
				height += drawingOuterHeightPt(fl)
				continue
			}
		}
		if strings.TrimSpace(run.Text) != "" {
			break
		}
	}
	return height
}

func drawingExclusionForFloat(fl FloatBox) (lineExclusion, bool) {
	placement, ok := fl.DrawingPlacement()
	if !ok || fl.BehindDoc || placement.BehindDoc || !placement.HasWrapExclusion() {
		return lineExclusion{}, false
	}
	wrapType := placement.WrapType
	if strings.TrimSpace(wrapType) == "" {
		wrapType = "square"
	}
	return drawingLineExclusion(fl, wrapType, placement.WrapSide, placement.WrapPolygon, placement.RotationDeg, emuToPoints(placement.Width), emuToPoints(placement.Height), placement.DistTop, placement.DistRight, placement.DistBottom, placement.DistLeft, emuToPoints(placement.EffectExtentT), emuToPoints(placement.EffectExtentR), emuToPoints(placement.EffectExtentB), emuToPoints(placement.EffectExtentL)), true
}

// pendingIndependentFloat is a floating drawing whose resolved page position
// does not depend on its anchor paragraph's flow position. Word lays floats out
// bottom-up: an arrow anchored in a later paragraph still indents the text of
// an earlier paragraph when their vertical bands overlap. Carrying such floats
// forward into wrap exclusion sets reproduces that behavior in the single-pass
// paginator.
type pendingIndependentFloat struct {
	fl    FloatBox
	order int // top-level block ordinal of the anchor paragraph
}

// drawingWrapPositionIsIndependent reports whether both the horizontal and the
// vertical anchor of a floating drawing resolve from page geometry alone, with
// no anchor-paragraph flow position required.
func drawingWrapPositionIsIndependent(placement wordir.DrawingPlacement) bool {
	switch strings.ToLower(strings.TrimSpace(placement.YRelative)) {
	case "paragraph", "line":
		return false
	}
	switch strings.ToLower(strings.TrimSpace(placement.XRelative)) {
	case "paragraph", "character", "column":
		return false
	}
	return true
}

// pendingIndependentFloats collects position-independent floating drawings from
// top-level paragraph blocks in block order. Non-wrapping, behind-text and
// flow-dependent drawings are excluded; they keep the existing anchor-local
// behavior.
func pendingIndependentFloats(blockList []wordir.Block, geom PageGeometry) []pendingIndependentFloat {
	var out []pendingIndependentFloat
	for order := range blockList {
		block := &blockList[order]
		if block.Paragraph == nil {
			continue
		}
		for i := range block.Paragraph.Runs {
			run := &block.Paragraph.Runs[i]
			drawing := drawingFromRun(run)
			if drawing == nil {
				continue
			}
			placement := drawing.DrawingPlacement()
			if !placement.Floating || placement.BehindDoc {
				continue
			}
			if strings.EqualFold(placement.WrapType, "topAndBottom") {
				continue
			}
			if !drawingWrapPositionIsIndependent(placement) {
				continue
			}
			fl, ok := resolveDrawingFloat(drawing, geom, 0, 0, 0)
			if !ok {
				continue
			}
			// The payload drives both the wrap exclusion geometry and the
			// render-time position; a payload-less box would be ignored.
			fl = attachDrawingPayload(fl, drawing)
			out = append(out, pendingIndependentFloat{fl: fl, order: order})
		}
	}
	return out
}

func drawingIsTopAndBottom(fl FloatBox) bool {
	placement, ok := fl.DrawingPlacement()
	return ok && placement.Floating && strings.EqualFold(placement.WrapType, "topAndBottom")
}

func drawingOuterHeightPt(fl FloatBox) float64 {
	placement, ok := fl.DrawingPlacement()
	if !ok {
		return 0
	}
	_, top, _, bottom := transformedDrawingBounds(fl, placement.RotationDeg)
	return (bottom - top) + placement.DistTop + placement.DistBottom + emuToPoints(placement.EffectExtentT) + emuToPoints(placement.EffectExtentB)
}

// paragraphDrawingRequiresNextPage preflights paragraph-relative anchors before
// the paragraph is committed. This deliberately reads the canonical drawing
// placement from the run rather than relying on the already-resolved FloatBox:
// textbox-heavy Word templates can have no ordinary paragraph text, but their
// anchor paragraph still owns the physical page of the drawing.
func paragraphDrawingRequiresNextPage(p *wordir.Paragraph, geom PageGeometry, paraFlowY, paraHeight float64) bool {
	if p == nil || geom.Height <= 0 {
		return false
	}
	paraPageY := paraFlowY + geom.MarginTop
	for i := range p.Runs {
		run := &p.Runs[i]
		placement, ok := drawingRunPlacement(run)
		if !ok || !placement.Floating || !strings.EqualFold(placement.YRelative, "paragraph") {
			continue
		}
		height := emuToPoints(placement.Height)
		if height <= 0 {
			if drawing := drawingFromRun(run); drawing != nil {
				height = emuToPoints(drawing.DrawingPlacement().Height)
			}
		}
		top := resolveY(placement.Y, placement.YRelative, placement.YAlign, geom, paraPageY, paraHeight, height)
		if top+height+placement.DistBottom+emuToPoints(placement.EffectExtentB) > geom.Height+0.01 {
			return true
		}
	}
	return false
}

// paragraphRelativeFloatOverflowsPage reports whether a float tied vertically
// to its anchor paragraph would paint beyond the current physical page.
// Page/margin-relative objects are intentionally excluded: those coordinates
// are already physical-page coordinates and may legitimately be clipped.
func paragraphRelativeFloatOverflowsPage(floats []FloatBox, geom PageGeometry) bool {
	if geom.Height <= 0 {
		return false
	}
	for i := range floats {
		fl := &floats[i]
		placement, ok := fl.DrawingPlacement()
		if !ok || !placement.Floating || !strings.EqualFold(placement.YRelative, "paragraph") {
			continue
		}
		if fl.Y+fl.Height > geom.Height+0.01 {
			return true
		}
	}
	return false
}
