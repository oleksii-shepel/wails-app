// Package sax provides a streaming text-extraction API for PPTX files,
// mirroring the Word SAX walker in office/pkg/word/parsers/ooxml/sax.
package sax

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/drawingxml"
	"office/pkg/ooxml/opc"
	"office/pkg/ooxml/rels"
	prel "office/pkg/presentation/parsers/ooxml/relationship"
)

const (
	relTypeOfficeDocument    = prel.OfficeDocument
	relTypeSlide             = prel.Slide
	relTypeNotesSlide        = prel.NotesSlide
	relTypeComments          = prel.Comments
	relTypeCommentAuthors    = prel.CommentAuthors
	strictRelNamespace       = "http://purl.oclc.org/ooxml/officeDocument/relationships"
	transitionalRelNamespace = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
)

func canonicalRelationshipType(value string) string { return prel.CanonicalType(value) }

func isRelationshipNamespace(value string) bool {
	return value == transitionalRelNamespace || value == strictRelNamespace
}

// EventType identifies a SAX event emitted while walking a PresentationML package.
type EventType string

const (
	EventStartPresentation      EventType = "startPresentation"
	EventEndPresentation        EventType = "endPresentation"
	EventStartSlide             EventType = "startSlide"
	EventEndSlide               EventType = "endSlide"
	EventStartNotes             EventType = "startNotes"
	EventEndNotes               EventType = "endNotes"
	EventStartComment           EventType = "startComment"
	EventEndComment             EventType = "endComment"
	EventStartShape             EventType = "startShape"
	EventEndShape               EventType = "endShape"
	EventStartGroup             EventType = "startGroup"
	EventEndGroup               EventType = "endGroup"
	EventStartParagraph         EventType = "startParagraph"
	EventParagraphProps         EventType = "paragraphProperties"
	EventStartRun               EventType = "startRun"
	EventRunProps               EventType = "runProperties"
	EventEndRun                 EventType = "endRun"
	EventEndParagraph           EventType = "endParagraph"
	EventText                   EventType = "text"
	EventBreak                  EventType = "break"
	EventTab                    EventType = "tab"
	EventField                  EventType = "field"
	EventTransform              EventType = "transform"
	EventPlaceholder            EventType = "placeholder"
	EventGeometry               EventType = "geometry"
	EventFill                   EventType = "fill"
	EventNoFill                 EventType = "noFill"
	EventGradientFill           EventType = "gradientFill"
	EventPatternFill            EventType = "patternFill"
	EventLine                   EventType = "line"
	EventLineStyleRef           EventType = "lineStyleRef"
	EventFillStyleRef           EventType = "fillStyleRef"
	EventThemeLineStyle         EventType = "themeLineStyle"
	EventThemeFillStyle         EventType = "themeFillStyle"
	EventThemeEffectStyle       EventType = "themeEffectStyle"
	EventEffectStyleRef         EventType = "effectStyleRef"
	EventEffect                 EventType = "effect"
	EventBlip                   EventType = "blip"
	EventRelationship           EventType = "relationship"
	EventRelationshipPayload    EventType = "relationshipPayload"
	EventThemeColor             EventType = "themeColor"
	EventColorMap               EventType = "colorMap"
	EventStartMaster            EventType = "startMaster"
	EventEndMaster              EventType = "endMaster"
	EventStartLayout            EventType = "startLayout"
	EventEndLayout              EventType = "endLayout"
	EventTextStyle              EventType = "textStyle"
	EventTextBodyProps          EventType = "textBodyProperties"
	EventThemeFont              EventType = "themeFont"
	EventGraphicObject          EventType = "graphicObject"
	EventStartTable             EventType = "startTable"
	EventEndTable               EventType = "endTable"
	EventStartTableRow          EventType = "startTableRow"
	EventEndTableRow            EventType = "endTableRow"
	EventStartTableCell         EventType = "startTableCell"
	EventEndTableCell           EventType = "endTableCell"
	EventTableColumn            EventType = "tableColumn"
	EventPresentationProperties EventType = "presentationProperties"
	EventSlideTransition        EventType = "slideTransition"
	EventSlideBackground        EventType = "slideBackground"
	EventBackgroundStyleRef     EventType = "backgroundStyleRef"
	EventBackgroundPicture      EventType = "backgroundPicture"
	EventSlideMasterVisibility  EventType = "slideMasterVisibility"
	EventShapeAction            EventType = "shapeAction"
	EventTableCellProps         EventType = "tableCellProperties"
	EventConnectorEndpoint      EventType = "connectorEndpoint"
	EventOLEPreview             EventType = "olePreview"
)

// VisualTransform preserves authored a:xfrm data without resolving inheritance.
type VisualTransform struct {
	X, Y, CX, CY                    int64
	ChildX, ChildY                  int64
	ChildCX, ChildCY                int64
	Rotation                        int
	FlipH, FlipV                    bool
	HasOffset, HasExtents           bool
	HasChildOffset, HasChildExtents bool
}

// VisualColor preserves an authored DrawingML color token without resolving theme inheritance.
type VisualColor struct {
	Kind     string // srgbClr, schemeClr, sysClr, prstClr
	Value    string
	Alpha    int // 0..100000; zero with HasAlpha means fully transparent
	HasAlpha bool

	// DrawingML color transforms belong to the color token, not to the shape
	// fill that happens to contain it. Keeping them here is essential for
	// theme fmtScheme styles where phClr/schemeClr is commonly tinted or
	// shaded before it becomes effective paint.
	Shade, Tint          int
	LumMod, LumOff       int
	SatMod, SatOff       int
	HasShade, HasTint    bool
	HasLumMod, HasLumOff bool
	HasSatMod, HasSatOff bool
}

type GradientStop struct {
	Position int
	Color    VisualColor
}

type GradientFill struct {
	Stops           []GradientStop
	Angle           int
	Path            string
	Flip            string
	RotateWithShape bool
	FillToLeft      int
	FillToTop       int
	FillToRight     int
	FillToBottom    int
	HasFillToRect   bool
}

type PatternFill struct {
	Preset     string
	Foreground VisualColor
	Background VisualColor
}

// ThemeFillStyle preserves an indexed DrawingML fmtScheme fill without
// flattening gradients or patterns to a representative color. Kind is one of
// solid, gradient, pattern, or none.
type ThemeFillStyle struct {
	Kind     string
	Color    VisualColor
	Gradient GradientFill
	Pattern  PatternFill
}

type LineProperties struct {
	WidthEMU                        int64
	Color                           VisualColor
	Cap, Join, Dash, Compound       string
	MiterLimit                      int
	HeadType, HeadWidth, HeadLength string
	TailType, TailWidth, TailLength string
}

type ThemeEffectStyle struct {
	Effects []EffectProperties
}

type EffectProperties struct {
	Kind                                            string
	Color                                           VisualColor
	BlurRad, Dist, Dir, Rad                         int64
	SX, SY, KX, KY                                  int
	Align                                           string
	RotateWithShape, Grow                           bool
	StartAlpha, StartPos, EndAlpha, EndPos, FadeDir int
}

// ParagraphProperties preserves authored DrawingML paragraph properties. Has*
// flags distinguish an explicit zero/false value from inheritance.
type ParagraphProperties struct {
	Level                                                           int
	HasLevel                                                        bool
	Align                                                           string
	MarginLeftEMU, MarginRightEMU, TextIndentEMU, DefaultTabSizeEMU int
	HasMarginLeft, HasMarginRight, HasTextIndent, HasDefaultTabSize bool
	FontAlign                                                       string
	TextDirection                                                   string
	TabStops                                                        []TabStop
	BulletChar, AutoNumType, BulletFontFamily                       string
	BulletColor                                                     VisualColor
	AutoNumStart, BulletSize, BulletSizePct                         int
	HasBullet, AutoNum, HasAutoNum, BulletFontTx                    bool
	SpaceBefore, SpaceAfter, SpaceBeforePct, SpaceAfterPct          int
	HasSpaceBefore, HasSpaceAfter                                   bool
	LineSpacingPct, LineSpacingPt                                   int
	HasLineSpacing                                                  bool
}

// TextBodyProperties preserves authored a:bodyPr facts.
type TextBodyProperties struct {
	Wrap, Anchor, Vert, AutofitType             string
	AnchorCtr, HasAnchorCtr                     bool
	Rot, Columns                                int
	HasRot, HasColumns                          bool
	ColumnSpacingEMU                            int64
	InsetT, InsetB, InsetL, InsetR              int64
	HasInsetT, HasInsetB, HasInsetL, HasInsetR  bool
	AutofitFontScale, AutofitLineSpaceReduction int
}

type TabStop struct {
	Pos   int64
	Align string
}

// RunProperties preserves authored DrawingML character properties before the
// master/layout cascade is applied by the SAX IR consumer.
type RunProperties struct {
	Bold, Italic, Underline, Strike             bool
	BoldSet, ItalicSet, UnderlineSet, StrikeSet bool
	FontSize                                    int
	FontFamily                                  string
	Color, Highlight                            VisualColor
	Baseline                                    int
	Caps                                        string
	Language, AltLanguage                       string
	CharacterSpacing, Kerning                   int
	HyperlinkRelID                              string
}

// Event is a single SAX event from a streamed PPTX package.
type Event struct {
	Type               EventType
	PartName           string
	SlideIndex         int
	ShapeID            int
	ShapeName          string
	ShapeKind          string
	Depth              int
	Placeholder        string
	Text               string
	FieldType          string
	Author             string
	Initials           string
	Date               string
	CommentX, CommentY int64

	// Visual facts are populated by the corresponding visual event types.
	Transform                                VisualTransform
	Geometry                                 string
	GeometryData                             *drawing.Geometry
	PlaceholderIdx                           int
	PlaceholderHasCustomPrompt               bool
	PlaceholderOrient                        string
	PlaceholderSize                          string
	Color                                    VisualColor
	Gradient                                 GradientFill
	Pattern                                  PatternFill
	LineWidthEMU                             int64
	Line                                     LineProperties
	ThemeFill                                ThemeFillStyle
	ThemeEffect                              ThemeEffectStyle
	Effect                                   EffectProperties
	RelID                                    string
	LinkRelID                                string
	CropLeft, CropTop, CropRight, CropBottom int

	// Contextual part ownership is resolved before slide XML is consumed. These
	// fields let downstream IR builders resolve authored relationship references
	// without materializing the typed OOXML project graph.
	LayoutPart                     string
	MasterPart                     string
	ThemePart                      string
	RelType                        string
	TargetPart                     string
	External                       bool
	Payload                        []byte
	ContentType                    string
	ThemeSlot                      string
	StyleIndex                     int
	ColorMap                       map[string]string
	ThemeFontKind, ThemeFontFamily string
	ThemeName                      string
	TextBody                       TextBodyProperties
	Paragraph                      ParagraphProperties
	Run                            RunProperties
	// Scope identifies the semantic source of shape events: slide, layout, or master.
	Scope string
	// TextStyleKind/TextStyleLevel identify master titleStyle/bodyStyle/otherStyle
	// level defaults emitted directly from the SAX stream.
	TextStyleKind  string
	TextStyleLevel int
	// GraphicObjectKind identifies complex PresentationML objects carried by a graphic frame or picture.
	GraphicObjectKind                                                    string
	OLEName, OLEProgID                                                   string
	TableValue                                                           int64
	CanvasWidthEMU, CanvasHeightEMU                                      int64
	TransitionType, TransitionSpeed                                      string
	TransitionAdvClick                                                   bool
	TransitionAdvTmMs                                                    int
	ShowMasterShapes, ShowMasterShapesSet                                bool
	Action, ActionRelID, Tooltip                                         string
	TableGridSpan, TableRowSpan                                          int
	TableHMerge, TableVMerge                                             bool
	TableAnchor                                                          string
	TableMarginLeft, TableMarginRight                                    int64
	TableMarginTop, TableMarginBottom                                    int64
	TableMarginLeftSet, TableMarginRightSet                              bool
	TableMarginTopSet, TableMarginBottomSet                              bool
	ConnectorEndpoint                                                    string
	ConnectorShapeID, ConnectorSiteIdx                                   int
	OLEShapeRef                                                          string
	PaintTarget                                                          string // shape, tableCell, borderLeft/right/top/bottom
	PictureTileTx, PictureTileTy                                         int64
	PictureTileSx, PictureTileSy                                         int
	PictureTileFlip, PictureTileAlign                                    string
	PictureFillLeft, PictureFillTop, PictureFillRight, PictureFillBottom int
}

// Handler consumes SAX events.
type Handler interface {
	HandleEvent(Event) error
}

// HandlerFunc adapts a function to the Handler interface.
type HandlerFunc func(Event) error

// HandleEvent implements Handler.
func (f HandlerFunc) HandleEvent(event Event) error {
	return f(event)
}

// Options control which parts and events are emitted.
type Options struct {
	IncludeNotes        bool
	IncludeComments     bool
	IncludeSlideNumbers bool
	IncludeHiddenSlides bool
	IncludeMasterText   bool
	IncludeChartText    bool
	IncludeSmartArtText bool
}

// WalkPath walks the text-bearing parts of a PPTX file on disk.
func WalkPath(path string, handler Handler, opts Options) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return err
	}
	return WalkReaderAt(file, info.Size(), handler, opts)
}

// WalkBytes walks the text-bearing parts of an in-memory PPTX byte slice.
func WalkBytes(data []byte, handler Handler, opts Options) error {
	return WalkReaderAt(bytes.NewReader(data), int64(len(data)), handler, opts)
}

// WalkStream walks the text-bearing parts of a non-seekable PPTX stream.
func WalkStream(r io.Reader, handler Handler, opts Options) error {
	file, err := os.CreateTemp("", "office-pptx-sax-")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(file.Name()) }()

	size, err := io.Copy(file, r)
	if err != nil {
		_ = file.Close()
		return err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		_ = file.Close()
		return err
	}
	return WalkReaderAt(file, size, handler, opts)
}

