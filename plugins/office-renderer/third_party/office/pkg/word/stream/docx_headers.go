package stream

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"strings"

	wordir "office/pkg/word/ir"
)

type docxStoryRef struct {
	Type     string
	ID       string
	PartName string
}

type docxHeaderFooterLoader struct {
	parts          map[string]*zip.File
	rels           map[string]string
	cache          map[string]wordir.Story
	body           io.ReadCloser
	dec            *xml.Decoder
	settingsPart   string
	evenAndOdd     bool
}

func newDOCXHeaderFooterLoader(r io.ReaderAt, size int64) (*docxHeaderFooterLoader, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}
	parts := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		parts[normalizeDOCXPartName(f.Name)] = f
	}
	mainPart, err := findDOCXMainPart(parts)
	if err != nil {
		return nil, err
	}
	mainFile := parts[normalizeDOCXPartName(mainPart)]
	if mainFile == nil {
		return nil, fmt.Errorf("word stream: main DOCX part %q not found", mainPart)
	}
	body, err := mainFile.Open()
	if err != nil {
		return nil, err
	}
	loader := &docxHeaderFooterLoader{
		parts: parts,
		rels:  make(map[string]string),
		cache: make(map[string]wordir.Story),
		body:  body,
		dec:   xml.NewDecoder(body),
	}
	if err := loader.loadMainRelationships(mainPart); err != nil {
		body.Close()
		return nil, err
	}
	if err := loader.loadHeaderFooterSettings(); err != nil {
		body.Close()
		return nil, err
	}
	return loader, nil
}

func (l *docxHeaderFooterLoader) Close() error {
	if l == nil || l.body == nil {
		return nil
	}
	err := l.body.Close()
	l.body = nil
	l.dec = nil
	return err
}

func (l *docxHeaderFooterLoader) loadMainRelationships(mainPart string) error {
	relsName := normalizeDOCXPartName(path.Join(path.Dir(mainPart), "_rels", path.Base(mainPart)+".rels"))
	file := l.parts[relsName]
	if file == nil {
		return nil
	}
	rc, err := file.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	dec := xml.NewDecoder(rc)
	for {
		token, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "Relationship" {
			continue
		}
		id := docxAttr(start, "Id")
		target := docxAttr(start, "Target")
		mode := strings.ToLower(docxAttr(start, "TargetMode"))
		if id == "" || target == "" || mode == "external" {
			continue
		}
		partName := normalizeDOCXPartName(path.Join(path.Dir(mainPart), target))
		l.rels[id] = partName
		if strings.HasSuffix(strings.ToLower(docxAttr(start, "Type")), "/settings") {
			l.settingsPart = partName
		}
	}
}

func (l *docxHeaderFooterLoader) loadHeaderFooterSettings() error {
	if l == nil {
		return nil
	}
	name := l.settingsPart
	if name == "" && l.parts["word/settings.xml"] != nil {
		name = "word/settings.xml"
	}
	file := l.parts[normalizeDOCXPartName(name)]
	if file == nil {
		return nil
	}
	rc, err := file.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	dec := xml.NewDecoder(rc)
	for {
		token, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "evenAndOddHeaders" {
			continue
		}
		l.evenAndOdd = docxOnOff(start, true)
		return nil
	}
}

// NextSection advances only as far as the next sectPr in document.xml. This
// intentionally mirrors the main bounded stream rather than pre-scanning all
// sections up front.
func (l *docxHeaderFooterLoader) NextSection() ([]docxStoryRef, []docxStoryRef, error) {
	if l == nil || l.dec == nil {
		return nil, nil, nil
	}
	for {
		token, err := l.dec.Token()
		if err != nil {
			if err == io.EOF {
				return nil, nil, nil
			}
			return nil, nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "sectPr" {
			continue
		}
		return l.parseSectionRefs(start)
	}
}

func (l *docxHeaderFooterLoader) parseSectionRefs(end xml.StartElement) ([]docxStoryRef, []docxStoryRef, error) {
	var headers []docxStoryRef
	var footers []docxStoryRef
	for {
		token, err := l.dec.Token()
		if err != nil {
			return nil, nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "headerReference":
				ref := l.resolveRef(t)
				if ref.ID != "" {
					headers = append(headers, ref)
				}
			case "footerReference":
				ref := l.resolveRef(t)
				if ref.ID != "" {
					footers = append(footers, ref)
				}
			}
		case xml.EndElement:
			if t.Name == end.Name {
				return headers, footers, nil
			}
		}
	}
}

func (l *docxHeaderFooterLoader) resolveRef(start xml.StartElement) docxStoryRef {
	id := docxAttr(start, "id")
	typ := docxAttr(start, "type")
	if typ == "" {
		typ = "default"
	}
	return docxStoryRef{Type: typ, ID: id, PartName: l.rels[id]}
}

func (l *docxHeaderFooterLoader) Load(ref docxStoryRef) (wordir.Story, error) {
	if l == nil || ref.PartName == "" {
		return wordir.Story{}, nil
	}
	name := normalizeDOCXPartName(ref.PartName)
	if story, ok := l.cache[name]; ok {
		return story, nil
	}
	file := l.parts[name]
	if file == nil {
		return wordir.Story{}, fmt.Errorf("word stream: header/footer part %q not found", ref.PartName)
	}
	rc, err := file.Open()
	if err != nil {
		return wordir.Story{}, err
	}
	defer rc.Close()
	story, err := parseDOCXHeaderFooterStory(rc)
	if err != nil {
		return wordir.Story{}, err
	}
	l.cache[name] = story
	return story, nil
}

func parseDOCXHeaderFooterStory(r io.Reader) (wordir.Story, error) {
	var story wordir.Story
	dec := xml.NewDecoder(r)
	for {
		token, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				return story, nil
			}
			return story, err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		switch start.Name.Local {
		case "p":
			p, err := parseDOCXNoteParagraph(dec, start)
			if err != nil {
				return story, err
			}
			story.Paragraphs = append(story.Paragraphs, p)
			copyP := p
			story.Blocks = append(story.Blocks, wordir.Block{Kind: "paragraph", Paragraph: &copyP})
			story.Text += p.Text
		case "tbl":
			table, err := parseDOCXNoteTable(dec, start)
			if err != nil {
				return story, err
			}
			story.Tables = append(story.Tables, table)
			copyT := table
			story.Blocks = append(story.Blocks, wordir.Block{Kind: "table", Table: &copyT})
		}
	}
}

func mergeSectionStoryRefs(previous []wordir.Reference, explicit []docxStoryRef) []wordir.Reference {
	byType := make(map[string]wordir.Reference, len(previous)+len(explicit))
	order := make([]string, 0, len(previous)+len(explicit))
	for _, ref := range previous {
		typ := ref.Type
		if typ == "" {
			typ = "default"
		}
		if _, seen := byType[typ]; !seen {
			order = append(order, typ)
		}
		ref.Type = typ
		byType[typ] = ref
	}
	for _, ref := range explicit {
		typ := ref.Type
		if typ == "" {
			typ = "default"
		}
		if _, seen := byType[typ]; !seen {
			order = append(order, typ)
		}
		byType[typ] = wordir.Reference{Type: typ, PartName: ref.PartName}
	}
	out := make([]wordir.Reference, 0, len(order))
	for _, typ := range order {
		out = append(out, byType[typ])
	}
	return out
}
