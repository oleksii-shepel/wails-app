package legacy

import (
	"encoding/binary"
	"testing"

	"office/pkg/officeart/legacy/escher"
	sheetir "office/pkg/sheet/ir"
)

// TestLegacyOfficeArtMissingBytes pins the container bookkeeping that decides
// whether a CONTINUE payload continues the open OfficeArt record.
func TestLegacyOfficeArtMissingBytes(t *testing.T) {
	record := buildLegacyOfficeArtRecordForPictureTest(0, 0xF004, uint16(escher.ContainerVersion), make([]byte, 40))
	if got := legacyOfficeArtMissingBytes(record); got != 0 {
		t.Fatalf("complete record missing bytes = %d, want 0", got)
	}
	if got := legacyOfficeArtMissingBytes(record[:len(record)-15]); got != 15 {
		t.Fatalf("truncated record missing bytes = %d, want 15", got)
	}
	smaller := buildLegacyOfficeArtRecordForPictureTest(0, 0xF00D, 0, make([]byte, 8))
	stream := append(append([]byte(nil), smaller...), record...)
	if got := legacyOfficeArtMissingBytes(stream); got != 0 {
		t.Fatalf("complete stream missing bytes = %d, want 0", got)
	}
	if got := legacyOfficeArtMissingBytes(stream[:len(stream)-7]); got != 7 {
		t.Fatalf("stream with mid-record tail missing bytes = %d, want 7", got)
	}
}

// TestParseWorksheetRecoversFragmentedOfficeArtPicture mirrors the drawing group
// layout in testdata/xls/apache-ConditionalFormattingSamples.xls: Excel caps
// MSODRAWING payloads and resumes a large container mid-record, interleaving the
// fragments with the OBJ/TXO records that own the preceding shapes. The resumed
// fragments carry no OfficeArt header, so only the open container's outstanding
// byte count identifies them; the TXO text and run payloads between them must
// stay with the TXO record.
func TestParseWorksheetRecoversFragmentedOfficeArtPicture(t *testing.T) {
	const (
		blipRef            = 7
		drawingTextObjType = 0x001E // Excel's object type for drawing text shapes
	)
	picture := fragmentedPictureSpContainerPayload(blipRef, 3, 6)
	group := buildLegacyOfficeArtRecordForPictureTest(0, 0xF002, uint16(escher.ContainerVersion), picture)
	if len(group) < 200 {
		t.Fatalf("test drawing group too small to fragment: %d bytes", len(group))
	}
	split := len(group) - 149 // Excel's trailing fragment is not header-aligned
	head, tail := group[:split], group[split:]
	if isLegacyOfficeArtContinuation(tail) {
		t.Fatal("tail fragment must resume mid-record, but it carries an OfficeArt header")
	}

	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	// A three-column table (A..C) so the picture anchor below lands beside it.
	for col := 0; col < 3; col++ {
		data = appendBiffRecord(data, biffLabelSst, fragmentedLabelSstPayload(0, col))
	}
	// The drawing group opens here and stays incomplete for the rest of the sheet.
	data = appendBiffRecord(data, biffMsoDrawing, head)
	// The drawing text object and its CONTINUE payloads sit between the fragments.
	data = appendBiffRecord(data, biffObj, objPayload(drawingTextObjType, 1))
	data = appendBiffRecord(data, biffTxO, txoPayload(4))
	data = appendBiffRecord(data, biffContinue, append([]byte{0x00}, []byte("Pane")...))
	data = appendBiffRecord(data, biffContinue, make([]byte, 16))
	data = appendBiffRecord(data, biffContinue, tail)
	data = appendBiffRecord(data, biffEOF, nil)

	store := map[uint32]*sheetir.DrawingImage{blipRef: {Data: []byte("png"), ContentType: "image/png"}}
	sst := []legacySharedString{{Text: "Region"}}
	sheet := parseBIFFWorksheet(data, boundSheet{offset: 0, name: "Panels"}, sst, nil, store, legacyFormulaWorkbookContext{})

	if len(sheet.Drawings) != 1 {
		t.Fatalf("drawings = %d, want the fragmented picture recovered", len(sheet.Drawings))
	}
	drawing := sheet.Drawings[0]
	if drawing.Type != sheetir.DrawingTypeImage || drawing.Image == nil {
		t.Fatalf("drawing = %#v, want image drawing", drawing)
	}
	if got := string(drawing.Image.Data); got != "png" {
		t.Fatalf("image payload = %q, want the blip from the drawing store", got)
	}
	// Column D (1-based 4) sits immediately right of the three-column table.
	if got := drawing.Position.FromCell.Col; got != 4 {
		t.Fatalf("picture from column = %d, want 4 beside the three-column table", got)
	}
	if got := drawing.Position.FromCell.Row; got != 2 {
		t.Fatalf("picture from row = %d, want 2", got)
	}
	if drawing.Position.ToCell.Col != 7 || drawing.Position.ToCell.Row != 21 {
		t.Fatalf("picture to cell = r%d c%d, want r21 c7", drawing.Position.ToCell.Row, drawing.Position.ToCell.Col)
	}
}

// fragmentedPictureSpContainerPayload builds a picture SpContainer with a blip
// reference and a two-cell anchor, padded so a caller can split it across
// MSODRAWING and CONTINUE records.
func fragmentedPictureSpContainerPayload(pib uint32, fromCol, toCol uint16) []byte {
	sp := make([]byte, 8)
	spRec := buildLegacyOfficeArtRecordForPictureTest(75, 0xF00A, 2, sp)
	props := []escher.Property{{ID: 0x0104, Value: pib}} // pib: blip index
	fopt := make([]byte, len(props)*6)
	for i, p := range props {
		binary.LittleEndian.PutUint16(fopt[i*6:i*6+2], p.ID)
		binary.LittleEndian.PutUint32(fopt[i*6+2:i*6+6], p.Value)
	}
	foptRec := buildLegacyOfficeArtRecordForPictureTest(uint16(len(props)), 0xF00B, 3, fopt)
	anchor := make([]byte, 18)
	// Two-cell client anchor: col/row of the top-left and bottom-right corners.
	binary.LittleEndian.PutUint16(anchor[2:4], fromCol)
	binary.LittleEndian.PutUint16(anchor[6:8], 1)
	binary.LittleEndian.PutUint16(anchor[10:12], toCol)
	binary.LittleEndian.PutUint16(anchor[14:16], 20)
	anchorRec := buildLegacyOfficeArtRecordForPictureTest(0, 0xF010, 0, anchor)
	textRec := buildLegacyOfficeArtRecordForPictureTest(0, 0xF00D, 0, make([]byte, 220))
	body := append([]byte(nil), spRec...)
	body = append(body, foptRec...)
	body = append(body, anchorRec...)
	body = append(body, textRec...)
	return buildLegacyOfficeArtRecordForPictureTest(0, 0xF004, uint16(escher.ContainerVersion), body)
}

func fragmentedLabelSstPayload(row, col int) []byte {
	payload := make([]byte, 10)
	binary.LittleEndian.PutUint16(payload[0:2], uint16(row))
	binary.LittleEndian.PutUint16(payload[2:4], uint16(col))
	binary.LittleEndian.PutUint32(payload[6:10], 0)
	return payload
}
