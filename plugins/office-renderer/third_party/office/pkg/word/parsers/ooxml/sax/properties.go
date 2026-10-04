package sax

import (
	"encoding/xml"
	"strconv"

	"office/pkg/ooxml/units"
)

// ParagraphProperties is the layout-critical subset of w:pPr promoted by the
// streaming parser. Values remain OOXML-native (twips / lexical enums) so the
// stream adapter can map them into the semantic Word IR without coupling SAX
// to pkg/word/ir.
type ParagraphProperties struct {
	StyleID           string
	Alignment         string
	IndentLeft        *int
	IndentRight       *int
	IndentFirstLine   *int
	IndentHanging     *int
	SpacingBefore     *int
	SpacingAfter      *int
	LineSpacing       *int
	LineRule          string
	KeepNext          bool
	KeepLines         bool
	PageBreakBefore   bool
	WidowControl      *bool
	SnapToGrid        *bool
	ContextualSpacing bool
	NumberingID       *int
	NumberingLevel    *int
	// Tabs holds custom tab stops from w:tabs > w:tab (position in twips,
	// alignment, leader). TOC styles rely on these for right-aligned page
	// numbers with dot leaders.
	Tabs []TabStop
	// Frame holds w:framePr when present. A framed paragraph is out-of-flow;
	// drop caps in particular must wrap the following text around them.
	Frame *FrameProperties
	// Shading is the resolved w:shd background fill; "auto" is normalized away
	// because it means "no fill" rather than a color.
	Shading string
	// Borders holds w:pBdr sides when the paragraph draws a border.
	Borders *ParagraphBorders
	// MarkRun holds w:pPr/w:rPr, the paragraph mark's direct run properties.
	MarkRun *RunProperties
}

// ParagraphBorderSide is one side of w:pBdr. Size stays in eighths of a point
// so the caller owns the unit conversion.
type ParagraphBorderSide struct {
	Val   string
	Size  int
	Space int
	Color string
}

// ParagraphBorders is the w:pBdr subset promoted by the streaming parser.
type ParagraphBorders struct {
	Top     *ParagraphBorderSide
	Left    *ParagraphBorderSide
	Bottom  *ParagraphBorderSide
	Right   *ParagraphBorderSide
	Between *ParagraphBorderSide
	Bar     *ParagraphBorderSide
}

// TabStop is one w:tab inside w:tabs.
type TabStop struct {
	Pos    int    // twips
	Algn   string // left, center, right, decimal, bar, clear
	Leader string // none, dot, hyphen, underscore, middleDot
}

// FrameProperties is the w:framePr subset promoted by the streaming parser.
// Dimensions and offsets stay in twips.
type FrameProperties struct {
	W       int    // width in twips (0 = auto)
	H       int    // height in twips (0 = auto)
	X       int    // horizontal offset in twips
	Y       int    // vertical offset in twips
	XAlign  string // left, right, center, inside, outside
	YAlign  string // top, bottom, center, inline
	HAnchor string // text, margin, page
	VAnchor string // text, margin, page
	Wrap    string // none, around, notBeside
	HSpace  int    // horizontal space in twips
	VSpace  int    // vertical space in twips
	DropCap string // drop, margin, none
	Lines   int    // body lines spanned by the drop cap (w:lines)
}

// RunProperties is the layout-critical subset of w:rPr promoted by the SAX
// stream. FontSizeHalfPoints is w:sz/@w:val and therefore uses half-points.
type RunProperties struct {
	StyleID              string
	Bold                 *bool
	Italic               *bool
	Strike               *bool
	Underline            string
	Color                string
	FontFamily           string
	FontSizeHalfPoints   *int
	FontSizeCSHalfPoints *int
	BoldCS               *bool
	CharacterSpacing     *int
	KerningHalfPoints    *int
	// Highlight is the w:highlight/@w:val color name ("yellow", "none", …).
	Highlight string
	// ShadingFill is the resolved w:shd background fill; "auto" is normalized
	// away because it means "no fill" rather than a color.
	ShadingFill string
	// VertAlign is w:vertAlign/@w:val: "superscript" | "subscript" | "baseline".
	VertAlign string
	// Position is w:position in half-points; positive raises, negative lowers.
	Position     *int
	Caps         *bool
	SmallCaps    *bool
	DoubleStrike *bool
	Vanish       *bool
	// SpecVanish is w:specVanish: the content is never displayed, even when
	// formatting marks are shown (list paragraph marks use it).
	SpecVanish *bool
	RTL        *bool
	// EmphasisMark is w:em/@w:val: "none" | "dot" | "comma" | "circle" | "underDot".
	EmphasisMark string
	// Border is w:bdr, the character border applied to every side of the run.
	Border *RunBorder
}

