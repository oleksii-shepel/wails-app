package mathtype

import "fmt"

type v4parser struct {
	r     reader
	eq    *Equation
	style Style
}

func parseV4(data []byte) (*Equation, error) {
	p := &v4parser{r: reader{b: data}, eq: &Equation{Fonts: map[int]FontDef{}}}
	v, _ := p.r.u8()
	pl, _ := p.r.u8()
	pr, _ := p.r.u8()
	pv, _ := p.r.u8()
	ps, _ := p.r.u8()
	p.eq.Header = Header{Version: Version(v), Platform: pl, Product: pr, ProductVersion: pv, ProductSubversion: ps}
	items, err := p.list(false)
	if err != nil {
		return nil, err
	}
	p.eq.Root = normalize(row(items))
	resolveEquationStyles(p.eq)
	return p.eq, nil
}
func (p *v4parser) list(stop bool) ([]Expr, error) {
	var out []Expr
	for p.r.left() > 0 {
		tag, e := p.r.u8()
		if e != nil {
			return nil, e
		}
		typ := tag & 0x0f
		opt := tag >> 4
		if typ == v3End {
			// END terminates every MTEF3 object list, including the top-level
			// equation list. Bytes after the equation END can belong to the
			// embedding/container and must not be interpreted as MTEF records.
			return out, nil
		}
		n, e := p.record(typ, opt)
		if e != nil {
			return nil, e
		}
		if n != nil {
			out = append(out, n)
		}
	}
	if stop {
		return nil, ErrInvalidMTEF
	}
	return out, nil
}
func (p *v4parser) record(typ, opt byte) (Expr, error) {
	var nud *Offset
	if opt&8 != 0 {
		o, e := p.r.nudge()
		if e != nil {
			return nil, e
		}
		nud = &o
	}
	st := p.style
	st.Nudge = nud
	switch typ {
	case v3Line:
		if opt&4 != 0 {
			if _, e := p.r.i16(); e != nil {
				return nil, e
			}
		}
		if opt&2 != 0 {
			if e := p.skipRuler(); e != nil {
				return nil, e
			}
		}
		if opt&1 != 0 {
			return &Row{Style: st}, nil
		}
		xs, e := p.list(true)
		return &Row{Items: xs, Style: st}, e
	case v3Char:
		tf, e := p.r.u8()
		if e != nil {
			return nil, e
		}
		ch8, e := p.r.u8()
		ch := uint16(ch8)
		if e != nil {
			return nil, e
		}
		// MathType uses private MTCode values for authored spacing. They are
		// layout instructions, not printable private-use glyphs.
		if sp, ok := v3MathSpace(ch); ok {
			return sp, nil
		}
		typeface := int(int8(tf - 128))
		// MTEF4 has no 16-bit MTCode field. MathType therefore writes authored
		// spacing through the fnSPACE typeface (24), with the byte carrying the
		// low part of the traditional MTCode spacing value. Treat it as layout,
		// never as a printable character; otherwise bytes such as 0x01/0x08 can
		// surface as missing-glyph boxes between otherwise valid symbols.
		if typeface == 24 { // fnSPACE
			if sp, ok := v4MathSpace(ch8); ok {
				return sp, nil
			}
			return &Space{Width: "0.278em"}, nil
		}
		g := &Glyph{Rune: decodeV4Char(typeface, ch8), Style: st}
		g.Style.Typeface = typeface
		if opt&2 != 0 {
			for {
				t, e := p.r.u8()
				if e != nil {
					return nil, e
				}
				if t&15 == 0 {
					break
				}
				if t&15 != v3Embell {
					return nil, fmt.Errorf("mathtype: expected embellishment")
				}
				if t>>4&8 != 0 {
					if _, e = p.r.nudge(); e != nil {
						return nil, e
					}
				}
				em, e := p.r.u8()
				if e != nil {
					return nil, e
				}
				g.Embellishments = append(g.Embellishments, Embellishment(em))
			}
		}
		return applyEmbellishments(g), nil
	case v3Tmpl:
		sel, e := p.r.u8()
		if e != nil {
			return nil, e
		}
		vr, e := p.r.u8()
		if e != nil {
			return nil, e
		}
		// MTEF3 TMPL records carry an options byte after selector/variation.
		// It is semantically used by fence and integral templates, but is still
		// present (normally zero) for the other template classes. Not consuming
		// it desynchronizes the following subobject list.
		option, e := p.r.u8()
		if e != nil {
			return nil, e
		}
		xs, e := p.list(true)
		if e != nil {
			return nil, e
		}
		return v3Template(sel, vr, option, xs, st), nil
	case v3Pile:
		ha, e := p.r.u8()
		if e != nil {
			return nil, e
		}
		va, e := p.r.u8()
		if e != nil {
			return nil, e
		}
		if opt&2 != 0 {
			if e = p.skipRuler(); e != nil {
				return nil, e
			}
		}
		xs, e := p.list(true)
		return &Pile{Lines: xs, HAlign: ha, VAlign: va, Style: st}, e
	case v3Matrix:
		va, e := p.r.u8()
		if e != nil {
			return nil, e
		}
		ha, e := p.r.u8()
		if e != nil {
			return nil, e
		}
		cellVA, e := p.r.u8()
		if e != nil {
			return nil, e
		}
		rr, e := p.r.u8()
		if e != nil {
			return nil, e
		}
		cc, e := p.r.u8()
		if e != nil {
			return nil, e
		}
		rb := (int(rr) + 1 + 3) / 4
		cb := (int(cc) + 1 + 3) / 4
		rawR := make([]byte, rb)
		rawC := make([]byte, cb)
		for i := range rawR {
			rawR[i], e = p.r.u8()
			if e != nil {
				return nil, e
			}
		}
		for i := range rawC {
			rawC[i], e = p.r.u8()
			if e != nil {
				return nil, e
			}
		}
		xs, e := p.list(true)
		return &Matrix{Rows: int(rr), Cols: int(cc), Cells: xs, HAlign: ha, VAlign: va, CellVAlign: cellVA, RowRules: unpackRules(rawR, int(rr)+1), ColRules: unpackRules(rawC, int(cc)+1), Style: st}, e
	case v3Ruler:
		return nil, p.skipRulerBody()
	case v3Font:
		idx, e := p.r.u8()
		if e != nil {
			return nil, e
		}
		sty, e := p.r.u8()
		if e != nil {
			return nil, e
		}
		name, e := p.r.cstr()
		if e != nil {
			return nil, e
		}
		p.eq.Fonts[int(idx)] = FontDef{Name: name, Italic: sty&1 != 0, Bold: sty&2 != 0}
		return nil, nil
	case v3Size:
		sz, e := p.readSize()
		if e != nil {
			return nil, e
		}
		p.style.Size = sz
		if p.eq.InitialSize == (Size{}) {
			p.eq.InitialSize = sz
		}
		return nil, nil
	case 10, 11, 12, 13, 14:
		p.style.Size = Size{Kind: SizeKind(typ - 10)}
		return nil, nil
	default:
		return nil, fmt.Errorf("mathtype: unsupported MTEF4 record %d", typ)
	}
}
func (p *v4parser) readSize() (Size, error) {
	a, e := p.r.u8()
	if e != nil {
		return Size{}, e
	}
	if a == 101 {
		v, e := p.r.i16()
		if e != nil {
			return Size{}, e
		}
		// MTEF3 stores explicit point sizes after marker 101 as -lsize,
		// i.e. a positive magnitude in 32nds of a point. Keep that authored
		// magnitude positive in the canonical IR.
		if v < 0 {
			v = -v
		}
		return Size{Points32: &v}, nil
	}
	if a == 100 {
		k, e := p.r.u8()
		if e != nil {
			return Size{}, e
		}
		d, e := p.r.i16()
		return Size{Kind: SizeKind(k), Delta: d}, e
	}
	d, e := p.r.u8()
	return Size{Kind: SizeKind(a), Delta: int16(d) - 128}, e
}
func (p *v4parser) skipRuler() error {
	t, e := p.r.u8()
	if e != nil {
		return e
	}
	if t&15 != v3Ruler {
		return fmt.Errorf("mathtype: ruler expected")
	}
	return p.skipRulerBody()
}
func (p *v4parser) skipRulerBody() error {
	n, e := p.r.u8()
	if e != nil {
		return e
	}
	for i := 0; i < int(n); i++ {
		if _, e = p.r.u8(); e != nil {
			return e
		}
		if _, e = p.r.u16(); e != nil {
			return e
		}
	}
	return nil
}

