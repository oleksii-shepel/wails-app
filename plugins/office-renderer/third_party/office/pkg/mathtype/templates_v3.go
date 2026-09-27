package mathtype

func at(xs []Expr, n int) Expr {
	if n >= 0 && n < len(xs) {
		return xs[n]
	}
	return nil
}

// v3Template maps the published MTEF3 selector table to the canonical math IR.
// Slot indexes follow the specification's template-subobject order.  Deleted
// slots can be represented by NULL LINE records, so keeping positional slots is
// important even when a slot has no visible content.
func v3Template(s, v, o byte, xs []Expr, st Style) Expr {
	switch {
	case s <= 12: // ParBoxClass: main slot + optional authored fence chars
		l, r := templateFences(s, v, xs)
		return &Fence{Body: at(xs, 0), Left: l, Right: r, Alignment: o, Style: st}
	case s == 13: // RootBoxClass: main slot, radicand/index slot
		if v == 1 {
			return &Radical{Body: at(xs, 0), Degree: at(xs, 1), Style: st}
		}
		return &Radical{Body: at(xs, 0), Style: st}
	case s == 14:
		return &Fraction{Numerator: at(xs, 0), Denominator: at(xs, 1), Compact: v == 1, Style: st}
	case s == 15 || s == 44: // ScrBoxClass
		sc := &Script{Leading: s == 44, Style: st}
		// ScrBoxClass has two positional subobjects: subscript, then
		// superscript. MathType 3.5 preserves a deleted slot as a NULL
		// LINE, so a superscript-only template is commonly [NULL, sup].
		// Older Equation Editor streams seen in the wild can omit the NULL
		// slot, so retain the one-slot form as a compatibility fallback.
		switch v {
		case 0:
			if len(xs) > 1 {
				sc.Sup = at(xs, 1)
			} else {
				sc.Sup = at(xs, 0)
			}
		case 1:
			sc.Sub = at(xs, 0)
		default:
			sc.Sub, sc.Sup = at(xs, 0), at(xs, 1)
		}
		return sc
	case s == 16:
		return &Accent{Body: at(xs, 0), Kind: EmbellMidBar, Under: true, Style: st}
	case s == 17:
		return &Accent{Body: at(xs, 0), Kind: EmbellOverBar, Style: st}
	case s >= 18 && s <= 20: // ArroBoxClass: main slot + authored arrow char
		kind := Embellishment(EmbellLeftArrow)
		if s == 19 {
			kind = EmbellRightArrow
		} else if s == 20 {
			kind = EmbellBothArrow
		}
		op := rune(0)
		if g := firstGlyph(at(xs, 1)); g != nil {
			op = g.Rune
		}
		return &Accent{Body: at(xs, 0), Kind: kind, Operator: op, Under: v == 1, Style: st}
	case s >= 21 && s <= 26:
		upper, lower := bigOperatorLimits(s, v, xs)
		op := integralRune(s, v)
		// BigOpBoxClass stores the authored large-operator character as its
		// fourth subobject. Prefer it when present; selector/variation remains
		// the fallback for streams that omit the character slot.
		if g := firstGlyph(at(xs, 3)); g != nil {
			op = g.Rune
		}
		return &LargeOperator{Operator: op, Body: at(xs, 0), Upper: upper, Lower: lower, Expand: o == 1, LimitsAboveBelow: s >= 24, Style: st}
	case s == 27 || s == 28: // horizontal brace: main, annotation, authored brace char
		op := '⏞'
		if g := firstGlyph(at(xs, 2)); g != nil {
			op = g.Rune
		}
		return &UnderOver{Base: at(xs, 0), Annotation: at(xs, 1), Under: s == 28, Operator: op, Style: st}
	case s >= 29 && s <= 38:
		upper, lower := bigOperatorLimits(s, v, xs)
		op := bigOperatorRune(s)
		if g := firstGlyph(at(xs, 3)); g != nil {
			op = g.Rune
		}
		return &LargeOperator{Operator: op, Body: at(xs, 0), Upper: upper, Lower: lower, LimitsAboveBelow: s%2 == 1, Style: st}
	case s == 39: // LimBoxClass: main, lower, upper
		lower, upper := at(xs, 1), at(xs, 2)
		switch v {
		case 0: // upper only
			lower = nil
		case 1: // lower only
			upper = nil
		}
		return &Limit{Body: at(xs, 0), Lower: lower, Upper: upper, Style: st}
	case s == 40: // LDivBoxClass: dividend, quotient
		q := at(xs, 1)
		if v == 1 { // tvLDIVWO: quotient/upper slot deleted
			q = nil
		}
		return &LongDivision{Dividend: at(xs, 0), Quotient: q, Style: st}
	case s == 41:
		return &Fraction{Numerator: at(xs, 0), Denominator: at(xs, 1), Slash: true, SlashVariant: v, Style: st}
	case s == 42 || s == 43: // user-supplied big operator is final structured slot
		opExpr := at(xs, 3)
		op := rune(0)
		if g, ok := soleGlyph(opExpr); ok {
			op = g.Rune
			opExpr = nil
		}
		upper, lower := bigOperatorLimits(s, v, xs)
		return &LargeOperator{Operator: op, OperatorExpr: opExpr, Body: at(xs, 0), Upper: upper, Lower: lower, LimitsAboveBelow: s == 43, Style: st}
	case s == 45: // DiracBoxClass: slots followed by optional authored delimiters
		lf, mid, rf := diracFences(v, xs)
		return &Dirac{Left: at(xs, 0), Right: at(xs, 1), LeftFence: lf, Middle: mid, RightFence: rf, HasLeft: v != 2, HasRight: v != 1, Style: st}
	case s == 46 || s == 47:
		kind := Embellishment(EmbellLeftArrow)
		if v == 1 {
			kind = EmbellRightArrow
		} else if v == 2 {
			kind = EmbellBothArrow
		}
		return &Accent{Body: at(xs, 0), Kind: kind, Under: s == 46, Style: st}
	case s == 48:
		return &Accent{Body: at(xs, 0), Kind: EmbellFrown, Style: st}
	default:
		return &Unsupported{Selector: s, Variation: v}
	}
}

