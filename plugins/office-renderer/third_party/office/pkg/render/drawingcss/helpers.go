package drawingcss

import (
	"fmt"
	"strconv"
	"strings"

	"office/pkg/ooxml/drawing"
)

// hexToRGB extracts a component from a hex color string.
func hexToRGB(hex string, component int) string {
	if len(hex) != 6 {
		return "0"
	}

	start := component * 2
	if start+2 > len(hex) {
		return "0"
	}

	// Parse hex byte
	val, err := strconv.ParseUint(hex[start:start+2], 16, 8)
	if err != nil {
		return "0"
	}

	return fmt.Sprintf("%d", val)
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampUnit(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func rgbComponents(hex string) string {
	return fmt.Sprintf("%s, %s, %s", hexToRGB(hex, 0), hexToRGB(hex, 1), hexToRGB(hex, 2))
}

func parseOpacity(raw string) (float64, bool) {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

// cssMapToString converts a CSS property map to a semicolon-separated string
// in deterministic (sorted) order.
func cssMapToString(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	// Collect keys in a fixed order for deterministic output.
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortStrings(keys)
	out := ""
	for i, k := range keys {
		if i > 0 {
			out += ";"
		}
		out += k + ":" + m[k]
	}
	return out
}

// htmlEscape escapes the five XML/HTML special characters.
func htmlEscape(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '&':
			out = append(out, '&', 'a', 'm', 'p', ';')
		case '<':
			out = append(out, '&', 'l', 't', ';')
		case '>':
			out = append(out, '&', 'g', 't', ';')
		case '"':
			out = append(out, '&', '#', '3', '4', ';')
		case '\'':
			out = append(out, '&', '#', '3', '9', ';')
		default:
			out = append(out, s[i])
		}
	}
	return string(out)
}

// sortStrings sorts a slice of strings in-place (insertion sort — small slices only).
func sortStrings(ss []string) {
	for i := 1; i < len(ss); i++ {
		for j := i; j > 0 && ss[j] < ss[j-1]; j-- {
			ss[j], ss[j-1] = ss[j-1], ss[j]
		}
	}
}

func cropRect(rect *drawing.Rect) (x, y, w, h float64) {
	x, y, w, h = 0, 0, 1, 1
	if rect == nil {
		return
	}
	x = clampUnit(rect.X)
	y = clampUnit(rect.Y)
	w = clampUnit(rect.Width)
	h = clampUnit(rect.Height)
	if w <= 0 {
		w = 1
	}
	if h <= 0 {
		h = 1
	}
	return
}

func normalizedHex(color string) string {
	color = strings.TrimSpace(strings.TrimPrefix(color, "#"))
	if len(color) != 6 {
		return ""
	}
	return color
}

func alphaOrOne(alpha float64) float64 {
	if alpha <= 0 {
		return 1
	}
	if alpha > 1 {
		return 1
	}
	return alpha
}
