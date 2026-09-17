package drawingcss

import (
	"bytes"
	"fmt"
	"image"
	"math"
	"sort"
	"strconv"
	"strings"

	"office/pkg/ooxml/drawing"
)

// NeedsSVGDefinition reports whether the fill must be emitted via an SVG defs
// paint server rather than a direct fill attribute.
func NeedsSVGDefinition(fill drawing.Fill) bool {
	switch f := fill.(type) {
	case drawing.GradientFill:
		return len(f.Stops) > 1
	case *drawing.GradientFill:
		return f != nil && len(f.Stops) > 1
	case drawing.PatternFill, *drawing.PatternFill:
		return true
	case drawing.PictureFill:
		return f.SourceURL() != ""
	case *drawing.PictureFill:
		return f != nil && f.SourceURL() != ""
	default:
		return false
	}
}

// SVGDefinition returns SVG defs markup for fills that require paint servers.
// The returned markup is a single gradient or pattern element without the outer
// <defs> wrapper.
func SVGDefinition(fill drawing.Fill, id string) string {
	if strings.TrimSpace(id) == "" {
		return ""
	}
	switch f := fill.(type) {
	case drawing.GradientFill:
		return gradientSVGDefinition(f, id)
	case *drawing.GradientFill:
		if f == nil {
			return ""
		}
		return gradientSVGDefinition(*f, id)
	case drawing.PatternFill:
		return patternSVGDefinition(f, id)
	case *drawing.PatternFill:
		if f == nil {
			return ""
		}
		return patternSVGDefinition(*f, id)
	case drawing.PictureFill:
		return pictureSVGDefinition(f, id)
	case *drawing.PictureFill:
		if f == nil {
			return ""
		}
		return pictureSVGDefinition(*f, id)
	default:
		return ""
	}
}

func gradientSVGDefinition(fill drawing.GradientFill, id string) string {
	if len(fill.Stops) == 0 {
		return ""
	}
	if len(fill.Stops) == 1 {
		return ""
	}
	var b strings.Builder
	switch fill.GradientType {
	case drawing.GradientTypeRadial, drawing.GradientTypePath:
		cx, cy := 0.5, 0.5
		if fill.FillToRect != nil {
			cx = clampUnit(fill.FillToRect.X + fill.FillToRect.Width/2.0)
			cy = clampUnit(fill.FillToRect.Y + fill.FillToRect.Height/2.0)
		}
		// DrawingML path gradients use fillToRect to position the focus. A radius
		// spanning half the bounding-box diagonal avoids clipping the authored
		// gradient prematurely when that focus is intentionally off-centre.
		fmt.Fprintf(&b, `<radialGradient id=%q cx="%.4f%%" cy="%.4f%%" r="70.7107%%">`, id, cx*100.0, cy*100.0)
	default:
		rad := fill.Angle * math.Pi / 180.0
		dx := math.Cos(rad)
		dy := math.Sin(rad)
		x1 := clampUnit(0.5 - dx/2.0)
		y1 := clampUnit(0.5 - dy/2.0)
		x2 := clampUnit(0.5 + dx/2.0)
		y2 := clampUnit(0.5 + dy/2.0)
		fmt.Fprintf(&b, `<linearGradient id=%q x1="%.4f" y1="%.4f" x2="%.4f" y2="%.4f">`, id, x1, y1, x2, y2)
	}
	for _, stop := range fill.Stops {
		color := stop.Color
		if color == "" {
			color = "000000"
		}
		fmt.Fprintf(&b, `<stop offset="%.2f%%" stop-color="#%s"`, clampUnit(stop.Position)*100.0, color)
		if stop.Alpha > 0 && stop.Alpha < 1.0 {
			fmt.Fprintf(&b, ` stop-opacity="%.4f"`, stop.Alpha)
		}
		b.WriteString(`/>`)
	}
	if fill.GradientType == drawing.GradientTypeRadial || fill.GradientType == drawing.GradientTypePath {
		b.WriteString(`</radialGradient>`)
	} else {
		b.WriteString(`</linearGradient>`)
	}
	return b.String()
}

