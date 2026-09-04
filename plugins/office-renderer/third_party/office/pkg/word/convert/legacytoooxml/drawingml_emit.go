package legacytoooxml

import (
	"fmt"
	"strings"

	"office/pkg/ooxml/drawing"
)

// emitFillXML emits DrawingML fill XML from a drawing.Fill value.
func emitFillXML(fill drawing.Fill) string {
	if fill == nil {
		return ""
	}
	switch f := fill.(type) {
	case drawing.SolidFill:
		return emitSolidFillXML(f)
	case *drawing.SolidFill:
		if f != nil {
			return emitSolidFillXML(*f)
		}
	case drawing.GradientFill:
		return emitGradientFillXML(f)
	case *drawing.GradientFill:
		if f != nil {
			return emitGradientFillXML(*f)
		}
	case drawing.PatternFill:
		return emitPatternFillXML(f)
	case *drawing.PatternFill:
		if f != nil {
			return emitPatternFillXML(*f)
		}
	case drawing.NoFill:
		return "<a:noFill/>"
	}
	return ""
}

func emitSolidFillXML(f drawing.SolidFill) string {
	var b strings.Builder
	b.WriteString(`<a:solidFill>`)
	if f.ThemeColor != "" {
		b.WriteString(fmt.Sprintf(`<a:schemeClr val="%s"`, f.ThemeColor))
		if f.ThemeShade != "" {
			b.WriteString(fmt.Sprintf(`><a:shade val="%s"/></a:schemeClr>`, f.ThemeShade))
		} else if f.ThemeTint != "" {
			b.WriteString(fmt.Sprintf(`><a:tint val="%s"/></a:schemeClr>`, f.ThemeTint))
		} else {
			b.WriteString(`/>`)
		}
	} else {
		b.WriteString(fmt.Sprintf(`<a:srgbClr val="%s"`, f.Color))
		alpha := emitAlphaXML(f.Alpha)
		if alpha != "" {
			b.WriteString(">" + alpha + `</a:srgbClr>`)
		} else {
			b.WriteString(`/>`)
		}
	}
	b.WriteString(`</a:solidFill>`)
	return b.String()
}

func emitGradientFillXML(f drawing.GradientFill) string {
	var b strings.Builder
	b.WriteString(`<a:gradFill`)
	if f.Flip != "" {
		b.WriteString(fmt.Sprintf(` flip="%s"`, f.Flip))
	}
	if f.RotateWithShape {
		b.WriteString(` rotWithShape="1"`)
	}
	b.WriteString(`><a:gsLst>`)
	for _, stop := range f.Stops {
		pos := int64(stop.Position * 100000)
		b.WriteString(fmt.Sprintf(`<a:gs pos="%d">`, pos))
		b.WriteString(fmt.Sprintf(`<a:srgbClr val="%s"`, stop.Color))
		alpha := emitAlphaXML(stop.Alpha)
		if alpha != "" {
			b.WriteString(">" + alpha + `</a:srgbClr>`)
		} else {
			b.WriteString(`/>`)
		}
		b.WriteString(`</a:gs>`)
	}
	b.WriteString(`</a:gsLst>`)
	if f.GradientType == "linear" {
		ang := int64(f.Angle * 60000)
		b.WriteString(fmt.Sprintf(`<a:lin ang="%d" scaled="1"/>`, ang))
	}
	b.WriteString(`</a:gradFill>`)
	return b.String()
}

func emitPatternFillXML(f drawing.PatternFill) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf(`<a:pattFill prst="%s">`, f.PatternType))
	b.WriteString(`<a:fgClr>`)
	b.WriteString(fmt.Sprintf(`<a:srgbClr val="%s"`, f.Foreground))
	alpha := emitAlphaXML(f.ForeAlpha)
	if alpha != "" {
		b.WriteString(">" + alpha + `</a:srgbClr>`)
	} else {
		b.WriteString(`/>`)
	}
	b.WriteString(`</a:fgClr>`)
	b.WriteString(`<a:bgClr>`)
	b.WriteString(fmt.Sprintf(`<a:srgbClr val="%s"`, f.Background))
	alpha = emitAlphaXML(f.BackAlpha)
	if alpha != "" {
		b.WriteString(">" + alpha + `</a:srgbClr>`)
	} else {
		b.WriteString(`/>`)
	}
	b.WriteString(`</a:bgClr>`)
	b.WriteString(`</a:pattFill>`)
	return b.String()
}

func emitAlphaXML(alpha float64) string {
	if alpha > 0 && alpha < 1.0 {
		val := int64(alpha * 100000)
		return fmt.Sprintf(`<a:alpha val="%d"/>`, val)
	}
	return ""
}

