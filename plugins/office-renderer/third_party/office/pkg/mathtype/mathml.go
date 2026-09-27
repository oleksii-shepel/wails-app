package mathtype

import (
	"fmt"
	"html"
	"strconv"
	"strings"
	"unicode"
)

func ToMathML(eq *Equation) (string, error) {
	if eq == nil || eq.Root == nil || !HasVisibleContent(eq.Root) {
		return "", ErrNoVisibleContent
	}
	var b strings.Builder
	b.WriteString(`<math xmlns="http://www.w3.org/1998/Math/MathML" display="block"><mstyle displaystyle="true" scriptlevel="0">`)
	writeMath(&b, eq.Root)
	b.WriteString(`</mstyle></math>`)
	return b.String(), nil
}
func MathML(data []byte) (string, error) {
	e, err := Parse(data)
	if err != nil {
		return "", err
	}
	return ToMathML(e)
}
func writeMath(b *strings.Builder, e Expr) {
	if e == nil {
		b.WriteString(`<mrow/>`)
		return
	}
	if st, ok := exprStyle(e); ok && st.Nudge != nil && (st.Nudge.X != 0 || st.Nudge.Y != 0) {
		// MTEF3 dimensional values, including nudges, are in 1/32 point.
		// mpadded keeps the offset local to this semantic object instead of
		// translating the complete equation box.
		b.WriteString(`<mpadded`)
		if st.Nudge.X != 0 {
			b.WriteString(` lspace="` + points32(*st.Nudge, true) + `"`)
		}
		if st.Nudge.Y != 0 {
			b.WriteString(` voffset="` + points32(*st.Nudge, false) + `"`)
		}
		b.WriteString(`>`)
		writeMathCore(b, e)
		b.WriteString(`</mpadded>`)
		return
	}
	writeMathCore(b, e)
}

