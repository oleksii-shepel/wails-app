package sax

import (
	presentationevent "office/pkg/presentation/parsers/event"
	"os"
)

// WalkPresentationEventsBytes exposes OOXML through the same format-neutral
// semantic event boundary used by legacy PPT. XML SAX remains the producer;
// callers no longer need to bind themselves to PresentationML token events.
func WalkPresentationEventsBytes(data []byte, consumer presentationevent.Consumer, opts Options) error {
	p, err := BuildContextualPresentationBytes(data, opts)
	if err != nil {
		return err
	}
	return presentationevent.EmitPresentation("pptx", p, consumer)
}

func WalkPresentationEventsPath(filePath string, consumer presentationevent.Consumer, opts Options) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	return WalkPresentationEventsBytes(data, consumer, opts)
}

// PresentationProducer adapts the OOXML SAX pipeline to the format-neutral
// producer contract.
type PresentationProducer struct {
	Data    []byte
	Options Options
}

func (p PresentationProducer) ProducePresentationEvents(consumer presentationevent.Consumer) error {
	return WalkPresentationEventsBytes(p.Data, consumer, p.Options)
}
