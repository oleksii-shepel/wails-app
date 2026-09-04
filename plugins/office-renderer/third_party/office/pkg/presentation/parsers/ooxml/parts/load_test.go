package parts_test

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"office/pkg/diag"
	"office/pkg/ooxml/opc"
	"office/pkg/presentation/parsers/ooxml/parts"
)

func openPptxCorpusPackage(t *testing.T, path string) *opc.Package {
	t.Helper()

	pkg, err := opc.Open(path)
	if err != nil {
		if opc.IsEncryptedPackageError(err) {
			t.Skip("encrypted package requires password")
		}
		if strings.Contains(err.Error(), "zip: not a valid zip file") {
			t.Skip("fixture is not a valid OPC zip package")
		}
		t.Fatalf("open: %v", err)
	}
	if pkg.PackageType() != opc.PackageTypePresentation {
		t.Skipf("fixture package type = %q, want %q", pkg.PackageType(), opc.PackageTypePresentation)
	}
	return pkg
}

func TestPptxCorpusLoad(t *testing.T) {
	fixturesDir := repoPath(t, "testdata", "pptx")
	entries, err := os.ReadDir(fixturesDir)
	if err != nil {
		t.Fatalf("read fixtures dir: %v", err)
	}

	var count int
	for _, entry := range entries {
		if entry.IsDir() || !isPptxFixtureName(entry.Name()) {
			continue
		}
		count++
		path := filepath.Join(fixturesDir, entry.Name())
		t.Run(entry.Name(), func(t *testing.T) {
			pkg := openPptxCorpusPackage(t, path)
			pptPkg, err := parts.LoadPackage(pkg)
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			if pptPkg.Presentation == nil {
				t.Fatal("presentation is nil")
			}
			if len(pptPkg.Slides) == 0 {
				t.Fatal("no slides loaded")
			}
			for _, slide := range pptPkg.Slides {
				if slide.Slide == nil {
					t.Errorf("slide relId=%s not loaded", slide.RelID)
				}
			}
			// Only require slide masters when the presentation declares them.
			if len(pptPkg.Presentation.SlideMasterIDs) > 0 && len(pptPkg.SlideMasters) == 0 {
				t.Error("no slide masters loaded")
			}
		})
	}

	if count == 0 {
		t.Fatal("no pptx fixtures found")
	}
}

func TestLoadPackageParsesEmbeddedFonts(t *testing.T) {
	path := repoPath(t, "testdata", "pptx", "sample-2.pptx")
	pkg, err := opc.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	pptPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(pptPkg.EmbeddedFonts) != 2 {
		t.Fatalf("embedded font count = %d, want 2", len(pptPkg.EmbeddedFonts))
	}
	fonts := map[string]*parts.EmbeddedFontData{}
	for _, font := range pptPkg.EmbeddedFonts {
		fonts[font.Typeface] = font
	}
	sourceCode := fonts["Source Code Pro"]
	if sourceCode == nil {
		t.Fatal("missing Source Code Pro embedded font")
	}
	if sourceCode.Regular == nil || sourceCode.Bold == nil || sourceCode.Italic == nil || sourceCode.BoldItalic == nil {
		t.Fatalf("expected all Source Code Pro styles, got %#v", sourceCode)
	}
	oswald := fonts["Oswald"]
	if oswald == nil {
		t.Fatal("missing Oswald embedded font")
	}
	if oswald.Regular == nil || oswald.Bold == nil {
		t.Fatalf("expected regular/bold Oswald faces, got %#v", oswald)
	}
}

