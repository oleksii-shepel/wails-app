package textmetrics

import (
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	sharedfontmetrics "office/pkg/fontmetrics"
	resolve "office/pkg/word/ir"
)

const TwipsPerPoint = 20.0

func RunWidthPt(catalog *sharedfontmetrics.Catalog, format resolve.RunFormat, rawText string, defaultSizePt float64) (float64, float64) {
	fontSizePt := EffectiveFontSizePt(format, false, defaultSizePt)
	text := rawText
	if format.Caps != nil && *format.Caps {
		text = strings.ToUpper(text)
	}
	widthPt := NaturalWidthPt(catalog, format, text, defaultSizePt)
	runeCount := utf8.RuneCountInString(text)
	if format.CharacterSpacing != nil && *format.CharacterSpacing != 0 && runeCount > 1 {
		widthPt += float64(*format.CharacterSpacing) / TwipsPerPoint * float64(runeCount-1)
	}
	if format.CharScale != nil && *format.CharScale > 0 && *format.CharScale != 100 {
		widthPt *= float64(*format.CharScale) / 100.0
	}
	if format.FitTextWidth != nil && *format.FitTextWidth > 0 && rawText != "" {
		widthPt = float64(*format.FitTextWidth) / TwipsPerPoint
	}
	return widthPt, fontSizePt
}

func NaturalWidthPt(catalog *sharedfontmetrics.Catalog, format resolve.RunFormat, text string, defaultSizePt float64) float64 {
	if text == "" {
		return 0
	}
	type segment struct {
		family              string
		text                []rune
		size                float64
		bold, italic, pairs bool
	}
	segments := make([]segment, 0, 4)
	for _, r := range text {
		complex := RuneUsesComplexScript(format, r)
		family := FontNameForRune(format, r, complex)
		size := EffectiveFontSizePt(format, complex, defaultSizePt)
		bold, italic := StyleForScript(format, complex)
		out := r
		if format.SmallCaps != nil && *format.SmallCaps && unicode.IsLower(r) {
			out = unicode.ToUpper(r)
			size *= 0.8
		}
		pairs := format.Kerning != nil && *format.Kerning > 0 && int(math.Round(size*2)) >= *format.Kerning
		if len(segments) > 0 {
			last := &segments[len(segments)-1]
			if last.family == family && last.size == size && last.bold == bold && last.italic == italic && last.pairs == pairs {
				last.text = append(last.text, out)
				continue
			}
		}
		segments = append(segments, segment{family: family, text: []rune{out}, size: size, bold: bold, italic: italic, pairs: pairs})
	}
	width := 0.0
	for _, seg := range segments {
		if catalog != nil && seg.family != "" {
			if measured, ok := catalog.MeasureTextWithPairs(string(seg.text), seg.family, seg.bold, seg.italic, seg.size, seg.pairs); ok {
				width += measured
				continue
			}
		}
		mono, scale := IsMonospace(seg.family), FontWidthScale(seg.family)
		fallbackWidth := 0.0
		for _, ch := range seg.text {
			fallbackWidth += CharWidthFactor(ch, mono) * seg.size * scale
		}
		// The generic fallback factors describe a regular face. Browser
		// synthetic bold is measurably wider, particularly in the legacy DOC
		// fixtures that rely on an unavailable Times-family face. Reserve that
		// width during physical-line breaking so a line cannot overrun its slot.
		if seg.bold {
			fallbackWidth *= 1.08
		}
		width += fallbackWidth
	}
	return width
}

