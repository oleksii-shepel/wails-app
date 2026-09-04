package render_test

import (
	"encoding/binary"

	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"office/pkg/diag"
	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/opc"
	"office/pkg/presentation/parsers/ooxml/parts"
	"office/pkg/presentation/parsers/ooxml/resolve"
	"office/pkg/presentation/render"
)

func TestHTMLIncludesDiagnosticsDetails(t *testing.T) {
	pres := &resolve.Presentation{
		Diagnostics: diag.List{{
			Severity: diag.SeverityWarning,
			Code:     "pptx/unsupported-smartart",
			Message:  "SmartArt content is not supported and was skipped",
		}},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "<details class=\"diagnostics\">") {
		t.Fatalf("expected diagnostics details block in output: %s", out)
	}
	if !strings.Contains(out, "[pptx/unsupported-smartart]") {
		t.Fatalf("expected diagnostic code in output: %s", out)
	}
}

func TestHTMLRendersOLEObjectPlaceholder(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Index: 0,
			Shapes: []*resolve.Shape{{
				Kind:            resolve.ShapeOleObject,
				Name:            "Object",
				Bounds:          resolve.Rect{X: 914400, Y: 685800, W: 1828800, H: 1371600},
				OLEObjectName:   "Document",
				OLEObjectProgID: "Word.Document.12",
			}},
		}},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="shape shape--ole-object"`) {
		t.Fatalf("expected ole-object shape class in output: %s", out)
	}
	if !strings.Contains(out, "[Document]") {
		t.Fatalf("expected object name placeholder label in output: %s", out)
	}
}

func TestHTMLRendersOLEObjectIcon(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Index: 0,
			Shapes: []*resolve.Shape{{
				Kind:          resolve.ShapeOleObject,
				Name:          "Object",
				Bounds:        resolve.Rect{X: 914400, Y: 685800, W: 1828800, H: 1371600},
				OLEObjectName: "Document",
				OLEIconData:   []byte("fake-png"),
				OLEIconMIME:   "image/png",
			}},
		}},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="shape shape--ole-object"`) {
		t.Fatalf("expected ole-object shape class in output: %s", out)
	}
	if !strings.Contains(out, `data:image/png;base64,`) {
		t.Fatalf("expected inline icon data URI in output: %s", out)
	}
}

func TestHTMLRendersSlideBackgroundPictureTile(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Index: 0,
			BgFill: drawing.PictureFill{
				ImageURL: "data:image/png;base64,abc",
				Tile:     true,
			},
		}},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<svg class="slide-background"`) {
		t.Fatalf("expected slide background svg in output: %s", out)
	}
	if !strings.Contains(out, "url(#slide-bg-fill-0)") {
		t.Fatalf("expected background fill reference in output: %s", out)
	}
}

func TestHTMLRendersSmartArtEllipse(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{
				Shapes: []*resolve.Shape{
					{
						Kind:     resolve.ShapeSmartArt,
						Bounds:   resolve.Rect{X: 914400, Y: 685800, W: 1828800, H: 1371600},
						Fill:     drawing.SolidFill{Color: "4472C4"},
						Geometry: "ellipse",
						Paragraphs: []*resolve.Paragraph{
							{Runs: []*resolve.Run{{Text: "Node"}}},
						},
					},
				},
			},
		},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "<ellipse") {
		t.Fatalf("expected ellipse SVG in output: %s", out)
	}
	if !strings.Contains(out, "Node") {
		t.Fatalf("expected node text in output: %s", out)
	}
}

func TestHTMLRendersRunFontFamily(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{
				Index: 0,
				Shapes: []*resolve.Shape{
					{
						Kind:   resolve.ShapeText,
						Bounds: resolve.Rect{W: 1000, H: 1000},
						Paragraphs: []*resolve.Paragraph{
							{
								Runs: []*resolve.Run{
									{Text: "Styled text", FontSize: 3600, FontFamily: "Oswald", Color: "FFFFFF"},
								},
							},
						},
					},
				},
			},
		},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `font-family:Oswald`) {
		t.Fatalf("expected font-family style in html: %s", html)
	}
}

func TestHTMLRendersSlideSectionAttribute(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{Index: 0, SectionName: "Introduction"},
			{Index: 1, SectionName: ""},
			{Index: 2, SectionName: "Details"},
		},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `data-section="Introduction"`) {
		t.Fatalf("expected Introduction section attribute in html: %s", html)
	}
	if strings.Contains(html, `data-section=""`) {
		t.Fatalf("unexpected empty section attribute in html: %s", html)
	}
	if !strings.Contains(html, `data-section="Details"`) {
		t.Fatalf("expected Details section attribute in html: %s", html)
	}
}

func TestHTMLRendersLineBreaksAsNewlines(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Index: 0,
			Shapes: []*resolve.Shape{{
				Kind:   resolve.ShapeText,
				Bounds: resolve.Rect{W: 1000, H: 1000},
				Paragraphs: []*resolve.Paragraph{{
					Runs: []*resolve.Run{
						{Text: "Line1"},
						{Text: "\n", Bold: true},
						{Text: "Line2"},
					},
				}},
			}},
		}},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	// The break should be rendered as an HTML <br> tag between the text fragments.
	if !strings.Contains(html, "Line1<span style=\"font-weight:bold;color:#000000\"><br></span>Line2") {
		t.Fatalf("expected <br> between Line1 and Line2 in html: %s", html)
	}
}

func TestHTMLRendersParagraphTextDirection(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Index: 0,
			Shapes: []*resolve.Shape{{
				Kind:   resolve.ShapeText,
				Bounds: resolve.Rect{W: 1000, H: 1000},
				Paragraphs: []*resolve.Paragraph{{
					TextDirection: "rtl",
					Runs:          []*resolve.Run{{Text: "مرحبا"}},
				}},
			}},
		}},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `direction:rtl`) {
		t.Fatalf("expected paragraph direction in html: %s", html)
	}
}

func TestHTMLRendersEmbeddedFontFaces(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		EmbeddedFonts: []*resolve.EmbeddedFont{{
			Family: "Source Code Pro",
			Faces: []*resolve.EmbeddedFontFace{{
				Family: "Source Code Pro",
				Weight: 400,
				Style:  "normal",
				Data:   fakeEmbeddedEOT(),
			}},
		}},
		Slides: []*resolve.Slide{{
			Index: 0,
			Shapes: []*resolve.Shape{{
				Kind:   resolve.ShapeText,
				Bounds: resolve.Rect{W: 1000, H: 1000},
				Paragraphs: []*resolve.Paragraph{{
					Runs: []*resolve.Run{{Text: "Styled text", FontFamily: "Source Code Pro", Color: "111111"}},
				}},
			}},
		}},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `@font-face{font-family:'Source Code Pro'`) {
		t.Fatalf("expected embedded font-face rule in html: %s", html)
	}
	if !strings.Contains(html, `format("embedded-opentype")`) {
		t.Fatalf("expected embedded-opentype source in html: %s", html)
	}
	if !strings.Contains(html, `font-family:'Source Code Pro'`) {
		t.Fatalf("expected embedded family name in html (no fallbacks): %s", html)
	}
	if strings.Contains(html, `Consolas`) {
		t.Fatalf("expected no fallback stack for embedded family in html: %s", html)
	}
}

func TestHTMLRendersTableShape(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Index: 0,
			Shapes: []*resolve.Shape{{
				Kind:   resolve.ShapeTable,
				Bounds: resolve.Rect{X: 1000, Y: 2000, W: 3000000, H: 2000000},
				Table: &resolve.Table{
					ColumnWidthsEMU: []int64{1000, 2000},
					Rows: []*resolve.TableRow{{
						Cells: []*resolve.TableCell{{
							GridSpan:   2,
							Fill:       drawing.GradientFill{GradientType: drawing.GradientTypeLinear, Angle: 90, Stops: []drawing.GradientStop{{Position: 0.0, Color: "FF0000", Alpha: 1.0}, {Position: 1.0, Color: "FFFFFF", Alpha: 1.0}}},
							FillColor:  "FF0000",
							Effects:    drawing.EffectList{Effects: []drawing.Effect{drawing.Shadow{Color: "663300", Alpha: 0.6, Blur: 2.0, Distance: 1.0, Angle: 45}}},
							Paragraphs: []*resolve.Paragraph{{Runs: []*resolve.Run{{Text: "Top", Bold: true, Color: "FFFFFF"}}}},
						}},
					}, {
						Cells: []*resolve.TableCell{{
							RowSpan:    2,
							FillColor:  "CCE5FF",
							Paragraphs: []*resolve.Paragraph{{Runs: []*resolve.Run{{Text: "Left"}}}},
						}, {
							FillColor:  "CCE5FF",
							Paragraphs: []*resolve.Paragraph{{Runs: []*resolve.Run{{Text: "Right"}}}},
						}},
					}, {
						Cells: []*resolve.TableCell{{
							VMerge: true,
						}, {
							FillColor:  "E2F0D9",
							Paragraphs: []*resolve.Paragraph{{Runs: []*resolve.Run{{Text: "Bottom"}}}},
						}},
					}},
				},
			}},
		}},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for _, want := range []string{
		`class="ppt-table"`,
		`colspan="2"`,
		`rowspan="2"`,
		`background:linear-gradient(`,
		`background:#CCE5FF`,
		`background:#E2F0D9`,
		`box-shadow:`,
		`font-weight:bold`,
		`color:#FFFFFF`,
		`Top`,
		`Bottom`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("expected %q in html: %s", want, html)
		}
	}
}

func fakeEmbeddedEOT() []byte {
	data := make([]byte, 36)
	binary.LittleEndian.PutUint32(data[0:4], uint32(len(data)))
	binary.LittleEndian.PutUint32(data[4:8], 0)
	binary.LittleEndian.PutUint32(data[8:12], 0x00020002)
	binary.LittleEndian.PutUint16(data[34:36], 0x504C)
	return data
}

func TestHTMLRendersRunStrikeAndUnderline(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Index: 0,
			Shapes: []*resolve.Shape{{
				Kind:   resolve.ShapeText,
				Bounds: resolve.Rect{W: 1000, H: 1000},
				Paragraphs: []*resolve.Paragraph{{
					Runs: []*resolve.Run{{Text: "Styled text", Underline: true, Strike: true, FontFamily: "Aptos"}},
				}},
			}},
		}},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `text-decoration:underline line-through`) {
		t.Fatalf("expected underline and strike in html: %s", html)
	}
}

func TestHTMLRendersRunCapsAndBaseline(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Index: 0,
			Shapes: []*resolve.Shape{{
				Kind:   resolve.ShapeText,
				Bounds: resolve.Rect{W: 1000, H: 1000},
				Paragraphs: []*resolve.Paragraph{{
					Runs: []*resolve.Run{
						{Text: "caps", AllCaps: true},
						{Text: "small", SmallCaps: true},
						{Text: "super", Superscript: true},
						{Text: "sub", Subscript: true},
					},
				}},
			}},
		}},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `<p>CAPS<span`) || !strings.Contains(html, `font-variant:small-caps`) {
		t.Fatalf("expected uppercase text in html: %s", html)
	}
	if !strings.Contains(html, `font-variant:small-caps`) {
		t.Fatalf("expected small-caps styling in html: %s", html)
	}
	if !strings.Contains(html, `<sup>super</sup>`) {
		t.Fatalf("expected superscript tag in html: %s", html)
	}
	if !strings.Contains(html, `<sub>sub</sub>`) {
		t.Fatalf("expected subscript tag in html: %s", html)
	}
}