func templateFences(s, v byte, xs []Expr) (rune, rune) {
	l, r := fences(s, v)
	// ParBoxClass stores the main slot first and then only the fence
	// characters that are present for the selected variation. Preserve those
	// authored characters; selector-derived delimiters are only a fallback.
	switch v {
	case 0:
		if g := firstGlyph(at(xs, 1)); g != nil && isSemanticDelimiterRune(g.Rune) {
			l = g.Rune
		}
		if g := firstGlyph(at(xs, 2)); g != nil && isSemanticDelimiterRune(g.Rune) {
			r = g.Rune
		}
	case 1:
		if g := firstGlyph(at(xs, 1)); g != nil && isSemanticDelimiterRune(g.Rune) {
			l = g.Rune
		}
	case 2:
		if g := firstGlyph(at(xs, 1)); g != nil && isSemanticDelimiterRune(g.Rune) {
			r = g.Rune
		}
	}
	return l, r
}

// MTEF3 uses layout-font code points such as e/f in fence-character slots.
// They are not literal operators: selector/variation supplies their semantic
// delimiter. Preserve only a real delimiter glyph as an authored override.
func isSemanticDelimiterRune(r rune) bool {
	if r > 127 {
		return true
	}
	switch r {
	case '(', ')', '[', ']', '{', '}', '|', '‖', '⌊', '⌋', '⌈', '⌉', '⟨', '⟩':
		return true
	}
	return false
}

func diracFences(v byte, xs []Expr) (left, middle, right rune) {
	left, middle, right = '⟨', '|', '⟩'
	// The first two objects are always the left/right content slots. Optional
	// delimiter characters then occur in stream order.
	i := 2
	if v != 2 {
		if g := firstGlyph(at(xs, i)); g != nil {
			left = g.Rune
		}
		i++
	}
	if g := firstGlyph(at(xs, i)); g != nil {
		middle = g.Rune
	}
	i++
	if v != 1 {
		if g := firstGlyph(at(xs, i)); g != nil {
			right = g.Rune
		}
	}
	return
}

