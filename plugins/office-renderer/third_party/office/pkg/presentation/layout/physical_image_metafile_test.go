package layout

import (
	"encoding/binary"
	"strings"
	"testing"

	"office/pkg/presentation/ir"
)

func TestPhysicalPresentationImageConvertsWMFToSVG(t *testing.T) {
	wmf := testPhysicalWMFWithEmbeddedEMF()
	shape := &ir.Shape{
		Kind:      ir.ShapeImage,
		Name:      "WMF picture",
		ImageData: wmf,
		ImageMIME: "image/x-wmf",
	}
	plan := resolveImageVisualPlanWithComposition(shape, PhysicalCompositionPlan{})
	if plan.MIME != "image/svg+xml" {
		t.Fatalf("mime = %q, want image/svg+xml", plan.MIME)
	}
	if !strings.Contains(string(plan.Data), "<svg") {
		t.Fatalf("artifact was not converted to SVG: %q", plan.Data)
	}
	if plan.Alt != "WMF picture" {
		t.Fatalf("alt = %q", plan.Alt)
	}
}

func testPhysicalWMFWithEmbeddedEMF() []byte {
	const emfHeader = uint32(1)
	const emfEOF = uint32(14)
	emf := make([]byte, 128)
	binary.LittleEndian.PutUint32(emf[0:4], emfHeader)
	binary.LittleEndian.PutUint32(emf[4:8], 108)
	binary.LittleEndian.PutUint32(emf[16:20], 256)
	binary.LittleEndian.PutUint32(emf[20:24], 128)
	copy(emf[40:44], []byte(" EMF"))
	binary.LittleEndian.PutUint32(emf[48:52], uint32(len(emf)))
	binary.LittleEndian.PutUint32(emf[108:112], emfEOF)
	binary.LittleEndian.PutUint32(emf[112:116], 20)

	chunk := make([]byte, 34+len(emf))
	copy(chunk[:4], []byte("WMFC"))
	copy(chunk[34:], emf)
	payload := make([]byte, 4+len(chunk))
	binary.LittleEndian.PutUint16(payload[0:2], 0x000f)
	binary.LittleEndian.PutUint16(payload[2:4], uint16(len(chunk)))
	copy(payload[4:], chunk)
	recBytes := 6 + len(payload)
	if recBytes%2 != 0 {
		payload = append(payload, 0)
		recBytes++
	}
	wmf := make([]byte, 22+18+recBytes+6)
	binary.LittleEndian.PutUint32(wmf[:4], 0x9ac6cdd7)
	off := 22
	binary.LittleEndian.PutUint16(wmf[off:off+2], 1)
	binary.LittleEndian.PutUint16(wmf[off+2:off+4], 9)
	binary.LittleEndian.PutUint16(wmf[off+4:off+6], 0x0300)
	off += 18
	binary.LittleEndian.PutUint32(wmf[off:off+4], uint32(recBytes/2))
	binary.LittleEndian.PutUint16(wmf[off+4:off+6], 0x0626)
	copy(wmf[off+6:], payload)
	off += recBytes
	binary.LittleEndian.PutUint32(wmf[off:off+4], 3)
	return wmf
}
