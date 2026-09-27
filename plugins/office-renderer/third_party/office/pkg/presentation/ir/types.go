package ir

import (
	"office/pkg/diag"
	officeartir "office/pkg/officeart/ir"
	"office/pkg/ooxml/drawing"
)

// Presentation is the resolved PresentationML IR.
type Presentation struct {
	// SlideWidthEMU and SlideHeightEMU are the canvas dimensions.
	SlideWidthEMU  int64
	SlideHeightEMU int64
	Slides         []*Slide
	// Sections holds ordered section names when the deck defines slide sections.
	Sections []string
	// EmbeddedFonts are presentation-level font faces that can be emitted into HTML.
	EmbeddedFonts []*EmbeddedFont
	// SlideMasters holds the slide masters for this presentation. When empty,
	// a default master and blank layout are created on write-back.
	SlideMasters []*SlideMaster
	// Theme is the first resolved theme, if any (presentation-level).
	Theme       *Theme
	Diagnostics diag.List
}

// PictureTile preserves DrawingML a:tile placement metadata for picture fills.
// Scale values use DrawingML percentages (100000 = 100%); offsets are EMU.
type PictureTile struct {
	Tx        int64
	Ty        int64
	Sx        int
	Sy        int
	Flip      string
	Alignment string
}

// Slide is a resolved presentation slide.
type Slide struct {
	// Index is the 0-based slide position in the deck.
	Index int
	// SectionName is the organizational section this slide belongs to, if any.
	SectionName string
	// MasterID is the SlideIdentifier referenced by SlideAtom.masterID (legacy PPT).
	// It is intentionally distinct from the master persist-directory RefID.
	MasterID int
	// MasterPersistID is the persist-directory RefID of the resolved legacy PPT master.
	// It is zero when the source format does not expose that identity or the master is unresolved.
	MasterPersistID int
	// LayoutType is the slide layout type from SSlideLayoutAtom (legacy PPT).
	// Values include "title", "titleBody", "blank", "titleOnly", etc.
	LayoutType string
	// BgColor is the slide background as sRGB hex (e.g. "FFFFFF"), or "" for white/default.
	BgColor string
	// BgFill preserves the shared DrawingML background fill when available.
	BgFill drawing.Fill
	// BgPictureTile preserves a:tile scale/offset/alignment/flip metadata.
	BgPictureTile *PictureTile
	// Shapes holds all content shapes on the slide in tree order.
	Shapes []*Shape
	// SmartArt holds embedded SmartArt diagrams that have not been flattened to
	// primitive shapes. The converter turns these into dgm:dataModel parts.
	SmartArt []*SmartArt
	// Notes holds the notes slide content, if present.
	Notes *NotesSlide
	// Header holds the header placeholder shape, if present.
	Header *Shape
	// Footer holds the footer placeholder shape, if present.
	Footer *Shape
	// DateTime holds the date/time placeholder shape, if present.
	DateTime *Shape
	// SlideNumber holds the slide number placeholder shape, if present.
	SlideNumber *Shape
	// Transition holds the slide transition metadata when present in the source file.
	Transition *SlideTransition
	// AnimationTimeline contains scheduler-relevant events resolved from p:timing.
	// The source timing XML remains preserved separately by the OOXML parts layer.
	AnimationTimeline []AnimationEvent
	// Master is the slide master assigned to this slide, if any.
	Master *SlideMaster
	// Layout is the slide layout assigned to this slide, if any.
	Layout *SlideLayout
	// Comments holds the resolved comments for this slide, ordered by author then index.
	Comments []*Comment
	// Diagnostics accumulates non-fatal issues encountered while resolving
	// this slide (e.g., SmartArt parsing warnings).
	Diagnostics diag.List
}

