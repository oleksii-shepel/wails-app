package legacy

import "testing"

func legacyTestPalette(seed byte) []byte {
	out := make([]byte, 32)
	for i := 0; i < 8; i++ {
		off := i * 4
		out[off], out[off+1], out[off+2] = seed+byte(i), seed+byte(i)+1, seed+byte(i)+2
	}
	return out
}

func TestPersistColorSchemesPreserveMasterAndSlideOwnership(t *testing.T) {
	masterA := buildPptAtomRecord(0, pptRTColorSchemeAtom, legacyTestPalette(0x10))
	masterB := buildPptAtomRecord(0, pptRTColorSchemeAtom, legacyTestPalette(0x30))
	slideOverride := buildPptAtomRecord(0, pptRTColorSchemeAtom, legacyTestPalette(0x50))
	events := legacyPptPersistEventIndex{
		masters: map[int]PersistEvent{11: {PersistID: 11, Payload: masterA}, 12: {PersistID: 12, Payload: masterB}},
		slides:  map[int]PersistEvent{21: {PersistID: 21, Payload: slideOverride}, 22: {PersistID: 22}},
	}
	entries := []legacyPptMasterEntry{{RefID: 11, SlideIdentifier: 101}, {RefID: 12, SlideIdentifier: 102}}
	masters := masterColorSchemesFromPersistEvents(events, entries, nil)
	slides := slideColorSchemesFromPersistEvents(events)
	if got := masters[101][0]; got != "101112" {
		t.Fatalf("master 101 color 0 = %q", got)
	}
	if got := masters[102][0]; got != "303132" {
		t.Fatalf("master 102 color 0 = %q", got)
	}
	if got := effectiveLegacyPptSlideColorScheme(legacySlideText{MasterID: 101}, 21, nil, masters, slides)[0]; got != "505152" {
		t.Fatalf("slide override color 0 = %q", got)
	}
	if got := effectiveLegacyPptSlideColorScheme(legacySlideText{MasterID: 102}, 22, nil, masters, slides)[0]; got != "303132" {
		t.Fatalf("inherited master color 0 = %q", got)
	}
}

func TestPresentationColorSchemeUsesOwnedPersistPayloads(t *testing.T) {
	doc := buildPptAtomRecord(0, pptRTColorSchemeAtom, legacyTestPalette(0x70))
	master := buildPptAtomRecord(0, pptRTColorSchemeAtom, legacyTestPalette(0x20))
	events := legacyPptPersistEventIndex{document: PersistEvent{Payload: doc}, masters: map[int]PersistEvent{7: {Payload: master}}}
	got := presentationColorSchemeFromPersistEvents(events, []legacyPptMasterEntry{{RefID: 7, SlideIdentifier: 9}})
	if got[0] != "707172" {
		t.Fatalf("document-owned fallback = %q", got[0])
	}
}
