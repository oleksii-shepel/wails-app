package sax_test

import (
	"office/pkg/presentation/ir"
	prel "office/pkg/presentation/parsers/ooxml/relationship"
	"office/pkg/presentation/parsers/ooxml/sax"
	"testing"
)

func TestWalkBytesEmitsSlideContextBeforeAuthoredVisualEvents(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":                          `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/></Types>`,
		"_rels/.rels":                                  `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":                         `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="rIdSlide1"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels":              `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdSlide1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":                        `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="2" name="shape"/></p:nvSpPr></p:sp></p:spTree></p:cSld></p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels":             `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdLayout" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/><Relationship Id="rIdImage" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/image1.png"/></Relationships>`,
		"ppt/slideLayouts/slideLayout1.xml":            `<p:sldLayout xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"/>`,
		"ppt/slideLayouts/_rels/slideLayout1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdMaster" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/></Relationships>`,
		"ppt/slideMasters/slideMaster1.xml":            `<p:sldMaster xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"/>`,
		"ppt/slideMasters/_rels/slideMaster1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdTheme" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="../theme/theme1.xml"/></Relationships>`,
		"ppt/theme/theme1.xml":                         `<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"/>`,
	})
	var events []sax.Event
	err := sax.WalkBytes(data, sax.HandlerFunc(func(e sax.Event) error { events = append(events, e); return nil }), sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	var start *sax.Event
	relBeforeShape := false
	shapeSeen := false
	for i := range events {
		e := &events[i]
		if e.Type == sax.EventStartSlide {
			start = e
		}
		if e.Type == sax.EventStartShape {
			shapeSeen = true
		}
		if e.Type == sax.EventRelationship && e.RelID == "rIdImage" {
			if shapeSeen {
				t.Fatal("relationship context emitted after shape stream began")
			}
			relBeforeShape = e.RelType == prel.Image && e.TargetPart == "ppt/media/image1.png"
		}
	}
	if start == nil {
		t.Fatal("missing slide start")
	}
	if start.LayoutPart != "ppt/slidelayouts/slidelayout1.xml" || start.MasterPart != "ppt/slidemasters/slidemaster1.xml" || start.ThemePart != "ppt/theme/theme1.xml" {
		t.Fatalf("context = layout %q master %q theme %q", start.LayoutPart, start.MasterPart, start.ThemePart)
	}
	if !relBeforeShape {
		t.Fatal("image relationship was not resolved before authored shape events")
	}
}

