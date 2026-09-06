package ir

type TextWrap string

const (
	TextWrapSquare TextWrap = "square"
	TextWrapNone   TextWrap = "none"
)

type TextVertical string

const (
	TextHorizontal      TextVertical = ""
	TextVerticalRL      TextVertical = "vert"
	TextVertical270     TextVertical = "vert270"
	TextWordArtVertical TextVertical = "wordArtVert"
)

type TextAnchor string

const (
	TextAnchorTop    TextAnchor = "top"
	TextAnchorCenter TextAnchor = "center"
	TextAnchorBottom TextAnchor = "bottom"
)

type AutoFit string

const (
	AutoFitDefault AutoFit = ""
	AutoFitNone    AutoFit = "none"
	AutoFitShape   AutoFit = "shape"
	AutoFitNormal  AutoFit = "normal"
)

// Insets stores text-body padding in EMU.
type Insets struct {
	Left   int64
	Top    int64
	Right  int64
	Bottom int64
}

type TextBody struct {
	Insets Insets
	Wrap   TextWrap
	Vert   TextVertical
	Anchor TextAnchor

	AnchorCenter bool
	Upright      bool
	RotationDeg  float64
	AutoFit      AutoFit

	Columns   int
	ColumnGap int64

	Paragraphs []Paragraph
}

// GeometryText preserves the shared OfficeArt geometry-text/WordArt semantics
// that cannot be represented by an ordinary text body alone.
type GeometryText struct {
	Text       string
	FontFamily string
	FontSizePt float64
	Spacing    float64
	Align      uint32
	Warp       string

	ReverseRows   bool
	Vertical      bool
	Kerning       bool
	Tight         bool
	Stretch       bool
	ShrinkFit     bool
	BestFit       bool
	Normalize     bool
	DxMeasure     bool
	Bold          bool
	Italic        bool
	Underline     bool
	Shadow        bool
	SmallCaps     bool
	Strikethrough bool
}

type Paragraph struct {
	Runs          []TextRun
	Align         string
	Level         int
	SpaceBeforePt float64
	SpaceAfterPt  float64
	LineHeightPt  float64
}

type TextRun struct {
	Text       string
	FontFamily string
	FontSizePt float64
	Bold       bool
	Italic     bool
	Underline  bool
	Color      string
}
