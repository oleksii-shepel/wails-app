package parts

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"

	"office/pkg/ooxml/drawingxml"
	"office/pkg/ooxml/media"
	"office/pkg/ooxml/opc"
	"office/pkg/ooxml/rels"
)

func TestWritePackageRoundTrip(t *testing.T) {
	pkg := &Package{
		Presentation: &Presentation{
			SlideSize: &SlideSize{CX: 9144000, CY: 5143500},
			SlideIDs:  []IDRef{{ID: 256, RelID: "rId2"}},
		},
		Slides: []*SlideEntry{
			{
				RelID:    "rId2",
				PartName: "ppt/slides/slide1.xml",
				Slide: &Slide{
					CSld: CommonSlideData{
						SpTree: ShapeTree{
							Items: []ShapeTreeItem{
								{Kind: ShapeTreeItemShape, Index: 0},
							},
							Shapes: []Shape{
								{
									TxBody: &TextBody{
										Paragraphs: []DMLParagraph{
											{
												Inlines: []DMLParaInline{
													{Kind: DMLParaInlineRun, Run: &DMLRun{Text: "Hello, Slide!"}},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := WritePackage(pkg, &buf); err != nil {
		t.Fatalf("write package: %v", err)
	}

	// Read back.
	opcPkg, err := opc.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("open written bytes: %v", err)
	}

	if got := opcPkg.PackageType(); got != opc.PackageTypePresentation {
		t.Fatalf("package type = %q, want presentation", got)
	}

	loaded, err := LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load package: %v", err)
	}

	if len(loaded.Slides) != 1 {
		t.Fatalf("slides = %d, want 1", len(loaded.Slides))
	}

	if loaded.Slides[0].Slide == nil {
		t.Fatal("slide data missing after round-trip")
	}
	if len(loaded.Slides[0].Slide.CSld.SpTree.Shapes) != 1 {
		t.Fatalf("shapes = %d, want 1", len(loaded.Slides[0].Slide.CSld.SpTree.Shapes))
	}
	shape := loaded.Slides[0].Slide.CSld.SpTree.Shapes[0]
	if shape.TxBody == nil || len(shape.TxBody.Paragraphs) != 1 {
		t.Fatal("text body missing or empty")
	}
	if shape.TxBody.Paragraphs[0].Inlines[0].Run.Text != "Hello, Slide!" {
		t.Fatalf("text = %q, want Hello, Slide!", shape.TxBody.Paragraphs[0].Inlines[0].Run.Text)
	}
}

func TestWritePackageTransitionRoundTrip(t *testing.T) {
	pkg := &Package{
		Presentation: &Presentation{
			SlideSize: &SlideSize{CX: 9144000, CY: 5143500},
			SlideIDs:  []IDRef{{ID: 256, RelID: "rId2"}},
		},
		Slides: []*SlideEntry{
			{
				RelID:    "rId2",
				PartName: "ppt/slides/slide1.xml",
				Slide: &Slide{
					CSld: CommonSlideData{
						SpTree: ShapeTree{},
					},
					Transition: &SlideTransition{
						Type:     "fade",
						Spd:      "med",
						AdvClick: true,
						AdvTmMs:  3000,
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := WritePackage(pkg, &buf); err != nil {
		t.Fatalf("write package: %v", err)
	}

	opcPkg, err := opc.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("open written bytes: %v", err)
	}

	loaded, err := LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load package: %v", err)
	}

	if len(loaded.Slides) != 1 {
		t.Fatalf("slides = %d, want 1", len(loaded.Slides))
	}

	tr := loaded.Slides[0].Slide.Transition
	if tr == nil {
		t.Fatal("transition missing after round-trip")
	}
	if tr.Type != "fade" {
		t.Fatalf("type = %q, want fade", tr.Type)
	}
	if tr.Spd != "med" {
		t.Fatalf("spd = %q, want med", tr.Spd)
	}
	if !tr.AdvClick {
		t.Fatal("advClick = false, want true")
	}
	if tr.AdvTmMs != 3000 {
		t.Fatalf("advTmMs = %d, want 3000", tr.AdvTmMs)
	}
}

func TestWritePackageMediaRoundTrip(t *testing.T) {
	pkg := &Package{
		Presentation: &Presentation{
			SlideSize: &SlideSize{CX: 9144000, CY: 5143500},
			SlideIDs:  []IDRef{{ID: 256, RelID: "rId2"}},
		},
		Slides: []*SlideEntry{
			{
				RelID:    "rId2",
				PartName: "ppt/slides/slide1.xml",
				Slide: &Slide{
					CSld: CommonSlideData{
						SpTree: ShapeTree{},
					},
				},
				MediaParts: map[string]media.Part{
					"rIdMedia1": {
						Name:        "ppt/media/media1.mp4",
						ContentType: "video/mp4",
						Data:        []byte{0x00, 0x00, 0x00, 0x18},
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := WritePackage(pkg, &buf); err != nil {
		t.Fatalf("write package: %v", err)
	}

	opcPkg, err := opc.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("open written bytes: %v", err)
	}

	loaded, err := LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load package: %v", err)
	}

	if len(loaded.Slides) != 1 {
		t.Fatalf("slides = %d, want 1", len(loaded.Slides))
	}

	mp, ok := loaded.Slides[0].MediaParts["rIdMedia1"]
	if !ok {
		t.Fatal("media part rIdMedia1 missing after round-trip")
	}
	if mp.ContentType != "video/mp4" {
		t.Fatalf("content type = %q, want video/mp4", mp.ContentType)
	}
	if len(mp.Data) != 4 {
		t.Fatalf("data length = %d, want 4", len(mp.Data))
	}
}

func TestWritePackageMediaRelTypes(t *testing.T) {
	pkg := &Package{
		Presentation: &Presentation{
			SlideSize: &SlideSize{CX: 9144000, CY: 5143500},
			SlideIDs:  []IDRef{{ID: 256, RelID: "rId2"}},
		},
		Slides: []*SlideEntry{
			{
				RelID:    "rId2",
				PartName: "ppt/slides/slide1.xml",
				Slide: &Slide{
					CSld: CommonSlideData{SpTree: ShapeTree{}},
				},
				MediaParts: map[string]media.Part{
					"rIdAudio1": {Name: "ppt/media/media1.mp3", ContentType: "audio/mpeg", Data: []byte{0xFF}},
					"rIdVideo1": {Name: "ppt/media/media2.mp4", ContentType: "video/mp4", Data: []byte{0x00}},
					"rIdImage1": {Name: "ppt/media/image1.png", ContentType: "image/png", Data: []byte{0x89}},
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := WritePackage(pkg, &buf); err != nil {
		t.Fatalf("write package: %v", err)
	}

	opcPkg, err := opc.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("open written bytes: %v", err)
	}

	relsDoc, ok, err := opcPkg.Relationships("ppt/slides/slide1.xml")
	if err != nil {
		t.Fatalf("slide rels: %v", err)
	}
	if !ok {
		t.Fatal("slide rels missing")
	}

	wantTypes := map[string]string{
		"rIdAudio1": relTypeAudio,
		"rIdVideo1": relTypeVideo,
		"rIdImage1": relTypeImage,
	}
	for _, r := range relsDoc.Relationships {
		want, ok := wantTypes[r.ID]
		if !ok {
			continue
		}
		if r.Type != want {
			t.Errorf("rel %s type = %q, want %q", r.ID, r.Type, want)
		}
	}
}

func TestWritePackageOleObjectRoundTrip(t *testing.T) {
	pkg := &Package{
		Presentation: &Presentation{
			SlideSize: &SlideSize{CX: 9144000, CY: 5143500},
			SlideIDs:  []IDRef{{ID: 256, RelID: "rId2"}},
		},
		Slides: []*SlideEntry{
			{
				RelID:    "rId2",
				PartName: "ppt/slides/slide1.xml",
				Slide: &Slide{
					CSld: CommonSlideData{
						SpTree: ShapeTree{
							Items: []ShapeTreeItem{{Kind: ShapeTreeItemOleObject, Index: 0}},
							OleObjectFrames: []OleObjectFrame{{
								ID:         4,
								Name:       "Object 1",
								Xfrm:       &Transform2D{Off: Offset{X: 1000, Y: 2000}, Ext: Extents{CX: 3000, CY: 4000}},
								RelID:      "rIdOle1",
								Embed:      true,
								ShowAsIcon: true,
								ProgID:     "Word.Document.12",
								ObjectName: "Document",
								ImgW:       3000,
								ImgH:       4000,
								IconRelID:  "rIdIcon1",
							}},
						},
					},
				},
				OleObjectParts: map[string]media.Part{
					"rIdOle1": {
						Name:        "ppt/embeddings/oleObject1.bin",
						ContentType: "application/vnd.openxmlformats-officedocument.oleObject",
						Data:        []byte("OLE payload"),
					},
				},
				MediaParts: map[string]media.Part{
					"rIdIcon1": {
						Name:        "ppt/media/image1.png",
						ContentType: "image/png",
						Data:        []byte("PNG icon"),
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := WritePackage(pkg, &buf); err != nil {
		t.Fatalf("write package: %v", err)
	}

	opcPkg, err := opc.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("open written bytes: %v", err)
	}
	loaded, err := LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load package: %v", err)
	}
	if len(loaded.Slides) != 1 {
		t.Fatalf("slides = %d, want 1", len(loaded.Slides))
	}
	slide := loaded.Slides[0]
	if len(slide.Slide.CSld.SpTree.OleObjectFrames) != 1 {
		t.Fatalf("ole object frames = %d, want 1", len(slide.Slide.CSld.SpTree.OleObjectFrames))
	}
	frame := slide.Slide.CSld.SpTree.OleObjectFrames[0]
	if frame.RelID != "rIdOle1" {
		t.Fatalf("frame rel id = %q, want rIdOle1", frame.RelID)
	}
	if frame.IconRelID != "rIdIcon1" {
		t.Fatalf("frame icon rel id = %q, want rIdIcon1", frame.IconRelID)
	}
	if got := string(slide.OleObjectParts["rIdOle1"].Data); got != "OLE payload" {
		t.Fatalf("ole payload = %q, want OLE payload", got)
	}
	if got := string(slide.MediaParts["rIdIcon1"].Data); got != "PNG icon" {
		t.Fatalf("icon payload = %q, want PNG icon", got)
	}
}
func TestWritePackagePreservesOpaquePartsAndRelationships(t *testing.T) {
	base := &Package{
		Presentation: &Presentation{SlideSize: &SlideSize{CX: 9144000, CY: 5143500}},
		Slides:       []*SlideEntry{{PartName: "ppt/slides/slide1.xml", Slide: &Slide{CSld: CommonSlideData{SpTree: ShapeTree{}}}}},
	}
	var first bytes.Buffer
	if err := WritePackage(base, &first); err != nil {
		t.Fatalf("write base package: %v", err)
	}
	op, err := opc.OpenBytes(first.Bytes())
	if err != nil {
		t.Fatalf("open base: %v", err)
	}
	const customName = "ppt/customXml/vendor1.xml"
	const customData = `<vendor:payload xmlns:vendor="urn:vendor:test"><vendor:value>keep-me</vendor:value></vendor:payload>`
	op.AddPart(customName, "application/xml", []byte(customData))

	relsPartName := "ppt/slides/_rels/slide1.xml.rels"
	relDoc := rels.Document{Relationships: []rels.Relationship{{
		ID: "rId999", Type: "urn:vendor:test/relationship", Target: "../customXml/vendor1.xml",
	}}}
	relsData, err := xml.Marshal(relDoc)
	if err != nil {
		t.Fatalf("marshal slide rels: %v", err)
	}
	op.AddPart(relsPartName, "application/vnd.openxmlformats-package.relationships+xml", relsData)
	modified, err := op.WriteToBytes()
	if err != nil {
		t.Fatalf("write modified source: %v", err)
	}

	source, err := opc.OpenBytes(modified)
	if err != nil {
		t.Fatalf("reopen modified source: %v", err)
	}
	loaded, err := LoadPackage(source)
	if err != nil {
		t.Fatalf("load package: %v", err)
	}
	var second bytes.Buffer
	if err := WritePackage(loaded, &second); err != nil {
		t.Fatalf("round-trip write: %v", err)
	}
	out, err := opc.OpenBytes(second.Bytes())
	if err != nil {
		t.Fatalf("open round-trip output: %v", err)
	}
	custom, ok := out.Part(customName)
	if !ok || string(custom.Data) != customData {
		t.Fatalf("opaque custom part not preserved: ok=%v data=%q", ok, custom.Data)
	}
	gotRels, ok, err := out.Relationships("ppt/slides/slide1.xml")
	if err != nil || !ok {
		t.Fatalf("read output slide rels: ok=%v err=%v", ok, err)
	}
	found := false
	for _, rel := range gotRels.Relationships {
		if rel.ID == "rId999" && rel.Type == "urn:vendor:test/relationship" && rel.Target == "../customXml/vendor1.xml" {
			found = true
		}
	}
	if !found {
		t.Fatalf("opaque relationship not preserved: %#v", gotRels.Relationships)
	}
}

func TestSlideRoundTripPreservesColorMapExtensionsAndMasterFlags(t *testing.T) {
	xmlData := []byte(`<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" show="1" showMasterSp="0" showMasterPhAnim="1"><p:cSld><p:spTree/></p:cSld><p:clrMapOvr><a:overrideClrMapping accent1="accent2"/></p:clrMapOvr><p:extLst><p:ext uri="{VENDOR}"><x:feature xmlns:x="urn:vendor:test" enabled="1"/></p:ext></p:extLst></p:sld>`)
	var slide Slide
	if err := xml.Unmarshal(xmlData, &slide); err != nil {
		t.Fatalf("unmarshal slide: %v", err)
	}
	if slide.ClrMapOvr == nil || slide.ShowMasterSp == nil || *slide.ShowMasterSp || slide.ShowMasterPhAnim == nil || !*slide.ShowMasterPhAnim {
		t.Fatalf("slide metadata not parsed: %+v", slide)
	}
	if len(slide.ExtLstRawXML) == 0 {
		t.Fatal("expected extLst raw XML")
	}
	encoded, err := xml.Marshal(slide)
	if err != nil {
		t.Fatalf("marshal slide: %v", err)
	}
	out := string(encoded)
	for _, want := range []string{"clrMapOvr", "overrideClrMapping", `showMasterSp="0"`, `showMasterPhAnim="1"`, "{VENDOR}", "urn:vendor:test"} {
		if !strings.Contains(out, want) {
			t.Fatalf("round-trip output missing %q: %s", want, out)
		}
	}
}

func TestEffectDagMarshalRoundTripPreservesNestedContainers(t *testing.T) {
	input := []byte(`<a:effectDag xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="root" type="tree"><a:outerShdw blurRad="12700" dist="25400" dir="5400000"><a:srgbClr val="112233"/></a:outerShdw><a:cont name="nested" type="sib"><a:blur rad="38100" grow="0"/><a:reflection blurRad="12700" stA="60000" endA="0"/></a:cont></a:effectDag>`)
	var dag EffectDag
	if err := xml.Unmarshal(input, &dag); err != nil {
		t.Fatalf("unmarshal effectDag: %v", err)
	}
	encoded, err := xml.Marshal(dag)
	if err != nil {
		t.Fatalf("marshal effectDag: %v", err)
	}
	out := string(encoded)
	for _, want := range []string{`name="root"`, `type="tree"`, "outerShdw", `name="nested"`, `type="sib"`, "blur", `grow="false"`, "reflection"} {
		if !strings.Contains(out, want) {
			t.Fatalf("effectDag output missing %q: %s", want, out)
		}
	}
	var got EffectDag
	if err := xml.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("reparse effectDag: %v", err)
	}
	if len(got.Items) != 2 || got.Items[1].Container == nil || len(got.Items[1].Container.Items) != 2 {
		t.Fatalf("nested effectDag structure lost: %+v", got)
	}
}

func TestBlipFillRoundTripPreservesGeometryAndEffects(t *testing.T) {
	rot := false
	fill := BlipFill{
		Dpi:          144,
		RotWithShape: &rot,
		SrcRect:      &SrcRect{L: 1000, T: 2000, R: 3000, B: 4000},
		Stretch:      &drawingxml.Stretch{FillRect: &drawingxml.FillRect{L: 5000, T: 6000, R: 7000, B: 8000}},
		Blip: &Blip{
			Embed:        "rIdImage1",
			Grayscl:      &struct{}{},
			AlphaBiLevel: &BlipBiLevel{Thresh: 45000},
			HSL:          &BlipHSL{Hue: 60000, Sat: 80000, Lum: 90000},
			ClrRepl:      &BlipColorChoice{SrgbClr: &EffectColorRef{Val: "AABBCC"}},
		},
	}
	encoded, err := xml.Marshal(fill)
	if err != nil {
		t.Fatalf("marshal blipFill: %v", err)
	}
	out := string(encoded)
	for _, want := range []string{`dpi="144"`, `rotWithShape="false"`, "srcRect", "fillRect", "grayscl", "alphaBiLevel", "hsl", "clrRepl"} {
		if !strings.Contains(out, want) {
			t.Fatalf("blipFill output missing %q: %s", want, out)
		}
	}
	var got BlipFill
	if err := xml.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("reparse blipFill: %v", err)
	}
	if got.Dpi != 144 || got.RotWithShape == nil || *got.RotWithShape || got.SrcRect == nil || got.Stretch == nil || got.Stretch.FillRect == nil {
		t.Fatalf("picture geometry metadata lost: %+v", got)
	}
	if got.Blip == nil || got.Blip.AlphaBiLevel == nil || got.Blip.HSL == nil || got.Blip.ClrRepl == nil {
		t.Fatalf("picture effects lost: %+v", got.Blip)
	}
}

func TestWritePackagePrunesRemovedManagedPartsButKeepsOpaqueReferences(t *testing.T) {
	base := &Package{
		Presentation: &Presentation{SlideSize: &SlideSize{CX: 9144000, CY: 5143500}},
		Slides: []*SlideEntry{{
			PartName: "ppt/slides/slide1.xml",
			Slide: &Slide{CSld: CommonSlideData{SpTree: ShapeTree{
				Items:    []ShapeTreeItem{{Kind: ShapeTreeItemPicture, Index: 0}},
				Pictures: []Picture{{BlipFill: BlipFill{Blip: &Blip{Embed: "rIdImage1"}}}},
			}}},
			MediaParts: map[string]media.Part{"rIdImage1": {
				Name: "ppt/media/image1.png", ContentType: "image/png", Data: []byte{1, 2, 3},
			}},
		}},
	}
	var first bytes.Buffer
	if err := WritePackage(base, &first); err != nil {
		t.Fatalf("write base: %v", err)
	}
	op, err := opc.OpenBytes(first.Bytes())
	if err != nil {
		t.Fatalf("open base: %v", err)
	}
	const customName = "ppt/customXml/vendor-cleanup.xml"
	op.AddPart(customName, "application/xml", []byte(`<vendor xmlns="urn:test">keep</vendor>`))
	doc, ok, err := op.Relationships("ppt/slides/slide1.xml")
	if err != nil || !ok {
		t.Fatalf("slide rels: ok=%v err=%v", ok, err)
	}
	doc.Relationships = append(doc.Relationships, rels.Relationship{
		ID: "rId999", Type: "urn:vendor:relationship", Target: "../customXml/vendor-cleanup.xml",
	})
	relsData, err := xml.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal rels: %v", err)
	}
	op.AddPart("ppt/slides/_rels/slide1.xml.rels", "application/vnd.openxmlformats-package.relationships+xml", relsData)
	modified, err := op.WriteToBytes()
	if err != nil {
		t.Fatalf("write source: %v", err)
	}
	source, err := opc.OpenBytes(modified)
	if err != nil {
		t.Fatalf("reopen source: %v", err)
	}
	loaded, err := LoadPackage(source)
	if err != nil {
		t.Fatalf("load source: %v", err)
	}
	// Simulate an edit that removes the only picture and its modeled relationship.
	loaded.Slides[0].Slide.CSld.SpTree = ShapeTree{}
	loaded.Slides[0].MediaParts = nil

	var outBuf bytes.Buffer
	if err := WritePackage(loaded, &outBuf); err != nil {
		t.Fatalf("write edited: %v", err)
	}
	out, err := opc.OpenBytes(outBuf.Bytes())
	if err != nil {
		t.Fatalf("open edited: %v", err)
	}
	if _, ok := out.Part("ppt/media/image1.png"); ok {
		t.Fatal("removed image part was retained")
	}
	if _, ok := out.Part(customName); !ok {
		t.Fatal("opaque vendor part was pruned")
	}
	gotRels, ok, err := out.Relationships("ppt/slides/slide1.xml")
	if err != nil || !ok {
		t.Fatalf("output rels: ok=%v err=%v", ok, err)
	}
	var vendorFound, imageFound bool
	for _, rel := range gotRels.Relationships {
		vendorFound = vendorFound || rel.Type == "urn:vendor:relationship"
		imageFound = imageFound || relationshipTypeIs(rel.Type, relTypeImage)
	}
	if !vendorFound {
		t.Fatal("opaque vendor relationship was not preserved")
	}
	if imageFound {
		t.Fatal("removed image relationship was resurrected")
	}
}

func TestWritePackageLinkedOLEObjectRoundTrip(t *testing.T) {
	pkg := &Package{
		Presentation: &Presentation{SlideSize: &SlideSize{CX: 9144000, CY: 5143500}},
		Slides: []*SlideEntry{{
			PartName: "ppt/slides/slide1.xml",
			Slide: &Slide{CSld: CommonSlideData{SpTree: ShapeTree{
				Items:           []ShapeTreeItem{{Kind: ShapeTreeItemOleObject, Index: 0}},
				OleObjectFrames: []OleObjectFrame{{ID: 2, Name: "Object", RelID: "rIdOle", Embed: false, ProgID: "Excel.Sheet.12", ObjectName: "Linked"}},
			}}},
			OleObjectLinks: map[string]string{"rIdOle": "https://example.com/report.xlsx"},
		}},
	}
	var buf bytes.Buffer
	if err := WritePackage(pkg, &buf); err != nil {
		t.Fatalf("write: %v", err)
	}
	opcPkg, err := opc.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	loaded, err := LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := loaded.Slides[0].OleObjectLinks["rIdOle"]; got != "https://example.com/report.xlsx" {
		t.Fatalf("linked OLE target = %q", got)
	}
	if loaded.Slides[0].Slide.CSld.SpTree.OleObjectFrames[0].Embed {
		t.Fatal("linked OLE object became embedded")
	}
}
