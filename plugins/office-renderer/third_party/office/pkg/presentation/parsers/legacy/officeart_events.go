package legacy

import officeartescher "office/pkg/officeart/legacy/escher"

// OfficeArtEvent is the semantic binary-PPT equivalent of a DrawingML SAX
// shape-tree event. Container events preserve their complete payload so family
// decoders can consume one SpContainer without recursively walking the slide
// again.
type OfficeArtEvent struct {
	Record     RecordEvent
	GroupStart bool
	GroupEnd   bool
	ShapeStart bool
	ShapeEnd   bool
	Payload    []byte
	ShapeNode  *officeartescher.ShapeNode
}

type OfficeArtConsumer interface{ HandleOfficeArtEvent(OfficeArtEvent) error }
type OfficeArtConsumerFunc func(OfficeArtEvent) error

func (f OfficeArtConsumerFunc) HandleOfficeArtEvent(e OfficeArtEvent) error { return f(e) }

// WalkOfficeArtEvents performs one forward traversal of a slide payload and
// exposes OfficeArt group/shape containers in physical source order.
func WalkOfficeArtEvents(data []byte, consumer OfficeArtConsumer) error {
	return WalkRecordEvents(data, RecordConsumerFunc(func(r RecordEvent) error {
		e := OfficeArtEvent{Record: r, Payload: r.Payload}
		switch r.Type {
		case legacyOfficeArtSpgrContainer:
			e.GroupStart = r.Start
			e.GroupEnd = !r.Start
		case legacyOfficeArtSpContainer:
			e.ShapeStart = r.Start
			e.ShapeEnd = !r.Start
			if r.Start {
				e.ShapeNode, _ = officeartescher.ParseShapeContainerPayload(r.Payload)
			}
		default:
			return nil
		}
		return consumer.HandleOfficeArtEvent(e)
	}))
}

// CollectOfficeArtEvents materializes the semantic OfficeArt event sequence from
// one slide drawing with exactly one binary traversal. The resulting slice is a
// transient dispatch buffer: family decoders may inspect it independently
// without reparsing the Escher tree.
func CollectOfficeArtEvents(data []byte) ([]OfficeArtEvent, error) {
	var events []OfficeArtEvent
	err := WalkOfficeArtEvents(data, OfficeArtConsumerFunc(func(event OfficeArtEvent) error {
		events = append(events, event)
		return nil
	}))
	return events, err
}

func walkCollectedOfficeArtEvents(events []OfficeArtEvent, consumer OfficeArtConsumer) error {
	for _, event := range events {
		if err := consumer.HandleOfficeArtEvent(event); err != nil {
			return err
		}
	}
	return nil
}
