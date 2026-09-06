package sax

import (
	"encoding/xml"
	"strconv"
)

// ParagraphProperties is the layout-critical subset of w:pPr promoted by the
// streaming parser. Values remain OOXML-native (twips / lexical enums) so the
// stream adapter can map them into the semantic Word IR without coupling SAX
// to pkg/word/ir.
type ParagraphProperties struct {
	StyleID         string
	Alignment       string
	IndentLeft      *int
	IndentRight     *int
	IndentFirstLine *int
	IndentHanging   *int
	SpacingBefore   *int
	SpacingAfter    *int
	LineSpacing     *int
	LineRule        string
	KeepNext        bool
	KeepLines       bool
	PageBreakBefore bool
	WidowControl    *bool
}

// RunProperties is the layout-critical subset of w:rPr promoted by the SAX
// stream. FontSizeHalfPoints is w:sz/@w:val and therefore uses half-points.
type RunProperties struct {
	StyleID            string
	Bold               *bool
	Italic             *bool
	Strike             *bool
	Underline          string
	Color              string
	FontFamily         string
	FontSizeHalfPoints *int
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
	Type         string
	PageWidth    *int
	PageHeight   *int
	MarginTop    *int
	MarginRight  *int
	MarginBottom *int
	MarginLeft   *int
	MarginHeader *int
	MarginFooter *int
	Columns      *int
	ColumnSpace  *int
	TitlePage    bool
	Headers      []HeaderFooterReference
	Footers      []HeaderFooterReference
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
				out.IndentLeft = intAttr(t, "left")
				if out.IndentLeft == nil {
					out.IndentLeft = intAttr(t, "start")
				}
				out.IndentRight = intAttr(t, "right")
				if out.IndentRight == nil {
					out.IndentRight = intAttr(t, "end")
				}
				out.IndentFirstLine = intAttr(t, "firstLine")
				out.IndentHanging = intAttr(t, "hanging")
			case "spacing":
				out.SpacingBefore = intAttr(t, "before")
				out.SpacingAfter = intAttr(t, "after")
				out.LineSpacing = intAttr(t, "line")
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
				out.PageWidth = intAttr(t, "w")
				out.PageHeight = intAttr(t, "h")
			case "pgMar":
				out.MarginTop = intAttr(t, "top")
				out.MarginRight = intAttr(t, "right")
				out.MarginBottom = intAttr(t, "bottom")
				out.MarginLeft = intAttr(t, "left")
				out.MarginHeader = intAttr(t, "header")
				out.MarginFooter = intAttr(t, "footer")
			case "cols":
				out.Columns = intAttr(t, "num")
				out.ColumnSpace = intAttr(t, "space")
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
