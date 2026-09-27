package mathtype

// MTEF5 uses a different selector table from MTEF3. This mapper intentionally
// covers structural templates whose semantics are losslessly representable in
// our canonical IR; unknown selectors remain explicit Unsupported nodes.
func v5Template(s byte, v uint16, o byte, xs []Expr, st Style) Expr {
	// v5 fence family occupies selectors 0..8; root=10, fraction=11, bars=12/13,
	// arrow=14, integrals=15..20, braces=21/22, sums/products follow.
	if s <= 8 {
		l, r := v5Fences(s, v)
		return &Fence{Body: at(xs, 0), Left: l, Right: r, Alignment: o, Style: st}
	}
	switch s {
	case 10:
		if len(xs) > 1 {
			return &Radical{Degree: at(xs, 0), Body: at(xs, 1), Style: st}
		}
		return &Radical{Body: at(xs, 0), Style: st}
	case 11:
		return &Fraction{Numerator: at(xs, 0), Denominator: at(xs, 1), Style: st}
	case 12:
		return &Accent{Body: at(xs, 0), Kind: EmbellMidBar, Style: st}
	case 13:
		return &Accent{Body: at(xs, 0), Kind: EmbellOverBar, Style: st}
	}
	if s >= 15 && s <= 20 {
		return &LargeOperator{Operator: v5IntegralRune(s, v), Body: at(xs, 0), Upper: at(xs, 1), Lower: at(xs, 2), Expand: o == 1, LimitsAboveBelow: s >= 18, Style: st}
	}
	return &Unsupported{Selector: s, Variation: byte(v)}
}
func v5Fences(s byte, v uint16) (rune, rune) {
	pairs := [][2]rune{{'(', ')'}, {'[', ']'}, {'{', '}'}, {'⟨', '⟩'}, {'|', '|'}, {'‖', '‖'}, {'⌊', '⌋'}, {'⌈', '⌉'}, {0, 0}}
	p := pairs[int(s)]
	if v&1 != 0 {
		p[1] = 0
	}
	if v&2 != 0 {
		p[0] = 0
	}
	return p[0], p[1]
}
func v5IntegralRune(s byte, v uint16) rune {
	switch s {
	case 16, 19:
		return '∬'
	case 17, 20:
		return '∭'
	}
	if v&0x10 != 0 {
		return '∮'
	}
	return '∫'
}
