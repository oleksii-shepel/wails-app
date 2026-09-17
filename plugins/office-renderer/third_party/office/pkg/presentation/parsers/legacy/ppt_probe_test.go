package legacy

import (
	"encoding/binary"
	"fmt"
	"strings"
	"testing"

	"office/pkg/ooxml/cfb"
)

func TestProbePowerPointGeneratedNotesDateFixture(t *testing.T) {
	path := "../../../../testdata/ppt/notes-date-fixture-20260428.ppt"
	f, err := cfb.Open(path)
	if err != nil {
		t.Fatalf("cfb.Open: %v", err)
	}
	stream, ok := f.Stream("PowerPoint Document")
	if !ok {
		t.Fatal("PowerPoint Document stream missing")
	}
	fontTable := parseLegacyPptFontTable(stream)
	colorScheme := parseLegacyPptColorScheme(stream)
	ctx, ok := extractPptPersistContext(f, stream)
	if !ok {
		t.Fatal("extractPptPersistContext = false")
	}
	_, docPayload, _, ok := readPptRecord(stream, ctx.documentOffset)
	if !ok {
		t.Fatal("read document record failed")
	}
	t.Logf("slidePersistIDs=%v notesPersistIDs=%v", ctx.slidePersistIDs, ctx.notesPersistIDs)
	for pos := 0; pos+8 <= len(docPayload); {
		record, payload, next, ok := readPptRecord(docPayload, pos)
		if !ok {
			break
		}
		if record.Type == pptRTSlideListWithText {
			t.Logf("document slide list instance=%d", record.Instance)
			dumpPptRecordTree(t, payload, 1, 3)
			if record.Instance == 1 {
				t.Logf("parsed notes slide list = %+v", parseSlideListWithText(payload, nil, nil, nil))
			}
		}
		pos = next
	}
	for _, persistID := range ctx.notesPersistIDs {
		persistOffset, ok := ctx.persistDirectory[persistID]
		if !ok {
			continue
		}
		record, payload, _, ok := readPptRecord(stream, persistOffset)
		if !ok {
			continue
		}
		texts := collectProbeTexts(payload)
		if len(texts) > 0 {
			t.Logf("notes persist id=%d type=0x%04x version=0x%x instance=%d texts=%q", persistID, record.Type, record.Version, record.Instance, strings.Join(texts, " | "))
		} else {
			t.Logf("notes persist id=%d type=0x%04x version=0x%x instance=%d", persistID, record.Type, record.Version, record.Instance)
		}
		shapes := collectLegacyPptVisualScene(payload).textShapes(legacySlideText{PersistID: persistID}, fontTable, colorScheme, nil)
		t.Logf("parsed note-persist shapes = %#v", shapes)
		if len(texts) > 0 || persistID == ctx.notesPersistIDs[0] {
			walkPptRecords(payload, func(record pptRecord, childPayload []byte) {
				switch record.Type {
				case pptRTTextHeaderAtom:
					if len(childPayload) >= 4 {
						t.Logf("walk text header instance=%d textType=%d", record.Instance, int(binary.LittleEndian.Uint32(childPayload[:4])))
					}
				case pptRTTextCharsAtom:
					if text := normalizePptText(decodePptUTF16(childPayload)); text != "" {
						t.Logf("walk text chars instance=%d text=%q", record.Instance, text)
					}
				case pptRTTextBytesAtom:
					if text := normalizePptText(decodePptBytes(childPayload)); text != "" {
						t.Logf("walk text bytes instance=%d text=%q", record.Instance, text)
					}
				case pptRTOutlineTextRefAtom:
					if len(childPayload) >= 4 {
						t.Logf("walk outline ref=%d", int(binary.LittleEndian.Uint32(childPayload[:4])))
					}
				case pptRTCString:
					if text := normalizePptText(decodePptUTF16(childPayload)); text != "" {
						t.Logf("walk cstring instance=%d text=%q", record.Instance, text)
					}
				}
			})
			dumpPptRecordTree(t, payload, 1, 2)
		}
	}
}

