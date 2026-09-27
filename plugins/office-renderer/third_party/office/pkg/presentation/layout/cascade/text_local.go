package cascade

import (
	"office/pkg/presentation/ir"
	"office/pkg/presentation/parsers/ooxml/parts"
)

// ParagraphFromLocal composes paragraph-local PresentationML properties over
// the already-resolved inherited text defaults.
func ParagraphFromLocal(ap *parts.DMLParagraph, defaults TextDefaults, colors map[string]string, theme *ir.Theme) (*ir.Paragraph, TextDefaults) {
	if ap == nil {
		return nil, defaults
	}
	// Paragraph-local defRPr is the character default for runs without rPr.
	// endParaRPr formats only the paragraph end mark; it must not alter run defaults.
	if ap.Properties != nil {
		defaults = MergeEndParagraphRunDefaults(defaults, ap.Properties.DefRPr, colors, theme)
	}
	p := &ir.Paragraph{BulletChar: defaults.BulletChar, BulletImageRelID: defaults.BulletImageRelID, BulletNone: defaults.BulletNone, AutoNum: defaults.AutoNum, AutoNumType: defaults.AutoNumType, AutoNumStart: defaults.AutoNumStart, BulletColor: defaults.BulletColor, BulletFontFamily: defaults.BulletFontFamily, BulletFontTx: defaults.BulletFontTx, BulletSize: defaults.BulletSize, BulletSizePct: defaults.BulletSizePct, TabStops: defaults.TabStops}
	if defaults.MarginLeftEMU != nil {
		p.MarginLeftEMU = *defaults.MarginLeftEMU
		p.HasMarginLeft = true
	}
	if defaults.MarginRightEMU != nil {
		p.MarginRightEMU = *defaults.MarginRightEMU
		p.HasMarginRight = true
	}
	if defaults.TextIndentEMU != nil {
		p.TextIndentEMU = *defaults.TextIndentEMU
		p.HasTextIndent = true
	}
	if defaults.DefaultTabSizeEMU != nil {
		p.DefaultTabSizeEMU = *defaults.DefaultTabSizeEMU
	}
	p.FontAlign = defaults.FontAlign
	if defaults.SpaceBefore != nil {
		p.SpaceBefore = *defaults.SpaceBefore
		p.HasSpaceBefore = true
	} else if defaults.SpaceBeforePct != nil {
		p.SpaceBeforePct = *defaults.SpaceBeforePct
	}
	if defaults.SpaceAfter != nil {
		p.SpaceAfter = *defaults.SpaceAfter
		p.HasSpaceAfter = true
	} else if defaults.SpaceAfterPct != nil {
		p.SpaceAfterPct = *defaults.SpaceAfterPct
	}
	if defaults.LineSpacingPt != nil {
		p.LineSpacingPt = *defaults.LineSpacingPt
		p.HasLineSpacing = true
	} else if defaults.LineSpacingPct != nil {
		p.LineSpacingPct = *defaults.LineSpacingPct
		p.HasLineSpacing = true
	}
	pr := ap.Properties
	if pr == nil {
		p.Align = defaults.Align
		return p, defaults
	}
	p.Level, p.Align = pr.Lvl, pr.Algn
	if pr.MarL != nil {
		p.MarginLeftEMU = *pr.MarL
		p.HasMarginLeft = true
	}
	if pr.MarR != nil {
		p.MarginRightEMU = *pr.MarR
		p.HasMarginRight = true
	}
	if pr.Indent != nil {
		p.TextIndentEMU = *pr.Indent
		p.HasTextIndent = true
	}
	if pr.DefTabSz != nil {
		p.DefaultTabSizeEMU = *pr.DefTabSz
	}
	if pr.FontAlgn != "" {
		p.FontAlign = pr.FontAlgn
	}
	if v := SpacingPoints(pr.SpcBef); v != nil {
		p.SpaceBefore = *v
		p.HasSpaceBefore = true
		p.SpaceBeforePct = 0
	} else if v := SpacingPercentValue(pr.SpcBef); v != nil {
		p.SpaceBeforePct = *v
		p.SpaceBefore = 0
		p.HasSpaceBefore = false
	}
	if v := SpacingPoints(pr.SpcAft); v != nil {
		p.SpaceAfter = *v
		p.HasSpaceAfter = true
		p.SpaceAfterPct = 0
	} else if v := SpacingPercentValue(pr.SpcAft); v != nil {
		p.SpaceAfterPct = *v
		p.SpaceAfter = 0
		p.HasSpaceAfter = false
	}
	if v := SpacingPoints(pr.LnSpc); v != nil {
		p.LineSpacingPt = *v
		p.LineSpacingPct = 0
		p.HasLineSpacing = true
	} else if v := SpacingPercent(pr.LnSpc); v != nil {
		p.LineSpacingPct = *v
		p.LineSpacingPt = 0
		p.HasLineSpacing = true
	}
	if pr.BuNone != nil {
		p.BulletChar = ""
		p.BulletImageRelID = ""
		p.BulletNone = true
		p.AutoNum = false
		p.AutoNumType = ""
		p.AutoNumStart = 0
	} else {
		if pr.BuChar != nil && pr.BuChar.Char != "" {
			p.BulletChar = pr.BuChar.Char
			p.BulletImageRelID = ""
			p.BulletNone = false
		} else if pr.BuBlip != nil && pr.BuBlip.Blip.Embed != "" {
			p.BulletChar = ""
			p.BulletImageRelID = pr.BuBlip.Blip.Embed
			p.AutoNum = false
			p.BulletNone = false
		} else if pr.BuAutoNum != nil {
			// A local automatic-number marker replaces any bullet inherited from
			// layout/master. Keeping the inherited buChar makes text paint prefer
			// the glyph (often a Wingdings arrow) over the authored number.
			p.BulletChar = ""
			p.BulletImageRelID = ""
			p.AutoNum = true
			p.BulletNone = false
			p.AutoNumType = pr.BuAutoNum.Type
			p.AutoNumStart = pr.BuAutoNum.StartAt
		}
	}
	if v := BulletColor(pr.BuClr, pr.BuClrScheme, colors); v != "" {
		p.BulletColor = v
	}
	if pr.BuFontTx != nil {
		p.BulletFontTx = true
		p.BulletFontFamily = ""
	} else if v := BulletFontFamily(pr.BuFont); v != "" {
		p.BulletFontTx = false
		p.BulletFontFamily = v
	}
	if v := BulletSize(pr.BuSzPts); v > 0 {
		p.BulletSize = v
		p.BulletSizePct = 0
	}
	if v := BulletSizePercent(pr.BuSzPct); v > 0 {
		p.BulletSizePct = v
		p.BulletSize = 0
	}
	if len(pr.TabStops) > 0 {
		p.TabStops = TabStops(pr.TabStops)
	}
	if p.Align == "" {
		p.Align = defaults.Align
	}
	return p, defaults
}

