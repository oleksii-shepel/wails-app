package sax

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"office/pkg/ooxml/opc"
	"office/pkg/ooxml/rels"
	wordparts "office/pkg/word/parsers/ooxml/parts"
)

const relationshipTypeOfficeDocument = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument"
const (
	relationshipTypeHeader    = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/header"
	relationshipTypeFooter    = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer"
	relationshipTypeFootnotes = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/footnotes"
	relationshipTypeEndnotes  = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/endnotes"
	relationshipTypeComments  = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/comments"
	relationshipTypeHyperlink = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink"
)

// StoryKind identifies which Word story a SAX event belongs to.
type StoryKind string

const (
	// StoryDocument is the main word/document.xml story.
	StoryDocument StoryKind = "document"
	// StoryHeader is a referenced header story.
	StoryHeader StoryKind = "header"
	// StoryFooter is a referenced footer story.
	StoryFooter StoryKind = "footer"
	// StoryFootnote is a referenced footnotes story part.
	StoryFootnote StoryKind = "footnote"
	// StoryEndnote is a referenced endnotes story part.
	StoryEndnote StoryKind = "endnote"
	// StoryComment is a referenced comments story part.
	StoryComment StoryKind = "comment"
)

// EventType identifies a SAX event emitted while walking a WordprocessingML document.
type EventType string

const (
	// EventStartDocument marks the start of the main document story.
	EventStartDocument EventType = "startDocument"
	// EventEndDocument marks the end of the main document story.
	EventEndDocument EventType = "endDocument"
	// EventStartParagraph marks the start of a paragraph.
	EventStartParagraph EventType = "startParagraph"
	// EventEndParagraph marks the end of a paragraph.
	EventEndParagraph EventType = "endParagraph"
	// EventStartTable marks the start of a table.
	EventStartTable EventType = "startTable"
	// EventEndTable marks the end of a table.
	EventEndTable EventType = "endTable"
	// EventStartTableRow marks the start of a table row.
	EventStartTableRow EventType = "startTableRow"
	// EventEndTableRow marks the end of a table row.
	EventEndTableRow EventType = "endTableRow"
	// EventStartTableCell marks the start of a table cell.
	EventStartTableCell EventType = "startTableCell"
	// EventEndTableCell marks the end of a table cell.
	EventEndTableCell EventType = "endTableCell"
	// EventStartContentControl marks the start of a structured document tag.
	EventStartContentControl EventType = "startContentControl"
	// EventEndContentControl marks the end of a structured document tag.
	EventEndContentControl EventType = "endContentControl"
	// EventStartCustomXML marks the start of a customXml wrapper.
	EventStartCustomXML EventType = "startCustomXML"
	// EventEndCustomXML marks the end of a customXml wrapper.
	EventEndCustomXML EventType = "endCustomXML"
	// EventStartSmartTag marks the start of a smartTag wrapper.
	EventStartSmartTag EventType = "startSmartTag"
	// EventEndSmartTag marks the end of a smartTag wrapper.
	EventEndSmartTag EventType = "endSmartTag"
	// EventStartDirection marks the start of a dir/bdo wrapper.
	EventStartDirection EventType = "startDirection"
	// EventEndDirection marks the end of a dir/bdo wrapper.
	EventEndDirection EventType = "endDirection"
	// EventText emits visible text content.
	EventText EventType = "text"
	// EventStartHyperlink marks the start of a hyperlink container.
	EventStartHyperlink EventType = "startHyperlink"
	// EventEndHyperlink marks the end of a hyperlink container.
	EventEndHyperlink EventType = "endHyperlink"
	// EventBookmarkStart marks a bookmarkStart boundary.
	EventBookmarkStart EventType = "bookmarkStart"
	// EventBookmarkEnd marks a bookmarkEnd boundary.
	EventBookmarkEnd EventType = "bookmarkEnd"
	// EventCommentRangeStart marks a commentRangeStart boundary.
	EventCommentRangeStart EventType = "commentRangeStart"
	// EventCommentRangeEnd marks a commentRangeEnd boundary.
	EventCommentRangeEnd EventType = "commentRangeEnd"
	// EventTab emits a tab character boundary.
	EventTab EventType = "tab"
	// EventBreak emits a Word line/page/column break boundary.
	EventBreak EventType = "break"
	// EventSymbol emits a decoded w:sym symbol token.
	EventSymbol EventType = "symbol"
	// EventSoftHyphen emits an optional hyphen token.
	EventSoftHyphen EventType = "softHyphen"
	// EventNoBreakHyphen emits a non-breaking hyphen token.
	EventNoBreakHyphen EventType = "noBreakHyphen"
	// EventAnnotationReference emits an annotationRef token.
	EventAnnotationReference EventType = "annotationReference"
	// EventAutoText emits an auto-text placeholder token such as day/month/year/pgNum.
	EventAutoText EventType = "autoText"
	// EventFieldInstruction emits a raw field instruction token sequence.
	EventFieldInstruction EventType = "fieldInstruction"
	// EventFieldChar emits a complex-field boundary from w:fldChar.
	EventFieldChar EventType = "fieldChar"
	// EventStartRevision marks the start of a tracked revision wrapper.
	EventStartRevision EventType = "startRevision"
	// EventEndRevision marks the end of a tracked revision wrapper.
	EventEndRevision EventType = "endRevision"
	// EventFootnoteReference emits a footnote reference run token.
	EventFootnoteReference EventType = "footnoteReference"
	// EventEndnoteReference emits an endnote reference run token.
	EventEndnoteReference EventType = "endnoteReference"
	// EventCommentReference emits a comment reference run token.
	EventCommentReference EventType = "commentReference"
	// EventRuby emits a ruby token with extracted base and annotation text.
	EventRuby EventType = "ruby"
	// EventMath emits an OMML math token.
	EventMath EventType = "math"
	// EventDrawing emits a drawing token.
	EventDrawing EventType = "drawing"
	// EventEmbeddedObject emits a w:object/OLE token.
	EventEmbeddedObject EventType = "embeddedObject"
	// EventContentPart emits a contentPart relationship token.
	EventContentPart EventType = "contentPart"
	// EventAltChunk emits an altChunk relationship token.
	EventAltChunk EventType = "altChunk"
	// EventStartNote marks the start of a footnote/endnote entry within its story part.
	EventStartNote EventType = "startNote"
	// EventEndNote marks the end of a footnote/endnote entry within its story part.
	EventEndNote EventType = "endNote"
	// EventStartComment marks the start of a comment entry within comments.xml.
	EventStartComment EventType = "startComment"
	// EventEndComment marks the end of a comment entry within comments.xml.
	EventEndComment EventType = "endComment"
	// EventStartStory marks the start of any streamed story.
	EventStartStory EventType = "startStory"
	// EventEndStory marks the end of any streamed story.
	EventEndStory EventType = "endStory"
)

// Event is a single SAX event from a streamed Word story.
type Event struct {
	Type                   EventType
	Story                  StoryKind
	PartName               string
	StyleID                string
	Text                   string
	BreakType              string
	FieldInstr             string
	FieldCharType          string
	RelationshipID         string
	RelationshipType       string
	RelationshipTarget     string
	HyperlinkAnchor        string
	HyperlinkTarget        string
	BookmarkID             string
	BookmarkName           string
	RevisionKind           string
	RevisionID             string
	NoteID                 string
	NoteType               string
	CommentID              string
	SymbolFont             string
	SymbolChar             string
	AutoTextKind           string
	MarkupURI              string
	MarkupElement          string
	ControlID              string
	ControlAlias           string
	ControlTag             string
	ControlType            string
	Direction              string
	TargetExternal         bool
	RelatedRelationshipIDs []string
	RelatedTargets         []string
	RelatedParts           []RelatedPart
	RawXML                 string
	Display                bool
	ProgID                 string
	MathText               string
	MathModel              *MathNode
	RubyBaseText           string
	RubyText               string
	RubyModel              *wordparts.Ruby
	DrawingKind            string
	DrawingDocID           string
	DrawingName            string
	DrawingDescription     string
	DrawingTitle           string
	DrawingHidden          bool
	WidthEMU               int64
	HeightEMU              int64
	GraphicDataURI         string
	DrawingGraphicKind     string
	DrawingHasTextBox      bool
	DrawingModel           *wordparts.Drawing
	ControlProperties      *wordparts.SdtProperties
	CustomXMLModel         *wordparts.CustomXml
	ContentPartModel       *wordparts.ContentPartRef
	AltChunkModel          *wordparts.AltChunk
	EmbeddedObjectModel    *wordparts.OLEObjectRun
	MatchSource            bool
	PayloadContentType     string
	PayloadSize            int64
	PayloadData            []byte
	Author                 string
	Date                   string
}

// RelatedPart describes a relationship-backed payload referenced by an event.
type RelatedPart struct {
	RelationshipID   string
	RelationshipType string
	Target           string
	External         bool
	ContentType      string
	Size             int64
}

