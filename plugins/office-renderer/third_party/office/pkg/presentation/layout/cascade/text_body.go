package cascade

import (
	"office/pkg/ooxml/drawing"
	"office/pkg/presentation/parsers/ooxml/parts"
)

const (
	DefaultTextInsetLeftRightEMU int64 = 91440
	DefaultTextInsetTopBottomEMU int64 = 45720
)

// TextBodyStyle is the effective DrawingML text-frame composition after
// master -> layout -> shape inheritance has been applied.
type TextBodyStyle struct {
	Anchor                    string
	AnchorCtr                 bool
	Rot                       int
	Wrap                      string
	LIns                      int64
	RIns                      int64
	TIns                      int64
	BIns                      int64
	Autofit                   string
	AutofitFontScale          int
	AutofitLineSpaceReduction int
	Columns                   int
	ColumnSpacingEMU          int64
}

// EffectiveTextBodyStyle composes text body properties in inheritance order.
// Layout owns this choice so OOXML resolution and rendering consume one result.
func EffectiveTextBodyStyle(shapeBody, layoutBody, masterBody *parts.TextBody) TextBodyStyle {
	style := TextBodyStyle{
		Anchor: "t",
		LIns:   DefaultTextInsetLeftRightEMU,
		RIns:   DefaultTextInsetLeftRightEMU,
		TIns:   DefaultTextInsetTopBottomEMU,
		BIns:   DefaultTextInsetTopBottomEMU,
	}
	applyTextBodyStyle(&style, masterBody)
	applyTextBodyStyle(&style, layoutBody)
	applyTextBodyStyle(&style, shapeBody)
	return style
}

func applyTextBodyStyle(style *TextBodyStyle, body *parts.TextBody) {
	if style == nil || body == nil || body.BodyPr == nil {
		return
	}
	pr := body.BodyPr
	if pr.Anchor != "" {
		style.Anchor = pr.Anchor
	}
	if pr.AnchorCtr != nil {
		style.AnchorCtr = *pr.AnchorCtr
	}
	if pr.Rot != 0 {
		style.Rot = pr.Rot
	}
	if pr.Wrap != "" {
		style.Wrap = pr.Wrap
	}
	if pr.LIns != nil {
		style.LIns = *pr.LIns
	}
	if pr.RIns != nil {
		style.RIns = *pr.RIns
	}
	if pr.TIns != nil {
		style.TIns = *pr.TIns
	}
	if pr.BIns != nil {
		style.BIns = *pr.BIns
	}

	switch {
	case pr.NoAutofit != nil:
		style.Autofit = "noAutofit"
		style.AutofitFontScale = 0
		style.AutofitLineSpaceReduction = 0
	case pr.SpAutoFit != nil:
		style.Autofit = "spAutoFit"
		style.AutofitFontScale = 0
		style.AutofitLineSpaceReduction = 0
	case pr.NormAutofit != nil:
		style.Autofit = "normAutofit"
		style.AutofitFontScale = 0
		style.AutofitLineSpaceReduction = 0
		if pr.NormAutofit.FontScale != nil {
			style.AutofitFontScale = *pr.NormAutofit.FontScale
		}
		if pr.NormAutofit.LnSpcReduction != nil {
			style.AutofitLineSpaceReduction = *pr.NormAutofit.LnSpcReduction
		}
	}
	if pr.NumCol != nil && *pr.NumCol > 1 {
		style.Columns = *pr.NumCol
		if pr.SpcCol != nil {
			style.ColumnSpacingEMU = *pr.SpcCol
		}
	}
}

func (style TextBodyStyle) DrawingProps() drawing.TextBodyProperties {
	return drawing.TextBodyProperties{
		Anchor:                    style.Anchor,
		AnchorCtr:                 style.AnchorCtr,
		Rot:                       style.Rot,
		Wrap:                      style.Wrap,
		AutofitType:               style.Autofit,
		AutofitFontScale:          style.AutofitFontScale,
		AutofitLineSpaceReduction: style.AutofitLineSpaceReduction,
		InsetL:                    style.LIns,
		InsetR:                    style.RIns,
		InsetT:                    style.TIns,
		InsetB:                    style.BIns,
		Columns:                   style.Columns,
		ColumnSpacingEMU:          style.ColumnSpacingEMU,
	}
}
