package sax

import (
	"io"
	"strings"

	wordparts "office/pkg/word/parsers/ooxml/parts"
)

// Summary is a higher-level typed view collected from the SAX stream.
type Summary struct {
	Paragraphs      []Paragraph
	Hyperlinks      []Hyperlink
	ContentControls []ContentControl
	Math            []Math
	Drawings        []Drawing
	EmbeddedObjects []EmbeddedObject
	ContentParts    []ContentPart
	AltChunks       []AltChunk
}

// Paragraph is a typed paragraph summary assembled from SAX events.
type Paragraph struct {
	Story    StoryKind
	PartName string
	StyleID  string
	Text     string
	InTable  bool
}

// Hyperlink is a typed hyperlink summary assembled from SAX events.
type Hyperlink struct {
	Story              StoryKind
	PartName           string
	Text               string
	RelationshipID     string
	RelationshipType   string
	RelationshipTarget string
	Anchor             string
	Target             string
}

// ContentControl is a typed content-control summary assembled from SAX events.
type ContentControl struct {
	Story      StoryKind
	PartName   string
	Text       string
	ID         string
	Alias      string
	Tag        string
	Type       string
	Properties *wordparts.SdtProperties
}

// Math is a typed math summary assembled from SAX events.
type Math struct {
	Story    StoryKind
	PartName string
	Text     string
	Display  bool
	Model    *MathNode
	RawXML   string
}

// Drawing is a typed drawing summary assembled from SAX events.
type Drawing struct {
	Story              StoryKind
	PartName           string
	Kind               string
	GraphicKind        string
	RelationshipID     string
	RelationshipType   string
	RelationshipTarget string
	DrawingDocID       string
	Name               string
	Description        string
	Title              string
	Hidden             bool
	WidthEMU           int64
	HeightEMU          int64
	HasTextBox         bool
	RelatedParts       []RelatedPart
	Model              *wordparts.Drawing
}

// EmbeddedObject is a typed embedded-object summary assembled from SAX events.
type EmbeddedObject struct {
	Story              StoryKind
	PartName           string
	ProgID             string
	RelationshipID     string
	RelationshipType   string
	RelationshipTarget string
	RelatedParts       []RelatedPart
	Model              *wordparts.OLEObjectRun
}

// ContentPart is a typed content-part summary assembled from SAX events.
type ContentPart struct {
	Story              StoryKind
	PartName           string
	RelationshipID     string
	RelationshipType   string
	RelationshipTarget string
	PayloadContentType string
	PayloadSize        int64
	PayloadData        []byte
	RelatedParts       []RelatedPart
	Model              *wordparts.ContentPartRef
}

// AltChunk is a typed altChunk summary assembled from SAX events.
type AltChunk struct {
	Story              StoryKind
	PartName           string
	RelationshipID     string
	RelationshipType   string
	RelationshipTarget string
	MatchSource        bool
	PayloadContentType string
	PayloadSize        int64
	PayloadData        []byte
	RelatedParts       []RelatedPart
	Model              *wordparts.AltChunk
}

// CollectPath collects a higher-level typed summary from a DOCX file on disk.
func CollectPath(path string, opts Options) (*Summary, error) {
	var collector summaryCollector
	if err := WalkPath(path, &collector, opts); err != nil {
		return nil, err
	}
	return collector.summary(), nil
}

// CollectBytes collects a higher-level typed summary from an in-memory DOCX byte slice.
func CollectBytes(data []byte, opts Options) (*Summary, error) {
	var collector summaryCollector
	if err := WalkBytes(data, &collector, opts); err != nil {
		return nil, err
	}
	return collector.summary(), nil
}

// CollectStream collects a higher-level typed summary from a non-seekable DOCX stream.
func CollectStream(r io.Reader, opts Options) (*Summary, error) {
	var collector summaryCollector
	if err := WalkStream(r, &collector, opts); err != nil {
		return nil, err
	}
	return collector.summary(), nil
}

// CollectReaderAt collects a higher-level typed summary from a DOCX reader/size pair.
func CollectReaderAt(r io.ReaderAt, size int64, opts Options) (*Summary, error) {
	var collector summaryCollector
	if err := WalkReaderAt(r, size, &collector, opts); err != nil {
		return nil, err
	}
	return collector.summary(), nil
}

type summaryCollector struct {
	out             Summary
	paragraph       *Paragraph
	paragraphText   strings.Builder
	hyperlinks      []*Hyperlink
	contentControls []*ContentControl
	tableDepth      int
}

