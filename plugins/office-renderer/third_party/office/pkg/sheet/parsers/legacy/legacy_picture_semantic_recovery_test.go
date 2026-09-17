package legacy

import (
	"encoding/binary"
	"testing"

	"office/pkg/officeart/legacy/escher"
	sheetir "office/pkg/sheet/ir"
)

func TestLegacyWorksheetPictureSurvivesMissingBStorePayload(t *testing.T) {
	sheet := &sheetir.Sheet{Name: "Pictures"}
	payload := legacyPictureShapePayloadForTest(1, 0, 0)
	got := parseLegacyWorksheetImageDrawings(payload, sheet, nil, nil)
	if len(got) != 1 || got[0].Type != sheetir.DrawingTypeImage || got[0].Image == nil {
		t.Fatalf("unresolved picture = %#v, want retained semantic image drawing", got)
	}
	if len(got[0].Image.Data) != 0 {
		t.Fatalf("unresolved picture unexpectedly acquired payload: %#v", got[0].Image)
	}
}

func TestLegacyWorksheetPicturePreservesOfficeArtCropAndTransform(t *testing.T) {
	sheet := &sheetir.Sheet{Name: "Pictures"}
	payload := legacyPictureShapePayloadForTest(1, 16384, 90*65536)
	store := map[uint32]*sheetir.DrawingImage{1: {Data: []byte{1, 2, 3}, ContentType: "image/png"}}
	got := parseLegacyWorksheetImageDrawings(payload, sheet, nil, store)
	if len(got) != 1 || got[0].Image == nil {
		t.Fatalf("picture = %#v", got)
	}
	img := got[0].Image
	if img.CropLeft != 25000 || img.RotationDeg != 90 || !img.FlipH {
		t.Fatalf("picture visual semantics = %+v, want crop=25000 rotation=90 flipH", img)
	}
}

func legacyPictureShapePayloadForTest(pib uint32, cropLeft uint32, rotation uint32) []byte {
	// Sp + FOPT + ClientAnchor inside an SpContainer. The helper intentionally
	// uses the shared OfficeArt property ids so pib can never alias crop again.
	sp := make([]byte, 8)
	binary.LittleEndian.PutUint32(sp[0:4], 7)
	binary.LittleEndian.PutUint32(sp[4:8], 0x40) // flipH
	spRec := buildLegacyOfficeArtRecordForPictureTest(75, 0xF00A, 2, sp)
	props := []escher.Property{{ID: 0x0104, Value: pib}}
	if cropLeft != 0 {
		props = append(props, escher.Property{ID: escher.PropCropLeft, Value: cropLeft})
	}
	if rotation != 0 {
		props = append(props, escher.Property{ID: escher.PropRotation, Value: rotation})
	}
	fopt := make([]byte, len(props)*6)
	for i, p := range props {
		binary.LittleEndian.PutUint16(fopt[i*6:i*6+2], p.ID)
		binary.LittleEndian.PutUint32(fopt[i*6+2:i*6+6], p.Value)
	}
	foptRec := buildLegacyOfficeArtRecordForPictureTest(uint16(len(props)), 0xF00B, 3, fopt)
	anchor := make([]byte, 18)
	// two-cell client anchor: col1,row1 -> col2,row2
	binary.LittleEndian.PutUint16(anchor[2:4], 0)
	binary.LittleEndian.PutUint16(anchor[6:8], 0)
	binary.LittleEndian.PutUint16(anchor[10:12], 1)
	binary.LittleEndian.PutUint16(anchor[14:16], 1)
	anchorRec := buildLegacyOfficeArtRecordForPictureTest(0, 0xF010, 0, anchor)
	body := append(append(spRec, foptRec...), anchorRec...)
	return buildLegacyOfficeArtRecordForPictureTest(0, 0xF004, 0xF, body)
}

func buildLegacyOfficeArtRecordForPictureTest(instance, recType, version uint16, body []byte) []byte {
	out := make([]byte, 8+len(body))
	binary.LittleEndian.PutUint16(out[0:2], (instance<<4)|(version&0xF))
	binary.LittleEndian.PutUint16(out[2:4], recType)
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(body)))
	copy(out[8:], body)
	return out
}