func TestLoadPackageParsesSlideSections(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
  <Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
  <Override PartName="/ppt/slides/slide2.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
</Relationships>`,
		"ppt/presentation.xml": `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:sldIdLst>
    <p:sldId id="256" r:id="rIdSlide1"/>
    <p:sldId id="257" r:id="rIdSlide2"/>
  </p:sldIdLst>
  <p:sectionLst>
    <p:section name="Introduction">
      <p:sldIdLst>
        <p:sldId id="256"/>
      </p:sldIdLst>
    </p:section>
    <p:section name="Details">
      <p:sldIdLst>
        <p:sldId id="257"/>
      </p:sldIdLst>
    </p:section>
  </p:sectionLst>
</p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdSlide1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>
  <Relationship Id="rIdSlide2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide2.xml"/>
</Relationships>`,
		"ppt/slides/slide1.xml": `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld></p:sld>`,
		"ppt/slides/slide2.xml": `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld></p:sld>`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	pptPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(pptPkg.Presentation.Sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(pptPkg.Presentation.Sections))
	}
	if pptPkg.Presentation.Sections[0].Name != "Introduction" {
		t.Fatalf("section[0].Name = %q, want Introduction", pptPkg.Presentation.Sections[0].Name)
	}
	if len(pptPkg.Presentation.Sections[0].SlideIDs) != 1 || pptPkg.Presentation.Sections[0].SlideIDs[0].ID != 256 {
		t.Fatalf("section[0] slide IDs = %+v, want [256]", pptPkg.Presentation.Sections[0].SlideIDs)
	}
	if pptPkg.Presentation.Sections[1].Name != "Details" {
		t.Fatalf("section[1].Name = %q, want Details", pptPkg.Presentation.Sections[1].Name)
	}
	if len(pptPkg.Presentation.Sections[1].SlideIDs) != 1 || pptPkg.Presentation.Sections[1].SlideIDs[0].ID != 257 {
		t.Fatalf("section[1] slide IDs = %+v, want [257]", pptPkg.Presentation.Sections[1].SlideIDs)
	}
}

func TestLoadPackageParsesSlideAndLayoutThemeOverrides(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
  <Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
  <Override PartName="/ppt/slideLayouts/slideLayout1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>
  <Override PartName="/ppt/themeOverrides/slideOverride1.xml" ContentType="application/vnd.openxmlformats-officedocument.themeOverride+xml"/>
  <Override PartName="/ppt/themeOverrides/layoutOverride1.xml" ContentType="application/vnd.openxmlformats-officedocument.themeOverride+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
</Relationships>`,
		"ppt/presentation.xml": `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:sldIdLst><p:sldId id="256" r:id="rIdSlide1"/></p:sldIdLst>
</p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdSlide1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>
</Relationships>`,
		"ppt/slides/slide1.xml": `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld></p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdLayout1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
  <Relationship Id="rIdThemeOverride1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/themeOverride" Target="../themeOverrides/slideOverride1.xml"/>
</Relationships>`,
		"ppt/slideLayouts/slideLayout1.xml": `<p:sldLayout xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld></p:sldLayout>`,
		"ppt/slideLayouts/_rels/slideLayout1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdThemeOverride1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/themeOverride" Target="../themeOverrides/layoutOverride1.xml"/>
</Relationships>`,
		"ppt/themeOverrides/slideOverride1.xml": `<a:themeOverride xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <a:clrScheme name="Slide Override">
    <a:accent1><a:srgbClr val="AA0000"/></a:accent1>
  </a:clrScheme>
</a:themeOverride>`,
		"ppt/themeOverrides/layoutOverride1.xml": `<a:themeOverride xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <a:fontScheme name="Layout Fonts">
    <a:majorFont><a:latin typeface="Oswald"/></a:majorFont>
    <a:minorFont><a:latin typeface="Aptos"/></a:minorFont>
  </a:fontScheme>
</a:themeOverride>`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	pptPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(pptPkg.Slides) != 1 {
		t.Fatalf("slide count = %d, want 1", len(pptPkg.Slides))
	}
	entry := pptPkg.Slides[0]
	if entry.Slide == nil || entry.Slide.ThemeOverride == nil || entry.Slide.ThemeOverride.ClrScheme == nil {
		t.Fatalf("slide theme override = %#v, want parsed clrScheme", entry.Slide)
	}
	if entry.Slide.ThemeOverride.ClrScheme.Accent1 == nil || entry.Slide.ThemeOverride.ClrScheme.Accent1.SrgbClr == nil || entry.Slide.ThemeOverride.ClrScheme.Accent1.SrgbClr.Val != "AA0000" {
		t.Fatalf("slide override accent1 = %#v, want AA0000", entry.Slide.ThemeOverride.ClrScheme.Accent1)
	}
	if entry.Layout == nil || entry.Layout.ThemeOverride == nil || entry.Layout.ThemeOverride.FontScheme == nil {
		t.Fatalf("layout theme override = %#v, want parsed fontScheme", entry.Layout)
	}
	if got := entry.Layout.ThemeOverride.FontScheme.MajorFont.Latin.Typeface; got != "Oswald" {
		t.Fatalf("layout major font = %q, want Oswald", got)
	}
}

func TestLoadPackagePreservesThemeDefaultsAndAlternateColorSchemes(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
  <Override PartName="/ppt/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
</Relationships>`,
		"ppt/presentation.xml": `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"/>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdTheme1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="theme/theme1.xml"/>
</Relationships>`,
		"ppt/theme/theme1.xml": `<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Office Theme">
  <a:themeElements>
    <a:clrScheme name="Office">
      <a:dk1><a:srgbClr val="000000"/></a:dk1>
      <a:lt1><a:srgbClr val="FFFFFF"/></a:lt1>
    </a:clrScheme>
    <a:fontScheme name="Office">
      <a:majorFont><a:latin typeface="Calibri"/></a:majorFont>
      <a:minorFont><a:latin typeface="Arial"/></a:minorFont>
    </a:fontScheme>
    <a:fmtScheme name="Office"/>
  </a:themeElements>
  <a:objectDefaults>
    <a:spDef>
      <a:spPr><a:solidFill><a:schemeClr val="accent1"/></a:solidFill></a:spPr>
    </a:spDef>
    <a:lnDef><a:ln w="12700"/></a:lnDef>
    <a:txDef><a:bodyPr anchor="ctr"/></a:txDef>
  </a:objectDefaults>
  <a:extraClrSchemeLst>
    <a:extraClrScheme>
      <a:clrScheme name="Alt Office">
        <a:accent1><a:srgbClr val="123456"/></a:accent1>
      </a:clrScheme>
      <a:clrMap bg1="lt1" tx1="dk1"/>
    </a:extraClrScheme>
  </a:extraClrSchemeLst>
</a:theme>`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	pptPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	theme := pptPkg.Themes["ppt/theme/theme1.xml"]
	if theme == nil {
		t.Fatalf("theme = nil, want parsed theme")
	}
	if theme.ObjectDefaults == nil || theme.ObjectDefaults.ShapeDefault == nil || theme.ObjectDefaults.LineDefault == nil || theme.ObjectDefaults.TextDefault == nil {
		t.Fatalf("object defaults = %#v, want preserved shape/line/text defaults", theme.ObjectDefaults)
	}
	if theme.ObjectDefaults.LineDefault.Ln == nil || theme.ObjectDefaults.LineDefault.Ln.W != 12700 {
		t.Fatalf("line default = %#v, want width 12700", theme.ObjectDefaults.LineDefault)
	}
	if theme.ObjectDefaults.TextDefault.BodyPr == nil || theme.ObjectDefaults.TextDefault.BodyPr.Anchor != "ctr" {
		t.Fatalf("text default bodyPr = %#v, want anchor ctr", theme.ObjectDefaults.TextDefault)
	}
	if theme.ExtraClrSchemes == nil || len(theme.ExtraClrSchemes.Schemes) != 1 {
		t.Fatalf("extraClrSchemes = %#v, want one alternate scheme", theme.ExtraClrSchemes)
	}
	alt := theme.ExtraClrSchemes.Schemes[0]
	if alt.ClrScheme == nil || alt.ClrScheme.Accent1 == nil || alt.ClrScheme.Accent1.SrgbClr == nil || alt.ClrScheme.Accent1.SrgbClr.Val != "123456" {
		t.Fatalf("alternate color scheme = %#v, want accent1 123456", alt)
	}
	if alt.ClrMap == nil || alt.ClrMap.Bg1 != "lt1" || alt.ClrMap.Tx1 != "dk1" {
		t.Fatalf("alternate color map = %#v, want bg1=lt1 tx1=dk1", alt.ClrMap)
	}
}

