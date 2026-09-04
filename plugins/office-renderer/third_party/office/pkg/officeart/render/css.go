package render

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"office/pkg/officeart/ir"
)

const emuPerPoint = 12700.0

func pt(emu int64) float64 {
	return float64(emu) / emuPerPoint
}

func ShapeStyle(shape *ir.Shape) map[string]string {
	w := pt(shape.Transform.Rect.W)
	h := pt(shape.Transform.Rect.H)
	if shape.Kind == ir.ShapeConnector {
		if w == 0 {
			w = 1
		}
		if h == 0 {
			h = 1
		}
	}
	style := map[string]string{
		"position":   "absolute",
		"left":       fmt.Sprintf("%.4fpt", pt(shape.Transform.Rect.X)),
		"top":        fmt.Sprintf("%.4fpt", pt(shape.Transform.Rect.Y)),
		"width":      fmt.Sprintf("%.4fpt", w),
		"height":     fmt.Sprintf("%.4fpt", h),
		"box-sizing": "border-box",
	}

	var transforms []string
	if shape.Transform.RotationDeg != 0 {
		transforms = append(transforms, fmt.Sprintf("rotate(%gdeg)", shape.Transform.RotationDeg))
	}
	if shape.Transform.FlipH {
		transforms = append(transforms, "scaleX(-1)")
	}
	if shape.Transform.FlipV {
		transforms = append(transforms, "scaleY(-1)")
	}
	if len(transforms) > 0 {
		style["transform"] = strings.Join(transforms, " ")
		style["transform-origin"] = "center center"
	}

	vectorShape := shape.Kind == ir.ShapeConnector || len(shape.Geometry.Paths) > 0
	if !vectorShape {
		applyFill(style, shape.Fill)
		applyLine(style, shape.Line)
	}
	applyEffects(style, shape.Effects)
	if vectorShape {
		style["overflow"] = "visible"
	}
	if shape.Image != nil {
		crop := shape.Image.Crop
		if crop.Left != 0 || crop.Top != 0 || crop.Right != 0 || crop.Bottom != 0 {
			style["overflow"] = "hidden"
		}
	}
	return style
}

func StyleString(style map[string]string) string {
	if len(style) == 0 {
		return ""
	}
	keys := make([]string, 0, len(style))
	for k := range style {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte(':')
		b.WriteString(style[k])
		b.WriteByte(';')
	}
	return b.String()
}

func applyFill(style map[string]string, fill ir.Fill) {
	switch fill.Kind {
	case ir.FillNone:
		style["background"] = "transparent"
	case ir.FillSolid:
		if fill.Color != "" {
			style["background"] = cssColor(fill.Color, fill.Alpha)
		}
	case ir.FillGradient:
		if fill.Gradient != nil && len(fill.Gradient.Stops) > 0 {
			var stops []string
			for _, stop := range fill.Gradient.Stops {
				stops = append(stops, fmt.Sprintf("%s %.2f%%", cssColor(stop.Color, stop.Alpha), stop.Position*100))
			}
			style["background"] = fmt.Sprintf("linear-gradient(%gdeg,%s)", fill.Gradient.Angle, strings.Join(stops, ","))
		}
	case ir.FillPattern:
		if fill.Pattern != nil && fill.Pattern.Foreground != "" {
			style["background"] = cssColor(fill.Pattern.Foreground, 1)
		}
	case ir.FillPicture:
		if fill.Picture != nil && fill.Picture.Image.URL != "" {
			style["background-image"] = fmt.Sprintf("url(%q)", fill.Picture.Image.URL)
			style["background-size"] = "100% 100%"
		}
	}
}

func applyLine(style map[string]string, line ir.Line) {
	if !line.Visible {
		style["border"] = "none"
		return
	}
	width := line.WidthPt
	if width <= 0 {
		width = 1
	}
	style["border-style"] = "solid"
	style["border-width"] = fmt.Sprintf("%gpt", width)
	if line.Color != "" {
		style["border-color"] = cssColor(line.Color, line.Alpha)
	}
	if line.Dash != "" && line.Dash != "solid" {
		style["border-style"] = "dashed"
	}
}

func applyEffects(style map[string]string, effects ir.Effects) {
	if effects.Shadow == nil {
		return
	}
	s := effects.Shadow
	rad := s.AngleDeg * 3.141592653589793 / 180
	x := s.Distance * math.Cos(rad)
	y := s.Distance * math.Sin(rad)
	shadow := fmt.Sprintf("%gpt %gpt %gpt %s", x, y, s.BlurRadius, cssColor(s.Color, s.Alpha))
	if s.Inner {
		shadow += " inset"
	}
	style["box-shadow"] = shadow
}

func cssColor(hex string, alpha float64) string {
	hex = strings.TrimPrefix(hex, "#")
	if alpha <= 0 {
		alpha = 1
	}
	if alpha >= 0.999 {
		return "#" + hex
	}
	if len(hex) != 6 {
		return "#" + hex
	}
	var r, g, b int
	_, _ = fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	return fmt.Sprintf("rgba(%d,%d,%d,%.4f)", r, g, b, alpha)
}