func writeMathCore(b *strings.Builder, e Expr) {
	switch x := e.(type) {
	case *Row:
		b.WriteString(`<mrow>`)
		for _, i := range x.Items {
			writeMath(b, i)
		}
		b.WriteString(`</mrow>`)
	case *Glyph:
		// MathML token whitespace is collapsed by browsers. MTEF uses ordinary
		// space glyphs for authored gaps inside mixed text/math expressions (for
		// example, "Delay    e…"), so preserve each one as an explicit math
		// advance instead of emitting an empty <mi>.
		if unicode.IsSpace(x.Rune) {
			b.WriteString(`<mspace width="0.25em"/>`)
			return
		}
		tag := "mi"
		if x.Rune >= '0' && x.Rune <= '9' {
			tag = "mn"
		}
		if strings.ContainsRune("=+-−×÷<>≤≥≈≠±∓·⋅,;:()[]{}|∫∬∭∮∑∏⋃⋂", x.Rune) {
			tag = "mo"
		}
		fmt.Fprintf(b, "<%s", tag)
		writeStyle(b, x.Style)
		b.WriteString(">" + html.EscapeString(string(x.Rune)) + "</" + tag + ">")
	case *Fraction:
		if x.Slash {
			if x.SlashVariant == 2 {
				b.WriteString(`<mstyle scriptlevel="+1">`)
			}
			b.WriteString(`<mrow>`)
			writeMath(b, x.Numerator)
			b.WriteString(`<mo>/</mo>`)
			writeMath(b, x.Denominator)
			b.WriteString(`</mrow>`)
			if x.SlashVariant == 2 {
				b.WriteString(`</mstyle>`)
			}
		} else {
			if x.Compact {
				b.WriteString(`<mstyle scriptlevel="+1">`)
			}
			b.WriteString(`<mfrac>`)
			writeMath(b, x.Numerator)
			writeMath(b, x.Denominator)
			b.WriteString(`</mfrac>`)
			if x.Compact {
				b.WriteString(`</mstyle>`)
			}
		}
	case *Script:
		if x.Sub != nil && x.Sup != nil {
			b.WriteString(`<msubsup>`)
			writeMath(b, x.Base)
			writeMath(b, x.Sub)
			writeMath(b, x.Sup)
			b.WriteString(`</msubsup>`)
		} else if x.Sub != nil {
			b.WriteString(`<msub>`)
			writeMath(b, x.Base)
			writeMath(b, x.Sub)
			b.WriteString(`</msub>`)
		} else {
			b.WriteString(`<msup>`)
			writeMath(b, x.Base)
			writeMath(b, x.Sup)
			b.WriteString(`</msup>`)
		}
	case *Radical:
		if x.Degree != nil {
			b.WriteString(`<mroot>`)
			writeMath(b, x.Body)
			writeMath(b, x.Degree)
			b.WriteString(`</mroot>`)
		} else {
			b.WriteString(`<msqrt>`)
			writeMath(b, x.Body)
			b.WriteString(`</msqrt>`)
		}
	case *Fence:
		b.WriteString(`<mrow>`)
		if x.Left != 0 {
			fmt.Fprintf(b, `<mo stretchy="true" fence="true" symmetric="%t">%s</mo>`, x.Alignment == 0, html.EscapeString(string(x.Left)))
		}
		writeMath(b, x.Body)
		if x.Right != 0 {
			fmt.Fprintf(b, `<mo stretchy="true" fence="true" symmetric="%t">%s</mo>`, x.Alignment == 0, html.EscapeString(string(x.Right)))
		}
		b.WriteString(`</mrow>`)
	case *Accent:
		op := string(x.Operator)
		if x.Operator == 0 {
			op = accentRune(x.Kind)
		}
		if x.Kind == EmbellPrime || x.Kind == EmbellDPrime || x.Kind == EmbellTriplePrime {
			b.WriteString(`<msup>`)
			writeMath(b, x.Body)
			fmt.Fprintf(b, `<mo>%s</mo></msup>`, op)
		} else if x.Under {
			b.WriteString(`<munder accentunder="true">`)
			writeMath(b, x.Body)
			fmt.Fprintf(b, `<mo stretchy="true">%s</mo></munder>`, op)
		} else {
			b.WriteString(`<mover accent="true">`)
			writeMath(b, x.Body)
			fmt.Fprintf(b, `<mo stretchy="true">%s</mo></mover>`, op)
		}
	case *UnderOver:
		tag := "mover"
		if x.Under {
			tag = "munder"
		}
		fmt.Fprintf(b, "<%s>", tag)
		writeMath(b, x.Base)
		b.WriteString(`<mrow><mo stretchy="true">` + html.EscapeString(string(x.Operator)) + `</mo>`)
		writeMath(b, x.Annotation)
		b.WriteString(`</mrow>`)
		fmt.Fprintf(b, "</%s>", tag)
	case *Limit:
		if x.Lower != nil && x.Upper != nil {
			b.WriteString(`<munderover>`)
			writeMath(b, x.Body)
			writeMath(b, x.Lower)
			writeMath(b, x.Upper)
			b.WriteString(`</munderover>`)
		} else if x.Lower != nil {
			b.WriteString(`<munder>`)
			writeMath(b, x.Body)
			writeMath(b, x.Lower)
			b.WriteString(`</munder>`)
		} else if x.Upper != nil {
			b.WriteString(`<mover>`)
			writeMath(b, x.Body)
			writeMath(b, x.Upper)
			b.WriteString(`</mover>`)
		} else {
			writeMath(b, x.Body)
		}
	case *LongDivision:
		b.WriteString(`<mrow><menclose notation="longdiv">`)
		writeMath(b, x.Dividend)
		b.WriteString(`</menclose>`)
		if x.Quotient != nil {
			writeMath(b, x.Quotient)
		}
		b.WriteString(`</mrow>`)
	case *Dirac:
		b.WriteString(`<mrow>`)
		if x.HasLeft {
			lf := x.LeftFence
			if lf == 0 {
				lf = '⟨'
			}
			fmt.Fprintf(b, `<mo stretchy="true">%s</mo>`, html.EscapeString(string(lf)))
		}
		writeMath(b, x.Left)
		mid := x.Middle
		if mid == 0 {
			mid = '|'
		}
		fmt.Fprintf(b, `<mo stretchy="true">%s</mo>`, html.EscapeString(string(mid)))
		writeMath(b, x.Right)
		if x.HasRight {
			rf := x.RightFence
			if rf == 0 {
				rf = '⟩'
			}
			fmt.Fprintf(b, `<mo stretchy="true">%s</mo>`, html.EscapeString(string(rf)))
		}
		b.WriteString(`</mrow>`)
	case *LargeOperator:
		base := func() {
			if x.OperatorExpr != nil {
				writeMath(b, x.OperatorExpr)
				return
			}
			op := html.EscapeString(string(x.Operator))
			// MTEF's integral option controls whether the operator expands to
			// its contents. Other big operators retain their native large-op
			// geometry and do not need an artificial CSS scale.
			stretchy := "false"
			if x.Expand {
				stretchy = "true"
			}
			fmt.Fprintf(b, `<mo largeop="true" movablelimits="false" stretchy="%s">%s</mo>`, stretchy, op)
		}
		if x.LimitsAboveBelow {
			if x.Lower != nil && x.Upper != nil {
				b.WriteString(`<munderover>`)
				base()
				writeMath(b, x.Lower)
				writeMath(b, x.Upper)
				b.WriteString(`</munderover>`)
			} else if x.Lower != nil {
				b.WriteString(`<munder>`)
				base()
				writeMath(b, x.Lower)
				b.WriteString(`</munder>`)
			} else if x.Upper != nil {
				b.WriteString(`<mover>`)
				base()
				writeMath(b, x.Upper)
				b.WriteString(`</mover>`)
			} else {
				base()
			}
		} else {
			// Integral-style limits belong beside the operator. msub/msup is
			// the MathML semantic equivalent; movablelimits is disabled so a
			// display-style browser does not move them above/below.
			if x.Lower != nil && x.Upper != nil {
				b.WriteString(`<msubsup>`)
				base()
				writeMath(b, x.Lower)
				writeMath(b, x.Upper)
				b.WriteString(`</msubsup>`)
			} else if x.Lower != nil {
				b.WriteString(`<msub>`)
				base()
				writeMath(b, x.Lower)
				b.WriteString(`</msub>`)
			} else if x.Upper != nil {
				b.WriteString(`<msup>`)
				base()
				writeMath(b, x.Upper)
				b.WriteString(`</msup>`)
			} else {
				base()
			}
		}
		writeMath(b, x.Body)
	case *Matrix:
		frame := matrixFrame(x)
		if frame != "" {
			b.WriteString(`<menclose notation="` + frame + `">`)
		}
		fmt.Fprintf(b, `<mtable align="%s" columnalign="%s" rowalign="%s"`, mathMatrixAlign(x.VAlign), mathHAlign(x.HAlign), mathCellVAlign(x.CellVAlign))
		writeMatrixPartitionMetadata(b, x)
		if lines := matrixInteriorLines(x.ColRules, x.Cols); lines != "" {
			b.WriteString(` columnlines="` + lines + `"`)
		}
		if lines := matrixInteriorLines(x.RowRules, x.Rows); lines != "" {
			b.WriteString(` rowlines="` + lines + `"`)
		}
		b.WriteString(`>`)
		for r := 0; r < x.Rows; r++ {
			b.WriteString(`<mtr>`)
			for c := 0; c < x.Cols; c++ {
				b.WriteString(`<mtd>`)
				i := r*x.Cols + c
				if i < len(x.Cells) {
					writeMath(b, x.Cells[i])
				}
				b.WriteString(`</mtd>`)
			}
			b.WriteString(`</mtr>`)
		}
		b.WriteString(`</mtable>`)
		if frame != "" {
			b.WriteString(`</menclose>`)
		}
	case *Pile:
		fmt.Fprintf(b, `<mtable columnalign="%s" rowalign="%s">`, mathHAlign(x.HAlign), mathVAlign(x.VAlign))
		for _, l := range x.Lines {
			b.WriteString(`<mtr><mtd>`)
			writeMath(b, l)
			b.WriteString(`</mtd></mtr>`)
		}
		b.WriteString(`</mtable>`)
	case *Space:
		b.WriteString(`<mspace width="` + html.EscapeString(x.Width) + `"/>`)
	case *Unsupported:
		fmt.Fprintf(b, `<merror><mtext>unsupported template %d/%d</mtext></merror>`, x.Selector, x.Variation)
	}
}
func exprStyle(e Expr) (Style, bool) {
	switch x := e.(type) {
	case *Row:
		return x.Style, true
	case *Glyph:
		return x.Style, true
	case *Fraction:
		return x.Style, true
	case *Script:
		return x.Style, true
	case *Radical:
		return x.Style, true
	case *Fence:
		return x.Style, true
	case *Accent:
		return x.Style, true
	case *LargeOperator:
		return x.Style, true
	case *UnderOver:
		return x.Style, true
	case *Limit:
		return x.Style, true
	case *LongDivision:
		return x.Style, true
	case *Dirac:
		return x.Style, true
	case *Matrix:
		return x.Style, true
	case *Pile:
		return x.Style, true
	default:
		return Style{}, false
	}
}

