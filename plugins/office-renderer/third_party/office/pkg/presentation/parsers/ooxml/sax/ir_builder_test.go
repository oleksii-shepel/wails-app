package sax_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"office/pkg/ooxml/drawing"
	"office/pkg/presentation/ir"
	"office/pkg/presentation/parsers/ooxml/sax"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseSlidesBytesBuildsSourceOrderedNestedIR(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":             `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/><Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/></Types>`,
		"_rels/.rels":                     `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":            `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="rIdSlide1"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdSlide1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":           `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name="root"/></p:nvGrpSpPr><p:grpSpPr/><p:grpSp><p:nvGrpSpPr><p:cNvPr id="10" name="G"/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="100" y="200"/><a:ext cx="1000" cy="800"/><a:chOff x="0" y="0"/><a:chExt cx="500" cy="400"/></a:xfrm></p:grpSpPr><p:sp><p:nvSpPr><p:cNvPr id="11" name="T"/><p:nvPr><p:ph type="title" idx="2"/></p:nvPr></p:nvSpPr><p:spPr><a:xfrm rot="120000"><a:off x="5" y="6"/><a:ext cx="300" cy="100"/></a:xfrm><a:prstGeom prst="roundRect"/><a:solidFill><a:srgbClr val="AABBCC"/></a:solidFill></p:spPr><p:txBody><a:p><a:r><a:t>Hello</a:t></a:r></a:p></p:txBody></p:sp></p:grpSp><p:pic><p:nvPicPr><p:cNvPr id="12" name="P"/></p:nvPicPr><p:blipFill><a:blip r:embed="rIdImg"/><a:srcRect l="1000"/></p:blipFill><p:spPr><a:xfrm><a:off x="20" y="30"/><a:ext cx="40" cy="50"/></a:xfrm></p:spPr></p:pic></p:spTree></p:cSld></p:sld>`,
	})
	slides, err := sax.ParseSlidesBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(slides) != 1 || len(slides[0].Shapes) != 2 {
		t.Fatalf("slides/tree = %#v", slides)
	}
	g := slides[0].Shapes[0]
	if g.Kind != ir.ShapeGroup || len(g.Children) != 1 || g.GroupTransform == nil || g.GroupTransform.ChildExtent.W != 500 {
		t.Fatalf("group = %#v", g)
	}
	s := g.Children[0]
	if s.ID != 11 || s.Bounds != (ir.Rect{X: 5, Y: 6, W: 300, H: 100}) || s.RotationDeg != 2 || s.Geometry != "roundRect" || s.FillColor != "AABBCC" || s.Placeholder != ir.PlaceholderTitle {
		t.Fatalf("shape = %#v", s)
	}
	if len(s.Paragraphs) != 1 || len(s.Paragraphs[0].Runs) != 1 || s.Paragraphs[0].Runs[0].Text != "Hello" {
		t.Fatalf("text = %#v", s.Paragraphs)
	}
	p := slides[0].Shapes[1]
	if p.Kind != ir.ShapeImage || p.ImageRelID != "rIdImg" || p.ImageCropLeft != 0.01 || p.Bounds.X != 20 {
		t.Fatalf("picture = %#v", p)
	}
}

func TestParseSlidesPreservesSyntaxOnlyEmptyTextParagraph(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":             `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/></Types>`,
		"_rels/.rels":                     `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="r" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":            `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="s"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":           `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="2" name="empty geometry"/></p:nvSpPr><p:spPr><a:prstGeom prst="ellipse"/><a:solidFill><a:srgbClr val="FFFFFF"/></a:solidFill></p:spPr><p:txBody><a:bodyPr/><a:p/></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`,
	})
	slides, err := sax.ParseSlidesBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(slides) != 1 || len(slides[0].Shapes) != 1 {
		t.Fatalf("slides = %#v", slides)
	}
	if got := slides[0].Shapes[0].Paragraphs; len(got) != 1 || got[0] == nil || len(got[0].Runs) != 1 || got[0].Runs[0].Text != "" {
		t.Fatalf("syntax-only empty txBody paragraphs = %#v, want one empty paragraph-mark run", got)
	}
}

