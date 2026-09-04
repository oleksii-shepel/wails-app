package legacy

import "testing"

func TestOfficeArtAndPPTArrowKindsRemainDistinctContracts(t *testing.T) {
	generic := map[uint32]ArrowKind{
		1: ArrowTriangle,
		2: ArrowStealth,
		3: ArrowDiamond,
		4: ArrowOval,
		5: ArrowOpen,
	}
	for raw, want := range generic {
		if got := ArrowKindFromOfficeArt(raw); got != want {
			t.Fatalf("generic arrow kind %d = %q, want %q", raw, got, want)
		}
	}

	ppt := map[uint32]ArrowKind{
		2: ArrowTriangle,
		3: ArrowOpen,
		4: ArrowStealth,
		5: ArrowDiamond,
		6: ArrowOval,
	}
	for raw, want := range ppt {
		if got := PPTArrowKind(raw); got != want {
			t.Fatalf("PPT arrow kind %d = %q, want %q", raw, got, want)
		}
	}
}

func TestOfficeArtAndPPTArrowSizesRemainDistinctContracts(t *testing.T) {
	if ArrowSizeFromOfficeArt(0) != ArrowSmall ||
		ArrowSizeFromOfficeArt(1) != ArrowMedium ||
		ArrowSizeFromOfficeArt(2) != ArrowLarge {
		t.Fatal("generic OfficeArt size mapping changed")
	}
	if PPTArrowSize(1) != ArrowSmall ||
		PPTArrowSize(2) != ArrowMedium ||
		PPTArrowSize(3) != ArrowLarge {
		t.Fatal("PPT compatibility size mapping changed")
	}
}
