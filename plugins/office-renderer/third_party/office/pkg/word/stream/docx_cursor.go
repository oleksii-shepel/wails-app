package stream

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"

	wordsax "office/pkg/word/parsers/ooxml/sax"
)

// docxPropertyCursor walks word/document.xml lazily alongside the structural
// SAX walker. It parses at most one paragraph ahead, preserving the early-stop
// property of bounded page rendering while still promoting pPr/rPr/sectPr.
type docxPropertyCursor struct {
	rc io.ReadCloser
	dec *xml.Decoder

	pendingText []wordsax.RunProperties
	finalSection *wordsax.SectionProperties
	done bool
}

func newDOCXPropertyCursor(r io.ReaderAt, size int64) (*docxPropertyCursor, error) {
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
	cursor := &docxPropertyCursor{rc: rc, dec: xml.NewDecoder(rc)}
	if err := cursor.seekBody(); err != nil {
		_ = rc.Close()
		return nil, err
	}
	return cursor, nil
}

func (c *docxPropertyCursor) Close() error {
	if c == nil || c.rc == nil {
		return nil
	}
	err := c.rc.Close()
	c.rc = nil
	return err
}

func (c *docxPropertyCursor) seekBody() error {
	for {
		token, err := c.dec.Token()
		if err != nil {
			return err
		}
		if start, ok := token.(xml.StartElement); ok && start.Name.Local == "body" {
			return nil
		}
	}
}

func (c *docxPropertyCursor) NextParagraph() (*docxParagraphPlan, error) {
	if c == nil || c.done {
		return nil, nil
	}
	// A well-formed SAX sequence consumes all visible text from the previous
	// paragraph before starting the next. Reset defensively so an atomic token
	// unsupported by the property scanner cannot shift all later formatting.
	c.pendingText = nil

	for {
		token, err := c.dec.Token()
		if err != nil {
			if err == io.EOF {
				c.done = true
				return nil, nil
			}
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "p":
				local := &docxPropertyPlan{}
				paragraph, err := scanDOCXParagraph(c.dec, t, local)
				if err != nil {
					return nil, err
				}
				c.pendingText = local.TextRuns
				return &paragraph, nil
			case "sectPr":
				section, err := scanDOCXSection(c.dec, t)
				if err != nil {
					return nil, err
				}
				c.finalSection = &section
			}
		case xml.EndElement:
			if t.Name.Local == "body" {
				c.done = true
				return nil, nil
			}
		}
	}
}

func (c *docxPropertyCursor) NextTextRun() *wordsax.RunProperties {
	if c == nil || len(c.pendingText) == 0 {
		return nil
	}
	props := c.pendingText[0]
	c.pendingText = c.pendingText[1:]
	return &props
}

func (c *docxPropertyCursor) FinalSection() (*wordsax.SectionProperties, error) {
	if c == nil {
		return nil, nil
	}
	if c.finalSection != nil || c.done {
		return c.finalSection, nil
	}
	for {
		token, err := c.dec.Token()
		if err != nil {
			if err == io.EOF {
				c.done = true
				return c.finalSection, nil
			}
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "sectPr" {
				section, err := scanDOCXSection(c.dec, t)
				if err != nil {
					return nil, err
				}
				c.finalSection = &section
			}
		case xml.EndElement:
			if t.Name.Local == "body" {
				c.done = true
				return c.finalSection, nil
			}
		}
	}
}