func TestParseSlidesPreservesEndParaRPrForEmptyParagraph(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":             `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/></Types>`,
		"_rels/.rels":                     `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="r" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":            `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="s"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":           `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:nvGrpSpPr/><p:grpSpPr/><p:sp><p:nvSpPr><p:cNvPr id="2" name="empty paragraph mark"/></p:nvSpPr><p:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="914400" cy="457200"/></a:xfrm><a:prstGeom prst="ellipse"/><a:solidFill><a:srgbClr val="FFFFFF"/></a:solidFill></p:spPr><p:txBody><a:bodyPr/><a:lstStyle><a:defPPr><a:defRPr sz="2800"/></a:defPPr></a:lstStyle><a:p><a:endParaRPr sz="2000"/></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`,
	})
	slides, err := sax.ParseSlidesBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(slides) != 1 {
		t.Fatalf("slides = %#v", slides)
	}
	var target *ir.Shape
	var walk func([]*ir.Shape)
	walk = func(shapes []*ir.Shape) {
		for _, shape := range shapes {
			if shape == nil || target != nil {
				continue
			}
			if len(shape.Paragraphs) != 0 {
				target = shape
				return
			}
			walk(shape.Children)
		}
	}
	walk(slides[0].Shapes)
	if target == nil {
		t.Fatalf("slides = %#v, want one paragraph-bearing shape", slides)
	}
	got := target.Paragraphs
	if len(got) != 1 || got[0] == nil || len(got[0].Runs) != 1 {
		t.Fatalf("empty paragraph = %#v, want one paragraph-mark run", got)
	}
	if gotSize := got[0].Runs[0].FontSize; gotSize != 2000 {
		t.Fatalf("paragraph-mark font size = %d, want endParaRPr size 2000", gotSize)
	}
}

func TestContextualBuilderOwnsRichRunFieldsHyperlinksAndParagraphTabs(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":              `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/></Types>`,
		"_rels/.rels":                      `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="r" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":             `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="s"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels":  `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/_rels/slide1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="web" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="https://example.test/docs" TargetMode="External"/></Relationships>`,
		"ppt/slides/slide1.xml":            `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="2" name="text"/></p:nvSpPr><p:txBody><a:bodyPr/><a:p><a:pPr rtl="1"><a:tabLst><a:tab pos="914400" algn="ctr"/></a:tabLst></a:pPr><a:r><a:rPr lang="en-US" altLang="uk-UA" sz="2400" b="1" spc="120" kern="900"><a:hlinkClick r:id="web"/><a:latin typeface="Aptos"/></a:rPr><a:t>linked</a:t></a:r><a:fld id="{1}" type="slidenum"><a:rPr i="1"/><a:t>7</a:t></a:fld></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`,
	})
	slides, err := sax.ParseSlidesBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	p := slides[0].Shapes[0].Paragraphs[0]
	if p.TextDirection != "rtl" || len(p.TabStops) != 1 || p.TabStops[0].Pos != 914400 || p.TabStops[0].Algn != "ctr" {
		t.Fatalf("paragraph semantics = %#v", p)
	}
	if len(p.Runs) != 2 {
		t.Fatalf("runs = %#v", p.Runs)
	}
	r := p.Runs[0]
	if r.Text != "linked" || r.HyperlinkURL != "https://example.test/docs" || !r.Bold || r.FontSize != 2400 || r.FontFamily != "Aptos" {
		t.Fatalf("hyperlink run = %#v", r)
	}
	f := p.Runs[1]
	if f.Text != "" || f.FieldKind != "slidenum" || !f.Italic {
		t.Fatalf("field run = %#v", f)
	}
}

