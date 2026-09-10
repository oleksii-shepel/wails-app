package ir

// Drawing is the format-neutral geometry contract shared by Word images,
// text boxes, and shapes. Parsers normalize DOC/DOCX details into these
// semantics; layout and render code should consume this contract instead of
// switching on the source format or payload type.
type Drawing interface {
	DrawingKind() string
	DrawingPlacement() DrawingPlacement
}

// DrawingPlacement is the canonical, paint-independent placement/wrap state for
// a Word drawing. Coordinates and extents are EMU unless explicitly marked as
// points. WrapPolygon coordinates are object-relative points.
type DrawingPlacement struct {
	Floating bool
	X, Y     int64
	Width    int64
	Height   int64

	XRelative string
	YRelative string
	XAlign    string
	YAlign    string

	ZIndex    int64
	BehindDoc bool

	WrapType    string
	WrapSide    string
	WrapPolygon [][2]float64
	DistTop     float64
	DistBottom  float64
	DistLeft    float64
	DistRight   float64

	SimplePosAttr bool
	SimplePosX    int64
	SimplePosY    int64

	EffectExtentL int64
	EffectExtentT int64
	EffectExtentR int64
	EffectExtentB int64

	Locked       bool
	LayoutInCell bool
	AllowOverlap bool

	SizeRelHPct  int64
	SizeRelHFrom string
	SizeRelVPct  int64
	SizeRelVFrom string

	RotationDeg float64
	FlipH       bool
	FlipV       bool
}

func (p DrawingPlacement) IsFloating() bool { return p.Floating }

func (p DrawingPlacement) HasWrapExclusion() bool {
	if !p.Floating || p.BehindDoc {
		return false
	}
	switch p.WrapType {
	case "none":
		return false
	case "square", "tight", "through", "topAndBottom":
		return true
	default:
		// Legacy DOC shapes historically omit an explicit wrap type; Word treats
		// those floating shapes as square-wrapped unless their anchor says otherwise.
		return true
	}
}

func (p DrawingPlacement) EffectiveX() int64 {
	if p.SimplePosAttr {
		return p.SimplePosX
	}
	return p.X
}

func (p DrawingPlacement) EffectiveY() int64 {
	if p.SimplePosAttr {
		return p.SimplePosY
	}
	return p.Y
}

// ApplyTo copies canonical geometry/flow state into any supported drawing
// payload. Paint/content state stays owned by the concrete payload.
func (p DrawingPlacement) ApplyTo(d Drawing) {
	switch value := d.(type) {
	case *Image:
		p.applyToImage(value)
	case *TextBox:
		p.applyToTextBox(value)
	case *Shape:
		p.applyToShape(value)
	}
}

// ApplyToTextBox is retained as the public textbox projection used by shape
// content code; generic parser/layout code should prefer ApplyTo.
func (p DrawingPlacement) ApplyToTextBox(t *TextBox) { p.applyToTextBox(t) }

func (p DrawingPlacement) applyToImage(i *Image) {
	if i == nil {
		return
	}
	i.Floating = p.Floating
	i.X, i.Y = p.X, p.Y
	i.Width, i.Height = p.Width, p.Height
	i.XRelative, i.YRelative = p.XRelative, p.YRelative
	i.XAlign, i.YAlign = p.XAlign, p.YAlign
	i.ZIndex, i.BehindDoc = p.ZIndex, p.BehindDoc
	i.WrapType, i.WrapSide = p.WrapType, p.WrapSide
	i.WrapPolygon = cloneWrapPolygon(p.WrapPolygon)
	i.DistTop, i.DistBottom = p.DistTop, p.DistBottom
	i.DistLeft, i.DistRight = p.DistLeft, p.DistRight
	i.SimplePosAttr = p.SimplePosAttr
	i.SimplePosX, i.SimplePosY = p.SimplePosX, p.SimplePosY
	i.EffectExtentL, i.EffectExtentT = p.EffectExtentL, p.EffectExtentT
	i.EffectExtentR, i.EffectExtentB = p.EffectExtentR, p.EffectExtentB
	i.Locked, i.LayoutInCell, i.AllowOverlap = p.Locked, p.LayoutInCell, p.AllowOverlap
	i.SizeRelHPct, i.SizeRelHFrom = p.SizeRelHPct, p.SizeRelHFrom
	i.SizeRelVPct, i.SizeRelVFrom = p.SizeRelVPct, p.SizeRelVFrom
	i.RotationDeg, i.FlipH, i.FlipV = p.RotationDeg, p.FlipH, p.FlipV
}