// SlideTransition describes the slide-change animation effect for this slide.
type SlideTransition struct {
	// Type is the transition effect name (e.g. "fade", "push", "wipe", "blinds",
	// "fly", "zoom", "split", "wheel", "strips", "cut", "dissolve", "checker",
	// "randomBar", "plus", "wedge", "circle", "diamond"). Empty means unspecified.
	Type string
	// Spd is the transition speed: "slow", "med", or "fast". Empty means default.
	Spd string
	// AdvClick is true when clicking advances to the next slide (default true).
	AdvClick bool
	// AdvTmMs is the auto-advance time in milliseconds. 0 means click-only advance.
	AdvTmMs int
}

// NotesSlide is a resolved notes slide.
type NotesSlide struct {
	// BgColor is the notes background as sRGB hex (e.g. "FFFFFF"), or "" for default styling.
	BgColor string
	// Shapes holds all content shapes on the notes slide in tree order.
	Shapes []*Shape
}

// Comment is a resolved annotation left by an author on a slide.
type Comment struct {
	// AuthorName is the display name of the author, or empty if not found in commentAuthors.
	AuthorName string
	// Initials is the author's initials, or empty if not available.
	Initials string
	// DateTime is the ISO 8601 date/time string from the XML.
	DateTime string
	// Text is the plain-text body of the comment.
	Text string
	// X and Y are the comment anchor position in EMU.
	X int64
	Y int64
}

// EffectOverlay is a resolved DrawingML fillOverlay effect.
// Blend stores the OOXML blend mode and Fill stores the resolved overlay fill.
type EffectOverlay struct {
	Blend string
	Fill  drawing.Fill
}