// MathNode is a normalized OMML node tree for EventMath.
type MathNode struct {
	Kind       string
	Text       string
	Attributes map[string]string
	Children   []*MathNode
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

// Options control which SAX events are emitted.
type Options struct {
	// IncludeFieldInstructions emits EventFieldInstruction for w:fldSimple@instr
	// and w:instrText runs. The default omits those events.
	IncludeFieldInstructions bool
	// IncludeHeadersAndFooters walks header/footer parts referenced from the
	// main document sections in addition to the main body story.
	IncludeHeadersAndFooters bool
	// IncludeFootnotes walks the related footnotes part after the main body.
	IncludeFootnotes bool
	// IncludeEndnotes walks the related endnotes part after the main body.
	IncludeEndnotes bool
	// IncludeComments walks the related comments part after the main body.
	IncludeComments bool
}

// WalkPath walks the main document story from a DOCX file on disk.
func WalkPath(path string, handler Handler, opts Options) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	info, err := file.Stat()
	if err != nil {
		return err
	}
	return WalkReaderAt(file, info.Size(), handler, opts)
}

// WalkBytes walks the main document story from an in-memory DOCX byte slice.
func WalkBytes(data []byte, handler Handler, opts Options) error {
	return WalkReaderAt(bytes.NewReader(data), int64(len(data)), handler, opts)
}

// WalkStream walks the main document story from a non-seekable DOCX stream.
//
// For plain ZIP-backed OOXML packages this avoids building parts.Package or IR.
// Encrypted OLE-backed OOXML may still fall back to the existing materialized
// OPC path so decryption can proceed.
func WalkStream(r io.Reader, handler Handler, opts Options) error {
	file, err := os.CreateTemp("", "office-word-sax-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer func() {
		_ = os.Remove(name)
	}()
	defer func() {
		_ = file.Close()
	}()

	size, err := io.Copy(file, r)
	if err != nil {
		return err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	return WalkReaderAt(file, size, handler, opts)
}

// WalkReaderAt walks the main document story from a DOCX reader/size pair.
func WalkReaderAt(r io.ReaderAt, size int64, handler Handler, opts Options) error {
	if handler == nil {
		return fmt.Errorf("word sax: handler cannot be nil")
	}

	zr, err := zip.NewReader(r, size)
	if err == nil {
		return walkZipReader(zr, handler, opts)
	}

	// Encrypted OOXML in OLE containers still needs the existing OPC fallback.
	pkg, openErr := opc.OpenReaderAt(r, size)
	if openErr != nil {
		return openErr
	}
	return walkMaterializedPackage(pkg, handler, opts)
}

type walker struct {
	handler          Handler
	opts             Options
	story            StoryKind
	partName         string
	headerRefs       map[string]struct{}
	footerRefs       map[string]struct{}
	relationshipRefs map[string]relationshipRef
	lookupPart       func(string) (partContentType string, data []byte, ok bool)
}

type relationshipRef struct {
	target   string
	relType  string
	external bool
}

type drawingToken struct {
	kind            string
	docID           string
	name            string
	description     string
	title           string
	hidden          bool
	widthEMU        int64
	heightEMU       int64
	graphicDataURI  string
	graphicKind     string
	hasTextBox      bool
	relationshipIDs []string
	targets         []string
	model           *wordparts.Drawing
}

type objectToken struct {
	progID          string
	relationshipIDs []string
	targets         []string
	model           *wordparts.OLEObjectRun
}

func walkZipReader(zr *zip.Reader, handler Handler, opts Options) error {
	files := make(map[string]*zip.File, len(zr.File))
	for _, file := range zr.File {
		files[normalizePartName(file.Name)] = file
	}
	contentTypes, err := loadZipContentTypes(files)
	if err != nil {
		return err
	}
	mainPartName, err := findMainDocumentPartInZip(files)
	if err != nil {
		return err
	}
	file := files[normalizePartName(mainPartName)]
	if file == nil {
		return fmt.Errorf("word sax: main document part %q not found", mainPartName)
	}
	rc, err := file.Open()
	if err != nil {
		return err
	}
	defer func() {
		_ = rc.Close()
	}()
	w := &walker{
		handler:          handler,
		opts:             opts,
		story:            StoryDocument,
		partName:         normalizePartName(mainPartName),
		headerRefs:       make(map[string]struct{}),
		footerRefs:       make(map[string]struct{}),
		relationshipRefs: loadZipRelationshipRefs(files, mainPartName),
		lookupPart: func(name string) (string, []byte, bool) {
			return loadZipPart(files, contentTypes, name)
		},
	}
	if err := w.walkXML(rc); err != nil {
		return err
	}
	if opts.IncludeHeadersAndFooters {
		if err := walkRelatedPartsInZip(files, contentTypes, mainPartName, w.headerRefs, StoryHeader, handler, opts); err != nil {
			return err
		}
		if err := walkRelatedPartsInZip(files, contentTypes, mainPartName, w.footerRefs, StoryFooter, handler, opts); err != nil {
			return err
		}
	}
	if opts.IncludeFootnotes {
		if err := walkRelationshipTypePartsInZip(files, contentTypes, mainPartName, relationshipTypeFootnotes, StoryFootnote, handler, opts); err != nil {
			return err
		}
	}
	if opts.IncludeEndnotes {
		if err := walkRelationshipTypePartsInZip(files, contentTypes, mainPartName, relationshipTypeEndnotes, StoryEndnote, handler, opts); err != nil {
			return err
		}
	}
	if opts.IncludeComments {
		if err := walkRelationshipTypePartsInZip(files, contentTypes, mainPartName, relationshipTypeComments, StoryComment, handler, opts); err != nil {
			return err
		}
	}
	return nil
}

func walkMaterializedPackage(pkg *opc.Package, handler Handler, opts Options) error {
	mainPartName, err := findMainDocumentPartInPackage(pkg)
	if err != nil {
		return err
	}
	part, ok := pkg.Part(mainPartName)
	if !ok {
		return fmt.Errorf("word sax: main document part %q not found", mainPartName)
	}
	w := &walker{
		handler:          handler,
		opts:             opts,
		story:            StoryDocument,
		partName:         normalizePartName(mainPartName),
		headerRefs:       make(map[string]struct{}),
		footerRefs:       make(map[string]struct{}),
		relationshipRefs: loadPackageRelationshipRefs(pkg, mainPartName),
		lookupPart: func(name string) (string, []byte, bool) {
			part, ok := pkg.Part(name)
			if !ok {
				return "", nil, false
			}
			return part.ContentType, part.Data, true
		},
	}
	if err := w.walkXML(bytes.NewReader(part.Data)); err != nil {
		return err
	}
	if opts.IncludeHeadersAndFooters {
		if err := walkRelatedPartsInPackage(pkg, mainPartName, w.headerRefs, StoryHeader, handler, opts); err != nil {
			return err
		}
		if err := walkRelatedPartsInPackage(pkg, mainPartName, w.footerRefs, StoryFooter, handler, opts); err != nil {
			return err
		}
	}
	if opts.IncludeFootnotes {
		if err := walkRelationshipTypePartsInPackage(pkg, mainPartName, relationshipTypeFootnotes, StoryFootnote, handler, opts); err != nil {
			return err
		}
	}
	if opts.IncludeEndnotes {
		if err := walkRelationshipTypePartsInPackage(pkg, mainPartName, relationshipTypeEndnotes, StoryEndnote, handler, opts); err != nil {
			return err
		}
	}
	if opts.IncludeComments {
		if err := walkRelationshipTypePartsInPackage(pkg, mainPartName, relationshipTypeComments, StoryComment, handler, opts); err != nil {
			return err
		}
	}
	return nil
}

func findMainDocumentPartInZip(files map[string]*zip.File) (string, error) {
	relsFile := files[normalizePartName(rels.RelationshipsPartName(""))]
	if relsFile == nil {
		return "", fmt.Errorf("word sax: root relationships not found")
	}
	data, err := readZipFile(relsFile)
	if err != nil {
		return "", err
	}
	doc, err := rels.Parse(data)
	if err != nil {
		return "", err
	}
	for _, relationship := range doc.Relationships {
		if relationship.Type == relationshipTypeOfficeDocument && !relationship.IsExternal() {
			return rels.ResolveTarget("", relationship.Target), nil
		}
	}
	return "", fmt.Errorf("word sax: officeDocument relationship not found")
}

func findMainDocumentPartInPackage(pkg *opc.Package) (string, error) {
	rootRels, ok, err := pkg.Relationships("")
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("word sax: root relationships not found")
	}
	for _, relationship := range rootRels.Relationships {
		if relationship.Type == relationshipTypeOfficeDocument && !relationship.IsExternal() {
			return rels.ResolveTarget("", relationship.Target), nil
		}
	}
	return "", fmt.Errorf("word sax: officeDocument relationship not found")
}

func loadZipRelationshipRefs(files map[string]*zip.File, sourcePart string) map[string]relationshipRef {
	relsFile := files[normalizePartName(rels.RelationshipsPartName(sourcePart))]
	if relsFile == nil {
		return nil
	}
	data, err := readZipFile(relsFile)
	if err != nil {
		return nil
	}
	doc, err := rels.Parse(data)
	if err != nil {
		return nil
	}
	out := make(map[string]relationshipRef, len(doc.Relationships))
	for _, relationship := range doc.Relationships {
		target := relationship.Target
		if !relationship.IsExternal() {
			target = rels.ResolveTarget(sourcePart, relationship.Target)
		}
		out[relationship.ID] = relationshipRef{
			target:   target,
			relType:  relationship.Type,
			external: relationship.IsExternal(),
		}
	}
	return out
}

