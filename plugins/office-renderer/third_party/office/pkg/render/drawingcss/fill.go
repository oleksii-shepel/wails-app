package drawingcss

import (
	"bytes"
	"fmt"
	"image"
	"math"

	"office/pkg/ooxml/drawing"
)

// Fill returns CSS properties for a DrawingML fill.
func Fill(f drawing.Fill) map[string]string {
	switch fill := f.(type) {
	case drawing.NoFill:
		return NoFill(fill)
	case *drawing.NoFill:
		if fill == nil {
			return nil
		}
		return NoFill(*fill)
	case drawing.SolidFill:
		return SolidFill(fill)
	case *drawing.SolidFill:
		if fill == nil {
			return nil
		}
		return SolidFill(*fill)
	case drawing.GradientFill:
		return GradientFill(fill)
	case *drawing.GradientFill:
		if fill == nil {
			return nil
		}
		return GradientFill(*fill)
	case drawing.PatternFill:
		return PatternFill(fill)
	case *drawing.PatternFill:
		if fill == nil {
			return nil
		}
		return PatternFill(*fill)
	case drawing.PictureFill:
		return PictureFill(fill)
	case *drawing.PictureFill:
		if fill == nil {
			return nil
		}
		return PictureFill(*fill)
	default:
		return nil
	}
}

// NoFill returns CSS properties for no fill (transparent).
func NoFill(f drawing.NoFill) map[string]string {
	return map[string]string{
		"fill": "none",
	}
}

// SolidFill returns CSS properties for a solid color fill.
func SolidFill(f drawing.SolidFill) map[string]string {
	color := f.Color
	if color == "" && f.ThemeColor != "" {
		// In a real implementation, this would resolve theme colors
		color = "000000" // Default to black
	}

	// Apply color transforms (lumMod, lumOff, satMod, etc.)
	color = f.ApplyColorTransforms(color)

	// Apply transparency if alpha < 1.0
	alpha := f.ResolvedAlpha()
	if alpha < 1.0 {
		// Convert hex to rgba
		return map[string]string{
			"fill": fmt.Sprintf("rgba(%s, %s, %s, %.2f)",
				hexToRGB(color, 0),
				hexToRGB(color, 1),
				hexToRGB(color, 2),
				alpha),
		}
	}

	return map[string]string{
		"fill": "#" + color,
	}
}

// GradientFill returns CSS properties for a gradient fill.
func GradientFill(f drawing.GradientFill) map[string]string {
	// Generate CSS gradient
	if len(f.Stops) == 0 {
		return map[string]string{"fill": "none"}
	}

	if len(f.Stops) == 1 {
		return map[string]string{"fill": "#" + f.Stops[0].Color}
	}

	// Build gradient stops
	stops := ""
	for i, stop := range f.Stops {
		if i > 0 {
			stops += ", "
		}
		color := stop.Color
		if stop.Alpha < 1.0 {
			color = fmt.Sprintf("rgba(%s, %s, %s, %.2f)",
				hexToRGB(stop.Color, 0),
				hexToRGB(stop.Color, 1),
				hexToRGB(stop.Color, 2),
				stop.Alpha)
		} else {
			color = "#" + stop.Color
		}
		stops += fmt.Sprintf("%s %.1f%%", color, stop.Position*100)
	}

	var gradient string
	switch f.GradientType {
	case drawing.GradientTypeLinear:
		// Convert angle to CSS angle (CSS uses 0deg = up, DrawingML uses 0deg = right)
		cssAngle := math.Mod(90-f.Angle+360, 360)
		gradient = fmt.Sprintf("linear-gradient(%.1fdeg, %s)", cssAngle, stops)
	case drawing.GradientTypeRadial:
		gradient = fmt.Sprintf("radial-gradient(circle, %s)", stops)
	default:
		gradient = fmt.Sprintf("linear-gradient(0deg, %s)", stops)
	}

	return map[string]string{
		"fill": gradient,
	}
}

// PatternFill returns CSS properties for a pattern fill.
func PatternFill(f drawing.PatternFill) map[string]string {
	// Pattern fills are complex to implement in CSS
	// For now, use the foreground color as solid fill
	if f.Foreground != "" {
		return map[string]string{"fill": "#" + f.Foreground}
	}
	return map[string]string{"fill": "none"}
}

// PictureFill returns CSS properties for a picture/image fill.
func PictureFill(f drawing.PictureFill) map[string]string {
	if f.SourceURL() == "" {
		return map[string]string{"fill": "none"}
	}

	source := f.SourceURL()
	props := map[string]string{
		"fill": fmt.Sprintf("url('%s')", source),
	}

	if f.Tile {
		props["background-image"] = fmt.Sprintf("url('%s')", source)
		props["background-repeat"] = "repeat"

		sx := float64(f.TileScaleX) / 100000.0
		sy := float64(f.TileScaleY) / 100000.0
		if sx <= 0 {
			sx = 1.0
		}
		if sy <= 0 {
			sy = 1.0
		}

		imgW, imgH := 100, 100
		if len(f.ImageData) > 0 {
			if cfg, _, err := image.DecodeConfig(bytes.NewReader(f.ImageData)); err == nil {
				imgW = cfg.Width
				imgH = cfg.Height
			}
		}

		tileW := float64(imgW) * sx
		tileH := float64(imgH) * sy
		offsetX := float64(f.TileOffsetX) / 12700.0
		offsetY := float64(f.TileOffsetY) / 12700.0

		props["background-size"] = fmt.Sprintf("%.2fpx %.2fpx", tileW, tileH)
		props["background-position"] = fmt.Sprintf("%.2fpx %.2fpx", offsetX, offsetY)
	}

	if f.Alpha < 1.0 {
		props["opacity"] = fmt.Sprintf("%.2f", f.Alpha)
	}

	return props
}
