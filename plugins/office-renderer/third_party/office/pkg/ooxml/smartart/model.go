package smartart

import (
	"office/pkg/diag"
	"office/pkg/ooxml/drawing"
)

// Connection represents a single data-model edge between two nodes.
type Connection struct {
	// SrcID is the source node identifier.
	SrcID string

	// DestID is the destination node identifier.
	DestID string
}

// Diagram represents a parsed SmartArt diagram.
type Diagram struct {
	// Type is the diagram category (hierarchy, process, cycle, etc.)
	Type string

	// Nodes are the data elements in the diagram
	Nodes []*Node

	// HelperPoints holds transition and presentation helper points that are not
	// rendered as diagram shapes but must be preserved for round-trip fidelity.
	HelperPoints []*HelperPoint

	// Connections are the data-model edges in declaration order (excluding
	// presentation-only connections). They are used to match pre-rendered
	// drawing connectors to the diagram's logical edges.
	Connections []Connection

	// Layout defines the positioning algorithm
	Layout *Layout

	// Style defines visual styling
	Style *Style

	// ExtLst holds the raw diagram-level dgm:extLst element, if any.
	ExtLst []byte

	// Bg holds the raw dgm:bg element, if any.
	Bg []byte

	// Whole holds the raw dgm:whole element, if any.
	Whole []byte
	// BackgroundFill is the semantic fill parsed from dgm:bg when available.
	BackgroundFill drawing.Fill
	// WholeEffect is the semantic whole-diagram effect parsed from dgm:whole.
	WholeEffect drawing.EffectList

	// RootAttrs holds the raw attributes of the dgm:dataModel root element so
	// namespace declarations and other root attributes can be preserved during
	// serialization.
	RootAttrs string

	// Diagnostics accumulates non-fatal parser issues for this diagram.
	Diagnostics diag.List
}

// Node represents a single element in the diagram.
type Node struct {
	// ID is the unique identifier for this node
	ID string

	// Type is the node type (root, node, assistant, etc.)
	Type string

	// Text is the displayed content (plain-text fallback).
	Text string

	// Paragraphs holds styled text runs when the source provides them.
	Paragraphs []*TextParagraph

	// Children are subordinate nodes
	Children []*Node

	// Parent is the parent node (nil for root)
	Parent *Node

	// Level is the depth in the hierarchy
	Level int

	// Assistant is true when the source marks this node as an assistant
	// (dgm:pt/@type="asst").
	Assistant bool

	// Position is the layout position (calculated or supplied by the drawing part)
	Position Position

	// RotationDeg is the clockwise rotation of the node in degrees, supplied by
	// the pre-rendered drawing part or by the layout engine.
	RotationDeg float64

	// FlipH and FlipV reflect horizontal/vertical flips from the drawing part.
	FlipH bool
	FlipV bool

	// HasPosition is true when Position was supplied by a pre-rendered drawing
	// part and should not be overwritten by the layout engine.
	HasPosition bool

	// DrawingOrder is the position of this node in the source dsp:spTree,
	// used to preserve front-to-back ordering.
	DrawingOrder int

	// Fill, Line, Geometry, and TextColor allow the pre-rendered drawing part
	// to override the diagram-wide style for this node.
	Fill                drawing.Fill
	Line                *drawing.LineStyle
	Effect              drawing.EffectList
	Geometry            string
	GeometryAdjustments []float64
	CustomGeometry      *drawing.Geometry
	TextColor           string

	// TextBodyProps holds text-body properties (anchor, rotation, insets, etc.)
	// extracted from the node's dgm:t/a:bodyPr or from the pre-rendered drawing
	// part's dsp:txBody/a:bodyPr.
	TextBodyProps drawing.TextBodyProperties

	// PropertySet holds attributes from the node's dgm:prSet element, such as
	// layout/style/color type IDs, placeholder flag, and placeholder text.
	PropertySet PropertySet

	// ExtLst holds the raw dgm:extLst child of this point, if any.
	ExtLst []byte
}

// HelperPoint represents a non-rendered dgm:pt (pres, parTrans, sibTrans).
type HelperPoint struct {
	// ModelID is the point's modelId (or id) attribute.
	ModelID string

	// Type is the point type (pres, parTrans, sibTrans).
	Type string

	// RawXML is the verbatim dgm:pt element so unmodeled content is preserved.
	RawXML []byte

	// Text is the plain-text content, if any.
	Text string
}

// PropertySet captures selected attributes from a dgm:prSet element.
type PropertySet struct {
	// LayoutTypeID maps the node to a layout-definition style label.
	LayoutTypeID string
	// QuickStyleTypeID maps the node to a quick-style label.
	QuickStyleTypeID string
	// ColorTypeID maps the node to a colors-definition label.
	ColorTypeID string
	// Placeholder is true when the node is a placeholder.
	Placeholder bool
	// PlaceholderText is the placeholder text to show when the node is empty.
	PlaceholderText string
	// CustomAngle is an optional custom rotation angle in degrees.
	CustomAngle int
}