func loadPackageRelationshipRefs(pkg *opc.Package, sourcePart string) map[string]relationshipRef {
	relsDoc, ok, err := pkg.Relationships(sourcePart)
	if err != nil || !ok || relsDoc == nil {
		return nil
	}
	out := make(map[string]relationshipRef, len(relsDoc.Relationships))
	for _, relationship := range relsDoc.Relationships {
		target := relationship.Target
		if !relationship.IsExternal() {
			target = rels.ResolveTarget(sourcePart, relationship.Target)
		}
		out[relationship.ID] = relationshipRef{
			target:   target,
			relType:  relationship.Type,
			external: relationship.IsExternal(),
		}
	}
	return out
}

func loadZipContentTypes(files map[string]*zip.File) (*opc.ContentTypes, error) {
	file := files[normalizePartName("[Content_Types].xml")]
	if file == nil {
		return nil, nil
	}
	data, err := readZipFile(file)
	if err != nil {
		return nil, err
	}
	var contentTypes opc.ContentTypes
	if err := xml.Unmarshal(data, &contentTypes); err != nil {
		return nil, err
	}
	return &contentTypes, nil
}

func loadZipPart(files map[string]*zip.File, contentTypes *opc.ContentTypes, name string) (string, []byte, bool) {
	file := files[normalizePartName(name)]
	if file == nil {
		return "", nil, false
	}
	data, err := readZipFile(file)
	if err != nil {
		return "", nil, false
	}
	return resolveZipContentType(contentTypes, name), data, true
}

func resolveZipContentType(contentTypes *opc.ContentTypes, name string) string {
	if contentTypes == nil {
		return ""
	}
	name = normalizePartName(name)
	for _, override := range contentTypes.Overrides {
		if normalizePartName(override.PartName) == name {
			return override.ContentType
		}
	}
	ext := extension(name)
	if ext == "" {
		return ""
	}
	for _, def := range contentTypes.Defaults {
		if strings.EqualFold(def.Extension, ext) {
			return def.ContentType
		}
	}
	return ""
}

func walkRelatedPartsInZip(files map[string]*zip.File, contentTypes *opc.ContentTypes, sourcePart string, relIDs map[string]struct{}, story StoryKind, handler Handler, opts Options) error {
	if len(relIDs) == 0 {
		return nil
	}
	relsFile := files[normalizePartName(rels.RelationshipsPartName(sourcePart))]
	if relsFile == nil {
		return nil
	}
	data, err := readZipFile(relsFile)
	if err != nil {
		return err
	}
	doc, err := rels.Parse(data)
	if err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(relIDs))
	for _, relationship := range doc.Relationships {
		if _, ok := relIDs[relationship.ID]; !ok || relationship.IsExternal() {
			continue
		}
		partName := rels.ResolveTarget(sourcePart, relationship.Target)
		partName = normalizePartName(partName)
		if _, ok := seen[partName]; ok {
			continue
		}
		seen[partName] = struct{}{}
		file := files[partName]
		if file == nil {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return err
		}
		w := &walker{
			handler:          handler,
			opts:             opts,
			story:            story,
			partName:         partName,
			relationshipRefs: loadZipRelationshipRefs(files, partName),
			lookupPart: func(name string) (string, []byte, bool) {
				return loadZipPart(files, contentTypes, name)
			},
		}
		err = w.walkXML(rc)
		_ = rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func walkRelatedPartsInPackage(pkg *opc.Package, sourcePart string, relIDs map[string]struct{}, story StoryKind, handler Handler, opts Options) error {
	if len(relIDs) == 0 {
		return nil
	}
	relsDoc, ok, err := pkg.Relationships(sourcePart)
	if err != nil || !ok {
		return err
	}
	seen := make(map[string]struct{}, len(relIDs))
	for _, relationship := range relsDoc.Relationships {
		if _, ok := relIDs[relationship.ID]; !ok || relationship.IsExternal() {
			continue
		}
		partName := normalizePartName(rels.ResolveTarget(sourcePart, relationship.Target))
		if _, ok := seen[partName]; ok {
			continue
		}
		seen[partName] = struct{}{}
		part, ok := pkg.Part(partName)
		if !ok {
			continue
		}
		w := &walker{
			handler:          handler,
			opts:             opts,
			story:            story,
			partName:         partName,
			relationshipRefs: loadPackageRelationshipRefs(pkg, partName),
			lookupPart: func(name string) (string, []byte, bool) {
				part, ok := pkg.Part(name)
				if !ok {
					return "", nil, false
				}
				return part.ContentType, part.Data, true
			},
		}
		if err := w.walkXML(bytes.NewReader(part.Data)); err != nil {
			return err
		}
	}
	return nil
}

func walkRelationshipTypePartsInZip(files map[string]*zip.File, contentTypes *opc.ContentTypes, sourcePart, relationshipType string, story StoryKind, handler Handler, opts Options) error {
	relsFile := files[normalizePartName(rels.RelationshipsPartName(sourcePart))]
	if relsFile == nil {
		return nil
	}
	data, err := readZipFile(relsFile)
	if err != nil {
		return err
	}
	doc, err := rels.Parse(data)
	if err != nil {
		return err
	}
	seen := make(map[string]struct{})
	for _, relationship := range doc.Relationships {
		if relationship.Type != relationshipType || relationship.IsExternal() {
			continue
		}
		partName := normalizePartName(rels.ResolveTarget(sourcePart, relationship.Target))
		if _, ok := seen[partName]; ok {
			continue
		}
		seen[partName] = struct{}{}
		file := files[partName]
		if file == nil {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return err
		}
		w := &walker{
			handler:          handler,
			opts:             opts,
			story:            story,
			partName:         partName,
			relationshipRefs: loadZipRelationshipRefs(files, partName),
			lookupPart: func(name string) (string, []byte, bool) {
				return loadZipPart(files, contentTypes, name)
			},
		}
		err = w.walkXML(rc)
		_ = rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func walkRelationshipTypePartsInPackage(pkg *opc.Package, sourcePart, relationshipType string, story StoryKind, handler Handler, opts Options) error {
	relsDoc, ok, err := pkg.Relationships(sourcePart)
	if err != nil || !ok {
		return err
	}
	seen := make(map[string]struct{})
	for _, relationship := range relsDoc.Relationships {
		if relationship.Type != relationshipType || relationship.IsExternal() {
			continue
		}
		partName := normalizePartName(rels.ResolveTarget(sourcePart, relationship.Target))
		if _, ok := seen[partName]; ok {
			continue
		}
		seen[partName] = struct{}{}
		part, ok := pkg.Part(partName)
		if !ok {
			continue
		}
		w := &walker{
			handler:          handler,
			opts:             opts,
			story:            story,
			partName:         partName,
			relationshipRefs: loadPackageRelationshipRefs(pkg, partName),
			lookupPart: func(name string) (string, []byte, bool) {
				part, ok := pkg.Part(name)
				if !ok {
					return "", nil, false
				}
				return part.ContentType, part.Data, true
			},
		}
		if err := w.walkXML(bytes.NewReader(part.Data)); err != nil {
			return err
		}
	}
	return nil
}

func (w *walker) walkXML(r io.Reader) error {
	if err := w.emit(Event{Type: EventStartStory}); err != nil {
		return err
	}
	if w.story == StoryDocument {
		if err := w.emit(Event{Type: EventStartDocument}); err != nil {
			return err
		}
	}
	defer func() {
		if w.story == StoryDocument {
			_ = w.emit(Event{Type: EventEndDocument})
		}
		_ = w.emit(Event{Type: EventEndStory})
	}()

	dec := xml.NewDecoder(r)
	for {
		token, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		switch start.Name.Local {
		case "document":
			return w.walkDocument(dec, start)
		case "hdr", "ftr":
			return w.walkBodyContainer(dec, start)
		case "footnotes":
			return w.walkNotes(dec, start, "footnote")
		case "endnotes":
			return w.walkNotes(dec, start, "endnote")
		case "comments":
			return w.walkComments(dec, start)
		default:
			if err := dec.Skip(); err != nil {
				return err
			}
		}
	}
}

func (w *walker) walkNotes(dec *xml.Decoder, start xml.StartElement, noteName string) error {
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == noteName {
				if err := w.walkNote(dec, t); err != nil {
					return err
				}
				continue
			}
			if err := dec.Skip(); err != nil {
				return err
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return nil
			}
		}
	}
}

func (w *walker) walkNote(dec *xml.Decoder, start xml.StartElement) error {
	noteType := attrValue(start, "type")
	if noteType != "" && noteType != "normal" {
		return dec.Skip()
	}
	if err := w.emit(Event{
		Type:     EventStartNote,
		NoteID:   attrValue(start, "id"),
		NoteType: noteType,
	}); err != nil {
		return err
	}
	if err := w.walkBodyContainer(dec, start); err != nil {
		return err
	}
	return w.emit(Event{
		Type:     EventEndNote,
		NoteID:   attrValue(start, "id"),
		NoteType: noteType,
	})
}

func (w *walker) walkComments(dec *xml.Decoder, start xml.StartElement) error {
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "comment" {
				commentID := attrValue(t, "id")
				if err := w.emit(Event{
					Type:      EventStartComment,
					CommentID: commentID,
					Author:    attrValue(t, "author"),
					Date:      attrValue(t, "date"),
				}); err != nil {
					return err
				}
				if err := w.walkBodyContainer(dec, t); err != nil {
					return err
				}
				if err := w.emit(Event{
					Type:      EventEndComment,
					CommentID: commentID,
					Author:    attrValue(t, "author"),
					Date:      attrValue(t, "date"),
				}); err != nil {
					return err
				}
				continue
			}
			if err := dec.Skip(); err != nil {
				return err
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return nil
			}
		}
	}
}

