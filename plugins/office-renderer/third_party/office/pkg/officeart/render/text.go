package render

import (
	"fmt"

	"office/pkg/officeart/ir"
)

func TextStyle(body *ir.TextBody) map[string]string {
	if body == nil {
		return nil
	}
	style := map[string]string{
		"box-sizing":     "border-box",
		"position":       "absolute",
		"inset":          "0",
		"padding-left":   fmt.Sprintf("%.4fpt", pt(body.Insets.Left)),
		"padding-top":    fmt.Sprintf("%.4fpt", pt(body.Insets.Top)),
		"padding-right":  fmt.Sprintf("%.4fpt", pt(body.Insets.Right)),
		"padding-bottom": fmt.Sprintf("%.4fpt", pt(body.Insets.Bottom)),
	}

	switch body.Wrap {
	case ir.TextWrapNone:
		style["white-space"] = "nowrap"
		style["overflow"] = "visible"
	default:
		style["white-space"] = "normal"
		style["overflow"] = "hidden"
	}

	switch body.Vert {
	case ir.TextVerticalRL, ir.TextWordArtVertical:
		style["writing-mode"] = "vertical-rl"
		style["text-orientation"] = "mixed"
	case ir.TextVertical270:
		style["writing-mode"] = "vertical-lr"
		style["text-orientation"] = "mixed"
	}

	style["display"] = "flex"
	style["flex-direction"] = "column"
	switch body.Anchor {
	case ir.TextAnchorCenter:
		style["justify-content"] = "center"
	case ir.TextAnchorBottom:
		style["justify-content"] = "flex-end"
	default:
		style["justify-content"] = "flex-start"
	}
	if body.AnchorCenter {
		style["text-align"] = "center"
	}
	return style
}