// WalkReaderAt walks the text-bearing parts of a PPTX package from a seekable reader.
// ZIP and materialized OPC inputs share the same semantic traversal. Only part access
// differs; presentation/relationship interpretation is intentionally single-path.
func WalkReaderAt(r io.ReaderAt, size int64, handler Handler, opts Options) error {
	access, err := openPackageAccess(r, size)
	if err != nil {
		return err
	}
	ctx := &walkContext{access: access, handler: handler, opts: opts}
	return ctx.walk()
}

type packageAccess interface {
	openPart(name string) (io.ReadCloser, error)
	relationships(sourcePart string) (map[string]relEntry, error)
}

type zipPackageAccess struct{ files map[string]*zip.File }

func openPackageAccess(r io.ReaderAt, size int64) (packageAccess, error) {
	if zr, err := zip.NewReader(r, size); err == nil {
		files := make(map[string]*zip.File, len(zr.File))
		for _, f := range zr.File {
			files[normalizePartName(f.Name)] = f
		}
		return &zipPackageAccess{files: files}, nil
	}
	pkg, err := opc.OpenReaderAt(r, size)
	if err != nil {
		return nil, err
	}
	return &opcPackageAccess{pkg: pkg}, nil
}

func (a *zipPackageAccess) openPart(name string) (io.ReadCloser, error) {
	f, ok := a.files[normalizePartName(name)]
	if !ok {
		return nil, fmt.Errorf("part %q not found", name)
	}
	return f.Open()
}

func (a *zipPackageAccess) relationships(sourcePart string) (map[string]relEntry, error) {
	if sourcePart == "" {
		return a.readRelsPart("_rels/.rels", "")
	}
	relDir, base := path.Dir(sourcePart), path.Base(sourcePart)
	relsName := relDir + "/_rels/" + base + ".rels"
	if relDir == "." {
		relsName = "_rels/" + base + ".rels"
	}
	return a.readRelsPart(relsName, sourcePart)
}

func (a *zipPackageAccess) readRelsPart(relsName, sourcePart string) (map[string]relEntry, error) {
	rc, err := a.openPart(relsName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	return decodeRelationships(rc, sourcePart)
}

type opcPackageAccess struct{ pkg *opc.Package }

func (a *opcPackageAccess) openPart(name string) (io.ReadCloser, error) {
	part, ok := a.pkg.Part(name)
	if !ok {
		return nil, fmt.Errorf("part %q not found", name)
	}
	return io.NopCloser(bytes.NewReader(part.Data)), nil
}

func (a *opcPackageAccess) relationships(sourcePart string) (map[string]relEntry, error) {
	rset, ok, err := a.pkg.Relationships(sourcePart)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("relationships for %q not found", sourcePart)
	}
	out := make(map[string]relEntry, len(rset.Relationships))
	for _, rel := range rset.Relationships {
		entry := relEntry{Type: prel.CanonicalType(rel.Type), External: rel.IsExternal()}
		if entry.External {
			entry.Target = rel.Target
		} else {
			entry.Target = rels.ResolveTarget(sourcePart, rel.Target)
		}
		out[rel.ID] = entry
	}
	return out, nil
}

type walkContext struct {
	access         packageAccess
	commentAuthors map[string]commentAuthor
	handler        Handler
	opts           Options
}

func (c *walkContext) walk() error {
	mainPart, err := c.findMainPart()
	if err != nil {
		return err
	}

	pres, err := c.readPresentation(mainPart)
	if err != nil {
		return err
	}
	if err := c.loadCommentAuthors(mainPart); err != nil {
		return err
	}

	slideTargets := c.resolveSlideTargets(mainPart)

	emit := func(ev Event) error {
		return c.handler.HandleEvent(ev)
	}

	if err := emit(Event{Type: EventStartPresentation}); err != nil {
		return err
	}
	if err := c.walkShapeTreePartScoped(mainPart, -1, "presentation", emit); err != nil {
		return err
	}
	// Discover presentation-level themes even when no slide/master traversal reaches them.
	// This closes the theme-only/degenerate package gap that previously forced ComposePackage.
	if rels, err := c.access.relationships(mainPart); err == nil {
		seenTheme := map[string]bool{}
		for _, rel := range rels {
			if rel.External || !strings.Contains(strings.ToLower(rel.Type), "/theme") || seenTheme[rel.Target] {
				continue
			}
			seenTheme[rel.Target] = true
			if err := c.walkThemeColors(rel.Target, -1, emit); err != nil {
				return err
			}
		}
	}
	for i, sld := range pres.Slides {
		if sld.Hidden && !c.opts.IncludeHiddenSlides {
			continue
		}
		slideName, ok := slideTargets[sld.RelID]
		if !ok {
			continue
		}
		context, err := c.resolveSlideContext(slideName)
		if err != nil {
			return err
		}
		if err := emit(Event{Type: EventStartSlide, PartName: slideName, SlideIndex: i, LayoutPart: context.LayoutPart, MasterPart: context.MasterPart, ThemePart: context.ThemePart}); err != nil {
			return err
		}
		if err := c.emitPartRelationships(slideName, i, "slide", context.Relationships, emit); err != nil {
			return err
		}
		if context.ThemePart != "" {
			if err := c.walkThemeColors(context.ThemePart, i, emit); err != nil {
				return err
			}
		}
		// A layout themeOverride is a sparse overlay on the master's base theme,
		// not a replacement theme. Stream it second so only authored slots replace
		// the base values for this slide context.
		if context.ThemeOverridePart != "" {
			if err := c.walkThemeColors(context.ThemeOverridePart, i, emit); err != nil {
				return err
			}
		}
		// Stream inherited visual sources before the authored slide. Consumers can
		// build immutable master/layout contexts without materializing OOXML parts.
		if context.MasterPart != "" {
			if rels, err := c.access.relationships(context.MasterPart); err == nil {
				if err := c.emitPartRelationships(context.MasterPart, i, "master", contextualRelationshipsFromEntries(rels), emit); err != nil {
					return err
				}
			}
			if err := emit(Event{Type: EventStartMaster, PartName: context.MasterPart, SlideIndex: i, Scope: "master"}); err != nil {
				return err
			}
			if err := c.walkColorMap(context.MasterPart, i, "master", emit); err != nil {
				return err
			}
			if err := c.walkBackgroundStyleRef(context.MasterPart, i, "master", emit); err != nil {
				return err
			}
			if err := c.walkBackgroundPicture(context.MasterPart, i, "master", emit); err != nil {
				return err
			}
			if err := c.walkShapeTreePartScoped(context.MasterPart, i, "master", emit); err != nil {
				return err
			}
			if err := emit(Event{Type: EventEndMaster, PartName: context.MasterPart, SlideIndex: i, Scope: "master"}); err != nil {
				return err
			}
		}
		if context.LayoutPart != "" {
			if rels, err := c.access.relationships(context.LayoutPart); err == nil {
				if err := c.emitPartRelationships(context.LayoutPart, i, "layout", contextualRelationshipsFromEntries(rels), emit); err != nil {
					return err
				}
			}
			if err := emit(Event{Type: EventStartLayout, PartName: context.LayoutPart, SlideIndex: i, Scope: "layout"}); err != nil {
				return err
			}
			if err := c.walkMasterShapeVisibilityPart(context.LayoutPart, i, "layout", emit); err != nil {
				return err
			}
			if err := c.walkColorMap(context.LayoutPart, i, "layout", emit); err != nil {
				return err
			}
			if err := c.walkBackgroundStyleRef(context.LayoutPart, i, "layout", emit); err != nil {
				return err
			}
			if err := c.walkBackgroundPicture(context.LayoutPart, i, "layout", emit); err != nil {
				return err
			}
			if err := c.walkShapeTreePartScoped(context.LayoutPart, i, "layout", emit); err != nil {
				return err
			}
			if err := emit(Event{Type: EventEndLayout, PartName: context.LayoutPart, SlideIndex: i, Scope: "layout"}); err != nil {
				return err
			}
		}
		if err := c.walkColorMap(slideName, i, "slide", emit); err != nil {
			return err
		}
		if err := c.walkBackgroundStyleRef(slideName, i, "slide", emit); err != nil {
			return err
		}
		if err := c.walkBackgroundPicture(slideName, i, "slide", emit); err != nil {
			return err
		}
		if err := c.walkSlideMetadataPart(slideName, i, emit); err != nil {
			return err
		}
		if err := c.walkOLEPreviewParts(i, context.Relationships, emit); err != nil {
			return err
		}
		if err := c.walkShapeTreePartScoped(slideName, i, "slide", emit); err != nil {
			return err
		}
		if err := emit(Event{Type: EventEndSlide, PartName: slideName, SlideIndex: i}); err != nil {
			return err
		}

		if c.opts.IncludeNotes || c.opts.IncludeComments {
			slideRels, err := c.access.relationships(slideName)
			if err == nil {
				if c.opts.IncludeNotes {
					for _, entry := range slideRels {
						if entry.External || canonicalRelationshipType(entry.Type) != relTypeNotesSlide {
							continue
						}
						if err := emit(Event{Type: EventStartNotes, SlideIndex: i}); err != nil {
							return err
						}
						if err := c.walkShapeTreePart(entry.Target, i, emit); err != nil {
							return err
						}
						if err := emit(Event{Type: EventEndNotes, SlideIndex: i}); err != nil {
							return err
						}
					}
				}
				if c.opts.IncludeComments {
					for _, entry := range slideRels {
						if entry.External || canonicalRelationshipType(entry.Type) != relTypeComments {
							continue
						}
						if err := c.walkCommentsPart(entry.Target, i, emit); err != nil {
							return err
						}
					}
				}
			}
		}
	}
	if err := emit(Event{Type: EventEndPresentation}); err != nil {
		return err
	}
	return nil
}

type relEntry struct {
	Type     string
	Target   string
	External bool
}

func decodeRelationships(r io.Reader, sourcePart string) (map[string]relEntry, error) {
	result := make(map[string]relEntry)
	dec := xml.NewDecoder(r)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "Relationship" {
			continue
		}
		var id, relType, target, targetMode string
		for _, attr := range start.Attr {
			switch attr.Name.Local {
			case "Id":
				id = attr.Value
			case "Type":
				relType = attr.Value
			case "Target":
				target = attr.Value
			case "TargetMode":
				targetMode = attr.Value
			}
		}
		if id == "" || relType == "" || target == "" {
			continue
		}
		entry := relEntry{Type: prel.CanonicalType(relType), External: strings.EqualFold(targetMode, "External")}
		if entry.External {
			entry.Target = target
		} else {
			entry.Target = rels.ResolveTarget(sourcePart, target)
		}
		result[id] = entry
	}
	return result, nil
}

type contextualRelationship struct {
	ID, Type, Target string
	External         bool
}

type slideContext struct {
	LayoutPart        string
	MasterPart        string
	ThemePart         string
	ThemeOverridePart string
	Relationships     []contextualRelationship
}

// resolveSlideContext follows only package relationships. It deliberately does
// not parse layout/master/theme XML: SAX consumers receive the immutable part
// chain first and can apply semantic inheritance while consuming slide events.
func (c *walkContext) resolveSlideContext(slidePart string) (slideContext, error) {
	var out slideContext
	slideRels, err := c.access.relationships(slidePart)
	if err != nil {
		// Relationship parts are optional for slides without external resources.
		return out, nil
	}
	ids := make([]string, 0, len(slideRels))
	for id := range slideRels {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		entry := slideRels[id]
		out.Relationships = append(out.Relationships, contextualRelationship{ID: id, Type: canonicalRelationshipType(entry.Type), Target: entry.Target, External: entry.External})
		if !entry.External && canonicalRelationshipType(entry.Type) == prel.SlideLayout {
			out.LayoutPart = entry.Target
		}
	}
	if out.LayoutPart == "" {
		return out, nil
	}
	layoutRels, err := c.access.relationships(out.LayoutPart)
	if err != nil {
		return out, nil
	}
	for _, entry := range layoutRels {
		if entry.External {
			continue
		}
		switch canonicalRelationshipType(entry.Type) {
		case prel.SlideMaster:
			out.MasterPart = entry.Target
		case prel.ThemeOverride:
			out.ThemeOverridePart = entry.Target
		}
	}
	if out.MasterPart == "" {
		return out, nil
	}
	masterRels, err := c.access.relationships(out.MasterPart)
	if err != nil {
		return out, nil
	}
	for _, entry := range masterRels {
		if !entry.External && canonicalRelationshipType(entry.Type) == prel.Theme {
			out.ThemePart = entry.Target
			break
		}
	}
	return out, nil
}

func (c *walkContext) findMainPart() (string, error) {
	rootRels, err := c.access.relationships("")
	if err != nil {
		return "", err
	}
	for _, entry := range rootRels {
		if !entry.External && canonicalRelationshipType(entry.Type) == relTypeOfficeDocument {
			return entry.Target, nil
		}
	}
	return "", fmt.Errorf("officeDocument relationship not found")
}

func (c *walkContext) resolveSlideTargets(mainPart string) map[string]string {
	presRels, err := c.access.relationships(mainPart)
	if err != nil {
		return nil
	}
	targets := make(map[string]string)
	for id, entry := range presRels {
		if !entry.External && canonicalRelationshipType(entry.Type) == relTypeSlide {
			targets[id] = entry.Target
		}
	}
	return targets
}

func (c *walkContext) readPresentation(mainPart string) (*presentation, error) {
	rc, err := c.access.openPart(mainPart)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	dec := xml.NewDecoder(rc)
	var pres presentation
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch start.Name.Local {
		case "sldId":
			var sld slideRef
			for _, attr := range start.Attr {
				if attr.Name.Local == "id" && attr.Name.Space == "" {
					sld.ID, _ = strconv.Atoi(attr.Value)
				} else if attr.Name.Local == "show" {
					sld.Hidden = attr.Value == "0"
				} else if isRelationshipNamespace(attr.Name.Space) && attr.Name.Local == "id" {
					sld.RelID = attr.Value
				}
			}
			pres.Slides = append(pres.Slides, sld)
		case "sldIdLst":
			// continue reading children
		}
	}
	return &pres, nil
}