// RunBorder is the w:bdr subset promoted by the SAX run-property scanner. Size
// stays in eighths of a point so the caller owns the unit conversion.
type RunBorder struct {
	Val   string
	Size  int
	Space int
	Color string
}

// HeaderFooterReference is a format-neutral section story reference. ID is the
// relationship identifier in the main document part; Type is first/default/even.
type HeaderFooterReference struct {
	Type string
	ID   string
}

// SectionProperties is the page-layout subset of w:sectPr required by bounded
// pagination. All dimensions remain in twips. Header/footer references stay as
// relationship IDs until the DOCX stream adapter resolves their package parts.
type SectionProperties struct {
	Type             string
	PageWidth        *int
	PageHeight       *int
	MarginTop        *int
	MarginRight      *int
	MarginBottom     *int
	MarginLeft       *int
	MarginHeader     *int
	MarginFooter     *int
	Columns          *int
	ColumnSpace      *int
	TitlePage        bool
	DocGridType      string
	DocGridLinePitch *int
	DocGridCharSpace *int
	Headers          []HeaderFooterReference
	Footers          []HeaderFooterReference
}

func parseParagraphPropertiesRich(dec *xml.Decoder, start xml.StartElement) (ParagraphProperties, error) {
	var out ParagraphProperties
	for {
		token, err := dec.Token()
		if err != nil {
			return out, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "pStyle":
				out.StyleID = attrValue(t, "val")
			case "jc":
				out.Alignment = attrValue(t, "val")
			case "ind":
				out.IndentLeft = twipsAttr(t, "left")
				if out.IndentLeft == nil {
					out.IndentLeft = twipsAttr(t, "start")
				}
				out.IndentRight = twipsAttr(t, "right")
				if out.IndentRight == nil {
					out.IndentRight = twipsAttr(t, "end")
				}
				out.IndentFirstLine = twipsAttr(t, "firstLine")
				out.IndentHanging = twipsAttr(t, "hanging")
			case "spacing":
				out.SpacingBefore = twipsAttr(t, "before")
				out.SpacingAfter = twipsAttr(t, "after")
				out.LineSpacing = twipsAttr(t, "line")
				out.LineRule = attrValue(t, "lineRule")
			case "keepNext":
				out.KeepNext = onOffElement(t, true)
			case "keepLines":
				out.KeepLines = onOffElement(t, true)
			case "pageBreakBefore":
				out.PageBreakBefore = onOffElement(t, true)
			case "widowControl":
				v := onOffElement(t, true)
				out.WidowControl = &v
			case "snapToGrid":
				v := onOffElement(t, true)
				out.SnapToGrid = &v
			case "contextualSpacing":
				out.ContextualSpacing = onOffElement(t, true)
			case "numPr":
				id, level, err := parseParagraphNumberingProperties(dec, t)
				if err != nil {
					return out, err
				}
				out.NumberingID, out.NumberingLevel = id, level
				continue
			}
			if err := dec.Skip(); err != nil {
				return out, err
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return out, nil
			}
		}
	}
}

func parseParagraphNumberingProperties(dec *xml.Decoder, start xml.StartElement) (*int, *int, error) {
	var id, level *int
	for {
		token, err := dec.Token()
		if err != nil {
			return nil, nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "numId":
				id = intAttr(t, "val")
			case "ilvl":
				level = intAttr(t, "val")
			}
			if err := dec.Skip(); err != nil {
				return nil, nil, err
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return id, level, nil
			}
		}
	}
}

