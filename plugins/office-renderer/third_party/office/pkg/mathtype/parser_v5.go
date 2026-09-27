package mathtype

import "fmt"

type v5parser struct {
	r     reader
	eq    *Equation
	style Style
}

func parseV5(data []byte) (*Equation, error) {
	p := &v5parser{r: reader{b: data}, eq: &Equation{Fonts: map[int]FontDef{}}}
	v, _ := p.r.u8()
	pl, _ := p.r.u8()
	pr, _ := p.r.u8()
	pv, _ := p.r.u8()
	ps, _ := p.r.u8()
	app, e := p.r.cstr()
	if e != nil {
		return nil, e
	}
	op, e := p.r.u8()
	if e != nil {
		return nil, e
	}
	p.eq.Header = Header{Version: Version(v), Platform: pl, Product: pr, ProductVersion: pv, ProductSubversion: ps, Application: app, Options: op}
	xs, e := p.list(false)
	if e != nil {
		return nil, e
	}
	p.eq.Root = normalize(row(xs))
	resolveEquationStyles(p.eq)
	return p.eq, nil
}
func (p *v5parser) list(stop bool) ([]Expr, error) {
	var xs []Expr
	for p.r.left() > 0 {
		typ, e := p.r.u8()
		if e != nil {
			return nil, e
		}
		if typ == 0 {
			if stop {
				return xs, nil
			}
			continue
		}
		if typ >= 100 {
			n, e := p.uint()
			if e != nil || p.r.left() < n {
				return nil, ErrInvalidMTEF
			}
			p.r.p += n
			continue
		}
		opt := byte(0)
		if typ != 7 && typ != 9 && typ < 10 {
			opt, e = p.r.u8()
			if e != nil {
				return nil, e
			}
		}
		n, e := p.record(typ, opt)
		if e != nil {
			return nil, e
		}
		if n != nil {
			xs = append(xs, n)
		}
	}
	if stop {
		return nil, ErrInvalidMTEF
	}
	return xs, nil
}
func (p *v5parser) sint() (int, error) {
	b, e := p.r.u8()
	if e != nil {
		return 0, e
	}
	if b != 255 {
		return int(b) - 128, nil
	}
	v, e := p.r.u16()
	return int(int32(v) - 32768), e
}
func (p *v5parser) uint() (int, error) {
	b, e := p.r.u8()
	if e != nil {
		return 0, e
	}
	if b != 255 {
		return int(b), nil
	}
	v, e := p.r.u16()
	return int(v), e
}
func (p *v5parser) record(t, o byte) (Expr, error) {
	var nud *Offset
	if o&8 != 0 {
		x, e := p.r.nudge()
		if e != nil {
			return nil, e
		}
		nud = &x
	}
	st := p.style
	st.Nudge = nud
	switch t {
	case 1:
		if o&4 != 0 {
			if _, e := p.r.i16(); e != nil {
				return nil, e
			}
		}
		if o&2 != 0 {
			if e := p.skipRuler(); e != nil {
				return nil, e
			}
		}
		if o&1 != 0 {
			return &Row{Style: st}, nil
		}
		x, e := p.list(true)
		return &Row{Items: x, Style: st}, e
	case 2:
		tf, e := p.sint()
		if e != nil {
			return nil, e
		}
		var ch uint16
		if o&0x20 == 0 {
			ch, e = p.r.u16()
			if e != nil {
				return nil, e
			}
		}
		if o&4 != 0 {
			b, e := p.r.u8()
			if e != nil {
				return nil, e
			}
			if ch == 0 {
				ch = uint16(b)
			}
		}
		if o&0x10 != 0 {
			v, e := p.r.u16()
			if e != nil {
				return nil, e
			}
			if ch == 0 {
				ch = v
			}
		}
		g := &Glyph{Rune: rune(ch), Style: st}
		g.Style.Typeface = tf
		if o&1 != 0 {
			for {
				tt, e := p.r.u8()
				if e != nil {
					return nil, e
				}
				if tt == 0 {
					break
				}
				oo, e := p.r.u8()
				if e != nil {
					return nil, e
				}
				if tt != 6 {
					return nil, fmt.Errorf("mathtype: MTEF5 embell expected")
				}
				if oo&8 != 0 {
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
	case 3:
		sel, e := p.r.u8()
		if e != nil {
			return nil, e
		}
		a, e := p.r.u8()
		if e != nil {
			return nil, e
		}
		vr := uint16(a)
		if a&0x80 != 0 {
			b, e := p.r.u8()
			if e != nil {
				return nil, e
			}
			vr = uint16(a&0x7f) | uint16(b)<<8
		}
		option := byte(0)
		if sel <= 8 || sel == 9 || sel >= 15 && sel <= 20 {
			option, e = p.r.u8()
			if e != nil {
				return nil, e
			}
		}
		x, e := p.list(true)
		if e != nil {
			return nil, e
		}
		return v5Template(sel, vr, option, x, st), nil
	case 4:
		ha, e := p.r.u8()
		if e != nil {
			return nil, e
		}
		va, e := p.r.u8()
		if e != nil {
			return nil, e
		}
		if o&2 != 0 {
			if e = p.skipRuler(); e != nil {
				return nil, e
			}
		}
		x, e := p.list(true)
		return &Pile{Lines: x, HAlign: ha, VAlign: va, Style: st}, e
	case 5:
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
		rb := (int(rr) + 4) / 4
		cb := (int(cc) + 4) / 4
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
		x, e := p.list(true)
		return &Matrix{Rows: int(rr), Cols: int(cc), Cells: x, HAlign: ha, VAlign: va, CellVAlign: cellVA, RowRules: unpackRules(rawR, int(rr)+1), ColRules: unpackRules(rawC, int(cc)+1), Style: st}, e
	case 7:
		return nil, p.skipRulerBody()
	case 9:
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
		p.style.Size = Size{Kind: SizeKind(t - 10)}
		return nil, nil
	default:
		return nil, fmt.Errorf("mathtype: MTEF5 record %d not yet supported safely", t)
	}
}
func (p *v5parser) readSize() (Size, error) {
	a, e := p.r.u8()
	if e != nil {
		return Size{}, e
	}
	if a == 101 {
		v, e := p.r.i16()
		if e != nil {
			return Size{}, e
		}
		v = -v
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
func (p *v5parser) skipRuler() error {
	t, e := p.r.u8()
	if e != nil {
		return e
	}
	if t != 7 {
		return fmt.Errorf("mathtype: MTEF5 ruler expected")
	}
	return p.skipRulerBody()
}
func (p *v5parser) skipRulerBody() error {
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