// RunFromLocal composes run-local PresentationML character properties over
// inherited defaults. Hyperlink relationship lookup remains at the resolver boundary.
func RunFromLocal(text string, props *parts.DMLRunPr, defaults TextDefaults, colors map[string]string, theme *ir.Theme) *ir.Run {
	r := &ir.Run{Text: text, Bold: defaults.Bold != nil && *defaults.Bold, Italic: defaults.Italic != nil && *defaults.Italic, Underline: defaults.Underline != "" && defaults.Underline != "none", Strike: defaults.Strike != "" && defaults.Strike != "noStrike", AllCaps: defaults.Caps == "all", SmallCaps: defaults.Caps == "small", Superscript: defaults.Baseline != nil && *defaults.Baseline > 0, Subscript: defaults.Baseline != nil && *defaults.Baseline < 0, BaselineShiftPct: baselinePct(defaults.Baseline), FontSize: defaults.FontSize, FontFamily: defaults.FontFamily, Color: defaults.Color, Highlight: defaults.Highlight}
	if defaults.CharSpacing != nil {
		r.CharSpacing = *defaults.CharSpacing
	}
	if defaults.Kern != nil {
		r.Kern = *defaults.Kern
	}
	if props == nil {
		return r
	}
	if props.Bold != nil {
		r.Bold = *props.Bold
	}
	if props.Italic != nil {
		r.Italic = *props.Italic
	}
	if props.Underline != "" {
		r.Underline = props.Underline != "none"
	}
	if props.Strike != "" {
		r.Strike = props.Strike != "noStrike"
	}
	if props.Cap != "" {
		r.AllCaps = props.Cap == "all"
		r.SmallCaps = props.Cap == "small"
	}
	if props.Baseline != nil {
		r.Superscript = *props.Baseline > 0
		r.Subscript = *props.Baseline < 0
		r.BaselineShiftPct = float64(*props.Baseline) / 1000.0
	}
	if props.FontSize > 0 {
		r.FontSize = props.FontSize
	}
	if props.Spacing != nil {
		r.CharSpacing = *props.Spacing
	}
	if props.Kern != nil {
		r.Kern = *props.Kern
	}
	if v := RunColor(props, colors); v != "" {
		r.Color = v
	}
	if v := RunFontFamily(props, theme); v != "" {
		r.FontFamily = v
	}
	if v := RunHighlight(props, colors); v != "" {
		r.Highlight = v
	}
	return r
}

func baselinePct(v *int) float64 {
	if v == nil {
		return 0
	}
	return float64(*v) / 1000.0
}
