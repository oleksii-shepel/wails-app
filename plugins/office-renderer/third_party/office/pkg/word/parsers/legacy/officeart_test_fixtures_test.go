package legacy

import "encoding/binary"

// legacyTestOfficeArtRecord builds one OfficeArt record from the packed
// recVer/recInstance word used by the existing legacy Word tests.
func legacyTestOfficeArtRecord(verInstance, recType uint16, payload []byte) []byte {
	out := make([]byte, 8+len(payload))
	binary.LittleEndian.PutUint16(out[0:2], verInstance)
	binary.LittleEndian.PutUint16(out[2:4], recType)
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(payload)))
	copy(out[8:], payload)
	return out
}

// legacyTestOfficeArtTextFlowSpContainer creates the smallest shape container
// needed to exercise FSP ownership plus the textFlow FOPT property.
func legacyTestOfficeArtTextFlowSpContainer(shapeID, textFlow uint32) []byte {
	fspBody := make([]byte, 8)
	binary.LittleEndian.PutUint32(fspBody[0:4], shapeID)
	fsp := legacyTestOfficeArtRecord(
		(75<<4)|0x2,
		legacyOfficeArtFSPRecord,
		fspBody,
	)

	foptBody := make([]byte, 6)
	binary.LittleEndian.PutUint16(foptBody[0:2], legacyOfficeArtTextFlowProperty)
	binary.LittleEndian.PutUint32(foptBody[2:6], textFlow)
	fopt := legacyTestOfficeArtRecord(
		(1<<4)|0x3,
		legacyOfficeArtFOPTRecord,
		foptBody,
	)

	return legacyTestOfficeArtRecord(
		0x000F,
		legacyOfficeArtSpContainer,
		append(fsp, fopt...),
	)
}
