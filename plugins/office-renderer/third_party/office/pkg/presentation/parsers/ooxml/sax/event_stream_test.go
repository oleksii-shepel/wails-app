package sax_test

import (
	"reflect"
	"testing"

	"office/pkg/presentation/parsers/ooxml/sax"
)

func TestWalkBytesEmitsNestedShapeEventsInSourceOrder(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":             `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/><Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/></Types>`,
		"_rels/.rels":                     `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":            `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="rIdSlide1"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdSlide1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":           `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name="root"/></p:nvGrpSpPr><p:grpSpPr/><p:grpSp><p:nvGrpSpPr><p:cNvPr id="10" name="Group 1"/></p:nvGrpSpPr><p:grpSpPr/><p:sp><p:nvSpPr><p:cNvPr id="11" name="Child"/></p:nvSpPr><p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:t>inside</a:t></a:r></a:p></p:txBody></p:sp></p:grpSp><p:pic><p:nvPicPr><p:cNvPr id="12" name="Picture"/></p:nvPicPr></p:pic></p:spTree></p:cSld></p:sld>`,
	})

	var got []string
	err := sax.WalkBytes(data, sax.HandlerFunc(func(ev sax.Event) error {
		switch ev.Type {
		case sax.EventStartSlide:
			got = append(got, "slide+@"+ev.PartName)
		case sax.EventStartShape:
			got = append(got, ev.ShapeKind+"+"+itoa(ev.ShapeID)+"/d"+itoa(ev.Depth))
		case sax.EventStartGroup:
			got = append(got, "group+")
		case sax.EventText:
			got = append(got, "text:"+ev.Text)
		case sax.EventEndGroup:
			got = append(got, "group-")
		case sax.EventEndShape:
			got = append(got, ev.ShapeKind+"-"+itoa(ev.ShapeID)+"/d"+itoa(ev.Depth))
		}
		return nil
	}), sax.Options{})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	want := []string{"slide+@ppt/slides/slide1.xml", "grpSp+10/d0", "group+", "sp+11/d1", "text:inside", "sp-11/d1", "group-", "grpSp-10/d0", "pic+12/d0", "pic-12/d0"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events:\n got %#v\nwant %#v", got, want)
	}
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	b := [24]byte{}
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}
