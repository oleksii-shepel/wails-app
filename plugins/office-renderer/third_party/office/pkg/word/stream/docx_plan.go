package stream

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"

	"office/pkg/ooxml/units"
	wordsax "office/pkg/word/parsers/ooxml/sax"
)

type docxParagraphPlan struct {
	Properties wordsax.ParagraphProperties
	Section    *wordsax.SectionProperties
}

type docxPropertyPlan struct {
	Paragraphs   []docxParagraphPlan
	TextRuns     []wordsax.RunProperties
	FinalSection *wordsax.SectionProperties
}

func scanDOCXPropertyPlan(r io.ReaderAt, size int64) (*docxPropertyPlan, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}
	files := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		files[normalizeDOCXPartName(f.Name)] = f
	}
	mainPart, err := findDOCXMainPart(files)
	if err != nil {
		return nil, err
	}
	file := files[normalizeDOCXPartName(mainPart)]
	if file == nil {
		return nil, fmt.Errorf("word stream: main DOCX part %q not found", mainPart)
	}
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	plan := &docxPropertyPlan{}
	dec := xml.NewDecoder(rc)
	for {
		token, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				return plan, nil
			}
			return nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local == "body" {
			if err := scanDOCXBody(dec, start, plan); err != nil {
				return nil, err
			}
			return plan, nil
		}
	}
}

func scanDOCXBody(dec *xml.Decoder, end xml.StartElement, plan *docxPropertyPlan) error {
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "p":
				paragraph, err := scanDOCXParagraph(dec, t, plan)
				if err != nil {
					return err
				}
				plan.Paragraphs = append(plan.Paragraphs, paragraph)
			case "sectPr":
				section, err := scanDOCXSection(dec, t)
				if err != nil {
					return err
				}
				plan.FinalSection = &section
			default:
				if err := scanDOCXBody(dec, t, plan); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if t.Name == end.Name {
				return nil
			}
		}
	}
}

func scanDOCXParagraph(dec *xml.Decoder, end xml.StartElement, plan *docxPropertyPlan) (docxParagraphPlan, error) {
	var out docxParagraphPlan
	for {
		token, err := dec.Token()
		if err != nil {
			return out, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "pPr":
				props, section, err := scanDOCXParagraphProperties(dec, t)
				if err != nil {
					return out, err
				}
				out.Properties = props
				out.Section = section
			case "r":
				if err := scanDOCXRun(dec, t, plan); err != nil {
					return out, err
				}
			default:
				if err := scanDOCXParagraphInline(dec, t, plan); err != nil {
					return out, err
				}
			}
		case xml.EndElement:
			if t.Name == end.Name {
				return out, nil
			}
		}
	}
}

func scanDOCXParagraphInline(dec *xml.Decoder, end xml.StartElement, plan *docxPropertyPlan) error {
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "r" {
				if err := scanDOCXRun(dec, t, plan); err != nil {
					return err
				}
			} else if err := scanDOCXParagraphInline(dec, t, plan); err != nil {
				return err
			}
		case xml.EndElement:
			if t.Name == end.Name {
				return nil
			}
		}
	}
}

func scanDOCXRun(dec *xml.Decoder, end xml.StartElement, plan *docxPropertyPlan) error {
	var props wordsax.RunProperties
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "rPr":
				p, err := scanDOCXRunProperties(dec, t)
				if err != nil {
					return err
				}
				props = p
			case "t", "delText":
				var text string
				if err := dec.DecodeElement(&text, &t); err != nil {
					return err
				}
				if text != "" {
					plan.TextRuns = append(plan.TextRuns, props)
				}
			default:
				if err := dec.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if t.Name == end.Name {
				return nil
			}
		}
	}
}

