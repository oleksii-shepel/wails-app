package ir

type GeometryKind string

const (
	GeometryNone   GeometryKind = ""
	GeometryPreset GeometryKind = "preset"
	GeometryCustom GeometryKind = "custom"
)

// Geometry is intentionally independent from DrawingML and Escher geometry
// structures. Frontends normalize either representation into this form.
type Geometry struct {
	Kind        GeometryKind
	Preset      string
	Adjustments []float64
	Paths       []Path
	ViewBox     Rect
}

// Path is a normalized vector path.
type Path struct {
	Commands []PathCommand
	FillRule string
	Fill     bool
	Stroke   bool
}

// PathCommand uses Office-independent SVG-like command names: M, L, C, Q, Z.
type PathCommand struct {
	Op     string
	Points []PointF
}

// PointF is used for normalized/custom geometry coordinates.
type PointF struct {
	X float64
	Y float64
}
