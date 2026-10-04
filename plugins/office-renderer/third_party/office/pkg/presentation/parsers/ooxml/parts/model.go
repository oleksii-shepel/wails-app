package parts

import (
	"bytes"
	"encoding/xml"
	"io"
	"strconv"
	"strings"

	"office/pkg/diag"
	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/drawingxml"
	"office/pkg/ooxml/fontdata"
	"office/pkg/ooxml/media"
	prel "office/pkg/presentation/parsers/ooxml/relationship"
)

// Scene3D stores a subset of DrawingML scene3d camera and lighting metadata.
type Scene3D struct {
	Camera   *SceneCamera   `xml:"camera"`
	LightRig *SceneLightRig `xml:"lightRig"`
}

// SceneCamera stores the scene camera preset and rotation.
type SceneCamera struct {
	Prst string         `xml:"prst,attr,omitempty"`
	FOV  int64          `xml:"fov,attr,omitempty"`
	Zoom int64          `xml:"zoom,attr,omitempty"`
	Rot  *SceneRotation `xml:"rot"`
}

// SceneLightRig stores the scene light rig preset and direction.
type SceneLightRig struct {
	Rig string         `xml:"rig,attr,omitempty"`
	Dir string         `xml:"dir,attr,omitempty"`
	Rot *SceneRotation `xml:"rot"`
}

// SceneRotation stores camera rotation values in OOXML angle units.
type SceneRotation struct {
	Lat int64 `xml:"lat,attr,omitempty"`
	Lon int64 `xml:"lon,attr,omitempty"`
	Rev int64 `xml:"rev,attr,omitempty"`
}

// Shape3D stores a subset of DrawingML sp3d geometry metadata.
type Shape3D struct {
	Z             int64                 `xml:"z,attr,omitempty"`
	ExtrusionH    int64                 `xml:"extrusionH,attr,omitempty"`
	ContourW      int64                 `xml:"contourW,attr,omitempty"`
	PrstMaterial  string                `xml:"prstMaterial,attr,omitempty"`
	BevelT        *Shape3DBevel         `xml:"bevelT"`
	BevelB        *Shape3DBevel         `xml:"bevelB"`
	ContourClr    *ColorRef             `xml:"contourClr>srgbClr"`
	ContourScheme *drawingxml.SchemeClr `xml:"contourClr>schemeClr"`
	ExtrusionClr  *ColorRef             `xml:"extrusionClr>srgbClr"`
	ExtrudeScheme *drawingxml.SchemeClr `xml:"extrusionClr>schemeClr"`
}

// Shape3DBevel stores the bevel size for the top or bottom face.
type Shape3DBevel struct {
	W    int64  `xml:"w,attr,omitempty"`
	H    int64  `xml:"h,attr,omitempty"`
	Prst string `xml:"prst,attr,omitempty"`
}

const (
	transitionalOfficeRelBase = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/"
	strictOfficeRelBase       = "http://purl.oclc.org/ooxml/officeDocument/relationships/"

	relTypeOfficeDocument = prel.OfficeDocument
	relTypeSlide          = prel.Slide
	relTypeSlideMaster    = prel.SlideMaster
	relTypeSlideLayout    = prel.SlideLayout
	relTypeTheme          = prel.Theme
	relTypeThemeOverride  = prel.ThemeOverride
	relTypeFont           = prel.Font
	relTypeImage          = prel.Image
	relTypeAudio          = prel.Audio
	relTypeVideo          = prel.Video
	relTypeNotesMaster    = prel.NotesMaster
	relTypeNotesSlide     = prel.NotesSlide
	relTypeComments       = prel.Comments
	relTypeCommentAuthors = prel.CommentAuthors
	relTypeHyperlink      = prel.Hyperlink
	relTypeTableStyles    = prel.TableStyles

	drawingGraphicDataURIChartTransitional   = "http://schemas.openxmlformats.org/drawingml/2006/chart"
	drawingGraphicDataURIChartStrict         = "http://purl.oclc.org/ooxml/drawingml/chart"
	drawingGraphicDataURIDiagramTransitional = "http://schemas.openxmlformats.org/drawingml/2006/diagram"
	drawingGraphicDataURIDiagramStrict       = "http://purl.oclc.org/ooxml/drawingml/diagram"
	drawingGraphicDataURITableTransitional   = "http://schemas.openxmlformats.org/drawingml/2006/table"
	drawingGraphicDataURITableStrict         = "http://purl.oclc.org/ooxml/drawingml/table"
	drawingGraphicDataURIOLETransitional     = "http://schemas.openxmlformats.org/presentationml/2006/ole"
	drawingGraphicDataURIOLEStrict           = "http://purl.oclc.org/ooxml/presentationml/ole"

	relTypeOLEObject = prel.OLEObject
	relTypePackage   = prel.Package
)

func canonicalRelationshipType(value string) string { return prel.CanonicalType(value) }

func relationshipTypeIs(value, expected string) bool { return prel.Is(value, expected) }

// Package contains the typed PresentationML parts resolved from an OOXML package.
type Package struct {
	MainPartName string
	Presentation *Presentation
	// PreservedParts contains the original OPC parts captured when a package is loaded.
	// WritePackage seeds its output from these bytes before overlaying modeled edits so
	// extension parts and other unsupported package content survive a load/save cycle.
	PreservedParts []PreservedPart
	NotesMaster    *NotesMaster
	// Slides is ordered by the slide sequence in the presentation.
	Slides []*SlideEntry
	// SlideMasters contains all loaded slide master parts.
	SlideMasters []*SlideMasterEntry
	// Themes maps part name to parsed theme.
	Themes        map[string]*Theme
	TableStyles   *TableStyleList
	MediaParts    map[string]media.Part
	EmbeddedFonts []*EmbeddedFontData
	// CommentAuthors maps author ID to CommentAuthor, populated from ppt/commentAuthors.xml.
	CommentAuthors map[int]CommentAuthor
	Diagnostics    diag.List
}

// PreservedPart is an opaque OPC part retained for lossless package-level round trips.
// Parsed/edited parts are overwritten by WritePackage; untouched parts are copied verbatim.
type PreservedPart struct {
	Name        string
	ContentType string
	Data        []byte
}

// NotesMaster represents a ppt/notesMasters/notesMasterN.xml part.
// ECMA-376 Part 1 §19.3.
type NotesMaster struct {
	XMLName  xml.Name        `xml:"notesMaster"`
	CSld     CommonSlideData `xml:"cSld"`
	TxStyles *MasterTxStyles `xml:"txStyles"`
}

// SlideEntry pairs a presentation slide relationship with its parsed slide part.
type SlideEntry struct {
	// RelID is the presentation-level relationship ID for this slide.
	RelID string
	// PartName is the resolved OPC part name.
	PartName string
	Slide    *Slide
	// Notes is the notes slide for this slide, if present.
	Notes *NotesSlide
	// Comments contains the comments for this slide, if any.
	Comments []*PptxComment
	// LayoutPartName is the part name of this slide's layout.
	LayoutPartName string
	Layout         *SlideLayout
	// MasterPartName is the OPC part name of the slide layout's master,
	// resolved from the layout's relationships (POI XSLF parity: each slide
	// should use the master linked by its layout, not a global default).
	MasterPartName string
	// MediaParts maps slide-local relationship IDs to image media loaded from this slide.
	MediaParts map[string]media.Part
	// LayoutMediaParts maps layout-local relationship IDs to media loaded from this slide's layout.
	LayoutMediaParts map[string]media.Part
	// SmartArtParts contains SmartArt diagrams embedded in this slide.
	SmartArtParts []*SmartArtPart
	// ChartParts maps chart relationship IDs to chart XML data.
	ChartParts map[string][]byte
	// ChartFrames maps chart relationship IDs to the hosting p:graphicFrame
	// transform, so the chart can be rendered at its real position and size.
	ChartFrames map[string]*Transform2D
	// HlinkURLs maps slide-local hyperlink relationship IDs to their external target URLs.
	HlinkURLs map[string]string
	// OleObjectParts maps oleObj relationship IDs (r:id on p:oleObj) to the
	// embedded object or package payload loaded from ppt/embeddings.
	OleObjectParts map[string]media.Part
	// OleObjectLinks maps linked OLE relationship IDs to external targets.
	// Linked objects use p:link and an External oleObject/package relationship.
	OleObjectLinks map[string]string
}

// CommentAuthor represents a p:cmAuthor from ppt/commentAuthors.xml.
// ECMA-376 Part 1 §19.5.43.
type CommentAuthor struct {
	ID       int    // id attribute
	Name     string // name attribute
	Initials string // initials attribute
}

// PptxComment represents a single p:cm element from a slide comments part.
// ECMA-376 Part 1 §19.5.45.
type PptxComment struct {
	AuthorID int    // authorId attribute — references CommentAuthor.ID
	Index    int    // idx attribute — per-author comment counter
	DateTime string // dt attribute — ISO 8601 date/time string
	Text     string // p:text child element content
	X        int64  // p:pos cx attribute (EMU)
	Y        int64  // p:pos cy attribute (EMU)
}

// SlideMasterEntry pairs a slide master relationship with its parsed content.
type SlideMasterEntry struct {
	RelID    string
	PartName string
	Master   *SlideMaster
	// Layouts contains the slide layouts linked from this master.
	Layouts []*SlideLayout
	// MediaParts maps master-local relationship IDs to media.
	MediaParts map[string]media.Part
}

// Presentation represents ppt/presentation.xml.
// ECMA-376 Part 1 §19.2.
type Presentation struct {
	XMLName           xml.Name           `xml:"presentation"`
	SlideSize         *SlideSize         `xml:"sldSz"`
	NotesSz           *NotesSize         `xml:"notesSz"`
	SlideMasterIDs    []IDRef            `xml:"sldMasterIdLst>sldMasterId"`
	SlideIDs          []IDRef            `xml:"sldIdLst>sldId"`
	Sections          []Section          `xml:"sectionLst>section"`
	EmbeddedFontDecls []EmbeddedFontDecl `xml:"embeddedFontLst>embeddedFont"`
	DefaultTextStyle  *DMLTextListStyle  `xml:"defaultTextStyle"`
}

// SlideSize holds the slide canvas dimensions in EMU.
type SlideSize struct {
	CX   int64  `xml:"cx,attr"`
	CY   int64  `xml:"cy,attr"`
	Type string `xml:"type,attr,omitempty"`
}

// NotesSize holds the notes page dimensions in EMU.
type NotesSize struct {
	CX int64 `xml:"cx,attr"`
	CY int64 `xml:"cy,attr"`
}

// IDRef carries the numeric id and r:id relationship reference from sldId/sldMasterId/sldLayoutId elements.
type IDRef struct {
	ID    int    `xml:"http://schemas.openxmlformats.org/presentationml/2006/main id,attr"`
	RelID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
}

// Section represents a p:section in ppt/presentation.xml.
// ECMA-376 Part 1 §19.2.22.
type Section struct {
	Name     string           `xml:"name,attr"`
	SlideIDs []SectionSlideID `xml:"sldIdLst>sldId"`
}

// SectionSlideID carries the numeric slide id inside a section's sldIdLst.
type SectionSlideID struct {
	ID int `xml:"id,attr"`
}

// EmbeddedFontDecl describes a presentation-level embedded font family and the
// relationship IDs for its available styles.
type EmbeddedFontDecl struct {
	Font       FontRef               `xml:"font"`
	Regular    *EmbeddedFontStyleRef `xml:"regular"`
	Bold       *EmbeddedFontStyleRef `xml:"bold"`
	Italic     *EmbeddedFontStyleRef `xml:"italic"`
	BoldItalic *EmbeddedFontStyleRef `xml:"boldItalic"`
}

// EmbeddedFontStyleRef points to an embedded font part via a relationship ID.
type EmbeddedFontStyleRef struct {
	RelID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
	// FontKey is the OOXML fontKey GUID needed to undo ODTTF obfuscation.
	FontKey string `xml:"fontKey,attr,omitempty"`
}

type EmbeddedFontData = fontdata.EmbeddedFontData
type FontPart = fontdata.FontPart

// SlideTransition holds parsed p:transition metadata for a slide.
// ECMA-376 Part 1 §19.3.1.44.
type SlideTransition struct {
	// Type is the transition effect name derived from the child element local name
	// (e.g. "fade", "push", "wipe", "blinds", "fly", "zoom", "split", "wheel",
	// "strips", "cut", "dissolve", "checker", "randomBar", "plus", "wedge",
	// "circle", "diamond"). Empty string means no specific effect was set.
	Type string
	// Spd is the transition speed: "slow", "med", or "fast". Empty means default (fast).
	Spd string
	// AdvClick is true when the slide advances on a mouse click (default true).
	AdvClick bool
	// AdvTmMs is the auto-advance time in milliseconds. 0 means no auto-advance.
	AdvTmMs int
	// Dir is the transition direction (e.g. "l", "r", "u", "d", "lu", "ru", etc.)
	// stored as an attribute on the transition type child element.
	Dir string
	// Dur is the transition duration in milliseconds, stored as a `dur` attribute
	// on the transition type child element. 0 means not specified.
	Dur int
	// Sound is the relationship ID of a sound embedded for this transition.
	Sound string
}

// UnmarshalXML decodes a p:transition element.
func (t *SlideTransition) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	*t = SlideTransition{AdvClick: true}
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "spd":
			t.Spd = attr.Value
		case "advClick":
			t.AdvClick = attr.Value != "0"
		case "advTm":
			n, _ := strconv.Atoi(attr.Value)
			t.AdvTmMs = n
		}
	}
	depth := 1
	for depth > 0 {
		inner, err := d.Token()
		if err != nil {
			break
		}
		switch tok := inner.(type) {
		case xml.StartElement:
			if depth == 1 {
				switch tok.Name.Local {
				case "snd":
					for _, attr := range tok.Attr {
						if attr.Name.Local == "embed" {
							t.Sound = attr.Value
						}
					}
				default:
					if t.Type == "" {
						t.Type = tok.Name.Local
						for _, attr := range tok.Attr {
							switch attr.Name.Local {
							case "dir":
								t.Dir = attr.Value
							case "dur":
								n, _ := strconv.Atoi(attr.Value)
								t.Dur = n
							}
						}
					}
				}
			}
			depth++
		case xml.EndElement:
			depth--
		}
	}
	return nil
}

