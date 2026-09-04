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
	"strconv"
	"strings"

	"office/pkg/ooxml/opc"
	"office/pkg/ooxml/rels"
)

const (
	relTypeOfficeDocument    = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument"
	relTypeSlide             = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide"
	relTypeNotesSlide        = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesSlide"
	relTypeComments          = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/comments"
	strictRelPrefix          = "http://purl.oclc.org/ooxml/officeDocument/relationships/"
	transitionalRelPrefix    = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/"
	strictRelNamespace       = "http://purl.oclc.org/ooxml/officeDocument/relationships"
	transitionalRelNamespace = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
)

func canonicalRelationshipType(value string) string {
	if strings.HasPrefix(value, strictRelPrefix) {
		return transitionalRelPrefix + strings.TrimPrefix(value, strictRelPrefix)
	}
	return value
}

func isRelationshipNamespace(value string) bool {
	return value == transitionalRelNamespace || value == strictRelNamespace
}

// EventType identifies a SAX event emitted while walking a PresentationML package.
type EventType string

const (
	EventStartPresentation EventType = "startPresentation"
	EventEndPresentation   EventType = "endPresentation"
	EventStartSlide        EventType = "startSlide"
	EventEndSlide          EventType = "endSlide"
	EventStartNotes        EventType = "startNotes"
	EventEndNotes          EventType = "endNotes"
	EventStartComment      EventType = "startComment"
	EventEndComment        EventType = "endComment"
	EventStartShape        EventType = "startShape"
	EventEndShape          EventType = "endShape"
	EventStartParagraph    EventType = "startParagraph"
	EventEndParagraph      EventType = "endParagraph"
	EventText              EventType = "text"
	EventBreak             EventType = "break"
	EventTab               EventType = "tab"
	EventField             EventType = "field"
)

// Event is a single SAX event from a streamed PPTX package.
type Event struct {
	Type        EventType
	PartName    string
	SlideIndex  int
	ShapeID     int
	ShapeName   string
	Placeholder string
	Text        string
	FieldType   string
	Author      string
	Date        string
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
func WalkReaderAt(r io.ReaderAt, size int64, handler Handler, opts Options) error {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return walkMaterialized(r, size, handler, opts)
	}

	files := make(map[string]*zip.File)
	for _, f := range zr.File {
		files[normalizePartName(f.Name)] = f
	}

	ctx := &walkContext{
		files:   files,
		handler: handler,
		opts:    opts,
	}
	return ctx.walk()
}

