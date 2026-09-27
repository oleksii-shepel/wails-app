package layout

import (
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"office/pkg/fontmetrics"
	ir "office/pkg/presentation/ir"
)

// TextFragment is a measured slice of a canonical run. Run remains the owner
// of visual styling; Text is the physical-line slice to materialize.
type TextFragment struct {
	Run     *ir.Run
	Visual  RunVisualPlan
	Text    string
	WidthPt float64
	// Baseline-side extents are materialized by layout so serializers do not
	// ask the browser to reconcile mixed-family baselines. TopOffsetPt places
	// this fragment's natural line box inside the owning physical line box.
	AscentPt, DescentPt float64
	TopOffsetPt         float64
}

// PhysicalTextLine is one layout-owned horizontal line. BreakAfter records why
// the next line exists: "authored" for a source line break and "wrap" for a
// layout decision. Serializers must not recompute either decision.
type PhysicalTextLine struct {
	Fragments           []TextFragment
	WidthPt, HeightPt   float64
	AscentPt, DescentPt float64
	BreakAfter          string
}

type ParagraphLineLayout struct {
	Lines           []PhysicalTextLine
	ContentHeightPt float64
}

// FontAdvanceMetrics is the canonical, renderer-neutral advance model used by
// presentation line layout. It deliberately models families rather than using
// one global coefficient, keeping line decisions aligned with the emitted font.
type FontAdvanceMetrics struct {
	Profile                                 string
	Lower, Upper, Digit, Punct, Space, Wide float64
	NarrowLower, WideLower                  float64
	NarrowUpper, WideUpper                  float64
	BoldScale, ItalicScale                  float64
	// AdvanceScale accounts for condensed/expanded faces. It is applied after
	// glyph-specific advance lookup so exact Latin tables and category fallbacks
	// are scaled identically.
	AdvanceScale float64
	// LineBoxScale is the natural single-line advance for the effective face.
	// It is deliberately kept beside advance metrics so wrapping and vertical
	// composition are derived from the same resolved font model.
	LineBoxScale float64
	// AscentShare and DescentShare split the natural line box around the baseline.
	// Physical mixed-run lines compose maxima on each side independently instead
	// of taking the tallest run as one undifferentiated scalar.
	AscentShare, DescentShare float64
	// FaceAscent/FaceDescent are the real font ascent/descent in em units, as a
	// browser resolves them. They differ from LineBoxScale (Office's single-line
	// advance) and are what decides where a glyph baseline actually lands inside
	// an emitted line box. Zero means unknown: baseline anchoring is skipped.
	FaceAscent, FaceDescent float64
}

// FontFallbackClass is the canonical metric/fallback family shared by physical
// layout and serializers. Keeping this classification in layout prevents CSS
// fallback stacks from drifting away from the metrics used to make wrap and
// baseline decisions.
type FontFallbackClass string

const (
	FontFallbackSans      FontFallbackClass = "sans"
	FontFallbackCondensed FontFallbackClass = "condensed"
	FontFallbackSerif     FontFallbackClass = "serif"
	FontFallbackMono      FontFallbackClass = "mono"
)

func ResolveFontFallbackClass(family string) FontFallbackClass {
	f := strings.ToLower(strings.TrimSpace(family))
	if strings.Contains(f, "source code pro") {
		return FontFallbackMono
	}
	words := strings.FieldsFunc(f, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	})
	for _, word := range words {
		if word == "code" || word == "mono" || word == "monospace" || word == "monospaced" {
			return FontFallbackMono
		}
	}
	if strings.Contains(f, "times") || strings.Contains(f, "cambria") ||
		strings.Contains(f, "serif") || strings.Contains(f, "playfair") {
		return FontFallbackSerif
	}
	if strings.Contains(f, "oswald") || strings.Contains(f, "condensed") {
		return FontFallbackCondensed
	}
	return FontFallbackSans
}