func (w *walker) walkDocument(dec *xml.Decoder, start xml.StartElement) error {
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "body" {
				return w.walkBody(dec, t)
			}
			if err := dec.Skip(); err != nil {
				return err
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return nil
			}
		}
	}
}

func (w *walker) walkBody(dec *xml.Decoder, start xml.StartElement) error {
	return w.walkBodyContainer(dec, start)
}

func (w *walker) walkBodyContainer(dec *xml.Decoder, start xml.StartElement) error {
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "p":
				if err := w.walkParagraph(dec, t); err != nil {
					return err
				}
			case "tbl":
				if err := w.walkTable(dec, t); err != nil {
					return err
				}
			case "sdt":
				if err := w.walkSdtBlock(dec, t); err != nil {
					return err
				}
			case "customXml":
				if err := w.walkCustomXMLBlock(dec, t); err != nil {
					return err
				}
			case "sdtContent":
				if err := w.walkBodyContainer(dec, t); err != nil {
					return err
				}
			case "altChunk":
				if err := w.walkAltChunk(dec, t); err != nil {
					return err
				}
			case "sectPr":
				if err := w.collectSectionReferences(dec, t); err != nil {
					return err
				}
			default:
				if err := dec.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return nil
			}
		}
	}
}

func (w *walker) collectSectionReferences(dec *xml.Decoder, start xml.StartElement) error {
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "headerReference":
				if w.headerRefs != nil {
					if id := attrValue(t, "id"); id != "" {
						w.headerRefs[id] = struct{}{}
					}
				}
				if err := dec.Skip(); err != nil {
					return err
				}
			case "footerReference":
				if w.footerRefs != nil {
					if id := attrValue(t, "id"); id != "" {
						w.footerRefs[id] = struct{}{}
					}
				}
				if err := dec.Skip(); err != nil {
					return err
				}
			default:
				if err := dec.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return nil
			}
		}
	}
}

func (w *walker) walkParagraph(dec *xml.Decoder, start xml.StartElement) error {
	var styleID string
	started := false
	ensureStarted := func() error {
		if started {
			return nil
		}
		started = true
		return w.emit(Event{Type: EventStartParagraph, StyleID: styleID})
	}

	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if !started && t.Name.Local == "pPr" {
				parsedStyleID, err := parseParagraphProperties(dec, t)
				if err != nil {
					return err
				}
				styleID = parsedStyleID
				continue
			}
			if err := ensureStarted(); err != nil {
				return err
			}
			switch t.Name.Local {
			case "bookmarkStart":
				if err := w.emit(Event{
					Type:         EventBookmarkStart,
					BookmarkID:   attrValue(t, "id"),
					BookmarkName: attrValue(t, "name"),
				}); err != nil {
					return err
				}
				if err := dec.Skip(); err != nil {
					return err
				}
				continue
			case "bookmarkEnd":
				if err := w.emit(Event{
					Type:       EventBookmarkEnd,
					BookmarkID: attrValue(t, "id"),
				}); err != nil {
					return err
				}
				if err := dec.Skip(); err != nil {
					return err
				}
				continue
			case "commentRangeStart":
				if err := w.emit(Event{
					Type:      EventCommentRangeStart,
					CommentID: attrValue(t, "id"),
				}); err != nil {
					return err
				}
				if err := dec.Skip(); err != nil {
					return err
				}
				continue
			case "commentRangeEnd":
				if err := w.emit(Event{
					Type:      EventCommentRangeEnd,
					CommentID: attrValue(t, "id"),
				}); err != nil {
					return err
				}
				if err := dec.Skip(); err != nil {
					return err
				}
				continue
			}
			if err := w.walkParagraphElement(dec, t); err != nil {
				return err
			}
		case xml.EndElement:
			if t.Name == start.Name {
				if err := ensureStarted(); err != nil {
					return err
				}
				return w.emit(Event{Type: EventEndParagraph, StyleID: styleID})
			}
		}
	}
}

func parseParagraphProperties(dec *xml.Decoder, start xml.StartElement) (string, error) {
	var styleID string
	for {
		token, err := dec.Token()
		if err != nil {
			return "", err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "pStyle" && styleID == "" {
				styleID = attrValue(t, "val")
			}
			if err := dec.Skip(); err != nil {
				return "", err
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return styleID, nil
			}
		}
	}
}

func (w *walker) walkParagraphElement(dec *xml.Decoder, start xml.StartElement) error {
	switch start.Name.Local {
	case "r":
		return w.walkRun(dec, start)
	case "fldSimple":
		if w.opts.IncludeFieldInstructions {
			if instr := attrValue(start, "instr"); instr != "" {
				if err := w.emit(Event{Type: EventFieldInstruction, FieldInstr: instr}); err != nil {
					return err
				}
			}
		}
		return w.walkParagraphContainer(dec, start)
	case "hyperlink":
		return w.walkHyperlink(dec, start)
	case "ins", "del", "moveFrom", "moveTo":
		return w.walkRevision(dec, start)
	case "smartTag":
		return w.walkSmartTag(dec, start)
	case "customXml":
		return w.walkCustomXMLInline(dec, start)
	case "sdt":
		return w.walkSdtInline(dec, start)
	case "sdtContent":
		return w.walkParagraphContainer(dec, start)
	case "dir", "bdo":
		return w.walkDirection(dec, start)
	case "ruby":
		return w.walkRuby(dec, start)
	case "oMath":
		return w.walkMath(dec, start, false)
	case "oMathPara":
		return w.walkMath(dec, start, true)
	case "subDoc":
		return w.walkParagraphContainer(dec, start)
	default:
		return dec.Skip()
	}
}

func (w *walker) walkRevision(dec *xml.Decoder, start xml.StartElement) error {
	revisionID := attrValue(start, "id")
	author := attrValue(start, "author")
	date := attrValue(start, "date")
	if err := w.emit(Event{
		Type:         EventStartRevision,
		RevisionKind: start.Name.Local,
		RevisionID:   revisionID,
		Author:       author,
		Date:         date,
	}); err != nil {
		return err
	}
	if err := w.walkParagraphContainer(dec, start); err != nil {
		return err
	}
	return w.emit(Event{
		Type:         EventEndRevision,
		RevisionKind: start.Name.Local,
		RevisionID:   revisionID,
		Author:       author,
		Date:         date,
	})
}

func (w *walker) walkSmartTag(dec *xml.Decoder, start xml.StartElement) error {
	event := Event{
		MarkupURI:     attrValue(start, "uri"),
		MarkupElement: attrValue(start, "element"),
	}
	event.Type = EventStartSmartTag
	if err := w.emit(event); err != nil {
		return err
	}
	if err := w.walkParagraphContainer(dec, start); err != nil {
		return err
	}
	event.Type = EventEndSmartTag
	return w.emit(event)
}

func (w *walker) walkCustomXMLInline(dec *xml.Decoder, start xml.StartElement) error {
	model, contentStart, err := parseCustomXML(dec, start)
	if err != nil {
		return err
	}
	event := Event{
		MarkupURI:      model.URI,
		MarkupElement:  model.Element,
		CustomXMLModel: model,
	}
	event.Type = EventStartCustomXML
	if err := w.emit(event); err != nil {
		return err
	}
	if err := w.walkCustomXMLInlineContent(dec, start.Name, contentStart); err != nil {
		return err
	}
	event.Type = EventEndCustomXML
	return w.emit(event)
}

func (w *walker) walkCustomXMLBlock(dec *xml.Decoder, start xml.StartElement) error {
	model, contentStart, err := parseCustomXML(dec, start)
	if err != nil {
		return err
	}
	event := Event{
		MarkupURI:      model.URI,
		MarkupElement:  model.Element,
		CustomXMLModel: model,
	}
	event.Type = EventStartCustomXML
	if err := w.emit(event); err != nil {
		return err
	}
	if err := w.walkCustomXMLBlockContent(dec, start.Name, contentStart); err != nil {
		return err
	}
	event.Type = EventEndCustomXML
	return w.emit(event)
}

func (w *walker) walkDirection(dec *xml.Decoder, start xml.StartElement) error {
	event := Event{Direction: attrValue(start, "val")}
	event.Type = EventStartDirection
	if err := w.emit(event); err != nil {
		return err
	}
	if err := w.walkParagraphContainer(dec, start); err != nil {
		return err
	}
	event.Type = EventEndDirection
	return w.emit(event)
}

func (w *walker) walkSdtInline(dec *xml.Decoder, start xml.StartElement) error {
	meta, inlinesStart, err := parseSdt(dec, start)
	if err != nil {
		return err
	}
	if err := w.emit(Event{
		Type:              EventStartContentControl,
		ControlID:         meta.id,
		ControlAlias:      meta.alias,
		ControlTag:        meta.tag,
		ControlType:       meta.kind,
		ControlProperties: meta.props,
	}); err != nil {
		return err
	}
	if inlinesStart.Name.Local != "" {
		if err := w.walkParagraphContainer(dec, inlinesStart); err != nil {
			return err
		}
	}
	return w.emit(Event{
		Type:              EventEndContentControl,
		ControlID:         meta.id,
		ControlAlias:      meta.alias,
		ControlTag:        meta.tag,
		ControlType:       meta.kind,
		ControlProperties: meta.props,
	})
}

