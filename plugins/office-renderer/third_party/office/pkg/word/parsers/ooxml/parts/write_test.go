package parts

import (
	"bytes"
	"testing"

	"office/pkg/ooxml/fontdata"
	"office/pkg/ooxml/opc"
)

func TestWritePackageRoundTrip(t *testing.T) {
	pkg := &Package{
		Document: &Document{
			Body: &Body{
				Paragraphs: []*Paragraph{
					{
						Runs: []*Run{
							{
								Text: []*Text{{Value: "Hello, World!"}},
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

	if got := opcPkg.PackageType(); got != opc.PackageTypeWord {
		t.Fatalf("package type = %q, want word", got)
	}

	loaded, err := LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load package: %v", err)
	}

	if loaded.Document == nil || loaded.Document.Body == nil {
		t.Fatal("missing document body after round-trip")
	}
	if len(loaded.Document.Body.Paragraphs) != 1 {
		t.Fatalf("paragraphs = %d, want 1", len(loaded.Document.Body.Paragraphs))
	}
	if len(loaded.Document.Body.Paragraphs[0].Runs) != 1 {
		t.Fatalf("runs = %d, want 1", len(loaded.Document.Body.Paragraphs[0].Runs))
	}
	if loaded.Document.Body.Paragraphs[0].Runs[0].Text[0].Value != "Hello, World!" {
		t.Fatalf("text = %q, want Hello, World!", loaded.Document.Body.Paragraphs[0].Runs[0].Text[0].Value)
	}
}

func TestWritePackageRoundTripWithCustomXMLParts(t *testing.T) {
	pkg := &Package{
		Document: &Document{
			Body: &Body{
				Paragraphs: []*Paragraph{
					{
						Runs: []*Run{
							{
								Text: []*Text{{Value: "Hello, World!"}},
							},
						},
					},
				},
			},
		},
		CustomXMLParts: map[string][]byte{
			"{STORE-1}": []byte(`<root><customer>Bound Customer</customer></root>`),
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

	if len(loaded.CustomXMLParts) != 1 {
		t.Fatalf("custom XML parts = %d, want 1", len(loaded.CustomXMLParts))
	}
	if got := string(loaded.CustomXMLParts["{STORE-1}"]); got != `<root><customer>Bound Customer</customer></root>` {
		t.Fatalf("custom XML part = %q, want payload", got)
	}
}

func TestWritePackageRoundTripWithHeaderFooter(t *testing.T) {
	pkg := &Package{
		Document: &Document{
			Body: &Body{
				Section: &SectionProperties{
					HeaderRefs: []HeaderFooterReference{
						{Type: "default", ID: "rId10"},
					},
					FooterRefs: []HeaderFooterReference{
						{Type: "default", ID: "rId11"},
					},
				},
			},
		},
		PartRels: map[string]*Relationships{
			"word/document.xml": {
				Relationships: []*Relationship{
					{ID: "rId10", Type: relTypeHeader, Target: "header1.xml"},
					{ID: "rId11", Type: relTypeFooter, Target: "footer1.xml"},
				},
			},
		},
		Headers: map[string]*Header{
			"word/header1.xml": {
				Paragraphs: []*Paragraph{
					{Runs: []*Run{{Text: []*Text{{Value: "Header text"}}}}},
				},
			},
		},
		Footers: map[string]*Footer{
			"word/footer1.xml": {
				Paragraphs: []*Paragraph{
					{Runs: []*Run{{Text: []*Text{{Value: "Footer text"}}}}},
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

	if len(loaded.Headers) != 1 {
		t.Fatalf("headers = %d, want 1", len(loaded.Headers))
	}
	if len(loaded.Footers) != 1 {
		t.Fatalf("footers = %d, want 1", len(loaded.Footers))
	}
	mainRels := loaded.PartRels[loaded.MainPartName]
	if mainRels == nil {
		t.Fatal("missing document rels after round-trip")
	}
	var foundHeaderRel, foundFooterRel bool
	for _, r := range mainRels.Relationships {
		if r.Type == relTypeHeader && r.Target == "header1.xml" {
			foundHeaderRel = true
		}
		if r.Type == relTypeFooter && r.Target == "footer1.xml" {
			foundFooterRel = true
		}
	}
	if !foundHeaderRel {
		t.Error("header relationship missing from document.xml.rels")
	}
	if !foundFooterRel {
		t.Error("footer relationship missing from document.xml.rels")
	}
}

func TestWritePackageRoundTripWithFootnotesEndnotesComments(t *testing.T) {
	pkg := &Package{
		Document: &Document{
			Body: &Body{
				Paragraphs: []*Paragraph{
					{Runs: []*Run{{Text: []*Text{{Value: "Body"}}}}},
				},
			},
		},
		Footnotes: &FootnotesPart{
			Notes: []*Note{
				{ID: 1, Paragraphs: []*Paragraph{{Runs: []*Run{{Text: []*Text{{Value: "Footnote 1"}}}}}}},
			},
		},
		Endnotes: &EndnotesPart{
			Notes: []*Note{
				{ID: 1, Paragraphs: []*Paragraph{{Runs: []*Run{{Text: []*Text{{Value: "Endnote 1"}}}}}}},
			},
		},
		Comments: &CommentsPart{
			Comments: []*Comment{
				{ID: 1, Author: "Alice", Paragraphs: []*Paragraph{{Runs: []*Run{{Text: []*Text{{Value: "Comment 1"}}}}}}},
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

	if loaded.Footnotes == nil {
		t.Fatal("footnotes missing after round-trip")
	}
	if len(loaded.Footnotes.Notes) != 1 || loaded.Footnotes.Notes[0].Paragraphs[0].Runs[0].Text[0].Value != "Footnote 1" {
		t.Fatal("footnote content mismatch")
	}
	if loaded.Endnotes == nil {
		t.Fatal("endnotes missing after round-trip")
	}
	if len(loaded.Endnotes.Notes) != 1 || loaded.Endnotes.Notes[0].Paragraphs[0].Runs[0].Text[0].Value != "Endnote 1" {
		t.Fatal("endnote content mismatch")
	}
	if loaded.Comments == nil {
		t.Fatal("comments missing after round-trip")
	}
	if len(loaded.Comments.Comments) != 1 || loaded.Comments.Comments[0].Paragraphs[0].Runs[0].Text[0].Value != "Comment 1" {
		t.Fatal("comment content mismatch")
	}

	// Verify relationships exist.
	mainRels := loaded.PartRels[loaded.MainPartName]
	if mainRels == nil {
		t.Fatal("missing document rels")
	}
	var foundFN, foundEN, foundCM bool
	for _, r := range mainRels.Relationships {
		switch r.Type {
		case relTypeFootnotes:
			foundFN = true
		case relTypeEndnotes:
			foundEN = true
		case relTypeComments:
			foundCM = true
		}
	}
	if !foundFN {
		t.Error("footnotes relationship missing")
	}
	if !foundEN {
		t.Error("endnotes relationship missing")
	}
	if !foundCM {
		t.Error("comments relationship missing")
	}
}

func TestWritePackageRoundTripWithSmartArt(t *testing.T) {
	pkg := &Package{
		Document: &Document{
			Body: &Body{
				Paragraphs: []*Paragraph{
					{Runs: []*Run{{Text: []*Text{{Value: "Body"}}}}},
				},
			},
		},
		ScopedSmartArtParts: map[RelationshipRef][]byte{
			{SourcePart: "word/document.xml", ID: "rId20"}: []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawing/2006/diagram"><dgm:pt modelId="0" type="doc" /></dgm:dataModel>`),
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

	if len(loaded.SmartArtPartsFor(loaded.MainPartName)) != 1 {
		t.Fatalf("smartArt parts = %d, want 1", len(loaded.SmartArtPartsFor(loaded.MainPartName)))
	}
	mainRels := loaded.PartRels[loaded.MainPartName]
	if mainRels == nil {
		t.Fatal("missing document rels")
	}
	var found bool
	for _, r := range mainRels.Relationships {
		if r.Type == relTypeDiagramData {
			found = true
		}
	}
	if !found {
		t.Error("diagramData relationship missing")
	}
}

func TestWritePackageRoundTripWithEmbeddedFonts(t *testing.T) {
	pkg := &Package{
		Document: &Document{
			Body: &Body{
				Paragraphs: []*Paragraph{
					{Runs: []*Run{{Text: []*Text{{Value: "Body"}}}}},
				},
			},
		},
		FontTable: &FontTable{
			Fonts: []*Font{
				{
					Name:         "TestFont",
					EmbedRegular: &FontEmbedRef{RelID: "rId50"},
				},
			},
		},
		EmbeddedFonts: []*fontdata.EmbeddedFontData{
			{
				Typeface: "TestFont",
				Regular: &fontdata.FontPart{
					Name:        "word/fonts/testfont.odttf",
					ContentType: "application/vnd.openxmlformats-officedocument.obfuscatedFont",
					Data:        []byte("fontdata"),
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

	// Verify font part exists.
	fontPart, ok := opcPkg.Part("word/fonts/testfont.odttf")
	if !ok {
		t.Fatal("embedded font part missing")
	}
	if string(fontPart.Data) != "fontdata" {
		t.Fatalf("font data = %q, want fontdata", string(fontPart.Data))
	}

	// Verify fontTable.xml.rels exists.
	fontRelsPart, ok := opcPkg.Part("word/_rels/fontTable.xml.rels")
	if !ok {
		t.Fatal("fontTable.xml.rels missing")
	}
	if !bytes.Contains(fontRelsPart.Data, []byte(relTypeFont)) {
		t.Fatal("fontTable.xml.rels does not contain font relationship")
	}
}

func TestWritePackageRoundTripWithScopedMainPartRelationships(t *testing.T) {
	pkg := &Package{
		MainPartName: "word/document.xml",
		Document: &Document{
			Body: &Body{
				Paragraphs: []*Paragraph{
					{
						Hyperlinks: []*Hyperlink{
							{
								ID:   "rIdHlink1",
								Runs: []*Run{{Text: []*Text{{Value: "Example"}}}},
							},
						},
						Runs: []*Run{
							{DrawingXML: []byte(`<wp:inline xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"><wp:extent cx="914400" cy="914400"/><wp:docPr id="1" name="Picture 1"/><a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:pic xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:blipFill><a:blip xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:embed="rIdImage1"/></pic:blipFill></pic:pic></a:graphicData></a:graphic></wp:inline>`)},
						},
					},
				},
			},
		},
	}
	pkg.SetHyperlinkTarget(pkg.MainPartName, "rIdHlink1", "https://example.com")
	pkg.SetMediaPart(pkg.MainPartName, "rIdImage1", MediaPart{
		ContentType: "image/png",
		Data:        []byte{0x89, 0x50, 0x4e, 0x47},
	}, "media/image1.png")

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

	if got := loaded.HyperlinkTarget("word/document.xml", "rIdHlink1"); got != "https://example.com" {
		t.Fatalf("hyperlink target = %q, want https://example.com", got)
	}
	if _, ok := loaded.MediaPartFor("word/document.xml", "rIdImage1"); !ok {
		t.Fatal("scoped media relationship missing after round-trip")
	}
}

func TestWritePackageRoundTripWithScopedHeaderRelationships(t *testing.T) {
	pkg := &Package{
		MainPartName: "word/document.xml",
		Document: &Document{
			Body: &Body{
				Section: &SectionProperties{
					HeaderRefs: []HeaderFooterReference{
						{Type: "default", ID: "rIdHdr"},
					},
				},
			},
		},
		Headers: map[string]*Header{
			"word/header1.xml": {
				Paragraphs: []*Paragraph{
					{
						Hyperlinks: []*Hyperlink{
							{
								ID:   "rIdHlink1",
								Runs: []*Run{{Text: []*Text{{Value: "Header link"}}}},
							},
						},
						Runs: []*Run{
							{DrawingXML: []byte(`<wp:inline xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"><wp:extent cx="914400" cy="914400"/><wp:docPr id="1" name="Picture 1"/><a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:pic xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:blipFill><a:blip xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:embed="rIdImage1"/></pic:blipFill></pic:pic></a:graphicData></a:graphic></wp:inline>`)},
						},
					},
				},
			},
		},
	}
	pkg.UpsertRelationship(pkg.MainPartName, &Relationship{
		ID:     "rIdHdr",
		Type:   relTypeHeader,
		Target: "header1.xml",
	})
	pkg.SetHyperlinkTarget("word/header1.xml", "rIdHlink1", "https://header.example.com")
	pkg.SetMediaPart("word/header1.xml", "rIdImage1", MediaPart{
		ContentType: "image/png",
		Data:        []byte{0x89, 0x50, 0x4e, 0x47},
	}, "media/header1-rIdImage1.png")

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

	if got := loaded.HyperlinkTarget("word/header1.xml", "rIdHlink1"); got != "https://header.example.com" {
		t.Fatalf("header hyperlink target = %q, want https://header.example.com", got)
	}
	if _, ok := loaded.MediaPartFor("word/header1.xml", "rIdImage1"); !ok {
		t.Fatal("header media relationship missing after round-trip")
	}
	if _, ok := opcPkg.Part("word/_rels/header1.xml.rels"); !ok {
		t.Fatal("header rels part missing")
	}
}

func TestWritePackageRoundTripWithScopedNoteAndCommentRelationships(t *testing.T) {
	pkg := &Package{
		MainPartName:      "word/document.xml",
		FootnotesPartName: "word/footnotes.xml",
		EndnotesPartName:  "word/endnotes.xml",
		CommentsPartName:  "word/comments.xml",
		Document: &Document{
			Body: &Body{
				Paragraphs: []*Paragraph{
					{Runs: []*Run{{Text: []*Text{{Value: "Body"}}}}},
				},
			},
		},
		Footnotes: &FootnotesPart{
			Notes: []*Note{
				{
					ID: 1,
					Paragraphs: []*Paragraph{
						{
							Hyperlinks: []*Hyperlink{
								{ID: "rIdFootLink", Runs: []*Run{{Text: []*Text{{Value: "Footnote link"}}}}},
							},
						},
					},
				},
			},
		},
		Endnotes: &EndnotesPart{
			Notes: []*Note{
				{
					ID: 1,
					Paragraphs: []*Paragraph{
						{
							Hyperlinks: []*Hyperlink{
								{ID: "rIdEndLink", Runs: []*Run{{Text: []*Text{{Value: "Endnote link"}}}}},
							},
						},
					},
				},
			},
		},
		Comments: &CommentsPart{
			Comments: []*Comment{
				{
					ID: 1,
					Paragraphs: []*Paragraph{
						{
							Hyperlinks: []*Hyperlink{
								{ID: "rIdCommentLink", Runs: []*Run{{Text: []*Text{{Value: "Comment link"}}}}},
							},
						},
					},
				},
			},
		},
	}
	pkg.SetHyperlinkTarget(pkg.FootnotesPartName, "rIdFootLink", "https://footnote.example.com")
	pkg.SetHyperlinkTarget(pkg.EndnotesPartName, "rIdEndLink", "https://endnote.example.com")
	pkg.SetHyperlinkTarget(pkg.CommentsPartName, "rIdCommentLink", "https://comment.example.com")

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

	if got := loaded.HyperlinkTarget("word/footnotes.xml", "rIdFootLink"); got != "https://footnote.example.com" {
		t.Fatalf("footnote hyperlink target = %q, want https://footnote.example.com", got)
	}
	if got := loaded.HyperlinkTarget("word/endnotes.xml", "rIdEndLink"); got != "https://endnote.example.com" {
		t.Fatalf("endnote hyperlink target = %q, want https://endnote.example.com", got)
	}
	if got := loaded.HyperlinkTarget("word/comments.xml", "rIdCommentLink"); got != "https://comment.example.com" {
		t.Fatalf("comment hyperlink target = %q, want https://comment.example.com", got)
	}

	if _, ok := opcPkg.Part("word/_rels/footnotes.xml.rels"); !ok {
		t.Fatal("footnotes rels part missing")
	}
	if _, ok := opcPkg.Part("word/_rels/endnotes.xml.rels"); !ok {
		t.Fatal("endnotes rels part missing")
	}
	if _, ok := opcPkg.Part("word/_rels/comments.xml.rels"); !ok {
		t.Fatal("comments rels part missing")
	}
}
func TestWritePackagePreservesOpaquePartsAndRootRelationships(t *testing.T) {
	pkg := &Package{
		Document: &Document{Body: &Body{}},
		OpaqueParts: map[string]OpaquePart{
			"word/vendor/data.bin": {
				ContentType: "application/x-vendor-data",
				Data:        []byte{1, 2, 3, 4},
				Relationships: &Relationships{Relationships: []*Relationship{
					{ID: "rAsset", Type: "urn:vendor:asset", Target: "asset.bin"},
				}},
			},
			"word/vendor/asset.bin": {ContentType: "application/x-vendor-asset", Data: []byte{9, 8, 7}},
		},
		RootRelationships: &Relationships{Relationships: []*Relationship{
			// Deliberately collides with the generated officeDocument rId1.
			{ID: "rId1", Type: "urn:vendor:root-relation", Target: "word/vendor/data.bin"},
		}},
	}
	var buf bytes.Buffer
	if err := WritePackage(pkg, &buf); err != nil {
		t.Fatalf("write package: %v", err)
	}
	opcPkg, err := opc.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("open written package: %v", err)
	}
	part, ok := opcPkg.Part("word/vendor/data.bin")
	if !ok || !bytes.Equal(part.Data, []byte{1, 2, 3, 4}) {
		t.Fatalf("opaque part = %#v, ok=%v", part, ok)
	}
	rootRels, ok, err := opcPkg.Relationships("")
	if err != nil || !ok {
		t.Fatalf("root rels: ok=%v err=%v", ok, err)
	}
	found := false
	for _, rel := range rootRels.Relationships {
		if rel.Type == "urn:vendor:root-relation" && rel.Target == "word/vendor/data.bin" {
			found = true
			if rel.ID == "rId1" {
				t.Fatal("opaque root relationship kept a colliding generated ID")
			}
		}
	}
	if !found {
		t.Fatalf("vendor root relationship not preserved: %#v", rootRels.Relationships)
	}
	vendorRels, ok, err := opcPkg.Relationships("word/vendor/data.bin")
	if err != nil || !ok || len(vendorRels.Relationships) != 1 || vendorRels.Relationships[0].ID != "rAsset" {
		t.Fatalf("opaque relationship chain lost: ok=%v err=%v rels=%#v", ok, err, vendorRels)
	}
}
