package drawingcss

import (
	"fmt"
	"strings"

	"office/pkg/ooxml/drawing"
)

// TextBodyProps returns CSS properties that approximate text body layout.
func TextBodyProps(bp drawing.TextBodyProperties) map[string]string {
	props := make(map[string]string)

	// Vertical alignment maps to flexbox align-items / justify-content.
	switch bp.Anchor {
	case "t":
		props["justify-content"] = "flex-start"
	case "ctr":
		props["justify-content"] = "center"
	case "b":
		props["justify-content"] = "flex-end"
	}

	// Horizontal center anchor.
	if bp.AnchorCtr {
		props["text-align"] = "center"
	}

	// Text wrapping.
	if bp.Wrap == "none" {
		props["white-space"] = "nowrap"
	}

	// Build transform list for rotation + vertical text.
	var transforms []string
	if bp.Rot != 0 {
		deg := float64(bp.Rot) / 60000.0
		transforms = append(transforms, fmt.Sprintf("rotate(%.2fdeg)", deg))
	}
	switch bp.Vert {
	case "vert":
		props["writing-mode"] = "vertical-rl"
		props["text-orientation"] = "mixed"
	case "vert270":
		props["writing-mode"] = "vertical-lr"
		props["text-orientation"] = "mixed"
		transforms = append(transforms, "rotate(180deg)")
	}
	if len(transforms) > 0 {
		props["transform"] = strings.Join(transforms, " ")
	}

	// Padding from insets.
	insetT := bp.InsetT
	if insetT == 0 {
		insetT = 45720 // OOXML default
	}
	insetB := bp.InsetB
	if insetB == 0 {
		insetB = 45720
	}
	insetL := bp.InsetL
	if insetL == 0 {
		insetL = 91440
	}
	insetR := bp.InsetR
	if insetR == 0 {
		insetR = 91440
	}
	props["padding"] = fmt.Sprintf("%.2fpt %.2fpt %.2fpt %.2fpt",
		drawing.EMUToPoints(insetT),
		drawing.EMUToPoints(insetR),
		drawing.EMUToPoints(insetB),
		drawing.EMUToPoints(insetL),
	)

	// Multi-column text
	if bp.Columns > 1 {
		props["column-count"] = fmt.Sprintf("%d", bp.Columns)
		if bp.ColumnSpacingEMU > 0 {
			props["column-gap"] = fmt.Sprintf("%.2fpt", drawing.EMUToPoints(bp.ColumnSpacingEMU))
		}
	}

	return props
}

// ParagraphProperties returns CSS properties for a paragraph.
func ParagraphProperties(pp drawing.ParagraphProperties) map[string]string {
	props := make(map[string]string)

	switch pp.Align {
	case "l":
		props["text-align"] = "left"
	case "r":
		props["text-align"] = "right"
	case "ctr":
		props["text-align"] = "center"
	case "just":
		props["text-align"] = "justify"
	}

	if pp.MarginLeftEMU > 0 {
		props["margin-left"] = fmt.Sprintf("%.2fpt", drawing.EMUToPoints(pp.MarginLeftEMU))
	}
	if pp.IndentEMU != 0 {
		props["text-indent"] = fmt.Sprintf("%.2fpt", drawing.EMUToPoints(pp.IndentEMU))
	}
	if pp.SpaceBefore > 0 {
		props["margin-top"] = fmt.Sprintf("%.2fpt", float64(pp.SpaceBefore)/100)
	}
	if pp.SpaceAfter > 0 {
		props["margin-bottom"] = fmt.Sprintf("%.2fpt", float64(pp.SpaceAfter)/100)
	}
	if pp.LineSpacing > 0 {
		props["line-height"] = fmt.Sprintf("%.2fpt", float64(pp.LineSpacing)/100)
	}

	return props
}

// RunProperties returns CSS properties for a text run.
func RunProperties(rp drawing.RunProperties) map[string]string {
	props := make(map[string]string)

	if rp.Bold {
		props["font-weight"] = "bold"
	}
	if rp.Italic {
		props["font-style"] = "italic"
	}
	var decorations []string
	if rp.Underline {
		decorations = append(decorations, "underline")
	}
	if rp.Strike {
		decorations = append(decorations, "line-through")
	}
	if len(decorations) > 0 {
		td := ""
		for i, d := range decorations {
			if i > 0 {
				td += " "
			}
			td += d
		}
		props["text-decoration"] = td
	}
	if rp.FontSizePt > 0 {
		props["font-size"] = fmt.Sprintf("%.2fpt", rp.FontSizePt)
	}
	if rp.FontFamily != "" {
		props["font-family"] = rp.FontFamily
	}
	if rp.Color != "" {
		props["color"] = "#" + rp.Color
	}

	return props
}

// TextRunSpanHTML returns an HTML <span> element for the run, or just the escaped text
// if no formatting is applied.
func TextRunSpanHTML(r drawing.TextRun) string {
	css := RunProperties(r.Props)
	text := htmlEscape(r.Text)
	if len(css) == 0 {
		return text
	}
	style := cssMapToString(css)
	return fmt.Sprintf("<span style=%q>%s</span>", style, text)
}

// TextBodyHTML returns a minimal HTML fragment for the full text body.
// The caller is responsible for wrapping it in a correctly sized container.
func TextBodyHTML(tb drawing.TextBody) string {
	if len(tb.Paragraphs) == 0 {
		return ""
	}
	var out string
	for _, p := range tb.Paragraphs {
		pCSS := ParagraphProperties(p.Props)
		pStyle := cssMapToString(pCSS)
		tag := "p"
		if pStyle != "" {
			out += fmt.Sprintf("<%s style=%q>", tag, pStyle)
		} else {
			out += "<" + tag + ">"
		}
		for _, r := range p.Runs {
			out += TextRunSpanHTML(r)
		}
		out += "</" + tag + ">"
	}
	return out
}
