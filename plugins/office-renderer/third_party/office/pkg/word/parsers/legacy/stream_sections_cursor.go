package legacy

import (
	"sort"

	wordir "office/pkg/word/ir"
)

// legacySectionBoundaryRunCursor splits visible text runs at PlcfSed section
// end CPs. The event layer emits a section before the first run at the next CP;
// without this splitter a CHPX/PAPX run that straddled the boundary could place
// text from two sections under one layout section.
type legacySectionBoundaryRunCursor struct {
	inner      legacyRunCursor
	boundaries []int
	queue      []binaryWordRun
}

func newLegacySectionBoundaryRunCursor(inner legacyRunCursor, sections []wordir.Section) legacyRunCursor {
	if inner == nil || len(sections) == 0 {
		return inner
	}
	boundaries := make([]int, 0, len(sections))
	for _, section := range sections {
		if section.Source == nil || section.Source.EndCP <= 0 {
			continue
		}
		boundaries = append(boundaries, section.Source.EndCP)
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
	return &legacySectionBoundaryRunCursor{inner: inner, boundaries: uniq}
}

func (c *legacySectionBoundaryRunCursor) Next() (binaryWordRun, bool, error) {
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

func (c *legacySectionBoundaryRunCursor) expand(run binaryWordRun) {
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
			c.queue = append(c.queue, cloneDocRunWithText(run, string(runes[start:off]), run.StartCP+start))
		}
		start = off
	}
	if start < len(runes) {
		c.queue = append(c.queue, cloneDocRunWithText(run, string(runes[start:]), run.StartCP+start))
	}
}
