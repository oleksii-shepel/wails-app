package legacy

import (
	"encoding/binary"
	"testing"
)

func TestClass20SlideShowInfoUsesNormativeMSPPTFlagBits(t *testing.T) {
	payload := make([]byte, 16)
	binary.LittleEndian.PutUint32(payload[0:4], 2500)
	payload[9] = 10 // wipe
	// fManualAdvance (bit 0), fHidden (bit 2), fAutoAdvance (bit 10).
	// Hidden must not be mistaken for auto-advance.
	binary.LittleEndian.PutUint16(payload[10:12], (1<<0)|(1<<2)|(1<<10))
	payload[12] = 0x02 // fast

	got, ok := parseLegacyPptSlideShowInfo(payload)
	if !ok || got == nil {
		t.Fatal("SSlideInfoAtom was not parsed")
	}
	if !got.AdvClick {
		t.Fatal("fManualAdvance bit 0 was not preserved")
	}
	if got.AdvTmMs != 2500 {
		t.Fatalf("auto advance = %d, want 2500 from fAutoAdvance bit 10", got.AdvTmMs)
	}
	if got.Spd != "fast" {
		t.Fatalf("speed = %q, want fast", got.Spd)
	}
}

func TestClass20SlideShowInfoDoesNotTreatLegacyBitFiveAsAutoAdvance(t *testing.T) {
	payload := make([]byte, 16)
	binary.LittleEndian.PutUint32(payload[0:4], 3000)
	binary.LittleEndian.PutUint16(payload[10:12], 1<<5)
	got, ok := parseLegacyPptSlideShowInfo(payload)
	if !ok {
		t.Fatal("SSlideInfoAtom was not parsed")
	}
	if got.AdvTmMs != 0 {
		t.Fatalf("auto advance = %d, want 0; bit 5 is not fAutoAdvance", got.AdvTmMs)
	}
	if got.AdvClick {
		t.Fatal("manual advance became true without fManualAdvance bit")
	}
}

func TestClass20SlideShowInfoSpeedMapping(t *testing.T) {
	for _, tc := range []struct {
		raw  byte
		want string
	}{
		{0x00, "slow"},
		{0x01, "med"},
		{0x02, "fast"},
		{0xFF, ""},
	} {
		payload := make([]byte, 16)
		payload[12] = tc.raw
		got, ok := parseLegacyPptSlideShowInfo(payload)
		if !ok {
			t.Fatalf("speed %#x: atom not parsed", tc.raw)
		}
		if got.Spd != tc.want {
			t.Fatalf("speed %#x = %q, want %q", tc.raw, got.Spd, tc.want)
		}
	}
}

func TestClass20SlideShowInfoRejectsTruncatedAtom(t *testing.T) {
	if got, ok := parseLegacyPptSlideShowInfo(make([]byte, 11)); ok || got != nil {
		t.Fatalf("truncated atom = %#v, ok=%v; want nil,false", got, ok)
	}
}
