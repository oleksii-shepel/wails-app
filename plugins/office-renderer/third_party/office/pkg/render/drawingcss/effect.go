package drawingcss

import (
	"fmt"
	"math"
	"strings"

	"office/pkg/ooxml/drawing"
)

// Effect returns CSS properties for a single DrawingML effect.
func Effect(effect drawing.Effect) map[string]string {
	switch e := effect.(type) {
	case drawing.Shadow:
		return Shadow(e)
	case *drawing.Shadow:
		if e == nil {
			return nil
		}
		return Shadow(*e)
	case drawing.InnerShadow:
		return InnerShadow(e)
	case *drawing.InnerShadow:
		if e == nil {
			return nil
		}
		return InnerShadow(*e)
	case drawing.Glow:
		return Glow(e)
	case *drawing.Glow:
		if e == nil {
			return nil
		}
		return Glow(*e)
	case drawing.SoftEdge:
		return SoftEdge(e)
	case *drawing.SoftEdge:
		if e == nil {
			return nil
		}
		return SoftEdge(*e)
	case drawing.Reflection:
		return Reflection(e)
	case *drawing.Reflection:
		if e == nil {
			return nil
		}
		return Reflection(*e)
	case drawing.Blur:
		return Blur(e)
	case *drawing.Blur:
		if e == nil {
			return nil
		}
		return Blur(*e)
	case drawing.ThreeD:
		return ThreeD(e)
	case *drawing.ThreeD:
		if e == nil {
			return nil
		}
		return ThreeD(*e)
	case drawing.Grayscale:
		return Grayscale(e)
	case *drawing.Grayscale:
		if e == nil {
			return nil
		}
		return Grayscale(*e)
	case drawing.BiLevel:
		return BiLevel(e)
	case *drawing.BiLevel:
		if e == nil {
			return nil
		}
		return BiLevel(*e)
	case drawing.AlphaBiLevel:
		return AlphaBiLevel(e)
	case *drawing.AlphaBiLevel:
		if e == nil {
			return nil
		}
		return AlphaBiLevel(*e)
	case drawing.Duotone:
		return Duotone(e)
	case *drawing.Duotone:
		if e == nil {
			return nil
		}
		return Duotone(*e)
	case drawing.AlphaMod:
		return AlphaMod(e)
	case *drawing.AlphaMod:
		if e == nil {
			return nil
		}
		return AlphaMod(*e)
	case drawing.AlphaOff:
		return AlphaOff(e)
	case *drawing.AlphaOff:
		if e == nil {
			return nil
		}
		return AlphaOff(*e)
	case drawing.AlphaModFix:
		return AlphaModFix(e)
	case *drawing.AlphaModFix:
		if e == nil {
			return nil
		}
		return AlphaModFix(*e)
	case drawing.AlphaRepl:
		return AlphaRepl(e)
	case *drawing.AlphaRepl:
		if e == nil {
			return nil
		}
		return AlphaRepl(*e)
	case drawing.AlphaOutset:
		return AlphaOutset(e)
	case *drawing.AlphaOutset:
		if e == nil {
			return nil
		}
		return AlphaOutset(*e)
	case drawing.AlphaInv:
		return AlphaInv(e)
	case *drawing.AlphaInv:
		if e == nil {
			return nil
		}
		return AlphaInv(*e)
	case drawing.AlphaCeiling:
		return AlphaCeiling(e)
	case *drawing.AlphaCeiling:
		if e == nil {
			return nil
		}
		return AlphaCeiling(*e)
	case drawing.AlphaFloor:
		return AlphaFloor(e)
	case *drawing.AlphaFloor:
		if e == nil {
			return nil
		}
		return AlphaFloor(*e)
	case drawing.DeSat:
		return DeSat(e)
	case *drawing.DeSat:
		if e == nil {
			return nil
		}
		return DeSat(*e)
	case drawing.DeSatLow:
		return DeSatLow(e)
	case *drawing.DeSatLow:
		if e == nil {
			return nil
		}
		return DeSatLow(*e)
	case drawing.DeSatMod:
		return DeSatMod(e)
	case *drawing.DeSatMod:
		if e == nil {
			return nil
		}
		return DeSatMod(*e)
	case drawing.HSLEffect:
		return HSLEffect(e)
	case *drawing.HSLEffect:
		if e == nil {
			return nil
		}
		return HSLEffect(*e)
	case drawing.Lum:
		return Lum(e)
	case *drawing.Lum:
		if e == nil {
			return nil
		}
		return Lum(*e)
	case drawing.ClrChange:
		return ClrChange(e)
	case *drawing.ClrChange:
		if e == nil {
			return nil
		}
		return ClrChange(*e)
	case drawing.ClrRepl:
		return ClrRepl(e)
	case *drawing.ClrRepl:
		if e == nil {
			return nil
		}
		return ClrRepl(*e)
	case drawing.FillOverlay:
		return FillOverlay(e)
	case *drawing.FillOverlay:
		if e == nil {
			return nil
		}
		return FillOverlay(*e)
	case drawing.BlendMode:
		return BlendMode(e)
	case *drawing.BlendMode:
		if e == nil {
			return nil
		}
		return BlendMode(*e)
	case drawing.RelativeOffset:
		return RelativeOffset(e)
	case *drawing.RelativeOffset:
		if e == nil {
			return nil
		}
		return RelativeOffset(*e)
	case drawing.Tint:
		return Tint(e)
	case *drawing.Tint:
		if e == nil {
			return nil
		}
		return Tint(*e)
	case drawing.TransformEffect:
		return TransformEffect(e)
	case *drawing.TransformEffect:
		if e == nil {
			return nil
		}
		return TransformEffect(*e)
	default:
		return nil
	}
}

