package word97

import "testing"

func TestMainStoryCursorStopsBeforeSideStories(t *testing.T) {
	inner := &sliceCursor{runs: []Run{
		{Text: "body\r", StartCP: 0},
		{Text: "textbox\r", StartCP: 5},
	}}
	cursor := mainStoryCursor(inner, 5)
	var got string
	for {
		run, ok, err := cursor.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if !ok {
			break
		}
		got += run.Text
	}
	if got != "body\r" {
		t.Fatalf("main story = %q, want %q", got, "body\r")
	}
}

func TestMainStoryCursorClipsRunAtMainStoryBoundary(t *testing.T) {
	inner := &sliceCursor{runs: []Run{{Text: "body\rtextbox", StartCP: 0}}}
	cursor := mainStoryCursor(inner, 5)
	run, ok, err := cursor.Next()
	if err != nil || !ok {
		t.Fatalf("Next = (%+v, %v, %v)", run, ok, err)
	}
	if run.Text != "body\r" {
		t.Fatalf("text = %q, want %q", run.Text, "body\r")
	}
	if _, ok, err := cursor.Next(); err != nil || ok {
		t.Fatalf("second Next = (ok=%v, err=%v), want EOF", ok, err)
	}
}