// MarshalXML encodes a p:transition element with its type child.
func (t SlideTransition) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if t.Spd != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "spd"}, Value: t.Spd})
	}
	if !t.AdvClick {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "advClick"}, Value: "0"})
	}
	if t.AdvTmMs > 0 {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "advTm"}, Value: strconv.Itoa(t.AdvTmMs)})
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if t.Type != "" {
		typeElem := xml.StartElement{Name: xml.Name{Local: t.Type}}
		if t.Dir != "" {
			typeElem.Attr = append(typeElem.Attr, xml.Attr{Name: xml.Name{Local: "dir"}, Value: t.Dir})
		}
		if t.Dur > 0 {
			typeElem.Attr = append(typeElem.Attr, xml.Attr{Name: xml.Name{Local: "dur"}, Value: strconv.Itoa(t.Dur)})
		}
		if err := e.EncodeToken(typeElem); err != nil {
			return err
		}
		if err := e.EncodeToken(xml.EndElement{Name: typeElem.Name}); err != nil {
			return err
		}
	}
	if t.Sound != "" {
		sndElem := xml.StartElement{Name: xml.Name{Local: "snd"}}
		sndElem.Attr = append(sndElem.Attr, xml.Attr{Name: xml.Name{Local: "embed"}, Value: t.Sound})
		if err := e.EncodeToken(sndElem); err != nil {
			return err
		}
		if err := e.EncodeToken(xml.EndElement{Name: sndElem.Name}); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// Slide represents a ppt/slides/slideN.xml part.
// ECMA-376 Part 1 §19.3.
type Slide struct {
	XMLName          xml.Name        `xml:"sld"`
	CSld             CommonSlideData `xml:"cSld"`
	ClrMapOvr        *ColorMapOverride
	ThemeOverride    *ThemeOverride `xml:"-"`
	Show             *bool          `xml:"show,attr"`
	ShowMasterSp     *bool          `xml:"showMasterSp,attr"`
	ShowMasterPhAnim *bool          `xml:"showMasterPhAnim,attr"`
	Transition       *SlideTransition
	// ExtLstRawXML preserves the top-level p:extLst payload verbatim for round-trip fidelity.
	ExtLstRawXML []byte
	// AnimSteps maps shape ID (from sp/@id) to a 1-based click-step number.
	// Shapes not present in the map have no entrance animation (always visible).
	AnimSteps map[int]int
	// EmphasisEffects maps shape ID to emphasis animation effect names
	// extracted from p:timing nodeType="emphEffect" nodes.
	EmphasisEffects map[int][]string
	// MotionPaths maps shape ID to motion path data extracted from
	// p:animMotion elements inside the timing tree.
	MotionPaths map[int]AnimMotionPath
	// AnimationTriggers maps shape ID to trigger event names
	// (e.g. "onClick", "onHover") extracted from p:cond@evt.
	AnimationTriggers map[int][]string
	// AnimationEvents preserves scheduler-relevant leaf timing semantics.
	AnimationEvents []AnimationEvent
	// TimingRawXML holds the verbatim p:timing element bytes for round-trip
	// write-back. When non-empty it takes precedence over the flat animation
	// maps during marshaling.
	TimingRawXML []byte
}

// AnimMotionPath stores a motion path animation for a shape.
type AnimMotionPath struct {
	Path   string // SVG-like path data (e.g. "M 0 0 L 100 100")
	Dur    int    // duration in milliseconds (0 = not specified)
	Origin string // "parent" or "layout"
}

// AnimationEvent is a resolved leaf event from the PowerPoint timing tree.
// Raw timing XML remains authoritative for round-trip; this model is used by
// renderers that need scheduling semantics without reparsing XML.
type AnimationEvent struct {
	ShapeID      int
	NodeType     string // clickEffect, withEffect, afterEffect, emphEffect
	Behavior     string // animEffect, animMotion, animRot, animScale, animColor, anim, set
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

// UnmarshalXML implements xml.Unmarshaler so that p:transition and p:timing
// siblings of p:cSld are captured in addition to the standard fields.
func (s *Slide) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		b := attr.Value != "0" && attr.Value != "false"
		switch attr.Name.Local {
		case "show":
			s.Show = &b
		case "showMasterSp":
			s.ShowMasterSp = &b
		case "showMasterPhAnim":
			s.ShowMasterPhAnim = &b
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		switch se := tok.(type) {
		case xml.StartElement:
			switch se.Name.Local {
			case "clrMapOvr":
				var override ColorMapOverride
				if err := d.DecodeElement(&override, &se); err != nil {
					return err
				}
				s.ClrMapOvr = &override
			case "cSld":
				if err := d.DecodeElement(&s.CSld, &se); err != nil {
					return err
				}
			case "transition":
				tr := parseSlideTransition(d, se)
				s.Transition = tr
			case "timing":
				raw, err := captureRawXMLElementPPTX(d, se)
				if err == nil {
					s.TimingRawXML = raw
					// Also parse flat maps from the captured bytes.
					rawDec := xml.NewDecoder(bytes.NewReader(raw))
					// First token is the start element we already have.
					steps, emphasis, motions, triggers, err := parseTimingAnimSteps(rawDec, se)
					if err == nil {
						if len(steps) > 0 {
							s.AnimSteps = steps
						}
						if len(emphasis) > 0 {
							s.EmphasisEffects = emphasis
						}
						if len(motions) > 0 {
							s.MotionPaths = motions
						}
						if len(triggers) > 0 {
							s.AnimationTriggers = triggers
						}
					}
					s.AnimationEvents = parseTimingAnimationEvents(raw)
				}
			case "extLst":
				raw, err := captureRawXMLElementPPTX(d, se)
				if err != nil {
					return err
				}
				s.ExtLstRawXML = raw
			default:
				if err := d.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if se.Name == start.Name {
				return nil
			}
		}
	}
}

// parseSlideTransition reads a p:transition element and returns a SlideTransition.
func parseSlideTransition(d *xml.Decoder, start xml.StartElement) *SlideTransition {
	tr := &SlideTransition{AdvClick: true} // advance-on-click defaults to true
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "spd":
			tr.Spd = attr.Value
		case "advClick":
			tr.AdvClick = attr.Value != "0"
		case "advTm":
			n, _ := strconv.Atoi(attr.Value)
			tr.AdvTmMs = n
		}
	}
	// First child element name = transition type (e.g. "fade", "push", "wipe").
	depth := 1
	for depth > 0 {
		inner, err := d.Token()
		if err != nil {
			break
		}
		switch t := inner.(type) {
		case xml.StartElement:
			if depth == 1 {
				switch t.Name.Local {
				case "snd":
					for _, attr := range t.Attr {
						if attr.Name.Local == "embed" {
							tr.Sound = attr.Value
						}
					}
				default:
					if tr.Type == "" {
						tr.Type = t.Name.Local
						for _, attr := range t.Attr {
							switch attr.Name.Local {
							case "dir":
								tr.Dir = attr.Value
							case "dur":
								n, _ := strconv.Atoi(attr.Value)
								tr.Dur = n
							}
						}
					}
				}
			}
			depth++
		case xml.EndElement:
			depth--
		}
	}
	return tr
}

// parseTimingAnimSteps scans a p:timing element and returns:
//  1. a map of shape ID → 1-based click-step for entrance animations
//  2. a map of shape ID → emphasis animation effect names
//  3. a map of shape ID → motion path data
//  4. a map of shape ID → trigger event names
//
// Each p:cTn with nodeType="clickEffect" increments the step counter; shapes
// sharing the same grpId fire in the same step. nodeType="emphEffect" nodes
// are tracked separately and their child animation element names (animScale,
// animRot, animMotion, anim, set) are used to infer the effect type.
// p:animMotion elements anywhere in the timing tree are captured as motion
// paths keyed by their target shape. p:cond@evt attributes are captured as
// trigger events.
func parseTimingAnimSteps(d *xml.Decoder, start xml.StartElement) (map[int]int, map[int][]string, map[int]AnimMotionPath, map[int][]string, error) {
	steps := map[int]int{}
	emphasis := map[int][]string{}
	motions := map[int]AnimMotionPath{}
	triggers := map[int][]string{}
	currentStep := 0
	pendingStep := 0
	grpToStep := map[int]int{}
	inEmph := false
	emphDepth := 0
	emphShapeID := 0
	emphEffect := ""
	motionShapeID := 0
	motionPath := AnimMotionPath{}
	triggerEvent := ""
	triggerShapeID := 0
	inTriggerCond := false

	for {
		tok, err := d.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, nil, nil, nil, err
		}
		switch se := tok.(type) {
		case xml.StartElement:
			if se.Name.Local == "cTn" {
				var nodeType string
				grpID := -1
				for _, attr := range se.Attr {
					switch attr.Name.Local {
					case "nodeType":
						nodeType = attr.Value
					case "grpId":
						n, _ := strconv.Atoi(attr.Value)
						grpID = n
					}
				}
				switch nodeType {
				case "clickEffect":
					if inEmph {
						inEmph = false
						emphDepth = 0
						emphShapeID = 0
						emphEffect = ""
					}
					if grpID >= 0 {
						// Treat the group ID as a 0-based step index so that
						// round-tripped step numbers survive save/load cycles.
						pendingStep = grpID + 1
						grpToStep[grpID] = pendingStep
					} else {
						currentStep++
						pendingStep = currentStep
					}
				case "withEffect", "afterEffect":
					if inEmph {
						inEmph = false
						emphDepth = 0
						emphShapeID = 0
						emphEffect = ""
					}
					// Fires alongside or after the current click step.
					if pendingStep == 0 {
						pendingStep = currentStep
					}
				case "emphEffect":
					inEmph = true
					emphDepth = 1
					emphShapeID = 0
					emphEffect = ""
				default:
					if inEmph {
						emphDepth++
					}
				}
			}
			if se.Name.Local == "spTgt" && pendingStep > 0 && !inEmph && !inTriggerCond {
				for _, attr := range se.Attr {
					if attr.Name.Local == "spid" {
						id, _ := strconv.Atoi(attr.Value)
						if id > 0 {
							if _, exists := steps[id]; !exists {
								steps[id] = pendingStep
							}
						}
						break
					}
				}
			}
			if se.Name.Local == "spTgt" && inEmph {
				for _, attr := range se.Attr {
					if attr.Name.Local == "spid" {
						emphShapeID, _ = strconv.Atoi(attr.Value)
						break
					}
				}
			}
			// Infer emphasis effect type from child animation element.
			if inEmph && emphEffect == "" {
				switch se.Name.Local {
				case "animScale":
					emphEffect = "growShrink"
				case "animRot":
					emphEffect = "spin"
				case "animMotion":
					emphEffect = "motion"
				case "anim":
					emphEffect = "animate"
				case "set":
					emphEffect = "set"
				}
			}
			// Capture motion path data from animMotion elements.
			if se.Name.Local == "animMotion" {
				for _, attr := range se.Attr {
					switch attr.Name.Local {
					case "path":
						motionPath.Path = attr.Value
					case "dur":
						motionPath.Dur = parseAnimDuration(attr.Value)
					case "origin":
						motionPath.Origin = attr.Value
					}
				}
			}
			if se.Name.Local == "spTgt" && motionPath.Path != "" && motionShapeID == 0 {
				for _, attr := range se.Attr {
					if attr.Name.Local == "spid" {
						motionShapeID, _ = strconv.Atoi(attr.Value)
						break
					}
				}
			}
			// Capture trigger events from cond elements.
			if se.Name.Local == "cond" {
				for _, attr := range se.Attr {
					if attr.Name.Local == "evt" {
						triggerEvent = attr.Value
						inTriggerCond = true
						break
					}
				}
			}
			if se.Name.Local == "spTgt" && triggerEvent != "" && triggerShapeID == 0 {
				for _, attr := range se.Attr {
					if attr.Name.Local == "spid" {
						triggerShapeID, _ = strconv.Atoi(attr.Value)
						break
					}
				}
			}
		case xml.EndElement:
			if se.Name == start.Name {
				return steps, emphasis, motions, triggers, nil
			}
			// When leaving an emphEffect cTn container, commit the collected effect.
			if se.Name.Local == "cTn" && inEmph {
				emphDepth--
				if emphDepth == 0 {
					if emphShapeID > 0 && emphEffect != "" {
						emphasis[emphShapeID] = append(emphasis[emphShapeID], emphEffect)
					}
					inEmph = false
					emphShapeID = 0
					emphEffect = ""
				}
			}
			// When leaving an animMotion, commit the motion path.
			if se.Name.Local == "animMotion" {
				if motionShapeID > 0 && motionPath.Path != "" {
					motions[motionShapeID] = motionPath
				}
				motionShapeID = 0
				motionPath = AnimMotionPath{}
			}
			// When leaving a cond, commit the trigger.
			if se.Name.Local == "cond" {
				if triggerShapeID > 0 && triggerEvent != "" {
					triggers[triggerShapeID] = append(triggers[triggerShapeID], triggerEvent)
				}
				triggerEvent = ""
				triggerShapeID = 0
				inTriggerCond = false
			}
		}
	}
	return steps, emphasis, motions, triggers, nil
}

// parseTimingAnimationEvents extracts scheduler-relevant timing leaves while
// leaving TimingRawXML as the lossless source of truth.
func parseTimingAnimationEvents(raw []byte) []AnimationEvent {
	type ctx struct {
		event             AnimationEvent
		depth             int
		inStartConditions int
	}
	dec := xml.NewDecoder(bytes.NewReader(raw))
	var stack []*ctx
	var events []AnimationEvent
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			for _, c := range stack {
				c.depth++
			}
			if t.Name.Local == "cTn" {
				e := AnimationEvent{RepeatCount: 1}
				for _, a := range t.Attr {
					switch a.Name.Local {
					case "nodeType":
						e.NodeType = a.Value
					case "dur":
						e.DurationMs = parseAnimDuration(a.Value)
					case "repeatCount":
						if a.Value == "indefinite" {
							e.RepeatCount = -1
						} else if v, er := strconv.ParseFloat(strings.TrimSuffix(a.Value, "%"), 64); er == nil {
							if strings.HasSuffix(a.Value, "%") {
								v /= 100
							}
							e.RepeatCount = v
						}
					case "repeatDur":
						e.RepeatDurMs = parseAnimDuration(a.Value)
					case "autoRev":
						e.AutoReverse = a.Value == "1" || strings.EqualFold(a.Value, "true")
					case "fill":
						e.Fill = a.Value
					}
				}
				if e.NodeType == "clickEffect" || e.NodeType == "withEffect" || e.NodeType == "afterEffect" || e.NodeType == "emphEffect" {
					stack = append(stack, &ctx{event: e})
				}
			}
			if len(stack) == 0 {
				continue
			}
			c := stack[len(stack)-1]
			switch t.Name.Local {
			case "stCondLst":
				c.inStartConditions++
			case "cond":
				if c.inStartConditions > 0 {
					for _, a := range t.Attr {
						switch a.Name.Local {
						case "delay":
							c.event.DelayMs = parseAnimDuration(a.Value)
						case "evt":
							c.event.Trigger = a.Value
						}
					}
				}
			case "animEffect", "animMotion", "animRot", "animScale", "animColor", "anim", "set":
				if c.event.Behavior == "" {
					c.event.Behavior = t.Name.Local
				}
				for _, a := range t.Attr {
					switch a.Name.Local {
					case "filter":
						c.event.Effect = a.Value
					case "path":
						c.event.MotionPath = a.Value
					case "origin":
						c.event.MotionOrigin = a.Value
					}
				}
			case "spTgt":
				for _, a := range t.Attr {
					if a.Name.Local == "spid" {
						c.event.ShapeID, _ = strconv.Atoi(a.Value)
					}
				}
			}
		case xml.EndElement:
			if len(stack) == 0 {
				continue
			}
			c := stack[len(stack)-1]
			if t.Name.Local == "stCondLst" && c.inStartConditions > 0 {
				c.inStartConditions--
			}
			c.depth--
			if t.Name.Local == "cTn" && c.depth <= 0 {
				if c.event.ShapeID > 0 {
					if c.event.Trigger == "" {
						switch c.event.NodeType {
						case "clickEffect":
							c.event.Trigger = "onClick"
						case "withEffect":
							c.event.Trigger = "withPrevious"
						case "afterEffect":
							c.event.Trigger = "afterPrevious"
						}
					}
					events = append(events, c.event)
				}
				stack = stack[:len(stack)-1]
			}
		}
	}
	return events
}

// parseAnimDuration converts an animation duration string to milliseconds.
// Supports "indefinite" (0), numeric milliseconds, and ISO 8601 durations.
func parseAnimDuration(v string) int {
	v = strings.TrimSpace(v)
	if v == "indefinite" || v == "" {
		return 0
	}
	// Handle simple millisecond values
	if n, err := strconv.Atoi(v); err == nil {
		return n
	}
	// Handle ISO 8601 duration like "PT0.5S" or "PT500MS"
	v = strings.ToUpper(v)
	if strings.HasPrefix(v, "PT") {
		v = v[2:]
	}
	if strings.HasSuffix(v, "S") {
		v = v[:len(v)-1]
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return int(f * 1000)
		}
	}
	if strings.HasSuffix(v, "MS") {
		v = v[:len(v)-2]
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return 0
}

// NotesSlide represents a ppt/notesslides/notesslideN.xml part.
// ECMA-376 Part 1 §19.3.
type NotesSlide struct {
	XMLName       xml.Name        `xml:"notes"`
	CSld          CommonSlideData `xml:"cSld"`
	ThemeOverride *ThemeOverride  `xml:"-"`
}

// CommentAuthorList is the parsed form of ppt/commentAuthors.xml.
// ECMA-376 Part 1 §19.5.43.
type CommentAuthorList struct {
	XMLName xml.Name      `xml:"cmAuthorLst"`
	Authors []xmlCmAuthor `xml:"cmAuthor"`
}

type xmlCmAuthor struct {
	ID       int    `xml:"id,attr"`
	Name     string `xml:"name,attr"`
	Initials string `xml:"initials,attr"`
}

// CommentList is the parsed form of a per-slide ppt/comments/commentsN.xml.
// ECMA-376 Part 1 §19.5.45.
type CommentList struct {
	XMLName  xml.Name `xml:"cmLst"`
	Comments []xmlCm  `xml:"cm"`
}

type xmlCm struct {
	AuthorID int       `xml:"authorId,attr"`
	Index    int       `xml:"idx,attr"`
	DateTime string    `xml:"dt,attr"`
	Pos      *xmlCmPos `xml:"pos"`
	Text     string    `xml:"text"`
}

type xmlCmPos struct {
	X int64 `xml:"x,attr"`
	Y int64 `xml:"y,attr"`
}

// SlideMaster represents a ppt/slidemasters/slidemasterN.xml part.
type SlideMaster struct {
	XMLName      xml.Name        `xml:"sldMaster"`
	ClrMap       *ColorMap       `xml:"clrMap"`
	CSld         CommonSlideData `xml:"cSld"`
	HF           *HeaderFooter   `xml:"hf"`
	SlideLayouts []IDRef         `xml:"sldLayoutIdLst>sldLayoutId"`
	TxStyles     *MasterTxStyles `xml:"txStyles"`
}

// HeaderFooter stores slide-master placeholder enablement flags.
type HeaderFooter struct {
	DateTime    *bool `xml:"dt,attr,omitempty"`
	Header      *bool `xml:"hdr,attr,omitempty"`
	Footer      *bool `xml:"ftr,attr,omitempty"`
	SlideNumber *bool `xml:"sldNum,attr,omitempty"`
}

// ColorMap represents p:clrMap / a:overrideClrMapping theme-slot remapping.
type ColorMap struct {
	Bg1           string `xml:"bg1,attr,omitempty"`
	Tx1           string `xml:"tx1,attr,omitempty"`
	Bg2           string `xml:"bg2,attr,omitempty"`
	Tx2           string `xml:"tx2,attr,omitempty"`
	Accent1       string `xml:"accent1,attr,omitempty"`
	Accent2       string `xml:"accent2,attr,omitempty"`
	Accent3       string `xml:"accent3,attr,omitempty"`
	Accent4       string `xml:"accent4,attr,omitempty"`
	Accent5       string `xml:"accent5,attr,omitempty"`
	Accent6       string `xml:"accent6,attr,omitempty"`
	Hlink         string `xml:"hlink,attr,omitempty"`
	FollowedHlink string `xml:"followedHlink,attr,omitempty"`
}

// MasterColorMapping represents a:masterClrMapping.
type MasterColorMapping struct {
	XMLName xml.Name `xml:"http://schemas.openxmlformats.org/drawingml/2006/main masterClrMapping"`
}