func parseRunProperties(dec *xml.Decoder, start xml.StartElement) (RunProperties, error) {
	var out RunProperties
	for {
		token, err := dec.Token()
		if err != nil {
			return out, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "rStyle":
				out.StyleID = attrValue(t, "val")
			case "b":
				v := onOffElement(t, true)
				out.Bold = &v
			case "bCs":
				v := onOffElement(t, true)
				out.BoldCS = &v
			case "i":
				v := onOffElement(t, true)
				out.Italic = &v
			case "strike":
				v := onOffElement(t, true)
				out.Strike = &v
			case "u":
				out.Underline = attrValue(t, "val")
				if out.Underline == "" {
					out.Underline = "single"
				}
			case "color":
				out.Color = attrValue(t, "val")
			case "rFonts":
				out.FontFamily = attrValue(t, "ascii")
				if out.FontFamily == "" {
					out.FontFamily = attrValue(t, "hAnsi")
				}
			case "sz":
				out.FontSizeHalfPoints = intAttr(t, "val")
			case "szCs":
				out.FontSizeCSHalfPoints = intAttr(t, "val")
			case "spacing":
				out.CharacterSpacing = intAttr(t, "val")
			case "kern":
				out.KerningHalfPoints = intAttr(t, "val")
			}
			if err := dec.Skip(); err != nil {
				return out, err
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return out, nil
			}
		}
	}
}

func parseSectionProperties(dec *xml.Decoder, start xml.StartElement, onHeader func(string), onFooter func(string)) (SectionProperties, error) {
	var out SectionProperties
	for {
		token, err := dec.Token()
		if err != nil {
			return out, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "headerReference":
				id := attrValue(t, "id")
				out.Headers = append(out.Headers, HeaderFooterReference{Type: attrValue(t, "type"), ID: id})
				if onHeader != nil {
					onHeader(id)
				}
			case "footerReference":
				id := attrValue(t, "id")
				out.Footers = append(out.Footers, HeaderFooterReference{Type: attrValue(t, "type"), ID: id})
				if onFooter != nil {
					onFooter(id)
				}
			case "type":
				out.Type = attrValue(t, "val")
			case "pgSz":
				out.PageWidth = twipsAttr(t, "w")
				out.PageHeight = twipsAttr(t, "h")
			case "pgMar":
				out.MarginTop = twipsAttr(t, "top")
				out.MarginRight = twipsAttr(t, "right")
				out.MarginBottom = twipsAttr(t, "bottom")
				out.MarginLeft = twipsAttr(t, "left")
				out.MarginHeader = twipsAttr(t, "header")
				out.MarginFooter = twipsAttr(t, "footer")
			case "cols":
				out.Columns = intAttr(t, "num")
				out.ColumnSpace = twipsAttr(t, "space")
			case "docGrid":
				out.DocGridType = attrValue(t, "type")
				out.DocGridLinePitch = twipsAttr(t, "linePitch")
				out.DocGridCharSpace = intAttr(t, "charSpace")
			case "titlePg":
				out.TitlePage = onOffElement(t, true)
			}
			if err := dec.Skip(); err != nil {
				return out, err
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return out, nil
			}
		}
	}
}

// intAttr reads an integer property. Values that are not plain integers are
// left unset; attributes that hold a length should use twipsAttr instead.
func intAttr(start xml.StartElement, local string) *int {
	value := attrValue(start, local)
	if value == "" {
		return nil
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return nil
	}
	return &n
}

// twipsAttr reads a measurement attribute. Transitional documents store a bare
// count of twentieths of a point, while the strict vocabulary writes an
// ST_UniversalMeasure ("612pt", "1in"), so the unit is resolved here instead of
// dropping an authored page size, indent or spacing. Attributes measured in
// other units (half-point font sizes, eighth-point border widths) must keep
// using intAttr so their values are not rescaled.
func twipsAttr(start xml.StartElement, local string) *int {
	value := attrValue(start, local)
	if value == "" {
		return nil
	}
	twips, ok := units.ParseMeasureTwips(value)
	if !ok {
		return nil
	}
	n := int(twips)
	return &n
}

func onOffElement(start xml.StartElement, defaultValue bool) bool {
	value := attrValue(start, "val")
	if value == "" {
		return defaultValue
	}
	switch value {
	case "0", "false", "off", "no":
		return false
	default:
		return true
	}
}
