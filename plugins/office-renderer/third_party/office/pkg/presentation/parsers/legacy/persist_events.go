package legacy

import (
	"encoding/binary"
	"sort"
)

// PersistEventKind is the semantic lifecycle exposed by the legacy PPT persist
// directory.  It deliberately sits above raw record events and below authored
// presentation IR: consumers see document/master/slide/notes ownership without
// needing to reopen the PowerPoint Document stream.
type PersistEventKind uint8

const (
	PersistDocumentStart PersistEventKind = iota + 1
	PersistMaster
	PersistSlide
	PersistNotes
	PersistResource
	PersistDocumentEnd
)

type PersistEvent struct {
	Kind      PersistEventKind
	PersistID int
	Offset    int
	Record    RecordEvent
	Payload   []byte
}

type PersistConsumer interface{ HandlePptPersistEvent(PersistEvent) error }
type PersistConsumerFunc func(PersistEvent) error

func (f PersistConsumerFunc) HandlePptPersistEvent(e PersistEvent) error { return f(e) }

// WalkPersistEvents walks the persist-owned presentation lifecycle once in
// authored order.  Master, slide and notes records are materialized from the
// persist directory exactly once and their payload is shared by downstream
// semantic consumers.
func WalkPersistEvents(stream []byte, ctx pptPersistContext, consumer PersistConsumer) error {
	if consumer == nil {
		return nil
	}
	docEvent := PersistEvent{Kind: PersistDocumentStart, Offset: ctx.documentOffset}
	if rec, payload, _, ok := readPptRecord(stream, ctx.documentOffset); ok {
		docEvent.Record = RecordEvent{Type: rec.Type, Instance: rec.Instance, Version: rec.Version, Offset: ctx.documentOffset, Length: rec.Length, Start: true, Payload: payload}
		docEvent.Payload = payload
	}
	if err := consumer.HandlePptPersistEvent(docEvent); err != nil {
		return err
	}
	seen := map[int]bool{}
	emit := func(kind PersistEventKind, id int) error {
		if id == 0 || seen[id] {
			return nil
		}
		offset, ok := ctx.persistDirectory[id]
		if !ok {
			return nil
		}
		rec, payload, _, ok := readPptRecord(stream, offset)
		if !ok {
			return nil
		}
		seen[id] = true
		return consumer.HandlePptPersistEvent(PersistEvent{
			Kind: kind, PersistID: id, Offset: offset,
			Record:  RecordEvent{Type: rec.Type, Instance: rec.Instance, Version: rec.Version, Offset: offset, Length: rec.Length, Start: true, Payload: payload},
			Payload: payload,
		})
	}
	for _, master := range ctx.masterEntries {
		if err := emit(PersistMaster, master.RefID); err != nil {
			return err
		}
	}
	for _, id := range ctx.slidePersistIDs {
		if err := emit(PersistSlide, id); err != nil {
			return err
		}
	}
	for _, id := range ctx.notesPersistIDs {
		if err := emit(PersistNotes, id); err != nil {
			return err
		}
	}
	// Emit every remaining persisted object once as a resource event. This closes
	// the last normal-path reason for semantic consumers (notably embedded OLE
	// storage) to seek back into the PowerPoint Document stream.
	resourceIDs := make([]int, 0, len(ctx.persistDirectory))
	for id := range ctx.persistDirectory {
		if id != 0 && !seen[id] {
			resourceIDs = append(resourceIDs, id)
		}
	}
	sort.Ints(resourceIDs)
	for _, id := range resourceIDs {
		if err := emit(PersistResource, id); err != nil {
			return err
		}
	}
	return consumer.HandlePptPersistEvent(PersistEvent{Kind: PersistDocumentEnd, Offset: ctx.documentOffset})
}

type legacyPptPersistEventIndex struct {
	document  PersistEvent
	masters   map[int]PersistEvent
	slides    map[int]PersistEvent
	notes     map[int]PersistEvent
	resources map[int]PersistEvent
}

