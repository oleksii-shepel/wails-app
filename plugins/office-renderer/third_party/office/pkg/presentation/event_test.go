package presentation

import (
	"bytes"
	"testing"

	presedit "office/pkg/presentation/edit"
)

func TestWalkBytesFacadeEmitsPresentationEvents(t *testing.T) {
	data := makePPTXFixture(t, "presentation event text")
	var sawText bool
	err := WalkBytes(data, EventHandlerFunc(func(event Event) error {
		if event.Type == EventText && event.Text == "presentation event text" {
			sawText = true
		}
		return nil
	}), EventOptions{})
	if err != nil {
		t.Fatalf("WalkBytes: %v", err)
	}
	if !sawText {
		t.Fatal("expected root presentation event facade to emit text event")
	}
}

func makePPTXFixture(t *testing.T, text string) []byte {
	t.Helper()
	p := presedit.NewPresentation()
	s := p.AddBlankSlide()
	s.AddTextBox(100, 100, 3000, 1200).SetText(text)
	var buf bytes.Buffer
	if err := p.SaveAsPPTX(&buf); err != nil {
		t.Fatalf("SaveAsPPTX: %v", err)
	}
	return buf.Bytes()
}