func points32(o Offset, horizontal bool) string {
	v := o.Y
	if horizontal {
		v = o.X
	}
	return strconv.FormatFloat(float64(v)/32.0, 'f', 3, 64) + "pt"
}

func writeStyle(b *strings.Builder, s Style) {
	if s.Font != "" {
		b.WriteString(` mathfamily="` + html.EscapeString(s.Font) + `"`)
	}
	if s.Size.Points32 != nil {
		pt := float64(*s.Size.Points32) / 32
		b.WriteString(` mathsize="` + strconv.FormatFloat(pt, 'f', 3, 64) + `pt"`)
	}
	if s.Size.Points32 == nil {
		switch s.Size.Kind {
		case SizeSub:
			b.WriteString(` scriptlevel="1"`)
		case SizeSub2:
			b.WriteString(` scriptlevel="2"`)
		case SizeSymbol:
			b.WriteString(` mathsize="150%"`)
		case SizeSubSymbol:
			b.WriteString(` mathsize="100%"`)
		}
	}
	if s.Bold && s.Italic {
		b.WriteString(` mathvariant="bold-italic"`)
	} else if s.Bold {
		b.WriteString(` mathvariant="bold"`)
	} else if s.Italic {
		b.WriteString(` mathvariant="italic"`)
	}
}

