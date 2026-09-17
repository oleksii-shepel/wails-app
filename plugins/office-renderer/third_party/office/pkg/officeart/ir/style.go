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

	// ScaleAngleWithAspect preserves msofillShadeScale semantics until physical layout,
	// where the final painted width/height are known.
	ScaleAngleWithAspect bool

	// LegacyShadeType preserves the originating MSOFILLTYPE shade semantic
	// (shade, shadeCenter, shadeShape, shadeScale, shadeTitle) so host-specific
	// behavior is not lost when several variants share one canonical geometry.
	LegacyShadeType string

	// FillTo* are normalized OfficeArt center-rectangle insets. They are
	// preserved separately from geometry because radial/concentric fills use
	// them to position the gradient focus inside the painted shape.
	HasFillTo    bool
	FillToLeft   float64
	FillToTop    float64
	FillToRight  float64
	FillToBottom float64
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
