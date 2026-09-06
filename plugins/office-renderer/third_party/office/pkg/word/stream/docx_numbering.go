package stream

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"

	wordir "office/pkg/word/ir"
)

// docxNumberingCursor walks only paragraph properties from document.xml and
// returns one direct numPr reference per paragraph. It advances lazily with the
// SAX body stream, so bounded page reads do not scan the remaining document.
type docxNumberingCursor struct {
	dec   *xml.Decoder
	close io.Closer
}

func newDOCXNumberingCursor(r io.ReaderAt, size int64) (*docxNumberingCursor, error) {
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
	return &docxNumberingCursor{dec: xml.NewDecoder(rc), close: rc}, nil
}

func (c *docxNumberingCursor) Close() error {
	if c == nil || c.close == nil {
		return nil
	}
	return c.close.Close()
}

func (c *docxNumberingCursor) NextParagraph() (*wordir.NumberingReference, error) {
	if c == nil || c.dec == nil {
		return nil, nil
	}
	for {
		token, err := c.dec.Token()
		if err != nil {
			if err == io.EOF {
				return nil, nil
			}
			return nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "p" {
			continue
		}
		return scanDOCXParagraphNumbering(c.dec, start)
	}
}

func scanDOCXParagraphNumbering(dec *xml.Decoder, paragraph xml.StartElement) (*wordir.NumberingReference, error) {
	for {
		token, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "pPr" {
				return scanDOCXPPrNumbering(dec, t)
			}
			if err := dec.Skip(); err != nil {
				return nil, err
			}
		case xml.EndElement:
			if t.Name == paragraph.Name {
				return nil, nil
			}
		}
	}
}

func scanDOCXPPrNumbering(dec *xml.Decoder, ppr xml.StartElement) (*wordir.NumberingReference, error) {
	for {
		token, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "numPr" {
				return scanDOCXNumPr(dec, t)
			}
			if err := dec.Skip(); err != nil {
				return nil, err
			}
		case xml.EndElement:
			if t.Name == ppr.Name {
				return nil, nil
			}
		}
	}
}

func scanDOCXNumPr(dec *xml.Decoder, numPr xml.StartElement) (*wordir.NumberingReference, error) {
	ref := wordir.NumberingReference{}
	seen := false
	for {
		token, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			value, _ := strconv.Atoi(docxAttr(t, "val"))
			switch t.Name.Local {
			case "numId":
				ref.NumID = value
				seen = true
			case "ilvl":
				ref.Level = value
			}
			if err := dec.Skip(); err != nil {
				return nil, err
			}
		case xml.EndElement:
			if t.Name == numPr.Name {
				if !seen {
					return nil, nil
				}
				return &ref, nil
			}
		}
	}
}
