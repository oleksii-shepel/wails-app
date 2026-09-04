package edit

import (
	"bytes"
	"testing"

	presir "office/pkg/presentation/ir"
)

func TestThemeEditAPI(t *testing.T) {
	p := NewPresentation()
	if p.Theme() != nil {
		t.Fatal("expected no theme by default")
	}

	theme := p.AddTheme()
	if theme == nil {
		t.Fatal("expected theme")
	}
	theme.SetName("Office Theme")
	theme.SetColor("dk1", "000000")
	theme.SetColor("lt1", "FFFFFF")
	theme.SetColor("accent1", "4472C4")
	theme.SetMajorFont("Calibri Light")
	theme.SetMinorFont("Calibri")

	if p.Theme().Name() != "Office Theme" {
		t.Fatalf("theme name = %q", p.Theme().Name())
	}
	if p.Theme().Color("accent1") != "4472C4" {
		t.Fatalf("accent1 = %q", p.Theme().Color("accent1"))
	}
	if p.Theme().MajorFont() != "Calibri Light" {
		t.Fatalf("major font = %q", p.Theme().MajorFont())
	}

	p.SetTheme(nil)
	if p.Theme() != nil {
		t.Fatal("expected theme removed")
	}
}

func TestRoundTripTheme(t *testing.T) {
	p := NewPresentation()
	theme := p.AddTheme()
	theme.SetName("Custom Theme")
	theme.SetColor("dk1", "111111")
	theme.SetColor("hlink", "0000FF")
	theme.SetMajorFont("Arial")
	theme.SetMinorFont("Times New Roman")

	var buf bytes.Buffer
	if err := p.SaveAsPPTX(&buf); err != nil {
		t.Fatalf("SaveAsPPTX: %v", err)
	}

	loaded, err := LoadFromPPTXBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("LoadFromPPTXBytes: %v", err)
	}
	lt := loaded.Theme()
	if lt == nil {
		t.Fatal("expected theme after load")
	}
	if lt.Name() != "Custom Theme" {
		t.Fatalf("theme name = %q", lt.Name())
	}
	if lt.Color("dk1") != "111111" {
		t.Fatalf("dk1 = %q", lt.Color("dk1"))
	}
	if lt.Color("hlink") != "0000FF" {
		t.Fatalf("hlink = %q", lt.Color("hlink"))
	}
	if lt.MajorFont() != "Arial" {
		t.Fatalf("major font = %q", lt.MajorFont())
	}
	if lt.MinorFont() != "Times New Roman" {
		t.Fatalf("minor font = %q", lt.MinorFont())
	}
}

func TestPlaceholderDetailsEdit(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	shape := s.AddPlaceholder(presir.PlaceholderBody, 0, 0, 1000, 1000)

	shape.SetPlaceholderIdx(3)
	shape.SetPlaceholderOrient("vert")
	shape.SetPlaceholderSize("half")
	shape.SetPlaceholderHasCustomPrompt(true)

	if shape.PlaceholderIdx() != 3 {
		t.Fatalf("idx = %d", shape.PlaceholderIdx())
	}
	if shape.PlaceholderOrient() != "vert" {
		t.Fatalf("orient = %q", shape.PlaceholderOrient())
	}
	if shape.PlaceholderSize() != "half" {
		t.Fatalf("size = %q", shape.PlaceholderSize())
	}
	if !shape.PlaceholderHasCustomPrompt() {
		t.Fatal("expected custom prompt")
	}
}

func TestRoundTripPlaceholderDetails(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	shape := s.AddPlaceholder(presir.PlaceholderTitle, 0, 0, 1000, 1000)
	shape.SetPlaceholderIdx(1)
	shape.SetPlaceholderOrient("vert")
	shape.SetPlaceholderSize("quarter")
	shape.SetPlaceholderHasCustomPrompt(true)

	var buf bytes.Buffer
	if err := p.SaveAsPPTX(&buf); err != nil {
		t.Fatalf("SaveAsPPTX: %v", err)
	}

	loaded, err := LoadFromPPTXBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("LoadFromPPTXBytes: %v", err)
	}
	loadedShape := loaded.Slide(0).Shapes()[0]
	if loadedShape.Placeholder() != presir.PlaceholderTitle {
		t.Fatalf("kind = %v", loadedShape.Placeholder())
	}
	if loadedShape.PlaceholderIdx() != 1 {
		t.Fatalf("idx = %d", loadedShape.PlaceholderIdx())
	}
	if loadedShape.PlaceholderOrient() != "vert" {
		t.Fatalf("orient = %q", loadedShape.PlaceholderOrient())
	}
	if loadedShape.PlaceholderSize() != "quarter" {
		t.Fatalf("size = %q", loadedShape.PlaceholderSize())
	}
	if !loadedShape.PlaceholderHasCustomPrompt() {
		t.Fatal("expected custom prompt after load")
	}
}

func TestNewPlaceholderKinds(t *testing.T) {
	cases := []struct {
		kind presir.PlaceholderKind
		want string
	}{
		{presir.PlaceholderHeader, "hdr"},
		{presir.PlaceholderContent, "obj"},
		{presir.PlaceholderChart, "chart"},
		{presir.PlaceholderTable, "tbl"},
		{presir.PlaceholderPicture, "pic"},
		{presir.PlaceholderMedia, "media"},
		{presir.PlaceholderClipArt, "clipArt"},
		{presir.PlaceholderDiagram, "dgm"},
		{presir.PlaceholderSlideImage, "sldImg"},
	}

	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			p := NewPresentation()
			s := p.AddBlankSlide()
			_ = s.AddPlaceholder(tc.kind, 0, 0, 1000, 1000)

			var buf bytes.Buffer
			if err := p.SaveAsPPTX(&buf); err != nil {
				t.Fatalf("SaveAsPPTX: %v", err)
			}

			loaded, err := LoadFromPPTXBytes(buf.Bytes())
			if err != nil {
				t.Fatalf("LoadFromPPTXBytes: %v", err)
			}
			loadedShape := loaded.Slide(0).Shapes()[0]
			if loadedShape.Placeholder() != tc.kind {
				t.Fatalf("kind = %v, want %v", loadedShape.Placeholder(), tc.kind)
			}
		})
	}
}

func TestPlaceholderDefaultNames(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	if shape := s.AddPlaceholder(presir.PlaceholderChart, 0, 0, 1000, 1000); shape.Name() != "Chart Placeholder 8" {
		t.Fatalf("name = %q", shape.Name())
	}
}