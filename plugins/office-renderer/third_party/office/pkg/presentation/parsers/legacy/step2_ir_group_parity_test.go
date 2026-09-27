package legacy

import (
	"encoding/binary"
	"testing"

	presentationir "office/pkg/presentation/ir"
)

func TestStep2StructuralDrawingRootDoesNotDiscardAuthoredNestedGroup(t *testing.T) {
	rect32 := func(left, top, right, bottom int32) []byte {
		buf := make([]byte, 16)
		binary.LittleEndian.PutUint32(buf[0:4], uint32(left))
		binary.LittleEndian.PutUint32(buf[4:8], uint32(top))
		binary.LittleEndian.PutUint32(buf[8:12], uint32(right))
		binary.LittleEndian.PutUint32(buf[12:16], uint32(bottom))
		return buf
	}

	clientAnchor := func(top, left, right, bottom uint16) []byte {
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint16(buf[0:2], top)
		binary.LittleEndian.PutUint16(buf[2:4], left)
		binary.LittleEndian.PutUint16(buf[4:6], right)
		binary.LittleEndian.PutUint16(buf[6:8], bottom)
		return buf
	}
	textShape := func(shapeID uint32, left, top, right, bottom int32, text string) []byte {
		textbox := append(
			buildPptAtomRecord(0, pptRTTextHeaderAtom, marshalUint32LE(pptTextTypeBody)),
			buildPptAtomRecord(0, pptRTTextBytesAtom, []byte(text))...,
		)
		return buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F, append(
			append(
				buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(shapeID, legacyOfficeArtSpFlagChild)),
				buildOfficeArtRecord(0, legacyOfficeArtChildAnchor, 0x0000, rect32(left, top, right, bottom))...,
			),
			buildOfficeArtRecord(0, legacyOfficeArtClientTextbox, 0x0000, textbox)...,
		))
	}

	// A real PPT drawing normally starts with a structural SpgrContainer whose
	// root shape establishes drawing topology but has no authored placement.
	// Nested authored groups must pass through that transparent root and reach
	// canonical IR as top-level ShapeGroup nodes.
	structuralRootShape := buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F, append(
		buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(0x900, 0x01)),
		buildOfficeArtRecord(0, legacyOfficeArtSpgr, 0x0000, rect32(0, 0, 5760, 4320))...,
	))

	authoredGroupRoot := buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F, append(
		append(
			buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(0x910, 0x03)),
			buildOfficeArtRecord(0, legacyOfficeArtSpgr, 0x0000, rect32(0, 0, 1000, 1000))...,
		),
		buildOfficeArtRecord(0, legacyOfficeArtClientAnchor, 0x0000, clientAnchor(100, 100, 2100, 1700))...,
	))
	authoredGroup := buildOfficeArtRecord(0, legacyOfficeArtSpgrContainer, 0x000F, append(
		authoredGroupRoot,
		textShape(0x911, 100, 100, 800, 600, "Nested child")...,
	))
	structuralRoot := buildOfficeArtRecord(0, legacyOfficeArtSpgrContainer, 0x000F, append(structuralRootShape, authoredGroup...))
	payload := buildPptContainerRecord(0, pptRTPPDrawing, 0x000F, structuralRoot)

	scene := collectLegacyPptVisualScene(payload)
	flat := buildLegacyPptTextShapesFromMetas(scene.textMetas(nil, nil, nil), legacySlideText{})
	groups := buildLegacyPptGroupShapesFromSceneWithOwnership(
		scene.dispatch.containers,
		legacyPptDiscoveredShapeSource(legacyPptShapeIndexByOriginalID(flat)),
		nil, nil, nil, nil,
	)
	if len(groups) != 1 {
		t.Fatalf("len(groups) = %d, want 1 authored group through structural root", len(groups))
	}
	if groups[0].Kind != presentationir.ShapeGroup || groups[0].OriginalShapeID != 0x910 {
		t.Fatalf("group = kind %v id %#x, want ShapeGroup %#x", groups[0].Kind, groups[0].OriginalShapeID, 0x910)
	}
	if len(groups[0].Children) != 1 || len(groups[0].Children[0].Paragraphs) == 0 {
		t.Fatalf("group children = %#v, want one text child", groups[0].Children)
	}
	if got := paragraphText(groups[0].Children[0].Paragraphs[0]); got != "Nested child" {
		t.Fatalf("nested child text = %q, want Nested child", got)
	}
}
