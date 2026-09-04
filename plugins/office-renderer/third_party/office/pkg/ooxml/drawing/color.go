package drawing

import (
	"fmt"
	"strconv"
	"strings"
)

// ColorScheme represents the 12 standard DrawingML theme color slots.
// ECMA-376 Part 1 §20.1.4.1.
type ColorScheme struct {
	Dk1      string // Dark 1 (text 1)
	Lt1      string // Light 1 (background 1)
	Dk2      string // Dark 2 (text 2)
	Lt2      string // Light 2 (background 2)
	Accent1  string
	Accent2  string
	Accent3  string
	Accent4  string
	Accent5  string
	Accent6  string
	Hlink    string // Hyperlink
	FolHlink string // Followed hyperlink
}

// Index returns a map from slot name to hex RGB value (e.g., "dk1" → "4472C4").
// Includes both primary names (dk1, lt1, ...) and common aliases (tx1, bg1, text1, background1).
func (cs *ColorScheme) Index() map[string]string {
	index := make(map[string]string)
	if cs == nil {
		return index
	}

	// Primary slots
	if cs.Dk1 != "" {
		index["dk1"] = cs.Dk1
		index["tx1"] = cs.Dk1
		index["text1"] = cs.Dk1
	}
	if cs.Lt1 != "" {
		index["lt1"] = cs.Lt1
		index["bg1"] = cs.Lt1
		index["background1"] = cs.Lt1
	}
	if cs.Dk2 != "" {
		index["dk2"] = cs.Dk2
		index["tx2"] = cs.Dk2
		index["text2"] = cs.Dk2
	}
	if cs.Lt2 != "" {
		index["lt2"] = cs.Lt2
		index["bg2"] = cs.Lt2
		index["background2"] = cs.Lt2
	}
	if cs.Accent1 != "" {
		index["accent1"] = cs.Accent1
	}
	if cs.Accent2 != "" {
		index["accent2"] = cs.Accent2
	}
	if cs.Accent3 != "" {
		index["accent3"] = cs.Accent3
	}
	if cs.Accent4 != "" {
		index["accent4"] = cs.Accent4
	}
	if cs.Accent5 != "" {
		index["accent5"] = cs.Accent5
	}
	if cs.Accent6 != "" {
		index["accent6"] = cs.Accent6
	}
	if cs.Hlink != "" {
		index["hlink"] = cs.Hlink
	}
	if cs.FolHlink != "" {
		index["folHlink"] = cs.FolHlink
	}

	return index
}

// ResolveThemeColor resolves a theme color slot name to a hex RGB value.
// Returns empty string if the slot is not found.
func (cs *ColorScheme) ResolveThemeColor(slot string) string {
	if cs == nil {
		return ""
	}
	index := cs.Index()

	// Normalize slot name (case-insensitive, remove whitespace)
	slot = strings.ToLower(strings.TrimSpace(slot))

	// Check primary name first
	if hex, ok := index[slot]; ok {
		return hex
	}

	// Check common aliases
	switch slot {
	case "tx1", "text1":
		return cs.Dk1
	case "bg1", "background1":
		return cs.Lt1
	case "tx2", "text2":
		return cs.Dk2
	case "bg2", "background2":
		return cs.Lt2
	}

	return ""
}

// ApplyColorModifier applies themeShade and themeTint modifiers to a base hex color.
// themeShade: hex 00–FF where FF = no change, 00 = black (darken)
// themeTint: hex 00–FF where FF = no change, 00 = white (lighten)
// Returns modified hex color or original if modifiers are invalid.
func ApplyColorModifier(baseHex, themeShade, themeTint string) string {
	if baseHex == "" {
		return ""
	}

	// Parse base color
	r, g, b, ok := parseHexRGB(baseHex)
	if !ok {
		return baseHex
	}

	// Apply shade (darken) if present
	if themeShade != "" {
		shade, ok := parseHexPercent(themeShade)
		if ok && shade >= 0 && shade <= 1 {
			r = uint8(float64(r) * shade)
			g = uint8(float64(g) * shade)
			b = uint8(float64(b) * shade)
		}
	}

	// Apply tint (lighten) if present
	if themeTint != "" {
		tint, ok := parseHexPercent(themeTint)
		if ok && tint >= 0 && tint <= 1 {
			// Tint = mix with white: color * tint + 255 * (1 - tint)
			invTint := 1 - tint
			r = uint8(float64(r)*tint + 255*invTint)
			g = uint8(float64(g)*tint + 255*invTint)
			b = uint8(float64(b)*tint + 255*invTint)
		}
	}

	return formatHexRGB(r, g, b)
}

// parseHexRGB parses a 6-digit hex RGB string like "4472C4".
func parseHexRGB(hex string) (r, g, b uint8, ok bool) {
	hex = strings.TrimPrefix(strings.ToUpper(hex), "#")
	if len(hex) != 6 {
		return 0, 0, 0, false
	}

	var rr, gg, bb uint64
	var err error
	rr, err = parseHexByte(hex[0:2])
	if err != nil {
		return 0, 0, 0, false
	}
	gg, err = parseHexByte(hex[2:4])
	if err != nil {
		return 0, 0, 0, false
	}
	bb, err = parseHexByte(hex[4:6])
	if err != nil {
		return 0, 0, 0, false
	}

	return uint8(rr), uint8(gg), uint8(bb), true
}

// parseHexPercent parses a hex percentage value (00–FF) to a float 0–1.
func parseHexPercent(hex string) (float64, bool) {
	val, err := parseHexByte(hex)
	if err != nil {
		return 0, false
	}
	return float64(val) / 255.0, true
}

// parseHexByte parses a 2-digit hex string to uint64.
func parseHexByte(hex string) (uint64, error) {
	// Simple implementation - in production you'd use strconv.ParseUint
	hex = strings.ToUpper(hex)
	var result uint64
	for _, ch := range hex {
		result <<= 4
		if ch >= '0' && ch <= '9' {
			result |= uint64(ch - '0')
		} else if ch >= 'A' && ch <= 'F' {
			result |= uint64(ch - 'A' + 10)
		} else {
			return 0, &strconv.NumError{Func: "ParseUint", Num: hex, Err: strconv.ErrSyntax}
		}
	}
	return result, nil
}

// formatHexRGB formats R,G,B bytes as 6-digit hex.
func formatHexRGB(r, g, b uint8) string {
	return fmt.Sprintf("%02X%02X%02X", r, g, b)
}