func TestHTMLRendersEllipseTextWithShapeAwarePadding(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Index: 0,
			Shapes: []*resolve.Shape{{
				Kind:     resolve.ShapeText,
				Geometry: "ellipse",
				Bounds:   resolve.Rect{W: 914400, H: 914400},
				Paragraphs: []*resolve.Paragraph{{
					Runs: []*resolve.Run{{Text: "Portfolio"}},
				}},
			}},
		}},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `padding:11.52pt 12.96pt 11.52pt 12.96pt`) {
		t.Fatalf("expected ellipse-aware padding in html: %s", html)
	}
	if !strings.Contains(html, `clip-path:ellipse(50% 50% at 50% 50%)`) {
		t.Fatalf("expected ellipse clip-path in html: %s", html)
	}
}

func TestHTMLRendersCroppedImages(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Index: 0,
			Shapes: []*resolve.Shape{{
				Kind:            resolve.ShapeImage,
				Name:            "Headshot",
				Bounds:          resolve.Rect{W: 1000, H: 1000},
				ImageData:       []byte("image-bytes"),
				ImageMIME:       "image/png",
				ImageCropLeft:   0.10,
				ImageCropTop:    0.05,
				ImageCropRight:  0.20,
				ImageCropBottom: 0.15,
			}},
		}},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `class="shape shape--image"`) {
		t.Fatalf("expected image wrapper in html: %s", html)
	}
	if !strings.Contains(html, `class="shape__image"`) {
		t.Fatalf("expected image element class in html: %s", html)
	}
	if !strings.Contains(html, `width:142.8571%`) || !strings.Contains(html, `height:125.0000%`) {
		t.Fatalf("expected cropped image scaling in html: %s", html)
	}
	if !strings.Contains(html, `left:-14.2857%`) || !strings.Contains(html, `top:-6.2500%`) {
		t.Fatalf("expected cropped image offset in html: %s", html)
	}
}

func TestHTMLRendersEmbeddedVideoShape(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Index: 0,
			Shapes: []*resolve.Shape{{
				Kind:            resolve.ShapeMedia,
				Name:            "Demo Video",
				MediaKind:       "video",
				Bounds:          resolve.Rect{W: 1000, H: 1000},
				MediaData:       []byte("video-bytes"),
				MediaMIME:       "video/mp4",
				MediaPosterData: []byte("poster-bytes"),
				MediaPosterMIME: "image/png",
			}},
		}},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for _, want := range []string{
		`class="shape shape--media"`,
		`<video controls preload="metadata"`,
		`src="data:video/mp4;base64,`,
		`poster="data:image/png;base64,`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("expected %q in html: %s", want, html)
		}
	}
}

func TestHTMLExternalizesImagesWhenConfigured(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Index: 0,
			Shapes: []*resolve.Shape{{
				Kind:      resolve.ShapeImage,
				Name:      "Headshot",
				Bounds:    resolve.Rect{W: 1000, H: 1000},
				ImageData: []byte("image-bytes"),
				ImageMIME: "image/png",
			}},
		}},
	}

	html, assets, err := render.HTMLWithOptions(pres, render.HTMLOptions{
		ImageMode:        render.HTMLImageModeExternal,
		ExternalImageDir: "sample-1_files",
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(html, `src="data:image/png;base64,`) {
		t.Fatalf("expected external image src in html: %s", html)
	}
	if !strings.Contains(html, `src="sample-1_files/image-001.png"`) {
		t.Fatalf("expected sidecar image src in html: %s", html)
	}
	if len(assets) != 1 {
		t.Fatalf("expected one sidecar image asset, got %d", len(assets))
	}
	if assets[0].Path != "sample-1_files/image-001.png" {
		t.Fatalf("unexpected asset path: %s", assets[0].Path)
	}
	if assets[0].ContentType != "image/png" {
		t.Fatalf("unexpected asset content type: %s", assets[0].ContentType)
	}
	if string(assets[0].Data) != "image-bytes" {
		t.Fatalf("unexpected asset bytes: %q", string(assets[0].Data))
	}
}

func TestHTMLExternalizesMediaWhenConfigured(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Index: 0,
			Shapes: []*resolve.Shape{{
				Kind:      resolve.ShapeMedia,
				Name:      "Audio Clip",
				MediaKind: "audio",
				Bounds:    resolve.Rect{W: 1000, H: 1000},
				MediaData: []byte("audio-bytes"),
				MediaMIME: "audio/mpeg",
			}},
		}},
	}

	html, assets, err := render.HTMLWithOptions(pres, render.HTMLOptions{
		ImageMode:        render.HTMLImageModeExternal,
		ExternalImageDir: "sample-1_files",
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `src="sample-1_files/media-001.mp3"`) {
		t.Fatalf("expected external media src in html: %s", html)
	}
	if len(assets) != 1 {
		t.Fatalf("expected one sidecar media asset, got %d", len(assets))
	}
	if assets[0].Path != "sample-1_files/media-001.mp3" {
		t.Fatalf("unexpected asset path: %s", assets[0].Path)
	}
	if assets[0].ContentType != "audio/mpeg" {
		t.Fatalf("unexpected asset content type: %s", assets[0].ContentType)
	}
	if string(assets[0].Data) != "audio-bytes" {
		t.Fatalf("unexpected asset bytes: %q", string(assets[0].Data))
	}
}

func TestHTMLUsesPlaceholderForNonBrowserPresentationImages(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Index: 0,
			Shapes: []*resolve.Shape{{
				Kind:      resolve.ShapeImage,
				Name:      "VectorLogo",
				Bounds:    resolve.Rect{W: 1000, H: 1000},
				ImageData: []byte("vector-bytes"),
				ImageMIME: "image/x-emf",
			}},
		}},
	}

	html, assets, err := render.HTMLWithOptions(pres, render.HTMLOptions{
		ImageMode:        render.HTMLImageModeExternal,
		ExternalImageDir: "sample-1_files",
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(html, `<img`) {
		t.Fatalf("expected placeholder instead of img tag: %s", html)
	}
	if !strings.Contains(html, `shape--image-placeholder`) || !strings.Contains(html, `[image/x-emf]`) {
		t.Fatalf("expected emf placeholder in html: %s", html)
	}
	if len(assets) != 0 {
		t.Fatalf("expected no sidecar assets for placeholder images, got %d", len(assets))
	}
}

func TestHTMLExternalizesFontsWhenConfigured(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		EmbeddedFonts: []*resolve.EmbeddedFont{{
			Family: "Source Code Pro",
			Faces: []*resolve.EmbeddedFontFace{{
				Family:      "Source Code Pro",
				Weight:      400,
				Style:       "normal",
				ContentType: "font/truetype",
				Data:        []byte("true-embedded-font"),
			}},
		}},
	}

	html, assets, err := render.HTMLWithOptions(pres, render.HTMLOptions{
		FontMode:        render.HTMLFontModeExternal,
		ExternalFontDir: "sample-1_fonts",
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(html, `data:font/truetype;base64,`) {
		t.Fatalf("expected external font src in html: %s", html)
	}
	if !strings.Contains(html, `url("sample-1_fonts/font-001.ttf")`) {
		t.Fatalf("expected sidecar font src in html: %s", html)
	}
	if len(assets) != 1 {
		t.Fatalf("expected one sidecar font asset, got %d", len(assets))
	}
	if assets[0].Path != "sample-1_fonts/font-001.ttf" {
		t.Fatalf("unexpected asset path: %s", assets[0].Path)
	}
	if assets[0].ContentType != "font/truetype" {
		t.Fatalf("unexpected asset content type: %s", assets[0].ContentType)
	}
	if string(assets[0].Data) != "true-embedded-font" {
		t.Fatalf("unexpected asset bytes: %q", string(assets[0].Data))
	}
}

func TestHTMLExternalizesAllAssetsWhenConfigured(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		EmbeddedFonts: []*resolve.EmbeddedFont{{
			Family: "Source Code Pro",
			Faces: []*resolve.EmbeddedFontFace{{
				Family:      "Source Code Pro",
				Weight:      400,
				Style:       "normal",
				ContentType: "font/truetype",
				Data:        []byte("true-embedded-font"),
			}},
		}},
		Slides: []*resolve.Slide{{
			Index: 0,
			Shapes: []*resolve.Shape{{
				Kind:      resolve.ShapeImage,
				Name:      "Headshot",
				Bounds:    resolve.Rect{W: 1000, H: 1000},
				ImageData: []byte("image-bytes"),
				ImageMIME: "image/png",
			}},
		}},
	}

	html, assets, err := render.HTMLWithOptions(pres, render.HTMLOptions{
		ImageMode:        render.HTMLImageModeExternal,
		ExternalImageDir: "sample-1_files",
		FontMode:         render.HTMLFontModeExternal,
		ExternalFontDir:  "sample-1_files",
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(html, `data:image/`) || strings.Contains(html, `data:font/`) {
		t.Fatalf("expected no inline asset URLs in html: %s", html)
	}
	if !strings.Contains(html, `src="sample-1_files/image-001.png"`) {
		t.Fatalf("expected external image src in html: %s", html)
	}
	if !strings.Contains(html, `url("sample-1_files/font-001.ttf")`) {
		t.Fatalf("expected external font src in html: %s", html)
	}
	if len(assets) != 2 {
		t.Fatalf("expected two sidecar assets, got %d", len(assets))
	}
}

func TestHTMLResetsAutoNumberAcrossSpacerParagraphs(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{
				Index: 0,
				Shapes: []*resolve.Shape{
					{
						Kind:   resolve.ShapeText,
						Bounds: resolve.Rect{W: 1000, H: 1000},
						Paragraphs: []*resolve.Paragraph{
							{AutoNum: true, AutoNumType: "arabicPeriod", Runs: []*resolve.Run{{Text: "First"}}},
							{Runs: []*resolve.Run{}},
							{AutoNum: true, AutoNumType: "arabicPeriod", Runs: []*resolve.Run{{Text: "Second"}}},
						},
					},
				},
			},
		},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Count(html, `<span class="bullet-marker">1.</span>`) != 2 {
		t.Fatalf("expected numbering reset after spacer paragraph: %s", html)
	}
}

func TestHTMLRendersMarkerStyle(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{
				Index: 0,
				Shapes: []*resolve.Shape{
					{
						Kind:   resolve.ShapeText,
						Bounds: resolve.Rect{W: 1000, H: 1000},
						Paragraphs: []*resolve.Paragraph{
							{
								AutoNum:          true,
								AutoNumType:      "arabicPeriod",
								BulletSize:       2400,
								BulletFontFamily: "Arial",
								BulletColor:      "111111",
								Runs:             []*resolve.Run{{Text: "Styled marker"}},
							},
						},
					},
				},
			},
		},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `<span class="bullet-marker" style="font-size:24.0pt;font-family:Arial,sans-serif;color:#111111">1.</span>`) {
		t.Fatalf("expected marker style in html: %s", html)
	}
}