func TestContextualBuilderPreservesLocalParagraphListMarkers(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":             `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/></Types>`,
		"_rels/.rels":                     `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="r" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":            `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="s"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":           `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="2" name="text"/></p:nvSpPr><p:txBody><a:bodyPr/><a:p><a:pPr marL="457200" indent="-228600"><a:lnSpc><a:spcPct val="115000"/></a:lnSpc><a:spcBef><a:spcPts val="0"/></a:spcBef><a:spcAft><a:spcPts val="0"/></a:spcAft><a:buClr><a:srgbClr val="424242"/></a:buClr><a:buSzPts val="2400"/><a:buFont typeface="Roboto"/><a:buChar char="●"/></a:pPr><a:r><a:t>Bullet</a:t></a:r></a:p><a:p><a:pPr marL="457200" indent="-228600"><a:lnSpc><a:spcPct val="115000"/></a:lnSpc><a:spcBef><a:spcPts val="0"/></a:spcBef><a:spcAft><a:spcPts val="0"/></a:spcAft><a:buClr><a:srgbClr val="424242"/></a:buClr><a:buSzPts val="2400"/><a:buFont typeface="Roboto"/><a:buAutoNum type="arabicPeriod"/></a:pPr><a:r><a:t>Number</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`,
	})
	slides, err := sax.ParseSlidesBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	paragraphs := slides[0].Shapes[0].Paragraphs
	if got := paragraphs[0].BulletChar; got != "●" {
		t.Fatalf("bullet char = %q, want black circle", got)
	}
	if p := paragraphs[1]; !p.AutoNum || p.AutoNumType != "arabicPeriod" || p.BulletChar != "" || p.BulletNone {
		t.Fatalf("auto number = %#v", p)
	}
}