func TestProbeLegacyTableFixtures(t *testing.T) {
	for _, name := range []string{"table_test.ppt", "apache-table_test.ppt", "bug57820-initTableNullRefrenceException.ppt"} {
		path := "../../../../testdata/ppt/" + name
		f, err := cfb.Open(path)
		if err != nil {
			t.Fatalf("%s: cfb.Open: %v", name, err)
		}
		stream, ok := f.Stream("PowerPoint Document")
		if !ok {
			t.Fatalf("%s: PowerPoint Document stream missing", name)
		}
		fontTable := parseLegacyPptFontTable(stream)
		colorScheme := parseLegacyPptColorScheme(stream)
		ctx, ok := extractPptPersistContext(f, stream)
		if !ok {
			t.Fatalf("%s: extractPptPersistContext = false", name)
		}
		slides := extractSlideTextsFromDocument(stream, ctx.documentOffset, ctx.notesListInstance, ctx.persistDirectory, fontTable, colorScheme, nil)
		for _, slide := range slides {
			t.Logf("%s extracted slide persist=%d fragments=%d text=%q", name, slide.PersistID, len(slide.Fragments), joinSlideFragments(slide.Fragments))
			for _, frag := range slide.Fragments {
				t.Logf("%s   frag textType=%d outlineRef=%d text=%q", name, frag.TextType, frag.OutlineRef, frag.Text)
			}
		}
		t.Logf("%s slidePersistIDs=%v notesPersistIDs=%v", name, ctx.slidePersistIDs, ctx.notesPersistIDs)
		for _, persistID := range ctx.slidePersistIDs {
			offset, ok := ctx.persistDirectory[persistID]
			if !ok {
				continue
			}
			record, payload, _, ok := readPptRecord(stream, offset)
			if !ok {
				continue
			}
			t.Logf("%s persist=%d type=0x%04x version=0x%x instance=%d", name, persistID, record.Type, record.Version, record.Instance)
			textShapes := collectLegacyPptVisualScene(payload).textShapes(legacySlideText{PersistID: persistID}, fontTable, colorScheme, nil)
			tableShapes := collectLegacyPptVisualScene(payload).tables(fontTable, colorScheme, nil)
			t.Logf("%s persist=%d textShapes=%d tableShapes=%d", name, persistID, len(textShapes), len(tableShapes))
			dumpPptRecordTree(t, payload, 1, 2)
			for pos := 0; pos+8 <= len(payload); {
				child, childPayload, next, ok := readPptRecord(payload, pos)
				if !ok {
					break
				}
				if child.Type == pptRTPPDrawing {
					dumpedTextbox := 0
					walkLegacyPptOfficeArtRecords(childPayload, func(recType, inst, version uint16, body []byte) {
						if recType == legacyOfficeArtFOPT && legacyPptFOPTMarksTable(body, int(inst)) {
							t.Logf("%s persist=%d OfficeArt rec=0x%04x inst=%d version=0x%x TABLE", name, persistID, recType, inst, version)
							return
						}
						if recType == legacyOfficeArtSpgrContainer || recType == legacyOfficeArtSpContainer || recType == legacyOfficeArtSpgr || recType == legacyOfficeArtSp || recType == legacyOfficeArtFOPT || recType == legacyOfficeArtChildAnchor || recType == legacyOfficeArtClientAnchor || recType == legacyOfficeArtClientTextbox {
							t.Logf("%s persist=%d OfficeArt rec=0x%04x inst=%d version=0x%x len=%d", name, persistID, recType, inst, version, len(body))
						}
						if recType == legacyOfficeArtSpContainer {
							shapeType, outlineRef, inlineText, hasTextbox := probeLegacyPptSpContainerText(body, fontTable, colorScheme)
							if hasTextbox || shapeType == 20 {
								t.Logf("%s persist=%d SpContainer shapeType=%d outlineRef=%d inline=%q hasTextbox=%t", name, persistID, shapeType, outlineRef, inlineText, hasTextbox)
								if hasTextbox && dumpedTextbox < 3 {
									dumpedTextbox++
									dumpLegacyPptClientTextboxRecords(t, name, persistID, body)
								}
							}
						}
					})
				}
				pos = next
			}
		}
	}
}

