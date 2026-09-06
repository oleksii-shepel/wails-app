package pre97

import (
	"errors"
	"testing"

	"office/pkg/ooxml/cfb"
)

type fakeCompatibility struct {
	called bool
	err error
}

func (f *fakeCompatibility) OpenOpenedCFB(_ *cfb.File) (Cursor, *Context, error) {
	f.called = true
	if f.err != nil {
		return nil, nil, f.err
	}
	return &sliceRunCursor{runs: []Run{{Text: "hello\r"}}}, &Context{}, nil
}

type sliceRunCursor struct {
	runs []Run
	index int
}

func (c *sliceRunCursor) Next() (Run, bool, error) {
	if c == nil || c.index >= len(c.runs) {
		return Run{}, false, nil
	}
	run := c.runs[c.index]
	c.index++
	return run, true, nil
}

func TestWalkOpenedCFBRequiresInputs(t *testing.T) {
	if err := WalkOpenedCFB(nil, &fakeCompatibility{}, func(Event) error { return nil }); err == nil {
		t.Fatal("WalkOpenedCFB(nil, ...) = nil, want error")
	}
}

func TestWalkOpenedCFBDelegatesOpenButCanonicalWalkEmitsEvents(t *testing.T) {
	service := &fakeCompatibility{}
	file := &cfb.File{}
	var seen []EventType
	err := WalkOpenedCFB(file, service, func(event Event) error {
		seen = append(seen, event.Type)
		return nil
	})
	if err != nil {
		t.Fatalf("WalkOpenedCFB: %v", err)
	}
	if !service.called {
		t.Fatal("compatibility opener was not called")
	}
	want := []EventType{EventStartDocument, EventStartParagraph, EventText, EventEndParagraph, EventEndDocument}
	if len(seen) != len(want) {
		t.Fatalf("events = %v, want %v", seen, want)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("events[%d] = %s, want %s", i, seen[i], want[i])
		}
	}
}

func TestWalkOpenedCFBPreservesNotApplicable(t *testing.T) {
	service := &fakeCompatibility{err: ErrNotApplicable}
	err := WalkOpenedCFB(&cfb.File{}, service, func(Event) error { return nil })
	if !errors.Is(err, ErrNotApplicable) {
		t.Fatalf("error = %v, want ErrNotApplicable", err)
	}
}
