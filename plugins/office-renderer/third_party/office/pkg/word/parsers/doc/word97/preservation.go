package word97

import (
	"encoding/json"

	"office/pkg/ooxml/cfb"
	wordir "office/pkg/word/ir"
)

const (
	PreservationStreamName = "OfficeMeta"
	PreservationVersion    = 1
)

// PreservationMeta is the reader-side model for the private OfficeMeta
// companion stream written by this library. Native DOC structures remain
// authoritative; this data only restores semantic payloads that do not yet
// have a dependable native Word 97 encoding.
type PreservationMeta struct {
	Version int `json:"version"`

	OLE []PreservedOLE `json:"ole,omitempty"`

	FootnoteSeparator          *wordir.Story        `json:"footnoteSeparator,omitempty"`
	FootnoteContinuationSep    *wordir.Story        `json:"footnoteContinuationSep,omitempty"`
	FootnoteContinuationNotice *wordir.Story        `json:"footnoteContinuationNotice,omitempty"`
	EndnoteSeparator           *wordir.Story        `json:"endnoteSeparator,omitempty"`
	EndnoteContinuationSep     *wordir.Story        `json:"endnoteContinuationSep,omitempty"`
	EndnoteContinuationNotice  *wordir.Story        `json:"endnoteContinuationNotice,omitempty"`
	Headers                    map[string]wordir.Story `json:"headers,omitempty"`
	Footers                    map[string]wordir.Story `json:"footers,omitempty"`
	SectionHeaders             [][]wordir.Reference `json:"sectionHeaders,omitempty"`
	SectionFooters             [][]wordir.Reference `json:"sectionFooters,omitempty"`
	Footnotes                  map[int]wordir.Story  `json:"footnotes,omitempty"`
	Endnotes                   map[int]wordir.Story  `json:"endnotes,omitempty"`
	Comments                   map[int]wordir.Story  `json:"comments,omitempty"`
}

type PreservedOLE struct {
	ProgID     string        `json:"progId,omitempty"`
	Data       []byte        `json:"data,omitempty"`
	MIME       string        `json:"mime,omitempty"`
	LinkURL    string        `json:"linkUrl,omitempty"`
	Preview    *wordir.Image `json:"preview,omitempty"`
	Type       string        `json:"type,omitempty"`
	DrawAspect string        `json:"drawAspect,omitempty"`
	ObjectID   string        `json:"objectId,omitempty"`
	UpdateMode string        `json:"updateMode,omitempty"`
	RelID      string        `json:"relId,omitempty"`
	Text       string        `json:"text,omitempty"`
}

// ParsePreservation decodes one OfficeMeta payload. Malformed or future
// versions are ignored so private metadata can never make a readable DOC fail.
func ParsePreservation(data []byte) *PreservationMeta {
	if len(data) == 0 {
		return nil
	}
	var meta PreservationMeta
	if json.Unmarshal(data, &meta) != nil || meta.Version != PreservationVersion {
		return nil
	}
	return &meta
}

func LoadPreservation(f *cfb.File) *PreservationMeta {
	if f == nil {
		return nil
	}
	data, ok := f.Stream(PreservationStreamName)
	if !ok {
		return nil
	}
	return ParsePreservation(data)
}

// OLERuns converts preservation entries into the canonical placeholder runs
// consumed by the sequential drawing/OLE cursor.
func (m *PreservationMeta) OLERuns() []wordir.Run {
	if m == nil || len(m.OLE) == 0 {
		return nil
	}
	out := make([]wordir.Run, 0, len(m.OLE))
	for _, o := range m.OLE {
		text := o.Text
		if text == "" {
			text = o.ProgID
		}
		if text == "" {
			text = "Embedded Object"
		}
		out = append(out, wordir.Run{
			Kind:          "olePlaceholder",
			Text:          text,
			OLERelID:      o.RelID,
			OLEProgID:     o.ProgID,
			OLEData:       append([]byte(nil), o.Data...),
			OLEMIME:       o.MIME,
			OLELinkURL:    o.LinkURL,
			OLEPreview:    clonePreservedImage(o.Preview),
			OLEType:       o.Type,
			OLEDrawAspect: o.DrawAspect,
			OLEObjectID:   o.ObjectID,
			OLEUpdateMode: o.UpdateMode,
		})
	}
	return out
}

// ApplyToDocument overlays preserved secondary stories and section references
// after sequential materialization. OLE payloads are applied earlier by the
// sequential cursor so object anchors retain their document positions.
func (m *PreservationMeta) ApplyToDocument(doc *wordir.Document) {
	if m == nil || doc == nil {
		return
	}
	if m.FootnoteSeparator != nil {
		doc.FootnoteSeparator = *m.FootnoteSeparator
	}
	if m.FootnoteContinuationSep != nil {
		doc.FootnoteContinuationSep = *m.FootnoteContinuationSep
	}
	if m.FootnoteContinuationNotice != nil {
		doc.FootnoteContinuationNotice = *m.FootnoteContinuationNotice
	}
	if m.EndnoteSeparator != nil {
		doc.EndnoteSeparator = *m.EndnoteSeparator
	}
	if m.EndnoteContinuationSep != nil {
		doc.EndnoteContinuationSep = *m.EndnoteContinuationSep
	}
	if m.EndnoteContinuationNotice != nil {
		doc.EndnoteContinuationNotice = *m.EndnoteContinuationNotice
	}
	mergePreservedStringStories(&doc.Headers, m.Headers)
	mergePreservedStringStories(&doc.Footers, m.Footers)
	mergePreservedIntStories(&doc.Footnotes, m.Footnotes)
	mergePreservedIntStories(&doc.Endnotes, m.Endnotes)
	mergePreservedIntStories(&doc.Comments, m.Comments)

	n := len(m.SectionHeaders)
	if len(m.SectionFooters) > n {
		n = len(m.SectionFooters)
	}
	if n > len(doc.Sections) {
		doc.Sections = append(doc.Sections, make([]wordir.Section, n-len(doc.Sections))...)
	}
	for i := 0; i < n; i++ {
		if i < len(m.SectionHeaders) && m.SectionHeaders[i] != nil {
			doc.Sections[i].Headers = append([]wordir.Reference(nil), m.SectionHeaders[i]...)
		}
		if i < len(m.SectionFooters) && m.SectionFooters[i] != nil {
			doc.Sections[i].Footers = append([]wordir.Reference(nil), m.SectionFooters[i]...)
		}
	}
}

func mergePreservedStringStories(dst *map[string]wordir.Story, src map[string]wordir.Story) {
	if len(src) == 0 {
		return
	}
	if *dst == nil {
		*dst = make(map[string]wordir.Story)
	}
	for key, story := range src {
		(*dst)[key] = story
	}
}

func mergePreservedIntStories(dst *map[int]wordir.Story, src map[int]wordir.Story) {
	if len(src) == 0 {
		return
	}
	if *dst == nil {
		*dst = make(map[int]wordir.Story)
	}
	for key, story := range src {
		(*dst)[key] = story
	}
}

func clonePreservedImage(src *wordir.Image) *wordir.Image {
	if src == nil {
		return nil
	}
	out := *src
	out.Data = append([]byte(nil), src.Data...)
	return &out
}