// Shadow returns CSS properties for a shadow effect.
func Shadow(s drawing.Shadow) map[string]string {
	props := make(map[string]string)

	// Calculate offset
	rad := s.Angle * math.Pi / 180
	offsetX := s.Distance * math.Cos(rad)
	offsetY := s.Distance * math.Sin(rad)

	// Build shadow value
	color := s.Color
	if s.Alpha < 1.0 {
		color = fmt.Sprintf("rgba(%s, %s, %s, %.2f)",
			hexToRGB(s.Color, 0),
			hexToRGB(s.Color, 1),
			hexToRGB(s.Color, 2),
			s.Alpha)
	} else {
		color = "#" + s.Color
	}

	shadowValue := fmt.Sprintf("%.1fpt %.1fpt %.1fpt %s",
		offsetX, offsetY, s.Blur, color)

	props["filter"] = fmt.Sprintf("drop-shadow(%s)", shadowValue)

	return props
}

// InnerShadow returns CSS properties for an inner shadow effect.
func InnerShadow(s drawing.InnerShadow) map[string]string {
	rad := s.Angle * math.Pi / 180
	offsetX := s.Distance * math.Cos(rad)
	offsetY := s.Distance * math.Sin(rad)

	var colorVal string
	if s.Alpha < 1.0 {
		colorVal = fmt.Sprintf("rgba(%s, %s, %s, %.2f)",
			hexToRGB(s.Color, 0),
			hexToRGB(s.Color, 1),
			hexToRGB(s.Color, 2),
			s.Alpha)
	} else {
		colorVal = "#" + s.Color
	}

	return map[string]string{
		"box-shadow": fmt.Sprintf("inset %.1fpt %.1fpt %.1fpt %s", offsetX, offsetY, s.Blur, colorVal),
	}
}

// Glow returns CSS properties for a glow effect.
func Glow(g drawing.Glow) map[string]string {
	props := make(map[string]string)

	// Glow is implemented as a combination of blur and color
	color := g.Color
	if g.Alpha < 1.0 {
		color = fmt.Sprintf("rgba(%s, %s, %s, %.2f)",
			hexToRGB(g.Color, 0),
			hexToRGB(g.Color, 1),
			hexToRGB(g.Color, 2),
			g.Alpha)
	} else {
		color = "#" + g.Color
	}

	// Create a glow filter
	// Note: CSS doesn't have a direct glow equivalent, so we use drop-shadow
	// with zero offset and blur radius
	glowValue := fmt.Sprintf("0 0 %.1fpt %s", g.Radius, color)
	props["filter"] = fmt.Sprintf("drop-shadow(%s)", glowValue)

	return props
}