func TestContextualBuilderPreservesSample1AuthoredListMarkers(t *testing.T) {
	data, err := os.ReadFile(saxRepoPath(t, "testdata", "pptx", "sample-1.pptx"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	slides, err := sax.ParseSlidesBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	var markerEvents []sax.ParagraphProperties
	if err := sax.WalkBytes(data, sax.HandlerFunc(func(e sax.Event) error {
		if e.Type == sax.EventParagraphProps && e.SlideIndex == 1 && e.ShapeID == 78 && (e.Paragraph.HasBullet || e.Paragraph.HasAutoNum) {
			markerEvents = append(markerEvents, e.Paragraph)
		}
		return nil
	}), sax.Options{}); err != nil {
		t.Fatal(err)
	}
	bullet := findSAXParagraphContaining(t, slides[1].Shapes, "Progressive enhancement")
	if bullet.BulletChar != "●" {
		t.Fatalf("sample-1 bullet paragraph marker = %#v; marker events = %#v", bullet, markerEvents)
	}
	number := findSAXParagraphContaining(t, slides[3].Shapes, "Create account")
	if !number.AutoNum || number.AutoNumType != "arabicPeriod" {
		t.Fatalf("sample-1 numbered paragraph marker = %#v", number)
	}
}

func TestContextualBuilderInheritsSample2LayoutPlaceholderTextStyles(t *testing.T) {
	data, err := os.ReadFile(saxRepoPath(t, "testdata", "pptx", "sample-2.pptx"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	slides, err := sax.ParseSlidesBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	title := findSAXParagraphContaining(t, slides[0].Shapes, "Your Name")
	if len(title.Runs) != 1 {
		t.Fatalf("title runs = %#v", title.Runs)
	}
	if got := title.Runs[0]; got.FontSize != 6000 || got.Color != "FFFFFF" {
		t.Fatalf("title run = %#v, want 60pt white", got)
	}
	subtitle := findSAXParagraphContaining(t, slides[0].Shapes, "Digital experience designer")
	if len(subtitle.Runs) != 1 {
		t.Fatalf("subtitle runs = %#v", subtitle.Runs)
	}
	if got := subtitle.Runs[0]; got.FontSize != 3600 || got.FontFamily != "Oswald" || got.Color != "424242" {
		t.Fatalf("subtitle run = %#v, want 36pt Oswald gray", got)
	}
}

func TestContextualBuilderDoesNotMaterializeUnrelatedSample3LayoutArtwork(t *testing.T) {
	data, err := os.ReadFile(saxRepoPath(t, "testdata", "pptx", "sample-3.pptx"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	slides, err := sax.ParseSlidesBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(slides) == 0 {
		t.Fatal("no slides")
	}
	var names []string
	for _, shape := range slides[0].Shapes {
		if shape != nil {
			names = append(names, shape.Name)
		}
	}

	// sample-3 slide 1 is related to slideLayout11.xml. Google Shape;10;p2
	// and Google Shape;11;p2 belong to slideLayout1.xml, which is not the
	// slide's layout. They must therefore never be projected into slide 1's
	// effective scene. The previous assertion expected these unrelated shapes
	// and turned correct relationship-scoped inheritance into a test failure.
	if containsString(names, "Google Shape;10;p2") || containsString(names, "Google Shape;11;p2") {
		t.Fatalf("slide 1 materialized artwork from unrelated layout1: %q", names)
	}
	if !containsString(names, "Google Shape;58;p13") || !containsString(names, "Google Shape;62;p13") {
		t.Fatalf("slide 1 authored scene missing expected shapes: %q", names)
	}
}

func TestContextualBuilderInheritsSample3MasterTitleHighlight(t *testing.T) {
	data, err := os.ReadFile(saxRepoPath(t, "testdata", "pptx", "sample-3.pptx"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	slides, err := sax.ParseSlidesBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(slides) == 0 {
		t.Fatal("no slides")
	}
	title := findSAXParagraphContaining(t, slides[0].Shapes, "WEST COAST")
	if len(title.Runs) == 0 {
		t.Fatalf("title runs = %#v", title.Runs)
	}
	for _, run := range title.Runs {
		if run != nil && strings.TrimSpace(run.Text) != "" && run.Highlight != "F8E71C" {
			t.Fatalf("title run highlight = %#v, want inherited master yellow highlight", run)
		}
	}
}

func TestContextualBuilderReproduces45541FooterLayoutPlaceholders(t *testing.T) {
	data, err := os.ReadFile(saxRepoPath(t, "testdata", "pptx", "45541_Footer.pptx"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	slides, err := sax.ParseSlidesBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(slides) == 0 {
		t.Fatal("no slides")
	}
	title := findSAXShapeContaining(t, slides[0].Shapes, "Removing financing obstacles")
	if title == nil || title.Placeholder != ir.PlaceholderTitle {
		t.Fatalf("title shape = %#v", title)
	}
	titlePara := findSAXParagraphContaining(t, []*ir.Shape{title}, "Removing financing obstacles")
	if len(titlePara.Runs) == 0 || titlePara.Runs[0].Color != "FFFF00" {
		t.Fatalf("title run color = %#v, want master clrMap tx1->lt1 yellow", titlePara.Runs)
	}
	wantTitle := ir.Rect{X: 685800, Y: 228600, W: 7772400, H: 1905000}
	if title.Bounds != wantTitle {
		t.Fatalf("title bounds = %#v, want layout2 inherited bounds %#v", title.Bounds, wantTitle)
	}
	centerText := findSAXParagraphContaining(t, slides[0].Shapes, "Access to Finance for Local Governments")
	if centerText.Align != "ctr" {
		t.Fatalf("center text alignment = %q, want theme object default center", centerText.Align)
	}
	var ole *ir.Shape
	var walkOLE func([]*ir.Shape)
	walkOLE = func(items []*ir.Shape) {
		for _, shape := range items {
			if shape == nil || ole != nil {
				continue
			}
			if shape.Kind == ir.ShapeOleObject {
				ole = shape
				return
			}
			walkOLE(shape.Children)
		}
	}
	walkOLE(slides[0].Shapes)
	if ole == nil || len(ole.OLEIconData) == 0 || ole.OLEIconMIME != "image/png" {
		t.Fatalf("OLE preview = %#v", ole)
	}
	body := findSAXShapeContaining(t, slides[1].Shapes, "Project preparation")
	if body == nil || body.Placeholder != ir.PlaceholderBody {
		t.Fatalf("body shape = %#v", body)
	}
	wantBody := ir.Rect{X: 685800, Y: 2133600, W: 7772400, H: 3581400}
	if body.Bounds != wantBody {
		t.Fatalf("body bounds = %#v, want authored/layout bounds %#v", body.Bounds, wantBody)
	}
	if text := collectShapeText(slides[0].Shapes); strings.Contains(text, "testdoc") || strings.Contains(text, "‹#›") {
		t.Fatalf("footer/date/slide-number template text leaked into slide scene: %q", text)
	}
}

func findSAXParagraphContaining(t *testing.T, shapes []*ir.Shape, needle string) *ir.Paragraph {
	t.Helper()
	for _, shape := range shapes {
		if shape == nil {
			continue
		}
		for _, p := range shape.Paragraphs {
			var text strings.Builder
			for _, r := range p.Runs {
				if r != nil {
					text.WriteString(r.Text)
				}
			}
			if strings.Contains(text.String(), needle) {
				return p
			}
		}
		if p := findSAXParagraphContainingInChildren(shape.Children, needle); p != nil {
			return p
		}
	}
	t.Fatalf("paragraph containing %q not found", needle)
	return nil
}

func findSAXShapeContaining(t *testing.T, shapes []*ir.Shape, needle string) *ir.Shape {
	t.Helper()
	for _, shape := range shapes {
		if shape == nil {
			continue
		}
		if strings.Contains(collectShapeText([]*ir.Shape{shape}), needle) {
			return shape
		}
		if s := findSAXShapeContainingInChildren(shape.Children, needle); s != nil {
			return s
		}
	}
	t.Fatalf("shape containing %q not found", needle)
	return nil
}

func findSAXShapeContainingInChildren(shapes []*ir.Shape, needle string) *ir.Shape {
	for _, shape := range shapes {
		if shape == nil {
			continue
		}
		if strings.Contains(collectShapeText([]*ir.Shape{shape}), needle) {
			return shape
		}
		if s := findSAXShapeContainingInChildren(shape.Children, needle); s != nil {
			return s
		}
	}
	return nil
}

func collectShapeText(shapes []*ir.Shape) string {
	var out strings.Builder
	var walk func([]*ir.Shape)
	walk = func(items []*ir.Shape) {
		for _, shape := range items {
			if shape == nil {
				continue
			}
			for _, p := range shape.Paragraphs {
				for _, r := range p.Runs {
					if r != nil {
						out.WriteString(r.Text)
					}
				}
			}
			walk(shape.Children)
		}
	}
	walk(shapes)
	return out.String()
}

func findSAXParagraphContainingInChildren(shapes []*ir.Shape, needle string) *ir.Paragraph {
	for _, shape := range shapes {
		if shape == nil {
			continue
		}
		for _, p := range shape.Paragraphs {
			var text strings.Builder
			for _, r := range p.Runs {
				if r != nil {
					text.WriteString(r.Text)
				}
			}
			if strings.Contains(text.String(), needle) {
				return p
			}
		}
		if p := findSAXParagraphContainingInChildren(shape.Children, needle); p != nil {
			return p
		}
	}
	return nil
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func saxRepoPath(t *testing.T, elems ...string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
	return filepath.Join(append([]string{root}, elems...)...)
}

func TestContextualBuilderOwnsGradientPatternAndNoFill(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":             `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/></Types>`,
		"_rels/.rels":                     `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="r" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":            `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="s"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml": `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree>
<p:sp><p:nvSpPr><p:cNvPr id="2" name="gradient"/></p:nvSpPr><p:spPr><a:gradFill rotWithShape="1"><a:gsLst><a:gs pos="0"><a:srgbClr val="FF0000"><a:alpha val="50000"/></a:srgbClr></a:gs><a:gs pos="100000"><a:srgbClr val="0000FF"/></a:gs></a:gsLst><a:lin ang="5400000"/></a:gradFill></p:spPr></p:sp>
<p:sp><p:nvSpPr><p:cNvPr id="3" name="pattern"/></p:nvSpPr><p:spPr><a:pattFill prst="smallGrid"><a:fgClr><a:srgbClr val="112233"/></a:fgClr><a:bgClr><a:srgbClr val="EEEEEE"/></a:bgClr></a:pattFill></p:spPr></p:sp>
<p:sp><p:nvSpPr><p:cNvPr id="4" name="none"/></p:nvSpPr><p:spPr><a:noFill/></p:spPr></p:sp>
</p:spTree></p:cSld></p:sld>`,
	})
	slides, err := sax.ParseSlidesBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	g, ok := slides[0].Shapes[0].Fill.(drawing.GradientFill)
	if !ok || len(g.Stops) != 2 || g.Angle != 90 || g.Stops[0].Alpha != 0.5 || g.Stops[0].Color != "FF0000" {
		t.Fatalf("gradient = %#v", slides[0].Shapes[0].Fill)
	}
	p, ok := slides[0].Shapes[1].Fill.(drawing.PatternFill)
	if !ok || p.PatternType != drawing.PatternTypeSmallGrid || p.Foreground != "112233" || p.Background != "EEEEEE" {
		t.Fatalf("pattern = %#v", slides[0].Shapes[1].Fill)
	}
	if _, ok := slides[0].Shapes[2].Fill.(drawing.NoFill); !ok {
		t.Fatalf("noFill = %#v", slides[0].Shapes[2].Fill)
	}
}

func TestContextualBuilderLoadsRelationshipPayloadsWithoutComposer(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":              `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/></Types>`,
		"_rels/.rels":                      `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="r" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":             `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="s"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels":  `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/_rels/slide1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="img" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/image1.png"/><Relationship Id="chart" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart" Target="../charts/chart1.xml"/></Relationships>`,
		"ppt/media/image1.png":             "PNGDATA",
		"ppt/charts/chart1.xml":            `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart/></c:chartSpace>`,
		"ppt/slides/slide1.xml":            `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart"><p:cSld><p:spTree><p:pic><p:nvPicPr><p:cNvPr id="2" name="picture"/></p:nvPicPr><p:blipFill><a:blip r:embed="img"/></p:blipFill><p:spPr/></p:pic><p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="3" name="chart"/></p:nvGraphicFramePr><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart r:id="chart"/></a:graphicData></a:graphic></p:graphicFrame></p:spTree></p:cSld></p:sld>`,
	})
	pres, err := sax.ParseBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(pres.Slides) != 1 || len(pres.Slides[0].Shapes) != 2 {
		t.Fatalf("tree = %#v", pres.Slides)
	}
	pic := pres.Slides[0].Shapes[0]
	if string(pic.ImageData) != "PNGDATA" || pic.ImageMIME != "image/png" {
		t.Fatalf("picture payload = %q %q", pic.ImageData, pic.ImageMIME)
	}
	chart := pres.Slides[0].Shapes[1]
	if chart.Kind != ir.ShapeChart || len(chart.ChartData) == 0 {
		t.Fatalf("chart payload = %#v", chart)
	}
}

func TestContextualBuilderResolvesScriptSpecificRunTypeface(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":             `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/></Types>`,
		"_rels/.rels":                     `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="r" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":            `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="s"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml": `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="2" name="text"/></p:nvSpPr><p:txBody><a:bodyPr/><a:p>` +
			// U+F0FC is a symbol slot the decoder leaves in place, so the run stays
			// symbol-typed and the family has to come from <a:sym>.
			`<a:r><a:rPr><a:latin typeface="Times New Roman"/><a:sym typeface="Symbol"/></a:rPr><a:t></a:t></a:r>` +
			`<a:r><a:rPr><a:sym typeface="Math1"/></a:rPr><a:t>2</a:t></a:r>` +
			`<a:r><a:rPr><a:latin typeface="Arial"/><a:sym typeface="Symbol"/></a:rPr><a:t>normal</a:t></a:r>` +
			`</a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`,
	})
	slides, err := sax.ParseSlidesBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	runs := slides[0].Shapes[0].Paragraphs[0].Runs
	if len(runs) != 3 {
		t.Fatalf("runs = %#v", runs)
	}
	if got := runs[0].FontFamily; got != "Symbol" {
		t.Fatalf("PUA symbol family = %q, want Symbol", got)
	}
	if got := runs[1].FontFamily; got != "Math1" {
		t.Fatalf("sym-only ASCII family = %q, want Math1", got)
	}
	if got := runs[2].FontFamily; got != "Arial" {
		t.Fatalf("normal latin family = %q, want Arial", got)
	}
}

func TestContextualBuilderDecodesSymbolPUAOperatorsAndDigits(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":             `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/></Types>`,
		"_rels/.rels":                     `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="r" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":            `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="s"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml": `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="2" name="text"/></p:nvSpPr><p:txBody><a:bodyPr/><a:p>` +
			`<a:r><a:rPr><a:latin typeface="Times New Roman"/><a:sym typeface="Symbol"/></a:rPr><a:t>  </a:t></a:r>` +
			`<a:r><a:rPr><a:latin typeface="Times New Roman"/><a:sym typeface="Symbol"/></a:rPr><a:t> = </a:t></a:r>` +
			`</a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`,
	})
	slides, err := sax.ParseSlidesBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	runs := slides[0].Shapes[0].Paragraphs[0].Runs
	if got := runs[0].Text; got != "Δ > 0" {
		t.Fatalf("first text = %q, want %q", got, "Δ > 0")
	}
	if got := runs[1].Text; got != "Δ = 0" {
		t.Fatalf("second text = %q, want %q", got, "Δ = 0")
	}
	for i, run := range runs {
		if run.FontFamily != "Times New Roman" {
			t.Fatalf("run %d family = %q, want Times New Roman", i, run.FontFamily)
		}
	}
}

func TestContextualBuilderMixedSymbolRunUsesSymbolOnlyForPUASlot(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":             `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/></Types>`,
		"_rels/.rels":                     `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="r" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":            `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="s"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml": `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="2" name="text"/></p:nvSpPr><p:txBody><a:bodyPr/><a:p>` +
			`<a:r><a:rPr><a:sym typeface="Symbol"/></a:rPr><a:t> and its frequency-domain equivalent</a:t></a:r>` +
			`</a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`,
	})
	slides, err := sax.ParseSlidesBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	run := slides[0].Shapes[0].Paragraphs[0].Runs[0]
	if run.Text != "… and its frequency-domain equivalent" {
		t.Fatalf("text = %q", run.Text)
	}
	if run.FontFamily == "Symbol" {
		t.Fatalf("mixed normal text retained Symbol family")
	}
}

func TestContextualBuilderFieldsInheritParagraphListStyleTypeface(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":             `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/></Types>`,
		"_rels/.rels":                     `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="r" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":            `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="s"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":           `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="2" name="field"/></p:nvSpPr><p:txBody><a:bodyPr/><a:lstStyle><a:lvl1pPr><a:defRPr sz="2400"><a:latin typeface="Times New Roman"/></a:defRPr></a:lvl1pPr></a:lstStyle><a:p><a:fld id="{1}" type="slidenum"><a:rPr sz="1200" b="1"/><a:t>1</a:t></a:fld></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`,
	})
	slides, err := sax.ParseSlidesBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	r := slides[0].Shapes[0].Paragraphs[0].Runs[0]
	if r.FieldKind != "slidenum" || r.Text != "" || r.FontFamily != "Times New Roman" || r.FontSize != 1200 || !r.Bold {
		t.Fatalf("field run = %#v", r)
	}
}

func TestPass33ContextualBuilderInheritsShapeListStyleTabStops(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":             `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/></Types>`,
		"_rels/.rels":                     `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="r" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":            `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="s"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":           `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="2" name="tabbed"/></p:nvSpPr><p:txBody><a:bodyPr/><a:lstStyle><a:lvl1pPr><a:tabLst><a:tab pos="227013" algn="l"/></a:tabLst><a:defRPr sz="2000"/></a:lvl1pPr></a:lstStyle><a:p><a:r><a:rPr sz="2000"/><a:t>\tvalue</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`,
	})
	slides, err := sax.ParseSlidesBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	p := slides[0].Shapes[0].Paragraphs[0]
	if len(p.TabStops) != 1 || p.TabStops[0].Pos != 227013 || p.TabStops[0].Algn != "l" {
		t.Fatalf("paragraph tab stops = %#v, want inherited 227013/l", p.TabStops)
	}
}