func TestLoadPackageParsesLineBreaks(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
  <Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
</Relationships>`,
		"ppt/presentation.xml": `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:sldIdLst><p:sldId id="256" r:id="rIdSlide1"/></p:sldIdLst>
</p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdSlide1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>
</Relationships>`,
		"ppt/slides/slide1.xml": `<p:sld
  xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
  xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:cSld>
    <p:spTree>
      <p:nvGrpSpPr><p:cNvPr id="1" name=""/></p:nvGrpSpPr>
      <p:grpSpPr/>
      <p:sp>
        <p:nvSpPr><p:cNvPr id="2" name="TextBox"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>
        <p:spPr/>
        <p:txBody>
          <a:bodyPr/>
          <a:p>
            <a:r><a:t>First line</a:t></a:r>
            <a:br><a:rPr sz="2400"/></a:br>
            <a:r><a:t>Second line</a:t></a:r>
          </a:p>
        </p:txBody>
      </p:sp>
    </p:spTree>
  </p:cSld>
</p:sld>`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	pptPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(pptPkg.Slides) != 1 || pptPkg.Slides[0].Slide == nil {
		t.Fatalf("expected one loaded slide")
	}
	shapes := pptPkg.Slides[0].Slide.CSld.SpTree.Shapes
	if len(shapes) != 1 || shapes[0].TxBody == nil {
		t.Fatalf("expected one text shape")
	}
	paras := shapes[0].TxBody.Paragraphs
	if len(paras) != 1 {
		t.Fatalf("expected 1 paragraph, got %d", len(paras))
	}
	if len(paras[0].Breaks) != 1 {
		t.Fatalf("expected 1 break, got %d", len(paras[0].Breaks))
	}
	if len(paras[0].Inlines) != 3 {
		t.Fatalf("expected 3 inlines, got %d", len(paras[0].Inlines))
	}
	if paras[0].Inlines[0].Kind != parts.DMLParaInlineRun || paras[0].Inlines[0].Run == nil || paras[0].Inlines[0].Run.Text != "First line" {
		t.Fatalf("inline[0] = %+v, want First line run", paras[0].Inlines[0])
	}
	if paras[0].Inlines[1].Kind != parts.DMLParaInlineBreak || paras[0].Inlines[1].Break == nil {
		t.Fatalf("inline[1] = %+v, want break", paras[0].Inlines[1])
	}
	if paras[0].Inlines[1].Break.Properties == nil || paras[0].Inlines[1].Break.Properties.FontSize != 2400 {
		t.Fatalf("break font size = %v, want 2400", paras[0].Inlines[1].Break.Properties)
	}
	if paras[0].Inlines[2].Kind != parts.DMLParaInlineRun || paras[0].Inlines[2].Run == nil || paras[0].Inlines[2].Run.Text != "Second line" {
		t.Fatalf("inline[2] = %+v, want Second line run", paras[0].Inlines[2])
	}
}

func TestLoadPackageReportsUnsupportedFeatureDiagnostics(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
  <Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
</Relationships>`,
		"ppt/presentation.xml": `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:sldIdLst><p:sldId id="256" r:id="rIdSlide1"/></p:sldIdLst>
</p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdSlide1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>
</Relationships>`,
		"ppt/slides/slide1.xml": `<p:sld
  xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
  xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:cSld>
    <p:spTree>
      <p:nvGrpSpPr><p:cNvPr id="1" name=""/></p:nvGrpSpPr>
      <p:grpSpPr/>
      <p:graphicFrame>
        <a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"/></a:graphic>
      </p:graphicFrame>
      <p:graphicFrame>
        <a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/diagram"/></a:graphic>
      </p:graphicFrame>
    </p:spTree>
  </p:cSld>
</p:sld>`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	pptPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	// Charts are supported via relationship-based chart parts, so unsupported-chart
	// should NOT appear even when a graphicFrame has a chart URI.
	if hasDiagnosticCode(pptPkg.Diagnostics, "pptx/unsupported-chart") {
		t.Fatalf("unexpected stale chart diagnostic in %+v", pptPkg.Diagnostics)
	}
	if hasDiagnosticCode(pptPkg.Diagnostics, "pptx/unsupported-smartart") {
		t.Fatalf("unexpected stale smartart diagnostic in %+v", pptPkg.Diagnostics)
	}
	if hasDiagnosticCode(pptPkg.Diagnostics, "pptx/unsupported-ole") {
		t.Fatalf("unexpected stale ole diagnostic in %+v", pptPkg.Diagnostics)
	}
}

