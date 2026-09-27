package mathtype

// resolveEquationStyles binds MTEF typeface references to the FONT records
// collected while decoding. Built-in/negative typeface ids remain semantic
// typeface ids; only explicit FONT definitions become concrete font families.
func resolveEquationStyles(eq *Equation) {
	if eq == nil || eq.Root == nil || len(eq.Fonts) == 0 {
		return
	}
	var walk func(Expr)
	apply := func(st *Style) {
		if st == nil || st.Font != "" {
			return
		}
		if f, ok := eq.Fonts[st.Typeface]; ok {
			st.Font = f.Name
			st.Bold = st.Bold || f.Bold
			st.Italic = st.Italic || f.Italic
		}
	}
	walk = func(e Expr) {
		switch x := e.(type) {
		case *Row:
			apply(&x.Style)
			for _, c := range x.Items {
				walk(c)
			}
		case *Glyph:
			apply(&x.Style)
		case *Fraction:
			apply(&x.Style)
			walk(x.Numerator)
			walk(x.Denominator)
		case *Script:
			apply(&x.Style)
			walk(x.Base)
			walk(x.Sub)
			walk(x.Sup)
		case *Radical:
			apply(&x.Style)
			walk(x.Body)
			walk(x.Degree)
		case *Fence:
			apply(&x.Style)
			walk(x.Body)
		case *Accent:
			apply(&x.Style)
			walk(x.Body)
		case *LargeOperator:
			apply(&x.Style)
			walk(x.OperatorExpr)
			walk(x.Body)
			walk(x.Lower)
			walk(x.Upper)
		case *UnderOver:
			apply(&x.Style)
			walk(x.Base)
			walk(x.Annotation)
		case *Limit:
			apply(&x.Style)
			walk(x.Body)
			walk(x.Lower)
			walk(x.Upper)
		case *LongDivision:
			apply(&x.Style)
			walk(x.Dividend)
			walk(x.Quotient)
		case *Dirac:
			apply(&x.Style)
			walk(x.Left)
			walk(x.Right)
		case *Matrix:
			apply(&x.Style)
			for _, c := range x.Cells {
				walk(c)
			}
		case *Pile:
			apply(&x.Style)
			for _, c := range x.Lines {
				walk(c)
			}
		}
	}
	walk(eq.Root)
}
