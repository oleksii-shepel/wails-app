package layout

// PageSink consumes finalized pages from pagination. Returning false tells the
// paginator that no later pages are required and pagination may stop.
//
// A Page passed to Emit is complete for rendering: body blocks, floats and
// page-local note boxes have already been resolved.
type PageSink interface {
	Emit(Page) bool
}

// PageSinkFunc adapts a function to PageSink.
type PageSinkFunc func(Page) bool

func (f PageSinkFunc) Emit(page Page) bool {
	if f == nil {
		return false
	}
	return f(page)
}

// PageWindowSink retains only the requested page range and asks pagination to
// stop as soon as the upper bound is satisfied.
type PageWindowSink struct {
	window *PageWindow
}

func NewPageWindowSink(start, end int) *PageWindowSink {
	return &PageWindowSink{window: NewPageWindow(start, end)}
}

func (s *PageWindowSink) Emit(page Page) bool {
	if s == nil || s.window == nil {
		return false
	}
	return s.window.Push(page)
}

func (s *PageWindowSink) Pages() []Page {
	if s == nil || s.window == nil {
		return nil
	}
	return s.window.Pages()
}

func (s *PageWindowSink) Done(pageNumber int) bool {
	return s != nil && s.window != nil && s.window.Done(pageNumber)
}