func (w *walker) walkSdtBlock(dec *xml.Decoder, start xml.StartElement) error {
	meta, contentStart, err := parseSdt(dec, start)
	if err != nil {
		return err
	}
	if err := w.emit(Event{
		Type:              EventStartContentControl,
		ControlID:         meta.id,
		ControlAlias:      meta.alias,
		ControlTag:        meta.tag,
		ControlType:       meta.kind,
		ControlProperties: meta.props,
	}); err != nil {
		return err
	}
	if contentStart.Name.Local != "" {
		if err := w.walkBodyContainer(dec, contentStart); err != nil {
			return err
		}
	}
	return w.emit(Event{
		Type:              EventEndContentControl,
		ControlID:         meta.id,
		ControlAlias:      meta.alias,
		ControlTag:        meta.tag,
		ControlType:       meta.kind,
		ControlProperties: meta.props,
	})
}

func (w *walker) walkRuby(dec *xml.Decoder, start xml.StartElement) error {
	base, text, rawXML, model, err := decodeRubyToken(dec, start)
	if err != nil {
		return err
	}
	return w.emit(Event{
		Type:         EventRuby,
		RubyBaseText: base,
		RubyText:     text,
		RubyModel:    model,
		RawXML:       rawXML,
	})
}

func (w *walker) walkMath(dec *xml.Decoder, start xml.StartElement, display bool) error {
	rawXML, err := decodeRawElement(dec, start)
	if err != nil {
		return err
	}
	mathText, err := decodeMathText(rawXML)
	if err != nil {
		return err
	}
	mathModel, err := parseMathModel(rawXML)
	if err != nil {
		return err
	}
	return w.emit(Event{
		Type:      EventMath,
		RawXML:    rawXML,
		Display:   display,
		MathText:  mathText,
		MathModel: mathModel,
	})
}

func (w *walker) walkAltChunk(dec *xml.Decoder, start xml.StartElement) error {
	rawXML, model, err := decodeAltChunkToken(dec, start)
	if err != nil {
		return err
	}
	event := Event{
		Type:          EventAltChunk,
		RawXML:        rawXML,
		AltChunkModel: model,
	}
	if model != nil && model.Properties != nil {
		event.MatchSource = model.Properties.MatchSrc == nil || model.Properties.MatchSrc.Val == nil || bool(*model.Properties.MatchSrc.Val)
	}
	w.populateRelationship(&event, attrValue(start, "id"))
	event.RelatedParts = w.describeRelationships([]string{event.RelationshipID})
	w.populatePayload(&event)
	return w.emit(event)
}

func (w *walker) walkHyperlink(dec *xml.Decoder, start xml.StartElement) error {
	relID := attrValue(start, "id")
	startEvent := Event{
		Type:            EventStartHyperlink,
		HyperlinkAnchor: attrValue(start, "anchor"),
	}
	w.populateRelationship(&startEvent, relID)
	if startEvent.TargetExternal && startEvent.RelationshipType == relationshipTypeHyperlink {
		startEvent.HyperlinkTarget = startEvent.RelationshipTarget
	}
	if err := w.emit(startEvent); err != nil {
		return err
	}
	if err := w.walkParagraphContainer(dec, start); err != nil {
		return err
	}
	endEvent := Event{
		Type:            EventEndHyperlink,
		HyperlinkAnchor: attrValue(start, "anchor"),
	}
	w.populateRelationship(&endEvent, relID)
	if endEvent.TargetExternal && endEvent.RelationshipType == relationshipTypeHyperlink {
		endEvent.HyperlinkTarget = endEvent.RelationshipTarget
	}
	return w.emit(endEvent)
}

func (w *walker) walkParagraphContainer(dec *xml.Decoder, start xml.StartElement) error {
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if err := w.walkParagraphElement(dec, t); err != nil {
				return err
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return nil
			}
		}
	}
}

func (w *walker) walkRun(dec *xml.Decoder, start xml.StartElement) error {
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "t", "delText":
				var text string
				if err := dec.DecodeElement(&text, &t); err != nil {
					return err
				}
				if text != "" {
					if err := w.emit(Event{Type: EventText, Text: text}); err != nil {
						return err
					}
				}
			case "instrText":
				var instr string
				if err := dec.DecodeElement(&instr, &t); err != nil {
					return err
				}
				if w.opts.IncludeFieldInstructions && instr != "" {
					if err := w.emit(Event{Type: EventFieldInstruction, FieldInstr: instr}); err != nil {
						return err
					}
				}
			case "delInstrText":
				var instr string
				if err := dec.DecodeElement(&instr, &t); err != nil {
					return err
				}
				if w.opts.IncludeFieldInstructions && instr != "" {
					if err := w.emit(Event{Type: EventFieldInstruction, FieldInstr: instr}); err != nil {
						return err
					}
				}
			case "fldChar":
				if err := w.emit(Event{
					Type:          EventFieldChar,
					FieldCharType: attrValue(t, "fldCharType"),
				}); err != nil {
					return err
				}
				if err := dec.Skip(); err != nil {
					return err
				}
			case "footnoteReference":
				if err := w.emit(Event{
					Type:   EventFootnoteReference,
					NoteID: attrValue(t, "id"),
				}); err != nil {
					return err
				}
				if err := dec.Skip(); err != nil {
					return err
				}
			case "endnoteReference":
				if err := w.emit(Event{
					Type:   EventEndnoteReference,
					NoteID: attrValue(t, "id"),
				}); err != nil {
					return err
				}
				if err := dec.Skip(); err != nil {
					return err
				}
			case "commentReference":
				if err := w.emit(Event{
					Type:      EventCommentReference,
					CommentID: attrValue(t, "id"),
				}); err != nil {
					return err
				}
				if err := dec.Skip(); err != nil {
					return err
				}
			case "annotationRef":
				if err := w.emit(Event{Type: EventAnnotationReference}); err != nil {
					return err
				}
				if err := dec.Skip(); err != nil {
					return err
				}
			case "commentRangeStart":
				if err := w.emit(Event{
					Type:      EventCommentRangeStart,
					CommentID: attrValue(t, "id"),
				}); err != nil {
					return err
				}
				if err := dec.Skip(); err != nil {
					return err
				}
			case "commentRangeEnd":
				if err := w.emit(Event{
					Type:      EventCommentRangeEnd,
					CommentID: attrValue(t, "id"),
				}); err != nil {
					return err
				}
				if err := dec.Skip(); err != nil {
					return err
				}
			case "tab":
				if err := w.emit(Event{Type: EventTab}); err != nil {
					return err
				}
				if err := dec.Skip(); err != nil {
					return err
				}
			case "ptab":
				if err := w.emit(Event{Type: EventTab}); err != nil {
					return err
				}
				if err := dec.Skip(); err != nil {
					return err
				}
			case "br":
				breakType := attrValue(t, "type")
				if breakType == "" {
					breakType = "textWrapping"
				}
				if err := w.emit(Event{Type: EventBreak, BreakType: breakType}); err != nil {
					return err
				}
				if err := dec.Skip(); err != nil {
					return err
				}
			case "cr":
				if err := w.emit(Event{Type: EventBreak, BreakType: "carriageReturn"}); err != nil {
					return err
				}
				if err := dec.Skip(); err != nil {
					return err
				}
			case "col":
				if err := w.emit(Event{Type: EventBreak, BreakType: "column"}); err != nil {
					return err
				}
				if err := dec.Skip(); err != nil {
					return err
				}
			case "lastRenderedPageBreak":
				if err := w.emit(Event{Type: EventBreak, BreakType: "renderedPage"}); err != nil {
					return err
				}
				if err := dec.Skip(); err != nil {
					return err
				}
			case "softHyphen":
				if err := w.emit(Event{Type: EventSoftHyphen, Text: "\u00ad"}); err != nil {
					return err
				}
				if err := dec.Skip(); err != nil {
					return err
				}
			case "noBreakHyphen":
				if err := w.emit(Event{Type: EventNoBreakHyphen, Text: "\u2011"}); err != nil {
					return err
				}
				if err := dec.Skip(); err != nil {
					return err
				}
			case "sym":
				text, font, char, err := decodeSymbolToken(dec, t)
				if err != nil {
					return err
				}
				if err := w.emit(Event{
					Type:       EventSymbol,
					Text:       text,
					SymbolFont: font,
					SymbolChar: char,
				}); err != nil {
					return err
				}
			case "drawing":
				rawXML, drawing, err := decodeDrawingToken(dec, t, w.relationshipRefs)
				if err != nil {
					return err
				}
				event := Event{
					Type:                   EventDrawing,
					RawXML:                 rawXML,
					DrawingKind:            drawing.kind,
					DrawingDocID:           drawing.docID,
					DrawingName:            drawing.name,
					DrawingDescription:     drawing.description,
					DrawingTitle:           drawing.title,
					DrawingHidden:          drawing.hidden,
					WidthEMU:               drawing.widthEMU,
					HeightEMU:              drawing.heightEMU,
					GraphicDataURI:         drawing.graphicDataURI,
					DrawingGraphicKind:     drawing.graphicKind,
					DrawingHasTextBox:      drawing.hasTextBox,
					DrawingModel:           drawing.model,
					RelatedRelationshipIDs: drawing.relationshipIDs,
					RelatedTargets:         drawing.targets,
				}
				setPrimaryRelationship(&event, drawing.relationshipIDs, drawing.targets, w.relationshipRefs)
				event.RelatedParts = w.describeRelationships(drawing.relationshipIDs)
				if err := w.emit(event); err != nil {
					return err
				}
			case "object":
				rawXML, object, err := decodeObjectToken(dec, t, w.relationshipRefs)
				if err != nil {
					return err
				}
				event := Event{
					Type:                   EventEmbeddedObject,
					RawXML:                 rawXML,
					ProgID:                 object.progID,
					EmbeddedObjectModel:    object.model,
					RelatedRelationshipIDs: object.relationshipIDs,
					RelatedTargets:         object.targets,
				}
				setPrimaryRelationship(&event, object.relationshipIDs, object.targets, w.relationshipRefs)
				event.RelatedParts = w.describeRelationships(object.relationshipIDs)
				if err := w.emit(event); err != nil {
					return err
				}
			case "contentPart":
				event := Event{
					Type:             EventContentPart,
					ContentPartModel: &wordparts.ContentPartRef{RelID: attrValue(t, "id")},
				}
				w.populateRelationship(&event, attrValue(t, "id"))
				event.RelatedParts = w.describeRelationships([]string{event.RelationshipID})
				w.populatePayload(&event)
				if err := w.emit(event); err != nil {
					return err
				}
				if err := dec.Skip(); err != nil {
					return err
				}
			case "ruby":
				if err := w.walkRuby(dec, t); err != nil {
					return err
				}
			case "oMath":
				if err := w.walkMath(dec, t, false); err != nil {
					return err
				}
			case "oMathPara":
				if err := w.walkMath(dec, t, true); err != nil {
					return err
				}
			case "dayShort", "dayLong", "monthShort", "monthLong", "yearShort", "yearLong", "pgNum":
				if err := w.emit(Event{Type: EventAutoText, AutoTextKind: t.Name.Local}); err != nil {
					return err
				}
				if err := dec.Skip(); err != nil {
					return err
				}
			default:
				if err := dec.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return nil
			}
		}
	}
}