// walkColorMap streams the effective scheme-slot aliases authored by a master,
// layout, or slide part. Master clrMap establishes the base aliases; clrMapOvr
// contributes only explicit overrides. The IR builder performs scope inheritance.
func (c *walkContext) walkColorMap(partName string, slideIdx int, scope string, emit func(Event) error) error {
	r, err := c.access.openPart(partName)
	if err != nil {
		return err
	}
	defer r.Close()
	dec := xml.NewDecoder(r)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		local := start.Name.Local
		if local != "clrMap" && local != "overrideClrMapping" {
			continue
		}
		m := make(map[string]string)
		for _, a := range start.Attr {
			if a.Name.Space == "" && a.Value != "" {
				m[a.Name.Local] = a.Value
			}
		}
		if len(m) > 0 {
			return emit(Event{Type: EventColorMap, PartName: partName, SlideIndex: slideIdx, Scope: scope, ColorMap: m})
		}
		return nil
	}
}

func (c *walkContext) walkThemeColors(partName string, slideIdx int, emit func(Event) error) error {
	rc, err := c.access.openPart(partName)
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()
	dec := xml.NewDecoder(rc)
	var slot, fontGroup, themeStyleList string
	inScheme := false
	inExtraClrScheme := false
	themeStyleIndex := 0
	var themeLine *LineProperties
	var themeFill *ThemeFillStyle
	var themeEffect *ThemeEffectStyle
	var themeEffectItem *EffectProperties
	var themeGradientStop *GradientStop
	var themePatternSlot string
	var inObjectDefaults, inObjectDefaultShapeDef, inObjectDefaultListStyle bool
	var objectDefaultParagraph *ParagraphProperties
	var objectDefaultRun *RunProperties
	var objectDefaultRunColorDepth int
	var objectDefaultSpacingTarget string
	var objectDefaultBulletColor *VisualColor
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		switch v := tok.(type) {
		case xml.StartElement:
			local := v.Name.Local
			if local == "extraClrScheme" {
				inExtraClrScheme = true
			}
			switch local {
			case "objectDefaults":
				inObjectDefaults = true
			case "spDef":
				if inObjectDefaults {
					inObjectDefaultShapeDef = true
				}
			case "lstStyle":
				if inObjectDefaultShapeDef {
					inObjectDefaultListStyle = true
				}
			}
			if inObjectDefaultListStyle {
				switch local {
				case "defPPr":
					pp := paragraphPropsFromAttrs(v.Attr, xmlAttr)
					pp.Level, pp.HasLevel = 0, true
					objectDefaultParagraph = &pp
				case "spcBef", "spcAft", "lnSpc":
					if objectDefaultParagraph != nil {
						objectDefaultSpacingTarget = local
					}
				case "spcPts", "spcPct":
					if objectDefaultParagraph != nil {
						applySpacing(objectDefaultParagraph, objectDefaultSpacingTarget, xmlIntAttr(v.Attr, "val"), local == "spcPct")
					}
				case "buNone":
					if objectDefaultParagraph != nil {
						objectDefaultParagraph.HasBullet, objectDefaultParagraph.BulletChar, objectDefaultParagraph.HasAutoNum, objectDefaultParagraph.AutoNum = true, "", true, false
					}
				case "buFontTx":
					if objectDefaultParagraph != nil {
						objectDefaultParagraph.BulletFontTx = true
						objectDefaultParagraph.BulletFontFamily = ""
					}
				case "buClr":
					if objectDefaultParagraph != nil {
						objectDefaultBulletColor = &objectDefaultParagraph.BulletColor
					}
				case "defRPr":
					if objectDefaultParagraph != nil {
						rp := runPropsFromAttrs(v.Attr, xmlAttr)
						objectDefaultRun = &rp
					}
				case "solidFill":
					if objectDefaultRun != nil {
						objectDefaultRunColorDepth++
					}
				case "latin":
					if objectDefaultRun != nil {
						objectDefaultRun.FontFamily = xmlAttr(v.Attr, "typeface")
					}
				case "srgbClr", "schemeClr", "sysClr", "prstClr":
					if objectDefaultRun != nil && objectDefaultRunColorDepth > 0 {
						c := VisualColor{Kind: local, Value: xmlAttr(v.Attr, "val")}
						if local == "sysClr" && c.Value == "" {
							c.Value = xmlAttr(v.Attr, "lastClr")
						}
						objectDefaultRun.Color = c
					} else if objectDefaultBulletColor != nil {
						c := VisualColor{Kind: local, Value: xmlAttr(v.Attr, "val")}
						if local == "sysClr" && c.Value == "" {
							c.Value = xmlAttr(v.Attr, "lastClr")
						}
						*objectDefaultBulletColor = c
					}
				}
			}
			if local == "lnStyleLst" || local == "fillStyleLst" || local == "bgFillStyleLst" || local == "effectStyleLst" {
				themeStyleList, themeStyleIndex = local, 0
			}
			if themeStyleList == "lnStyleLst" && local == "ln" {
				themeStyleIndex++
				themeLine = &LineProperties{WidthEMU: xmlInt64Attr(v.Attr, "w"), Cap: xmlAttr(v.Attr, "cap"), Compound: xmlAttr(v.Attr, "cmpd")}
			}
			if themeLine != nil {
				switch local {
				case "prstDash":
					themeLine.Dash = xmlAttr(v.Attr, "val")
				case "headEnd":
					themeLine.HeadType, themeLine.HeadWidth, themeLine.HeadLength = xmlAttr(v.Attr, "type"), xmlAttr(v.Attr, "w"), xmlAttr(v.Attr, "len")
				case "tailEnd":
					themeLine.TailType, themeLine.TailWidth, themeLine.TailLength = xmlAttr(v.Attr, "type"), xmlAttr(v.Attr, "w"), xmlAttr(v.Attr, "len")
				case "round":
					themeLine.Join = "round"
				case "bevel":
					themeLine.Join = "bevel"
				case "miter":
					themeLine.Join = "miter"
					themeLine.MiterLimit, _ = strconv.Atoi(xmlAttr(v.Attr, "lim"))
				}
			}
			if themeStyleList == "effectStyleLst" && local == "effectStyle" {
				themeStyleIndex++
				themeEffect = &ThemeEffectStyle{}
			}
			if themeEffect != nil {
				switch local {
				case "outerShdw", "innerShdw", "glow", "softEdge", "reflection", "blur":
					fx := EffectProperties{Kind: local}
					fx.BlurRad, fx.Dist, fx.Dir, fx.Rad = xmlInt64Attr(v.Attr, "blurRad"), xmlInt64Attr(v.Attr, "dist"), xmlInt64Attr(v.Attr, "dir"), xmlInt64Attr(v.Attr, "rad")
					fx.SX, fx.SY, fx.KX, fx.KY = xmlIntAttr(v.Attr, "sx"), xmlIntAttr(v.Attr, "sy"), xmlIntAttr(v.Attr, "kx"), xmlIntAttr(v.Attr, "ky")
					fx.Align, fx.RotateWithShape, fx.Grow = xmlAttr(v.Attr, "algn"), xmlBoolAttr(v.Attr, "rotWithShape"), xmlBoolAttr(v.Attr, "grow")
					fx.StartAlpha, fx.StartPos, fx.EndAlpha, fx.EndPos, fx.FadeDir = xmlIntAttr(v.Attr, "stA"), xmlIntAttr(v.Attr, "stPos"), xmlIntAttr(v.Attr, "endA"), xmlIntAttr(v.Attr, "endPos"), xmlIntAttr(v.Attr, "fadeDir")
					themeEffect.Effects = append(themeEffect.Effects, fx)
					themeEffectItem = &themeEffect.Effects[len(themeEffect.Effects)-1]
				}
			}
			if (themeStyleList == "fillStyleLst" || themeStyleList == "bgFillStyleLst") && (local == "solidFill" || local == "gradFill" || local == "pattFill" || local == "blipFill" || local == "noFill") {
				themeStyleIndex++
				idx := themeStyleIndex
				if themeStyleList == "bgFillStyleLst" {
					idx += 1000
				}
				themeFill = &ThemeFillStyle{}
				switch local {
				case "solidFill":
					themeFill.Kind = "solid"
				case "gradFill":
					themeFill.Kind = "gradient"
					themeFill.Gradient.Flip = xmlAttr(v.Attr, "flip")
					themeFill.Gradient.RotateWithShape = xmlBoolAttr(v.Attr, "rotWithShape")
				case "pattFill":
					themeFill.Kind = "pattern"
					themeFill.Pattern.Preset = xmlAttr(v.Attr, "prst")
				case "noFill":
					themeFill.Kind = "none"
				case "blipFill":
					themeFill.Kind = "blip"
				}
				_ = idx
			}
			if themeFill != nil {
				switch local {
				case "gs":
					if themeFill.Kind == "gradient" {
						st := GradientStop{Position: xmlIntAttr(v.Attr, "pos")}
						themeFill.Gradient.Stops = append(themeFill.Gradient.Stops, st)
						themeGradientStop = &themeFill.Gradient.Stops[len(themeFill.Gradient.Stops)-1]
					}
				case "lin":
					if themeFill.Kind == "gradient" {
						themeFill.Gradient.Angle = xmlIntAttr(v.Attr, "ang")
					}
				case "path":
					if themeFill.Kind == "gradient" {
						themeFill.Gradient.Path = xmlAttr(v.Attr, "path")
					}
				case "fillToRect":
					if themeFill.Kind == "gradient" {
						themeFill.Gradient.FillToLeft = xmlIntAttr(v.Attr, "l")
						themeFill.Gradient.FillToTop = xmlIntAttr(v.Attr, "t")
						themeFill.Gradient.FillToRight = xmlIntAttr(v.Attr, "r")
						themeFill.Gradient.FillToBottom = xmlIntAttr(v.Attr, "b")
						themeFill.Gradient.HasFillToRect = true
					}
				case "fgClr", "bgClr":
					if themeFill.Kind == "pattern" {
						themePatternSlot = local
					}
				}
			}
			if (themeLine != nil || themeFill != nil || themeEffectItem != nil) && (local == "srgbClr" || local == "schemeClr" || local == "sysClr" || local == "prstClr") {
				col := VisualColor{Kind: local, Value: xmlAttr(v.Attr, "val")}
				if local == "sysClr" && xmlAttr(v.Attr, "lastClr") != "" {
					col.Value = xmlAttr(v.Attr, "lastClr")
				}
				if themeLine != nil {
					themeLine.Color = col
				}
				if themeEffectItem != nil {
					themeEffectItem.Color = col
				}
				if themeFill != nil {
					switch themeFill.Kind {
					case "solid":
						themeFill.Color = col
					case "gradient":
						if themeGradientStop != nil {
							themeGradientStop.Color = col
						}
					case "pattern":
						if themePatternSlot == "fgClr" {
							themeFill.Pattern.Foreground = col
						} else if themePatternSlot == "bgClr" {
							themeFill.Pattern.Background = col
						}
					}
				}
			}
			if local == "theme" {
				if name := xmlAttr(v.Attr, "name"); name != "" {
					if err := emit(Event{Type: EventThemeColor, PartName: partName, SlideIndex: slideIdx, ThemeName: name}); err != nil {
						return err
					}
				}
			}
			if local == "majorFont" || local == "minorFont" {
				fontGroup = local
			}
			if fontGroup != "" && (local == "latin" || local == "ea" || local == "cs") {
				if family := xmlAttr(v.Attr, "typeface"); family != "" {
					if err := emit(Event{Type: EventThemeFont, PartName: partName, SlideIndex: slideIdx, ThemeFontKind: fontGroup + ":" + local, ThemeFontFamily: family}); err != nil {
						return err
					}
				}
			}
			if local == "clrScheme" && !inExtraClrScheme {
				inScheme = true
				continue
			}
			if !inScheme {
				continue
			}
			switch local {
			case "dk1", "lt1", "dk2", "lt2", "accent1", "accent2", "accent3", "accent4", "accent5", "accent6", "hlink", "folHlink":
				slot = local
			case "srgbClr", "sysClr":
				if slot == "" {
					continue
				}
				value := xmlAttr(v.Attr, "val")
				if local == "sysClr" && xmlAttr(v.Attr, "lastClr") != "" {
					value = xmlAttr(v.Attr, "lastClr")
				}
				if value != "" {
					if err := emit(Event{Type: EventThemeColor, PartName: partName, SlideIndex: slideIdx, ThemeSlot: slot, Color: VisualColor{Kind: "srgbClr", Value: value}}); err != nil {
						return err
					}
				}
			}
		case xml.EndElement:
			local := v.Name.Local
			if local == "extraClrScheme" {
				inExtraClrScheme = false
			}
			switch local {
			case "solidFill":
				if objectDefaultRun != nil && objectDefaultRunColorDepth > 0 {
					objectDefaultRunColorDepth--
				}
			case "defPPr":
				if objectDefaultParagraph != nil {
					ev := Event{Type: EventTextStyle, PartName: partName, SlideIndex: slideIdx, Scope: "presentation", TextStyleKind: "default", TextStyleLevel: 0, Paragraph: *objectDefaultParagraph}
					if objectDefaultRun != nil {
						ev.Run = *objectDefaultRun
					}
					if err := emit(ev); err != nil {
						return err
					}
				}
				objectDefaultParagraph, objectDefaultRun = nil, nil
				objectDefaultSpacingTarget = ""
				objectDefaultRunColorDepth = 0
				objectDefaultBulletColor = nil
			case "buClr":
				objectDefaultBulletColor = nil
			case "lstStyle":
				if inObjectDefaultShapeDef {
					inObjectDefaultListStyle = false
				}
			case "spDef":
				if inObjectDefaults {
					inObjectDefaultShapeDef = false
				}
			case "objectDefaults":
				inObjectDefaults = false
			}
			if local == "ln" && themeLine != nil {
				if err := emit(Event{Type: EventThemeLineStyle, PartName: partName, SlideIndex: slideIdx, StyleIndex: themeStyleIndex, Line: *themeLine}); err != nil {
					return err
				}
				themeLine = nil
			}
			if themeEffectItem != nil && local == themeEffectItem.Kind {
				themeEffectItem = nil
			}
			if local == "effectStyle" && themeEffect != nil {
				if err := emit(Event{Type: EventThemeEffectStyle, PartName: partName, SlideIndex: slideIdx, StyleIndex: themeStyleIndex, ThemeEffect: *themeEffect}); err != nil {
					return err
				}
				themeEffect = nil
				themeEffectItem = nil
			}
			if themeFill != nil && (local == "solidFill" || local == "gradFill" || local == "pattFill" || local == "blipFill" || local == "noFill") {
				idx := themeStyleIndex
				if themeStyleList == "bgFillStyleLst" {
					idx += 1000
				}
				if err := emit(Event{Type: EventThemeFillStyle, PartName: partName, SlideIndex: slideIdx, StyleIndex: idx, ThemeFill: *themeFill}); err != nil {
					return err
				}
				themeFill = nil
				themeGradientStop = nil
				themePatternSlot = ""
			}
			if local == "gs" {
				themeGradientStop = nil
			}
			if local == "fgClr" || local == "bgClr" {
				themePatternSlot = ""
			}
			if local == "lnStyleLst" || local == "fillStyleLst" || local == "bgFillStyleLst" || local == "effectStyleLst" {
				themeStyleList = ""
				themeStyleIndex = 0
			}
			if local == "majorFont" || local == "minorFont" {
				fontGroup = ""
			}
			if local == "clrScheme" {
				inScheme = false
				slot = ""
				continue
			}
		}
	}
}