func TestHTMLRendersTextBodyAndParagraphStyles(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{
				Index: 0,
				Shapes: []*resolve.Shape{
					{
						Kind:               resolve.ShapeText,
						Bounds:             resolve.Rect{W: 1000, H: 1000},
						TextAnchor:         "ctr",
						TextInsetTopEMU:    91440,
						TextInsetRightEMU:  91440,
						TextInsetBottomEMU: 91440,
						TextInsetLeftEMU:   91440,
						Paragraphs: []*resolve.Paragraph{
							{
								Align:          "ctr",
								SpaceBefore:    0,
								SpaceAfter:     0,
								HasSpaceBefore: true,
								HasSpaceAfter:  true,
								LineSpacingPct: 100000,
								HasLineSpacing: true,
								Runs:           []*resolve.Run{{Text: "Styled paragraph"}},
							},
						},
					},
				},
			},
		},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `justify-content:center`) || !strings.Contains(html, `padding:7.20pt 7.20pt 7.20pt 7.20pt`) {
		t.Fatalf("expected text body styles in html: %s", html)
	}
	if !strings.Contains(html, `<p style="text-align:center;margin-top:0.00pt;margin-bottom:0.00pt;line-height:1.0000">`) {
		t.Fatalf("expected paragraph styles in html: %s", html)
	}
}

func TestHTMLRendersNoAutofitTextOverflowVisible(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Index: 0,
			Shapes: []*resolve.Shape{{
				Kind:              resolve.ShapeText,
				Bounds:            resolve.Rect{W: 9144000, H: 6858000},
				TextAllowOverflow: true,
				Paragraphs:        []*resolve.Paragraph{{Runs: []*resolve.Run{{Text: "Project name"}}}},
			}},
		}},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `<div class="shape shape--text" style="left:0.0000%;top:0.0000%;width:100.0000%;height:100.0000%;overflow:visible">`) {
		t.Fatalf("expected no-autofit text shape to allow overflow: %s", html)
	}
}

func TestHTMLAppliesNormAutofitFontAndLineScaling(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{
				Index: 0,
				Shapes: []*resolve.Shape{
					{
						Kind:   resolve.ShapeText,
						Bounds: resolve.Rect{W: 9144000, H: 6858000},
						TextBodyProps: drawing.TextBodyProperties{
							AutofitType:               "normAutofit",
							AutofitFontScale:          92000,
							AutofitLineSpaceReduction: 18000,
						},
						Paragraphs: []*resolve.Paragraph{
							{
								HasLineSpacing: true,
								LineSpacingPct: 100000,
								AutoNum:        true,
								AutoNumType:    "arabicPeriod",
								BulletSize:     2400,
								Runs: []*resolve.Run{
									{
										Text:     "Scaled text",
										FontSize: 2400,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `line-height:0.8200`) {
		t.Fatalf("expected normAutofit line spacing reduction in html: %s", html)
	}
	if !strings.Contains(html, `font-size:22.1pt`) {
		t.Fatalf("expected normAutofit font scaling in html: %s", html)
	}
}

func TestHTMLSpAutofitShapeUsesMinHeight(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{
				Index: 0,
				Shapes: []*resolve.Shape{
					{
						Kind:         resolve.ShapeText,
						Bounds:       resolve.Rect{W: 4572000, H: 1143000},
						TextAutoGrow: true,
						Paragraphs:   []*resolve.Paragraph{{Runs: []*resolve.Run{{Text: "Growing text", FontSize: 2400}}}},
					},
				},
			},
		},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(html, ";height:16.") {
		t.Fatalf("expected no fixed height on spAutoFit shape but found it: %s", html)
	}
	if !strings.Contains(html, `min-height:`) {
		t.Fatalf("expected min-height on spAutoFit shape outer div: %s", html)
	}
	if !strings.Contains(html, `height:auto`) {
		t.Fatalf("expected height:auto on spAutoFit shape content div: %s", html)
	}
	if !strings.Contains(html, `overflow:visible`) {
		t.Fatalf("expected overflow:visible on spAutoFit shape content div: %s", html)
	}
}

func TestHTMLNoAutofitInnerDivAllowsOverflow(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{
				Index: 0,
				Shapes: []*resolve.Shape{
					{
						Kind:              resolve.ShapeText,
						Bounds:            resolve.Rect{W: 4572000, H: 1143000},
						TextAllowOverflow: true,
						Paragraphs:        []*resolve.Paragraph{{Runs: []*resolve.Run{{Text: "Overflowing text", FontSize: 2400}}}},
					},
				},
			},
		},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `height:auto`) {
		t.Fatalf("expected height:auto on noAutofit shape content div: %s", html)
	}
	if !strings.Contains(html, `overflow:visible`) {
		t.Fatalf("expected overflow:visible on noAutofit shape content div: %s", html)
	}
}

func TestHTMLPreservesInsetsForSentinelPlaceholder(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Index: 0,
			Shapes: []*resolve.Shape{{
				Kind:                resolve.ShapeText,
				Bounds:              resolve.Rect{W: 9144000, H: 6858000},
				SentinelPlaceholder: true,
				TextInsetTopEMU:     91440,
				TextInsetRightEMU:   91440,
				TextInsetBottomEMU:  91440,
				TextInsetLeftEMU:    91440,
				Paragraphs:          []*resolve.Paragraph{{Runs: []*resolve.Run{{Text: "Project name"}}}},
			}},
		}},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `padding:7.20pt 7.20pt 7.20pt 7.20pt`) {
		t.Fatalf("expected sentinel placeholder to preserve all insets: %s", html)
	}
}

func TestHTMLRendersParagraphMarginAndIndent(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{
				Index: 0,
				Shapes: []*resolve.Shape{
					{
						Kind:   resolve.ShapeText,
						Bounds: resolve.Rect{W: 1000, H: 1000},
						Paragraphs: []*resolve.Paragraph{
							{
								MarginLeftEMU: 457200,
								TextIndentEMU: -228600,
								HasMarginLeft: true,
								HasTextIndent: true,
								Runs:          []*resolve.Run{{Text: "Indented bullet"}},
							},
						},
					},
				},
			},
		},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `margin-left:36.00pt`) || !strings.Contains(html, `text-indent:-18.00pt`) {
		t.Fatalf("expected paragraph margin and indent styles in html: %s", html)
	}
}

func TestHTMLAppliesPercentageSpacingBeforeAfter(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Index: 0,
			Shapes: []*resolve.Shape{{
				Kind:   resolve.ShapeText,
				Bounds: resolve.Rect{W: 4572000, H: 2286000},
				Paragraphs: []*resolve.Paragraph{{
					SpaceBeforePct: 100000, // 100% = 1em before
					SpaceAfterPct:  50000,  // 50% = 0.5em after
					Runs:           []*resolve.Run{{Text: "Spaced paragraph"}},
				}},
			}},
		}},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `margin-top:1.0000em`) {
		t.Fatalf("expected margin-top:1.0000em for 100%% spacing before: %s", html)
	}
	if !strings.Contains(html, `margin-bottom:0.5000em`) {
		t.Fatalf("expected margin-bottom:0.5000em for 50%% spacing after: %s", html)
	}
}

func TestHTMLRendersMultiColumnTextBody(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Index: 0,
			Shapes: []*resolve.Shape{{
				Kind:   resolve.ShapeText,
				Bounds: resolve.Rect{W: 4572000, H: 2286000},
				TextBodyProps: drawing.TextBodyProperties{
					Columns:          2,
					ColumnSpacingEMU: 457200, // 36pt gap
				},
				Paragraphs: []*resolve.Paragraph{{
					Runs: []*resolve.Run{{Text: "Column text"}},
				}},
			}},
		}},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `column-count:2`) {
		t.Fatalf("expected column-count:2 in html: %s", html)
	}
	if !strings.Contains(html, `column-gap:36.00pt`) {
		t.Fatalf("expected column-gap:36.00pt in html: %s", html)
	}
}

func TestHTMLDoesNotForceTitlePlaceholderBold(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{
				Index: 0,
				Shapes: []*resolve.Shape{
					{
						Kind:        resolve.ShapeText,
						Placeholder: resolve.PlaceholderTitle,
						Bounds:      resolve.Rect{W: 1000, H: 1000},
						Paragraphs: []*resolve.Paragraph{
							{Runs: []*resolve.Run{{Text: "Skills & expertise", FontFamily: "Oswald", FontSize: 3000}}},
						},
					},
				},
			},
		},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(html, `.ph-title { font-weight: bold; }`) {
		t.Fatalf("expected title placeholders to rely on run styling instead of forced bold: %s", html)
	}
}

func TestHTMLRendersTriangleGeometryAndRotation(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{
				Index: 0,
				Shapes: []*resolve.Shape{
					{
						Kind:        resolve.ShapeText,
						Bounds:      resolve.Rect{W: 1000, H: 1000},
						FillColor:   "E91D63",
						Geometry:    "triangle",
						RotationDeg: 180,
						Paragraphs: []*resolve.Paragraph{
							{Runs: []*resolve.Run{{Text: ""}}},
						},
					},
				},
			},
		},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `<svg class="shape__vector" viewBox="0 0 0.0787 0.0787" preserveAspectRatio="none"`) {
		t.Fatalf("expected triangle vector svg in html: %s", html)
	}
	if !strings.Contains(html, `d="M 0.0000 0.0787 L 0.0394 0.0000 L 0.0787 0.0787 Z" fill="#E91D63"`) {
		t.Fatalf("expected triangle path in html: %s", html)
	}
	if !strings.Contains(html, `transform:rotate(180.00deg);transform-origin:50% 50%`) {
		t.Fatalf("expected triangle rotation in html: %s", html)
	}
}

func TestHTMLRendersEllipseGeometryViaSharedPath(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{
				Index: 0,
				Shapes: []*resolve.Shape{
					{
						Kind:      resolve.ShapeText,
						Bounds:    resolve.Rect{W: 914400, H: 914400},
						FillColor: "673AB7",
						Geometry:  "ellipse",
						Paragraphs: []*resolve.Paragraph{
							{Runs: []*resolve.Run{{Text: ""}}},
						},
					},
				},
			},
		},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `<svg class="shape__vector" viewBox="0 0 72.0000 72.0000" preserveAspectRatio="none"`) {
		t.Fatalf("expected ellipse vector svg in html: %s", html)
	}
	if !strings.Contains(html, `<ellipse cx="36.0000" cy="36.0000" rx="36.0000" ry="36.0000" fill="#673AB7"/>`) {
		t.Fatalf("expected ellipse element in html: %s", html)
	}
}