// OverrideColorMapping represents a:overrideClrMapping.
type OverrideColorMapping struct {
	XMLName xml.Name `xml:"http://schemas.openxmlformats.org/drawingml/2006/main overrideClrMapping"`
	ColorMap
}

// ColorMapOverride represents p:clrMapOvr.
type ColorMapOverride struct {
	XMLName            xml.Name              `xml:"clrMapOvr"`
	MasterClrMapping   *MasterColorMapping   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main masterClrMapping"`
	OverrideClrMapping *OverrideColorMapping `xml:"http://schemas.openxmlformats.org/drawingml/2006/main overrideClrMapping"`
}

// MasterTxStyles holds the master-level title/body/other text styles.
type MasterTxStyles struct {
	TitleStyle *DMLTextListStyle `xml:"titleStyle"`
	BodyStyle  *DMLTextListStyle `xml:"bodyStyle"`
	OtherStyle *DMLTextListStyle `xml:"otherStyle"`
}

// SlideLayout represents a ppt/slidelayouts/slidelayoutN.xml part.
type SlideLayout struct {
	XMLName       xml.Name          `xml:"sldLayout"`
	ClrMapOvr     *ColorMapOverride `xml:"clrMapOvr"`
	ThemeOverride *ThemeOverride    `xml:"-"`
	CSld          CommonSlideData   `xml:"cSld"`
	Type          string            `xml:"type,attr,omitempty"`
	MatchingName  string            `xml:"matchingName,attr,omitempty"`
	// PartName is the OPC part name populated during loading.
	PartName string `xml:"-"`
	// RelID is the presentation-level relationship ID assigned during write-back.
	RelID string `xml:"-"`
}

// CommonSlideData is the shared content structure across slide, master, and layout.
type CommonSlideData struct {
	Name   string    `xml:"name,attr,omitempty"`
	Bg     *SlideBg  `xml:"bg"`
	SpTree ShapeTree `xml:"spTree"`
}

// SlideBg holds the background fill specification for a slide.
type SlideBg struct {
	BgPrNoFill   *struct{}             `xml:"bgPr>noFill"`
	BgPrSrgb     *drawingxml.SRGBClr   `xml:"bgPr>solidFill>srgbClr"`
	BgPrScheme   *drawingxml.SchemeClr `xml:"bgPr>solidFill>schemeClr"`
	BgPrGradFill *GradFill             `xml:"bgPr>gradFill"`
	BgPrPattFill *PattFill             `xml:"bgPr>pattFill"`
	BgPrBlipFill *BlipFill             `xml:"bgPr>blipFill"`
	BgRef        *BgRefSpec            `xml:"bgRef"`
}

// BgRefSpec stores a p:bgRef theme reference and its optional color override.
type BgRefSpec struct {
	Idx    int                   `xml:"idx,attr"`
	Srgb   *drawingxml.SRGBClr   `xml:"srgbClr"`
	Scheme *drawingxml.SchemeClr `xml:"schemeClr"`
}

// ShapeTree is a container of shapes on a slide.
type ShapeTree struct {
	Shapes          []Shape          `xml:"-"`
	Pictures        []Picture        `xml:"-"`
	GroupShapes     []GroupShape     `xml:"-"`
	Connectors      []Connector      `xml:"-"`
	Tables          []TableFrame     `xml:"-"`
	GraphicFrames   []GraphicFrame   `xml:"-"`
	SmartArtFrames  []SmartArtFrame  `xml:"-"`
	OleObjectFrames []OleObjectFrame `xml:"-"`
	Items           []ShapeTreeItem  `xml:"-"`
	Diagnostics     diag.List        `xml:"-"`
}

// ShapeTreeItemKind identifies the XML child type stored in a ShapeTree.
type ShapeTreeItemKind string

const (
	ShapeTreeItemShape        ShapeTreeItemKind = "sp"
	ShapeTreeItemPicture      ShapeTreeItemKind = "pic"
	ShapeTreeItemGroup        ShapeTreeItemKind = "grpSp"
	ShapeTreeItemConnector    ShapeTreeItemKind = "cxnSp"
	ShapeTreeItemTable        ShapeTreeItemKind = "tbl"
	ShapeTreeItemGraphicFrame ShapeTreeItemKind = "graphicFrame"
	ShapeTreeItemSmartArt     ShapeTreeItemKind = "smartArt"
	ShapeTreeItemOleObject    ShapeTreeItemKind = "oleObject"
)

// ShapeTreeItem points to a child element in one of the ShapeTree collections.
type ShapeTreeItem struct {
	Kind  ShapeTreeItemKind
	Index int
}

// alternateContent captures shapes inside mc:AlternateContent for parsing.
type alternateContent struct {
	Choice   []alternateChoice `xml:"http://schemas.openxmlformats.org/markup-compatibility/2006 Choice"`
	Fallback alternateFallback `xml:"http://schemas.openxmlformats.org/markup-compatibility/2006 Fallback"`
}

type alternateChoice struct {
	Requires string       `xml:"Requires,attr"`
	Shapes   []Shape      `xml:"sp"`
	Pics     []Picture    `xml:"pic"`
	GrpSp    []GroupShape `xml:"grpSp"`
	CxnSp    []Connector  `xml:"cxnSp"`
}

type alternateFallback struct {
	Shapes []Shape      `xml:"sp"`
	Pics   []Picture    `xml:"pic"`
	GrpSp  []GroupShape `xml:"grpSp"`
	CxnSp  []Connector  `xml:"cxnSp"`
}

func appendAlternateContentItems(t *ShapeTree, shapes []Shape, pics []Picture, groups []GroupShape, connectors []Connector) {
	for _, sp := range shapes {
		idx := len(t.Shapes)
		t.Shapes = append(t.Shapes, sp)
		t.Items = append(t.Items, ShapeTreeItem{Kind: ShapeTreeItemShape, Index: idx})
	}
	for _, pic := range pics {
		idx := len(t.Pictures)
		t.Pictures = append(t.Pictures, pic)
		t.Items = append(t.Items, ShapeTreeItem{Kind: ShapeTreeItemPicture, Index: idx})
	}
	for _, grp := range groups {
		idx := len(t.GroupShapes)
		t.GroupShapes = append(t.GroupShapes, grp)
		t.Items = append(t.Items, ShapeTreeItem{Kind: ShapeTreeItemGroup, Index: idx})
	}
	for _, cx := range connectors {
		idx := len(t.Connectors)
		t.Connectors = append(t.Connectors, cx)
		t.Items = append(t.Items, ShapeTreeItem{Kind: ShapeTreeItemConnector, Index: idx})
	}
}

func alternateChoiceRequiresUnsupportedExtension(requires string) bool {
	for _, token := range strings.Fields(strings.ToLower(strings.TrimSpace(requires))) {
		switch token {
		case "a14", "a15", "a16", "a16cex", "p14", "p15", "p16", "thm15":
			return true
		}
	}
	return false
}

// TableFrame is a parsed graphicFrame-backed table with frame geometry.
type TableFrame struct {
	ID    int
	Name  string
	Desc  string
	Xfrm  *Transform2D
	Table *PresentationTable
}

// GraphicFrame is a graphicFrame element for charts and other graphic objects.
type GraphicFrame struct {
	CNvPr             CnvPr
	CNvGraphicFramePr CnvGraphicFramePr
	NvPr              NvPr
	Xfrm              *Transform2D
	Graphic           Graphic
}

// SmartArtFrame preserves a raw p:graphicFrame that hosts a SmartArt diagram.
// The raw XML is re-emitted on write-back so that diagrams are not lost when
// a loaded deck is saved again.
type SmartArtFrame struct {
	ID     int
	Name   string
	Desc   string
	Xfrm   *Transform2D
	RelIDs map[string]string // dm, lo, qs, cs slide relationship IDs
	RawXML []byte
}

// OleObjectFrame holds a parsed p:graphicFrame that hosts an embedded OLE object.
type OleObjectFrame struct {
	ID         int
	Name       string
	Desc       string
	Xfrm       *Transform2D
	RelID      string // r:id of the embed/link relationship
	Embed      bool   // true if <p:embed>, false if <p:link>
	ShowAsIcon bool
	ProgID     string
	ObjectName string
	ImgW       int64
	ImgH       int64
	IconRelID  string // r:embed of the fallback icon image, if any
	RawXML     []byte // the full p:graphicFrame element for round-trip support
}

