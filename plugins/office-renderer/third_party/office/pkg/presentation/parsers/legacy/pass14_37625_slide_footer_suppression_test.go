package legacy

import "testing"

func TestPass14PersistHeaderFooterProjectionKeepsExplicitDisableAtom(t *testing.T) {
	atom := append(marshalLegacyUint16LE(0), marshalLegacyUint16LE(0)...)
	payload := buildPptContainerRecord(0, pptRTHeadersFooters, 0x000F,
		buildPptAtomRecord(0, pptRTHeadersFootersAtom, atom),
	)

	infos := parsePersistHeaderFooterInfos(map[int]PersistEvent{
		101: {PersistID: 101, Payload: payload},
	}, []int{101})

	info, ok := infos[101]
	if !ok {
		t.Fatal("explicit zero-flag HeadersFootersAtom was dropped from persist projection")
	}
	if !info.hasAtom {
		t.Fatal("hasAtom = false, want true for explicit slide-local suppression")
	}
	if info.flags != 0 {
		t.Fatalf("flags = 0x%04x, want 0x0000", info.flags)
	}

	defaults := legacyPptHeaderFooterInfo{
		hasAtom:    true,
		flags:      legacyPptHFFlagDateVisible | legacyPptHFFlagUserDate | legacyPptHFFlagFooter,
		dateText:   "26 August 2004",
		footerText: "Transport CDM Workshop",
	}
	effective := defaults.mergedWith(info)
	if effective.dateEnabled() || effective.footerEnabled() {
		t.Fatalf("effective flags = 0x%04x, want slide-local atom to suppress date/footer defaults", effective.flags)
	}
	if effective.dateText != "" || effective.footerText != "" {
		t.Fatalf("effective date/footer = %q/%q, want both cleared", effective.dateText, effective.footerText)
	}
}

func TestPass14LegacyPersistHeaderFooterMapKeepsExplicitDisableAtom(t *testing.T) {
	infos := mapLegacyPptPersistHeaderFooterInfos([]int{101}, []legacyPptHeaderFooterInfo{{
		hasAtom: true,
		flags:   0,
	}})
	info, ok := infos[101]
	if !ok {
		t.Fatal("explicit zero-flag HeadersFootersAtom was dropped from legacy persist mapping")
	}
	if !info.hasAtom || info.flags != 0 {
		t.Fatalf("mapped info = %#v, want hasAtom=true flags=0", info)
	}
}
