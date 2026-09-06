package stream

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"

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
				out.IndentLeft = docxIntAttrAny(t, "left", "start")
				out.IndentRight = docxIntAttrAny(t, "right", "end")
				out.IndentFirstLine = docxIntAttr(t, "firstLine")
				out.IndentHanging = docxIntAttr(t, "hanging")
			case "spacing":
				out.SpacingBefore = docxIntAttr(t, "before")
				out.SpacingAfter = docxIntAttr(t, "after")
				out.LineSpacing = docxIntAttr(t, "line")
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
				out.PageWidth = docxIntAttr(t, "w")
				out.PageHeight = docxIntAttr(t, "h")
			case "pgMar":
				out.MarginTop = docxIntAttr(t, "top")
				out.MarginRight = docxIntAttr(t, "right")
				out.MarginBottom = docxIntAttr(t, "bottom")
				out.MarginLeft = docxIntAttr(t, "left")
				out.MarginHeader = docxIntAttr(t, "header")
				out.MarginFooter = docxIntAttr(t, "footer")
			case "cols":
				out.Columns = docxIntAttr(t, "num")
				out.ColumnSpace = docxIntAttr(t, "space")
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