// Shape is a resolved content object on a slide (text box, image, group, or connector).
type Shape struct {
	// OfficeArt is the canonical cross-format drawing representation.
	OfficeArt *officeartir.Shape
	// ID is the non-visual shape id from the XML.
	ID int
	// OriginalShapeID is the original ESCHER shape ID from the legacy parser.
	OriginalShapeID uint32
	// Name is the non-visual shape name.
	Name string
	// Kind classifies the shape content.
	Kind ShapeKind
	// Placeholder indicates a semantic role if this shape is a placeholder.
	Placeholder PlaceholderKind
	// PlaceholderIdx is the placeholder index (p:ph@idx) used for matching
	// placeholders across master, layout, and slide.
	PlaceholderIdx int
	// PlaceholderOrient is the placeholder orientation: "horz" or "vert".
	PlaceholderOrient string
	// PlaceholderSize is the placeholder size: "full", "half", or "quarter".
	PlaceholderSize string
	// PlaceholderHasCustomPrompt indicates whether the placeholder shows a
	// custom prompt string instead of the default.
	PlaceholderHasCustomPrompt bool
	// Bounds is the shape position and size in EMU.
	Bounds Rect
	// FillColor is the shape's solid fill in sRGB hex (e.g. "FF0000"), or "" for no/inherited fill.
	FillColor string
	// Fill preserves shared DrawingML fill semantics when available.
	Fill drawing.Fill
	// PictureTile preserves a:tile metadata when Fill is a tiled picture fill.
	PictureTile *PictureTile
	// Effects preserves shared DrawingML visual effects when available.
	Effects drawing.EffectList
	// EffectOverlay preserves a DrawingML fillOverlay effect separately from the
	// shared effect list, which does not currently expose a fill-overlay primitive.
	EffectOverlay *EffectOverlay
	// Geometry is the DrawingML preset geometry name (e.g. "rect", "ellipse", "roundRect").
	// Empty or "rect" means a plain rectangle.
	Geometry string
	// GeometryData preserves shared DrawingML geometry for custom shapes.
	GeometryData *drawing.Geometry
	// GeometryAdjustments holds preset-geometry adjustment values (0-1) when
	// the source supplies them via a:avLst.
	GeometryAdjustments []float64
	// DrawingOrder preserves the source drawing order for SmartArt shapes.
	DrawingOrder int
	// HasTransform preserves a:xfrm provenance so placeholder inheritance can
	// distinguish an absent transform from an authored zero-valued transform.
	HasTransform bool
	// RotationDeg is the clockwise shape rotation in degrees.
	RotationDeg float64
	// FlipH indicates horizontal mirroring of the shape.
	FlipH bool
	// FlipV indicates vertical mirroring of the shape.
	FlipV bool
	// TextAnchor is the vertical text anchoring mode from bodyPr ("t", "ctr", "b").
	TextAnchor string
	// TextInset* are the resolved text-box insets in EMU. HasTextInsets preserves
	// source provenance so an explicit zero inset is not confused with "unset".
	HasTextInsets      bool
	TextInsetLeftEMU   int64
	TextInsetRightEMU  int64
	TextInsetTopEMU    int64
	TextInsetBottomEMU int64
	// TextAllowOverflow preserves noAutofit text-body semantics.
	TextAllowOverflow bool
	// TextAutoGrow marks shapes with spAutoFit where the container grows to fit content.
	TextAutoGrow bool
	// SentinelPlaceholder marks textbox-like shapes that carry the Google Slides sentinel placeholder idx.
	SentinelPlaceholder bool
	// TextBodyProps holds DrawingML text body properties for CSS generation.
	TextBodyProps drawing.TextBodyProperties
	// Paragraphs holds text content for Text shapes.
	Paragraphs []*Paragraph
	// ChartData holds the raw chart XML bytes for Chart shapes.
	ChartData []byte
	// ImageRelID is the slide-local relationship ID for Image shapes.
	ImageRelID string
	// ImageData holds the raw image bytes for inline embedding (nil if not loaded).
	ImageData []byte
	// ImageMIME is the MIME type for ImageData (e.g. "image/jpeg").
	ImageMIME string
	// ImageEffects preserves DrawingML effects attached directly to a picture's a:blip.
	// Picture-fill blip effects remain on drawing.PictureFill.BlipEffects.
	ImageEffects []drawing.Effect
	// ImageDPI is the authored blip-fill DPI used for intrinsic size calculation.
	ImageDPI int
	// ImageRotWithShape controls whether image pixels rotate with their picture shape.
	// When ImageRotWithShapeSet is false, DrawingML's default behavior is used.
	ImageRotWithShape    bool
	ImageRotWithShapeSet bool
	// ImageCrop* describe the visible source rectangle as fractions from each edge.
	ImageCropLeft   float64
	ImageCropTop    float64
	ImageCropRight  float64
	ImageCropBottom float64
	// ImageFill* describe the destination rectangle used by a:stretch/a:fillRect.
	ImageFillLeft   float64
	ImageFillTop    float64
	ImageFillRight  float64
	ImageFillBottom float64
	// MediaKind classifies rich media placeholders as "audio" or "video".
	MediaKind string
	// MediaRelID is the slide-local relationship ID for embedded media.
	MediaRelID string
	// MediaData holds the raw embedded media payload when loaded.
	MediaData []byte
	// MediaLinkURL holds an external audio/video target when the presentation links media instead of embedding it.
	MediaLinkURL string
	// MediaMIME is the MIME type for MediaData or MediaLinkURL (e.g. "video/mp4", "audio/mpeg").
	MediaMIME string
	// MediaPosterData holds the preview/poster image bytes when the media shape includes one.
	MediaPosterData []byte
	// MediaPosterMIME is the MIME type for MediaPosterData.
	MediaPosterMIME string
	// GroupTransform preserves the canonical local-to-parent coordinate system for
	// group shapes. Parsers populate it; layout owns materialization into slide space.
	GroupTransform *VisualTransform
	// Children holds nested shapes for Group shapes in their local coordinate space.
	Children []*Shape
	// LineColor is the stroke color for Connector shapes (sRGB hex).
	LineColor string
	// LineWidthPt is the stroke width in points for Connector shapes.
	LineWidthPt float64
	// LineStyle preserves shared DrawingML stroke semantics when available.
	LineStyle *drawing.LineStyle
	// ConnectorType is the DrawingML preset geometry for connector shapes.
	ConnectorType string
	// ConnectorFlipH and ConnectorFlipV mirror the connector geometry.
	ConnectorFlipH bool
	ConnectorFlipV bool
	// ConnectorAdjustments holds preset adjustment values for connector geometry.
	ConnectorAdjustments []int
	// ConnectorStart and ConnectorEnd are resolved anchor points in EMU when the
	// connector references shape connection sites.
	ConnectorStart *Point
	ConnectorEnd   *Point
	// ConnectorPath is an optional polyline (in EMU) used for elbow/curved
	// SmartArt connectors. When set, it overrides the straight line between
	// ConnectorStart and ConnectorEnd.
	ConnectorPath []Point
	// ArrowHead and ArrowTail describe endpoint decorations ("none","triangle","arrow","stealth").
	ArrowHead       string
	ArrowHeadWidth  string
	ArrowHeadLength string
	ArrowTail       string
	ArrowTailWidth  string
	ArrowTailLength string
	// AnimStep is the 1-based click-step at which this shape enters via animation.
	// 0 means the shape is always visible (no entrance animation in the source file).
	AnimStep int
	// AnimationOnly marks legacy paint-state overlays that exist only to materialize
	// an animation step. Static/document rendering suppresses them; playback may use them.
	AnimationOnly bool
	// EmphasisEffects holds emphasis animation effect names (e.g. "spin", "growShrink")
	// extracted from p:timing nodeType="emphEffect" nodes.
	EmphasisEffects []string
	// MotionPath holds a motion path animation for this shape, if any.
	MotionPath *AnimMotionPath
	// AnimationTriggers holds trigger event names (e.g. "onClick", "onHover")
	// for this shape's animations.
	AnimationTriggers []string
	// AnimationDelayMs preserves authored delays for automatic builds.
	AnimationDelayMs int
	// HyperlinkURL is the external URL to navigate to when the shape is clicked.
	// Non-empty for shapes with a p:hlinkClick hyperlink relationship.
	HyperlinkURL string
	// ActionLink is a human-readable navigation target for ppaction:// action links
	// (e.g. "firstSlide", "lastSlide", "nextSlide", "prevSlide", "endShow", or "slideN" for slide N).
	ActionLink string
	// Table holds the resolved table model for table shapes.
	Table *Table

	// OLEObjectData holds the raw embedded OLE object or package payload.
	OLEObjectData []byte
	// OLEMathML is semantic MathML recovered from a native MathType OLE stream.
	OLEMathML string
	// OLEMathColor is the authored foreground/recolor applied to the OLE preview.
	// Legacy PPT commonly stores MathType equation color as OfficeArt pictureRecolor.
	OLEMathColor string
	// OLEObjectMIME is the content type of the embedded OLE object payload.
	OLEObjectMIME string
	// OLEObjectName is the display name of the OLE object (p:oleObj/@name).
	OLEObjectName string
	// OLEObjectProgID is the OLE ProgID (e.g. Word.Document.12).
	OLEObjectProgID string
	// OLEObjectLinkURL is the external target for linked OLE objects.
	// It is empty for embedded objects.
	OLEObjectLinkURL string
	// OLEIconData holds the icon image bytes when showAsIcon is used.
	OLEIconData []byte
	// OLEIconMIME is the content type of the icon image.
	OLEIconMIME string

	// SmartArtNodeID is the data-model node identifier for shapes produced from
	// a SmartArt diagram. It is used to resolve connections between diagram nodes.
	SmartArtNodeID string
	// SmartArtPicturePlaceholder marks an unpopulated SmartArt picture node.
	// PowerPoint paints a picture-placeholder glyph inside this geometry.
	SmartArtPicturePlaceholder bool

	// SmartArtConnections lists the data-model node IDs that are children of this
	// SmartArt shape, derived from dgm:cxn connections.
	SmartArtConnections []string
}