func (c *walkContext) walkShapeTreePart(partName string, slideIdx int, emit func(Event) error) error {
	return c.walkShapeTreePartScoped(partName, slideIdx, "", emit)
}

func contextualRelationshipsFromEntries(entries map[string]relEntry) []contextualRelationship {
	out := make([]contextualRelationship, 0, len(entries))
	for id, rel := range entries {
		out = append(out, contextualRelationship{ID: id, Type: rel.Type, Target: rel.Target, External: rel.External})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (c *walkContext) emitPartRelationships(partName string, slideIdx int, scope string, rels []contextualRelationship, emit func(Event) error) error {
	for _, rel := range rels {
		if err := emit(Event{Type: EventRelationship, PartName: partName, SlideIndex: slideIdx, Scope: scope, RelID: rel.ID, RelType: rel.Type, TargetPart: rel.Target, External: rel.External}); err != nil {
			return err
		}
		if rel.External || !saxOwnsRelationshipPayload(rel.Type) {
			continue
		}
		payload, err := c.readPartBytes(rel.Target)
		if err != nil {
			// Broken or intentionally skeletal relationship fixtures must not prevent
			// structural SAX parsing; absence remains explicit as no payload event.
			continue
		}
		if err := emit(Event{Type: EventRelationshipPayload, PartName: partName, SlideIndex: slideIdx, Scope: scope, RelID: rel.ID, RelType: rel.Type, TargetPart: rel.Target, Payload: payload, ContentType: contentTypeForPart(rel.Target)}); err != nil {
			return err
		}
	}
	return nil
}

func (c *walkContext) walkOLEPreviewParts(slideIdx int, rels []contextualRelationship, emit func(Event) error) error {
	for _, rel := range rels {
		if rel.External || !strings.HasSuffix(canonicalRelationshipType(rel.Type), "/vmlDrawing") {
			continue
		}
		if err := c.walkOLEPreviewPart(rel.Target, slideIdx, emit); err != nil {
			return err
		}
	}
	return nil
}

func (c *walkContext) walkOLEPreviewPart(partName string, slideIdx int, emit func(Event) error) error {
	partRels, err := c.access.relationships(partName)
	if err != nil {
		return nil
	}
	rc, err := c.access.openPart(partName)
	if err != nil {
		return nil
	}
	defer func() { _ = rc.Close() }()
	dec := xml.NewDecoder(rc)
	var shapeID string
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		switch se := tok.(type) {
		case xml.StartElement:
			switch se.Name.Local {
			case "shape":
				shapeID = xmlAttr(se.Attr, "id")
			case "imagedata":
				if shapeID == "" {
					continue
				}
				relID := xmlAttr(se.Attr, "relid")
				if relID == "" {
					relID = xmlAttr(se.Attr, "id")
				}
				rel := partRels[relID]
				if rel.External || canonicalRelationshipType(rel.Type) != prel.Image {
					continue
				}
				payload, err := c.readPartBytes(rel.Target)
				if err != nil {
					continue
				}
				if err := emit(Event{Type: EventOLEPreview, PartName: partName, SlideIndex: slideIdx, Scope: "slide", OLEShapeRef: shapeID, RelID: relID, RelType: rel.Type, TargetPart: rel.Target, Payload: payload, ContentType: contentTypeForPart(rel.Target)}); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if se.Name.Local == "shape" {
				shapeID = ""
			}
		}
	}
}

func (c *walkContext) walkShapeTreePartScoped(partName string, slideIdx int, scope string, emit func(Event) error) error {
	rc, err := c.access.openPart(partName)
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()
	return walkShapeTreeReader(rc, partName, slideIdx, func(ev Event) error {
		ev.Scope = scope
		return emit(ev)
	})
}

// walkMasterShapeVisibilityPart reads the part-local showMasterSp contract
// without conflating layout metadata with slide metadata such as backgrounds or
// transitions. Slide layouts can independently suppress their master's artwork.
func (c *walkContext) walkMasterShapeVisibilityPart(partName string, slideIdx int, scope string, emit func(Event) error) error {
	rc, err := c.access.openPart(partName)
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()
	dec := xml.NewDecoder(rc)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		for _, a := range se.Attr {
			if a.Name.Local != "showMasterSp" {
				continue
			}
			show := a.Value != "0" && a.Value != "false"
			return emit(Event{Type: EventSlideMasterVisibility, PartName: partName, SlideIndex: slideIdx, Scope: scope, ShowMasterShapes: show, ShowMasterShapesSet: true})
		}
		// showMasterSp is an attribute of the part root; once the root start
		// element is consumed, no descendant can change this visibility scope.
		return nil
	}
}

func (c *walkContext) walkBackgroundStyleRef(partName string, slideIdx int, scope string, emit func(Event) error) error {
	rc, err := c.access.openPart(partName)
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()
	dec := xml.NewDecoder(rc)
	inBG := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		switch se := tok.(type) {
		case xml.StartElement:
			switch se.Name.Local {
			case "bg":
				inBG = true
			case "bgRef":
				if !inBG {
					continue
				}
				styleIndex, _ := strconv.Atoi(attrValue(se.Attr, "idx"))
				color := VisualColor{}
				for {
					child, e := dec.Token()
					if e != nil {
						return e
					}
					switch v := child.(type) {
					case xml.StartElement:
						if v.Name.Local == "srgbClr" || v.Name.Local == "schemeClr" || v.Name.Local == "sysClr" || v.Name.Local == "prstClr" {
							value := attrValue(v.Attr, "val")
							if v.Name.Local == "sysClr" && value == "" {
								value = attrValue(v.Attr, "lastClr")
							}
							color = VisualColor{Kind: v.Name.Local, Value: value}
						}
					case xml.EndElement:
						if v.Name.Local == "bgRef" {
							return emit(Event{Type: EventBackgroundStyleRef, PartName: partName, SlideIndex: slideIdx, Scope: scope, StyleIndex: styleIndex, Color: color})
						}
					}
				}
			}
		case xml.EndElement:
			if se.Name.Local == "bg" {
				inBG = false
			}
		}
	}
}

// walkBackgroundPicture streams a direct p:bg/p:bgPr/a:blipFill as a
// part-owned background event. Background relationship IDs are local to the
// slide/layout/master part, so they must never be resolved through a shape or
// through another part's relationship table.
func (c *walkContext) walkBackgroundPicture(partName string, slideIdx int, scope string, emit func(Event) error) error {
	rc, err := c.access.openPart(partName)
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()
	dec := xml.NewDecoder(rc)
	intAttrValue := func(attrs []xml.Attr, local string) int {
		v, _ := strconv.Atoi(attrValue(attrs, local))
		return v
	}
	inBG, inBGPr, inBlipFill := false, false, false
	var ev Event
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		switch se := tok.(type) {
		case xml.StartElement:
			switch se.Name.Local {
			case "bg":
				inBG = true
			case "bgPr":
				if inBG {
					inBGPr = true
				}
			case "blipFill":
				if inBGPr {
					inBlipFill = true
					ev = Event{Type: EventBackgroundPicture, PartName: partName, SlideIndex: slideIdx, Scope: scope}
				}
			case "blip":
				if inBlipFill {
					ev.RelID = attrValue(se.Attr, "embed")
					if ev.RelID == "" {
						ev.RelID = attrValue(se.Attr, "link")
					}
				}
			case "srcRect":
				if inBlipFill {
					ev.CropLeft = intAttrValue(se.Attr, "l")
					ev.CropTop = intAttrValue(se.Attr, "t")
					ev.CropRight = intAttrValue(se.Attr, "r")
					ev.CropBottom = intAttrValue(se.Attr, "b")
				}
			case "fillRect":
				if inBlipFill {
					ev.PictureFillLeft = intAttrValue(se.Attr, "l")
					ev.PictureFillTop = intAttrValue(se.Attr, "t")
					ev.PictureFillRight = intAttrValue(se.Attr, "r")
					ev.PictureFillBottom = intAttrValue(se.Attr, "b")
				}
			case "tile":
				if inBlipFill {
					ev.PictureTileTx, _ = strconv.ParseInt(attrValue(se.Attr, "tx"), 10, 64)
					ev.PictureTileTy, _ = strconv.ParseInt(attrValue(se.Attr, "ty"), 10, 64)
					ev.PictureTileSx = intAttrValue(se.Attr, "sx")
					ev.PictureTileSy = intAttrValue(se.Attr, "sy")
					ev.PictureTileFlip = attrValue(se.Attr, "flip")
					ev.PictureTileAlign = attrValue(se.Attr, "algn")
				}
			}
		case xml.EndElement:
			switch se.Name.Local {
			case "blipFill":
				if inBlipFill {
					inBlipFill = false
					if ev.RelID != "" {
						return emit(ev)
					}
				}
			case "bgPr":
				inBGPr = false
			case "bg":
				inBG = false
			}
		}
	}
}

func (c *walkContext) walkSlideMetadataPart(partName string, slideIdx int, emit func(Event) error) error {
	rc, err := c.access.openPart(partName)
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()
	dec := xml.NewDecoder(rc)
	inBG, inBGSolid := false, false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		switch se := tok.(type) {
		case xml.StartElement:
			switch se.Name.Local {
			case "sld":
				for _, a := range se.Attr {
					if a.Name.Local != "showMasterSp" {
						continue
					}
					show := a.Value != "0" && a.Value != "false"
					if err := emit(Event{Type: EventSlideMasterVisibility, PartName: partName, SlideIndex: slideIdx, ShowMasterShapes: show, ShowMasterShapesSet: true}); err != nil {
						return err
					}
					break
				}
			case "bg", "bgPr":
				inBG = true
			case "solidFill":
				if inBG {
					inBGSolid = true
				}
			case "srgbClr", "schemeClr", "sysClr", "prstClr":
				if inBGSolid {
					v := attrValue(se.Attr, "val")
					if se.Name.Local == "sysClr" && v == "" {
						v = attrValue(se.Attr, "lastClr")
					}
					if err := emit(Event{Type: EventSlideBackground, PartName: partName, SlideIndex: slideIdx, Color: VisualColor{Kind: se.Name.Local, Value: v}}); err != nil {
						return err
					}
				}
			case "transition":
				ev := Event{Type: EventSlideTransition, PartName: partName, SlideIndex: slideIdx, TransitionAdvClick: true}
				for _, a := range se.Attr {
					switch a.Name.Local {
					case "spd":
						ev.TransitionSpeed = a.Value
					case "advClick":
						ev.TransitionAdvClick = a.Value != "0" && a.Value != "false"
					case "advTm":
						ev.TransitionAdvTmMs, _ = strconv.Atoi(a.Value)
					}
				}
				for {
					child, e := dec.Token()
					if e != nil {
						return e
					}
					switch v := child.(type) {
					case xml.StartElement:
						ev.TransitionType = v.Name.Local
						if e := dec.Skip(); e != nil {
							return e
						}
					case xml.EndElement:
						if v.Name.Local == "transition" {
							if e := emit(ev); e != nil {
								return e
							}
							goto transitionDone
						}
					}
				}
			transitionDone:
			}
		case xml.EndElement:
			switch se.Name.Local {
			case "solidFill":
				inBGSolid = false
			case "bg", "bgPr":
				inBG = false
			}
		}
	}
}

