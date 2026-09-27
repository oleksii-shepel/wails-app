package mathtype

func normalize(e Expr) Expr {
	switch x := e.(type) {
	case *Row:
		out := make([]Expr, 0, len(x.Items))
		for _, it := range x.Items {
			it = normalize(it)
			if s, ok := it.(*Script); ok && s.Base == nil && !s.Leading && len(out) > 0 {
				s.Base = out[len(out)-1]
				out[len(out)-1] = s
			} else {
				out = append(out, it)
			}
		}
		x.Items = normalizeScriptedClosingFences(out)
	case *Fraction:
		x.Numerator = normalize(x.Numerator)
		x.Denominator = normalize(x.Denominator)
	case *Radical:
		x.Body = normalize(x.Body)
		x.Degree = normalize(x.Degree)
	case *Fence:
		x.Body = normalize(x.Body)
	case *Accent:
		x.Body = normalize(x.Body)
	case *UnderOver:
		x.Base = normalize(x.Base)
		x.Annotation = normalize(x.Annotation)
	case *Limit:
		x.Body = normalize(x.Body)
		x.Lower = normalize(x.Lower)
		x.Upper = normalize(x.Upper)
	case *LongDivision:
		x.Dividend = normalize(x.Dividend)
		x.Quotient = normalize(x.Quotient)
	case *Dirac:
		x.Left = normalize(x.Left)
		x.Right = normalize(x.Right)
	case *LargeOperator:
		x.OperatorExpr = normalize(x.OperatorExpr)
		x.Body = normalize(x.Body)
		x.Lower = normalize(x.Lower)
		x.Upper = normalize(x.Upper)
	case *Matrix:
		for i := range x.Cells {
			x.Cells[i] = normalize(x.Cells[i])
		}
	case *Pile:
		for i := range x.Lines {
			x.Lines[i] = normalize(x.Lines[i])
		}
	}
	return e
}

// normalizeScriptedClosingFences repairs the common MathType representation
// where a script follows a literal closing delimiter. MTEF then means the
// script applies to the complete parenthesized/bracketed expression, not to
// the closing glyph alone. Leaving it attached to the glyph also prevents the
// browser MathML stretcher from sizing the complementary delimiters together.
func normalizeScriptedClosingFences(items []Expr) []Expr {
	for i := 0; i < len(items); i++ {
		s, ok := items[i].(*Script)
		if !ok || s.Leading || s.Base == nil {
			continue
		}
		close, ok := bareDelimiter(s.Base)
		if !ok {
			continue
		}
		open, ok := matchingOpenDelimiter(close)
		if !ok {
			continue
		}

		depth := 0
		start := -1
		for j := i - 1; j >= 0; j-- {
			r, bare := bareDelimiter(items[j])
			if !bare {
				continue
			}
			if r == close {
				depth++
				continue
			}
			if r == open {
				if depth > 0 {
					depth--
					continue
				}
				start = j
				break
			}
		}
		if start < 0 {
			continue
		}

		bodyItems := append([]Expr(nil), items[start+1:i]...)
		var body Expr = &Row{Items: bodyItems}
		if len(bodyItems) == 1 {
			body = bodyItems[0]
		}
		s.Base = &Fence{Left: open, Right: close, Body: body}
		items = append(items[:start], append([]Expr{s}, items[i+1:]...)...)
		i = start - 1
	}
	return items
}

func bareDelimiter(e Expr) (rune, bool) {
	g, ok := e.(*Glyph)
	if !ok {
		return 0, false
	}
	switch g.Rune {
	case '(', ')', '[', ']', '{', '}':
		return g.Rune, true
	default:
		return 0, false
	}
}

func matchingOpenDelimiter(close rune) (rune, bool) {
	switch close {
	case ')':
		return '(', true
	case ']':
		return '[', true
	case '}':
		return '{', true
	default:
		return 0, false
	}
}
