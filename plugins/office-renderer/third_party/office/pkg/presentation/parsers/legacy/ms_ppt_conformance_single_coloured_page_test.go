package legacy

import (
    "path/filepath"
    "testing"
)

// MS-PPT conformance checkpoint for the smallest style-bearing legacy PPT fixture.
//
// The fixture exercises the normative text path:
//   StyleTextPropAtom -> TextCFRun -> TextCFException -> CFMasks/CFStyle/ColorIndexStruct.
// It intentionally asserts parser IR, not renderer HTML, so a renderer fallback cannot
// hide a character-run decoding regression.
func TestMSPPTConformanceSingleColouredPageCharacterRuns(t *testing.T) {
    path := filepath.Join("..", "..", "..", "..", "testdata", "ppt", "Single_Coloured_Page.ppt")
    pres, err := ParsePpt(path)
    if err != nil {
        t.Fatalf("ParsePpt: %v", err)
    }
    if len(pres.Slides) != 1 {
        t.Fatalf("slides = %d, want 1", len(pres.Slides))
    }

    var subtitleRuns []struct {
        text, color string
        bold, italic bool
    }
    for _, shape := range pres.Slides[0].Shapes {
        if shape == nil || len(shape.Paragraphs) != 3 {
            continue
        }
        if paragraphText(shape.Paragraphs[0]) != "This is the subtitle, in bold" {
            continue
        }
        for _, para := range shape.Paragraphs {
            if len(para.Runs) != 1 || para.Runs[0] == nil {
                t.Fatalf("paragraph %q runs = %#v, want exactly one run", paragraphText(para), para.Runs)
            }
            r := para.Runs[0]
            subtitleRuns = append(subtitleRuns, struct {
                text, color string
                bold, italic bool
            }{r.Text, r.Color, r.BoldSet && r.Bold, r.ItalicSet && r.Italic})
        }
        break
    }
    if len(subtitleRuns) != 3 {
        t.Fatalf("subtitle run count = %d, want 3", len(subtitleRuns))
    }

    want := []struct {
        text, color string
        bold, italic bool
    }{
        {"This is the subtitle, in bold", "000000", true, false},
        {"This bit is blue and italic", "333399", false, true},
        {"This bit is red (normal)", "FF3300", false, false},
    }
    for i := range want {
        if subtitleRuns[i] != want[i] {
            t.Fatalf("run %d = %#v, want %#v", i, subtitleRuns[i], want[i])
        }
    }
}

// MS-PPT 2.12.2 ColorIndexStruct: 0xFE selects the RGB bytes, 0x00..0x07
// select the slide color scheme, and 0xFF is undefined.
func TestMSPPTColorIndexStructResolution(t *testing.T) {
    scheme := map[uint8]string{0: "112233", 1: "445566", 7: "AABBCC"}
    tests := []struct {
        name string
        r, g, b, index byte
        want string
    }{
        {"srgb", 0x33, 0x33, 0x99, 0xFE, "333399"},
        {"scheme-background", 0, 0, 0, 0x00, "112233"},
        {"scheme-text", 0, 0, 0, 0x01, "445566"},
        {"scheme-accent3", 0, 0, 0, 0x07, "AABBCC"},
        {"undefined", 0xFF, 0x00, 0x00, 0xFF, ""},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if got := resolveLegacyPptColorRef(tt.r, tt.g, tt.b, tt.index, scheme); got != tt.want {
                t.Fatalf("resolveLegacyPptColorRef(%02X,%02X,%02X,%02X) = %q, want %q", tt.r, tt.g, tt.b, tt.index, got, tt.want)
            }
        })
    }
}