func attrValue(attrs []xml.Attr, local string) string {
	for _, a := range attrs {
		if a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}

func (c *walkContext) readPartBytes(partName string) ([]byte, error) {
	rc, err := c.access.openPart(partName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	return io.ReadAll(rc)
}

func saxOwnsRelationshipPayload(relType string) bool {
	relType = canonicalRelationshipType(relType)
	if relType == prel.Image || relType == prel.Audio || relType == prel.Video || relType == prel.OLEObject || relType == prel.Package {
		return true
	}
	return strings.HasSuffix(relType, "/chart") || strings.HasSuffix(relType, "/diagramData") || strings.HasSuffix(relType, "/media")
}

func contentTypeForPart(partName string) string {
	switch strings.ToLower(path.Ext(partName)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".emf":
		return "image/emf"
	case ".wmf":
		return "image/wmf"
	case ".mp4", ".m4v":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".avi":
		return "video/x-msvideo"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".m4a":
		return "audio/mp4"
	case ".xml":
		return "application/xml"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case ".bin":
		return "application/octet-stream"
	default:
		return "application/octet-stream"
	}
}

func (c *walkContext) walkCommentsPart(partName string, slideIdx int, emit func(Event) error) error {
	rc, err := c.access.openPart(partName)
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()
	return walkCommentsReader(rc, partName, slideIdx, func(ev Event) error {
		if ev.Type == EventStartComment {
			if author, ok := c.commentAuthors[ev.Author]; ok {
				ev.Author, ev.Initials = author.Name, author.Initials
			}
		}
		return emit(ev)
	})
}

type commentAuthor struct{ Name, Initials string }

func (c *walkContext) loadCommentAuthors(mainPart string) error {
	c.commentAuthors = make(map[string]commentAuthor)
	rels, err := c.access.relationships(mainPart)
	if err != nil {
		return nil
	}
	part := ""
	for _, rel := range rels {
		if !rel.External && canonicalRelationshipType(rel.Type) == relTypeCommentAuthors {
			part = rel.Target
			break
		}
	}
	if part == "" {
		return nil
	}
	rc, err := c.access.openPart(part)
	if err != nil {
		return err
	}
	defer rc.Close()
	dec := xml.NewDecoder(rc)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "cmAuthor" {
			continue
		}
		id, name, initials := xmlAttr(se.Attr, "id"), xmlAttr(se.Attr, "name"), xmlAttr(se.Attr, "initials")
		c.commentAuthors[id] = commentAuthor{Name: name, Initials: initials}
	}
}

type presentation struct {
	Slides []slideRef
}

type slideRef struct {
	ID     int
	RelID  string
	Hidden bool
}

func normalizePartName(name string) string {
	name = strings.TrimPrefix(name, "/")
	name = strings.TrimPrefix(name, "./")
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "\\", "/")
	return strings.ToLower(name)
}

func walkShapeTree(data []byte, partName string, slideIdx int, emit func(Event) error) error {
	return walkShapeTreeReader(bytes.NewReader(data), partName, slideIdx, emit)
}