func scanDOCXParagraphProperties(dec *xml.Decoder, end xml.StartElement) (wordsax.ParagraphProperties, *wordsax.SectionProperties, error) {
	var out wordsax.ParagraphProperties
	var section *wordsax.SectionProperties
	for {
		token, err := dec.Token()
		if err != nil {
			return out, section, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "pStyle":
				out.StyleID = docxAttr(t, "val")
			case "jc":
				out.Alignment = docxAttr(t, "val")
			case "ind":
				out.IndentLeft = docxTwipsAttrAny(t, "left", "start")
				out.IndentRight = docxTwipsAttrAny(t, "right", "end")
				out.IndentFirstLine = docxTwipsAttr(t, "firstLine")
				out.IndentHanging = docxTwipsAttr(t, "hanging")
			case "spacing":
				out.SpacingBefore = docxTwipsAttr(t, "before")
				out.SpacingAfter = docxTwipsAttr(t, "after")
				out.LineSpacing = docxTwipsAttr(t, "line")
				out.LineRule = docxAttr(t, "lineRule")
			case "keepNext":
				out.KeepNext = docxOnOff(t, true)
			case "keepLines":
				out.KeepLines = docxOnOff(t, true)
			case "pageBreakBefore":
				out.PageBreakBefore = docxOnOff(t, true)
			case "widowControl":
				v := docxOnOff(t, true)
				out.WidowControl = &v
			case "numPr":
				id, level, err := scanDOCXParagraphNumberingProperties(dec, t)
				if err != nil {
					return out, section, err
				}
				out.NumberingID, out.NumberingLevel = id, level
				continue
			case "tabs":
				tabs, err := scanDOCXParagraphTabs(dec, t)
				if err != nil {
					return out, section, err
				}
				out.Tabs = tabs
				continue
			case "shd":
				if fill := docxShadingFill(t); fill != "" {
					out.Shading = fill
				}
			case "pBdr":
				borders, err := scanDOCXParagraphBorders(dec, t)
				if err != nil {
					return out, section, err
				}
				out.Borders = borders
				continue
			case "framePr":
				frame := wordsax.FrameProperties{
					W:       docxIntValueOf(docxAttr(t, "w")),
					H:       docxIntValueOf(docxAttr(t, "h")),
					X:       docxIntValueOf(docxAttr(t, "x")),
					Y:       docxIntValueOf(docxAttr(t, "y")),
					XAlign:  docxAttr(t, "xAlign"),
					YAlign:  docxAttr(t, "yAlign"),
					HAnchor: docxAttr(t, "hAnchor"),
					VAnchor: docxAttr(t, "vAnchor"),
					Wrap:    docxAttr(t, "wrap"),
					HSpace:  docxIntValueOf(docxAttr(t, "hSpace")),
					VSpace:  docxIntValueOf(docxAttr(t, "vSpace")),
					DropCap: docxAttr(t, "dropCap"),
					Lines:   docxIntValueOf(docxAttr(t, "lines")),
				}
				out.Frame = &frame
			case "rPr":
				mark, err := scanDOCXRunProperties(dec, t)
				if err != nil {
					return out, section, err
				}
				out.MarkRun = &mark
				continue
			case "sectPr":
				s, err := scanDOCXSection(dec, t)
				if err != nil {
					return out, section, err
				}
				section = &s
				continue
			}
			if err := dec.Skip(); err != nil {
				return out, section, err
			}
		case xml.EndElement:
			if t.Name == end.Name {
				return out, section, nil
			}
		}
	}
}

func scanDOCXParagraphTabs(dec *xml.Decoder, end xml.StartElement) ([]wordsax.TabStop, error) {
	var tabs []wordsax.TabStop
	for {
		token, err := dec.Token()
		if err != nil {
			return tabs, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "tab" {
				stop := wordsax.TabStop{Algn: docxAttr(t, "val"), Leader: docxAttr(t, "leader")}
				if pos := docxTwipsAttr(t, "pos"); pos != nil {
					stop.Pos = *pos
				}
				tabs = append(tabs, stop)
			}
			if err := dec.Skip(); err != nil {
				return tabs, err
			}
		case xml.EndElement:
			if t.Name == end.Name {
				return tabs, nil
			}
		}
	}
}

func scanDOCXParagraphNumberingProperties(dec *xml.Decoder, end xml.StartElement) (*int, *int, error) {
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
				id = docxIntAttr(t, "val")
			case "ilvl":
				level = docxIntAttr(t, "val")
			}
			if err := dec.Skip(); err != nil {
				return nil, nil, err
			}
		case xml.EndElement:
			if t.Name == end.Name {
				return id, level, nil
			}
		}
	}
}

