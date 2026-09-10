package layout

import (
	"math"
	"sort"
	"strings"
)

// lineSpan is an available horizontal interval for a physical line band.
type lineSpan struct {
	Left, Right float64
}

func (s lineSpan) Width() float64 {
	if s.Right <= s.Left {
		return 0
	}
	return s.Right - s.Left
}

// lineBand is the canonical horizontal geometry of one physical text line.
// Spans, rather than a scalar width, are authoritative.
type lineBand struct {
	Top, Bottom float64
	Spans       []lineSpan
	// Constrained is true when a float exclusion changed the horizontal text
	// region for this band, including ordinary one-sided Square/Tight wrapping.
	Constrained bool
}

// resolveLineBand is the single boundary from page geometry + float exclusions
// to physical line geometry. Measurement and pagination consume this result
// rather than independently calculating available widths.
func resolveLineBand(lineLeft, lineRight, lineTop, lineBottom float64, exclusions []lineExclusion) lineBand {
	spans := lineSpansForBand(lineLeft, lineRight, lineTop, lineBottom, exclusions)
	constrained := len(spans) != 1
	if len(spans) == 1 {
		constrained = math.Abs(spans[0].Left-lineLeft) > 0.01 || math.Abs(spans[0].Right-lineRight) > 0.01
	}
	return lineBand{
		Top:         lineTop,
		Bottom:      lineBottom,
		Spans:       spans,
		Constrained: constrained,
	}
}

// paragraphLinePlan is the geometry contract shared by measurement and
// pagination. It owns the vertical bands and all horizontal text regions for
// those bands, so callers no longer need to recompute float intersections.
type paragraphLinePlan struct {
	Bands []lineBand
}

func (p paragraphLinePlan) TotalHeight() float64 {
	if len(p.Bands) == 0 {
		return 0
	}
	return p.Bands[len(p.Bands)-1].Bottom - p.Bands[0].Top
}

func planParagraphLineBands(firstLeft, firstRight, continuationLeft, continuationRight, startY float64, lineHeights []float64, fallbackHeight float64, exclusions []lineExclusion) paragraphLinePlan {
	if len(lineHeights) == 0 {
		lineHeights = []float64{fallbackHeight}
	}
	bands := make([]lineBand, 0, len(lineHeights))
	y := startY
	for i, h := range lineHeights {
		if h <= 0 {
			h = fallbackHeight
		}
		if h <= 0 {
			h = 1
		}
		left, right := continuationLeft, continuationRight
		if i == 0 {
			left, right = firstLeft, firstRight
		}
		bands = append(bands, resolveLineBand(left, right, y, y+h, exclusions))
		y += h
	}
	return paragraphLinePlan{Bands: bands}
}

func lineSpansForBand(lineLeft, lineRight, lineTop, lineBottom float64, exclusions []lineExclusion) []lineSpan {
	if lineRight <= lineLeft {
		return nil
	}
	allowed := []lineSpan{{lineLeft, lineRight}}
	for _, e := range exclusions {
		if e.bottom <= lineTop || e.top >= lineBottom {
			continue
		}
		if e.fullWidth {
			return nil
		}
		blocked := clipLineSpans(exclusionIntervalsForBand(e, lineTop, lineBottom), lineLeft, lineRight)
		if len(blocked) == 0 {
			continue
		}
		allowed = intersectLineSpans(allowed, allowedSpansForExclusion(lineLeft, lineRight, blocked, e.wrapSide))
		if len(allowed) == 0 {
			return nil
		}
	}
	return allowed
}

func widestLineSpan(spans []lineSpan) (lineSpan, bool) {
	if len(spans) == 0 {
		return lineSpan{}, false
	}
	best := spans[0]
	for _, s := range spans[1:] {
		if s.Width() > best.Width() {
			best = s
		}
	}
	return best, best.Width() > 0
}

func exclusionIntervalsForBand(e lineExclusion, top, bottom float64) []lineSpan {
	if len(e.polygon) < 3 {
		return []lineSpan{{e.left, e.right}}
	}
	v := exactPolygonIntervalsAtBand(e.polygon, top, bottom)
	for i := range v {
		v[i].Left -= e.padLeft
		v[i].Right += e.padRight
	}
	return mergeLineSpans(v)
}

// exactPolygonIntervalsAtBand projects the polygon over the complete vertical
// line band. Critical slabs are split at every polygon vertex; within a slab
// polygon topology is constant and edge extrema are linear.
func exactPolygonIntervalsAtBand(poly [][2]float64, top, bottom float64) []lineSpan {
	if len(poly) < 3 || bottom <= top {
		return nil
	}
	ys := []float64{top, bottom}
	for _, p := range poly {
		if p[1] > top && p[1] < bottom {
			ys = append(ys, p[1])
		}
	}
	sort.Float64s(ys)
	ys = uniqueFloats(ys)
	projected := make([]lineSpan, 0, len(ys)*4)
	for i := 0; i+1 < len(ys); i++ {
		a, b := ys[i], ys[i+1]
		if b-a <= 1e-9 {
			continue
		}
		eps := math.Min((b-a)*1e-7, 1e-7)
		for _, y := range []float64{a + eps, b - eps, (a + b) / 2} {
			for _, iv := range polygonIntervalsAtY(poly, y) {
				projected = append(projected, lineSpan{iv[0], iv[1]})
			}
		}
		for e := 0; e < len(poly); e++ {
			for _, p := range clippedEdgeEndpoints(poly[e], poly[(e+1)%len(poly)], a, b) {
				projected = append(projected, lineSpan{p[0], p[0]})
			}
		}
	}
	return mergeProjectedLineSpans(projected)
}

