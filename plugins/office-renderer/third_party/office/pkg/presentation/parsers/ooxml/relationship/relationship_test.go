package relationship

import "testing"

func TestCanonicalTypeNormalizesStrictOfficeRelationship(t *testing.T) {
	got := CanonicalType("  " + StrictOfficeBase + "slide  ")
	if got != Slide {
		t.Fatalf("CanonicalType(strict slide) = %q, want %q", got, Slide)
	}
}

func TestCanonicalTypePreservesUnknownRelationship(t *testing.T) {
	const custom = "urn:example:custom"
	if got := CanonicalType(custom); got != custom {
		t.Fatalf("CanonicalType(custom) = %q", got)
	}
}