// SoftEdge returns CSS properties for a soft edge effect.
func SoftEdge(se drawing.SoftEdge) map[string]string {
	props := make(map[string]string)

	// Soft edge is essentially a blur
	props["filter"] = fmt.Sprintf("blur(%.1fpt)", se.Radius)

	return props
}

// Reflection returns CSS properties for a reflection effect.
func Reflection(r drawing.Reflection) map[string]string {
	// CSS cannot replicate a true reflection without pseudo-elements, but we can
	// approximate the visual footprint using a transparent box-shadow below the shape.
	// The shadow is a vertically flipped, partially transparent clone at `Distance` offset.
	props := make(map[string]string)

	alpha := r.Alpha
	if alpha <= 0 {
		alpha = 0.3 // sensible default
	}
	dist := r.Distance
	blur := r.Blur
	if blur <= 0 {
		blur = 1
	}

	// Map direction to an offset direction.
	rad := r.Direction * math.Pi / 180
	offX := dist * math.Cos(rad)
	offY := dist * math.Sin(rad)

	shadowColor := fmt.Sprintf("rgba(128,128,128,%.2f)", alpha)
	props["box-shadow"] = fmt.Sprintf("%.1fpt %.1fpt %.1fpt %s",
		offX, offY, blur, shadowColor)
	return props
}

// Blur returns CSS properties for a blur effect.
func Blur(b drawing.Blur) map[string]string {
	props := make(map[string]string)

	props["filter"] = fmt.Sprintf("blur(%.1fpt)", b.Radius)

	return props
}

// ThreeD returns CSS properties that approximate DrawingML scene3d/sp3d shape styling.
func ThreeD(t drawing.ThreeD) map[string]string {
	if t.ExtrusionDepth <= 0 && t.ContourWidth <= 0 && t.BevelTopHeight <= 0 && t.BevelBottomHeight <= 0 && t.RotationX == 0 && t.RotationY == 0 && t.RotationZ == 0 {
		return nil
	}

	props := make(map[string]string)
	transforms := make([]string, 0, 4)
	transforms = append(transforms, "perspective(960px)")
	if rx := threeDRotateX(t); rx != 0 {
		transforms = append(transforms, fmt.Sprintf("rotateX(%.2fdeg)", rx))
	}
	if ry := threeDRotateY(t); ry != 0 {
		transforms = append(transforms, fmt.Sprintf("rotateY(%.2fdeg)", ry))
	}
	if rz := threeDRotateZ(t); rz != 0 {
		transforms = append(transforms, fmt.Sprintf("rotate(%.2fdeg)", rz))
	}
	if len(transforms) > 0 {
		props["transform"] = strings.Join(transforms, " ")
		props["transform-style"] = "preserve-3d"
		props["transform-origin"] = "50% 50%"
	}

	shadowColor := t.ExtrusionColor
	if shadowColor == "" {
		shadowColor = "666666"
	}
	highlightColor := t.ContourColor
	if highlightColor == "" {
		highlightColor = "FFFFFF"
	}
	shadowAlpha, highlightAlpha := threeDMaterialOpacities(t.Material)
	xOff, yOff := threeDShadowOffset(t)
	depthBlur := math.Max(t.ExtrusionDepth*0.35, 1.0)
	// Combine contour and bevel dimensions; use both width and height for a
	// more accurate bevel approximation.
	bevelTop := math.Max(t.BevelTopWidth, t.BevelTopHeight) * 0.6
	bevelBottom := math.Max(t.BevelBottomWidth, t.BevelBottomHeight) * 0.6
	contour := math.Max(t.ContourWidth, math.Max(bevelTop, bevelBottom))
	var shadows []string
	if contour > 0 {
		shadows = append(shadows,
			fmt.Sprintf("inset %.2fpt %.2fpt %.2fpt rgba(%s, %.2f)", -xOff*0.35, -yOff*0.35, math.Max(contour*0.4, 0.75), rgbComponents(highlightColor), highlightAlpha),
			fmt.Sprintf("inset %.2fpt %.2fpt %.2fpt rgba(%s, %.2f)", xOff*0.45, yOff*0.45, math.Max(contour*0.5, 0.75), rgbComponents(shadowColor), shadowAlpha),
		)
	}
	if t.ExtrusionDepth > 0 {
		shadows = append(shadows, fmt.Sprintf("%.2fpt %.2fpt %.2fpt rgba(%s, %.2f)", xOff, yOff, depthBlur, rgbComponents(shadowColor), shadowAlpha+0.06))
		props["filter"] = fmt.Sprintf("drop-shadow(%.2fpt %.2fpt %.2fpt rgba(%s, %.2f))", xOff, yOff, depthBlur*0.85, rgbComponents(shadowColor), shadowAlpha+0.1)
	}
	if len(shadows) > 0 {
		props["box-shadow"] = strings.Join(shadows, ", ")
	}
	return props
}

