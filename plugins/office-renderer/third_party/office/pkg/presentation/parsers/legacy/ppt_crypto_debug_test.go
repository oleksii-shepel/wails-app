package legacy

import (
	"encoding/binary"
	"testing"

	"office/pkg/ooxml/cfb"
)

func TestDebugPptEncryption(t *testing.T) {
	PptPassword = "crypto"
	defer func() { PptPassword = "" }()

	f, err := cfb.Open("../../../../testdata/ppt/apache-cryptoapi-proc2356.ppt")
	if err != nil {
		t.Fatal(err)
	}
	stream, ok := f.Stream("PowerPoint Document")
	if !ok {
		t.Fatal("missing PowerPoint Document stream")
	}

	currentUser, ok := f.Stream("Current User")
	if !ok {
		t.Fatal("missing Current User stream")
	}
	currentEditOffset, ok := parseCurrentUserOffset(currentUser)
	if !ok {
		t.Fatal("bad Current User offset")
	}
	t.Logf("currentEditOffset: %d", currentEditOffset)

	encInfo, encOffset, ok := findPptEncryptionInfo(stream, currentEditOffset)
	t.Logf("findPptEncryptionInfo ok=%v encOffset=%d", ok, encOffset)
	if !ok {
		// Try manual walk
		seen := map[int]struct{}{}
		for offset := currentEditOffset; offset != 0; {
			if _, ok := seen[offset]; ok {
				break
			}
			seen[offset] = struct{}{}
			record, payload, _, ok2 := readPptRecord(stream, offset)
			if !ok2 {
				t.Logf("readPptRecord failed at offset %d", offset)
				break
			}
			t.Logf("offset %d: type=0x%04X len=%d payload_len=%d", offset, record.Type, record.Length, len(payload))
			if record.Type == pptRTUserEditAtom {
				lastEdit := int(binary.LittleEndian.Uint32(payload[4:8]))
				persistPtr := int(binary.LittleEndian.Uint32(payload[8:12]))
				t.Logf("  lastEdit=%d persistPtr=%d", lastEdit, persistPtr)
				if len(payload) >= 28 {
					encID := int(binary.LittleEndian.Uint32(payload[24:28]))
					t.Logf("  encryptSessionPersistIdRef=%d", encID)
				}
			}
			if record.Type == pptRTUserEditAtom {
				offset = int(binary.LittleEndian.Uint32(payload[4:8]))
			} else {
				break
			}
		}
		return
	}

	keyMaterial, err := verifyPptPassword(encInfo)
	t.Logf("verifyPptPassword err=%v keyMaterial_len=%d", err, len(keyMaterial))
	if err != nil {
		return
	}

	// Try building persist directory with fallback
	persistDir, err := buildPptPersistDirectoryWithFallback(stream, currentEditOffset, keyMaterial, encInfo)
	t.Logf("buildPptPersistDirectoryWithFallback err=%v entries=%d", err, len(persistDir))
	for persistID, offset := range persistDir {
		if offset+8 > len(stream) {
			continue
		}
		recType := binary.LittleEndian.Uint16(stream[offset+2:])
		recLen := int(binary.LittleEndian.Uint32(stream[offset+4:]))
		if recType == pptRTDocument || recType == pptRTSlideListWithText {
			t.Logf("persistID=%d offset=%d type=0x%04X len=%d", persistID, offset, recType, recLen)
		}
	}
}