func ResolveFontAdvanceMetrics(family string) FontAdvanceMetrics {
	f := strings.ToLower(strings.TrimSpace(family))
	// Layout must measure the face the browser will actually paint. When an
	// authored family is unavailable/unknown, presentation HTML falls back to a
	// metric-compatible family. Use the same family class here rather than a
	// generic average, otherwise layout wraps for one font and CSS paints another.
	if f == "" {
		f = "arial"
	} else if strings.Contains(f, "source code pro") {
		f = "consolas"
	} else if strings.Contains(f, "oswald") {
		// Oswald is materially narrower than Arial Narrow. Keep its identity so
		// physical wrapping uses the display face's own advance class instead of
		// widening it through the generic Arial-Narrow approximation.
		f = "oswald"
	} else if strings.Contains(f, "condensed") {
		f = "arial narrow"
	}
	m := FontAdvanceMetrics{
		Lower: .48, Upper: .57, Digit: .53, Punct: .31, Space: .278, Wide: 1,
		NarrowLower: .25, WideLower: .78, NarrowUpper: .30, WideUpper: .88,
		BoldScale: 1.01, ItalicScale: 1.0, AdvanceScale: 1.0, LineBoxScale: 1.117,
		AscentShare: .80, DescentShare: .20,
	}
	switch {
	case strings.Contains(f, "oswald"):
		// Oswald's uppercase display metrics are substantially condensed. Use
		// the stable Arial Latin proportions as the glyph-shape model, then apply
		// an Oswald-specific final advance scale. This is deliberately a font
		// metric, not a title/fixture wrapping exception.
		m.Profile = "arial"
		m.Lower, m.Upper, m.Digit, m.Punct, m.Space = .50, .667, .556, .278, .278
		m.NarrowLower, m.WideLower, m.NarrowUpper, m.WideUpper = .222, .833, .278, .944
		m.AdvanceScale = .70
		m.LineBoxScale = 1.117
	case strings.Contains(f, "lucida sans unicode"):
		// Lucida Sans Unicode is the authored face used by older Office themes.
		// Treating it as Arial made layout advances systematically wider than the
		// browser/PowerPoint face and caused premature physical wrapping. Keep a
		// dedicated profile so measurement and emitted typography agree.
		m.Profile = "lucida"
		m.Lower, m.Upper, m.Digit, m.Punct, m.Space = .47, .62, .53, .26, .25
		m.NarrowLower, m.WideLower, m.NarrowUpper, m.WideUpper = .20, .78, .26, .88
		m.BoldScale = 1.01
		m.LineBoxScale = 1.20
		m.AscentShare, m.DescentShare = .82, .18
	case strings.Contains(f, "arial"), strings.Contains(f, "helvetica"), strings.Contains(f, "liberation sans"), strings.Contains(f, "lucida sans"):
		m.Profile = "arial"
		// Helvetica/Arial-compatible advances. The narrow/wide classes matter
		// much more to PowerPoint wrapping than a single average lowercase width:
		// e.g. Arial 'l' is ~0.22em while 'm' is ~0.83em.
		m.Lower, m.Upper, m.Digit, m.Punct, m.Space = .50, .667, .556, .278, .278
		m.NarrowLower, m.WideLower, m.NarrowUpper, m.WideUpper = .222, .833, .278, .944
		m.LineBoxScale = 1.117
		m.FaceAscent, m.FaceDescent = .905, .212
		if strings.Contains(f, "narrow") {
			// Keep the canonical Arial glyph table and scale the final advances.
			// Scaling only the fallback fields does not affect ASCII because
			// standardLatinAdvanceFactor resolves those glyphs first.
			m.AdvanceScale = .82
		}
	case strings.Contains(f, "times"), strings.Contains(f, "cambria"), strings.Contains(f, "serif"), strings.Contains(f, "playfair"):
		m.Profile = "times"
		m.Lower, m.Upper, m.Digit, m.Punct, m.Space = .444, .667, .50, .278, .25
		m.NarrowLower, m.WideLower, m.NarrowUpper, m.WideUpper = .278, .778, .333, .889
		m.LineBoxScale = 1.20
		// Times New Roman OS/2 winAscent 1825 / winDescent 443 over 2048 upem.
		m.AscentShare, m.DescentShare = .8046, .1954
		m.FaceAscent, m.FaceDescent = .891, .216
	case strings.Contains(f, "wingdings"), strings.Contains(f, "webdings"), f == "symbol":
		// Legacy PPT commonly selects symbol faces through symbolFontRef.  Their
		// private-use glyphs are effectively em-cell advances; measuring them as
		// lowercase Arial made bullets/symbol runs too narrow even after the
		// correct font reference had been resolved.
		m.Profile = "symbol"
		m.Lower, m.Upper, m.Digit, m.Punct, m.Space, m.Wide = 1, 1, 1, 1, .50, 1
		m.NarrowLower, m.WideLower, m.NarrowUpper, m.WideUpper = 1, 1, 1, 1
		m.LineBoxScale = 1.20
	case strings.Contains(f, "gothic"), strings.Contains(f, "mincho"), strings.Contains(f, "meiryo"),
		strings.Contains(f, "malgun"), strings.Contains(f, "simsun"), strings.Contains(f, "simhei"),
		strings.Contains(f, "mingliu"), strings.Contains(f, "pmingliu"), strings.Contains(f, "ms pgothic"):
		// East-Asian Office faces use a full-em ideographic cell while Latin
		// glyphs remain approximately half-width.  Keep this as a physical font
		// metric profile rather than a parser or renderer special case.
		m.Profile = "cjk"
		m.Lower, m.Upper, m.Digit, m.Punct, m.Space, m.Wide = .50, .50, .50, .50, .50, 1
		m.NarrowLower, m.WideLower, m.NarrowUpper, m.WideUpper = .50, .50, .50, .50
		m.LineBoxScale = 1.20
		m.AscentShare, m.DescentShare = .88, .12
	case strings.Contains(f, "courier"), strings.Contains(f, "consolas"), strings.Contains(f, "mono"):
		m.Profile = "mono"
		m.Lower, m.Upper, m.Digit, m.Punct, m.Space = .60, .60, .60, .60, .60
		m.NarrowLower, m.WideLower, m.NarrowUpper, m.WideUpper = .60, .60, .60, .60
		m.LineBoxScale = 1.20
	case strings.Contains(f, "calibri"), strings.Contains(f, "aptos"):
		m.Profile = "calibri"
		m.Lower, m.Upper, m.Digit, m.Punct, m.Space = .46, .60, .51, .27, .226
		m.NarrowLower, m.WideLower, m.NarrowUpper, m.WideUpper = .21, .78, .27, .88
		m.LineBoxScale = 1.20
		m.AscentShare, m.DescentShare = .79, .21
	}
	if m.Profile == "" {
		return ResolveFontAdvanceMetrics("Arial")
	}
	return m
}

