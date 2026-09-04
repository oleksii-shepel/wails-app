package legacy

import (
	"encoding/json"
	"strings"

	"office/pkg/ooxml/cfb"
	wordir "office/pkg/word/ir"
)

const legacyPreservationMetaStream = "OfficeMeta"
const legacyPreservationMetaVersion = 1

// legacyPreservationMeta is a lossless companion stream for semantic content
// that does not have a dependable native Word 97 encoding in the current
// writer. Native DOC structures remain authoritative for text/layout; this
// stream only restores embedded-object payloads and rich secondary stories on
// a library write->read round trip.
type legacyPreservationMeta struct {
	Version int `json:"version"`

	OLE []legacyOLEPreservation `json:"ole,omitempty"`

	FootnoteSeparator          *wordir.Story           `json:"footnoteSeparator,omitempty"`
	FootnoteContinuationSep    *wordir.Story           `json:"footnoteContinuationSep,omitempty"`
	FootnoteContinuationNotice *wordir.Story           `json:"footnoteContinuationNotice,omitempty"`
	EndnoteSeparator           *wordir.Story           `json:"endnoteSeparator,omitempty"`
	EndnoteContinuationSep     *wordir.Story           `json:"endnoteContinuationSep,omitempty"`
	EndnoteContinuationNotice  *wordir.Story           `json:"endnoteContinuationNotice,omitempty"`
	Headers                    map[string]wordir.Story `json:"headers,omitempty"`
	Footers                    map[string]wordir.Story `json:"footers,omitempty"`
	SectionHeaders             [][]wordir.Reference    `json:"sectionHeaders,omitempty"`
	SectionFooters             [][]wordir.Reference    `json:"sectionFooters,omitempty"`
	Footnotes                  map[int]wordir.Story    `json:"footnotes,omitempty"`
	Endnotes                   map[int]wordir.Story    `json:"endnotes,omitempty"`
	Comments                   map[int]wordir.Story    `json:"comments,omitempty"`
}