// ShapeKind classifies the type of a shape.
type ShapeKind int

const (
	ShapeText              ShapeKind = iota // text box or placeholder (p:sp with txBody)
	ShapeImage                              // picture (p:pic)
	ShapeGroup                              // group shape (p:grpSp)
	ShapeConnector                          // connector line (p:cxnSp)
	ShapeSmartArtConnector                  // connector between SmartArt nodes
	ShapeSmartArt                           // SmartArt diagram (converted shapes)
	ShapeChart                              // embedded chart (converted to SVG)
	ShapeTable                              // graphicFrame table rendered as HTML table
	ShapeMedia                              // embedded audio/video rendered with HTML media controls
	ShapeOleObject                          // embedded OLE object rendered as placeholder or icon
	ShapeOther                              // other (not yet resolved)
)

// Table is a positioned presentation table.
type Table struct {
	ColumnWidthsEMU []int64
	Rows            []*TableRow
}

// TableRow is a resolved table row.
type TableRow struct {
	HeightEMU int64
	Cells     []*TableCell
}

// TableCell is a resolved table cell.
type TableCell struct {
	GridSpan        int
	RowSpan         int
	HMerge          bool
	VMerge          bool
	Fill            drawing.Fill
	FillColor       string
	Effects         drawing.EffectList
	TextAnchor      string
	MarginLeftEMU   int64
	MarginRightEMU  int64
	MarginTopEMU    int64
	MarginBottomEMU int64
	BorderLeft      *drawing.LineStyle
	BorderRight     *drawing.LineStyle
	BorderTop       *drawing.LineStyle
	BorderBottom    *drawing.LineStyle
	Paragraphs      []*Paragraph
}