// LayoutParagraphLines resolves authored breaks, tabs and calculated wraps into
// physical line boxes. Text indent and list marker reservation apply to the
// first physical line only; continuation lines recover the paragraph width.
func LayoutParagraphLines(p *ir.Paragraph, metrics ParagraphMetrics, fontScale float64, markerText string, markerFontPt float64) ParagraphLineLayout {
	return layoutParagraphLinesWithMarker(p, metrics, fontScale, markerText, markerFontPt, "", measurePlainTextWidthPt(" ", markerFontPt, "", false, false))
}

func layoutParagraphLinesWithMarker(p *ir.Paragraph, metrics ParagraphMetrics, fontScale float64, markerText string, markerFontPt float64, markerFontFamily string, markerGapPt float64) ParagraphLineLayout {
	return layoutParagraphLinesWithMarkerAndCatalog(p, metrics, fontScale, markerText, markerFontPt, markerFontFamily, markerGapPt, nil)
}

func layoutParagraphLinesWithMarkerAndCatalog(p *ir.Paragraph, metrics ParagraphMetrics, fontScale float64, markerText string, markerFontPt float64, markerFontFamily string, markerGapPt float64, catalog *fontmetrics.Catalog) ParagraphLineLayout {
	if p == nil {
		return ParagraphLineLayout{}
	}
	continuationWidth := math.Max(0, metrics.AvailableWidthPt)
	firstWidth := metrics.FirstLineAvailableWidthPt
	// Compatibility for callers/tests that construct ParagraphMetrics directly.
	// A zero first-line width means "not materialized" unless the paragraph's
	// continuation width is also zero.
	if firstWidth <= 0 && continuationWidth > 0 {
		firstWidth = continuationWidth
		if metrics.HasTextIndent {
			firstWidth = math.Max(0, firstWidth-metrics.TextIndentPt)
		}
	}
	if markerText != "" {
		if markerFontPt <= 0 {
			markerFontPt = paragraphBaseFontPt(p, fontScale)
		}
		firstWidth -= measurePlainTextWidthPtWithCatalog(markerText, markerFontPt, markerFontFamily, false, false, catalog) + markerGapPt
	}
	firstWidth = math.Max(0, firstWidth)

	var out ParagraphLineLayout
	line := PhysicalTextLine{}
	lineIndex := 0
	if markerText != "" && markerFontPt > 0 {
		line.AscentPt, line.DescentPt = paragraphPhysicalBaselineExtentsPt(markerFontPt, markerFontFamily, metrics)
		line.HeightPt = line.AscentPt + line.DescentPt
	}
	currentLimit := func() float64 {
		if lineIndex == 0 {
			return firstWidth
		}
		return continuationWidth
	}
	flush := func(force bool, reason string) {
		if !force && len(line.Fragments) == 0 {
			return
		}
		if line.HeightPt <= 0 {
			line.AscentPt, line.DescentPt = paragraphPhysicalBaselineExtentsPt(paragraphBaseFontPt(p, fontScale), paragraphBaseFontFamily(p), metrics)
			line.HeightPt = line.AscentPt + line.DescentPt
		}
		finalizePhysicalTextLineFragments(&line)
		line.BreakAfter = reason
		out.ContentHeightPt += line.HeightPt
		out.Lines = append(out.Lines, line)
		line = PhysicalTextLine{}
		lineIndex++
	}
	appendFrag := func(run *ir.Run, text string, w, h float64) {
		if text == "" {
			return
		}
		fontPt := runFontPt(run, p, fontScale)
		a, d := runPhysicalBaselineExtentsPt(run, fontPt, runFontFamily(run, p), metrics)
		n := len(line.Fragments)
		if n > 0 && line.Fragments[n-1].Run == run && text != "	" && line.Fragments[n-1].Text != "	" {
			line.Fragments[n-1].Text += text
			line.Fragments[n-1].WidthPt += w
		} else {
			line.Fragments = append(line.Fragments, TextFragment{Run: run, Visual: resolveRunVisualPlan(run, fontScale, fontPt), Text: text, WidthPt: w, AscentPt: a, DescentPt: d})
		}
		line.WidthPt += w
		if a > line.AscentPt {
			line.AscentPt = a
		}
		if d > line.DescentPt {
			line.DescentPt = d
		}
		line.HeightPt = line.AscentPt + line.DescentPt
		if line.HeightPt <= 0 && h > line.HeightPt {
			line.HeightPt = h
		}
	}
	leadingRunCharacterSpacingPt := func(run *ir.Run) float64 {
		if run == nil || run.CharSpacing == 0 || len(line.Fragments) == 0 {
			return 0
		}
		prev := line.Fragments[len(line.Fragments)-1]
		if prev.Run != run || prev.Text == "\t" || prev.Text == "\n" {
			return 0
		}
		return float64(run.CharSpacing) / 100.0
	}
	appendWrappedToken := func(run *ir.Run, token string, lineH float64) {
		for _, r := range token {
			part := string(r)
			w := measureRunTextWidthPtWithCatalog(part, run, runFontPt(run, p, fontScale), catalog) + leadingRunCharacterSpacingPt(run)
			if lim := currentLimit(); lim > 0 && line.WidthPt > 0 && line.WidthPt+w > lim {
				flush(false, "wrap")
				w = measureRunTextWidthPtWithCatalog(part, run, runFontPt(run, p, fontScale), catalog)
			}
			// If one glyph itself is wider than an empty line, keep it on that
			// line and let the following glyph wrap. Creating an empty physical
			// line before an unbreakable glyph would manufacture a source line.
			// (The line.WidthPt > 0 guard above is therefore intentional.)
			appendFrag(run, part, w, lineH)
		}
	}
	tokens := paragraphLayoutTokens(p)
	for tokenIndex, item := range tokens {
		run, token := item.run, item.text
		fontPt := runFontPt(run, p, fontScale)
		lineH := paragraphPhysicalLineHeightPt(fontPt, runFontFamily(run, p), metrics)
		if token == "\n" {
			flush(true, "authored")
			continue
		}
		if token == "\t" {
			fieldWidth, decimalPrefix := followingTabFieldMetricsWithCatalog(tokens, tokenIndex+1, p, fontScale, catalog)
			tw := nextAlignedTabAdvancePt(line.WidthPt, p, fontPt, fieldWidth, decimalPrefix)
			// Binary PPT commonly persists a leading tab on a non-bulleted
			// continuation paragraph that also carries a hanging indent. The
			// hanging indent already moves the first-line origin left; advancing to
			// the next global tab stop a second time can add 70-110pt and manufacture
			// extra wraps. In PowerPoint that leading tab simply returns the cursor
			// to the paragraph's normal text edge.
			if line.WidthPt == 0 && markerText == "" && metrics.TextIndentPt < 0 {
				tw = math.Min(tw, -metrics.TextIndentPt)
			}
			if lim := currentLimit(); lim > 0 && line.WidthPt > 0 && line.WidthPt+tw+fieldWidth > lim {
				flush(false, "wrap")
				tw = nextAlignedTabAdvancePt(line.WidthPt, p, fontPt, fieldWidth, decimalPrefix)
			}
			appendFrag(run, token, tw, lineH)
			continue
		}
		tw := measureRunTextWidthPtWithCatalog(token, run, fontPt, catalog) + leadingRunCharacterSpacingPt(run)
		if lim := currentLimit(); lim > 0 && line.WidthPt > 0 && line.WidthPt+tw > lim && strings.TrimSpace(token) != "" {
			flush(false, "wrap")
			tw = measureRunTextWidthPtWithCatalog(token, run, fontPt, catalog)
		}
		if lim := currentLimit(); lim > 0 && tw > lim && strings.TrimSpace(token) != "" {
			appendWrappedToken(run, token, lineH)
			continue
		}
		appendFrag(run, token, tw, lineH)
	}
	flush(len(out.Lines) == 0, "")
	return out
}

