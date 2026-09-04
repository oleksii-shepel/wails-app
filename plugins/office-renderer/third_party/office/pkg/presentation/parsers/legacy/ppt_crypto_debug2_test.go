package legacy

import (
	"encoding/binary"
	"testing"

	"office/pkg/ooxml/cfb"
)

func TestDebugPptEncryptionHello(t *testing.T) {
	f, err := cfb.Open("../../../../testdata/ppt/apache-Password_Protected-hello.ppt")
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
	t.Logf("stream length: %d", len(stream))

	// Check if UserEditAtom is readable
	if currentEditOffset+8 <= len(stream) {
		recType := binary.LittleEndian.Uint16(stream[currentEditOffset+2:])
		recLen := int(binary.LittleEndian.Uint32(stream[currentEditOffset+4:]))
		t.Logf("UserEditAtom header: type=0x%04X len=%d", recType, recLen)
		if currentEditOffset+8+recLen <= len(stream) {
			payload := stream[currentEditOffset+8 : currentEditOffset+8+recLen]
			persistPtr := int(binary.LittleEndian.Uint32(payload[8:12]))
			t.Logf("persistPointersOffset: %d", persistPtr)
			if persistPtr+8 <= len(stream) {
				ptrType := binary.LittleEndian.Uint16(stream[persistPtr+2:])
				ptrLen := int(binary.LittleEndian.Uint32(stream[persistPtr+4:]))
				t.Logf("PersistPtrHolder header: type=0x%04X len=%d", ptrType, ptrLen)
			}
		}
	}

	dir := buildPptPersistDirectoryFromChain(stream, currentEditOffset)
	t.Logf("buildPptPersistDirectoryFromChain entries: %d", len(dir))
	for k, v := range dir {
		t.Logf("  persistID=%d -> offset=%d", k, v)
	}

	// Check if DocumentEncryptionAtom scan works
	encOffset, ok := scanForDocumentEncryptionAtom(stream)
	t.Logf("scanForDocumentEncryptionAtom: offset=%d ok=%v", encOffset, ok)
}