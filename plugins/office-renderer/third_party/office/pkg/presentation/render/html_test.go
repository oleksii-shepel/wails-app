package render_test

import (
	"bytes"
	"encoding/binary"

	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"office/pkg/diag"
	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/opc"
	"office/pkg/presentation/edit"
	ir "office/pkg/presentation/ir"
	"office/pkg/presentation/parsers/ooxml/resolve"
	"office/pkg/presentation/render"
	wordedit "office/pkg/word/edit"
	wordlegacy "office/pkg/word/parsers/legacy"
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

func TestHTMLDoesNotPaintUnavailableEquationOLEPlaceholder(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU: 9144000, SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{Shapes: []*resolve.Shape{{
			Kind: resolve.ShapeOleObject, Bounds: resolve.Rect{W: 914400, H: 914400},
			OLEObjectName: "Microsoft Equation 3.0", OLEObjectProgID: "Equation.3",
		}}}},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "[Microsoft Equation 3.0]") {
		t.Fatalf("unavailable Equation OLE must not paint a placeholder: %s", out)
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

func TestHTMLRendersSVGPreviewOLEObjectInline(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Index: 0,
			Shapes: []*resolve.Shape{{
				Kind:          resolve.ShapeOleObject,
				Name:          "Object 35",
				Bounds:        resolve.Rect{X: 914400, Y: 685800, W: 1828800, H: 1371600},
				OLEObjectName: "Document",
				OLEIconData:   []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="10" height="10" fill="#99ffcc"/></svg>`),
				OLEIconMIME:   "image/svg+xml",
			}},
		}},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `shape--ole-object-preview`) {
		t.Fatalf("expected inline OLE preview host in output: %s", out)
	}
	if !strings.Contains(out, `<div class="shape--ole-object-preview-svg"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10" preserveAspectRatio="xMidYMid meet"><rect width="10" height="10" fill="#99ffcc"/></svg></div>`) {
		t.Fatalf("expected inline SVG OLE preview in output: %s", out)
	}
	if strings.Contains(out, `data:image/svg+xml;base64,`) {
		t.Fatalf("expected inline SVG preview instead of SVG data URI image: %s", out)
	}
}

func TestHTMLRendersWordOLEPreviewBeforeNativeContent(t *testing.T) {
	doc := wordedit.NewDocument()
	doc.AddParagraph().AddRun().SetText("Embedded legacy Word preview")
	var payload bytes.Buffer
	if err := wordlegacy.WriteDocument(doc.IR, &payload); err != nil {
		t.Fatalf("write legacy doc: %v", err)
	}
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Index: 0,
			Shapes: []*resolve.Shape{{
				Kind:            resolve.ShapeOleObject,
				Name:            "Object 35",
				Bounds:          resolve.Rect{X: 914400, Y: 685800, W: 1828800, H: 1371600},
				OLEObjectName:   "Document",
				OLEObjectProgID: "Word.Document.8",
				OLEObjectMIME:   "application/msword",
				OLEObjectData:   payload.Bytes(),
				OLEIconData:     []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="10" height="10" fill="#99ffcc"/></svg>`),
				OLEIconMIME:     "image/svg+xml",
			}},
		}},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `shape--ole-object-preview`) {
		t.Fatalf("expected preview-first OLE rendering in output: %s", out)
	}
	if strings.Contains(out, `data:image/svg+xml;base64,`) {
		t.Fatalf("expected inline SVG preview instead of SVG data URI image: %s", out)
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

func TestHTMLRendersLineBreaksAsPhysicalLines(t *testing.T) {
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

	if !strings.Contains(html, "Line1") {
		t.Fatalf("expected first physical line in html: %s", html)
	}
	if !strings.Contains(html, "Line2") {
		t.Fatalf("expected second physical line in html: %s", html)
	}

	line1 := strings.Index(html, "Line1")
	line2 := strings.Index(html, "Line2")
	if line1 < 0 || line2 <= line1 {
		t.Fatalf("expected Line1 before Line2 in html: %s", html)
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
		`drop-shadow(`,
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
	if !strings.Contains(html, `<span class="bullet-marker" style="font-size:24.0pt;font-family:Arial,sans-serif;color:#111111;margin-right:`) || !strings.Contains(html, `pt">1.</span>`) {
		t.Fatalf("expected marker style and physical marker gap in html: %s", html)
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

	if !strings.Contains(html, `justify-content:center`) ||
		!strings.Contains(html, `padding:7.20pt 7.20pt 7.20pt 7.20pt`) {
		t.Fatalf("expected text body styles in html: %s", html)
	}

	if !strings.Contains(
		html,
		`<p style="text-align:center;margin-bottom:0.0000pt;line-height:1.0000">`,
	) {
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
	if !strings.Contains(html, `class="shape shape--text"`) ||
		!strings.Contains(html, `width:100.0000%;height:100.0000%;overflow:visible`) {
		t.Fatalf("expected no-autofit text shape to allow overflow: %s", html)
	}
	if !strings.Contains(html, `height:100%;overflow:visible`) {
		t.Fatalf("no-autofit content must preserve authored frame height while allowing overflow: %s", html)
	}
	if strings.Contains(html, `height:auto;overflow:visible`) {
		t.Fatalf("no-autofit must not turn the authored text frame into an auto-height box: %s", html)
	}
}

func TestHTMLMeasuredTextContentAvoidsInnerClipping(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Index: 0,
			Shapes: []*resolve.Shape{{
				Kind:   resolve.ShapeText,
				Bounds: resolve.Rect{W: 2743200, H: 457200},
				Paragraphs: []*resolve.Paragraph{{
					Runs: []*resolve.Run{{Text: "Measured text", FontSize: 2400}},
				}},
			}},
		}},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `height:100%;overflow:visible;justify-content:flex-start`) {
		t.Fatalf("measured text content should preserve authored height but avoid inner clipping: %s", html)
	}
}

func TestHTMLMeasuredWrappedLinesOwnHangingIndent(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Index: 0,
			Shapes: []*resolve.Shape{{
				Kind:   resolve.ShapeText,
				Bounds: resolve.Rect{W: 1828800, H: 1371600},
				Paragraphs: []*resolve.Paragraph{{
					MarginLeftEMU: 514350,
					TextIndentEMU: -514350,
					HasMarginLeft: true,
					HasTextIndent: true,
					AutoNum:       true,
					AutoNumStart:  1,
					Runs: []*resolve.Run{{
						Text:     "Things that equal the same thing also equal one another.",
						FontSize: 2800,
					}},
				}},
			}},
		}},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Count(html, `class="ppt-physical-line"`) < 2 {
		t.Fatalf("expected measured wrapping: %s", html)
	}
	if !strings.Contains(html, `margin-left:-40.5000pt`) {
		t.Fatalf("marker should own the authored hanging-indent origin: %s", html)
	}
	if strings.Contains(html, `white-space:pre;text-indent:-40.5000pt`) {
		t.Fatalf("marker paragraph must not apply hanging indent twice: %s", html)
	}
	if !strings.Contains(html, `white-space:pre;text-indent:0.0000pt`) {
		t.Fatalf("continuation physical lines should reset hanging indent: %s", html)
	}
	if strings.Contains(html, `white-space:pre-wrap`) {
		t.Fatalf("measured physical lines must not wrap again in the browser: %s", html)
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
	// Measured paragraphs no longer expose normAutofit as a paragraph-level
	// CSS multiplier. Layout owns the reduced physical line box: 24pt * 0.92
	// font scale * 1.117 Arial leading * 0.82 line-space scale = 20.2240pt.
	if !strings.Contains(html, `line-height:20.2240pt;height:20.2240pt;min-height:20.2240pt`) {
		t.Fatalf("expected normAutofit line spacing reduction in physical line box: %s", html)
	}
	if !strings.Contains(html, `line-height:0`) {
		t.Fatalf("expected measured paragraph browser strut to be disabled: %s", html)
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
	if !strings.Contains(html, `height:100%;overflow:visible`) {
		t.Fatalf("expected authored-height noAutofit content with visible overflow: %s", html)
	}
	if strings.Contains(html, `height:auto`) {
		t.Fatalf("noAutofit content must not auto-grow beyond its authored frame: %s", html)
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
	if !strings.Contains(html, `margin-left:36.0000pt`) || !strings.Contains(html, `text-indent:-18.0000pt`) {
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
					// Space-before is suppressed on the first body paragraph.
					Runs: []*resolve.Run{{Text: "Lead paragraph", FontSize: 1200}},
				}, {
					SpaceBeforePct: 100000, // 100% of the physical first-line box
					SpaceAfterPct:  50000,  // 50% of the physical last-line box
					Runs:           []*resolve.Run{{Text: "Spaced paragraph", FontSize: 1200}},
				}},
			}},
		}},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `margin-top:13.4040pt`) {
		t.Fatalf("expected 100%% spacing before to use the physical line height: %s", html)
	}
	if !strings.Contains(html, `margin-bottom:6.7020pt`) {
		t.Fatalf("expected 50%% spacing after to use half the physical line height: %s", html)
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
	if shapeStart := strings.Index(html, `class="shape shape--text" style="`); shapeStart >= 0 {
		if shapeEnd := strings.Index(html[shapeStart:], `>`); shapeEnd >= 0 && strings.Contains(html[shapeStart:shapeStart+shapeEnd], `box-shadow:`) {
			t.Fatalf("text shape outer frame must not use rectangular box-shadow: %s", html)
		}
	}
	if !strings.Contains(html, `drop-shadow(`) {
		t.Fatalf("expected physical text-shape shadow to use geometry-aware drop-shadow: %s", html)
	}
	if !strings.Contains(html, `-webkit-box-reflect:`) {
		t.Fatalf("expected physical reflection in html: %s", html)
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
	if !strings.Contains(html, `<line x1="144.0000" y1="54.0000" x2="270.0000" y2="54.0000" stroke="#595959" stroke-width="3.00"/>`) {
		t.Fatalf("expected line trim to scale with effective stroke width: %s", html)
	}
	if !strings.Contains(html, `<polygon points="288.0000,54.0000 270.0000,60.0000 270.0000,48.0000" fill="#595959"/>`) {
		t.Fatalf("expected tail arrow geometry to scale with effective stroke width: %s", html)
	}
}

func TestHTMLArrowMarkersInheritConnectorOpacity(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU: 9144000, SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{Index: 0, Shapes: []*resolve.Shape{{
			Kind: resolve.ShapeConnector, Bounds: resolve.Rect{X: 1828800, Y: 685800, W: 1828800},
			LineColor: "336699", LineWidthPt: 2, ConnectorType: "straightConnector1",
			LineStyle: &drawing.LineStyle{Width: 2, Color: "336699", Alpha: 0.35},
			ArrowTail: "triangle", ArrowTailWidth: "med", ArrowTailLength: "med",
		}}}},
	}
	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `stroke-opacity="0.3500"`) {
		t.Fatalf("connector shaft must retain authored opacity: %s", html)
	}
	if !strings.Contains(html, `fill="#336699" fill-opacity="0.3500"`) {
		t.Fatalf("filled arrow marker must inherit connector opacity: %s", html)
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
	if !strings.Contains(html, `stroke-linecap="round"`) || !strings.Contains(html, `stroke-linejoin="bevel"`) || !strings.Contains(html, `stroke-dasharray="9, 9, 3, 9"`) {
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
	if !strings.Contains(html, `<line x1="144.0000" y1="54.0000" x2="279.0000" y2="54.0000" stroke="#595959" stroke-width="3.00"/>`) {
		t.Fatalf("expected small arrow length to scale with the 3pt stroke: %s", html)
	}
	if !strings.Contains(html, `<polygon points="288.0000,54.0000 279.0000,57.3750 279.0000,50.6250" fill="#595959"/>`) {
		t.Fatalf("expected small arrow width to render narrower triangle: %s", html)
	}
	if !strings.Contains(html, `<line x1="144.0000" y1="108.0000" x2="270.0000" y2="108.0000" stroke="#595959" stroke-width="3.00"/>`) {
		t.Fatalf("expected large arrow length to scale with the 3pt stroke: %s", html)
	}
	if !strings.Contains(html, `<polygon points="288.0000,108.0000 270.0000,114.0000 270.0000,102.0000" fill="#595959"/>`) {
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
	if !strings.Contains(html, `mix-blend-mode:screen`) || !strings.Contains(html, `mix-blend-mode:multiply`) {
		t.Fatalf("expected 3D scene-light overlays in html: %s", html)
	}
	if !strings.Contains(html, `shape-fill-overlay-0`) || !strings.Contains(html, `shape-fill-overlay-1`) {
		t.Fatalf("expected SVG defs for 3D light gradients in html: %s", html)
	}
}

func TestHTMLRendersThreeDAndAuthoredShadowTogether(t *testing.T) {
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
				Effects: drawing.EffectList{Effects: []drawing.Effect{
					drawing.Shadow{Color: "000000", Alpha: 0.35, Blur: 3, Distance: 5, Angle: 45},
					drawing.ThreeD{
						CameraPreset: "legacyPerspectiveTopLeft", CameraFOV: 45, CameraZoom: 1.1,
						LightRig: "threePt", LightDirection: "tl", RotationX: -15, RotationY: -10,
						ExtrusionDepth: 6, Material: "metal", ExtrusionColor: "595959",
					},
				}},
			}},
		}},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if shapeStart := strings.Index(html, `class="shape shape--text" style="`); shapeStart >= 0 {
		if shapeEnd := strings.Index(html[shapeStart:], `>`); shapeEnd >= 0 && strings.Contains(html[shapeStart:shapeStart+shapeEnd], `box-shadow:`) {
			t.Fatalf("text shape outer frame must not use rectangular box-shadow: %s", html)
		}
	}
	if !strings.Contains(html, `drop-shadow(`) {
		t.Fatalf("expected authored shadow to survive physical 3D composition as drop-shadow: %s", html)
	}
	if !strings.Contains(html, `perspective(`) || !strings.Contains(html, `shape__3d-extrusion`) {
		t.Fatalf("expected 3D composition to survive alongside authored shadow: %s", html)
	}
}

func TestHTMLPresentationOuterShadowNeverUsesShapeBoxShadow(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Shapes: []*resolve.Shape{{
				Kind:      resolve.ShapeText,
				Bounds:    resolve.Rect{W: 3000000, H: 1500000},
				Geometry:  "triangle",
				FillColor: "2DA2BF",
				Effects: drawing.EffectList{Effects: []drawing.Effect{
					drawing.Shadow{Color: "000000", Alpha: 1, Blur: 4, Distance: 3, Angle: 270},
				}},
			}},
		}},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `filter:drop-shadow(`) {
		t.Fatalf("expected authored outer shadow to follow painted geometry: %s", html)
	}
	for _, marker := range []string{`class="shape shape--text" style="`, `class="shape shape--smartart" style="`, `class="shape shape--chart" style="`} {
		if at := strings.Index(html, marker); at >= 0 {
			end := strings.Index(html[at:], `">`)
			if end >= 0 && strings.Contains(html[at:at+end], `box-shadow:`) {
				t.Fatalf("presentation positioning wrapper must not own box-shadow: %s", html[at:at+end])
			}
		}
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
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("read: %v", readErr)
			}
			edited, err := edit.LoadFromPPTXBytes(data)
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			pres := edited.IR
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

	if !strings.Contains(html, "margin-right:36.0000pt") {
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
	if strings.Contains(out, "text-align:center") {
		t.Fatalf("anchorCtr must not override paragraph alignment, got:\n%s", out)
	}
	if !strings.Contains(out, "transform-origin:center center") {
		t.Fatalf("expected centered text anchor transform origin, got:\n%s", out)
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
	// Physical text layout owns the bottom-anchor displacement. Once measured,
	// the renderer must not apply flex-end a second time; the displacement is
	// serialized on the physical progression edge instead.
	if !strings.Contains(out, "justify-content:flex-start") {
		t.Errorf("expected measured placement to neutralize semantic flex anchoring, got: %s", out)
	}
	if strings.Contains(out, "text-align:center") {
		t.Errorf("anchorCtr must not override paragraph alignment, got: %s", out)
	}
	if !strings.Contains(out, "transform-origin:center center") {
		t.Errorf("expected anchorCtr transform origin, got: %s", out)
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
	// vertical-rl progresses from the right edge. The authored right inset is
	// therefore combined with the measured bottom-anchor offset on that edge.
	if !strings.Contains(out, "padding:7.87pt 106.97pt 15.75pt 23.62pt") {
		t.Errorf("expected bodyPr insets plus physical progression offset as padding, got: %s", out)
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
	if !strings.Contains(out, `class="shape shape--smartart"`) ||
		!strings.Contains(out, `width:20.0000%`) ||
		!strings.Contains(out, `min-height:20.0000%`) ||
		!strings.Contains(out, `overflow:visible`) {
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
	if !strings.Contains(out, `class="shape shape--smartart"`) ||
		!strings.Contains(out, `height:20.0000%`) ||
		!strings.Contains(out, `overflow:visible`) {
		t.Fatalf("expected noAutofit SmartArt outer div to preserve height but allow overflow, got: %s", out)
	}
	if !strings.Contains(out, `inset:0;height:100%;overflow:visible`) {
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
	// SmartArt uses the same physical text composition boundary as ordinary
	// text. The normAutofit reduction therefore belongs to the measured line
	// box, not to a paragraph-level browser line-height multiplier.
	if !strings.Contains(out, `line-height:20.2240pt;height:20.2240pt;min-height:20.2240pt`) {
		t.Fatalf("expected SmartArt normAutofit reduction in physical line box, got: %s", out)
	}
	if !strings.Contains(out, `line-height:0`) {
		t.Fatalf("expected SmartArt measured paragraph browser strut to be disabled: %s", out)
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
	for _, want := range []string{`<feComponentTransfer`, `operator="dilate"`, `mode="difference"`, `flood-color="rgb(0,255,0)"`, `flood-opacity="0.750000"`, `x="-2.7778%"`, `y="-3.7037%"`, `width="105.5556%"`, `height="107.4074%"`} {
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

func TestHTMLKeepsMeasuredMarkerInsideFirstPhysicalLineBox(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU: 9144000, SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{Shapes: []*resolve.Shape{{
			Kind: resolve.ShapeText, Bounds: resolve.Rect{W: 914400, H: 685800},
			Paragraphs: []*resolve.Paragraph{{
				AutoNum: true, AutoNumType: "arabicPeriod", BulletSize: 2400,
				BulletFontFamily: "Arial", Runs: []*resolve.Run{{Text: "Measured marker", FontSize: 1200}},
			}},
		}}}},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	line := strings.Index(out, `<span class="ppt-physical-line"`)
	marker := strings.Index(out, `<span class="bullet-marker"`)
	lineEnd := strings.Index(out[line:], `</span>`)
	if line < 0 || marker < 0 || marker < line || lineEnd < 0 || marker >= line+lineEnd {
		t.Fatalf("measured marker must be serialized inside the first physical line box: %s", out)
	}
}

func TestHTMLCounterRotatedCroppedPictureUsesPhysicalPictureCenterPivot(t *testing.T) {
	pres := &resolve.Presentation{SlideWidthEMU: 9144000, SlideHeightEMU: 6858000, Slides: []*resolve.Slide{{Shapes: []*resolve.Shape{{
		Kind: resolve.ShapeImage, ID: 141, Name: "Cropped counter rotation", Bounds: resolve.Rect{W: 914400, H: 914400}, RotationDeg: 30,
		ImageData: []byte{0x89, 0x50, 0x4e, 0x47}, ImageMIME: "image/png",
		ImageCropLeft: .10, ImageCropTop: .20, ImageCropRight: .25, ImageCropBottom: .05,
		ImageFillLeft: .05, ImageFillTop: .10, ImageFillRight: .20, ImageFillBottom: .15,
		ImageRotWithShapeSet: true, ImageRotWithShape: false,
	}}}}}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "transform:rotate(-30.00deg)") || !strings.Contains(out, "transform-origin:49.0000% 60.0000%") {
		t.Fatalf("expected physical picture-center counter-rotation pivot: %s", out)
	}
}

func TestHTMLPictureEffectsRespectGeometryMaskPaintOrder(t *testing.T) {
	pres := &resolve.Presentation{SlideWidthEMU: 9144000, SlideHeightEMU: 6858000, Slides: []*resolve.Slide{{Shapes: []*resolve.Shape{{
		Kind: resolve.ShapeImage, ID: 991, Bounds: resolve.Rect{W: 914400, H: 685800}, Geometry: "ellipse",
		ImageData: []byte{0x89, 0x50, 0x4e, 0x47}, ImageMIME: "image/png",
		Effects: drawing.EffectList{Effects: []drawing.Effect{
			drawing.Shadow{Color: "112233", Alpha: .5, Blur: 3, Distance: 4},
			drawing.InnerShadow{Color: "445566", Alpha: .75, Blur: 2, Distance: 1},
			drawing.Glow{Color: "778899", Alpha: .4, Radius: 5},
		}},
	}}}}}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "filter:drop-shadow(") {
		t.Fatalf("outer picture effect must use alpha-aware drop-shadow: %s", out)
	}
	if strings.Contains(out, "box-shadow:2.828") {
		t.Fatalf("outer picture shadow must not be rectangular box-shadow: %s", out)
	}
	if !strings.Contains(out, "shape__image-clip") || !strings.Contains(out, "clip-path:ellipse(") || !strings.Contains(out, "box-shadow:inset ") {
		t.Fatalf("inner picture effect must be serialized on geometry clip surface: %s", out)
	}
}

func TestHTMLKeepsSmartArtTextUprightInsideRotatedNode(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU: 9144000, SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{Shapes: []*resolve.Shape{{
			Kind: resolve.ShapeSmartArt, Bounds: resolve.Rect{X: 914400, Y: 685800, W: 1828800, H: 1371600},
			RotationDeg: 180,
			Paragraphs:  []*resolve.Paragraph{{Runs: []*resolve.Run{{Text: "Upright SmartArt", FontSize: 2400}}}},
		}}}},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "transform:rotate(180.00deg)") {
		t.Fatalf("expected rotated SmartArt geometry, got: %s", out)
	}
	if !strings.Contains(out, "transform:rotate(-180.00deg)") {
		t.Fatalf("expected nested SmartArt text to counter-rotate, got: %s", out)
	}
}

func TestHTMLSmartArtNoAutofitPreservesAuthoredTextFrameHeight(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU: 9144000, SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{Shapes: []*resolve.Shape{{
			Kind: resolve.ShapeSmartArt, Bounds: resolve.Rect{X: 914400, Y: 685800, W: 3657600, H: 914400},
			RotationDeg:   180,
			TextBodyProps: drawing.TextBodyProperties{AutofitType: "noAutofit", Anchor: "ctr"},
			Paragraphs:    []*resolve.Paragraph{{Runs: []*resolve.Run{{Text: "Text remains centered in the authored SmartArt frame", FontSize: 2400}}}},
		}}}},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="shape shape--smartart"`) {
		t.Fatalf("expected SmartArt output: %s", out)
	}
	if !strings.Contains(out, "height:100%;overflow:visible") {
		t.Fatalf("noAutofit must preserve authored SmartArt text height while allowing overflow: %s", out)
	}
	if strings.Contains(out, "height:auto;overflow:visible") {
		t.Fatalf("noAutofit must not convert SmartArt text to auto-height: %s", out)
	}
}

func TestHTMLRendersExactBaselineShiftWithoutBrowserSupSizing(t *testing.T) {
	pres := &resolve.Presentation{SlideWidthEMU: 9144000, SlideHeightEMU: 6858000, Slides: []*resolve.Slide{{Index: 0, Shapes: []*resolve.Shape{{Kind: resolve.ShapeText, Bounds: resolve.Rect{W: 914400, H: 914400}, Paragraphs: []*resolve.Paragraph{{Runs: []*resolve.Run{{Text: "x2", FontSize: 2400, Superscript: true, BaselineShiftPct: 30}}}}}}}}}
	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `position:relative;bottom:0.3000em`) {
		t.Fatalf("expected exact baseline shift: %s", html)
	}
	if strings.Contains(html, `<sup>`) {
		t.Fatalf("exact baseline shift must not invoke browser sup sizing: %s", html)
	}
}

func TestHTMLMaterializesHangingMarkerOriginWithoutPhysicalLineTextIndent(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU: 9144000, SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{Shapes: []*resolve.Shape{{
			Kind: resolve.ShapeText, Bounds: resolve.Rect{W: 1828800, H: 685800},
			Paragraphs: []*resolve.Paragraph{{
				HasMarginLeft: true, MarginLeftEMU: 457200,
				HasTextIndent: true, TextIndentEMU: -381000,
				BulletChar: "•", BulletSize: 1200, BulletFontFamily: "Arial",
				Runs: []*resolve.Run{{Text: "Hanging item", FontSize: 1200, FontFamily: "Arial"}},
			}},
		}}}},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	markerStart := strings.Index(out, `class="bullet-marker"`)
	if markerStart < 0 {
		t.Fatalf("missing physical marker: %s", out)
	}
	markerEnd := strings.Index(out[markerStart:], `>`)
	if markerEnd < 0 {
		t.Fatalf("malformed physical marker: %s", out)
	}
	markerTag := out[markerStart : markerStart+markerEnd]
	if !strings.Contains(markerTag, `font-size:12.0pt`) || !strings.Contains(markerTag, `font-family:Arial,sans-serif`) || !strings.Contains(markerTag, `margin-left:-30.0000pt`) {
		t.Fatalf("hanging marker origin was not materialized on marker: %s", markerTag)
	}
	line := strings.Index(out, `class="ppt-physical-line"`)
	if line < 0 {
		t.Fatalf("missing physical line: %s", out)
	}
	end := strings.Index(out[line:], `>`)
	if end < 0 {
		t.Fatalf("malformed physical line: %s", out)
	}
	lineTag := out[line : line+end]
	if strings.Contains(lineTag, `text-indent:-30.0000pt`) {
		t.Fatalf("marker paragraph must not also apply hanging indent to physical line: %s", lineTag)
	}
}

func TestHTMLUsesLayoutOwnedMarkerBaselinePlacement(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU: 9144000, SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{Shapes: []*resolve.Shape{{
			Kind: resolve.ShapeText, Bounds: resolve.Rect{W: 3657600, H: 685800},
			Paragraphs: []*resolve.Paragraph{{
				BulletChar: "•", BulletSize: 1800, BulletFontFamily: "Times New Roman",
				Runs: []*resolve.Run{{Text: "Large text", FontSize: 2800, FontFamily: "Arial"}},
			}},
		}}}},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	start := strings.Index(out, `class="bullet-marker"`)
	if start < 0 {
		t.Fatalf("missing marker: %s", out)
	}
	end := strings.Index(out[start:], `>`)
	if end < 0 {
		t.Fatalf("malformed marker: %s", out)
	}
	tag := out[start : start+end]
	if !strings.Contains(tag, `display:inline-block`) || !strings.Contains(tag, `vertical-align:top`) || !strings.Contains(tag, `position:relative`) {
		t.Fatalf("marker baseline placement is browser-owned: %s", tag)
	}
	if strings.Contains(tag, `top:0.0000pt`) {
		t.Fatalf("smaller marker was not positioned on the taller text baseline: %s", tag)
	}
}

func TestClass17SlideNumberWrapperDoesNotEscapePhysicalPaintOrder(t *testing.T) {
	before := &ir.Shape{Kind: ir.ShapeText, Name: "before", Bounds: ir.Rect{W: 914400, H: 914400},
		Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{Text: "BEFORE"}}}}}
	number := &ir.Shape{Kind: ir.ShapeText, Name: "number", Bounds: ir.Rect{W: 914400, H: 914400},
		Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{Text: "NUMBER"}}}}}
	pres := &ir.Presentation{SlideWidthEMU: 9144000, SlideHeightEMU: 6858000,
		Slides: []*ir.Slide{{Index: 1, Shapes: []*ir.Shape{before}, SlideNumber: number}}}
	got, _, err := render.HTMLWithOptions(pres, render.HTMLOptions{})
	if err != nil {
		t.Fatal(err)
	}
	beforeAt := strings.Index(got, "BEFORE")
	wrapperAt := strings.Index(got, `class="slide-number"`)
	numberAt := strings.Index(got, "NUMBER")
	if beforeAt < 0 || wrapperAt < 0 || numberAt < 0 || !(beforeAt < wrapperAt && wrapperAt < numberAt) {
		t.Fatalf("slide-number wrapper escaped physical order: before=%d wrapper=%d number=%d", beforeAt, wrapperAt, numberAt)
	}
}

func TestHTMLFixedTextFrameClipsInsideEffectOverflow(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU: 9144000, SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{Shapes: []*resolve.Shape{{
			Kind:          resolve.ShapeText,
			Bounds:        resolve.Rect{X: 914400, Y: 685800, W: 1828800, H: 457200},
			TextBodyProps: drawing.TextBodyProperties{AutofitType: "normAutofit"},
			Effects: drawing.EffectList{Effects: []drawing.Effect{
				drawing.Shadow{Color: "000000", Alpha: .4, Blur: 4, Distance: 6, Angle: 45},
			}},
			Paragraphs: []*resolve.Paragraph{{Runs: []*resolve.Run{{Text: "Fixed text with outer shadow", FontSize: 2400}}}},
		}}}},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	shapeAt := strings.Index(out, `class="shape shape--text"`)
	if shapeAt < 0 {
		t.Fatalf("missing text shape: %s", out)
	}
	shapeEnd := strings.Index(out[shapeAt:], `>`)
	if shapeEnd < 0 {
		t.Fatalf("malformed text shape: %s", out)
	}
	shapeTag := out[shapeAt : shapeAt+shapeEnd]
	if !strings.Contains(shapeTag, `overflow:visible`) {
		t.Fatalf("effect paint should keep outer shape overflow visible: %s", shapeTag)
	}
	clipAt := strings.Index(out[shapeAt:], `class="shape__text-clip"`)
	if clipAt < 0 {
		t.Fatalf("fixed text frame needs its own clipping layer: %s", out)
	}
	clipEnd := strings.Index(out[shapeAt+clipAt:], `>`)
	clipTag := out[shapeAt+clipAt : shapeAt+clipAt+clipEnd]
	if !strings.Contains(clipTag, `overflow:hidden`) {
		t.Fatalf("fixed text clip layer must own overflow clipping: %s", clipTag)
	}
	contentAt := strings.Index(out[shapeAt:], `class="shape__content"`)
	if contentAt < 0 {
		t.Fatalf("missing physical line container: %s", out)
	}
	contentEnd := strings.Index(out[shapeAt+contentAt:], `>`)
	contentTag := out[shapeAt+contentAt : shapeAt+contentAt+contentEnd]
	if !strings.Contains(contentTag, `overflow:visible`) {
		t.Fatalf("physical line container should remain overflow visible inside frame clip: %s", contentTag)
	}
}

func TestHTMLNoAutofitTextOverflowIsNotReclippedByShapeGeometry(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU: 9144000, SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{Shapes: []*resolve.Shape{{
			Kind:              resolve.ShapeText,
			Geometry:          "ellipse",
			Bounds:            resolve.Rect{X: 914400, Y: 685800, W: 914400, H: 457200},
			TextBodyProps:     drawing.TextBodyProperties{AutofitType: "noAutofit"},
			TextAllowOverflow: true,
			Paragraphs:        []*resolve.Paragraph{{Runs: []*resolve.Run{{Text: "Overflowing ellipse label", FontSize: 2400}}}},
		}}}},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="shape__content"`) || !strings.Contains(out, `overflow:visible`) {
		t.Fatalf("expected visible noAutofit text overflow: %s", out)
	}
	if strings.Contains(out, `class="shape__text-clip"`) {
		t.Fatalf("noAutofit text must not regain a fixed clipping frame: %s", out)
	}
	if strings.Contains(out, `class="shape__content" style="position:relative;z-index:1;display:flex;flex-direction:column;box-sizing:border-box;width:100%;height:100%;overflow:visible;justify-content:flex-start;padding:3.60pt 7.20pt 3.60pt 7.20pt;clip-path:ellipse(`) {
		t.Fatalf("noAutofit text content must not be re-clipped by ellipse geometry: %s", out)
	}
}

func TestHTMLClippedTextStillUsesAuthoredGeometryClip(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU: 9144000, SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{Shapes: []*resolve.Shape{{
			Kind:          resolve.ShapeText,
			Geometry:      "ellipse",
			Bounds:        resolve.Rect{X: 914400, Y: 685800, W: 914400, H: 457200},
			TextBodyProps: drawing.TextBodyProperties{AutofitType: "normAutofit"},
			Paragraphs:    []*resolve.Paragraph{{Runs: []*resolve.Run{{Text: "Contained ellipse label", FontSize: 2400}}}},
		}}}},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	clipAt := strings.Index(out, `class="shape__text-clip"`)
	if clipAt < 0 {
		t.Fatalf("clipped ellipse text must have a fixed clipping frame: %s", out)
	}
	clipEnd := strings.Index(out[clipAt:], `>`)
	clipTag := out[clipAt : clipAt+clipEnd]
	if !strings.Contains(clipTag, `overflow:hidden`) || !strings.Contains(clipTag, `clip-path:ellipse(50% 50% at 50% 50%)`) {
		t.Fatalf("clipped ellipse frame must retain authored geometry clip: %s", clipTag)
	}
	contentAt := strings.Index(out, `class="shape__content"`)
	if contentAt < 0 {
		t.Fatalf("missing physical line container: %s", out)
	}
	contentEnd := strings.Index(out[contentAt:], `>`)
	contentTag := out[contentAt : contentAt+contentEnd]
	if strings.Contains(contentTag, `clip-path:`) {
		t.Fatalf("physical line container must not own geometry clipping: %s", contentTag)
	}
}