func paragraphPhysicalLineHeightPt(fontPt float64, family string, metrics ParagraphMetrics) float64 {
	if metrics.LineHeightPt > 0 {
		return metrics.LineHeightPt
	}
	return fontPt * ResolveFontAdvanceMetrics(family).LineBoxScale * metrics.LineHeightScale
}

func paragraphPhysicalBaselineExtentsPt(fontPt float64, family string, metrics ParagraphMetrics) (float64, float64) {
	h := paragraphPhysicalLineHeightPt(fontPt, family, metrics)
	if h <= 0 {
		return 0, 0
	}
	// Absolute authored line spacing owns the complete line box. Do not let font
	// family proportions enlarge that explicit distance.
	if metrics.LineHeightPt > 0 {
		return h * .80, h * .20
	}
	fm := ResolveFontAdvanceMetrics(family)
	a, d := fm.AscentShare, fm.DescentShare
	if a <= 0 || d < 0 || a+d <= 0 {
		a, d = .80, .20
	}
	t := a + d
	// Percentage line spacing compresses the box above the baseline: PowerPoint
	// keeps the face's natural descent and absorbs the reduction in the ascent.
	if natural := paragraphPhysicalLineHeightPt(fontPt, family, ParagraphMetrics{LineHeightScale: 1}); natural > 0 {
		if descent := natural * d / t; descent > 0 && descent < h {
			return h - descent, descent
		}
	}
	return h * a / t, h * d / t
}

