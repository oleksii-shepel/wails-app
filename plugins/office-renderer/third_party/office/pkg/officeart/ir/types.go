package ir

// ShapeKind classifies an Office drawing node without tying it to a file format.
type ShapeKind string

const (
	ShapeUnknown   ShapeKind = "unknown"
	ShapeAuto      ShapeKind = "shape"
	ShapeTextBox   ShapeKind = "textbox"
	ShapePicture   ShapeKind = "picture"
	ShapeConnector ShapeKind = "connector"
	ShapeGroup     ShapeKind = "group"
)

// Rect is an Office drawing rectangle in EMU.
type Rect struct {
	X int64
	Y int64
	W int64
	H int64
}

// Point is an Office drawing point in EMU.
type Point struct {
	X int64
	Y int64
}

// Shape is the canonical OfficeArt visual node used by all six Office formats.
type Shape struct {
	ID        uint32
	Name      string
	Kind      ShapeKind
	ZOrder    int
	Hidden    bool
	Transform Transform

	Geometry Geometry
	Fill     Fill
	Line     Line
	Effects  Effects

	Text         *TextBody
	GeometryText *GeometryText
	Image        *Image

	Connector *Connector
	Children  []*Shape

	// Source preserves non-rendering provenance useful for diagnostics and
	// lossless adapters. Renderers must not branch on Source.Format.
	Source SourceRef
}

// SourceRef identifies where a shared shape came from without leaking parser
// structures into the shared IR.
type SourceRef struct {
	Format  string // doc, docx, xls, xlsx, ppt, pptx
	Part    string
	Record  uint16
	Offset  int64
	ShapeID uint32
}

// Image describes picture content, source cropping and OfficeArt picture
// adjustments. These semantics are shared by DOC/XLS/PPT and should not be
// re-decoded by application adapters.
type Image struct {
	Data     []byte
	MIMEType string
	RelID    string
	URL      string
	DPI      int

	Crop Crop

	Brightness float64 // CSS-style multiplier; 1 = unchanged, 0 = unset
	Contrast   float64 // CSS-style multiplier; 1 = unchanged, 0 = unset
	Grayscale  bool
	BiLevel    bool
}

// Crop stores normalized [0,1] crop fractions from each source edge.
type Crop struct {
	Left   float64
	Top    float64
	Right  float64
	Bottom float64
}

// Connector describes connector routing and endpoints.
type Connector struct {
	Type        string
	Start       *Point
	End         *Point
	Path        []Point
	StartArrow  Arrow
	EndArrow    Arrow
	Adjustments []float64
}

// Arrow describes one connector/line endpoint decoration.
type Arrow struct {
	Type   string
	Width  string
	Length string
}