func patternSVGDefinition(fill drawing.PatternFill, id string) string {
	tileW, tileH := patternTileSize(fill.PatternType)
	fg := normalizedHex(fill.Foreground)
	bg := normalizedHex(fill.Background)
	if fg == "" && bg == "" {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<pattern id=%q patternUnits="userSpaceOnUse" width="%.0f" height="%.0f">`, id, tileW, tileH)
	if bg != "" {
		fmt.Fprintf(&b, `<rect x="0" y="0" width="%.0f" height="%.0f" fill="#%s"`, tileW, tileH, bg)
		if fill.BackAlpha > 0 && fill.BackAlpha < 1.0 {
			fmt.Fprintf(&b, ` fill-opacity="%.4f"`, fill.BackAlpha)
		}
		b.WriteString(`/>`)
	}
	writePatternForeground(&b, strings.ToLower(string(fill.PatternType)), fg, fill.ForeAlpha, tileW, tileH)
	b.WriteString(`</pattern>`)
	return b.String()
}

func pictureSVGDefinition(fill drawing.PictureFill, id string) string {
	source := fill.SourceURL()
	if source == "" {
		return ""
	}
	cropX, cropY, cropW, cropH := cropRect(fill.Crop)
	imageOpacity, imageStyle := pictureSVGImageEffects(fill)

	if !fill.Tile {
		// Stretch mode: single image fills the entire shape
		imageW := 100.0 / cropW
		imageH := 100.0 / cropH
		imageX := -(cropX / cropW) * 100.0
		imageY := -(cropY / cropH) * 100.0
		var b strings.Builder
		fmt.Fprintf(&b, `<pattern id=%q patternUnits="objectBoundingBox" width="1" height="1">`, id)
		fmt.Fprintf(&b, `<image href=%q x="%.4f%%" y="%.4f%%" width="%.4f%%" height="%.4f%%" preserveAspectRatio="none"`, source, imageX, imageY, imageW, imageH)
		if imageOpacity > 0 && imageOpacity < 1.0 {
			fmt.Fprintf(&b, ` opacity="%.4f"`, imageOpacity)
		}
		if imageStyle != "" {
			fmt.Fprintf(&b, ` style=%q`, imageStyle)
		}
		b.WriteString(`/>`)
		b.WriteString(`</pattern>`)
		return b.String()
	}

	// Tile mode: repeat the image at its natural (or scaled) size
	imgW, imgH := 100, 100
	if len(fill.ImageData) > 0 {
		if cfg, _, err := image.DecodeConfig(bytes.NewReader(fill.ImageData)); err == nil {
			imgW = cfg.Width
			imgH = cfg.Height
		}
	}

	sx := float64(fill.TileScaleX) / 100000.0
	sy := float64(fill.TileScaleY) / 100000.0
	if sx <= 0 {
		sx = 1.0
	}
	if sy <= 0 {
		sy = 1.0
	}

	tileW := float64(imgW) * sx
	tileH := float64(imgH) * sy

	// Convert EMU offsets to points (1 pt = 12700 EMU)
	offsetX := float64(fill.TileOffsetX) / 12700.0
	offsetY := float64(fill.TileOffsetY) / 12700.0

	var b strings.Builder
	fmt.Fprintf(&b, `<pattern id=%q patternUnits="userSpaceOnUse" width="%.4f" height="%.4f" x="%.4f" y="%.4f"`, id, tileW, tileH, offsetX, offsetY)

	// Handle flip via patternTransform
	flipTransform := ""
	switch fill.TileFlip {
	case "x":
		flipTransform = fmt.Sprintf(` patternTransform="translate(%.4f, 0) scale(-1, 1)"`, tileW)
	case "y":
		flipTransform = fmt.Sprintf(` patternTransform="translate(0, %.4f) scale(1, -1)"`, tileH)
	case "xy":
		flipTransform = fmt.Sprintf(` patternTransform="translate(%.4f, %.4f) scale(-1, -1)"`, tileW, tileH)
	}
	b.WriteString(flipTransform)
	b.WriteString(`>`)

	// The image inside the pattern fills the tile at the (possibly cropped) size
	innerW := tileW / cropW
	innerH := tileH / cropH
	innerX := -(cropX / cropW) * tileW
	innerY := -(cropY / cropH) * tileH
	fmt.Fprintf(&b, `<image href=%q x="%.4f" y="%.4f" width="%.4f" height="%.4f" preserveAspectRatio="none"`, source, innerX, innerY, innerW, innerH)
	if imageOpacity > 0 && imageOpacity < 1.0 {
		fmt.Fprintf(&b, ` opacity="%.4f"`, imageOpacity)
	}
	if imageStyle != "" {
		fmt.Fprintf(&b, ` style=%q`, imageStyle)
	}
	b.WriteString(`/>`)
	b.WriteString(`</pattern>`)
	return b.String()
}

func pictureSVGImageEffects(fill drawing.PictureFill) (float64, string) {
	alpha := fill.Alpha
	if alpha <= 0 || alpha > 1.0 {
		alpha = 1.0
	}
	if len(fill.BlipEffects) == 0 {
		return alpha, ""
	}
	props := EffectList(drawing.EffectList{Effects: fill.BlipEffects})
	if len(props) == 0 {
		return alpha, ""
	}
	if rawOpacity, ok := props["opacity"]; ok {
		if effectOpacity, err := strconv.ParseFloat(strings.TrimSpace(rawOpacity), 64); err == nil && effectOpacity >= 0 {
			alpha *= effectOpacity
		}
		delete(props, "opacity")
	}
	if len(props) == 0 {
		return alpha, ""
	}
	keys := make([]string, 0, len(props))
	for key := range props {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	styles := make([]string, 0, len(keys))
	for _, key := range keys {
		styles = append(styles, key+":"+props[key])
	}
	return alpha, strings.Join(styles, ";")
}

func writePatternForeground(b *strings.Builder, pattern, fg string, alpha, tileW, tileH float64) {
	if b == nil || fg == "" {
		return
	}
	alphaAttr := ""
	if alpha > 0 && alpha < 1.0 {
		alphaAttr = fmt.Sprintf(` opacity="%.4f"`, alpha)
	}
	switch pattern {
	case "soliddiamond", "soliddmnd":
		fmt.Fprintf(b, `<polygon points="%.0f,0 %.0f,%.0f %.0f,%.0f 0,%.0f" fill="#%s"%s/>`, tileW/2, tileW, tileH/2, tileW/2, tileH, tileH/2, fg, alphaAttr)
	case "dkhorizontal", "dkhorz", "darkhorizontal":
		fmt.Fprintf(b, `<rect x="0" y="0" width="%.0f" height="2" fill="#%s"%s/>`, tileW, fg, alphaAttr)
		fmt.Fprintf(b, `<rect x="0" y="%.0f" width="%.0f" height="2" fill="#%s"%s/>`, tileH/2, tileW, fg, alphaAttr)
	case "dkvertical", "dkvert", "darkvertical":
		fmt.Fprintf(b, `<rect x="0" y="0" width="2" height="%.0f" fill="#%s"%s/>`, tileH, fg, alphaAttr)
		fmt.Fprintf(b, `<rect x="%.0f" y="0" width="2" height="%.0f" fill="#%s"%s/>`, tileW/2, tileH, fg, alphaAttr)
	case "dkdowndiagonal", "dkdndiag", "darkdowndiagonal":
		fmt.Fprintf(b, `<path d="M -2,2 L 2,-2 M 0,%.0f L %.0f,0 M %.0f,%.0f L %.0f,%.0f" stroke="#%s" stroke-width="2"%s fill="none"/>`, tileH, tileW, tileW-2, tileH+2, tileW+2, tileH-2, fg, alphaAttr)
	case "dkupdiagonal", "dkupdiag", "darkupdiagonal":
		fmt.Fprintf(b, `<path d="M -2,%.0f L 2,%.0f M 0,0 L %.0f,%.0f M %.0f,-2 L %.0f,2" stroke="#%s" stroke-width="2"%s fill="none"/>`, tileH-2, tileH+2, tileW, tileH, tileW-2, tileW+2, fg, alphaAttr)
	case "smgrid", "smallgrid":
		fmt.Fprintf(b, `<path d="M 0 0 H %.0f M 0 %.0f H %.0f M 0 0 V %.0f M %.0f 0 V %.0f" stroke="#%s" stroke-width="1"%s fill="none"/>`, tileW, tileH/2, tileW, tileH, tileW/2, tileH, fg, alphaAttr)
	case "lggrid", "largegrid":
		fmt.Fprintf(b, `<path d="M 0 0 H %.0f M 0 0 V %.0f" stroke="#%s" stroke-width="1.5"%s fill="none"/>`, tileW, tileH, fg, alphaAttr)
	case "dashhorizontal", "dashhorz", "dashedhorizontal":
		fmt.Fprintf(b, `<path d="M 0 1 H %.0f M 0 %.0f H %.0f" stroke="#%s" stroke-width="1.5" stroke-dasharray="4 3"%s fill="none"/>`, tileW, tileH/2+1, tileW, fg, alphaAttr)
	case "dashvertical", "dashvert", "dashedvertical":
		fmt.Fprintf(b, `<path d="M 1 0 V %.0f M %.0f 0 V %.0f" stroke="#%s" stroke-width="1.5" stroke-dasharray="4 3"%s fill="none"/>`, tileH, tileW/2+1, tileH, fg, alphaAttr)
	// Light horizontal — single thin line per tile
	case "lthorizontal", "lthorz", "lighthorizontal":
		fmt.Fprintf(b, `<rect x="0" y="0" width="%.0f" height="1" fill="#%s"%s/>`, tileW, fg, alphaAttr)
	// Light vertical — single thin column per tile
	case "ltvertical", "ltvert", "lightvertical":
		fmt.Fprintf(b, `<rect x="0" y="0" width="1" height="%.0f" fill="#%s"%s/>`, tileH, fg, alphaAttr)
	// Light down-diagonal — single thin stroke
	case "ltdowndiagonal", "ltdndiag", "lightdowndiagonal":
		fmt.Fprintf(b, `<path d="M 0,%.0f L %.0f,0" stroke="#%s" stroke-width="1"%s fill="none"/>`, tileH, tileW, fg, alphaAttr)
	// Light up-diagonal
	case "ltupdiagonal", "ltupdiag", "lightupdiagonal":
		fmt.Fprintf(b, `<path d="M 0,0 L %.0f,%.0f" stroke="#%s" stroke-width="1"%s fill="none"/>`, tileW, tileH, fg, alphaAttr)
	// Open (outline) diamond
	case "opendiamond", "opendmnd":
		fmt.Fprintf(b, `<polygon points="%.0f,0 %.0f,%.0f %.0f,%.0f 0,%.0f" fill="none" stroke="#%s" stroke-width="1"%s/>`, tileW/2, tileW, tileH/2, tileW/2, tileH, tileH/2, fg, alphaAttr)
	// Large checker board — 2×2 alternating squares
	case "lgcheckerboard", "lgcheck", "largecheckerboard":
		h := tileH / 2
		w := tileW / 2
		fmt.Fprintf(b, `<rect x="0" y="0" width="%.0f" height="%.0f" fill="#%s"%s/>`, w, h, fg, alphaAttr)
		fmt.Fprintf(b, `<rect x="%.0f" y="%.0f" width="%.0f" height="%.0f" fill="#%s"%s/>`, w, h, w, h, fg, alphaAttr)
	// Small checker board
	case "smcheckerboard", "smcheck", "smallcheckerboard":
		h := tileH / 4
		w := tileW / 4
		for row := 0; row < 4; row++ {
			for col := 0; col < 4; col++ {
				if (row+col)%2 == 0 {
					fmt.Fprintf(b, `<rect x="%.0f" y="%.0f" width="%.0f" height="%.0f" fill="#%s"%s/>`, float64(col)*w, float64(row)*h, w, h, fg, alphaAttr)
				}
			}
		}
	// Diagonal brick — diagonal lines offset every half tile
	case "diagbrick", "diagonalbrick":
		fmt.Fprintf(b, `<path d="M 0,%.0f L %.0f,0 M -%.0f,%.0f L %.0f,%.0f" stroke="#%s" stroke-width="1"%s fill="none"/>`, tileH, tileW, tileW/4, tileH*0.75, tileW*1.25, -tileH*0.25, fg, alphaAttr)
	// Horizontal brick — rectangular brick pattern
	case "horzbrk", "horizontalbrick":
		hw := tileW / 2
		hh := tileH / 2
		fmt.Fprintf(b, `<rect x="0" y="0" width="%.0f" height="%.0f" fill="none" stroke="#%s" stroke-width="1"%s/>`, tileW, hh, fg, alphaAttr)
		fmt.Fprintf(b, `<line x1="%.0f" y1="0" x2="%.0f" y2="%.0f" stroke="#%s" stroke-width="1"%s/>`, hw, hw, hh, fg, alphaAttr)
		fmt.Fprintf(b, `<rect x="0" y="%.0f" width="%.0f" height="%.0f" fill="none" stroke="#%s" stroke-width="1"%s/>`, hh, tileW, hh, fg, alphaAttr)
	// Cross — plus sign
	case "cross":
		fmt.Fprintf(b, `<path d="M %.0f 0 V %.0f M 0 %.0f H %.0f" stroke="#%s" stroke-width="1.5"%s fill="none"/>`, tileW/2, tileH, tileH/2, tileW, fg, alphaAttr)
	// Diagonal cross — X
	case "diagcross", "diagonalcross":
		fmt.Fprintf(b, `<path d="M 0,0 L %.0f,%.0f M %.0f,0 L 0,%.0f" stroke="#%s" stroke-width="1"%s fill="none"/>`, tileW, tileH, tileW, tileH, fg, alphaAttr)
	// Trellis — diagonal cross with fill
	case "trellis", "dktrellis":
		fmt.Fprintf(b, `<path d="M 0,0 L %.0f,%.0f M %.0f,0 L 0,%.0f" stroke="#%s" stroke-width="2"%s fill="none"/>`, tileW, tileH, tileW, tileH, fg, alphaAttr)
	// Dotted grid — dots at intersections
	case "dottedgrid", "dotgrid":
		r := 1.0
		fmt.Fprintf(b, `<circle cx="0" cy="0" r="%.1f" fill="#%s"%s/>`, r, fg, alphaAttr)
		fmt.Fprintf(b, `<circle cx="%.0f" cy="0" r="%.1f" fill="#%s"%s/>`, tileW, r, fg, alphaAttr)
		fmt.Fprintf(b, `<circle cx="0" cy="%.0f" r="%.1f" fill="#%s"%s/>`, tileH, r, fg, alphaAttr)
		fmt.Fprintf(b, `<circle cx="%.0f" cy="%.0f" r="%.1f" fill="#%s"%s/>`, tileW/2, tileH/2, r, fg, alphaAttr)
	// Dotted diamond — dots in diamond arrangement
	case "dotteddiamond", "dotdmnd":
		r := 1.0
		fmt.Fprintf(b, `<circle cx="%.0f" cy="0" r="%.1f" fill="#%s"%s/>`, tileW/2, r, fg, alphaAttr)
		fmt.Fprintf(b, `<circle cx="0" cy="%.0f" r="%.1f" fill="#%s"%s/>`, tileH/2, r, fg, alphaAttr)
		fmt.Fprintf(b, `<circle cx="%.0f" cy="%.0f" r="%.1f" fill="#%s"%s/>`, tileW, tileH/2, r, fg, alphaAttr)
		fmt.Fprintf(b, `<circle cx="%.0f" cy="%.0f" r="%.1f" fill="#%s"%s/>`, tileW/2, tileH, r, fg, alphaAttr)
	// Wave — sinusoidal stroke
	case "wave":
		fmt.Fprintf(b, `<path d="M 0,%.0f Q %.0f,0 %.0f,%.0f T %.0f,%.0f" stroke="#%s" stroke-width="1"%s fill="none"/>`, tileH/2, tileW/4, tileW/2, tileH/2, tileW, tileH/2, fg, alphaAttr)
	// ZigZag — V-shape pattern
	case "zigzag":
		fmt.Fprintf(b, `<path d="M 0,%.0f L %.0f,0 L %.0f,%.0f" stroke="#%s" stroke-width="1"%s fill="none"/>`, tileH, tileW/2, tileW, tileH, fg, alphaAttr)
	// Narrow upward diagonal
	case "narwupdiag", "narrowupwarddiagonal":
		fmt.Fprintf(b, `<path d="M 0,%.0f L %.0f,0 M %.0f,%.0f L %.0f,%.0f" stroke="#%s" stroke-width="1"%s fill="none"/>`, tileH, tileW/2, tileW/2, tileH, tileW, tileH/2, fg, alphaAttr)
	// Narrow downward diagonal
	case "narwdowndiag", "narrowdownwarddiagonal":
		fmt.Fprintf(b, `<path d="M 0,0 L %.0f,%.0f M %.0f,0 L %.0f,%.0f" stroke="#%s" stroke-width="1"%s fill="none"/>`, tileW/2, tileH, tileW/2, tileW, tileH, fg, alphaAttr)
	// Wide upward diagonal
	case "wideupdiag", "wideupwarddiagonal":
		fmt.Fprintf(b, `<path d="M -%.0f,%.0f L %.0f,0" stroke="#%s" stroke-width="3"%s fill="none"/>`, tileW/4, tileH, tileW, fg, alphaAttr)
	// Wide downward diagonal
	case "widedowndiag", "widedownwarddiagonal":
		fmt.Fprintf(b, `<path d="M -%.0f,0 L %.0f,%.0f" stroke="#%s" stroke-width="3"%s fill="none"/>`, tileW/4, tileW, tileH, fg, alphaAttr)
	// Dashed up diagonal
	case "dashupdiag", "dashedupdiagonal":
		fmt.Fprintf(b, `<path d="M 0,%.0f L %.0f,0" stroke="#%s" stroke-width="1" stroke-dasharray="3 2"%s fill="none"/>`, tileH, tileW, fg, alphaAttr)
	// Dashed down diagonal
	case "dashdowndiag", "dasheddowndiagonal":
		fmt.Fprintf(b, `<path d="M 0,0 L %.0f,%.0f" stroke="#%s" stroke-width="1" stroke-dasharray="3 2"%s fill="none"/>`, tileW, tileH, fg, alphaAttr)
	// Divot — small V notches
	case "divot":
		hw := tileW / 2
		qh := tileH / 4
		fmt.Fprintf(b, `<path d="M %.0f,0 L %.0f,%.0f L %.0f,0" stroke="#%s" stroke-width="1"%s fill="none"/>`, hw/2, hw, qh, hw*1.5, fg, alphaAttr)
		fmt.Fprintf(b, `<path d="M %.0f,%.0f L %.0f,%.0f L %.0f,%.0f" stroke="#%s" stroke-width="1"%s fill="none"/>`, 0.0, tileH-qh, hw/2, tileH, tileW, tileH-qh, fg, alphaAttr)
	// Shingle — diagonal curved tiles
	case "shingle":
		fmt.Fprintf(b, `<path d="M 0,%.0f Q %.0f,0 %.0f,0" stroke="#%s" stroke-width="1"%s fill="none"/>`, tileH, tileW/2, tileW, fg, alphaAttr)
	// Sphere — circle per tile
	case "sphere":
		r := math.Min(tileW, tileH) / 2 * 0.8
		fmt.Fprintf(b, `<circle cx="%.1f" cy="%.1f" r="%.1f" fill="none" stroke="#%s" stroke-width="1"%s/>`, tileW/2, tileH/2, r, fg, alphaAttr)
	// Plaid (coarse checker with inner grid)
	case "plaid":
		hw := tileW / 2
		hh := tileH / 2
		fmt.Fprintf(b, `<rect x="0" y="0" width="%.0f" height="%.0f" fill="#%s"%s/>`, hw, hh, fg, alphaAttr)
		fmt.Fprintf(b, `<rect x="%.0f" y="%.0f" width="%.0f" height="%.0f" fill="#%s"%s/>`, hw, hh, hw, hh, fg, alphaAttr)
		fmt.Fprintf(b, `<path d="M 0 0 H %.0f M 0 %.0f H %.0f" stroke="#%s" stroke-width="0.5" opacity="0.4" fill="none"/>`, tileW, tileH, tileW, fg)
	// Large confetti — scattered triangles
	case "lgconfetti", "largeconfetti":
		fmt.Fprintf(b, `<polygon points="1,1 %.0f,1 1,%.0f" fill="#%s"%s/>`, tileW/3, tileH/3, fg, alphaAttr)
		fmt.Fprintf(b, `<polygon points="%.0f,%.0f %.0f,%.0f %.0f,%.0f" fill="#%s"%s/>`, tileW*0.6, tileH*0.5, tileW, tileH*0.5, tileW*0.8, tileH, fg, alphaAttr)
	// Small confetti — tiny scattered dots
	case "smconfetti", "smallconfetti":
		r := 1.2
		fmt.Fprintf(b, `<circle cx="%.0f" cy="%.0f" r="%.1f" fill="#%s"%s/>`, tileW*0.2, tileH*0.2, r, fg, alphaAttr)
		fmt.Fprintf(b, `<circle cx="%.0f" cy="%.0f" r="%.1f" fill="#%s"%s/>`, tileW*0.7, tileH*0.6, r, fg, alphaAttr)
		fmt.Fprintf(b, `<circle cx="%.0f" cy="%.0f" r="%.1f" fill="#%s"%s/>`, tileW*0.4, tileH*0.85, r, fg, alphaAttr)
	// Solid fill
	case "solid":
		fmt.Fprintf(b, `<rect x="0" y="0" width="%.0f" height="%.0f" fill="#%s"%s/>`, tileW, tileH, fg, alphaAttr)
	default:
		if ratio, ok := patternCoverageRatio(pattern); ok {
			fmt.Fprintf(b, `<rect x="0" y="0" width="%.0f" height="%.0f" fill="#%s" opacity="%.4f"/>`, tileW, tileH, fg, ratio*alphaOrOne(alpha))
		}
		// For unknown patterns, fall through with no foreground drawing;
		// the background tile (already emitted) provides the base color.
	}
}

func patternTileSize(pattern drawing.PatternType) (float64, float64) {
	switch strings.ToLower(string(pattern)) {
	case "lggrid", "largegrid":
		return 12, 12
	case "lgcheckerboard", "lgcheck", "largecheckerboard":
		return 16, 16
	case "smcheckerboard", "smcheck", "smallcheckerboard":
		return 16, 16
	case "plaid":
		return 16, 16
	case "horzbrk", "horizontalbrick":
		return 16, 8
	case "diagbrick", "diagonalbrick":
		return 16, 16
	case "sphere":
		return 12, 12
	case "wave", "zigzag":
		return 16, 8
	case "shingle", "divot":
		return 16, 8
	case "trellis", "dktrellis":
		return 8, 8
	default:
		return 8, 8
	}
}

func patternCoverageRatio(pattern string) (float64, bool) {
	if !strings.HasPrefix(pattern, "pct") {
		return 0, false
	}
	value, err := strconv.Atoi(strings.TrimPrefix(pattern, "pct"))
	if err != nil {
		return 0, false
	}
	return clampUnit(float64(value) / 100.0), true
}