// walkShapeTreeReader is the primary forward-only PresentationML shape walker.
// It consumes XML tokens directly from the part stream; callers do not materialize
// the slide XML before semantic events are emitted.
func walkShapeTreeReader(r io.Reader, partName string, slideIdx int, emit func(Event) error) error {
	dec := xml.NewDecoder(r)
	type shapeCtx struct {
		id         int
		name       string
		kind       string
		depth      int
		pending    bool
		started    bool
		listStyles map[int]textStyleLevel
	}
	type transformCtx struct {
		shape *shapeCtx
		value VisualTransform
	}
	type paintCtx struct {
		shape       *shapeCtx
		kind        EventType
		width       int64
		color       VisualColor
		gradient    GradientFill
		pattern     PatternFill
		stop        *GradientStop
		patternSlot string
		line        LineProperties
		target      string
	}
	type effectCtx struct {
		shape *shapeCtx
		value EffectProperties
	}
	var shapes []*shapeCtx
	var paragraphs []bool
	var tableStack []int
	var textBuf strings.Builder
	var captureText bool
	var fieldType string
	var elements []string
	var transform *transformCtx
	var paint *paintCtx
	var effect *effectCtx
	var runProps *RunProperties
	var runColorDepth int
	var textStyleKind string
	var textStyleLevelIdx int
	var textStyleParagraph *ParagraphProperties
	var textStyleRun *RunProperties
	var shapeListStyleLevel int
	var shapeListStyleParagraph *ParagraphProperties
	var shapeListStyleRun *RunProperties
	var activeParagraphStyle *textStyleLevel
	var bulletColorTarget *VisualColor
	var paragraphBulletColor VisualColor
	var spacingTarget string
	var paragraphPropsActive bool
	var bodyProps *TextBodyProperties
	var lineStyleRef *VisualColor
	var lineStyleRefIndex int
	var fillStyleRef *VisualColor
	var fillStyleRefIndex int
	var effectStyleRef *VisualColor
	var effectStyleRefIndex int
	var activeVisualColor *VisualColor

	pushShape := func(kind string) {
		shapes = append(shapes, &shapeCtx{kind: kind, depth: len(shapes), pending: true})
	}
	peekShape := func() *shapeCtx {
		if len(shapes) == 0 {
			return nil
		}
		return shapes[len(shapes)-1]
	}
	popShape := func() *shapeCtx {
		if len(shapes) == 0 {
			return nil
		}
		last := shapes[len(shapes)-1]
		shapes = shapes[:len(shapes)-1]
		return last
	}
	baseEvent := func(typ EventType, shape *shapeCtx) Event {
		ev := Event{Type: typ, PartName: partName, SlideIndex: slideIdx}
		if shape != nil {
			ev.ShapeID, ev.ShapeName, ev.ShapeKind, ev.Depth = shape.id, shape.name, shape.kind, shape.depth
		}
		return ev
	}
	attr := func(attrs []xml.Attr, local string) string {
		for _, a := range attrs {
			if a.Name.Local == local {
				return a.Value
			}
		}
		return ""
	}
	int64Attr := func(attrs []xml.Attr, local string) int64 {
		v, _ := strconv.ParseInt(attr(attrs, local), 10, 64)
		return v
	}
	intAttr := func(attrs []xml.Attr, local string) int {
		v, _ := strconv.Atoi(attr(attrs, local))
		return v
	}
	boolAttr := func(attrs []xml.Attr, local string) bool {
		v := attr(attrs, local)
		return v == "1" || strings.EqualFold(v, "true")
	}
	parent := func() string {
		if len(elements) < 2 {
			return ""
		}
		return elements[len(elements)-2]
	}
	paintTarget := func() string {
		for i := len(elements) - 2; i >= 0; i-- {
			switch elements[i] {
			case "tcPr":
				return "tableCell"
			case "spPr", "grpSpPr", "pic", "cxnSp":
				return "shape"
			}
		}
		return "shape"
	}
	inShapeListStyle := func() bool {
		if peekShape() == nil || textStyleKind != "" {
			return false
		}
		for i := len(elements) - 2; i >= 0; i-- {
			if elements[i] == "lstStyle" {
				return true
			}
			if elements[i] == "txBody" || elements[i] == "sp" || elements[i] == "pic" || elements[i] == "cxnSp" || elements[i] == "grpSp" || elements[i] == "graphicFrame" {
				return false
			}
		}
		return false
	}
	runPropertiesEmpty := func(r RunProperties) bool {
		return !r.Bold && !r.Italic && !r.Underline && !r.Strike &&
			!r.BoldSet && !r.ItalicSet && !r.UnderlineSet && !r.StrikeSet &&
			r.FontSize == 0 && r.FontFamily == "" && r.Color.Value == "" && r.Highlight.Value == "" &&
			r.Baseline == 0 && r.Caps == "" && r.Language == "" && r.AltLanguage == "" &&
			r.CharacterSpacing == 0 && r.Kerning == 0 && r.HyperlinkRelID == ""
	}
	emitParagraphListStyle := func(sh *shapeCtx, level int) error {
		activeParagraphStyle = nil
		if sh == nil || len(sh.listStyles) == 0 {
			return nil
		}
		style, ok := sh.listStyles[level]
		if !ok {
			style, ok = sh.listStyles[0]
		}
		if !ok {
			return nil
		}
		activeParagraphStyle = &style
		if err := emit(Event{Type: EventParagraphProps, PartName: partName, SlideIndex: slideIdx, ShapeID: sh.id, ShapeName: sh.name, ShapeKind: sh.kind, Depth: sh.depth, Paragraph: style.Paragraph}); err != nil {
			return err
		}
		if !runPropertiesEmpty(style.Run) {
			ev := baseEvent(EventRunProps, sh)
			ev.Run = style.Run
			if err := emit(ev); err != nil {
				return err
			}
		}
		return nil
	}

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		switch se := tok.(type) {
		case xml.StartElement:
			local := se.Name.Local
			elements = append(elements, local)
			switch local {
			case "sldSz":
				if slideIdx < 0 {
					cx, _ := strconv.ParseInt(attr(se.Attr, "cx"), 10, 64)
					cy, _ := strconv.ParseInt(attr(se.Attr, "cy"), 10, 64)
					if err := emit(Event{Type: EventPresentationProperties, PartName: partName, SlideIndex: slideIdx, CanvasWidthEMU: cx, CanvasHeightEMU: cy}); err != nil {
						return err
					}
				}
			case "sp", "pic", "cxnSp", "grpSp", "graphicFrame":
				pushShape(local)
			case "cNvPr":
				if sh := peekShape(); sh != nil && sh.pending {
					sh.id = intAttr(se.Attr, "id")
					sh.name = attr(se.Attr, "name")
					start := baseEvent(EventStartShape, sh)
					if err := emit(start); err != nil {
						return err
					}
					if sh.kind == "grpSp" {
						start.Type = EventStartGroup
						if err := emit(start); err != nil {
							return err
						}
					}
					sh.pending = false
					sh.started = true
				}
			case "stCxn", "endCxn":
				if sh := peekShape(); sh != nil && sh.kind == "cxnSp" {
					ev := baseEvent(EventConnectorEndpoint, sh)
					ev.ConnectorEndpoint = local
					ev.ConnectorShapeID = intAttr(se.Attr, "id")
					ev.ConnectorSiteIdx = intAttr(se.Attr, "idx")
					if err := emit(ev); err != nil {
						return err
					}
				}
			case "ph":
				if sh := peekShape(); sh != nil {
					ev := baseEvent(EventPlaceholder, sh)
					ev.Placeholder = attr(se.Attr, "type")
					ev.PlaceholderIdx = intAttr(se.Attr, "idx")
					ev.PlaceholderHasCustomPrompt = boolAttr(se.Attr, "hasCustomPrompt")
					ev.PlaceholderOrient = attr(se.Attr, "orient")
					ev.PlaceholderSize = attr(se.Attr, "sz")
					if err := emit(ev); err != nil {
						return err
					}
				}
			case "xfrm":
				if sh := peekShape(); sh != nil {
					transform = &transformCtx{shape: sh}
					transform.value.Rotation = intAttr(se.Attr, "rot")
					transform.value.FlipH = boolAttr(se.Attr, "flipH")
					transform.value.FlipV = boolAttr(se.Attr, "flipV")
				}
			case "off":
				if transform != nil {
					transform.value.X, transform.value.Y = xmlInt64Attr(se.Attr, "x"), xmlInt64Attr(se.Attr, "y")
					transform.value.HasOffset = true
				}
			case "ext":
				if transform != nil {
					transform.value.CX, transform.value.CY = int64Attr(se.Attr, "cx"), int64Attr(se.Attr, "cy")
					transform.value.HasExtents = true
				}
			case "chOff":
				if transform != nil {
					transform.value.ChildX, transform.value.ChildY = xmlInt64Attr(se.Attr, "x"), xmlInt64Attr(se.Attr, "y")
					transform.value.HasChildOffset = true
				}
			case "chExt":
				if transform != nil {
					transform.value.ChildCX, transform.value.ChildCY = int64Attr(se.Attr, "cx"), int64Attr(se.Attr, "cy")
					transform.value.HasChildExtents = true
				}
			case "prstGeom":
				if sh := peekShape(); sh != nil {
					ev := baseEvent(EventGeometry, sh)
					ev.Geometry = attr(se.Attr, "prst")
					if err := emit(ev); err != nil {
						return err
					}
				}
			case "custGeom":
				if sh := peekShape(); sh != nil {
					var cg drawingxml.CustomGeometry
					if err := dec.DecodeElement(&cg, &se); err != nil {
						return err
					}
					ev := baseEvent(EventGeometry, sh)
					ev.Geometry = "custom"
					ev.GeometryData = cg.DrawingGeometry()
					if err := emit(ev); err != nil {
						return err
					}
					elements = elements[:len(elements)-1]
				}
			case "noFill":
				if runProps == nil {
					if sh := peekShape(); sh != nil {
						kind := EventNoFill
						if parent() == "ln" {
							kind = EventLine
						}
						paint = &paintCtx{shape: sh, kind: kind, target: paintTarget(), width: func() int64 {
							if kind == EventLine {
								return int64Attr(se.Attr, "w")
							}
							return 0
						}()}
					}
				}
			case "gradFill":
				if runProps == nil {
					if sh := peekShape(); sh != nil {
						paint = &paintCtx{shape: sh, kind: EventGradientFill, target: paintTarget()}
						paint.gradient.Flip = attr(se.Attr, "flip")
						paint.gradient.RotateWithShape = boolAttr(se.Attr, "rotWithShape")
					}
				}
			case "gs":
				if paint != nil && paint.kind == EventGradientFill {
					st := GradientStop{Position: intAttr(se.Attr, "pos")}
					paint.gradient.Stops = append(paint.gradient.Stops, st)
					paint.stop = &paint.gradient.Stops[len(paint.gradient.Stops)-1]
				}
			case "lin":
				if paint != nil && paint.kind == EventGradientFill {
					paint.gradient.Angle = intAttr(se.Attr, "ang")
				}
			case "path":
				if paint != nil && paint.kind == EventGradientFill {
					paint.gradient.Path = attr(se.Attr, "path")
				}
			case "pattFill":
				if runProps == nil {
					if sh := peekShape(); sh != nil {
						paint = &paintCtx{shape: sh, kind: EventPatternFill, target: paintTarget()}
						paint.pattern.Preset = attr(se.Attr, "prst")
					}
				}
			case "fgClr", "bgClr":
				if paint != nil && paint.kind == EventPatternFill {
					paint.patternSlot = local
				}
			case "solidFill":
				if runProps != nil {
					runColorDepth++
				} else if sh := peekShape(); sh != nil {
					if paint != nil && paint.kind == EventLine {
						// solidFill belongs to the active line/border.
					} else {
						paint = &paintCtx{shape: sh, kind: EventFill, target: paintTarget()}
					}
				}
			case "lnRef":
				if sh := peekShape(); sh != nil {
					lineStyleRef = &VisualColor{}
					lineStyleRefIndex, _ = strconv.Atoi(xmlAttr(se.Attr, "idx"))
				}
			case "effectRef":
				if sh := peekShape(); sh != nil {
					effectStyleRef = &VisualColor{}
					effectStyleRefIndex, _ = strconv.Atoi(xmlAttr(se.Attr, "idx"))
				}
			case "fillRef":
				if sh := peekShape(); sh != nil {
					fillStyleRef = &VisualColor{}
					fillStyleRefIndex, _ = strconv.Atoi(xmlAttr(se.Attr, "idx"))
				}
			case "ln", "lnL", "lnR", "lnT", "lnB":
				if sh := peekShape(); sh != nil {
					target := paintTarget()
					if local == "lnL" {
						target = "borderLeft"
					} else if local == "lnR" {
						target = "borderRight"
					} else if local == "lnT" {
						target = "borderTop"
					} else if local == "lnB" {
						target = "borderBottom"
					}
					paint = &paintCtx{shape: sh, kind: EventLine, target: target, width: int64Attr(se.Attr, "w")}
					paint.line.WidthEMU = paint.width
					paint.line.Cap, paint.line.Compound = attr(se.Attr, "cap"), attr(se.Attr, "cmpd")
				}
			case "prstDash":
				if paint != nil && paint.kind == EventLine {
					paint.line.Dash = attr(se.Attr, "val")
				}
			case "round", "bevel":
				if paint != nil && paint.kind == EventLine {
					paint.line.Join = local
				}
			case "miter":
				if paint != nil && paint.kind == EventLine {
					paint.line.Join = "miter"
					paint.line.MiterLimit = intAttr(se.Attr, "lim")
				}
			case "headEnd":
				if paint != nil && paint.kind == EventLine {
					paint.line.HeadType, paint.line.HeadWidth, paint.line.HeadLength = attr(se.Attr, "type"), attr(se.Attr, "w"), attr(se.Attr, "len")
				}
			case "tailEnd":
				if paint != nil && paint.kind == EventLine {
					paint.line.TailType, paint.line.TailWidth, paint.line.TailLength = attr(se.Attr, "type"), attr(se.Attr, "w"), attr(se.Attr, "len")
				}
			case "outerShdw", "innerShdw", "glow", "softEdge", "reflection", "blur":
				// Effects nested in rPr/defRPr/endParaRPr belong to text, not
				// to the containing presentation shape. Promoting those effects
				// to Shape.Effects creates large shadowed placeholder rectangles.
				if sh := peekShape(); sh != nil && runProps == nil {
					effect = &effectCtx{shape: sh}
					effect.value.Kind = local
					effect.value.BlurRad, effect.value.Dist, effect.value.Dir, effect.value.Rad = int64Attr(se.Attr, "blurRad"), int64Attr(se.Attr, "dist"), int64Attr(se.Attr, "dir"), int64Attr(se.Attr, "rad")
					effect.value.SX, effect.value.SY, effect.value.KX, effect.value.KY = intAttr(se.Attr, "sx"), intAttr(se.Attr, "sy"), intAttr(se.Attr, "kx"), intAttr(se.Attr, "ky")
					effect.value.Align, effect.value.RotateWithShape, effect.value.Grow = attr(se.Attr, "algn"), boolAttr(se.Attr, "rotWithShape"), boolAttr(se.Attr, "grow")
					effect.value.StartAlpha, effect.value.StartPos, effect.value.EndAlpha, effect.value.EndPos, effect.value.FadeDir = intAttr(se.Attr, "stA"), intAttr(se.Attr, "stPos"), intAttr(se.Attr, "endA"), intAttr(se.Attr, "endPos"), intAttr(se.Attr, "fadeDir")
				}
			case "srgbClr", "schemeClr", "sysClr", "prstClr":
				c := VisualColor{Kind: local, Value: attr(se.Attr, "val")}
				if local == "sysClr" && c.Value == "" {
					c.Value = attr(se.Attr, "lastClr")
				}
				activeVisualColor = nil
				if runProps != nil && parent() == "highlight" {
					runProps.Highlight = c
					activeVisualColor = &runProps.Highlight
				} else if runProps != nil && runColorDepth > 0 {
					runProps.Color = c
					activeVisualColor = &runProps.Color
				} else if bulletColorTarget != nil {
					*bulletColorTarget = c
					activeVisualColor = bulletColorTarget
				} else if lineStyleRef != nil {
					*lineStyleRef = c
					activeVisualColor = lineStyleRef
				} else if fillStyleRef != nil {
					*fillStyleRef = c
					activeVisualColor = fillStyleRef
				} else if effectStyleRef != nil {
					*effectStyleRef = c
					activeVisualColor = effectStyleRef
				} else if effect != nil {
					effect.value.Color = c
					activeVisualColor = &effect.value.Color
				} else if paint != nil {
					switch {
					case paint.kind == EventGradientFill && paint.stop != nil:
						paint.stop.Color = c
						activeVisualColor = &paint.stop.Color
					case paint.kind == EventPatternFill && paint.patternSlot == "fgClr":
						paint.pattern.Foreground = c
						activeVisualColor = &paint.pattern.Foreground
					case paint.kind == EventPatternFill && paint.patternSlot == "bgClr":
						paint.pattern.Background = c
						activeVisualColor = &paint.pattern.Background
					default:
						paint.color = c
						activeVisualColor = &paint.color
					}
				}
			case "shade", "tint", "lumMod", "lumOff", "satMod", "satOff":
				if activeVisualColor != nil {
					v := intAttr(se.Attr, "val")
					switch local {
					case "shade":
						activeVisualColor.Shade, activeVisualColor.HasShade = v, true
					case "tint":
						activeVisualColor.Tint, activeVisualColor.HasTint = v, true
					case "lumMod":
						activeVisualColor.LumMod, activeVisualColor.HasLumMod = v, true
					case "lumOff":
						activeVisualColor.LumOff, activeVisualColor.HasLumOff = v, true
					case "satMod":
						activeVisualColor.SatMod, activeVisualColor.HasSatMod = v, true
					case "satOff":
						activeVisualColor.SatOff, activeVisualColor.HasSatOff = v, true
					}
				}
			case "alpha":
				if effect != nil {
					effect.value.Color.Alpha = intAttr(se.Attr, "val")
					effect.value.Color.HasAlpha = true
				} else if paint != nil {
					v := intAttr(se.Attr, "val")
					switch {
					case paint.kind == EventGradientFill && paint.stop != nil:
						paint.stop.Color.Alpha = v
						paint.stop.Color.HasAlpha = true
					case paint.kind == EventPatternFill && paint.patternSlot == "fgClr":
						paint.pattern.Foreground.Alpha = v
						paint.pattern.Foreground.HasAlpha = true
					case paint.kind == EventPatternFill && paint.patternSlot == "bgClr":
						paint.pattern.Background.Alpha = v
						paint.pattern.Background.HasAlpha = true
					default:
						paint.color.Alpha = v
						paint.color.HasAlpha = true
					}
				}
			case "blip":
				if sh := peekShape(); sh != nil {
					ev := baseEvent(EventBlip, sh)
					for _, a := range se.Attr {
						if a.Name.Local == "embed" && isRelationshipNamespace(a.Name.Space) {
							ev.RelID = a.Value
						}
						if a.Name.Local == "link" && isRelationshipNamespace(a.Name.Space) {
							ev.LinkRelID = a.Value
						}
					}
					if err := emit(ev); err != nil {
						return err
					}
				}
			case "srcRect":
				if sh := peekShape(); sh != nil {
					ev := baseEvent(EventBlip, sh)
					ev.CropLeft, ev.CropTop, ev.CropRight, ev.CropBottom = intAttr(se.Attr, "l"), intAttr(se.Attr, "t"), intAttr(se.Attr, "r"), intAttr(se.Attr, "b")
					if err := emit(ev); err != nil {
						return err
					}
				}
			case "tile":
				if sh := peekShape(); sh != nil {
					ev := baseEvent(EventBlip, sh)
					ev.PictureTileTx, ev.PictureTileTy = int64Attr(se.Attr, "tx"), int64Attr(se.Attr, "ty")
					ev.PictureTileSx, ev.PictureTileSy = intAttr(se.Attr, "sx"), intAttr(se.Attr, "sy")
					ev.PictureTileFlip, ev.PictureTileAlign = attr(se.Attr, "flip"), attr(se.Attr, "algn")
					if err := emit(ev); err != nil {
						return err
					}
				}
			case "fillRect":
				if sh := peekShape(); sh != nil {
					ev := baseEvent(EventBlip, sh)
					ev.PictureFillLeft, ev.PictureFillTop, ev.PictureFillRight, ev.PictureFillBottom = intAttr(se.Attr, "l"), intAttr(se.Attr, "t"), intAttr(se.Attr, "r"), intAttr(se.Attr, "b")
					if err := emit(ev); err != nil {
						return err
					}
				}
			case "bodyPr":
				if peekShape() != nil {
					bp := TextBodyProperties{Wrap: attr(se.Attr, "wrap"), Anchor: attr(se.Attr, "anchor"), Vert: attr(se.Attr, "vert"), Rot: intAttr(se.Attr, "rot"), HasRot: attr(se.Attr, "rot") != "", Columns: intAttr(se.Attr, "numCol"), HasColumns: attr(se.Attr, "numCol") != "", ColumnSpacingEMU: int64Attr(se.Attr, "spcCol"), AnchorCtr: boolAttr(se.Attr, "anchorCtr"), HasAnchorCtr: attr(se.Attr, "anchorCtr") != ""}
					for _, x := range []struct {
						name string
						dst  *int64
						has  *bool
					}{{"tIns", &bp.InsetT, &bp.HasInsetT}, {"bIns", &bp.InsetB, &bp.HasInsetB}, {"lIns", &bp.InsetL, &bp.HasInsetL}, {"rIns", &bp.InsetR, &bp.HasInsetR}} {
						if attr(se.Attr, x.name) != "" {
							*x.dst = int64Attr(se.Attr, x.name)
							*x.has = true
						}
					}
					bodyProps = &bp
				}
			case "noAutofit", "spAutoFit":
				if bodyProps != nil {
					bodyProps.AutofitType = local
				}
			case "normAutofit":
				if bodyProps != nil {
					bodyProps.AutofitType = local
					bodyProps.AutofitFontScale = intAttr(se.Attr, "fontScale")
					bodyProps.AutofitLineSpaceReduction = intAttr(se.Attr, "lnSpcReduction")
				}
			case "titleStyle", "bodyStyle", "otherStyle":
				textStyleKind = local
			case "defaultTextStyle":
				textStyleKind = "default"
			case "lvl1pPr", "lvl2pPr", "lvl3pPr", "lvl4pPr", "lvl5pPr", "lvl6pPr", "lvl7pPr", "lvl8pPr", "lvl9pPr":
				if textStyleKind != "" {
					textStyleLevelIdx = int(local[3] - '1')
					pp := paragraphPropsFromAttrs(se.Attr, attr)
					pp.Level, pp.HasLevel = textStyleLevelIdx, true
					textStyleParagraph = &pp
				} else if inShapeListStyle() {
					shapeListStyleLevel = int(local[3] - '1')
					pp := paragraphPropsFromAttrs(se.Attr, attr)
					pp.Level, pp.HasLevel = shapeListStyleLevel, true
					shapeListStyleParagraph = &pp
				}
			case "spcBef", "spcAft", "lnSpc":
				if textStyleParagraph != nil || shapeListStyleParagraph != nil || paragraphPropsActive {
					spacingTarget = local
				}
			case "spcPts", "spcPct":
				if textStyleParagraph != nil {
					applySpacing(textStyleParagraph, spacingTarget, intAttr(se.Attr, "val"), local == "spcPct")
				} else if shapeListStyleParagraph != nil {
					applySpacing(shapeListStyleParagraph, spacingTarget, intAttr(se.Attr, "val"), local == "spcPct")
				} else if paragraphPropsActive {
					if sh := peekShape(); sh != nil {
						pp := ParagraphProperties{}
						applySpacing(&pp, spacingTarget, intAttr(se.Attr, "val"), local == "spcPct")
						ev := baseEvent(EventParagraphProps, sh)
						ev.Paragraph = pp
						if err := emit(ev); err != nil {
							return err
						}
					}
				}
			case "buChar":
				if textStyleParagraph != nil {
					textStyleParagraph.BulletChar = attr(se.Attr, "char")
					textStyleParagraph.HasBullet = true
				} else if shapeListStyleParagraph != nil {
					shapeListStyleParagraph.BulletChar = attr(se.Attr, "char")
					shapeListStyleParagraph.HasBullet = true
				} else if paragraphPropsActive {
					if sh := peekShape(); sh != nil {
						ev := baseEvent(EventParagraphProps, sh)
						ev.Paragraph = ParagraphProperties{BulletChar: attr(se.Attr, "char"), HasBullet: true}
						if err := emit(ev); err != nil {
							return err
						}
					}
				}
			case "buAutoNum":
				pp := ParagraphProperties{AutoNum: true, HasAutoNum: true, AutoNumType: attr(se.Attr, "type"), AutoNumStart: intAttr(se.Attr, "startAt")}
				if textStyleParagraph != nil {
					textStyleParagraph.AutoNum, textStyleParagraph.HasAutoNum, textStyleParagraph.AutoNumType, textStyleParagraph.AutoNumStart = pp.AutoNum, pp.HasAutoNum, pp.AutoNumType, pp.AutoNumStart
				} else if shapeListStyleParagraph != nil {
					shapeListStyleParagraph.AutoNum, shapeListStyleParagraph.HasAutoNum, shapeListStyleParagraph.AutoNumType, shapeListStyleParagraph.AutoNumStart = pp.AutoNum, pp.HasAutoNum, pp.AutoNumType, pp.AutoNumStart
				} else if paragraphPropsActive {
					if sh := peekShape(); sh != nil {
						ev := baseEvent(EventParagraphProps, sh)
						ev.Paragraph = pp
						if err := emit(ev); err != nil {
							return err
						}
					}
				}
			case "buNone":
				pp := ParagraphProperties{HasBullet: true, HasAutoNum: true}
				if textStyleParagraph != nil {
					textStyleParagraph.HasBullet, textStyleParagraph.BulletChar, textStyleParagraph.HasAutoNum, textStyleParagraph.AutoNum = true, "", true, false
				} else if shapeListStyleParagraph != nil {
					shapeListStyleParagraph.HasBullet, shapeListStyleParagraph.BulletChar, shapeListStyleParagraph.HasAutoNum, shapeListStyleParagraph.AutoNum = true, "", true, false
				} else if paragraphPropsActive {
					if sh := peekShape(); sh != nil {
						ev := baseEvent(EventParagraphProps, sh)
						ev.Paragraph = pp
						if err := emit(ev); err != nil {
							return err
						}
					}
				}
			case "buFontTx":
				if textStyleParagraph != nil {
					textStyleParagraph.BulletFontTx = true
					textStyleParagraph.BulletFontFamily = ""
				} else if shapeListStyleParagraph != nil {
					shapeListStyleParagraph.BulletFontTx = true
					shapeListStyleParagraph.BulletFontFamily = ""
				} else if paragraphPropsActive {
					if sh := peekShape(); sh != nil {
						ev := baseEvent(EventParagraphProps, sh)
						ev.Paragraph = ParagraphProperties{BulletFontTx: true}
						if err := emit(ev); err != nil {
							return err
						}
					}
				}
			case "buFont":
				value := attr(se.Attr, "typeface")
				if textStyleParagraph != nil {
					textStyleParagraph.BulletFontFamily = value
				} else if shapeListStyleParagraph != nil {
					shapeListStyleParagraph.BulletFontFamily = value
				} else if paragraphPropsActive && value != "" {
					if sh := peekShape(); sh != nil {
						ev := baseEvent(EventParagraphProps, sh)
						ev.Paragraph = ParagraphProperties{BulletFontFamily: value}
						if err := emit(ev); err != nil {
							return err
						}
					}
				}
			case "buClr":
				switch {
				case textStyleParagraph != nil:
					bulletColorTarget = &textStyleParagraph.BulletColor
				case shapeListStyleParagraph != nil:
					bulletColorTarget = &shapeListStyleParagraph.BulletColor
				case paragraphPropsActive:
					paragraphBulletColor = VisualColor{}
					bulletColorTarget = &paragraphBulletColor
				}
			case "buSzPts", "buSzPct":
				value := intAttr(se.Attr, "val")
				if textStyleParagraph != nil {
					if local == "buSzPts" {
						textStyleParagraph.BulletSize = value
					} else {
						textStyleParagraph.BulletSizePct = value
					}
				} else if shapeListStyleParagraph != nil {
					if local == "buSzPts" {
						shapeListStyleParagraph.BulletSize = value
					} else {
						shapeListStyleParagraph.BulletSizePct = value
					}
				} else if paragraphPropsActive {
					if sh := peekShape(); sh != nil {
						pp := ParagraphProperties{}
						if local == "buSzPts" {
							pp.BulletSize = value
						} else {
							pp.BulletSizePct = value
						}
						ev := baseEvent(EventParagraphProps, sh)
						ev.Paragraph = pp
						if err := emit(ev); err != nil {
							return err
						}
					}
				}
			case "pPr":
				if sh := peekShape(); sh != nil {
					paragraphPropsActive = true
					pp := paragraphPropsFromAttrs(se.Attr, attr)
					if pp.HasLevel {
						if err := emitParagraphListStyle(sh, pp.Level); err != nil {
							return err
						}
					}
					ev := baseEvent(EventParagraphProps, sh)
					ev.Paragraph = pp
					if err := emit(ev); err != nil {
						return err
					}
				}
			case "rPr", "defRPr", "endParaRPr":
				rp := runPropsFromAttrs(se.Attr, attr)
				if local == "defRPr" && textStyleParagraph != nil {
					textStyleRun = &rp
					runProps = textStyleRun
				} else if local == "defRPr" && shapeListStyleParagraph != nil {
					shapeListStyleRun = &rp
					runProps = shapeListStyleRun
				} else {
					runProps = &rp
				}
			case "hlinkClick", "hlinkHover":
				if runProps != nil {
					for _, a := range se.Attr {
						if a.Name.Local == "id" && isRelationshipNamespace(a.Name.Space) {
							runProps.HyperlinkRelID = a.Value
						}
					}
				} else if sh := peekShape(); sh != nil {
					ev := baseEvent(EventShapeAction, sh)
					ev.Action = attr(se.Attr, "action")
					ev.Tooltip = attr(se.Attr, "tooltip")
					for _, a := range se.Attr {
						if a.Name.Local == "id" && isRelationshipNamespace(a.Name.Space) {
							ev.ActionRelID = a.Value
						}
					}
					if err := emit(ev); err != nil {
						return err
					}
				}
			case "tab":
				// a:tab inside a:tabLst is a paragraph tab stop; p:tab is handled below.
				if parent() == "tabLst" && peekShape() != nil {
					ev := baseEvent(EventParagraphProps, peekShape())
					ev.Paragraph.TabStops = []TabStop{{Pos: int64Attr(se.Attr, "pos"), Align: attr(se.Attr, "algn")}}
					if err := emit(ev); err != nil {
						return err
					}
				} else {
					if err := emit(Event{Type: EventTab, PartName: partName, SlideIndex: slideIdx}); err != nil {
						return err
					}
				}
			case "latin":
				if runProps != nil {
					runProps.FontFamily = attr(se.Attr, "typeface")
				}
			case "chart":
				if sh := peekShape(); sh != nil {
					ev := baseEvent(EventGraphicObject, sh)
					ev.GraphicObjectKind = "chart"
					for _, a := range se.Attr {
						if a.Name.Local == "id" && isRelationshipNamespace(a.Name.Space) {
							ev.RelID = a.Value
						}
					}
					if err := emit(ev); err != nil {
						return err
					}
				}
			case "relIds":
				if sh := peekShape(); sh != nil {
					ev := baseEvent(EventGraphicObject, sh)
					ev.GraphicObjectKind = "smartArt"
					for _, a := range se.Attr {
						if a.Name.Local == "dm" && isRelationshipNamespace(a.Name.Space) {
							ev.RelID = a.Value
						}
					}
					if err := emit(ev); err != nil {
						return err
					}
				}
			case "videoFile", "audioFile":
				if sh := peekShape(); sh != nil {
					ev := baseEvent(EventGraphicObject, sh)
					ev.GraphicObjectKind = strings.TrimSuffix(local, "File")
					for _, a := range se.Attr {
						if a.Name.Local == "link" && isRelationshipNamespace(a.Name.Space) {
							ev.RelID = a.Value
						}
					}
					if err := emit(ev); err != nil {
						return err
					}
				}
			case "oleObj":
				if sh := peekShape(); sh != nil {
					ev := baseEvent(EventGraphicObject, sh)
					ev.GraphicObjectKind = "ole"
					for _, a := range se.Attr {
						switch {
						case a.Name.Local == "id" && isRelationshipNamespace(a.Name.Space):
							ev.RelID = a.Value
						case a.Name.Local == "name":
							ev.OLEName = a.Value
						case a.Name.Local == "progId":
							ev.OLEProgID = a.Value
						case a.Name.Local == "spid":
							ev.OLEShapeRef = a.Value
						}
					}
					if err := emit(ev); err != nil {
						return err
					}
				}
			case "tbl":
				tableStack = append(tableStack, 0)
				if sh := peekShape(); sh != nil {
					ev := baseEvent(EventStartTable, sh)
					ev.GraphicObjectKind = "table"
					if err := emit(ev); err != nil {
						return err
					}
				}
			case "gridCol":
				if len(tableStack) > 0 {
					ev := baseEvent(EventTableColumn, peekShape())
					ev.TableValue = int64Attr(se.Attr, "w")
					if err := emit(ev); err != nil {
						return err
					}
				}
			case "tr":
				if len(tableStack) > 0 {
					tableStack[len(tableStack)-1] = 0
					ev := baseEvent(EventStartTableRow, peekShape())
					ev.TableValue = int64Attr(se.Attr, "h")
					if err := emit(ev); err != nil {
						return err
					}
				}
			case "tc":
				if len(tableStack) > 0 {
					idx := len(tableStack) - 1
					if tableStack[idx] > 0 {
						if err := emit(Event{Type: EventText, PartName: partName, SlideIndex: slideIdx, Text: "\t"}); err != nil {
							return err
						}
					}
					tableStack[idx]++
					ev := baseEvent(EventStartTableCell, peekShape())
					ev.TableValue = int64(intAttr(se.Attr, "gridSpan"))
					ev.TableGridSpan = intAttr(se.Attr, "gridSpan")
					ev.TableRowSpan = intAttr(se.Attr, "rowSpan")
					ev.TableHMerge = boolAttr(se.Attr, "hMerge")
					ev.TableVMerge = boolAttr(se.Attr, "vMerge")
					if err := emit(ev); err != nil {
						return err
					}
				}
			case "tcPr":
				if len(tableStack) > 0 {
					ev := baseEvent(EventTableCellProps, peekShape())
					ev.TableAnchor = attr(se.Attr, "anchor")
					ev.TableMarginLeft = int64Attr(se.Attr, "marL")
					ev.TableMarginRight = int64Attr(se.Attr, "marR")
					ev.TableMarginTop = int64Attr(se.Attr, "marT")
					ev.TableMarginBottom = int64Attr(se.Attr, "marB")
					ev.TableMarginLeftSet = attr(se.Attr, "marL") != ""
					ev.TableMarginRightSet = attr(se.Attr, "marR") != ""
					ev.TableMarginTopSet = attr(se.Attr, "marT") != ""
					ev.TableMarginBottomSet = attr(se.Attr, "marB") != ""
					if err := emit(ev); err != nil {
						return err
					}
				}
			case "p":
				paragraphs = append(paragraphs, true)
				if err := emit(Event{Type: EventStartParagraph, PartName: partName, SlideIndex: slideIdx}); err != nil {
					return err
				}
				if err := emitParagraphListStyle(peekShape(), 0); err != nil {
					return err
				}
			case "r":
				if len(paragraphs) > 0 {
					if err := emit(baseEvent(EventStartRun, peekShape())); err != nil {
						return err
					}
					if activeParagraphStyle != nil {
						ev := baseEvent(EventRunProps, peekShape())
						ev.Run = activeParagraphStyle.Run
						if err := emit(ev); err != nil {
							return err
						}
					}
				}
			case "fld":
				fieldType = attr(se.Attr, "type")
				if len(paragraphs) > 0 {
					ev := baseEvent(EventStartRun, peekShape())
					ev.FieldType = fieldType
					if err := emit(ev); err != nil {
						return err
					}
				}
			case "t":
				captureText = true
				textBuf.Reset()
			case "br":
				if err := emit(Event{Type: EventBreak, PartName: partName, SlideIndex: slideIdx}); err != nil {
					return err
				}
			}
		case xml.EndElement:
			local := se.Name.Local
			if local == "srgbClr" || local == "schemeClr" || local == "sysClr" || local == "prstClr" {
				activeVisualColor = nil
			}
			switch local {
			case "pPr":
				paragraphPropsActive = false
				spacingTarget = ""
				bulletColorTarget = nil
				paragraphBulletColor = VisualColor{}
			case "buClr":
				if paragraphPropsActive && textStyleParagraph == nil && shapeListStyleParagraph == nil && paragraphBulletColor.Value != "" {
					if sh := peekShape(); sh != nil {
						ev := baseEvent(EventParagraphProps, sh)
						ev.Paragraph = ParagraphProperties{BulletColor: paragraphBulletColor}
						if err := emit(ev); err != nil {
							return err
						}
					}
				}
				bulletColorTarget = nil
				paragraphBulletColor = VisualColor{}
			case "bodyPr":
				if bodyProps != nil {
					ev := baseEvent(EventTextBodyProps, peekShape())
					ev.TextBody = *bodyProps
					if err := emit(ev); err != nil {
						return err
					}
					bodyProps = nil
				}
			case "defRPr":
				runProps = nil
				runColorDepth = 0
			case "lvl1pPr", "lvl2pPr", "lvl3pPr", "lvl4pPr", "lvl5pPr", "lvl6pPr", "lvl7pPr", "lvl8pPr", "lvl9pPr":
				if textStyleKind != "" && textStyleParagraph != nil {
					ev := Event{Type: EventTextStyle, PartName: partName, SlideIndex: slideIdx, TextStyleKind: textStyleKind, TextStyleLevel: textStyleLevelIdx, Paragraph: *textStyleParagraph}
					if textStyleRun != nil {
						ev.Run = *textStyleRun
					}
					if err := emit(ev); err != nil {
						return err
					}
				} else if shapeListStyleParagraph != nil {
					if sh := peekShape(); sh != nil {
						if sh.listStyles == nil {
							sh.listStyles = make(map[int]textStyleLevel)
						}
						style := textStyleLevel{Paragraph: *shapeListStyleParagraph}
						if shapeListStyleRun != nil {
							style.Run = *shapeListStyleRun
						}
						sh.listStyles[shapeListStyleLevel] = style
					}
				}
				textStyleParagraph, textStyleRun = nil, nil
				shapeListStyleParagraph, shapeListStyleRun = nil, nil
				spacingTarget = ""
			case "titleStyle", "bodyStyle", "otherStyle":
				textStyleKind = ""
			case "endParaRPr":
				// endParaRPr is a text-property scope, not shape paint. Keep its
				// solidFill/noFill children out of the shape visual event stream.
				// Paragraph-default projection can consume these properties separately;
				// clearing the scope here prevents them from overwriting <p:spPr>.
				runProps = nil
				runColorDepth = 0
			case "rPr":
				if runProps != nil {
					ev := baseEvent(EventRunProps, peekShape())
					ev.Run = *runProps
					if err := emit(ev); err != nil {
						return err
					}
					runProps = nil
					runColorDepth = 0
				}
			case "xfrm":
				if transform != nil {
					ev := baseEvent(EventTransform, transform.shape)
					ev.Transform = transform.value
					if err := emit(ev); err != nil {
						return err
					}
					transform = nil
				}
			case "noFill":
				if paint != nil && paint.kind == EventNoFill {
					ev := baseEvent(EventNoFill, paint.shape)
					ev.PaintTarget = paint.target
					if err := emit(ev); err != nil {
						return err
					}
					paint = nil
				}
			case "gradFill":
				if paint != nil && paint.kind == EventGradientFill {
					ev := baseEvent(EventGradientFill, paint.shape)
					ev.PaintTarget = paint.target
					ev.Gradient = paint.gradient
					if err := emit(ev); err != nil {
						return err
					}
					paint = nil
				}
			case "gs":
				if paint != nil && paint.kind == EventGradientFill {
					paint.stop = nil
				}
			case "pattFill":
				if paint != nil && paint.kind == EventPatternFill {
					ev := baseEvent(EventPatternFill, paint.shape)
					ev.PaintTarget = paint.target
					ev.Pattern = paint.pattern
					if err := emit(ev); err != nil {
						return err
					}
					paint = nil
				}
			case "fgClr", "bgClr":
				if paint != nil && paint.kind == EventPatternFill {
					paint.patternSlot = ""
				}
			case "solidFill":
				if runProps != nil && runColorDepth > 0 {
					runColorDepth--
				} else if paint != nil && paint.kind == EventFill {
					ev := baseEvent(EventFill, paint.shape)
					ev.PaintTarget = paint.target
					ev.Color = paint.color
					if err := emit(ev); err != nil {
						return err
					}
					paint = nil
				}
			case "lnRef":
				if lineStyleRef != nil {
					if sh := peekShape(); sh != nil && (lineStyleRef.Value != "" || lineStyleRefIndex != 0) {
						ev := baseEvent(EventLineStyleRef, sh)
						ev.Color = *lineStyleRef
						ev.StyleIndex = lineStyleRefIndex
						if err := emit(ev); err != nil {
							return err
						}
					}
					lineStyleRef = nil
					lineStyleRefIndex = 0
				}
			case "effectRef":
				if effectStyleRef != nil {
					if sh := peekShape(); sh != nil && (effectStyleRef.Value != "" || effectStyleRefIndex != 0) {
						ev := baseEvent(EventEffectStyleRef, sh)
						ev.Color = *effectStyleRef
						ev.StyleIndex = effectStyleRefIndex
						if err := emit(ev); err != nil {
							return err
						}
					}
					effectStyleRef = nil
					effectStyleRefIndex = 0
				}
			case "fillRef":
				if fillStyleRef != nil {
					if sh := peekShape(); sh != nil && (fillStyleRef.Value != "" || fillStyleRefIndex != 0) {
						ev := baseEvent(EventFillStyleRef, sh)
						ev.Color = *fillStyleRef
						ev.StyleIndex = fillStyleRefIndex
						if err := emit(ev); err != nil {
							return err
						}
					}
					fillStyleRef = nil
					fillStyleRefIndex = 0
				}
			case "ln", "lnL", "lnR", "lnT", "lnB":
				if paint != nil && paint.kind == EventLine {
					ev := baseEvent(EventLine, paint.shape)
					ev.PaintTarget = paint.target
					ev.Color = paint.color
					paint.line.Color = paint.color
					ev.LineWidthEMU = paint.width
					ev.Line = paint.line
					if err := emit(ev); err != nil {
						return err
					}
					paint = nil
				}
			case "outerShdw", "innerShdw", "glow", "softEdge", "reflection", "blur":
				if effect != nil && effect.value.Kind == local {
					ev := baseEvent(EventEffect, effect.shape)
					ev.Effect = effect.value
					if err := emit(ev); err != nil {
						return err
					}
					effect = nil
				}
			case "sp", "pic", "cxnSp", "grpSp", "graphicFrame":
				sh := popShape()
				if sh != nil && sh.started {
					end := baseEvent(EventEndShape, sh)
					if sh.kind == "grpSp" {
						end.Type = EventEndGroup
						if err := emit(end); err != nil {
							return err
						}
					}
					end.Type = EventEndShape
					if err := emit(end); err != nil {
						return err
					}
				}
			case "tc":
				if len(tableStack) > 0 {
					if err := emit(baseEvent(EventEndTableCell, peekShape())); err != nil {
						return err
					}
				}
			case "tr":
				if len(tableStack) > 0 {
					if err := emit(baseEvent(EventEndTableRow, peekShape())); err != nil {
						return err
					}
					if err := emit(Event{Type: EventText, PartName: partName, SlideIndex: slideIdx, Text: "\n"}); err != nil {
						return err
					}
				}
			case "tbl":
				if len(tableStack) > 0 {
					if err := emit(baseEvent(EventEndTable, peekShape())); err != nil {
						return err
					}
					tableStack = tableStack[:len(tableStack)-1]
				}
			case "p":
				activeParagraphStyle = nil
				if len(paragraphs) > 0 {
					paragraphs = paragraphs[:len(paragraphs)-1]
					if len(tableStack) == 0 {
						if err := emit(Event{Type: EventEndParagraph, PartName: partName, SlideIndex: slideIdx}); err != nil {
							return err
						}
					}
				}
			case "t":
				if captureText {
					if err := emit(Event{Type: EventText, PartName: partName, SlideIndex: slideIdx, Text: textBuf.String(), FieldType: fieldType}); err != nil {
						return err
					}
					if fieldType != "" {
					}
					captureText = false
					textBuf.Reset()
				}
			case "r":
				if len(paragraphs) > 0 {
					if err := emit(baseEvent(EventEndRun, peekShape())); err != nil {
						return err
					}
				}
			case "fld":
				if len(paragraphs) > 0 {
					if err := emit(baseEvent(EventEndRun, peekShape())); err != nil {
						return err
					}
				}
				fieldType = ""
			}
			if len(elements) > 0 {
				elements = elements[:len(elements)-1]
			}
		case xml.CharData:
			if captureText {
				textBuf.Write(se)
			}
		}
	}
	return nil
}