func v4MathSpace(ch byte) (*Space, bool) {
	switch ch {
	case 0x01:
		return &Space{Width: "0em"}, true
	case 0x02:
		return &Space{Width: "0.167em"}, true
	case 0x04:
		return &Space{Width: "0.222em"}, true
	case 0x05:
		return &Space{Width: "0.333em"}, true
	case 0x08:
		return &Space{Width: "0.278em"}, true
	default:
		return nil, false
	}
}

// decodeV4Char resolves the built-in MathType style encodings that are not
// ASCII-compatible. MTEF4 CHAR stores only an 8-bit character value; its
// meaning is determined by the CHAR typeface. In particular, Greek styles
// store Latin keyboard positions (for example d/r) but mean Greek glyphs.
func decodeV4Char(typeface int, ch byte) rune {
	if typeface == 4 { // fnLCGREEK
		if r, ok := lowerGreekV4[ch]; ok {
			return r
		}
	}
	if typeface == 5 { // fnUCGREEK
		if r, ok := upperGreekV4[ch]; ok {
			return r
		}
	}
	if typeface == 6 { // fnSYMBOL
		if r, ok := symbolV4[ch]; ok {
			return r
		}
	}
	return rune(ch)
}

var lowerGreekV4 = map[byte]rune{
	'a': 'α', 'b': 'β', 'g': 'γ', 'd': 'δ', 'e': 'ε', 'z': 'ζ',
	'h': 'η', 'q': 'θ', 'i': 'ι', 'k': 'κ', 'l': 'λ', 'm': 'μ',
	'n': 'ν', 'x': 'ξ', 'o': 'ο', 'p': 'π', 'r': 'ρ', 's': 'σ',
	't': 'τ', 'u': 'υ', 'f': 'φ', 'c': 'χ', 'y': 'ψ', 'w': 'ω',
}