// parseOleObjectFrame extracts the first p:oleObj from a raw p:graphicFrame
// element. It returns nil if no oleObj child is present.
func parseOleObjectFrame(cnvpr CnvPr, xfrm *Transform2D, raw []byte) *OleObjectFrame {
	type olePicBlip struct {
		Embed string `xml:"embed,attr"`
	}
	type olePic struct {
		BlipFill struct {
			Blip olePicBlip `xml:"blip"`
		} `xml:"blipFill"`
	}
	type oleObj struct {
		RelID      string    `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
		Name       string    `xml:"name,attr"`
		ShowAsIcon string    `xml:"showAsIcon,attr"`
		ProgID     string    `xml:"progId,attr"`
		ImgW       int64     `xml:"imgW,attr"`
		ImgH       int64     `xml:"imgH,attr"`
		Embed      *struct{} `xml:"embed"`
		Link       *struct{} `xml:"link"`
		Pic        olePic    `xml:"pic"`
	}

	dec := xml.NewDecoder(bytes.NewReader(raw))
	var found *oleObj
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local != "oleObj" {
			continue
		}
		var obj oleObj
		if err := dec.DecodeElement(&obj, &start); err != nil {
			continue
		}
		if found == nil {
			found = &obj
			continue
		}
		// Prefer a fallback oleObj that carries an icon image.
		if obj.Pic.BlipFill.Blip.Embed != "" {
			found = &obj
		}
	}
	if found == nil || found.RelID == "" {
		return nil
	}

	showAsIcon := false
	switch strings.ToLower(strings.TrimSpace(found.ShowAsIcon)) {
	case "1", "true":
		showAsIcon = true
	}

	return &OleObjectFrame{
		ID:         cnvpr.ID,
		Name:       cnvpr.Name,
		Desc:       cnvpr.Desc,
		Xfrm:       xfrm,
		RelID:      found.RelID,
		Embed:      found.Embed != nil,
		ShowAsIcon: showAsIcon,
		ProgID:     found.ProgID,
		ObjectName: found.Name,
		ImgW:       found.ImgW,
		ImgH:       found.ImgH,
		IconRelID:  found.Pic.BlipFill.Blip.Embed,
		RawXML:     raw,
	}
}

// parseSmartArtRelIDs extracts the dgm:relIds relationship attributes from a
// raw p:graphicFrame element.
func parseSmartArtRelIDs(raw []byte) map[string]string {
	relIDs := map[string]string{}
	dec := xml.NewDecoder(bytes.NewReader(raw))
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local == "relIds" {
			for _, attr := range start.Attr {
				if attr.Name.Space != "" && attr.Name.Space != "http://schemas.openxmlformats.org/officeDocument/2006/relationships" {
					continue
				}
				switch attr.Name.Local {
				case "dm", "lo", "qs", "cs":
					relIDs[attr.Name.Local] = attr.Value
				}
			}
			break
		}
	}
	return relIDs
}

// CnvGraphicFramePr is p:cNvGraphicFramePr.
type CnvGraphicFramePr struct {
	GraphicFrameLocks *GraphicFrameLocks `xml:"a:graphicFrameLocks"`
}

// GraphicFrameLocks is a:graphicFrameLocks.
type GraphicFrameLocks struct {
	NoGrp bool `xml:"noGrp,attr,omitempty"`
}

// NvPr is p:nvPr.
type NvPr struct{}

// Graphic is a:graphic.
type Graphic struct {
	GraphicData GraphicData `xml:"a:graphicData"`
}

// GraphicData is a:graphicData.
type GraphicData struct {
	URI   string    `xml:"uri,attr"`
	Chart *ChartRef `xml:"c:chart"`
}

// ChartRef is c:chart with r:id.
type ChartRef struct {
	RelID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
}

// UnmarshalXML preserves the original child order from p:spTree so z-order can
// be reproduced during rendering.
func (t *ShapeTree) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	*t = ShapeTree{}
	for {
		tok, err := d.Token()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		switch se := tok.(type) {
		case xml.StartElement:
			switch se.Name.Local {
			case "sp":
				idx := len(t.Shapes)
				var shape Shape
				if err := d.DecodeElement(&shape, &se); err != nil {
					return err
				}
				t.Shapes = append(t.Shapes, shape)
				t.Items = append(t.Items, ShapeTreeItem{Kind: ShapeTreeItemShape, Index: idx})
			case "pic":
				idx := len(t.Pictures)
				var pic Picture
				if err := d.DecodeElement(&pic, &se); err != nil {
					return err
				}
				t.Pictures = append(t.Pictures, pic)
				t.Items = append(t.Items, ShapeTreeItem{Kind: ShapeTreeItemPicture, Index: idx})
			case "grpSp":
				idx := len(t.GroupShapes)
				var grp GroupShape
				if err := d.DecodeElement(&grp, &se); err != nil {
					return err
				}
				t.GroupShapes = append(t.GroupShapes, grp)
				t.Items = append(t.Items, ShapeTreeItem{Kind: ShapeTreeItemGroup, Index: idx})
			case "cxnSp":
				idx := len(t.Connectors)
				var cx Connector
				if err := d.DecodeElement(&cx, &se); err != nil {
					return err
				}
				t.Connectors = append(t.Connectors, cx)
				t.Items = append(t.Items, ShapeTreeItem{Kind: ShapeTreeItemConnector, Index: idx})
			case "graphicFrame":
				raw, err := captureRawXMLElementPPTX(d, se)
				if err != nil {
					return err
				}
				var frame struct {
					CNvPr   CnvPr        `xml:"nvGraphicFramePr>cNvPr"`
					Xfrm    *Transform2D `xml:"xfrm"`
					Graphic struct {
						GraphicData struct {
							URI   string             `xml:"uri,attr,omitempty"`
							Table *PresentationTable `xml:"tbl"`
							Chart struct {
								RelID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
							} `xml:"chart"`
						} `xml:"graphicData"`
					} `xml:"graphic"`
				}
				if err := xml.Unmarshal(raw, &frame); err != nil {
					return err
				}
				uri := normalizePresentationGraphicDataURI(frame.Graphic.GraphicData.URI)
				if uri == drawingGraphicDataURITableTransitional && frame.Graphic.GraphicData.Table != nil {
					idx := len(t.Tables)
					t.Tables = append(t.Tables, TableFrame{
						ID:    frame.CNvPr.ID,
						Name:  frame.CNvPr.Name,
						Desc:  frame.CNvPr.Desc,
						Xfrm:  frame.Xfrm,
						Table: frame.Graphic.GraphicData.Table,
					})
					t.Items = append(t.Items, ShapeTreeItem{Kind: ShapeTreeItemTable, Index: idx})
					continue
				}
				if uri == drawingGraphicDataURIChartTransitional || uri == drawingGraphicDataURIChartStrict {
					idx := len(t.GraphicFrames)
					t.GraphicFrames = append(t.GraphicFrames, GraphicFrame{
						CNvPr: frame.CNvPr,
						Xfrm:  frame.Xfrm,
						Graphic: Graphic{
							GraphicData: GraphicData{
								URI:   frame.Graphic.GraphicData.URI,
								Chart: &ChartRef{RelID: frame.Graphic.GraphicData.Chart.RelID},
							},
						},
					})
					t.Items = append(t.Items, ShapeTreeItem{Kind: ShapeTreeItemGraphicFrame, Index: idx})
					continue
				}
				if uri == drawingGraphicDataURIDiagramTransitional || uri == drawingGraphicDataURIDiagramStrict {
					idx := len(t.SmartArtFrames)
					t.SmartArtFrames = append(t.SmartArtFrames, SmartArtFrame{
						ID:     frame.CNvPr.ID,
						Name:   frame.CNvPr.Name,
						Desc:   frame.CNvPr.Desc,
						Xfrm:   frame.Xfrm,
						RelIDs: parseSmartArtRelIDs(raw),
						RawXML: raw,
					})
					t.Items = append(t.Items, ShapeTreeItem{Kind: ShapeTreeItemSmartArt, Index: idx})
					continue
				}
				if uri == drawingGraphicDataURIOLETransitional || uri == drawingGraphicDataURIOLEStrict {
					if ole := parseOleObjectFrame(frame.CNvPr, frame.Xfrm, raw); ole != nil {
						idx := len(t.OleObjectFrames)
						t.OleObjectFrames = append(t.OleObjectFrames, *ole)
						t.Items = append(t.Items, ShapeTreeItem{Kind: ShapeTreeItemOleObject, Index: idx})
					}
					continue
				}
			case "AlternateContent":
				var alt alternateContent
				if err := d.DecodeElement(&alt, &se); err != nil {
					return err
				}
				var merged bool
				for _, choice := range alt.Choice {
					if alternateChoiceRequiresUnsupportedExtension(choice.Requires) {
						continue
					}
					appendAlternateContentItems(t, choice.Shapes, choice.Pics, choice.GrpSp, choice.CxnSp)
					merged = true
					break
				}
				if !merged {
					appendAlternateContentItems(t, alt.Fallback.Shapes, alt.Fallback.Pics, alt.Fallback.GrpSp, alt.Fallback.CxnSp)
				}
			default:
				if err := d.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if se.Name == start.Name {
				return nil
			}
		}
	}
}

func normalizePresentationGraphicDataURI(uri string) string {
	uri = strings.TrimSpace(strings.ToLower(uri))
	switch {
	case strings.Contains(uri, "/chart"):
		return drawingGraphicDataURIChartTransitional
	case strings.Contains(uri, "/diagram"):
		return drawingGraphicDataURIDiagramTransitional
	case strings.Contains(uri, "/table"):
		return drawingGraphicDataURITableTransitional
	case strings.Contains(uri, "/ole"):
		return drawingGraphicDataURIOLETransitional
	default:
		return uri
	}
}

// Connector is a connector shape (p:cxnSp) — a line or arrow between shapes.
type Connector struct {
	NvCxnSpPr NonVisualCxnSpProps `xml:"nvCxnSpPr"`
	SpPr      ConnectorProperties `xml:"spPr"`
	Style     *ShapeStyle         `xml:"style"`
}

// NonVisualCxnSpProps holds identity info for a connector.
type NonVisualCxnSpProps struct {
	CNvPr      CnvPr                     `xml:"cNvPr"`
	CNvCxnSpPr *ConnectorConnectionProps `xml:"cNvCxnSpPr"`
}

// ConnectorConnectionProps stores optional start/end connection-site references.
type ConnectorConnectionProps struct {
	StCxn  *ConnectionSiteRef `xml:"stCxn"`
	EndCxn *ConnectionSiteRef `xml:"endCxn"`
}

// ConnectionSiteRef points to a connection site on another shape.
type ConnectionSiteRef struct {
	ID  int `xml:"id,attr"`
	Idx int `xml:"idx,attr"`
}

// ConnectorProperties holds the geometry and line style for a connector.
type ConnectorProperties struct {
	Xfrm     *Transform2D    `xml:"xfrm"`
	PrstGeom *PresetGeometry `xml:"prstGeom"`
	Line     *ConnectorLine  `xml:"ln"`
}

// PresetGeometry describes a preset connector/shape geometry and its adjustment guides.
type PresetGeometry = drawingxml.PresetGeometry

// GeometryGuide stores a single adjustment guide from a preset geometry.
type GeometryGuide = drawingxml.GeometryGuide

// CustomGeometry describes a DrawingML custom geometry and its guide/path lists.
type CustomGeometry = drawingxml.CustomGeometry

// CustomPath stores one custom-geometry path in its local coordinate space.
type CustomPath = drawingxml.CustomPath

// FormulaPoint stores a custom-path coordinate that may be a formula.
type FormulaPoint = drawingxml.FormulaPoint

// CustomPathCommand stores one ordered command from a custom geometry path.
type CustomPathCommand = drawingxml.CustomPathCommand

// ConnectorLine holds line styling for a connector or shape border.
type ConnectorLine struct {
	W         int64                 `xml:"w,attr,omitempty"` // line width in EMU (1 pt = 12700)
	Cap       string                `xml:"cap,attr,omitempty"`
	Cmpd      string                `xml:"cmpd,attr,omitempty"`
	SolidFill *drawingxml.SolidFill `xml:"solidFill"`
	NoFill    *struct{}             `xml:"noFill"`
	PrstDash  *PresetDash           `xml:"prstDash"`
	Round     *struct{}             `xml:"round"`
	Bevel     *struct{}             `xml:"bevel"`
	Miter     *LineMiter            `xml:"miter"`
	HeadEnd   *LineEnd              `xml:"headEnd"`
	TailEnd   *LineEnd              `xml:"tailEnd"`
}

// PresetDash stores a DrawingML preset dash style.
type PresetDash struct {
	Val string `xml:"val,attr,omitempty"`
}

// LineMiter stores an optional miter limit on a line join.
type LineMiter struct {
	Lim int `xml:"lim,attr,omitempty"`
}

// LineEnd describes an arrowhead or endpoint decoration on a connector line.
type LineEnd struct {
	Type string `xml:"type,attr,omitempty"` // "none","triangle","arrow","stealth","diamond","oval"
	W    string `xml:"w,attr,omitempty"`    // "sm","med","lg"
	Len  string `xml:"len,attr,omitempty"`  // "sm","med","lg"
}

// ShapeStyleRef references a theme style-matrix entry with optional color override.
type ShapeStyleRef struct {
	Idx       int                   `xml:"idx,attr,omitempty"`
	Color     *ColorRef             `xml:"srgbClr"`
	SchemeClr *drawingxml.SchemeClr `xml:"schemeClr"`
}

// ShapeFontRef references a theme font scheme entry with optional color override.
type ShapeFontRef struct {
	Idx       string                `xml:"idx,attr,omitempty"` // "major" or "minor"
	Color     *ColorRef             `xml:"srgbClr"`
	SchemeClr *drawingxml.SchemeClr `xml:"schemeClr"`
}

// ShapeStyle holds theme style references for a shape (a:style / p:style).
type ShapeStyle struct {
	LnRef     *ShapeStyleRef `xml:"lnRef"`
	FillRef   *ShapeStyleRef `xml:"fillRef"`
	EffectRef *ShapeStyleRef `xml:"effectRef"`
	FontRef   *ShapeFontRef  `xml:"fontRef"`
}

// Shape is a text or connector shape (p:sp).
type Shape struct {
	NvSpPr NonVisualShapeProps `xml:"nvSpPr"`
	SpPr   ShapeProperties     `xml:"spPr"`
	Style  *ShapeStyle         `xml:"style"`
	TxBody *TextBody           `xml:"txBody"`
}

// Picture is an image shape (p:pic).
type Picture struct {
	NvPicPr  NonVisualPicProps `xml:"nvPicPr"`
	BlipFill BlipFill          `xml:"blipFill"`
	SpPr     ShapeProperties   `xml:"spPr"`
	Style    *ShapeStyle       `xml:"style"`
}

// GroupShape is a p:grpSp container of nested shapes.
type GroupShape struct {
	NvGrpSpPr       *NonVisualGroupShapeProps `xml:"nvGrpSpPr"`
	GrpSpPr         *GroupShapeProperties     `xml:"grpSpPr"`
	Shapes          []Shape                   `xml:"-"`
	Pictures        []Picture                 `xml:"-"`
	GroupShapes     []GroupShape              `xml:"-"`
	Connectors      []Connector               `xml:"-"`
	Tables          []TableFrame              `xml:"-"`
	SmartArtFrames  []SmartArtFrame           `xml:"-"`
	OleObjectFrames []OleObjectFrame          `xml:"-"`
	Items           []ShapeTreeItem           `xml:"-"`
	Diagnostics     diag.List                 `xml:"-"`
}

// NonVisualGroupShapeProps holds identity info for a group shape.
type NonVisualGroupShapeProps struct {
	CNvPr CnvPr `xml:"cNvPr"`
}

// GroupShapeProperties holds geometry and transform for a group shape.
type GroupShapeProperties struct {
	Xfrm *Transform2D `xml:"xfrm"`
}

// UnmarshalXML preserves child order within a group shape.
func (g *GroupShape) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	*g = GroupShape{}
	for {
		tok, err := d.Token()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		switch se := tok.(type) {
		case xml.StartElement:
			switch se.Name.Local {
			case "nvGrpSpPr":
				var nv NonVisualGroupShapeProps
				if err := d.DecodeElement(&nv, &se); err != nil {
					return err
				}
				g.NvGrpSpPr = &nv
			case "grpSpPr":
				var props GroupShapeProperties
				if err := d.DecodeElement(&props, &se); err != nil {
					return err
				}
				g.GrpSpPr = &props
			case "sp":
				idx := len(g.Shapes)
				var shape Shape
				if err := d.DecodeElement(&shape, &se); err != nil {
					return err
				}
				g.Shapes = append(g.Shapes, shape)
				g.Items = append(g.Items, ShapeTreeItem{Kind: ShapeTreeItemShape, Index: idx})
			case "pic":
				idx := len(g.Pictures)
				var pic Picture
				if err := d.DecodeElement(&pic, &se); err != nil {
					return err
				}
				g.Pictures = append(g.Pictures, pic)
				g.Items = append(g.Items, ShapeTreeItem{Kind: ShapeTreeItemPicture, Index: idx})
			case "grpSp":
				idx := len(g.GroupShapes)
				var grp GroupShape
				if err := d.DecodeElement(&grp, &se); err != nil {
					return err
				}
				g.GroupShapes = append(g.GroupShapes, grp)
				g.Items = append(g.Items, ShapeTreeItem{Kind: ShapeTreeItemGroup, Index: idx})
			case "cxnSp":
				idx := len(g.Connectors)
				var cx Connector
				if err := d.DecodeElement(&cx, &se); err != nil {
					return err
				}
				g.Connectors = append(g.Connectors, cx)
				g.Items = append(g.Items, ShapeTreeItem{Kind: ShapeTreeItemConnector, Index: idx})
			case "graphicFrame":
				raw, err := captureRawXMLElementPPTX(d, se)
				if err != nil {
					return err
				}
				var frame struct {
					CNvPr   CnvPr        `xml:"nvGraphicFramePr>cNvPr"`
					Xfrm    *Transform2D `xml:"xfrm"`
					Graphic struct {
						GraphicData struct {
							URI   string             `xml:"uri,attr,omitempty"`
							Table *PresentationTable `xml:"tbl"`
						} `xml:"graphicData"`
					} `xml:"graphic"`
				}
				if err := xml.Unmarshal(raw, &frame); err != nil {
					return err
				}
				uri := normalizePresentationGraphicDataURI(frame.Graphic.GraphicData.URI)
				if uri == drawingGraphicDataURITableTransitional && frame.Graphic.GraphicData.Table != nil {
					idx := len(g.Tables)
					g.Tables = append(g.Tables, TableFrame{
						ID:    frame.CNvPr.ID,
						Name:  frame.CNvPr.Name,
						Desc:  frame.CNvPr.Desc,
						Xfrm:  frame.Xfrm,
						Table: frame.Graphic.GraphicData.Table,
					})
					g.Items = append(g.Items, ShapeTreeItem{Kind: ShapeTreeItemTable, Index: idx})
					continue
				}
				if uri == drawingGraphicDataURIDiagramTransitional || uri == drawingGraphicDataURIDiagramStrict {
					idx := len(g.SmartArtFrames)
					g.SmartArtFrames = append(g.SmartArtFrames, SmartArtFrame{
						ID:     frame.CNvPr.ID,
						Name:   frame.CNvPr.Name,
						Desc:   frame.CNvPr.Desc,
						Xfrm:   frame.Xfrm,
						RelIDs: parseSmartArtRelIDs(raw),
						RawXML: raw,
					})
					g.Items = append(g.Items, ShapeTreeItem{Kind: ShapeTreeItemSmartArt, Index: idx})
					continue
				}
				if uri == drawingGraphicDataURIOLETransitional || uri == drawingGraphicDataURIOLEStrict {
					if ole := parseOleObjectFrame(frame.CNvPr, frame.Xfrm, raw); ole != nil {
						idx := len(g.OleObjectFrames)
						g.OleObjectFrames = append(g.OleObjectFrames, *ole)
						g.Items = append(g.Items, ShapeTreeItem{Kind: ShapeTreeItemOleObject, Index: idx})
					}
					continue
				}
			default:
				if err := d.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if se.Name == start.Name {
				return nil
			}
		}
	}
}

// NonVisualShapeProps holds identity and placeholder info for a shape.
type NonVisualShapeProps struct {
	CNvPr CnvPr         `xml:"cNvPr"`
	NvPr  PlaceholderPr `xml:"nvPr"`
}

// NonVisualPicProps holds identity info and application metadata for a picture.
type NonVisualPicProps struct {
	CNvPr    CnvPr                            `xml:"cNvPr"`
	CNvPicPr *CNvPicPr                        `xml:"cNvPicPr,omitempty"`
	NvPr     ApplicationNonVisualDrawingProps `xml:"nvPr"`
}

// CNvPicPr holds picture locks for a picture's non-visual properties.
type CNvPicPr struct {
	PicLocks *PicLocks `xml:"picLocks"`
}

// PicLocks controls which picture properties are locked.
type PicLocks struct {
	NoChangeAspect bool `xml:"noChangeAspect,attr,omitempty"`
	NoCrop         bool `xml:"noCrop,attr,omitempty"`
	NoRotate       bool `xml:"noRotate,attr,omitempty"`
	NoSelect       bool `xml:"noSelect,attr,omitempty"`
}

// CnvPr holds the id and name of a shape or picture.
type CnvPr struct {
	ID         int         `xml:"id,attr"`
	Name       string      `xml:"name,attr"`
	Desc       string      `xml:"descr,attr,omitempty"`
	HlinkClick *HlinkClick `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hlinkClick"`
}

// HlinkClick holds a click hyperlink reference on a shape or run.
type HlinkClick struct {
	// RelID is the r:id of the hyperlink relationship (external URL or slide target).
	RelID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
	// Action is a ppaction:// URI for built-in navigation actions (e.g. "ppaction://hlinksldjump").
	Action string `xml:"action,attr,omitempty"`
}

// PlaceholderPr holds placeholder type info.
type PlaceholderPr struct {
	Ph *PlaceholderInfo `xml:"ph"`
}

// ApplicationNonVisualDrawingProps stores placeholder and media metadata from nvPr.
type ApplicationNonVisualDrawingProps struct {
	Ph           *PlaceholderInfo
	AudioFile    *MediaReference
	VideoFile    *MediaReference
	WavAudioFile *MediaReference
	Media        *MediaReference
	// Playback hints (not part of the core OOXML nvPr schema; written as
	// attributes on the media element when non-default).
	AutoPlay   bool
	Loop       bool
	HideAtShow bool
	Volume     string // "mute", "low", "medium", "loud"
}

// MediaReference stores a relationship-backed media pointer from nvPr metadata.
type MediaReference struct {
	Link  string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships link,attr,omitempty"`
	Embed string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships embed,attr,omitempty"`
}

// RelID returns the embedded relationship ID when present, otherwise the linked ID.
func (m *MediaReference) RelID() string {
	if m == nil {
		return ""
	}
	if m.Embed != "" {
		return m.Embed
	}
	return m.Link
}

// PrimaryMedia returns the first explicit media reference found on nvPr.
func (p *ApplicationNonVisualDrawingProps) PrimaryMedia() *MediaReference {
	if p == nil {
		return nil
	}
	if p.Media != nil && p.Media.RelID() != "" {
		return p.Media
	}
	if p.VideoFile != nil && p.VideoFile.RelID() != "" {
		return p.VideoFile
	}
	if p.AudioFile != nil && p.AudioFile.RelID() != "" {
		return p.AudioFile
	}
	if p.WavAudioFile != nil && p.WavAudioFile.RelID() != "" {
		return p.WavAudioFile
	}
	return nil
}

// MediaKind hints the intended media class when nvPr explicitly declares one.
func (p *ApplicationNonVisualDrawingProps) MediaKind() string {
	if p == nil {
		return ""
	}
	if p.VideoFile != nil && p.VideoFile.RelID() != "" {
		return "video"
	}
	if (p.AudioFile != nil && p.AudioFile.RelID() != "") || (p.WavAudioFile != nil && p.WavAudioFile.RelID() != "") {
		return "audio"
	}
	return ""
}

// MarshalXML encodes nvPr children: ph, audioFile, videoFile, wavAudioFile, media, extLst.
func (p ApplicationNonVisualDrawingProps) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if p.Ph != nil {
		if err := e.EncodeElement(p.Ph, xml.StartElement{Name: xml.Name{Local: "ph"}}); err != nil {
			return err
		}
	}
	if p.AudioFile != nil {
		if err := encodeMediaRef(e, "audioFile", p.AudioFile, &p); err != nil {
			return err
		}
	}
	if p.VideoFile != nil {
		if err := encodeMediaRef(e, "videoFile", p.VideoFile, &p); err != nil {
			return err
		}
	}
	if p.WavAudioFile != nil {
		if err := encodeMediaRef(e, "wavAudioFile", p.WavAudioFile, nil); err != nil {
			return err
		}
	}
	if p.Media != nil {
		if err := encodeMediaRef(e, "media", p.Media, nil); err != nil {
			return err
		}
	}
	if p.Media != nil && p.Media.Embed != "" {
		if err := encodeMediaExtLst(e, p.Media.Embed); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

func encodeMediaRef(e *xml.Encoder, local string, ref *MediaReference, playback *ApplicationNonVisualDrawingProps) error {
	start := xml.StartElement{Name: xml.Name{Local: local}}
	if ref.Embed != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "embed"}, Value: ref.Embed})
	}
	if ref.Link != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "link"}, Value: ref.Link})
	}
	if playback != nil {
		if playback.AutoPlay {
			start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "autoPlay"}, Value: "1"})
		}
		if playback.Loop {
			start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "loop"}, Value: "1"})
		}
		if playback.HideAtShow {
			start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "hideAtShow"}, Value: "1"})
		}
		if playback.Volume != "" {
			start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "vol"}, Value: playback.Volume})
		}
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

func encodeMediaExtLst(e *xml.Encoder, embed string) error {
	extLst := xml.StartElement{Name: xml.Name{Local: "extLst"}}
	if err := e.EncodeToken(extLst); err != nil {
		return err
	}
	extStart := xml.StartElement{
		Name: xml.Name{Local: "ext"},
		Attr: []xml.Attr{{Name: xml.Name{Local: "uri"}, Value: "{DAA4B4D4-6D71-4841-9C94-3DE7FCFB9230}"}},
	}
	if err := e.EncodeToken(extStart); err != nil {
		return err
	}
	mediaStart := xml.StartElement{Name: xml.Name{Local: "media"}}
	mediaStart.Attr = append(mediaStart.Attr, xml.Attr{Name: xml.Name{Local: "embed"}, Value: embed})
	if err := e.EncodeToken(mediaStart); err != nil {
		return err
	}
	if err := e.EncodeToken(xml.EndElement{Name: mediaStart.Name}); err != nil {
		return err
	}
	if err := e.EncodeToken(xml.EndElement{Name: extStart.Name}); err != nil {
		return err
	}
	if err := e.EncodeToken(xml.EndElement{Name: extLst.Name}); err != nil {
		return err
	}
	return nil
}

