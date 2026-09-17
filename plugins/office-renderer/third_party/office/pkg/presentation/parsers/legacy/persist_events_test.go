package legacy

import (
	"encoding/binary"
	"reflect"
	"testing"
)

func TestWalkPersistEventsOwnsMasterSlideNotesLifecycle(t *testing.T) {
	record := func(typ uint16, payload []byte) []byte {
		b := make([]byte, 8+len(payload))
		binary.LittleEndian.PutUint16(b[2:4], typ)
		binary.LittleEndian.PutUint32(b[4:8], uint32(len(payload)))
		copy(b[8:], payload)
		return b
	}
	master := record(pptRTMainMasterContent, []byte{1})
	slide := record(pptRTSlide, []byte{2})
	notes := record(pptRTNotes, []byte{3})
	stream := append(append(append([]byte{}, master...), slide...), notes...)
	ctx := pptPersistContext{
		persistDirectory: map[int]int{10: 0, 20: len(master), 30: len(master) + len(slide)},
		masterEntries:    []legacyPptMasterEntry{{RefID: 10}}, slidePersistIDs: []int{20}, notesPersistIDs: []int{30},
	}
	var got []PersistEventKind
	var payloads []byte
	if err := WalkPersistEvents(stream, ctx, PersistConsumerFunc(func(e PersistEvent) error {
		got = append(got, e.Kind)
		if len(e.Payload) != 0 {
			payloads = append(payloads, e.Payload[0])
		}
		return nil
	})); err != nil {
		t.Fatal(err)
	}
	want := []PersistEventKind{PersistDocumentStart, PersistMaster, PersistSlide, PersistNotes, PersistDocumentEnd}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lifecycle = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(payloads, []byte{1, 1, 2, 3}) {
		t.Fatalf("payloads = %v", payloads)
	}
}

func TestWalkPersistEventsEmitsUnownedPersistResourcesOnce(t *testing.T) {
	record := func(typ uint16, payload []byte) []byte {
		b := make([]byte, 8+len(payload))
		binary.LittleEndian.PutUint16(b[2:4], typ)
		binary.LittleEndian.PutUint32(b[4:8], uint32(len(payload)))
		copy(b[8:], payload)
		return b
	}
	doc := record(pptRTDocument, nil)
	resource := record(pptRTExOleObjStg, []byte{9, 8, 7})
	stream := append(doc, resource...)
	ctx := pptPersistContext{documentOffset: 0, persistDirectory: map[int]int{77: len(doc)}}
	var resources []PersistEvent
	if err := WalkPersistEvents(stream, ctx, PersistConsumerFunc(func(e PersistEvent) error {
		if e.Kind == PersistResource {
			resources = append(resources, e)
		}
		return nil
	})); err != nil {
		t.Fatal(err)
	}
	if len(resources) != 1 || resources[0].PersistID != 77 || resources[0].Record.Type != pptRTExOleObjStg {
		t.Fatalf("resources = %#v", resources)
	}
	if !reflect.DeepEqual(resources[0].Payload, []byte{9, 8, 7}) {
		t.Fatalf("resource payload = %v", resources[0].Payload)
	}
}

func TestHydrateLegacyPptSlideMetadataFromPersistEventsUsesPersistedSlideAtom(t *testing.T) {
	atom := make([]byte, 24)
	binary.LittleEndian.PutUint32(atom[0:4], 7)
	binary.LittleEndian.PutUint32(atom[12:16], 0x80000000)
	binary.LittleEndian.PutUint32(atom[16:20], 284)
	binary.LittleEndian.PutUint16(atom[20:22], 0x0007)
	payload := buildPptAtomRecord(0, pptRTSlideAtom, atom)
	slides := []legacySlideText{{PersistID: 254, MasterID: -1}}
	events := map[int]PersistEvent{254: {Record: RecordEvent{Type: pptRTSlide}, Payload: payload}}

	got := hydrateLegacyPptSlideMetadataFromPersistEvents(slides, events)
	if got[0].MasterID != int(int32(-2147483648)) {
		t.Fatalf("MasterID = %d, want MainMaster sentinel", got[0].MasterID)
	}
	if got[0].NotesPersistID != 284 {
		t.Fatalf("NotesPersistID = %d, want 284", got[0].NotesPersistID)
	}
	if got[0].LayoutType == "" {
		t.Fatal("LayoutType was not hydrated from persisted SlideAtom")
	}
	if !got[0].HasSlideFlags || !got[0].FollowMasterObjects || !got[0].FollowMasterScheme || !got[0].FollowMasterBackground {
		t.Fatalf("SlideFlags were not hydrated: %#v", got[0])
	}
}

func TestEffectiveLegacyPptSlideColorSchemeHonorsFollowMasterSchemeFlag(t *testing.T) {
	fallback := map[uint8]string{4: "111111"}
	master := map[int]map[uint8]string{42: {4: "224466"}}
	local := map[int]map[uint8]string{254: {4: "FF0000"}}

	slide := legacySlideText{PersistID: 254, MasterID: 42, HasSlideFlags: true, FollowMasterScheme: true}
	got := effectiveLegacyPptSlideColorScheme(slide, 254, fallback, master, local)
	if got[4] != "224466" {
		t.Fatalf("scheme[4] = %q, want master 224466 when fMasterScheme is set", got[4])
	}

	slide.FollowMasterScheme = false
	got = effectiveLegacyPptSlideColorScheme(slide, 254, fallback, master, local)
	if got[4] != "FF0000" {
		t.Fatalf("scheme[4] = %q, want slide-local FF0000 when fMasterScheme is clear", got[4])
	}
}