func (p DrawingPlacement) applyToTextBox(t *TextBox) {
	if t == nil {
		return
	}
	t.Floating = p.Floating
	t.X, t.Y = p.X, p.Y
	t.Width, t.Height = p.Width, p.Height
	t.XRelative, t.YRelative = p.XRelative, p.YRelative
	t.XAlign, t.YAlign = p.XAlign, p.YAlign
	t.ZIndex, t.BehindDoc = p.ZIndex, p.BehindDoc
	t.WrapType, t.WrapSide = p.WrapType, p.WrapSide
	t.WrapPolygon = cloneWrapPolygon(p.WrapPolygon)
	t.DistTop, t.DistBottom = p.DistTop, p.DistBottom
	t.DistLeft, t.DistRight = p.DistLeft, p.DistRight
	t.SimplePosAttr = p.SimplePosAttr
	t.SimplePosX, t.SimplePosY = p.SimplePosX, p.SimplePosY
	t.EffectExtentL, t.EffectExtentT = p.EffectExtentL, p.EffectExtentT
	t.EffectExtentR, t.EffectExtentB = p.EffectExtentR, p.EffectExtentB
	t.Locked, t.LayoutInCell, t.AllowOverlap = p.Locked, p.LayoutInCell, p.AllowOverlap
	t.SizeRelHPct, t.SizeRelHFrom = p.SizeRelHPct, p.SizeRelHFrom
	t.SizeRelVPct, t.SizeRelVFrom = p.SizeRelVPct, p.SizeRelVFrom
	t.RotationDeg, t.FlipH, t.FlipV = p.RotationDeg, p.FlipH, p.FlipV
}

func (p DrawingPlacement) applyToShape(s *Shape) {
	if s == nil {
		return
	}
	s.Floating = p.Floating
	s.X, s.Y = p.X, p.Y
	s.Width, s.Height = p.Width, p.Height
	s.XRelative, s.YRelative = p.XRelative, p.YRelative
	s.XAlign, s.YAlign = p.XAlign, p.YAlign
	s.ZIndex, s.BehindDoc = p.ZIndex, p.BehindDoc
	s.WrapType, s.WrapSide = p.WrapType, p.WrapSide
	s.WrapPolygon = cloneWrapPolygon(p.WrapPolygon)
	s.DistTop, s.DistBottom = p.DistTop, p.DistBottom
	s.DistLeft, s.DistRight = p.DistLeft, p.DistRight
	s.SimplePosAttr = p.SimplePosAttr
	s.SimplePosX, s.SimplePosY = p.SimplePosX, p.SimplePosY
	s.EffectExtentL, s.EffectExtentT = p.EffectExtentL, p.EffectExtentT
	s.EffectExtentR, s.EffectExtentB = p.EffectExtentR, p.EffectExtentB
	s.Locked, s.LayoutInCell, s.AllowOverlap = p.Locked, p.LayoutInCell, p.AllowOverlap
	s.SizeRelHPct, s.SizeRelHFrom = p.SizeRelHPct, p.SizeRelHFrom
	s.SizeRelVPct, s.SizeRelVFrom = p.SizeRelVPct, p.SizeRelVFrom
	s.RotationDeg, s.FlipH, s.FlipV = p.RotationDeg, p.FlipH, p.FlipV
}

func (i *Image) DrawingKind() string { return "image" }

func (i *Image) DrawingPlacement() DrawingPlacement {
	if i == nil {
		return DrawingPlacement{}
	}
	return DrawingPlacement{
		Floating: i.Floating, X: i.X, Y: i.Y, Width: i.Width, Height: i.Height,
		XRelative: i.XRelative, YRelative: i.YRelative, XAlign: i.XAlign, YAlign: i.YAlign,
		ZIndex: i.ZIndex, BehindDoc: i.BehindDoc,
		WrapType: i.WrapType, WrapSide: i.WrapSide, WrapPolygon: cloneWrapPolygon(i.WrapPolygon),
		DistTop: i.DistTop, DistBottom: i.DistBottom, DistLeft: i.DistLeft, DistRight: i.DistRight,
		SimplePosAttr: i.SimplePosAttr, SimplePosX: i.SimplePosX, SimplePosY: i.SimplePosY,
		EffectExtentL: i.EffectExtentL, EffectExtentT: i.EffectExtentT, EffectExtentR: i.EffectExtentR, EffectExtentB: i.EffectExtentB,
		Locked: i.Locked, LayoutInCell: i.LayoutInCell, AllowOverlap: i.AllowOverlap,
		SizeRelHPct: i.SizeRelHPct, SizeRelHFrom: i.SizeRelHFrom, SizeRelVPct: i.SizeRelVPct, SizeRelVFrom: i.SizeRelVFrom,
		RotationDeg: i.RotationDeg, FlipH: i.FlipH, FlipV: i.FlipV,
	}
}