func EffectiveFontSizePt(format resolve.RunFormat, complex bool, defaultSizePt float64) float64 {
	size := defaultSizePt
	if complex && format.FontSizeCS != nil && *format.FontSizeCS > 0 {
		size = float64(*format.FontSizeCS) / 2
	} else if format.FontSize != nil && *format.FontSize > 0 {
		size = float64(*format.FontSize) / 2
	}
	if format.VertAlignVal == "superscript" || format.VertAlignVal == "subscript" {
		size *= 0.65
	}
	return size
}
func StyleForScript(format resolve.RunFormat, complex bool) (bool, bool) {
	bold := format.Bold != nil && *format.Bold
	italic := format.Italic != nil && *format.Italic
	if complex {
		if format.BoldCS != nil {
			bold = *format.BoldCS
		}
		if format.ItalicCS != nil {
			italic = *format.ItalicCS
		}
	}
	return bold, italic
}
func RuneUsesComplexScript(format resolve.RunFormat, r rune) bool {
	if format.ComplexScript != nil && *format.ComplexScript {
		return true
	}
	if format.RTL != nil && *format.RTL {
		return true
	}
	return unicode.In(r, unicode.Arabic, unicode.Hebrew, unicode.Syriac, unicode.Thaana, unicode.Nko)
}
func RuneUsesEastAsia(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hangul, unicode.Hiragana, unicode.Katakana, unicode.Bopomofo)
}
func FontNameForRune(format resolve.RunFormat, r rune, complex bool) string {
	if complex {
		if format.FontCS != "" {
			return format.FontCS
		}
		if format.FontHAnsi != "" {
			return format.FontHAnsi
		}
		return format.FontASCII
	}
	if RuneUsesEastAsia(r) {
		if format.FontEastAsia != "" {
			return format.FontEastAsia
		}
		if format.FontHAnsi != "" {
			return format.FontHAnsi
		}
		return format.FontASCII
	}
	if r <= 0x7f {
		if format.FontASCII != "" {
			return format.FontASCII
		}
		if format.FontHAnsi != "" {
			return format.FontHAnsi
		}
	} else if format.FontHAnsi != "" {
		return format.FontHAnsi
	}
	if format.FontASCII != "" {
		return format.FontASCII
	}
	if format.FontEastAsia != "" {
		return format.FontEastAsia
	}
	return format.FontCS
}
func InterWordSpaceWidthPt(catalog *sharedfontmetrics.Catalog, format resolve.RunFormat, defaultSizePt, previousFontSizePt float64) float64 {
	current := EffectiveFontSizePt(format, false, defaultSizePt)
	if catalog != nil {
		spaceFormat := format
		spaceFormat.FitTextWidth = nil
		family := FontNameForRune(spaceFormat, ' ', false)
		bold, italic := StyleForScript(spaceFormat, false)
		if catalog.Match(family, bold, italic) != nil {
			if width, _ := RunWidthPt(catalog, spaceFormat, " ", defaultSizePt); width > 0 {
				return width
			}
		}
	}
	size := math.Max(previousFontSizePt, current)
	if size <= 0 {
		size = defaultSizePt
	}
	return size * 0.28
}
func CharWidthFactor(ch rune, monospace bool) float64 {
	if monospace {
		return .6
	}
	switch {
	case ch >= 0x4E00 && ch <= 0x9FFF, ch >= 0x3000 && ch <= 0x303F, ch >= 0xF900 && ch <= 0xFAFF, ch >= 0xFF00 && ch <= 0xFFEF:
		return 1
	case ch == 'i' || ch == 'l' || ch == '1' || ch == '!' || ch == '|' || ch == '.' || ch == ',' || ch == ';' || ch == ':' || ch == '\'' || ch == '`':
		return .28
	case ch >= 'A' && ch <= 'Z', ch >= 0xC0 && ch <= 0xD6, ch >= 0xD8 && ch <= 0xDE:
		return .62
	case ch >= '0' && ch <= '9':
		return .55
	default:
		return .45
	}
}
func IsMonospace(name string) bool {
	l := strings.ToLower(name)
	for _, m := range []string{"courier", "consolas", "lucida console", "monaco", "menlo", "source code pro", "fira mono", "fira code", "inconsolata", "roboto mono", "dejavu sans mono", "liberation mono", "anonymous pro", "cascadia code", "cascadia mono", "jetbrains mono", "hack"} {
		if strings.Contains(l, m) {
			return true
		}
	}
	return false
}
func FontWidthScale(name string) float64 {
	l := strings.ToLower(name)
	for _, f := range []string{"calibri", "arial", "aptos", "segoe", "helvetica", "verdana", "tahoma"} {
		if strings.Contains(l, f) {
			return .92
		}
	}
	return 1
}
