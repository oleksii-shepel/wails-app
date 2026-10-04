package fontmetrics

import (
	"strings"

	shared "office/pkg/fontmetrics"
)

// FamilyFallbacks returns the CSS font stack a renderer should use after an
// authored family: the families a browser will actually find when the source
// font is not installed. Measurement consults the same list so layout and the
// browser resolve a missing family to the same face instead of measuring with
// guessed per-character widths while the browser paints something wider.
func FamilyFallbacks(name string) []string {
	lower := normalizeFallbackFamily(name)
	if lower == "" {
		return nil
	}
	if fallbacks, ok := commonFontFallbacks[lower]; ok {
		return append([]string(nil), fallbacks...)
	}
	switch {
	case containsAnyFontToken(lower, "code", "mono", "typewriter"):
		return []string{"Consolas", "Courier New", "monospace"}
	case containsAnyFontToken(lower, "script", "handwriting", "calligraphy", "cursive"):
		return []string{"Brush Script MT", "cursive"}
	case containsAnyFontToken(lower, "display", "condensed", "narrow"):
		return []string{"Arial Narrow", "Arial", "sans-serif"}
	case containsAnyFontToken(lower, "mincho", "song", "ming", "myeong", "batang"):
		return []string{"Times New Roman", "serif"}
	case containsAnyFontToken(lower, "serif", "roman", "garamond", "georgia", "cambria", "baskerville", "bodoni", "didot", "antiqua", "palatino", "bookman", "times"):
		return []string{"Times New Roman", "Times", "serif"}
	case containsAnyFontToken(lower, "gothic", "sans", "grotesk", "grotesque", "helvetica", "arial", "verdana", "tahoma", "frutiger", "univers", "aptos", "calibri", "segoe", "yahei", "meiryo", "gulim", "dotum", "malgun"):
		return []string{"Arial", "sans-serif"}
	case containsAnyFontToken(lower, "black", "fantasy", "decorative"):
		return []string{"Impact", "Arial Black", "fantasy"}
	default:
		return []string{"Arial", "sans-serif"}
	}
}

// ResolveFamily returns the family to measure text with: the authored family
// when the catalog knows it, otherwise the first fallback the catalog can
// actually measure. It reports "" when neither has metrics, leaving the
// caller's own estimate in place.
func ResolveFamily(catalog *shared.Catalog, family string, bold, italic bool) string {
	if catalog == nil || catalog.Empty() {
		return ""
	}
	if strings.TrimSpace(family) != "" && catalog.Match(family, bold, italic) != nil {
		return family
	}
	for _, fallback := range FamilyFallbacks(family) {
		if catalog.Match(fallback, bold, italic) != nil {
			return fallback
		}
	}
	return ""
}

var commonFontFallbacks = map[string][]string{
	"aller":             {"Calibri", "Arial", "sans-serif"},
	"aller light":       {"Segoe UI", "Calibri", "Arial", "sans-serif"},
	"aptos":             {"Arial", "sans-serif"},
	"arial narrow":      {"Arial", "sans-serif"},
	"book antiqua":      {"Palatino", "serif"},
	"brush script mt":   {"Segoe Script", "cursive"},
	"calibri":           {"Arial", "sans-serif"},
	"cambria":           {"Times New Roman", "Times", "serif"},
	"comic sans ms":     {"Comic Sans", "cursive"},
	"consolas":          {"Courier New", "monospace"},
	"constantia":        {"Georgia", "Times New Roman", "serif"},
	"courier new":       {"Courier", "monospace"},
	"garamond":          {"Georgia", "Times New Roman", "serif"},
	"georgia":           {"Times New Roman", "Times", "serif"},
	"meiryo":            {"Yu Gothic", "MS PGothic", "sans-serif"},
	"microsoft yahei":   {"SimHei", "sans-serif"},
	"ms pgothic":        {"Yu Gothic", "Meiryo", "sans-serif"},
	"oswald":            {"Arial Narrow", "Arial", "sans-serif"},
	"palatino linotype": {"Palatino", "Book Antiqua", "serif"},
	"segoe script":      {"Brush Script MT", "cursive"},
	"simsun":            {"PMingLiU", "serif"},
	"source code pro":   {"Consolas", "Courier New", "monospace"},
	"times new roman":   {"Times", "serif"},
	"yu gothic":         {"Meiryo", "MS PGothic", "sans-serif"},
}

func containsAnyFontToken(name string, tokens ...string) bool {
	for _, token := range tokens {
		if strings.Contains(name, token) {
			return true
		}
	}
	return false
}

func normalizeFallbackFamily(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