func TestProbeLegacyExternalObjectFixtures(t *testing.T) {
	for _, name := range []string{"sound.ppt", "testPPT_oleWorkbook.ppt", "ole2-embedding-2003.ppt", "embedded_excel_ole_chart.ppt", "ppt_with_embeded.ppt"} {
		path := "../../../../testdata/ppt/" + name
		f, err := cfb.Open(path)
		if err != nil {
			t.Fatalf("%s: cfb.Open: %v", name, err)
		}
		stream, ok := f.Stream("PowerPoint Document")
		if !ok {
			t.Fatalf("%s: PowerPoint Document stream missing", name)
		}
		ctx, ok := extractPptPersistContext(f, stream)
		if !ok {
			t.Fatalf("%s: extractPptPersistContext = false", name)
		}
		record, payload, _, ok := readPptRecord(stream, ctx.documentOffset)
		if !ok || record.Type != pptRTDocument {
			t.Fatalf("%s: read document failed", name)
		}
		t.Logf("%s slidePersistIDs=%v", name, ctx.slidePersistIDs)
		for pos := 0; pos+8 <= len(payload); {
			child, childPayload, next, ok := readPptRecord(payload, pos)
			if !ok {
				break
			}
			if child.Type == pptRTExObjList {
				t.Logf("%s ExObjList len=%d", name, len(childPayload))
				dumpPptRecordTree(t, childPayload, 1, 4)
				for subPos := 0; subPos+8 <= len(childPayload); {
					sub, subPayload, subNext, subOK := readPptRecord(childPayload, subPos)
					if !subOK {
						break
					}
					if sub.Type == pptRTExOleEmbed {
						for innerPos := 0; innerPos+8 <= len(subPayload); {
							inner, innerPayload, innerNext, innerOK := readPptRecord(subPayload, innerPos)
							if !innerOK {
								break
							}
							if inner.Type == pptRTExOleObjAtom && len(innerPayload) >= 20 {
								exObjID := binary.LittleEndian.Uint32(innerPayload[8:12])
								persistID := binary.LittleEndian.Uint32(innerPayload[16:20])
								t.Logf("%s ExOleObjAtom exObjID=%d persistID=%d type=%d", name, exObjID, persistID, binary.LittleEndian.Uint32(innerPayload[4:8]))
								if offset, ok := ctx.persistDirectory[int(persistID)]; ok {
									rec, recPayload, _, ok := readPptRecord(stream, offset)
									if ok {
										t.Logf("%s persist target id=%d type=0x%04x version=0x%x instance=%d len=%d", name, persistID, rec.Type, rec.Version, rec.Instance, len(recPayload))
										t.Logf("%s persist target id=%d decoded storage len=%d", name, persistID, len(parseLegacyPptOleStorage(stream, ctx.persistDirectory, int(persistID))))
										if len(recPayload) >= 12 {
											t.Logf("%s persist target id=%d rawPrefix=% x zlibPrefix=% x", name, persistID, recPayload[:12], recPayload[4:12])
										}
									}
								}
							}
							innerPos = innerNext
						}
					}
					subPos = subNext
				}
			}
			pos = next
		}
		for _, persistID := range ctx.slidePersistIDs {
			offset, ok := ctx.persistDirectory[persistID]
			if !ok {
				continue
			}
			record, payload, _, ok := readPptRecord(stream, offset)
			if !ok {
				continue
			}
			t.Logf("%s persist=%d type=0x%04x version=0x%x instance=%d", name, persistID, record.Type, record.Version, record.Instance)
			for pos := 0; pos+8 <= len(payload); {
				child, childPayload, next, ok := readPptRecord(payload, pos)
				if !ok {
					break
				}
				if child.Type == pptRTPPDrawing {
					walkLegacyPptOfficeArtRecords(childPayload, func(recType, inst, version uint16, body []byte) {
						if recType != legacyOfficeArtSpContainer && recType != legacyOfficeArtClientData {
							return
						}
						if recType == legacyOfficeArtSpContainer {
							shapeType, _, inlineText, hasTextbox := probeLegacyPptSpContainerText(body, nil, nil)
							t.Logf("%s persist=%d SpContainer shapeType=%d hasTextbox=%t inline=%q", name, persistID, shapeType, hasTextbox, inlineText)
						}
						if recType == legacyOfficeArtClientData {
							t.Logf("%s persist=%d ClientData len=%d", name, persistID, len(body))
							dumpPptRecordTree(t, body, 2, 4)
						}
						_ = inst
						_ = version
					})
				}
				pos = next
			}
		}
	}
}