// emitLineStyleXML emits DrawingML line XML from a drawing.LineStyle value.
func emitLineStyleXML(ls *drawing.LineStyle) string {
	if ls == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<a:ln`)
	if ls.Width > 0 {
		w := int64(ls.Width * 12700)
		b.WriteString(fmt.Sprintf(` w="%d"`, w))
	}
	if ls.Cap != "" {
		b.WriteString(fmt.Sprintf(` cap="%s"`, ls.Cap))
	}
	if ls.Join != "" {
		b.WriteString(fmt.Sprintf(` join="%s"`, ls.Join))
	}
	if ls.Compound {
		if ls.CompoundType != "" {
			b.WriteString(fmt.Sprintf(` cmpd="%s"`, ls.CompoundType))
		} else {
			b.WriteString(` cmpd="dbl"`)
		}
	}
	b.WriteString(`>`)
	if ls.Color != "" {
		b.WriteString(fmt.Sprintf(`<a:solidFill><a:srgbClr val="%s"/></a:solidFill>`, ls.Color))
	}
	if ls.Dash != "" && ls.Dash != "solid" {
		b.WriteString(fmt.Sprintf(`<a:prstDash val="%s"/>`, ls.Dash))
	}
	b.WriteString(`</a:ln>`)
	return b.String()
}

// emitEffectListXML emits DrawingML effect list XML from a drawing.EffectList value.
func emitEffectListXML(el drawing.EffectList) string {
	if len(el.Effects) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<a:effectLst>`)
	for _, eff := range el.Effects {
		if eff == nil {
			continue
		}
		switch e := eff.(type) {
		case drawing.Shadow:
			b.WriteString(emitShadowXML(e))
		case *drawing.Shadow:
			if e != nil {
				b.WriteString(emitShadowXML(*e))
			}
		case drawing.InnerShadow:
			b.WriteString(emitInnerShadowXML(e))
		case *drawing.InnerShadow:
			if e != nil {
				b.WriteString(emitInnerShadowXML(*e))
			}
		case drawing.Glow:
			b.WriteString(emitGlowXML(e))
		case *drawing.Glow:
			if e != nil {
				b.WriteString(emitGlowXML(*e))
			}
		case drawing.SoftEdge:
			b.WriteString(emitSoftEdgeXML(e))
		case *drawing.SoftEdge:
			if e != nil {
				b.WriteString(emitSoftEdgeXML(*e))
			}
		case drawing.Reflection:
			b.WriteString(emitReflectionXML(e))
		case *drawing.Reflection:
			if e != nil {
				b.WriteString(emitReflectionXML(*e))
			}
		case drawing.Blur:
			b.WriteString(emitBlurXML(e))
		case *drawing.Blur:
			if e != nil {
				b.WriteString(emitBlurXML(*e))
			}
		}
	}
	b.WriteString(`</a:effectLst>`)
	return b.String()
}

func emitShadowXML(s drawing.Shadow) string {
	var b strings.Builder
	b.WriteString(`<a:outerShdw`)
	if s.Blur > 0 {
		b.WriteString(fmt.Sprintf(` blurRad="%d"`, int64(s.Blur*12700)))
	}
	if s.Distance > 0 {
		b.WriteString(fmt.Sprintf(` dist="%d"`, int64(s.Distance*12700)))
	}
	if s.Angle != 0 {
		b.WriteString(fmt.Sprintf(` dir="%d"`, int64(s.Angle*60000)))
	}
	if s.ScaleX > 0 && s.ScaleX != 1.0 {
		b.WriteString(fmt.Sprintf(` sx="%d"`, int64(s.ScaleX*100000)))
	}
	if s.ScaleY > 0 && s.ScaleY != 1.0 {
		b.WriteString(fmt.Sprintf(` sy="%d"`, int64(s.ScaleY*100000)))
	}
	b.WriteString(`>`)
	b.WriteString(fmt.Sprintf(`<a:srgbClr val="%s"`, s.Color))
	alpha := emitAlphaXML(s.Alpha)
	if alpha != "" {
		b.WriteString(">" + alpha + `</a:srgbClr>`)
	} else {
		b.WriteString(`/>`)
	}
	b.WriteString(`</a:outerShdw>`)
	return b.String()
}