func TestHTMLRendersAdvancedArrowCalloutGeometry(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Index: 0,
			Shapes: []*resolve.Shape{{
				Kind:        resolve.ShapeText,
				Bounds:      resolve.Rect{W: 2540000, H: 1270000},
				FillColor:   "FF7043",
				LineColor:   "1E1E1E",
				LineWidthPt: 1.5,
				Geometry:    "rightArrowCallout",
				Paragraphs:  []*resolve.Paragraph{{Runs: []*resolve.Run{{Text: ""}}}},
			}},
		}},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `<svg class="shape__vector" viewBox="0 0 200.0000 100.0000" preserveAspectRatio="none"`) {
		t.Fatalf("expected arrow callout svg in html: %s", html)
	}
	if !strings.Contains(html, `L 200.0000 50.0000`) {
		t.Fatalf("expected right-arrow callout tip in html path: %s", html)
	}
}

func TestHTMLUsesSharedFillForSVGShapes(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{
				Index: 0,
				Shapes: []*resolve.Shape{
					{
						Kind:     resolve.ShapeText,
						Bounds:   resolve.Rect{W: 914400, H: 914400},
						Geometry: "ellipse",
						Fill: drawing.SolidFill{
							Color: "673AB7",
							Alpha: 0.5,
						},
						Paragraphs: []*resolve.Paragraph{
							{Runs: []*resolve.Run{{Text: ""}}},
						},
					},
				},
			},
		},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `fill="rgba(103, 58, 183, 0.50)"`) {
		t.Fatalf("expected shared fill CSS value in html: %s", html)
	}
}

func TestHTMLUsesSharedEffectsForSVGShapes(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{
				Index: 0,
				Shapes: []*resolve.Shape{
					{
						Kind:      resolve.ShapeText,
						Bounds:    resolve.Rect{W: 914400, H: 914400},
						Geometry:  "ellipse",
						FillColor: "673AB7",
						Effects: drawing.EffectList{
							Effects: []drawing.Effect{
								drawing.Shadow{Color: "000000", Alpha: 0.25, Blur: 2.0, Distance: 3.0, Angle: 45.0},
								drawing.Reflection{Alpha: 0.20, Blur: 1.5, Distance: 4.0, Direction: 90.0},
							},
						},
						Paragraphs: []*resolve.Paragraph{
							{Runs: []*resolve.Run{{Text: ""}}},
						},
					},
				},
			},
		},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `overflow:visible`) {
		t.Fatalf("expected effect shapes to allow overflow in html: %s", html)
	}
	if !strings.Contains(html, `filter:drop-shadow(`) {
		t.Fatalf("expected shared effect filter in html: %s", html)
	}
	if !strings.Contains(html, `box-shadow:`) {
		t.Fatalf("expected shared reflection box-shadow in html: %s", html)
	}
}

func TestHTMLRendersBentConnectorPolyline(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{
				Index: 0,
				Shapes: []*resolve.Shape{
					{
						Kind:                 resolve.ShapeConnector,
						Bounds:               resolve.Rect{X: 914400, Y: 685800, W: 1828800, H: 685800},
						LineColor:            "595959",
						LineWidthPt:          2.25,
						ConnectorType:        "bentConnector3",
						ConnectorFlipH:       true,
						ConnectorAdjustments: []int{50000},
					},
				},
			},
		},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	// Slide 720×540pt. Bounds X=72pt,Y=54pt,W=144pt,H=54pt, flipH → start=(216,54) end=(72,108).
	// adj1=50000 → bend at midpoint y=81pt. Points: (216,54)→(216,81)→(72,81)→(72,108).
	if !strings.Contains(html, `<polyline points="216.0000,54.0000 144.0000,54.0000 144.0000,108.0000 72.0000,108.0000"`) {
		t.Fatalf("expected bent connector polyline in html: %s", html)
	}
}

func TestHTMLRendersConnectorTailArrowGeometry(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{
				Index: 0,
				Shapes: []*resolve.Shape{
					{
						Kind:            resolve.ShapeConnector,
						Bounds:          resolve.Rect{X: 1828800, Y: 685800, W: 1828800, H: 0},
						LineColor:       "595959",
						LineWidthPt:     3.0,
						ConnectorType:   "straightConnector1",
						ArrowTail:       "triangle",
						ArrowTailWidth:  "lg",
						ArrowTailLength: "lg",
					},
				},
			},
		},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `<line x1="144.0000" y1="54.0000" x2="276.0000" y2="54.0000" stroke="#595959" stroke-width="3.00"/>`) {
		t.Fatalf("expected line trimmed to the tail arrow base: %s", html)
	}
	if !strings.Contains(html, `<polygon points="288.0000,54.0000 276.0000,58.0000 276.0000,50.0000" fill="#595959"/>`) {
		t.Fatalf("expected acute tail arrow polygon in html: %s", html)
	}
}

func TestHTMLRendersConnectorSharedLineStyle(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{
				Index: 0,
				Shapes: []*resolve.Shape{
					{
						Kind:          resolve.ShapeConnector,
						Bounds:        resolve.Rect{X: 1828800, Y: 685800, W: 1828800, H: 0},
						LineColor:     "595959",
						LineWidthPt:   3.0,
						ConnectorType: "straightConnector1",
						LineStyle: &drawing.LineStyle{
							Width: 3.0,
							Color: "595959",
							Cap:   drawing.LineCapRound,
							Join:  drawing.LineJoinBevel,
							Dash:  drawing.DashPatternDashDot,
						},
					},
				},
			},
		},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `stroke-linecap="round"`) || !strings.Contains(html, `stroke-linejoin="bevel"`) || !strings.Contains(html, `stroke-dasharray="3, 3, 1, 3"`) {
		t.Fatalf("expected shared line-style attrs in html: %s", html)
	}
}

func TestHTMLAppliesArrowSizePresets(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{
				Index: 0,
				Shapes: []*resolve.Shape{
					{
						Kind:            resolve.ShapeConnector,
						Bounds:          resolve.Rect{X: 1828800, Y: 685800, W: 1828800, H: 0},
						LineColor:       "595959",
						LineWidthPt:     3.0,
						ConnectorType:   "straightConnector1",
						ArrowTail:       "triangle",
						ArrowTailWidth:  "sm",
						ArrowTailLength: "sm",
					},
					{
						Kind:            resolve.ShapeConnector,
						Bounds:          resolve.Rect{X: 1828800, Y: 1371600, W: 1828800, H: 0},
						LineColor:       "595959",
						LineWidthPt:     3.0,
						ConnectorType:   "straightConnector1",
						ArrowTail:       "triangle",
						ArrowTailWidth:  "lg",
						ArrowTailLength: "lg",
					},
				},
			},
		},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `<line x1="144.0000" y1="54.0000" x2="282.0000" y2="54.0000" stroke="#595959" stroke-width="3.00"/>`) {
		t.Fatalf("expected small arrow length to trim the line by 6pt: %s", html)
	}
	if !strings.Contains(html, `<polygon points="288.0000,54.0000 282.0000,56.2500 282.0000,51.7500" fill="#595959"/>`) {
		t.Fatalf("expected small arrow width to render narrower triangle: %s", html)
	}
	if !strings.Contains(html, `<line x1="144.0000" y1="108.0000" x2="276.0000" y2="108.0000" stroke="#595959" stroke-width="3.00"/>`) {
		t.Fatalf("expected large arrow length to trim the line by 12pt: %s", html)
	}
	if !strings.Contains(html, `<polygon points="288.0000,108.0000 276.0000,112.0000 276.0000,104.0000" fill="#595959"/>`) {
		t.Fatalf("expected large arrow width to render wider triangle: %s", html)
	}
}

func TestHTMLRendersConnectorBoundingBox(t *testing.T) {
	// Connector geometry always comes from the xfrm bounding box.
	// Slide 720×540pt. Bounds X=144pt,Y=54pt,W=144pt,H=108pt.
	// straightConnector1: start=(144,54), end=(288,162).
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{
				Index: 0,
				Shapes: []*resolve.Shape{
					{
						Kind:          resolve.ShapeConnector,
						Bounds:        resolve.Rect{X: 1828800, Y: 685800, W: 1828800, H: 1371600},
						LineColor:     "595959",
						LineWidthPt:   2.25,
						ConnectorType: "straightConnector1",
					},
				},
			},
		},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `<line x1="144.0000" y1="54.0000" x2="288.0000" y2="162.0000"`) {
		t.Fatalf("expected line to use bounding-box coordinates: %s", html)
	}
}

func TestHTMLRendersPresetShapePathViaSharedPrimitives(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{
				Index: 0,
				Shapes: []*resolve.Shape{
					{
						Kind:        resolve.ShapeText,
						Bounds:      resolve.Rect{X: 914400, Y: 685800, W: 1524000, H: 762000},
						Geometry:    "triangle",
						FillColor:   "E91D63",
						LineColor:   "222222",
						LineWidthPt: 1.5,
						Paragraphs:  []*resolve.Paragraph{{Runs: []*resolve.Run{{Text: ""}}}},
					},
				},
			},
		},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `<path d="M 0.0000 60.0000 L 60.0000 0.0000 L 120.0000 60.0000 Z" fill="#E91D63"`) {
		t.Fatalf("expected shared triangle path in html: %s", html)
	}
	if !strings.Contains(html, `stroke="#222222" stroke-width="1.50"`) {
		t.Fatalf("expected stroke attrs on shared shape path: %s", html)
	}
}

func TestHTMLRendersCustomShapePathViaSharedPrimitives(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{
				Index: 0,
				Shapes: []*resolve.Shape{
					{
						Kind:   resolve.ShapeText,
						Bounds: resolve.Rect{X: 914400, Y: 685800, W: 1524000, H: 762000},
						GeometryData: &drawing.Geometry{
							Type: drawing.GeometryTypeCustom,
							Paths: []drawing.Path{{
								FillRule: "nonzero",
								Commands: []drawing.PathCommand{
									{Type: "M", Points: []drawing.Point{{X: 0, Y: 1}}},
									{Type: "L", Points: []drawing.Point{{X: 0.5, Y: 0}}},
									{Type: "L", Points: []drawing.Point{{X: 1, Y: 1}}},
									{Type: "Z"},
								},
							}},
						},
						FillColor:   "E91D63",
						LineColor:   "222222",
						LineWidthPt: 1.5,
						Paragraphs:  []*resolve.Paragraph{{Runs: []*resolve.Run{{Text: ""}}}},
					},
				},
			},
		},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `<path d="M 0.0000 60.0000 L 60.0000 0.0000 L 120.0000 60.0000 Z" fill="#E91D63"`) {
		t.Fatalf("expected shared custom path in html: %s", html)
	}
}

