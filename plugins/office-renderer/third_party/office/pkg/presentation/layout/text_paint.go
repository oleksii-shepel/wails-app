package layout

import ir "office/pkg/presentation/ir"

// ParagraphVisualPlan is the renderer-neutral physical paragraph contract.
// Layout resolves paragraph geometry, direction, spacing, tabs and marker paint;
// serializers only map these values to their output vocabulary.
type ParagraphVisualPlan struct {
	Metrics          ParagraphMetrics
	Align            string
	TextDirection    string
	FontAlign        string
	HasLineSpacing   bool
	FallbackIndentPt float64
	HasTabs          bool
	TabSizePt        float64
	TabStops         []ir.TabStop
	Marker           MarkerVisualPlan
	Lines            []PhysicalTextLine
	LinesMeasured    bool
}

type MarkerVisualPlan struct {
	Kind          string
	Text          string
	ImageData     []byte
	ImageMIME     string
	CounterKey    string
	NumberingType string
	Start         int
	FontSizePt    float64
	FontFamily    string
	Color         string
	GapPt         float64
	// Baseline-side extents and TopOffsetPt place the marker inside the first
	// physical line box using the same layout-owned baseline as text fragments.
	AscentPt, DescentPt float64
	TopOffsetPt         float64
	// OffsetPt is the authored first-line marker origin relative to the paragraph
	// left margin. It is materialized by layout so serializers do not infer
	// hanging-indent placement from CSS text-indent behavior.
	OffsetPt float64
}

// RunVisualPlan is the physical paint contract for a resolved text run. The
// canonical run remains the semantic text/field/hyperlink carrier; all visual
// decisions are projected here before serialization.
type RunVisualPlan struct {
	Bold, Italic, Underline, Strike  bool
	AllCaps, SmallCaps               bool
	Superscript, Subscript           bool
	BaselineShiftPct                 float64
	FontSizePt                       float64
	CharSpacingPt                    float64
	KernPt                           float64
	KerningSpecified, KerningEnabled bool
	FontFamily                       string
	Color, Highlight                 string
	Effects                          EffectVisualPlan
}