func clippedEdgeEndpoints(a, b [2]float64, top, bottom float64) [][2]float64 {
	minY, maxY := math.Min(a[1], b[1]), math.Max(a[1], b[1])
	if maxY < top || minY > bottom {
		return nil
	}
	out := make([][2]float64, 0, 2)
	for _, y := range []float64{math.Max(minY, top), math.Min(maxY, bottom)} {
		if math.Abs(b[1]-a[1]) < 1e-12 {
			return append(out, a, b)
		}
		t := (y - a[1]) / (b[1] - a[1])
		if t >= 0 && t <= 1 {
			out = append(out, [2]float64{a[0] + t*(b[0]-a[0]), y})
		}
	}
	return out
}

func mergeProjectedLineSpans(spans []lineSpan) []lineSpan {
	if len(spans) == 0 {
		return nil
	}
	positive := make([]lineSpan, 0, len(spans))
	points := make([]float64, 0)
	for _, s := range spans {
		if s.Right > s.Left {
			positive = append(positive, s)
		} else {
			points = append(points, s.Left)
		}
	}
	positive = mergeLineSpans(positive)
	if len(positive) == 0 {
		return nil
	}
	for _, x := range points {
		best := -1
		d := math.Inf(1)
		for i, s := range positive {
			v := 0.0
			if x < s.Left {
				v = s.Left - x
			} else if x > s.Right {
				v = x - s.Right
			}
			if v < d {
				best, d = i, v
			}
		}
		if best >= 0 && d < 1e-4 {
			positive[best].Left = math.Min(positive[best].Left, x)
			positive[best].Right = math.Max(positive[best].Right, x)
		}
	}
	return mergeLineSpans(positive)
}

func allowedSpansForExclusion(left, right float64, blocked []lineSpan, side string) []lineSpan {
	if len(blocked) == 0 {
		return []lineSpan{{left, right}}
	}
	l, r := blocked[0].Left, blocked[len(blocked)-1].Right
	switch strings.ToLower(strings.TrimSpace(side)) {
	case "left":
		if l <= left {
			return nil
		}
		return []lineSpan{{left, l}}
	case "right":
		if r >= right {
			return nil
		}
		return []lineSpan{{r, right}}
	case "largest":
		a, b := lineSpan{left, l}, lineSpan{r, right}
		if b.Width() > a.Width() {
			if b.Width() <= 0 {
				return nil
			}
			return []lineSpan{b}
		}
		if a.Width() <= 0 {
			return nil
		}
		return []lineSpan{a}
	default:
		return complementLineSpans(left, right, blocked)
	}
}

func clipLineSpans(spans []lineSpan, left, right float64) []lineSpan {
	out := make([]lineSpan, 0, len(spans))
	for _, s := range spans {
		s.Left = math.Max(left, s.Left)
		s.Right = math.Min(right, s.Right)
		if s.Right > s.Left {
			out = append(out, s)
		}
	}
	return mergeLineSpans(out)
}

func complementLineSpans(left, right float64, blocked []lineSpan) []lineSpan {
	blocked = clipLineSpans(blocked, left, right)
	out := make([]lineSpan, 0, len(blocked)+1)
	cur := left
	for _, s := range blocked {
		if s.Left > cur {
			out = append(out, lineSpan{cur, s.Left})
		}
		cur = math.Max(cur, s.Right)
	}
	if cur < right {
		out = append(out, lineSpan{cur, right})
	}
	return out
}

func intersectLineSpans(a, b []lineSpan) []lineSpan {
	out := make([]lineSpan, 0, min(len(a), len(b)))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		s, e := math.Max(a[i].Left, b[j].Left), math.Min(a[i].Right, b[j].Right)
		if e > s {
			out = append(out, lineSpan{s, e})
		}
		if a[i].Right < b[j].Right {
			i++
		} else {
			j++
		}
	}
	return out
}

func mergeLineSpans(spans []lineSpan) []lineSpan {
	if len(spans) == 0 {
		return nil
	}
	sort.Slice(spans, func(i, j int) bool {
		if spans[i].Left == spans[j].Left {
			return spans[i].Right < spans[j].Right
		}
		return spans[i].Left < spans[j].Left
	})
	out := make([]lineSpan, 0, len(spans))
	for _, s := range spans {
		if s.Right <= s.Left {
			continue
		}
		if len(out) == 0 || s.Left > out[len(out)-1].Right+1e-9 {
			out = append(out, s)
		} else if s.Right > out[len(out)-1].Right {
			out[len(out)-1].Right = s.Right
		}
	}
	return out
}

func uniqueFloats(v []float64) []float64 {
	if len(v) == 0 {
		return nil
	}
	out := v[:1]
	for _, x := range v[1:] {
		if math.Abs(x-out[len(out)-1]) > 1e-9 {
			out = append(out, x)
		}
	}
	return out
}