// threeDMaterialOpacities returns shadow and highlight alphas based on the
// DrawingML preset material. Values are tuned to approximate Office’s
// material lighting response in CSS.
func threeDMaterialOpacities(material string) (shadowAlpha, highlightAlpha float64) {
	switch strings.ToLower(strings.TrimSpace(material)) {
	case "metal", "metal2", "legacymetal", "softmetal":
		return 0.36, 0.28
	case "plastic", "legacyplastic":
		return 0.32, 0.24
	case "matte", "legacymatte", "warmmatte":
		return 0.26, 0.20
	case "dkedge", "softedge":
		return 0.30, 0.22
	case "clear":
		return 0.22, 0.18
	case "powder", "translucentpowder":
		return 0.18, 0.15
	case "flat":
		return 0.24, 0.18
	case "legacywireframe":
		return 0.15, 0.12
	default:
		return 0.28, 0.22
	}
}

func threeDRotateX(t drawing.ThreeD) float64 {
	rx := t.RotationX
	if rx == 0 {
		switch t.CameraPreset {
		case "legacyPerspectiveTopLeft", "perspectiveAboveLeftFacing", "perspectiveHeroicLeftFacing", "perspectiveHeroicExtremeLeftFacing":
			rx = -22
		case "legacyPerspectiveTop", "perspectiveAbove", "perspectiveHeroicExtremeTop":
			rx = -18
		case "legacyPerspectiveTopRight", "perspectiveAboveRightFacing", "perspectiveHeroicRightFacing", "perspectiveHeroicExtremeRightFacing":
			rx = -22
		case "legacyPerspectiveBottomLeft", "perspectiveBelowLeftFacing":
			rx = 22
		case "legacyPerspectiveBottom", "perspectiveBelow", "perspectiveHeroicExtremeBottom":
			rx = 18
		case "legacyPerspectiveBottomRight", "perspectiveBelowRightFacing":
			rx = 22
		case "perspectiveRelaxed", "perspectiveRelaxedModerately":
			rx = -8
		case "orthographicTop":
			rx = -35
		case "orthographicBottom":
			rx = 35
		}
	}
	return clampFloat(rx, -50, 50)
}

func threeDRotateY(t drawing.ThreeD) float64 {
	ry := t.RotationY
	if ry == 0 {
		switch t.CameraPreset {
		case "legacyPerspectiveLeft", "legacyPerspectiveTopLeft", "legacyPerspectiveBottomLeft", "perspectiveFrontLeftFacing", "perspectiveContrastingLeftFacing", "perspectiveHeroicLeftFacing", "perspectiveHeroicExtremeLeftFacing":
			ry = -22
		case "legacyPerspectiveRight", "legacyPerspectiveTopRight", "legacyPerspectiveBottomRight", "perspectiveFrontRightFacing", "perspectiveContrastingRightFacing", "perspectiveHeroicRightFacing", "perspectiveHeroicExtremeRightFacing":
			ry = 22
		case "perspectiveRelaxed", "perspectiveRelaxedModerately":
			ry = 8
		case "orthographicLeft":
			ry = -35
		case "orthographicRight":
			ry = 35
		}
	}
	return clampFloat(ry, -50, 50)
}

func threeDRotateZ(t drawing.ThreeD) float64 {
	rz := t.RotationZ
	if rz == 0 {
		switch t.CameraPreset {
		case "legacyObliqueTopLeft", "obliqueTopLeft":
			rz = -10
		case "legacyObliqueTopRight", "obliqueTopRight":
			rz = 10
		case "legacyObliqueBottomLeft", "obliqueBottomLeft":
			rz = -10
		case "legacyObliqueBottomRight", "obliqueBottomRight":
			rz = 10
		case "legacyObliqueLeft", "obliqueLeft":
			rz = -5
		case "legacyObliqueRight", "obliqueRight":
			rz = 5
		}
	}
	return clampFloat(rz, -25, 25)
}