// SmartArtShape represents a resolved SmartArt shape for rendering.
type SmartArtShape struct {
	Type      string
	Text      string
	X         float64 // Position in points
	Y         float64 // Position in points
	Width     float64 // Width in points
	Height    float64 // Height in points
	Fill      drawing.Fill
	Line      *drawing.LineStyle
	TextColor string
}

// PlaceholderKind identifies a shape's semantic placeholder role.
type PlaceholderKind int

const (
	PlaceholderNone PlaceholderKind = iota
	PlaceholderTitle
	PlaceholderBody
	PlaceholderSubtitle
	PlaceholderDateTime
	PlaceholderSlideNumber
	PlaceholderFooter
	PlaceholderHeader
	PlaceholderContent
	PlaceholderChart
	PlaceholderTable
	PlaceholderPicture
	PlaceholderMedia
	PlaceholderClipArt
	PlaceholderDiagram
	PlaceholderSlideImage
	PlaceholderOther
)

// Rect is a position and size in EMU.
type Rect struct {
	X, Y, W, H int64
}

// Point is a single position in EMU.
type Point struct {
	X, Y int64
}

// Paragraph is a DrawingML paragraph with resolved runs.
type Paragraph struct {
	// Level is the indent level (0–8).
	Level int
	// Align is "l", "ctr", "r", "just", or "" (inherited).
	Align string
	// TextDirection is "ltr", "rtl", or "" (inherited).
	TextDirection string
	// MarginLeftEMU and TextIndentEMU are resolved paragraph offsets in EMU.
	MarginLeftEMU int
	TextIndentEMU int
	HasMarginLeft bool
	HasTextIndent bool
	// MarginRightEMU is the resolved right margin in EMU.
	MarginRightEMU int
	HasMarginRight bool
	// DefaultTabSizeEMU is the resolved default tab size in EMU.
	DefaultTabSizeEMU int
	// TabStops are custom tab stops defined for this paragraph.
	TabStops []TabStop
	// FontAlign controls vertical alignment of text within a line.
	FontAlign string // "t", "ctr", "base", "b", ""
	// BulletChar is the explicit bullet character (e.g. "●"), or "" if none.
	BulletChar string
	// BulletSpecified records that the source explicitly set bullet enablement even when the bullet glyph itself is inherited.
	// This is needed by legacy PPT, where fHasBullet=true may omit bulletChar and therefore inherit the marker glyph.
	BulletSpecified bool `json:"bulletSpecified,omitempty"`
	// BulletNone records an explicit no-bullet setting so inherited bullets do not leak back in.
	BulletNone bool
	// AutoNum is true when the paragraph uses automatic numbering (1. 2. 3. etc.).
	AutoNum bool
	// AutoNumType is the numbering style (e.g. "arabicPeriod", "arabicParenR").
	AutoNumType string
	// AutoNumStart is the optional numbering start value from buAutoNum@startAt.
	AutoNumStart int
	// BulletColor is the resolved list marker color.
	BulletColor string
	// BulletColorExplicit records that the marker color was authored independently
	// of the paragraph text color. Legacy PPT needs this provenance to distinguish
	// a real bullet color from the default "follow text" behavior.
	BulletColorExplicit bool
	// BulletFontFamily is the resolved list marker font family.
	BulletFontFamily string
	// BulletFontTx means a:buFontTx: the marker must use the effective paragraph text font,
	// overriding any bullet font inherited from layout/master.
	BulletFontTx bool
	// BulletImageData/MIME carry an OOXML a:buBlip image marker after relationship resolution.
	BulletImageData []byte
	BulletImageMIME string
	// BulletImageRelID is parser-transient OOXML provenance until the relationship payload is materialized.
	BulletImageRelID string `json:"-"`
	// BulletSize is the resolved list marker size in hundredths of a point.
	BulletSize int
	// BulletSizePct is the resolved list marker size as a percentage in
	// thousandths of a percent (e.g. 100000 = 100%). When non-zero it takes
	// precedence over BulletSize and scales the first run's font size.
	BulletSizePct int
	// SpaceBefore/After are paragraph margins in hundredths of a point when explicitly resolved.
	SpaceBefore    int
	SpaceAfter     int
	HasSpaceBefore bool
	HasSpaceAfter  bool
	// SpaceBeforePct/AfterPct are paragraph margins in thousandths of a percent of the font size (100000 = 100%).
	// Used when spcPct is specified instead of spcPts. Non-zero means percentage-based spacing is set.
	SpaceBeforePct int
	SpaceAfterPct  int
	// LineSpacingPct is paragraph line spacing in thousandths of a percent.
	// LineSpacingPt is absolute line spacing in hundredths of a point (DrawingML spcPts).
	// Only one form is active at a time; preserving both forms prevents the cascade
	// from discarding an authored absolute line box.
	LineSpacingPct int
	LineSpacingPt  int
	HasLineSpacing bool
	Runs           []*Run
}

