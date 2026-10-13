package layout

import (
	"strings"

	ir "office/pkg/presentation/ir"
)

// TextGeometryVisualPlan is the physical WordArt/OfficeArt geometry-text
// contract. Geometry text is not an ordinary bodyPr concern: it can alter the
// glyph orientation, spacing, fitting and decoration independently of the
// paragraph projection used for semantic text extraction.
type TextGeometryVisualPlan struct {
	Enabled       bool
	Text          string
	FontFamily    string
	FontSizePt    float64
	LetterSpaceEm float64
	Align         string
	Warp          string
	// PathData is the normalized physical baseline used to paint WordArt as
	// vector text-on-path. The renderer serializes it and does not interpret
	// OfficeArt preset names. Coordinates use a stable 0..100 local viewport.
	PathData    string
	StartOffset string
	TextAnchor  string
	Vector      bool
	// FitToShape marks legacy WordArt, whose glyphs PowerPoint stretches to
	// fill the shape's bounding rectangle: MS-ODRAW's geometry text is laid out
	// against the container, not at its authored size. 41246-2 slide 16's "Fin"
	// is authored at 54pt in a 134x106pt box and renders with its capitals
	// spanning the full box height.
	FitToShape bool
	Vertical    bool
	ReverseRows bool
	Kerning     bool
	Tight       bool
	Stretch     bool
	ShrinkFit   bool
	BestFit     bool
	Normalize   bool
	DxMeasure   bool
	Bold        bool
	Italic      bool
	Underline   bool
	Shadow      bool
	SmallCaps   bool
	Strike      bool
}

func ResolveTextGeometryVisualPlan(shape *ir.Shape) TextGeometryVisualPlan {
	if shape == nil || shape.OfficeArt == nil || shape.OfficeArt.GeometryText == nil {
		return TextGeometryVisualPlan{}
	}
	g := shape.OfficeArt.GeometryText
	if strings.TrimSpace(g.Text) == "" {
		return TextGeometryVisualPlan{}
	}
	plan := TextGeometryVisualPlan{
		Enabled: true, FitToShape: true, Text: g.Text, FontFamily: g.FontFamily, FontSizePt: g.FontSizePt,
		LetterSpaceEm: g.Spacing, Warp: g.Warp, Vertical: g.Vertical, ReverseRows: g.ReverseRows,
		Kerning: g.Kerning, Tight: g.Tight, Stretch: g.Stretch, ShrinkFit: g.ShrinkFit,
		BestFit: g.BestFit, Normalize: g.Normalize, DxMeasure: g.DxMeasure,
		Bold: g.Bold, Italic: g.Italic, Underline: g.Underline, Shadow: g.Shadow,
		SmallCaps: g.SmallCaps, Strike: g.Strikethrough,
	}
	switch g.Align {
	case 1:
		plan.Align = "center"
	case 2:
		plan.Align = "right"
	case 3:
		plan.Align = "justify"
	default:
		plan.Align = "left"
	}
	plan.PathData = resolveWordArtPathData(plan.Warp, plan.Vertical)
	plan.StartOffset, plan.TextAnchor = resolveWordArtAlignment(g.Align)
	plan.Vector = plan.PathData != ""

	// OfficeArt spacing is a 16.16 em-like multiplier. Zero is the authored
	// default and should not produce CSS letter spacing.
	if plan.LetterSpaceEm > 8 || plan.LetterSpaceEm < -8 {
		plan.LetterSpaceEm = 0
	}
	return plan
}

func resolveWordArtAlignment(align uint32) (startOffset, textAnchor string) {
	switch align {
	case 2:
		return "5%", "start"
	case 3:
		return "95%", "end"
	default:
		return "50%", "middle"
	}
}

// resolveWordArtPathData normalizes legacy OfficeArt text-effect presets into
// physical local geometry. Keeping this mapping in layout is important: HTML
// must not know that textArchUpCurve and textArchUpPour are OfficeArt presets.
func resolveWordArtPathData(warp string, vertical bool) string {
	if vertical {
		return "M 50 5 L 50 95"
	}
	switch strings.ToLower(strings.TrimSpace(warp)) {
	case "textarchupcurve", "textarchuppour", "archup", "textcurveup", "textoncurve":
		return "M 5 78 Q 50 5 95 78"
	case "textarchdowncurve", "textarchdownpour", "archdown", "textcurvedown":
		return "M 5 22 Q 50 95 95 22"
	case "textcirclecurve", "textcirclepour", "circle", "textring", "textonring", "textringinside", "textringoutside":
		return "M 5 55 A 45 45 0 0 1 95 55"
	case "textbuttoncurve", "textbuttonpour", "textcanup":
		return "M 5 62 Q 50 18 95 62"
	case "textcandown":
		return "M 5 38 Q 50 82 95 38"
	case "textwave", "textwave1", "textwave2", "textwave3", "textwave4", "wave":
		return "M 3 52 C 18 18 32 86 50 52 C 68 18 82 86 97 52"
	case "textcascadeup", "textslantup", "textfadeup":
		return "M 5 22 L 95 78"
	case "textcascadedown", "textslantdown", "textfadedown":
		return "M 5 78 L 95 22"
	case "texttriangle", "textchevron":
		return "M 5 75 L 50 20 L 95 75"
	case "texttriangleinverted", "textchevroninverted":
		return "M 5 25 L 50 80 L 95 25"
	default:
		// Unknown WordArt still gets a physical baseline. The preset identity is
		// retained in Warp for diagnostics/future exact geometry support.
		return "M 5 50 L 95 50"
	}
}