func TestLoadPackageUsesAlternateContentFallbackForUnsupportedChoice(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
  <Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
</Relationships>`,
		"ppt/presentation.xml": `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:sldIdLst><p:sldId id="256" r:id="rIdSlide1"/></p:sldIdLst>
</p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdSlide1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>
</Relationships>`,
		"ppt/slides/slide1.xml": `<p:sld
  xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
  xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
  xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006"
  xmlns:a14="http://schemas.microsoft.com/office/drawing/2010/main">
  <p:cSld>
    <p:spTree>
      <p:nvGrpSpPr><p:cNvPr id="1" name=""/></p:nvGrpSpPr>
      <p:grpSpPr/>
      <mc:AlternateContent>
        <mc:Choice Requires="a14">
          <p:sp>
            <p:nvSpPr><p:cNvPr id="2" name="Unsupported Choice"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>
            <p:spPr/>
            <p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:t>wrong</a:t></a:r></a:p></p:txBody>
          </p:sp>
        </mc:Choice>
        <mc:Fallback>
          <p:sp>
            <p:nvSpPr><p:cNvPr id="3" name="Fallback Shape"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>
            <p:spPr/>
            <p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:t>right</a:t></a:r></a:p></p:txBody>
          </p:sp>
        </mc:Fallback>
      </mc:AlternateContent>
    </p:spTree>
  </p:cSld>
</p:sld>`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	pptPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := len(pptPkg.Slides[0].Slide.CSld.SpTree.Shapes); got != 1 {
		t.Fatalf("shape count = %d, want 1", got)
	}
	shape := pptPkg.Slides[0].Slide.CSld.SpTree.Shapes[0]
	if shape.NvSpPr.CNvPr.Name != "Fallback Shape" {
		t.Fatalf("shape name = %q, want fallback shape", shape.NvSpPr.CNvPr.Name)
	}
	if len(pptPkg.Slides[0].Slide.CSld.SpTree.Items) != 1 || pptPkg.Slides[0].Slide.CSld.SpTree.Items[0].Kind != parts.ShapeTreeItemShape {
		t.Fatalf("shape tree items = %+v", pptPkg.Slides[0].Slide.CSld.SpTree.Items)
	}
}

func TestLoadPackageCapturesEmbeddedVideoPicture(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Default Extension="png" ContentType="image/png"/>
  <Default Extension="mp4" ContentType="video/mp4"/>
  <Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
  <Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
</Relationships>`,
		"ppt/presentation.xml": `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:sldIdLst><p:sldId id="256" r:id="rIdSlide1"/></p:sldIdLst>
</p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdSlide1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>
</Relationships>`,
		"ppt/slides/_rels/slide1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdPoster" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/poster.png"/>
  <Relationship Id="rIdVideo" Type="http://schemas.microsoft.com/office/2007/relationships/media" Target="../media/movie.mp4"/>
</Relationships>`,
		"ppt/slides/slide1.xml": `<p:sld
  xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
  xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
  xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
  xmlns:p14="http://schemas.microsoft.com/office/powerpoint/2010/main">
  <p:cSld>
    <p:spTree>
      <p:nvGrpSpPr><p:cNvPr id="1" name=""/></p:nvGrpSpPr>
      <p:grpSpPr/>
      <p:pic>
        <p:nvPicPr>
          <p:cNvPr id="2" name="Video 1"/>
          <p:cNvPicPr/>
          <p:nvPr>
            <a:videoFile r:link="rIdVideo"/>
            <p:extLst>
              <p:ext uri="{28A0092B-C50C-407E-A947-70E740481C1C}">
                <p14:media r:embed="rIdVideo"/>
              </p:ext>
            </p:extLst>
          </p:nvPr>
        </p:nvPicPr>
        <p:blipFill><a:blip r:embed="rIdPoster"/></p:blipFill>
        <p:spPr><a:xfrm><a:off x="100" y="200"/><a:ext cx="300" cy="400"/></a:xfrm></p:spPr>
      </p:pic>
    </p:spTree>
  </p:cSld>
</p:sld>`,
		"ppt/media/poster.png": "PNG",
		"ppt/media/movie.mp4":  "mp4-bytes",
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	pptPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := pptPkg.Slides[0].MediaParts["rIdVideo"].ContentType; got != "video/mp4" {
		t.Fatalf("video content type = %q, want video/mp4", got)
	}
	pic := pptPkg.Slides[0].Slide.CSld.SpTree.Pictures[0]
	if pic.NvPicPr.NvPr.PrimaryMedia() == nil || pic.NvPicPr.NvPr.PrimaryMedia().RelID() != "rIdVideo" {
		t.Fatalf("primary media = %#v, want rIdVideo", pic.NvPicPr.NvPr.PrimaryMedia())
	}
	if pic.NvPicPr.NvPr.MediaKind() != "video" {
		t.Fatalf("media kind = %q, want video", pic.NvPicPr.NvPr.MediaKind())
	}
}

func TestLoadPackageParsesGraphicFrameTable(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
	<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
	<Default Extension="xml" ContentType="application/xml"/>
	<Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
	<Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
	<Override PartName="/ppt/tableStyles.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.tableStyles+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
	<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
</Relationships>`,
		"ppt/presentation.xml": `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
	<p:sldIdLst><p:sldId id="256" r:id="rIdSlide1"/></p:sldIdLst>
</p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
	<Relationship Id="rIdSlide1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>
	<Relationship Id="rIdTableStyles" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/tableStyles" Target="tableStyles.xml"/>
</Relationships>`,
		"ppt/tableStyles.xml": `<a:tblStyleLst xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" def="{STYLE-ID}">
	<a:tblStyle styleId="{STYLE-ID}" styleName="Accent Header">
		<a:wholeTbl>
			<a:tcStyle>
				<a:fill><a:solidFill><a:schemeClr val="accent1"/></a:solidFill></a:fill>
			</a:tcStyle>
		</a:wholeTbl>
		<a:firstRow>
			<a:tcTxStyle b="on"><a:schemeClr val="lt1"/></a:tcTxStyle>
		</a:firstRow>
	</a:tblStyle>
</a:tblStyleLst>`,
		"ppt/slides/slide1.xml": `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
	<p:cSld>
		<p:spTree>
			<p:nvGrpSpPr><p:cNvPr id="1" name=""/></p:nvGrpSpPr>
			<p:grpSpPr/>
			<p:graphicFrame>
				<p:nvGraphicFramePr>
					<p:cNvPr id="5" name="Table 4" descr="table desc"/>
					<p:cNvGraphicFramePr/>
				</p:nvGraphicFramePr>
				<p:xfrm><a:off x="1000" y="2000"/><a:ext cx="3000" cy="4000"/></p:xfrm>
				<a:graphic>
					<a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/table">
						<a:tbl>
							<a:tblPr firstRow="1" bandRow="1"><a:tableStyleId>{STYLE-ID}</a:tableStyleId></a:tblPr>
							<a:tblGrid><a:gridCol w="1000"/><a:gridCol w="2000"/></a:tblGrid>
							<a:tr h="500">
								<a:tc gridSpan="2">
									<a:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:t>Top</a:t></a:r></a:p></a:txBody>
									<a:tcPr marL="100" marR="200"><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></a:tcPr>
								</a:tc>
							</a:tr>
							<a:tr h="600">
								<a:tc rowSpan="2"><a:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:t>Left</a:t></a:r></a:p></a:txBody></a:tc>
								<a:tc><a:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:t>Right</a:t></a:r></a:p></a:txBody></a:tc>
							</a:tr>
							<a:tr h="600">
								<a:tc vMerge="1"><a:txBody><a:bodyPr/><a:lstStyle/><a:p/></a:txBody></a:tc>
								<a:tc><a:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:t>Bottom</a:t></a:r></a:p></a:txBody></a:tc>
							</a:tr>
						</a:tbl>
					</a:graphicData>
				</a:graphic>
			</p:graphicFrame>
		</p:spTree>
	</p:cSld>
</p:sld>`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	pptPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(pptPkg.Slides) != 1 || pptPkg.Slides[0].Slide == nil {
		t.Fatalf("expected one loaded slide, got %#v", pptPkg.Slides)
	}
	tables := pptPkg.Slides[0].Slide.CSld.SpTree.Tables
	if len(tables) != 1 {
		t.Fatalf("table count = %d, want 1", len(tables))
	}
	if got := tables[0].Name; got != "Table 4" {
		t.Fatalf("table name = %q, want %q", got, "Table 4")
	}
	if got := tables[0].Table.Rows[0].Cells[0].GridSpan; got != 2 {
		t.Fatalf("gridSpan = %d, want 2", got)
	}
	if got := tables[0].Table.Rows[2].Cells[0].VMerge; !got {
		t.Fatalf("expected vMerge continuation cell")
	}
	if tables[0].Table.Props == nil || !tables[0].Table.Props.FirstRow || !tables[0].Table.Props.BandRow {
		t.Fatalf("expected parsed table props, got %#v", tables[0].Table.Props)
	}
	if got := tables[0].Table.Props.TableStyleID; got != "{STYLE-ID}" {
		t.Fatalf("tableStyleId = %q, want {STYLE-ID}", got)
	}
	if pptPkg.TableStyles == nil || len(pptPkg.TableStyles.Styles) != 1 {
		t.Fatalf("expected loaded table styles, got %#v", pptPkg.TableStyles)
	}
	if hasDiagnosticCode(pptPkg.Diagnostics, "pptx/unsupported-table") {
		t.Fatalf("unexpected unsupported table diagnostic: %+v", pptPkg.Diagnostics)
	}
}