func emitInnerShadowXML(s drawing.InnerShadow) string {
	var b strings.Builder
	b.WriteString(`<a:innerShdw`)
	if s.Blur > 0 {
		b.WriteString(fmt.Sprintf(` blurRad="%d"`, int64(s.Blur*12700)))
	}
	if s.Distance > 0 {
		b.WriteString(fmt.Sprintf(` dist="%d"`, int64(s.Distance*12700)))
	}
	if s.Angle != 0 {
		b.WriteString(fmt.Sprintf(` dir="%d"`, int64(s.Angle*60000)))
	}
	b.WriteString(`>`)
	b.WriteString(fmt.Sprintf(`<a:srgbClr val="%s"`, s.Color))
	alpha := emitAlphaXML(s.Alpha)
	if alpha != "" {
		b.WriteString(">" + alpha + `</a:srgbClr>`)
	} else {
		b.WriteString(`/>`)
	}
	b.WriteString(`</a:innerShdw>`)
	return b.String()
}

func emitGlowXML(g drawing.Glow) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf(`<a:glow rad="%d">`, int64(g.Radius*12700)))
	b.WriteString(fmt.Sprintf(`<a:srgbClr val="%s"`, g.Color))
	alpha := emitAlphaXML(g.Alpha)
	if alpha != "" {
		b.WriteString(">" + alpha + `</a:srgbClr>`)
	} else {
		b.WriteString(`/>`)
	}
	b.WriteString(`</a:glow>`)
	return b.String()
}

func emitSoftEdgeXML(s drawing.SoftEdge) string {
	return fmt.Sprintf(`<a:softEdge rad="%d"/>`, int64(s.Radius*12700))
}

func emitReflectionXML(r drawing.Reflection) string {
	var b strings.Builder
	b.WriteString(`<a:reflection`)
	if r.Blur > 0 {
		b.WriteString(fmt.Sprintf(` blurRad="%d"`, int64(r.Blur*12700)))
	}
	if r.Distance > 0 {
		b.WriteString(fmt.Sprintf(` dist="%d"`, int64(r.Distance*12700)))
	}
	if r.Alpha > 0 && r.Alpha < 1.0 {
		b.WriteString(fmt.Sprintf(` algn="%d"`, int64(r.Alpha*100000)))
	}
	b.WriteString(`/>`)
	return b.String()
}

func emitBlurXML(bl drawing.Blur) string {
	return fmt.Sprintf(`<a:blur rad="%d" grow="%t"/>`, int64(bl.Radius*12700), bl.Grow)
}

// emitScene3DXML emits a:scene3d and a:sp3d elements from a drawing.ThreeD value.
func emitScene3DXML(td *drawing.ThreeD) string {
	if td == nil {
		return ""
	}
	var b strings.Builder
	if td.CameraPreset != "" || td.LightRig != "" {
		b.WriteString(`<a:scene3d>`)
		if td.CameraPreset != "" {
			b.WriteString(fmt.Sprintf(`<a:camera prst="%s"/>`, td.CameraPreset))
		}
		if td.LightRig != "" {
			dir := td.LightDirection
			if dir == "" {
				dir = "t"
			}
			b.WriteString(fmt.Sprintf(`<a:lightRig rig="%s" dir="%s"/>`, td.LightRig, dir))
		}
		b.WriteString(`</a:scene3d>`)
	}
	b.WriteString(`<a:sp3d`)
	if td.ExtrusionDepth > 0 {
		b.WriteString(fmt.Sprintf(` extrusionH="%d"`, int64(td.ExtrusionDepth*12700)))
	}
	if td.ContourWidth > 0 {
		b.WriteString(fmt.Sprintf(` contourW="%d"`, int64(td.ContourWidth*12700)))
	}
	if td.Material != "" {
		b.WriteString(fmt.Sprintf(` prstMaterial="%s"`, td.Material))
	}
	b.WriteString(`>`)
	if td.BevelTopWidth > 0 || td.BevelTopHeight > 0 {
		b.WriteString(fmt.Sprintf(`<a:bevelT w="%d" h="%d"/>`, int64(td.BevelTopWidth*12700), int64(td.BevelTopHeight*12700)))
	}
	if td.BevelBottomWidth > 0 || td.BevelBottomHeight > 0 {
		b.WriteString(fmt.Sprintf(`<a:bevelB w="%d" h="%d"/>`, int64(td.BevelBottomWidth*12700), int64(td.BevelBottomHeight*12700)))
	}
	if td.ContourColor != "" {
		b.WriteString(fmt.Sprintf(`<a:contourClr><a:srgbClr val="%s"/></a:contourClr>`, td.ContourColor))
	}
	if td.ExtrusionColor != "" {
		b.WriteString(fmt.Sprintf(`<a:extrusionClr><a:srgbClr val="%s"/></a:extrusionClr>`, td.ExtrusionColor))
	}
	b.WriteString(`</a:sp3d>`)
	return b.String()
}