func (w *walker) walkTable(dec *xml.Decoder, start xml.StartElement) error {
	if err := w.emit(Event{Type: EventStartTable}); err != nil {
		return err
	}
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "tr" {
				if err := w.walkTableRow(dec, t); err != nil {
					return err
				}
				continue
			}
			if err := dec.Skip(); err != nil {
				return err
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return w.emit(Event{Type: EventEndTable})
			}
		}
	}
}

func (w *walker) walkTableRow(dec *xml.Decoder, start xml.StartElement) error {
	if err := w.emit(Event{Type: EventStartTableRow}); err != nil {
		return err
	}
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "tc" {
				if err := w.walkTableCell(dec, t); err != nil {
					return err
				}
				continue
			}
			if err := dec.Skip(); err != nil {
				return err
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return w.emit(Event{Type: EventEndTableRow})
			}
		}
	}
}

func (w *walker) walkTableCell(dec *xml.Decoder, start xml.StartElement) error {
	if err := w.emit(Event{Type: EventStartTableCell}); err != nil {
		return err
	}
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "p":
				if err := w.walkParagraph(dec, t); err != nil {
					return err
				}
			case "tbl":
				if err := w.walkTable(dec, t); err != nil {
					return err
				}
			case "sdt":
				if err := w.walkSdtBlock(dec, t); err != nil {
					return err
				}
			case "customXml":
				if err := w.walkCustomXMLBlock(dec, t); err != nil {
					return err
				}
			case "sdtContent":
				if err := w.walkBodyContainer(dec, t); err != nil {
					return err
				}
			default:
				if err := dec.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return w.emit(Event{Type: EventEndTableCell})
			}
		}
	}
}

func (w *walker) emit(event Event) error {
	event.Story = w.story
	event.PartName = w.partName
	return w.handler.HandleEvent(event)
}

func (w *walker) populateRelationship(event *Event, relID string) {
	if event == nil {
		return
	}
	event.RelationshipID = relID
	if relID == "" || w.relationshipRefs == nil {
		return
	}
	ref, ok := w.relationshipRefs[relID]
	if !ok {
		return
	}
	event.RelationshipType = ref.relType
	event.RelationshipTarget = ref.target
	event.TargetExternal = ref.external
}

func (w *walker) populatePayload(event *Event) {
	if event == nil || event.RelationshipTarget == "" || event.TargetExternal || w.lookupPart == nil {
		return
	}
	contentType, data, ok := w.lookupPart(event.RelationshipTarget)
	if !ok {
		return
	}
	event.PayloadContentType = contentType
	event.PayloadSize = int64(len(data))
	event.PayloadData = data
}

func (w *walker) describeRelationships(relIDs []string) []RelatedPart {
	if len(relIDs) == 0 {
		return nil
	}
	parts := make([]RelatedPart, 0, len(relIDs))
	for _, relID := range relIDs {
		ref, ok := w.relationshipRefs[relID]
		if !ok {
			continue
		}
		part := RelatedPart{
			RelationshipID:   relID,
			RelationshipType: ref.relType,
			Target:           ref.target,
			External:         ref.external,
		}
		if !ref.external && ref.target != "" && w.lookupPart != nil {
			if contentType, data, ok := w.lookupPart(ref.target); ok {
				part.ContentType = contentType
				part.Size = int64(len(data))
			}
		}
		parts = append(parts, part)
	}
	return parts
}

func setPrimaryRelationship(event *Event, relIDs, targets []string, refs map[string]relationshipRef) {
	if event == nil || len(relIDs) == 0 {
		return
	}
	event.RelationshipID = relIDs[0]
	if len(targets) > 0 {
		event.RelationshipTarget = targets[0]
	}
	if refs == nil {
		return
	}
	if ref, ok := refs[relIDs[0]]; ok {
		event.RelationshipType = ref.relType
		event.TargetExternal = ref.external
	}
}

type sdtMeta struct {
	id    string
	alias string
	tag   string
	kind  string
	props *wordparts.SdtProperties
}

func parseSdt(dec *xml.Decoder, start xml.StartElement) (sdtMeta, xml.StartElement, error) {
	var meta sdtMeta
	for {
		token, err := dec.Token()
		if err != nil {
			return sdtMeta{}, xml.StartElement{}, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "sdtPr":
				m, err := parseSdtProperties(dec, t)
				if err != nil {
					return sdtMeta{}, xml.StartElement{}, err
				}
				meta = m
			case "sdtContent":
				return meta, t, nil
			default:
				if err := dec.Skip(); err != nil {
					return sdtMeta{}, xml.StartElement{}, err
				}
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return meta, xml.StartElement{}, nil
			}
		}
	}
}

func parseCustomXML(dec *xml.Decoder, start xml.StartElement) (*wordparts.CustomXml, xml.StartElement, error) {
	model := &wordparts.CustomXml{
		URI:     attrValue(start, "uri"),
		Element: attrValue(start, "element"),
	}
	for {
		token, err := dec.Token()
		if err != nil {
			return nil, xml.StartElement{}, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "customXmlPr" {
				var pr wordparts.CustomXmlPr
				if err := dec.DecodeElement(&pr, &t); err != nil {
					return nil, xml.StartElement{}, err
				}
				model.Properties = &pr
				continue
			}
			return model, t, nil
		case xml.EndElement:
			if t.Name == start.Name {
				return model, xml.StartElement{}, nil
			}
		}
	}
}

func (w *walker) walkCustomXMLInlineContent(dec *xml.Decoder, wrapper xml.Name, first xml.StartElement) error {
	if first.Name.Local != "" {
		if err := w.walkParagraphElement(dec, first); err != nil {
			return err
		}
	}
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if err := w.walkParagraphElement(dec, t); err != nil {
				return err
			}
		case xml.EndElement:
			if t.Name == wrapper {
				return nil
			}
		}
	}
}

func (w *walker) walkCustomXMLBlockContent(dec *xml.Decoder, wrapper xml.Name, first xml.StartElement) error {
	if first.Name.Local != "" {
		if err := w.walkBlockChild(dec, first); err != nil {
			return err
		}
	}
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if err := w.walkBlockChild(dec, t); err != nil {
				return err
			}
		case xml.EndElement:
			if t.Name == wrapper {
				return nil
			}
		}
	}
}

func (w *walker) walkBlockChild(dec *xml.Decoder, start xml.StartElement) error {
	switch start.Name.Local {
	case "p":
		return w.walkParagraph(dec, start)
	case "tbl":
		return w.walkTable(dec, start)
	case "sdt":
		return w.walkSdtBlock(dec, start)
	case "customXml":
		return w.walkCustomXMLBlock(dec, start)
	case "altChunk":
		return w.walkAltChunk(dec, start)
	default:
		return dec.Skip()
	}
}