func TestContextualBuilderDropsNoPaintWhitespaceConversionArtifacts(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":             `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/></Types>`,
		"_rels/.rels":                     `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="r" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":            `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="s"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml": `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree>
			<p:sp><p:nvSpPr><p:cNvPr id="2" name="conversion spacer"/></p:nvSpPr><p:spPr><a:xfrm><a:off x="100" y="100"/><a:ext cx="20" cy="96"/></a:xfrm><a:prstGeom prst="rect"/><a:noFill/><a:ln><a:noFill/></a:ln></p:spPr><p:txBody><a:bodyPr/><a:p><a:r><a:t xml:space="preserve"> </a:t></a:r></a:p></p:txBody></p:sp>
			<p:sp><p:nvSpPr><p:cNvPr id="3" name="real text"/></p:nvSpPr><p:spPr><a:prstGeom prst="rect"/><a:noFill/><a:ln><a:noFill/></a:ln></p:spPr><p:txBody><a:bodyPr/><a:p><a:r><a:t>Visible</a:t></a:r></a:p></p:txBody></p:sp>
		</p:spTree></p:cSld></p:sld>`,
	})
	slides, err := sax.ParseSlidesBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(slides) != 1 || len(slides[0].Shapes) != 1 {
		t.Fatalf("effective shapes = %#v, want only visible text shape", slides)
	}
	if got := slides[0].Shapes[0].ID; got != 3 {
		t.Fatalf("remaining shape id = %d, want 3", got)
	}
}

func TestParseSlidesMaterializesBlipTransparentColorKey(t *testing.T) {
	var pngBuf bytes.Buffer
	img := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	img.SetNRGBA(1, 0, color.NRGBA{B: 255, A: 255})
	if err := png.Encode(&pngBuf, img); err != nil {
		t.Fatal(err)
	}
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":              `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Default Extension="png" ContentType="image/png"/></Types>`,
		"_rels/.rels":                      `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="r" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":             `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="s"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels":  `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/_rels/slide1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="img" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/image1.png"/></Relationships>`,
		"ppt/media/image1.png":             string(pngBuf.Bytes()),
		"ppt/slides/slide1.xml":            `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:cSld><p:spTree><p:pic><p:nvPicPr><p:cNvPr id="2" name="keyed"/></p:nvPicPr><p:blipFill><a:blip r:embed="img"><a:clrChange><a:clrFrom><a:srgbClr val="FF0000"/></a:clrFrom><a:clrTo><a:srgbClr val="FF0000"><a:alpha val="0"/></a:srgbClr></a:clrTo></a:clrChange></a:blip><a:srcRect l="10000" t="20000" r="30000" b="40000"/><a:stretch><a:fillRect/></a:stretch></p:blipFill><p:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="100" cy="100"/></a:xfrm></p:spPr></p:pic></p:spTree></p:cSld></p:sld>`,
	})
	slides, err := sax.ParseSlidesBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	sh := slides[0].Shapes[0]
	if sh.ImageCropLeft != .1 || sh.ImageCropTop != .2 || sh.ImageCropRight != .3 || sh.ImageCropBottom != .4 {
		t.Fatalf("crop = %v,%v,%v,%v", sh.ImageCropLeft, sh.ImageCropTop, sh.ImageCropRight, sh.ImageCropBottom)
	}
	if sh.ImageMIME != "image/png" || len(sh.ImageEffects) != 0 {
		t.Fatalf("mime/effects = %q %#v", sh.ImageMIME, sh.ImageEffects)
	}
	decoded, err := png.Decode(bytes.NewReader(sh.ImageData))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, a := decoded.At(0, 0).RGBA(); a != 0 {
		t.Fatalf("red key alpha = %d, want 0", a)
	}
	if _, _, _, a := decoded.At(1, 0).RGBA(); a == 0 {
		t.Fatal("non-keyed blue pixel became transparent")
	}
}