func TestHTMLRotatedFixedTextKeepsFlowInsideClipFrame(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU: 9144000, SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{Shapes: []*resolve.Shape{{
			Kind:   resolve.ShapeText,
			Bounds: resolve.Rect{X: 914400, Y: 685800, W: 1828800, H: 914400},
			TextBodyProps: drawing.TextBodyProperties{
				AutofitType: "normAutofit",
				Rot:         5400000,
				HasRot:      true,
				Anchor:      "ctr",
			},
			Paragraphs: []*resolve.Paragraph{{Runs: []*resolve.Run{{Text: "Rotated fixed text", FontSize: 2400}}}},
		}}}},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	clipAt := strings.Index(out, `class="shape__text-clip"`)
	contentAt := strings.Index(out, `class="shape__content"`)
	if clipAt < 0 || contentAt < 0 || clipAt > contentAt {
		t.Fatalf("rotated physical text must remain inside its clipping frame: %s", out)
	}
	contentEnd := strings.Index(out[contentAt:], `>`)
	contentTag := out[contentAt : contentAt+contentEnd]
	if !strings.Contains(contentTag, `transform:rotate(`) {
		t.Fatalf("layout-owned text rotation missing from clipped content: %s", contentTag)
	}
}

func TestHTMLPhysicalParagraphsDoNotFlexShrinkInsideOverflowingTextFrame(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU: 9144000, SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{Shapes: []*resolve.Shape{{
			Kind:              resolve.ShapeText,
			Bounds:            resolve.Rect{X: 914400, Y: 685800, W: 2743200, H: 228600},
			TextBodyProps:     drawing.TextBodyProperties{AutofitType: "noAutofit"},
			TextAllowOverflow: true,
			Paragraphs: []*resolve.Paragraph{
				{Runs: []*resolve.Run{{Text: "First physical paragraph", FontSize: 2400}}},
				{Runs: []*resolve.Run{{Text: "Second physical paragraph", FontSize: 2400}}},
			},
		}}}},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `.shape--text p { margin: 0; flex-shrink: 0; }`) {
		t.Fatalf("physical paragraphs must not shrink to the authored frame: %s", out)
	}
}

