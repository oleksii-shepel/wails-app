package render

import (
	"strings"
	"testing"

	"office/pkg/officeart/legacy"
	"office/pkg/officeart/legacy/escher"
)

func TestLegacyHTMLUsesSharedRenderer(t *testing.T) {
	node := &escher.ShapeNode{
		ShapeID: 7,
		Properties: []escher.Property{
			{ID: 0x0104, Value: 1},
		},
	}
	store := legacy.BStore{
		1: legacy.BSEntry{
			Media: legacy.Media{
				MIME: "image/png",
				Data: []byte{1, 2, 3},
			},
		},
	}
	html, err := LegacyHTML(node, store)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "data:image/png;base64,") {
		t.Fatalf("html = %q", html)
	}
}

func TestLegacyHTMLRendersSharedWordArtTextAndStyle(t *testing.T) {
	node := &escher.ShapeNode{
		ShapeID:   92,
		ShapeType: 1,
		Properties: []escher.Property{
			{ID: escher.PropGTextUnicode, IsComplex: true, Complex: []byte{'W', 0, 'o', 0, 'r', 0, 'd', 0, 'A', 0, 'r', 0, 't', 0, 0, 0}},
			{ID: escher.PropGTextFont, IsComplex: true, Complex: []byte{'A', 0, 'r', 0, 'i', 0, 'a', 0, 'l', 0, 0, 0}},
			{ID: escher.PropGTextSize, Value: 24 << 16},
		},
	}
	out, err := LegacyHTML(node, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"WordArt", `font-family:&#34;Arial&#34;`, "font-size:24.00pt"} {
		if !strings.Contains(out, want) {
			t.Fatalf("html %q missing %q", out, want)
		}
	}
}