func TestContextualBuilderResolvesSchemeColorFromStreamedTheme(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":                          `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/></Types>`,
		"_rels/.rels":                                  `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":                         `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="s1"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels":              `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="s1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":                        `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="2" name="accent"/></p:nvSpPr><p:spPr><a:solidFill><a:schemeClr val="accent1"/></a:solidFill></p:spPr></p:sp></p:spTree></p:cSld></p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels":             `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="l1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/></Relationships>`,
		"ppt/slideLayouts/slideLayout1.xml":            `<p:sldLayout xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"/>`,
		"ppt/slideLayouts/_rels/slideLayout1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="m1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/></Relationships>`,
		"ppt/slideMasters/slideMaster1.xml":            `<p:sldMaster xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"/>`,
		"ppt/slideMasters/_rels/slideMaster1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="t1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="../theme/theme1.xml"/></Relationships>`,
		"ppt/theme/theme1.xml":                         `<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:themeElements><a:clrScheme name="x"><a:dk1><a:srgbClr val="000000"/></a:dk1><a:lt1><a:srgbClr val="FFFFFF"/></a:lt1><a:accent1><a:srgbClr val="336699"/></a:accent1></a:clrScheme></a:themeElements></a:theme>`,
	})
	slides, err := sax.BuildContextualSlidesBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(slides) != 1 || len(slides[0].Shapes) != 1 {
		t.Fatalf("slides = %#v", slides)
	}
	if got := slides[0].Shapes[0].FillColor; got != "336699" {
		t.Fatalf("scheme fill = %q, want 336699", got)
	}
}

func TestContextualBuilderInheritsLayoutThenMasterPlaceholderVisuals(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":                          `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/></Types>`,
		"_rels/.rels":                                  `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="r" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":                         `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="s"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels":              `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":                        `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="30" name="slide body"/><p:nvPr><p:ph type="body" idx="7"/></p:nvPr></p:nvSpPr></p:sp></p:spTree></p:cSld></p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels":             `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="l" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/></Relationships>`,
		"ppt/slideLayouts/slideLayout1.xml":            `<p:sldLayout xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="20" name="layout body"/><p:nvPr><p:ph type="body" idx="7"/></p:nvPr></p:nvSpPr><p:spPr><a:xfrm><a:off x="100" y="200"/><a:ext cx="300" cy="400"/></a:xfrm><a:solidFill><a:srgbClr val="112233"/></a:solidFill></p:spPr></p:sp></p:spTree></p:cSld></p:sldLayout>`,
		"ppt/slideLayouts/_rels/slideLayout1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="m" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/></Relationships>`,
		"ppt/slideMasters/slideMaster1.xml":            `<p:sldMaster xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="10" name="master body"/><p:nvPr><p:ph type="body" idx="7"/></p:nvPr></p:nvSpPr><p:spPr><a:prstGeom prst="roundRect"/><a:ln w="25400"><a:solidFill><a:srgbClr val="445566"/></a:solidFill></a:ln></p:spPr></p:sp></p:spTree></p:cSld></p:sldMaster>`,
		"ppt/slideMasters/_rels/slideMaster1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`,
	})
	slides, err := sax.BuildContextualSlidesBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(slides) != 1 || len(slides[0].Shapes) != 1 {
		t.Fatalf("slides = %#v", slides)
	}
	s := slides[0].Shapes[0]
	if s.Bounds.X != 100 || s.Bounds.Y != 200 || s.Bounds.W != 300 || s.Bounds.H != 400 {
		t.Fatalf("layout bounds = %#v", s.Bounds)
	}
	if s.FillColor != "112233" {
		t.Fatalf("layout fill = %q", s.FillColor)
	}
	if s.Geometry != "roundRect" || s.LineColor != "445566" || s.LineWidthPt != 2 {
		t.Fatalf("master fallback = geometry %q line %q width %v", s.Geometry, s.LineColor, s.LineWidthPt)
	}
	if slides[0].Layout == nil || slides[0].Master == nil || len(slides[0].Layout.Shapes) != 1 || len(slides[0].Master.Shapes) != 1 {
		t.Fatal("context shapes were not retained on canonical slide")
	}
}

func TestContextualBuilderStreamsAndInheritsPlaceholderTextProperties(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":                          `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/></Types>`,
		"_rels/.rels":                                  `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="r" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":                         `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="s"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels":              `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":                        `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="30" name="slide body"/><p:nvPr><p:ph type="body" idx="7"/></p:nvPr></p:nvSpPr><p:txBody><a:bodyPr/><a:p><a:pPr algn="r"/><a:r><a:rPr i="1"/><a:t>Hello</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels":             `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="l" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/></Relationships>`,
		"ppt/slideLayouts/slideLayout1.xml":            `<p:sldLayout xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="20" name="layout body"/><p:nvPr><p:ph type="body" idx="7"/></p:nvPr></p:nvSpPr><p:txBody><a:bodyPr/><a:p><a:pPr marL="914400" indent="-228600"/><a:r><a:rPr sz="2400" b="1"><a:latin typeface="Aptos"/></a:rPr><a:t>layout</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sldLayout>`,
		"ppt/slideLayouts/_rels/slideLayout1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="m" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/></Relationships>`,
		"ppt/slideMasters/slideMaster1.xml":            `<p:sldMaster xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="10" name="master body"/><p:nvPr><p:ph type="body" idx="7"/></p:nvPr></p:nvSpPr><p:txBody><a:bodyPr/><a:p><a:pPr fontAlgn="base"/><a:r><a:rPr u="sng"><a:solidFill><a:srgbClr val="336699"/></a:solidFill></a:rPr><a:t>master</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sldMaster>`,
		"ppt/slideMasters/_rels/slideMaster1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`,
	})
	slides, err := sax.BuildContextualSlidesBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	p := slides[0].Shapes[0].Paragraphs[0]
	if p.Align != "r" || !p.HasMarginLeft || p.MarginLeftEMU != 914400 || !p.HasTextIndent || p.TextIndentEMU != -228600 || p.FontAlign != "base" {
		t.Fatalf("paragraph cascade = %#v", p)
	}
	r := p.Runs[0]
	if r.Text != "Hello" || !r.Italic || !r.ItalicSet || !r.Bold || !r.BoldSet || !r.Underline || !r.UnderlineSet || r.FontSize != 2400 || r.FontFamily != "Aptos" || r.Color != "336699" {
		t.Fatalf("run cascade = %#v", r)
	}
}