func firstGlyph(e Expr) *Glyph {
	switch x := e.(type) {
	case *Glyph:
		return x
	case *Row:
		for _, it := range x.Items {
			if g := firstGlyph(it); g != nil {
				return g
			}
		}
	}
	return nil
}

// soleGlyph distinguishes BigOp's glyph slot from its structured-expression
// slot. firstGlyph is intentionally broader for template delimiters.
func soleGlyph(e Expr) (*Glyph, bool) {
	switch x := e.(type) {
	case *Glyph:
		return x, true
	case *Row:
		if len(x.Items) == 1 {
			return soleGlyph(x.Items[0])
		}
	}
	return nil, false
}

func bigOperatorRune(s byte) rune {
	switch s {
	case 29, 30:
		return '∑'
	case 31, 32:
		return '∏'
	case 33, 34:
		return '∐'
	case 35, 36:
		return '⋃'
	case 37, 38:
		return '⋂'
	}
	return 0
}

func fences(s, v byte) (rune, rune) {
	pairs := [][2]rune{{'⟨', '⟩'}, {'(', ')'}, {'{', '}'}, {'[', ']'}, {'|', '|'}, {'‖', '‖'}, {'⌊', '⌋'}, {'⌈', '⌉'}, {'{', '{'}, {'}', '}'}, {'}', '{'}, {'{', ')'}, {'(', '}'}}
	p := pairs[int(s)]
	if v == 1 {
		p[1] = 0
	}
	if v == 2 {
		p[0] = 0
	}
	return p[0], p[1]
}
func bigOperatorLimits(s, v byte, xs []Expr) (upper, lower Expr) {
	upperSlot, lowerSlot := at(xs, 1), at(xs, 2)
	switch {
	case s == 21: // single integral: none, lower, both, contour-none, contour-lower
		switch v {
		case 1, 4:
			return nil, lowerSlot
		case 2:
			return upperSlot, lowerSlot
		default:
			return nil, nil
		}
	case s == 22 || s == 23: // double/triple: none or lower (plain/area/volume)
		if v == 1 || v == 3 {
			return nil, lowerSlot
		}
		return nil, nil
	case s == 24: // summation-style single integral: both, lower, contour-lower
		if v == 0 {
			return upperSlot, lowerSlot
		}
		return nil, lowerSlot
	case s == 25 || s == 26: // summation-style double/triple always have lower limit
		return nil, lowerSlot
	case s >= 29 && s <= 38:
		// Odd selectors support lower/both/none. Integral-style even selectors
		// support lower/both. In both families variation 0 is lower-only and 1
		// is both.
		switch v {
		case 0:
			return nil, lowerSlot
		case 1:
			return upperSlot, lowerSlot
		default:
			return nil, nil
		}
	case s == 42 || s == 43: // custom operator: upper, lower, both
		// MTEF3/4 stores these limits in source order (lower, upper), unlike
		// the built-in BigOp templates above.
		lowerSlot, upperSlot := at(xs, 1), at(xs, 2)
		switch v {
		case 0:
			return upperSlot, nil
		case 1:
			return nil, lowerSlot
		case 2:
			return upperSlot, lowerSlot
		}
	}
	return upperSlot, lowerSlot
}

func integralRune(s, v byte) rune {
	switch s {
	case 21:
		if v == 3 || v == 4 {
			return '∮'
		}
		return '∫'
	case 22:
		if v == 2 || v == 3 {
			return '∯'
		}
		return '∬'
	case 23:
		if v == 2 || v == 3 {
			return '∰'
		}
		return '∭'
	case 24:
		if v == 2 {
			return '∮'
		}
		return '∫'
	case 25:
		if v == 0 {
			return '∯'
		}
		return '∬'
	case 26:
		if v == 0 {
			return '∰'
		}
		return '∭'
	}
	return '∫'
}
func applyEmbellishments(g *Glyph) Expr {
	var e Expr = g
	for _, x := range g.Embellishments {
		e = &Accent{Body: e, Kind: x, Style: g.Style}
	}
	return e
}