// runPhysicalBaselineExtentsPt projects an authored baseline shift into the
// physical line box. BaselineShiftPct is expressed as a percentage of the
// run's own font size; positive values move the glyph box upward and negative
// values move it downward. The canonical run keeps the authored value while
// layout owns the enlarged ascent/descent required to avoid clipping mixed
// superscript/subscript lines.
func runPhysicalBaselineExtentsPt(run *ir.Run, fontPt float64, family string, metrics ParagraphMetrics) (float64, float64) {
	a, d := paragraphPhysicalBaselineExtentsPt(fontPt, family, metrics)
	if run == nil || run.BaselineShiftPct == 0 || fontPt <= 0 {
		return a, d
	}
	shiftPt := fontPt * run.BaselineShiftPct / 100.0
	if shiftPt > 0 {
		a += shiftPt
		d = math.Max(0, d-shiftPt)
	} else {
		shiftPt = -shiftPt
		d += shiftPt
		a = math.Max(0, a-shiftPt)
	}
	return a, d
}

func CanLayoutParagraphLines(p *ir.Paragraph, metrics ParagraphMetrics, fontScale float64) bool {
	if p == nil {
		return false
	}
	base := paragraphBaseFontPt(p, fontScale)
	if metrics.AvailableWidthPt <= 0 || metrics.AvailableWidthPt < base*.5 {
		return false
	}
	for _, r := range p.Runs {
		if r != nil && (r.MathML != "" || r.FieldKind != "") {
			return false
		}
	}
	return true
}
func paragraphBaseFontPt(p *ir.Paragraph, scale float64) float64 {
	for _, r := range p.Runs {
		if r != nil && r.FontSize > 0 {
			return float64(r.FontSize) / 100 * scale
		}
	}
	return 12 * scale
}
func paragraphBaseFontFamily(p *ir.Paragraph) string {
	if p != nil {
		for _, r := range p.Runs {
			if r != nil && strings.TrimSpace(r.FontFamily) != "" {
				return r.FontFamily
			}
		}
	}
	return "Arial"
}
func runFontFamily(r *ir.Run, p *ir.Paragraph) string {
	if r != nil && strings.TrimSpace(r.FontFamily) != "" {
		return r.FontFamily
	}
	return paragraphBaseFontFamily(p)
}

const officeScriptFontScale = .65

func runFontPt(r *ir.Run, p *ir.Paragraph, scale float64) float64 {
	fontPt := paragraphBaseFontPt(p, scale)
	if r != nil && r.FontSize > 0 {
		fontPt = float64(r.FontSize) / 100 * scale
	}
	// PowerPoint paints explicit script runs at a reduced effective size. Keep
	// that reduction in the shared physical metric path: wrapping, line
	// extents, and emitted run styles must agree. The canonical run retains its
	// authored point size and baseline displacement unchanged.
	if r != nil && (r.Superscript || r.Subscript) {
		fontPt *= officeScriptFontScale
	}
	return fontPt
}