func TestContextualBuilderUsesLayoutListStyleForTitleAndSubtitle(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":             `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/></Types>`,
		"_rels/.rels":                     `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="r" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":            `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="s"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml": `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree>
<p:sp><p:nvSpPr><p:cNvPr id="30" name="title"/><p:nvPr><p:ph type="ctrTitle"/></p:nvPr></p:nvSpPr><p:txBody><a:bodyPr/><a:p><a:r><a:rPr b="1"/><a:t>Math 260</a:t></a:r></a:p></p:txBody></p:sp>
<p:sp><p:nvSpPr><p:cNvPr id="31" name="subtitle"/><p:nvPr><p:ph type="subTitle" idx="1"/></p:nvPr></p:nvSpPr><p:txBody><a:bodyPr/><a:p><a:r><a:t>Lecture 1</a:t></a:r></a:p></p:txBody></p:sp>
</p:spTree></p:cSld></p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="l" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/></Relationships>`,
		"ppt/slideLayouts/slideLayout1.xml": `<p:sldLayout xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree>
<p:sp><p:nvSpPr><p:cNvPr id="20" name="layout title"/><p:nvPr><p:ph type="ctrTitle"/></p:nvPr></p:nvSpPr><p:txBody><a:bodyPr/><a:lstStyle><a:lvl1pPr algn="r"><a:defRPr sz="4800" b="1"/></a:lvl1pPr></a:lstStyle><a:p><a:r><a:t>template title</a:t></a:r></a:p></p:txBody></p:sp>
<p:sp><p:nvSpPr><p:cNvPr id="21" name="layout subtitle"/><p:nvPr><p:ph type="subTitle" idx="1"/></p:nvPr></p:nvSpPr><p:txBody><a:bodyPr/><a:lstStyle><a:lvl1pPr algn="r"><a:buNone/></a:lvl1pPr></a:lstStyle><a:p><a:r><a:t>template subtitle</a:t></a:r></a:p></p:txBody></p:sp>
</p:spTree></p:cSld></p:sldLayout>`,
		"ppt/slideLayouts/_rels/slideLayout1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="m" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/></Relationships>`,
		"ppt/slideMasters/slideMaster1.xml": `<p:sldMaster xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree/></p:cSld><p:txStyles>
<p:titleStyle><a:lvl1pPr algn="l"><a:defRPr sz="4100" b="1"/></a:lvl1pPr></p:titleStyle>
<p:bodyStyle><a:lvl1pPr algn="l"><a:buChar char="*"/><a:defRPr sz="2700"/></a:lvl1pPr></p:bodyStyle>
</p:txStyles></p:sldMaster>`,
		"ppt/slideMasters/_rels/slideMaster1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`,
	})
	slides, err := sax.BuildContextualSlidesBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(slides) != 1 || len(slides[0].Shapes) != 2 {
		t.Fatalf("slides = %#v", slides)
	}
	title := slides[0].Shapes[0].Paragraphs[0]
	if title.Align != "r" || title.Runs[0].FontSize != 4800 || !title.Runs[0].Bold {
		t.Fatalf("title style = paragraph %#v run %#v", title, title.Runs[0])
	}
	subtitle := slides[0].Shapes[1].Paragraphs[0]
	if subtitle.Align != "r" || subtitle.BulletChar != "" || !subtitle.BulletNone || subtitle.Runs[0].FontSize != 2700 || subtitle.Runs[0].Bold {
		t.Fatalf("subtitle style = paragraph %#v run %#v", subtitle, subtitle.Runs[0])
	}
}