func threeDShadowOffset(t drawing.ThreeD) (float64, float64) {
	dist := math.Max(t.ExtrusionDepth, math.Max(t.ContourWidth, 1.5))
	angle := 45.0
	switch t.LightDirection {
	case "t", "tl":
		angle = 315
	case "tr":
		angle = 225
	case "l":
		angle = 0
	case "r":
		angle = 180
	case "bl":
		angle = 45
	case "b", "br":
		angle = 135
	}
	rad := angle * math.Pi / 180
	return dist * math.Cos(rad), dist * math.Sin(rad)
}

// Grayscale returns CSS properties for a grayscale image effect.
func Grayscale(g drawing.Grayscale) map[string]string {
	return map[string]string{"filter": "grayscale(100%)"}
}

// BiLevel returns CSS properties for a bi-level image effect.
func BiLevel(b drawing.BiLevel) map[string]string {
	// Approximate bi-level with a high-contrast threshold
	return map[string]string{"filter": "contrast(1000%) grayscale(100%)"}
}

// AlphaBiLevel returns CSS properties for an alpha bi-level effect.
func AlphaBiLevel(a drawing.AlphaBiLevel) map[string]string {
	return nil
}

// Duotone returns CSS properties for a duotone image effect.
func Duotone(d drawing.Duotone) map[string]string {
	// Approximate duotone with a CSS filter chain
	return map[string]string{"filter": "grayscale(100%) sepia(100%) saturate(200%)"}
}

// AlphaMod returns CSS properties for an alpha modifier effect.
func AlphaMod(a drawing.AlphaMod) map[string]string {
	amt := float64(a.Amount) / 100000.0
	if amt <= 0 || amt >= 1 {
		return nil
	}
	return map[string]string{"opacity": fmt.Sprintf("%.4f", amt)}
}

// AlphaOff returns CSS properties for an alpha offset effect.
func AlphaOff(a drawing.AlphaOff) map[string]string {
	return nil
}

// AlphaModFix returns CSS properties for a fixed alpha modifier effect.
func AlphaModFix(a drawing.AlphaModFix) map[string]string {
	amt := float64(a.Amount) / 100000.0
	if amt <= 0 || amt >= 1 {
		return nil
	}
	return map[string]string{"opacity": fmt.Sprintf("%.4f", amt)}
}

// AlphaRepl returns CSS properties for an alpha replace effect.
func AlphaRepl(a drawing.AlphaRepl) map[string]string {
	alpha := float64(a.Alpha) / 100000.0
	return map[string]string{"opacity": fmt.Sprintf("%.4f", alpha)}
}

// AlphaOutset returns CSS properties for an alpha outset effect.
func AlphaOutset(a drawing.AlphaOutset) map[string]string {
	if a.Radius <= 0 {
		return nil
	}
	// Approximate alpha outset by expanding edges with a blur.
	return map[string]string{"filter": fmt.Sprintf("blur(%.1fpt)", a.Radius)}
}

// AlphaInv returns CSS properties for an alpha inverse effect.
func AlphaInv(a drawing.AlphaInv) map[string]string {
	return nil // Cannot be directly represented in CSS
}

// AlphaCeiling returns CSS properties for an alpha ceiling effect.
func AlphaCeiling(a drawing.AlphaCeiling) map[string]string {
	return nil // Cannot be directly represented in CSS
}

// AlphaFloor returns CSS properties for an alpha floor effect.
func AlphaFloor(a drawing.AlphaFloor) map[string]string {
	return nil // Cannot be directly represented in CSS
}

// DeSat returns CSS properties for a desaturation effect.
func DeSat(d drawing.DeSat) map[string]string {
	return map[string]string{"filter": "saturate(0%)"}
}

// DeSatLow returns CSS properties for a low desaturation effect.
func DeSatLow(d drawing.DeSatLow) map[string]string {
	return map[string]string{"filter": "saturate(50%)"}
}

// DeSatMod returns CSS properties for a desaturation modifier effect.
func DeSatMod(d drawing.DeSatMod) map[string]string {
	amt := float64(d.Amount) / 100000.0
	return map[string]string{"filter": fmt.Sprintf("saturate(%.0f%%)", amt*100)}
}