type legacyOLEPreservation struct {
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

func buildLegacyPreservationMeta(doc *wordir.Document) []byte {
	if doc == nil {
		return nil
	}
	meta := legacyPreservationMeta{Version: legacyPreservationMetaVersion}
	walkLegacyDocumentRuns(doc, func(run *wordir.Run) {
		if run == nil || run.Kind != "olePlaceholder" {
			return
		}
		meta.OLE = append(meta.OLE, legacyOLEPreservation{
			ProgID: run.OLEProgID, Data: append([]byte(nil), run.OLEData...), MIME: run.OLEMIME,
			LinkURL: run.OLELinkURL, Preview: cloneLegacyMetaImage(run.OLEPreview), Type: run.OLEType,
			DrawAspect: run.OLEDrawAspect, ObjectID: run.OLEObjectID, UpdateMode: run.OLEUpdateMode,
			RelID: run.OLERelID, Text: run.Text,
		})
	})

	meta.FootnoteSeparator = cloneLegacyMetaStoryIfRich(doc.FootnoteSeparator)
	meta.FootnoteContinuationSep = cloneLegacyMetaStoryIfRich(doc.FootnoteContinuationSep)
	meta.FootnoteContinuationNotice = cloneLegacyMetaStoryIfRich(doc.FootnoteContinuationNotice)
	meta.EndnoteSeparator = cloneLegacyMetaStoryIfRich(doc.EndnoteSeparator)
	meta.EndnoteContinuationSep = cloneLegacyMetaStoryIfRich(doc.EndnoteContinuationSep)
	meta.EndnoteContinuationNotice = cloneLegacyMetaStoryIfRich(doc.EndnoteContinuationNotice)
	meta.Headers = cloneLegacyMetaStringStories(doc.Headers)
	meta.Footers = cloneLegacyMetaStringStories(doc.Footers)
	if len(meta.Headers) > 0 || len(meta.Footers) > 0 {
		meta.SectionHeaders = make([][]wordir.Reference, len(doc.Sections))
		meta.SectionFooters = make([][]wordir.Reference, len(doc.Sections))
		for i := range doc.Sections {
			meta.SectionHeaders[i] = append([]wordir.Reference(nil), doc.Sections[i].Headers...)
			meta.SectionFooters[i] = append([]wordir.Reference(nil), doc.Sections[i].Footers...)
		}
	}
	meta.Footnotes = cloneLegacyMetaIntStories(doc.Footnotes)
	meta.Endnotes = cloneLegacyMetaIntStories(doc.Endnotes)
	meta.Comments = cloneLegacyMetaIntStories(doc.Comments)

	if len(meta.OLE) == 0 && meta.FootnoteSeparator == nil && meta.FootnoteContinuationSep == nil &&
		meta.FootnoteContinuationNotice == nil && meta.EndnoteSeparator == nil && meta.EndnoteContinuationSep == nil &&
		meta.EndnoteContinuationNotice == nil && len(meta.Headers) == 0 && len(meta.Footers) == 0 &&
		len(meta.Footnotes) == 0 && len(meta.Endnotes) == 0 && len(meta.Comments) == 0 {
		return nil
	}
	data, err := json.Marshal(meta)
	if err != nil {
		return nil
	}
	return data
}

func applyLegacyPreservationMeta(f *cfb.File, doc *wordir.Document) {
	if f == nil || doc == nil {
		return
	}
	data, ok := f.Stream(legacyPreservationMetaStream)
	if !ok || len(data) == 0 {
		return
	}
	var meta legacyPreservationMeta
	if json.Unmarshal(data, &meta) != nil || meta.Version != legacyPreservationMetaVersion {
		// Private preservation metadata must never make an otherwise-readable DOC
		// fail. Ignore malformed/future versions and keep native binary recovery.
		return
	}

	if meta.FootnoteSeparator != nil {
		doc.FootnoteSeparator = *meta.FootnoteSeparator
	}
	if meta.FootnoteContinuationSep != nil {
		doc.FootnoteContinuationSep = *meta.FootnoteContinuationSep
	}
	if meta.FootnoteContinuationNotice != nil {
		doc.FootnoteContinuationNotice = *meta.FootnoteContinuationNotice
	}
	if meta.EndnoteSeparator != nil {
		doc.EndnoteSeparator = *meta.EndnoteSeparator
	}
	if meta.EndnoteContinuationSep != nil {
		doc.EndnoteContinuationSep = *meta.EndnoteContinuationSep
	}
	if meta.EndnoteContinuationNotice != nil {
		doc.EndnoteContinuationNotice = *meta.EndnoteContinuationNotice
	}
	mergeLegacyMetaStringStories(&doc.Headers, meta.Headers)
	mergeLegacyMetaStringStories(&doc.Footers, meta.Footers)
	mergeLegacyMetaIntStories(&doc.Footnotes, meta.Footnotes)
	mergeLegacyMetaIntStories(&doc.Endnotes, meta.Endnotes)
	mergeLegacyMetaIntStories(&doc.Comments, meta.Comments)
	if len(meta.SectionHeaders) > 0 || len(meta.SectionFooters) > 0 {
		n := len(meta.SectionHeaders)
		if len(meta.SectionFooters) > n {
			n = len(meta.SectionFooters)
		}
		if n > len(doc.Sections) {
			doc.Sections = append(doc.Sections, make([]wordir.Section, n-len(doc.Sections))...)
		}
		for i := 0; i < n; i++ {
			if i < len(meta.SectionHeaders) && meta.SectionHeaders[i] != nil {
				doc.Sections[i].Headers = append([]wordir.Reference(nil), meta.SectionHeaders[i]...)
			}
			if i < len(meta.SectionFooters) && meta.SectionFooters[i] != nil {
				doc.Sections[i].Footers = append([]wordir.Reference(nil), meta.SectionFooters[i]...)
			}
		}
	}

	oleIndex := 0
	walkLegacyDocumentRuns(doc, func(run *wordir.Run) {
		if run == nil || run.Kind != "olePlaceholder" || oleIndex >= len(meta.OLE) {
			return
		}
		o := meta.OLE[oleIndex]
		oleIndex++
		run.OLEProgID = o.ProgID
		run.OLEData = append([]byte(nil), o.Data...)
		run.OLEMIME = o.MIME
		run.OLELinkURL = o.LinkURL
		run.OLEPreview = cloneLegacyMetaImage(o.Preview)
		run.OLEType = o.Type
		run.OLEDrawAspect = o.DrawAspect
		run.OLEObjectID = o.ObjectID
		run.OLEUpdateMode = o.UpdateMode
		run.OLERelID = o.RelID
		if o.Text != "" {
			run.Text = o.Text
		} else if o.ProgID != "" {
			run.Text = o.ProgID
		}
	})
}

func mergeLegacyMetaStringStories(dst *map[string]wordir.Story, src map[string]wordir.Story) {
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

func mergeLegacyMetaIntStories(dst *map[int]wordir.Story, src map[int]wordir.Story) {
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

func applyLegacyOLECompoundRecovery(f *cfb.File, doc *wordir.Document) {
	if f == nil || doc == nil {
		return
	}
	payloads := make([][]byte, 0)
	for _, name := range f.StreamNames() {
		lower := strings.ToLower(name)
		if !strings.Contains(lower, "ole10native") && !strings.Contains(lower, "objectpool") && !strings.Contains(lower, "package") {
			continue
		}
		data, ok := f.Stream(name)
		if !ok || len(data) == 0 {
			continue
		}
		payloads = append(payloads, append([]byte(nil), data...))
	}
	if len(payloads) == 0 {
		return
	}
	index := 0
	walkLegacyDocumentRuns(doc, func(run *wordir.Run) {
		if run == nil || run.Kind != "olePlaceholder" || len(run.OLEData) > 0 || index >= len(payloads) {
			return
		}
		run.OLEData = payloads[index]
		if run.OLEMIME == "" {
			run.OLEMIME = "application/octet-stream"
		}
		if run.OLEType == "" {
			run.OLEType = "Embed"
		}
		index++
	})
}

func applyLegacyOLEMetafilePreviews(f *cfb.File, doc *wordir.Document) {
	if f == nil || doc == nil {
		return
	}
	dataStream, ok := f.Stream("Data")
	if !ok || len(dataStream) == 0 {
		dataStream, ok = f.Stream("DataStream")
	}
	if !ok || len(dataStream) == 0 {
		return
	}
	attachLegacyOLEMetafilePreviews(doc, extractLegacyDrawingMetafileImages(dataStream))
}

type legacyOLEPreviewCandidate struct {
	run   *wordir.Run
	score int
	order int
}

// attachLegacyOLEMetafilePreviews associates Data-stream metafile previews with
// their visible OLE anchors. Binary Word files may contain several 0x01 object
// anchors for one visible object; assigning previews to anchors merely in stream
// order can therefore put an Excel preview into an unrelated paragraph. Word's
// visible embedded-object anchor is typically an otherwise-empty object
// paragraph, and centered objects retain that paragraph alignment. Prefer those
// semantic anchors while retaining document order as the tie-breaker.
func attachLegacyOLEMetafilePreviews(doc *wordir.Document, previews []*wordir.Image) {
	if doc == nil || len(previews) == 0 {
		return
	}
	candidates := collectLegacyOLEPreviewCandidates(doc)
	for _, source := range previews {
		if source == nil || len(source.Data) == 0 || len(candidates) == 0 {
			continue
		}
		best := 0
		for i := 1; i < len(candidates); i++ {
			if candidates[i].score > candidates[best].score ||
				(candidates[i].score == candidates[best].score && candidates[i].order < candidates[best].order) {
				best = i
			}
		}
		preview := *source
		preview.Data = append([]byte(nil), source.Data...)
		candidates[best].run.OLEPreview = &preview
		candidates = append(candidates[:best], candidates[best+1:]...)
	}
}

func collectLegacyOLEPreviewCandidates(doc *wordir.Document) []legacyOLEPreviewCandidate {
	var out []legacyOLEPreviewCandidate
	order := 0
	visitParagraph := func(paragraph *wordir.Paragraph) {
		if paragraph == nil {
			return
		}
		oleCount := 0
		nonOLEText := false
		for i := range paragraph.Runs {
			run := &paragraph.Runs[i]
			if run.Kind == "olePlaceholder" {
				oleCount++
				continue
			}
			if strings.TrimSpace(run.Text) != "" {
				nonOLEText = true
			}
		}
		for i := range paragraph.Runs {
			run := &paragraph.Runs[i]
			if run.Kind != "olePlaceholder" || run.OLEPreview != nil {
				continue
			}
			score := 0
			if strings.EqualFold(strings.TrimSpace(paragraph.Format.Alignment), "center") {
				score += 1000
			}
			if oleCount == 1 && !nonOLEText {
				score += 200
			}
			if run.OLEProgID != "" || run.OLEObjectID != "" || run.OLEType != "" || run.OLELinkURL != "" {
				score += 100
			}
			out = append(out, legacyOLEPreviewCandidate{run: run, score: score, order: order})
			order++
		}
	}
	if len(doc.Blocks) > 0 {
		for i := range doc.Blocks {
			collectLegacyOLECandidateBlock(&doc.Blocks[i], visitParagraph)
		}
		return out
	}
	for i := range doc.Paragraphs {
		visitParagraph(&doc.Paragraphs[i])
	}
	for i := range doc.Tables {
		collectLegacyOLECandidateTable(&doc.Tables[i], visitParagraph)
	}
	return out
}

func collectLegacyOLECandidateBlock(block *wordir.Block, visit func(*wordir.Paragraph)) {
	if block == nil {
		return
	}
	if block.Paragraph != nil {
		visit(block.Paragraph)
	}
	if block.Table != nil {
		collectLegacyOLECandidateTable(block.Table, visit)
	}
}

func collectLegacyOLECandidateTable(table *wordir.Table, visit func(*wordir.Paragraph)) {
	if table == nil {
		return
	}
	for ri := range table.Rows {
		for ci := range table.Rows[ri].Cells {
			cell := &table.Rows[ri].Cells[ci]
			for pi := range cell.Paragraphs {
				visit(&cell.Paragraphs[pi])
			}
			for bi := range cell.Blocks {
				collectLegacyOLECandidateBlock(&cell.Blocks[bi], visit)
			}
			for ti := range cell.Tables {
				collectLegacyOLECandidateTable(&cell.Tables[ti], visit)
			}
		}
	}
}

func cloneLegacyMetaStoryIfRich(story wordir.Story) *wordir.Story {
	if !legacyStoryNeedsPreservation(story) {
		return nil
	}
	cloned := cloneLegacyMetaStory(story)
	return &cloned
}

func cloneLegacyMetaStringStories(src map[string]wordir.Story) map[string]wordir.Story {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]wordir.Story)
	for key, story := range src {
		if legacyStoryNeedsPreservation(story) {
			out[key] = cloneLegacyMetaStory(story)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func cloneLegacyMetaIntStories(src map[int]wordir.Story) map[int]wordir.Story {
	if len(src) == 0 {
		return nil
	}
	out := make(map[int]wordir.Story)
	for key, story := range src {
		if legacyStoryNeedsPreservation(story) {
			out[key] = cloneLegacyMetaStory(story)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func legacyStoryNeedsPreservation(story wordir.Story) bool {
	if len(story.Fields) > 0 || story.Author != "" || story.Date != "" {
		return true
	}
	found := false
	walkLegacyStoryRuns(&story, func(run *wordir.Run) {
		if run == nil || found {
			return
		}
		if run.Kind == "image" || run.Kind == "shape" || run.Kind == "textbox" || run.Kind == "olePlaceholder" ||
			run.Image != nil || run.Shape != nil || run.TextBox != nil || run.OLEPreview != nil || len(run.OLEData) > 0 || run.FieldInstr != "" {
			found = true
		}
	})
	if found {
		return true
	}
	for _, p := range story.Paragraphs {
		if len(p.Bookmarks) > 0 {
			return true
		}
	}
	for _, b := range story.Blocks {
		if b.Paragraph != nil && len(b.Paragraph.Bookmarks) > 0 {
			return true
		}
	}
	return false
}

func cloneLegacyMetaStory(story wordir.Story) wordir.Story {
	// JSON is intentionally used as a compact deep-copy mechanism for the
	// preservation-only path. If a future IR member cannot be marshalled, retain
	// the original value rather than losing native DOC readability.
	data, err := json.Marshal(story)
	if err != nil {
		return story
	}
	var out wordir.Story
	if json.Unmarshal(data, &out) != nil {
		return story
	}
	return out
}

func cloneLegacyMetaImage(src *wordir.Image) *wordir.Image {
	if src == nil {
		return nil
	}
	data, err := json.Marshal(src)
	if err != nil {
		return src
	}
	var out wordir.Image
	if json.Unmarshal(data, &out) != nil {
		return src
	}
	return &out
}

func walkLegacyDocumentRuns(doc *wordir.Document, visit func(*wordir.Run)) {
	if doc == nil || visit == nil {
		return
	}
	if len(doc.Blocks) > 0 {
		for i := range doc.Blocks {
			walkLegacyBlockRuns(&doc.Blocks[i], visit)
		}
		return
	}
	for i := range doc.Paragraphs {
		walkLegacyParagraphRuns(&doc.Paragraphs[i], visit)
	}
	for i := range doc.Tables {
		walkLegacyTableRuns(&doc.Tables[i], visit)
	}
}

func walkLegacyStoryRuns(story *wordir.Story, visit func(*wordir.Run)) {
	if story == nil || visit == nil {
		return
	}
	for i := range story.Paragraphs {
		walkLegacyParagraphRuns(&story.Paragraphs[i], visit)
	}
	for i := range story.Blocks {
		walkLegacyBlockRuns(&story.Blocks[i], visit)
	}
	for i := range story.Tables {
		walkLegacyTableRuns(&story.Tables[i], visit)
	}
}

func walkLegacyBlockRuns(block *wordir.Block, visit func(*wordir.Run)) {
	if block == nil {
		return
	}
	if block.Paragraph != nil {
		walkLegacyParagraphRuns(block.Paragraph, visit)
	}
	if block.Table != nil {
		walkLegacyTableRuns(block.Table, visit)
	}
}

func walkLegacyParagraphRuns(paragraph *wordir.Paragraph, visit func(*wordir.Run)) {
	if paragraph == nil {
		return
	}
	for i := range paragraph.Runs {
		visit(&paragraph.Runs[i])
	}
}

func walkLegacyTableRuns(table *wordir.Table, visit func(*wordir.Run)) {
	if table == nil {
		return
	}
	for ri := range table.Rows {
		for ci := range table.Rows[ri].Cells {
			cell := &table.Rows[ri].Cells[ci]
			for pi := range cell.Paragraphs {
				walkLegacyParagraphRuns(&cell.Paragraphs[pi], visit)
			}
			for bi := range cell.Blocks {
				walkLegacyBlockRuns(&cell.Blocks[bi], visit)
			}
		}
	}
}

func legacyObjectLabel(run *wordir.Run) string {
	if run == nil {
		return "Embedded Object"
	}
	for _, candidate := range []string{run.Text, run.OLEProgID, run.OLEObjectID} {
		if strings.TrimSpace(candidate) != "" {
			return candidate
		}
	}
	return "Embedded Object"
}