func TestHTMLRendersThreeDShapeEffects(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Index: 0,
			Shapes: []*resolve.Shape{{
				Kind:       resolve.ShapeText,
				Bounds:     resolve.Rect{W: 1828800, H: 914400},
				Geometry:   "ellipse",
				FillColor:  "FF7043",
				Paragraphs: []*resolve.Paragraph{{Runs: []*resolve.Run{{Text: ""}}}},
				Effects: drawing.EffectList{Effects: []drawing.Effect{drawing.ThreeD{
					CameraPreset:   "legacyPerspectiveTopLeft",
					CameraFOV:      45,
					CameraZoom:     1.1,
					LightRig:       "threePt",
					LightDirection: "tl",
					RotationX:      -15,
					RotationY:      -10,
					ExtrusionDepth: 6,
					ContourWidth:   2,
					BevelTopHeight: 1,
					BevelTopPreset: "relaxedInset",
					Material:       "metal",
					ExtrusionColor: "595959",
				}}},
			}},
		}},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `perspective(`) || !strings.Contains(html, `rotateX(-15.0000deg)`) || !strings.Contains(html, `rotateY(10.0000deg)`) {
		t.Fatalf("expected camera-aware 3D transform in html: %s", html)
	}
	if !strings.Contains(html, `shape__3d-extrusion`) || !strings.Contains(html, `data-depth="6.00"`) {
		t.Fatalf("expected projected extrusion layers in html: %s", html)
	}
	if !strings.Contains(html, `--ppt-3d-material:metal`) || !strings.Contains(html, `--ppt-3d-light-rig:threePt`) {
		t.Fatalf("expected 3D material/light metadata in html: %s", html)
	}
	if !strings.Contains(html, `drop-shadow(`) {
		t.Fatalf("expected 3D drop shadow in html: %s", html)
	}
	if !strings.Contains(html, `mix-blend-mode:screen`) || !strings.Contains(html, `mix-blend-mode:multiply`) {
		t.Fatalf("expected 3D scene-light overlays in html: %s", html)
	}
	if !strings.Contains(html, `shape-fill-overlay-0`) || !strings.Contains(html, `shape-fill-overlay-1`) {
		t.Fatalf("expected SVG defs for 3D light gradients in html: %s", html)
	}
}

func TestPptxCorpusRender(t *testing.T) {
	fixturesDir := repoPath(t, "testdata", "pptx")
	entries, err := os.ReadDir(fixturesDir)
	if err != nil {
		t.Fatalf("read fixtures dir: %v", err)
	}

	var count, totalDiags int
	for _, entry := range entries {
		if entry.IsDir() || !isPptxFixtureName(entry.Name()) {
			continue
		}
		count++
		path := filepath.Join(fixturesDir, entry.Name())
		t.Run(entry.Name(), func(t *testing.T) {
			pkg, err := opc.Open(path)
			if err != nil {
				if opc.IsEncryptedPackageError(err) {
					t.Skip("encrypted package requires password")
				}
				if strings.Contains(err.Error(), "zip: not a valid zip file") {
					t.Skip("fixture is not a valid OPC zip package")
				}
				t.Fatalf("open: %v", err)
			}
			if pkg.PackageType() != opc.PackageTypePresentation {
				t.Skipf("fixture package type = %q, want %q", pkg.PackageType(), opc.PackageTypePresentation)
			}
			pptPkg, err := parts.LoadPackage(pkg)
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			pres, err := resolve.ResolvePackage(pptPkg)
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			// Count only warnings and errors; info diagnostics (e.g. no-theme) are expected.
			for _, d := range pres.Diagnostics {
				if d.Severity != "info" {
					totalDiags++
				}
			}
			html, err := render.HTML(pres)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if !strings.Contains(html, "<!DOCTYPE html>") || !strings.Contains(html, "class=\"slide\"") {
				t.Fatalf("unexpected HTML output")
			}
			if !strings.Contains(html, "class=\"canvas\"") {
				t.Fatalf("no canvas divs rendered")
			}
		})
	}

	if count == 0 {
		t.Fatal("no pptx fixtures found")
	}
	if totalDiags != 0 {
		t.Fatalf("total corpus warning/error diagnostics = %d, want 0", totalDiags)
	}
}

func repoPath(t *testing.T, elems ...string) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
	return filepath.Join(append([]string{root}, elems...)...)
}

func isPptxFixtureName(name string) bool {
	return strings.EqualFold(filepath.Ext(name), ".pptx") && !strings.HasPrefix(name, "~$")
}

func TestHTMLCutoutShapesPreserveEvenoddFillRule(t *testing.T) {
	shapes := []struct {
		name      string
		geometry  string
		fillColor string
	}{
		{"donut", "donut", "4472C4"},
		{"frame", "frame", "70AD47"},
		{"smileyFace", "smileyFace", "FFC000"},
	}

	for _, s := range shapes {
		t.Run(s.name, func(t *testing.T) {
			pres := &resolve.Presentation{
				SlideWidthEMU:  9144000,
				SlideHeightEMU: 6858000,
				Slides: []*resolve.Slide{
					{
						Index: 0,
						Shapes: []*resolve.Shape{
							{
								Kind:      resolve.ShapeText,
								Bounds:    resolve.Rect{W: 914400, H: 914400},
								FillColor: s.fillColor,
								Geometry:  s.geometry,
								Paragraphs: []*resolve.Paragraph{
									{Runs: []*resolve.Run{{Text: ""}}},
								},
							},
						},
					},
				},
			}

			html, err := render.HTML(pres)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if !strings.Contains(html, `fill-rule="evenodd"`) {
				t.Fatalf("%s: expected fill-rule=\"evenodd\" in SVG output\n%s", s.name, html)
			}
			if !strings.Contains(html, `fill="#`+s.fillColor+`"`) {
				t.Fatalf("%s: expected fill color #%s in SVG output\n%s", s.name, s.fillColor, html)
			}
		})
	}
}

func TestHTMLRendersPictureFillOnAutoShape(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Index: 0,
			Shapes: []*resolve.Shape{{
				Kind:   resolve.ShapeText,
				Bounds: resolve.Rect{W: 914400, H: 685800},
				Fill: drawing.PictureFill{
					ImageData:   []byte{0x89, 0x50, 0x4E, 0x47},
					ContentType: "image/png",
					Crop:        &drawing.Rect{X: 0.1, Y: 0.0, Width: 0.8, Height: 1.0},
				},
			}},
		}},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `<svg class="shape__vector"`) || !strings.Contains(html, `<pattern id="shape-fill"`) {
		t.Fatalf("expected SVG pattern definition for picture fill\n%s", html)
	}
	if !strings.Contains(html, `fill="url(#shape-fill)"`) || !strings.Contains(html, `<image href="data:image/png;base64,`) {
		t.Fatalf("expected picture fill image materialized into SVG output\n%s", html)
	}
}

func TestHTMLRendersPatternFillOnAutoShape(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Index: 0,
			Shapes: []*resolve.Shape{{
				Kind:   resolve.ShapeText,
				Bounds: resolve.Rect{W: 914400, H: 685800},
				Fill: drawing.PatternFill{
					PatternType: "smGrid",
					Foreground:  "FF0000",
					Background:  "FFFFFF",
				},
			}},
		}},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `<pattern id="shape-fill"`) || !strings.Contains(html, `fill="url(#shape-fill)"`) {
		t.Fatalf("expected SVG pattern fill definition in output\n%s", html)
	}
	if !strings.Contains(html, `stroke="#FF0000"`) || !strings.Contains(html, `fill="#FFFFFF"`) {
		t.Fatalf("expected foreground and background pattern styling in output\n%s", html)
	}
}

func TestHTMLRendersSlideNumberField(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Index: 2, // 0-based; should render as slide 3
			Shapes: []*resolve.Shape{{
				Kind:   resolve.ShapeText,
				Bounds: resolve.Rect{W: 914400, H: 685800},
				Paragraphs: []*resolve.Paragraph{{
					Runs: []*resolve.Run{{
						FieldKind: "slidenum",
						Text:      "",
					}},
				}},
			}},
		}},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, "3") {
		t.Fatalf("expected slide number 3 in output\n%s", html)
	}
}

func TestHTMLRendersSlideNumberPlaceholder(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Index: 41, // 0-based; should render as slide 42
			SlideNumber: &resolve.Shape{
				Kind:   resolve.ShapeText,
				Bounds: resolve.Rect{W: 914400, H: 685800},
				Paragraphs: []*resolve.Paragraph{{
					Runs: []*resolve.Run{{
						FieldKind: "slidenum",
						Bold:      true,
					}},
				}},
			},
		}},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `class="slide-number"`) {
		t.Fatalf("expected slide-number placeholder wrapper in output\n%s", html)
	}
	if !strings.Contains(html, `>42</span>`) {
		t.Fatalf("expected slide number 42 in placeholder output\n%s", html)
	}
}

func TestHTMLRenderPPTXMathRun(t *testing.T) {
	mathML := `<math xmlns="http://www.w3.org/1998/Math/MathML"><mi>x</mi></math>`
	prs := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{
				Index: 0,
				Shapes: []*resolve.Shape{
					{
						Kind:   resolve.ShapeText,
						Bounds: resolve.Rect{W: 1000, H: 1000},
						Paragraphs: []*resolve.Paragraph{
							{
								Runs: []*resolve.Run{
									{MathML: mathML},
								},
							},
						},
					},
				},
			},
		},
	}
	html, err := render.HTML(prs)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(html, "<math") {
		t.Fatalf("expected <math> in PPTX render output\n%s", html)
	}
	if !strings.Contains(html, "<mi>x</mi>") {
		t.Fatalf("expected <mi>x</mi> in PPTX render output\n%s", html)
	}
}

func TestHTMLRendersParagraphRightMargin(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Index: 0,
			Shapes: []*resolve.Shape{{
				Kind:   resolve.ShapeText,
				Bounds: resolve.Rect{W: 1000, H: 1000},
				Paragraphs: []*resolve.Paragraph{{
					MarginRightEMU: 457200,
					HasMarginRight: true,
					Runs:           []*resolve.Run{{Text: "Right margined"}},
				}},
			}},
		}},
	}
	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, "margin-right:36.00pt") {
		t.Fatalf("expected margin-right style in html: %s", html)
	}
}

func TestHTMLRendersParagraphFontAlign(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Index: 0,
			Shapes: []*resolve.Shape{{
				Kind:   resolve.ShapeText,
				Bounds: resolve.Rect{W: 1000, H: 1000},
				Paragraphs: []*resolve.Paragraph{{
					FontAlign: "ctr",
					Runs:      []*resolve.Run{{Text: "Center-aligned font"}},
				}},
			}},
		}},
	}
	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, "vertical-align:middle") {
		t.Fatalf("expected vertical-align:middle in html: %s", html)
	}
}

func TestHTMLRendersBulletSizePercentage(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Index: 0,
			Shapes: []*resolve.Shape{{
				Kind:   resolve.ShapeText,
				Bounds: resolve.Rect{W: 1000, H: 1000},
				Paragraphs: []*resolve.Paragraph{{
					BulletChar:    "•",
					BulletSizePct: 150000,                                                // 150% of base font size
					Runs:          []*resolve.Run{{Text: "Bullet text", FontSize: 1200}}, // 12pt base
				}},
			}},
		}},
	}
	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	// 12pt * 1.5 = 18pt
	if !strings.Contains(html, "font-size:18.0pt") {
		t.Fatalf("expected font-size:18.0pt for 150%% bullet size of 12pt in html: %s", html)
	}
}

func TestHTMLRendersTabStopsMetadata(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Index: 0,
			Shapes: []*resolve.Shape{{
				Kind:   resolve.ShapeText,
				Bounds: resolve.Rect{W: 1000, H: 1000},
				Paragraphs: []*resolve.Paragraph{{
					TabStops: []resolve.TabStop{
						{Pos: 457200, Algn: "l"},
						{Pos: 914400, Algn: "ctr"},
					},
					Runs: []*resolve.Run{{Text: "Tabbed text\twith tabs"}},
				}},
			}},
		}},
	}
	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `data-tab-stops="457200:l,914400:ctr"`) {
		t.Fatalf("expected data-tab-stops attribute in html: %s", html)
	}
	if !strings.Contains(html, "tab-size:") {
		t.Fatalf("expected tab-size CSS in html: %s", html)
	}
}