func paragraphPropsFromAttrs(attrs []xml.Attr, attr func([]xml.Attr, string) string) ParagraphProperties {
	pp := ParagraphProperties{Align: attr(attrs, "algn"), FontAlign: attr(attrs, "fontAlgn")}
	if v := attr(attrs, "rtl"); v == "1" || strings.EqualFold(v, "true") {
		pp.TextDirection = "rtl"
	}
	if v := attr(attrs, "lvl"); v != "" {
		pp.Level, _ = strconv.Atoi(v)
		pp.HasLevel = true
	}
	if v := attr(attrs, "marL"); v != "" {
		pp.MarginLeftEMU, _ = strconv.Atoi(v)
		pp.HasMarginLeft = true
	}
	if v := attr(attrs, "marR"); v != "" {
		pp.MarginRightEMU, _ = strconv.Atoi(v)
		pp.HasMarginRight = true
	}
	if v := attr(attrs, "indent"); v != "" {
		pp.TextIndentEMU, _ = strconv.Atoi(v)
		pp.HasTextIndent = true
	}
	if v := attr(attrs, "defTabSz"); v != "" {
		pp.DefaultTabSizeEMU, _ = strconv.Atoi(v)
		pp.HasDefaultTabSize = true
	}
	return pp
}

func runPropsFromAttrs(attrs []xml.Attr, attr func([]xml.Attr, string) string) RunProperties {
	rp := RunProperties{}
	if v := attr(attrs, "b"); v != "" {
		rp.BoldSet = true
		rp.Bold = v == "1" || strings.EqualFold(v, "true")
	}
	if v := attr(attrs, "i"); v != "" {
		rp.ItalicSet = true
		rp.Italic = v == "1" || strings.EqualFold(v, "true")
	}
	if v := attr(attrs, "u"); v != "" {
		rp.UnderlineSet = true
		rp.Underline = v != "none"
	}
	if v := attr(attrs, "strike"); v != "" {
		rp.StrikeSet = true
		rp.Strike = v != "noStrike"
	}
	rp.FontSize, _ = strconv.Atoi(attr(attrs, "sz"))
	rp.Baseline, _ = strconv.Atoi(attr(attrs, "baseline"))
	rp.Caps = attr(attrs, "cap")
	rp.Language = attr(attrs, "lang")
	rp.AltLanguage = attr(attrs, "altLang")
	rp.CharacterSpacing, _ = strconv.Atoi(attr(attrs, "spc"))
	rp.Kerning, _ = strconv.Atoi(attr(attrs, "kern"))
	return rp
}