func scanDOCXRunProperties(dec *xml.Decoder, end xml.StartElement) (wordsax.RunProperties, error) {
	var out wordsax.RunProperties
	for {
		token, err := dec.Token()
		if err != nil {
			return out, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "rStyle":
				out.StyleID = docxAttr(t, "val")
			case "b":
				v := docxOnOff(t, true)
				out.Bold = &v
			case "bCs":
				v := docxOnOff(t, true)
				out.BoldCS = &v
			case "i":
				v := docxOnOff(t, true)
				out.Italic = &v
			case "strike":
				v := docxOnOff(t, true)
				out.Strike = &v
			case "u":
				out.Underline = docxAttr(t, "val")
				if out.Underline == "" {
					out.Underline = "single"
				}
			case "color":
				out.Color = docxAttr(t, "val")
			case "rFonts":
				out.FontFamily = docxAttr(t, "ascii")
				if out.FontFamily == "" {
					out.FontFamily = docxAttr(t, "hAnsi")
				}
			case "sz":
				out.FontSizeHalfPoints = docxIntAttr(t, "val")
			case "szCs":
				out.FontSizeCSHalfPoints = docxIntAttr(t, "val")
			case "spacing":
				out.CharacterSpacing = docxIntAttr(t, "val")
			case "kern":
				out.KerningHalfPoints = docxIntAttr(t, "val")
			case "highlight":
				out.Highlight = docxAttr(t, "val")
			case "shd":
				out.ShadingFill = docxShadingFill(t)
			case "vertAlign":
				out.VertAlign = docxAttr(t, "val")
			case "position":
				out.Position = docxIntAttr(t, "val")
			case "caps":
				v := docxOnOff(t, true)
				out.Caps = &v
			case "smallCaps":
				v := docxOnOff(t, true)
				out.SmallCaps = &v
			case "dstrike":
				v := docxOnOff(t, true)
				out.DoubleStrike = &v
			case "vanish":
				v := docxOnOff(t, true)
				out.Vanish = &v
			case "specVanish":
				v := docxOnOff(t, true)
				out.SpecVanish = &v
			case "rtl":
				v := docxOnOff(t, true)
				out.RTL = &v
			case "em":
				out.EmphasisMark = docxAttr(t, "val")
			case "bdr":
				border := &wordsax.RunBorder{
					Val:   docxAttr(t, "val"),
					Color: docxAttr(t, "color"),
				}
				if border.Val == "" {
					border.Val = "single"
				}
				if size := docxIntAttr(t, "sz"); size != nil {
					border.Size = *size
				}
				if space := docxIntAttr(t, "space"); space != nil {
					border.Space = *space
				}
				out.Border = border
			}
			if err := dec.Skip(); err != nil {
				return out, err
			}
		case xml.EndElement:
			if t.Name == end.Name {
				return out, nil
			}
		}
	}
}

// docxShadingFill resolves a w:shd background fill. w:fill carries the color;
// Word also accepts w:color when the pattern is a solid fill, so fall back to
// it. "auto" means "no shading" and must stay distinguishable from unset.
func docxShadingFill(start xml.StartElement) string {
	fill := docxAttr(start, "fill")
	if fill == "" || fill == "auto" {
		fill = docxAttr(start, "color")
	}
	if fill == "auto" {
		return ""
	}
	return fill
}

// scanDOCXParagraphBorders reads w:pBdr sides. Each side is a self-closing
// element, so the decoder is drained until pBdr closes.
func scanDOCXParagraphBorders(dec *xml.Decoder, end xml.StartElement) (*wordsax.ParagraphBorders, error) {
	borders := &wordsax.ParagraphBorders{}
	assign := func(name string, side *wordsax.ParagraphBorderSide) {
		switch name {
		case "top":
			borders.Top = side
		case "left", "start":
			if borders.Left == nil {
				borders.Left = side
			}
		case "bottom":
			borders.Bottom = side
		case "right", "end":
			if borders.Right == nil {
				borders.Right = side
			}
		case "between":
			borders.Between = side
		case "bar":
			borders.Bar = side
		}
	}
	for {
		token, err := dec.Token()
		if err != nil {
			return borders, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			assign(t.Name.Local, docxBorderSide(t))
			if err := dec.Skip(); err != nil {
				return borders, err
			}
		case xml.EndElement:
			if t.Name == end.Name {
				return borders, nil
			}
		}
	}
}

func docxBorderSide(start xml.StartElement) *wordsax.ParagraphBorderSide {
	side := &wordsax.ParagraphBorderSide{
		Val:   docxAttr(start, "val"),
		Color: docxAttr(start, "color"),
	}
	if side.Val == "" {
		side.Val = "single"
	}
	if size := docxIntAttr(start, "sz"); size != nil {
		side.Size = *size
	}
	if space := docxIntAttr(start, "space"); space != nil {
		side.Space = *space
	}
	return side
}