var upperGreekV4 = map[byte]rune{
	'A': 'Α', 'B': 'Β', 'G': 'Γ', 'D': 'Δ', 'E': 'Ε', 'Z': 'Ζ',
	'H': 'Η', 'Q': 'Θ', 'I': 'Ι', 'K': 'Κ', 'L': 'Λ', 'M': 'Μ',
	'N': 'Ν', 'X': 'Ξ', 'O': 'Ο', 'P': 'Π', 'R': 'Ρ', 'S': 'Σ',
	'T': 'Τ', 'U': 'Υ', 'F': 'Φ', 'C': 'Χ', 'Y': 'Ψ', 'W': 'Ω',
}

var symbolV4 = map[byte]rune{
	0x22: '∀',
	0x24: '∃',
	0x27: '∋',
	0x2A: '∗',
	0x2D: '−',
	0x40: '≅',
	0x41: 'Α',
	0x42: 'Β',
	0x43: 'Χ',
	0x44: '∆',
	0x45: 'Ε',
	0x46: 'Φ',
	0x47: 'Γ',
	0x48: 'Η',
	0x49: 'Ι',
	0x4A: 'ϑ',
	0x4B: 'Κ',
	0x4C: 'Λ',
	0x4D: 'Μ',
	0x4E: 'Ν',
	0x4F: 'Ο',
	0x50: 'Π',
	0x51: 'Θ',
	0x52: 'Ρ',
	0x53: 'Σ',
	0x54: 'Τ',
	0x55: 'Υ',
	0x56: 'ς',
	0x57: 'Ω',
	0x58: 'Ξ',
	0x59: 'Ψ',
	0x5A: 'Ζ',
	0x5C: '∴',
	0x5E: '⊥',
	0x61: 'α',
	0x62: 'β',
	0x63: 'χ',
	0x64: 'δ',
	0x65: 'ε',
	0x66: 'φ',
	0x67: 'γ',
	0x68: 'η',
	0x69: 'ι',
	0x6A: 'ϕ',
	0x6B: 'κ',
	0x6C: 'λ',
	0x6D: 'µ',
	0x6E: 'ν',
	0x6F: 'ο',
	0x70: 'π',
	0x71: 'θ',
	0x72: 'ρ',
	0x73: 'σ',
	0x74: 'τ',
	0x75: 'υ',
	0x76: 'ϖ',
	0x77: 'ω',
	0x78: 'ξ',
	0x79: 'ψ',
	0x7A: 'ζ',
	0x7E: '∼',
	0xA0: '€',
	0xA1: 'ϒ',
	0xA2: '′',
	0xA3: '≤',
	0xA4: '⁄',
	0xA5: '∞',
	0xA6: 'ƒ',
	0xA7: '♣',
	0xA8: '♦',
	0xA9: '♥',
	0xAA: '♠',
	0xAB: '↔',
	0xAC: '←',
	0xAD: '↑',
	0xAE: '→',
	0xAF: '↓',
	0xB0: '°',
	0xB1: '±',
	0xB2: '″',
	0xB3: '≥',
	0xB4: '×',
	0xB5: '∝',
	0xB6: '∂',
	0xB7: '•',
	0xB8: '÷',
	0xB9: '≠',
	0xBA: '≡',
	0xBB: '≈',
	0xBC: '…',
	0xBF: '↵',
	0xC0: 'ℵ',
	0xC1: 'ℑ',
	0xC2: 'ℜ',
	0xC3: '℘',
	0xC4: '⊗',
	0xC5: '⊕',
	0xC6: '∅',
	0xC7: '∩',
	0xC8: '∪',
	0xC9: '⊃',
	0xCA: '⊇',
	0xCB: '⊄',
	0xCC: '⊂',
	0xCD: '⊆',
	0xCE: '∈',
	0xCF: '∉',
	0xD0: '∠',
	0xD1: '∇',
	0xD5: '∏',
	0xD6: '√',
	0xD7: '⋅',
	0xD8: '¬',
	0xD9: '∧',
	0xDA: '∨',
	0xDB: '⇔',
	0xDC: '⇐',
	0xDD: '⇑',
	0xDE: '⇒',
	0xDF: '⇓',
	0xE0: '◊',
	0xE1: '〈',
	0xE5: '∑',
	0xF1: '〉',
	0xF2: '∫',
	0xF3: '⌠',
	0xF5: '⌡',
}
