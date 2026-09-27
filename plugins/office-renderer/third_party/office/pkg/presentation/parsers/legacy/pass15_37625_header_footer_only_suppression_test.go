package legacy

import "testing"

func TestPass15HeaderFooterOnlyPersistMovesExplicitDisableToPreviousSlide(t *testing.T) {
	slides := []legacySlideText{
		{PersistID: 100, Fragments: []legacySlideFragment{{Text: "Title"}}},
		// PowerPoint can emit header/footer metadata as its own prompt-only persist
		// immediately after the real slide. This record must not become a slide.
		{PersistID: 101, Fragments: []legacySlideFragment{{Text: "Click to edit Master title style"}}},
	}

	infos := map[int]legacyPptHeaderFooterInfo{
		101: {hasAtom: true, flags: 0},
	}

	normalizedSlides, normalizedInfos := normalizeLegacyPptHeaderFooterOnlySlides(slides, nil, infos)
	if len(normalizedSlides) != 1 {
		t.Fatalf("slides = %d, want 1", len(normalizedSlides))
	}
	info, ok := normalizedInfos[100]
	if !ok {
		t.Fatal("explicit footer suppression was not moved onto previous real slide")
	}
	if !info.hasAtom || info.flags != 0 {
		t.Fatalf("moved suppression = hasAtom %v flags %#x, want true/0", info.hasAtom, info.flags)
	}
	if _, ok := normalizedInfos[101]; ok {
		t.Fatal("header/footer-only synthetic persist still owns suppression after normalization")
	}
}