// HSLEffect returns CSS properties for an HSL modifier effect.
func HSLEffect(h drawing.HSLEffect) map[string]string {
	var filters []string
	if h.Hue != 0 {
		deg := float64(h.Hue) / 60000.0
		filters = append(filters, fmt.Sprintf("hue-rotate(%.2fdeg)", deg))
	}
	if h.Sat > 0 && h.Sat != 100000 {
		amt := float64(h.Sat) / 100000.0
		filters = append(filters, fmt.Sprintf("saturate(%.0f%%)", amt*100))
	}
	if h.Lum > 0 && h.Lum != 100000 {
		amt := float64(h.Lum) / 100000.0
		filters = append(filters, fmt.Sprintf("brightness(%.0f%%)", amt*100))
	}
	if len(filters) == 0 {
		return nil
	}
	return map[string]string{"filter": strings.Join(filters, " ")}
}

// Lum returns CSS properties for a luminance effect.
func Lum(l drawing.Lum) map[string]string {
	var filters []string
	if l.Brightness > 0 && l.Brightness != 100000 {
		amt := float64(l.Brightness) / 100000.0
		filters = append(filters, fmt.Sprintf("brightness(%.0f%%)", amt*100))
	}
	if l.Contrast > 0 && l.Contrast != 100000 {
		amt := float64(l.Contrast) / 100000.0
		filters = append(filters, fmt.Sprintf("contrast(%.0f%%)", amt*100))
	}
	if len(filters) == 0 {
		return nil
	}
	return map[string]string{"filter": strings.Join(filters, " ")}
}

// ClrChange returns CSS properties for a color change effect.
func ClrChange(c drawing.ClrChange) map[string]string {
	// Cannot be directly represented in CSS
	return nil
}

// ClrRepl returns CSS properties for a color replacement effect.
func ClrRepl(c drawing.ClrRepl) map[string]string {
	// Cannot be directly represented in CSS
	return nil
}

// FillOverlay returns CSS properties for a fill overlay effect.
func FillOverlay(f drawing.FillOverlay) map[string]string {
	// Cannot be directly represented in CSS
	return nil
}

// BlendMode returns CSS properties for a blend wrapper effect.
func BlendMode(b drawing.BlendMode) map[string]string {
	mode := strings.TrimSpace(strings.ToLower(b.Mode))
	if mode == "" || mode == "over" {
		return nil
	}
	switch mode {
	case "mult":
		mode = "multiply"
	case "screen", "darken", "lighten":
	default:
		return nil
	}
	return map[string]string{"mix-blend-mode": mode}
}

// RelativeOffset returns CSS properties for a relative translation effect.
func RelativeOffset(r drawing.RelativeOffset) map[string]string {
	if r.X == 0 && r.Y == 0 {
		return nil
	}
	return map[string]string{
		"transform": fmt.Sprintf("translate(%.2f%%, %.2f%%)", r.X*100, r.Y*100),
	}
}

// Tint returns CSS properties for a tint effect.
func Tint(t drawing.Tint) map[string]string {
	var filters []string
	if t.Hue != 0 {
		filters = append(filters, fmt.Sprintf("hue-rotate(%.2fdeg)", float64(t.Hue)/60000.0))
	}
	if t.Amount != 0 {
		amount := clampFloat(float64(t.Amount)/100000.0, -1, 1)
		if amount > 0 {
			filters = append(filters, fmt.Sprintf("brightness(%.0f%%)", (1+amount)*100))
		} else {
			filters = append(filters, fmt.Sprintf("brightness(%.0f%%)", (1+(amount*0.5))*100))
		}
	}
	if len(filters) == 0 {
		return nil
	}
	return map[string]string{"filter": strings.Join(filters, " ")}
}