func splitWrapTokens(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	start := 0
	for i, r := range s {
		if r == '\n' || r == '\t' {
			if i > start {
				out = append(out, s[start:i])
			}
			out = append(out, string(r))
			start = i + utf8.RuneLen(r)
			continue
		}
		if unicode.IsSpace(r) {
			end := i + utf8.RuneLen(r)
			if end > start {
				out = append(out, s[start:end])
			}
			start = end
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func measureRunTextWidthPt(s string, r *ir.Run, fontPt float64) float64 {
	return measureRunTextWidthPtWithCatalog(s, r, fontPt, nil)
}

func measureRunTextWidthPtWithCatalog(s string, r *ir.Run, fontPt float64, catalog *fontmetrics.Catalog) float64 {
	family := ""
	bold, italic := false, false
	if r != nil {
		family = r.FontFamily
		bold = r.Bold
		italic = r.Italic
	}
	w := measurePlainTextWidthPtWithCatalog(s, fontPt, family, bold, italic, catalog)
	if r != nil && r.CharSpacing != 0 {
		n := utf8.RuneCountInString(s)
		if n > 1 {
			w += float64(n-1) * float64(r.CharSpacing) / 100.0
		}
	}
	return w
}
func decodedSymbolText(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Co, r) {
			return false
		}
	}
	return s != ""
}

func measurePlainTextWidthPt(s string, fontPt float64, family string, bold, italic bool) float64 {
	return measurePlainTextWidthPtWithCatalog(s, fontPt, family, bold, italic, nil)
}

func measurePlainTextWidthPtWithCatalog(s string, fontPt float64, family string, bold, italic bool, catalog *fontmetrics.Catalog) float64 {
	if catalog != nil {
		measurementFamily := family
		// Legacy Symbol runs are decoded before layout, but the canonical run can
		// still retain FontFamily=Symbol. Measuring normalized ASCII/Unicode with
		// the physical Symbol face re-introduces the old full-cell advances and can
		// create a wrap that PowerPoint never had (for example "= 733 nsec").
		// Use the same Times-compatible face that the HTML serializer falls back to
		// for decoded textual Symbol content. Private-use glyphs remain Symbol.
		if ResolveFontAdvanceMetrics(family).Profile == "symbol" && decodedSymbolText(s) {
			measurementFamily = "Times New Roman"
		}
		if width, ok := catalog.MeasureText(s, measurementFamily, bold, italic, fontPt); ok {
			return width
		}
	}
	m := ResolveFontAdvanceMetrics(family)
	scale := 1.0
	if bold {
		scale *= m.BoldScale
	}
	if italic {
		scale *= m.ItalicScale
	}
	w := 0.0
	for _, r := range s {
		w += fontPt * runeAdvanceFactor(r, m)
	}
	return w * scale * m.AdvanceScale
}
func runeAdvanceFactor(r rune, m FontAdvanceMetrics) float64 {
	if m.Profile == "symbol" && (unicode.Is(unicode.Co, r) || unicode.Is(unicode.Sm, r) || unicode.Is(unicode.Sc, r) || unicode.Is(unicode.Sk, r) || unicode.Is(unicode.So, r)) {
		return m.Wide
	}
	// Legacy Symbol runs are often decoded to ordinary Unicode before layout.
	// Once decoded, ASCII digits/letters and Greek letters are rendered through
	// the browser's math/serif fallback, not as full-em private-use Symbol cells.
	// Keep full-em treatment for actual symbol/private-use glyphs above, but use
	// Times-compatible advances for normalized textual content. This prevents
	// equations/labels from wrapping one word too early.
	metricProfile := m.Profile
	if m.Profile == "symbol" && !unicode.Is(unicode.Co, r) {
		metricProfile = "times"
	}
	// Standard Office fonts have highly non-uniform Latin advances. Use exact
	// canonical factors for the common ASCII repertoire before falling back to
	// script/category classes. This keeps wrapping deterministic without asking
	// the browser or host font engine to make layout decisions.
	if f, ok := standardLatinAdvanceFactor(r, metricProfile); ok {
		return f
	}
	if m.Profile == "symbol" && metricProfile == "times" {
		tm := ResolveFontAdvanceMetrics("Times New Roman")
		switch {
		case unicode.IsSpace(r):
			return tm.Space
		case unicode.IsUpper(r):
			return tm.Upper
		case unicode.IsLower(r):
			return tm.Lower
		case unicode.IsDigit(r):
			return tm.Digit
		case unicode.IsPunct(r):
			return tm.Punct
		}
	}
	switch {
	case unicode.IsSpace(r):
		return m.Space
	case unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hangul, r):
		return m.Wide
	case unicode.IsUpper(r):
		switch r {
		case 'I':
			return m.NarrowUpper
		case 'M', 'W':
			return m.WideUpper
		}
		return m.Upper
	case unicode.IsLower(r):
		switch r {
		case 'i', 'j', 'l', 't', 'f', 'r':
			return m.NarrowLower
		case 'm', 'w':
			return m.WideLower
		}
		return m.Lower
	case unicode.IsDigit(r):
		return m.Digit
	case unicode.IsPunct(r):
		// Common punctuation has stable standard-font widths that materially
		// affect wrapping around labels and list prefixes.
		switch r {
		case '.', ',', ':', ';', '!', '\'', '`':
			return m.Punct * .72
		case '-', '–':
			return m.Punct * 1.2
		case '—':
			return m.Punct * 3.0
		}
		return m.Punct
	default:
		return m.Lower
	}
}
func standardLatinAdvanceFactor(r rune, profile string) (float64, bool) {
	if profile == "mono" && r >= 32 && r <= 126 {
		return .60, true
	}
	// Widths are normalized em advances for the metric-compatible families used
	// by Office. Values intentionally cover the glyphs that most affect labels,
	// bullets and prose wrapping; uncommon glyphs use the category fallback.
	var table map[rune]float64
	switch profile {
	case "arial":
		table = map[rune]float64{
			' ': .278, '!': .278, '"': .355, '#': .556, '$': .556, '%': .889, '&': .667, '\'': .191,
			'(': .333, ')': .333, '*': .389, '+': .584, ',': .278, '-': .333, '.': .278, '/': .278,
			':': .278, ';': .278, '<': .584, '=': .584, '>': .584, '?': .556, '@': 1.015,
			'A': .667, 'B': .667, 'C': .722, 'D': .722, 'E': .667, 'F': .611, 'G': .778, 'H': .722,
			'I': .278, 'J': .500, 'K': .667, 'L': .556, 'M': .833, 'N': .722, 'O': .778, 'P': .667,
			'Q': .778, 'R': .722, 'S': .667, 'T': .611, 'U': .722, 'V': .667, 'W': .944, 'X': .667,
			'Y': .667, 'Z': .611,
			'a': .556, 'b': .556, 'c': .500, 'd': .556, 'e': .556, 'f': .278, 'g': .556, 'h': .556,
			'i': .222, 'j': .222, 'k': .500, 'l': .222, 'm': .833, 'n': .556, 'o': .556, 'p': .556,
			'q': .556, 'r': .333, 's': .500, 't': .278, 'u': .556, 'v': .500, 'w': .722, 'x': .500,
			'y': .500, 'z': .500,
		}
	case "times":
		table = map[rune]float64{
			' ': .250, '!': .333, '"': .408, '#': .500, '$': .500, '%': .833, '&': .778, '\'': .180,
			'(': .333, ')': .333, '*': .500, '+': .564, ',': .250, '-': .333, '.': .250, '/': .278,
			':': .278, ';': .278, '?': .444,
			'A': .722, 'B': .667, 'C': .667, 'D': .722, 'E': .611, 'F': .556, 'G': .722, 'H': .722,
			'I': .333, 'J': .389, 'K': .722, 'L': .611, 'M': .889, 'N': .722, 'O': .722, 'P': .556,
			'Q': .722, 'R': .667, 'S': .556, 'T': .611, 'U': .722, 'V': .722, 'W': .944, 'X': .722,
			'Y': .722, 'Z': .611,
			'a': .444, 'b': .500, 'c': .444, 'd': .500, 'e': .444, 'f': .333, 'g': .500, 'h': .500,
			'i': .278, 'j': .278, 'k': .500, 'l': .278, 'm': .778, 'n': .500, 'o': .500, 'p': .500,
			'q': .500, 'r': .333, 's': .389, 't': .278, 'u': .500, 'v': .500, 'w': .722, 'x': .500,
			'y': .500, 'z': .444,
		}
	}
	if table == nil {
		return 0, false
	}
	if r >= '0' && r <= '9' {
		if profile == "arial" {
			return .556, true
		}
		if profile == "times" {
			return .500, true
		}
	}
	v, ok := table[r]
	return v, ok
}