func scanDOCXSection(dec *xml.Decoder, end xml.StartElement) (wordsax.SectionProperties, error) {
	var out wordsax.SectionProperties
	for {
		token, err := dec.Token()
		if err != nil {
			return out, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "type":
				out.Type = docxAttr(t, "val")
			case "pgSz":
				out.PageWidth = docxTwipsAttr(t, "w")
				out.PageHeight = docxTwipsAttr(t, "h")
			case "pgMar":
				out.MarginTop = docxTwipsAttr(t, "top")
				out.MarginRight = docxTwipsAttr(t, "right")
				out.MarginBottom = docxTwipsAttr(t, "bottom")
				out.MarginLeft = docxTwipsAttr(t, "left")
				out.MarginHeader = docxTwipsAttr(t, "header")
				out.MarginFooter = docxTwipsAttr(t, "footer")
			case "cols":
				out.Columns = docxIntAttr(t, "num")
				out.ColumnSpace = docxTwipsAttr(t, "space")
			case "docGrid":
				// Word snaps every line to the document grid by default, so the
				// pitch decides how many lines a page holds. Dropping it makes
				// the streamed document page denser than the source.
				out.DocGridType = docxAttr(t, "type")
				out.DocGridLinePitch = docxTwipsAttr(t, "linePitch")
				out.DocGridCharSpace = docxIntAttr(t, "charSpace")
			case "titlePg":
				out.TitlePage = docxOnOff(t, true)
			}
			if err := dec.Skip(); err != nil {
				return out, err
			}
		case xml.EndElement:
			if t.Name == end.Name {
				return out, nil
			}
		}
	}
}

func findDOCXMainPart(files map[string]*zip.File) (string, error) {
	root := files["_rels/.rels"]
	if root == nil {
		if files["word/document.xml"] != nil {
			return "word/document.xml", nil
		}
		return "", fmt.Errorf("word stream: root relationships not found")
	}
	rc, err := root.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()
	dec := xml.NewDecoder(rc)
	for {
		token, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "Relationship" {
			continue
		}
		if strings.HasSuffix(docxAttr(start, "Type"), "/officeDocument") {
			target := docxAttr(start, "Target")
			if target != "" {
				return normalizeDOCXPartName(target), nil
			}
		}
	}
	if files["word/document.xml"] != nil {
		return "word/document.xml", nil
	}
	return "", fmt.Errorf("word stream: officeDocument relationship not found")
}

func normalizeDOCXPartName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = strings.TrimPrefix(name, "/")
	name = path.Clean(name)
	if name == "." {
		return ""
	}
	return strings.ToLower(name)
}

func docxAttr(start xml.StartElement, local string) string {
	for _, attr := range start.Attr {
		if attr.Name.Local == local {
			return attr.Value
		}
	}
	return ""
}

// docxIntAttr reads an integer property. Values that are not plain integers are
// left unset; attributes that hold a length should use docxTwipsAttr instead.
func docxIntAttr(start xml.StartElement, local string) *int {
	value := docxAttr(start, local)
	if value == "" {
		return nil
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return nil
	}
	return &n
}

// docxTwipsAttr reads a measurement attribute. Transitional documents store a
// bare count of twentieths of a point, while the strict vocabulary writes an
// ST_UniversalMeasure ("612pt", "1in"), so the unit is resolved here instead of
// dropping an authored page size, indent or spacing. Attributes measured in
// other units (half-point font sizes, eighth-point border widths) must keep
// using docxIntAttr so their values are not rescaled.
func docxTwipsAttr(start xml.StartElement, local string) *int {
	value := docxAttr(start, local)
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

func docxTwipsAttrAny(start xml.StartElement, names ...string) *int {
	for _, name := range names {
		if v := docxTwipsAttr(start, name); v != nil {
			return v
		}
	}
	return nil
}

func docxIntAttrAny(start xml.StartElement, names ...string) *int {
	for _, name := range names {
		if v := docxIntAttr(start, name); v != nil {
			return v
		}
	}
	return nil
}

func docxOnOff(start xml.StartElement, defaultValue bool) bool {
	value := strings.ToLower(strings.TrimSpace(docxAttr(start, "val")))
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

func docxIntValueOf(value string) int {
	if value == "" {
		return 0
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return n
}
