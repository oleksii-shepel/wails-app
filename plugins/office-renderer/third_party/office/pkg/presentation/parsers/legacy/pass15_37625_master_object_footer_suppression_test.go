package legacy

import "testing"

func TestPass15MasterObjectsOffSuppressesInheritedHeaderFooterDefaults(t *testing.T) {
	defaults := legacyPptHeaderFooterInfo{
		hasAtom:    true,
		flags:      legacyPptHFFlagDateVisible | legacyPptHFFlagUserDate | legacyPptHFFlagFooter,
		dateText:   "26 August 2004",
		footerText: "Transport CDM Workshop",
	}
	slide := legacySlideText{HasSlideFlags: true, FollowMasterObjects: false}

	got := effectiveLegacyPptHeaderFooterInfo(slide, defaults, legacyPptHeaderFooterInfo{})
	if got.dateEnabled() || got.footerEnabled() || got.dateText != "" || got.footerText != "" {
		t.Fatalf("master objects disabled inherited footer/date: %+v", got)
	}
}

func TestPass15MasterObjectsOffKeepsExplicitSlideLocalHeaderFooterOverride(t *testing.T) {
	defaults := legacyPptHeaderFooterInfo{
		hasAtom:    true,
		flags:      legacyPptHFFlagDateVisible | legacyPptHFFlagUserDate | legacyPptHFFlagFooter,
		dateText:   "document date",
		footerText: "document footer",
	}
	override := legacyPptHeaderFooterInfo{
		hasAtom:    true,
		flags:      legacyPptHFFlagFooter,
		footerText: "slide footer",
	}
	slide := legacySlideText{HasSlideFlags: true, FollowMasterObjects: false}

	got := effectiveLegacyPptHeaderFooterInfo(slide, defaults, override)
	if got.dateEnabled() || got.dateText != "" {
		t.Fatalf("slide-local override inherited document date: %+v", got)
	}
	if !got.footerEnabled() || got.footerText != "slide footer" {
		t.Fatalf("slide-local footer = %+v, want explicit slide footer", got)
	}
}