type paragraphLayoutToken struct {
	run  *ir.Run
	text string
}

func paragraphLayoutTokens(p *ir.Paragraph) []paragraphLayoutToken {
	if p == nil {
		return nil
	}
	var out []paragraphLayoutToken
	for _, run := range p.Runs {
		if run == nil {
			continue
		}
		for _, text := range splitWrapTokens(run.Text) {
			out = append(out, paragraphLayoutToken{run: run, text: text})
		}
	}
	return out
}

func followingTabFieldMetrics(tokens []paragraphLayoutToken, start int, p *ir.Paragraph, fontScale float64) (width, decimalPrefix float64) {
	return followingTabFieldMetricsWithCatalog(tokens, start, p, fontScale, nil)
}

func followingTabFieldMetricsWithCatalog(tokens []paragraphLayoutToken, start int, p *ir.Paragraph, fontScale float64, catalog *fontmetrics.Catalog) (width, decimalPrefix float64) {
	seenDecimal := false
	for i := start; i < len(tokens); i++ {
		t := tokens[i]
		if t.text == "\t" || t.text == "\n" {
			break
		}
		fontPt := runFontPt(t.run, p, fontScale)
		for _, r := range t.text {
			w := measureRunTextWidthPtWithCatalog(string(r), t.run, fontPt, catalog)
			width += w
			if !seenDecimal {
				if r == '.' || r == ',' {
					seenDecimal = true
				} else {
					decimalPrefix += w
				}
			}
		}
	}
	if !seenDecimal {
		decimalPrefix = width
	}
	return width, decimalPrefix
}

func nextTabAdvancePt(x float64, p *ir.Paragraph, fontPt float64) float64 {
	return nextAlignedTabAdvancePt(x, p, fontPt, 0, 0)
}

// nextAlignedTabAdvancePt resolves the physical advance to the next authored tab
// stop. PowerPoint tab positions identify the alignment point, not necessarily
// the left edge of the following field: centered/right/decimal tabs therefore
// need the following field's measured width before the tab fragment is fixed.
func nextAlignedTabAdvancePt(x float64, p *ir.Paragraph, fontPt, fieldWidth, decimalPrefixWidth float64) float64 {
	for _, stop := range p.TabStops {
		pos := float64(stop.Pos) / 12700.0
		if pos <= x+.01 {
			continue
		}
		target := pos
		switch stop.Algn {
		case "ctr":
			target -= fieldWidth / 2
		case "r":
			target -= fieldWidth
		case "dec":
			target -= decimalPrefixWidth
		}
		if target > x+.01 {
			return target - x
		}
	}
	step := 0.0
	if p.DefaultTabSizeEMU > 0 {
		step = float64(p.DefaultTabSizeEMU) / 12700.0
	}
	if step <= 0 {
		step = fontPt * 4
	}
	next := math.Floor(x/step+1) * step
	if next <= x {
		next = x + step
	}
	return next - x
}