func (t *TextBox) DrawingKind() string { return "textbox" }

func (t *TextBox) DrawingPlacement() DrawingPlacement {
	if t == nil {
		return DrawingPlacement{}
	}
	return DrawingPlacement{
		Floating: t.Floating, X: t.X, Y: t.Y, Width: t.Width, Height: t.Height,
		XRelative: t.XRelative, YRelative: t.YRelative, XAlign: t.XAlign, YAlign: t.YAlign,
		ZIndex: t.ZIndex, BehindDoc: t.BehindDoc,
		WrapType: t.WrapType, WrapSide: t.WrapSide, WrapPolygon: cloneWrapPolygon(t.WrapPolygon),
		DistTop: t.DistTop, DistBottom: t.DistBottom, DistLeft: t.DistLeft, DistRight: t.DistRight,
		SimplePosAttr: t.SimplePosAttr, SimplePosX: t.SimplePosX, SimplePosY: t.SimplePosY,
		EffectExtentL: t.EffectExtentL, EffectExtentT: t.EffectExtentT, EffectExtentR: t.EffectExtentR, EffectExtentB: t.EffectExtentB,
		Locked: t.Locked, LayoutInCell: t.LayoutInCell, AllowOverlap: t.AllowOverlap,
		SizeRelHPct: t.SizeRelHPct, SizeRelHFrom: t.SizeRelHFrom, SizeRelVPct: t.SizeRelVPct, SizeRelVFrom: t.SizeRelVFrom,
		RotationDeg: t.RotationDeg, FlipH: t.FlipH, FlipV: t.FlipV,
	}
}

func (s *Shape) DrawingKind() string { return "shape" }

func (s *Shape) DrawingPlacement() DrawingPlacement {
	if s == nil {
		return DrawingPlacement{}
	}
	return DrawingPlacement{
		Floating: s.Floating, X: s.X, Y: s.Y, Width: s.Width, Height: s.Height,
		XRelative: s.XRelative, YRelative: s.YRelative, XAlign: s.XAlign, YAlign: s.YAlign,
		ZIndex: s.ZIndex, BehindDoc: s.BehindDoc,
		WrapType: s.WrapType, WrapSide: s.WrapSide, WrapPolygon: cloneWrapPolygon(s.WrapPolygon),
		DistTop: s.DistTop, DistBottom: s.DistBottom, DistLeft: s.DistLeft, DistRight: s.DistRight,
		SimplePosAttr: s.SimplePosAttr, SimplePosX: s.SimplePosX, SimplePosY: s.SimplePosY,
		EffectExtentL: s.EffectExtentL, EffectExtentT: s.EffectExtentT, EffectExtentR: s.EffectExtentR, EffectExtentB: s.EffectExtentB,
		Locked: s.Locked, LayoutInCell: s.LayoutInCell, AllowOverlap: s.AllowOverlap,
		SizeRelHPct: s.SizeRelHPct, SizeRelHFrom: s.SizeRelHFrom, SizeRelVPct: s.SizeRelVPct, SizeRelVFrom: s.SizeRelVFrom,
		RotationDeg: s.RotationDeg, FlipH: s.FlipH, FlipV: s.FlipV,
	}
}

// DrawingFromRun returns the drawing payload carried by a run, independent of
// whether the source document represented it as a picture, textbox, or shape.
func DrawingFromRun(run *Run) Drawing {
	if run == nil {
		return nil
	}
	if run.Image != nil {
		return run.Image
	}
	if run.TextBox != nil {
		return run.TextBox
	}
	if run.Shape != nil {
		return run.Shape
	}
	return nil
}

func cloneWrapPolygon(points [][2]float64) [][2]float64 {
	if len(points) == 0 {
		return nil
	}
	out := make([][2]float64, len(points))
	copy(out, points)
	return out
}