func Test37625PictureBulletMaterializesRelationshipPayload(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":              `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Default Extension="png" ContentType="image/png"/><Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/><Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/></Types>`,
		"_rels/.rels":                      `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":             `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="rIdSlide1"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels":  `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdSlide1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":            `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="2" name="Body"/></p:nvSpPr><p:txBody><a:bodyPr/><a:p><a:pPr marL="343080" indent="-343080"><a:buSzPct val="113071"/><a:buBlip><a:blip r:embed="rIdBullet"/></a:buBlip></a:pPr><a:r><a:rPr sz="2800"/><a:t>The policy context</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdBullet" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/bullet.png"/></Relationships>`,
		"ppt/media/bullet.png":             "PNG-BULLET",
	})
	pres, err := sax.ParseBytes(data, sax.Options{})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	p := pres.Slides[0].Shapes[0].Paragraphs[0]
	if got := string(p.BulletImageData); got != "PNG-BULLET" {
		t.Fatalf("bullet image = %q", got)
	}
	if p.BulletImageMIME != "image/png" || p.BulletSizePct != 113071 {
		t.Fatalf("bullet image props = mime %q size %d", p.BulletImageMIME, p.BulletSizePct)
	}
	if p.BulletNone || p.BulletChar != "" || p.AutoNum {
		t.Fatalf("picture bullet lost exclusive marker semantics: %#v", p)
	}
}