// UnmarshalXML captures placeholder and media metadata, including p14:media inside extLst.
func (p *ApplicationNonVisualDrawingProps) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	*p = ApplicationNonVisualDrawingProps{}
	for {
		tok, err := d.Token()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		switch se := tok.(type) {
		case xml.StartElement:
			switch se.Name.Local {
			case "ph":
				var ph PlaceholderInfo
				if err := d.DecodeElement(&ph, &se); err != nil {
					return err
				}
				p.Ph = &ph
			case "audioFile":
				ref := mediaReferenceFromStart(se)
				p.AudioFile = &ref
				if err := d.Skip(); err != nil {
					return err
				}
			case "videoFile":
				ref := mediaReferenceFromStart(se)
				p.VideoFile = &ref
				if err := d.Skip(); err != nil {
					return err
				}
			case "wavAudioFile":
				ref := mediaReferenceFromStart(se)
				p.WavAudioFile = &ref
				if err := d.Skip(); err != nil {
					return err
				}
			case "extLst", "ext":
				if err := collectApplicationMediaRefs(d, se, p); err != nil {
					return err
				}
			default:
				if err := d.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if se.Name == start.Name {
				return nil
			}
		}
	}
}

func mediaReferenceFromStart(start xml.StartElement) MediaReference {
	ref := MediaReference{}
	for _, attr := range start.Attr {
		if attr.Name.Space != "" && attr.Name.Space != "http://schemas.openxmlformats.org/officeDocument/2006/relationships" {
			continue
		}
		switch attr.Name.Local {
		case "link":
			ref.Link = attr.Value
		case "embed":
			ref.Embed = attr.Value
		}
	}
	return ref
}

func collectApplicationMediaRefs(d *xml.Decoder, start xml.StartElement, props *ApplicationNonVisualDrawingProps) error {
	for {
		tok, err := d.Token()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		switch se := tok.(type) {
		case xml.StartElement:
			if se.Name.Local == "media" {
				ref := mediaReferenceFromStart(se)
				if ref.RelID() != "" {
					props.Media = &ref
				}
				if err := d.Skip(); err != nil {
					return err
				}
				continue
			}
			if err := collectApplicationMediaRefs(d, se, props); err != nil {
				return err
			}
		case xml.EndElement:
			if se.Name == start.Name {
				return nil
			}
		}
	}
}

// PlaceholderInfo identifies the semantic placeholder role.
type PlaceholderInfo struct {
	Type            string `xml:"type,attr,omitempty"`            // "title", "body", "dt", "sldNum", etc.
	Idx             int    `xml:"idx,attr,omitempty"`             // placeholder index for matching
	Orient          string `xml:"orient,attr,omitempty"`          // "horz" or "vert"
	Size            string `xml:"sz,attr,omitempty"`              // "full", "half", or "quarter"
	HasCustomPrompt *bool  `xml:"hasCustomPrompt,attr,omitempty"` // custom prompt flag
}

// ShapeProperties holds geometry and transform for a shape.
type ShapeProperties struct {
	Xfrm             *Transform2D          `xml:"xfrm"`
	PrstGeom         *PresetGeometry       `xml:"prstGeom"`
	CustGeom         *CustomGeometry       `xml:"custGeom"`
	Scene3D          *Scene3D              `xml:"scene3d"`
	Shape3D          *Shape3D              `xml:"sp3d"`
	SolidFill        *ColorRef             `xml:"solidFill>srgbClr"`
	FillScheme       *drawingxml.SchemeClr `xml:"solidFill>schemeClr"`
	GradFill         *GradFill             `xml:"gradFill"`
	PattFill         *PattFill             `xml:"pattFill"`
	BlipFill         *BlipFill             `xml:"blipFill"`
	NoFill           *struct{}             `xml:"noFill"`
	Line             *ConnectorLine        `xml:"ln"`
	OuterShadow      *OuterShadowEffect    `xml:"effectLst>outerShdw"`
	PresetShadow     *PresetShadowEffect   `xml:"effectLst>prstShdw"`
	FillOverlay      *FillOverlayEffect    `xml:"effectLst>fillOverlay"`
	InnerShadow      *InnerShadowEffect    `xml:"effectLst>innerShdw"`
	Glow             *GlowEffect           `xml:"effectLst>glow"`
	SoftEdge         *SoftEdgeEffect       `xml:"effectLst>softEdge"`
	Reflection       *ReflectionEffect     `xml:"effectLst>reflection"`
	Blur             *BlurEffect           `xml:"effectLst>blur"`
	EffectDag        *EffectDag            `xml:"effectDag"`
	ParseDiagnostics diag.List
}

// EffectDag stores an ordered DrawingML effect container (a:effectDag / a:cont).
// The presentation resolver currently treats containers as transparent wrappers
// and flattens supported leaf effects in document order. Container metadata is
// retained so richer DAG compositing can be added without reparsing XML.
type EffectDag struct {
	Name  string
	Type  string
	Items []EffectDagItem
}

// EffectDagItem is one ordered child of an EffectDag container. Exactly one
// field is populated for items parsed from valid DrawingML.
type EffectDagItem struct {
	Container    *EffectDag
	OuterShadow  *OuterShadowEffect
	PresetShadow *PresetShadowEffect
	FillOverlay  *FillOverlayEffect
	InnerShadow  *InnerShadowEffect
	Glow         *GlowEffect
	SoftEdge     *SoftEdgeEffect
	Reflection   *ReflectionEffect
	Blur         *BlurEffect
}

func (dag *EffectDag) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	if dag == nil {
		return nil
	}
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "name":
			dag.Name = attr.Value
		case "type":
			dag.Type = attr.Value
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			var item EffectDagItem
			switch t.Name.Local {
			case "cont":
				var value EffectDag
				if err := d.DecodeElement(&value, &t); err != nil {
					return err
				}
				item.Container = &value
			case "outerShdw":
				var value OuterShadowEffect
				if err := d.DecodeElement(&value, &t); err != nil {
					return err
				}
				item.OuterShadow = &value
			case "prstShdw":
				var value PresetShadowEffect
				if err := d.DecodeElement(&value, &t); err != nil {
					return err
				}
				item.PresetShadow = &value
			case "fillOverlay":
				var value FillOverlayEffect
				if err := d.DecodeElement(&value, &t); err != nil {
					return err
				}
				item.FillOverlay = &value
			case "innerShdw":
				var value InnerShadowEffect
				if err := d.DecodeElement(&value, &t); err != nil {
					return err
				}
				item.InnerShadow = &value
			case "glow":
				var value GlowEffect
				if err := d.DecodeElement(&value, &t); err != nil {
					return err
				}
				item.Glow = &value
			case "softEdge":
				var value SoftEdgeEffect
				if err := d.DecodeElement(&value, &t); err != nil {
					return err
				}
				item.SoftEdge = &value
			case "reflection":
				var value ReflectionEffect
				if err := d.DecodeElement(&value, &t); err != nil {
					return err
				}
				item.Reflection = &value
			case "blur":
				var value BlurEffect
				if err := d.DecodeElement(&value, &t); err != nil {
					return err
				}
				item.Blur = &value
			default:
				if err := d.Skip(); err != nil {
					return err
				}
				continue
			}
			dag.Items = append(dag.Items, item)
		case xml.EndElement:
			if t.Name == start.Name {
				return nil
			}
		}
	}
}