func walkMaterialized(r io.ReaderAt, size int64, handler Handler, opts Options) error {
	pkg, err := opc.OpenReaderAt(r, size)
	if err != nil {
		return err
	}

	mainPart, err := findMainPart(pkg)
	if err != nil {
		return err
	}

	pres, err := readPresentation(pkg, mainPart)
	if err != nil {
		return err
	}

	presRels, _, _ := pkg.Relationships(mainPart)
	slideTargets := resolveSlideTargets(mainPart, presRels)

	emit := func(ev Event) error {
		return handler.HandleEvent(ev)
	}
	if err := emit(Event{Type: EventStartPresentation}); err != nil {
		return err
	}
	for i, sld := range pres.Slides {
		if sld.Hidden && !opts.IncludeHiddenSlides {
			continue
		}
		slideName, ok := slideTargets[sld.RelID]
		if !ok {
			continue
		}
		if err := emit(Event{Type: EventStartSlide, SlideIndex: i}); err != nil {
			return err
		}
		if part, ok := pkg.Part(slideName); ok {
			if err := walkShapeTree(part.Data, slideName, i, emit); err != nil {
				return err
			}
		}
		if err := emit(Event{Type: EventEndSlide, SlideIndex: i}); err != nil {
			return err
		}

		if opts.IncludeNotes || opts.IncludeComments {
			slideRels, _, _ := pkg.Relationships(slideName)
			if opts.IncludeNotes {
				for _, rel := range slideRels.Relationships {
					if canonicalRelationshipType(rel.Type) == relTypeNotesSlide && !rel.IsExternal() {
						notesName := rels.ResolveTarget(slideName, rel.Target)
						if part, ok := pkg.Part(notesName); ok {
							if err := emit(Event{Type: EventStartNotes, SlideIndex: i}); err != nil {
								return err
							}
							if err := walkShapeTree(part.Data, notesName, i, emit); err != nil {
								return err
							}
							if err := emit(Event{Type: EventEndNotes, SlideIndex: i}); err != nil {
								return err
							}
						}
					}
				}
			}
			if opts.IncludeComments {
				for _, rel := range slideRels.Relationships {
					if canonicalRelationshipType(rel.Type) == relTypeComments && !rel.IsExternal() {
						commentsName := rels.ResolveTarget(slideName, rel.Target)
						if part, ok := pkg.Part(commentsName); ok {
							if err := walkComments(part.Data, commentsName, i, emit); err != nil {
								return err
							}
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

type walkContext struct {
	files   map[string]*zip.File
	handler Handler
	opts    Options
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

	slideTargets := c.resolveSlideTargets(mainPart)

	emit := func(ev Event) error {
		return c.handler.HandleEvent(ev)
	}

	if err := emit(Event{Type: EventStartPresentation}); err != nil {
		return err
	}
	for i, sld := range pres.Slides {
		if sld.Hidden && !c.opts.IncludeHiddenSlides {
			continue
		}
		slideName, ok := slideTargets[sld.RelID]
		if !ok {
			continue
		}
		if err := emit(Event{Type: EventStartSlide, SlideIndex: i}); err != nil {
			return err
		}
		if err := c.walkShapeTreePart(slideName, i, emit); err != nil {
			return err
		}
		if err := emit(Event{Type: EventEndSlide, SlideIndex: i}); err != nil {
			return err
		}

		if c.opts.IncludeNotes || c.opts.IncludeComments {
			slideRels, err := c.readRelationships(slideName)
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

func (c *walkContext) openPart(name string) (io.ReadCloser, error) {
	f, ok := c.files[normalizePartName(name)]
	if !ok {
		return nil, fmt.Errorf("part %q not found", name)
	}
	return f.Open()
}

type relEntry struct {
	Type     string
	Target   string
	External bool
}

func (c *walkContext) readRelationships(sourcePart string) (map[string]relEntry, error) {
	if sourcePart == "" {
		return c.readRelsPart("_rels/.rels", "")
	}
	relDir := path.Dir(sourcePart)
	base := path.Base(sourcePart)
	relsName := relDir + "/_rels/" + base + ".rels"
	if relDir == "." {
		relsName = "_rels/" + base + ".rels"
	}
	return c.readRelsPart(relsName, sourcePart)
}

func (c *walkContext) readRelsPart(relsName, sourcePart string) (map[string]relEntry, error) {
	rc, err := c.openPart(relsName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()

	result := make(map[string]relEntry)
	dec := xml.NewDecoder(rc)
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
		if id != "" && relType != "" && target != "" {
			entry := relEntry{Type: canonicalRelationshipType(relType), External: strings.EqualFold(targetMode, "External")}
			if entry.External {
				entry.Target = target
			} else {
				entry.Target = rels.ResolveTarget(sourcePart, target)
			}
			result[id] = entry
		}
	}
	return result, nil
}

func (c *walkContext) findMainPart() (string, error) {
	rootRels, err := c.readRelationships("")
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
	presRels, err := c.readRelationships(mainPart)
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
	rc, err := c.openPart(mainPart)
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

func (c *walkContext) walkShapeTreePart(partName string, slideIdx int, emit func(Event) error) error {
	rc, err := c.openPart(partName)
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(rc)
	if err != nil {
		return err
	}
	return walkShapeTree(data, partName, slideIdx, emit)
}

func (c *walkContext) walkCommentsPart(partName string, slideIdx int, emit func(Event) error) error {
	rc, err := c.openPart(partName)
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(rc)
	if err != nil {
		return err
	}
	return walkComments(data, partName, slideIdx, emit)
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
	dec := xml.NewDecoder(bytes.NewReader(data))
	type shapeCtx struct {
		id      int
		name    string
		pending bool
	}
	var shapes []*shapeCtx
	var paragraphs []bool
	var tableStack []int // row cell counts per open table
	var textBuf strings.Builder
	var captureText bool
	var fieldType string

	pushShape := func() {
		shapes = append(shapes, &shapeCtx{pending: true})
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
			switch local {
			case "sp", "pic", "cxnSp", "grpSp":
				pushShape()
			case "cNvPr":
				if s := peekShape(); s != nil && s.pending {
					for _, attr := range se.Attr {
						switch attr.Name.Local {
						case "id":
							s.id, _ = strconv.Atoi(attr.Value)
						case "name":
							s.name = attr.Value
						}
					}
					if err := emit(Event{Type: EventStartShape, PartName: partName, SlideIndex: slideIdx, ShapeID: s.id, ShapeName: s.name}); err != nil {
						return err
					}
					s.pending = false
				}
			case "tbl":
				tableStack = append(tableStack, 0)
			case "tr":
				if len(tableStack) > 0 {
					tableStack[len(tableStack)-1] = 0
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
				}
			case "p":
				paragraphs = append(paragraphs, true)
				if err := emit(Event{Type: EventStartParagraph, PartName: partName, SlideIndex: slideIdx}); err != nil {
					return err
				}
			case "fld":
				fieldType = ""
				for _, attr := range se.Attr {
					if attr.Name.Local == "type" {
						fieldType = attr.Value
						break
					}
				}
			case "t":
				captureText = true
				textBuf.Reset()
			case "br":
				if err := emit(Event{Type: EventBreak, PartName: partName, SlideIndex: slideIdx}); err != nil {
					return err
				}
			case "tab":
				if err := emit(Event{Type: EventTab, PartName: partName, SlideIndex: slideIdx}); err != nil {
					return err
				}
			}
		case xml.EndElement:
			local := se.Name.Local
			switch local {
			case "sp", "pic", "cxnSp", "grpSp":
				s := popShape()
				if s != nil {
					if err := emit(Event{Type: EventEndShape, PartName: partName, SlideIndex: slideIdx}); err != nil {
						return err
					}
				}
			case "p":
				if len(paragraphs) > 0 {
					paragraphs = paragraphs[:len(paragraphs)-1]
					// Inside a table, cells are separated by tabs/rows; paragraph
					// breaks inside cells would otherwise introduce spurious newlines.
					if len(tableStack) == 0 {
						if err := emit(Event{Type: EventEndParagraph, PartName: partName, SlideIndex: slideIdx}); err != nil {
							return err
						}
					}
				}
			case "t":
				if captureText {
					if err := emit(Event{Type: EventText, PartName: partName, SlideIndex: slideIdx, Text: textBuf.String()}); err != nil {
						return err
					}
					captureText = false
					textBuf.Reset()
				}
			case "fld":
				if fieldType != "" {
					if err := emit(Event{Type: EventField, PartName: partName, SlideIndex: slideIdx, FieldType: fieldType}); err != nil {
						return err
					}
					fieldType = ""
				}
			case "tr":
				if len(tableStack) > 0 {
					if err := emit(Event{Type: EventText, PartName: partName, SlideIndex: slideIdx, Text: "\n"}); err != nil {
						return err
					}
				}
			case "tbl":
				if len(tableStack) > 0 {
					tableStack = tableStack[:len(tableStack)-1]
				}
			}
		case xml.CharData:
			if captureText {
				textBuf.Write(se)
			}
		}
	}
	return nil
}

func walkComments(data []byte, partName string, slideIdx int, emit func(Event) error) error {
	dec := xml.NewDecoder(bytes.NewReader(data))
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