func parseSdtProperties(dec *xml.Decoder, start xml.StartElement) (sdtMeta, error) {
	meta := sdtMeta{props: &wordparts.SdtProperties{}}
	for {
		token, err := dec.Token()
		if err != nil {
			return sdtMeta{}, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "id":
				meta.id = attrValue(t, "val")
				if id, err := strconv.Atoi(meta.id); err == nil {
					meta.props.ID = id
				}
				if err := dec.Skip(); err != nil {
					return sdtMeta{}, err
				}
			case "alias":
				meta.alias = attrValue(t, "val")
				meta.props.Alias = meta.alias
				if err := dec.Skip(); err != nil {
					return sdtMeta{}, err
				}
			case "tag":
				meta.tag = attrValue(t, "val")
				meta.props.Tag = meta.tag
				if err := dec.Skip(); err != nil {
					return sdtMeta{}, err
				}
			case "text":
				if meta.kind == "" {
					meta.kind = "plainText"
					meta.props.Type = meta.kind
				}
				if err := dec.Skip(); err != nil {
					return sdtMeta{}, err
				}
			case "checked":
				meta.props.Checked = parseOnOffAttr(attrValue(t, "val"), true)
				if err := dec.Skip(); err != nil {
					return sdtMeta{}, err
				}
			case "checkbox":
				if err := parseSdtCheckbox14(dec, t, meta.props); err != nil {
					return sdtMeta{}, err
				}
			case "richText", "checkBox", "comboBox", "dropDownList", "date", "picture", "docPartObj", "docPartList", "citation", "equation", "group", "repeatingSection", "repeatingSectionItem", "contentPart":
				if meta.kind == "" {
					meta.kind = t.Name.Local
					meta.props.Type = meta.kind
				}
				switch t.Name.Local {
				case "date":
					if err := parseSdtDate(dec, t, meta.props); err != nil {
						return sdtMeta{}, err
					}
				case "docPartObj":
					if err := parseSdtDocPartObj(dec, t, meta.props); err != nil {
						return sdtMeta{}, err
					}
				case "comboBox", "dropDownList":
					if err := parseSdtList(dec, t, meta.props); err != nil {
						return sdtMeta{}, err
					}
				default:
					if err := dec.Skip(); err != nil {
						return sdtMeta{}, err
					}
				}
			case "showingPlcHdr":
				meta.props.ShowingPlcHdr = true
				if err := dec.Skip(); err != nil {
					return sdtMeta{}, err
				}
			case "placeholder":
				if err := parseSdtPlaceholder(dec, t, meta.props); err != nil {
					return sdtMeta{}, err
				}
			case "dataBinding":
				meta.props.DataXPath = attrValue(t, "xpath")
				meta.props.DataStoreID = attrValue(t, "storeItemID")
				if err := dec.Skip(); err != nil {
					return sdtMeta{}, err
				}
			default:
				if err := dec.Skip(); err != nil {
					return sdtMeta{}, err
				}
			}
		case xml.EndElement:
			if t.Name == start.Name {
				if meta.props.Type == "" {
					meta.props.Type = meta.kind
				}
				return meta, nil
			}
		}
	}
}

func parseSdtCheckbox14(dec *xml.Decoder, start xml.StartElement, props *wordparts.SdtProperties) error {
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "checked" {
				props.Checked14 = parseOnOffAttr(attrValue(t, "val"), true)
			}
			if err := dec.Skip(); err != nil {
				return err
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return nil
			}
		}
	}
}

func parseSdtDate(dec *xml.Decoder, start xml.StartElement, props *wordparts.SdtProperties) error {
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "dateFormat":
				props.DateFormat = attrValue(t, "val")
			case "fullDate":
				props.DateFullDate = attrValue(t, "val")
			case "lid":
				props.DateLanguage = attrValue(t, "val")
			case "calendar":
				props.DateCalendar = attrValue(t, "val")
			}
			if err := dec.Skip(); err != nil {
				return err
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return nil
			}
		}
	}
}

func parseSdtDocPartObj(dec *xml.Decoder, start xml.StartElement, props *wordparts.SdtProperties) error {
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "docPartGallery":
				props.DocPartGallery = attrValue(t, "val")
			case "docPartCategory":
				props.DocPartCategory = attrValue(t, "val")
			case "docPartUnique":
				props.DocPartUnique = true
			}
			if err := dec.Skip(); err != nil {
				return err
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return nil
			}
		}
	}
}

func parseSdtList(dec *xml.Decoder, start xml.StartElement, props *wordparts.SdtProperties) error {
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "listItem" {
				props.ListItems = append(props.ListItems, wordparts.SdtListItem{
					DisplayText: attrValue(t, "displayText"),
					Value:       attrValue(t, "value"),
				})
			}
			if err := dec.Skip(); err != nil {
				return err
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return nil
			}
		}
	}
}

func parseSdtPlaceholder(dec *xml.Decoder, start xml.StartElement, props *wordparts.SdtProperties) error {
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "docPart":
				props.PlaceholderDocPart = attrValue(t, "val")
			case "docPartGallery":
				props.PlaceholderDocPartGallery = attrValue(t, "val")
			}
			if err := dec.Skip(); err != nil {
				return err
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return nil
			}
		}
	}
}

func decodeSymbolToken(dec *xml.Decoder, start xml.StartElement) (text, font, char string, err error) {
	font = attrValue(start, "font")
	char = attrValue(start, "char")
	if err := dec.Skip(); err != nil {
		return "", "", "", err
	}
	r := symToRune(char)
	if r != 0 {
		text = string(r)
	}
	return text, font, char, nil
}

func decodeRubyToken(dec *xml.Decoder, start xml.StartElement) (base, text, rawXML string, model *wordparts.Ruby, err error) {
	rawXML, err = decodeRawElement(dec, start)
	if err != nil {
		return "", "", "", nil, err
	}
	rubyModel, err := parseRubyModel(rawXML)
	if err != nil {
		return "", "", "", nil, err
	}
	base = collectRubyRunText(rubyModel.BaseRuns)
	text = collectRubyRunText(rubyModel.TextRuns)
	return base, text, rawXML, rubyModel, nil
}

func collectSimpleText(dec *xml.Decoder, start xml.StartElement) (string, error) {
	var b strings.Builder
	for {
		token, err := dec.Token()
		if err != nil {
			return "", err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "t", "delText":
				var text string
				if err := dec.DecodeElement(&text, &t); err != nil {
					return "", err
				}
				b.WriteString(text)
			default:
				value, err := collectSimpleText(dec, t)
				if err != nil {
					return "", err
				}
				b.WriteString(value)
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return b.String(), nil
			}
		}
	}
}

func decodeRawElement(dec *xml.Decoder, start xml.StartElement) (string, error) {
	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	if err := enc.EncodeToken(start); err != nil {
		return "", err
	}
	depth := 1
	for depth > 0 {
		token, err := dec.Token()
		if err != nil {
			return "", err
		}
		switch token.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		}
		if err := enc.EncodeToken(token); err != nil {
			return "", err
		}
	}
	if err := enc.Flush(); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func decodeMathText(rawXML string) (string, error) {
	var b strings.Builder
	dec := xml.NewDecoder(strings.NewReader(rawXML))
	for {
		token, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				return b.String(), nil
			}
			return "", err
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "t" {
			continue
		}
		var text string
		if err := dec.DecodeElement(&text, &start); err != nil {
			return "", err
		}
		b.WriteString(text)
	}
}

func decodeAltChunkToken(dec *xml.Decoder, start xml.StartElement) (string, *wordparts.AltChunk, error) {
	rawXML, err := decodeRawElement(dec, start)
	if err != nil {
		return "", nil, err
	}
	var chunk wordparts.AltChunk
	if err := xml.Unmarshal([]byte(rawXML), &chunk); err != nil {
		return "", nil, err
	}
	return rawXML, &chunk, nil
}

func decodeDrawingToken(dec *xml.Decoder, start xml.StartElement, refs map[string]relationshipRef) (rawXML string, drawing drawingToken, err error) {
	rawXML, err = decodeRawElement(dec, start)
	if err != nil {
		return "", drawingToken{}, err
	}
	drawing, err = summarizeDrawingToken(rawXML, refs)
	if err != nil {
		return "", drawingToken{}, err
	}
	return rawXML, drawing, nil
}

func summarizeDrawingToken(rawXML string, refs map[string]relationshipRef) (drawingToken, error) {
	var drawing wordparts.Drawing
	if err := xml.Unmarshal([]byte(rawXML), &drawing); err != nil {
		return drawingToken{}, err
	}
	out := drawingToken{model: &drawing}
	var docPr *wordparts.DrawingDocPr
	var graphicData *wordparts.GraphicData
	switch {
	case drawing.Inline != nil:
		out.kind = "inline"
		if drawing.Inline.Extent != nil {
			out.widthEMU = drawing.Inline.Extent.CX
			out.heightEMU = drawing.Inline.Extent.CY
		}
		docPr = firstDrawingDocPr(drawing.Inline.DocPr, drawing.Inline.CNvPr)
		if drawing.Inline.Graphic != nil {
			graphicData = drawing.Inline.Graphic.GraphicData
		}
	case drawing.Anchor != nil:
		out.kind = "anchor"
		if drawing.Anchor.Extent != nil {
			out.widthEMU = drawing.Anchor.Extent.CX
			out.heightEMU = drawing.Anchor.Extent.CY
		}
		docPr = firstDrawingDocPr(drawing.Anchor.DocPr, drawing.Anchor.CNvPr)
		if drawing.Anchor.Graphic != nil {
			graphicData = drawing.Anchor.Graphic.GraphicData
		}
	}
	if docPr != nil {
		out.docID = strconv.Itoa(docPr.ID)
		out.name = docPr.Name
		out.description = docPr.Desc
		out.title = docPr.Title
		out.hidden = docPr.Hidden
	}
	if graphicData != nil {
		out.graphicDataURI = graphicData.URI
		out.graphicKind = classifyGraphicData(graphicData)
		out.hasTextBox = graphicHasTextBox(graphicData)
		out.relationshipIDs = appendUnique(out.relationshipIDs, collectGraphicRelationshipIDs(graphicData)...)
	}
	out.targets = resolveRelationshipTargets(refs, out.relationshipIDs)
	return out, nil
}