func finalizePhysicalTextLineFragments(line *PhysicalTextLine) {
	if line == nil {
		return
	}
	for i := range line.Fragments {
		f := &line.Fragments[i]
		if f.AscentPt > 0 || f.DescentPt > 0 {
			f.TopOffsetPt = physicalBaselineTopOffsetPt(line.AscentPt, f.AscentPt, f.DescentPt, f.Visual.FontSizePt, fragmentFontFamily(f))
		}
	}
}

func fragmentFontFamily(f *TextFragment) string {
	if f == nil {
		return ""
	}
	if strings.TrimSpace(f.Visual.FontFamily) != "" {
		return f.Visual.FontFamily
	}
	if f.Run != nil {
		return f.Run.FontFamily
	}
	return ""
}

// physicalBaselineTopOffsetPt places an emitted line-box child so its glyph
// baseline lands on the layout-owned baseline. Serializers emit the box with a
// fixed height/line-height, so the consumer would otherwise centre the face's
// own ascent/descent inside that box and silently move the baseline.
func physicalBaselineTopOffsetPt(lineAscentPt, boxAscentPt, boxDescentPt, fontPt float64, family string) float64 {
	fm := ResolveFontAdvanceMetrics(family)
	boxHeightPt := boxAscentPt + boxDescentPt
	if fm.FaceAscent > 0 && fm.FaceDescent > 0 && fontPt > 0 && boxHeightPt > 0 {
		faceBaselinePt := (boxHeightPt-(fm.FaceAscent+fm.FaceDescent)*fontPt)/2 + fm.FaceAscent*fontPt
		return lineAscentPt - faceBaselinePt
	}
	return math.Max(0, lineAscentPt-boxAscentPt)
}

// LayoutParagraphAuthoredLines materializes only source-authored line breaks.
// Width is measured for diagnostics/tabs, but no calculated wrap is introduced.
func LayoutParagraphAuthoredLines(p *ir.Paragraph, metrics ParagraphMetrics, fontScale float64, markerText string, markerFontPt float64) ParagraphLineLayout {
	return layoutParagraphAuthoredLinesWithMarker(p, metrics, fontScale, markerText, markerFontPt, "", measurePlainTextWidthPt(" ", markerFontPt, "", false, false))
}

func layoutParagraphAuthoredLinesWithMarker(p *ir.Paragraph, metrics ParagraphMetrics, fontScale float64, markerText string, markerFontPt float64, markerFontFamily string, markerGapPt float64) ParagraphLineLayout {
	if p == nil {
		return ParagraphLineLayout{}
	}
	var out ParagraphLineLayout
	line := PhysicalTextLine{}
	flush := func(force bool, reason string) {
		if !force && len(line.Fragments) == 0 {
			return
		}
		if line.HeightPt <= 0 {
			line.AscentPt, line.DescentPt = paragraphPhysicalBaselineExtentsPt(paragraphBaseFontPt(p, fontScale), paragraphBaseFontFamily(p), metrics)
			line.HeightPt = line.AscentPt + line.DescentPt
		}
		finalizePhysicalTextLineFragments(&line)
		line.BreakAfter = reason
		out.ContentHeightPt += line.HeightPt
		out.Lines = append(out.Lines, line)
		line = PhysicalTextLine{}
	}
	appendFrag := func(run *ir.Run, text string, w, h float64) {
		if text == "" {
			return
		}
		fontPt := runFontPt(run, p, fontScale)
		a, d := paragraphPhysicalBaselineExtentsPt(fontPt, runFontFamily(run, p), metrics)
		n := len(line.Fragments)
		if n > 0 && line.Fragments[n-1].Run == run && text != "	" && line.Fragments[n-1].Text != "	" {
			line.Fragments[n-1].Text += text
			line.Fragments[n-1].WidthPt += w
		} else {
			line.Fragments = append(line.Fragments, TextFragment{Run: run, Visual: resolveRunVisualPlan(run, fontScale, fontPt), Text: text, WidthPt: w, AscentPt: a, DescentPt: d})
		}
		line.WidthPt += w
		if a > line.AscentPt {
			line.AscentPt = a
		}
		if d > line.DescentPt {
			line.DescentPt = d
		}
		line.HeightPt = line.AscentPt + line.DescentPt
		if line.HeightPt <= 0 && h > line.HeightPt {
			line.HeightPt = h
		}
	}
	tokens := paragraphLayoutTokens(p)
	for tokenIndex, item := range tokens {
		run, token := item.run, item.text
		fontPt := runFontPt(run, p, fontScale)
		lineH := paragraphPhysicalLineHeightPt(fontPt, runFontFamily(run, p), metrics)
		switch token {
		case "\n":
			flush(true, "authored")
		case "\t":
			fieldWidth, decimalPrefix := followingTabFieldMetrics(tokens, tokenIndex+1, p, fontScale)
			appendFrag(run, token, nextAlignedTabAdvancePt(line.WidthPt, p, fontPt, fieldWidth, decimalPrefix), lineH)
		default:
			appendFrag(run, token, measureRunTextWidthPt(token, run, fontPt), lineH)
		}
	}
	flush(len(out.Lines) == 0, "")
	return out
}