func (dag EffectDag) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if start.Name.Local == "" || start.Name.Local == "EffectDag" {
		start.Name.Local = "effectDag"
	}
	if dag.Name != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "name"}, Value: dag.Name})
	}
	if dag.Type != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "type"}, Value: dag.Type})
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	for _, item := range dag.Items {
		var value any
		local := ""
		switch {
		case item.Container != nil:
			value, local = item.Container, "cont"
		case item.OuterShadow != nil:
			value, local = item.OuterShadow, "outerShdw"
		case item.PresetShadow != nil:
			value, local = item.PresetShadow, "prstShdw"
		case item.FillOverlay != nil:
			value, local = item.FillOverlay, "fillOverlay"
		case item.InnerShadow != nil:
			value, local = item.InnerShadow, "innerShdw"
		case item.Glow != nil:
			value, local = item.Glow, "glow"
		case item.SoftEdge != nil:
			value, local = item.SoftEdge, "softEdge"
		case item.Reflection != nil:
			value, local = item.Reflection, "reflection"
		case item.Blur != nil:
			value, local = item.Blur, "blur"
		default:
			continue
		}
		if err := e.EncodeElement(value, xml.StartElement{Name: xml.Name{Space: start.Name.Space, Local: local}}); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// EffectAlpha stores an alpha percentage in thousandths of a percent.
type EffectAlpha struct {
	Val int `xml:"val,attr,omitempty"`
}

// EffectColorRef holds an effect color with optional alpha.
type EffectColorRef struct {
	Val   string       `xml:"val,attr"`
	Alpha *EffectAlpha `xml:"alpha"`
}

// EffectSchemeColor references a theme color with optional alpha.
type EffectSchemeColor struct {
	Val   string       `xml:"val,attr"`
	Alpha *EffectAlpha `xml:"alpha"`
}

// OuterShadowEffect stores DrawingML outer shadow attributes.
type OuterShadowEffect struct {
	BlurRad      int64              `xml:"blurRad,attr,omitempty"`
	Dist         int64              `xml:"dist,attr,omitempty"`
	Dir          int                `xml:"dir,attr,omitempty"`
	Sx           int                `xml:"sx,attr,omitempty"`
	Sy           int                `xml:"sy,attr,omitempty"`
	Kx           int                `xml:"kx,attr,omitempty"`
	Ky           int                `xml:"ky,attr,omitempty"`
	Algn         string             `xml:"algn,attr,omitempty"`
	RotWithShape *bool              `xml:"rotWithShape,attr,omitempty"`
	SrgbClr      *EffectColorRef    `xml:"srgbClr"`
	SchemeClr    *EffectSchemeColor `xml:"schemeClr"`
}

// FillOverlayEffect stores DrawingML fillOverlay metadata. The fill is preserved
// using the same fill structures as shape fills, while Blend retains the OOXML
// blend mode (for example over, mult, screen, darken, or lighten).
type FillOverlayEffect struct {
	Blend      string                `xml:"blend,attr,omitempty"`
	SolidFill  *ColorRef             `xml:"solidFill>srgbClr"`
	FillScheme *drawingxml.SchemeClr `xml:"solidFill>schemeClr"`
	GradFill   *GradFill             `xml:"gradFill"`
	PattFill   *PattFill             `xml:"pattFill"`
	BlipFill   *BlipFill             `xml:"blipFill"`
	NoFill     *struct{}             `xml:"noFill"`
}

// DrawingFill resolves the overlay fill using the normal DrawingML fill path.
func (f *FillOverlayEffect) DrawingFill(themeColors map[string]string) drawing.Fill {
	if f == nil {
		return nil
	}
	return (&ShapeProperties{
		SolidFill: f.SolidFill, FillScheme: f.FillScheme, GradFill: f.GradFill,
		PattFill: f.PattFill, BlipFill: f.BlipFill, NoFill: f.NoFill,
	}).DrawingFill(themeColors)
}

// PresetShadowEffect stores a DrawingML preset shadow (a:prstShdw).
// Prst preserves the original shdw1..shdw20 identifier for round-trip fidelity.
type PresetShadowEffect struct {
	Prst      string             `xml:"prst,attr,omitempty"`
	Dist      int64              `xml:"dist,attr,omitempty"`
	Dir       int                `xml:"dir,attr,omitempty"`
	SrgbClr   *EffectColorRef    `xml:"srgbClr"`
	SchemeClr *EffectSchemeColor `xml:"schemeClr"`
}

// GlowEffect stores DrawingML glow attributes.
type GlowEffect struct {
	Rad       int64              `xml:"rad,attr,omitempty"`
	SrgbClr   *EffectColorRef    `xml:"srgbClr"`
	SchemeClr *EffectSchemeColor `xml:"schemeClr"`
}

// SoftEdgeEffect stores DrawingML soft-edge radius.
type SoftEdgeEffect struct {
	Rad int64 `xml:"rad,attr,omitempty"`
}

// InnerShadowEffect stores DrawingML inner shadow attributes (a:innerShdw).
type InnerShadowEffect struct {
	BlurRad   int64              `xml:"blurRad,attr,omitempty"`
	Dist      int64              `xml:"dist,attr,omitempty"`
	Dir       int                `xml:"dir,attr,omitempty"`
	SrgbClr   *EffectColorRef    `xml:"srgbClr"`
	SchemeClr *EffectSchemeColor `xml:"schemeClr"`
}

// BlurEffect stores DrawingML blur radius and grow behavior (a:blur).
type BlurEffect struct {
	Rad  int64 `xml:"rad,attr,omitempty"`
	Grow *bool `xml:"grow,attr,omitempty"`
}

// ReflectionEffect stores DrawingML reflection attributes.
type ReflectionEffect struct {
	BlurRad      int64  `xml:"blurRad,attr,omitempty"`
	StA          int    `xml:"stA,attr,omitempty"`
	StPos        int    `xml:"stPos,attr,omitempty"`
	EndA         int    `xml:"endA,attr,omitempty"`
	EndPos       int    `xml:"endPos,attr,omitempty"`
	Dist         int64  `xml:"dist,attr,omitempty"`
	Dir          int    `xml:"dir,attr,omitempty"`
	FadeDir      int    `xml:"fadeDir,attr,omitempty"`
	Sx           int    `xml:"sx,attr,omitempty"`
	Sy           int    `xml:"sy,attr,omitempty"`
	Kx           int    `xml:"kx,attr,omitempty"`
	Ky           int    `xml:"ky,attr,omitempty"`
	Algn         string `xml:"algn,attr,omitempty"`
	RotWithShape *bool  `xml:"rotWithShape,attr,omitempty"`
}

// DrawingGeometry converts shape geometry to the shared DrawingML form.
func (sp *ShapeProperties) DrawingGeometry() *drawing.Geometry {
	if sp == nil {
		return nil
	}
	if sp.CustGeom != nil {
		geom, diags := sp.CustGeom.DrawingGeometryWithDiagnostics()
		sp.ParseDiagnostics = append(sp.ParseDiagnostics, diags...)
		return geom
	}
	if sp.PrstGeom != nil {
		return sp.PrstGeom.DrawingGeometry()
	}
	return nil
}

// DrawingFill converts shape fill properties to the shared DrawingML fill abstraction.
func (sp *ShapeProperties) DrawingFill(themeColors map[string]string) drawing.Fill {
	if sp == nil || sp.NoFill != nil {
		return drawing.NoFill{}
	}
	if sp.SolidFill != nil || sp.FillScheme != nil {
		sf := &drawingxml.SolidFill{}
		if sp.SolidFill != nil {
			sf.SRGBClr = &drawingxml.SRGBClr{Val: sp.SolidFill.Val}
		}
		if sp.FillScheme != nil {
			sf.SchemeClr = sp.FillScheme
		}
		if result := drawingxml.ResolveSolidFill(sf, themeColors); result != nil {
			return *result
		}
	}
	if sp.GradFill != nil {
		if result := drawingxml.ResolveGradientFill(gradFillToDrawingXML(sp.GradFill), themeColors); result != nil && len(result.Stops) > 0 {
			return *result
		}
	}
	if sp.PattFill != nil {
		if result := drawingxml.ResolvePatternFill(pattFillToDrawingXML(sp.PattFill), themeColors); result != nil && (result.Foreground != "" || result.Background != "") {
			return *result
		}
	}
	if sp.BlipFill != nil {
		if result := drawingxml.ResolveBlipFill(blipFillToDrawingXML(sp.BlipFill)); result != nil {
			result.DPI = sp.BlipFill.Dpi
			// bool XML decoding cannot distinguish omitted from false here; true is always explicit.
			if sp.BlipFill.RotWithShape != nil {
				result.RotWithShape = *sp.BlipFill.RotWithShape
				result.RotWithShapeSet = true
			}
			if sp.BlipFill.Blip != nil {
				result.BlipEffects = sp.BlipFill.Blip.DrawingEffects(themeColors)
			}
			return *result
		}
		return drawing.NoFill{}
	}
	return nil
}

func gradFillToDrawingXML(gf *GradFill) *drawingxml.GradientFill {
	if gf == nil {
		return nil
	}
	dxGF := &drawingxml.GradientFill{
		Flip: gf.Flip,
	}
	if gf.Lin != nil {
		dxGF.Linear = &drawingxml.LinearGradient{Angle: int(gf.Lin.Ang)}
	}
	if gf.GsLst != nil {
		for _, stopXML := range gf.GsLst.Stops {
			dxStop := drawingxml.GradientStop{Pos: int(stopXML.Pos)}
			if stopXML.SrgbClr != nil {
				dxStop.SRGBClr = &drawingxml.SRGBClr{Val: stopXML.SrgbClr.Val}
			}
			if stopXML.SchemeClr != nil {
				dxStop.SchemeClr = stopXML.SchemeClr
			}
			if stopXML.Alpha != nil {
				if dxStop.SRGBClr != nil {
					dxStop.SRGBClr.Alpha = &drawingxml.ColorTransform{Val: stopXML.Alpha.Val}
				} else if dxStop.SchemeClr != nil {
					dxStop.SchemeClr.Alpha = &drawingxml.ColorTransform{Val: stopXML.Alpha.Val}
				}
			}
			dxGF.StopList = append(dxGF.StopList, dxStop)
		}
	}
	return dxGF
}

func pattFillToDrawingXML(pf *PattFill) *drawingxml.PatternFill {
	if pf == nil {
		return nil
	}
	dxPF := &drawingxml.PatternFill{Prst: pf.Prst}
	if pf.FgClr != nil {
		dxPF.FgClr = &drawingxml.ColorChoice{SRGBClr: &drawingxml.SRGBClr{Val: pf.FgClr.Val}}
	}
	if pf.FgSchemeClr != nil {
		if dxPF.FgClr == nil {
			dxPF.FgClr = &drawingxml.ColorChoice{}
		}
		dxPF.FgClr.SchemeClr = pf.FgSchemeClr
	}
	if pf.BgClr != nil {
		dxPF.BgClr = &drawingxml.ColorChoice{SRGBClr: &drawingxml.SRGBClr{Val: pf.BgClr.Val}}
	}
	if pf.BgSchemeClr != nil {
		if dxPF.BgClr == nil {
			dxPF.BgClr = &drawingxml.ColorChoice{}
		}
		dxPF.BgClr.SchemeClr = pf.BgSchemeClr
	}
	return dxPF
}

func blipFillToDrawingXML(bf *BlipFill) *drawingxml.BlipFill {
	if bf == nil {
		return nil
	}
	result := &drawingxml.BlipFill{Dpi: bf.Dpi}
	if bf.RotWithShape != nil {
		value := "0"
		if *bf.RotWithShape {
			value = "1"
		}
		result.RotWithShape = &value
	}
	if bf.Blip != nil {
		result.Blip = &drawingxml.BlipRef{Embed: bf.Blip.Embed}
	}
	if bf.SrcRect != nil {
		result.SrcRect = &drawingxml.SrcRect{
			L: bf.SrcRect.L,
			T: bf.SrcRect.T,
			R: bf.SrcRect.R,
			B: bf.SrcRect.B,
		}
	}
	if bf.Tile != nil {
		result.Tile = &drawingxml.Tile{
			Tx:   bf.Tile.Tx,
			Ty:   bf.Tile.Ty,
			Sx:   bf.Tile.Sx,
			Sy:   bf.Tile.Sy,
			Flip: bf.Tile.Flip,
			Algn: bf.Tile.Algn,
		}
	}
	if bf.Stretch != nil {
		result.Stretch = &drawingxml.Stretch{}
		if bf.Stretch.FillRect != nil {
			result.Stretch.FillRect = &drawingxml.FillRect{
				L: bf.Stretch.FillRect.L,
				T: bf.Stretch.FillRect.T,
				R: bf.Stretch.FillRect.R,
				B: bf.Stretch.FillRect.B,
			}
		}
	}
	return result
}

// DrawingLineStyle converts PresentationML line properties to the shared DrawingML line abstraction.
func (ln *ConnectorLine) DrawingLineStyle(themeColors map[string]string) *drawing.LineStyle {
	return drawingxml.ResolveLineStyle(connectorLineToDrawingXML(ln), themeColors)
}

func connectorLineToDrawingXML(ln *ConnectorLine) *drawingxml.Line {
	if ln == nil {
		return nil
	}
	dxLn := &drawingxml.Line{
		W:         ln.W,
		Cap:       ln.Cap,
		Cmpd:      ln.Cmpd,
		SolidFill: ln.SolidFill,
		NoFill:    ln.NoFill,
	}
	if ln.PrstDash != nil {
		dxLn.PrstDash = &drawingxml.PresetDash{Val: ln.PrstDash.Val}
	}
	if ln.Round != nil {
		dxLn.Round = &drawingxml.LineRound{}
	}
	if ln.Bevel != nil {
		dxLn.Bevel = &drawingxml.LineBevel{}
	}
	if ln.Miter != nil {
		dxLn.Miter = &drawingxml.LineMiter{Lim: ln.Miter.Lim}
	}
	if ln.HeadEnd != nil {
		dxLn.HeadEnd = &drawingxml.LineEnd{
			Type: ln.HeadEnd.Type,
			W:    ln.HeadEnd.W,
			Len:  ln.HeadEnd.Len,
		}
	}
	if ln.TailEnd != nil {
		dxLn.TailEnd = &drawingxml.LineTailEnd{
			Type: ln.TailEnd.Type,
			W:    ln.TailEnd.W,
			Len:  ln.TailEnd.Len,
		}
	}
	return dxLn
}

// DrawingEffectList converts PresentationML effects to shared DrawingML effects.
func (sp *ShapeProperties) DrawingEffectList(themeColors map[string]string) drawing.EffectList {
	if sp == nil {
		return drawing.EffectList{}
	}
	var effects []drawing.Effect
	if threeD := sp.DrawingThreeDEffect(themeColors); threeD != nil {
		effects = append(effects, *threeD)
	}
	if sp.OuterShadow != nil {
		effects = append(effects, outerShadowDrawingEffect(sp.OuterShadow, themeColors, ""))
	}
	if sp.PresetShadow != nil {
		effects = append(effects, presetShadowDrawingEffect(sp.PresetShadow, themeColors))
	}
	if sp.FillOverlay != nil {
		effects = append(effects, drawing.FillOverlay{Fill: sp.FillOverlay.DrawingFill(themeColors), BlendMode: sp.FillOverlay.Blend})
	}
	if sp.InnerShadow != nil {
		color, alpha := resolveEffectColor(sp.InnerShadow.SrgbClr, sp.InnerShadow.SchemeClr, themeColors)
		effects = append(effects, drawing.InnerShadow{
			Color:    color,
			Alpha:    alpha,
			Blur:     drawing.EMUToPoints(sp.InnerShadow.BlurRad),
			Distance: drawing.EMUToPoints(sp.InnerShadow.Dist),
			Angle:    float64(sp.InnerShadow.Dir) / 60000.0,
		})
	}
	if sp.Glow != nil {
		color, alpha := resolveEffectColor(sp.Glow.SrgbClr, sp.Glow.SchemeClr, themeColors)
		effects = append(effects, drawing.Glow{
			Color:  color,
			Alpha:  alpha,
			Radius: drawing.EMUToPoints(sp.Glow.Rad),
		})
	}
	if sp.SoftEdge != nil {
		effects = append(effects, drawing.SoftEdge{
			Radius: drawing.EMUToPoints(sp.SoftEdge.Rad),
		})
	}
	if sp.Reflection != nil {
		alpha := 0.3
		if sp.Reflection.StA > 0 {
			alpha = float64(sp.Reflection.StA) / 100000.0
		}
		effects = append(effects, reflectionDrawingEffect(sp.Reflection, alpha, ""))
	}
	if sp.Blur != nil {
		effects = append(effects, drawing.Blur{Radius: drawing.EMUToPoints(sp.Blur.Rad), Grow: boolValue(sp.Blur.Grow)})
	}
	appendEffectDagDrawingEffects(&effects, sp.EffectDag, themeColors)
	return drawing.EffectList{Effects: effects}
}

func appendEffectDagDrawingEffects(dst *[]drawing.Effect, dag *EffectDag, themeColors map[string]string) {
	appendEffectDagDrawingEffectsInContainer(dst, dag, themeColors, "")
}

func appendEffectDagDrawingEffectsInContainer(dst *[]drawing.Effect, dag *EffectDag, themeColors map[string]string, inheritedContainer string) {
	if dst == nil || dag == nil {
		return
	}
	containerType := dag.Type
	if containerType == "" {
		containerType = inheritedContainer
	}
	for _, item := range dag.Items {
		switch {
		case item.Container != nil:
			appendEffectDagDrawingEffectsInContainer(dst, item.Container, themeColors, containerType)
		case item.OuterShadow != nil:
			*dst = append(*dst, outerShadowDrawingEffect(item.OuterShadow, themeColors, containerType))
		case item.PresetShadow != nil:
			shadow := presetShadowDrawingEffect(item.PresetShadow, themeColors)
			shadow.ContainerType = containerType
			*dst = append(*dst, shadow)
		case item.FillOverlay != nil:
			*dst = append(*dst, drawing.FillOverlay{Fill: item.FillOverlay.DrawingFill(themeColors), BlendMode: item.FillOverlay.Blend, ContainerType: containerType})
		case item.InnerShadow != nil:
			color, alpha := resolveEffectColor(item.InnerShadow.SrgbClr, item.InnerShadow.SchemeClr, themeColors)
			*dst = append(*dst, drawing.InnerShadow{
				Color: color, Alpha: alpha, Blur: drawing.EMUToPoints(item.InnerShadow.BlurRad),
				Distance: drawing.EMUToPoints(item.InnerShadow.Dist), Angle: float64(item.InnerShadow.Dir) / 60000.0,
				ContainerType: containerType,
			})
		case item.Glow != nil:
			color, alpha := resolveEffectColor(item.Glow.SrgbClr, item.Glow.SchemeClr, themeColors)
			*dst = append(*dst, drawing.Glow{Color: color, Alpha: alpha, Radius: drawing.EMUToPoints(item.Glow.Rad), ContainerType: containerType})
		case item.SoftEdge != nil:
			*dst = append(*dst, drawing.SoftEdge{Radius: drawing.EMUToPoints(item.SoftEdge.Rad), ContainerType: containerType})
		case item.Reflection != nil:
			alpha := 0.3
			if item.Reflection.StA > 0 {
				alpha = float64(item.Reflection.StA) / 100000.0
			}
			*dst = append(*dst, reflectionDrawingEffect(item.Reflection, alpha, containerType))
		case item.Blur != nil:
			*dst = append(*dst, drawing.Blur{Radius: drawing.EMUToPoints(item.Blur.Rad), Grow: boolValue(item.Blur.Grow), ContainerType: containerType})
		}
	}
}

func outerShadowDrawingEffect(effect *OuterShadowEffect, themeColors map[string]string, containerType string) drawing.Shadow {
	if effect == nil {
		return drawing.Shadow{ScaleX: 1, ScaleY: 1, ContainerType: containerType}
	}
	color, alpha := resolveEffectColor(effect.SrgbClr, effect.SchemeClr, themeColors)
	return drawing.Shadow{
		Color: color, Alpha: alpha, Blur: drawing.EMUToPoints(effect.BlurRad), Distance: drawing.EMUToPoints(effect.Dist),
		Angle: float64(effect.Dir) / 60000.0, ScaleX: effectScale(effect.Sx), ScaleY: effectScale(effect.Sy),
		SkewX: float64(effect.Kx) / 60000.0, SkewY: float64(effect.Ky) / 60000.0,
		Align: effect.Algn != "", Alignment: effect.Algn, RotateWithShape: boolValue(effect.RotWithShape), ContainerType: containerType,
	}
}

func reflectionDrawingEffect(effect *ReflectionEffect, alpha float64, containerType string) drawing.Reflection {
	if effect == nil {
		return drawing.Reflection{Alpha: alpha, Size: 1, Fade: true, ContainerType: containerType}
	}
	return drawing.Reflection{
		Alpha: alpha, Blur: drawing.EMUToPoints(effect.BlurRad), Distance: drawing.EMUToPoints(effect.Dist),
		Direction: float64(effect.Dir) / 60000.0, Size: reflectionSize(effect), Fade: reflectionFades(effect),
		StartAlpha: effect.StA, StartPosition: effect.StPos, EndAlpha: effect.EndA, EndPosition: effect.EndPos,
		FadeDirection: float64(effect.FadeDir) / 60000.0, ScaleX: effectScale(effect.Sx), ScaleY: effectScale(effect.Sy),
		SkewX: float64(effect.Kx) / 60000.0, SkewY: float64(effect.Ky) / 60000.0,
		Alignment: effect.Algn, RotateWithShape: boolValue(effect.RotWithShape), ContainerType: containerType,
	}
}

func boolValue(value *bool) bool {
	return value != nil && *value
}

func effectScale(value int) float64 {
	if value == 0 {
		return 1.0
	}
	return float64(value) / 100000.0
}

func reflectionSize(effect *ReflectionEffect) float64 {
	if effect == nil || effect.Sy == 0 {
		return 1.0
	}
	return float64(effect.Sy) / 100000.0
}

func reflectionFades(effect *ReflectionEffect) bool {
	if effect == nil {
		return true
	}
	if effect.EndA != 0 || effect.EndPos != 0 || effect.StPos != 0 {
		return effect.EndA < effect.StA || effect.EndPos > effect.StPos
	}
	return true
}

func presetShadowDrawingEffect(effect *PresetShadowEffect, themeColors map[string]string) drawing.Shadow {
	if effect == nil {
		return drawing.Shadow{ScaleX: 1.0, ScaleY: 1.0}
	}
	color, alpha := resolveEffectColor(effect.SrgbClr, effect.SchemeClr, themeColors)
	return drawing.Shadow{
		Color: color, Alpha: alpha, Distance: drawing.EMUToPoints(effect.Dist),
		Angle: float64(effect.Dir) / 60000.0, ScaleX: 1.0, ScaleY: 1.0,
	}
}

// DrawingThreeDEffect converts scene3d/sp3d metadata into a shared 3D approximation.
func (sp *ShapeProperties) DrawingThreeDEffect(themeColors map[string]string) *drawing.ThreeD {
	if sp == nil || (sp.Scene3D == nil && sp.Shape3D == nil) {
		return nil
	}
	effect := &drawing.ThreeD{}
	if sp.Scene3D != nil {
		if sp.Scene3D.Camera != nil {
			effect.CameraPreset = sp.Scene3D.Camera.Prst
			if sp.Scene3D.Camera.FOV != 0 {
				effect.CameraFOV = float64(sp.Scene3D.Camera.FOV) / 60000.0
			}
			if sp.Scene3D.Camera.Zoom != 0 {
				effect.CameraZoom = float64(sp.Scene3D.Camera.Zoom) / 100000.0
			}
			if sp.Scene3D.Camera.Rot != nil {
				effect.RotationX = float64(sp.Scene3D.Camera.Rot.Lat) / 60000.0
				effect.RotationY = float64(sp.Scene3D.Camera.Rot.Lon) / 60000.0
				effect.RotationZ = float64(sp.Scene3D.Camera.Rot.Rev) / 60000.0
			}
		}
		if sp.Scene3D.LightRig != nil {
			effect.LightRig = sp.Scene3D.LightRig.Rig
			effect.LightDirection = sp.Scene3D.LightRig.Dir
			if sp.Scene3D.LightRig.Rot != nil {
				effect.LightRotationX = float64(sp.Scene3D.LightRig.Rot.Lat) / 60000.0
				effect.LightRotationY = float64(sp.Scene3D.LightRig.Rot.Lon) / 60000.0
				effect.LightRotationZ = float64(sp.Scene3D.LightRig.Rot.Rev) / 60000.0
			}
		}
	}
	if sp.Shape3D != nil {
		effect.Material = sp.Shape3D.PrstMaterial
		effect.ExtrusionDepth = drawing.EMUToPoints(maxInt64(sp.Shape3D.ExtrusionH, sp.Shape3D.Z))
		effect.ContourWidth = drawing.EMUToPoints(sp.Shape3D.ContourW)
		if sp.Shape3D.BevelT != nil {
			effect.BevelTopWidth = drawing.EMUToPoints(sp.Shape3D.BevelT.W)
			effect.BevelTopHeight = drawing.EMUToPoints(sp.Shape3D.BevelT.H)
			effect.BevelTopPreset = sp.Shape3D.BevelT.Prst
		}
		if sp.Shape3D.BevelB != nil {
			effect.BevelBottomWidth = drawing.EMUToPoints(sp.Shape3D.BevelB.W)
			effect.BevelBottomHeight = drawing.EMUToPoints(sp.Shape3D.BevelB.H)
			effect.BevelBottomPreset = sp.Shape3D.BevelB.Prst
		}
		effect.ContourColor = resolve3DColor(sp.Shape3D.ContourClr, sp.Shape3D.ContourScheme, themeColors)
		effect.ExtrusionColor = resolve3DColor(sp.Shape3D.ExtrusionClr, sp.Shape3D.ExtrudeScheme, themeColors)
	}
	if effect.CameraPreset == "" && effect.LightRig == "" && effect.LightDirection == "" && effect.Material == "" && effect.RotationX == 0 && effect.RotationY == 0 && effect.RotationZ == 0 && effect.ExtrusionDepth == 0 && effect.ContourWidth == 0 && effect.BevelTopHeight == 0 && effect.BevelBottomHeight == 0 {
		return nil
	}
	return effect
}

func resolveEffectColor(srgb *EffectColorRef, scheme *EffectSchemeColor, themeColors map[string]string) (string, float64) {
	if srgb != nil {
		return srgb.Val, effectAlphaValue(srgb.Alpha)
	}
	if scheme != nil {
		color := "000000"
		if themeColors != nil {
			if resolved := themeColors[scheme.Val]; resolved != "" {
				color = resolved
			}
		}
		return color, effectAlphaValue(scheme.Alpha)
	}
	return "000000", 1.0
}

func resolve3DColor(srgb *ColorRef, scheme *drawingxml.SchemeClr, themeColors map[string]string) string {
	if srgb != nil && srgb.Val != "" {
		return srgb.Val
	}
	if scheme != nil && themeColors != nil {
		if color := themeColors[scheme.Val]; color != "" {
			return color
		}
	}
	return ""
}

func maxInt64(values ...int64) int64 {
	var max int64
	for _, value := range values {
		if value > max {
			max = value
		}
	}
	return max
}
func effectAlphaValue(alpha *EffectAlpha) float64 {
	if alpha == nil {
		return 1.0
	}
	value := float64(alpha.Val) / 100000.0
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

// Transform2D holds position and size in EMU.
// For group transforms, ChOff and ChExt define the child coordinate system.
type Transform2D struct {
	Off   Offset  `xml:"off"`
	Ext   Extents `xml:"ext"`
	ChOff Offset  `xml:"chOff"`
	ChExt Extents `xml:"chExt"`
	Rot   int     `xml:"rot,attr,omitempty"` // rotation in 1/60000 of a degree
	FlipH bool    `xml:"flipH,attr,omitempty"`
	FlipV bool    `xml:"flipV,attr,omitempty"`
}

// Offset is the top-left position of a shape in EMU.
type Offset struct {
	X int64 `xml:"x,attr"`
	Y int64 `xml:"y,attr"`
}

// Extents is the width and height of a shape in EMU.
type Extents struct {
	CX int64 `xml:"cx,attr"`
	CY int64 `xml:"cy,attr"`
}

// BlipFill describes an image fill (r:embed points to a media relationship).
type BlipFill struct {
	Blip         *Blip               `xml:"blip"`
	SrcRect      *SrcRect            `xml:"srcRect"`
	Tile         *drawingxml.Tile    `xml:"tile"`
	Stretch      *drawingxml.Stretch `xml:"stretch"`
	Dpi          int                 `xml:"dpi,attr,omitempty"`
	RotWithShape *bool               `xml:"rotWithShape,attr,omitempty"`
}

// Blip references an embedded image by relationship ID and preserves image effects
// carried directly by a:blip.
type Blip struct {
	Embed        string           `xml:"embed,attr"`
	Grayscl      *struct{}        `xml:"grayscl"`
	BiLevel      *BlipBiLevel     `xml:"biLevel"`
	Duotone      *BlipDuotone     `xml:"duotone"`
	Lum          *BlipLum         `xml:"lum"`
	HSL          *BlipHSL         `xml:"hsl"`
	AlphaMod     *BlipAmount      `xml:"alphaMod"`
	AlphaModFix  *BlipAlphaModFix `xml:"alphaModFix"`
	AlphaOff     *BlipAmount      `xml:"alphaOff"`
	AlphaRepl    *BlipAlphaRepl   `xml:"alphaRepl"`
	AlphaBiLevel *BlipBiLevel     `xml:"alphaBiLevel"`
	AlphaOutset  *BlipAlphaOutset `xml:"alphaOutset"`
	AlphaInv     *struct{}        `xml:"alphaInv"`
	AlphaCeiling *struct{}        `xml:"alphaCeiling"`
	AlphaFloor   *struct{}        `xml:"alphaFloor"`
	ClrChange    *BlipColorChange `xml:"clrChange"`
	ClrRepl      *BlipColorChoice `xml:"clrRepl"`
}

type BlipBiLevel struct {
	Thresh int `xml:"thresh,attr,omitempty"`
}

type BlipLum struct {
	Bright   int `xml:"bright,attr,omitempty"`
	Contrast int `xml:"contrast,attr,omitempty"`
}

type BlipHSL struct {
	Hue int `xml:"hue,attr,omitempty"`
	Sat int `xml:"sat,attr,omitempty"`
	Lum int `xml:"lum,attr,omitempty"`
}

type BlipAmount struct {
	Amt int `xml:"amt,attr,omitempty"`
}

// BlipAlphaModFix keeps presence of the optional amt attribute distinct from
// an authored zero. PowerPoint emits <a:alphaModFix/> for ordinary pictures;
// that shorthand is a no-op (100%), not fully transparent (0%).
type BlipAlphaModFix struct {
	Amt *int `xml:"amt,attr"`
}

func (a *BlipAlphaModFix) amount() int {
	if a == nil || a.Amt == nil {
		return 100000
	}
	return *a.Amt
}

type BlipAlphaRepl struct {
	A int `xml:"a,attr,omitempty"`
}

type BlipAlphaOutset struct {
	Rad int64 `xml:"rad,attr,omitempty"`
}

// BlipColorChoice captures the color choices used by picture effects.
type BlipColorChoice struct {
	SrgbClr   *EffectColorRef    `xml:"srgbClr"`
	SchemeClr *EffectSchemeColor `xml:"schemeClr"`
}

type BlipDuotone struct {
	// clr1/clr2 are used by some producer dialects and SmartArt drawing parts.
	Clr1 BlipColorChoice `xml:"clr1"`
	Clr2 BlipColorChoice `xml:"clr2"`
	// Standard DrawingML may place two color choices directly under duotone.
	SrgbClr   []EffectColorRef    `xml:"srgbClr"`
	SchemeClr []EffectSchemeColor `xml:"schemeClr"`
}

type BlipColorChange struct {
	UseAlpha *bool           `xml:"useA,attr,omitempty"`
	From     BlipColorChoice `xml:"clrFrom"`
	To       BlipColorChoice `xml:"clrTo"`
}

// DrawingEffects resolves a:blip effects to the shared DrawingML effect model.
func (b *Blip) DrawingEffects(themeColors map[string]string) []drawing.Effect {
	if b == nil {
		return nil
	}
	var effects []drawing.Effect
	if b.Grayscl != nil {
		effects = append(effects, drawing.Grayscale{})
	}
	if b.BiLevel != nil {
		effects = append(effects, drawing.BiLevel{Threshold: b.BiLevel.Thresh})
	}
	if b.Duotone != nil {
		c1, c2 := b.Duotone.resolvedColors(themeColors)
		if c1 != "" || c2 != "" {
			effects = append(effects, drawing.Duotone{Color1: c1, Color2: c2})
		}
	}
	if b.Lum != nil {
		effects = append(effects, drawing.Lum{Brightness: b.Lum.Bright, Contrast: b.Lum.Contrast})
	}
	if b.HSL != nil {
		effects = append(effects, drawing.HSLEffect{Hue: b.HSL.Hue, Sat: b.HSL.Sat, Lum: b.HSL.Lum})
	}
	if b.AlphaMod != nil {
		effects = append(effects, drawing.AlphaMod{Amount: b.AlphaMod.Amt})
	}
	if b.AlphaModFix != nil {
		effects = append(effects, drawing.AlphaModFix{Amount: b.AlphaModFix.amount()})
	}
	if b.AlphaOff != nil {
		effects = append(effects, drawing.AlphaOff{Amount: b.AlphaOff.Amt})
	}
	if b.AlphaRepl != nil {
		effects = append(effects, drawing.AlphaRepl{Alpha: b.AlphaRepl.A})
	}
	if b.AlphaBiLevel != nil {
		effects = append(effects, drawing.AlphaBiLevel{Threshold: b.AlphaBiLevel.Thresh})
	}
	if b.AlphaOutset != nil {
		effects = append(effects, drawing.AlphaOutset{Radius: drawing.EMUToPoints(b.AlphaOutset.Rad)})
	}
	if b.AlphaInv != nil {
		effects = append(effects, drawing.AlphaInv{})
	}
	if b.AlphaCeiling != nil {
		effects = append(effects, drawing.AlphaCeiling{})
	}
	if b.AlphaFloor != nil {
		effects = append(effects, drawing.AlphaFloor{})
	}
	if b.ClrChange != nil {
		from, fromAlpha := b.ClrChange.From.resolve(themeColors)
		to, toAlpha := b.ClrChange.To.resolve(themeColors)
		useAlpha := true // CT_ColorChangeEffect.useA defaults to true when omitted.
		if b.ClrChange.UseAlpha != nil {
			useAlpha = *b.ClrChange.UseAlpha
		}
		effects = append(effects, drawing.ClrChange{From: from, To: to, FromAlpha: fromAlpha, ToAlpha: toAlpha, UseAlpha: useAlpha})
	}
	if b.ClrRepl != nil {
		color, _ := b.ClrRepl.resolve(themeColors)
		effects = append(effects, drawing.ClrRepl{Color: color})
	}
	return effects
}

func (c *BlipColorChoice) resolve(themeColors map[string]string) (string, float64) {
	if c == nil || (c.SrgbClr == nil && c.SchemeClr == nil) {
		return "", 1
	}
	return resolveEffectColor(c.SrgbClr, c.SchemeClr, themeColors)
}

func (d *BlipDuotone) resolvedColors(themeColors map[string]string) (string, string) {
	if d == nil {
		return "", ""
	}
	c1, _ := d.Clr1.resolve(themeColors)
	c2, _ := d.Clr2.resolve(themeColors)
	if c1 != "" || c2 != "" {
		return c1, c2
	}
	var colors []string
	for i := range d.SrgbClr {
		color, _ := resolveEffectColor(&d.SrgbClr[i], nil, themeColors)
		colors = append(colors, color)
	}
	for i := range d.SchemeClr {
		color, _ := resolveEffectColor(nil, &d.SchemeClr[i], themeColors)
		colors = append(colors, color)
	}
	if len(colors) > 0 {
		c1 = colors[0]
	}
	if len(colors) > 1 {
		c2 = colors[1]
	}
	return c1, c2
}

// SrcRect stores picture crop percentages in thousandths of a percent.
type SrcRect struct {
	L int `xml:"l,attr,omitempty"`
	T int `xml:"t,attr,omitempty"`
	R int `xml:"r,attr,omitempty"`
	B int `xml:"b,attr,omitempty"`
}

// TextBody holds the paragraph content of a text shape.
type TextBody struct {
	BodyPr     *DMLBodyPr        `xml:"bodyPr"`
	ListStyle  *DMLTextListStyle `xml:"lstStyle"`
	Paragraphs []DMLParagraph    `xml:"p"`
}

// DMLBodyPr holds text box body properties such as insets, vertical anchoring,
// autofit behavior, vertical text, and columns.
type DMLBodyPr struct {
	Anchor      string          `xml:"anchor,attr,omitempty"`
	AnchorCtr   *bool           `xml:"anchorCtr,attr,omitempty"`
	Rot         int             `xml:"rot,attr,omitempty"`
	Wrap        string          `xml:"wrap,attr,omitempty"`
	Vert        string          `xml:"vert,attr,omitempty"`
	NumCol      *int            `xml:"numCol,attr,omitempty"`
	SpcCol      *int64          `xml:"spcCol,attr,omitempty"`
	LIns        *int64          `xml:"lIns,attr,omitempty"`
	RIns        *int64          `xml:"rIns,attr,omitempty"`
	TIns        *int64          `xml:"tIns,attr,omitempty"`
	BIns        *int64          `xml:"bIns,attr,omitempty"`
	NoAutofit   *struct{}       `xml:"noAutofit"`
	SpAutoFit   *struct{}       `xml:"spAutoFit"`
	NormAutofit *DMLNormAutofit `xml:"normAutofit"`
}

type DMLNormAutofit struct {
	FontScale      *int `xml:"fontScale,attr,omitempty"`
	LnSpcReduction *int `xml:"lnSpcReduction,attr,omitempty"`
}

// DrawingTextBodyProperties converts DMLBodyPr to drawing.TextBodyProperties.
func (bp *DMLBodyPr) DrawingTextBodyProperties() drawing.TextBodyProperties {
	if bp == nil {
		return drawing.TextBodyProperties{}
	}

	props := drawing.TextBodyProperties{
		Anchor:    bp.Anchor,
		AnchorCtr: bp.AnchorCtr != nil && *bp.AnchorCtr,
		Rot:       bp.Rot,
		Wrap:      bp.Wrap,
		Vert:      bp.Vert,
	}

	// Set columns
	if bp.NumCol != nil {
		props.Columns = *bp.NumCol
	}
	if bp.SpcCol != nil {
		props.ColumnSpacingEMU = *bp.SpcCol
	}

	// Set insets
	if bp.LIns != nil {
		props.InsetL = *bp.LIns
	}
	if bp.RIns != nil {
		props.InsetR = *bp.RIns
	}
	if bp.TIns != nil {
		props.InsetT = *bp.TIns
	}
	if bp.BIns != nil {
		props.InsetB = *bp.BIns
	}

	// Set autofit type
	switch {
	case bp.NoAutofit != nil:
		props.AutofitType = "noAutofit"
	case bp.SpAutoFit != nil:
		props.AutofitType = "spAutoFit"
	case bp.NormAutofit != nil:
		props.AutofitType = "normAutofit"
		if bp.NormAutofit.FontScale != nil {
			props.AutofitFontScale = *bp.NormAutofit.FontScale
		}
		if bp.NormAutofit.LnSpcReduction != nil {
			props.AutofitLineSpaceReduction = *bp.NormAutofit.LnSpcReduction
		}
	}

	return props
}

// DMLTextListStyle holds default paragraph/run styling by indent level.
type DMLTextListStyle struct {
	DefPPr  *DMLTextStyleLevel `xml:"defPPr"`
	Lvl1PPr *DMLTextStyleLevel `xml:"lvl1pPr"`
	Lvl2PPr *DMLTextStyleLevel `xml:"lvl2pPr"`
	Lvl3PPr *DMLTextStyleLevel `xml:"lvl3pPr"`
	Lvl4PPr *DMLTextStyleLevel `xml:"lvl4pPr"`
	Lvl5PPr *DMLTextStyleLevel `xml:"lvl5pPr"`
	Lvl6PPr *DMLTextStyleLevel `xml:"lvl6pPr"`
	Lvl7PPr *DMLTextStyleLevel `xml:"lvl7pPr"`
	Lvl8PPr *DMLTextStyleLevel `xml:"lvl8pPr"`
	Lvl9PPr *DMLTextStyleLevel `xml:"lvl9pPr"`
}

// Level returns the style level matching the 0-based paragraph indent level.
func (s *DMLTextListStyle) Level(level int) *DMLTextStyleLevel {
	if s == nil {
		return nil
	}
	switch level {
	case 0:
		if s.Lvl1PPr != nil {
			return s.Lvl1PPr
		}
	case 1:
		if s.Lvl2PPr != nil {
			return s.Lvl2PPr
		}
	case 2:
		if s.Lvl3PPr != nil {
			return s.Lvl3PPr
		}
	case 3:
		if s.Lvl4PPr != nil {
			return s.Lvl4PPr
		}
	case 4:
		if s.Lvl5PPr != nil {
			return s.Lvl5PPr
		}
	case 5:
		if s.Lvl6PPr != nil {
			return s.Lvl6PPr
		}
	case 6:
		if s.Lvl7PPr != nil {
			return s.Lvl7PPr
		}
	case 7:
		if s.Lvl8PPr != nil {
			return s.Lvl8PPr
		}
	case 8:
		if s.Lvl9PPr != nil {
			return s.Lvl9PPr
		}
	}
	return s.DefPPr
}

// DMLTextStyleLevel holds default paragraph formatting and default run props.
type DMLTextStyleLevel struct {
	Algn        string                `xml:"algn,attr,omitempty"`
	Lvl         int                   `xml:"lvl,attr,omitempty"`
	Indent      *int                  `xml:"indent,attr,omitempty"`
	MarL        *int                  `xml:"marL,attr,omitempty"`
	MarR        *int                  `xml:"marR,attr,omitempty"`
	DefTabSz    *int                  `xml:"defTabSz,attr,omitempty"`
	FontAlgn    string                `xml:"fontAlgn,attr,omitempty"`
	LnSpc       *DMLSpacing           `xml:"lnSpc"`
	SpcBef      *DMLSpacing           `xml:"spcBef"`
	SpcAft      *DMLSpacing           `xml:"spcAft"`
	BuChar      *BuChar               `xml:"buChar"`
	BuBlip      *BuBlip               `xml:"buBlip"`
	BuAutoNum   *BuAutoNum            `xml:"buAutoNum"`
	BuNone      *struct{}             `xml:"buNone"`
	BuClr       *ColorRef             `xml:"buClr>srgbClr"`
	BuClrScheme *drawingxml.SchemeClr `xml:"buClr>schemeClr"`
	BuFont      *BuFont               `xml:"buFont"`
	BuFontTx    *struct{}             `xml:"buFontTx"`
	BuSzPts     *BuSzPts              `xml:"buSzPts"`
	BuSzPct     *BuSzPct              `xml:"buSzPct"`
	TabStops    []TabStop             `xml:"tabLst>tab"`
	DefRPr      *DMLRunPr             `xml:"defRPr"`
}

// DMLParagraph is a DrawingML paragraph (a:p).
type DMLParagraph struct {
	Runs       []DMLRun      `xml:"r"`
	Fields     []DMLField    `xml:"fld"`
	Breaks     []DMLBreak    `xml:"br"`
	Properties *DMLParaProps `xml:"pPr"`
	// EndParaRPr carries character properties for the paragraph end mark.
	// It is not a default for ordinary runs; pPr/defRPr owns that role.
	EndParaRPr *DMLRunPr `xml:"endParaRPr"`
	// Inlines preserves ordered run / field / math / break content from UnmarshalXML.
	// When non-nil the resolver uses this slice instead of Runs + Fields.
	Inlines []DMLParaInline `xml:"-"`
}

// DMLParaInlineKind identifies the content type of a DMLParaInline.
type DMLParaInlineKind string

const (
	DMLParaInlineRun   DMLParaInlineKind = "run"
	DMLParaInlineField DMLParaInlineKind = "field"
	DMLParaInlineMath  DMLParaInlineKind = "math"
	DMLParaInlineBreak DMLParaInlineKind = "break"
)

// DMLParaInline is a single ordered inline element inside a DrawingML paragraph.
type DMLParaInline struct {
	Kind  DMLParaInlineKind
	Run   *DMLRun
	Field *DMLField
	Math  *DMLMath
	Break *DMLBreak
}

// DMLBreak is a DrawingML line break element (a:br).
type DMLBreak struct {
	Properties *DMLRunPr `xml:"rPr"`
}

// DMLMath holds the raw XML bytes of an m:oMath or m:oMathPara element found
// inside a DrawingML paragraph.
type DMLMath struct {
	RawXML  []byte
	Display bool // true for m:oMathPara (block/display mode)
}

// UnmarshalXML captures runs, fields, and OMML math in document order.
func (p *DMLParagraph) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch se := tok.(type) {
		case xml.StartElement:
			switch se.Name.Local {
			case "pPr":
				var pp DMLParaProps
				if err := d.DecodeElement(&pp, &se); err != nil {
					return err
				}
				p.Properties = &pp
			case "r":
				var r DMLRun
				if err := d.DecodeElement(&r, &se); err != nil {
					return err
				}
				p.Runs = append(p.Runs, r)
				p.Inlines = append(p.Inlines, DMLParaInline{Kind: DMLParaInlineRun, Run: &p.Runs[len(p.Runs)-1]})
			case "fld":
				var f DMLField
				if err := d.DecodeElement(&f, &se); err != nil {
					return err
				}
				p.Fields = append(p.Fields, f)
				p.Inlines = append(p.Inlines, DMLParaInline{Kind: DMLParaInlineField, Field: &p.Fields[len(p.Fields)-1]})
			case "br":
				var br DMLBreak
				if err := d.DecodeElement(&br, &se); err != nil {
					return err
				}
				p.Breaks = append(p.Breaks, br)
				p.Inlines = append(p.Inlines, DMLParaInline{Kind: DMLParaInlineBreak, Break: &p.Breaks[len(p.Breaks)-1]})
			case "endParaRPr":
				var rpr DMLRunPr
				if err := d.DecodeElement(&rpr, &se); err != nil {
					return err
				}
				p.EndParaRPr = &rpr
			case "oMath", "oMathPara":
				raw, err := captureRawXMLElementPPTX(d, se)
				if err != nil {
					return err
				}
				p.Inlines = append(p.Inlines, DMLParaInline{
					Kind: DMLParaInlineMath,
					Math: &DMLMath{RawXML: raw, Display: se.Name.Local == "oMathPara"},
				})
			default:
				if err := d.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if se.Name == start.Name {
				return nil
			}
		}
	}
}

// DMLField is a DrawingML field element (a:fld) containing auto-generated content
// such as slide numbers or dates.
type DMLField struct {
	ID         string    `xml:"id,attr,omitempty"`
	Type       string    `xml:"type,attr,omitempty"` // e.g. "slidenum", "datetime", "datetime1", etc.
	Text       string    `xml:"t"`
	Properties *DMLRunPr `xml:"rPr"`
}

// DMLParaProps holds paragraph-level formatting.
type DMLParaProps struct {
	Algn        string                `xml:"algn,attr,omitempty"`     // "l", "ctr", "r", "just"
	Lvl         int                   `xml:"lvl,attr,omitempty"`      // indent level 0–8
	Indent      *int                  `xml:"indent,attr,omitempty"`   // EMU
	MarL        *int                  `xml:"marL,attr,omitempty"`     // left margin EMU
	MarR        *int                  `xml:"marR,attr,omitempty"`     // right margin EMU
	DefTabSz    *int                  `xml:"defTabSz,attr,omitempty"` // default tab size EMU
	FontAlgn    string                `xml:"fontAlgn,attr,omitempty"` // "auto", "t", "ctr", "base", "b"
	LnSpc       *DMLSpacing           `xml:"lnSpc"`
	SpcBef      *DMLSpacing           `xml:"spcBef"`
	SpcAft      *DMLSpacing           `xml:"spcAft"`
	BuChar      *BuChar               `xml:"buChar"`
	BuBlip      *BuBlip               `xml:"buBlip"`
	BuAutoNum   *BuAutoNum            `xml:"buAutoNum"`
	BuNone      *struct{}             `xml:"buNone"`
	BuClr       *ColorRef             `xml:"buClr>srgbClr"`
	BuClrScheme *drawingxml.SchemeClr `xml:"buClr>schemeClr"`
	BuFont      *BuFont               `xml:"buFont"`
	BuFontTx    *struct{}             `xml:"buFontTx"`
	BuSzPts     *BuSzPts              `xml:"buSzPts"`
	BuSzPct     *BuSzPct              `xml:"buSzPct"`
	TabStops    []TabStop             `xml:"tabLst>tab"`
	// DefRPr is the paragraph-local default character formatting. It is the
	// authoritative source for runs that do not carry their own rPr; dropping
	// it makes those runs incorrectly fall through to layout/master sizes.
	DefRPr *DMLRunPr `xml:"defRPr"`
}

// TabStop is a custom tab stop within a paragraph.
type TabStop struct {
	Pos  int    `xml:"pos,attr"`            // position in EMU
	Algn string `xml:"algn,attr,omitempty"` // "l", "ctr", "r", "dec"
}

// BuChar is an explicit bullet character (e.g. "●", "•").
type BuChar struct {
	Char string `xml:"char,attr"`
}

type BuBlip struct {
	Blip Blip `xml:"blip"`
}

// BuAutoNum is an automatic numbering bullet (e.g. "1.", "a.", "i.").
type BuAutoNum struct {
	Type    string `xml:"type,attr"` // "arabicPeriod", "arabicParenR", etc.
	StartAt int    `xml:"startAt,attr,omitempty"`
}

// BuFont describes the bullet/number font face.
type BuFont struct {
	Typeface string `xml:"typeface,attr,omitempty"`
}

// BuSzPts stores bullet/number size in hundredths of a point.
type BuSzPts struct {
	Val int `xml:"val,attr,omitempty"`
}

// BuSzPct stores bullet/number size as a percentage in thousandths of a percent
// (e.g. 100000 = 100%).
type BuSzPct struct {
	Val int `xml:"val,attr,omitempty"`
}

// DMLSpacing stores a paragraph spacing value as either points or percent.
type DMLSpacing struct {
	SpcPts *SpacingPts `xml:"spcPts"`
	SpcPct *SpacingPct `xml:"spcPct"`
}

// SpacingPts stores spacing in hundredths of a point.
type SpacingPts struct {
	Val int `xml:"val,attr,omitempty"`
}

// SpacingPct stores spacing in thousandths of a percent (100000 = 100%).
type SpacingPct struct {
	Val int `xml:"val,attr,omitempty"`
}

// DMLRun is a DrawingML text run (a:r).
type DMLRun struct {
	Text       string    `xml:"t"`
	Properties *DMLRunPr `xml:"rPr"`
}

// DMLRunPr holds run-level formatting.
type DMLRunPr struct {
	Lang            string                `xml:"lang,attr,omitempty"`
	FontSize        int                   `xml:"sz,attr,omitempty"`   // hundredths of a point
	Spacing         *int                  `xml:"spc,attr,omitempty"`  // hundredths of a point between characters
	Kern            *int                  `xml:"kern,attr,omitempty"` // minimum font size for kerning, hundredths of a point
	Bold            *bool                 `xml:"b,attr,omitempty"`
	Italic          *bool                 `xml:"i,attr,omitempty"`
	Underline       string                `xml:"u,attr,omitempty"`
	Strike          string                `xml:"strike,attr,omitempty"`
	Cap             string                `xml:"cap,attr,omitempty"`
	Baseline        *int                  `xml:"baseline,attr,omitempty"`
	SolidFill       *ColorRef             `xml:"solidFill>srgbClr"`
	SchemeColor     *drawingxml.SchemeClr `xml:"solidFill>schemeClr"`
	Highlight       *ColorRef             `xml:"highlight>srgbClr"`
	HighlightScheme *drawingxml.SchemeClr `xml:"highlight>schemeClr"`
	Latin           *FontRef              `xml:"latin"`
	EA              *FontRef              `xml:"ea"`
	CS              *FontRef              `xml:"cs"`
	HlinkClick      *HlinkClick           `xml:"hlinkClick"`
}

// ColorRef holds an sRGB hex color value.
type ColorRef struct {
	Val string `xml:"val,attr"`
}

// GradientStopXML represents a gradient stop in DrawingML XML.
type GradientStopXML struct {
	Pos       int64                 `xml:"pos,attr"` // position in 1/100000 percent (0-100000)
	SrgbClr   *ColorRef             `xml:"srgbClr"`
	SchemeClr *drawingxml.SchemeClr `xml:"schemeClr"`
	Alpha     *EffectAlpha          `xml:"alpha"`
}

// PattFill represents a DrawingML pattern fill (a:pattFill).
type PattFill struct {
	Prst        string                `xml:"prst,attr"` // pattern type (e.g., "pct5", "ltHorz", "dkgrid")
	FgClr       *ColorRef             `xml:"fgClr>srgbClr"`
	FgSchemeClr *drawingxml.SchemeClr `xml:"fgClr>schemeClr"`
	BgClr       *ColorRef             `xml:"bgClr>srgbClr"`
	BgSchemeClr *drawingxml.SchemeClr `xml:"bgClr>schemeClr"`
}

// GradFill represents a DrawingML gradient fill (a:gradFill).
type GradFill struct {
	Lin      *GradientLinear   `xml:"lin"`
	GsLst    *GradientStopList `xml:"gsLst"`
	TileRect *RectXML          `xml:"tileRect"`
	Flip     string            `xml:"flip,attr,omitempty"`
}

// GradientLinear represents a linear gradient (a:lin).
type GradientLinear struct {
	Ang int64 `xml:"ang,attr"` // angle in 1/60000 degrees
}

// GradientStopList represents a list of gradient stops (a:gsLst).
type GradientStopList struct {
	Stops []GradientStopXML `xml:"gs"`
}

// RectXML represents a rectangle in DrawingML (a:rect).
type RectXML struct {
	L int64 `xml:"l,attr,omitempty"` // left in 1/100000 percent
	T int64 `xml:"t,attr,omitempty"` // top in 1/100000 percent
	R int64 `xml:"r,attr,omitempty"` // right in 1/100000 percent
	B int64 `xml:"b,attr,omitempty"` // bottom in 1/100000 percent
}

// Theme represents a DrawingML theme part (a:theme).
type Theme struct {
	XMLName         xml.Name              `xml:"theme"`
	Name            string                `xml:"name,attr,omitempty"`
	Elements        ThemeElements         `xml:"themeElements"`
	ObjectDefaults  *ObjectDefaults       `xml:"objectDefaults"`
	ExtraClrSchemes *ExtraColorSchemeList `xml:"extraClrSchemeLst"`
}

// ThemeOverride represents a DrawingML theme override part (a:themeOverride).
type ThemeOverride struct {
	XMLName    xml.Name      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main themeOverride"`
	ClrScheme  *ColorScheme  `xml:"clrScheme"`
	FontScheme *FontScheme   `xml:"fontScheme"`
	FmtScheme  *FormatScheme `xml:"fmtScheme"`
}

// ObjectDefaults preserves a:objectDefaults for future resolver/write-back use.
type ObjectDefaults struct {
	ShapeDefault *ShapeDefault `xml:"spDef"`
	LineDefault  *LineDefault  `xml:"lnDef"`
	TextDefault  *TextDefault  `xml:"txDef"`
}

// ShapeDefault preserves a:spDef default shape properties.
type ShapeDefault struct {
	SpPr  *ShapeProperties `xml:"spPr"`
	Style *ShapeStyle      `xml:"style"`
}

// LineDefault preserves a:lnDef default line properties.
type LineDefault struct {
	Ln *ConnectorLine `xml:"ln"`
}

// TextDefault preserves a:txDef default text-body properties.
type TextDefault struct {
	BodyPr    *DMLBodyPr        `xml:"bodyPr"`
	ListStyle *DMLTextListStyle `xml:"lstStyle"`
	P         []DMLParagraph    `xml:"p"`
}

// ExtraColorSchemeList preserves alternate theme color schemes.
type ExtraColorSchemeList struct {
	Schemes []ExtraColorScheme `xml:"extraClrScheme"`
}

// ExtraColorScheme preserves one a:extraClrScheme entry.
type ExtraColorScheme struct {
	ClrScheme *ColorScheme `xml:"clrScheme"`
	ClrMap    *ColorMap    `xml:"clrMap"`
}

// ThemeElements holds the color, font, and format schemes.
type ThemeElements struct {
	ClrScheme  ColorScheme  `xml:"clrScheme"`
	FontScheme FontScheme   `xml:"fontScheme"`
	FmtScheme  FormatScheme `xml:"fmtScheme"`
}

// FormatScheme holds the subset of the theme style matrix needed for table fillRef resolution.
type FormatScheme struct {
	FillStyleList   []ThemeFormatFillStyle   `xml:"fillStyleLst"`
	BGFillStyleList []ThemeFormatFillStyle   `xml:"bgFillStyleLst"`
	LineStyleList   []ConnectorLine          `xml:"lnStyleLst>ln"`
	EffectStyleList []ThemeFormatEffectStyle `xml:"effectStyleLst>effectStyle"`
}

// ThemeFormatFillStyle stores a single fill entry from a theme style matrix.
type ThemeFormatFillStyle struct {
	SolidFill  *ColorRef             `xml:"solidFill>srgbClr"`
	FillScheme *drawingxml.SchemeClr `xml:"solidFill>schemeClr"`
	GradFill   *GradFill             `xml:"gradFill"`
	PattFill   *PattFill             `xml:"pattFill"`
	BlipFill   *BlipFill             `xml:"blipFill"`
	NoFill     *struct{}             `xml:"noFill"`
}

// ThemeFormatEffectStyle stores a single effect entry from a theme style matrix.
type ThemeFormatEffectStyle struct {
	OuterShadow  *OuterShadowEffect  `xml:"effectLst>outerShdw"`
	PresetShadow *PresetShadowEffect `xml:"effectLst>prstShdw"`
	FillOverlay  *FillOverlayEffect  `xml:"effectLst>fillOverlay"`
	InnerShadow  *InnerShadowEffect  `xml:"effectLst>innerShdw"`
	Glow         *GlowEffect         `xml:"effectLst>glow"`
	SoftEdge     *SoftEdgeEffect     `xml:"effectLst>softEdge"`
	Reflection   *ReflectionEffect   `xml:"effectLst>reflection"`
	Blur         *BlurEffect         `xml:"effectLst>blur"`
	EffectDag    *EffectDag          `xml:"effectDag"`
}

// ColorScheme holds the named theme colors.
type ColorScheme struct {
	Name     string      `xml:"name,attr"`
	DK1      *ThemeColor `xml:"dk1"`
	LT1      *ThemeColor `xml:"lt1"`
	DK2      *ThemeColor `xml:"dk2"`
	LT2      *ThemeColor `xml:"lt2"`
	Accent1  *ThemeColor `xml:"accent1"`
	Accent2  *ThemeColor `xml:"accent2"`
	Accent3  *ThemeColor `xml:"accent3"`
	Accent4  *ThemeColor `xml:"accent4"`
	Accent5  *ThemeColor `xml:"accent5"`
	Accent6  *ThemeColor `xml:"accent6"`
	HLink    *ThemeColor `xml:"hlink"`
	FolHLink *ThemeColor `xml:"folHlink"`
}

// ThemeColor holds a color value (sRGB or sys color).
type ThemeColor struct {
	SrgbClr *SrgbColor `xml:"srgbClr"`
	SysClr  *SysColor  `xml:"sysClr"`
}

// SrgbColor is a hex sRGB color.
type SrgbColor struct {
	Val string `xml:"val,attr"`
}

// SysColor is a system-named color with a last-used RGB fallback.
type SysColor struct {
	Val     string `xml:"val,attr"`
	LastClr string `xml:"lastClr,attr,omitempty"`
}

// FontScheme holds major/minor font references.
type FontScheme struct {
	Name      string    `xml:"name,attr,omitempty"`
	MajorFont FontGroup `xml:"majorFont"`
	MinorFont FontGroup `xml:"minorFont"`
}

// FontGroup holds Latin, East-Asian, complex-script, and per-script fonts.
type FontGroup struct {
	Latin      FontRef     `xml:"latin"`
	EA         FontRef     `xml:"ea"`
	CS         FontRef     `xml:"cs"`
	SupplFonts []SupplFont `xml:"font"`
}

// FontRef holds a single font reference element (e.g. <a:latin typeface="Calibri"/>).
type FontRef struct {
	Typeface string `xml:"typeface,attr"`
}

// SupplFont is a supplemental per-script font.
type SupplFont struct {
	Script   string `xml:"script,attr"`
	Typeface string `xml:"typeface,attr"`
}

// captureRawXMLElementPPTX re-encodes the full element tree (from start through
// its matching end tag) into a byte slice, for deferred OMML conversion.
func captureRawXMLElementPPTX(d *xml.Decoder, start xml.StartElement) ([]byte, error) {
	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	if err := enc.EncodeToken(start); err != nil {
		return nil, err
	}
	depth := 1
	for depth > 0 {
		tok, err := d.Token()
		if err != nil {
			return nil, err
		}
		if err := enc.EncodeToken(tok); err != nil {
			return nil, err
		}
		switch tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		}
	}
	if err := enc.Flush(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
