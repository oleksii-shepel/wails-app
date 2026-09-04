package drawingcss_test

import (
	"fmt"
	"testing"

	"office/pkg/ooxml/drawing"
	drawingcss "office/pkg/render/drawingcss"
)

func TestFillTypesCSS(t *testing.T) {
	tests := []struct {
		name string
		fill drawing.Fill
	}{
		{
			name: "no fill",
			fill: drawing.NoFill{},
		},
		{
			name: "solid fill",
			fill: drawing.SolidFill{Color: "FF0000", Alpha: 1.0},
		},
		{
			name: "solid fill with transparency",
			fill: drawing.SolidFill{Color: "00FF00", Alpha: 0.5},
		},
		{
			name: "gradient fill",
			fill: drawing.GradientFill{
				GradientType: drawing.GradientTypeLinear,
				Angle:        45.0,
				Stops: []drawing.GradientStop{
					{Position: 0.0, Color: "FF0000", Alpha: 1.0},
					{Position: 1.0, Color: "0000FF", Alpha: 1.0},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := drawingcss.Fill(tt.fill)
			if c == nil {
				t.Error("Fill CSS returned nil")
			}
			fmt.Printf("%s: %v\n", tt.name, c)
		})
	}
}

func TestPictureFillCSSEmitsTiling(t *testing.T) {
	fill := drawing.PictureFill{
		ImageURL:    "data:image/png;base64,abc",
		Tile:        true,
		TileScaleX:  50000,
		TileScaleY:  50000,
		TileOffsetX: 12700,
		TileOffsetY: 25400,
	}
	css := drawingcss.Fill(fill)
	if css == nil {
		t.Fatal("Fill CSS returned nil")
	}
	if css["background-image"] == "" {
		t.Fatalf("expected background-image, got %v", css)
	}
	if css["background-repeat"] != "repeat" {
		t.Fatalf("expected background-repeat: repeat, got %v", css)
	}
	if css["background-size"] == "" {
		t.Fatalf("expected background-size, got %v", css)
	}
	if css["background-position"] == "" {
		t.Fatalf("expected background-position, got %v", css)
	}
}
