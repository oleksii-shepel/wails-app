package stream

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"

	wordir "office/pkg/word/ir"
)

type docxNoteLoader struct {
	reader io.ReaderAt
	size   int64
	parts  map[string]*zip.File
	cache  map[string]map[int]wordir.Story
}

func newDOCXNoteLoader(r io.ReaderAt, size int64) (*docxNoteLoader, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}
	parts := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		parts[normalizeDOCXPartName(f.Name)] = f
	}
	return &docxNoteLoader{
		reader: r,
		size:   size,
		parts:  parts,
		cache:  make(map[string]map[int]wordir.Story),
	}, nil
}

func (l *docxNoteLoader) Load(kind string, id int) (wordir.Story, bool, error) {
	if l == nil {
		return wordir.Story{}, false, nil
	}
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind != "footnote" && kind != "endnote" {
		return wordir.Story{}, false, fmt.Errorf("word stream: unsupported note kind %q", kind)
	}
	if stories, ok := l.cache[kind]; ok {
		story, found := stories[id]
		return story, found, nil
	}

	partName := l.notePartName(kind)
	if partName == "" {
		l.cache[kind] = map[int]wordir.Story{}
		return wordir.Story{}, false, nil
	}
	file := l.parts[normalizeDOCXPartName(partName)]
	if file == nil {
		l.cache[kind] = map[int]wordir.Story{}
		return wordir.Story{}, false, nil
	}
	rc, err := file.Open()
	if err != nil {
		return wordir.Story{}, false, err
	}
	defer rc.Close()
	stories, err := parseDOCXNoteStories(rc, kind)
	if err != nil {
		return wordir.Story{}, false, err
	}
	l.cache[kind] = stories
	story, found := stories[id]
	return story, found, nil
}

func (l *docxNoteLoader) notePartName(kind string) string {
	mainPart, err := findDOCXMainPart(l.parts)
	if err != nil {
		mainPart = "word/document.xml"
	}
	relsName := normalizeDOCXPartName(path.Join(path.Dir(mainPart), "_rels", path.Base(mainPart)+".rels"))
	if relsFile := l.parts[relsName]; relsFile != nil {
		if rc, err := relsFile.Open(); err == nil {
			defer rc.Close()
			dec := xml.NewDecoder(rc)
			for {
				token, err := dec.Token()
				if err != nil {
					break
				}
				start, ok := token.(xml.StartElement)
				if !ok || start.Name.Local != "Relationship" {
					continue
				}
				typ := strings.ToLower(docxAttr(start, "Type"))
				if strings.HasSuffix(typ, "/"+kind+"s") {
					target := docxAttr(start, "Target")
					if target != "" {
						return normalizeDOCXPartName(path.Join(path.Dir(mainPart), target))
					}
				}
			}
		}
	}
	fallback := "word/" + kind + "s.xml"
	if l.parts[fallback] != nil {
		return fallback
	}
	return ""
}

func parseDOCXNoteStories(r io.Reader, kind string) (map[int]wordir.Story, error) {
	out := make(map[int]wordir.Story)
	dec := xml.NewDecoder(r)
	noteElement := kind
	for {
		token, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				return out, nil
			}
			return nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != noteElement {
			continue
		}
		noteType := docxAttr(start, "type")
		if noteType != "" && noteType != "normal" {
			if err := dec.Skip(); err != nil {
				return nil, err
			}
			continue
		}
		id, err := strconv.Atoi(docxAttr(start, "id"))
		if err != nil || id < 0 {
			if err := dec.Skip(); err != nil {
				return nil, err
			}
			continue
		}
		story, err := parseDOCXNoteStory(dec, start, id)
		if err != nil {
			return nil, err
		}
		out[id] = story
	}
}

func parseDOCXNoteStory(dec *xml.Decoder, end xml.StartElement, noteID int) (wordir.Story, error) {
	var story wordir.Story
	for {
		token, err := dec.Token()
		if err != nil {
			return story, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "p":
				p, err := parseDOCXNoteParagraph(dec, t, noteID)
				if err != nil {
					return story, err
				}
				story.Paragraphs = append(story.Paragraphs, p)
				copyP := p
				story.Blocks = append(story.Blocks, wordir.Block{Kind: "paragraph", Paragraph: &copyP})
				story.Text += p.Text
			case "tbl":
				table, err := parseDOCXNoteTable(dec, t, noteID)
				if err != nil {
					return story, err
				}
				story.Tables = append(story.Tables, table)
				copyT := table
				story.Blocks = append(story.Blocks, wordir.Block{Kind: "table", Table: &copyT})
				story.Text += table.Text
			default:
				if err := dec.Skip(); err != nil {
					return story, err
				}
			}
		case xml.EndElement:
			if t.Name == end.Name {
				return story, nil
			}
		}
	}
}