func TestLoadPackageParsesOLEObjectFrames(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
  <Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
  <Override PartName="/ppt/embeddings/oleObject1.bin" ContentType="application/vnd.openxmlformats-officedocument.oleObject"/>
  <Override PartName="/ppt/media/image1.png" ContentType="image/png"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
</Relationships>`,
		"ppt/presentation.xml": `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:sldIdLst><p:sldId id="256" r:id="rIdSlide1"/></p:sldIdLst>
</p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdSlide1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>
</Relationships>`,
		"ppt/slides/_rels/slide1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/oleObject" Target="../embeddings/oleObject1.bin"/>
  <Relationship Id="rId4" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/image1.png"/>
</Relationships>`,
		"ppt/slides/slide1.xml": `<p:sld
  xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
  xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
  xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
  xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006">
  <p:cSld>
    <p:spTree>
      <p:nvGrpSpPr><p:cNvPr id="1" name=""/></p:nvGrpSpPr>
      <p:grpSpPr/>
      <p:graphicFrame>
        <p:nvGraphicFramePr>
          <p:cNvPr id="4" name="Object"/>
          <p:cNvGraphicFramePr><a:graphicFrameLocks noGrp="1"/></p:cNvGraphicFramePr>
          <p:nvPr/>
        </p:nvGraphicFramePr>
        <p:xfrm>
          <a:off x="1000000" y="1000000"/>
          <a:ext cx="1000000" cy="1000000"/>
        </p:xfrm>
        <a:graphic>
          <a:graphicData uri="http://schemas.openxmlformats.org/presentationml/2006/ole">
            <mc:AlternateContent>
              <mc:Choice xmlns:v="urn:schemas-microsoft-com:vml" Requires="v">
                <p:oleObj name="Document" r:id="rId3" showAsIcon="1" progId="Word.Document.12">
                  <p:embed/>
                </p:oleObj>
              </mc:Choice>
              <mc:Fallback>
                <p:oleObj name="Document" r:id="rId3" showAsIcon="1" progId="Word.Document.12">
                  <p:embed/>
                  <p:pic>
                    <p:blipFill><a:blip r:embed="rId4"/></p:blipFill>
                    <p:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="914400" cy="771480"/></a:xfrm></p:spPr>
                  </p:pic>
                </p:oleObj>
              </mc:Fallback>
            </mc:AlternateContent>
          </a:graphicData>
        </a:graphic>
      </p:graphicFrame>
    </p:spTree>
  </p:cSld>
</p:sld>`,
		"ppt/embeddings/oleObject1.bin": `OLE payload`,
		"ppt/media/image1.png":          `PNG icon`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	pptPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(pptPkg.Slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(pptPkg.Slides))
	}
	slide := pptPkg.Slides[0]
	if len(slide.Slide.CSld.SpTree.OleObjectFrames) != 1 {
		t.Fatalf("expected 1 OLE object frame, got %d", len(slide.Slide.CSld.SpTree.OleObjectFrames))
	}
	frame := slide.Slide.CSld.SpTree.OleObjectFrames[0]
	if frame.RelID != "rId3" {
		t.Errorf("frame.RelID = %q, want rId3", frame.RelID)
	}
	if !frame.Embed {
		t.Errorf("expected embedded OLE object")
	}
	if !frame.ShowAsIcon {
		t.Errorf("expected showAsIcon=true")
	}
	if frame.ProgID != "Word.Document.12" {
		t.Errorf("frame.ProgID = %q, want Word.Document.12", frame.ProgID)
	}
	if frame.ObjectName != "Document" {
		t.Errorf("frame.ObjectName = %q, want Document", frame.ObjectName)
	}
	if frame.IconRelID != "rId4" {
		t.Errorf("frame.IconRelID = %q, want rId4", frame.IconRelID)
	}
	if slide.OleObjectParts == nil {
		t.Fatalf("expected OleObjectParts to be loaded")
	}
	part, ok := slide.OleObjectParts["rId3"]
	if !ok {
		t.Fatalf("expected OleObjectParts[rId3] to be loaded")
	}
	if string(part.Data) != "OLE payload" {
		t.Errorf("OleObjectParts data = %q, want OLE payload", string(part.Data))
	}
	if slide.MediaParts == nil {
		t.Fatalf("expected MediaParts to be loaded")
	}
	icon, ok := slide.MediaParts["rId4"]
	if !ok {
		t.Fatalf("expected MediaParts[rId4] to be loaded")
	}
	if string(icon.Data) != "PNG icon" {
		t.Errorf("icon data = %q, want PNG icon", string(icon.Data))
	}
}

