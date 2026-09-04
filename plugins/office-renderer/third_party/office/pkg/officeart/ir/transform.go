package ir

import "math"

// Transform stores authored shape bounds and rotation/mirroring.
//
// RotationDeg is clockwise, matching the public Office IR convention.
type Transform struct {
	Rect        Rect
	RotationDeg float64
	FlipH       bool
	FlipV       bool
}

// Matrix is a 2D affine matrix:
//
//	[a c e]
//	[b d f]
//	[0 0 1]
type Matrix struct {
	A, B, C, D, E, F float64
}

func IdentityMatrix() Matrix {
	return Matrix{A: 1, D: 1}
}

func (m Matrix) Mul(n Matrix) Matrix {
	return Matrix{
		A: m.A*n.A + m.C*n.B,
		B: m.B*n.A + m.D*n.B,
		C: m.A*n.C + m.C*n.D,
		D: m.B*n.C + m.D*n.D,
		E: m.A*n.E + m.C*n.F + m.E,
		F: m.B*n.E + m.D*n.F + m.F,
	}
}

// LocalMatrix returns the authored transform around the shape center.
// Coordinates are in EMU.
func (t Transform) LocalMatrix() Matrix {
	cx := float64(t.Rect.X) + float64(t.Rect.W)/2
	cy := float64(t.Rect.Y) + float64(t.Rect.H)/2

	m := Matrix{A: 1, D: 1, E: float64(t.Rect.X), F: float64(t.Rect.Y)}
	center := Matrix{A: 1, D: 1, E: cx - float64(t.Rect.X), F: cy - float64(t.Rect.Y)}
	uncenter := Matrix{A: 1, D: 1, E: -(cx - float64(t.Rect.X)), F: -(cy - float64(t.Rect.Y))}

	sx, sy := 1.0, 1.0
	if t.FlipH {
		sx = -1
	}
	if t.FlipV {
		sy = -1
	}
	scale := Matrix{A: sx, D: sy}

	rad := t.RotationDeg * math.Pi / 180
	rotate := Matrix{
		A: math.Cos(rad),
		B: math.Sin(rad),
		C: -math.Sin(rad),
		D: math.Cos(rad),
	}

	return m.Mul(center).Mul(rotate).Mul(scale).Mul(uncenter)
}

// EffectiveMatrix composes parent/group transforms from outermost to innermost.
func EffectiveMatrix(parents []Matrix, local Matrix) Matrix {
	out := IdentityMatrix()
	for _, parent := range parents {
		out = out.Mul(parent)
	}
	return out.Mul(local)
}