func TestContextualBuilderAppliesMasterTextStylesByPlaceholderAndLevel(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":                          `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/></Types>`,
		"_rels/.rels":                                  `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="r" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":                         `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="s"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels":              `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":                        `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="30" name="body"/><p:nvPr><p:ph type="body" idx="7"/></p:nvPr></p:nvSpPr><p:txBody><a:bodyPr/><a:p><a:pPr lvl="1" algn="r"/><a:r><a:t>Hello</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels":             `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="l" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/></Relationships>`,
		"ppt/slideLayouts/slideLayout1.xml":            `<p:sldLayout xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"/>`,
		"ppt/slideLayouts/_rels/slideLayout1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="m" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/></Relationships>`,
		"ppt/slideMasters/slideMaster1.xml":            `<p:sldMaster xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree/></p:cSld><p:txStyles><p:bodyStyle><a:lvl2pPr marL="914400" indent="-228600" fontAlgn="base"><a:lnSpc><a:spcPct val="120000"/></a:lnSpc><a:spcBef><a:spcPts val="600"/></a:spcBef><a:buFont typeface="Wingdings"/><a:buChar char="•"/><a:buSzPct val="125000"/><a:defRPr sz="2200" b="1"><a:solidFill><a:srgbClr val="336699"/></a:solidFill><a:latin typeface="Aptos"/></a:defRPr></a:lvl2pPr></p:bodyStyle></p:txStyles></p:sldMaster>`,
		"ppt/slideMasters/_rels/slideMaster1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`,
	})
	slides, err := sax.BuildContextualSlidesBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	p := slides[0].Shapes[0].Paragraphs[0]
	if p.Align != "r" || p.MarginLeftEMU != 914400 || p.TextIndentEMU != -228600 || p.FontAlign != "base" {
		t.Fatalf("master paragraph style = %#v", p)
	}
	if p.BulletChar != "•" || p.BulletFontFamily != "Wingdings" || p.BulletSizePct != 125000 || !p.HasSpaceBefore || p.SpaceBefore != 600 || !p.HasLineSpacing || p.LineSpacingPct != 120000 {
		t.Fatalf("master list/spacing style = %#v", p)
	}
	r := p.Runs[0]
	if r.Text != "Hello" || r.FontSize != 2200 || r.FontFamily != "Aptos" || !r.Bold || !r.BoldSet || r.Color != "336699" {
		t.Fatalf("master default run style = %#v", r)
	}
}

func TestContextualBuilderStreamsBodyPropsThemeFontsAndPresentationDefaults(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":                          `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/></Types>`,
		"_rels/.rels":                                  `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="r" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":                         `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="s"/></p:sldIdLst><p:defaultTextStyle><a:lvl1pPr marL="123"><a:defRPr sz="1800" b="1"/></a:lvl1pPr></p:defaultTextStyle></p:presentation>`,
		"ppt/_rels/presentation.xml.rels":              `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":                        `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="2" name="body"/><p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr><p:txBody><a:bodyPr wrap="none" anchor="ctr" lIns="0" rIns="91440" numCol="2" spcCol="1000"><a:normAutofit fontScale="80000" lnSpcReduction="10000"/></a:bodyPr><a:p><a:r><a:t>Hello</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels":             `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="l" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/></Relationships>`,
		"ppt/slideLayouts/slideLayout1.xml":            `<p:sldLayout xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"/>`,
		"ppt/slideLayouts/_rels/slideLayout1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="m" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/></Relationships>`,
		"ppt/slideMasters/slideMaster1.xml":            `<p:sldMaster xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"/>`,
		"ppt/slideMasters/_rels/slideMaster1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="t" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="../theme/theme1.xml"/></Relationships>`,
		"ppt/theme/theme1.xml":                         `<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:themeElements><a:clrScheme name="x"/><a:fontScheme name="x"><a:majorFont><a:latin typeface="Major Face"/></a:majorFont><a:minorFont><a:latin typeface="Minor Face"/></a:minorFont></a:fontScheme></a:themeElements></a:theme>`,
	})
	slides, err := sax.BuildContextualSlidesBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	s := slides[0].Shapes[0]
	if s.TextBodyProps.Wrap != "none" || s.TextBodyProps.Anchor != "ctr" || s.TextBodyProps.Columns != 2 || s.TextBodyProps.AutofitType != "normAutofit" {
		t.Fatalf("body props = %#v", s.TextBodyProps)
	}
	if !s.HasTextInsets || s.TextBodyProps.InsetL != 0 || s.TextBodyProps.InsetR != 91440 {
		t.Fatalf("insets = %#v", s.TextBodyProps)
	}
	r := s.Paragraphs[0].Runs[0]
	if r.FontSize != 1800 || !r.Bold || r.FontFamily != "Minor Face" {
		t.Fatalf("default/theme run = %#v", r)
	}
	if s.Paragraphs[0].MarginLeftEMU != 123 {
		t.Fatalf("default margin = %d", s.Paragraphs[0].MarginLeftEMU)
	}
}