func repoPath(t *testing.T, elems ...string) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", ".."))
	return filepath.Join(append([]string{root}, elems...)...)
}

func isPptxFixtureName(name string) bool {
	return strings.EqualFold(filepath.Ext(name), ".pptx") && !strings.HasPrefix(name, "~$")
}

func TestLoadPackageContinuesAfterMalformedSlideAndPreservesDiagnostic(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
  <Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
  <Override PartName="/ppt/slides/slide2.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
</Types>`,
		"_rels/.rels":                     `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":            `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="rIdSlide1"/><p:sldId id="257" r:id="rIdSlide2"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdSlide1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/><Relationship Id="rIdSlide2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide2.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":           `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld>`,
		"ppt/slides/slide2.xml":           `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld></p:sld>`,
	})
	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	pptPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load should tolerate malformed slide: %v", err)
	}
	if len(pptPkg.Slides) != 2 {
		t.Fatalf("slides=%d, want 2", len(pptPkg.Slides))
	}
	if pptPkg.Slides[0].Slide != nil {
		t.Fatal("malformed slide should remain unresolved")
	}
	if pptPkg.Slides[1].Slide == nil {
		t.Fatal("valid sibling slide should still load")
	}
	if !hasDiagnosticCode(pptPkg.Diagnostics, "pptx/malformed-slide") {
		t.Fatalf("missing malformed-slide diagnostic: %+v", pptPkg.Diagnostics)
	}
}

