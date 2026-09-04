package resolve

import "testing"

func TestParseVMLStyleExtent(t *testing.T) {
	tests := []struct {
		style string
		wantW int64
		wantH int64
	}{
		{"width:72pt;height:36pt", 914400, 457200},
		{"height:36pt;width:72pt", 914400, 457200},
		{"width:2.54cm;height:2.54cm", 914400, 914400},
		{"width:25.4mm;height:12.7mm", 914400, 457200},
		{"width:1in;height:0.5in", 914400, 457200},
		{"width:96px;height:48px", 914400, 457200},
		{"position:absolute;left:0pt;top:0pt;width:100pt;height:50pt;z-index:1", 1270000, 635000},
		{"", 0, 0},
		{"width:invalid;height:notanumber", 0, 0},
	}
	for _, tc := range tests {
		gotW, gotH := parseVMLStyleExtent(tc.style)
		if gotW != tc.wantW || gotH != tc.wantH {
			t.Errorf("parseVMLStyleExtent(%q) = (%d, %d), want (%d, %d)",
				tc.style, gotW, gotH, tc.wantW, tc.wantH)
		}
	}
}

func TestParseVMLTextBoxVert(t *testing.T) {
	tests := []struct {
		name         string
		shapeStyle   string
		textboxStyle string
		want         string
	}{
		{name: "textbox style top to bottom", textboxStyle: "mso-layout-flow-alt:top-to-bottom", want: "vert"},
		{name: "textbox style bottom to top", textboxStyle: "mso-layout-flow-alt:bottom-to-top", want: "vert270"},
		{name: "shape style vertical", shapeStyle: "layout-flow:vertical", want: "vert"},
		{name: "horizontal default", textboxStyle: "layout-flow:horizontal", want: ""},
	}
	for _, tc := range tests {
		if got := parseVMLTextBoxVert(tc.shapeStyle, tc.textboxStyle); got != tc.want {
			t.Fatalf("%s: parseVMLTextBoxVert(%q, %q) = %q, want %q", tc.name, tc.shapeStyle, tc.textboxStyle, got, tc.want)
		}
	}
}