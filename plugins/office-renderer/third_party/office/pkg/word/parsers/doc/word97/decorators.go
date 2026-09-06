package word97

import (
	"fmt"
	"sort"

	wordir "office/pkg/word/ir"
)

func cloneRunWithText(run Run, text string, startCP int) Run {
	run.Text = text
	run.StartCP = startCP
	return run
}

// referenceCursor injects note/comment reference runs at their physical CPs.
type referenceCursor struct {
	inner                    Cursor
	footnotes, endnotes      map[int]int
	comments                 map[int]int
	queue                    []Run
}

func NewReferenceCursor(inner Cursor, footnotes, endnotes, comments map[int]int) Cursor {
	if inner == nil || (len(footnotes) == 0 && len(endnotes) == 0 && len(comments) == 0) {
		return inner
	}
	return &referenceCursor{inner: inner, footnotes: footnotes, endnotes: endnotes, comments: comments}
}

func (c *referenceCursor) Next() (Run, bool, error) {
	for {
		if len(c.queue) > 0 {
			run := c.queue[0]
			c.queue = c.queue[1:]
			return run, true, nil
		}
		run, ok, err := c.inner.Next()
		if err != nil || !ok {
			return run, ok, err
		}
		c.expand(run)
	}
}

func (c *referenceCursor) expand(run Run) {
	if run.Kind != "" || run.Text == "" {
		c.queue = append(c.queue, run)
		return
	}
	runes := []rune(run.Text)
	segmentStart := 0
	segmentCP := run.StartCP
	for i := 0; i < len(runes); i++ {
		cp := run.StartCP + i
		footnoteID, hasFootnote := c.footnotes[cp]
		endnoteID, hasEndnote := c.endnotes[cp]
		commentID, hasComment := c.comments[cp]
		if !hasFootnote && !hasEndnote && !hasComment {
			continue
		}
		if i > segmentStart {
			c.queue = append(c.queue, cloneRunWithText(run, string(runes[segmentStart:i]), segmentCP))
		}
		if hasFootnote {
			c.queue = append(c.queue, Run{Kind: "footnoteRef", NoteID: footnoteID, StartCP: cp, Format: run.Format, ParagraphFormat: run.ParagraphFormat})
		}
		if hasEndnote {
			c.queue = append(c.queue, Run{Kind: "endnoteRef", NoteID: endnoteID, StartCP: cp, Format: run.Format, ParagraphFormat: run.ParagraphFormat})
		}
		if hasComment {
			c.queue = append(c.queue, Run{Kind: "commentRef", CommentID: commentID, StartCP: cp, Format: run.Format, ParagraphFormat: run.ParagraphFormat})
		}
		segmentStart = i
		segmentCP = cp
	}
	if segmentStart < len(runes) {
		c.queue = append(c.queue, cloneRunWithText(run, string(runes[segmentStart:]), segmentCP))
	}
}

// bookmarkCursor injects bookmark boundaries into the normal CP-ordered stream.
type bookmarkCursor struct {
	inner   Cursor
	starts  map[int][]string
	ends    map[int][]string
	queue   []Run
	emitted map[string]bool
}

func NewBookmarkCursor(inner Cursor, starts, ends map[int][]string) Cursor {
	if inner == nil || (len(starts) == 0 && len(ends) == 0) {
		return inner
	}
	return &bookmarkCursor{inner: inner, starts: starts, ends: ends, emitted: make(map[string]bool)}
}

func (c *bookmarkCursor) Next() (Run, bool, error) {
	for {
		if len(c.queue) > 0 {
			run := c.queue[0]
			c.queue = c.queue[1:]
			return run, true, nil
		}
		run, ok, err := c.inner.Next()
		if err != nil || !ok {
			return run, ok, err
		}
		c.expand(run)
	}
}

func (c *bookmarkCursor) emitBoundary(kind, name string, cp int, run Run) {
	key := fmt.Sprintf("%s|%d|%s", kind, cp, name)
	if c.emitted[key] {
		return
	}
	c.emitted[key] = true
	c.queue = append(c.queue, Run{Kind: kind, Anchor: name, StartCP: cp, Format: run.Format, ParagraphFormat: run.ParagraphFormat})
}

func (c *bookmarkCursor) expand(run Run) {
	if run.Kind != "" || run.Text == "" {
		c.queue = append(c.queue, run)
		return
	}
	runes := []rune(run.Text)
	segmentStart := 0
	segmentCP := run.StartCP
	for i := 0; i < len(runes); i++ {
		cp := run.StartCP + i
		starts := c.starts[cp]
		ends := c.ends[cp]
		if len(starts) == 0 && len(ends) == 0 {
			continue
		}
		if i > segmentStart {
			c.queue = append(c.queue, cloneRunWithText(run, string(runes[segmentStart:i]), segmentCP))
		}
		for _, name := range ends {
			c.emitBoundary("bookmarkEnd", name, cp, run)
		}
		for _, name := range starts {
			c.emitBoundary("bookmarkStart", name, cp, run)
		}
		segmentStart = i
		segmentCP = cp
	}
	endCP := run.StartCP + len(runes)
	if segmentStart < len(runes) {
		c.queue = append(c.queue, cloneRunWithText(run, string(runes[segmentStart:]), segmentCP))
	}
	for _, name := range c.ends[endCP] {
		c.emitBoundary("bookmarkEnd", name, endCP, run)
	}
	for _, name := range c.starts[endCP] {
		c.emitBoundary("bookmarkStart", name, endCP, run)
	}
}

// sectionBoundaryCursor splits visible runs at physical PlcfSed section ends.
type sectionBoundaryCursor struct {
	inner      Cursor
	boundaries []int
	queue      []Run
}

func NewSectionBoundaryCursor(inner Cursor, sections []wordir.Section) Cursor {
	if inner == nil || len(sections) == 0 {
		return inner
	}
	boundaries := make([]int, 0, len(sections))
	for _, section := range sections {
		if section.Source != nil && section.Source.EndCP > 0 {
			boundaries = append(boundaries, section.Source.EndCP)
		}
	}
	if len(boundaries) == 0 {
		return inner
	}
	sort.Ints(boundaries)
	uniq := boundaries[:0]
	for _, cp := range boundaries {
		if len(uniq) == 0 || uniq[len(uniq)-1] != cp {
			uniq = append(uniq, cp)
		}
	}
	return &sectionBoundaryCursor{inner: inner, boundaries: uniq}
}

func (c *sectionBoundaryCursor) Next() (Run, bool, error) {
	for {
		if len(c.queue) > 0 {
			run := c.queue[0]
			c.queue = c.queue[1:]
			return run, true, nil
		}
		run, ok, err := c.inner.Next()
		if err != nil || !ok {
			return run, ok, err
		}
		c.expand(run)
	}
}

func (c *sectionBoundaryCursor) expand(run Run) {
	if run.Kind != "" || run.Text == "" {
		c.queue = append(c.queue, run)
		return
	}
	runes := []rune(run.Text)
	endCP := run.StartCP + len(runes)
	start := 0
	for _, boundary := range c.boundaries {
		if boundary <= run.StartCP || boundary >= endCP {
			continue
		}
		off := boundary - run.StartCP
		if off > start {
			c.queue = append(c.queue, cloneRunWithText(run, string(runes[start:off]), run.StartCP+start))
		}
		start = off
	}
	if start < len(runes) {
		c.queue = append(c.queue, cloneRunWithText(run, string(runes[start:]), run.StartCP+start))
	}
}
