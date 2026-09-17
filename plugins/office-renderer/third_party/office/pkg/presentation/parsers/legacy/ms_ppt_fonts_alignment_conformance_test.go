package legacy

import (
    "path/filepath"
    "testing"

    presentationir "office/pkg/presentation/ir"
)

// MS-PPT fixture conformance for StyleTextPropAtom paragraph/character runs.
// This fixture deliberately mixes left/center/right alignment, RGB colors,
// decoration, typeface changes and a local font-size override in one subtitle
// text body.  The assertions are on canonical IR, before layout/rendering.
func TestMSPPTFontsAndAlignmentsFixturePreservesLocalTextFormatting(t *testing.T) {
    path := filepath.Join("..", "..", "..", "..", "testdata", "ppt", "apache-Single_Coloured_Page_With_Fonts_and_Alignments.ppt")
    pres, err := ParsePpt(path)
    if err != nil {
        t.Fatalf("ParsePpt: %v", err)
    }
    if len(pres.Slides) != 1 {
        t.Fatalf("slides = %d, want 1", len(pres.Slides))
    }

    paragraphs := map[string]*presentationir.Paragraph{}
    for _, shape := range pres.Slides[0].Shapes {
        collectMSPPTFixtureParagraphs(shape, paragraphs)
    }

    assert := func(text, align, family, color string, size int, bold, italic, underline bool) {
        t.Helper()
        p := paragraphs[text]
        if p == nil {
            t.Fatalf("missing paragraph %q", text)
        }
        if p.Align != align {
            t.Fatalf("%q alignment = %q, want %q", text, p.Align, align)
        }
        if len(p.Runs) != 1 || p.Runs[0] == nil {
            t.Fatalf("%q runs = %#v, want one run", text, p.Runs)
        }
        r := p.Runs[0]
        if r.FontSize != size || r.FontFamily != family || r.Color != color || r.Bold != bold || r.Italic != italic || r.Underline != underline {
            t.Fatalf("%q run = {size:%d family:%q color:%q bold:%v italic:%v underline:%v}, want {size:%d family:%q color:%q bold:%v italic:%v underline:%v}",
                text, r.FontSize, r.FontFamily, r.Color, r.Bold, r.Italic, r.Underline,
                size, family, color, bold, italic, underline)
        }
    }

    assert("This is a title, it’s in black", "r", "Arial", "000000", 4400, false, false, false)
    assert("This is the subtitle, in bold", "l", "Arial", "000000", 2000, true, false, false)
    assert("This bit is blue and italic", "ctr", "Arial", "333399", 2000, false, true, false)
    assert("This bit is red (normal)", "r", "Arial", "FF3300", 2000, false, false, false)
    assert("This bit is underlined, and in Times New Roman, but at a larger font size of 24 (rather than 20)", "l", "Times New Roman", "000000", 2400, false, false, true)
}

func collectMSPPTFixtureParagraphs(shape *presentationir.Shape, out map[string]*presentationir.Paragraph) {
    if shape == nil {
        return
    }
    for _, p := range shape.Paragraphs {
        if p == nil {
            continue
        }
        out[paragraphText(p)] = p
    }
    for _, child := range shape.Children {
        collectMSPPTFixtureParagraphs(child, out)
    }
}
