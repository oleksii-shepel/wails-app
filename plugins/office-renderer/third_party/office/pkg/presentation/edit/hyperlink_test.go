package edit

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestShapeSetHyperlinkURL(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	shape := s.AddTextBox(0, 0, 1000, 1000)
	shape.SetText("click me")

	shape.SetHyperlinkURL("https://example.com")
	if shape.HyperlinkURL() != "https://example.com" {
		t.Fatalf("hyperlink = %q", shape.HyperlinkURL())
	}
	if shape.ActionLink() != "" {
		t.Fatal("action link should be cleared when URL is set")
	}

	shape.SetHyperlinkURL("")
	if shape.HyperlinkURL() != "" {
		t.Fatal("expected hyperlink removed")
	}
}

func TestShapeSetActionLink(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	shape := s.AddTextBox(0, 0, 1000, 1000)
	shape.SetActionLink("nextSlide")
	if shape.ActionLink() != "nextSlide" {
		t.Fatalf("action = %q", shape.ActionLink())
	}
	if shape.HyperlinkURL() != "" {
		t.Fatal("URL should be cleared when action is set")
	}
}

func TestRunSetHyperlinkURL(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	shape := s.AddTextBox(0, 0, 1000, 1000)
	run := shape.AddText("link").Runs()[0]
	run.SetHyperlinkURL("https://example.com/run")
	if run.HyperlinkURL() != "https://example.com/run" {
		t.Fatalf("run hyperlink = %q", run.HyperlinkURL())
	}
	run.RemoveHyperlink()
	if run.HyperlinkURL() != "" {
		t.Fatal("expected hyperlink removed")
	}
}

func TestRoundTripShapeHyperlinkURL(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	shape := s.AddTextBox(0, 0, 1000, 1000)
	shape.SetText("Example")
	shape.SetHyperlinkURL("https://example.com")

	var buf bytes.Buffer
	if err := p.SaveAsPPTX(&buf); err != nil {
		t.Fatalf("SaveAsPPTX: %v", err)
	}

	loaded, err := LoadFromPPTXBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("LoadFromPPTXBytes: %v", err)
	}
	loadedShape := loaded.Slide(0).Shapes()[0]
	if loadedShape.HyperlinkURL() != "https://example.com" {
		t.Fatalf("loaded hyperlink = %q", loadedShape.HyperlinkURL())
	}
	if loadedShape.ActionLink() != "" {
		t.Fatalf("loaded action = %q", loadedShape.ActionLink())
	}
}

func TestRoundTripImageHyperlinkURL(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	shape := s.AddImage(minimalPNG(), "image/png", 0, 0, 1000, 1000)
	shape.SetHyperlinkURL("https://example.com/image")

	var buf bytes.Buffer
	if err := p.SaveAsPPTX(&buf); err != nil {
		t.Fatalf("SaveAsPPTX: %v", err)
	}

	loaded, err := LoadFromPPTXBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("LoadFromPPTXBytes: %v", err)
	}
	loadedShape := loaded.Slide(0).Shapes()[0]
	if loadedShape.HyperlinkURL() != "https://example.com/image" {
		t.Fatalf("loaded hyperlink = %q", loadedShape.HyperlinkURL())
	}
}

func TestRoundTripRunHyperlinkURL(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	shape := s.AddTextBox(0, 0, 1000, 1000)
	shape.AddParagraph().AddRun("link text").SetHyperlinkURL("https://example.com/run")

	var buf bytes.Buffer
	if err := p.SaveAsPPTX(&buf); err != nil {
		t.Fatalf("SaveAsPPTX: %v", err)
	}

	loaded, err := LoadFromPPTXBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("LoadFromPPTXBytes: %v", err)
	}
	loadedRun := loaded.Slide(0).Shapes()[0].Paragraphs()[0].Runs()[0]
	if loadedRun.HyperlinkURL() != "https://example.com/run" {
		t.Fatalf("loaded run hyperlink = %q", loadedRun.HyperlinkURL())
	}
}

func TestRoundTripShapeActionLink(t *testing.T) {
	cases := []struct {
		action string
		want   string
	}{
		{"firstSlide", "firstSlide"},
		{"lastSlide", "lastSlide"},
		{"nextSlide", "nextSlide"},
		{"prevSlide", "prevSlide"},
		{"endShow", "endShow"},
		{"slide3", "slide3"},
	}

	for _, tc := range cases {
		t.Run(tc.action, func(t *testing.T) {
			p := NewPresentation()
			s := p.AddBlankSlide()
			shape := s.AddTextBox(0, 0, 1000, 1000)
			shape.SetActionLink(tc.action)

			var buf bytes.Buffer
			if err := p.SaveAsPPTX(&buf); err != nil {
				t.Fatalf("SaveAsPPTX: %v", err)
			}

			loaded, err := LoadFromPPTXBytes(buf.Bytes())
			if err != nil {
				t.Fatalf("LoadFromPPTXBytes: %v", err)
			}
			loadedShape := loaded.Slide(0).Shapes()[0]
			if loadedShape.ActionLink() != tc.want {
				t.Fatalf("action = %q, want %q", loadedShape.ActionLink(), tc.want)
			}
		})
	}
}

func TestHyperlinkSerializesNamespace(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	shape := s.AddTextBox(0, 0, 1000, 1000)
	shape.SetHyperlinkURL("https://example.com")

	var buf bytes.Buffer
	if err := p.SaveAsPPTX(&buf); err != nil {
		t.Fatalf("SaveAsPPTX: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("zip reader: %v", err)
	}
	var slideXML, relsXML string
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		switch f.Name {
		case "ppt/slides/slide1.xml":
			slideXML = string(data)
		case "ppt/slides/_rels/slide1.xml.rels":
			relsXML = string(data)
		}
	}
	if !strings.Contains(slideXML, "<a:hlinkClick") {
		t.Fatal("expected a:hlinkClick in serialized slide XML")
	}
	if !strings.Contains(slideXML, `r:id="rId`) {
		t.Fatal("expected r:id on hyperlink")
	}
	if !strings.Contains(relsXML, "Type=\"http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink\"") {
		t.Fatal("expected hyperlink relationship type in .rels")
	}
}