func matrixRuleName(v byte) string {
	switch v {
	case 1:
		return "solid"
	case 2:
		return "dashed"
	case 3:
		// MathML rowlines/columnlines do not have a portable dotted value.
		// Use the closest native rendering while retaining the exact authored
		// partition code in data-mtef-*-parts below.
		return "dashed"
	default:
		return "none"
	}
}

func matrixPartitionCodes(rules []byte) string {
	if len(rules) == 0 {
		return ""
	}
	parts := make([]string, len(rules))
	for i, v := range rules {
		parts[i] = strconv.Itoa(int(v & 3))
	}
	return strings.Join(parts, " ")
}

func writeMatrixPartitionMetadata(b *strings.Builder, x *Matrix) {
	if x == nil {
		return
	}
	if v := matrixPartitionCodes(x.RowRules); v != "" {
		b.WriteString(` data-mtef-row-parts="` + v + `"`)
	}
	if v := matrixPartitionCodes(x.ColRules); v != "" {
		b.WriteString(` data-mtef-col-parts="` + v + `"`)
	}
}

func matrixInteriorLines(rules []byte, count int) string {
	if count <= 1 || len(rules) < count+1 {
		return ""
	}
	parts := make([]string, 0, count-1)
	any := false
	for i := 1; i < count; i++ {
		v := matrixRuleName(rules[i])
		if v != "none" {
			any = true
		}
		parts = append(parts, v)
	}
	if !any {
		return ""
	}
	return strings.Join(parts, " ")
}

func matrixFrame(x *Matrix) string {
	if x == nil {
		return ""
	}
	top, bottom, left, right := byte(0), byte(0), byte(0), byte(0)
	if len(x.RowRules) > 0 {
		top = x.RowRules[0]
		bottom = x.RowRules[len(x.RowRules)-1]
	}
	if len(x.ColRules) > 0 {
		left = x.ColRules[0]
		right = x.ColRules[len(x.ColRules)-1]
	}
	// menclose provides portable outer-side geometry only for solid rules.
	// Dashed/dotted outer partitions remain exact in Matrix.RowRules/ColRules and
	// are serialized on the mtable as data-mtef-*-parts for renderer recovery.
	var n []string
	if top == 1 {
		n = append(n, "top")
	}
	if bottom == 1 {
		n = append(n, "bottom")
	}
	if left == 1 {
		n = append(n, "left")
	}
	if right == 1 {
		n = append(n, "right")
	}
	return strings.Join(n, " ")
}

func mathHAlign(v byte) string {
	// MTEF: 1=left, 2=center, 3=right, 4=relational, 5=decimal.
	// MathML columnalign cannot directly express relational/decimal alignment;
	// keep those authored values in IR and use center as the safe projection.
	switch v {
	case 1:
		return "left"
	case 3:
		return "right"
	default:
		return "center"
	}
}

func mathCellVAlign(v byte) string {
	// MTEF pile/matrix cell alignment: 0=top-line baseline, 1=center-line
	// baseline, 2=bottom-line baseline, 3=vertical center. MathML's
	// rowalign is cell-content alignment, so top/baseline/bottom/center are
	// the closest semantic projection.
	switch v {
	case 0:
		return "top"
	case 2:
		return "bottom"
	case 3:
		return "center"
	default:
		return "baseline"
	}
}

func mathVAlign(v byte) string { return mathCellVAlign(v) }

func mathMatrixAlign(v byte) string {
	// Matrix VAlign positions the matrix as a whole in its containing line,
	// unlike CellVAlign which controls content inside each matrix cell.
	switch v {
	case 0:
		return "top"
	case 2:
		return "bottom"
	case 3:
		return "center"
	default:
		return "baseline"
	}
}
func accentRune(e Embellishment) string {
	switch e {
	case EmbellDot:
		return "˙"
	case EmbellDDot:
		return "¨"
	case EmbellTDot:
		return "⃛"
	case EmbellTilde:
		return "˜"
	case EmbellHat:
		return "ˆ"
	case EmbellNot:
		return "⁄"
	case EmbellRightArrow:
		return "→"
	case EmbellLeftArrow:
		return "←"
	case EmbellBothArrow:
		return "↔"
	case EmbellOverBar, EmbellMidBar:
		return "¯"
	case EmbellPrime:
		return "′"
	case EmbellDPrime:
		return "″"
	case EmbellTriplePrime:
		return "‴"
	case EmbellBackPrime:
		return "‵"
	case EmbellFrown:
		return "⌢"
	case EmbellSmile:
		return "⌣"
	}
	return "^"
}
