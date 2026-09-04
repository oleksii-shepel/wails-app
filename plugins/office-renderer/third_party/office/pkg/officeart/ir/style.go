package ir

type FillKind string

const (
	FillUnset    FillKind = ""
	FillNone     FillKind = "none"
	FillSolid    FillKind = "solid"
	FillGradient FillKind = "gradient"
	FillPattern  FillKind = "pattern"
	FillPicture  FillKind = "picture"
)

type Fill struct {
	Kind  FillKind
	Color string
	Alpha float64

	Gradient *GradientFill
	Pattern  *PatternFill
	Picture  *PictureFill
}

type GradientFill struct {
	Type  string
	Angle float64
	Stops []GradientStop
}

type GradientStop struct {
	Position float64
	Color    string
	Alpha    float64
}

type PatternFill struct {
	Preset     string
	Foreground string
	Background string
}

type PictureFill struct {
	Image Image
	Tile  *Tile
}

type Tile struct {
	OffsetXEMU int64
	OffsetYEMU int64
	ScaleX     float64
	ScaleY     float64
}

type Line struct {
	Visible bool
	Color   string
	Alpha   float64
	WidthPt float64
	Dash    string
	Cap     string
	Join    string

	StartArrow Arrow
	EndArrow   Arrow
}

type Effects struct {
	Shadow *Shadow
	Glow   *Glow
	Blur   *Blur
}

type Shadow struct {
	Color      string
	Alpha      float64
	BlurRadius float64
	Distance   float64
	AngleDeg   float64
	Inner      bool
}

type Glow struct {
	Color    string
	Alpha    float64
	RadiusPt float64
}

type Blur struct {
	RadiusPt float64
}