func probeLegacyPptSpContainerText(data []byte, fontTable map[uint16]string, colorScheme map[uint8]string) (uint16, int, string, bool) {
	shapeType, _ := legacyPptSpContainerShapeType(data)
	outlineRef := -1
	inlineText := ""
	hasTextbox := false
	for off := 0; off+8 <= len(data); {
		recType := binary.LittleEndian.Uint16(data[off+2 : off+4])
		recLen := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if recLen < 0 || off+8+recLen > len(data) {
			break
		}
		payload := data[off+8 : off+8+recLen]
		if recType == legacyOfficeArtClientTextbox {
			hasTextbox = true
			_, outlineRef = parseLegacyPptTextBoxFragments(payload, fontTable, colorScheme, nil)
			if inlineFrags, _ := parseLegacyPptTextBoxFragmentsRecursive(payload, fontTable, colorScheme, nil); len(inlineFrags) > 0 {
				inlineText = joinSlideFragments(legacySlideText{Fragments: inlineFrags}.Fragments)
			}
		}
		off += 8 + recLen
	}
	return shapeType, outlineRef, inlineText, hasTextbox
}

func dumpLegacyPptClientTextboxRecords(t *testing.T, name string, persistID int, data []byte) {
	t.Helper()
	for off := 0; off+8 <= len(data); {
		recType := binary.LittleEndian.Uint16(data[off+2 : off+4])
		recLen := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if recLen < 0 || off+8+recLen > len(data) {
			break
		}
		payload := data[off+8 : off+8+recLen]
		if recType == legacyOfficeArtClientTextbox {
			for pos := 0; pos+8 <= len(payload); {
				record, childPayload, next, ok := readPptRecord(payload, pos)
				if !ok {
					break
				}
				text := ""
				switch record.Type {
				case pptRTTextCharsAtom:
					text = normalizePptText(decodePptUTF16(childPayload))
				case pptRTTextBytesAtom:
					text = normalizePptText(decodePptBytes(childPayload))
				case pptRTCString:
					text = normalizePptText(decodePptUTF16(childPayload))
				}
				if text != "" {
					t.Logf("%s persist=%d ClientTextbox rec=0x%04x inst=%d version=0x%x len=%d text=%q", name, persistID, record.Type, record.Instance, record.Version, len(childPayload), text)
				} else {
					t.Logf("%s persist=%d ClientTextbox rec=0x%04x inst=%d version=0x%x len=%d", name, persistID, record.Type, record.Instance, record.Version, len(childPayload))
				}
				pos = next
			}
			return
		}
		off += 8 + recLen
	}
}

func collectProbeTexts(data []byte) []string {
	texts := make([]string, 0, 4)
	walkPptRecords(data, func(record pptRecord, childPayload []byte) {
		switch record.Type {
		case pptRTTextCharsAtom:
			if text := normalizePptText(decodePptUTF16(childPayload)); text != "" {
				texts = append(texts, text)
			}
		case pptRTTextBytesAtom:
			if text := normalizePptText(decodePptBytes(childPayload)); text != "" {
				texts = append(texts, text)
			}
		}
	})
	return texts
}

func dumpPptRecordTree(t *testing.T, data []byte, depth int, maxDepth int) {
	t.Helper()
	if depth > maxDepth {
		return
	}
	for pos := 0; pos+8 <= len(data); {
		record, payload, next, ok := readPptRecord(data, pos)
		if !ok {
			return
		}
		indent := strings.Repeat("  ", depth)
		t.Logf("%stype=0x%04x version=0x%x instance=%d len=%d", indent, record.Type, record.Version, record.Instance, len(payload))
		if record.Version == 0x0F && len(payload) > 0 {
			dumpPptRecordTree(t, payload, depth+1, maxDepth)
		} else if (record.Type == pptRTTextCharsAtom || record.Type == pptRTTextBytesAtom) && len(payload) > 0 {
			text := ""
			if record.Type == pptRTTextCharsAtom {
				text = normalizePptText(decodePptUTF16(payload))
			} else {
				text = normalizePptText(decodePptBytes(payload))
			}
			if text != "" {
				t.Logf("%stext=%q", indent+"  ", text)
			}
		} else if record.Type == pptRTTextHeaderAtom && len(payload) >= 4 {
			t.Logf("%stextType=%d", indent+"  ", int(uint32(payload[0])|uint32(payload[1])<<8|uint32(payload[2])<<16|uint32(payload[3])<<24))
		} else if record.Type == pptRTHeadersFootersAtom && len(payload) >= 4 {
			t.Logf("%sheaderFooter=%s", indent+"  ", fmt.Sprintf("format=%d flags=0x%04x", int(uint16(payload[0])|uint16(payload[1])<<8), int(uint16(payload[2])|uint16(payload[3])<<8)))
		}
		pos = next
	}
}