func TestLoadPackageContinuesAfterMalformedLayoutAndMaster(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
  <Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
  <Override PartName="/ppt/slideLayouts/slideLayout1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>
  <Override PartName="/ppt/slideMasters/slideMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"/>
</Types>`,
		"_rels/.rels":                       `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":              `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rIdMaster1"/></p:sldMasterIdLst><p:sldIdLst><p:sldId id="256" r:id="rIdSlide1"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels":   `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdSlide1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/><Relationship Id="rIdMaster1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="slideMasters/slideMaster1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":             `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld></p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels":  `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdLayout1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/></Relationships>`,
		"ppt/slideLayouts/slideLayout1.xml": `<p:sldLayout xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld>`,
		"ppt/slideMasters/slideMaster1.xml": `<p:sldMaster xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld>`,
	})
	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	pptPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load should tolerate malformed layout/master: %v", err)
	}
	if len(pptPkg.Slides) != 1 || pptPkg.Slides[0].Slide == nil {
		t.Fatal("valid slide should load")
	}
	if pptPkg.Slides[0].Layout != nil {
		t.Fatal("malformed layout should be skipped")
	}
	if !hasDiagnosticCode(pptPkg.Diagnostics, "pptx/malformed-slide-layout") {
		t.Fatalf("missing layout diagnostic: %+v", pptPkg.Diagnostics)
	}
	if !hasDiagnosticCode(pptPkg.Diagnostics, "pptx/malformed-slide-master") {
		t.Fatalf("missing master diagnostic: %+v", pptPkg.Diagnostics)
	}
}

func TestLoadPackageReportsMalformedOptionalPartsWithoutFailing(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
  <Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
</Types>`,
		"_rels/.rels":          `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml": `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="rIdSlide1"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdSlide1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>
  <Relationship Id="rIdTheme" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="theme/theme1.xml"/>
  <Relationship Id="rIdTableStyles" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/tableStyles" Target="tableStyles.xml"/>
  <Relationship Id="rIdAuthors" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/commentAuthors" Target="commentAuthors.xml"/>
  <Relationship Id="rIdNotesMaster" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesMaster" Target="notesMasters/notesMaster1.xml"/>
</Relationships>`,
		"ppt/slides/slide1.xml":             `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld></p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels":  `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdNotes" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesSlide" Target="../notesSlides/notesSlide1.xml"/><Relationship Id="rIdComments" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/comments" Target="../comments/comment1.xml"/></Relationships>`,
		"ppt/theme/theme1.xml":              `<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">`,
		"ppt/tableStyles.xml":               `<a:tblStyleLst xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">`,
		"ppt/commentAuthors.xml":            `<p:cmAuthorLst xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">`,
		"ppt/notesMasters/notesMaster1.xml": `<p:notesMaster xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">`,
		"ppt/notesSlides/notesSlide1.xml":   `<p:notes xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">`,
		"ppt/comments/comment1.xml":         `<p:cmLst xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">`,
	})
	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	pptPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load should tolerate malformed optional parts: %v", err)
	}
	for _, code := range []string{
		"pptx/malformed-theme",
		"pptx/malformed-table-styles",
		"pptx/malformed-comment-authors",
		"pptx/malformed-notes-master",
		"pptx/malformed-notes-slide",
		"pptx/malformed-comments",
	} {
		if !hasDiagnosticCode(pptPkg.Diagnostics, code) {
			t.Errorf("missing %s diagnostic: %+v", code, pptPkg.Diagnostics)
		}
	}
}

