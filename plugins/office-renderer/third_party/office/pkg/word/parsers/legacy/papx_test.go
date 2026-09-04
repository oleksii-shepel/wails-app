package legacy

import (
	"testing"
)

func TestParsePapxGrpprlWithMetadataSprmTPropRMark(t *testing.T) {
	// sprmTPropRMark (0xD667) with 7-byte PropRMarkOperand:
	// 1 byte fLock, 2 bytes ibst, 4 bytes dttm
	grpprl := []byte{
		0x67, 0xD6, 0x07, // sprmTPropRMark with length byte prefix (7)
		0x01,             // fLock = 1 (present)
		0x02, 0x00,       // ibst = 2
		0xE5, 0x07, 0x0A, 0x00, // dttm = 0x000A07E5 (packed date)
	}
	_, meta := parsePapxGrpprlWithMetadata(grpprl)
	if meta.TableRowProps == nil {
		t.Fatal("TableRowProps = nil, want non-nil")
	}
	if meta.TableRowProps.propertyRevision == nil {
		t.Fatal("propertyRevision = nil, want non-nil")
	}
	pr := meta.TableRowProps.propertyRevision
	if !pr.Present {
		t.Fatal("Present = false, want true")
	}
	if pr.Ibst != 2 {
		t.Fatalf("Ibst = %d, want 2", pr.Ibst)
	}
	if !pr.HasIbst {
		t.Fatal("HasIbst = false, want true")
	}
	if pr.Dttm != 0x000A07E5 {
		t.Fatalf("Dttm = 0x%08X, want 0x000A07E5", pr.Dttm)
	}
	if !pr.HasDttm {
		t.Fatal("HasDttm = false, want true")
	}
}

func TestParsePapxGrpprlWithMetadataSprmTPropRMarkShortPayload(t *testing.T) {
	// A truncated payload (less than 7 bytes) should not crash but should not
	// populate propertyRevision either.
	grpprl := []byte{
		0x67, 0xD6, 0x03, // sprmTPropRMark with length=3 (too short for PropRMarkOperand)
		0x01, 0x05, 0x00, // only 3 bytes of payload
	}
	_, meta := parsePapxGrpprlWithMetadata(grpprl)
	if meta.TableRowProps != nil && meta.TableRowProps.propertyRevision != nil {
		t.Fatal("propertyRevision should be nil for short payload")
	}
}

func TestParsePapxGrpprlWithMetadataSprmTPropRMarkAbsent(t *testing.T) {
	grpprl := []byte{
		0x67, 0xD6, 0x07,
		0x00,       // fLock = 0 (not present)
		0x03, 0x00, // ibst = 3
		0xE5, 0x07, 0x0A, 0x00,
	}
	_, meta := parsePapxGrpprlWithMetadata(grpprl)
	if meta.TableRowProps == nil || meta.TableRowProps.propertyRevision == nil {
		t.Fatal("propertyRevision = nil")
	}
	if meta.TableRowProps.propertyRevision.Present {
		t.Fatal("Present = true, want false")
	}
}