func decodeObjectToken(dec *xml.Decoder, start xml.StartElement, refs map[string]relationshipRef) (rawXML string, object objectToken, err error) {
	rawXML, err = decodeRawElement(dec, start)
	if err != nil {
		return "", objectToken{}, err
	}
	r := xml.NewDecoder(strings.NewReader(rawXML))
	for {
		token, err := r.Token()
		if err != nil {
			if err == io.EOF {
				object.targets = resolveRelationshipTargets(refs, object.relationshipIDs)
				return rawXML, object, nil
			}
			return "", objectToken{}, err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		switch start.Name.Local {
		case "OLEObject":
			object.progID = attrValue(start, "ProgID")
			object.relationshipIDs = appendUnique(object.relationshipIDs, attrValue(start, "id"))
			object.model = &wordparts.OLEObjectRun{ProgID: object.progID}
		case "imagedata":
			object.relationshipIDs = appendUnique(object.relationshipIDs, attrValue(start, "id"))
		}
	}
}

func resolveRelationshipTargets(refs map[string]relationshipRef, relIDs []string) []string {
	if len(relIDs) == 0 {
		return nil
	}
	targets := make([]string, 0, len(relIDs))
	for _, relID := range relIDs {
		ref, ok := refs[relID]
		if !ok || ref.target == "" {
			continue
		}
		targets = appendUnique(targets, ref.target)
	}
	return targets
}

func firstDrawingDocPr(values ...*wordparts.DrawingDocPr) *wordparts.DrawingDocPr {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func classifyGraphicData(graphicData *wordparts.GraphicData) string {
	if graphicData == nil {
		return ""
	}
	switch {
	case graphicData.Picture != nil || graphicData.DrawingMLPic != nil:
		return "picture"
	case graphicData.Chart != nil:
		return "chart"
	case graphicData.DiagramIDs != nil:
		return "diagram"
	case graphicData.Shape != nil || graphicData.DrawingMLShape != nil:
		return "shape"
	case graphicData.GroupShape != nil || graphicData.DrawingMLGroupShape != nil:
		return "groupShape"
	case graphicData.WordprocessingGroup != nil:
		return "wordprocessingGroup"
	case graphicData.Canvas != nil:
		return "canvas"
	case graphicData.GraphicFrame != nil || graphicData.DrawingMLGraphicFr != nil:
		return "graphicFrame"
	case graphicData.ContentPart != nil:
		return "contentPart"
	case graphicData.Table != nil:
		return "table"
	default:
		return normalizeGraphicDataURI(graphicData.URI)
	}
}

func graphicHasTextBox(graphicData *wordparts.GraphicData) bool {
	if graphicData == nil {
		return false
	}
	if graphicData.Shape != nil && graphicData.Shape.TextBox != nil {
		return true
	}
	if graphicData.GroupShape != nil && graphicData.GroupShape.Shape != nil && graphicData.GroupShape.Shape.TextBox != nil {
		return true
	}
	if graphicData.WordprocessingGroup != nil && graphicData.WordprocessingGroup.Shape != nil && graphicData.WordprocessingGroup.Shape.TextBox != nil {
		return true
	}
	if graphicData.Canvas != nil && graphicData.Canvas.Shape != nil && graphicData.Canvas.Shape.TextBox != nil {
		return true
	}
	return false
}

func collectGraphicRelationshipIDs(graphicData *wordparts.GraphicData) []string {
	var relIDs []string
	if graphicData == nil {
		return nil
	}
	if graphicData.Picture != nil && graphicData.Picture.BlipFill != nil && graphicData.Picture.BlipFill.Blip != nil {
		relIDs = appendUnique(relIDs, graphicData.Picture.BlipFill.Blip.Embed)
	}
	if graphicData.DrawingMLPic != nil && graphicData.DrawingMLPic.BlipFill != nil && graphicData.DrawingMLPic.BlipFill.Blip != nil {
		relIDs = appendUnique(relIDs, graphicData.DrawingMLPic.BlipFill.Blip.Embed)
	}
	if graphicData.Chart != nil {
		relIDs = appendUnique(relIDs, graphicData.Chart.ID)
	}
	if graphicData.DiagramIDs != nil {
		relIDs = appendUnique(relIDs,
			graphicData.DiagramIDs.DataModelID,
			graphicData.DiagramIDs.LayoutID,
			graphicData.DiagramIDs.QuickStyleID,
			graphicData.DiagramIDs.ColorStyleID,
		)
	}
	return relIDs
}

func appendUnique(values []string, added ...string) []string {
	for _, value := range added {
		if value == "" {
			continue
		}
		exists := false
		for _, existing := range values {
			if existing == value {
				exists = true
				break
			}
		}
		if !exists {
			values = append(values, value)
		}
	}
	return values
}

func parseInt64Attr(start xml.StartElement, local string) int64 {
	value := strings.TrimSpace(attrValue(start, local))
	if value == "" {
		return 0
	}
	v, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

func parseOnOffAttr(value string, defaultValue bool) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "":
		return defaultValue
	case "1", "true", "t", "on", "yes":
		return true
	case "0", "false", "f", "off", "no":
		return false
	default:
		return defaultValue
	}
}

func collectRubyRunText(runs []*wordparts.Run) string {
	var b strings.Builder
	for _, run := range runs {
		if run == nil {
			continue
		}
		for _, text := range run.Text {
			if text != nil {
				b.WriteString(text.Value)
			}
		}
		for _, content := range run.Contents {
			if content.Text != nil {
				b.WriteString(content.Text.Value)
			}
		}
	}
	return b.String()
}

func parseRubyModel(rawXML string) (*wordparts.Ruby, error) {
	model := &wordparts.Ruby{}
	dec := xml.NewDecoder(strings.NewReader(rawXML))
	for {
		token, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				return model, nil
			}
			return nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		switch start.Name.Local {
		case "rubyPr":
			var pr wordparts.RubyPr
			if err := dec.DecodeElement(&pr, &start); err != nil {
				return nil, err
			}
			model.RubyPr = &pr
		case "rubyBase":
			value, err := collectSimpleText(dec, start)
			if err != nil {
				return nil, err
			}
			model.BaseRuns = append(model.BaseRuns, textRun(value))
		case "rt":
			value, err := collectSimpleText(dec, start)
			if err != nil {
				return nil, err
			}
			model.TextRuns = append(model.TextRuns, textRun(value))
		}
	}
}

func textRun(value string) *wordparts.Run {
	if value == "" {
		return &wordparts.Run{}
	}
	return &wordparts.Run{
		Text: []*wordparts.Text{{Value: value}},
	}
}

func parseMathModel(rawXML string) (*MathNode, error) {
	dec := xml.NewDecoder(strings.NewReader(rawXML))
	for {
		token, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				return nil, nil
			}
			return nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		return parseMathNode(dec, start)
	}
}

func parseMathNode(dec *xml.Decoder, start xml.StartElement) (*MathNode, error) {
	node := &MathNode{
		Kind:       start.Name.Local,
		Attributes: make(map[string]string, len(start.Attr)),
	}
	for _, attr := range start.Attr {
		key := attr.Name.Local
		if key == "" {
			key = attr.Name.Space
		}
		node.Attributes[key] = attr.Value
	}
	var text strings.Builder
	for {
		token, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			child, err := parseMathNode(dec, t)
			if err != nil {
				return nil, err
			}
			node.Children = append(node.Children, child)
		case xml.CharData:
			value := strings.TrimSpace(string(t))
			if value != "" {
				text.WriteString(value)
			}
		case xml.EndElement:
			if t.Name == start.Name {
				node.Text = text.String()
				if len(node.Attributes) == 0 {
					node.Attributes = nil
				}
				return node, nil
			}
		}
	}
}

func normalizeGraphicDataURI(uri string) string {
	uri = strings.TrimSpace(strings.ToLower(uri))
	switch {
	case strings.Contains(uri, "/chart"):
		return "chart"
	case strings.Contains(uri, "/diagram"):
		return "diagram"
	case strings.Contains(uri, "/picture"):
		return "picture"
	default:
		return uri
	}
}

func extension(name string) string {
	name = normalizePartName(name)
	lastDot := strings.LastIndexByte(name, '.')
	if lastDot < 0 || lastDot == len(name)-1 {
		return ""
	}
	return name[lastDot+1:]
}

func symToRune(hexChar string) rune {
	hexChar = strings.TrimSpace(hexChar)
	if hexChar == "" {
		return 0
	}
	v, err := strconv.ParseInt(hexChar, 16, 64)
	if err != nil {
		return 0
	}
	if v >= 0xF000 && v <= 0xF0FF {
		v -= 0xF000
	}
	return rune(v)
}

func attrValue(start xml.StartElement, local string) string {
	for _, attr := range start.Attr {
		if attr.Name.Local == local {
			return attr.Value
		}
	}
	return ""
}

func readZipFile(file *zip.File) ([]byte, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rc.Close()
	}()
	return io.ReadAll(rc)
}

func normalizePartName(name string) string {
	var b bytes.Buffer
	for i := 0; i < len(name); i++ {
		ch := name[i]
		if ch == '\\' {
			ch = '/'
		}
		if ch >= 'A' && ch <= 'Z' {
			ch += 'a' - 'A'
		}
		b.WriteByte(ch)
	}
	out := b.String()
	for len(out) > 0 && (out[0] == '/' || (len(out) >= 2 && out[:2] == "./")) {
		if out[0] == '/' {
			out = out[1:]
		} else {
			out = out[2:]
		}
	}
	return out
}