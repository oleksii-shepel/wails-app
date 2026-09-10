package word97

import (
	"strings"

	"office/pkg/ooxml/cfb"
	wordir "office/pkg/word/ir"
	docofficeart "office/pkg/word/parsers/doc/officeart"
)

// RecoverOLECompoundPayloads attaches otherwise-unclaimed embedded compound
// payload streams to canonical Word97 OLE placeholders in document order.
func RecoverOLECompoundPayloads(f *cfb.File, doc *wordir.Document) {
	if f == nil || doc == nil {
		return
	}
	payloads := make([][]byte, 0)
	for _, name := range f.StreamNames() {
		lower := strings.ToLower(name)
		if !strings.Contains(lower, "ole10native") &&
			!strings.Contains(lower, "objectpool") &&
			!strings.Contains(lower, "package") {
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
	walkDocumentRuns(doc, func(run *wordir.Run) {
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

// RecoverOLEPreviews discovers canonical OfficeArt metafile previews from the
// Word97 Data stream and associates them with the most likely visible OLE
// placeholders. Multiple structural 0x01 anchors can represent one visible
// object, so paragraph semantics are used instead of blind stream-order pairing.
func RecoverOLEPreviews(f *cfb.File, doc *wordir.Document) {
	if f == nil || doc == nil {
		return
	}
	data, ok := f.Stream("Data")
	if !ok || len(data) == 0 {
		data, ok = f.Stream("DataStream")
	}
	if !ok || len(data) == 0 {
		return
	}
	var previews []*wordir.Image
	for _, blip := range docofficeart.ExtractBLIPs(data) {
		if preview := blip.MetafileImage(); preview != nil {
			previews = append(previews, preview)
		}
	}
	AttachOLEPreviews(doc, previews)
}

type olePreviewCandidate struct {
	run   *wordir.Run
	score int
	order int
}

// AttachOLEPreviews is the semantic Word97 preview matcher. Centered,
// otherwise-empty object paragraphs are preferred; OLE metadata is a secondary
// signal and document order is the stable tie-breaker.
func AttachOLEPreviews(doc *wordir.Document, previews []*wordir.Image) {
	if doc == nil || len(previews) == 0 {
		return
	}
	candidates := collectOLEPreviewCandidates(doc)
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

func collectOLEPreviewCandidates(doc *wordir.Document) []olePreviewCandidate {
	var out []olePreviewCandidate
	order := 0
	visit := func(p *wordir.Paragraph) {
		if p == nil {
			return
		}
		oleCount, nonOLEText := 0, false
		for i := range p.Runs {
			if p.Runs[i].Kind == "olePlaceholder" {
				oleCount++
			} else if strings.TrimSpace(p.Runs[i].Text) != "" {
				nonOLEText = true
			}
		}
		for i := range p.Runs {
			run := &p.Runs[i]
			if run.Kind != "olePlaceholder" || run.OLEPreview != nil {
				continue
			}
			score := 0
			if strings.EqualFold(strings.TrimSpace(p.Format.Alignment), "center") {
				score += 1000
			}
			if oleCount == 1 && !nonOLEText {
				score += 200
			}
			if run.OLEProgID != "" || run.OLEObjectID != "" || run.OLEType != "" || run.OLELinkURL != "" {
				score += 100
			}
			out = append(out, olePreviewCandidate{run: run, score: score, order: order})
			order++
		}
	}
	if len(doc.Blocks) > 0 {
		for i := range doc.Blocks {
			walkOLEBlock(&doc.Blocks[i], visit)
		}
		return out
	}
	for i := range doc.Paragraphs {
		visit(&doc.Paragraphs[i])
	}
	for i := range doc.Tables {
		walkOLETable(&doc.Tables[i], visit)
	}
	return out
}

func walkDocumentRuns(doc *wordir.Document, visit func(*wordir.Run)) {
	if doc == nil || visit == nil {
		return
	}
	visitParagraph := func(p *wordir.Paragraph) {
		if p == nil {
			return
		}
		for i := range p.Runs {
			visit(&p.Runs[i])
		}
	}
	if len(doc.Blocks) > 0 {
		for i := range doc.Blocks {
			walkOLEBlock(&doc.Blocks[i], visitParagraph)
		}
		return
	}
	for i := range doc.Paragraphs {
		visitParagraph(&doc.Paragraphs[i])
	}
	for i := range doc.Tables {
		walkOLETable(&doc.Tables[i], visitParagraph)
	}
}

func walkOLEBlock(block *wordir.Block, visit func(*wordir.Paragraph)) {
	if block == nil {
		return
	}
	if block.Paragraph != nil {
		visit(block.Paragraph)
	}
	if block.Table != nil {
		walkOLETable(block.Table, visit)
	}
}

func walkOLETable(table *wordir.Table, visit func(*wordir.Paragraph)) {
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
				walkOLEBlock(&cell.Blocks[bi], visit)
			}
			for ti := range cell.Tables {
				walkOLETable(&cell.Tables[ti], visit)
			}
		}
	}
}