func TestHTMLRendersSmartArtTabStopsMetadata(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Index: 0,
			Shapes: []*resolve.Shape{{
				Kind:   resolve.ShapeSmartArt,
				Bounds: resolve.Rect{W: 1000, H: 1000},
				Paragraphs: []*resolve.Paragraph{{
					DefaultTabSizeEMU: 914400,
					TabStops: []resolve.TabStop{
						{Pos: 457200, Algn: "l"},
						{Pos: 914400, Algn: "ctr"},
					},
					Runs: []*resolve.Run{{Text: "Tabbed SmartArt\twith tabs"}},
				}},
			}},
		}},
	}
	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `data-tab-stops="457200:l,914400:ctr"`) {
		t.Fatalf("expected SmartArt tab stops metadata in html: %s", html)
	}
	if !strings.Contains(html, "tab-size:72.0pt") {
		t.Fatalf("expected SmartArt default tab size CSS in html: %s", html)
	}
}

func TestHTMLRendersRunHighlight(t *testing.T) {
	pres := &resolve.Presentation{
		Slides: []*resolve.Slide{{
			Shapes: []*resolve.Shape{{
				Kind:   resolve.ShapeText,
				Bounds: resolve.Rect{X: 0, Y: 0, W: 1000000, H: 500000},
				Paragraphs: []*resolve.Paragraph{{
					Runs: []*resolve.Run{{
						Text:      "highlighted",
						Highlight: "FFFF00",
					}},
				}},
			}},
		}},
	}

	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "background-color:#FFFF00") {
		t.Fatalf("expected background-color CSS, got:\n%s", out)
	}
}

func TestHTMLRendersRunEffects(t *testing.T) {
	pres := &resolve.Presentation{
		Slides: []*resolve.Slide{{
			Shapes: []*resolve.Shape{{
				Kind:   resolve.ShapeSmartArt,
				Bounds: resolve.Rect{X: 0, Y: 0, W: 1000000, H: 500000},
				Paragraphs: []*resolve.Paragraph{{
					Runs: []*resolve.Run{{
						Text: "glow",
						Effects: drawing.EffectList{Effects: []drawing.Effect{
							drawing.Glow{Color: "00FF00", Alpha: 1, Radius: 2},
						}},
					}},
				}},
			}},
		}},
	}

	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "drop-shadow") {
		t.Fatalf("expected run effect CSS, got:\n%s", out)
	}
}

func TestHTMLRendersBodyPrRotation(t *testing.T) {
	pres := &resolve.Presentation{
		Slides: []*resolve.Slide{{
			Shapes: []*resolve.Shape{{
				Kind:   resolve.ShapeText,
				Bounds: resolve.Rect{X: 0, Y: 0, W: 1000000, H: 500000},
				TextBodyProps: drawing.TextBodyProperties{
					Rot: 900000,
				},
				Paragraphs: []*resolve.Paragraph{{
					Runs: []*resolve.Run{{Text: "rotated"}},
				}},
			}},
		}},
	}

	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "rotate(15.00deg)") {
		t.Fatalf("expected rotate(15.00deg) CSS, got:\n%s", out)
	}
}

func TestHTMLRendersBodyPrAnchorCtr(t *testing.T) {
	pres := &resolve.Presentation{
		Slides: []*resolve.Slide{{
			Shapes: []*resolve.Shape{{
				Kind:   resolve.ShapeText,
				Bounds: resolve.Rect{X: 0, Y: 0, W: 1000000, H: 500000},
				TextBodyProps: drawing.TextBodyProperties{
					AnchorCtr: true,
				},
				Paragraphs: []*resolve.Paragraph{{
					Runs: []*resolve.Run{{Text: "centered"}},
				}},
			}},
		}},
	}

	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "text-align:center") {
		t.Fatalf("expected text-align:center CSS, got:\n%s", out)
	}
}

func TestHTMLRendersBodyPrWrapNone(t *testing.T) {
	pres := &resolve.Presentation{
		Slides: []*resolve.Slide{{
			Shapes: []*resolve.Shape{{
				Kind:   resolve.ShapeText,
				Bounds: resolve.Rect{X: 0, Y: 0, W: 1000000, H: 500000},
				TextBodyProps: drawing.TextBodyProperties{
					Wrap: "none",
				},
				Paragraphs: []*resolve.Paragraph{{
					Runs: []*resolve.Run{{Text: "nowrap"}},
				}},
			}},
		}},
	}

	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "white-space:nowrap") {
		t.Fatalf("expected white-space:nowrap CSS, got:\n%s", out)
	}
}

func TestHTMLRendersEmphasisEffects(t *testing.T) {
	pres := &resolve.Presentation{
		Slides: []*resolve.Slide{{
			Shapes: []*resolve.Shape{{
				Kind:            resolve.ShapeText,
				Bounds:          resolve.Rect{X: 0, Y: 0, W: 1000000, H: 500000},
				EmphasisEffects: []string{"spin", "growShrink"},
				Paragraphs: []*resolve.Paragraph{{
					Runs: []*resolve.Run{{Text: "spinny"}},
				}},
			}},
		}},
	}

	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `data-anim-emphasis="spin growShrink"`) {
		t.Fatalf("expected data-anim-emphasis attribute, got:\n%s", out)
	}
}

func TestHTMLRendersSmartArtBodyPr(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{
				Shapes: []*resolve.Shape{
					{
						Kind:   resolve.ShapeSmartArt,
						Bounds: resolve.Rect{X: 914400, Y: 685800, W: 1828800, H: 1371600},
						TextBodyProps: drawing.TextBodyProperties{
							Anchor:    "b",
							AnchorCtr: true,
							Rot:       900000,
							Wrap:      "none",
							Vert:      "vert",
							InsetT:    100000,
							InsetB:    200000,
							InsetL:    300000,
							InsetR:    400000,
						},
						Paragraphs: []*resolve.Paragraph{
							{Runs: []*resolve.Run{{Text: "Node"}}},
						},
					},
				},
			},
		},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "justify-content:flex-end") {
		t.Errorf("expected bottom anchor justify-content, got: %s", out)
	}
	if !strings.Contains(out, "text-align:center") {
		t.Errorf("expected anchorCtr text-align, got: %s", out)
	}
	if !strings.Contains(out, "rotate(15.00deg)") {
		t.Errorf("expected bodyPr rotation, got: %s", out)
	}
	if !strings.Contains(out, "white-space:nowrap") {
		t.Errorf("expected wrap=none, got: %s", out)
	}
	if !strings.Contains(out, "writing-mode:vertical-rl") {
		t.Errorf("expected vertical text, got: %s", out)
	}
	if !strings.Contains(out, "padding:7.87pt 31.50pt 15.75pt 23.62pt") {
		t.Errorf("expected bodyPr insets as padding, got: %s", out)
	}
}

func TestHTMLRendersSmartArtSpAutoFit(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{
				Shapes: []*resolve.Shape{
					{
						Kind:   resolve.ShapeSmartArt,
						Bounds: resolve.Rect{X: 914400, Y: 685800, W: 1828800, H: 1371600},
						TextBodyProps: drawing.TextBodyProperties{
							AutofitType: "spAutoFit",
						},
						Paragraphs: []*resolve.Paragraph{
							{Runs: []*resolve.Run{{Text: "Growing SmartArt", FontSize: 2400}}},
						},
					},
				},
			},
		},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<div class="shape shape--smartart" style="position:absolute;left:10.0000%;top:10.0000%;width:20.0000%;min-height:20.0000%;overflow:visible">`) {
		t.Fatalf("expected spAutoFit SmartArt outer div to use min-height and visible overflow, got: %s", out)
	}
	if !strings.Contains(out, `left:0;top:0;right:0;height:auto;overflow:visible`) {
		t.Fatalf("expected spAutoFit SmartArt text div to auto-size, got: %s", out)
	}
}

func TestHTMLRendersSmartArtNoAutofitOverflow(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{
				Shapes: []*resolve.Shape{
					{
						Kind:   resolve.ShapeSmartArt,
						Bounds: resolve.Rect{X: 914400, Y: 685800, W: 1828800, H: 1371600},
						TextBodyProps: drawing.TextBodyProperties{
							AutofitType: "noAutofit",
						},
						Paragraphs: []*resolve.Paragraph{
							{Runs: []*resolve.Run{{Text: "Overflow SmartArt", FontSize: 2400}}},
						},
					},
				},
			},
		},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `height:20.0000%;overflow:visible`) {
		t.Fatalf("expected noAutofit SmartArt outer div to preserve height but allow overflow, got: %s", out)
	}
	if !strings.Contains(out, `inset:0;height:auto;overflow:visible`) {
		t.Fatalf("expected noAutofit SmartArt text div to allow overflow, got: %s", out)
	}
}

func TestHTMLRendersSmartArtNormAutofitScaling(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{
				Shapes: []*resolve.Shape{
					{
						Kind:   resolve.ShapeSmartArt,
						Bounds: resolve.Rect{X: 914400, Y: 685800, W: 1828800, H: 1371600},
						TextBodyProps: drawing.TextBodyProperties{
							AutofitType:               "normAutofit",
							AutofitFontScale:          92000,
							AutofitLineSpaceReduction: 18000,
						},
						Paragraphs: []*resolve.Paragraph{
							{
								HasLineSpacing: true,
								LineSpacingPct: 100000,
								Runs: []*resolve.Run{
									{Text: "Scaled SmartArt", FontSize: 2400},
								},
							},
						},
					},
				},
			},
		},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `line-height:0.8200`) {
		t.Fatalf("expected SmartArt normAutofit line reduction, got: %s", out)
	}
	if !strings.Contains(out, `font-size:22.1pt`) {
		t.Fatalf("expected SmartArt normAutofit font scaling, got: %s", out)
	}
}

func TestHTMLRendersSmartArtConnectorArrowsAndDash(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{
				Shapes: []*resolve.Shape{
					{
						Kind:           resolve.ShapeSmartArtConnector,
						Bounds:         resolve.Rect{X: 0, Y: 0, W: 9144000, H: 6858000},
						ConnectorStart: &resolve.Point{X: 914400, Y: 685800},
						ConnectorEnd:   &resolve.Point{X: 2743200, Y: 2057400},
						LineColor:      "FF0000",
						LineWidthPt:    2.0,
						LineStyle: &drawing.LineStyle{
							Color:   "FF0000",
							Width:   2.0,
							Dash:    drawing.DashPatternDash,
							HeadEnd: &drawing.Arrowhead{Type: drawing.ArrowheadTriangle, Size: drawing.ArrowheadSizeMedium, Length: drawing.ArrowheadSizeMedium},
							TailEnd: &drawing.Arrowhead{Type: drawing.ArrowheadTriangle, Size: drawing.ArrowheadSizeMedium, Length: drawing.ArrowheadSizeMedium},
						},
						ArrowHead:       "triangle",
						ArrowHeadWidth:  "med",
						ArrowHeadLength: "med",
						ArrowTail:       "triangle",
						ArrowTailWidth:  "med",
						ArrowTailLength: "med",
					},
				},
			},
		},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "stroke-dasharray") {
		t.Errorf("expected dash array, got: %s", out)
	}
	if !strings.Contains(out, "<polygon") {
		t.Errorf("expected arrowhead polygon, got: %s", out)
	}
}

func TestHTMLRendersSmartArtPresetGeometry(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{
				Shapes: []*resolve.Shape{
					{
						Kind:     resolve.ShapeSmartArt,
						Bounds:   resolve.Rect{X: 914400, Y: 685800, W: 1828800, H: 1371600},
						Fill:     drawing.SolidFill{Color: "4472C4"},
						Geometry: "trapezoid",
						Paragraphs: []*resolve.Paragraph{
							{Runs: []*resolve.Run{{Text: "Trap"}}},
						},
					},
				},
			},
		},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "<path") {
		t.Fatalf("expected SVG path for trapezoid, got: %s", out)
	}
	if !strings.Contains(out, "Trap") {
		t.Fatalf("expected text, got: %s", out)
	}
}

func TestHTMLRendersSmartArtCustomGeometry(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{
				Shapes: []*resolve.Shape{
					{
						Kind:   resolve.ShapeSmartArt,
						Bounds: resolve.Rect{X: 914400, Y: 685800, W: 1828800, H: 914400},
						Fill:   drawing.SolidFill{Color: "4472C4"},
						GeometryData: &drawing.Geometry{
							Type: drawing.GeometryTypeCustom,
							Paths: []drawing.Path{{
								FillRule: "nonzero",
								Commands: []drawing.PathCommand{
									{Type: "M", Points: []drawing.Point{{X: 0, Y: 1}}},
									{Type: "L", Points: []drawing.Point{{X: 0.5, Y: 0}}},
									{Type: "L", Points: []drawing.Point{{X: 1, Y: 1}}},
									{Type: "Z"},
								},
							}},
						},
						Paragraphs: []*resolve.Paragraph{
							{Runs: []*resolve.Run{{Text: "Custom"}}},
						},
					},
				},
			},
		},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<path d="M 0.0000 72.0000 L 72.0000 0.0000 L 144.0000 72.0000 Z"`) {
		t.Fatalf("expected SmartArt custom geometry path, got: %s", out)
	}
	if !strings.Contains(out, "Custom") {
		t.Fatalf("expected text, got: %s", out)
	}
}