// TransformEffect returns CSS properties for a transform effect.
func TransformEffect(t drawing.TransformEffect) map[string]string {
	var transforms []string
	if t.TranslateX != 0 || t.TranslateY != 0 {
		transforms = append(transforms, fmt.Sprintf("translate(%.1fpt, %.1fpt)", t.TranslateX, t.TranslateY))
	}
	if t.SkewX != 0 {
		transforms = append(transforms, fmt.Sprintf("skewX(%.2fdeg)", t.SkewX))
	}
	if t.SkewY != 0 {
		transforms = append(transforms, fmt.Sprintf("skewY(%.2fdeg)", t.SkewY))
	}
	if t.ScaleX != 0 && t.ScaleX != 1 || t.ScaleY != 0 && t.ScaleY != 1 {
		scaleX := t.ScaleX
		scaleY := t.ScaleY
		if scaleX == 0 {
			scaleX = 1
		}
		if scaleY == 0 {
			scaleY = 1
		}
		transforms = append(transforms, fmt.Sprintf("scale(%.4f, %.4f)", scaleX, scaleY))
	}
	if len(transforms) == 0 {
		return nil
	}
	return map[string]string{"transform": strings.Join(transforms, " ")}
}

// EffectList returns CSS properties for a collection of effects.
func EffectList(el drawing.EffectList) map[string]string {
	props := make(map[string]string)

	var filters []string
	var shadows []string
	var transforms []string
	opacity := 1.0
	hasOpacity := false

	for _, effect := range el.Effects {
		if updatedOpacity, ok := applyOpacityEffect(effect, opacity); ok {
			opacity = updatedOpacity
			hasOpacity = true
		}
		css := Effect(effect)
		if css == nil {
			continue
		}
		if v, ok := css["filter"]; ok && v != "" {
			filters = append(filters, v)
		}
		if v, ok := css["box-shadow"]; ok && v != "" {
			shadows = append(shadows, v)
		}
		if v, ok := css["transform"]; ok && v != "" {
			transforms = append(transforms, v)
		}
		// Copy any other properties directly.
		for k, v := range css {
			if k == "filter" || k == "box-shadow" || k == "transform" {
				continue
			}
			if k == "opacity" {
				if _, handled := applyOpacityEffect(effect, opacity); handled {
					continue
				}
				if alpha, ok := parseOpacity(v); ok {
					opacity *= alpha
					hasOpacity = true
				}
				continue
			}
			props[k] = v
		}
	}

	if len(filters) > 0 {
		combined := filters[0]
		for _, f := range filters[1:] {
			combined += " " + f
		}
		props["filter"] = combined
	}
	if len(shadows) > 0 {
		combined := shadows[0]
		for _, s := range shadows[1:] {
			combined += ", " + s
		}
		props["box-shadow"] = combined
	}
	if len(transforms) > 0 {
		combined := transforms[0]
		for _, t := range transforms[1:] {
			combined += " " + t
		}
		props["transform"] = combined
	}
	if hasOpacity {
		props["opacity"] = fmt.Sprintf("%.4f", clampFloat(opacity, 0, 1))
	}

	return props
}

// ApproximateOpacity derives a best-effort whole-shape opacity from the effect list.
func ApproximateOpacity(el drawing.EffectList) (float64, bool) {
	opacity := 1.0
	found := false
	for _, effect := range el.Effects {
		if updatedOpacity, ok := applyOpacityEffect(effect, opacity); ok {
			opacity = updatedOpacity
			found = true
			continue
		}
		if alpha, ok := approximateEffectOpacity(effect); ok {
			opacity = clampFloat(opacity*alpha, 0, 1)
			found = true
		}
	}
	return clampFloat(opacity, 0, 1), found
}

func approximateEffectOpacity(effect drawing.Effect) (float64, bool) {
	switch e := effect.(type) {
	case drawing.Shadow:
		return approximateAlphaValue(e.Alpha)
	case *drawing.Shadow:
		if e == nil {
			return 0, false
		}
		return approximateAlphaValue(e.Alpha)
	case drawing.InnerShadow:
		return approximateAlphaValue(e.Alpha)
	case *drawing.InnerShadow:
		if e == nil {
			return 0, false
		}
		return approximateAlphaValue(e.Alpha)
	case drawing.Glow:
		return approximateAlphaValue(e.Alpha)
	case *drawing.Glow:
		if e == nil {
			return 0, false
		}
		return approximateAlphaValue(e.Alpha)
	case drawing.Reflection:
		return approximateAlphaValue(e.Alpha)
	case *drawing.Reflection:
		if e == nil {
			return 0, false
		}
		return approximateAlphaValue(e.Alpha)
	case drawing.FillOverlay:
		return approximateFillOpacity(e.Fill)
	case *drawing.FillOverlay:
		if e == nil {
			return 0, false
		}
		return approximateFillOpacity(e.Fill)
	default:
		return 0, false
	}
}

