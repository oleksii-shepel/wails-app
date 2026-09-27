package layout

import (
	"math"

	resolve "office/pkg/word/ir"
)

const wordScriptScale = 0.65

func runVerticalShiftPt(format resolve.RunFormat, fontSizePt float64) float64 {
	if format.Position != nil && *format.Position != 0 {
		// w:position uses half-points; positive values raise text. Physical Top
		// grows downward, so raising is a negative offset.
		return -float64(*format.Position) / 2.0
	}
	baseSizePt := fontSizePt
	if (format.VertAlignVal == "superscript" || format.VertAlignVal == "subscript") && fontSizePt > 0 {
		baseSizePt = fontSizePt / wordScriptScale
	}
	switch format.VertAlignVal {
	case "superscript":
		return -baseSizePt * 0.33
	case "subscript":
		return baseSizePt * 0.16
	default:
		return 0
	}
}

func positionedRunFinalTop(run resolve.Run, baseTop, heightPt, lineHeightPt float64) float64 {
	if heightPt <= 0 {
		heightPt = lineHeightPt
	}
	if inlineObjectRun(run) {
		// Inline objects own their vertical geometry in layout. Align the object
		// bottom to the physical line box instead of leaving CSS inline-baseline
		// rules to decide its Y position.
		return baseTop + math.Max(0, lineHeightPt-heightPt)
	}
	fontSizePt := heightPt
	if fontSizePt <= 0 {
		fontSizePt = 12
	}
	return baseTop + runVerticalShiftPt(run.Format, fontSizePt)
}

func inlineObjectRun(run resolve.Run) bool {
	switch run.Kind {
	case "image", "textbox", "shape", "olePlaceholder", "chart":
		return true
	default:
		return false
	}
}

func inlineObjectExtentPt(run resolve.Run) (widthPt, heightPt float64, ok bool) {
	switch run.Kind {
	case "image", "chart":
		if run.Image != nil && !run.Image.Floating {
			return emuToPoints(run.Image.Width), emuToPoints(run.Image.Height), true
		}
	case "textbox":
		if run.TextBox != nil && !run.TextBox.Floating {
			return emuToPoints(run.TextBox.Width), emuToPoints(run.TextBox.Height), true
		}
	case "shape":
		if run.Shape != nil && !run.Shape.Floating {
			return emuToPoints(run.Shape.Width), emuToPoints(run.Shape.Height), true
		}
	case "olePlaceholder":
		if run.OLEPreview != nil && !run.OLEPreview.Floating {
			return emuToPoints(run.OLEPreview.Width), emuToPoints(run.OLEPreview.Height), true
		}
	}
	return 0, 0, false
}

func runNeedsVerticalPositioning(run resolve.Run) bool {
	if inlineObjectRun(run) {
		return true
	}
	if run.Format.Position != nil && *run.Format.Position != 0 {
		return true
	}
	return run.Format.VertAlignVal == "superscript" || run.Format.VertAlignVal == "subscript"
}
