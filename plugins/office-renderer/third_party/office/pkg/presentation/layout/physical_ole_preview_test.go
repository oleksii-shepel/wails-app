package layout

import (
	"encoding/binary"
	"strings"
	"testing"

	"office/pkg/presentation/ir"
)

func TestPhysicalOLEPreviewNormalizesMetafileForSharedRendering(t *testing.T) {
	emf := make([]byte, 128)
	binary.LittleEndian.PutUint32(emf[0:4], 1)   // EMR_HEADER
	binary.LittleEndian.PutUint32(emf[4:8], 108) // header size
	binary.LittleEndian.PutUint32(emf[16:20], 256)
	binary.LittleEndian.PutUint32(emf[20:24], 128)
	copy(emf[40:44], []byte(" EMF"))
	binary.LittleEndian.PutUint32(emf[48:52], uint32(len(emf)))
	binary.LittleEndian.PutUint32(emf[108:112], 14) // EMR_EOF
	binary.LittleEndian.PutUint32(emf[112:116], 20)

	shape := &ir.Shape{Kind: ir.ShapeOleObject, OLEIconData: emf, OLEIconMIME: "image/x-wmf"}
	plan := ResolvePhysicalOLEPlan(shape)
	if plan == nil {
		t.Fatal("ResolvePhysicalOLEPlan returned nil")
	}
	if plan.IconMIME != "image/svg+xml" {
		t.Fatalf("IconMIME = %q, want image/svg+xml", plan.IconMIME)
	}
	if !strings.Contains(string(plan.IconData), "<svg") {
		t.Fatalf("normalized preview is not SVG: %q", string(plan.IconData))
	}
}

func TestPhysicalOLEPreviewKeepsBrowserImageUnchanged(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}
	shape := &ir.Shape{Kind: ir.ShapeOleObject, OLEIconData: png, OLEIconMIME: "image/png"}
	plan := ResolvePhysicalOLEPlan(shape)
	if plan.IconMIME != "image/png" {
		t.Fatalf("IconMIME = %q, want image/png", plan.IconMIME)
	}
	if string(plan.IconData) != string(png) {
		t.Fatalf("IconData changed for browser-native preview")
	}
}
