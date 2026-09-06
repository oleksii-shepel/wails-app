package layout

// PageWindow retains only a bounded range of finalized pages. It is designed
// for incremental pagination: once a page is finalized, callers Push it and
// may immediately discard older page display state.
type PageWindow struct {
	start int
	end   int
	pages []Page
}

// NewPageWindow creates a 1-based inclusive page window. end <= 0 means no
// upper bound and is normalized to the public sentinel 0. start values below
// 1 are normalized to 1.
func NewPageWindow(start, end int) *PageWindow {
	if start < 1 {
		start = 1
	}
	if end <= 0 {
		end = 0
	} else if end < start {
		end = start
	}

	capacity := 1
	if end >= start {
		capacity = end - start + 1
	}
	return &PageWindow{start: start, end: end, pages: make([]Page, 0, capacity)}
}

// Push accepts a finalized page. It returns false when pagination can stop
// because the requested upper bound has already been satisfied.
func (w *PageWindow) Push(page Page) bool {
	if w == nil {
		return false
	}
	if w.end > 0 && page.Number > w.end {
		return false
	}
	if page.Number >= w.start {
		w.pages = append(w.pages, page)
	}
	return w.end == 0 || page.Number < w.end
}

// Pages returns the retained pages. The returned slice aliases the window and
// should be treated as read-only.
func (w *PageWindow) Pages() []Page {
	if w == nil {
		return nil
	}
	return w.pages
}

// Done reports whether a finalized page satisfies the requested upper bound.
func (w *PageWindow) Done(pageNumber int) bool {
	return w != nil && w.end > 0 && pageNumber >= w.end
}

// Start returns the first requested 1-based page number.
func (w *PageWindow) Start() int {
	if w == nil {
		return 1
	}
	return w.start
}

// End returns the inclusive upper bound, or zero when unbounded.
func (w *PageWindow) End() int {
	if w == nil {
		return 0
	}
	return w.end
}