func TestHTMLEmptyTextCapableShapeDoesNotEmitPhantomContentLayer(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU: 9144000, SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{Shapes: []*resolve.Shape{{
			Kind:     resolve.ShapeText,
			Geometry: "ellipse",
			Bounds:   resolve.Rect{X: 914400, Y: 685800, W: 914400, H: 457200},
			Fill:     drawing.SolidFill{Color: "FF0000", Alpha: 1},
		}}}},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="shape shape--text"`) {
		t.Fatalf("expected vector shape wrapper: %s", out)
	}
	if strings.Contains(out, `class="shape__content"`) {
		t.Fatalf("text-capable shape without paragraphs must not emit phantom text content: %s", out)
	}
}

func Test37625PictureBulletRendersAsInlineMarkerImage(t *testing.T) {
	pres := &resolve.Presentation{SlideWidthEMU: 9144000, SlideHeightEMU: 6858000, Slides: []*resolve.Slide{{Shapes: []*resolve.Shape{{
		Kind: resolve.ShapeText, Bounds: resolve.Rect{W: 4000000, H: 1000000}, Paragraphs: []*resolve.Paragraph{{
			BulletImageData: []byte("PNG-BULLET"), BulletImageMIME: "image/png", BulletSizePct: 113071,
			HasMarginLeft: true, MarginLeftEMU: 343080, HasTextIndent: true, TextIndentEMU: -343080,
			Runs: []*resolve.Run{{Text: "The policy context", FontSize: 2800, FontFamily: "Palatino"}},
		}},
	}}}}}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(out, `class="bullet-marker bullet-marker--image"`) || !strings.Contains(out, `src="data:image/png;base64,`) {
		t.Fatalf("picture bullet was not rendered as an inline marker image: %s", out)
	}
	if !strings.Contains(out, `width:21.6113pt;height:21.6113pt`) {
		t.Fatalf("oversized picture bullet was not fitted to the authored hanging region: %s", out)
	}
}

