package mathtype

import "fmt"

func Parse(data []byte) (*Equation, error) {
	if len(data) < 5 {
		return nil, ErrInvalidMTEF
	}
	switch Version(data[0]) {
	case MTEF3:
		return parseV3(data)
	case MTEF4:
		return parseV4(data)
	case MTEF5:
		return parseV5(data)
	default:
		return nil, fmt.Errorf("%w: %d", ErrUnsupportedVersion, data[0])
	}
}

type reader struct {
	b []byte
	p int
}

func (r *reader) left() int { return len(r.b) - r.p }
func (r *reader) u8() (byte, error) {
	if r.left() < 1 {
		return 0, ErrInvalidMTEF
	}
	v := r.b[r.p]
	r.p++
	return v, nil
}
func (r *reader) u16() (uint16, error) {
	if r.left() < 2 {
		return 0, ErrInvalidMTEF
	}
	v := uint16(r.b[r.p]) | uint16(r.b[r.p+1])<<8
	r.p += 2
	return v, nil
}
func (r *reader) i16() (int16, error) { v, e := r.u16(); return int16(v), e }
func (r *reader) cstr() (string, error) {
	s := r.p
	for r.p < len(r.b) && r.b[r.p] != 0 {
		r.p++
	}
	if r.p >= len(r.b) {
		return "", ErrInvalidMTEF
	}
	v := string(r.b[s:r.p])
	r.p++
	return v, nil
}
func (r *reader) nudge() (Offset, error) {
	x, e := r.u8()
	if e != nil {
		return Offset{}, e
	}
	y, e := r.u8()
	if e != nil {
		return Offset{}, e
	}
	if x == 128 && y == 128 {
		xx, e := r.i16()
		if e != nil {
			return Offset{}, e
		}
		yy, e := r.i16()
		return Offset{xx, yy}, e
	}
	return Offset{int16(x) - 128, int16(y) - 128}, nil
}
func row(items []Expr) Expr {
	if len(items) == 1 {
		return items[0]
	}
	return &Row{Items: items}
}
