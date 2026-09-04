package drawingcss

import (
	"fmt"
	"math"
	"strings"

	"office/pkg/ooxml/drawing"
)

// SVGOverlay describes an SVG paint overlay plus any required defs entry.
type SVGOverlay struct {
	Fill      drawing.Fill
	BlendMode string
	DefID     string
	Def       string
}

// ThreeDLightOverlays synthesizes highlight and shade overlays from a 3D shape
// effect so HTML/SVG renderers can represent scene lighting explicitly.
func ThreeDLightOverlays(threeD *drawing.ThreeD) []drawing.FillOverlay {
	if threeD == nil {
		return nil
	}
	if threeD.ExtrusionDepth <= 0 && threeD.ContourWidth <= 0 && threeD.BevelTopHeight <= 0 && threeD.BevelBottomHeight <= 0 && threeD.Material == "" {
		return nil
	}
	lightAngle := threeDLightAngle(threeD.LightDirection)
	highlightAlpha, shadeAlpha := threeDLightOpacities(*threeD)
	highlightColor := strings.TrimSpace(threeD.ContourColor)
	if highlightColor == "" {
		highlightColor = "FFFFFF"
	}
	shadeColor := strings.TrimSpace(threeD.ExtrusionColor)
	if shadeColor == "" {
		shadeColor = "000000"
	}
	return []drawing.FillOverlay{
		{
			BlendMode: "screen",
			Fill: drawing.GradientFill{
				GradientType: drawing.GradientTypeLinear,
				Angle:        lightAngle,
				Stops: []drawing.GradientStop{
					{Position: 0.0, Color: highlightColor, Alpha: highlightAlpha},
					{Position: 0.35, Color: highlightColor, Alpha: highlightAlpha * 0.35},
					{Position: 1.0, Color: highlightColor, Alpha: 0},
				},
			},
		},
		{
			BlendMode: "mult",
			Fill: drawing.GradientFill{
				GradientType: drawing.GradientTypeLinear,
				Angle:        math.Mod(lightAngle+180.0, 360.0),
				Stops: []drawing.GradientStop{
					{Position: 0.0, Color: shadeColor, Alpha: shadeAlpha},
					{Position: 0.45, Color: shadeColor, Alpha: shadeAlpha * 0.4},
					{Position: 1.0, Color: shadeColor, Alpha: 0},
				},
			},
		},
	}
}

// BuildSVGOverlays resolves defs metadata for SVG overlays.
func BuildSVGOverlays(overlays []drawing.FillOverlay, defPrefix string) []SVGOverlay {
	renders := make([]SVGOverlay, 0, len(overlays))
	for i, overlay := range overlays {
		if overlay.Fill == nil {
			continue
		}
		rendered := SVGOverlay{
			Fill:      overlay.Fill,
			BlendMode: overlay.BlendMode,
		}
		if NeedsSVGDefinition(overlay.Fill) {
			rendered.DefID = fmt.Sprintf("%s-%d", defPrefix, i)
			rendered.Def = SVGDefinition(overlay.Fill, rendered.DefID)
		}
		renders = append(renders, rendered)
	}
	return renders
}

// SVGOverlayBlendStyle maps a DrawingML blend mode onto an SVG group style.
func SVGOverlayBlendStyle(mode string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "", "over":
		return ""
	case "screen":
		return "mix-blend-mode:screen"
	case "mult":
		return "mix-blend-mode:multiply"
	case "darken":
		return "mix-blend-mode:darken"
	case "lighten":
		return "mix-blend-mode:lighten"
	default:
		return ""
	}
}

func threeDLightAngle(direction string) float64 {
	switch strings.ToLower(strings.TrimSpace(direction)) {
	case "t":
		return 90
	case "tr":
		return 135
	case "r":
		return 180
	case "br":
		return 225
	case "b":
		return 270
	case "bl":
		return 315
	case "l":
		return 0
	case "tl":
		return 45
	default:
		return 45
	}
}

func threeDLightOpacities(threeD drawing.ThreeD) (highlight, shade float64) {
	highlight = 0.18
	shade = 0.14
	switch strings.ToLower(strings.TrimSpace(threeD.Material)) {
	case "metal", "metal2", "legacymetal":
		highlight = 0.30
		shade = 0.18
	case "matte", "plastic":
		highlight = 0.20
		shade = 0.12
	}
	switch strings.ToLower(strings.TrimSpace(threeD.LightRig)) {
	case "threept":
		highlight += 0.06
		shade += 0.03
	case "balanced", "soft":
		highlight += 0.03
	}
	if threeD.ContourWidth > 0 || threeD.BevelTopHeight > 0 || threeD.BevelBottomHeight > 0 {
		highlight += 0.02
	}
	if threeD.ExtrusionDepth > 0 {
		shade += 0.02
	}
	if highlight > 0.45 {
		highlight = 0.45
	}
	if shade > 0.30 {
		shade = 0.30
	}
	return highlight, shade
}
