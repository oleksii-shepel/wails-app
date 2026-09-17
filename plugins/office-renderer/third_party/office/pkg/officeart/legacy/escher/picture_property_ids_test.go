package escher

import "testing"

func TestOfficeArtPictureCropPropertyIDsDoNotAliasPib(t *testing.T) {
	if PropCropTop != 0x0100 || PropCropBottom != 0x0101 || PropCropLeft != 0x0102 || PropCropRight != 0x0103 {
		t.Fatalf("crop property ids = top:%#x bottom:%#x left:%#x right:%#x", PropCropTop, PropCropBottom, PropCropLeft, PropCropRight)
	}
	const pib = uint16(0x0104)
	if PropCropTop == pib || PropCropBottom == pib || PropCropLeft == pib || PropCropRight == pib {
		t.Fatal("OfficeArt pib 0x0104 must not be decoded as a crop edge")
	}
}