func TestContextualBuilderInheritsPlaceholderTransformAsUnit(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":                          `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/></Types>`,
		"_rels/.rels":                                  `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="r" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":                         `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="s"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels":              `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":                        `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="30" name="slide title"/><p:nvPr><p:ph type="title" idx="7"/></p:nvPr></p:nvSpPr></p:sp></p:spTree></p:cSld></p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels":             `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="l" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/></Relationships>`,
		"ppt/slideLayouts/slideLayout1.xml":            `<p:sldLayout xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="20" name="layout title"/><p:nvPr><p:ph type="title" idx="7"/></p:nvPr></p:nvSpPr><p:spPr><a:xfrm rot="1800000" flipH="1"><a:off x="100" y="200"/><a:ext cx="300" cy="400"/></a:xfrm></p:spPr></p:sp></p:spTree></p:cSld></p:sldLayout>`,
		"ppt/slideLayouts/_rels/slideLayout1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="m" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/></Relationships>`,
		"ppt/slideMasters/slideMaster1.xml":            `<p:sldMaster xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree/></p:cSld></p:sldMaster>`,
		"ppt/slideMasters/_rels/slideMaster1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`,
	})
	slides, err := sax.BuildContextualSlidesBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(slides) != 1 || len(slides[0].Shapes) != 1 {
		t.Fatalf("slides = %#v", slides)
	}
	s := slides[0].Shapes[0]
	if !s.HasTransform || s.Bounds != (ir.Rect{X: 100, Y: 200, W: 300, H: 400}) || s.RotationDeg != 30 || !s.FlipH {
		t.Fatalf("inherited transform = has=%v bounds=%#v rotation=%v flipH=%v", s.HasTransform, s.Bounds, s.RotationDeg, s.FlipH)
	}
}

func TestContextualBuilderAuthoredPlaceholderTransformOverridesTemplateTransform(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":                          `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/></Types>`,
		"_rels/.rels":                                  `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="r" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":                         `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="s"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels":              `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":                        `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="30" name="slide title"/><p:nvPr><p:ph type="title" idx="7"/></p:nvPr></p:nvSpPr><p:spPr><a:xfrm><a:off x="1" y="2"/><a:ext cx="3" cy="4"/></a:xfrm></p:spPr></p:sp></p:spTree></p:cSld></p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels":             `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="l" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/></Relationships>`,
		"ppt/slideLayouts/slideLayout1.xml":            `<p:sldLayout xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="20" name="layout title"/><p:nvPr><p:ph type="title" idx="7"/></p:nvPr></p:nvSpPr><p:spPr><a:xfrm rot="2700000" flipH="1"><a:off x="100" y="200"/><a:ext cx="300" cy="400"/></a:xfrm></p:spPr></p:sp></p:spTree></p:cSld></p:sldLayout>`,
		"ppt/slideLayouts/_rels/slideLayout1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="m" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/></Relationships>`,
		"ppt/slideMasters/slideMaster1.xml":            `<p:sldMaster xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree/></p:cSld></p:sldMaster>`,
		"ppt/slideMasters/_rels/slideMaster1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`,
	})
	slides, err := sax.BuildContextualSlidesBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(slides) != 1 || len(slides[0].Shapes) != 1 {
		t.Fatalf("slides = %#v", slides)
	}
	s := slides[0].Shapes[0]
	if !s.HasTransform || s.Bounds != (ir.Rect{X: 1, Y: 2, W: 3, H: 4}) || s.RotationDeg != 0 || s.FlipH {
		t.Fatalf("authored transform was overwritten: %#v", s)
	}
}

func TestContextualBuilderPreservesAuthoredParagraphBulletAndSpacingChildren(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":             `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/></Types>`,
		"_rels/.rels":                     `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="r" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":            `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="s"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":           `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="2" name="body"/></p:nvSpPr><p:txBody><a:bodyPr/><a:p><a:pPr marL="457200" indent="-228600"><a:lnSpc><a:spcPct val="115000"/></a:lnSpc><a:spcBef><a:spcPts val="600"/></a:spcBef><a:buFont typeface="Roboto"/><a:buSzPts val="2400"/><a:buChar char="●"/></a:pPr><a:r><a:t>Hello</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`,
	})
	slides, err := sax.BuildContextualSlidesBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	p := slides[0].Shapes[0].Paragraphs[0]
	if p.BulletChar != "●" || p.BulletFontFamily != "Roboto" || p.BulletSize != 2400 {
		t.Fatalf("authored bullet children lost: %#v", p)
	}
	if !p.HasLineSpacing || p.LineSpacingPct != 115000 || !p.HasSpaceBefore || p.SpaceBefore != 600 {
		t.Fatalf("authored paragraph spacing children lost: %#v", p)
	}
}