func ResolveParagraphVisualPlan(p *ir.Paragraph, frameWidthPt, fontScale, lineScale float64) ParagraphVisualPlan {
	if p == nil {
		return ParagraphVisualPlan{}
	}
	baseFontPt := 0.0
	baseFontSize := 0
	for _, run := range p.Runs {
		if run != nil && run.FontSize > 0 {
			baseFontSize = run.FontSize
			baseFontPt = float64(run.FontSize) / 100.0 * fontScale
			break
		}
	}
	plan := ParagraphVisualPlan{
		Metrics: ResolveParagraphMetrics(p, frameWidthPt, baseFontPt, lineScale),
		Align:   p.Align, TextDirection: p.TextDirection, FontAlign: p.FontAlign, HasLineSpacing: p.HasLineSpacing,
		TabStops: append([]ir.TabStop(nil), p.TabStops...),
	}
	indent := p.Level
	if p.BulletChar != "" || len(p.BulletImageData) != 0 || p.AutoNum {
		indent++
	}
	if !p.HasMarginLeft && !p.HasTextIndent && indent > 0 {
		// The historical renderer fallback was 2em per list level. Materialize
		// that distance in layout so wrapping and serialization share one width.
		if baseFontPt > 0 {
			plan.FallbackIndentPt = float64(indent) * 2 * baseFontPt
			plan.Metrics.MarginLeftPt += plan.FallbackIndentPt
			plan.Metrics.HasMarginLeft = true
			plan.Metrics.AvailableWidthPt = maxFloat(0, plan.Metrics.AvailableWidthPt-plan.FallbackIndentPt)
			plan.Metrics.FirstLineAvailableWidthPt = maxFloat(0, plan.Metrics.FirstLineAvailableWidthPt-plan.FallbackIndentPt)
		}
	}
	for _, run := range p.Runs {
		if run != nil && containsTab(run.Text) {
			plan.HasTabs = true
			break
		}
	}
	if plan.HasTabs {
		if p.DefaultTabSizeEMU > 0 {
			plan.TabSizePt = float64(p.DefaultTabSizeEMU) / 12700.0
		} else if len(p.TabStops) > 0 {
			plan.TabSizePt = float64(p.TabStops[0].Pos) / 12700.0
		}
	}
	markerFontFamily := p.BulletFontFamily
	// An unspecified bullet font follows the paragraph text font. Keep an
	// explicitly authored symbol font (Wingdings/Symbol/Webdings) intact; only
	// the absence of marker font ownership falls back to the first effective run.
	if p.BulletFontTx || markerFontFamily == "" {
		markerFontFamily = ""
		for _, run := range p.Runs {
			if run != nil && run.FontFamily != "" {
				markerFontFamily = run.FontFamily
				break
			}
		}
	}
	marker := MarkerVisualPlan{FontFamily: markerFontFamily, Color: p.BulletColor}
	if p.HasTextIndent {
		marker.OffsetPt = plan.Metrics.TextIndentPt
	}
	if marker.Color == "" {
		for _, run := range p.Runs {
			if run != nil && run.Color != "" {
				marker.Color = run.Color
				break
			}
		}
	}
	if p.BulletSizePct > 0 && baseFontSize > 0 {
		marker.FontSizePt = float64(baseFontSize) / 100.0 * (float64(p.BulletSizePct) / 100000.0) * fontScale
	} else if p.BulletSize > 0 {
		marker.FontSizePt = float64(p.BulletSize) / 100.0 * fontScale
	}
	if marker.FontSizePt <= 0 {
		marker.FontSizePt = baseFontPt
	}
	// Picture bullets are authored inside the paragraph's negative hanging-indent
	// region. Some converted PPTX files retain a large buSzPct value from the
	// source theme (37625 uses 113.071%), but PowerPoint still keeps the bitmap
	// comfortably inside that marker column. Rendering the percentage literally
	// can make the image wider than the entire hanging region. Cap only oversized
	// picture markers, leaving 20% of the authored region for visual separation
	// from the first text glyph. Character and autonumber markers remain exact.
	if len(p.BulletImageData) != 0 && p.HasTextIndent && plan.Metrics.TextIndentPt < 0 {
		hangingPt := -plan.Metrics.TextIndentPt
		if maxPicturePt := hangingPt * 0.8; maxPicturePt > 0 && marker.FontSizePt > maxPicturePt {
			marker.FontSizePt = maxPicturePt
		}
	}
	if marker.FontSizePt > 0 {
		marker.GapPt = measurePlainTextWidthPt(" ", marker.FontSizePt, marker.FontFamily, false, false)
		// DrawingML list paragraphs commonly use a negative text indent as a
		// hanging marker region (for example marL=36pt, indent=-30pt). The
		// marker belongs inside that authored region; it must not consume text
		// width a second time. Materialize the remaining hanging distance as
		// marker gap so the first text glyph starts at the paragraph margin.
		if p.HasTextIndent && plan.Metrics.TextIndentPt < 0 {
			markerText := p.BulletChar
			if markerText == "" && p.AutoNum {
				start := p.AutoNumStart
				if start <= 0 {
					start = 1
				}
				markerText = formatAutoNumForLayout(p.AutoNumType, start)
			}
			markerWidthPt := measurePlainTextWidthPt(markerText, marker.FontSizePt, marker.FontFamily, false, false)
			if len(p.BulletImageData) != 0 {
				markerWidthPt = marker.FontSizePt
			}
			hangingGapPt := -plan.Metrics.TextIndentPt - markerWidthPt
			if hangingGapPt > marker.GapPt {
				marker.GapPt = hangingGapPt
			}
		}
	}
	if len(p.BulletImageData) != 0 {
		marker.Kind, marker.ImageData, marker.ImageMIME = "image", append([]byte(nil), p.BulletImageData...), p.BulletImageMIME
	} else if p.BulletChar != "" {
		marker.Kind, marker.Text = "bullet", p.BulletChar
	} else if p.AutoNum {
		marker.Kind = "auto"
		marker.NumberingType = p.AutoNumType
		marker.CounterKey = p.AutoNumType + "-" + itoaSmall(p.Level)
		marker.Start = p.AutoNumStart
		if marker.Start <= 0 {
			marker.Start = 1
		}
	}
	plan.Marker = marker
	if CanLayoutParagraphLines(p, plan.Metrics, fontScale) {
		markerText := ""
		markerFontPt := marker.FontSizePt
		if marker.Kind == "bullet" {
			markerText = marker.Text
		} else if marker.Kind == "image" {
			markerText = "■"
		} else if marker.Kind == "auto" {
			// Exact numbering remains stateful, but width ownership belongs to
			// layout. Use the resolved start value as the physical representative.
			markerText = formatAutoNumForLayout(marker.NumberingType, marker.Start)
		}
		lines := layoutParagraphLinesWithMarker(p, plan.Metrics, fontScale, markerText, markerFontPt, plan.Marker.FontFamily, plan.Marker.GapPt)
		plan.Lines = append([]PhysicalTextLine(nil), lines.Lines...)
		plan.LinesMeasured = true
		materializeMarkerBaselinePlacement(&plan)
		materializePercentageParagraphSpacing(&plan)
	}
	return plan
}

func materializeMarkerBaselinePlacement(plan *ParagraphVisualPlan) {
	if plan == nil || plan.Marker.Kind == "" || plan.Marker.FontSizePt <= 0 || len(plan.Lines) == 0 {
		return
	}
	a, d := paragraphPhysicalBaselineExtentsPt(plan.Marker.FontSizePt, plan.Marker.FontFamily, plan.Metrics)
	plan.Marker.AscentPt, plan.Marker.DescentPt = a, d
	plan.Marker.TopOffsetPt = physicalBaselineTopOffsetPt(plan.Lines[0].AscentPt, a, d, plan.Marker.FontSizePt, plan.Marker.FontFamily)
}

