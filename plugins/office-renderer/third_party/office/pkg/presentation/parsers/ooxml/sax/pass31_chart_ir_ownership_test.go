package sax_test

import (
	"office/pkg/presentation/ir"
	"office/pkg/presentation/parsers/ooxml/sax"
	"testing"
)

func TestPass31ContextualBuilderKeepsAlternateContentChartSemantic(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":              `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Default Extension="png" ContentType="image/png"/></Types>`,
		"_rels/.rels":                      `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="r" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":             `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="s"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels":  `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/_rels/slide1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="chart" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart" Target="../charts/chart1.xml"/><Relationship Id="preview" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/chart-preview.png"/><Relationship Id="photo" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/photo.png"/></Relationships>`,
		"ppt/charts/chart1.xml":            `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart><c:plotArea/></c:chart></c:chartSpace>`,
		"ppt/media/chart-preview.png":      "PREVIEW",
		"ppt/media/photo.png":              "PHOTO",
		"ppt/slides/slide1.xml": `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006"><p:cSld><p:spTree>
<mc:AlternateContent><mc:Choice Requires="c"><p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="3" name="Chart 1"/></p:nvGraphicFramePr><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart r:id="chart"/></a:graphicData></a:graphic></p:graphicFrame></mc:Choice><mc:Fallback><p:pic><p:nvPicPr><p:cNvPr id="4" name="Chart Preview"/></p:nvPicPr><p:blipFill><a:blip r:embed="preview"/></p:blipFill><p:spPr/></p:pic></mc:Fallback></mc:AlternateContent>
<p:pic><p:nvPicPr><p:cNvPr id="5" name="Photo"/></p:nvPicPr><p:blipFill><a:blip r:embed="photo"/></p:blipFill><p:spPr/></p:pic>
</p:spTree></p:cSld></p:sld>`,
	})
	pres, err := sax.BuildContextualPresentationBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(pres.Slides) != 1 || len(pres.Slides[0].Shapes) != 2 {
		t.Fatalf("shapes = %#v, want semantic chart plus ordinary picture only", pres.Slides)
	}
	chart := pres.Slides[0].Shapes[0]
	if chart.Kind != ir.ShapeChart || len(chart.ChartData) == 0 || chart.Name != "Chart 1" {
		t.Fatalf("semantic chart = %#v", chart)
	}
	if len(chart.ImageData) != 0 {
		t.Fatalf("chart unexpectedly owns fallback preview bytes: %d", len(chart.ImageData))
	}
	photo := pres.Slides[0].Shapes[1]
	if photo.Kind != ir.ShapeImage || string(photo.ImageData) != "PHOTO" || photo.Name != "Photo" {
		t.Fatalf("ordinary picture = %#v", photo)
	}
}