// Run is a resolved text run.
type Run struct {
	Text         string
	MathML       string // non-empty when this run carries a converted OMML equation
	FieldKind    string // non-empty for auto-generated fields: "slidenum", "datetime", etc.
	HyperlinkURL string // non-empty when this run should render as an external hyperlink
	Bold         bool
	Italic       bool
	Underline    bool
	Strike       bool
	// BoldSet / ItalicSet / UnderlineSet / StrikeSet are true when the corresponding
	// boolean was explicitly present in the source (e.g. STPA mask bit set).  A false
	// value in Bold/Italic/Underline/Strike with a false *Set flag means "not mentioned
	// in source" and should be filled by master-style inheritance.
	BoldSet      bool
	ItalicSet    bool
	UnderlineSet bool
	StrikeSet    bool
	AllCaps      bool
	SmallCaps    bool
	Superscript  bool
	Subscript    bool
	// BaselineShiftPct preserves the authored baseline displacement as a percentage
	// of the run font size. Positive values raise text; negative values lower it.
	BaselineShiftPct float64
	FontSize         int // hundredths of a point (0 = inherited)
	CharSpacing      int // hundredths of a point; DrawingML rPr@spc
	Kern             int // hundredths of a point; DrawingML rPr@kern threshold
	FontFamily       string
	Color            string // sRGB hex, e.g. "FF0000", or "" for inherited
	Highlight        string // sRGB hex highlight color, or "" for none
	// Effects holds DrawingML text effects such as shadow/glow/reflection.
	Effects drawing.EffectList
}

