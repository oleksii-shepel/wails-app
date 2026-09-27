package mathtype

import "fmt"

const (
	v3End    = 0
	v3Line   = 1
	v3Char   = 2
	v3Tmpl   = 3
	v3Pile   = 4
	v3Matrix = 5
	v3Embell = 6
	v3Ruler  = 7
	v3Font   = 8
	v3Size   = 9
)

type v3parser struct {
	r     reader
	eq    *Equation
	style Style
}

func parseV3(data []byte) (*Equation, error) {
	p := &v3parser{r: reader{b: data}, eq: &Equation{Fonts: map[int]FontDef{}}}
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
func (p *v3parser) list(stop bool) ([]Expr, error) {
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
func (p *v3parser) record(typ, opt byte) (Expr, error) {
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
		ch, e := p.r.u16()
		if e != nil {
			return nil, e
		}
		// MathType uses private MTCode values for authored spacing. They are
		// layout instructions, not printable private-use glyphs.
		if sp, ok := v3MathSpace(ch); ok {
			return sp, nil
		}
		g := &Glyph{Rune: rune(ch), Style: st}
		g.Style.Typeface = int(int8(tf - 128))
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
		return nil, fmt.Errorf("mathtype: unsupported MTEF3 record %d", typ)
	}
}
func (p *v3parser) readSize() (Size, error) {
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
func (p *v3parser) skipRuler() error {
	t, e := p.r.u8()
	if e != nil {
		return e
	}
	if t&15 != v3Ruler {
		return fmt.Errorf("mathtype: ruler expected")
	}
	return p.skipRulerBody()
}
func (p *v3parser) skipRulerBody() error {
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
func unpackRules(raw []byte, n int) []byte {
	out := make([]byte, 0, n)
	for _, b := range raw {
		for s := 0; s < 8 && len(out) < n; s += 2 {
			out = append(out, (b>>s)&3)
		}
	}
	return out
}
func isV3Integral(s byte) bool { return s >= 21 && s <= 26 }

func v3MathSpace(ch uint16) (*Space, bool) {
	switch ch {
	case 0xEB01: // no space
		return &Space{Width: "0em"}, true
	case 0xEF04, 0xEF05: // tiny space
		return &Space{Width: "0.111em"}, true
	case 0xEB02: // small space
		return &Space{Width: "0.167em"}, true
	case 0xEB04: // medium space
		return &Space{Width: "0.222em"}, true
	case 0xEB05: // large space
		return &Space{Width: "0.333em"}, true
	case 0xEB08: // normal space
		return &Space{Width: "0.278em"}, true
	default:
		return nil, false
	}
}
