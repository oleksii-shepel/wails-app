package pre97

import (
	"fmt"
	"sort"

	wordir "office/pkg/word/ir"
)

func cloneRunText(run Run, text string, startCP int) Run {
	copyRun := run
	copyRun.Text = text
	copyRun.StartCP = startCP
	return copyRun
}

type referenceCursor struct {
	inner Cursor
	footnotes, endnotes, comments map[int]int
	queue []Run
}

func NewReferenceCursor(inner Cursor, footnotes, endnotes, comments map[int]int) Cursor {
	if inner == nil || (len(footnotes) == 0 && len(endnotes) == 0 && len(comments) == 0) { return inner }
	return &referenceCursor{inner: inner, footnotes: footnotes, endnotes: endnotes, comments: comments}
}

func (c *referenceCursor) Next() (Run, bool, error) {
	for {
		if len(c.queue) > 0 { r := c.queue[0]; c.queue = c.queue[1:]; return r, true, nil }
		run, ok, err := c.inner.Next(); if err != nil || !ok { return run, ok, err }
		c.expand(run)
	}
}

func (c *referenceCursor) expand(run Run) {
	if run.Kind != "" || run.Text == "" { c.queue = append(c.queue, run); return }
	runes := []rune(run.Text)
	segment, segmentCP := 0, run.StartCP
	for i := 0; i < len(runes); i++ {
		cp := run.StartCP + i
		fid, fok := c.footnotes[cp]
		eid, eok := c.endnotes[cp]
		cid, cok := c.comments[cp]
		if !fok && !eok && !cok { continue }
		if i > segment { c.queue = append(c.queue, cloneRunText(run, string(runes[segment:i]), segmentCP)) }
		if fok { c.queue = append(c.queue, Run{Kind:"footnoteRef", NoteID:fid, StartCP:cp, Format:run.Format, ParagraphFormat:run.ParagraphFormat}) }
		if eok { c.queue = append(c.queue, Run{Kind:"endnoteRef", NoteID:eid, StartCP:cp, Format:run.Format, ParagraphFormat:run.ParagraphFormat}) }
		if cok { c.queue = append(c.queue, Run{Kind:"commentRef", CommentID:cid, StartCP:cp, Format:run.Format, ParagraphFormat:run.ParagraphFormat}) }
		segment, segmentCP = i, cp
	}
	if segment < len(runes) { c.queue = append(c.queue, cloneRunText(run, string(runes[segment:]), segmentCP)) }
}

type bookmarkCursor struct {
	inner Cursor
	starts, ends map[int][]string
	queue []Run
	emitted map[string]bool
}

func NewBookmarkCursor(inner Cursor, starts, ends map[int][]string) Cursor {
	if inner == nil || (len(starts) == 0 && len(ends) == 0) { return inner }
	return &bookmarkCursor{inner:inner, starts:starts, ends:ends, emitted:make(map[string]bool)}
}

func (c *bookmarkCursor) Next() (Run, bool, error) {
	for {
		if len(c.queue) > 0 { r := c.queue[0]; c.queue = c.queue[1:]; return r, true, nil }
		run, ok, err := c.inner.Next(); if err != nil || !ok { return run, ok, err }
		c.expand(run)
	}
}

func (c *bookmarkCursor) emit(kind, name string, cp int, run Run) {
	key := fmt.Sprintf("%s|%d|%s", kind, cp, name)
	if c.emitted[key] { return }
	c.emitted[key] = true
	c.queue = append(c.queue, Run{Kind:kind, Anchor:name, StartCP:cp, Format:run.Format, ParagraphFormat:run.ParagraphFormat})
}

func (c *bookmarkCursor) expand(run Run) {
	if run.Kind != "" || run.Text == "" { c.queue = append(c.queue, run); return }
	runes := []rune(run.Text)
	segment, segmentCP := 0, run.StartCP
	for i := 0; i < len(runes); i++ {
		cp := run.StartCP + i
		starts, ends := c.starts[cp], c.ends[cp]
		if len(starts) == 0 && len(ends) == 0 { continue }
		if i > segment { c.queue = append(c.queue, cloneRunText(run, string(runes[segment:i]), segmentCP)) }
		for _, name := range ends { c.emit("bookmarkEnd", name, cp, run) }
		for _, name := range starts { c.emit("bookmarkStart", name, cp, run) }
		segment, segmentCP = i, cp
	}
	endCP := run.StartCP + len(runes)
	if segment < len(runes) { c.queue = append(c.queue, cloneRunText(run, string(runes[segment:]), segmentCP)) }
	for _, name := range c.ends[endCP] { c.emit("bookmarkEnd", name, endCP, run) }
	for _, name := range c.starts[endCP] { c.emit("bookmarkStart", name, endCP, run) }
}

type sectionBoundaryCursor struct {
	inner Cursor
	boundaries []int
	queue []Run
}

func NewSectionBoundaryCursor(inner Cursor, sections []wordir.Section) Cursor {
	if inner == nil || len(sections) == 0 { return inner }
	boundaries := make([]int, 0, len(sections))
	for _, sec := range sections { if sec.Source != nil && sec.Source.EndCP > 0 { boundaries = append(boundaries, sec.Source.EndCP) } }
	if len(boundaries) == 0 { return inner }
	sort.Ints(boundaries)
	uniq := boundaries[:0]
	for _, cp := range boundaries { if len(uniq) == 0 || uniq[len(uniq)-1] != cp { uniq = append(uniq, cp) } }
	return &sectionBoundaryCursor{inner:inner, boundaries:uniq}
}

func (c *sectionBoundaryCursor) Next() (Run, bool, error) {
	for {
		if len(c.queue) > 0 { r := c.queue[0]; c.queue = c.queue[1:]; return r, true, nil }
		run, ok, err := c.inner.Next(); if err != nil || !ok { return run, ok, err }
		c.expand(run)
	}
}

func (c *sectionBoundaryCursor) expand(run Run) {
	if run.Kind != "" || run.Text == "" { c.queue = append(c.queue, run); return }
	runes := []rune(run.Text)
	endCP, start := run.StartCP + len(runes), 0
	for _, boundary := range c.boundaries {
		if boundary <= run.StartCP || boundary >= endCP { continue }
		off := boundary - run.StartCP
		if off > start { c.queue = append(c.queue, cloneRunText(run, string(runes[start:off]), run.StartCP+start)) }
		start = off
	}
	if start < len(runes) { c.queue = append(c.queue, cloneRunText(run, string(runes[start:]), run.StartCP+start)) }
}