func TestHTMLNativeMathUsesSingleNaturalMetricFit(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU: 9144000, SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{Index: 0, Shapes: []*resolve.Shape{{
			Kind: resolve.ShapeOleObject, Bounds: resolve.Rect{X: 0, Y: 0, W: 1828800, H: 914400},
			OLEMathML: `<math><mi mathsize="18pt">x</mi></math>`,
		}}}},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "inner.style.fontSize=\"16px\"") {
		t.Fatalf("native MathML renderer must measure from one neutral natural size")
	}
	if !strings.Contains(out, "Math.min(box.width/content.width,box.height/content.height)") {
		t.Fatalf("native MathML must use one natural-metric fit to the OLE frame")
	}
	if strings.Contains(out, "data-math-base-ratio") {
		t.Fatalf("native MathML must not inherit preview font/viewBox normalization")
	}
	if strings.Contains(out, "preview.getBBox") || strings.Contains(out, "preview.children") {
		t.Fatalf("native MathML placement must not be derived from preview ink geometry")
	}

}

func TestHTMLNativeMathUsesPreviewMetricsOnlyInsideAbsoluteOLEFrame(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU: 9144000, SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{Index: 0, Shapes: []*resolve.Shape{{
			Kind: resolve.ShapeOleObject, Bounds: resolve.Rect{X: 914400, Y: 685800, W: 1828800, H: 914400},
			OLEMathML:   `<math><mi>x</mi></math>`,
			OLEIconMIME: "image/svg+xml",
			OLEIconData: []byte(`<svg viewBox="0 0 200 100"><text x="20" y="60" font-size="20">x</text></svg>`),
		}}}},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `left:10.0000%;top:10.0000%;width:20.0000%;height:13.3333%`) {
		t.Fatalf("absolute OLE frame placement changed: %s", out)
	}
	if !strings.Contains(out, `data-math-offset-x=`) || !strings.Contains(out, `data-math-baseline-y=`) || !strings.Contains(out, `data-math-center-y=`) {
		t.Fatalf("authored preview centre/baseline calibration was not emitted: %s", out)
	}
	if !strings.Contains(out, `originShift=(isFinite(offsetX)?offsetX:0)*box.width`) {
		t.Fatalf("preview horizontal origin calibration is not applied inside the OLE frame: %s", out)
	}
	if strings.Contains(out, `(box.width-content.width*scale)*0.5`) {
		t.Fatalf("MathML must not be recentered inside the authored OLE frame: %s", out)
	}
	if !strings.Contains(out, `box.height*baselineY-baselineOffset*scale`) || !strings.Contains(out, `baselineOffset=`) {
		t.Fatalf("native MathML must align its baseline to the authored preview baseline inside the absolute OLE frame: %s", out)
	}
	if !strings.Contains(out, `inner.style.top=topShift+"px"`) {
		t.Fatalf("native MathML must apply its computed preview baseline offset: %s", out)
	}
	if strings.Contains(out, "preview.getBBox") || strings.Contains(out, "preview.children") {
		t.Fatalf("browser preview geometry must not become placement authority: %s", out)
	}
}