func ResolveRunVisualPlan(run *ir.Run, fontScale float64) RunVisualPlan {
	return resolveRunVisualPlan(run, fontScale, 0)
}

// resolveRunVisualPlan materializes the same effective physical font size used
// by line measurement. effectiveFontPt is supplied by paragraph layout when a
// sparse run inherits its size from the paragraph; zero keeps the standalone
// authored-run behavior used by callers that have no paragraph context.
func resolveRunVisualPlan(run *ir.Run, fontScale, effectiveFontPt float64) RunVisualPlan {
	if run == nil {
		return RunVisualPlan{}
	}
	plan := RunVisualPlan{
		Bold: run.Bold, Italic: run.Italic, Underline: run.Underline, Strike: run.Strike,
		AllCaps: run.AllCaps, SmallCaps: run.SmallCaps, Superscript: run.Superscript, Subscript: run.Subscript, BaselineShiftPct: run.BaselineShiftPct,
		FontFamily: run.FontFamily, Color: run.Color, Highlight: run.Highlight,
		CharSpacingPt: float64(run.CharSpacing) / 100.0, KernPt: float64(run.Kern) / 100.0,
		Effects: ResolveEffectVisualPlan(run.Effects),
	}
	if effectiveFontPt > 0 {
		plan.FontSizePt = effectiveFontPt
	} else if run.FontSize > 0 {
		plan.FontSizePt = float64(run.FontSize) / 100.0 * fontScale
	}
	// DrawingML rPr@kern is a minimum font-size threshold, not an amount of
	// spacing. Resolve that authored condition once in physical layout so the
	// renderer does not reinterpret text metrics. A zero threshold means the
	// property was not authored/materialized.
	if run.Kern > 0 {
		plan.KerningSpecified = true
		plan.KerningEnabled = plan.FontSizePt > 0 && plan.FontSizePt >= plan.KernPt
	}
	return plan
}

func containsTab(s string) bool {
	for _, r := range s {
		if r == '\t' {
			return true
		}
	}
	return false
}
func itoaSmall(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoaSmall(-n)
	}
	b := [20]byte{}
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func formatAutoNumForLayout(kind string, n int) string {
	if n <= 0 {
		n = 1
	}
	switch kind {
	case "arabicParenR":
		return itoaSmall(n) + ")"
	case "alphaLcPeriod":
		return string(rune('a'+(n-1)%26)) + "."
	case "alphaUcPeriod":
		return string(rune('A'+(n-1)%26)) + "."
	default:
		return itoaSmall(n) + "."
	}
}

// ResolveNoWrapParagraphVisualPlan preserves authored line breaks and tabs while
// suppressing calculated wrapping. noWrap is therefore still a physical line
// contract rather than a request for the browser to rediscover source lines.
func ResolveNoWrapParagraphVisualPlan(p *ir.Paragraph, plan ParagraphVisualPlan, fontScale float64) ParagraphVisualPlan {
	if p == nil {
		return plan
	}
	markerText := ""
	markerFontPt := plan.Marker.FontSizePt
	if plan.Marker.Kind == "bullet" {
		markerText = plan.Marker.Text
	}
	if plan.Marker.Kind == "auto" {
		markerText = formatAutoNumForLayout(plan.Marker.NumberingType, plan.Marker.Start)
	}
	lines := layoutParagraphAuthoredLinesWithMarker(p, plan.Metrics, fontScale, markerText, markerFontPt, plan.Marker.FontFamily, plan.Marker.GapPt)
	plan.Lines = append([]PhysicalTextLine(nil), lines.Lines...)
	plan.LinesMeasured = true
	materializeMarkerBaselinePlacement(&plan)
	materializePercentageParagraphSpacing(&plan)
	return plan
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// materializePercentageParagraphSpacing resolves percentage paragraph spacing
// against the physical boundary line it belongs to. This matters for mixed-font
// paragraphs: the first/last visual line can be taller than the paragraph's first
// run, and PowerPoint percentage spacing is relative to text line height rather
// than an arbitrary base run.
func materializePercentageParagraphSpacing(plan *ParagraphVisualPlan) {
	if plan == nil || !plan.LinesMeasured || len(plan.Lines) == 0 {
		return
	}
	if plan.Metrics.SpaceBeforeScale > 0 {
		plan.Metrics.HasSpaceBefore = true
		plan.Metrics.SpaceBeforePt = plan.Lines[0].HeightPt * plan.Metrics.SpaceBeforeScale
	}
	if plan.Metrics.SpaceAfterScale > 0 {
		plan.Metrics.HasSpaceAfter = true
		plan.Metrics.SpaceAfterPt = plan.Lines[len(plan.Lines)-1].HeightPt * plan.Metrics.SpaceAfterScale
	}
}