func TestHTMLRendersSmartArtBullets(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{
				Shapes: []*resolve.Shape{
					{
						Kind:   resolve.ShapeSmartArt,
						Bounds: resolve.Rect{X: 914400, Y: 685800, W: 1828800, H: 1371600},
						Fill:   drawing.SolidFill{Color: "4472C4"},
						Paragraphs: []*resolve.Paragraph{
							{
								BulletChar:  "•",
								BulletColor: "FF0000",
								Runs:        []*resolve.Run{{Text: "Item one"}},
							},
							{
								AutoNum:     true,
								AutoNumType: "arabicPeriod",
								Runs:        []*resolve.Run{{Text: "Item two"}},
							},
						},
					},
				},
			},
		},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "bullet-marker") {
		t.Fatalf("expected bullet-marker span, got: %s", out)
	}
	if !strings.Contains(out, "Item one") || !strings.Contains(out, "Item two") {
		t.Fatalf("expected item text, got: %s", out)
	}
}

func TestHTMLRendersSmartArtPictureFill(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{
				Shapes: []*resolve.Shape{
					{
						Kind:   resolve.ShapeSmartArt,
						Bounds: resolve.Rect{X: 914400, Y: 685800, W: 1828800, H: 1371600},
						Fill: drawing.PictureFill{
							ImageData:   []byte{0x89, 0x50, 0x4E, 0x47},
							ContentType: "image/png",
						},
						Geometry: "ellipse",
						Paragraphs: []*resolve.Paragraph{
							{Runs: []*resolve.Run{{Text: "Pic"}}},
						},
					},
				},
			},
		},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<pattern id="smartart-fill"`) {
		t.Fatalf("expected SVG pattern definition for SmartArt picture fill\n%s", out)
	}
	if !strings.Contains(out, `fill="url(#smartart-fill)"`) || !strings.Contains(out, `<image href="data:image/png;base64,`) {
		t.Fatalf("expected picture fill applied to SmartArt shape\n%s", out)
	}
}

func TestHTMLAppliesPictureTileScaleOffsetAlignment(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Index: 0,
			BgFill: drawing.PictureFill{
				ImageData:   []byte{0x89, 0x50, 0x4E, 0x47},
				ContentType: "image/png",
				Tile:        true,
			},
			BgPictureTile: &resolve.PictureTile{
				Tx: 12700, Ty: 25400,
				Sx: 50000, Sy: 25000,
				Alignment: "ctr", Flip: "x",
			},
		}},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for _, want := range []string{
		`patternUnits="userSpaceOnUse"`,
		`x="2298700.0000"`,
		`y="2597150.0000"`,
		`width="4572000.0000"`,
		`height="1714500.0000"`,
		`data-ppt-tile-flip="x"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("expected %q in tiled background html: %s", want, html)
		}
	}
}

func TestHTMLAppliesPictureTileToShapeVector(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU: 9144000, SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{Index: 0, Shapes: []*resolve.Shape{{
			Kind:        resolve.ShapeText,
			Bounds:      resolve.Rect{W: 1270000, H: 635000}, // 100pt x 50pt
			Fill:        drawing.PictureFill{ImageData: []byte{0x89, 0x50, 0x4E, 0x47}, ContentType: "image/png", Tile: true},
			PictureTile: &resolve.PictureTile{Sx: 50000, Sy: 50000, Alignment: "br"},
		}}}},
	}
	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `patternUnits="userSpaceOnUse"`) ||
		!strings.Contains(html, `x="50.0000"`) || !strings.Contains(html, `y="25.0000"`) ||
		!strings.Contains(html, `width="50.0000"`) || !strings.Contains(html, `height="25.0000"`) {
		t.Fatalf("expected aligned 50%% tile in shape SVG: %s", html)
	}
}

func TestHTMLRendersAlternatingPictureTileFlipModes(t *testing.T) {
	tests := []struct {
		name  string
		flip  string
		wants []string
	}{
		{
			name: "horizontal",
			flip: "x",
			wants: []string{
				`width="9144000.0000" height="3429000.0000" data-ppt-tile-flip="x"`,
				`transform="translate(9144000.0000 0) scale(-1 1)"`,
			},
		},
		{
			name: "vertical",
			flip: "y",
			wants: []string{
				`width="4572000.0000" height="6858000.0000" data-ppt-tile-flip="y"`,
				`transform="translate(0 6858000.0000) scale(1 -1)"`,
			},
		},
		{
			name: "both",
			flip: "xy",
			wants: []string{
				`width="9144000.0000" height="6858000.0000" data-ppt-tile-flip="xy"`,
				`transform="translate(9144000.0000 0) scale(-1 1)"`,
				`transform="translate(0 6858000.0000) scale(1 -1)"`,
				`transform="translate(9144000.0000 6858000.0000) scale(-1 -1)"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pres := &resolve.Presentation{
				SlideWidthEMU:  9144000,
				SlideHeightEMU: 6858000,
				Slides: []*resolve.Slide{{
					Index: 0,
					BgFill: drawing.PictureFill{
						ImageURL: "data:image/png;base64,abc",
						Tile:     true,
					},
					BgPictureTile: &resolve.PictureTile{
						Sx: 50000, Sy: 50000, Flip: tt.flip,
					},
				}},
			}

			html, err := render.HTML(pres)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if !strings.Contains(html, `<svg x="0.0000" y="0.0000" width="4572000.0000" height="3429000.0000" viewBox="0 0 1 1" preserveAspectRatio="none">`) {
				t.Fatalf("expected base tile viewport for flip %q: %s", tt.flip, html)
			}
			for _, want := range tt.wants {
				if !strings.Contains(html, want) {
					t.Fatalf("flip %q: expected %q in html: %s", tt.flip, want, html)
				}
			}
		})
	}
}

func TestHTMLAppliesDrawingTransformToWholeTextShape(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{Shapes: []*resolve.Shape{{
			Kind:       resolve.ShapeText,
			Bounds:     resolve.Rect{W: 1828800, H: 914400},
			Paragraphs: []*resolve.Paragraph{{Runs: []*resolve.Run{{Text: "Moved"}}}},
			Effects: drawing.EffectList{Effects: []drawing.Effect{
				drawing.RelativeOffset{X: 0.1, Y: 0.2},
				drawing.TransformEffect{ScaleX: 1.2, ScaleY: 0.8, SkewX: 5},
			}},
		}}}},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "translate(10.0000%,20.0000%)") || !strings.Contains(out, "scale(1.2000,0.8000)") || !strings.Contains(out, "skewX(5.0000deg)") {
		t.Fatalf("expected whole-shape DrawingML transform in HTML:\n%s", out)
	}
}

func TestHTMLRendersSolidFillOverlayForImage(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{Shapes: []*resolve.Shape{{
			Kind:      resolve.ShapeImage,
			Bounds:    resolve.Rect{W: 1828800, H: 914400},
			ImageData: []byte{0x89, 0x50, 0x4e, 0x47},
			ImageMIME: "image/png",
			Effects: drawing.EffectList{Effects: []drawing.Effect{
				drawing.FillOverlay{Fill: drawing.SolidFill{Color: "FF0000", Alpha: 0.5}, BlendMode: "mult"},
			}},
		}}}},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="shape__effect-overlay"`) {
		t.Fatalf("expected fill overlay layer:\n%s", out)
	}
	if !strings.Contains(out, "mix-blend-mode:multiply") {
		t.Fatalf("expected multiply blend mode:\n%s", out)
	}
}

func TestHTMLRendersFillOverlayUsingVectorGeometry(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{Shapes: []*resolve.Shape{{
			Kind:     resolve.ShapeText,
			Geometry: "ellipse",
			Bounds:   resolve.Rect{W: 1828800, H: 914400},
			Fill:     drawing.SolidFill{Color: "00FF00", Alpha: 1},
			Effects: drawing.EffectList{Effects: []drawing.Effect{
				drawing.FillOverlay{Fill: drawing.SolidFill{Color: "FF0000", Alpha: 0.4}, BlendMode: "screen"},
			}},
		}}}},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<g class="shape__effect-overlay"`) {
		t.Fatalf("expected SVG fill overlay group:\n%s", out)
	}
	if !strings.Contains(out, "mix-blend-mode:screen") {
		t.Fatalf("expected screen blend mode:\n%s", out)
	}
}