func (c *summaryCollector) HandleEvent(event Event) error {
	switch event.Type {
	case EventStartTable:
		c.tableDepth++
	case EventEndTable:
		if c.tableDepth > 0 {
			c.tableDepth--
		}
	case EventStartParagraph:
		c.paragraph = &Paragraph{
			Story:    event.Story,
			PartName: event.PartName,
			StyleID:  event.StyleID,
			InTable:  c.tableDepth > 0,
		}
		c.paragraphText.Reset()
	case EventText, EventSymbol, EventSoftHyphen, EventNoBreakHyphen:
		c.appendVisible(event.Text)
	case EventMath:
		c.appendVisible(event.MathText)
		c.out.Math = append(c.out.Math, Math{
			Story:    event.Story,
			PartName: event.PartName,
			Text:     event.MathText,
			Display:  event.Display,
			Model:    event.MathModel,
			RawXML:   event.RawXML,
		})
	case EventTab:
		c.appendVisible("\t")
	case EventBreak:
		c.appendVisible("\n")
	case EventStartHyperlink:
		c.hyperlinks = append(c.hyperlinks, &Hyperlink{
			Story:              event.Story,
			PartName:           event.PartName,
			RelationshipID:     event.RelationshipID,
			RelationshipType:   event.RelationshipType,
			RelationshipTarget: event.RelationshipTarget,
			Anchor:             event.HyperlinkAnchor,
			Target:             event.HyperlinkTarget,
		})
	case EventEndHyperlink:
		c.finishHyperlink()
	case EventStartContentControl:
		c.contentControls = append(c.contentControls, &ContentControl{
			Story:      event.Story,
			PartName:   event.PartName,
			ID:         event.ControlID,
			Alias:      event.ControlAlias,
			Tag:        event.ControlTag,
			Type:       event.ControlType,
			Properties: event.ControlProperties,
		})
	case EventEndContentControl:
		c.finishContentControl()
	case EventDrawing:
		c.out.Drawings = append(c.out.Drawings, Drawing{
			Story:              event.Story,
			PartName:           event.PartName,
			Kind:               event.DrawingKind,
			GraphicKind:        event.DrawingGraphicKind,
			RelationshipID:     event.RelationshipID,
			RelationshipType:   event.RelationshipType,
			RelationshipTarget: event.RelationshipTarget,
			DrawingDocID:       event.DrawingDocID,
			Name:               event.DrawingName,
			Description:        event.DrawingDescription,
			Title:              event.DrawingTitle,
			Hidden:             event.DrawingHidden,
			WidthEMU:           event.WidthEMU,
			HeightEMU:          event.HeightEMU,
			HasTextBox:         event.DrawingHasTextBox,
			RelatedParts:       append([]RelatedPart(nil), event.RelatedParts...),
			Model:              event.DrawingModel,
		})
	case EventEmbeddedObject:
		c.out.EmbeddedObjects = append(c.out.EmbeddedObjects, EmbeddedObject{
			Story:              event.Story,
			PartName:           event.PartName,
			ProgID:             event.ProgID,
			RelationshipID:     event.RelationshipID,
			RelationshipType:   event.RelationshipType,
			RelationshipTarget: event.RelationshipTarget,
			RelatedParts:       append([]RelatedPart(nil), event.RelatedParts...),
			Model:              event.EmbeddedObjectModel,
		})
	case EventContentPart:
		c.out.ContentParts = append(c.out.ContentParts, ContentPart{
			Story:              event.Story,
			PartName:           event.PartName,
			RelationshipID:     event.RelationshipID,
			RelationshipType:   event.RelationshipType,
			RelationshipTarget: event.RelationshipTarget,
			PayloadContentType: event.PayloadContentType,
			PayloadSize:        event.PayloadSize,
			PayloadData:        append([]byte(nil), event.PayloadData...),
			RelatedParts:       append([]RelatedPart(nil), event.RelatedParts...),
			Model:              event.ContentPartModel,
		})
	case EventAltChunk:
		c.out.AltChunks = append(c.out.AltChunks, AltChunk{
			Story:              event.Story,
			PartName:           event.PartName,
			RelationshipID:     event.RelationshipID,
			RelationshipType:   event.RelationshipType,
			RelationshipTarget: event.RelationshipTarget,
			MatchSource:        event.MatchSource,
			PayloadContentType: event.PayloadContentType,
			PayloadSize:        event.PayloadSize,
			PayloadData:        append([]byte(nil), event.PayloadData...),
			RelatedParts:       append([]RelatedPart(nil), event.RelatedParts...),
			Model:              event.AltChunkModel,
		})
	case EventEndParagraph:
		c.finishParagraph()
	}
	return nil
}

func (c *summaryCollector) summary() *Summary {
	return &c.out
}

func (c *summaryCollector) appendVisible(text string) {
	if text == "" {
		return
	}
	if c.paragraph != nil {
		c.paragraphText.WriteString(text)
	}
	for _, hyperlink := range c.hyperlinks {
		hyperlink.Text += text
	}
	for _, control := range c.contentControls {
		control.Text += text
	}
}

func (c *summaryCollector) finishParagraph() {
	if c.paragraph == nil {
		return
	}
	c.paragraph.Text = c.paragraphText.String()
	c.out.Paragraphs = append(c.out.Paragraphs, *c.paragraph)
	c.paragraph = nil
	c.paragraphText.Reset()
}

func (c *summaryCollector) finishHyperlink() {
	if len(c.hyperlinks) == 0 {
		return
	}
	last := c.hyperlinks[len(c.hyperlinks)-1]
	c.hyperlinks = c.hyperlinks[:len(c.hyperlinks)-1]
	c.out.Hyperlinks = append(c.out.Hyperlinks, *last)
}

func (c *summaryCollector) finishContentControl() {
	if len(c.contentControls) == 0 {
		return
	}
	last := c.contentControls[len(c.contentControls)-1]
	c.contentControls = c.contentControls[:len(c.contentControls)-1]
	c.out.ContentControls = append(c.out.ContentControls, *last)
}