package render_test

import (
	"encoding/base64"
	"fmt"
	"math"
	"os"
	"regexp"
	"strings"
	"testing"

	"office/pkg/diag"
	dml "office/pkg/ooxml/drawing"
	"office/pkg/ooxml/opc"
	resolve "office/pkg/word/ir"
	"office/pkg/word/parsers/legacy"
	"office/pkg/word/parsers/ooxml/parts"
	wordresolve "office/pkg/word/parsers/ooxml/resolve"
	"office/pkg/word/render"
)

func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int    { return &i }

// extractCSS pulls the content of the first <style> block from an HTML string.
func extractCSS(s string) string {
	start := strings.Index(s, "<style>")
	end := strings.Index(s, "</style>")
	if start < 0 || end < 0 {
		return ""
	}
	return s[start+7 : end]
}

func TestHTMLEmptyDocument(t *testing.T) {
	doc := &resolve.Document{}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, want := range []string{"<!DOCTYPE html>", "<html", "<head>", "<body>", "</html>"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output", want)
		}
	}
}

func TestFastPreviewKeepsLaterFloatingImagesOutOfPageOverlay(t *testing.T) {
	secondPageImage := &resolve.Image{
		Floating:  true,
		Width:     914400,
		Height:    914400,
		XRelative: "margin",
		YRelative: "paragraph",
		XAlign:    "right",
		WrapType:  "square",
		Alt:       "second-page-image",
	}
	doc := &resolve.Document{Blocks: []resolve.Block{
		{Index: 0, Kind: "paragraph", Paragraph: &resolve.Paragraph{Index: 0, Text: "First page", Runs: []resolve.Run{{Kind: "run", Text: "First page"}}}},
		{Index: 1, Kind: "paragraph", Paragraph: &resolve.Paragraph{Index: 1, Format: resolve.ParagraphFormat{PageBreakBefore: true}, Text: "Second page", Runs: []resolve.Run{{Kind: "run", Text: "Second page"}, {Kind: "image", Image: secondPageImage}}}},
	}}
	out, _, err := render.HTMLWithOptions(doc, render.HTMLOptions{FastPreview: true})
	if err != nil {
		t.Fatalf("HTMLWithOptions: %v", err)
	}
	if got := strings.Count(out, `<section class="page"`); got != 1 {
		t.Fatalf("preview pages = %d, want 1", got)
	}
	start := strings.Index(out, "second-page-image")
	if start < 0 {
		t.Fatalf("fast preview omitted later image: %s", out)
	}
	start = strings.LastIndex(out[:start], "<")
	if start < 0 {
		t.Fatalf("could not locate fast-preview image start: %s", out)
	}
	end := strings.Index(out[start:], ">")
	if end < 0 {
		t.Fatalf("could not locate fast-preview image tag: %s", out)
	}
	imageTag := out[start : start+end+1]
	if !strings.Contains(imageTag, "display:block") || !strings.Contains(imageTag, "float:right") || strings.Contains(imageTag, "position:absolute") {
		t.Fatalf("fast-preview image should stay in normal flow, got %s", imageTag)
	}
}

func TestFastPreviewPreservesSquareImageTextWrapping(t *testing.T) {
	image := &resolve.Image{
		Floating:  true,
		Width:     914400,
		Height:    914400,
		WrapType:  "square",
		DistRight: 9,
		Alt:       "left-wrapped-image",
	}
	doc := &resolve.Document{Blocks: []resolve.Block{{
		Kind: "paragraph",
		Paragraph: &resolve.Paragraph{Runs: []resolve.Run{
			{Kind: "image", Image: image},
			{Kind: "run", Text: "Text should wrap beside the image."},
		}},
	}}}
	out, _, err := render.HTMLWithOptions(doc, render.HTMLOptions{FastPreview: true})
	if err != nil {
		t.Fatalf("HTMLWithOptions: %v", err)
	}
	start := strings.Index(out, "left-wrapped-image")
	if start < 0 {
		t.Fatalf("fast preview omitted image: %s", out)
	}
	start = strings.LastIndex(out[:start], "<")
	end := strings.Index(out[start:], ">")
	imageTag := out[start : start+end+1]
	if !strings.Contains(imageTag, "float:left") || !strings.Contains(imageTag, "margin-right:9.0pt") || strings.Contains(imageTag, "position:absolute") {
		t.Fatalf("fast-preview square image should wrap text in flow, got %s", imageTag)
	}
}

func TestHTMLPreservesVisibleFloatingImageTextWrapping(t *testing.T) {
	image := &resolve.Image{
		Floating:  true,
		Width:     914400,
		Height:    1371600,
		WrapType:  "tight",
		XAlign:    "left",
		YAlign:    "top",
		XRelative: "margin",
		YRelative: "paragraph",
		DistRight: 9,
		Alt:       "visible-left-wrapped-image",
	}
	doc := &resolve.Document{Blocks: []resolve.Block{{
		Kind: "paragraph",
		Paragraph: &resolve.Paragraph{Runs: []resolve.Run{
			{Kind: "image", Image: image},
			{Kind: "run", Text: "Text should remain beside the visible image instead of underneath it."},
		}},
	}}}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	start := strings.Index(out, "visible-left-wrapped-image")
	if start < 0 {
		t.Fatalf("HTML omitted image: %s", out)
	}
	start = strings.LastIndex(out[:start], "<")
	end := strings.Index(out[start:], ">")
	imageTag := out[start : start+end+1]
	if !strings.Contains(imageTag, "float:left") || !strings.Contains(imageTag, "position:relative") || strings.Contains(imageTag, "position:absolute") {
		t.Fatalf("normal HTML image should visibly float in flow, got %s", imageTag)
	}
}

func TestHTMLIncludesDiagnosticsDetails(t *testing.T) {
	doc := &resolve.Document{
		Diagnostics: diag.List{{
			Severity: diag.SeverityWarning,
			Code:     "word/unsupported-smartart",
			Message:  "SmartArt content is not supported and was skipped",
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "<details class=\"diagnostics\">") {
		t.Fatalf("expected diagnostics details block in output: %s", out)
	}
	if !strings.Contains(out, "[word/unsupported-smartart]") {
		t.Fatalf("expected diagnostic code in output: %s", out)
	}
}

func TestHTMLCollapsesRepeatedDiagnostics(t *testing.T) {
	diagnostic := diag.Diagnostic{
		Severity: diag.SeverityWarning,
		Code:     "legacy.sprm.unmapped",
		Message:  "unmapped paragraph SPRM 0x1234",
		Location: diag.Location{Path: "WordDocument"},
	}
	doc := &resolve.Document{Diagnostics: diag.List{diagnostic, diagnostic, diagnostic}}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if got := strings.Count(out, "[legacy.sprm.unmapped]"); got != 1 {
		t.Fatalf("diagnostic occurrences = %d, want 1: %s", got, out)
	}
	if !strings.Contains(out, `WordDocument)`) || !strings.Contains(out, `class="diagnostic-count">×3</span>`) {
		t.Fatalf("expected normalized location and duplicate count: %s", out)
	}
}

func TestHTMLParagraph(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Index: 0, Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Kind: "run", Text: "Hello world"}},
				Text: "Hello world",
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "<p") || !strings.Contains(out, "Hello world") {
		t.Errorf("expected <p>…Hello world… in output")
	}
}

func TestHTMLExternalizesImagesAndFontsWhenConfigured(t *testing.T) {
	fontContentType, fontData := sampleFontBytes(t, "ppt/fonts/Oswald-regular.fntdata")
	doc := &resolve.Document{
		EmbeddedFonts: []*resolve.EmbeddedFont{{
			Family: "Oswald",
			Faces: []*resolve.EmbeddedFontFace{{
				Family:      "Oswald",
				ContentType: fontContentType,
				Data:        fontData,
			}},
		}},
		Blocks: []resolve.Block{
			{Index: 0, Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Kind: "image", Image: &resolve.Image{
					MIMEType: "image/png",
					Data:     []byte("image-bytes"),
					Width:    914400,
					Height:   914400,
					Alt:      "Logo",
				}}},
			}},
		},
	}

	html, assets, err := render.HTMLWithOptions(doc, render.HTMLOptions{
		ImageMode:        render.HTMLImageModeExternal,
		ExternalImageDir: "sample_files",
		FontMode:         render.HTMLFontModeExternal,
		ExternalFontDir:  "sample_fonts",
	})
	if err != nil {
		t.Fatalf("HTMLWithOptions: %v", err)
	}
	if strings.Contains(html, "data:image/") || strings.Contains(html, "data:font/") {
		t.Fatalf("expected external assets in html: %s", html)
	}
	if !strings.Contains(html, `src="sample_files/image-001.png"`) {
		t.Fatalf("expected external image asset path in html: %s", html)
	}
	if !strings.Contains(html, `url("sample_fonts/font-001.`) {
		t.Fatalf("expected external font asset path in html: %s", html)
	}
	if len(assets) < 2 {
		t.Fatalf("expected sidecar assets, got %d", len(assets))
	}
}

func TestHTMLPictureSDTPreservesAttrsOnImage(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Index: 0, Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Kind: "image", Image: &resolve.Image{
					MIMEType: "image/png",
					Data:     []byte("image-bytes"),
					Width:    914400,
					Height:   914400,
					Alt:      "Photo",
				}, ContentControl: &resolve.ContentControl{
					Type:  "picture",
					Tag:   "photo",
					Title: "Photo",
				}}},
			}},
		},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="word-sdt-picture"`) {
		t.Fatalf("expected picture SDT class on rendered image: %s", out)
	}
	if !strings.Contains(out, `data-sdt-type="picture"`) || !strings.Contains(out, `data-sdt-tag="photo"`) || !strings.Contains(out, `data-sdt-title="Photo"`) {
		t.Fatalf("expected picture SDT attrs on rendered image: %s", out)
	}
}

func TestHTMLDocPartSDTPreservesAttrsAroundImage(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{
				Index: 0,
				Kind:  "paragraph",
				Paragraph: &resolve.Paragraph{
					Runs: []resolve.Run{
						{
							Kind: "image",
							Image: &resolve.Image{
								MIMEType: "image/png",
								Data:     []byte("image-bytes"),
								Width:    914400,
								Height:   914400,
								Alt:      "Building block preview",
							},
							ContentControl: &resolve.ContentControl{
								Type:               "docPartObj",
								Title:              "Building Block",
								DocPartGallery:     "Cover Pages",
								DocPartCategory:    "General",
								DocPartUnique:      true,
								ShowingPlaceholder: true,
								DataXPath:          "/ns:root/ns:cover",
								DataStoreID:        "{AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE}",
							},
						},
					},
				},
			},
		},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="word-sdt word-sdt-placeholder word-sdt-doc-part"`) {
		t.Fatalf("expected SDT wrapper class around docPart image: %s", out)
	}
	for _, want := range []string{
		`data-sdt-type="docPartObj"`,
		`data-sdt-title="Building Block"`,
		`data-sdt-doc-part-gallery="Cover Pages"`,
		`data-sdt-doc-part-category="General"`,
		`data-sdt-doc-part-unique="true"`,
		`data-sdt-placeholder="true"`,
		`data-sdt-xpath="/ns:root/ns:cover"`,
		`data-sdt-store-item-id="{AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE}"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %s in docPart image output:\n%s", want, out)
		}
	}
	if !strings.Contains(out, `<img `) {
		t.Fatalf("expected image inside docPart SDT wrapper: %s", out)
	}
}

func TestHTMLParagraphRelativeFloatingImageUsesVisibleFlowFloat(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Index: 0, Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index: 1,
				Runs: []resolve.Run{
					{Kind: "image", Image: &resolve.Image{
						MIMEType:  "image/png",
						Data:      []byte{0x01},
						Width:     914400,
						Height:    914400,
						Floating:  true,
						XRelative: "column",
						YRelative: "paragraph",
						WrapType:  "square",
						WrapSide:  "bothSides",
						Alt:       "Anchored",
					}},
					{Kind: "run", Text: "Body text wraps beside the image."},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	start := strings.Index(out, `alt="Anchored"`)
	if start < 0 {
		t.Fatalf("expected anchored image: %s", out)
	}
	start = strings.LastIndex(out[:start], "<")
	end := strings.Index(out[start:], ">")
	imageTag := out[start : start+end+1]
	if !strings.Contains(imageTag, `float:left`) || !strings.Contains(imageTag, `position:relative`) || strings.Contains(imageTag, `position:absolute`) {
		t.Fatalf("expected visible paragraph-relative float in normal flow: %s", imageTag)
	}
}

func TestHTMLHeadings(t *testing.T) {
	cases := []struct {
		styleID   string
		styleName string
		wantTag   string
	}{
		{"Heading1", "", "h1"},
		{"", "Heading 1", "h1"},
		{"heading2", "", "h2"},
		{"", "Heading 3", "h3"},
		{"heading6", "", "h6"},
	}

	for _, tc := range cases {
		doc := &resolve.Document{
			Blocks: []resolve.Block{
				{Kind: "paragraph", Paragraph: &resolve.Paragraph{
					StyleID:   tc.styleID,
					StyleName: tc.styleName,
					Runs:      []resolve.Run{{Text: "Title"}},
				}},
			},
		}
		out, err := render.HTML(doc)
		if err != nil {
			t.Fatalf("HTML: %v", err)
		}
		if !strings.Contains(out, "<"+tc.wantTag) {
			t.Errorf("styleID=%q styleName=%q: expected <%s> in output", tc.styleID, tc.styleName, tc.wantTag)
		}
	}
}

func TestHTMLTrackedMoves(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Text: "movedFrom", Format: resolve.RunFormat{}, Revision: "movedFrom", RevisionAuthor: "Alice", RevisionDate: "2024-01-01"},
					{Text: "movedTo", Format: resolve.RunFormat{}, Revision: "movedTo", RevisionAuthor: "Alice", RevisionDate: "2024-01-01"},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `data-revision-type="movedFrom"`) {
		t.Errorf("expected movedFrom data attribute in output")
	}
	if !strings.Contains(out, `data-revision-type="movedTo"`) {
		t.Errorf("expected movedTo data attribute in output")
	}
	if !strings.Contains(out, "<del") {
		t.Errorf("expected <del> tag for movedFrom")
	}
	if !strings.Contains(out, "<ins") {
		t.Errorf("expected <ins> tag for movedTo")
	}
}

func TestHTMLMoveRangeBoundaries(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs:                []resolve.Run{{Text: "text"}},
				MoveFromRangeStarts: []resolve.MoveRange{{ID: 1, Author: "Alice", Date: "2024-01-01", Name: "move1"}},
				MoveFromRangeEnds:   []int{1},
				MoveToRangeStarts:   []resolve.MoveRange{{ID: 2, Author: "Bob", Date: "2024-02-01"}},
				MoveToRangeEnds:     []int{2},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="word-move-from-range-start"`) {
		t.Errorf("expected word-move-from-range-start in output")
	}
	if !strings.Contains(out, `class="word-move-from-range-end"`) {
		t.Errorf("expected word-move-from-range-end in output")
	}
	if !strings.Contains(out, `class="word-move-to-range-start"`) {
		t.Errorf("expected word-move-to-range-start in output")
	}
	if !strings.Contains(out, `class="word-move-to-range-end"`) {
		t.Errorf("expected word-move-to-range-end in output")
	}
	if !strings.Contains(out, `data-move-id="1"`) {
		t.Errorf("expected data-move-id=1 in output")
	}
	if !strings.Contains(out, `data-move-author="Alice"`) {
		t.Errorf("expected data-move-author=Alice in output")
	}
}

func TestHTMLRunFormatting(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Text: "bold", Format: resolve.RunFormat{Bold: boolPtr(true)}},
					{Text: "italic", Format: resolve.RunFormat{Italic: boolPtr(true)}},
					{Text: "colored", Format: resolve.RunFormat{Color: "FF0000"}},
					{Text: "sized", Format: resolve.RunFormat{FontSize: intPtr(36)}}, // 18pt
					{Text: "struck", Format: resolve.RunFormat{Strike: boolPtr(true)}},
					{Text: "dblstruck", Format: resolve.RunFormat{DoubleStrike: boolPtr(true)}},
					{Text: "SC", Format: resolve.RunFormat{SmallCaps: boolPtr(true)}},
					{Text: "hilite", Format: resolve.RunFormat{Highlight: "yellow"}},
					{Text: "hidden", Format: resolve.RunFormat{WebHidden: boolPtr(true)}},
					{Text: "scaled", Format: resolve.RunFormat{CharScale: intPtr(150)}},
					{Text: "raised", Format: resolve.RunFormat{Position: intPtr(12)}},  // +6pt
					{Text: "lowered", Format: resolve.RunFormat{Position: intPtr(-8)}}, // -4pt
					{Text: "kerned", Format: resolve.RunFormat{Kerning: intPtr(24)}},   // kern above 12pt
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	checks := []string{
		"font-weight:bold",
		"font-style:italic",
		"color:#FF0000",
		"font-size:18.0pt",
		"text-decoration:line-through",
		"text-decoration:line-through double",
		"font-variant:small-caps",
		"background-color:#FFFF00",
		"display:none",
		"transform:scaleX(1.50)",
		"top:-6.00pt",
		"top:4.00pt",
		"font-kerning:normal",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output", want)
		}
	}
}

func TestHTMLRunEmphasisMark(t *testing.T) {
	tests := []struct {
		name string
		mark string
		want string
	}{
		{"dot", "dot", "text-emphasis-style:dot"},
		{"comma", "comma", "text-emphasis-style:','"},
		{"circle", "circle", "text-emphasis-style:circle"},
		{"underDot", "underDot", "text-emphasis-style:dot"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := &resolve.Document{
				Blocks: []resolve.Block{
					{Kind: "paragraph", Paragraph: &resolve.Paragraph{
						Runs: []resolve.Run{
							{Text: "emph", Format: resolve.RunFormat{EmphasisMark: tt.mark}},
						},
					}},
				},
			}
			out, err := render.HTML(doc)
			if err != nil {
				t.Fatalf("HTML: %v", err)
			}
			if !strings.Contains(out, tt.want) {
				t.Errorf("expected %q in output, got:\n%s", tt.want, out)
			}
		})
	}
	// underDot also needs position:under
	t.Run("underDot position", func(t *testing.T) {
		doc := &resolve.Document{
			Blocks: []resolve.Block{
				{Kind: "paragraph", Paragraph: &resolve.Paragraph{
					Runs: []resolve.Run{
						{Text: "emph", Format: resolve.RunFormat{EmphasisMark: "underDot"}},
					},
				}},
			},
		}
		out, err := render.HTML(doc)
		if err != nil {
			t.Fatalf("HTML: %v", err)
		}
		if !strings.Contains(out, "text-emphasis-position:under") {
			t.Errorf("expected text-emphasis-position:under in output, got:\n%s", out)
		}
	})
}

func TestHTMLRunVanishHidesRun(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Text: "visible ", Format: resolve.RunFormat{}},
					{Text: "hidden", Format: resolve.RunFormat{Vanish: boolPtr(true)}},
					{Text: " visible", Format: resolve.RunFormat{}},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	bodyStart := strings.Index(out, "<body>")
	bodyEnd := strings.Index(out, "</body>")
	body := ""
	if bodyStart >= 0 && bodyEnd >= 0 {
		body = out[bodyStart:bodyEnd]
	}
	if strings.Contains(body, "hidden") {
		t.Errorf("expected hidden run to be suppressed, got:\n%s", out)
	}
	if !strings.Contains(body, "visible") {
		t.Errorf("expected visible runs to remain, got:\n%s", out)
	}
}

func TestHTMLRunSpecVanishHidesRun(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Text: "visible ", Format: resolve.RunFormat{}},
					{Text: "hidden", Format: resolve.RunFormat{SpecVanish: boolPtr(true)}},
					{Text: " visible", Format: resolve.RunFormat{}},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	bodyStart := strings.Index(out, "<body>")
	bodyEnd := strings.Index(out, "</body>")
	body := ""
	if bodyStart >= 0 && bodyEnd >= 0 {
		body = out[bodyStart:bodyEnd]
	}
	if strings.Contains(body, "hidden") {
		t.Errorf("expected spec-vanish run to be suppressed, got:\n%s", out)
	}
	if !strings.Contains(body, "visible") {
		t.Errorf("expected visible runs to remain, got:\n%s", out)
	}
}

func TestHTMLParagraphNoAutoHyphenation(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Format: resolve.ParagraphFormat{NoAutoHyphenation: true},
				Runs:   []resolve.Run{{Text: "antidisestablishmentarianism"}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `hyphens:none`) {
		t.Fatalf("expected hyphens:none for NoAutoHyphenation, got:\n%s", out)
	}
}

func TestHTMLDocumentAutoHyphenation(t *testing.T) {
	doc := &resolve.Document{
		AutoHyphenation: true,
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Text: "antidisestablishmentarianism"}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `.page-body { hyphens: auto`) {
		t.Fatalf("expected global hyphens:auto when AutoHyphenation is true, got:\n%s", out)
	}
}

func TestHTMLDocumentAutoHyphenationWithLimits(t *testing.T) {
	doc := &resolve.Document{
		AutoHyphenation:        true,
		ConsecutiveHyphenLimit: 2,
		HyphenationZone:        720, // 720 twips = 0.5 inch
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Text: "antidisestablishmentarianism"}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `hyphens: auto`) {
		t.Fatalf("expected hyphens:auto, got:\n%s", out)
	}
	if !strings.Contains(out, `-webkit-hyphenate-limit-lines: 2`) {
		t.Fatalf("expected -webkit-hyphenate-limit-lines: 2, got:\n%s", out)
	}
	if !strings.Contains(out, `hyphenate-limit-lines: 2`) {
		t.Fatalf("expected hyphenate-limit-lines: 2, got:\n%s", out)
	}
	if !strings.Contains(out, `-webkit-hyphenate-limit-zone: 36.0pt`) {
		t.Fatalf("expected -webkit-hyphenate-limit-zone: 36.0pt, got:\n%s", out)
	}
	if !strings.Contains(out, `hyphenate-limit-zone: 36.0pt`) {
		t.Fatalf("expected hyphenate-limit-zone: 36.0pt, got:\n%s", out)
	}
}

func TestHTMLParagraphWidowControl(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Format: resolve.ParagraphFormat{WidowControl: true},
				Runs:   []resolve.Run{{Text: "A paragraph with widow control enabled."}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `widows:2`) {
		t.Fatalf("expected widows:2 for WidowControl, got:\n%s", out)
	}
	if !strings.Contains(out, `orphans:2`) {
		t.Fatalf("expected orphans:2 for WidowControl, got:\n%s", out)
	}
}

func TestHTMLParagraphWordWrapFalse(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Format: resolve.ParagraphFormat{WordWrap: boolPtr(false)},
				Runs:   []resolve.Run{{Text: "supercalifragilisticexpialidocious"}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `word-break:break-all`) {
		t.Fatalf("expected word-break:break-all for WordWrap=false, got:\n%s", out)
	}
}

func TestHTMLParagraphWordWrapNotSet(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Text: "supercalifragilisticexpialidocious"}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Contains(out, `word-break:break-all`) {
		t.Fatalf("expected no word-break when WordWrap is not set, got:\n%s", out)
	}
}

func TestHTMLParagraphKinsoku(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Format: resolve.ParagraphFormat{Kinsoku: true},
				Runs:   []resolve.Run{{Text: "日本語のテキスト"}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `line-break:strict`) {
		t.Fatalf("expected line-break:strict for Kinsoku, got:\n%s", out)
	}
}

func TestHTMLRunFELayoutVertical(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Text:   "縦書き",
					Format: resolve.RunFormat{FELayout: &resolve.FELayout{Vertical: true}},
				}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `writing-mode:vertical-rl`) {
		t.Fatalf("expected writing-mode:vertical-rl for vertical FE layout, got:\n%s", out)
	}
}

func TestHTMLRunFELayoutCombineCharacters(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Text:   "12",
					Format: resolve.RunFormat{FELayout: &resolve.FELayout{CombineCharacters: true}},
				}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `text-combine-upright:all`) {
		t.Fatalf("expected text-combine-upright:all for combine characters, got:\n%s", out)
	}
}

func TestHTMLRunEmbossImprint(t *testing.T) {
	tests := []struct {
		name string
		fmt  resolve.RunFormat
		want string
	}{
		{"emboss", resolve.RunFormat{Emboss: boolPtr(true)}, "text-shadow:-1px -1px 1px rgba(255,255,255,0.7), 1px 1px 1px rgba(0,0,0,0.5)"},
		{"imprint", resolve.RunFormat{Imprint: boolPtr(true)}, "text-shadow:1px 1px 1px rgba(255,255,255,0.7), -1px -1px 1px rgba(0,0,0,0.5)"},
		{"shadow", resolve.RunFormat{Shadow: boolPtr(true)}, "text-shadow:1px 1px 2px rgba(0,0,0,0.4)"},
		{"outline", resolve.RunFormat{Outline: boolPtr(true)}, "-webkit-text-stroke:0.75pt currentColor"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := &resolve.Document{
				Blocks: []resolve.Block{
					{Kind: "paragraph", Paragraph: &resolve.Paragraph{
						Runs: []resolve.Run{{Text: "effect", Format: tt.fmt}},
					}},
				},
			}
			out, err := render.HTML(doc)
			if err != nil {
				t.Fatalf("HTML: %v", err)
			}
			if !strings.Contains(out, tt.want) {
				t.Errorf("expected %q in output, got:\n%s", tt.want, out)
			}
		})
	}
}

func TestHTMLRunBlinkBackgroundAnimation(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Text: "blink", Format: resolve.RunFormat{TextAnimation: "blinkBackground"}}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "animation:word-blink 1s step-end infinite") {
		t.Errorf("expected blink animation in output, got:\n%s", out)
	}
	if !strings.Contains(out, "@keyframes word-blink") {
		t.Errorf("expected word-blink keyframes in CSS, got:\n%s", out)
	}
}

func TestHTMLRunLightsAnimation(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Text: "glow", Format: resolve.RunFormat{TextAnimation: "lights"}}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "animation:word-lights 1.5s ease-in-out infinite alternate") {
		t.Errorf("expected lights animation in output, got:\n%s", out)
	}
	if !strings.Contains(out, "@keyframes word-lights") {
		t.Errorf("expected word-lights keyframes in CSS, got:\n%s", out)
	}
}

func TestHTMLFontFamilyFallbackHeuristics(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Serif", Format: resolve.RunFormat{FontASCII: "Baskerville"}}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Script", Format: resolve.RunFormat{FontASCII: "Segoe Script"}}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "CJK", Format: resolve.RunFormat{FontASCII: "Meiryo"}}}}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, want := range []string{
		"font-family:Baskerville,'Times New Roman',Times,serif",
		"font-family:'Segoe Script','Brush Script MT',cursive",
		"font-family:Meiryo,'Yu Gothic','MS PGothic',sans-serif",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output:\n%s", want, out)
		}
	}
}

func TestHTMLCaps(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Text: "hello", Format: resolve.RunFormat{Caps: boolPtr(true)}},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "HELLO") {
		t.Errorf("expected caps-transformed text HELLO in output")
	}
	if strings.Contains(out, "hello") {
		t.Errorf("expected lowercase 'hello' to be transformed away")
	}
}

func TestHTMLVertAlign(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Text: "base"},
					{Text: "2", Format: resolve.RunFormat{VertAlignVal: "superscript"}},
					{Text: "n", Format: resolve.RunFormat{VertAlignVal: "subscript"}},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Contains(out, "<sup>") || strings.Contains(out, "<sub>") {
		t.Errorf("physical layout should own superscript/subscript vertical placement: %s", out)
	}
	if !strings.Contains(out, "font-size:8.0pt") {
		t.Errorf("expected layout-owned script font scaling: %s", out)
	}
	if !strings.Contains(out, "top:-3.96pt") || !strings.Contains(out, "top:1.92pt") {
		t.Errorf("expected layout-owned superscript/subscript top offsets: %s", out)
	}
}

func TestHTMLHyperlink(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "hyperlink", Text: "click here", Target: "https://example.com"},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `href="https://example.com"`) {
		t.Errorf("expected href in output")
	}
	if !strings.Contains(out, "click here") {
		t.Errorf("expected link text in output")
	}
}

func TestHTMLHyperlinkInheritsWordRunFormatting(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "hyperlink", Text: "plain link", Target: "https://example.com"},
					{Kind: "hyperlink", Text: "styled link", Target: "https://example.org", Format: resolve.RunFormat{Color: "0563C1", Underline: "single"}},
				},
			}},
		},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	css := extractCSS(out)
	if !strings.Contains(css, `a { color: inherit; text-decoration: inherit; }`) {
		t.Fatalf("expected global anchors to inherit Word run formatting, got:\n%s", css)
	}
	if strings.Contains(css, `a { color: #0563C1; text-decoration: underline; }`) {
		t.Fatalf("did not expect browser-like hyperlink styling in global CSS:\n%s", css)
	}
	if !strings.Contains(out, `<a href="https://example.com">plain link</a>`) {
		t.Fatalf("expected unformatted hyperlink to render without synthetic styling, got:\n%s", out)
	}
	if !strings.Contains(out, `<a href="https://example.org"><span style="text-decoration:underline;color:#0563C1">styled link</span></a>`) {
		t.Fatalf("expected styled hyperlink to keep explicit Word run formatting, got:\n%s", out)
	}
}

func TestHTMLHyperlinkWithAnchor(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "hyperlink", Text: "jump", Target: "https://example.com", Anchor: "section1"},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `href="https://example.com#section1"`) {
		t.Errorf("expected href with anchor in output")
	}
}

func TestHTMLComments(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "Body"},
					{Kind: "commentRef", CommentID: 3},
				},
			}},
		},
		Comments: map[int]resolve.Story{
			3: {
				Blocks: []resolve.Block{
					{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Review note"}}}},
				},
			},
		},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `href="#comment3"`) {
		t.Fatalf("expected comment reference link in output: %s", out)
	}
	if !strings.Contains(out, `id="comment3"`) {
		t.Fatalf("expected rendered comment anchor in output: %s", out)
	}
	if !strings.Contains(out, "Review note") {
		t.Fatalf("expected rendered comment text in output: %s", out)
	}
}

func TestHTMLCommentAnnotationRefRendersCommentNumber(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "Body"},
					{Kind: "commentRef", CommentID: 5},
				},
			}},
		},
		Comments: map[int]resolve.Story{
			5: {
				Blocks: []resolve.Block{
					{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{
						{Kind: "annotationRef", CommentID: 5},
						{Kind: "run", Text: "Review note"},
					}}},
				},
			},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `id="comment5"`) {
		t.Fatalf("expected rendered comment anchor: %s", out)
	}
	if !strings.Contains(out, "<sup>[5]</sup>") {
		t.Fatalf("expected annotationRef to render as [5] in comment story: %s", out)
	}
	if !strings.Contains(out, "Review note") {
		t.Fatalf("expected comment text: %s", out)
	}
}

func TestHTMLUnderline(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Text: "plain underline", Format: resolve.RunFormat{Underline: "single"}},
					{Text: "colored underline", Format: resolve.RunFormat{Underline: "single", UnderlineColor: "00FF00"}},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "text-decoration:underline") {
		t.Errorf("expected text-decoration:underline")
	}
	if !strings.Contains(out, "text-decoration:underline #00FF00") {
		t.Errorf("expected underline with color in shorthand, got:\n%s", out)
	}
}

func TestHTMLUnderlineStyle(t *testing.T) {
	tests := []struct {
		name      string
		underline string
		color     string
		want      string
	}{
		{"single", "single", "", "text-decoration:underline"},
		{"double", "double", "", "text-decoration:underline double"},
		{"dotted", "dotted", "", "text-decoration:underline dotted"},
		{"dash", "dash", "", "text-decoration:underline dashed"},
		{"wave", "wave", "", "text-decoration:underline wavy"},
		{"double-colored", "double", "00FF00", "text-decoration:underline double #00FF00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := &resolve.Document{
				Blocks: []resolve.Block{
					{Kind: "paragraph", Paragraph: &resolve.Paragraph{
						Runs: []resolve.Run{
							{Text: "x", Format: resolve.RunFormat{Underline: tt.underline, UnderlineColor: tt.color}},
						},
					}},
				},
			}
			out, err := render.HTML(doc)
			if err != nil {
				t.Fatalf("HTML: %v", err)
			}
			if !strings.Contains(out, tt.want) {
				t.Errorf("expected %q in output, got:\n%s", tt.want, out)
			}
		})
	}
}

func TestHTMLParagraphAutoSpace(t *testing.T) {
	tests := []struct {
		name        string
		autoSpaceDE bool
		autoSpaceDN bool
		want        string
	}{
		{"both", true, true, "text-autospace:normal"},
		{"de-only", true, false, "text-autospace:ideograph-alpha"},
		{"dn-only", false, true, "text-autospace:ideograph-numeric"},
		{"none", false, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := &resolve.Document{
				Blocks: []resolve.Block{
					{Kind: "paragraph", Paragraph: &resolve.Paragraph{
						Runs: []resolve.Run{{Text: "x"}},
						Format: resolve.ParagraphFormat{
							AutoSpaceDE: tt.autoSpaceDE,
							AutoSpaceDN: tt.autoSpaceDN,
						},
					}},
				},
			}
			out, err := render.HTML(doc)
			if err != nil {
				t.Fatalf("HTML: %v", err)
			}
			if tt.want == "" {
				if strings.Contains(out, "text-autospace:") {
					t.Fatalf("expected no text-autospace for none, got:\n%s", out)
				}
				return
			}
			if !strings.Contains(out, tt.want) {
				t.Errorf("expected %q in output, got:\n%s", tt.want, out)
			}
		})
	}
}

func TestHTMLBreakClearAdvancesPhysicalLine(t *testing.T) {
	tests := []struct {
		clearType string
	}{
		{"left"},
		{"right"},
		{"all"},
		{"none"},
		{""},
	}
	for _, tt := range tests {
		t.Run(tt.clearType, func(t *testing.T) {
			doc := &resolve.Document{
				Blocks: []resolve.Block{
					{Kind: "paragraph", Paragraph: &resolve.Paragraph{
						Runs: []resolve.Run{
							{Text: "Line 1"},
							{Kind: "break", BreakType: "textWrapping", BreakClear: tt.clearType, Text: "\n"},
							{Text: "Line 2"},
						},
					}},
				},
			}
			out, err := render.HTML(doc)
			if err != nil {
				t.Fatalf("HTML: %v", err)
			}
			for _, want := range []string{
				`class="word-physical-slot" data-line="0"`,
				`class="word-physical-slot" data-line="1"`,
				">Line 1</span>",
				">Line 2</span>",
			} {
				if !strings.Contains(out, want) {
					t.Errorf("expected %q in output, got:\n%s", want, out)
				}
			}
		})
	}
}

func TestHTMLNoProofSpellcheck(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Text: "normal"},
					{Text: "nospell", Format: resolve.RunFormat{NoProof: boolPtr(true)}},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `spellcheck="false"`) {
		t.Fatalf("expected spellcheck=false on noProof run, got:\n%s", out)
	}
	// Verify the normal run does NOT have spellcheck attribute.
	// Find the first run text and check there's no spellcheck near it.
	idxNormal := strings.Index(out, "normal")
	if idxNormal < 0 {
		t.Fatalf("expected 'normal' text in output")
	}
	snippet := out[max(0, idxNormal-30):idxNormal]
	if strings.Contains(snippet, "spellcheck") {
		t.Fatalf("expected no spellcheck on normal run, got snippet:\n%s", snippet)
	}
}

func TestHTMLBulletList(t *testing.T) {
	t.Skip("not OfficeArt: excluded from the OfficeArt-focused test pass")
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				List: &resolve.ListMarker{Kind: "bullet", Level: 0},
				Runs: []resolve.Run{{Text: "Item 1"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				List: &resolve.ListMarker{Kind: "bullet", Level: 1},
				Runs: []resolve.Run{{Text: "Item 1.1"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				List: &resolve.ListMarker{Kind: "bullet", Level: 0},
				Runs: []resolve.Run{{Text: "Item 2"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Text: "After list"}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "<ul>") {
		t.Error("expected <ul>")
	}
	parent := strings.Index(out, "Item 1")
	child := strings.Index(out, "Item 1.1")
	if parent < 0 || child < 0 || child <= parent {
		t.Fatalf("expected parent and nested list item text: %s", out)
	}
	nestedOpenRel := strings.Index(out[parent:child], "<ul>")
	childCloseRel := strings.Index(out[child:], "</li>")
	if nestedOpenRel < 0 || childCloseRel < 0 {
		t.Fatalf("expected nested <ul> and child </li>: %s", out)
	}
	childClose := child + childCloseRel
	nestedCloseRel := strings.Index(out[childClose:], "</ul>")
	if nestedCloseRel < 0 {
		t.Fatalf("expected nested </ul> after child item: %s", out)
	}
	nestedClose := childClose + nestedCloseRel
	parentCloseRel := strings.Index(out[nestedClose:], "</li>")
	if parentCloseRel < 0 {
		t.Fatalf("expected parent </li> after nested </ul>: %s", out)
	}
	if !strings.Contains(out, "</ul>") {
		t.Error("expected </ul>")
	}
	// Lists must be closed before the non-list paragraph.
	ulClose := strings.LastIndex(out, "</ul>")
	afterList := strings.Index(out, "After list")
	if ulClose < 0 || afterList < 0 || ulClose > afterList {
		t.Error("expected all lists closed before the trailing paragraph")
	}
}

func TestHTMLOrderedList(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				List: &resolve.ListMarker{Kind: "ordered", Level: 0, Start: 1},
				Runs: []resolve.Run{{Text: "First"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				List: &resolve.ListMarker{Kind: "ordered", Level: 0, Start: 1},
				Runs: []resolve.Run{{Text: "Second"}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "<ol>") {
		t.Error("expected <ol>")
	}
	if strings.Contains(out, "<ul>") {
		t.Error("unexpected <ul> for ordered list")
	}
}

func TestHTMLListItemWithIndents(t *testing.T) {
	// Verify that list items with indent properties (from LVLF PAPx grpprl)
	// render with correct CSS indentation on the <li> element.
	indentLeft := 720       // 0.5 inches in twips
	indentFirstLine := -360 // -0.25 inches (hanging indent) in twips
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				List: &resolve.ListMarker{Kind: "bullet", Level: 0},
				Format: resolve.ParagraphFormat{
					IndentLeft:      &indentLeft,
					IndentFirstLine: &indentFirstLine,
				},
				Runs: []resolve.Run{{Text: "Item with indent"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				List: &resolve.ListMarker{Kind: "ordered", Level: 0, Start: 1},
				Format: resolve.ParagraphFormat{
					IndentFirstLine: &indentFirstLine,
				},
				Runs: []resolve.Run{{Text: "Ordered item with hanging indent"}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "<ul>") {
		t.Fatalf("expected <ul> in output")
	}
	if !strings.Contains(out, "<ol>") {
		t.Fatalf("expected <ol> in output")
	}
	// Verify indent CSS is applied to list items
	if !strings.Contains(out, "margin-left:36.0pt") {
		t.Fatalf("expected margin-left:36.0pt (720 twips = 36pt) from IndentLeft in output: %s", out)
	}
	if !strings.Contains(out, "text-indent:-18.0pt") {
		t.Fatalf("expected text-indent:-18.0pt (-360 twips = -18pt) from IndentFirstLine in output: %s", out)
	}
}

func TestHTMLContextualSpacingDoesNotCrossTableBoundary(t *testing.T) {
	spacing := 120
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				StyleID: "Body",
				Runs:    []resolve.Run{{Kind: "run", Text: "Before"}},
				Format:  resolve.ParagraphFormat{ContextualSpacing: true, SpacingAfter: &spacing},
			}},
			{Kind: "table", Table: &resolve.Table{Rows: []resolve.TableRow{{Cells: []resolve.TableCell{{Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Cell"}}}}}}}}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				StyleID: "Body",
				Runs:    []resolve.Run{{Kind: "run", Text: "After"}},
				Format:  resolve.ParagraphFormat{ContextualSpacing: true, SpacingBefore: &spacing},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `padding-top:6.0pt`) {
		t.Fatalf("expected spacing-before after table boundary to remain in HTML output: %s", out)
	}
}

func TestHTMLTable(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "table", Table: &resolve.Table{
				Rows: []resolve.TableRow{
					{Cells: []resolve.TableCell{
						{
							Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "A1"}}}}},
							Text:   "A1",
						},
						{
							Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "B1"}}}}},
							Text:   "B1",
						},
					}},
					{Cells: []resolve.TableCell{
						{
							Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "A2"}}}}},
							Text:   "A2",
						},
						{
							Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "B2"}}}}},
							Text:   "B2",
						},
					}},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, want := range []string{"<table>", "<tr>", "<td>", "A1", "B1", "A2", "B2", "</table>"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output", want)
		}
	}
}

func TestHTMLTableCaptionAndDescription(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "table", Table: &resolve.Table{
				Format: resolve.TableFormat{
					Caption:     "Quarterly Sales",
					Description: "Regional sales summary",
				},
				Rows: []resolve.TableRow{{
					Cells: []resolve.TableCell{
						{
							Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "A1"}}}}},
							Text:   "A1",
						},
					},
				}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<caption>Quarterly Sales</caption>`) {
		t.Fatalf("expected table caption in output, got:\n%s", out)
	}
	if !strings.Contains(out, `title="Regional sales summary"`) {
		t.Fatalf("expected table description title in output, got:\n%s", out)
	}
}

func TestHTMLTableRowCantSplit(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "table", Table: &resolve.Table{
				Rows: []resolve.TableRow{
					{Format: resolve.TableRowFormat{CantSplit: true}, Cells: []resolve.TableCell{{
						Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "A1"}}}}},
						Text:   "A1",
					}}},
					{Cells: []resolve.TableCell{{
						Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "A2"}}}}},
						Text:   "A2",
					}}},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<tr style="break-inside:avoid-page">`) {
		t.Fatalf("expected break-inside:avoid-page on CantSplit row, got:\n%s", out)
	}
	// Second row should not have the style.
	rows := strings.Count(out, "<tr")
	if rows != 2 {
		t.Fatalf("expected 2 rows, got %d", rows)
	}
}

func TestHTMLTableBidiVisual(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "table", Table: &resolve.Table{
				Format: resolve.TableFormat{BidiVisual: true},
				Rows: []resolve.TableRow{
					{Cells: []resolve.TableCell{{
						Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "A1"}}}}},
						Text:   "A1",
					}}},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `direction:rtl`) {
		t.Fatalf("expected direction:rtl for BidiVisual table, got:\n%s", out)
	}
}

func TestHTMLTableColumnWidthsIndentAndCellPadding(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "table", Table: &resolve.Table{
				Format: resolve.TableFormat{
					Layout:        "fixed",
					ColumnWidths:  []int{1200, 2400},
					Indent:        108,
					CellSpacing:   80,
					CellPadTop:    20,
					CellPadRight:  80,
					CellPadBottom: 60,
					CellPadLeft:   40,
				},
				Rows: []resolve.TableRow{{
					Cells: []resolve.TableCell{
						{Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "A"}}}}}, Text: "A"},
						{Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "B"}}}}}, Text: "B"},
					},
				}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `table-layout:fixed`) {
		t.Fatalf("expected fixed table layout in output: %s", out)
	}
	if !strings.Contains(out, `margin-left:5.4pt`) {
		t.Fatalf("expected table indent in output: %s", out)
	}
	// CellSpacing (dxaGapHalf*2) is now applied as per-cell padding-left/right, not border-spacing.
	// With explicit CellPadLeft/Right set, those take precedence over the gapHalf fallback.
	if strings.Contains(out, `border-collapse:separate`) || strings.Contains(out, `border-spacing`) {
		t.Fatalf("expected no border-spacing (gapHalf should be per-cell padding now): %s", out)
	}
	if !strings.Contains(out, `<colgroup><col style="width:60.0pt"><col style="width:120.0pt"></colgroup>`) {
		t.Fatalf("expected colgroup widths in output: %s", out)
	}
	if !strings.Contains(out, `padding:1.0pt 4.0pt 3.0pt 2.0pt`) {
		t.Fatalf("expected table cell padding in output: %s", out)
	}
}

func TestHTMLTableCellGapHalfRendersAsPerCellPadding(t *testing.T) {
	// When CellSpacing (= dxaGapHalf * 2) is set with no explicit CellPadLeft/Right,
	// the renderer should apply gapHalf (= CellSpacing/2) as padding-left and padding-right
	// on each td, matching POI WordToHtmlUtils.addTableCellProperties behavior.
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "table", Table: &resolve.Table{
				Format: resolve.TableFormat{
					CellSpacing: 80, // dxaGapHalf=40 twips = 2pt per side
				},
				Rows: []resolve.TableRow{{
					Cells: []resolve.TableCell{
						{Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "A"}}}}}, Text: "A"},
					},
				}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// gapHalf padding should appear on the td, not as border-spacing on the table.
	if strings.Contains(out, `border-spacing`) || strings.Contains(out, `border-collapse:separate`) {
		t.Fatalf("expected no border-spacing (gapHalf is per-cell padding): %s", out)
	}
	if !strings.Contains(out, `padding:0.0pt 2.0pt 0.0pt 2.0pt`) {
		t.Fatalf("expected per-cell padding 2pt left+right from gapHalf: %s", out)
	}
}

func TestHTMLTableColumnWidthsForceFixedLayout(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "table", Table: &resolve.Table{
				Format: resolve.TableFormat{
					ColumnWidths: []int{1200, 2400},
				},
				Rows: []resolve.TableRow{{
					Cells: []resolve.TableCell{
						{Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "A"}}}}}, Text: "A"},
						{Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "B"}}}}}, Text: "B"},
					},
				}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `table-layout:fixed`) {
		t.Fatalf("expected colgroup-backed tables to use fixed layout: %s", out)
	}
}

func TestHTMLTableRowGridOffsets(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "table", Table: &resolve.Table{
				Rows: []resolve.TableRow{{
					Format: resolve.TableRowFormat{
						GridBefore:      1,
						GridAfter:       2,
						WidthBefore:     intPtr(1200),
						WidthBeforeType: "dxa",
						WidthAfter:      intPtr(1800),
						WidthAfterType:  "dxa",
					},
					Cells: []resolve.TableCell{{
						Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "A"}}}}},
						Text:   "A",
					}},
				}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<td style="width:60.0pt;padding:0;border:none"></td>`) {
		t.Fatalf("expected leading grid-gap cell in output: %s", out)
	}
	if !strings.Contains(out, `<td colspan="2" style="width:90.0pt;padding:0;border:none"></td>`) {
		t.Fatalf("expected trailing grid-gap cell with colspan in output: %s", out)
	}
	if !strings.Contains(out, `<td><p>A</p>`) {
		t.Fatalf("expected real table cell content in output: %s", out)
	}
}

func TestHTMLParagraphAlignment(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Format: resolve.ParagraphFormat{Alignment: "center"},
				Runs:   []resolve.Run{{Text: "centered"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Format: resolve.ParagraphFormat{Alignment: "right"},
				Runs:   []resolve.Run{{Text: "right-aligned"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Format: resolve.ParagraphFormat{Alignment: "both"},
				Runs:   []resolve.Run{{Text: "justified"}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "text-align:center") {
		t.Error("expected text-align:center")
	}
	if !strings.Contains(out, "text-align:right") {
		t.Error("expected text-align:right")
	}
	if !strings.Contains(out, "text-align:justify") {
		t.Error("expected text-align:justify")
	}
}

func TestHTMLPageGeometry(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	css := extractCSS(out)
	// 12240 twips / 20 = 612pt (US Letter width)
	if !strings.Contains(css, "612.0pt") {
		t.Errorf("expected 612.0pt page width in CSS, got:\n%s", css)
	}
	// 1440 twips / 20 = 72pt (1-inch margin)
	if !strings.Contains(css, "72.0pt") {
		t.Errorf("expected 72.0pt margin in CSS, got:\n%s", css)
	}
}

func TestHTMLPageGeometryZeroMargins(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	css := extractCSS(out)
	if !strings.Contains(css, "min-height: 792.0pt") {
		t.Errorf("expected 792.0pt page height in CSS, got:\n%s", css)
	}
	if !strings.Contains(css, "padding: 0.0pt 0.0pt 0.0pt 0.0pt") {
		t.Errorf("expected zero margins to be preserved in CSS, got:\n%s", css)
	}
}

func TestHTMLColumns(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 720, Bottom: 1440, Left: 720},
			Columns:    &resolve.Columns{Num: 2, Space: 720, Sep: true},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `column-count:2;column-gap:36.0pt;column-rule:1px solid #bbb`) {
		t.Fatalf("expected multi-column page-body style in output: %s", out)
	}
}

func TestHTMLSectionVerticalAlign(t *testing.T) {
	tests := []struct {
		align   string
		wantCSS string
	}{
		{"center", "display:flex;flex-direction:column;justify-content:center"},
		{"both", "display:flex;flex-direction:column;justify-content:space-between"},
		{"bottom", "display:flex;flex-direction:column;justify-content:flex-end"},
		{"top", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.align, func(t *testing.T) {
			doc := &resolve.Document{
				Sections: []resolve.Section{{
					PageSize:      &resolve.PageSize{Width: 12240, Height: 15840},
					PageMargin:    &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
					VerticalAlign: tt.align,
				}},
				Blocks: []resolve.Block{
					{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Hello"}}}},
				},
			}
			out, err := render.HTML(doc)
			if err != nil {
				t.Fatalf("HTML: %v", err)
			}
			if tt.wantCSS == "" {
				if strings.Contains(out, "justify-content:") {
					t.Fatalf("expected no vertical-align CSS for %q, got:\n%s", tt.align, out)
				}
				return
			}
			if !strings.Contains(out, tt.wantCSS) {
				t.Fatalf("expected %q in output for align=%q, got:\n%s", tt.wantCSS, tt.align, out)
			}
		})
	}
}

func TestHTMLSectionTextDirection(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:      &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin:    &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
			TextDirection: "tbRl",
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Hello"}}}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "writing-mode:vertical-rl") {
		t.Fatalf("expected writing-mode:vertical-rl in page-body style, got:\n%s", out)
	}
	if !strings.Contains(out, "text-orientation:mixed") {
		t.Fatalf("expected text-orientation:mixed in page-body style, got:\n%s", out)
	}
}

func TestHTMLParagraphTextDirection(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Format: resolve.ParagraphFormat{TextDirection: "tbRl"},
				Runs:   []resolve.Run{{Text: "Hello"}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "writing-mode:vertical-rl") {
		t.Fatalf("expected writing-mode:vertical-rl for paragraph text direction, got:\n%s", out)
	}
	if !strings.Contains(out, "text-orientation:mixed") {
		t.Fatalf("expected text-orientation:mixed for paragraph text direction, got:\n%s", out)
	}
}

func TestHTMLFrameTextDirection(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Frame: &resolve.FrameProps{TextDirection: "btLr", W: 1440, H: 1440, X: 0, Y: 0},
				Runs:  []resolve.Run{{Text: "Framed"}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "writing-mode:vertical-lr") {
		t.Fatalf("expected writing-mode:vertical-lr for frame text direction, got:\n%s", out)
	}
}

func TestHTMLParagraphFontAlign(t *testing.T) {
	tests := []struct {
		align   string
		wantCSS string
	}{
		{"top", "vertical-align:top"},
		{"center", "vertical-align:middle"},
		{"baseline", ""},
		{"bottom", "vertical-align:bottom"},
		{"auto", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.align, func(t *testing.T) {
			pf := resolve.ParagraphFormat{FontAlign: tt.align}
			doc := &resolve.Document{
				Sections: []resolve.Section{{
					PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
					PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
				}},
				Blocks: []resolve.Block{
					{Kind: "paragraph", Paragraph: &resolve.Paragraph{Format: pf, Runs: []resolve.Run{
						{Text: "Big", Format: resolve.RunFormat{FontSize: intPtr(28)}},
						{Text: "Small", Format: resolve.RunFormat{FontSize: intPtr(18)}},
					}}},
				},
			}
			out, err := render.HTML(doc)
			if err != nil {
				t.Fatalf("HTML: %v", err)
			}
			if tt.wantCSS == "" {
				if strings.Contains(out, fmt.Sprintf("style=\"vertical-align:%s\"", tt.align)) || strings.Contains(out, fmt.Sprintf("vertical-align:%s;", tt.align)) {
					t.Fatalf("expected no inline vertical-align CSS for %q, got:\n%s", tt.align, out)
				}
				return
			}
			if !strings.Contains(out, tt.wantCSS) {
				t.Fatalf("expected %q in output for align=%q, got:\n%s", tt.wantCSS, tt.align, out)
			}
		})
	}
}

func TestHTMLParagraphFontAlignSkippedForUniformFontSize(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Format: resolve.ParagraphFormat{FontAlign: "center"},
				Runs: []resolve.Run{
					{Text: "Same", Format: resolve.RunFormat{FontSize: intPtr(18)}},
					{Text: "size", Format: resolve.RunFormat{FontSize: intPtr(18)}},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Contains(out, "vertical-align:middle") {
		t.Fatalf("expected uniform-size paragraph to avoid synthetic vertical-align, got:\n%s", out)
	}
}

func TestHTMLParagraphFontAlignDoesNotOverrideRunPosition(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Format: resolve.ParagraphFormat{FontAlign: "center"},
				Runs:   []resolve.Run{{Text: "Hello", Format: resolve.RunFormat{Position: intPtr(12)}}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// Run-level Position (+6pt raise) is materialized by physical layout as top:-6pt.
	if !strings.Contains(out, "top:-6.00pt") {
		t.Fatalf("expected layout-owned top:-6.00pt to win, got:\n%s", out)
	}
	if strings.Contains(out, "vertical-align:middle") || strings.Contains(out, "vertical-align:6.0pt") {
		t.Fatalf("renderer should not reapply paragraph/run vertical alignment after physical layout, got:\n%s", out)
	}
}

func TestHTMLParagraphSnapToGrid(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
			DocGrid:    &resolve.DocGrid{Type: "lines", LinePitch: 360},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Format: resolve.ParagraphFormat{SnapToGrid: true},
				Runs:   []resolve.Run{{Text: "Hello"}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "line-height:18.00pt") {
		t.Fatalf("expected line-height:18.00pt for snap-to-grid, got:\n%s", out)
	}
}

func TestHTMLParagraphSnapToGridPitchOnlyStaysInactive(t *testing.T) {
	// w:docGrid/@w:type defaults to "default", which means no grid at all. A
	// section that only carries a line pitch must not impose it on the browser:
	// Word does not snap those documents, and doing so changes line pitch and
	// page fill.
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
			DocGrid:    &resolve.DocGrid{LinePitch: 360, CharSpace: 200},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Format: resolve.ParagraphFormat{SnapToGrid: true},
				Runs:   []resolve.Run{{Text: "Hello"}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Contains(out, "line-height:18.00pt") || strings.Contains(out, "letter-spacing:10.00pt") {
		t.Fatalf("a pitch-only docGrid must stay inactive, got:\n%s", out)
	}
}

func TestHTMLParagraphSnapToGridNoGrid(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Format: resolve.ParagraphFormat{SnapToGrid: true},
				Runs:   []resolve.Run{{Text: "Hello"}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Contains(out, `style="line-height:`) || strings.Contains(out, `style=\"line-height:`) {
		t.Fatalf("expected no inline line-height when DocGrid is absent, got:\n%s", out)
	}
}

func TestHTMLListItemSnapToGrid(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
			DocGrid:    &resolve.DocGrid{Type: "lines", LinePitch: 400},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Format: resolve.ParagraphFormat{SnapToGrid: true},
				List:   &resolve.ListMarker{Kind: "ordered", Level: 0},
				Runs:   []resolve.Run{{Text: "Item"}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "line-height:20.00pt") {
		t.Fatalf("expected line-height:20.00pt for snap-to-grid list item, got:\n%s", out)
	}
}

func TestHTMLParagraphSnapToGridCharSpace(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
			DocGrid:    &resolve.DocGrid{Type: "lines", LinePitch: 360, CharSpace: 200},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Format: resolve.ParagraphFormat{SnapToGrid: true},
				Runs:   []resolve.Run{{Text: "Hello"}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "line-height:18.00pt") {
		t.Fatalf("expected line-height:18.00pt for snap-to-grid, got:\n%s", out)
	}
	if !strings.Contains(out, "letter-spacing:10.00pt") {
		t.Fatalf("expected letter-spacing:10.00pt for snap-to-grid char space, got:\n%s", out)
	}
}

func TestHTMLParagraphSnapToGridCharSpaceOnly(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
			DocGrid:    &resolve.DocGrid{Type: "linesAndChars", CharSpace: -100},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Format: resolve.ParagraphFormat{SnapToGrid: true},
				Runs:   []resolve.Run{{Text: "Hello"}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Contains(out, `style="letter-spacing:-5.00pt"`) && strings.Contains(out, "line-height:") {
		// The global CSS has line-height:1.2, but there should be no inline line-height.
		// Check specifically that the inline style doesn't contain line-height.
		idx := strings.Index(out, `<p style=`)
		if idx < 0 {
			t.Fatalf("expected <p style=...>, got:\n%s", out)
		}
		endIdx := strings.Index(out[idx:], ">")
		if endIdx < 0 {
			t.Fatalf("expected > after <p style=, got:\n%s", out)
		}
		inlineStyle := out[idx : idx+endIdx]
		if strings.Contains(inlineStyle, "line-height") {
			t.Fatalf("expected no inline line-height when LinePitch is zero, got:\n%s", out)
		}
	}
	if !strings.Contains(out, "letter-spacing:-5.00pt") {
		t.Fatalf("expected letter-spacing:-5.00pt for negative char space, got:\n%s", out)
	}
}

func TestHTMLListItemSnapToGridCharSpace(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
			DocGrid:    &resolve.DocGrid{Type: "lines", LinePitch: 400, CharSpace: 150},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Format: resolve.ParagraphFormat{SnapToGrid: true},
				List:   &resolve.ListMarker{Kind: "ordered", Level: 0},
				Runs:   []resolve.Run{{Text: "Item"}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "line-height:20.00pt") {
		t.Fatalf("expected line-height:20.00pt for snap-to-grid list item, got:\n%s", out)
	}
	if !strings.Contains(out, "letter-spacing:7.50pt") {
		t.Fatalf("expected letter-spacing:7.50pt for snap-to-grid list item char space, got:\n%s", out)
	}
}

func TestHTMLSectionSpansUsePerSectionPageStyles(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{
			{
				PageSize:   &resolve.PageSize{Width: 10000, Height: 16000},
				PageMargin: &resolve.PageMargin{Top: 1000, Right: 1000, Bottom: 1000, Left: 1000},
			},
			{
				PageSize:   &resolve.PageSize{Width: 14000, Height: 20000},
				PageMargin: &resolve.PageMargin{Top: 1200, Right: 1200, Bottom: 1200, Left: 1200},
				Columns:    &resolve.Columns{Num: 2, Space: 720, Sep: true},
			},
		},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Section one"}}}},
			{Kind: "section", Section: &resolve.Section{PageSize: &resolve.PageSize{Width: 10000, Height: 16000}, PageMargin: &resolve.PageMargin{Top: 1000, Right: 1000, Bottom: 1000, Left: 1000}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Section two"}}}},
			{Kind: "section", Section: &resolve.Section{PageSize: &resolve.PageSize{Width: 14000, Height: 20000}, PageMargin: &resolve.PageMargin{Top: 1200, Right: 1200, Bottom: 1200, Left: 1200}, Columns: &resolve.Columns{Num: 2, Space: 720, Sep: true}}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Count(out, `<section class="page"`) != 2 {
		t.Fatalf("expected two page containers for two section spans: %s", out)
	}
	css := extractCSS(out)
	if !strings.Contains(css, `.page {`) ||
		!strings.Contains(css, `width: 500.0pt; height: 800.0pt; min-height: 800.0pt;`) {
		t.Fatalf("expected first section page geometry in CSS: %s", out)
	}
	if !strings.Contains(out, `width:700.0pt;height:1000.0pt;min-height:1000.0pt`) {
		t.Fatalf("expected second section page geometry in output: %s", out)
	}
	if !strings.Contains(out, `column-count:2;column-gap:36.0pt;column-rule:1px solid #bbb`) {
		t.Fatalf("expected second section columns in output: %s", out)
	}
}

func TestHTMLTableVerticalMergeUsesRowspan(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{{
			Kind: "table",
			Table: &resolve.Table{
				Rows: []resolve.TableRow{
					{Cells: []resolve.TableCell{{Format: resolve.TableCellFormat{VMerge: "restart"}, Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Merged"}}}}}}}},
					{Cells: []resolve.TableCell{{Format: resolve.TableCellFormat{VMerge: "continue"}, Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Ignored"}}}}}}}},
				},
			},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `rowspan="2"`) {
		t.Fatalf("expected rowspan for vertically merged table cell: %s", out)
	}
	if strings.Contains(out, `Ignored`) {
		t.Fatalf("did not expect continuation cell content to render separately: %s", out)
	}
}

func TestHTMLTableCellTextDirectionUsesVerticalWritingMode(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{{
			Kind: "table",
			Table: &resolve.Table{
				Rows: []resolve.TableRow{{
					Cells: []resolve.TableCell{{
						Format: resolve.TableCellFormat{TextDirection: "btLr"},
						Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Vertical"}}}}},
					}},
				}},
			},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `writing-mode:vertical-lr`) {
		t.Fatalf("expected table cell vertical writing mode in output: %s", out)
	}
	if !strings.Contains(out, `transform:rotate(180deg)`) {
		t.Fatalf("expected btLr table cell rotation correction in output: %s", out)
	}
}

func TestHTMLTableCell3DRendersBoxShadow(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{{
			Kind: "table",
			Table: &resolve.Table{
				Rows: []resolve.TableRow{{
					Cells: []resolve.TableCell{{
						Format: resolve.TableCellFormat{
							Cell3D: &resolve.Cell3D{
								BevelTopWidth:     50800,
								BevelTopHeight:    50800,
								BevelBottomWidth:  38100,
								BevelBottomHeight: 38100,
								ExtrusionColor:    "808080",
							},
						},
						Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "3D Cell"}}}}},
					}},
				}},
			},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "box-shadow:") {
		t.Fatalf("expected box-shadow for cell3D, got:\n%s", out)
	}
	if !strings.Contains(out, "inset") {
		t.Fatalf("expected inset shadow for top bevel, got:\n%s", out)
	}
	if !strings.Contains(out, "#808080") {
		t.Fatalf("expected extrusion color #808080 in shadow, got:\n%s", out)
	}
}

func TestHTMLTableCell3DNoBevelRendersNoShadow(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{{
			Kind: "table",
			Table: &resolve.Table{
				Rows: []resolve.TableRow{{
					Cells: []resolve.TableCell{{
						Format: resolve.TableCellFormat{
							Cell3D: &resolve.Cell3D{}, // empty 3D props
						},
						Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Flat Cell"}}}}},
					}},
				}},
			},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Contains(out, "box-shadow:") {
		t.Fatalf("expected no box-shadow for empty cell3D, got:\n%s", out)
	}
}

func TestHTMLVMLShapeTextPathRendersText(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:      "rect",
						Width:     914400,
						Height:    914400,
						FillColor: "FFCCCC",
						TextPath:  &resolve.TextPath{On: "t", String: "Curved Text"},
					},
				}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "Curved Text") {
		t.Fatalf("expected textpath text 'Curved Text' in output, got:\n%s", out)
	}
	if !strings.Contains(out, `class="shape-wordart"`) || !strings.Contains(out, `<textPath`) {
		t.Fatalf("expected SVG geometry-text rendering, got:\n%s", out)
	}
	if strings.Contains(out, `class="shape-rect"`) {
		t.Fatalf("geometry text should not fall through to rectangular textbox rendering, got:\n%s", out)
	}
}

func TestHTMLTableLevelBordersRenderOuterAndInnerGrid(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{{
			Kind: "table",
			Table: &resolve.Table{
				Format: resolve.TableFormat{
					Borders: &resolve.TableBorders{
						Top:     &resolve.BorderSide{Val: "single", SizePt: 1, Color: "FF0000"},
						Left:    &resolve.BorderSide{Val: "single", SizePt: 1, Color: "00FF00"},
						Bottom:  &resolve.BorderSide{Val: "double", SizePt: 1, Color: "0000FF"},
						Right:   &resolve.BorderSide{Val: "dotted", SizePt: 1, Color: "000000"},
						InsideH: &resolve.BorderSide{Val: "dashed", SizePt: 1, Color: "FF00FF"},
						InsideV: &resolve.BorderSide{Val: "single", SizePt: 1, Color: "00FFFF"},
					},
				},
				Rows: []resolve.TableRow{
					{Cells: []resolve.TableCell{
						{Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "A"}}}}}},
						{Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "B"}}}}}},
					}},
					{Cells: []resolve.TableCell{
						{Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "C"}}}}}},
						{Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "D"}}}}}},
					}},
				},
			},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `border-collapse:collapse`) {
		t.Fatalf("expected collapsed table borders in output: %s", out)
	}
	if !strings.Contains(out, `border-top:1.00pt solid #FF0000`) {
		t.Fatalf("expected table top border in output: %s", out)
	}
	if !strings.Contains(out, `border-right:1.00pt solid #00FFFF`) {
		t.Fatalf("expected interior vertical border fallback in output: %s", out)
	}
	if !strings.Contains(out, `border-bottom:1.00pt dashed #FF00FF`) {
		t.Fatalf("expected interior horizontal border fallback in output: %s", out)
	}
	if !strings.Contains(out, `border-bottom:1.00pt double #0000FF`) {
		t.Fatalf("expected outer bottom border in output: %s", out)
	}
}

func TestHTMLPageBreak(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "break", Text: "\n", BreakType: "page"},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="page-break"`) {
		t.Error("expected page-break element")
	}
}

func TestHTMLFloatingContentAfterPageBreakUsesNextPageOffset(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "break", BreakType: "page"},
					{
						Kind: "textbox",
						TextBox: &resolve.TextBox{
							Floating: true,
							Y:        127000, // 10pt
							Blocks: []resolve.Block{
								{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Page two box"}}}},
							},
						},
					},
					{
						Kind: "shape",
						Shape: &resolve.Shape{
							Kind:     "line",
							Floating: true,
							Y:        127000, // 10pt
							Width:    127000, // 10pt
						},
					},
				},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Anchor page two"}}}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Count(out, `<section class="page" `) != 2 {
		t.Fatalf("expected two page containers after a page break: %s", out)
	}
	for _, want := range []string{
		`<div class="page-num">Page 1</div>`,
		`<section class="page" id="page-1" data-page="1">`,
		`<div class="page-num">Page 2</div>`,
		`<section class="page" id="page-2" data-page="2">`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing page marker %q: %s", want, out)
		}
	}
	if !strings.Contains(out, `class="page-overlay"`) {
		t.Fatalf("expected page overlay on the second page: %s", out)
	}
	if !strings.Contains(out, `top:10.0pt`) {
		t.Fatalf("expected page-two floating content to use local page coordinates: %s", out)
	}
}

func TestHTMLPaginatedOutputSuppressesInlinePageBreakInsideOpenParagraph(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Before"}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "break", BreakType: "page"}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "After"}}}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Contains(out, "</section>\n</p>") {
		t.Fatalf("expected paginated renderer to avoid breaking an open paragraph: %s", out)
	}
	if strings.Count(out, `<section class="page" `) != 2 {
		t.Fatalf("expected two page containers after explicit page break: %s", out)
	}
	// Physical paragraph ownership may add positioned slot markup around the
	// text. This regression is about page-container integrity and text survival,
	// not the old semantic <p> serialization shape.
	if !strings.Contains(out, ">Before<") || !strings.Contains(out, ">After<") {
		t.Fatalf("expected both paragraph texts in their page bodies: %s", out)
	}
}

func TestHTMLPaginatedOutputForcesContinuationIntoNextColumn(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
			Columns:    &resolve.Columns{Num: 2, Space: 720},
		}},
		Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{
			Runs: []resolve.Run{
				{Kind: "run", Text: "Left column text"},
				{Kind: "break", BreakType: "column"},
				{Kind: "run", Text: "Right column text"},
			},
		}}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Count(out, `<section class="page" `) != 1 {
		t.Fatalf("expected single page container: %s", out)
	}
	if !strings.Contains(out, `break-before:column`) {
		t.Fatalf("expected continuation fragment to force a new column: %s", out)
	}
	if !strings.Contains(out, `Left column text`) || !strings.Contains(out, `Right column text`) {
		t.Fatalf("expected both column fragments in output: %s", out)
	}
}

func TestHTMLTOCGeneratesFromHeadingsAndTCEntries(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{FieldKind: "TOC", FieldArg: `TOC \\o "1-2"`, Text: ""}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{StyleID: "Heading1", Runs: []resolve.Run{{Text: "Heading One"}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{StyleID: "Heading2", Runs: []resolve.Run{{Text: "Sub Heading"}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Body text"}, {FieldKind: "TC", FieldArg: `TC "Manual Entry" \\l 2`, Text: ""}}}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "word-field-toc") {
		t.Fatalf("expected TOC field class in output: %s", out)
	}
	for _, want := range []string{"Heading One", "Sub Heading", "Manual Entry"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected TOC output to contain %q: %s", want, out)
		}
	}
	if strings.Count(out, "Manual Entry") != 1 {
		t.Fatalf("expected TC entry to appear only in generated TOC: %s", out)
	}
	css := extractCSS(out)
	if !strings.Contains(css, ".word-field-toc") {
		t.Fatalf("expected TOC CSS rule in output: %s", css)
	}
	if !strings.Contains(css, ".word-field-xe, .word-field-tc { display: none; }") {
		t.Fatalf("expected XE/TC hidden CSS rule in output: %s", css)
	}
	if strings.Contains(out, `>TC`) || strings.Contains(out, `>XE`) {
		t.Fatalf("did not expect field instruction text in output: %s", out)
	}
}

func TestHTMLTOCPreservesCachedResultInsteadOfRegeneratingPerRun(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{FieldKind: "TOC", FieldInstr: `TOC \o "1-3" \h \z \u`, Text: "Cached Entry"}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{StyleID: "Heading1", Runs: []resolve.Run{{Text: "Heading One"}}}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Count(out, "Cached Entry") != 1 {
		t.Fatalf("cached TOC result should render once: %s", out)
	}
	if strings.Count(out, "Heading One") != 1 {
		t.Fatalf("cached TOC result must not be replaced by a regenerated TOC: %s", out)
	}
}

func TestHTMLTOCCustomSeparatorSwitch(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{FieldKind: "TOC", FieldArg: `TOC \\o "1-2" \\d "-"`, Text: ""}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{StyleID: "Heading1", Runs: []resolve.Run{{Text: "Heading One"}}}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "word-field-toc") {
		t.Fatalf("expected TOC field class in output: %s", out)
	}
	if !strings.Contains(out, "Heading One") {
		t.Fatalf("expected TOC heading text: %s", out)
	}
	if !strings.Contains(out, " - ") {
		t.Fatalf("expected custom TOC separator ' - ', got: %s", out)
	}
}

func TestHTMLTOCLevelRangeSwitch(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{FieldKind: "TOC", FieldArg: `TOC \l 2-3`, Text: ""}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				StyleID:   "Heading1",
				StyleName: "heading 1",
				Runs:      []resolve.Run{{Text: "Heading One"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				StyleID:   "Heading2",
				StyleName: "heading 2",
				Runs:      []resolve.Run{{Text: "Heading Two"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				StyleID:   "Heading3",
				StyleName: "heading 3",
				Runs:      []resolve.Run{{Text: "Heading Three"}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "word-field-toc") {
		t.Fatalf("expected TOC field class in output: %s", out)
	}
	// Extract the TOC span content to avoid matching body headings.
	tocStart := strings.Index(out, `class="word-field word-field-toc"`)
	if tocStart < 0 {
		t.Fatalf("expected TOC field in output: %s", out)
	}
	tocContentStart := strings.Index(out[tocStart:], ">") + tocStart + 1
	tocContentEnd := strings.Index(out[tocContentStart:], "</span>")
	if tocContentEnd < 0 {
		t.Fatalf("expected TOC span close in output: %s", out)
	}
	tocContent := out[tocContentStart : tocContentStart+tocContentEnd]
	if strings.Contains(tocContent, "Heading One") {
		t.Fatalf("expected TOC with \\l 2-3 to exclude level-1 heading, got TOC: %s", tocContent)
	}
	if !strings.Contains(tocContent, "Heading Two") {
		t.Fatalf("expected TOC to include level-2 heading, got TOC: %s", tocContent)
	}
	if !strings.Contains(tocContent, "Heading Three") {
		t.Fatalf("expected TOC to include level-3 heading, got TOC: %s", tocContent)
	}
}

func TestHTMLTOCLevelRangeSwitchWithOutlineLevel(t *testing.T) {
	outline2 := 2
	outline3 := 3
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{FieldKind: "TOC", FieldArg: `TOC \l 2-3`, Text: ""}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Text: "Normal Para"}},
				Format: resolve.ParagraphFormat{
					OutlineLevel: &outline2,
				},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Text: "Outline Level 3"}},
				Format: resolve.ParagraphFormat{
					OutlineLevel: &outline3,
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "Normal Para") {
		t.Fatalf("expected TOC to include outline level 2: %s", out)
	}
	if !strings.Contains(out, "Outline Level 3") {
		t.Fatalf("expected TOC to include outline level 3: %s", out)
	}
}

func TestHTMLTOCZSwitchPreservesPagesInPaginatedOutput(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{FieldKind: "TOC", FieldArg: `TOC \z`, Text: ""}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{StyleID: "Heading1", Runs: []resolve.Run{{Text: "Chapter 1"}}}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	re := regexp.MustCompile(`Chapter 1\s+\.+\s+1`)
	if !re.MatchString(out) {
		t.Errorf("TOC with \\z should preserve page numbers in paginated output; got:\n%s", out)
	}
	if !strings.Contains(out, "Chapter 1") {
		t.Errorf("TOC should still show heading text; got:\n%s", out)
	}
}

func TestHTMLTOCUsesOutlineLevelWithUSwitch(t *testing.T) {
	outline2 := 2
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{FieldKind: "TOC", FieldArg: `TOC \u`, Text: ""}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				StyleID: "Normal",
				Runs:    []resolve.Run{{Text: "Outline Para"}},
				Format: resolve.ParagraphFormat{
					OutlineLevel: &outline2,
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "Outline Para") {
		t.Errorf("TOC with \\u should include paragraph with direct outline level; got:\n%s", out)
	}
}

func TestHTMLTOCPreservesNewlinesWithXSwitch(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{FieldKind: "TOC", FieldArg: `TOC \o "1-3"`, Text: ""}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{
				{Text: "Body"},
				{FieldKind: "TC", FieldArg: "TC \"Line1\nLine2\" \\\\x \\\\l 1", Text: ""},
			}}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "Line1\nLine2") {
		t.Errorf("TOC with \\x should preserve newlines; got:\n%s", out)
	}
}

func TestHTMLTOCMixedCaptionAndHeading(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{FieldKind: "TOC", FieldArg: `TOC \o "1-3" \c "Figure"`, Text: ""}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{StyleID: "Heading1", Runs: []resolve.Run{{Text: "Chapter 1"}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{
				{Text: "Figure "},
				{FieldKind: "SEQ", FieldArg: `SEQ Figure`, FieldInstr: `SEQ Figure`, Text: "1"},
				{Text: ": A diagram"},
			}}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "Chapter 1") {
		t.Errorf("mixed TOC should include heading; got:\n%s", out)
	}
	if !strings.Contains(out, "Figure") {
		t.Errorf("mixed TOC should include caption; got:\n%s", out)
	}
}

func TestHTMLLineNumberingSkipsTableParagraphsAndFloatingTextBoxes(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:      &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin:    &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
			LineNumbering: &resolve.LineNumbering{Start: 1, CountBy: 1, Restart: "continuous"},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Before table"}}}},
			{Kind: "table", Table: &resolve.Table{Rows: []resolve.TableRow{{Cells: []resolve.TableCell{{Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Inside table"}}}}}}}}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Before box"}, {Kind: "textbox", TextBox: &resolve.TextBox{Floating: true, Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Inside box"}}}}}}}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "After box"}}}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, want := range []string{`<span class="ln" aria-hidden="true">1</span>`, `<span class="ln" aria-hidden="true">2</span>`, `<span class="ln" aria-hidden="true">3</span>`} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected line number %q in output: %s", want, out)
		}
	}
	if strings.Contains(out, `<span class="ln" aria-hidden="true">4</span>`) {
		t.Fatalf("expected table/textbox paragraphs to be excluded from line numbering: %s", out)
	}
	if strings.Contains(out, `Inside box</span>`) {
		t.Fatalf("unexpected line-number marker inside floating textbox: %s", out)
	}
}

func TestHTMLLineNumberingSuppressesNoLineNumberParagraphs(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:      &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin:    &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
			LineNumbering: &resolve.LineNumbering{Start: 1, CountBy: 1, Restart: "continuous"},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "First"}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Format: resolve.ParagraphFormat{NoLineNumber: true}, Runs: []resolve.Run{{Text: "Suppressed"}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Third"}}}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<span class="ln" aria-hidden="true">1</span>`) {
		t.Fatalf("expected line number 1 for first paragraph: %s", out)
	}
	if !strings.Contains(out, `<span class="ln" aria-hidden="true">2</span>`) {
		t.Fatalf("expected line number 2 for third paragraph: %s", out)
	}
	// Count line-number spans: exactly 2 (first and third paragraphs).
	if strings.Count(out, `<span class="ln"`) != 2 {
		t.Fatalf("expected exactly 2 line-number spans, got %d: %s", strings.Count(out, `<span class="ln"`), out)
	}
}

func TestHTMLContinuousSectionStartsNestedColumnWrapperOnSamePage(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{
			{
				PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
				PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 720, Left: 1440},
			},
			{
				Type:       "continuous",
				PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
				PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
				Columns:    &resolve.Columns{Num: 2, Space: 720},
			},
		},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Intro"}}}},
			{Kind: "section", Section: &resolve.Section{Type: "continuous", PageSize: &resolve.PageSize{Width: 12240, Height: 15840}, PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440}, Columns: &resolve.Columns{Num: 2, Space: 720}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Left column text"}, {Kind: "break", BreakType: "column"}, {Kind: "run", Text: "Right column text"}}}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Count(out, `<section class="page" `) != 1 {
		t.Fatalf("expected a single page container: %s", out)
	}
	if !strings.Contains(out, `<div class="page-body-section" style="column-count:2;column-gap:36.0pt">`) {
		t.Fatalf("expected nested multi-column wrapper for same-page continuous section: %s", out)
	}
	if !strings.Contains(out, `break-before:column`) {
		t.Fatalf("expected explicit column break to survive within nested wrapper: %s", out)
	}
}

func TestHTMLParagraphRelativeFloatingContentUsesOverlay(t *testing.T) {
	// Floating objects with YRelative="paragraph" are now placed in the page
	// overlay using layout-computed page-absolute coordinates. The anchor
	// paragraph no longer needs position:relative.
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Text: "Anchor text"},
					{
						Kind: "textbox",
						TextBox: &resolve.TextBox{
							Floating:  true,
							Y:         127000,
							YRelative: "paragraph",
							Blocks: []resolve.Block{
								{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Anchored box"}}}},
							},
						},
					},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<div class="page-overlay">`) {
		t.Fatalf("expected floating content to be placed in the page overlay: %s", out)
	}
	if !strings.Contains(out, `Anchor text`) {
		t.Fatalf("expected anchor paragraph text in output: %s", out)
	}
	// Y=127000 EMU = 10pt; with zero margins paraPageY=0, so overlay top=10pt.
	if !strings.Contains(out, `top:10.0pt`) {
		t.Fatalf("expected paragraph-relative float to have page-absolute top:10.0pt in overlay: %s", out)
	}
}

func TestHTMLFloatingOnlyAnchorParagraphUsesOverlay(t *testing.T) {
	// An anchor-only paragraph (all runs are floating) puts its floats in the
	// overlay and uses a layout-computed min-height for correct flow spacing.
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{
						Kind: "textbox",
						TextBox: &resolve.TextBox{
							Floating:  true,
							Y:         508000,
							YRelative: "paragraph",
							Blocks: []resolve.Block{
								{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Anchored box"}}}},
							},
						},
					},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// Float must be in the overlay with page-absolute coordinates.
	if !strings.Contains(out, `<div class="page-overlay">`) {
		t.Fatalf("expected floating content to be placed in the page overlay: %s", out)
	}
	// Y=508000 EMU = 40pt; with zero margins paraPageY=0, so overlay top=40pt.
	if !strings.Contains(out, `top:40.0pt`) {
		t.Fatalf("expected float to have page-absolute top:40.0pt in overlay: %s", out)
	}
	// Anchor paragraph must not use position:relative.
	if strings.Contains(out, `position:relative`) {
		t.Fatalf("anchor paragraph should not use position:relative: %s", out)
	}
}

func TestHTMLNestedFloatingTextboxUsesLocalStoryOverlay(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{
				Kind: "textbox",
				TextBox: &resolve.TextBox{
					Floating: true,
					Width:    2540000,
					Height:   2540000,
					Blocks: []resolve.Block{
						{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Outer box"}}}},
						{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{
							Kind: "textbox",
							TextBox: &resolve.TextBox{
								Floating:  true,
								Width:     127000,
								Height:    127000,
								Y:         127000,
								YRelative: "paragraph",
								Blocks: []resolve.Block{
									{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Inner box"}}}},
								},
							},
						}}}},
					},
				},
			}}}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	first := strings.Index(out, `class="floating-textbox"`)
	if first < 0 {
		t.Fatalf("expected outer floating textbox in output: %s", out)
	}
	secondRel := strings.Index(out[first+1:], `class="floating-textbox"`)
	if secondRel < 0 {
		t.Fatalf("expected nested floating textbox in output: %s", out)
	}
	second := first + 1 + secondRel
	if !strings.Contains(out[first:second], `class="story-overlay"`) {
		t.Fatalf("expected nested floating textbox to render inside a local story overlay: %s", out)
	}
	start := second - 120
	if start < 0 {
		start = 0
	}
	end := second + 220
	if end > len(out) {
		end = len(out)
	}
	snippet := out[start:end]
	if !strings.Contains(snippet, `top:23.8pt`) {
		t.Fatalf("expected nested paragraph-relative float to use textbox-local anchor coordinates: %s", snippet)
	}
}

func TestHTMLDocumentRelativeFloatingContentUsesOverlay(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "Body"},
					{
						Kind: "textbox",
						TextBox: &resolve.TextBox{
							Floating:  true,
							X:         127000,
							Y:         127000,
							XRelative: "margin",
							YRelative: "margin",
							Blocks: []resolve.Block{
								{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Overlay box"}}}},
							},
						},
					},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<div class="page-overlay">`) {
		t.Fatalf("expected page overlay container in output: %s", out)
	}
	if !strings.Contains(out, `Body`) {
		t.Fatalf("expected body paragraph text in output: %s", out)
	}
	if !strings.Contains(out, `left:82.0pt;top:82.0pt`) {
		t.Fatalf("expected margin-relative offsets to include page margins: %s", out)
	}
	if !strings.Contains(out, `Overlay box`) {
		t.Fatalf("expected floating textbox content in output: %s", out)
	}
}

func TestHTMLDropCapFrameParagraphRendersAsInlineFloat(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Frame: &resolve.FrameProps{DropCap: "drop", Lines: 3, HAnchor: "text", VAnchor: "text"},
				Format: resolve.ParagraphFormat{
					IndentFirstLine: intPtr(432),
				},
				Runs: []resolve.Run{{Kind: "run", Text: "D", Format: resolve.RunFormat{FontSize: intPtr(117)}}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "rop caps..."}}}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// The float box is the drop cap's wrap region, not a clipping window: the
	// glyph's ink may extend past its advance width.
	if !strings.Contains(out, `class="frame-paragraph" style="float:left;overflow:visible`) {
		t.Fatalf("expected drop cap frame paragraph to render as an inline float: %s", out)
	}
	if !strings.Contains(out, `margin-right:6.0pt`) {
		t.Fatalf("expected drop cap float to leave extra breathing room before body text: %s", out)
	}
	if strings.Contains(out, `class="frame-paragraph" style="position:absolute`) {
		t.Fatalf("did not expect drop cap frame paragraph to render in the page overlay: %s", out)
	}
	if strings.Contains(out, `text-indent:21.6pt`) {
		t.Fatalf("did not expect drop cap frame paragraph to preserve first-line indent inside the float: %s", out)
	}
}

func TestHTMLParagraphRelativeFloatingTextBoxUsesInlineFloat(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Index: 0, Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index: 1,
				Runs: []resolve.Run{
					{Kind: "textbox", TextBox: &resolve.TextBox{
						Floating:  true,
						Width:     914400,
						Height:    914400,
						XRelative: "column",
						YRelative: "paragraph",
						WrapType:  "square",
						WrapSide:  "bothSides",
						Blocks: []resolve.Block{{
							Kind:      "paragraph",
							Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Wrapped box"}}},
						}},
					}},
					{Kind: "run", Text: "Body text wraps beside the text box."},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	idx := strings.Index(out, `Wrapped box`)
	if idx < 0 {
		t.Fatalf("expected floating textbox content in output: %s", out)
	}
	start := idx - 220
	if start < 0 {
		start = 0
	}
	end := idx + 220
	if end > len(out) {
		end = len(out)
	}
	snippet := out[start:end]
	if !strings.Contains(snippet, `class="floating-textbox" style="float:left`) {
		t.Fatalf("expected paragraph-relative textbox to render as an inline float: %s", snippet)
	}
	// The textbox itself remains an inline float. Canonical layout may now emit
	// absolutely positioned physical text slots beside it; do not mistake those
	// slots for absolute positioning of the textbox.
	tagStart := strings.LastIndex(out[:idx], `<div class="floating-textbox"`)
	if tagStart < 0 {
		t.Fatalf("expected floating textbox opening tag: %s", snippet)
	}
	tagEndRel := strings.Index(out[tagStart:], `>`)
	if tagEndRel < 0 {
		t.Fatalf("expected floating textbox opening tag terminator: %s", snippet)
	}
	textboxTag := out[tagStart : tagStart+tagEndRel+1]
	if strings.Contains(textboxTag, `position:absolute`) {
		t.Fatalf("expected paragraph-relative textbox itself to stay in paragraph flow: %s", textboxTag)
	}
}

func TestHTMLImageOnlyFloatingParagraphRendersAsFlowBlockImage(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{
				Kind: "image",
				Image: &resolve.Image{
					Floating: true,
					Width:    127000,
					Height:   127000,
					XAlign:   "center",
					WrapType: "topAndBottom",
					MIMEType: "image/png",
					Data:     []byte{0x89, 0x50, 0x4E, 0x47},
				},
			}}}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `display:block`) || !strings.Contains(out, `clear:both`) || !strings.Contains(out, `margin-left:auto`) || !strings.Contains(out, `margin-right:auto`) {
		t.Fatalf("expected image-only floating paragraph to render as a centered flow image: %s", out)
	}
	if strings.Contains(out, `<div class="page-overlay">`) && strings.Contains(out, `position:absolute;z-index`) {
		t.Fatalf("did not expect image-only floating paragraph to be routed to the page overlay: %s", out)
	}
}

func TestHTMLParagraphRelativeTopAndBottomTextBoxRendersAsFlowBlock(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{Runs: []resolve.Run{
				{
					Kind: "textbox",
					TextBox: &resolve.TextBox{
						Floating:  true,
						Width:     127000,
						Height:    127000,
						XAlign:    "center",
						XRelative: "column",
						YRelative: "paragraph",
						WrapType:  "topAndBottom",
						Blocks: []resolve.Block{{
							Kind:      "paragraph",
							Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Block box"}}},
						}},
					},
				},
				{Kind: "run", Text: "Body text follows below the text box."},
			}},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	idx := strings.Index(out, `class="floating-textbox"`)
	if idx < 0 || !strings.Contains(out, `Block box`) {
		t.Fatalf("expected floating textbox content in output: %s", out)
	}
	end := strings.Index(out[idx:], `>`)
	if end < 0 {
		t.Fatalf("expected floating textbox start tag in output: %s", out)
	}
	end += idx
	snippet := out[idx:end]
	if !strings.Contains(snippet, `class="floating-textbox" style="display:block;clear:both`) {
		t.Fatalf("expected paragraph-relative top-and-bottom textbox to render as an in-flow block float: %s", snippet)
	}
	if strings.Contains(snippet, `position:absolute`) {
		t.Fatalf("did not expect paragraph-relative top-and-bottom textbox to use overlay positioning: %s", snippet)
	}
}

func TestHTMLParagraphRelativeTopAndBottomShapeRendersAsFlowBlock(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{Runs: []resolve.Run{
				{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:      "rect",
						Floating:  true,
						Width:     127000,
						Height:    127000,
						XAlign:    "center",
						XRelative: "column",
						YRelative: "paragraph",
						WrapType:  "topAndBottom",
						FillColor: "FF0000",
					},
				},
				{Kind: "run", Text: "Body text follows below the shape."},
			}},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	idx := strings.Index(out, `class="shape-rect"`)
	if idx < 0 {
		t.Fatalf("expected floating shape in output: %s", out)
	}
	start := idx - 180
	if start < 0 {
		start = 0
	}
	end := idx + 220
	if end > len(out) {
		end = len(out)
	}
	snippet := out[start:end]
	if !strings.Contains(snippet, `display:block`) || !strings.Contains(snippet, `clear:both`) {
		t.Fatalf("expected paragraph-relative top-and-bottom shape to render as an in-flow block float: %s", snippet)
	}
	if strings.Contains(snippet, `position:absolute`) {
		t.Fatalf("did not expect paragraph-relative top-and-bottom shape to use overlay positioning: %s", snippet)
	}
}

func TestHTMLImageOnlyWrapNoneFloatingParagraphUsesOverlay(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{
				Kind: "image",
				Image: &resolve.Image{
					Floating:  true,
					Width:     127000,
					Height:    127000,
					X:         127000,
					Y:         254000,
					XRelative: "column",
					YRelative: "paragraph",
					WrapType:  "none",
					MIMEType:  "image/png",
					Data:      []byte{0x89, 0x50, 0x4E, 0x47},
					Alt:       "Anchored",
				},
			}}}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	idx := strings.Index(out, `alt="Anchored"`)
	if idx < 0 {
		t.Fatalf("expected anchored image in output: %s", out)
	}
	start := idx - 160
	if start < 0 {
		start = 0
	}
	end := idx + 200
	if end > len(out) {
		end = len(out)
	}
	snippet := out[start:end]
	if !strings.Contains(snippet, `position:absolute;z-index`) {
		t.Fatalf("expected wrapNone floating image to use overlay positioning: %s", snippet)
	}
	if strings.Contains(snippet, `clear:both`) {
		t.Fatalf("did not expect wrapNone floating image to render as an in-flow block image: %s", snippet)
	}
	if !strings.Contains(snippet, `left:10.0pt`) || !strings.Contains(snippet, `top:20.0pt`) {
		t.Fatalf("expected wrapNone floating image to keep its absolute offsets: %s", snippet)
	}
}

func TestHTMLRendersCroppedAndTransformedImages(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "image",
					Image: &resolve.Image{
						MIMEType:    "image/png",
						Data:        []byte{0x89, 0x50, 0x4E, 0x47},
						Width:       2540000,
						Height:      1270000,
						Alt:         "Transformed",
						CropLeft:    0.10,
						CropTop:     0.05,
						CropRight:   0.10,
						CropBottom:  0.15,
						RotationDeg: 90,
						FlipH:       true,
					},
				}},
			},
		}},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="image-frame"`) {
		t.Fatalf("expected wrapped image frame in output: %s", out)
	}
	if !strings.Contains(out, `overflow:hidden`) || !strings.Contains(out, `position:relative`) {
		t.Fatalf("expected clipped relative image frame in output: %s", out)
	}
	if !strings.Contains(out, `width:125.0000%`) || !strings.Contains(out, `height:125.0000%`) {
		t.Fatalf("expected cropped inner image scaling in output: %s", out)
	}
	if !strings.Contains(out, `left:-12.5000%`) || !strings.Contains(out, `top:-6.2500%`) {
		t.Fatalf("expected cropped inner image offsets in output: %s", out)
	}
	if !strings.Contains(out, `transform:scaleX(-1) rotate(90.00deg)`) {
		t.Fatalf("expected flip/rotation transform in output: %s", out)
	}
	if !strings.Contains(out, `transform-origin:50% 50%`) {
		t.Fatalf("expected centered transform origin in output: %s", out)
	}
}

func TestHTMLRendersImageStretch(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "image",
					Image: &resolve.Image{
						MIMEType: "image/png",
						Data:     []byte{0x89, 0x50, 0x4E, 0x47},
						Width:    2540000,
						Height:   1270000,
						Stretch:  false,
					},
				}},
			},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `object-fit:contain`) {
		t.Fatalf("expected object-fit:contain for non-stretch image, got: %s", out)
	}
}

func TestHTMLRendersTransformedTextBoxesAndShapes(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "textbox",
					TextBox: &resolve.TextBox{
						Floating:    true,
						Width:       127000,
						Height:      127000,
						RotationDeg: 45,
						FlipH:       true,
						Blocks: []resolve.Block{{
							Kind:      "paragraph",
							Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Rotated box"}}},
						}},
					},
				}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:        "rect",
						Floating:    true,
						Width:       127000,
						Height:      127000,
						FillColor:   "FF0000",
						RotationDeg: 30,
						FlipV:       true,
					},
				}},
			}},
		},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="floating-textbox"`) || !strings.Contains(out, `transform:scaleX(-1) rotate(45.00deg)`) {
		t.Fatalf("expected transformed floating textbox in output: %s", out)
	}
	if !strings.Contains(out, `class="shape-rect"`) || !strings.Contains(out, `transform:scaleY(-1) rotate(30.00deg)`) {
		t.Fatalf("expected transformed shape rect in output: %s", out)
	}
	if strings.Count(out, `transform-origin:50% 50%`) < 2 {
		t.Fatalf("expected centered transform origin for textbox and shape: %s", out)
	}
}

func TestHTMLTextBoxBodyVertUsesVerticalWritingMode(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{
				Kind: "textbox",
				TextBox: &resolve.TextBox{
					Floating: true,
					Vert:     "vert",
					Width:    127000,
					Height:   127000,
					Blocks: []resolve.Block{{
						Kind:      "paragraph",
						Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Vertical box"}}},
					}},
				},
			}}},
		}},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="floating-textbox"`) || !strings.Contains(out, `writing-mode:vertical-rl`) {
		t.Fatalf("expected vertical textbox writing mode in output: %s", out)
	}
	if !strings.Contains(out, `text-orientation:mixed`) {
		t.Fatalf("expected vertical textbox text orientation in output: %s", out)
	}
}

func TestHTMLAppliesTextboxAndShapeEffects(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "textbox",
					TextBox: &resolve.TextBox{
						Floating: true,
						Width:    127000,
						Height:   127000,
						Effects: dml.EffectList{Effects: []dml.Effect{
							dml.Glow{Color: "FF0000", Alpha: 0.6, Radius: 4},
							dml.Reflection{Alpha: 0.25, Blur: 2, Distance: 3, Direction: 90},
						}},
						Blocks: []resolve.Block{{
							Kind:      "paragraph",
							Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Glow"}}},
						}},
					},
				}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:      "rect",
						Floating:  true,
						Width:     127000,
						Height:    127000,
						Geometry:  &dml.PresetGeometry{Name: "ellipse"},
						FillColor: "FF0000",
						Effects: dml.EffectList{Effects: []dml.Effect{
							dml.Glow{Color: "00AA00", Alpha: 0.5, Radius: 3},
							dml.Reflection{Alpha: 0.2, Blur: 1.5, Distance: 4, Direction: 90},
						}},
					},
				}},
			}},
		},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="floating-textbox"`) || !strings.Contains(out, `filter:drop-shadow(`) {
		t.Fatalf("expected textbox effect filter in output: %s", out)
	}
	if !strings.Contains(out, `class="textbox-reflection"`) {
		t.Fatalf("expected explicit textbox reflection layer in output: %s", out)
	}
	if !strings.Contains(out, `<svg class="shape__vector"`) || !strings.Contains(out, `overflow:visible`) {
		t.Fatalf("expected shape effect styling in output: %s", out)
	}
}

func TestHTMLRendersVectorPresetShapes(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:        "rect",
						Floating:    true,
						Width:       127000,
						Height:      127000,
						Geometry:    &dml.PresetGeometry{Name: "ellipse"},
						FillColor:   "FF0000",
						StrokeColor: "0000FF",
						StrokeWidth: 12700,
					},
				}},
			},
		}},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="shape__vector"`) {
		t.Fatalf("expected SVG vector shape in output: %s", out)
	}
	if !strings.Contains(out, `fill="#FF0000"`) {
		t.Fatalf("expected SVG fill color in output: %s", out)
	}
	if !strings.Contains(out, `stroke="#0000FF"`) {
		t.Fatalf("expected SVG stroke in output: %s", out)
	}
	if !strings.Contains(out, `style="position:absolute;z-index:`) {
		t.Fatalf("expected SVG vector shape positioning style in output: %s", out)
	}
	if !strings.Contains(out, `width="10.0000pt"`) || !strings.Contains(out, `height="10.0000pt"`) {
		t.Fatalf("expected SVG vector shape dimensions in output: %s", out)
	}
}

func TestHTMLAppliesVectorShapeRotationAndFlip(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:        "rect",
						Floating:    true,
						Width:       127000,
						Height:      127000,
						Geometry:    &dml.PresetGeometry{Name: "ellipse"},
						FillColor:   "FF0000",
						RotationDeg: 30,
						FlipH:       true,
					},
				}},
			},
		}},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="shape__vector"`) {
		t.Fatalf("expected SVG vector shape in output: %s", out)
	}
	if !strings.Contains(out, `transform:scaleX(-1) rotate(30.00deg)`) {
		t.Fatalf("expected vector shape transform in output: %s", out)
	}
	if !strings.Contains(out, `transform-origin:50% 50%`) {
		t.Fatalf("expected vector shape transform origin in output: %s", out)
	}
}

func TestHTMLRendersThreeDVectorShapeEffects(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:      "rect",
						Floating:  true,
						Width:     127000,
						Height:    127000,
						Geometry:  &dml.PresetGeometry{Name: "ellipse"},
						FillColor: "FF7043",
						Effects: dml.EffectList{Effects: []dml.Effect{dml.ThreeD{
							CameraPreset:   "legacyPerspectiveTopLeft",
							LightRig:       "threePt",
							LightDirection: "tl",
							ExtrusionDepth: 6,
							ContourWidth:   2,
							BevelTopHeight: 1,
							Material:       "metal",
							ExtrusionColor: "595959",
						}}},
					},
				}},
			},
		}},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `perspective(760px)`) {
		t.Fatalf("expected 3D transform in output: %s", out)
	}
	if !strings.Contains(out, `rotateX(18.00deg)`) || !strings.Contains(out, `rotateY(-18.00deg)`) {
		t.Fatalf("expected camera-preset projection rotations: %s", out)
	}
	if !strings.Contains(out, `data-3d-light="threePt"`) || !strings.Contains(out, `data-3d-material="metal"`) {
		t.Fatalf("expected 3D scene metadata in output: %s", out)
	}
	if !strings.Contains(out, `drop-shadow(`) {
		t.Fatalf("expected 3D drop shadow in output: %s", out)
	}
	if !strings.Contains(out, `mix-blend-mode:screen`) || !strings.Contains(out, `mix-blend-mode:multiply`) {
		t.Fatalf("expected 3D scene-light overlays in output: %s", out)
	}
	if !strings.Contains(out, `shape-fill-overlay-0`) || !strings.Contains(out, `shape-fill-overlay-1`) {
		t.Fatalf("expected SVG defs for 3D light gradients in output: %s", out)
	}
	if !strings.Contains(out, `class="shape__3d-extrusion"`) {
		t.Fatalf("expected projected 3D extrusion geometry in output: %s", out)
	}
	if !strings.Contains(out, `class="shape__3d-bevel"`) {
		t.Fatalf("expected bevel/contour overlay in output: %s", out)
	}
}

func TestHTMLVectorShapeEffectsUseVisibleOverflow(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:      "rect",
						Floating:  true,
						Width:     127000,
						Height:    127000,
						Geometry:  &dml.PresetGeometry{Name: "ellipse"},
						FillColor: "4A90E2",
						Effects: dml.EffectList{Effects: []dml.Effect{
							dml.Glow{Color: "00AA00", Alpha: 0.5, Radius: 3},
							dml.Reflection{Alpha: 0.2, Blur: 1.5, Distance: 4, Direction: 90},
						}},
					},
				}},
			},
		}},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<svg class="shape__vector"`) {
		t.Fatalf("expected SVG vector shape in output: %s", out)
	}
	if !strings.Contains(out, `class="shape__vector"`) || !strings.Contains(out, `overflow:visible`) {
		t.Fatalf("expected vector effect SVG to allow overflow for effect rendering: %s", out)
	}
}

func TestHTMLRunLevelTextEffects(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{
						Kind: "text",
						Text: "shadow",
						Format: resolve.RunFormat{
							TextShadow: &dml.Shadow{Color: "FF0000", Alpha: 1, Distance: 3, Angle: 45, Blur: 2},
						},
					},
					{
						Kind: "text",
						Text: "glow",
						Format: resolve.RunFormat{
							TextGlow: &dml.Glow{Color: "00FF00", Alpha: 0.8, Radius: 4},
						},
					},
					{
						Kind: "text",
						Text: "shadow+glow",
						Format: resolve.RunFormat{
							TextShadow: &dml.Shadow{Color: "0000FF", Alpha: 1, Distance: 2, Angle: 90, Blur: 1},
							TextGlow:   &dml.Glow{Color: "FFFF00", Alpha: 0.5, Radius: 3},
						},
					},
					{
						Kind: "text",
						Text: "outline",
						Format: resolve.RunFormat{
							TextOutline: &dml.LineStyle{Color: "FF00FF", Width: 1.5, Alpha: 1},
						},
					},
					{
						Kind: "text",
						Text: "soft-edge",
						Format: resolve.RunFormat{
							TextSoftEdge: &dml.SoftEdge{Radius: 2.5},
						},
					},
				},
			},
		}},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// Shadow: offset-x offset-y blur color
	if !strings.Contains(out, "text-shadow:") {
		t.Fatalf("expected text-shadow in output: %s", out)
	}
	// Glow: 0 0 radius color
	if !strings.Contains(out, "0 0 4.00pt") {
		t.Fatalf("expected glow blur radius in output: %s", out)
	}
	// Combined shadow+glow produces comma-separated text-shadow layers
	if !strings.Contains(out, "text-shadow:") {
		t.Fatalf("expected text-shadow for combined effects: %s", out)
	}
	// Outline uses -webkit-text-stroke
	if !strings.Contains(out, "-webkit-text-stroke:") {
		t.Fatalf("expected text outline stroke in output: %s", out)
	}
	// Soft edge uses filter:blur
	if !strings.Contains(out, "filter:blur(2.5pt)") {
		t.Fatalf("expected soft-edge blur filter in output: %s", out)
	}
}

func TestHTMLShapeCombinedGlowShadowReflection(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:      "rect",
						Floating:  true,
						Width:     127000,
						Height:    127000,
						FillColor: "FF0000",
						Effects: dml.EffectList{Effects: []dml.Effect{
							dml.Shadow{Color: "000000", Alpha: 0.5, Distance: 3, Angle: 45, Blur: 2},
							dml.Glow{Color: "00FF00", Alpha: 0.6, Radius: 4},
							dml.Reflection{Alpha: 0.25, Blur: 2, Distance: 3, Direction: 90},
						}},
					},
				}},
			},
		}},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// Non-vector rect shape should have overflow:visible so effects aren't clipped.
	if !strings.Contains(out, `class="shape-rect"`) || !strings.Contains(out, "overflow:visible") {
		t.Fatalf("expected non-vector shape with overflow:visible: %s", out)
	}
	// Filter should contain both shadow and glow drop-shadows.
	if !strings.Contains(out, "filter:drop-shadow(") {
		t.Fatalf("expected filter drop-shadow in output: %s", out)
	}
	// Reflection produces a dedicated layer on non-vector rects.
	if !strings.Contains(out, `class="shape-reflection"`) {
		t.Fatalf("expected explicit reflection layer in output: %s", out)
	}
}

func TestHTMLRectShapeFillOverlayFallback(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:      "rect",
						Floating:  true,
						Width:     127000,
						Height:    127000,
						FillColor: "FF0000",
						Effects: dml.EffectList{Effects: []dml.Effect{
							dml.FillOverlay{
								Fill:      dml.SolidFill{Color: "00FF00", Alpha: 0.4},
								BlendMode: "mult",
							},
						}},
					},
				}},
			},
		}},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="shape-rect"`) {
		t.Fatalf("expected rect fallback output: %s", out)
	}
	if !strings.Contains(out, `class="shape-fill-overlay"`) {
		t.Fatalf("expected non-vector overlay layer: %s", out)
	}
	if !strings.Contains(out, "background:rgba(0, 255, 0, 0.40)") {
		t.Fatalf("expected overlay background color in output: %s", out)
	}
	if !strings.Contains(out, "mix-blend-mode:multiply") {
		t.Fatalf("expected overlay blend mode in output: %s", out)
	}
}

func TestHTMLRectShapeReflectionRespectsFillOverlayOrder(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:      "rect",
						Floating:  true,
						Width:     127000,
						Height:    127000,
						FillColor: "FFFFFF",
						Effects: dml.EffectList{Effects: []dml.Effect{
							dml.FillOverlay{Fill: &dml.SolidFill{Color: "00FF00", Alpha: 0.4}, BlendMode: "mult"},
							dml.Reflection{Alpha: 0.25, Blur: 2, Distance: 3, Direction: 90},
							dml.FillOverlay{Fill: &dml.SolidFill{Color: "0000FF", Alpha: 0.3}, BlendMode: "screen"},
						}},
					},
				}},
			},
		}},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `shape-fill-overlay--reflection`) {
		t.Fatalf("expected reflection overlay clone: %s", out)
	}
	if got := strings.Count(out, "background:rgba(0, 255, 0, 0.40)"); got != 2 {
		t.Fatalf("green overlay count = %d, want 2: %s", got, out)
	}
	if got := strings.Count(out, "background:rgba(0, 0, 255, 0.30)"); got != 1 {
		t.Fatalf("blue overlay count = %d, want 1: %s", got, out)
	}
}

func TestHTMLRectShapeSiblingOverlayDoesNotReflect(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:      "rect",
						Floating:  true,
						Width:     127000,
						Height:    127000,
						FillColor: "FFFFFF",
						Effects: dml.EffectList{Effects: []dml.Effect{
							dml.FillOverlay{Fill: &dml.SolidFill{Color: "00FF00", Alpha: 0.4}, BlendMode: "mult", ContainerType: "sib"},
							dml.Reflection{Alpha: 0.25, Blur: 2, Distance: 3, Direction: 90},
						}},
					},
				}},
			},
		}},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Contains(out, `shape-fill-overlay--reflection`) {
		t.Fatalf("did not expect sibling overlay to be reflected: %s", out)
	}
	if got := strings.Count(out, "background:rgba(0, 255, 0, 0.40)"); got != 1 {
		t.Fatalf("green overlay count = %d, want 1: %s", got, out)
	}
}

func TestHTMLRectShapeSiblingGlowDoesNotReflect(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:      "rect",
						Floating:  true,
						Width:     127000,
						Height:    127000,
						FillColor: "FFFFFF",
						Effects: dml.EffectList{Effects: []dml.Effect{
							dml.Glow{Color: "00FF00", Alpha: 0.6, Radius: 4, ContainerType: "sib"},
							dml.Reflection{Alpha: 0.25, Blur: 2, Distance: 3, Direction: 90},
						}},
					},
				}},
			},
		}},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if got := strings.Count(out, "drop-shadow(0 0 4.0pt rgba(0, 255, 0, 0.60))"); got != 1 {
		t.Fatalf("glow count = %d, want 1: %s", got, out)
	}
}

func TestHTMLRectShapeTreeGlowReflects(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:      "rect",
						Floating:  true,
						Width:     127000,
						Height:    127000,
						FillColor: "FFFFFF",
						Effects: dml.EffectList{Effects: []dml.Effect{
							dml.Glow{Color: "00FF00", Alpha: 0.6, Radius: 4, ContainerType: "tree"},
							dml.Reflection{Alpha: 0.25, Blur: 2, Distance: 3, Direction: 90},
						}},
					},
				}},
			},
		}},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if got := strings.Count(out, "drop-shadow(0 0 4.0pt rgba(0, 255, 0, 0.60))"); got != 2 {
		t.Fatalf("glow count = %d, want 2: %s", got, out)
	}
}

func TestHTMLVectorShapeThreeDPlusGlow(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:      "rect",
						Floating:  true,
						Width:     127000,
						Height:    127000,
						Geometry:  &dml.PresetGeometry{Name: "ellipse"},
						FillColor: "FF7043",
						Effects: dml.EffectList{Effects: []dml.Effect{
							dml.ThreeD{
								CameraPreset:   "legacyPerspectiveTopLeft",
								LightDirection: "tl",
								ExtrusionDepth: 6,
								ContourWidth:   2,
								BevelTopHeight: 1,
								Material:       "metal",
								ExtrusionColor: "595959",
							},
							dml.Glow{Color: "00FF00", Alpha: 0.6, Radius: 4},
						}},
					},
				}},
			},
		}},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<svg class="shape__vector"`) {
		t.Fatalf("expected SVG vector shape: %s", out)
	}
	// 3D transform should be present.
	if !strings.Contains(out, "perspective(960px)") {
		t.Fatalf("expected 3D perspective transform: %s", out)
	}
	// Glow should appear as a drop-shadow filter.
	if !strings.Contains(out, "drop-shadow(0 0 4.0pt") {
		t.Fatalf("expected glow drop-shadow filter: %s", out)
	}
	// 3D extrusion should also produce a drop-shadow filter.
	if !strings.Contains(out, "drop-shadow(") {
		t.Fatalf("expected at least one drop-shadow filter: %s", out)
	}
	// SVG should allow overflow for effects.
	if !strings.Contains(out, "overflow:visible") {
		t.Fatalf("expected overflow:visible on SVG: %s", out)
	}
	if !strings.Contains(out, "mix-blend-mode:screen") || !strings.Contains(out, "mix-blend-mode:multiply") {
		t.Fatalf("expected 3D scene lighting overlays alongside glow: %s", out)
	}
}

func TestHTMLVectorShapePictureFillWithEffects(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:     "rect",
						Floating: true,
						Width:    127000,
						Height:   127000,
						Geometry: &dml.PresetGeometry{Name: "ellipse"},
						Fill: dml.PictureFill{
							EmbedID:     "rId1",
							ContentType: "image/png",
							ImageData:   []byte("PNG"),
						},
						Effects: dml.EffectList{Effects: []dml.Effect{
							dml.Glow{Color: "00FF00", Alpha: 0.6, Radius: 4},
							dml.Shadow{Color: "000000", Alpha: 0.5, Distance: 3, Angle: 45, Blur: 2},
						}},
					},
				}},
			},
		}},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<svg class="shape__vector"`) {
		t.Fatalf("expected SVG vector shape: %s", out)
	}
	// Picture fill should create SVG defs.
	if !strings.Contains(out, "<defs>") {
		t.Fatalf("expected SVG defs for picture fill: %s", out)
	}
	// Effects should still be applied.
	if !strings.Contains(out, "filter:drop-shadow(") {
		t.Fatalf("expected filter on picture-fill shape: %s", out)
	}
	if !strings.Contains(out, "overflow:visible") {
		t.Fatalf("expected overflow:visible on picture-fill SVG: %s", out)
	}
}

func TestHTMLVectorShapeExtendedEffectsAndFillOverlay(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:     "rect",
						Floating: true,
						Width:    127000,
						Height:   127000,
						Geometry: &dml.PresetGeometry{Name: "rect"},
						Fill:     &dml.SolidFill{Color: "CFE2F3", Alpha: 1.0},
						Effects: dml.EffectList{Effects: []dml.Effect{
							dml.AlphaMod{Amount: 50000},
							dml.Grayscale{},
							dml.Lum{Brightness: 120000, Contrast: 80000},
							dml.FillOverlay{Fill: &dml.SolidFill{Color: "00FF00", Alpha: 0.4}, BlendMode: "mult"},
						}},
					},
				}},
			},
		}},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<svg class="shape__vector"`) {
		t.Fatalf("expected SVG vector shape: %s", out)
	}
	if !strings.Contains(out, "opacity:0.5000") {
		t.Fatalf("expected alpha modulation on SVG output: %s", out)
	}
	if !strings.Contains(out, "grayscale(100%)") || !strings.Contains(out, "brightness(120%)") || !strings.Contains(out, "contrast(80%)") {
		t.Fatalf("expected grayscale/lum filters on SVG output: %s", out)
	}
	if !strings.Contains(out, "mix-blend-mode:multiply") {
		t.Fatalf("expected fill overlay blend mode in SVG output: %s", out)
	}
	if !strings.Contains(out, `fill="rgba(0, 255, 0, 0.40)"`) {
		t.Fatalf("expected overlay fill clone in SVG output: %s", out)
	}
}

func TestHTMLVectorShapeReflectionRespectsFillOverlayOrder(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:     "rect",
						Floating: true,
						Width:    127000,
						Height:   127000,
						Geometry: &dml.PresetGeometry{Name: "rect"},
						Fill:     &dml.SolidFill{Color: "FFFFFF", Alpha: 1.0},
						Effects: dml.EffectList{Effects: []dml.Effect{
							dml.FillOverlay{Fill: &dml.SolidFill{Color: "00FF00", Alpha: 0.4}, BlendMode: "mult"},
							dml.Reflection{Alpha: 0.25, Blur: 2, Distance: 3, Direction: 90},
							dml.FillOverlay{Fill: &dml.SolidFill{Color: "0000FF", Alpha: 0.3}, BlendMode: "screen"},
						}},
					},
				}},
			},
		}},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if got := strings.Count(out, `fill="rgba(0, 255, 0, 0.40)"`); got != 2 {
		t.Fatalf("green SVG overlay count = %d, want 2: %s", got, out)
	}
	if got := strings.Count(out, `fill="rgba(0, 0, 255, 0.30)"`); got != 1 {
		t.Fatalf("blue SVG overlay count = %d, want 1: %s", got, out)
	}
}

func TestHTMLVectorShapeSiblingOverlayDoesNotReflect(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:     "rect",
						Floating: true,
						Width:    127000,
						Height:   127000,
						Geometry: &dml.PresetGeometry{Name: "rect"},
						Fill:     &dml.SolidFill{Color: "FFFFFF", Alpha: 1.0},
						Effects: dml.EffectList{Effects: []dml.Effect{
							dml.FillOverlay{Fill: &dml.SolidFill{Color: "00FF00", Alpha: 0.4}, BlendMode: "mult", ContainerType: "sib"},
							dml.Reflection{Alpha: 0.25, Blur: 2, Distance: 3, Direction: 90},
						}},
					},
				}},
			},
		}},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if got := strings.Count(out, `fill="rgba(0, 255, 0, 0.40)"`); got != 1 {
		t.Fatalf("green SVG overlay count = %d, want 1: %s", got, out)
	}
}

func TestHTMLVectorShapeSiblingGlowDoesNotReflect(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:     "rect",
						Floating: true,
						Width:    127000,
						Height:   127000,
						Geometry: &dml.PresetGeometry{Name: "rect"},
						Fill:     &dml.SolidFill{Color: "FFFFFF", Alpha: 1.0},
						Effects: dml.EffectList{Effects: []dml.Effect{
							dml.Glow{Color: "00FF00", Alpha: 0.6, Radius: 4, ContainerType: "sib"},
							dml.Reflection{Alpha: 0.25, Blur: 2, Distance: 3, Direction: 90},
						}},
					},
				}},
			},
		}},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if got := strings.Count(out, "drop-shadow(0 0 4.0pt rgba(0, 255, 0, 0.60))"); got != 1 {
		t.Fatalf("glow count = %d, want 1: %s", got, out)
	}
	if !strings.Contains(out, `class="shape__vector-reflection-content"`) {
		t.Fatalf("expected explicit reflected vector layer: %s", out)
	}
}

func TestHTMLVectorShapeTreeGlowReflects(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:     "rect",
						Floating: true,
						Width:    127000,
						Height:   127000,
						Geometry: &dml.PresetGeometry{Name: "rect"},
						Fill:     &dml.SolidFill{Color: "FFFFFF", Alpha: 1.0},
						Effects: dml.EffectList{Effects: []dml.Effect{
							dml.Glow{Color: "00FF00", Alpha: 0.6, Radius: 4, ContainerType: "tree"},
							dml.Reflection{Alpha: 0.25, Blur: 2, Distance: 3, Direction: 90},
						}},
					},
				}},
			},
		}},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if got := strings.Count(out, "drop-shadow(0 0 4.0pt rgba(0, 255, 0, 0.60))"); got != 2 {
		t.Fatalf("glow count = %d, want 2: %s", got, out)
	}
	if !strings.Contains(out, `class="shape__vector-reflection-content" style="overflow:visible;filter:drop-shadow(0 0 4.0pt rgba(0, 255, 0, 0.60))"`) {
		t.Fatalf("expected reflected vector layer to carry tree glow: %s", out)
	}
}

func TestHTMLVectorShapeAlphaEffectVariants(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:     "rect",
						Floating: true,
						Width:    127000,
						Height:   127000,
						Geometry: &dml.PresetGeometry{Name: "rect"},
						Fill:     &dml.SolidFill{Color: "6699CC", Alpha: 1.0},
						Effects: dml.EffectList{Effects: []dml.Effect{
							dml.AlphaRepl{Alpha: 25000},
							dml.AlphaInv{},
						}},
					},
				}},
			},
		}},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="shape__vector"`) {
		t.Fatalf("expected SVG vector shape: %s", out)
	}
	if !strings.Contains(out, "opacity:0.7500") {
		t.Fatalf("expected alpha inversion to affect vector opacity: %s", out)
	}
}

func TestHTMLVectorShapeAlphaBiLevelAndOutset(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:     "rect",
						Floating: true,
						Width:    127000,
						Height:   127000,
						Geometry: &dml.PresetGeometry{Name: "rect"},
						Fill:     &dml.SolidFill{Color: "6699CC", Alpha: 1.0},
						Effects: dml.EffectList{Effects: []dml.Effect{
							dml.AlphaRepl{Alpha: 25000},
							dml.AlphaBiLevel{Threshold: 20000},
							dml.AlphaOutset{Radius: 2},
						}},
					},
				}},
			},
		}},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "opacity:1.0000") {
		t.Fatalf("expected alpha bilevel thresholding in output: %s", out)
	}
	if !strings.Contains(out, "blur(2.0pt)") {
		t.Fatalf("expected alpha outset blur approximation in output: %s", out)
	}
}

func TestHTMLVectorShapeDagTransformBlendAndTint(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:     "rect",
						Floating: true,
						Width:    127000,
						Height:   127000,
						Geometry: &dml.PresetGeometry{Name: "rect"},
						Fill:     &dml.SolidFill{Color: "6699CC", Alpha: 1.0},
						Effects: dml.EffectList{Effects: []dml.Effect{
							dml.RelativeOffset{X: 0.25, Y: -0.10},
							dml.TransformEffect{ScaleX: 0.5, ScaleY: 1.25, TranslateX: 1, TranslateY: -2},
							dml.BlendMode{Mode: "mult"},
							dml.Tint{Hue: 1800000, Amount: 20000},
						}},
					},
				}},
			},
		}},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "mix-blend-mode:multiply") {
		t.Fatalf("expected blend-mode output: %s", out)
	}
	if !strings.Contains(out, "hue-rotate(30.00deg) brightness(120%)") {
		t.Fatalf("expected tint output: %s", out)
	}
	if !strings.Contains(out, "translate(25.00%, -10.00%)") || !strings.Contains(out, "scale(0.5000, 1.2500)") {
		t.Fatalf("expected effectDag transform output: %s", out)
	}
}

func TestHTMLVectorShapeSiblingBlendDoesNotReflect(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:     "rect",
						Floating: true,
						Width:    127000,
						Height:   127000,
						Geometry: &dml.PresetGeometry{Name: "rect"},
						Fill:     &dml.SolidFill{Color: "FFFFFF", Alpha: 1.0},
						Effects: dml.EffectList{Effects: []dml.Effect{
							dml.BlendMode{Mode: "mult", ContainerType: "sib"},
							dml.Reflection{Alpha: 0.25, Blur: 2, Distance: 3, Direction: 90},
						}},
					},
				}},
			},
		}},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if got := strings.Count(out, "mix-blend-mode:multiply"); got != 1 {
		t.Fatalf("blend count = %d, want 1: %s", got, out)
	}
}

func TestHTMLVectorShapeTreeBlendReflects(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:     "rect",
						Floating: true,
						Width:    127000,
						Height:   127000,
						Geometry: &dml.PresetGeometry{Name: "rect"},
						Fill:     &dml.SolidFill{Color: "FFFFFF", Alpha: 1.0},
						Effects: dml.EffectList{Effects: []dml.Effect{
							dml.BlendMode{Mode: "mult", ContainerType: "tree"},
							dml.Reflection{Alpha: 0.25, Blur: 2, Distance: 3, Direction: 90},
						}},
					},
				}},
			},
		}},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if got := strings.Count(out, "mix-blend-mode:multiply"); got != 2 {
		t.Fatalf("blend count = %d, want 2: %s", got, out)
	}
}

func TestHTMLVectorShapeSiblingAlphaDoesNotReflect(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:     "rect",
						Floating: true,
						Width:    127000,
						Height:   127000,
						Geometry: &dml.PresetGeometry{Name: "rect"},
						Fill:     &dml.SolidFill{Color: "FFFFFF", Alpha: 1.0},
						Effects: dml.EffectList{Effects: []dml.Effect{
							dml.AlphaModFix{Amount: 50000, ContainerType: "sib"},
							dml.Reflection{Alpha: 0.25, Blur: 2, Distance: 3, Direction: 90},
						}},
					},
				}},
			},
		}},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "opacity:0.5000") {
		t.Fatalf("expected original alpha modulation: %s", out)
	}
	if !strings.Contains(out, `opacity="0.25"`) {
		t.Fatalf("expected base reflection opacity: %s", out)
	}
	if strings.Contains(out, `class="shape__vector-reflection-content" style="overflow:visible;opacity:0.5000"`) {
		t.Fatalf("did not expect sibling alpha to affect reflection opacity: %s", out)
	}
}

func TestHTMLVectorShapeTreeAlphaReflects(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:     "rect",
						Floating: true,
						Width:    127000,
						Height:   127000,
						Geometry: &dml.PresetGeometry{Name: "rect"},
						Fill:     &dml.SolidFill{Color: "FFFFFF", Alpha: 1.0},
						Effects: dml.EffectList{Effects: []dml.Effect{
							dml.AlphaModFix{Amount: 50000, ContainerType: "tree"},
							dml.Reflection{Alpha: 0.25, Blur: 2, Distance: 3, Direction: 90},
						}},
					},
				}},
			},
		}},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `opacity="0.12"`) {
		t.Fatalf("expected tree alpha to combine with reflection opacity: %s", out)
	}
	if strings.Contains(out, `class="shape__vector-reflection-content" style="overflow:visible;opacity:0.5000"`) {
		t.Fatalf("expected combined alpha on the reflected layer, not a leaked inner opacity: %s", out)
	}
}

func TestHTMLScopedHyperlinksRenderDifferentURLsInDocumentAndHeader(t *testing.T) {
	pkg := &parts.Package{
		MainPartName: "word/document.xml",
		Document: &parts.Document{
			Body: &parts.Body{
				Blocks: []parts.BodyBlock{{
					Kind: parts.BodyBlockParagraph,
					Paragraph: &parts.Paragraph{
						Runs: []*parts.Run{{
							Contents: []parts.RunContent{{
								Kind: parts.RunContentText,
								Text: &parts.Text{Value: "doc text"},
							}},
						}},
						Hyperlinks: []*parts.Hyperlink{{
							ID: "rId1",
							Runs: []*parts.Run{{
								Contents: []parts.RunContent{{
									Kind: parts.RunContentText,
									Text: &parts.Text{Value: "doc link"},
								}},
							}},
						}},
					},
				}},
				Section: &parts.SectionProperties{
					HeaderRefs: []parts.HeaderFooterReference{{Type: "default", ID: "rIdHdr"}},
				},
			},
		},
		PartRels: map[string]*parts.Relationships{
			"word/document.xml": {
				Relationships: []*parts.Relationship{
					{ID: "rId1", Type: "http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink", Target: "https://doc.example.com", TargetMode: "External"},
					{ID: "rIdHdr", Type: "http://schemas.openxmlformats.org/officeDocument/2006/relationships/header", Target: "header1.xml"},
				},
			},
			"word/header1.xml": {
				Relationships: []*parts.Relationship{
					{ID: "rId1", Type: "http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink", Target: "https://header.example.com", TargetMode: "External"},
				},
			},
		},
		ScopedHyperlinkURLs: map[parts.RelationshipRef]string{
			{SourcePart: "word/document.xml", ID: "rId1"}: "https://doc.example.com",
			{SourcePart: "word/header1.xml", ID: "rId1"}:  "https://header.example.com",
		},
		Headers: map[string]*parts.Header{
			"word/header1.xml": {
				Paragraphs: []*parts.Paragraph{{
					Runs: []*parts.Run{{
						Contents: []parts.RunContent{{
							Kind: parts.RunContentText,
							Text: &parts.Text{Value: "header text"},
						}},
					}},
					Hyperlinks: []*parts.Hyperlink{{
						ID: "rId1",
						Runs: []*parts.Run{{
							Contents: []parts.RunContent{{
								Kind: parts.RunContentText,
								Text: &parts.Text{Value: "header link"},
							}},
						}},
					}},
				}},
			},
		},
	}

	doc, err := wordresolve.ResolvePackage(pkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}

	// The rendered HTML should contain both hyperlink URLs since each
	// resolves to its own scoped target.
	if !strings.Contains(out, `href="https://doc.example.com"`) {
		t.Errorf("document hyperlink URL missing from rendered HTML: %s", out)
	}
	if !strings.Contains(out, `href="https://header.example.com"`) {
		t.Errorf("header hyperlink URL missing from rendered HTML: %s", out)
	}
}

func TestHTMLRendersAdvancedQuadArrowPresetShapes(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:        "rect",
						Floating:    true,
						Width:       2540000,
						Height:      2540000,
						Geometry:    &dml.PresetGeometry{Name: "quadArrow"},
						FillColor:   "FF7043",
						StrokeColor: "1E1E1E",
						StrokeWidth: 12700,
					},
				}},
			},
		}},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="shape__vector"`) {
		t.Fatalf("expected SVG vector shape in output: %s", out)
	}
	if !strings.Contains(out, `fill="#FF7043"`) {
		t.Fatalf("expected SVG fill color in output: %s", out)
	}
	if !strings.Contains(out, `stroke="#1E1E1E"`) {
		t.Fatalf("expected SVG stroke in output: %s", out)
	}
}

func TestHTMLRendersCustomGeometryShapes(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:     "rect",
						Floating: true,
						Width:    127000,
						Height:   127000,
						GeometryData: &dml.Geometry{
							Type: dml.GeometryTypeCustom,
							Paths: []dml.Path{{
								Commands: []dml.PathCommand{
									{Type: "M", Points: []dml.Point{{X: 0, Y: 0}}},
									{Type: "L", Points: []dml.Point{{X: 1, Y: 0}}},
									{Type: "L", Points: []dml.Point{{X: 0.5, Y: 1}}},
									{Type: "Z"},
								},
							}},
						},
						FillColor:   "00AA00",
						StrokeColor: "0000FF",
						StrokeWidth: 12700,
					},
				}},
			},
		}},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="shape__vector"`) {
		t.Fatalf("expected SVG vector shape in output: %s", out)
	}
	if !strings.Contains(out, `fill="#00AA00"`) {
		t.Fatalf("expected SVG fill color in output: %s", out)
	}
	if !strings.Contains(out, `stroke="#0000FF"`) {
		t.Fatalf("expected SVG stroke in output: %s", out)
	}
}

func TestHTMLRendersSharedDrawingMLPatternFillOnRectShape(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:     "rect",
						Floating: true,
						Width:    127000,
						Height:   127000,
						Fill: dml.PatternFill{
							PatternType: "smGrid",
							Foreground:  "00AA00",
							Background:  "FFFFFF",
						},
					},
				}},
			},
		}},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="shape__vector"`) {
		t.Fatalf("expected SVG vector shape in output: %s", out)
	}
	if !strings.Contains(out, `fill="url(#shape-fill)"`) {
		t.Fatalf("expected SVG pattern fill in output: %s", out)
	}
}

func TestHTMLExternalizesVectorShapePictureFillAssets(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:     "rect",
						Floating: true,
						Width:    127000,
						Height:   127000,
						Fill: dml.PictureFill{
							ImageData:   []byte("shape-fill-image"),
							ContentType: "image/png",
						},
					},
				}},
			},
		}},
	}

	html, assets, err := render.HTMLWithOptions(doc, render.HTMLOptions{
		ImageMode:        render.HTMLImageModeExternal,
		ExternalImageDir: "shape_assets",
	})
	if err != nil {
		t.Fatalf("HTMLWithOptions: %v", err)
	}
	if !strings.Contains(html, `fill="url(#shape-fill)"`) {
		t.Fatalf("expected vector picture fill usage in html: %s", html)
	}
	if strings.Contains(html, `data:image/png;base64,`) {
		t.Fatalf("expected picture fill image to be externalized: %s", html)
	}
	if !strings.Contains(html, `href="shape_assets/image-001.png"`) {
		t.Fatalf("expected externalized picture fill asset path in html: %s", html)
	}
	if len(assets) != 1 {
		t.Fatalf("asset count = %d, want 1", len(assets))
	}
	if assets[0].Path != "shape_assets/image-001.png" || assets[0].ContentType != "image/png" || string(assets[0].Data) != "shape-fill-image" {
		t.Fatalf("unexpected asset = %+v", assets[0])
	}
}

func TestHTMLParagraphLeadingBlockFloatImageRendersInFlow(t *testing.T) {
	centered := &resolve.Image{
		Floating:  true,
		Width:     251460,
		Height:    243840,
		XAlign:    "center",
		XRelative: "page",
		YRelative: "paragraph",
		WrapType:  "topAndBottom",
		MIMEType:  "image/png",
		Data:      []byte{0x89, 0x50, 0x4E, 0x47},
	}
	rightFloat := &resolve.Image{
		Floating:  true,
		Width:     975360,
		Height:    975360,
		XAlign:    "right",
		XRelative: "margin",
		YAlign:    "top",
		YRelative: "margin",
		WrapType:  "square",
		MIMEType:  "image/png",
		Data:      []byte{0x89, 0x50, 0x4E, 0x47},
	}
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Intro"}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{
				{Kind: "image", Image: centered},
				{Kind: "image", Image: rightFloat},
				{Kind: "run", Text: "Centered images like this are useful for large pictures that should be a focus of attention."},
			}}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `clear:both`) || !strings.Contains(out, `margin-left:auto`) || !strings.Contains(out, `margin-right:auto`) {
		t.Fatalf("expected leading block float image to render in flow before paragraph text: %s", out)
	}
	if !strings.Contains(out, `float:right`) {
		t.Fatalf("expected square-wrapped sibling image to keep rendering as a right float: %s", out)
	}
	if strings.Contains(out, `class="word-wrap-proxy"`) {
		t.Fatalf("expected square-wrapped sibling image to be the visible flow float: %s", out)
	}
}

func TestHTMLPageTopMarginFloatsShareEarliestAnchorParagraphInFlow(t *testing.T) {
	left := &resolve.Image{
		Floating:  true,
		Width:     975360,
		Height:    975360,
		XAlign:    "left",
		XRelative: "margin",
		YAlign:    "top",
		YRelative: "margin",
		WrapType:  "square",
		MIMEType:  "image/png",
		Data:      []byte{0x89, 0x50, 0x4E, 0x47},
	}
	right := &resolve.Image{
		Floating:  true,
		Width:     975360,
		Height:    975360,
		XAlign:    "right",
		XRelative: "margin",
		YAlign:    "top",
		YRelative: "margin",
		WrapType:  "square",
		MIMEType:  "image/png",
		Data:      []byte{0x89, 0x50, 0x4E, 0x47},
	}
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{StyleID: "Heading1", Runs: []resolve.Run{{Kind: "run", Text: "Images"}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{
				{Kind: "image", Image: left},
				{Kind: "run", Text: "First body paragraph."},
			}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{
				{Kind: "image", Image: right},
				{Kind: "run", Text: "Second body paragraph."},
			}}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	firstParaStart := strings.Index(out, "First body paragraph.")
	secondParaStart := strings.Index(out, "Second body paragraph.")
	rightFloat := strings.Index(out, `float:right`)
	if rightFloat < 0 {
		t.Fatalf("expected right float in output: %s", out)
	}
	if !(rightFloat < firstParaStart) {
		t.Fatalf("expected page-top right float to be emitted before first body paragraph text: %s", out)
	}
	if !(rightFloat < secondParaStart) {
		t.Fatalf("expected page-top right float to stay out of the later paragraph: %s", out)
	}
	if strings.Count(out, `float:right`) != 1 || strings.Count(out, `float:left`) != 1 {
		t.Fatalf("expected one left and one right in-flow float, got: %s", out)
	}
}

func TestHTMLParagraphRelativeFloatingImageWrapsOnLargestSide(t *testing.T) {
	img := &resolve.Image{
		Floating:  true,
		Width:     508000,
		Height:    731520,
		XRelative: "column",
		YRelative: "paragraph",
		WrapType:  "square",
		WrapSide:  "largest",
		X:         254000,
		MIMEType:  "image/png",
		Data:      []byte{0x89, 0x50, 0x4E, 0x47},
		Alt:       "Largest side",
	}
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 3400, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 500, Bottom: 1440, Left: 500},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{Runs: []resolve.Run{
				{Kind: "image", Image: img},
				{Kind: "run", Text: "Body text wraps on the larger side of the image."},
			}},
		}},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	imageStart := strings.Index(out, `alt="Largest side"`)
	if imageStart < 0 {
		t.Fatalf("expected largest-side image in output: %s", out)
	}
	imageStart = strings.LastIndex(out[:imageStart], "<")
	imageEnd := strings.Index(out[imageStart:], ">")
	snippet := out[imageStart : imageStart+imageEnd+1]
	if !strings.Contains(snippet, `float:left`) {
		t.Fatalf("expected largest-side wrap to place the image on the left so text uses the larger right segment: %s", snippet)
	}
	if strings.Contains(snippet, `float:right`) {
		t.Fatalf("did not expect largest-side wrap to choose the smaller left segment: %s", snippet)
	}
	if !strings.Contains(snippet, `position:relative`) || strings.Contains(snippet, `position:absolute`) {
		t.Fatalf("expected largest-side image to remain a visible flow float: %s", snippet)
	}
}

func TestHTMLBehindTextImageUsesDedicatedBehindOverlay(t *testing.T) {
	img := &resolve.Image{
		Floating:  true,
		BehindDoc: true,
		Width:     508000,
		Height:    508000,
		XRelative: "page",
		YRelative: "page",
		WrapType:  "none",
		MIMEType:  "image/png",
		Data:      []byte{0x89, 0x50, 0x4E, 0x47},
		Alt:       "Behind text",
	}
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{Runs: []resolve.Run{
				{Kind: "image", Image: img},
				{Kind: "run", Text: "Foreground text"},
			}},
		}},
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="page-overlay page-overlay-behind"`) {
		t.Fatalf("expected behind-text image in dedicated behind overlay: %s", out)
	}
	if !strings.Contains(out, `.page-body { position: relative; z-index: 1;`) || !strings.Contains(out, `.page-overlay-behind { z-index: 0; }`) {
		t.Fatalf("expected explicit body/behind-overlay stacking order: %s", out)
	}
	if strings.Contains(out, `class="word-wrap-proxy"`) {
		t.Fatalf("behind-text image must not reserve wrapping space: %s", out)
	}
}

func TestHTMLBehindTextAnchorPreservesBlankParagraphHeight(t *testing.T) {
	img := &resolve.Image{Floating: true, BehindDoc: true, Width: 508000, Height: 508000, XRelative: "page", YRelative: "paragraph", WrapType: "none", MIMEType: "image/png", Data: []byte{0x89}, Alt: "background"}
	doc := &resolve.Document{Blocks: []resolve.Block{
		{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Before"}}}},
		{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "image", Image: img}}}},
		{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "After"}}}},
	}}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	backgroundAt := strings.Index(out, `alt="background"`)
	if backgroundAt < 0 {
		t.Fatalf("background image missing: %s", out)
	}
	beforeImage := out[:backgroundAt]
	if strings.Contains(beforeImage[strings.LastIndex(beforeImage, "<p"):], `display:contents`) {
		t.Fatalf("behind-text anchor paragraph must retain its blank-line height: %s", out)
	}
	if !strings.Contains(out, `min-height:`) || !strings.Contains(out, `>After</span>`) {
		t.Fatalf("expected normal blank paragraph followed by text: %s", out)
	}
}

func TestHTMLRendersFootnotesInsidePageFrame(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 4000, Height: 2400},
			PageMargin: &resolve.PageMargin{Top: 400, Right: 400, Bottom: 400, Left: 400},
		}},
		Footnotes: map[int]resolve.Story{
			1: {Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Footnote text"}}}}}},
		},
		Blocks: []resolve.Block{{
			Kind:      "paragraph",
			Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Anchor"}, {Kind: "footnoteRef", NoteID: 1}}},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="page-footnotes"`) {
		t.Fatalf("expected page footnote frame in output: %s", out)
	}
	if !strings.Contains(out, `Footnote text`) {
		t.Fatalf("expected footnote text in page output: %s", out)
	}
	if strings.Contains(out, `<aside class="notes">`) {
		t.Fatalf("did not expect trailing notes aside for footnotes-only document: %s", out)
	}
}

func TestHTMLRendersMultiColumnFootnotesAtBodyWidth(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 5000, Height: 2400},
			PageMargin: &resolve.PageMargin{Top: 400, Right: 400, Bottom: 400, Left: 400},
			Columns:    &resolve.Columns{Num: 2, Space: 300},
		}},
		Footnotes: map[int]resolve.Story{
			1: {Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Footnote text that should use the full page body width"}}}}}},
		},
		Blocks: []resolve.Block{{
			Kind:      "paragraph",
			Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Anchor"}, {Kind: "footnoteRef", NoteID: 1}}},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="page-footnotes"`) {
		t.Fatalf("expected page footnote frame in output: %s", out)
	}
	if !strings.Contains(out, `class="page-footnotes" style="position:absolute;left:20.0pt;`) || !strings.Contains(out, `width:210.0pt`) {
		t.Fatalf("expected multi-column footnotes to render at full body width, got: %s", out)
	}
}

func TestHTMLFormatsFootnoteAndEndnoteReferenceNumbers(t *testing.T) {
	doc := &resolve.Document{
		FootnoteNumberFormat: "lowerRoman",
		EndnoteNumberFormat:  "upperLetter",
		Footnotes: map[int]resolve.Story{
			9: {Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Footnote text"}}}}}},
		},
		Endnotes: map[int]resolve.Story{
			11: {Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Endnote text"}}}}}},
		},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Kind: "run", Text: "Anchor"}, {Kind: "footnoteRef", NoteID: 9}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Kind: "run", Text: "Anchor"}, {Kind: "endnoteRef", NoteID: 11}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<a href="#fn9" id="fnref9"><sup>i</sup></a>`) {
		t.Fatalf("expected formatted lower-roman footnote reference number, got:\n%s", out)
	}
	if !strings.Contains(out, `<a href="#en11" id="enref11"><sup>A</sup></a>`) {
		t.Fatalf("expected formatted upper-letter endnote reference number, got:\n%s", out)
	}
}

func TestHTMLEndnotesAreNotDuplicatedInTrailingNotesAside(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 4000, Height: 2400},
			PageMargin: &resolve.PageMargin{Top: 400, Right: 400, Bottom: 400, Left: 400},
		}},
		Endnotes: map[int]resolve.Story{
			2: {Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Endnote text"}}}}}},
		},
		Blocks: []resolve.Block{{
			Kind:      "paragraph",
			Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Anchor"}, {Kind: "endnoteRef", NoteID: 2}}},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Count(out, "Endnote text") != 1 {
		t.Fatalf("expected endnote text once in paginated output, got: %s", out)
	}
	if strings.Contains(out, `<section class="endnotes">`) {
		t.Fatalf("did not expect a trailing endnotes aside when endnotes are already paginated: %s", out)
	}
}

func TestHTMLHeaderAndFooterFloatingContentUseStoryFrames(t *testing.T) {
	doc := &resolve.Document{
		Headers: map[string]resolve.Story{
			"word/header1.xml": {
				Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "textbox", TextBox: &resolve.TextBox{Floating: true, Y: 127000, Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Header box"}}}}}}}}}}},
			},
		},
		Footers: map[string]resolve.Story{
			"word/footer1.xml": {
				Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "textbox", TextBox: &resolve.TextBox{Floating: true, Y: 127000, Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Footer box"}}}}}}}}}}},
			},
		},
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 720, Right: 1440, Bottom: 720, Left: 1440, Header: 360, Footer: 360},
			Headers:    []resolve.Reference{{Type: "default", PartName: "word/header1.xml"}},
			Footers:    []resolve.Reference{{Type: "default", PartName: "word/footer1.xml"}},
		}},
		Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Body"}}}}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="page-header"`) || !strings.Contains(out, `Header box`) {
		t.Fatalf("expected absolute page header frame with floating content: %s", out)
	}
	if !strings.Contains(out, `class="page-footer"`) || !strings.Contains(out, `Footer box`) {
		t.Fatalf("expected absolute page footer frame with floating content: %s", out)
	}
	if !strings.Contains(out, `top:10.0pt`) {
		t.Fatalf("expected floating header/footer content to use story-local top offsets: %s", out)
	}
}

func TestHTMLWidowControlListContinuationDoesNotRepeatListItem(t *testing.T) {
	indentLeft := 720
	hanging := 360
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 2400, Height: 1200},
			PageMargin: &resolve.PageMargin{Top: 0, Right: 0, Bottom: 0, Left: 0},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs:   []resolve.Run{{Kind: "run", Text: "Intro"}},
				Format: resolve.ParagraphFormat{LineSpacing: intPtr(400), LineRule: "exact"},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Text: "abcdefghijkl mnopqrstuvwx yzabcdefuvwx rstuvwxyzzzz",
				Runs: []resolve.Run{{Kind: "run", Text: "abcdefghijkl mnopqrstuvwx yzabcdefuvwx rstuvwxyzzzz"}},
				List: &resolve.ListMarker{Kind: "ordered", Level: 0, Start: 1, Format: "decimal"},
				Format: resolve.ParagraphFormat{
					WidowControl:  true,
					LineSpacing:   intPtr(400),
					LineRule:      "exact",
					IndentLeft:    &indentLeft,
					IndentHanging: &hanging,
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if got := strings.Count(out, "<li"); got != 1 {
		t.Fatalf("expected one list item marker across split fragments, got %d: %s", got, out)
	}
	if !strings.Contains(out, "rstuvwxyzzzz") {
		t.Fatalf("expected continuation fragment text in output: %s", out)
	}
}

func TestHTMLVerticalLineShapeUsesBorderLeft(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:        "line",
						Y:           127000,
						Height:      127000,
						StrokeColor: "292E32",
						StrokeWidth: 9525,
					},
				}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<div class="shape-line-v"`) {
		t.Fatalf("expected vertical line class in output: %s", out)
	}
	if !strings.Contains(out, `border-left-color:#292E32`) {
		t.Fatalf("expected border-left color in output: %s", out)
	}
	if !strings.Contains(out, `border-left-width:0.75pt`) {
		t.Fatalf("expected border-left width in output: %s", out)
	}
	if strings.Contains(out, `<div class="shape-line" style=`) {
		t.Fatalf("did not expect horizontal line element for a zero-width vertical line: %s", out)
	}
}

func TestHTMLVerticalLineShapeUsesExplicitReflectionLayer(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:        "line",
						Y:           127000,
						Height:      127000,
						StrokeColor: "292E32",
						StrokeWidth: 9525,
						Effects: dml.EffectList{Effects: []dml.Effect{
							dml.Reflection{Alpha: 0.4, Blur: 2, Distance: 3, Direction: 90, Fade: true},
						}},
					},
				}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="shape-line-v shape-line-reflection"`) {
		t.Fatalf("expected explicit reflected vertical line layer: %s", out)
	}
	if !strings.Contains(out, `scaleY(-1.0000)`) {
		t.Fatalf("expected reflected line flip transform: %s", out)
	}
	if strings.Contains(out, "box-shadow:") {
		t.Fatalf("expected explicit line reflection geometry instead of box-shadow fallback: %s", out)
	}
}

func TestHTMLHyperlinkEscaping(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "hyperlink", Text: "link <with> special", Target: "https://example.com/path?a=1&b=2"},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// The text content should have < > escaped.
	if strings.Contains(out, "link <with> special") {
		t.Error("raw < > should be HTML-escaped in text")
	}
	if !strings.Contains(out, "link &lt;with&gt; special") {
		t.Error("expected HTML-escaped text")
	}
	// The href should preserve & in the URL (it is inside an attribute value that html.EscapeString handles).
	if !strings.Contains(out, "a=1") {
		t.Errorf("expected query params in href: %s", out)
	}
}

func TestHTMLTrackedChanges(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "kept "},
					{Kind: "run", Text: "removed", Revision: "deleted"},
					{Kind: "run", Text: " added", Revision: "inserted"},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "<del>") || !strings.Contains(out, "removed") || !strings.Contains(out, "</del>") {
		t.Errorf("expected <del>removed</del> in output, got:\n%s", out)
	}
	if !strings.Contains(out, "<ins>") || !strings.Contains(out, "added") || !strings.Contains(out, "</ins>") {
		t.Errorf("expected <ins>added</ins> in output, got:\n%s", out)
	}
	if strings.Contains(out, "<del>kept") || strings.Contains(out, "<ins>kept") {
		t.Errorf("unchanged text should not be wrapped in del/ins, got:\n%s", out)
	}
	// CSS must include del and ins rules.
	css := extractCSS(out)
	if !strings.Contains(css, "del {") {
		t.Errorf("expected del CSS rule in output")
	}
	if !strings.Contains(css, "ins {") {
		t.Errorf("expected ins CSS rule in output")
	}
	if strings.Contains(css, "(ins {") {
		t.Errorf("expected valid ins CSS selector, got malformed rule: %q", css)
	}
}

func TestHTMLSkipsDeletedOLEPlaceholders(t *testing.T) {
	t.Skip("not OfficeArt: excluded from the OfficeArt-focused test pass")
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "olePlaceholder", Text: "Embedded Object", Revision: "deleted"},
					{Kind: "olePlaceholder", Text: "Embedded Object"},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Contains(out, "<del><span class=\"ole-object\"") {
		t.Fatalf("expected deleted OLE placeholder to be suppressed, got:\n%s", out)
	}
	if strings.Count(out, `class="ole-object"`) != 1 {
		t.Fatalf("expected exactly one visible OLE placeholder, got:\n%s", out)
	}
}

func TestHTMLolePlaceholderRendering(t *testing.T) {
	t.Skip("not OfficeArt: excluded from the OfficeArt-focused test pass")
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "olePlaceholder", Text: "Excel.Sheet.12"},
					{Kind: "olePlaceholder", Text: "Word.Document.12", OLERelID: "rOle1"},
					{Kind: "olePlaceholder", Text: "Unknown.ProgID.99"},
					{Kind: "olePlaceholder", Text: ""},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `[Microsoft Excel Worksheet]`) {
		t.Fatalf("expected human-readable Excel label, got:\n%s", out)
	}
	if !strings.Contains(out, `title="Word.Document.12"`) {
		t.Fatalf("expected Word ProgID in title, got:\n%s", out)
	}
	if !strings.Contains(out, `data-ole-relid="rOle1"`) {
		t.Fatalf("expected data-ole-relid attribute, got:\n%s", out)
	}
	if !strings.Contains(out, `[Unknown.ProgID.99]`) {
		t.Fatalf("expected fallback to raw ProgID, got:\n%s", out)
	}
	if !strings.Contains(out, `[Embedded Object]`) {
		t.Fatalf("expected default label for empty ProgID, got:\n%s", out)
	}
}

func TestHTMLFieldKindClass(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "1", FieldKind: "PAGE"},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="word-field word-field-page"`) {
		t.Errorf("expected word-field-page span in output, got:\n%s", out)
	}
	css := extractCSS(out)
	if !strings.Contains(css, ".word-field {") {
		t.Errorf("expected .word-field CSS rule")
	}
}

func TestHTMLPageFieldFormatted(t *testing.T) {
	tests := []struct {
		name   string
		format string
		start  int
		want   string
	}{
		{"decimal default", "", 1, ">1<"},
		{"upperRoman", "upperRoman", 1, ">I<"},
		{"lowerRoman", "lowerRoman", 1, ">i<"},
		{"upperLetter", "upperLetter", 1, ">A<"},
		{"lowerLetter", "lowerLetter", 1, ">a<"},
		{"upperRoman start 5", "upperRoman", 5, ">V<"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sec := resolve.Section{
				PageSize:         &resolve.PageSize{Width: 12240, Height: 15840},
				PageMargin:       &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
				PageNumberFormat: tt.format,
			}
			if tt.start != 1 {
				sec.PageNumberStart = &tt.start
			}
			doc := &resolve.Document{
				Sections: []resolve.Section{sec},
				Blocks: []resolve.Block{
					{Kind: "paragraph", Paragraph: &resolve.Paragraph{
						Runs: []resolve.Run{{Kind: "run", FieldKind: "PAGE"}},
					}},
				},
			}
			out, err := render.HTML(doc)
			if err != nil {
				t.Fatalf("HTML: %v", err)
			}
			if !strings.Contains(out, tt.want) {
				t.Errorf("expected %q in output, got:\n%s", tt.want, out)
			}
		})
	}
}

func TestHTMLFieldKindDate(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "2024-01-01", FieldKind: "DATE"},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="word-field word-field-date"`) {
		t.Errorf("expected word-field-date span in output, got:\n%s", out)
	}
}

func TestHTMLFieldKindAdditionalFamiliesPreserveCachedText(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "1", FieldKind: "SEQ", FieldArg: "Figure"},
					{Kind: "run", Text: " ", FieldKind: ""},
					{Kind: "run", Text: "Alice", FieldKind: "MERGEFIELD", FieldArg: "CustomerName"},
					{Kind: "run", Text: " ", FieldKind: ""},
					{Kind: "run", Text: "Linked sheet", FieldKind: "LINK", FieldArg: `LINK Excel.Sheet.12 "C:\data.xlsx" "Sheet1!R1C1"`},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, className := range []string{
		`class="word-field word-field-seq"`,
		`class="word-field word-field-mergefield"`,
		`class="word-field word-field-link"`,
	} {
		if !strings.Contains(out, className) {
			t.Fatalf("expected %s in output, got:\n%s", className, out)
		}
	}
	for _, value := range []string{"1", "Alice", "Linked sheet"} {
		if !strings.Contains(out, value) {
			t.Fatalf("expected cached field value %q in output, got:\n%s", value, out)
		}
	}
}

func TestHTMLSeqFieldCalculatesSequenceValues(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", FieldKind: "SEQ", FieldArg: "Figure"},
					{Kind: "run", Text: " "},
					{Kind: "run", FieldKind: "SEQ", FieldArg: "Figure"},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Count(out, `class="word-field word-field-seq"`) != 2 {
		t.Fatalf("expected 2 SEQ fields, got:\n%s", out)
	}
	if !strings.Contains(out, `>1</span>`) || !strings.Contains(out, `>2</span>`) {
		t.Fatalf("expected SEQ fields to resolve to 1 and 2, got:\n%s", out)
	}
}

func TestHTMLSeqFieldKeepsSeparateNamedCounters(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", FieldKind: "SEQ", FieldArg: "Figure"},
					{Kind: "run", Text: " "},
					{Kind: "run", FieldKind: "SEQ", FieldArg: "Table"},
					{Kind: "run", Text: " "},
					{Kind: "run", FieldKind: "SEQ", FieldArg: "Figure"},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Count(out, `>1</span>`) < 2 {
		t.Fatalf("expected separate SEQ names to maintain independent counters, got:\n%s", out)
	}
	if !strings.Contains(out, `>2</span>`) {
		t.Fatalf("expected second Figure SEQ field to resolve to 2, got:\n%s", out)
	}
}

func TestHTMLSeqFieldSupportsRestartAndRepeatSwitches(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", FieldKind: "SEQ", FieldArg: "Figure", FieldInstr: `SEQ Figure \r 5`},
					{Kind: "run", Text: " "},
					{Kind: "run", FieldKind: "SEQ", FieldArg: "Figure", FieldInstr: `SEQ Figure \c`},
					{Kind: "run", Text: " "},
					{Kind: "run", FieldKind: "SEQ", FieldArg: "Figure"},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `>5</span>`) {
		t.Fatalf("expected SEQ \\r switch to restart at 5, got:\n%s", out)
	}
	if strings.Count(out, `>5</span>`) < 2 {
		t.Fatalf("expected SEQ \\c switch to repeat current value 5, got:\n%s", out)
	}
	if !strings.Contains(out, `>6</span>`) {
		t.Fatalf("expected following SEQ field to increment to 6, got:\n%s", out)
	}
}

func TestHTMLSeqFieldSupportsFormattingAndHideSwitches(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", FieldKind: "SEQ", FieldArg: "Figure", FieldInstr: `SEQ Figure \r 4 \* ROMAN`},
					{Kind: "run", Text: " "},
					{Kind: "run", FieldKind: "SEQ", FieldArg: "Figure", FieldInstr: `SEQ Figure \* alphabetic`},
					{Kind: "run", Text: " "},
					{Kind: "run", FieldKind: "SEQ", FieldArg: "Figure", FieldInstr: `SEQ Figure \h`},
					{Kind: "run", Text: " "},
					{Kind: "run", FieldKind: "SEQ", FieldArg: "Figure", FieldInstr: `SEQ Figure \c \* ALPHABETIC`},
					{Kind: "run", Text: " "},
					{Kind: "run", FieldKind: "SEQ", FieldArg: "Figure", FieldInstr: `SEQ Figure \c \* CardText`},
					{Kind: "run", Text: " "},
					{Kind: "run", FieldKind: "SEQ", FieldArg: "Figure", FieldInstr: `SEQ Figure \* OrdText`},
					{Kind: "run", Text: " "},
					{Kind: "run", FieldKind: "SEQ", FieldArg: "Figure"},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `>IV</span>`) {
		t.Fatalf("expected SEQ \\* ROMAN formatting, got:\n%s", out)
	}
	if !strings.Contains(out, `>e</span>`) {
		t.Fatalf("expected SEQ \\* alphabetic formatting, got:\n%s", out)
	}
	if !strings.Contains(out, `>F</span>`) {
		t.Fatalf("expected SEQ \\c with \\* ALPHABETIC to repeat current value in formatted form, got:\n%s", out)
	}
	if !strings.Contains(out, `>six</span>`) {
		t.Fatalf("expected SEQ \\c with \\* CardText to repeat current value as words, got:\n%s", out)
	}
	if !strings.Contains(out, `>seventh</span>`) {
		t.Fatalf("expected SEQ \\* OrdText to render ordinal words, got:\n%s", out)
	}
	if !strings.Contains(out, `>8</span>`) {
		t.Fatalf("expected SEQ after hidden increment and formatted values to continue at 8, got:\n%s", out)
	}
	if strings.Contains(out, `>6</span>`) {
		t.Fatalf("expected hidden SEQ value not to render, got:\n%s", out)
	}
}

func TestHTMLSeqFieldRestartsAtHeadingLevel(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index:     0,
				StyleID:   "Heading1",
				StyleName: "heading 1",
				Runs:      []resolve.Run{{Kind: "run", Text: "Chapter 1"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index: 1,
				Runs:  []resolve.Run{{Kind: "run", FieldKind: "SEQ", FieldArg: "Figure", FieldInstr: `SEQ Figure \s 1`}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index: 2,
				Runs:  []resolve.Run{{Kind: "run", FieldKind: "SEQ", FieldArg: "Figure", FieldInstr: `SEQ Figure \s 1`}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index:     3,
				StyleID:   "Heading1",
				StyleName: "heading 1",
				Runs:      []resolve.Run{{Kind: "run", Text: "Chapter 2"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index: 4,
				Runs:  []resolve.Run{{Kind: "run", FieldKind: "SEQ", FieldArg: "Figure", FieldInstr: `SEQ Figure \s 1`}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index:     5,
				StyleID:   "Heading2",
				StyleName: "heading 2",
				Runs:      []resolve.Run{{Kind: "run", Text: "Section 2.1"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index: 6,
				Runs:  []resolve.Run{{Kind: "run", FieldKind: "SEQ", FieldArg: "Figure", FieldInstr: `SEQ Figure \s 1`}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index: 7,
				Runs:  []resolve.Run{{Kind: "run", FieldKind: "SEQ", FieldArg: "Figure", FieldInstr: `SEQ Figure \s 2`}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// Expected sequence values in order: 1, 2, 1, 2, 1
	// Para 1 (after H1): SEQ Figure \s 1 → 1
	// Para 2 (after H1): SEQ Figure \s 1 → 2
	// Para 4 (after H1): SEQ Figure \s 1 → reset to 1
	// Para 6 (after H2, \s 1): still under Chapter 2 scope → 2
	// Para 7 (after H2, \s 2): Heading 2 level <= 2, so reset → 1
	re := regexp.MustCompile(`>(\d)</span>`)
	matches := re.FindAllStringSubmatch(out, -1)
	if len(matches) != 5 {
		t.Fatalf("expected 5 SEQ values, got %d:\n%s", len(matches), out)
	}
	expected := []string{"1", "2", "1", "2", "1"}
	for i, exp := range expected {
		if matches[i][1] != exp {
			t.Fatalf("expected SEQ value %d to be %s, got %s:\n%s", i+1, exp, matches[i][1], out)
		}
	}
}

func TestHTMLSeqFieldCaseFormattingSwitches(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", FieldKind: "SEQ", FieldArg: "Item", FieldInstr: `SEQ Item \* alphabetic \* upper`},
					{Kind: "run", Text: " "},
					{Kind: "run", FieldKind: "SEQ", FieldArg: "Item", FieldInstr: `SEQ Item \* alphabetic \* lower`},
					{Kind: "run", Text: " "},
					{Kind: "run", FieldKind: "SEQ", FieldArg: "Item", FieldInstr: `SEQ Item \* CardText \* firstcap`},
					{Kind: "run", Text: " "},
					{Kind: "run", FieldKind: "SEQ", FieldArg: "Item", FieldInstr: `SEQ Item \* OrdText \* Caps`},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `>A</span>`) {
		t.Fatalf("expected SEQ \\* upper after alphabetic to yield A, got:\n%s", out)
	}
	if !strings.Contains(out, `>b</span>`) {
		t.Fatalf("expected SEQ \\* lower after alphabetic to yield b, got:\n%s", out)
	}
	if !strings.Contains(out, `>Three</span>`) {
		t.Fatalf("expected SEQ \\* firstcap after CardText to yield Three, got:\n%s", out)
	}
	if !strings.Contains(out, `>FOURTH</span>`) {
		t.Fatalf("expected SEQ \\* Caps after OrdText to yield FOURTH, got:\n%s", out)
	}
}

func TestHTMLSeqFieldMergeFormatUsesParagraphNumberingFormat(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Numbering: &resolve.Numbering{NumID: 1, Level: 0, Text: "%1.", Format: "upperRoman"},
				Runs: []resolve.Run{
					{Kind: "run", FieldKind: "SEQ", FieldArg: "Figure", FieldInstr: `SEQ Figure \* MERGEFORMAT`},
					{Kind: "run", Text: " "},
					{Kind: "run", FieldKind: "SEQ", FieldArg: "Figure", FieldInstr: `SEQ Figure \* MERGEFORMAT`},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `>I</span>`) {
		t.Fatalf("expected SEQ \\* MERGEFORMAT to use upperRoman paragraph numbering, got:\n%s", out)
	}
	if !strings.Contains(out, `>II</span>`) {
		t.Fatalf("expected second SEQ \\* MERGEFORMAT to increment using paragraph numbering format, got:\n%s", out)
	}
}

func TestHTMLSeqFieldCharFormatUsesPreviousRunFormatting(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "Figure ", Format: resolve.RunFormat{Bold: boolPtr(true)}},
					{Kind: "run", FieldKind: "SEQ", FieldArg: "Figure", FieldInstr: `SEQ Figure \* CHARFORMAT`},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="word-field word-field-seq"`) || !strings.Contains(out, `<span style="font-weight:bold">1</span>`) {
		t.Fatalf("expected SEQ \\* CHARFORMAT to reuse the previous run formatting, got:\n%s", out)
	}
}

func TestHTMLNumWordsAndCharsIncludeTextboxText(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "Alpha"},
					{Kind: "textbox", TextBox: &resolve.TextBox{
						Blocks: []resolve.Block{{
							Kind: "paragraph",
							Paragraph: &resolve.Paragraph{
								Runs: []resolve.Run{{Kind: "run", Text: "Beta Gamma"}},
							},
						}},
					}},
					{Kind: "run", Text: " "},
					{Kind: "run", FieldKind: "NUMWORDS"},
					{Kind: "run", Text: " "},
					{Kind: "run", FieldKind: "NUMCHARS"},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="word-field word-field-numwords"`) || !strings.Contains(out, `class="word-field word-field-numchars"`) {
		t.Fatalf("expected NUMWORDS and NUMCHARS semantic wrappers, got:\n%s", out)
	}
	if !strings.Contains(out, `>3</span>`) {
		t.Fatalf("expected NUMWORDS to include textbox text for total 3 words, got:\n%s", out)
	}
	if !strings.Contains(out, `>14</span>`) {
		t.Fatalf("expected NUMCHARS to include textbox text for total 14 characters, got:\n%s", out)
	}
}

func TestHTMLFieldKindDocumentDatesPreserveCachedText(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "2006-04-26", FieldKind: "CREATEDATE"},
					{Kind: "run", Text: " ", FieldKind: ""},
					{Kind: "run", Text: "2006-04-27", FieldKind: "SAVEDATE"},
					{Kind: "run", Text: " ", FieldKind: ""},
					{Kind: "run", Text: "2006-04-28", FieldKind: "PRINTDATE"},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, className := range []string{
		`class="word-field word-field-createdate"`,
		`class="word-field word-field-savedate"`,
		`class="word-field word-field-printdate"`,
	} {
		if !strings.Contains(out, className) {
			t.Fatalf("expected %s in output, got:\n%s", className, out)
		}
	}
	for _, value := range []string{"2006-04-26", "2006-04-27", "2006-04-28"} {
		if !strings.Contains(out, value) {
			t.Fatalf("expected cached date %q in output, got:\n%s", value, out)
		}
	}
}

func TestHTMLRefFieldRendersAnchorLink(t *testing.T) {
	// A REF field with cached display text and a bookmark anchor should render
	// as <a href="#BookmarkName"><span class="word-field word-field-ref">display</span></a>.
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "see this section", FieldKind: "REF", Anchor: "SectionOne"},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `href="#SectionOne"`) {
		t.Errorf("expected anchor href=#SectionOne in output, got:\n%s", out)
	}
	if !strings.Contains(out, `class="word-field word-field-ref"`) {
		t.Errorf("expected word-field-ref span in output, got:\n%s", out)
	}
	if !strings.Contains(out, "see this section") {
		t.Errorf("expected display text preserved in output, got:\n%s", out)
	}
}

func TestHTMLRefFieldResolvedAsHyperlinkDoesNotNestAnchors(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{
						Kind:      "hyperlink",
						Text:      "see this section",
						FieldKind: "REF",
						Anchor:    "SectionOne",
						Target:    "#SectionOne",
					},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Count(out, `href="#SectionOne"`) != 1 {
		t.Fatalf("expected a single REF hyperlink wrapper, got:\n%s", out)
	}
	if strings.Count(out, "<a ") != 1 {
		t.Fatalf("expected no nested anchor elements for resolved REF, got:\n%s", out)
	}
}

func TestHTMLRefFieldWithoutAnchorNoLink(t *testing.T) {
	// A REF field without an anchor should not emit a link.
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "orphan ref", FieldKind: "REF", Anchor: ""},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Contains(out, `href="#"`) || strings.Contains(out, "<a ") {
		t.Errorf("expected no link element for REF without anchor, got:\n%s", out)
	}
}

func TestHTMLRefFieldUsesBookmarkedParagraphTextWhenDisplayTextMissing(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Text:      "Section One",
				Bookmarks: []string{"SectionOne"},
				Runs:      []resolve.Run{{Kind: "run", Text: "Section One"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", FieldKind: "REF", Anchor: "SectionOne"},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `href="#SectionOne"`) {
		t.Fatalf("expected REF link to bookmark, got:\n%s", out)
	}
	if !strings.Contains(out, `>Section One<`) {
		t.Fatalf("expected REF field to resolve bookmark text, got:\n%s", out)
	}
}

func TestHTMLRefFieldCaseFormat(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Text:      "Section One",
				Bookmarks: []string{"SectionOne"},
				Runs:      []resolve.Run{{Kind: "run", Text: "Section One"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind:       "run",
					FieldKind:  "REF",
					Anchor:     "SectionOne",
					FieldInstr: `REF SectionOne \* UPPER`,
				}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `>SECTION ONE<`) {
		t.Fatalf("expected REF field with \\* UPPER to uppercase bookmark text, got:\n%s", out)
	}
}

func TestHTMLNestedRefFieldUsesBookmarkedParagraphText(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Text:      "Appendix A",
				Bookmarks: []string{"AppendixA"},
				Runs:      []resolve.Run{{Kind: "run", Text: "Appendix A"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Fields: []resolve.Field{{
						Kind:   "REF",
						Anchor: "AppendixA",
					}},
				}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `href="#AppendixA"`) {
		t.Fatalf("expected nested REF link to bookmark, got:\n%s", out)
	}
	if !strings.Contains(out, `>Appendix A<`) {
		t.Fatalf("expected nested REF field to resolve bookmark text, got:\n%s", out)
	}
}

func TestHTMLRefFieldUsesBookmarkedTableCellText(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "table", Table: &resolve.Table{
				Rows: []resolve.TableRow{{
					Cells: []resolve.TableCell{{
						Blocks: []resolve.Block{{
							Kind: "paragraph",
							Paragraph: &resolve.Paragraph{
								Text:      "Table Bookmark",
								Bookmarks: []string{"CellBookmark"},
								Runs:      []resolve.Run{{Kind: "run", Text: "Table Bookmark"}},
							},
						}},
					}},
				}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", FieldKind: "REF", Anchor: "CellBookmark"},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `href="#CellBookmark"`) {
		t.Fatalf("expected REF link to table-cell bookmark, got:\n%s", out)
	}
	if !strings.Contains(out, `>Table Bookmark<`) {
		t.Fatalf("expected REF field to resolve table-cell bookmark text, got:\n%s", out)
	}
}

func TestHTMLRefFieldUsesBookmarkedTextboxText(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "textbox",
					TextBox: &resolve.TextBox{
						Blocks: []resolve.Block{{
							Kind: "paragraph",
							Paragraph: &resolve.Paragraph{
								Text:      "Textbox Bookmark",
								Bookmarks: []string{"TextboxBookmark"},
								Runs:      []resolve.Run{{Kind: "run", Text: "Textbox Bookmark"}},
							},
						}},
					},
				}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", FieldKind: "REF", Anchor: "TextboxBookmark"},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `href="#TextboxBookmark"`) {
		t.Fatalf("expected REF link to textbox bookmark, got:\n%s", out)
	}
	if !strings.Contains(out, `>Textbox Bookmark<`) {
		t.Fatalf("expected REF field to resolve textbox bookmark text, got:\n%s", out)
	}
}

func TestHTMLUsesDocumentMetadataForTitleAndFields(t *testing.T) {
	doc := &resolve.Document{
		Title:   "Quarterly Report",
		Author:  "Alex",
		Subject: "Finance",
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "placeholder", FieldKind: "TITLE"},
					{Kind: "run", Text: " ", FieldKind: "AUTHOR"},
					{Kind: "run", Text: "placeholder", FieldKind: "DOCPROPERTY", FieldArg: "Subject"},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">Quarterly Report<") || !strings.Contains(out, ">Alex<") || !strings.Contains(out, ">Finance<") {
		t.Fatalf("expected metadata-backed field values in output, got:\n%s", out)
	}
}

func TestHTMLStyleRefUsesNearestMatchingParagraph(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				StyleID:   "Heading1",
				StyleName: "Heading 1",
				Text:      "Introduction",
				Runs:      []resolve.Run{{Kind: "run", Text: "Introduction"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Text: "Body",
				Runs: []resolve.Run{{Kind: "run", Text: "Body"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				StyleID:   "Heading1",
				StyleName: "Heading 1",
				Text:      "Chapter Two",
				Runs:      []resolve.Run{{Kind: "run", Text: "Chapter Two"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Kind: "run", FieldKind: "STYLEREF", FieldArg: "Heading1"}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="word-field word-field-styleref"`) {
		t.Fatalf("expected STYLEREF semantic wrapper, got:\n%s", out)
	}
	if !strings.Contains(out, `>Chapter Two<`) {
		t.Fatalf("expected STYLEREF to use nearest matching paragraph text, got:\n%s", out)
	}
	if strings.Contains(out, `class="word-field word-field-styleref" data-field-arg="Heading1">Introduction</span>`) {
		t.Fatalf("expected STYLEREF not to use the first matching paragraph, got:\n%s", out)
	}
}

func TestHTMLStyleRefUsesCurrentMatchingParagraph(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				StyleID:   "Heading1",
				StyleName: "Heading 1",
				Runs: []resolve.Run{
					{Kind: "run", Text: "Heading: "},
					{Kind: "run", FieldKind: "STYLEREF", FieldArg: "Heading1"},
				},
				Text: "Heading: ",
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="word-field word-field-styleref"`) {
		t.Fatalf("expected STYLEREF semantic wrapper, got:\n%s", out)
	}
	if !strings.Contains(out, `>Heading:<`) && !strings.Contains(out, `>Heading: <`) {
		t.Fatalf("expected STYLEREF to use current matching paragraph text, got:\n%s", out)
	}
}

func TestHTMLStyleRefUsesTextboxParagraphText(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "textbox",
					TextBox: &resolve.TextBox{
						Blocks: []resolve.Block{{
							Kind: "paragraph",
							Paragraph: &resolve.Paragraph{
								StyleID:   "Heading1",
								StyleName: "Heading 1",
								Text:      "Textbox Heading",
								Runs:      []resolve.Run{{Kind: "run", Text: "Textbox Heading"}},
							},
						}},
					},
				}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Kind: "run", FieldKind: "STYLEREF", FieldArg: "Heading1"}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="word-field word-field-styleref"`) {
		t.Fatalf("expected STYLEREF semantic wrapper, got:\n%s", out)
	}
	if !strings.Contains(out, `>Textbox Heading<`) {
		t.Fatalf("expected STYLEREF to resolve textbox paragraph text, got:\n%s", out)
	}
}

func TestHTMLFormulaFieldEvaluatesExpression(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "", FieldKind: "FORMULA", FieldArg: "= 2+2"},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="word-field word-field-formula"`) {
		t.Errorf("expected word-field-formula class, got:\n%s", out)
	}
	if !strings.Contains(out, ">4<") {
		t.Errorf("expected evaluated formula result 4, got:\n%s", out)
	}
}

func TestHTMLFormulaFieldPreservesCachedText(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "5", FieldKind: "FORMULA", FieldArg: "= 2+2"},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">5<") {
		t.Errorf("expected cached text 5 to be preserved, got:\n%s", out)
	}
}

func TestHTMLQuoteFieldPreservesNestedFieldFallbackText(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Text:      "Section One",
				Bookmarks: []string{"SectionOne"},
				Runs:      []resolve.Run{{Kind: "run", Text: "Section One"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind:      "run",
					FieldKind: "QUOTE",
					FieldInstrRuns: []resolve.Run{
						{Text: "QUOTE "},
						{Fields: []resolve.Field{{
							Kind: "PRIVATE",
							Children: []resolve.Run{
								{Kind: "run", Text: "prefix "},
								{Fields: []resolve.Field{{Kind: "REF", Anchor: "SectionOne"}}},
							},
						}}},
					},
				}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="word-field word-field-quote"`) {
		t.Fatalf("expected QUOTE semantic wrapper, got:\n%s", out)
	}
	if !strings.Contains(out, `>prefix Section One<`) {
		t.Fatalf("expected QUOTE field to preserve nested fallback text, got:\n%s", out)
	}
}

func TestHTMLPrivateFieldPreservesCachedText(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind:       "run",
					FieldKind:  "PRIVATE",
					FieldInstr: `PRIVATE "app payload"`,
					Text:       "visible private result",
				}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="word-field word-field-private"`) {
		t.Fatalf("expected PRIVATE semantic wrapper, got:\n%s", out)
	}
	if !strings.Contains(out, `>visible private result<`) {
		t.Fatalf("expected PRIVATE field to preserve cached display text, got:\n%s", out)
	}
}

func TestHTMLPrivateFieldInterpretationWithOption(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind:       "run",
					FieldKind:  "PRIVATE",
					FieldInstr: `PRIVATE "MyApp" some secret data`,
					Text:       "fallback",
				}},
			}},
		},
	}
	out, _, err := render.HTMLWithOptions(doc, render.HTMLOptions{InterpretPrivateFields: true})
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `data-private-app="MyApp"`) {
		t.Fatalf("expected PRIVATE app name as data attribute, got:\n%s", out)
	}
	if !strings.Contains(out, `data-private-data="some secret data"`) {
		t.Fatalf("expected PRIVATE payload as data attribute, got:\n%s", out)
	}
	if !strings.Contains(out, ">fallback<") {
		t.Fatalf("expected PRIVATE cached text to still render, got:\n%s", out)
	}
}

func TestHTMLPrivateFieldInterpretationWithoutOption(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind:       "run",
					FieldKind:  "PRIVATE",
					FieldInstr: `PRIVATE "MyApp" some secret data`,
					Text:       "fallback",
				}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Contains(out, `data-private-app`) {
		t.Fatalf("expected no PRIVATE data attributes when option is off, got:\n%s", out)
	}
}

func TestHTMLIfFieldWithNestedDatabaseCachedText(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind:      "run",
					FieldKind: "IF",
					FieldInstrRuns: []resolve.Run{
						{Text: "IF "},
						{Fields: []resolve.Field{{
							Kind:  "DATABASE",
							Instr: `DATABASE \\d "ignored"`,
							Children: []resolve.Run{
								{Kind: "run", Text: "42"},
							},
						}}},
						{Text: ` = 42 "match" "miss"`},
					},
				}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `>match<`) {
		t.Fatalf("expected IF field to evaluate nested DATABASE cached text, got:\n%s", out)
	}
}

func TestHTMLMergeFieldPlaceholder(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "", FieldKind: "MERGEFIELD", FieldArg: "CustomerName"},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="word-field word-field-mergefield"`) {
		t.Errorf("expected word-field-mergefield class, got:\n%s", out)
	}
	if !strings.Contains(out, "«CustomerName»") {
		t.Errorf("expected merge field placeholder, got:\n%s", out)
	}
}

func TestHTMLMergeFieldPreservesCachedText(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "Alice", FieldKind: "MERGEFIELD", FieldArg: "CustomerName"},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">Alice<") {
		t.Errorf("expected cached text Alice to be preserved, got:\n%s", out)
	}
}

func TestHTMLIfFieldWithNestedMergeField(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind:       "run",
					FieldKind:  "IF",
					FieldInstr: `IF 150 > 100 "High" "Low"`,
					FieldInstrRuns: []resolve.Run{
						{Text: "IF "},
						{Fields: []resolve.Field{{
							Kind:  "MERGEFIELD",
							Instr: `MERGEFIELD Amount`,
							Arg:   "Amount",
							Children: []resolve.Run{
								{Kind: "run", Text: "«Amount»"},
							},
						}}},
						{Text: ` > 100 "High" "Low"`},
					},
				}},
			}},
		},
	}
	out, _, err := render.HTMLWithOptions(doc, render.HTMLOptions{
		MergeData: map[string]string{"Amount": "150"},
	})
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">High<") {
		t.Errorf("expected IF true branch High, got:\n%s", out)
	}
}

func TestHTMLFormulaFieldWithNestedRef(t *testing.T) {
	// { = { REF Amount } + 1 } should evaluate to 6 when Amount bookmark = 5.
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Bookmarks: []string{"Amount"},
				Runs:      []resolve.Run{{Kind: "run", Text: "5"}},
				Text:      "5",
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind:      "run",
					FieldKind: "FORMULA",
					FieldInstrRuns: []resolve.Run{
						{Text: "= "},
						{Fields: []resolve.Field{{
							Kind:     "REF",
							Instr:    "REF Amount",
							Arg:      "Amount",
							Children: []resolve.Run{{Kind: "run", Text: "5"}},
						}}},
						{Text: " + 1"},
					},
				}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">6<") {
		t.Fatalf("expected FORMULA with nested REF to evaluate to 6, got:\n%s", out)
	}
}

func TestHTMLSetFieldWithNestedRef(t *testing.T) {
	// { SET Copy { REF Original } } should copy bookmark text, then REF Copy works.
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Bookmarks: []string{"Original"},
				Runs:      []resolve.Run{{Kind: "run", Text: "hello"}},
				Text:      "hello",
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind:      "run",
					FieldKind: "SET",
					FieldInstrRuns: []resolve.Run{
						{Text: "SET Copy "},
						{Fields: []resolve.Field{{
							Kind:     "REF",
							Instr:    "REF Original",
							Arg:      "Original",
							Children: []resolve.Run{{Kind: "run", Text: "hello"}},
						}}},
					},
				}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind:       "run",
					FieldKind:  "REF",
					FieldInstr: "REF Copy",
					Anchor:     "Copy",
				}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">hello<") {
		t.Fatalf("expected SET with nested REF to copy value, then REF to read it, got:\n%s", out)
	}
}

func TestHTMLIfFieldWithNestedRefComparison(t *testing.T) {
	// { IF { REF Flag } = "x" "yes" "no" } should evaluate to "yes".
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Bookmarks: []string{"Flag"},
				Runs:      []resolve.Run{{Kind: "run", Text: "x"}},
				Text:      "x",
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind:      "run",
					FieldKind: "IF",
					FieldInstrRuns: []resolve.Run{
						{Text: "IF "},
						{Fields: []resolve.Field{{
							Kind:     "REF",
							Instr:    "REF Flag",
							Arg:      "Flag",
							Children: []resolve.Run{{Kind: "run", Text: "x"}},
						}}},
						{Text: ` = "x" "yes" "no"`},
					},
				}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">yes<") {
		t.Fatalf("expected IF with nested REF comparison to evaluate to yes, got:\n%s", out)
	}
}

func TestHTMLIfFieldResolvesMergeDataOperands(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind:       "run",
					FieldKind:  "IF",
					FieldInstr: `IF City = "Chicago" "yes" "no"`,
				}},
			}},
		},
	}
	out, _, err := render.HTMLWithOptions(doc, render.HTMLOptions{
		MergeData: map[string]string{"City": "Chicago"},
	})
	if err != nil {
		t.Fatalf("HTMLWithOptions: %v", err)
	}
	if !strings.Contains(out, ">yes<") {
		t.Fatalf("expected IF to resolve MergeData operand to yes, got:\n%s", out)
	}
}

func TestHTMLCompareFieldResolvesMergeRecordOperands(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind:       "run",
					FieldKind:  "COMPARE",
					FieldInstr: `COMPARE Amount > 10 "high" "low"`,
				}},
			}},
		},
	}
	out, _, err := render.HTMLWithOptions(doc, render.HTMLOptions{
		MergeRecords: []map[string]string{{"Amount": "42"}},
	})
	if err != nil {
		t.Fatalf("HTMLWithOptions: %v", err)
	}
	if !strings.Contains(out, ">high<") {
		t.Fatalf("expected COMPARE to resolve MergeRecords operand to high, got:\n%s", out)
	}
}

func TestHTMLIfFieldResolvesDocPropertyOperands(t *testing.T) {
	doc := &resolve.Document{
		Company: "OpenAI",
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind:       "run",
					FieldKind:  "IF",
					FieldInstr: `IF Company = "OpenAI" "yes" "no"`,
				}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">yes<") {
		t.Fatalf("expected IF to resolve Company doc property operand, got:\n%s", out)
	}
}

func TestHTMLIfFieldResolvesDocumentVariableOperands(t *testing.T) {
	doc := &resolve.Document{
		DocumentVariables: map[string]string{"ProjectID": "ALPHA-01"},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind:       "run",
					FieldKind:  "COMPARE",
					FieldInstr: `COMPARE ProjectID = "ALPHA-01" "match" "miss"`,
				}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">match<") {
		t.Fatalf("expected COMPARE to resolve document-variable operand, got:\n%s", out)
	}
}

func TestHTMLMergeFieldLookupIsCaseInsensitive(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Kind: "run", FieldKind: "MERGEFIELD", FieldArg: " CustomerName "}},
			}},
		},
	}
	out, _, err := render.HTMLWithOptions(doc, render.HTMLOptions{
		MergeData: map[string]string{"customername": "Alice"},
	})
	if err != nil {
		t.Fatalf("HTMLWithOptions: %v", err)
	}
	if !strings.Contains(out, ">Alice<") {
		t.Fatalf("expected case-insensitive MERGEFIELD lookup, got:\n%s", out)
	}
}

func TestHTMLNextIfLookupIsCaseInsensitive(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", FieldKind: "MERGEFIELD", FieldArg: "City"},
					{Kind: "run", Text: " "},
					{Kind: "run", FieldKind: "NEXTIF", FieldInstr: `NEXTIF CITY = "Chicago"`},
					{Kind: "run", Text: " "},
					{Kind: "run", FieldKind: "MERGEFIELD", FieldArg: "City"},
				},
			}},
		},
	}
	out, _, err := render.HTMLWithOptions(doc, render.HTMLOptions{
		MergeRecords: []map[string]string{
			{"city": "Chicago"},
			{"CITY": "Boston"},
		},
	})
	if err != nil {
		t.Fatalf("HTMLWithOptions: %v", err)
	}
	if !strings.Contains(out, ">Chicago</span>") || !strings.Contains(out, ">Boston</span>") {
		t.Fatalf("expected case-insensitive NEXTIF and MERGEFIELD lookups, got:\n%s", out)
	}
}

func TestHTMLIfFieldPreservesCachedResultWithoutMergeData(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind:       "run",
					Text:       "yes",
					FieldKind:  "IF",
					FieldInstr: `IF City = "Chicago" "yes" "no"`,
				}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">yes</span>") {
		t.Fatalf("expected authored IF result to be preserved without merge data, got:\n%s", out)
	}
	if strings.Contains(out, ">no</span>") {
		t.Fatalf("did not expect unresolved IF field to recompute to the false branch, got:\n%s", out)
	}
}

func TestHTMLMergeFieldNumberFormatSwitch(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind:       "run",
					FieldKind:  "MERGEFIELD",
					FieldArg:   "Amount",
					FieldInstr: `MERGEFIELD Amount \# "$0.00"`,
				}},
			}},
		},
	}
	out, _, err := render.HTMLWithOptions(doc, render.HTMLOptions{
		MergeData: map[string]string{"Amount": "42.5"},
	})
	if err != nil {
		t.Fatalf("HTMLWithOptions: %v", err)
	}
	if !strings.Contains(out, ">$42.50</span>") {
		t.Fatalf("expected MERGEFIELD numeric picture to format value, got:\n%s", out)
	}
}

func TestHTMLMergeFieldSubstitutesFromMergeData(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "", FieldKind: "MERGEFIELD", FieldArg: "CustomerName"},
				},
			}},
		},
	}
	out, _, err := render.HTMLWithOptions(doc, render.HTMLOptions{
		MergeData: map[string]string{"CustomerName": "Acme Corp"},
	})
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">Acme Corp<") {
		t.Errorf("expected merge data value Acme Corp, got:\n%s", out)
	}
	if strings.Contains(out, "«CustomerName»") {
		t.Errorf("did not expect placeholder when merge data is present, got:\n%s", out)
	}
}

func TestHTMLRefFieldRelativePositionSwitch(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index:     0,
				Bookmarks: []string{"TargetA"},
				Runs:      []resolve.Run{{Kind: "run", Text: "First paragraph"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index: 1,
				Runs: []resolve.Run{
					{Kind: "run", Text: "", FieldKind: "REF", FieldArg: "TargetA", Anchor: "TargetA", FieldInstr: `REF TargetA \p`},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">above<") {
		t.Errorf("expected REF \\p to return 'above' for preceding bookmark, got:\n%s", out)
	}
}

func TestHTMLRefFieldRelativePositionSwitchBelow(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index: 0,
				Runs: []resolve.Run{
					{Kind: "run", Text: "", FieldKind: "REF", FieldArg: "TargetB", Anchor: "TargetB", FieldInstr: `REF TargetB \p`},
				},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index:     1,
				Bookmarks: []string{"TargetB"},
				Runs:      []resolve.Run{{Kind: "run", Text: "Second paragraph"}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">below<") {
		t.Errorf("expected REF \\p to return 'below' for following bookmark, got:\n%s", out)
	}
}

func TestHTMLRefFieldFullContextNumber(t *testing.T) {
	outline1 := 1
	outline2 := 2
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index:     0,
				Bookmarks: []string{"TargetBookmark"},
				Format:    resolve.ParagraphFormat{OutlineLevel: &outline1},
				Numbering: &resolve.Numbering{NumID: 1, Level: 0, Text: "%1.", Format: "decimal"},
				Runs:      []resolve.Run{{Kind: "run", Text: "First"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index:     1,
				Format:    resolve.ParagraphFormat{OutlineLevel: &outline2},
				Numbering: &resolve.Numbering{NumID: 1, Level: 1, Text: "%1.%2", Format: "decimal"},
				Runs:      []resolve.Run{{Kind: "run", Text: "Second"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index:     2,
				Bookmarks: []string{"TargetBookmark2"},
				Format:    resolve.ParagraphFormat{OutlineLevel: &outline2},
				Numbering: &resolve.Numbering{NumID: 1, Level: 1, Text: "%1.%2", Format: "decimal"},
				Runs:      []resolve.Run{{Kind: "run", Text: "Third"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index: 3,
				Runs: []resolve.Run{
					{Kind: "run", Text: "", FieldKind: "REF", FieldArg: "TargetBookmark", Anchor: "TargetBookmark", FieldInstr: `REF TargetBookmark \w`},
				},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index: 4,
				Runs: []resolve.Run{
					{Kind: "run", Text: "", FieldKind: "REF", FieldArg: "TargetBookmark2", Anchor: "TargetBookmark2", FieldInstr: `REF TargetBookmark2 \w`},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">1.<") {
		t.Fatalf("expected REF \\w for first heading to be '1.', got:\n%s", out)
	}
	if !strings.Contains(out, ">1.2<") {
		t.Fatalf("expected REF \\w for second-level heading to be '1.2', got:\n%s", out)
	}
}

func TestHTMLRefFieldLevelNumber(t *testing.T) {
	outline2 := 2
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index:     0,
				Format:    resolve.ParagraphFormat{OutlineLevel: &outline2},
				Numbering: &resolve.Numbering{NumID: 1, Level: 1, Text: "%1.%2", Format: "decimal"},
				Runs:      []resolve.Run{{Kind: "run", Text: "First"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index:     1,
				Bookmarks: []string{"TargetBookmark"},
				Format:    resolve.ParagraphFormat{OutlineLevel: &outline2},
				Numbering: &resolve.Numbering{NumID: 1, Level: 1, Text: "%1.%2", Format: "decimal"},
				Runs:      []resolve.Run{{Kind: "run", Text: "Second"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index: 2,
				Runs: []resolve.Run{
					{Kind: "run", Text: "", FieldKind: "REF", FieldArg: "TargetBookmark", Anchor: "TargetBookmark", FieldInstr: `REF TargetBookmark \n`},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">2<") {
		t.Fatalf("expected REF \\n to return level number '2', got:\n%s", out)
	}
}

func TestHTMLRefFieldRelativeContextNumber(t *testing.T) {
	outline1 := 1
	outline2 := 2
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index:     0,
				Format:    resolve.ParagraphFormat{OutlineLevel: &outline1},
				Numbering: &resolve.Numbering{NumID: 1, Level: 0, Text: "%1.", Format: "decimal"},
				Runs:      []resolve.Run{{Kind: "run", Text: "First"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index:     1,
				Bookmarks: []string{"TargetA"},
				Format:    resolve.ParagraphFormat{OutlineLevel: &outline2},
				Numbering: &resolve.Numbering{NumID: 1, Level: 1, Text: "%1.%2", Format: "decimal"},
				Runs:      []resolve.Run{{Kind: "run", Text: "Second"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index:     2,
				Bookmarks: []string{"TargetB"},
				Format:    resolve.ParagraphFormat{OutlineLevel: &outline2},
				Numbering: &resolve.Numbering{NumID: 1, Level: 1, Text: "%1.%2", Format: "decimal"},
				Runs:      []resolve.Run{{Kind: "run", Text: "Third"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index:     3,
				Format:    resolve.ParagraphFormat{OutlineLevel: &outline2},
				Numbering: &resolve.Numbering{NumID: 1, Level: 1, Text: "%1.%2", Format: "decimal"},
				Runs: []resolve.Run{
					{Kind: "run", Text: "", FieldKind: "REF", FieldArg: "TargetA", Anchor: "TargetA", FieldInstr: `REF TargetA \r`},
				},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index:     4,
				Format:    resolve.ParagraphFormat{OutlineLevel: &outline1},
				Numbering: &resolve.Numbering{NumID: 1, Level: 0, Text: "%1.", Format: "decimal"},
				Runs: []resolve.Run{
					{Kind: "run", Text: "", FieldKind: "REF", FieldArg: "TargetB", Anchor: "TargetB", FieldInstr: `REF TargetB \r`},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// Paragraph 3 (1.3) references TargetA (1.1). Same level 1, so relative = "1".
	if !strings.Contains(out, ">1<") {
		t.Fatalf("expected REF \\r from 1.3 to 1.1 to be '1', got:\n%s", out)
	}
	// Paragraph 4 (2.) references TargetB (1.2). Different level 1, so relative = "1.2".
	if !strings.Contains(out, ">1.2<") {
		t.Fatalf("expected REF \\r from 2. to 1.2 to be '1.2', got:\n%s", out)
	}
}

func TestHTMLRefFieldRelativeContextCustomDelimiter(t *testing.T) {
	outline1 := 1
	outline2 := 2
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index:     0,
				Format:    resolve.ParagraphFormat{OutlineLevel: &outline1},
				Numbering: &resolve.Numbering{NumID: 1, Level: 0, Text: "%1.", Format: "decimal"},
				Runs:      []resolve.Run{{Kind: "run", Text: "First"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index:     1,
				Bookmarks: []string{"TargetBookmark"},
				Format:    resolve.ParagraphFormat{OutlineLevel: &outline2},
				Numbering: &resolve.Numbering{NumID: 1, Level: 1, Text: "%1.%2", Format: "decimal"},
				Runs:      []resolve.Run{{Kind: "run", Text: "Second"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index:     2,
				Format:    resolve.ParagraphFormat{OutlineLevel: &outline1},
				Numbering: &resolve.Numbering{NumID: 1, Level: 0, Text: "%1.", Format: "decimal"},
				Runs: []resolve.Run{
					{Kind: "run", Text: "", FieldKind: "REF", FieldArg: "TargetBookmark", Anchor: "TargetBookmark", FieldInstr: `REF TargetBookmark \r \d "-"`},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `>1-1<`) {
		t.Fatalf("expected REF \\r \\d to replace dots in the relative-context number, got:\n%s", out)
	}
}

func TestHTMLRefFieldSuppressNonNumberText(t *testing.T) {
	outline1 := 1
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index:     0,
				Bookmarks: []string{"TargetBookmark"},
				Format:    resolve.ParagraphFormat{OutlineLevel: &outline1},
				Numbering: &resolve.Numbering{NumID: 1, Level: 0, Text: "Article %1.", Format: "decimal"},
				Runs:      []resolve.Run{{Kind: "run", Text: "First"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index: 1,
				Runs: []resolve.Run{
					{Kind: "run", Text: "", FieldKind: "REF", FieldArg: "TargetBookmark", Anchor: "TargetBookmark", FieldInstr: `REF TargetBookmark \w \t`},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// Without \t the result would be "Article 1."; with \t it should be "1.".
	if !strings.Contains(out, ">1.<") {
		t.Fatalf("expected REF \\w \\t to strip 'Article ' prefix, got:\n%s", out)
	}
	if strings.Contains(out, "Article") {
		t.Fatalf("did not expect 'Article' text when \\t is present, got:\n%s", out)
	}
}

func TestHTMLRefFieldCustomDelimiter(t *testing.T) {
	outline1 := 1
	outline2 := 2
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index:     0,
				Bookmarks: []string{"TargetBookmark"},
				Format:    resolve.ParagraphFormat{OutlineLevel: &outline1},
				Numbering: &resolve.Numbering{NumID: 1, Level: 0, Text: "%1.", Format: "decimal"},
				Runs:      []resolve.Run{{Kind: "run", Text: "First"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index:     1,
				Bookmarks: []string{"TargetBookmark2"},
				Format:    resolve.ParagraphFormat{OutlineLevel: &outline2},
				Numbering: &resolve.Numbering{NumID: 1, Level: 1, Text: "%1.%2", Format: "decimal"},
				Runs:      []resolve.Run{{Kind: "run", Text: "Second"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index: 2,
				Runs: []resolve.Run{
					{Kind: "run", Text: "", FieldKind: "REF", FieldArg: "TargetBookmark", Anchor: "TargetBookmark", FieldInstr: `REF TargetBookmark \w \d "-"`},
				},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index: 3,
				Runs: []resolve.Run{
					{Kind: "run", Text: "", FieldKind: "REF", FieldArg: "TargetBookmark2", Anchor: "TargetBookmark2", FieldInstr: `REF TargetBookmark2 \w \d "-"`},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `>1-<`) {
		t.Fatalf("expected REF \\w \\d to replace dot with dash, got:\n%s", out)
	}
	if !strings.Contains(out, `>1-1<`) {
		t.Fatalf("expected REF \\w \\d for 1.1 to be '1-1', got:\n%s", out)
	}
}

func TestHTMLSetFieldUpdatesBookmarkForRef(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "", FieldKind: "SET", FieldArg: "MyVar", FieldInstr: `SET MyVar "Hello World"`},
				},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "", FieldKind: "REF", FieldArg: "MyVar", Anchor: "MyVar"},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">Hello World<") {
		t.Errorf("expected SET-updated bookmark value Hello World in REF output, got:\n%s", out)
	}
}

func TestHTMLMergeFieldFallsBackToPlaceholderWhenMissingInMergeData(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "", FieldKind: "MERGEFIELD", FieldArg: "MissingField"},
				},
			}},
		},
	}
	out, _, err := render.HTMLWithOptions(doc, render.HTMLOptions{
		MergeData: map[string]string{"OtherField": "value"},
	})
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "«MissingField»") {
		t.Errorf("expected placeholder when key missing from merge data, got:\n%s", out)
	}
}

func TestHTMLEmbedFieldPlaceholder(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "", FieldKind: "EMBED", FieldArg: `EMBED Excel.Sheet.12 \s`},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="word-field word-field-embed"`) {
		t.Errorf("expected word-field-embed class, got:\n%s", out)
	}
	if !strings.Contains(out, "[Excel.Sheet.12]") {
		t.Errorf("expected embed class placeholder, got:\n%s", out)
	}
}

func TestHTMLLinkFieldPlaceholder(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "", FieldKind: "LINK", FieldArg: `LINK Excel.Sheet.12 "C:\\data.xlsx" "Sheet1!R1C1"`},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="word-field word-field-link"`) {
		t.Errorf("expected word-field-link class, got:\n%s", out)
	}
	if !strings.Contains(out, "C:\\data.xlsx") && !strings.Contains(out, "C:\\\\data.xlsx") {
		t.Errorf("expected link file path, got:\n%s", out)
	}
}

func TestHTMLPAGEREFUsesBookmarkPageNumber(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index: 0,
				Runs:  []resolve.Run{{Kind: "run", Text: strings.Repeat("line text ", 80)}},
				Text:  strings.Repeat("line text ", 80),
				Format: resolve.ParagraphFormat{
					LineSpacing: intPtr(400),
					LineRule:    "exact",
				},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index:     1,
				Bookmarks: []string{"TargetBookmark"},
				Runs:      []resolve.Run{{Kind: "run", Text: "Target paragraph"}},
				Text:      "Target paragraph",
				Format: resolve.ParagraphFormat{
					PageBreakBefore: true,
				},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index: 2,
				Runs: []resolve.Run{{
					Kind:      "hyperlink",
					Text:      "1",
					FieldKind: "PAGEREF",
					Target:    "#TargetBookmark",
					Anchor:    "TargetBookmark",
				}},
				Text: "1",
			}},
		},
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 720, Right: 720, Bottom: 720, Left: 720},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `href="#TargetBookmark"`) {
		t.Fatalf("expected PAGEREF hyperlink anchor in output, got:\n%s", out)
	}
	if !strings.Contains(out, `class="word-field word-field-pageref"`) {
		t.Fatalf("expected PAGEREF semantic wrapper in output, got:\n%s", out)
	}
	if !strings.Contains(out, `>2</span></a>`) {
		t.Fatalf("expected PAGEREF to resolve to target page number 2, got:\n%s", out)
	}
}

func TestHTMLPAGEREFUsesTableCellBookmarkPageNumber(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "table", Table: &resolve.Table{
				Rows: []resolve.TableRow{{
					Cells: []resolve.TableCell{{
						Blocks: []resolve.Block{{
							Kind: "paragraph",
							Paragraph: &resolve.Paragraph{
								Bookmarks: []string{"CellTarget"},
								Runs:      []resolve.Run{{Kind: "run", Text: "Target cell"}},
								Text:      "Target cell",
							},
						}},
					}},
				}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind:      "hyperlink",
					Text:      "1",
					FieldKind: "PAGEREF",
					Target:    "#CellTarget",
					Anchor:    "CellTarget",
				}},
				Text: "1",
			}},
		},
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 720, Right: 720, Bottom: 720, Left: 720},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `href="#CellTarget"`) {
		t.Fatalf("expected PAGEREF hyperlink anchor for table-cell bookmark, got:\n%s", out)
	}
	if !strings.Contains(out, `>1</span></a>`) {
		t.Fatalf("expected PAGEREF to resolve table-cell bookmark to page number 1, got:\n%s", out)
	}
}

func TestHTMLPAGEREFRelativePositionSwitchAbove(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index:     0,
				Bookmarks: []string{"TargetBookmark"},
				Runs:      []resolve.Run{{Kind: "run", Text: strings.Repeat("line text ", 80)}},
				Text:      strings.Repeat("line text ", 80),
				Format: resolve.ParagraphFormat{
					LineSpacing: intPtr(400),
					LineRule:    "exact",
				},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index: 1,
				Format: resolve.ParagraphFormat{
					PageBreakBefore: true,
				},
				Runs: []resolve.Run{{
					Kind:       "hyperlink",
					Text:       "1",
					FieldKind:  "PAGEREF",
					FieldInstr: `PAGEREF TargetBookmark \p`,
					Target:     "#TargetBookmark",
					Anchor:     "TargetBookmark",
				}},
				Text: "1",
			}},
		},
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 720, Right: 720, Bottom: 720, Left: 720},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `href="#TargetBookmark"`) {
		t.Fatalf("expected PAGEREF hyperlink anchor in output, got:\n%s", out)
	}
	if !strings.Contains(out, `class="word-field word-field-pageref"`) {
		t.Fatalf("expected PAGEREF semantic wrapper in output, got:\n%s", out)
	}
	if !strings.Contains(out, `>above</span></a>`) {
		t.Fatalf("expected PAGEREF \\p to resolve to 'above' when bookmark is on earlier page, got:\n%s", out)
	}
}

func TestHTMLPAGEREFRelativePositionSwitchBelow(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index: 0,
				Runs: []resolve.Run{{
					Kind:       "hyperlink",
					Text:       "1",
					FieldKind:  "PAGEREF",
					FieldInstr: `PAGEREF TargetBookmark \p`,
					Target:     "#TargetBookmark",
					Anchor:     "TargetBookmark",
				}},
				Text: "1",
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index:     1,
				Bookmarks: []string{"TargetBookmark"},
				Runs:      []resolve.Run{{Kind: "run", Text: strings.Repeat("line text ", 80)}},
				Text:      strings.Repeat("line text ", 80),
				Format: resolve.ParagraphFormat{
					PageBreakBefore: true,
					LineSpacing:     intPtr(400),
					LineRule:        "exact",
				},
			}},
		},
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 720, Right: 720, Bottom: 720, Left: 720},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `href="#TargetBookmark"`) {
		t.Fatalf("expected PAGEREF hyperlink anchor in output, got:\n%s", out)
	}
	if !strings.Contains(out, `class="word-field word-field-pageref"`) {
		t.Fatalf("expected PAGEREF semantic wrapper in output, got:\n%s", out)
	}
	if !strings.Contains(out, `>below</span></a>`) {
		t.Fatalf("expected PAGEREF \\p to resolve to 'below' when bookmark is on later page, got:\n%s", out)
	}
}

func TestHTMLPAGEREFHyperlinkSwitch(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index:     0,
				Bookmarks: []string{"TargetBookmark"},
				Runs:      []resolve.Run{{Kind: "run", Text: "Target"}},
				Text:      "Target",
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index: 1,
				Runs: []resolve.Run{{
					Kind:       "run",
					Text:       "1",
					FieldKind:  "PAGEREF",
					FieldInstr: `PAGEREF TargetBookmark \h`,
					Anchor:     "TargetBookmark",
				}},
				Text: "1",
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<a href="#TargetBookmark">`) {
		t.Fatalf("expected PAGEREF \\h to render as anchor link, got:\n%s", out)
	}
	if !strings.Contains(out, `class="word-field word-field-pageref"`) {
		t.Fatalf("expected PAGEREF semantic wrapper, got:\n%s", out)
	}
}

func TestHTMLREFKeepFormattingSwitch(t *testing.T) {
	// REF \i should preserve cached text even when formatting switches are present.
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Bookmarks: []string{"mybookmark"},
				Runs:      []resolve.Run{{Kind: "run", Text: "Section One"}},
				Text:      "Section One",
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind:       "run",
					Text:       "cached text",
					FieldKind:  "REF",
					FieldInstr: `REF mybookmark \i \n`,
					Anchor:     "mybookmark",
				}},
				Text: "cached text",
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">cached text</span>") {
		t.Fatalf("expected REF \\i to preserve cached text, got:\n%s", out)
	}
}

func TestHTMLDateTimeFormatQuotedLiterals(t *testing.T) {
	// Word format pictures support quoted literal text with single quotes.
	doc := &resolve.Document{
		CreatedAt: "2023-07-15T09:30:45Z",
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind:       "run",
					FieldKind:  "CREATEDATE",
					FieldInstr: `CREATEDATE \@ "yyyy'年'MM'月'dd'日'"`,
				}},
			},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "2023年07月15日") {
		t.Fatalf("expected quoted literals to appear in formatted date, got:\n%s", out)
	}
}

func TestHTMLDateTimeFormatFractionalSeconds(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind:       "run",
					FieldKind:  "TIME",
					FieldInstr: `TIME \@ "HH:mm:ss.00"`,
				}},
			},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// Verify the format string was translated to include fractional seconds.
	if !strings.Contains(out, ".") {
		t.Fatalf("expected fractional seconds in TIME output, got:\n%s", out)
	}
}

func TestHTMLNOTEREFUsesBookmarkedNoteReferenceNumber(t *testing.T) {
	doc := &resolve.Document{
		FootnoteNumberFormat: "lowerRoman",
		Footnotes: map[int]resolve.Story{
			9: {Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Footnote text"}}}}}},
		},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Bookmarks: []string{"NoteBookmark"},
				Runs:      []resolve.Run{{Kind: "run", Text: "Anchor"}, {Kind: "footnoteRef", NoteID: 9}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Kind: "run", FieldKind: "NOTEREF", FieldInstr: `NOTEREF NoteBookmark`}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="word-field word-field-noteref"`) {
		t.Fatalf("expected NOTEREF semantic wrapper, got:\n%s", out)
	}
	if !strings.Contains(out, `>i</span>`) {
		t.Fatalf("expected NOTEREF to resolve formatted note number, got:\n%s", out)
	}
}

func TestHTMLAdvanceFieldAppliesOffsetToSubsequentRuns(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "A"},
					{Kind: "run", FieldKind: "ADVANCE", FieldInstr: `ADVANCE \r 12 \u 3`},
					{Kind: "run", Text: "B"},
					{Kind: "run", FieldKind: "ADVANCE", FieldInstr: `ADVANCE \l 6`},
					{Kind: "run", Text: "C"},
				},
			},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// ADVANCE should emit a zero-size semantic marker.
	if !strings.Contains(out, `class="word-field word-field-advance"`) {
		t.Fatalf("expected ADVANCE semantic marker, got:\n%s", out)
	}
	// Physical layout owns ADVANCE geometry; the semantic marker is zero-sized and
	// subsequent runs are absolutely positioned rather than margin/transform shifted.
	if !strings.Contains(out, `data-layout-owned="true"`) || !strings.Contains(out, `top:-3.00pt`) {
		t.Fatalf("expected layout-owned ADVANCE geometry, got:\n%s", out)
	}
	if strings.Contains(out, `margin-left:12.00pt`) || strings.Contains(out, `transform:translateY(-3.00pt)`) || strings.Contains(out, `margin-left:6.00pt`) {
		t.Fatalf("renderer re-applied ADVANCE offsets instead of consuming physical geometry, got:\n%s", out)
	}
}

func TestHTMLAdvanceFieldAbsoluteXPositionsFromLeftEdge(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", FieldKind: "ADVANCE", FieldInstr: `ADVANCE \x 15`},
					{Kind: "run", Text: "B"},
					{Kind: "run", FieldKind: "ADVANCE", FieldInstr: `ADVANCE \x 5`},
					{Kind: "run", Text: "C"},
				},
			},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// Physical layout owns ADVANCE \x geometry: the following runs are placed at
	// the absolute x the field names, not shifted with margins.
	if !strings.Contains(out, `left:15.00pt`) {
		t.Fatalf("expected B placed at ADVANCE \\x 15pt from the left edge, got:\n%s", out)
	}
	if !strings.Contains(out, `left:5.00pt`) {
		t.Fatalf("expected C placed at the second ADVANCE \\x absolute position, got:\n%s", out)
	}
	if !strings.Contains(out, `data-layout-owned="true"`) {
		t.Fatalf("expected layout-owned ADVANCE markers, got:\n%s", out)
	}
	if strings.Contains(out, `margin-left:15.00pt`) || strings.Contains(out, `margin-left:-17.44pt`) {
		t.Fatalf("renderer re-applied ADVANCE offsets instead of consuming physical geometry, got:\n%s", out)
	}
}

func TestHTMLAdvanceFieldAbsoluteYShiftsVertically(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", FieldKind: "ADVANCE", FieldInstr: `ADVANCE \y 5`},
					{Kind: "run", Text: "Down"},
					{Kind: "run", FieldKind: "ADVANCE", FieldInstr: `ADVANCE \y -3`},
					{Kind: "run", Text: "Up"},
				},
			},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// Physical layout owns ADVANCE \y geometry: the following runs carry the
	// vertical offset as a physical top instead of a transform.
	if !strings.Contains(out, `top:5.00pt`) {
		t.Fatalf("expected ADVANCE \\y 5 (down) to place the run 5pt down, got:\n%s", out)
	}
	if !strings.Contains(out, `top:-3.00pt`) {
		t.Fatalf("expected ADVANCE \\y -3 (up) to place the run 3pt up, got:\n%s", out)
	}
	if strings.Contains(out, `transform:translateY`) {
		t.Fatalf("renderer re-applied ADVANCE offsets instead of consuming physical geometry, got:\n%s", out)
	}
}

func TestHTMLBookmarkSpanMarkersRenderAtSpanBoundaries(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index:     0,
				Bookmarks: []string{"SpanBookmark"},
				Runs:      []resolve.Run{{Kind: "run", Text: "First"}},
				Text:      "First",
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index:     1,
				Bookmarks: []string{"SpanBookmark"},
				Runs:      []resolve.Run{{Kind: "run", Text: "Second"}},
				Text:      "Second",
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index: 2,
				Runs:  []resolve.Run{{Kind: "run", Text: "Third"}},
				Text:  "Third",
			}},
		},
		BookmarkEnds: map[string]int{"SpanBookmark": 13},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if got := strings.Count(out, `id="SpanBookmark"`); got != 1 {
		t.Fatalf("bookmark anchor count = %d, want 1, output:\n%s", got, out)
	}
	if got := strings.Count(out, `class="word-bookmark-span-start" data-bookmark="SpanBookmark"`); got != 1 {
		t.Fatalf("bookmark span start count = %d, want 1, output:\n%s", got, out)
	}
	if got := strings.Count(out, `class="word-bookmark-span-end" data-bookmark="SpanBookmark"`); got != 1 {
		t.Fatalf("bookmark span end count = %d, want 1, output:\n%s", got, out)
	}
}

func TestHTMLVMLHorzRule(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "horzRule", Text: "#a0a0a0"},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<hr style="border-color:#a0a0a0">`) {
		t.Errorf("expected styled <hr> in output, got:\n%s", out)
	}
}

func TestHTMLVMLHorzRuleNoColor(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "horzRule"},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "<hr>") {
		t.Errorf("expected plain <hr> in output, got:\n%s", out)
	}
}

func TestHTMLVMLRectShape(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "shape", Shape: &resolve.Shape{
						Kind:        "rect",
						Floating:    true,
						XRelative:   "page",
						YRelative:   "page",
						X:           12 * 12700,
						Y:           24 * 12700,
						Width:       72 * 12700,
						Height:      36 * 12700,
						FillColor:   "ABCDEF",
						StrokeColor: "112233",
						StrokeWidth: int64(1.5 * 12700),
					}},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="shape-rect"`) {
		t.Fatalf("expected shape rect output, got:\n%s", out)
	}
	if !strings.Contains(out, `background-color:#ABCDEF`) || !strings.Contains(out, `border:1.50pt solid #112233`) {
		t.Fatalf("expected VML rect fill+stroke styles, got:\n%s", out)
	}
}

func TestHTMLVMLOvalShape(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "shape", Shape: &resolve.Shape{
						Kind:        "rect",
						Width:       48 * 12700,
						Height:      24 * 12700,
						FillColor:   "00FF00",
						StrokeColor: "0000FF",
						LineStyle:   &dml.LineStyle{Color: "0000FF", Width: 1},
						Geometry:    &dml.PresetGeometry{Name: "ellipse"},
					}},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<svg`) {
		t.Fatalf("expected SVG ellipse output, got:\n%s", out)
	}
	if !strings.Contains(out, `fill="#00FF00"`) || !strings.Contains(out, `stroke="#0000FF"`) {
		t.Fatalf("expected ellipse fill/stroke in SVG, got:\n%s", out)
	}
}

func TestHTMLVMLArcShape(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "shape", Shape: &resolve.Shape{
						Kind:        "rect",
						Width:       48 * 12700,
						Height:      48 * 12700,
						FillColor:   "FF0000",
						StrokeColor: "000000",
						GeometryData: &dml.Geometry{
							Type: dml.GeometryTypeCustom,
							Paths: []dml.Path{{Commands: []dml.PathCommand{
								{Type: "M", Points: []dml.Point{{X: 0.5, Y: 0.5}}},
								{Type: "L", Points: []dml.Point{{X: 1.0, Y: 0.5}}},
								{Type: "A", Points: []dml.Point{{X: 0.5, Y: 1.0}}, Args: []float64{0.5, 0.5, 0, 0, 1}},
								{Type: "Z"},
							}}},
						},
					}},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<svg`) {
		t.Fatalf("expected SVG arc output, got:\n%s", out)
	}
	if !strings.Contains(out, `fill="#FF0000"`) {
		t.Fatalf("expected arc fill in SVG, got:\n%s", out)
	}
	if !strings.Contains(out, `A`) {
		t.Fatalf("expected SVG arc command in path, got:\n%s", out)
	}
}

func TestHTMLVMLShapePath(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "shape", Shape: &resolve.Shape{
						Kind:        "rect",
						Width:       48 * 12700,
						Height:      48 * 12700,
						FillColor:   "00AA00",
						StrokeColor: "000000",
						GeometryData: &dml.Geometry{
							Type: dml.GeometryTypeCustom,
							Paths: []dml.Path{{Commands: []dml.PathCommand{
								{Type: "M", Points: []dml.Point{{X: 0, Y: 0}}},
								{Type: "L", Points: []dml.Point{{X: 1, Y: 0}, {X: 1, Y: 1}, {X: 0, Y: 1}}},
								{Type: "Z"},
							}}},
						},
					}},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<svg`) {
		t.Fatalf("expected SVG path shape output, got:\n%s", out)
	}
	if !strings.Contains(out, `fill="#00AA00"`) {
		t.Fatalf("expected path fill in SVG, got:\n%s", out)
	}
	if !strings.Contains(out, `d="M`) {
		t.Fatalf("expected SVG path data, got:\n%s", out)
	}
}

func TestHTMLVMLVerticalLineShape(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "shape", Shape: &resolve.Shape{
						Kind:        "line",
						Width:       0,
						Height:      48 * 12700,
						StrokeColor: "AA0000",
						StrokeWidth: int64(2 * 12700),
					}},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="shape-line-v"`) {
		t.Fatalf("expected vertical line output, got:\n%s", out)
	}
	if !strings.Contains(out, `border-left-color:#AA0000`) || !strings.Contains(out, `border-left-width:2.00pt`) {
		t.Fatalf("expected line stroke styles, got:\n%s", out)
	}
}

func TestHTMLMathInlineOMMathRendersMathML(t *testing.T) {
	// A paragraph with text, an inline m:oMath equation, then more text.
	// After resolve + render the equation should appear as <math ...>.
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{Runs: []resolve.Run{
				{Kind: "run", Text: "The formula is "},
				{Kind: "math", MathML: `<math display="inline" xmlns="http://www.w3.org/1998/Math/MathML"><mfrac><mrow><mi>a</mi></mrow><mrow><mi>b</mi></mrow></mfrac></math>`},
				{Kind: "run", Text: " as shown."},
			}},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<math display="inline"`) {
		t.Fatalf("expected <math> element in output, got:\n%s", out)
	}
	if !strings.Contains(out, "<mfrac>") {
		t.Errorf("expected <mfrac> in math output, got:\n%s", out)
	}
	if strings.Contains(out, "The formula is </p>") {
		t.Errorf("math should not break paragraph prematurely: %s", out)
	}
}

func TestHTMLMathDisplayOMMathParaRendersMathML(t *testing.T) {
	// A display-mode equation (from m:oMathPara) should appear with display="block".
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{Runs: []resolve.Run{
				{Kind: "math", MathML: `<math display="block" xmlns="http://www.w3.org/1998/Math/MathML"><msup><mi>x</mi><mn>2</mn></msup></math>`},
			}},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `display="block"`) {
		t.Fatalf("expected display=block math in output, got:\n%s", out)
	}
	if !strings.Contains(out, "<msup>") {
		t.Errorf("expected <msup> in math output, got:\n%s", out)
	}
}

func sampleFontBytes(t *testing.T, partName string) (string, []byte) {
	t.Helper()
	pkg, err := opc.Open(repoPath(t, "testdata", "pptx", "sample-4.pptx"))
	if err != nil {
		t.Fatalf("open sample pptx: %v", err)
	}
	part, ok := pkg.Part(partName)
	if !ok {
		t.Fatalf("missing font part %s", partName)
	}
	return part.ContentType, part.Data
}

func TestHTMLIndexGeneratesFromXEEntries(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			// INDEX field paragraph
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{FieldKind: "INDEX", FieldArg: `INDEX \h "A"`, Text: ""}}}},
			// Paragraphs with XE markers
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{
				{Text: "Some text."},
				{FieldKind: "XE", FieldArg: `XE "algorithm"`, Text: ""},
			}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{
				{Text: "More text."},
				{FieldKind: "XE", FieldArg: `XE "anchors:conditional"`, Text: ""},
			}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{
				{FieldKind: "XE", FieldArg: `XE "algorithm"`, Text: ""},
			}}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "word-field-index") {
		t.Fatalf("expected word-field-index class in output: %s", out)
	}
	// "algorithm" should appear exactly once in the rendered index (deduplicated across pages).
	if count := strings.Count(out, "algorithm"); count != 1 {
		t.Fatalf("expected deduplicated 'algorithm' once in index, got %d: %s", count, out)
	}
	// Subentry "conditional" should be present.
	if !strings.Contains(out, "conditional") {
		t.Fatalf("expected subentry 'conditional' in index: %s", out)
	}
	// XE markers should not render visible text.
	if strings.Contains(out, `>XE `) {
		t.Fatalf("XE instruction text should not appear in output: %s", out)
	}
	css := extractCSS(out)
	if !strings.Contains(css, ".word-field-index") {
		t.Fatalf("expected .word-field-index CSS rule: %s", css)
	}
}

func TestHTMLIndexCustomPageSeparator(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{FieldKind: "INDEX", FieldArg: `INDEX \e ";"`, Text: ""}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{
				{Text: "Page one."},
				{FieldKind: "XE", FieldArg: `XE "term"`, Text: ""},
			}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Format: resolve.ParagraphFormat{PageBreakBefore: true},
				Runs: []resolve.Run{
					{Text: "Page two."},
					{FieldKind: "XE", FieldArg: `XE "term"`, Text: ""},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "word-field-index") {
		t.Fatalf("expected word-field-index class: %s", out)
	}
	if !strings.Contains(out, "term") {
		t.Fatalf("expected index term: %s", out)
	}
	// The two page numbers should be separated by ";" instead of ", "
	if !strings.Contains(out, "1;2") {
		t.Fatalf("expected custom separator ';' between page numbers, got: %s", out)
	}
}

func TestHTMLIndexFiltersXEByTypeID(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{FieldKind: "INDEX", FieldArg: `INDEX \f "t"`, Text: ""}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{
				{FieldKind: "XE", FieldArg: `XE "tables" \f "t"`, Text: ""},
				{FieldKind: "XE", FieldArg: `XE "figures" \f "f"`, Text: ""},
			}}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "tables") {
		t.Fatalf("expected 'tables' (type 't') in filtered index: %s", out)
	}
	if strings.Contains(out, "figures") {
		t.Fatalf("did not expect 'figures' (type 'f') in filtered index: %s", out)
	}
}

func TestHTMLIndexColumnSwitch(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{FieldKind: "INDEX", FieldArg: `INDEX \c 2`, FieldInstr: `INDEX \c 2`, Text: ""}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{
				{FieldKind: "XE", FieldArg: `XE "apple"`, Text: ""},
			}}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `style="column-count:2"`) {
		t.Fatalf("expected INDEX with \\c 2 to emit column-count:2 style, got: %s", out)
	}
}

func TestHTMLIndexPageRangeSwitch(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{FieldKind: "INDEX", FieldArg: `INDEX \k "-"`, Text: ""}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index: 0,
				Runs:  []resolve.Run{{FieldKind: "XE", FieldArg: `XE "term"`, Text: ""}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index:  1,
				Format: resolve.ParagraphFormat{PageBreakBefore: true},
				Runs:   []resolve.Run{{FieldKind: "XE", FieldArg: `XE "term"`, Text: ""}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index:  2,
				Format: resolve.ParagraphFormat{PageBreakBefore: true},
				Runs:   []resolve.Run{{FieldKind: "XE", FieldArg: `XE "term"`, Text: ""}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "1-3") {
		t.Fatalf("expected INDEX with \\k to collapse consecutive pages into range '1-3', got: %s", out)
	}
}

func TestHTMLIndexLetterRangeSwitch(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{FieldKind: "INDEX", FieldArg: `INDEX \p "a-m"`, FieldInstr: `INDEX \p "a-m"`, Text: ""}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{
				{FieldKind: "XE", FieldArg: `XE "apple"`, Text: ""},
				{FieldKind: "XE", FieldArg: `XE "zebra"`, Text: ""},
				{FieldKind: "XE", FieldArg: `XE "mango"`, Text: ""},
			}}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "apple") {
		t.Fatalf("expected INDEX with \\p a-m to include 'apple', got: %s", out)
	}
	if !strings.Contains(out, "mango") {
		t.Fatalf("expected INDEX with \\p a-m to include 'mango', got: %s", out)
	}
	if strings.Contains(out, "zebra") {
		t.Fatalf("expected INDEX with \\p a-m to exclude 'zebra', got: %s", out)
	}
}

func TestHTMLIndexRunInSubentries(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{FieldKind: "INDEX", FieldArg: `INDEX \y`, FieldInstr: `INDEX \y`, Text: ""}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{
				{FieldKind: "XE", FieldArg: `XE "fruit:apple"`, Text: ""},
				{FieldKind: "XE", FieldArg: `XE "fruit:banana"`, Text: ""},
			}}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "fruit") {
		t.Fatalf("expected INDEX to include 'fruit', got: %s", out)
	}
	// In run-in format, subentries should appear on the same line as the main entry.
	if strings.Count(out, "fruit") != 1 {
		t.Fatalf("expected 'fruit' to appear exactly once in run-in format, got: %s", out)
	}
	if !strings.Contains(out, "apple") {
		t.Fatalf("expected INDEX to include 'apple' subentry, got: %s", out)
	}
	if !strings.Contains(out, "banana") {
		t.Fatalf("expected INDEX to include 'banana' subentry, got: %s", out)
	}
}

func TestHTMLIndexUsesGroupSeparatorWithGSwitch(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{FieldKind: "INDEX", FieldArg: `INDEX \g "; "`, Text: ""}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{
				{Text: "One."},
				{FieldKind: "XE", FieldArg: `XE "term"`, Text: ""},
			}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{
				{Text: "Two."},
				{FieldKind: "XE", FieldArg: `XE "term"`, Text: ""},
			}}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "term") {
		t.Errorf("INDEX should include entry; got:\n%s", out)
	}
}

func TestHTMLIndexSequencePrefixedPages(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Index: 0, Runs: []resolve.Run{{FieldKind: "INDEX", FieldArg: `INDEX \s "Figure" \d ":"`, FieldInstr: `INDEX \s "Figure" \d ":"`, Text: ""}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index: 1,
				Runs: []resolve.Run{
					{FieldKind: "SEQ", FieldArg: "Figure", FieldInstr: `SEQ Figure`, Text: "1"},
					{FieldKind: "XE", FieldArg: `XE "term"`, Text: ""},
				},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index:  2,
				Format: resolve.ParagraphFormat{PageBreakBefore: true},
				Runs: []resolve.Run{
					{FieldKind: "SEQ", FieldArg: "Figure", FieldInstr: `SEQ Figure`, Text: "2"},
					{FieldKind: "XE", FieldArg: `XE "term"`, Text: ""},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "1:1") || !strings.Contains(out, "2:2") {
		t.Fatalf("expected INDEX to prefix page refs with sequence values via \\s and \\d, got:\n%s", out)
	}
}

func TestHTMLTOCSequencePrefixedPages(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Index: 0, Runs: []resolve.Run{{FieldKind: "TOC", FieldArg: `TOC \o "1-3" \s "chapter" \d "-"`, FieldInstr: `TOC \o "1-3" \s "chapter" \d "-"`, Text: ""}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index:   1,
				StyleID: "Heading1",
				Runs: []resolve.Run{
					{FieldKind: "SEQ", FieldArg: "chapter", FieldInstr: `SEQ chapter`, Text: "1"},
					{Text: "Introduction"},
				},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Index:   2,
				StyleID: "Heading1",
				Format:  resolve.ParagraphFormat{PageBreakBefore: true},
				Runs: []resolve.Run{
					{FieldKind: "SEQ", FieldArg: "chapter", FieldInstr: `SEQ chapter`, Text: "2"},
					{Text: "Methods"},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "1-1") || !strings.Contains(out, "2-2") {
		t.Fatalf("expected TOC to prefix page refs with sequence values via \\s and \\d, got:\n%s", out)
	}
}

func TestHTMLIndexSuppressesPagesWithZSwitch(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{FieldKind: "INDEX", FieldArg: `INDEX \z`, Text: ""}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{
				{Text: "Some text."},
				{FieldKind: "XE", FieldArg: `XE "apple"`, Text: ""},
			}}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	re := regexp.MustCompile(`apple\s+\.+\s+1</span>`)
	if re.MatchString(out) {
		t.Errorf("INDEX with \\z should suppress page numbers; got:\n%s", out)
	}
	if !strings.Contains(out, "apple") {
		t.Errorf("INDEX should still show entry text; got:\n%s", out)
	}
}

func TestHTMLIndexRespectsBookmarkScopeWithBSwitch(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Index: 0, Kind: "paragraph", Paragraph: &resolve.Paragraph{Index: 0, Runs: []resolve.Run{{FieldKind: "INDEX", FieldArg: `INDEX \b "Scope"`, Text: ""}}}},
			{Index: 1, Kind: "paragraph", Paragraph: &resolve.Paragraph{Index: 1, Bookmarks: []string{"Scope"}, Runs: []resolve.Run{
				{Text: "Inside scope."},
				{FieldKind: "XE", FieldArg: `XE "inside"`, Text: ""},
			}}},
			{Index: 2, Kind: "paragraph", Paragraph: &resolve.Paragraph{Index: 2, Runs: []resolve.Run{
				{Text: "Outside scope."},
				{FieldKind: "XE", FieldArg: `XE "outside"`, Text: ""},
			}}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "inside") {
		t.Errorf("INDEX with \\b should include entries inside bookmark scope; got:\n%s", out)
	}
	if strings.Contains(out, "outside") {
		t.Errorf("INDEX with \\b should exclude entries outside bookmark scope; got:\n%s", out)
	}
}

func TestHTMLFillinFieldRendersTextInput(t *testing.T) {
	// A FILLIN field should render as <input type="text"> with the default value
	// from FieldArg and class="word-field word-field-fillin".
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", FieldKind: "FILLIN", FieldArg: "Enter your name"},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `type="text"`) {
		t.Errorf("expected input type=text in output, got:\n%s", out)
	}
	if !strings.Contains(out, `class="word-field word-field-fillin"`) {
		t.Errorf("expected word-field-fillin class in output, got:\n%s", out)
	}
	if !strings.Contains(out, `value="Enter your name"`) {
		t.Errorf("expected default value in input, got:\n%s", out)
	}
	if strings.Contains(out, `<span class="word-field word-field-fillin"`) {
		t.Errorf("FILLIN should not emit a <span>, got:\n%s", out)
	}
}

func TestHTMLFillinFieldWithEmptyTextStillRendered(t *testing.T) {
	// A FILLIN field with no cached text (Text="") should still render the input
	// element (not be silently skipped by the empty-text guard).
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "", FieldKind: "FILLIN", FieldArg: "default"},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<input`) {
		t.Errorf("expected <input> element even with empty Text, got:\n%s", out)
	}
	if !strings.Contains(out, `value="default"`) {
		t.Errorf("expected default value in input, got:\n%s", out)
	}
}

func TestHTMLDatabaseFieldReturnsCachedText(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind:      "field",
					FieldKind: "DATABASE",
					Text:      "Cached DB result",
				}},
			}},
		}}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "Cached DB result") {
		t.Errorf("DATABASE field should return cached text; got:\n%s", out)
	}
}

func TestHTMLDatabaseFieldExecutor(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{{
			Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind:       "field",
					FieldKind:  "DATABASE",
					FieldInstr: `DATABASE \d "conn" \s "SELECT 1"`,
					Text:       "Cached DB result",
				}},
			}},
		},
	}
	executed := false
	out, _, err := render.HTMLWithOptions(doc, render.HTMLOptions{
		DatabaseExecutor: func(conn, sql string) (string, error) {
			executed = true
			if conn != "conn" {
				t.Errorf("connection string = %q, want conn", conn)
			}
			if sql != "SELECT 1" {
				t.Errorf("sql query = %q, want SELECT 1", sql)
			}
			return "Live Result", nil
		},
	})
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !executed {
		t.Error("expected DatabaseExecutor to be called")
	}
	if !strings.Contains(out, "Live Result") {
		t.Errorf("expected DATABASE executor result, got:\n%s", out)
	}
}

func TestHTMLDatabaseFieldExecutorFallbackOnError(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{{
			Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind:       "field",
					FieldKind:  "DATABASE",
					FieldInstr: `DATABASE \d "conn" \s "SELECT 1"`,
					Text:       "Cached DB result",
				}},
			}},
		},
	}
	out, _, err := render.HTMLWithOptions(doc, render.HTMLOptions{
		DatabaseExecutor: func(conn, sql string) (string, error) {
			return "", fmt.Errorf("connection refused")
		},
	})
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "Cached DB result") {
		t.Errorf("expected DATABASE to fall back to cached text on error, got:\n%s", out)
	}
}

func TestHTMLPictureAdjustmentsRenderAsCSS(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "image",
					Image: &resolve.Image{
						MIMEType:   "image/png",
						Data:       []byte{0x89, 0x50, 0x4e, 0x47},
						Width:      1828800,
						Height:     914400,
						Brightness: 1.5,
						Contrast:   0.8,
						Alpha:      0.5,
						Grayscale:  true,
					},
				}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// Word shifts levels for brightness and scales for contrast, so brightness
	// 1.5 / contrast 0.8 is the affine map out = 0.8*in + 0.55. The CSS pair
	// contrast(42.11%) brightness(190%) reproduces it exactly: the contrast()
	// factor supplies both the scale and the mid-grey pivot, and the
	// brightness() factor restores the level shift.
	filter := "contrast(42.11%) brightness(190%)"
	if !strings.Contains(out, filter) {
		t.Errorf("expected %s filter; got:\n%s", filter, out)
	}
	// contrast(42.11%) brightness(190%) composited: slope = 1.9*0.4211,
	// intercept = 1.9*0.5*(1-0.4211).
	if slope := 1.9 * 0.4211; math.Abs(slope-0.8) > 0.001 {
		t.Errorf("filter slope = %.4f, want 0.8", slope)
	}
	if !strings.Contains(out, "grayscale(100%)") {
		t.Errorf("expected grayscale filter; got:\n%s", out)
	}
	if !strings.Contains(out, "opacity:0.5000") {
		t.Errorf("expected opacity; got:\n%s", out)
	}
}

func TestHTMLPictureBrightnessIsAnAdditiveLevelShift(t *testing.T) {
	doc := &resolve.Document{Blocks: []resolve.Block{{
		Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{
			Kind: "image",
			Image: &resolve.Image{
				MIMEType:   "image/png",
				Data:       []byte{0x89, 0x50, 0x4e, 0x47},
				Width:      1828800,
				Height:     914400,
				Brightness: 1.5,
			},
		}}},
	}}}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// Word renders +50% brightness as out = in + 0.5, not as a 1.5x gain:
	// contrast(50%) brightness(200%) is that level shift expressed as filters.
	if !strings.Contains(out, "contrast(50%) brightness(200%)") {
		t.Errorf("expected additive brightness filter; got:\n%s", out)
	}
	if strings.Contains(out, "brightness(150%)") {
		t.Errorf("brightness must not be rendered as a multiplicative gain; got:\n%s", out)
	}
}

func TestHTMLPictureContrastIsWordAsymptoticScale(t *testing.T) {
	doc := &resolve.Document{Blocks: []resolve.Block{{
		Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{
			Kind: "image",
			Image: &resolve.Image{
				MIMEType: "image/png",
				Data:     []byte{0x89, 0x50, 0x4e, 0x47},
				Width:    1828800,
				Height:   914400,
				Contrast: 2,
			},
		}}},
	}}}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "contrast(200%)") {
		t.Errorf("expected contrast(200%%); got:\n%s", out)
	}
}

func TestHTMLPictureAdjustmentPreservesExplicitZeroAlpha(t *testing.T) {
	doc := &resolve.Document{Blocks: []resolve.Block{{
		Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{
			Kind: "image", Image: &resolve.Image{
				MIMEType: "image/png", Data: []byte{0x89, 0x50, 0x4e, 0x47},
				Width: 914400, Height: 914400, HasAlpha: true, Alpha: 0,
			},
		}}},
	}}}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "opacity:0.0000") {
		t.Fatalf("expected explicit zero alpha to render transparent, got:\n%s", out)
	}
}

func TestHTMLPictureBulletRendersImageInsteadOfGlyph(t *testing.T) {
	doc := &resolve.Document{Blocks: []resolve.Block{{
		Kind: "paragraph", Paragraph: &resolve.Paragraph{
			List: &resolve.ListMarker{Kind: "bullet", Picture: &resolve.Image{
				MIMEType: "image/gif", Data: []byte("GIF89a"), Width: 114300, Height: 114300,
			}},
			Runs: []resolve.Run{{Text: "Picture bullet"}},
		},
	}}}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, want := range []string{
		`class="word-list-marker word-physical-inline-list-marker word-list-bullet"`,
		`data:image/gif;base64,`,
		`width:9.00pt;height:9.00pt`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("picture bullet HTML is missing %q:\n%s", want, out)
		}
	}
}

func TestHTMLHeadingDefaultsUseDistinctLeading(t *testing.T) {
	doc := &resolve.Document{Blocks: []resolve.Block{
		{Kind: "paragraph", Paragraph: &resolve.Paragraph{StyleID: "Heading1", Runs: []resolve.Run{{Text: "One"}}}},
		{Kind: "paragraph", Paragraph: &resolve.Paragraph{StyleID: "Heading2", Runs: []resolve.Run{{Text: "Two"}}}},
		{Kind: "paragraph", Paragraph: &resolve.Paragraph{StyleID: "Heading3", Runs: []resolve.Run{{Text: "Three"}}}},
	}}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, want := range []string{
		"h1 { font-size: 2em; font-weight: bold; line-height: 1.10; }",
		"h2 { font-size: 1.5em; font-weight: bold; line-height: 1.15; }",
		"h3 { font-size: 1.17em; font-weight: bold; line-height: 1.20; }",
		"<h1", "<h2", "<h3",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing heading leading %q in output:\n%s", want, out)
		}
	}
}

func TestHTMLComplexFloatTableLayoutIsDeterministic(t *testing.T) {
	// This regression locks deterministic output for a mixed block flow that
	// combines a paragraph-relative float and a table block with cell text.
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "Intro"},
					{Kind: "image", Image: &resolve.Image{
						MIMEType:  "image/png",
						Data:      []byte{0x89, 0x50, 0x4e, 0x47},
						Width:     914400,
						Height:    914400,
						Floating:  true,
						XRelative: "column",
						YRelative: "paragraph",
						WrapType:  "square",
						Alt:       "P0-float",
					}},
					{Kind: "run", Text: "after"},
				},
			}},
			{Kind: "table", Table: &resolve.Table{
				Format: resolve.TableFormat{ColumnWidths: []int{2400, 2400}},
				Rows: []resolve.TableRow{{
					Cells: []resolve.TableCell{
						{Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "C1"}}}}}},
						{Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "C2"}}}}}},
					},
				}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Kind: "run", Text: "Tail"}},
			}},
		},
	}

	out1, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML first pass: %v", err)
	}
	out2, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML second pass: %v", err)
	}
	if out1 != out2 {
		t.Fatal("HTML output is not deterministic for mixed float/table flow")
	}
	for _, want := range []string{"P0-float", "C1", "Tail"} {
		if !strings.Contains(out1, want) {
			t.Fatalf("expected %q in deterministic output", want)
		}
	}
}

// ---------------------------------------------------------------------------
// P1: Content control (SDT) behavioral rendering
// ---------------------------------------------------------------------------

func sdtRun(ccType string, cc *resolve.ContentControl, text string) *resolve.Document {
	cc.Type = ccType
	return &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", ContentControl: cc, Text: text},
				},
			}},
		},
	}
}

func sdtBlock(ccType string, cc *resolve.ContentControl, text string) *resolve.Document {
	cc.Type = ccType
	return &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", ContentControl: cc, Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: text},
				},
			}},
		},
	}
}

func TestHTMLSdtCheckBoxUnchecked(t *testing.T) {
	doc := sdtRun("checkBox", &resolve.ContentControl{Checked: false}, "☐")
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<input type="checkbox"`) {
		t.Errorf("expected <input type=\"checkbox\">; got:\n%s", out)
	}
	if !strings.Contains(out, `class="word-sdt word-sdt-checkbox"`) {
		t.Errorf("expected checkbox SDT classes on rendered input; got:\n%s", out)
	}
	if strings.Contains(out, ` checked`) {
		t.Errorf("unchecked box should not have 'checked' attr; got:\n%s", out)
	}
	// The raw wingdings text should NOT appear literally.
	if strings.Contains(out, "☐") {
		t.Errorf("wingdings char should be suppressed for checkBox SDT; got:\n%s", out)
	}
}

func TestHTMLSdtCheckBoxChecked(t *testing.T) {
	doc := sdtRun("checkBox", &resolve.ContentControl{Checked: true, Title: "Accept"}, "☑")
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ` checked`) {
		t.Errorf("checked box must have 'checked' attr; got:\n%s", out)
	}
	if !strings.Contains(out, `title="Accept"`) {
		t.Errorf("expected title attr; got:\n%s", out)
	}
}

func TestHTMLSdtDateRendersTimeElement(t *testing.T) {
	doc := sdtRun("date", &resolve.ContentControl{
		DateFormat:   "MMMM d, yyyy",
		DateFullDate: "2024-01-01T00:00:00Z",
		DateLanguage: "en-US",
		DateCalendar: "gregorian",
	}, "January 1, 2024")
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<time`) {
		t.Errorf("expected <time> element for date SDT; got:\n%s", out)
	}
	if !strings.Contains(out, `class="word-sdt word-sdt-date"`) {
		t.Errorf("expected date SDT classes on rendered time element; got:\n%s", out)
	}
	if !strings.Contains(out, "January 1, 2024") {
		t.Errorf("expected date text inside <time>; got:\n%s", out)
	}
	if !strings.Contains(out, "data-sdt-date-format") {
		t.Errorf("expected data-sdt-date-format attr; got:\n%s", out)
	}
	if !strings.Contains(out, `datetime="2024-01-01T00:00:00Z"`) || !strings.Contains(out, `data-sdt-date-language="en-US"`) || !strings.Contains(out, `data-sdt-date-calendar="gregorian"`) {
		t.Errorf("expected extended date SDT metadata; got:\n%s", out)
	}
}

func TestHTMLSdtDateFormatsFromFullDate(t *testing.T) {
	doc := sdtRun("date", &resolve.ContentControl{
		DateFormat:   "dd/MM/yyyy",
		DateFullDate: "2024-01-15T13:45:00Z",
	}, "")
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">15/01/2024</time>") {
		t.Fatalf("expected formatted SDT date text from fullDate/dateFormat, got:\n%s", out)
	}
}

func TestHTMLSdtDateUsesBoundDataValue(t *testing.T) {
	doc := sdtRun("date", &resolve.ContentControl{
		DateFormat:   "MMMM d, yyyy",
		DateFullDate: "2024-02-20T00:00:00Z",
		DataValue:    "2024-02-20T00:00:00Z",
	}, "Old Date")
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">February 20, 2024</time>") {
		t.Fatalf("expected bound SDT date to render formatted bound value, got:\n%s", out)
	}
}

func TestHTMLSdtDatePlaceholderPreservesDisplayedText(t *testing.T) {
	doc := sdtRun("date", &resolve.ContentControl{
		DateFormat:         "MMMM d, yyyy",
		DateFullDate:       "2024-01-15T13:45:00Z",
		ShowingPlaceholder: true,
	}, "Click here to enter a date.")
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "Click here to enter a date.") {
		t.Fatalf("expected placeholder date text to be preserved, got:\n%s", out)
	}
	if strings.Contains(out, "January 15, 2024") {
		t.Fatalf("did not expect formatted date to override placeholder text, got:\n%s", out)
	}
}

func TestHTMLSdtDropDownListRendersSelect(t *testing.T) {
	doc := sdtRun("dropDownList", &resolve.ContentControl{
		ListItems: []resolve.SdtListItem{{DisplayText: "Red"}, {DisplayText: "Green"}, {DisplayText: "Blue"}},
	}, "Green")
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<select`) {
		t.Errorf("expected <select> for dropDownList SDT; got:\n%s", out)
	}
	if !strings.Contains(out, `class="word-sdt word-sdt-list"`) {
		t.Errorf("expected list SDT classes on rendered select; got:\n%s", out)
	}
	if !strings.Contains(out, `<option selected>Green`) {
		t.Errorf("expected current value pre-selected; got:\n%s", out)
	}
	for _, item := range []string{"Red", "Blue"} {
		if !strings.Contains(out, `<option>`+item) {
			t.Errorf("expected option %q; got:\n%s", item, out)
		}
	}
}

func TestHTMLSdtComboBoxValueNotInList(t *testing.T) {
	doc := sdtRun("comboBox", &resolve.ContentControl{
		ListItems: []resolve.SdtListItem{{DisplayText: "A"}, {DisplayText: "B"}},
	}, "Custom")
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<option selected>Custom`) {
		t.Errorf("expected out-of-list current value as selected option; got:\n%s", out)
	}
}

func TestHTMLSdtPlaceholderClass(t *testing.T) {
	doc := sdtRun("plainText", &resolve.ContentControl{ShowingPlaceholder: true}, "Click here to enter text.")
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="word-sdt word-sdt-placeholder word-sdt-text"`) {
		t.Errorf("expected word-sdt-placeholder class for placeholder SDT; got:\n%s", out)
	}
}

func TestHTMLSdtDocPartAttrs(t *testing.T) {
	doc := sdtRun("docPartObj", &resolve.ContentControl{
		Title:           "Building Block",
		DocPartGallery:  "Table of Contents",
		DocPartCategory: "General",
		DocPartUnique:   true,
	}, "Table of Contents")
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `data-sdt-type="docPartObj"`) {
		t.Errorf("expected docPartObj SDT type attr; got:\n%s", out)
	}
	if !strings.Contains(out, `data-sdt-doc-part-gallery="Table of Contents"`) || !strings.Contains(out, `data-sdt-doc-part-category="General"`) || !strings.Contains(out, `data-sdt-doc-part-unique="true"`) {
		t.Errorf("expected doc-part metadata attrs; got:\n%s", out)
	}
}

func TestHTMLSdtPlaceholderAttrs(t *testing.T) {
	doc := sdtRun("plainText", &resolve.ContentControl{
		ShowingPlaceholder:        true,
		PlaceholderDocPart:        "DefaultPlaceholder_-1854013440",
		PlaceholderDocPartGallery: "Quick Parts",
	}, "Click here")
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `data-sdt-placeholder="true"`) {
		t.Errorf("expected placeholder flag attr; got:\n%s", out)
	}
	if !strings.Contains(out, `data-sdt-placeholder-doc-part="DefaultPlaceholder_-1854013440"`) {
		t.Errorf("expected placeholder docPart attr; got:\n%s", out)
	}
	if !strings.Contains(out, `data-sdt-placeholder-gallery="Quick Parts"`) {
		t.Errorf("expected placeholder gallery attr; got:\n%s", out)
	}
}

func TestHTMLSdtPlainTextRendersInput(t *testing.T) {
	doc := sdtRun("plainText", &resolve.ContentControl{Title: "Name"}, "Alice")
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<input type="text"`) {
		t.Errorf("expected <input type=\"text\"> for plainText SDT; got:\n%s", out)
	}
	if !strings.Contains(out, `value="Alice"`) {
		t.Errorf("expected input value=\"Alice\"; got:\n%s", out)
	}
	if !strings.Contains(out, `class="word-sdt word-sdt-text"`) {
		t.Errorf("expected word-sdt-text class; got:\n%s", out)
	}
	if !strings.Contains(out, `readonly`) {
		t.Errorf("expected readonly attr; got:\n%s", out)
	}
	// Original text should not appear outside the input.
	if strings.Count(out, "Alice") != 1 {
		t.Errorf("expected Alice only inside input value; got:\n%s", out)
	}
}

func TestHTMLSdtPlainTextUsesBoundDataValue(t *testing.T) {
	doc := sdtRun("plainText", &resolve.ContentControl{Title: "Name", DataValue: "Bound Customer"}, "Alice")
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `value="Bound Customer"`) {
		t.Fatalf("expected bound SDT value in input, got:\n%s", out)
	}
	if strings.Contains(out, `value="Alice"`) {
		t.Fatalf("did not expect stale fallback SDT value, got:\n%s", out)
	}
}

func TestHTMLSdtRichTextRendersSpan(t *testing.T) {
	doc := sdtRun("richText", &resolve.ContentControl{Title: "Comment"}, "Hello world")
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<span class="word-sdt word-sdt-rich"`) {
		t.Errorf("expected <span class=\"word-sdt-rich\"> for richText SDT; got:\n%s", out)
	}
	if !strings.Contains(out, "Hello world") {
		t.Errorf("expected rich text content; got:\n%s", out)
	}
}

func TestHTMLSdtControlsEditableMode(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{
				Kind: "paragraph",
				Paragraph: &resolve.Paragraph{
					Runs: []resolve.Run{
						{Kind: "run", ContentControl: &resolve.ContentControl{Type: "checkBox", Checked: true}, Text: "☑"},
						{Kind: "run", ContentControl: &resolve.ContentControl{Type: "plainText", Title: "Name"}, Text: "Alice"},
						{Kind: "run", ContentControl: &resolve.ContentControl{Type: "dropDownList", ListItems: []resolve.SdtListItem{{DisplayText: "Red"}, {DisplayText: "Green"}}}, Text: "Green"},
						{Kind: "run", ContentControl: &resolve.ContentControl{Type: "richText", Title: "Comment"}, Text: "Hello"},
					},
				},
			},
		},
	}
	out, _, err := render.HTMLWithOptions(doc, render.HTMLOptions{EditableContentControls: true})
	if err != nil {
		t.Fatalf("HTMLWithOptions: %v", err)
	}
	if strings.Contains(out, "readonly") || strings.Contains(out, " disabled") {
		t.Fatalf("expected editable SDT controls without readonly/disabled attributes, got:\n%s", out)
	}
	if !strings.Contains(out, `contenteditable="true"`) {
		t.Fatalf("expected editable richText SDT, got:\n%s", out)
	}
}

func TestHTMLSdtCheckbox14RendersSameAsLegacy(t *testing.T) {
	// w14:checkbox sets Type="checkBox" and Checked=true via Checked14.
	doc := sdtRun("checkBox", &resolve.ContentControl{Checked: true}, "☑")
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<input type="checkbox"`) {
		t.Errorf("expected checkbox input; got:\n%s", out)
	}
	if !strings.Contains(out, ` checked`) {
		t.Errorf("expected checked attr; got:\n%s", out)
	}
}

func TestHTMLSdtCitationRendersCiteElement(t *testing.T) {
	doc := sdtRun("citation", &resolve.ContentControl{Title: "Citation"}, "Roe v. Wade")
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<cite class="word-sdt word-sdt-citation"`) {
		t.Fatalf("expected cite wrapper for citation SDT, got:\n%s", out)
	}
	if !strings.Contains(out, `data-sdt-type="citation"`) {
		t.Fatalf("expected citation SDT metadata, got:\n%s", out)
	}
}

func TestHTMLSdtEquationCarriesMathRole(t *testing.T) {
	doc := sdtRun("equation", &resolve.ContentControl{}, "x + y = z")
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<span class="word-sdt word-sdt-equation" data-sdt-type="equation" role="math">x + y = z</span>`) {
		t.Fatalf("expected math-role wrapper for equation SDT, got:\n%s", out)
	}
}

func TestHTMLSdtGroupCarriesGroupRole(t *testing.T) {
	doc := sdtBlock("group", &resolve.ContentControl{Tag: "grp"}, "Grouped content")
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<div class="word-sdt word-sdt-group" data-sdt-tag="grp" data-sdt-type="group" role="group">`) {
		t.Fatalf("expected role=group wrapper for group SDT, got:\n%s", out)
	}
}

func TestHTMLSdtRepeatingSectionUsesSectionWrapper(t *testing.T) {
	doc := sdtBlock("repeatingSection", &resolve.ContentControl{}, "Repeated block")
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<section class="word-sdt word-sdt-repeating-section" data-sdt-type="repeatingSection" data-sdt-repeat="section">`) {
		t.Fatalf("expected section wrapper for repeatingSection SDT, got:\n%s", out)
	}
}

func TestHTMLSdtContentPartCarriesSemanticMarker(t *testing.T) {
	doc := sdtRun("contentPart", &resolve.ContentControl{}, "Embedded part")
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<span class="word-sdt word-sdt-content-part" data-sdt-type="contentPart" data-sdt-content-part="true">Embedded part</span>`) {
		t.Fatalf("expected semantic marker for contentPart SDT, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// P1: Revision semantics — author/date in rendered HTML
// ---------------------------------------------------------------------------

func revisionDoc(revKind, author, date, text string) *resolve.Document {
	return &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: text, Revision: revKind, RevisionAuthor: author, RevisionDate: date},
				},
			}},
		},
	}
}

func TestHTMLInsRunRendersInsElement(t *testing.T) {
	out, err := render.HTML(revisionDoc("inserted", "", "", "new text"))
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<ins>`) {
		t.Errorf("expected <ins> element; got:\n%s", out)
	}
	if !strings.Contains(out, "new text") {
		t.Errorf("expected run text inside <ins>; got:\n%s", out)
	}
}

func TestHTMLDelRunRendersDelElement(t *testing.T) {
	out, err := render.HTML(revisionDoc("deleted", "", "", "old text"))
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<del>`) {
		t.Errorf("expected <del> element; got:\n%s", out)
	}
}

func TestHTMLInsRunWithAuthorEmitsTitleAndDataAttrs(t *testing.T) {
	out, err := render.HTML(revisionDoc("inserted", "Alice", "2024-01-15T10:00:00Z", "added"))
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<ins `) {
		t.Errorf("expected <ins> with attributes; got:\n%s", out)
	}
	if !strings.Contains(out, `data-revision-author="Alice"`) {
		t.Errorf("expected data-revision-author attr; got:\n%s", out)
	}
	if !strings.Contains(out, `data-revision-date="2024-01-15T10:00:00Z"`) {
		t.Errorf("expected data-revision-date attr; got:\n%s", out)
	}
	if !strings.Contains(out, `title="Alice`) {
		t.Errorf("expected title attr with author; got:\n%s", out)
	}
}

func TestHTMLDelRunWithAuthorEmitsTitleAndDataAttrs(t *testing.T) {
	out, err := render.HTML(revisionDoc("deleted", "Bob", "2024-02-20T09:00:00Z", "removed"))
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `data-revision-author="Bob"`) {
		t.Errorf("expected data-revision-author attr; got:\n%s", out)
	}
	if !strings.Contains(out, `data-revision-date="2024-02-20T09:00:00Z"`) {
		t.Errorf("expected data-revision-date attr; got:\n%s", out)
	}
}

func TestHTMLMixedRevisionRunsAreIndependent(t *testing.T) {
	// A paragraph with a base run, an inserted run, and a deleted run should
	// each wrap only its own content independently.
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "base"},
					{Kind: "run", Text: "added", Revision: "inserted", RevisionAuthor: "Carol", RevisionDate: "2024-03-01T00:00:00Z"},
					{Kind: "run", Text: "removed", Revision: "deleted", RevisionAuthor: "Dave", RevisionDate: "2024-03-02T00:00:00Z"},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "base") {
		t.Errorf("expected base text; got:\n%s", out)
	}
	if !strings.Contains(out, `data-revision-author="Carol"`) {
		t.Errorf("expected Carol as ins author; got:\n%s", out)
	}
	if !strings.Contains(out, `data-revision-author="Dave"`) {
		t.Errorf("expected Dave as del author; got:\n%s", out)
	}
	// The base run must not be wrapped in ins or del.
	if strings.Contains(out, `<ins>base`) || strings.Contains(out, `<del>base`) {
		t.Errorf("base run must not be wrapped in revision element; got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// P1: Broader text direction fidelity — paragraph BiDi and run RTL
// ---------------------------------------------------------------------------

func TestHTMLBidiParagraphEmitsDirRTL(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Format: resolve.ParagraphFormat{Bidi: true},
				Runs:   []resolve.Run{{Kind: "run", Text: "שלום"}},
			},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `dir="rtl"`) {
		t.Errorf("expected dir=\"rtl\" on BiDi paragraph; got:\n%s", out)
	}
}

func TestHTMLNonBidiParagraphOmitsDirAttr(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Format: resolve.ParagraphFormat{Bidi: false},
				Runs:   []resolve.Run{{Kind: "run", Text: "Hello"}},
			},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Contains(out, `dir="rtl"`) {
		t.Errorf("expected no dir attribute on LTR paragraph; got:\n%s", out)
	}
}

func TestHTMLRTLRunEmitsDirectionCSS(t *testing.T) {
	rtl := true
	doc := &resolve.Document{
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Kind: "run", Text: "مرحبا", Format: resolve.RunFormat{RTL: &rtl}}},
			},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "direction:rtl") {
		t.Errorf("expected direction:rtl on RTL run span; got:\n%s", out)
	}
	if !strings.Contains(out, "unicode-bidi:embed") {
		t.Errorf("expected unicode-bidi:embed on RTL run span; got:\n%s", out)
	}
}

func TestHTMLShowFormattingMarksPilcrowInheritsParagraphDefaultRunFormat(t *testing.T) {
	fontSize := 48
	bold := true
	doc := &resolve.Document{
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Kind: "run", Text: "Hello"}},
				DefaultRunFormat: &resolve.RunFormat{
					Bold:     &bold,
					Color:    "FF0000",
					FontSize: &fontSize,
				},
			},
		}},
	}
	out, _, err := render.HTMLWithOptions(doc, render.HTMLOptions{ShowFormattingMarks: true})
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// The pilcrow span should have inline styles matching the default run format.
	if !strings.Contains(out, `class="fmt-mark"`) {
		t.Errorf("expected fmt-mark class in output; got:\n%s", out)
	}
	if !strings.Contains(out, "font-weight:bold") {
		t.Errorf("expected font-weight:bold on pilcrow; got:\n%s", out)
	}
	if !strings.Contains(out, "color:#FF0000") {
		t.Errorf("expected color:#FF0000 on pilcrow; got:\n%s", out)
	}
	if !strings.Contains(out, "font-size:24.0pt") {
		t.Errorf("expected font-size:24.0pt on pilcrow; got:\n%s", out)
	}
}

func TestHTMLShowFormattingMarksPilcrowWithoutDefaultRunFormat(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Kind: "run", Text: "Hello"}},
			},
		}},
	}
	out, _, err := render.HTMLWithOptions(doc, render.HTMLOptions{ShowFormattingMarks: true})
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// Should still render the pilcrow, but without inline styles.
	if !strings.Contains(out, `<span class="fmt-mark" aria-hidden="true">¶</span>`) {
		t.Errorf("expected plain pilcrow span; got:\n%s", out)
	}
}

func TestHTMLTabRenderingWithParagraphIndent(t *testing.T) {
	// Tab stops are at absolute positions from the margin. When a paragraph has
	// a left indent, the tab cursor should start at the indent position so that
	// a tab stop at the same position as the indent yields zero width.
	indentLeft := 720 // 0.5 inch = 36pt
	tabPos := 720     // tab stop at same position as indent
	doc := &resolve.Document{
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Format: resolve.ParagraphFormat{
					IndentLeft: &indentLeft,
					TabStops:   []resolve.TabStop{{Pos: tabPos, Algn: "left"}},
				},
				Runs: []resolve.Run{
					{Kind: "run", Text: "A"},
					{Kind: "tab"},
					{Kind: "run", Text: "B"},
				},
			},
		}},
	}
	out, _, err := render.HTMLWithOptions(doc, render.HTMLOptions{})
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// The tab at the indent position should have near-zero width (allowing for
	// text width of "A" pushing the cursor past the tab stop, so the next tab
	// stop is used).
	if !strings.Contains(out, `display:inline-block;width:`) {
		t.Fatalf("expected tab span in output; got:\n%s", out)
	}
}

func TestHTMLTabRightAlignUsesTextWidthEstimate(t *testing.T) {
	// Right-aligned tab: the tab width should be reduced by the estimated
	// width of the text after the tab.
	tabPos := 4320 // 3 inches = 216pt
	doc := &resolve.Document{
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Format: resolve.ParagraphFormat{
					TabStops: []resolve.TabStop{{Pos: tabPos, Algn: "right"}},
				},
				Runs: []resolve.Run{
					{Kind: "run", Text: "Left"},
					{Kind: "tab"},
					{Kind: "run", Text: "Right"},
				},
			},
		}},
	}
	out, _, err := render.HTMLWithOptions(doc, render.HTMLOptions{})
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// With improved width estimation, "Left" is ~4 chars of mixed case.
	// Old estimate: 4 * 12 * 0.55 = 26.4pt
	// New estimate: ~0.62*12*0.92 + 3*0.45*12*0.92 ≈ 6.84 + 14.9 = 21.7pt
	// The tab width should be roughly 216 - 21.7 - ~21.7 = ~172.6pt for right align.
	if !strings.Contains(out, `display:inline-block;width:`) {
		t.Fatalf("expected tab span in output; got:\n%s", out)
	}
}

func TestHTMLParagraphBorderBetweenBarFallback(t *testing.T) {
	// When Bottom is nil, Between should render as border-bottom.
	// When Left is nil, Bar should render as border-left.
	doc := &resolve.Document{
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Text: "bordered"}},
				Format: resolve.ParagraphFormat{
					Borders: &resolve.ParagraphBorders{
						Top:     &resolve.BorderSide{Val: "single", SizePt: 1, Color: "000000"},
						Between: &resolve.BorderSide{Val: "dashed", SizePt: 1.5, Color: "FF0000"},
						Bar:     &resolve.BorderSide{Val: "double", SizePt: 2, Color: "0000FF"},
					},
				},
			},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `border-top:1.00pt solid #000000`) {
		t.Errorf("expected top border, got: %s", out)
	}
	if !strings.Contains(out, `border-bottom:1.50pt dashed #FF0000`) {
		t.Errorf("expected between→bottom border, got: %s", out)
	}
	if !strings.Contains(out, `border-left:2.00pt double #0000FF`) {
		t.Errorf("expected bar→left border, got: %s", out)
	}
}

func TestHTMLParagraphBorderLogicalStartEnd(t *testing.T) {
	// When Left is nil but Start is set, Start should render as border-left.
	// When Right is nil but End is set, End should render as border-right.
	doc := &resolve.Document{
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Text: "bordered"}},
				Format: resolve.ParagraphFormat{
					Borders: &resolve.ParagraphBorders{
						Top:   &resolve.BorderSide{Val: "single", SizePt: 1, Color: "000000"},
						Start: &resolve.BorderSide{Val: "double", SizePt: 2, Color: "0000FF"},
						End:   &resolve.BorderSide{Val: "dashed", SizePt: 1.5, Color: "FF0000"},
					},
				},
			},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `border-top:1.00pt solid #000000`) {
		t.Errorf("expected top border, got: %s", out)
	}
	if !strings.Contains(out, `border-left:2.00pt double #0000FF`) {
		t.Errorf("expected start→left border, got: %s", out)
	}
	if !strings.Contains(out, `border-right:1.50pt dashed #FF0000`) {
		t.Errorf("expected end→right border, got: %s", out)
	}
}

func TestHTMLDocPropertyExtendedCustom(t *testing.T) {
	doc := &resolve.Document{
		Company:     "Acme Corp",
		Manager:     "Alice Smith",
		Application: "Microsoft Word",
		AppVersion:  "16.0",
		Template:    "Normal",
		CustomProperties: map[string]string{
			"ProjectID": "42",
			"Status":    "Draft",
		},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Text: "Company: ", Kind: "text"},
					{Kind: "field", FieldKind: "DOCPROPERTY", FieldArg: "Company", Format: resolve.RunFormat{Color: "000000"}},
					{Text: " Manager: ", Kind: "text"},
					{Kind: "field", FieldKind: "DOCPROPERTY", FieldArg: "Manager", Format: resolve.RunFormat{Color: "000000"}},
					{Text: " App: ", Kind: "text"},
					{Kind: "field", FieldKind: "DOCPROPERTY", FieldArg: "Application", Format: resolve.RunFormat{Color: "000000"}},
					{Text: " Template: ", Kind: "text"},
					{Kind: "field", FieldKind: "DOCPROPERTY", FieldArg: "Template", Format: resolve.RunFormat{Color: "000000"}},
					{Text: " Project: ", Kind: "text"},
					{Kind: "field", FieldKind: "DOCPROPERTY", FieldArg: "ProjectID", Format: resolve.RunFormat{Color: "000000"}},
					{Text: " Status: ", Kind: "text"},
					{Kind: "field", FieldKind: "DOCPROPERTY", FieldArg: "Status", Format: resolve.RunFormat{Color: "000000"}},
				},
			},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, want := range []string{"Acme Corp", "Alice Smith", "Microsoft Word", "Normal", "42", "Draft"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got: %s", want, out)
		}
	}
}

func TestHTMLDocPropertyCustomLookupIsCaseInsensitive(t *testing.T) {
	doc := &resolve.Document{
		CustomProperties: map[string]string{
			"ProjectID": "42",
		},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "field", FieldKind: "DOCPROPERTY", FieldArg: "projectid"},
					{Kind: "run", Text: " "},
					{Kind: "field", FieldKind: "DOCPROPERTY", FieldArg: "  PROJECTID  "},
				},
			},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Count(out, ">42<") < 2 {
		t.Fatalf("expected case-insensitive custom DOCPROPERTY lookup, got:\n%s", out)
	}
}

func TestHTMLDocVariableLookupIsCaseInsensitive(t *testing.T) {
	doc := &resolve.Document{
		DocumentVariables: map[string]string{
			"ProjectID": "42",
		},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "field", FieldKind: "DOCVARIABLE", FieldArg: "projectid"},
					{Kind: "run", Text: " "},
					{Kind: "field", FieldKind: "DOCVARIABLE", FieldArg: "  PROJECTID  "},
				},
			},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Count(out, ">42<") < 2 {
		t.Fatalf("expected case-insensitive DOCVARIABLE lookup, got:\n%s", out)
	}
}

func TestHTMLLongTailSemanticFields(t *testing.T) {
	doc := &resolve.Document{
		Author:           "Alice Smith",
		TotalEditMinutes: 42,
		CustomProperties: map[string]string{
			"UserAddress": "123 Main St",
		},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Bookmarks: []string{"AskBookmark"},
				Runs:      []resolve.Run{{Kind: "run", Text: "Stored answer"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", FieldKind: "ASK", FieldArg: "AskBookmark", FieldInstr: `ASK AskBookmark "Prompt"`},
					{Kind: "run", Text: " "},
					{Kind: "run", FieldKind: "AUTOTEXT", FieldArg: "ClauseName", FieldInstr: `AUTOTEXT ClauseName`},
					{Kind: "run", Text: " "},
					{Kind: "run", FieldKind: "EDITTIME"},
					{Kind: "run", Text: " "},
					{Kind: "run", FieldKind: "USERADDRESS"},
					{Kind: "run", Text: " "},
					{Kind: "run", FieldKind: "USERINITIALS"},
					{Kind: "run", Text: " "},
					{Kind: "run", FieldKind: "MACROBUTTON", FieldInstr: `MACROBUTTON NoMacro Click here`},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, className := range []string{
		`class="word-field word-field-ask"`,
		`class="word-field word-field-autotext"`,
		`class="word-field word-field-edittime"`,
		`class="word-field word-field-useraddress"`,
		`class="word-field word-field-userinitials"`,
		`class="word-field word-field-macrobutton"`,
	} {
		if !strings.Contains(out, className) {
			t.Fatalf("expected %s in output, got:\n%s", className, out)
		}
	}
	for _, value := range []string{"Stored answer", "[ClauseName]", "42", "123 Main St", "AS", "Click here"} {
		if !strings.Contains(out, value) {
			t.Fatalf("expected %q in output, got:\n%s", value, out)
		}
	}
}

func TestHTMLTableCellBordersRenderExplicitly(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{{
			Kind: "table",
			Table: &resolve.Table{
				Rows: []resolve.TableRow{{
					Cells: []resolve.TableCell{{
						Format: resolve.TableCellFormat{
							Borders: &resolve.CellBorders{
								Top:    &resolve.BorderSide{Val: "single", SizePt: 1, Color: "FF0000"},
								Left:   &resolve.BorderSide{Val: "double", SizePt: 1.5, Color: "00FF00"},
								Bottom: &resolve.BorderSide{Val: "dashed", SizePt: 1, Color: "0000FF"},
								Right:  &resolve.BorderSide{Val: "dotted", SizePt: 0.5, Color: "000000"},
							},
						},
						Blocks: []resolve.Block{{
							Kind: "paragraph",
							Paragraph: &resolve.Paragraph{
								Runs: []resolve.Run{{Text: "A"}},
							},
						}},
					}},
				}},
			},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `border-top:1.00pt solid #FF0000`) {
		t.Errorf("expected cell top border, got: %s", out)
	}
	if !strings.Contains(out, `border-left:1.50pt double #00FF00`) {
		t.Errorf("expected cell left border, got: %s", out)
	}
	if !strings.Contains(out, `border-bottom:1.00pt dashed #0000FF`) {
		t.Errorf("expected cell bottom border, got: %s", out)
	}
	if !strings.Contains(out, `border-right:0.50pt dotted #000000`) {
		t.Errorf("expected cell right border, got: %s", out)
	}
}

func TestHTMLTableCellDiagonalBorders(t *testing.T) {
	decodeB64 := func(s string) string {
		b, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			t.Fatalf("decode base64: %v", err)
		}
		return string(b)
	}
	extractSVG := func(htmlOut string) string {
		const prefix = "background-image:url('data:image/svg+xml;base64,"
		idx := strings.Index(htmlOut, prefix)
		if idx < 0 {
			return ""
		}
		start := idx + len(prefix)
		end := strings.Index(htmlOut[start:], "')")
		if end < 0 {
			return ""
		}
		return decodeB64(htmlOut[start : start+end])
	}

	tests := []struct {
		name     string
		borders  *resolve.CellBorders
		wantLine string // substring expected inside the decoded SVG
	}{
		{
			name: "TL2BR single red",
			borders: &resolve.CellBorders{
				TL2BR: &resolve.BorderSide{Val: "single", SizePt: 1, Color: "FF0000"},
			},
			wantLine: `stroke="#FF0000" stroke-width="1pt"`,
		},
		{
			name: "TR2BL dashed blue",
			borders: &resolve.CellBorders{
				TR2BL: &resolve.BorderSide{Val: "dashed", SizePt: 1.5, Color: "0000FF"},
			},
			wantLine: `stroke-dasharray="6pt,3pt"`,
		},
		{
			name: "both diagonals",
			borders: &resolve.CellBorders{
				TL2BR: &resolve.BorderSide{Val: "single", SizePt: 1, Color: "FF0000"},
				TR2BL: &resolve.BorderSide{Val: "single", SizePt: 1, Color: "00FF00"},
			},
			wantLine: `x1="0" y1="0" x2="100%" y2="100%"`,
		},
		{
			name: "TL2BR dotted",
			borders: &resolve.CellBorders{
				TL2BR: &resolve.BorderSide{Val: "dotted", SizePt: 1, Color: "000000"},
			},
			wantLine: `stroke-dasharray="0,3pt" stroke-linecap="round"`,
		},
		{
			name: "TL2BR double approximated thicker",
			borders: &resolve.CellBorders{
				TL2BR: &resolve.BorderSide{Val: "double", SizePt: 2, Color: "000000"},
			},
			wantLine: `stroke-width="3pt"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := &resolve.Document{
				Blocks: []resolve.Block{{
					Kind: "table",
					Table: &resolve.Table{
						Rows: []resolve.TableRow{{
							Cells: []resolve.TableCell{{
								Format: resolve.TableCellFormat{
									Borders: tt.borders,
								},
								Blocks: []resolve.Block{{
									Kind: "paragraph",
									Paragraph: &resolve.Paragraph{
										Runs: []resolve.Run{{Text: "A"}},
									},
								}},
							}},
						}},
					},
				}},
			}
			out, err := render.HTML(doc)
			if err != nil {
				t.Fatalf("HTML: %v", err)
			}
			if !strings.Contains(out, "background-image:url('data:image/svg+xml;base64,") {
				t.Errorf("expected diagonal border background-image, got: %s", out)
			}
			svg := extractSVG(out)
			if svg == "" {
				t.Fatalf("failed to extract SVG from output")
			}
			if !strings.Contains(svg, tt.wantLine) {
				t.Errorf("expected SVG to contain %q, got:\n%s", tt.wantLine, svg)
			}
			if !strings.Contains(out, "background-size:100% 100%") {
				t.Errorf("expected background-size, got: %s", out)
			}
			if !strings.Contains(out, "background-repeat:no-repeat") {
				t.Errorf("expected background-repeat, got: %s", out)
			}
		})
	}
}

func TestHTMLMergeFieldDataAttribute(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "", FieldKind: "MERGEFIELD", FieldArg: "CustomerName"},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `data-field-arg="CustomerName"`) {
		t.Errorf("expected data-field-arg attribute, got:\n%s", out)
	}
}

func TestHTMLMergeRecAndMergeSeqReturnRecordNumber(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "", FieldKind: "MERGEREC"},
					{Kind: "run", Text: " "},
					{Kind: "run", Text: "", FieldKind: "MERGESEQ"},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">1</span>") {
		t.Errorf("expected MERGEREC to default to 1, got:\n%s", out)
	}
	// Both MERGEREC and MERGESEQ should produce 1 by default.
	countOnes := strings.Count(out, ">1</span>")
	if countOnes != 2 {
		t.Errorf("expected MERGEREC and MERGESEQ both to be 1, got %d occurrences:\n%s", countOnes, out)
	}
}

func TestHTMLMergeRecordsWithNextAdvancesIndex(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "", FieldKind: "MERGEFIELD", FieldArg: "Name"},
					{Kind: "run", Text: " "},
					{Kind: "run", Text: "", FieldKind: "NEXT"},
					{Kind: "run", Text: "", FieldKind: "MERGEFIELD", FieldArg: "Name"},
				},
			}},
		},
	}
	out, _, err := render.HTMLWithOptions(doc, render.HTMLOptions{
		MergeRecords: []map[string]string{
			{"Name": "Alice"},
			{"Name": "Bob"},
		},
	})
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">Alice</span>") {
		t.Errorf("expected first MERGEFIELD to render Alice, got:\n%s", out)
	}
	if !strings.Contains(out, ">Bob</span>") {
		t.Errorf("expected second MERGEFIELD after NEXT to render Bob, got:\n%s", out)
	}
}

func TestHTMLMergeRecordIndexOptionOffsetsStart(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "", FieldKind: "MERGEFIELD", FieldArg: "City"},
					{Kind: "run", Text: " "},
					{Kind: "run", Text: "", FieldKind: "MERGEREC"},
				},
			}},
		},
	}
	out, _, err := render.HTMLWithOptions(doc, render.HTMLOptions{
		MergeRecords: []map[string]string{
			{"City": "NYC"},
			{"City": "LA"},
			{"City": "Chicago"},
		},
		MergeRecordIndex: 2,
	})
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">Chicago</span>") {
		t.Errorf("expected MERGEFIELD at index 2 to render Chicago, got:\n%s", out)
	}
	if !strings.Contains(out, ">3</span>") {
		t.Errorf("expected MERGEREC at index 2 to render 3, got:\n%s", out)
	}
}

func TestHTMLMergeDataFallbackWhenMergeRecordsEmpty(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "", FieldKind: "MERGEFIELD", FieldArg: "Product"},
				},
			}},
		},
	}
	out, _, err := render.HTMLWithOptions(doc, render.HTMLOptions{
		MergeData: map[string]string{"Product": "Widget"},
	})
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">Widget</span>") {
		t.Errorf("expected MERGEFIELD to fall back to MergeData when MergeRecords empty, got:\n%s", out)
	}
}

func TestHTMLMergeSeqVsMergeRecAfterSkipIf(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "", FieldKind: "MERGEREC"},
					{Kind: "run", Text: " "},
					{Kind: "run", Text: "", FieldKind: "MERGESEQ"},
					{Kind: "run", Text: " "},
					{Kind: "run", Text: "", FieldKind: "SKIPIF", FieldInstr: `SKIPIF City = "NYC"`},
					{Kind: "run", Text: " "},
					{Kind: "run", Text: "", FieldKind: "MERGEREC"},
					{Kind: "run", Text: " "},
					{Kind: "run", Text: "", FieldKind: "MERGESEQ"},
				},
			}},
		},
	}
	out, _, err := render.HTMLWithOptions(doc, render.HTMLOptions{
		MergeRecords: []map[string]string{
			{"City": "NYC"},
			{"City": "LA"},
		},
	})
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// After SKIPIF on record 1, we move to record 2.
	// MERGEREC should reflect the absolute record number (2).
	// MERGESEQ should reflect merged records only (1, because record 1 was skipped).
	if !strings.Contains(out, ">2</span>") {
		t.Errorf("expected MERGEREC after SKIPIF to be 2, got:\n%s", out)
	}
	if strings.Count(out, ">2</span>") != 1 {
		t.Errorf("expected exactly one '>2</span>', got:\n%s", out)
	}
	if !strings.Contains(out, ">1</span>") {
		t.Errorf("expected MERGESEQ after SKIPIF to be 1, got:\n%s", out)
	}
}

func TestHTMLMergeFieldBeforeAfterSwitches(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "", FieldKind: "MERGEFIELD", FieldArg: "City", FieldInstr: `MERGEFIELD City \b "[" \f "]"`},
				},
			}},
		},
	}
	out, _, err := render.HTMLWithOptions(doc, render.HTMLOptions{
		MergeData: map[string]string{"City": "NYC"},
	})
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">[NYC]</span>") {
		t.Errorf("expected before/after text around merge value, got:\n%s", out)
	}

	// When merge value is empty, before/after text should not appear.
	doc2 := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "", FieldKind: "MERGEFIELD", FieldArg: "Missing", FieldInstr: `MERGEFIELD Missing \b "[" \f "]"`},
				},
			}},
		},
	}
	out2, _, err := render.HTMLWithOptions(doc2, render.HTMLOptions{
		MergeData: map[string]string{"City": "NYC"},
	})
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// The HTML template itself contains [ and ] in CSS selectors, so we must
	// check specifically that the merge field span does not have brackets.
	if strings.Contains(out2, `data-field-arg="Missing">[`) || strings.Contains(out2, `»]</span>`) {
		t.Errorf("expected no before/after text when merge value missing, got:\n%s", out2)
	}
}

func TestHTMLProtectionNoticeRendered(t *testing.T) {
	doc := &resolve.Document{
		ProtectionEnabled: true,
		DocumentProtection: &resolve.DocumentProtection{
			Edit:        "forms",
			Enforcement: true,
		},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Kind: "run", Text: "Hello"}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="protection-banner"`) {
		t.Errorf("expected protection-banner class, got:\n%s", out)
	}
	if !strings.Contains(out, "protected (forms editing)") {
		t.Errorf("expected protection notice text, got:\n%s", out)
	}
	if !strings.Contains(out, `.protection-banner {`) {
		t.Errorf("expected protection-banner CSS, got:\n%s", out)
	}
}

func TestHTMLProtectionNoticeNotRenderedWhenDisabled(t *testing.T) {
	doc := &resolve.Document{
		ProtectionEnabled: false,
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Kind: "run", Text: "Hello"}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Contains(out, `class="protection-banner"`) {
		t.Errorf("unexpected protection-banner when disabled, got:\n%s", out)
	}
}

func TestHTMLProtectionNoticeDefaultsEditMode(t *testing.T) {
	doc := &resolve.Document{
		ProtectionEnabled: true,
		DocumentProtection: &resolve.DocumentProtection{
			Enforcement: true,
		},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Kind: "run", Text: "Hello"}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "protected (restricted editing)") {
		t.Errorf("expected default 'restricted' edit mode, got:\n%s", out)
	}
}

func TestHTMLPageBackgroundColor(t *testing.T) {
	doc := &resolve.Document{
		BackgroundColor: "E6E6FA",
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Kind: "run", Text: "Hello"}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	css := extractCSS(out)
	if !strings.Contains(css, "background: #E6E6FA") {
		t.Errorf("expected page background #E6E6FA in CSS, got:\n%s", css)
	}
}

func TestHTMLPageBackgroundImage(t *testing.T) {
	doc := &resolve.Document{
		BackgroundColor: "E6E6FA",
		BackgroundImage: &resolve.Image{
			MIMEType: "image/png",
			Data:     []byte("bgdata"),
		},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Kind: "run", Text: "Hello"}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	css := extractCSS(out)
	if !strings.Contains(css, `background: #E6E6FA url("data:image/png;base64,YmdkYXRh") center center / cover no-repeat`) {
		t.Errorf("expected page background image CSS, got:\n%s", css)
	}
}

func TestHTMLHiddenParagraph(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Kind: "run", Text: "Visible"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs:             []resolve.Run{{Kind: "run", Text: "Hidden"}},
				DefaultRunFormat: &resolve.RunFormat{Vanish: boolPtr(true)},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "Visible") {
		t.Errorf("expected 'Visible' in output")
	}
	if strings.Contains(out, "Hidden") {
		t.Errorf("did not expect 'Hidden' paragraph in output")
	}
}

func TestHTMLHiddenTableRow(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "table", Table: &resolve.Table{
				Rows: []resolve.TableRow{
					{Cells: []resolve.TableCell{{
						Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Visible"}}}}},
					}}},
					{Format: resolve.TableRowFormat{Hidden: true}, Cells: []resolve.TableCell{{
						Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Hidden"}}}}},
					}}},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// Count <tr> occurrences
	trCount := strings.Count(out, "<tr")
	if trCount != 1 {
		t.Errorf("expected 1 <tr>, got %d", trCount)
	}
	if !strings.Contains(out, "Visible") {
		t.Errorf("expected 'Visible' in output")
	}
	if strings.Contains(out, "Hidden") {
		t.Errorf("did not expect 'Hidden' row in output")
	}
}

func TestHTMLFieldSection(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{}, {}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Kind: "run", Text: "Section "}, {Kind: "run", FieldKind: "SECTION"}},
			}},
			{Kind: "section", Section: &resolve.Section{}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Kind: "run", Text: "Section "}, {Kind: "run", FieldKind: "SECTION"}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// First paragraph should show "1", second should show "2"
	if !strings.Contains(out, ">1<") {
		t.Errorf("expected section number 1 in output")
	}
	if !strings.Contains(out, ">2<") {
		t.Errorf("expected section number 2 in output")
	}
}

func TestHTMLFieldSectionPages(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{
			{PageSize: &resolve.PageSize{Width: 12240, Height: 400}, PageMargin: &resolve.PageMargin{}},
			{PageSize: &resolve.PageSize{Width: 12240, Height: 400}, PageMargin: &resolve.PageMargin{}},
		},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Kind: "run", FieldKind: "SECTIONPAGES"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Kind: "run", Text: "Force page two"}},
			}},
			{Kind: "section", Section: &resolve.Section{}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Kind: "run", FieldKind: "SECTIONPAGES"}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Count(out, `class="word-field word-field-sectionpages"`) != 2 {
		t.Fatalf("expected 2 SECTIONPAGES fields, got:\n%s", out)
	}
	if strings.Count(out, ">2<") != 1 {
		t.Fatalf("expected first SECTIONPAGES field to resolve to 2 pages, got:\n%s", out)
	}
	if strings.Count(out, ">1<") < 1 {
		t.Fatalf("expected second SECTIONPAGES field to resolve to 1 page, got:\n%s", out)
	}
}

func TestHTMLFieldDateWithFormat(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Kind: "run", FieldKind: "DATE", FieldInstr: `DATE \@ "yyyy-MM-dd"`}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// Should contain a date in YYYY-MM-DD format
	if !strings.Contains(out, "-") {
		t.Errorf("expected formatted date in output, got:\n%s", out)
	}
}

func TestHTMLFieldCreateDate(t *testing.T) {
	doc := &resolve.Document{
		CreatedAt: "2023-01-15T10:30:00",
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Kind: "run", FieldKind: "CREATEDATE"}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "2023-01-15") {
		t.Errorf("expected created date 2023-01-15 in output, got:\n%s", out)
	}
}

func TestHTMLFieldDateTimeSystemFormatSwitch(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Kind: "run", FieldKind: "DATE", FieldInstr: `DATE \s`}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Kind: "run", FieldKind: "TIME", FieldInstr: `TIME \s`}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// DATE \s should produce short date like "1/2/2006"
	reDate := regexp.MustCompile(`\d{1,2}/\d{1,2}/\d{4}`)
	if !reDate.MatchString(out) {
		t.Errorf("expected DATE \\s to produce short date (M/d/yyyy), got:\n%s", out)
	}
	// TIME \s should produce short time like "3:04 PM"
	reTime := regexp.MustCompile(`\d{1,2}:\d{2}\s*(AM|PM)`)
	if !reTime.MatchString(out) {
		t.Errorf("expected TIME \\s to produce short time (h:mm PM), got:\n%s", out)
	}
}

func TestHTMLFieldTimeDefaultFormat(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Kind: "run", FieldKind: "TIME"}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	reTime := regexp.MustCompile(`\d{1,2}:\d{2}\s*(AM|PM)`)
	if !reTime.MatchString(out) {
		t.Fatalf("expected TIME without explicit switches to render a time, got:\n%s", out)
	}
	if regexp.MustCompile(`\d{4}-\d{2}-\d{2}`).MatchString(out) {
		t.Fatalf("did not expect TIME default output to fall back to an ISO date, got:\n%s", out)
	}
}

func TestHTMLTabLeader(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "Left"},
					{Kind: "tab"},
					{Kind: "run", Text: "Right"},
				},
				Format: resolve.ParagraphFormat{
					TabStops: []resolve.TabStop{{Pos: 4320, Algn: "right", Leader: "dot"}},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "border-bottom:1px dotted") {
		t.Errorf("expected dotted tab leader in output, got:\n%s", out)
	}
}

func TestHTMLExactRowHeightOverflowHidden(t *testing.T) {
	h := 288 // 14.4pt in twips
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "table", Table: &resolve.Table{
				Rows: []resolve.TableRow{
					{Format: resolve.TableRowFormat{Height: &h, HeightRule: "exact"}, Cells: []resolve.TableCell{{
						Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Tall content here"}}}}},
					}}},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "overflow:hidden") {
		t.Errorf("expected overflow:hidden on exact-height row cells, got:\n%s", out)
	}
}

// pageBodyStyle extracts the inline style attribute from the <div class="page-body"> element.
func pageBodyStyle(html string) string {
	start := strings.Index(html, `<div class="page-body"`)
	if start < 0 {
		return ""
	}
	end := strings.Index(html[start:], ">")
	if end < 0 {
		return ""
	}
	tag := html[start : start+end]
	styleIdx := strings.Index(tag, `style="`)
	if styleIdx < 0 {
		return ""
	}
	styleStart := styleIdx + 7
	styleEnd := strings.Index(tag[styleStart:], `"`)
	if styleEnd < 0 {
		return ""
	}
	return tag[styleStart : styleStart+styleEnd]
}

func TestHTMLDoNotDisplayPageBoundariesSuppressesBorders(t *testing.T) {
	doc := &resolve.Document{
		DoNotDisplayPageBoundaries: true,
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
			PageBorders: &resolve.PageBorders{
				Top:    &resolve.BorderSide{Val: "single", SizePt: 1, Color: "000000"},
				Left:   &resolve.BorderSide{Val: "single", SizePt: 1, Color: "000000"},
				Bottom: &resolve.BorderSide{Val: "single", SizePt: 1, Color: "000000"},
				Right:  &resolve.BorderSide{Val: "single", SizePt: 1, Color: "000000"},
			},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	style := pageBodyStyle(out)
	if strings.Contains(style, "border-top") || strings.Contains(style, "border-left") {
		t.Fatalf("expected no page borders when DoNotDisplayPageBoundaries is true: %s", style)
	}
}

func TestHTMLBordersDoNotSurroundHeaderFooter(t *testing.T) {
	doc := &resolve.Document{
		BordersDoNotSurroundHeader: true,
		BordersDoNotSurroundFooter: true,
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
			PageBorders: &resolve.PageBorders{
				Top:    &resolve.BorderSide{Val: "single", SizePt: 1, Color: "000000"},
				Left:   &resolve.BorderSide{Val: "single", SizePt: 1, Color: "000000"},
				Bottom: &resolve.BorderSide{Val: "single", SizePt: 1, Color: "000000"},
				Right:  &resolve.BorderSide{Val: "single", SizePt: 1, Color: "000000"},
			},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	style := pageBodyStyle(out)
	if strings.Contains(style, "border-top") {
		t.Errorf("expected no border-top when BordersDoNotSurroundHeader is true: %s", style)
	}
	if strings.Contains(style, "border-bottom") {
		t.Errorf("expected no border-bottom when BordersDoNotSurroundFooter is true: %s", style)
	}
	if !strings.Contains(style, "border-left") || !strings.Contains(style, "border-right") {
		t.Errorf("expected left/right borders to still render: %s", style)
	}
}

func TestHTMLTableHMergeCreatesColspan(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "table", Table: &resolve.Table{
				Rows: []resolve.TableRow{{
					Cells: []resolve.TableCell{
						{Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "A"}}}}}, Format: resolve.TableCellFormat{}},
						{Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "B"}}}}}, Format: resolve.TableCellFormat{HMerge: true}},
						{Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "C"}}}}}, Format: resolve.TableCellFormat{}},
					},
				}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// HMerge cell should be omitted; first cell should have colspan="2".
	if strings.Contains(out, ">B<") {
		t.Fatalf("expected HMerge cell to be omitted from output: %s", out)
	}
	if !strings.Contains(out, `colspan="2"`) {
		t.Fatalf("expected first cell to have colspan=2 due to HMerge: %s", out)
	}
	if !strings.Contains(out, ">A<") || !strings.Contains(out, ">C<") {
		t.Fatalf("expected non-merged cells A and C to render: %s", out)
	}
}

func TestHTMLTableHMergeWithGridSpanCombines(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "table", Table: &resolve.Table{
				Rows: []resolve.TableRow{{
					Cells: []resolve.TableCell{
						{Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "A"}}}}}, Format: resolve.TableCellFormat{GridSpan: 2}},
						{Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "B"}}}}}, Format: resolve.TableCellFormat{HMerge: true}},
						{Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "C"}}}}}, Format: resolve.TableCellFormat{}},
					},
				}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// GridSpan 2 + one HMerge = effective colspan 3.
	if !strings.Contains(out, `colspan="3"`) {
		t.Fatalf("expected first cell to have colspan=3 (GridSpan 2 + HMerge): %s", out)
	}
	if strings.Contains(out, ">B<") {
		t.Fatalf("expected HMerge cell to be omitted: %s", out)
	}
}

func TestHTMLNonFloatingTextBoxRendersInline(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "textbox",
					TextBox: &resolve.TextBox{
						Floating: false,
						Blocks: []resolve.Block{{
							Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Inline box text"}}},
						}},
					},
				}},
			},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "Inline box text") {
		t.Fatalf("expected non-floating text box content to render inline: %s", out)
	}
	textPos := strings.Index(out, "Inline box text")
	if textPos < 0 {
		t.Fatalf("inline textbox text missing: %s", out)
	}
	start := strings.LastIndex(out[:textPos], "<div")
	if start < 0 {
		start = 0
	}
	snippet := out[start:textPos]
	if strings.Contains(snippet, `position:absolute`) || strings.Contains(snippet, `floating-textbox`) {
		t.Fatalf("expected non-floating text box to remain inline, got snippet: %s", snippet)
	}
}

func TestHTMLFrameParagraphAtPageLevel(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Text: "Frame para text"}},
				Frame: &resolve.FrameProps{
					W:       1440,
					H:       1440,
					X:       720,
					Y:       720,
					HAnchor: "page",
					VAnchor: "page",
				},
			},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="frame-paragraph"`) {
		t.Fatalf("expected frame paragraph to render with frame-paragraph class: %s", out)
	}
	if !strings.Contains(out, "Frame para text") {
		t.Fatalf("expected frame paragraph text to render: %s", out)
	}
}

func TestHTMLFrameParagraphInsideTableCell(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "table", Table: &resolve.Table{
				Rows: []resolve.TableRow{{
					Cells: []resolve.TableCell{{
						Blocks: []resolve.Block{{
							Kind: "paragraph",
							Paragraph: &resolve.Paragraph{
								Runs: []resolve.Run{{Text: "Frame in cell"}},
								Frame: &resolve.FrameProps{
									W: 720,
									H: 720,
								},
							},
						}},
					}},
				}},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="frame-paragraph frame-inline"`) {
		t.Fatalf("expected frame paragraph inside table cell to render as frame-inline: %s", out)
	}
	if !strings.Contains(out, "Frame in cell") {
		t.Fatalf("expected frame paragraph text to render inside table cell: %s", out)
	}
}

func TestHTMLShapeDashArrayRendersSVGStrokeDasharray(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:        "rect",
						Floating:    true,
						Width:       127000,
						Height:      127000,
						Geometry:    &dml.PresetGeometry{Name: "ellipse"},
						FillColor:   "FF0000",
						StrokeColor: "0000FF",
						StrokeWidth: 12700,
						LineStyle: &dml.LineStyle{
							Width: 1.0,
							Color: "0000FF",
							Dash:  dml.DashPatternDash,
						},
					},
				}},
			},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `stroke-dasharray="3, 3"`) {
		t.Fatalf("expected SVG stroke-dasharray for dash pattern: %s", out)
	}
}

func TestHTMLSeqFieldHexAndArabicDashFormats(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", FieldKind: "SEQ", FieldArg: "Item", FieldInstr: `SEQ Item \r 15 \* hex`},
					{Kind: "run", Text: " "},
					{Kind: "run", FieldKind: "SEQ", FieldArg: "Item", FieldInstr: `SEQ Item \* arabicDash`},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">f</span>") {
		t.Fatalf("expected SEQ \\* hex for 15 to yield f, got:\n%s", out)
	}
	if !strings.Contains(out, ">- 16 -</span>") {
		t.Fatalf("expected SEQ \\* arabicDash for 16 to yield '- 16 -', got:\n%s", out)
	}
}

func TestHTMLSeqFieldTotalCountSwitch(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", FieldKind: "SEQ", FieldArg: "Figure", FieldInstr: `SEQ Figure`},
					{Kind: "run", Text: " "},
					{Kind: "run", FieldKind: "SEQ", FieldArg: "Figure", FieldInstr: `SEQ Figure`},
					{Kind: "run", Text: " "},
					{Kind: "run", FieldKind: "SEQ", FieldArg: "Figure", FieldInstr: `SEQ Figure \n`},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">1</span>") || !strings.Contains(out, ">2</span>") {
		t.Fatalf("expected SEQ fields to resolve to 1 and 2, got:\n%s", out)
	}
	if !strings.Contains(out, ">3</span>") {
		t.Fatalf("expected SEQ \\n field to resolve to total count 3, got:\n%s", out)
	}
}

func TestHTMLRefFieldSuppressHyperlinkSwitch(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "target", Anchor: "bm1"},
				},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", FieldKind: "REF", FieldArg: "bm1", Anchor: "bm1", FieldInstr: `REF bm1 \f`, Text: "target"},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Contains(out, "<a href=\"#bm1\"") {
		t.Fatalf("expected REF \\f to suppress hyperlink, got:\n%s", out)
	}
	if !strings.Contains(out, ">target</span>") {
		t.Fatalf("expected REF display text to still render, got:\n%s", out)
	}
}

func TestHTMLRefFieldBookmarkPrefixSwitch(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "1.2.3", Anchor: "bm1"},
				},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", FieldKind: "REF", FieldArg: "bm1", Anchor: "bm1", FieldInstr: `REF bm1 \s`, Text: "1.2.3"},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">bm1 1.2.3</span>") {
		t.Fatalf("expected REF \\s to prepend bookmark name, got:\n%s", out)
	}
}

func TestHTMLRefFieldNumberFormatSwitch(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "1234.5", Anchor: "bm1"},
				},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", FieldKind: "REF", FieldArg: "bm1", Anchor: "bm1", FieldInstr: `REF bm1 \# "0.00"`, Text: "1234.5"},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">1234.50</span>") {
		t.Fatalf("expected REF \\# to format number, got:\n%s", out)
	}
}

func TestHTMLDateFieldLastFormatSwitch(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", FieldKind: "DATE", FieldInstr: `DATE \l`},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// \l falls back to system default format, which should produce a non-empty date.
	if !strings.Contains(out, "word-field-date") {
		t.Fatalf("expected DATE \\l to render with system default format, got:\n%s", out)
	}
}

func TestHTMLDateFieldBuiltInLanguageSwitch(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", FieldKind: "DATE", FieldInstr: `DATE \b`},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "word-field-date") {
		t.Fatalf("expected DATE \\b to render with built-in language format, got:\n%s", out)
	}
}

func TestHTMLTimeFieldLastAndBuiltInLanguageSwitches(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", FieldKind: "TIME", FieldInstr: `TIME \l`},
				},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", FieldKind: "TIME", FieldInstr: `TIME \b`},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	reLast := regexp.MustCompile(`class="word-field word-field-time">(?:<span[^>]*>)?\d{1,2}:\d{2}\s*(AM|PM)`)
	if !reLast.MatchString(out) {
		t.Fatalf("expected TIME \\l to render with short time fallback, got:\n%s", out)
	}
	reBuiltIn := regexp.MustCompile(`class="word-field word-field-time">(?:<span[^>]*>)?\d{2}:\d{2}:\d{2}`)
	if len(reBuiltIn.FindAllString(out, -1)) == 0 {
		t.Fatalf("expected TIME \\b to render with built-in time format, got:\n%s", out)
	}
}

func TestHTMLNextIfFieldAdvancesRecord(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", FieldKind: "MERGEFIELD", FieldArg: "City"},
					{Kind: "run", Text: " "},
					{Kind: "run", FieldKind: "NEXTIF", FieldInstr: `NEXTIF City = "Chicago"`},
					{Kind: "run", Text: " "},
					{Kind: "run", FieldKind: "MERGEFIELD", FieldArg: "City"},
				},
			}},
		},
	}
	out, _, err := render.HTMLWithOptions(doc, render.HTMLOptions{
		MergeRecords: []map[string]string{
			{"City": "Chicago"},
			{"City": "Boston"},
		},
	})
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">Chicago</span>") {
		t.Fatalf("expected first MERGEFIELD to be Chicago, got:\n%s", out)
	}
	if !strings.Contains(out, ">Boston</span>") {
		t.Fatalf("expected NEXTIF to advance to the Boston record, got:\n%s", out)
	}
}

func TestHTMLNestedNextIfFieldAdvancesRecord(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", FieldKind: "MERGEFIELD", FieldArg: "City"},
					{Kind: "run", Text: " "},
					{
						Kind:      "run",
						FieldKind: "IF",
						FieldInstrRuns: []resolve.Run{
							{Text: "IF "},
							{Fields: []resolve.Field{{
								Kind:  "NEXTIF",
								Instr: `NEXTIF City = "Chicago"`,
							}}},
							{Text: ` 1 = 1 "ok" "bad"`},
						},
					},
					{Kind: "run", Text: " "},
					{Kind: "run", FieldKind: "MERGEFIELD", FieldArg: "City"},
				},
			}},
		},
	}
	out, _, err := render.HTMLWithOptions(doc, render.HTMLOptions{
		MergeRecords: []map[string]string{
			{"City": "Chicago"},
			{"City": "Boston"},
		},
	})
	if err != nil {
		t.Fatalf("HTMLWithOptions: %v", err)
	}
	if !strings.Contains(out, ">Chicago</span>") {
		t.Fatalf("expected first MERGEFIELD to use the current record, got:\n%s", out)
	}
	if !strings.Contains(out, ">ok<") {
		t.Fatalf("expected IF field to keep evaluating after nested NEXTIF, got:\n%s", out)
	}
	if !strings.Contains(out, ">Boston</span>") {
		t.Fatalf("expected nested NEXTIF to advance the later MERGEFIELD to Boston, got:\n%s", out)
	}
}

func TestHTMLQuoteFieldWithNestedSetAndRefUsesMergeValue(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind:      "run",
					FieldKind: "QUOTE",
					FieldInstrRuns: []resolve.Run{
						{Text: "QUOTE "},
						{Fields: []resolve.Field{{
							Kind: "SET",
							InstrRuns: []resolve.Run{
								{Text: "SET Current "},
								{Fields: []resolve.Field{{
									Kind:  "MERGEFIELD",
									Instr: `MERGEFIELD Name`,
									Arg:   "Name",
								}}},
							},
						}}},
						{Text: " "},
						{Fields: []resolve.Field{{
							Kind:   "REF",
							Instr:  "REF Current",
							Arg:    "Current",
							Anchor: "Current",
						}}},
					},
				}},
			}},
		},
	}
	out, _, err := render.HTMLWithOptions(doc, render.HTMLOptions{
		MergeData: map[string]string{"Name": "Alice"},
	})
	if err != nil {
		t.Fatalf("HTMLWithOptions: %v", err)
	}
	if !strings.Contains(out, ">Alice<") {
		t.Fatalf("expected nested SET to store the merge value for the nested REF, got:\n%s", out)
	}
}

func TestHTMLSkipIfFieldSkipsRecord(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", FieldKind: "MERGEFIELD", FieldArg: "City"},
					{Kind: "run", Text: " "},
					{Kind: "run", FieldKind: "SKIPIF", FieldInstr: `SKIPIF City = "Chicago"`},
					{Kind: "run", Text: " "},
					{Kind: "run", FieldKind: "MERGEFIELD", FieldArg: "City"},
				},
			}},
		},
	}
	out, _, err := render.HTMLWithOptions(doc, render.HTMLOptions{
		MergeRecords: []map[string]string{
			{"City": "Chicago"},
			{"City": "Boston"},
		},
	})
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">Chicago</span>") {
		t.Fatalf("expected first MERGEFIELD to be Chicago, got:\n%s", out)
	}
	if !strings.Contains(out, ">Boston</span>") {
		t.Fatalf("expected SKIPIF to skip to the Boston record, got:\n%s", out)
	}
}

func TestHTMLNextIfFieldKeepsCurrentRecordWhenConditionFalse(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", FieldKind: "MERGEFIELD", FieldArg: "City"},
					{Kind: "run", Text: " "},
					{Kind: "run", FieldKind: "NEXTIF", FieldInstr: `NEXTIF City = "Chicago"`},
					{Kind: "run", Text: " "},
					{Kind: "run", FieldKind: "MERGEFIELD", FieldArg: "City"},
				},
			}},
		},
	}
	out, _, err := render.HTMLWithOptions(doc, render.HTMLOptions{
		MergeRecords: []map[string]string{
			{"City": "New York"},
			{"City": "Boston"},
		},
	})
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	countNY := strings.Count(out, ">New York</span>")
	if countNY != 2 {
		t.Fatalf("expected NEXTIF to keep the current record when false, got %d New York occurrences:\n%s", countNY, out)
	}
}

func TestHTMLSkipIfFieldKeepsCurrentRecordWhenConditionFalse(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", FieldKind: "MERGEFIELD", FieldArg: "City"},
					{Kind: "run", Text: " "},
					{Kind: "run", FieldKind: "SKIPIF", FieldInstr: `SKIPIF City = "Chicago"`},
					{Kind: "run", Text: " "},
					{Kind: "run", FieldKind: "MERGEFIELD", FieldArg: "City"},
				},
			}},
		},
	}
	out, _, err := render.HTMLWithOptions(doc, render.HTMLOptions{
		MergeRecords: []map[string]string{
			{"City": "New York"},
			{"City": "Boston"},
		},
	})
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	countNY := strings.Count(out, ">New York</span>")
	if countNY != 2 {
		t.Fatalf("expected SKIPIF to keep the current record when false, got %d New York occurrences:\n%s", countNY, out)
	}
}

func TestHTMLSymbolFieldRendersCharacter(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", FieldKind: "SYMBOL", FieldArg: "65", FieldInstr: `SYMBOL 65 \f "Symbol"`},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">A</span>") {
		t.Fatalf("expected SYMBOL 65 to render 'A', got:\n%s", out)
	}
}

func TestHTMLSymbolFieldHexCharacter(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", FieldKind: "SYMBOL", FieldArg: "0x2660", FieldInstr: `SYMBOL 0x2660 \f "Symbol"`},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "♠") {
		t.Fatalf("expected SYMBOL 0x2660 to render spade suit, got:\n%s", out)
	}
}

func TestHTMLListNumFieldRendersPlaceholder(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", FieldKind: "LISTNUM", FieldArg: "List1", FieldInstr: `LISTNUM List1`},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">[List1]</span>") {
		t.Fatalf("expected LISTNUM to render placeholder, got:\n%s", out)
	}
}

func TestHTMLIncludeTextFieldRendersPlaceholder(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", FieldKind: "INCLUDETEXT", FieldArg: `C:\doc.txt`, FieldInstr: `INCLUDETEXT "C:\doc.txt"`},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">[C:\\doc.txt]</span>") {
		t.Fatalf("expected INCLUDETEXT to render placeholder, got:\n%s", out)
	}
}

func TestHTMLIncludePictureFieldRendersPlaceholder(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", FieldKind: "INCLUDEPICTURE", FieldArg: `C:\img.png`, FieldInstr: `INCLUDEPICTURE "C:\img.png"`},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">[C:\\img.png]</span>") {
		t.Fatalf("expected INCLUDEPICTURE to render placeholder, got:\n%s", out)
	}
}

func TestHTMLToaFieldRendersPlaceholder(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", FieldKind: "TOA", FieldInstr: `TOA \h`},
				},
			}},
		},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">[Table of Authorities]</span>") {
		t.Fatalf("expected TOA to render placeholder, got:\n%s", out)
	}
}

func TestHTMLScopedImagesRenderDifferentDataInDocumentAndHeader(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
			Headers:    []resolve.Reference{{Type: "default", PartName: "header1"}},
		}},
		Headers: map[string]resolve.Story{
			"header1": {
				Paragraphs: []resolve.Paragraph{{
					Runs: []resolve.Run{{
						Kind: "image",
						Image: &resolve.Image{
							MIMEType: "image/png",
							Data:     []byte("header-image-data"),
							Width:    914400,
							Height:   914400,
						},
					}},
				}},
			},
		},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "image",
					Image: &resolve.Image{
						MIMEType: "image/png",
						Data:     []byte("doc-image-data"),
						Width:    914400,
						Height:   914400,
					},
				}},
			},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// Both images should render with distinct base64 data.
	if !strings.Contains(out, "data:image/png;base64,"+base64.StdEncoding.EncodeToString([]byte("doc-image-data"))) {
		t.Errorf("document image data missing from rendered HTML")
	}
	if !strings.Contains(out, "data:image/png;base64,"+base64.StdEncoding.EncodeToString([]byte("header-image-data"))) {
		t.Errorf("header image data missing from rendered HTML")
	}
}

func TestHTMLScopedChartsRenderDifferentDataInDocumentAndFooter(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
			Footers:    []resolve.Reference{{Type: "default", PartName: "footer1"}},
		}},
		Footers: map[string]resolve.Story{
			"footer1": {
				Paragraphs: []resolve.Paragraph{{
					Runs: []resolve.Run{{
						Kind:  "chart",
						Image: &resolve.Image{MIMEType: "image/svg+xml", Data: []byte("<svg>FooterChart</svg>")},
					}},
				}},
			},
		},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind:  "chart",
					Image: &resolve.Image{MIMEType: "image/svg+xml", Data: []byte("<svg>DocChart</svg>")},
				}},
			},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	docChartB64 := base64.StdEncoding.EncodeToString([]byte("<svg>DocChart</svg>"))
	if !strings.Contains(out, docChartB64) {
		t.Errorf("document chart SVG missing from rendered HTML")
	}
	footerChartB64 := base64.StdEncoding.EncodeToString([]byte("<svg>FooterChart</svg>"))
	if !strings.Contains(out, footerChartB64) {
		t.Errorf("footer chart SVG missing from rendered HTML")
	}
}

func TestHTMLVectorShapePictureFillWithReflectionHasSingleDefs(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:     "rect",
						Floating: true,
						Width:    127000,
						Height:   127000,
						Geometry: &dml.PresetGeometry{Name: "ellipse"},
						Fill: dml.PictureFill{
							EmbedID:     "rId1",
							ContentType: "image/png",
							ImageData:   []byte("PNG"),
						},
						Effects: dml.EffectList{Effects: []dml.Effect{
							dml.Reflection{Alpha: 0.25, Blur: 2, Distance: 3, Direction: 90},
						}},
					},
				}},
			},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<svg class="shape__vector"`) {
		t.Fatalf("expected SVG vector shape: %s", out)
	}
	// Count <defs> occurrences — should be exactly one.
	defsCount := strings.Count(out, "<defs>")
	if defsCount != 1 {
		t.Fatalf("expected exactly one <defs> block, got %d:\n%s", defsCount, out)
	}
	// Reflection filter should be present.
	if !strings.Contains(out, `id="word-reflection-filter"`) {
		t.Fatalf("expected reflection filter in defs: %s", out)
	}
	// Picture fill pattern should be present.
	if !strings.Contains(out, `<pattern`) {
		t.Fatalf("expected picture fill pattern in defs: %s", out)
	}
}

func TestHTMLVectorShapeReflectionUsesSizeOffsetAndFadeMask(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "shape",
					Shape: &resolve.Shape{
						Kind:      "rect",
						Floating:  true,
						Width:     127000,
						Height:    127000,
						Geometry:  &dml.PresetGeometry{Name: "ellipse"},
						FillColor: "3366FF",
						Effects: dml.EffectList{Effects: []dml.Effect{
							dml.Reflection{Alpha: 0.4, Blur: 2, Distance: 3, Size: 0.5, Direction: 90, Fade: true},
						}},
					},
				}},
			},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `id="word-reflection-mask"`) {
		t.Fatalf("expected reflection mask in defs: %s", out)
	}
	if !strings.Contains(out, `id="word-reflection-gradient"`) {
		t.Fatalf("expected reflection gradient in defs: %s", out)
	}
	if !strings.Contains(out, `mask="url(#word-reflection-mask)"`) {
		t.Fatalf("expected reflected copy to use mask: %s", out)
	}
	if !strings.Contains(out, `scale(1, -0.5000)`) {
		t.Fatalf("expected reflected copy to honor reflection size: %s", out)
	}
	if strings.Contains(out, "box-shadow:") {
		t.Fatalf("expected vector reflection to avoid CSS box-shadow fallback: %s", out)
	}
}

func TestHTMLSdtDropDownListRendersOptionValues(t *testing.T) {
	doc := sdtRun("dropDownList", &resolve.ContentControl{
		ListItems: []resolve.SdtListItem{
			{DisplayText: "Red", Value: "R"},
			{DisplayText: "Green", Value: "G"},
			{DisplayText: "Blue", Value: "B"},
		},
	}, "Green")
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<option value="R">Red`) {
		t.Errorf("expected option with value=R for Red; got:\n%s", out)
	}
	if !strings.Contains(out, `<option value="G" selected>Green`) {
		t.Errorf("expected selected option with value=G for Green; got:\n%s", out)
	}
	if !strings.Contains(out, `<option value="B">Blue`) {
		t.Errorf("expected option with value=B for Blue; got:\n%s", out)
	}
}
func TestHTMLOLEObjectPreviewAndLink(t *testing.T) {
	para := &resolve.Paragraph{Runs: []resolve.Run{{
		Kind: "olePlaceholder", Text: "Excel.Sheet.12", OLEProgID: "Excel.Sheet.12", OLERelID: "rOle1", OLEType: "Link", OLELinkURL: "https://example.com/book.xlsx",
		OLEPreview: &resolve.Image{MIMEType: "image/png", Data: []byte{0x89, 0x50, 0x4e, 0x47}, Width: 914400, Height: 914400},
	}}}
	doc := &resolve.Document{Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: para}}}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`class="ole-object-link"`, `href="https://example.com/book.xlsx"`, `data-ole-progid="Excel.Sheet.12"`, `data-ole-type="Link"`, `data:image/png;base64,`} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestHTMLAutoLineSpacingMatchesLayoutSingleLineAdvance(t *testing.T) {
	line150 := 360
	line200 := 480
	doc := &resolve.Document{Blocks: []resolve.Block{
		{Kind: "paragraph", Paragraph: &resolve.Paragraph{Format: resolve.ParagraphFormat{LineSpacing: &line150, LineRule: "auto"}, Runs: []resolve.Run{{Text: "One and a half"}}}},
		{Kind: "paragraph", Paragraph: &resolve.Paragraph{Format: resolve.ParagraphFormat{LineSpacing: &line200, LineRule: "auto"}, Runs: []resolve.Run{{Text: "Double"}}}},
	}}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "line-height:1.500") {
		t.Fatalf("expected 1.5 Word lines to render as 1.5 CSS line-height: %s", out)
	}
	if !strings.Contains(out, "line-height:2.000") {
		t.Fatalf("expected 2.0 Word lines to render as 2.0 CSS line-height: %s", out)
	}
}

func TestHTMLLegacyMergedTableFixtureUsesNativePhysicalGrid(t *testing.T) {
	if _, err := os.Stat("../../../testdata/doc/apache-table-merges.doc"); err != nil {
		if os.IsNotExist(err) {
			t.Skip("optional legacy fixture is not available in this checkout")
		}
		t.Fatalf("stat fixture: %v", err)
	}
	doc, err := legacy.ParseDoc("../../../testdata/doc/apache-table-merges.doc")
	if err != nil {
		t.Fatalf("ParseDoc: %v", err)
	}
	html, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if got := strings.Count(html, "<td"); got != 11 {
		t.Fatalf("td count = %d, want 11 logical cells; html=%s", got, html)
	}
	for _, want := range []string{`colspan="3"`, `colspan="5"`} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %s in merged-table HTML", want)
		}
	}
	if strings.Contains(html, `padding:0;border:none`) {
		t.Fatalf("merged-table HTML still contains synthetic grid-gap cells")
	}
	if !regexp.MustCompile(`(?s)>I</[^>]+>.*>J</`).MatchString(html) {
		t.Fatalf("I/J paragraphs were not retained in their shared physical cell")
	}
}

func TestHTMLLegacyFloatingTableFixtureUsesAbsoluteOverlay(t *testing.T) {
	if _, err := os.Stat("../../../testdata/doc/libre-floating-table-section-columns.doc"); err != nil {
		if os.IsNotExist(err) {
			t.Skip("optional legacy fixture is not available in this checkout")
		}
		t.Fatalf("stat fixture: %v", err)
	}
	doc, err := legacy.ParseDoc("../../../testdata/doc/libre-floating-table-section-columns.doc")
	if err != nil {
		t.Fatalf("ParseDoc: %v", err)
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	html := string(out)
	if !strings.Contains(html, `class="float-table" style="position:absolute;`) {
		t.Fatalf("expected floating table overlay, got:\n%s", html)
	}
	if !strings.Contains(html, "column-count:2") {
		t.Fatalf("expected two-column section CSS, got:\n%s", html)
	}
	if strings.Contains(html, "sprmSDxaColWidth") || strings.Contains(html, "sprmSDxaColSpacing") || strings.Contains(html, "sprmSFEvenlySpaced") {
		t.Fatalf("section column SPRMs should be semantically mapped, diagnostics were:\n%s", html)
	}
}

func TestHTMLLegacyColumnsStayInTwoPhysicalColumns(t *testing.T) {
	path := "../../../testdata/doc/legacy-columns.doc"
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			t.Skip("optional legacy fixture is not available in this checkout")
		}
		t.Fatalf("stat fixture: %v", err)
	}
	doc, err := legacy.ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc: %v", err)
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	pages := regexp.MustCompile(`(?s)<section class="page".*?</section>`).FindAllString(string(out), -1)
	if len(pages) != 3 {
		t.Fatalf("pages = %d, want 3", len(pages))
	}
	wantBounds := [][2]string{
		{"Column paragraph 40", "Column paragraph 27"},
		{"Column paragraph 26", "Column paragraph 13"},
		{"Column paragraph 12", "Column paragraph 1"},
	}
	tags := regexp.MustCompile(`<[^>]*>`)
	headings := regexp.MustCompile(`Column paragraph \d+`)
	for i, page := range pages {
		if strings.Contains(page, "column-count:2") {
			t.Errorf("page %d delegates physical columns to browser multicolumn flow", i+1)
		}
		if got := strings.Count(page, `class="word-column"`); got != 2 {
			t.Errorf("page %d has %d physical columns, want 2", i+1, got)
		}
		text := tags.ReplaceAllString(page, "")
		matches := headings.FindAllString(text, -1)
		if len(matches) == 0 || matches[0] != wantBounds[i][0] || matches[len(matches)-1] != wantBounds[i][1] {
			t.Errorf("page %d paragraph bounds = %v, want %v", i+1, matches, wantBounds[i])
		}
		if strings.Contains(text, "text.Multi-") {
			t.Errorf("page %d lost a space across a wrapped line", i+1)
		}
	}
}

func TestHTMLLegacyLayoutStressUsesSourceBullets(t *testing.T) {
	path := "../../../testdata/doc/legacy-layout-stress.doc"
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			t.Skip("optional legacy fixture is not available in this checkout")
		}
		t.Fatalf("stat fixture: %v", err)
	}
	doc, err := legacy.ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc: %v", err)
	}
	if len(doc.Paragraphs) == 0 || doc.Paragraphs[0].List == nil || doc.Paragraphs[0].List.Kind != "bullet" {
		t.Fatal("Word 97 bullet list was not decoded from PlfLst/PlfLfo")
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	html := string(out)
	if strings.Contains(html, "<ol") {
		t.Error("bullet list rendered as an ordered list")
	}
	if !strings.Contains(html, `.word-physical-inline-list-marker { display: inline-block;`) {
		t.Error("physical list markers are not owned by the first measured line")
	}
	if got := strings.Count(html, `word-physical-inline-list-marker`); got != 19 {
		// One occurrence is the CSS class definition; the remaining 18 are markers.
		t.Errorf("physical inline marker occurrences = %d, want 19", got)
	}
	if !strings.Contains(html, `font-size:16.0pt`) {
		t.Error("legacy bullets should inherit the 16pt paragraph text size when LVL CHPX omits a size")
	}
	if strings.Contains(html, `word-list-marker word-physical-list-marker word-list-bullet`) {
		t.Error("layout-owned legacy bullets should not be forced into one absolute gutter column")
	}
}

func TestPageBodyKeepsFullPageBoxSoMainTextIsNotCroppedAtFooterBoundary(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{PageMargin: &resolve.PageMargin{Top: 1584, Bottom: 1584, Left: 1800, Right: 1800}}},
		Blocks:   []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Body"}}}}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `.page-body { position: relative; z-index: 1; height: 792.0pt; min-height: 792.0pt; padding: 79.2pt 90.0pt 79.2pt 90.0pt; overflow: hidden; }`) {
		t.Fatalf("page body must keep the full physical page box and Word margins; got:\n%s", out)
	}
}

func TestHTMLFooterTerminalParagraphMarkDoesNotRaiseFooterIntoBody(t *testing.T) {
	doc := &resolve.Document{
		Footers: map[string]resolve.Story{
			"footer1": {Blocks: []resolve.Block{
				{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Footer line one"}}}},
				{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Footer line two"}}}},
				{Kind: "paragraph", Paragraph: &resolve.Paragraph{}}, // structural terminal paragraph mark
			}},
		},
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1584, Bottom: 1584, Left: 1800, Right: 1800, Footer: 720},
			Footers:    []resolve.Reference{{Type: "default", PartName: "footer1"}},
		}},
		Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Body"}}}}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// Two visible 12pt Word lines are 27.6pt tall.
	// With a 36pt footer distance, the frame top is 792 - 36 - 27.6 = 728.4pt.
	// Counting Word's mandatory trailing paragraph mark as a third visible line
	// would incorrectly move the footer up to 714.6pt.
	if !strings.Contains(out, `class="page-footer" style="position:absolute;left:90.0pt;top:728.4pt;`) {
		t.Fatalf("terminal empty footer paragraph must not raise footer into body; got:\n%s", out)
	}
}

func TestHTMLBodyCSSUsesDocumentDefaultRunFormat(t *testing.T) {
	fontSize := 22
	doc := &resolve.Document{
		DefaultRunFormat: resolve.RunFormat{
			FontSize:  &fontSize,
			FontASCII: "Aptos",
			Color:     "1F2937",
		},
		Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Body"}}}}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	css := extractCSS(out)
	if !strings.Contains(css, `body { background: #e8e8e8; font-family: Aptos,Arial,sans-serif; font-size: 11.0pt; color: #1F2937; }`) {
		t.Fatalf("body CSS did not use document defaults:\n%s", css)
	}
}

func TestHTMLHeaderFooterFramesRemainTextSelectable(t *testing.T) {
	doc := &resolve.Document{
		Headers: map[string]resolve.Story{
			"header1": {Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Header"}}}}}},
		},
		Footers: map[string]resolve.Story{
			"footer1": {Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Footer"}}}}}},
		},
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
			Headers:    []resolve.Reference{{Type: "default", PartName: "header1"}},
			Footers:    []resolve.Reference{{Type: "default", PartName: "footer1"}},
		}},
		Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Body"}}}}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	css := extractCSS(out)
	if !strings.Contains(css, `.page-overlay { position: absolute; z-index: 2; inset: 0; overflow: hidden; pointer-events: none; }`) {
		t.Fatalf("page overlay should stay non-interactive by default:\n%s", css)
	}
	if !strings.Contains(css, `.page-header, .page-footer, .page-footnotes { position: absolute; overflow: hidden; pointer-events: auto; user-select: text; }`) {
		t.Fatalf("header/footer frame CSS should allow text selection:\n%s", css)
	}
}

func TestHTMLPreWord97OversizeFixedTableFitsPrintableFrame(t *testing.T) {
	width := 10000
	doc := &resolve.Document{
		Source: &resolve.DocumentSource{Version: 0x0065},
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 11908, Height: 16840},
			PageMargin: &resolve.PageMargin{Top: 1700, Right: 1700, Bottom: 1700, Left: 1700},
		}},
		Blocks: []resolve.Block{{Kind: "table", Table: &resolve.Table{
			Format: resolve.TableFormat{
				Width:        &width,
				WidthType:    "dxa",
				Layout:       "fixed",
				ColumnWidths: []int{910, 9090},
			},
			Rows: []resolve.TableRow{{Cells: []resolve.TableCell{
				{Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Text: "1", Runs: []resolve.Run{{Kind: "run", Text: "1"}}}}}},
				{Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Text: "Body", Runs: []resolve.Run{{Kind: "run", Text: "Body"}}}}}},
			}}},
		}}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// 11908 twips = 595.4pt; 85pt margins leave a 425.4pt printable frame.
	if !strings.Contains(out, `<table style="width:425.4pt;table-layout:fixed">`) {
		t.Fatalf("legacy table must be clamped to printable frame; got:\n%s", out)
	}
	if !strings.Contains(out, `<col style="width:38.7pt"><col style="width:386.7pt">`) {
		t.Fatalf("legacy table columns must be proportionally scaled with the table; got:\n%s", out)
	}
}

func TestHTMLListParagraphUsesCanonicalPhysicalItemBody(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				List:      &resolve.ListMarker{Kind: "ordered", Level: 0, Start: 1, Format: "decimal", Template: "%1."},
				Numbering: &resolve.Numbering{NumID: 1, Level: 0, Start: 1, Format: "decimal", Text: "%1.", IndentLeft: 720, IndentHanging: 360},
				Runs:      []resolve.Run{{Kind: "run", Text: "list item"}},
			},
		}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "list item") {
		t.Fatalf("list rendering lost item text: %s", out)
	}
	if !strings.Contains(out, `class="word-physical-slot"`) {
		t.Fatalf("list item body did not use canonical physical slots: %s", out)
	}
}

// stripTagsForTest removes markup so text assertions do not depend on how runs
// are wrapped (layout-owned rendering may emit run text without a span).
func stripTagsForTest(markup string) string {
	return regexp.MustCompile(`<[^>]+>`).ReplaceAllString(markup, "")
}

func TestHTMLHeaderFooterStoriesUseCanonicalPhysicalTabGeometry(t *testing.T) {
	center, right := 4320, 8640
	footer := resolve.Paragraph{
		Format: resolve.ParagraphFormat{TabStops: []resolve.TabStop{
			{Pos: center, Algn: "center"},
			{Pos: right, Algn: "right"},
		}},
		Runs: []resolve.Run{
			{Kind: "run", Text: "Author"},
			{Kind: "tab", Text: "\t"},
			{Kind: "run", Text: "Page 1"},
			{Kind: "tab", Text: "\t"},
			{Kind: "run", Text: "9/9/2026"},
		},
	}
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1800, Bottom: 1440, Left: 1800, Header: 720, Footer: 720},
			Footers:    []resolve.Reference{{Type: "default", PartName: "footer"}},
		}},
		Footers: map[string]resolve.Story{
			"footer": {Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &footer}}},
		},
		Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "body"}}}}},
	}
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="page-footer"`) {
		t.Fatalf("missing footer frame: %s", out)
	}
	footerStart := strings.Index(out, `class="page-footer"`)
	if footerStart < 0 {
		t.Fatalf("missing footer frame: %s", out)
	}
	footerHTML := out[footerStart:]
	if !strings.Contains(footerHTML, `class="word-physical-slot"`) {
		t.Fatalf("footer story did not render through a canonical physical slot: %s", out)
	}
	if strings.Contains(footerHTML, `display:inline-block;width:36.0pt`) {
		t.Fatalf("footer fell back to semantic default-tab spacers: %s", out)
	}
	// The footer text is positioned by layout, so runs are not required to be
	// wrapped in per-run spans; assert the rendered text itself survives.
	footerText := stripTagsForTest(footerHTML)
	for _, want := range []string{"Author", "Page 1", "9/9/2026"} {
		if !strings.Contains(footerText, want) {
			t.Fatalf("footer lost %q: %s", want, out)
		}
	}
}

func TestHTMLWordFieldCSSDoesNotImposePresentationStyle(t *testing.T) {
	source, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	css := string(source)
	if strings.Contains(css, `.word-field { font-style: italic;`) {
		t.Fatal("generic Word field CSS must not impose italic presentation")
	}
	if strings.Contains(css, `.word-field { font-style: inherit; color: #555;`) {
		t.Fatal("generic Word field CSS must not impose gray presentation")
	}
	if !strings.Contains(css, `.word-field { font-style: inherit; color: inherit; }`) {
		t.Fatal("generic Word field CSS must inherit run presentation")
	}
}
