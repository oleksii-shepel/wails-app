package docformat

import (
	"encoding/binary"
	"testing"
)

func TestDecodeCHPXMapsCoreRunFormattingAndAnchors(t *testing.T) {
	// sprmCFBold=true, sprmCHps=30 half-points, sprmCFSpec=true,
	// sprmCFData=true, sprmCFObj=true.
	grpprl := []byte{
		0x35, 0x08, 0x01,
		0x43, 0x4A, 0x1E, 0x00,
		0x55, 0x08, 0x01,
		0x06, 0x08, 0x01,
		0x56, 0x08, 0x01,
	}
	got := DecodeCHPX(grpprl, nil)
	if got.Format.Bold == nil || !*got.Format.Bold {
		t.Fatalf("Bold = %#v, want true", got.Format.Bold)
	}
	if got.Format.FontSize == nil || *got.Format.FontSize != 30 {
		t.Fatalf("FontSize = %#v, want 30", got.Format.FontSize)
	}
	if !got.Spec || !got.Data || !got.Obj {
		t.Fatalf("flags Spec/Data/Obj = %v/%v/%v, want true/true/true", got.Spec, got.Data, got.Obj)
	}
}

func TestDecodeCHPXMapsRevisionAttribution(t *testing.T) {
	grpprl := []byte{
		0x01, 0x08, 0x01, // sprmCFRMarkIns
		0x04, 0x48, 0x07, 0x00, // sprmCIbstRMark
		0x05, 0x68, 0x78, 0x56, 0x34, 0x12, // sprmCDttmRMark
	}
	got := DecodeCHPX(grpprl, nil)
	if got.Revision != "inserted" || !got.HasIbstRMark || got.IbstRMark != 7 || !got.HasDttmRMark || got.DttmRMark != 0x12345678 {
		t.Fatalf("revision metadata = %#v", got)
	}
}

func TestParseCHPXFKPUsesNativeDecoderByDefault(t *testing.T) {
	page := make([]byte, 512)
	page[511] = 1
	binary.LittleEndian.PutUint32(page[0:4], 100)
	binary.LittleEndian.PutUint32(page[4:8], 101)
	page[8] = 20
	page[40] = 3
	copy(page[41:44], []byte{0x35, 0x08, 0x01})

	got := ParseCHPXFKP(page, nil, nil)
	if len(got) != 1 || got[0].FC != 100 || got[0].Format.Bold == nil || !*got[0].Format.Bold {
		t.Fatalf("CHPX = %#v", got)
	}
}