// Position represents a node's location in the diagram.
type Position struct {
	X, Y          float64
	Width, Height float64
}

// TextRun represents a single styled text run inside a SmartArt paragraph.
type TextRun struct {
	Text         string
	Bold         bool
	Italic       bool
	Underline    bool
	FontSize     int    // hundredths of a point
	FontFamily   string // Latin typeface
	FontFamilyEA string // East Asian typeface
	FontFamilyCS string // Complex Script typeface
	Color        string
	Effects      drawing.EffectList
}

// TextParagraph represents a paragraph of styled text inside a SmartArt shape.
type TextParagraph struct {
	Align             string
	Level             int
	DefaultTabSizeEMU int
	TabStops          []TabStop
	BulletChar        string
	AutoNum           bool
	AutoNumType       string
	AutoNumStart      int
	BulletColor       string
	BulletFontFamily  string
	BulletSize        int
	BulletSizePct     int
	// SpaceBefore and SpaceAfter are paragraph spacing values in hundredths of
	// a point when the source used a:spcBef/a:spcAft@spcPts.
	SpaceBefore int
	SpaceAfter  int
	// SpaceBeforePct and SpaceAfterPct are thousandths of a percent of the
	// font size when the source used spcPct.
	SpaceBeforePct int
	SpaceAfterPct  int
	// LineSpacing is the line spacing value. LineSpacingPct is thousandths of
	// a percent of the font size; LineSpacingPts is hundredths of a point.
	LineSpacingPct int
	LineSpacingPts int
	Runs           []TextRun
}

// TabStop is a DrawingML tab stop inside a SmartArt text paragraph.
type TabStop struct {
	Pos  int64
	Algn string
}

// Layout defines how nodes are positioned.
type Layout struct {
	// Type is the layout algorithm (hierarchy, flowchart, etc.)
	Type string

	// Direction for hierarchical layouts (topDown, leftRight, etc.)
	Direction string

	// Spacing between elements
	HSpacing, VSpacing float64

	// Connector* fields preserve the active dgm:alg type="conn" parameters so
	// presentation renderers can synthesize connector geometry when no dsp:drawing
	// connector is available. Values retain the DiagramML token vocabulary.
	ConnectorRouting    string
	ConnectorBendPoint  string
	ConnectorBeginPts   string
	ConnectorEndPts     string
	ConnectorSource     string
	ConnectorDest       string
	ConnectorDimension  string
	ConnectorBeginStyle string
	ConnectorEndStyle   string
	ConnectorSourceIDs  []string
	ConnectorDestIDs    []string

	// Definition is the parsed dgm:layoutDef algorithm tree. When present, the
	// layout engine can execute it for higher-fidelity fallback layouts.
	Definition *LayoutDefinition
}

// ThemeFormatScheme holds resolved DrawingML fills, lines, effects, and font
// slots from the active theme's a:fmtScheme. It is format-agnostic so the
// shared smartart package can resolve a:fillRef/a:lnRef/a:effectRef/a:fontRef
// without importing Word or Presentation part types.
type ThemeFormatScheme struct {
	FillStyles   []ThemeFillStyle
	LineStyles   []ThemeLineStyle
	EffectStyles []drawing.EffectList
	BgFillStyles []ThemeFillStyle
	MajorFont    string
	MinorFont    string
}

// ThemeFillStyle is a resolved fill entry plus a flag indicating whether the
// entry's color is the placeholder (phClr) that should be replaced by the
// a:fillRef/a:lnRef child color when one is supplied.
type ThemeFillStyle struct {
	Fill            drawing.Fill
	UsesPlaceholder bool
}

// ThemeLineStyle is a resolved line entry plus a placeholder-color flag.
type ThemeLineStyle struct {
	Line            *drawing.LineStyle
	UsesPlaceholder bool
}

// Style defines visual appearance.
type Style struct {
	// Fill is the DrawingML fill for shapes.
	Fill drawing.Fill

	// Line is the DrawingML line style for shape borders.
	Line *drawing.LineStyle

	// Effect is the resolved effect list for shapes (shadows, glows, etc.).
	Effect drawing.EffectList

	// TextColor is the default text color (hex RGB).
	TextColor string

	// Shape type (rectangle, ellipse, etc.)
	ShapeType string

	// ColorPalette is an ordered list of hex RGB colors extracted from the
	// diagram's colorsDef part (when available). The renderer can cycle
	// through these instead of using hardcoded defaults.
	ColorPalette []string

	// LinePalette is an ordered list of hex RGB colors extracted from the
	// diagram's colorsDef part for shape/connector outlines.
	LinePalette []string

	// TextFillPalette is an ordered list of hex RGB colors for text fills.
	TextFillPalette []string

	// TextLinePalette is an ordered list of hex RGB colors for text outlines.
	TextLinePalette []string

	// EffectPalette is an ordered list of hex RGB colors for effects.
	EffectPalette []string

	// ConnectorLine is the line style to use for connectors when the source
	// (pre-rendered drawing part or styleDef) provides one.
	ConnectorLine *drawing.LineStyle
}