func applySpacing(pp *ParagraphProperties, target string, value int, pct bool) {
	if pp == nil {
		return
	}
	switch target {
	case "spcBef":
		pp.HasSpaceBefore = true
		if pct {
			pp.SpaceBeforePct = value
		} else {
			pp.SpaceBefore = value
		}
	case "spcAft":
		pp.HasSpaceAfter = true
		if pct {
			pp.SpaceAfterPct = value
		} else {
			pp.SpaceAfter = value
		}
	case "lnSpc":
		pp.HasLineSpacing = true
		if pct {
			pp.LineSpacingPct = value
		} else {
			pp.LineSpacingPt = value
		}
	}
}

func walkComments(data []byte, partName string, slideIdx int, emit func(Event) error) error {
	return walkCommentsReader(bytes.NewReader(data), partName, slideIdx, emit)
}

func walkCommentsReader(r io.Reader, partName string, slideIdx int, emit func(Event) error) error {
	dec := xml.NewDecoder(r)
	var cm *Event
	var capture bool
	var buf strings.Builder
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil
		}
		switch se := tok.(type) {
		case xml.StartElement:
			switch se.Name.Local {
			case "cm":
				cm = &Event{Type: EventStartComment, PartName: partName, SlideIndex: slideIdx}
				for _, attr := range se.Attr {
					switch attr.Name.Local {
					case "authorId":
						cm.Author = attr.Value
					case "dt":
						cm.Date = attr.Value
					}
				}
			case "text":
				capture = true
				buf.Reset()
			case "pos":
				if cm != nil {
					cm.CommentX, cm.CommentY = xmlInt64Attr(se.Attr, "x"), xmlInt64Attr(se.Attr, "y")
				}
			}
		case xml.EndElement:
			switch se.Name.Local {
			case "cm":
				if cm != nil {
					if err := emit(*cm); err != nil {
						return err
					}
					if err := emit(Event{Type: EventEndComment, PartName: partName, SlideIndex: slideIdx}); err != nil {
						return err
					}
					cm = nil
				}
			case "text":
				if cm != nil {
					cm.Text = buf.String()
				}
				capture = false
				buf.Reset()
			}
		case xml.CharData:
			if capture {
				buf.Write(se)
			}
		}
	}
	return nil
}

// Helper functions for the materialized fallback path.

func findMainPart(pkg *opc.Package) (string, error) {
	rootRels, ok, err := pkg.Relationships("")
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("root relationships not found")
	}
	for _, r := range rootRels.Relationships {
		if canonicalRelationshipType(r.Type) == relTypeOfficeDocument && !r.IsExternal() {
			return rels.ResolveTarget("", r.Target), nil
		}
	}
	return "", fmt.Errorf("officeDocument relationship not found")
}

func readPresentation(pkg *opc.Package, mainPart string) (*presentation, error) {
	p, ok := pkg.Part(mainPart)
	if !ok {
		return nil, fmt.Errorf("main part %q not found", mainPart)
	}
	var pres presentation
	dec := xml.NewDecoder(bytes.NewReader(p.Data))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local != "sldId" {
			continue
		}
		var sld slideRef
		for _, attr := range start.Attr {
			switch attr.Name.Local {
			case "id":
				sld.ID, _ = strconv.Atoi(attr.Value)
			case "show":
				sld.Hidden = attr.Value == "0"
			default:
				if isRelationshipNamespace(attr.Name.Space) && attr.Name.Local == "id" {
					sld.RelID = attr.Value
				}
			}
		}
		pres.Slides = append(pres.Slides, sld)
	}
	return &pres, nil
}

func resolveSlideTargets(mainPart string, relsDoc *rels.Document) map[string]string {
	targets := make(map[string]string)
	if relsDoc == nil {
		return targets
	}
	for _, r := range relsDoc.Relationships {
		if canonicalRelationshipType(r.Type) == relTypeSlide && !r.IsExternal() {
			targets[r.ID] = rels.ResolveTarget(mainPart, r.Target)
		}
	}
	return targets
}

func xmlAttr(attrs []xml.Attr, local string) string {
	for _, a := range attrs {
		if a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}

func xmlInt64Attr(attrs []xml.Attr, local string) int64 {
	v, _ := strconv.ParseInt(xmlAttr(attrs, local), 10, 64)
	return v
}

func xmlIntAttr(attrs []xml.Attr, local string) int {
	v, _ := strconv.Atoi(xmlAttr(attrs, local))
	return v
}

func xmlBoolAttr(attrs []xml.Attr, local string) bool {
	v := xmlAttr(attrs, local)
	return v == "1" || strings.EqualFold(v, "true")
}