func approximateAlphaValue(alpha float64) (float64, bool) {
	if alpha <= 0 {
		return 0, false
	}
	return clampFloat(alpha, 0, 1), true
}

func approximateFillOpacity(fill drawing.Fill) (float64, bool) {
	switch f := fill.(type) {
	case drawing.SolidFill:
		return f.ResolvedAlpha(), true
	case *drawing.SolidFill:
		if f == nil {
			return 0, false
		}
		return f.ResolvedAlpha(), true
	case drawing.GradientFill:
		if len(f.Stops) == 0 {
			return 0, false
		}
		total := 0.0
		for _, stop := range f.Stops {
			alpha := stop.Alpha
			if alpha <= 0 {
				alpha = 1
			}
			total += clampFloat(alpha, 0, 1)
		}
		return clampFloat(total/float64(len(f.Stops)), 0, 1), true
	case *drawing.GradientFill:
		if f == nil {
			return 0, false
		}
		return approximateFillOpacity(*f)
	case drawing.PatternFill:
		return clampFloat(math.Max(f.ForeAlpha, f.BackAlpha), 0, 1), true
	case *drawing.PatternFill:
		if f == nil {
			return 0, false
		}
		return approximateFillOpacity(*f)
	case drawing.PictureFill:
		return approximateAlphaValue(f.Alpha)
	case *drawing.PictureFill:
		if f == nil {
			return 0, false
		}
		return approximateAlphaValue(f.Alpha)
	default:
		if fill == nil {
			return 0, false
		}
		if props := Fill(fill); props != nil {
			if rawOpacity, ok := props["opacity"]; ok {
				return parseOpacity(rawOpacity)
			}
		}
		return 0, false
	}
}

func applyOpacityEffect(effect drawing.Effect, current float64) (float64, bool) {
	switch e := effect.(type) {
	case drawing.AlphaMod:
		return clampFloat(current*(float64(e.Amount)/100000.0), 0, 1), true
	case *drawing.AlphaMod:
		if e == nil {
			return 0, false
		}
		return clampFloat(current*(float64(e.Amount)/100000.0), 0, 1), true
	case drawing.AlphaModFix:
		return clampFloat(current*(float64(e.Amount)/100000.0), 0, 1), true
	case *drawing.AlphaModFix:
		if e == nil {
			return 0, false
		}
		return clampFloat(current*(float64(e.Amount)/100000.0), 0, 1), true
	case drawing.AlphaRepl:
		return clampFloat(float64(e.Alpha)/100000.0, 0, 1), true
	case *drawing.AlphaRepl:
		if e == nil {
			return 0, false
		}
		return clampFloat(float64(e.Alpha)/100000.0, 0, 1), true
	case drawing.AlphaOff:
		return clampFloat(current+(float64(e.Amount)/100000.0), 0, 1), true
	case *drawing.AlphaOff:
		if e == nil {
			return 0, false
		}
		return clampFloat(current+(float64(e.Amount)/100000.0), 0, 1), true
	case drawing.AlphaBiLevel:
		threshold := clampFloat(float64(e.Threshold)/100000.0, 0, 1)
		if current >= threshold {
			return 1, true
		}
		return 0, true
	case *drawing.AlphaBiLevel:
		if e == nil {
			return 0, false
		}
		threshold := clampFloat(float64(e.Threshold)/100000.0, 0, 1)
		if current >= threshold {
			return 1, true
		}
		return 0, true
	case drawing.AlphaInv:
		return clampFloat(1-current, 0, 1), true
	case *drawing.AlphaInv:
		return clampFloat(1-current, 0, 1), true
	case drawing.AlphaCeiling:
		return clampFloat(math.Ceil(current), 0, 1), true
	case *drawing.AlphaCeiling:
		return clampFloat(math.Ceil(current), 0, 1), true
	case drawing.AlphaFloor:
		return clampFloat(math.Floor(current), 0, 1), true
	case *drawing.AlphaFloor:
		return clampFloat(math.Floor(current), 0, 1), true
	default:
		return 0, false
	}
}
