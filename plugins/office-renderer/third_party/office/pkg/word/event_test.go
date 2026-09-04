package word

import "testing"

func TestWalkBytesFacadeEmitsWordEvents(t *testing.T) {
	data := makeDOCXFixture(t, "event facade text")
	var sawText bool
	err := WalkBytes(data, EventHandlerFunc(func(event Event) error {
		if event.Type == EventText && event.Text == "event facade text" {
			sawText = true
		}
		return nil
	}), EventOptions{})
	if err != nil {
		t.Fatalf("WalkBytes: %v", err)
	}
	if !sawText {
		t.Fatal("expected root word event facade to emit text event")
	}
}