func parseDOCXNoteParagraph(dec *xml.Decoder, end xml.StartElement, noteID int) (wordir.Paragraph, error) {
	var p wordir.Paragraph
	for {
		token, err := dec.Token()
		if err != nil {
			return p, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "pPr":
				props, _, err := scanDOCXParagraphProperties(dec, t)
				if err != nil {
					return p, err
				}
				p.StyleID = props.StyleID
				p.Format = paragraphFormatFromSAX(props)
			case "r":
				runs, text, err := parseDOCXNoteRun(dec, t, noteID)
				if err != nil {
					return p, err
				}
				p.Runs = append(p.Runs, runs...)
				p.Text += text
			default:
				if err := dec.Skip(); err != nil {
					return p, err
				}
			}
		case xml.EndElement:
			if t.Name == end.Name {
				return p, nil
			}
		}
	}
}

func parseDOCXNoteRun(dec *xml.Decoder, end xml.StartElement, noteID int) ([]wordir.Run, string, error) {
	var propsFormat wordir.RunFormat
	var styleID string
	var runs []wordir.Run
	var text strings.Builder
	for {
		token, err := dec.Token()
		if err != nil {
			return nil, "", err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "rPr":
				props, err := scanDOCXRunProperties(dec, t)
				if err != nil {
					return nil, "", err
				}
				styleID = props.StyleID
				propsFormat = runFormatFromSAX(props)
			case "footnoteRef", "endnoteRef":
				// The note mark that Word prints before the note text. The
				// renderer resolves its display number from the document's note
				// numbering state, so the run only needs the kind and the id.
				kind := "footnoteRef"
				if t.Name.Local == "endnoteRef" {
					kind = "endnoteRef"
				}
				runs = append(runs, wordir.Run{Kind: kind, NoteID: noteID, StyleID: styleID, Format: propsFormat})
				if err := dec.Skip(); err != nil {
					return nil, "", err
				}
			case "t", "delText":
				var value string
				if err := dec.DecodeElement(&value, &t); err != nil {
					return nil, "", err
				}
				if value != "" {
					runs = append(runs, wordir.Run{Kind: "run", Text: value, StyleID: styleID, Format: propsFormat})
					text.WriteString(value)
				}
			case "tab":
				runs = append(runs, wordir.Run{Kind: "tab", StyleID: styleID, Format: propsFormat})
				text.WriteByte('\t')
				if err := dec.Skip(); err != nil {
					return nil, "", err
				}
			case "br":
				breakType := docxAttr(t, "type")
				if breakType == "" {
					breakType = "textWrapping"
				}
				runs = append(runs, wordir.Run{Kind: "break", BreakType: breakType, StyleID: styleID, Format: propsFormat})
				if err := dec.Skip(); err != nil {
					return nil, "", err
				}
			default:
				if err := dec.Skip(); err != nil {
					return nil, "", err
				}
			}
		case xml.EndElement:
			if t.Name == end.Name {
				return runs, text.String(), nil
			}
		}
	}
}

func parseDOCXNoteTable(dec *xml.Decoder, end xml.StartElement, noteID int) (wordir.Table, error) {
	var table wordir.Table
	for {
		token, err := dec.Token()
		if err != nil {
			return table, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local != "tr" {
				if err := dec.Skip(); err != nil {
					return table, err
				}
				continue
			}
			row, err := parseDOCXNoteRow(dec, t, noteID)
			if err != nil {
				return table, err
			}
			table.Rows = append(table.Rows, row)
			for _, cell := range row.Cells {
				table.Text += cell.Text
			}
		case xml.EndElement:
			if t.Name == end.Name {
				return table, nil
			}
		}
	}
}

func parseDOCXNoteRow(dec *xml.Decoder, end xml.StartElement, noteID int) (wordir.TableRow, error) {
	var row wordir.TableRow
	for {
		token, err := dec.Token()
		if err != nil {
			return row, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local != "tc" {
				if err := dec.Skip(); err != nil {
					return row, err
				}
				continue
			}
			cell, err := parseDOCXNoteCell(dec, t, noteID)
			if err != nil {
				return row, err
			}
			row.Cells = append(row.Cells, cell)
		case xml.EndElement:
			if t.Name == end.Name {
				return row, nil
			}
		}
	}
}

func parseDOCXNoteCell(dec *xml.Decoder, end xml.StartElement, noteID int) (wordir.TableCell, error) {
	var cell wordir.TableCell
	for {
		token, err := dec.Token()
		if err != nil {
			return cell, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "p":
				p, err := parseDOCXNoteParagraph(dec, t, noteID)
				if err != nil {
					return cell, err
				}
				cell.Paragraphs = append(cell.Paragraphs, p)
				copyP := p
				cell.Blocks = append(cell.Blocks, wordir.Block{Kind: "paragraph", Paragraph: &copyP})
				cell.Text += p.Text
			case "tbl":
				table, err := parseDOCXNoteTable(dec, t, noteID)
				if err != nil {
					return cell, err
				}
				cell.Tables = append(cell.Tables, table)
				copyT := table
				cell.Blocks = append(cell.Blocks, wordir.Block{Kind: "table", Table: &copyT})
				cell.Text += table.Text
			default:
				if err := dec.Skip(); err != nil {
					return cell, err
				}
			}
		case xml.EndElement:
			if t.Name == end.Name {
				return cell, nil
			}
		}
	}
}