func collectLegacyPptPersistEventIndex(stream []byte, ctx pptPersistContext) legacyPptPersistEventIndex {
	idx := legacyPptPersistEventIndex{masters: map[int]PersistEvent{}, slides: map[int]PersistEvent{}, notes: map[int]PersistEvent{}, resources: map[int]PersistEvent{}}
	_ = WalkPersistEvents(stream, ctx, PersistConsumerFunc(func(e PersistEvent) error {
		switch e.Kind {
		case PersistDocumentStart:
			idx.document = e
		case PersistMaster:
			idx.masters[e.PersistID] = e
		case PersistSlide:
			idx.slides[e.PersistID] = e
		case PersistNotes:
			idx.notes[e.PersistID] = e
		case PersistResource:
			idx.resources[e.PersistID] = e
		}
		return nil
	}))
	return idx
}

// parsePersistHeaderFooterInfos projects per-persist header/footer overrides from
// payloads already materialized by WalkPersistEvents. It is intentionally
// stream-free: slide and notes semantic consumers must not reopen persist
// records after the lifecycle boundary has handed them an event.
func parsePersistHeaderFooterInfos(events map[int]PersistEvent, persistIDs []int) map[int]legacyPptHeaderFooterInfo {
	if len(events) == 0 || len(persistIDs) == 0 {
		return nil
	}
	out := make(map[int]legacyPptHeaderFooterInfo)
	for _, persistID := range persistIDs {
		e, ok := events[persistID]
		if !ok || len(e.Payload) == 0 {
			continue
		}
		info := parseLegacyPptHeaderFooterInfoFromPersist(e.Payload)
		if info.hasRenderableContent() {
			out[persistID] = info
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// parseMasterStylesFromPersistEvents resolves each master's text bundle from
// the already-opened master event. Document defaults are supplied once by the
// caller and no master persist object is looked up or decoded a second time.
func parseMasterStylesFromPersistEvents(events legacyPptPersistEventIndex, entries []legacyPptMasterEntry, docBundle legacyPptMasterTextBundle, fontTable map[uint16]string, colorScheme map[uint8]string) map[int]legacyPptMasterTextStyles {
	if len(events.masters) == 0 || len(entries) == 0 {
		return nil
	}
	out := make(map[int]legacyPptMasterTextStyles)
	for _, entry := range entries {
		e, ok := events.masters[entry.RefID]
		if !ok || (e.Record.Type != pptRTMainMasterContent && e.Record.Type != pptRTMainMaster) {
			continue
		}
		masterBundle := parseLegacyPptMainMasterTextBundle(e.Payload, fontTable, colorScheme)
		styles := composeLegacyPptMasterTextStyles(docBundle, masterBundle)
		if len(styles) == 0 {
			styles = parseLegacyPptDirectMasterTextStyles(e.Payload, fontTable, colorScheme)
		}
		if len(styles) != 0 {
			out[entry.SlideIdentifier] = styles
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// masterTextFallbackFromPersistEvents builds the presentation-wide fallback
// without walking the physical PowerPoint Document stream. Per-master styles
// remain authoritative; the fallback is only used when a slide cannot bind a
// specific master identifier.
func masterTextFallbackFromPersistEvents(events legacyPptPersistEventIndex, entries []legacyPptMasterEntry, fontTable map[uint16]string, colorScheme map[uint8]string) legacyPptMasterTextStyles {
	docBundle := parseLegacyPptDocumentTextBundle(events.document.Payload, fontTable, colorScheme)
	perMaster := parseMasterStylesFromPersistEvents(events, entries, docBundle, fontTable, colorScheme)
	for _, entry := range entries {
		if styles := perMaster[entry.SlideIdentifier]; len(styles) != 0 {
			return styles
		}
	}
	if len(docBundle.base) != 0 {
		return docBundle.base
	}
	return nil
}

// extractSlideTextsFromPersistEvents projects the authored slide/notes lists
// from the Document event and hydrates notes from their already-opened persist
// events. It is the sequential replacement for extractSlideTextsFromDocument:
// no persist object is reopened through the directory after WalkPersistEvents.
func extractSlideTextsFromPersistEvents(events legacyPptPersistEventIndex, notesListInstance uint16, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string) []legacySlideText {
	payload := events.document.Payload
	if len(payload) == 0 {
		return nil
	}
	if notesListInstance == 0 {
		notesListInstance = 2
	}
	var slides []legacySlideText
	var notes []legacySlideText
	for pos := 0; pos+8 <= len(payload); {
		record, body, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		if record.Type == pptRTSlideListWithText {
			switch record.Instance {
			case 0:
				slides = mergeLegacySlides(slides, parseSlideListWithText(body, fontTable, colorScheme, exHyperlinks))
			case 1:
				// master list is consumed by the master lifecycle
			case notesListInstance:
				notes = mergeLegacySlides(notes, parseSlideListWithText(body, fontTable, colorScheme, exHyperlinks))
			}
		}
		pos = next
	}
	// SlideListWithText owns text order, but SlideAtom itself lives in the persisted
	// Slide container. Hydrate master/layout/notes identity from that authoritative
	// event before any master style, background, placeholder, or artwork lookup.
	// Without this step every event-driven slide retained MasterID=-1 and silently
	// bypassed the authored MainMaster (0x80000000) or explicit master.
	slides = hydrateLegacyPptSlideMetadataFromPersistEvents(slides, events.slides)
	notes = hydrateLegacyPptNotesFromPersistEvents(notes, events.notes, fontTable, colorScheme, exHyperlinks)
	if len(notes) != 0 {
		slides = attachNotesByIndex(slides, notes)
	}
	return slides
}

func hydrateLegacyPptSlideMetadataFromPersistEvents(slides []legacySlideText, events map[int]PersistEvent) []legacySlideText {
	for i := range slides {
		e, ok := events[slides[i].PersistID]
		if !ok || e.Record.Type != pptRTSlide || len(e.Payload) == 0 {
			continue
		}
		walkPptRecords(e.Payload, func(record pptRecord, body []byte) {
			if record.Type != pptRTSlideAtom || len(body) < 16 {
				return
			}
			slides[i].LayoutType = legacyPptSlideLayoutType(body)
			// MS-PPT SlideAtom: SSlideLayoutAtom (12 bytes), masterIdRef (4),
			// notesIdRef (4), flags (2+). Preserve 0x80000000 as int32: it is
			// the authored MainMaster sentinel and is also the master-list key.
			slides[i].MasterID = int(int32(binary.LittleEndian.Uint32(body[12:16])))
			if len(body) >= 20 {
				slides[i].NotesPersistID = int(binary.LittleEndian.Uint32(body[16:20]))
			}
			if len(body) >= 22 {
				// MS-PPT SlideFlags immediately follows notesIdRef. A slide-local
				// ColorSchemeAtom can be physically present but inactive when
				// fMasterScheme says the master's scheme is authoritative.
				flags := binary.LittleEndian.Uint16(body[20:22])
				slides[i].HasSlideFlags = true
				slides[i].FollowMasterObjects = flags&0x0001 != 0
				slides[i].FollowMasterScheme = flags&0x0002 != 0
				slides[i].FollowMasterBackground = flags&0x0004 != 0
			}
		})
	}
	return slides
}

func hydrateLegacyPptNotesFromPersistEvents(notes []legacySlideText, events map[int]PersistEvent, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string) []legacySlideText {
	for index := range notes {
		e, ok := events[notes[index].PersistID]
		if !ok || e.Record.Type != pptRTNotes || len(e.Payload) == 0 {
			continue
		}
		scene := collectLegacyPptVisualScene(e.Payload)
		metas := scene.textMetas(fontTable, colorScheme, exHyperlinks)
		shapes := buildLegacyPptTextShapesFromMetas(metas, notes[index])
		if fragments := legacyPptNotesFragmentsFromShapes(shapes); len(fragments) != 0 {
			notes[index].Fragments = mergeLegacyPptNotesFragments(notes[index].Fragments, fragments)
		}
	}
	return notes
}

// colorSchemeFromPersistPayload extracts the authored ColorSchemeAtom owned by a
// single persist object. Unlike parseLegacyPptColorScheme it never scans the
// complete PowerPoint Document stream, so master and slide palettes retain their
// actual ownership.
func colorSchemeFromPersistPayload(payload []byte) map[uint8]string {
	var result map[uint8]string
	walkPptRecords(payload, func(record pptRecord, body []byte) {
		if record.Type != pptRTColorSchemeAtom {
			return
		}
		if palette, _ := parseLegacyPptColorSchemePayload(body); len(palette) != 0 {
			result = palette
		}
	})
	return result
}

func cloneLegacyPptColorScheme(src map[uint8]string) map[uint8]string {
	if len(src) == 0 {
		return nil
	}
	out := make(map[uint8]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

// masterColorSchemesFromPersistEvents binds each master's palette to the
// SlideIdentifier referenced by SlideAtom.masterID.
func masterColorSchemesFromPersistEvents(events legacyPptPersistEventIndex, entries []legacyPptMasterEntry, fallback map[uint8]string) map[int]map[uint8]string {
	out := make(map[int]map[uint8]string)
	for _, entry := range entries {
		scheme := fallback
		if e, ok := events.masters[entry.RefID]; ok {
			if authored := colorSchemeFromPersistPayload(e.Payload); len(authored) != 0 {
				scheme = authored
			}
		}
		if len(scheme) != 0 {
			out[entry.SlideIdentifier] = cloneLegacyPptColorScheme(scheme)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// slideColorSchemesFromPersistEvents records only explicit slide overrides.
// Missing entries intentionally mean "inherit from master".
func slideColorSchemesFromPersistEvents(events legacyPptPersistEventIndex) map[int]map[uint8]string {
	out := make(map[int]map[uint8]string)
	for id, e := range events.slides {
		if scheme := colorSchemeFromPersistPayload(e.Payload); len(scheme) != 0 {
			out[id] = scheme
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func effectiveLegacyPptSlideColorScheme(slide legacySlideText, persistID int, fallback map[uint8]string, masters map[int]map[uint8]string, slides map[int]map[uint8]string) map[uint8]string {
	if slide.HasSlideFlags && slide.FollowMasterScheme {
		if scheme := masters[slide.MasterID]; len(scheme) != 0 {
			return scheme
		}
		return fallback
	}
	if scheme := slides[persistID]; len(scheme) != 0 {
		return scheme
	}
	if scheme := masters[slide.MasterID]; len(scheme) != 0 {
		return scheme
	}
	return fallback
}

// masterBackgroundsFromPersistEvents resolves background paint at the persist
// ownership boundary. Keys are master persist RefIDs; SlideIdentifier remains
// only an authored relationship identity used to select the master color scheme.
func masterBackgroundsFromPersistEvents(events legacyPptPersistEventIndex, entries []legacyPptMasterEntry, schemes map[int]map[uint8]string, fallback map[uint8]string) map[int]string {
	out := make(map[int]string)
	for _, entry := range entries {
		e, ok := events.masters[entry.RefID]
		if !ok {
			continue
		}
		scheme := schemes[entry.SlideIdentifier]
		if len(scheme) == 0 {
			scheme = fallback
		}
		if fill := parseLegacyPptBgFillFromContainer(e.Payload, scheme); fill != "" {
			out[entry.RefID] = fill
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// presentationColorSchemeFromPersistEvents chooses the presentation fallback
// palette from owned lifecycle payloads. Document ownership wins, followed by
// authored master order. This replaces the old whole-stream "most frequent
// palette" heuristic on valid PPT97 persist chains.
func presentationColorSchemeFromPersistEvents(events legacyPptPersistEventIndex, entries []legacyPptMasterEntry) map[uint8]string {
	if scheme := colorSchemeFromPersistPayload(events.document.Payload); len(scheme) != 0 {
		return scheme
	}
	for _, entry := range entries {
		if e, ok := events.masters[entry.RefID]; ok {
			if scheme := colorSchemeFromPersistPayload(e.Payload); len(scheme) != 0 {
				return scheme
			}
		}
	}
	return nil
}