// TabStop is a custom tab stop within a paragraph.
type TabStop struct {
	Pos  int64  // position in EMU
	Algn string // "l", "ctr", "r", "dec"
}

// AnimationEvent is a scheduler-relevant PowerPoint timing event.
type AnimationEvent struct {
	ShapeID      int
	Step         int
	NodeType     string
	Behavior     string
	Effect       string
	Trigger      string
	DelayMs      int
	DurationMs   int
	RepeatCount  float64
	RepeatDurMs  int
	AutoReverse  bool
	Fill         string
	MotionPath   string
	MotionOrigin string
}

// AnimMotionPath stores a motion path animation for a shape.
type AnimMotionPath struct {
	Path   string // SVG-like path data
	Dur    int    // duration in milliseconds
	Origin string // "parent" or "layout"
}

// Theme holds resolved color and font scheme data for a presentation.
type Theme struct {
	Name        string
	ColorScheme map[string]string // slot name → sRGB hex (e.g. "dk1" → "000000")
	MajorFont   string
	MinorFont   string
	MajorEAFont string
	MinorEAFont string
	MajorCSFont string
	MinorCSFont string
}

// SlideMaster is a resolved slide master containing layouts and shared shapes.
type SlideMaster struct {
	// Name is the display name of the master.
	Name string
	// Shapes holds shared master shapes (e.g. title, footer placeholders).
	Shapes []*Shape
	// Layouts holds the layouts that belong to this master.
	Layouts []*SlideLayout
}

// SlideLayout is a resolved slide layout within a master.
type SlideLayout struct {
	// Type is the layout type (e.g. "blank", "title", "titleBody").
	Type string
	// Name is the display name of the layout.
	Name string
	// Master is the layout's parent master.
	Master *SlideMaster
	// Shapes holds layout-level placeholder shapes.
	Shapes []*Shape
}

// SmartArt represents an editable SmartArt diagram on a slide.
type SmartArt struct {
	// Type is the diagram category (hierarchy, process, cycle, etc.).
	Type string
	// LayoutID, QuickStyleID, and ColorID preserve requested SmartArt layout,
	// quick-style, and color-style identifiers for editable diagrams.
	LayoutID     string
	QuickStyleID string
	ColorID      string
	// X, Y, W, H define the hosting graphic frame in EMU.
	X, Y, W, H int64
	// FillColor, LineColor, and TextColor are optional diagram-wide formatting
	// overrides serialized into the generated data model/style approximation.
	FillColor string
	LineColor string
	TextColor string
	// Nodes are the data points in declaration order.
	Nodes []*SmartArtNode
	// Connections are parent-child edges between node IDs.
	Connections []SmartArtConnection
}

// SmartArtNode is a single data point inside a SmartArt diagram.
type SmartArtNode struct {
	// ID is a caller-supplied identifier. It is serialized as the point modelId.
	ID string
	// Type is the node type (root, node, assistant, etc.).
	Type string
	// Text is the displayed content.
	Text string
	// FillColor and TextColor are optional node-level formatting overrides.
	FillColor string
	TextColor string
	// Children lists the IDs of child nodes.
	Children []string
}

// SmartArtConnection is a logical edge between two SmartArt nodes.
type SmartArtConnection struct {
	SrcID  string
	DestID string
}

// EmbeddedFont is a resolved presentation font family with one or more usable faces.
type EmbeddedFont struct {
	Family string
	Faces  []*EmbeddedFontFace
}

// EmbeddedFontFace is a concrete font asset bound to a CSS weight/style.
type EmbeddedFontFace struct {
	Family      string
	Weight      int
	Style       string
	ContentType string
	// Name is the original OPC part name, preserved for round-trip write-back.
	Name string
	Data []byte
}