func TestLoadPackageSupportsStrictOfficeRelationshipTypes(t *testing.T) {
	const strict = "http://purl.oclc.org/ooxml/officeDocument/relationships/"
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
  <Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
  <Override PartName="/ppt/slideLayouts/slideLayout1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>
  <Override PartName="/ppt/slideMasters/slideMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"/>
  <Override PartName="/ppt/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>
</Types>`,
		"_rels/.rels":                                  `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="` + strict + `officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":                         `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rIdMaster1"/></p:sldMasterIdLst><p:sldIdLst><p:sldId id="256" r:id="rIdSlide1"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels":              `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdSlide1" Type="` + strict + `slide" Target="slides/slide1.xml"/><Relationship Id="rIdMaster1" Type="` + strict + `slideMaster" Target="slideMasters/slideMaster1.xml"/><Relationship Id="rIdTheme" Type="` + strict + `theme" Target="theme/theme1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":                        `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld></p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels":             `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdLayout1" Type="` + strict + `slideLayout" Target="../slideLayouts/slideLayout1.xml"/></Relationships>`,
		"ppt/slideLayouts/slideLayout1.xml":            `<p:sldLayout xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld></p:sldLayout>`,
		"ppt/slideLayouts/_rels/slideLayout1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdMasterBack" Type="` + strict + `slideMaster" Target="../slideMasters/slideMaster1.xml"/></Relationships>`,
		"ppt/slideMasters/slideMaster1.xml":            `<p:sldMaster xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld></p:sldMaster>`,
		"ppt/theme/theme1.xml":                         `<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Strict Theme"><a:themeElements><a:clrScheme name="Strict"><a:dk1><a:srgbClr val="000000"/></a:dk1><a:lt1><a:srgbClr val="FFFFFF"/></a:lt1></a:clrScheme></a:themeElements></a:theme>`,
	})
	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	pptPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load strict relationships: %v", err)
	}
	if len(pptPkg.Slides) != 1 || pptPkg.Slides[0].Slide == nil {
		t.Fatal("strict slide relationship was not resolved")
	}
	if pptPkg.Slides[0].Layout == nil {
		t.Fatal("strict slideLayout relationship was not resolved")
	}
	if pptPkg.Slides[0].MasterPartName != "ppt/slidemasters/slidemaster1.xml" {
		t.Fatalf("master part=%q", pptPkg.Slides[0].MasterPartName)
	}
	if len(pptPkg.SlideMasters) != 1 || pptPkg.SlideMasters[0].Master == nil {
		t.Fatal("strict slideMaster relationship was not resolved")
	}
	if len(pptPkg.Themes) != 1 {
		t.Fatalf("themes=%d, want 1", len(pptPkg.Themes))
	}
}

func TestLoadPackageReportsMalformedSlideRelationshipsWithoutDroppingSlide(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
  <Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
</Types>`,
		"_rels/.rels":                      `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":             `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="rIdSlide1"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels":  `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdSlide1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":            `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld></p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship`,
	})
	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	pptPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load should tolerate malformed slide relationships: %v", err)
	}
	if len(pptPkg.Slides) != 1 || pptPkg.Slides[0].Slide == nil {
		t.Fatal("valid slide content should remain available")
	}
	if !hasDiagnosticCode(pptPkg.Diagnostics, "pptx/malformed-slide-relationships") {
		t.Fatalf("missing malformed slide relationships diagnostic: %+v", pptPkg.Diagnostics)
	}
}

func hasDiagnosticCode(diagnostics diag.List, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}

func buildPackage(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, data := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := entry.Write([]byte(data)); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buffer.Bytes()
}