func TestHTMLNativeMathUsesOutlinedPreviewForegroundColor(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU: 9144000, SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{Index: 0, Shapes: []*resolve.Shape{{
			Kind: resolve.ShapeOleObject, Bounds: resolve.Rect{W: 1828800, H: 914400},
			OLEMathML:   `<math><mi>x</mi></math>`,
			OLEIconMIME: "image/svg+xml",
			OLEIconData: []byte(`<svg viewBox="0 0 100 40"><path style="fill:#ffff00;stroke:none" d="M1 1h10v10z"/></svg>`),
		}}}},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `color:#ffff00`) {
		t.Fatalf("outlined MathType preview color was not inherited by native MathML: %s", out)
	}
}

func TestHTMLNativeMathPrefersTextColorOverDecorativePathPaint(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU: 9144000, SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{Index: 0, Shapes: []*resolve.Shape{{
			Kind: resolve.ShapeOleObject, Bounds: resolve.Rect{W: 1828800, H: 914400},
			OLEMathML:   `<math><mi>x</mi></math>`,
			OLEIconMIME: "image/svg+xml",
			OLEIconData: []byte(`<svg><path fill="#000000" d="M0 0h1v1z"/><text fill="#ffffff">x</text></svg>`),
		}}}},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `color:#ffffff`) {
		t.Fatalf("native MathML did not prefer authored text paint: %s", out)
	}
}
