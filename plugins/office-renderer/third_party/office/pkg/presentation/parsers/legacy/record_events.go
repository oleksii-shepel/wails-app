package legacy

// RecordEvent is the binary-PPT equivalent of an XML SAX token. It exposes a
// forward, nested traversal of the PowerPoint record stream without requiring
// callers to materialize a record tree.
type RecordEvent struct {
	Type     uint16
	Instance uint16
	Version  uint16
	Depth    int
	Offset   int
	Length   int
	Start    bool
	Payload  []byte // leaf payload; container payload is present on start events for semantic consumers
}

type RecordConsumer interface{ HandlePptRecordEvent(RecordEvent) error }
type RecordConsumerFunc func(RecordEvent) error

func (f RecordConsumerFunc) HandlePptRecordEvent(e RecordEvent) error { return f(e) }

// WalkRecordEvents traverses a PPT record sequence in physical source order.
// Version 0xF records are containers and generate balanced start/end events;
// atom records generate one leaf event. This is the low-level event producer
// that subsequent legacy takeover passes can connect directly to the shared
// presentation semantic consumer.
func WalkRecordEvents(data []byte, consumer RecordConsumer) error {
	return walkRecordEvents(data, 0, 0, consumer)
}

func walkRecordEvents(data []byte, base, depth int, consumer RecordConsumer) error {
	for pos := 0; pos < len(data); {
		rec, payload, next, ok := readPptRecord(data, pos)
		if !ok {
			break
		}
		e := RecordEvent{Type: rec.Type, Instance: rec.Instance, Version: rec.Version, Depth: depth, Offset: base + pos, Length: rec.Length}
		if rec.Version == 0x0F {
			e.Start = true
			e.Payload = payload
			if err := consumer.HandlePptRecordEvent(e); err != nil {
				return err
			}
			if err := walkRecordEvents(payload, base+pos+8, depth+1, consumer); err != nil {
				return err
			}
			e.Start = false
			e.Payload = nil
			if err := consumer.HandlePptRecordEvent(e); err != nil {
				return err
			}
		} else {
			e.Payload = payload
			if err := consumer.HandlePptRecordEvent(e); err != nil {
				return err
			}
		}
		pos = next
	}
	return nil
}
