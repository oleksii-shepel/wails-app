package layout

import wordir "office/pkg/word/ir"

// Drawing returns the canonical drawing payload carried by a resolved float.
// Non-drawing floats (tables and frame paragraphs) deliberately return nil.
func (fl FloatBox) Drawing() wordir.Drawing {
	switch {
	case fl.Image != nil:
		return fl.Image
	case fl.TextBox != nil:
		return fl.TextBox
	case fl.Shape != nil:
		return fl.Shape
	default:
		return nil
	}
}

// DrawingPlacement projects the resolved float back to the shared drawing
// geometry contract. A FloatBox is, by definition, floating; keep that fact
// authoritative even when a directly-constructed payload omitted Floating.
func (fl FloatBox) DrawingPlacement() (wordir.DrawingPlacement, bool) {
	drawing := fl.Drawing()
	if drawing == nil {
		return wordir.DrawingPlacement{}, false
	}
	placement := drawing.DrawingPlacement()
	placement.Floating = true
	placement.BehindDoc = fl.BehindDoc || placement.BehindDoc
	return placement, true
}