func TestHTMLRendersReflectionFade(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{Shapes: []*resolve.Shape{{
			Kind:       resolve.ShapeText,
			Bounds:     resolve.Rect{W: 1828800, H: 914400},
			Paragraphs: []*resolve.Paragraph{{Runs: []*resolve.Run{{Text: "Reflect"}}}},
			Effects: drawing.EffectList{Effects: []drawing.Effect{
				drawing.Reflection{Alpha: 0.6, Distance: 4, Direction: 0, StartAlpha: 60000, StartPosition: 0, EndAlpha: 5000, EndPosition: 90000, Fade: true},
			}},
		}}}},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "-webkit-box-reflect:below 4.00pt") {
		t.Fatalf("expected reflection CSS:\n%s", out)
	}
	if !strings.Contains(out, "rgba(0,0,0,0.6000)") || !strings.Contains(out, "90.00%") {
		t.Fatalf("expected reflection fade ramp:\n%s", out)
	}
}

func TestHTMLAppliesEffectsToTableContainer(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{Shapes: []*resolve.Shape{{
			Kind:    resolve.ShapeTable,
			Bounds:  resolve.Rect{W: 1828800, H: 914400},
			Table:   &resolve.Table{Rows: []*resolve.TableRow{{Cells: []*resolve.TableCell{{Paragraphs: []*resolve.Paragraph{{Runs: []*resolve.Run{{Text: "Cell"}}}}}}}}},
			Effects: drawing.EffectList{Effects: []drawing.Effect{drawing.RelativeOffset{X: 0.05, Y: 0.1}}},
		}}}},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "translate(5.0000%,10.0000%)") {
		t.Fatalf("expected table container effect transform:\n%s", out)
	}
}

func TestHTMLRendersStandalonePictureBlipEffects(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU: 9144000, SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{Index: 0, Shapes: []*resolve.Shape{{
			ID: 17, Kind: resolve.ShapeImage, Name: "Filtered picture",
			Bounds:    resolve.Rect{W: 914400, H: 685800},
			ImageData: []byte{0x89, 0x50, 0x4e, 0x47}, ImageMIME: "image/png",
			ImageEffects: []drawing.Effect{
				drawing.Grayscale{},
				drawing.Duotone{Color1: "112233", Color2: "DDEEFF"},
				drawing.Lum{Brightness: 10000, Contrast: 20000},
				drawing.AlphaModFix{Amount: 75000},
			},
		}}}},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, want := range []string{
		`data-ppt-image-effects="grayscl duotone lum alphaModFix"`,
		`<filter id="picture-blip-filter-1"`,
		`type="saturate" values="0"`,
		`<feComponentTransfer`,
		`filter:url(#picture-blip-filter-1)`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in rendered picture\n%s", want, out)
		}
	}
}

func TestHTMLRendersPictureFillBlipEffectsInsidePattern(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU: 9144000, SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{Index: 0, Shapes: []*resolve.Shape{{
			ID: 22, Kind: resolve.ShapeText,
			Bounds:   resolve.Rect{W: 914400, H: 685800},
			Geometry: "ellipse",
			Fill: drawing.PictureFill{
				ImageData: []byte{0x89, 0x50, 0x4e, 0x47}, ContentType: "image/png", Stretch: true,
				BlipEffects: []drawing.Effect{drawing.BiLevel{Threshold: 40000}, drawing.ClrRepl{Color: "336699"}},
			},
		}}}},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `-blip-filter`) || !strings.Contains(out, `filter:url(#`) {
		t.Fatalf("expected picture-fill filter in SVG output\n%s", out)
	}
	if !strings.Contains(out, `<feFlood flood-color="#336699"`) {
		t.Fatalf("expected color replacement filter\n%s", out)
	}
	if !strings.Contains(out, `<feComposite`) {
		t.Fatalf("expected color replacement alpha composite\n%s", out)
	}
}

func TestHTMLCounterRotatesPictureWhenBlipFillDoesNotRotateWithShape(t *testing.T) {
	pres := &resolve.Presentation{SlideWidthEMU: 9144000, SlideHeightEMU: 6858000, Slides: []*resolve.Slide{{Shapes: []*resolve.Shape{{
		Kind: resolve.ShapeImage, ID: 41, Name: "Photo", Bounds: resolve.Rect{W: 914400, H: 914400}, RotationDeg: 30,
		ImageData: []byte{0x89, 0x50, 0x4e, 0x47}, ImageMIME: "image/png", ImageDPI: 144,
		ImageRotWithShapeSet: true, ImageRotWithShape: false,
	}}}}}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "transform:rotate(-30.00deg)") {
		t.Fatalf("expected counter-rotation in image style: %s", out)
	}
	if !strings.Contains(out, `data-ppt-image-dpi="144"`) || !strings.Contains(out, `data-ppt-rot-with-shape="false"`) {
		t.Fatalf("expected image geometry metadata: %s", out)
	}
}

func TestHTMLRendersPictureStretchFillRect(t *testing.T) {
	pres := &resolve.Presentation{SlideWidthEMU: 9144000, SlideHeightEMU: 6858000, Slides: []*resolve.Slide{{Shapes: []*resolve.Shape{{
		Kind: resolve.ShapeImage, ID: 51, Name: "Inset", Bounds: resolve.Rect{W: 914400, H: 914400},
		ImageData: []byte{0x89, 0x50, 0x4e, 0x47}, ImageMIME: "image/png",
		ImageFillLeft: 0.1, ImageFillTop: 0.2, ImageFillRight: 0.3, ImageFillBottom: 0.1,
	}}}}}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"left:10.0000%", "top:20.0000%", "right:30.0000%", "bottom:10.0000%", `data-ppt-fill-rect="0.10000,0.20000,0.30000,0.10000"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output: %s", want, out)
		}
	}
}

func TestHTMLHidesPictureWhenSourceCropIsEmpty(t *testing.T) {
	pres := &resolve.Presentation{SlideWidthEMU: 9144000, SlideHeightEMU: 6858000, Slides: []*resolve.Slide{{Shapes: []*resolve.Shape{{
		Kind: resolve.ShapeImage, ID: 52, Name: "Empty crop", Bounds: resolve.Rect{W: 914400, H: 914400},
		ImageData: []byte{0x89, 0x50, 0x4e, 0x47}, ImageMIME: "image/png",
		ImageCropLeft: 0.6, ImageCropRight: 0.4,
	}}}}}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "visibility:hidden") {
		t.Fatalf("expected empty crop to hide image: %s", out)
	}
}

func TestHTMLHidesPictureWhenStretchFillRectIsEmpty(t *testing.T) {
	pres := &resolve.Presentation{SlideWidthEMU: 9144000, SlideHeightEMU: 6858000, Slides: []*resolve.Slide{{Shapes: []*resolve.Shape{{
		Kind: resolve.ShapeImage, ID: 53, Name: "Empty destination", Bounds: resolve.Rect{W: 914400, H: 914400},
		ImageData: []byte{0x89, 0x50, 0x4e, 0x47}, ImageMIME: "image/png",
		ImageFillLeft: 0.7, ImageFillRight: 0.3,
	}}}}}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "display:none") {
		t.Fatalf("expected empty fill rect to hide frame: %s", out)
	}
}

func TestHTMLRendersExtendedImageTransparencyEffects(t *testing.T) {
	pres := &resolve.Presentation{SlideWidthEMU: 9144000, SlideHeightEMU: 6858000, Slides: []*resolve.Slide{{Shapes: []*resolve.Shape{{Kind: resolve.ShapeImage, ID: 77, Bounds: resolve.Rect{W: 914400, H: 685800}, ImageData: []byte{0x89, 0x50, 0x4e, 0x47}, ImageMIME: "image/png", ImageEffects: []drawing.Effect{drawing.AlphaBiLevel{Threshold: 40000}, drawing.AlphaCeiling{}, drawing.AlphaFloor{}, drawing.AlphaOutset{Radius: 2}, drawing.ClrChange{From: "FF0000", To: "00FF00", UseAlpha: true, ToAlpha: .75}}}}}}}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`<feComponentTransfer`, `operator="dilate"`, `mode="difference"`, `flood-color="rgb(0,255,0)"`, `flood-opacity="0.750000"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %s", want, out)
		}
	}
}

func TestHTMLRendersSyntheticSmartArtCurveAsBezier(t *testing.T) {
	start := resolve.Point{X: 12700, Y: 12700}
	end := resolve.Point{X: 127000, Y: 63500}
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Index: 0,
			Shapes: []*resolve.Shape{{
				Kind:           resolve.ShapeSmartArtConnector,
				ConnectorType:  "curved",
				ConnectorStart: &start,
				ConnectorEnd:   &end,
				LineColor:      "123456",
				LineWidthPt:    1.5,
			}},
		}},
	}
	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(html, `<path d="M 1.0000 1.0000 C `) {
		t.Fatalf("expected cubic SmartArt connector path\n%s", html)
	}
}

func TestHTMLRendersSmartArtThreeDExtrusion(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Shapes: []*resolve.Shape{{
				Kind:      resolve.ShapeSmartArt,
				Bounds:    resolve.Rect{W: 1828800, H: 914400},
				Geometry:  "roundRect",
				FillColor: "4472C4",
				Effects: drawing.EffectList{Effects: []drawing.Effect{drawing.ThreeD{
					CameraPreset:   "perspectiveRelaxed",
					RotationX:      8,
					RotationY:      -12,
					ExtrusionDepth: 9,
					ExtrusionColor: "203864",
					Material:       "plastic",
				}}},
				Paragraphs: []*resolve.Paragraph{{Runs: []*resolve.Run{{Text: "3D"}}}},
			}},
		}},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `shape__3d-extrusion`) || !strings.Contains(out, `data-depth="9.00"`) {
		t.Fatalf("expected SmartArt extrusion geometry\n%s", out)
	}
	if !strings.Contains(out, `rotateX(8.0000deg)`) || !strings.Contains(out, `rotateY(12.0000deg)`) {
		t.Fatalf("expected SmartArt camera transform\n%s", out)
	}
}

func TestHTMLRendersLinkedOLEObjectAsExternalLink(t *testing.T) {
	pres := &resolve.Presentation{SlideWidthEMU: 9144000, SlideHeightEMU: 6858000, Slides: []*resolve.Slide{{Index: 0, Shapes: []*resolve.Shape{{
		Kind: resolve.ShapeOleObject, Name: "Linked object", Bounds: resolve.Rect{W: 1828800, H: 914400},
		OLEObjectName: "Linked report", OLEObjectProgID: "Excel.Sheet.12", OLEObjectLinkURL: "https://example.com/report.xlsx",
	}}}}}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="ole-object-link"`) || !strings.Contains(out, `href="https://example.com/report.xlsx"`) || !strings.Contains(out, `rel="noopener noreferrer"`) {
		t.Fatalf("expected safe external OLE link in HTML: %s", out)
	}
}
func TestHTMLRendersLinkedLegacyVideoMedia(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Shapes: []*resolve.Shape{{
				Kind:         resolve.ShapeMedia,
				Name:         "Linked legacy video",
				Bounds:       resolve.Rect{W: 1828800, H: 1028700},
				MediaKind:    "video",
				MediaMIME:    "video/mp4",
				MediaLinkURL: "https://example.test/movie.mp4",
			}},
		}},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<video`) || !strings.Contains(out, `src="https://example.test/movie.mp4"`) {
		t.Fatalf("linked video not rendered from URL:\n%s", out)
	}
}
