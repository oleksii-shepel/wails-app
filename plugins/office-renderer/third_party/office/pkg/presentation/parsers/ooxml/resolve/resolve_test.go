package resolve_test

import (
	"bytes"
	"encoding/xml"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/drawingxml"
	"office/pkg/ooxml/media"
	"office/pkg/ooxml/opc"
	"office/pkg/presentation/parsers/ooxml/parts"
	"office/pkg/presentation/parsers/ooxml/resolve"
)

func TestResolvePackageInheritsLayoutPlaceholderRunStyle(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		Slides: []*parts.SlideEntry{
			{
				Slide: &parts.Slide{
					CSld: parts.CommonSlideData{
						SpTree: parts.ShapeTree{
							Shapes: []parts.Shape{
								{
									NvSpPr: parts.NonVisualShapeProps{
										CNvPr: parts.CnvPr{ID: 1, Name: "Subtitle"},
										NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "subTitle", Idx: 1}},
									},
									SpPr: parts.ShapeProperties{
										Xfrm: &parts.Transform2D{
											Off: parts.Offset{X: 1000, Y: 2000},
											Ext: parts.Extents{CX: 3000, CY: 4000},
										},
									},
									TxBody: &parts.TextBody{
										Paragraphs: []parts.DMLParagraph{
											{
												Properties: &parts.DMLParaProps{Algn: "ctr"},
												Runs: []parts.DMLRun{
													{Text: "Digital experience designer", Properties: &parts.DMLRunPr{Lang: "en"}},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				Layout: &parts.SlideLayout{
					CSld: parts.CommonSlideData{
						SpTree: parts.ShapeTree{
							Shapes: []parts.Shape{
								{
									NvSpPr: parts.NonVisualShapeProps{
										CNvPr: parts.CnvPr{ID: 2, Name: "Layout Subtitle"},
										NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "subTitle", Idx: 1}},
									},
									TxBody: &parts.TextBody{
										ListStyle: &parts.DMLTextListStyle{
											Lvl1PPr: &parts.DMLTextStyleLevel{
												Algn:   "ctr",
												DefRPr: &parts.DMLRunPr{FontSize: 3600, SchemeColor: &drawingxml.SchemeClr{Val: "lt1"}, Latin: &parts.FontRef{Typeface: "Oswald"}},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		Themes: map[string]*parts.Theme{
			"ppt/theme/theme1.xml": {
				Name: "Test Theme",
				Elements: parts.ThemeElements{
					ClrScheme: parts.ColorScheme{
						LT1: &parts.ThemeColor{SrgbClr: &parts.SrgbColor{Val: "FFFFFF"}},
					},
					FontScheme: parts.FontScheme{
						MajorFont: parts.FontGroup{Latin: parts.FontRef{Typeface: "Arial"}},
						MinorFont: parts.FontGroup{Latin: parts.FontRef{Typeface: "Arial"}},
					},
				},
			},
		},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(pres.Slides) != 1 || len(pres.Slides[0].Shapes) != 1 {
		t.Fatalf("unexpected shape count: %#v", pres.Slides)
	}
	shape := pres.Slides[0].Shapes[0]
	if len(shape.Paragraphs) != 1 || len(shape.Paragraphs[0].Runs) != 1 {
		t.Fatalf("unexpected paragraph count: %#v", shape.Paragraphs)
	}
	run := shape.Paragraphs[0].Runs[0]
	if run.FontSize != 3600 {
		t.Fatalf("font size = %d, want 3600", run.FontSize)
	}
	if run.FontFamily != "Oswald" {
		t.Fatalf("font family = %q, want %q", run.FontFamily, "Oswald")
	}
	if run.Color != "FFFFFF" {
		t.Fatalf("color = %q, want %q", run.Color, "FFFFFF")
	}
}

func TestResolvePackageAppliesMasterColorMapToSlideSchemeColor(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000}},
		SlideMasters: []*parts.SlideMasterEntry{{
			PartName: "ppt/slideMasters/slideMaster1.xml",
			Master: &parts.SlideMaster{
				ClrMap: &parts.ColorMap{Tx1: "lt1"},
			},
		}},
		Slides: []*parts.SlideEntry{{
			MasterPartName: "ppt/slideMasters/slideMaster1.xml",
			Slide: &parts.Slide{
				CSld: parts.CommonSlideData{
					SpTree: parts.ShapeTree{
						Shapes: []parts.Shape{{
							NvSpPr: parts.NonVisualShapeProps{
								CNvPr: parts.CnvPr{ID: 1, Name: "Title"},
							},
							TxBody: &parts.TextBody{
								Paragraphs: []parts.DMLParagraph{{
									Runs: []parts.DMLRun{{
										Text:       "Hello",
										Properties: &parts.DMLRunPr{SchemeColor: &drawingxml.SchemeClr{Val: "tx1"}},
									}},
								}},
							},
						}},
					},
				},
			},
		}},
		Themes: map[string]*parts.Theme{
			"ppt/theme/theme1.xml": {
				Elements: parts.ThemeElements{
					ClrScheme: parts.ColorScheme{
						DK1: &parts.ThemeColor{SrgbClr: &parts.SrgbColor{Val: "000000"}},
						LT1: &parts.ThemeColor{SrgbClr: &parts.SrgbColor{Val: "FFFFFF"}},
					},
				},
			},
		},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	run := pres.Slides[0].Shapes[0].Paragraphs[0].Runs[0]
	if run.Color != "FFFFFF" {
		t.Fatalf("run color = %q, want FFFFFF", run.Color)
	}
}

func TestResolvePackageAppliesLayoutColorMapOverrideToInheritedDefaults(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000}},
		Slides: []*parts.SlideEntry{{
			Slide: &parts.Slide{
				CSld: parts.CommonSlideData{
					SpTree: parts.ShapeTree{
						Shapes: []parts.Shape{{
							NvSpPr: parts.NonVisualShapeProps{
								CNvPr: parts.CnvPr{ID: 1, Name: "Body"},
								NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "body", Idx: 1}},
							},
							TxBody: &parts.TextBody{
								Paragraphs: []parts.DMLParagraph{{
									Runs: []parts.DMLRun{{Text: "Hello"}},
								}},
							},
						}},
					},
				},
			},
			Layout: &parts.SlideLayout{
				ClrMapOvr: &parts.ColorMapOverride{
					OverrideClrMapping: &parts.OverrideColorMapping{
						ColorMap: parts.ColorMap{Tx1: "accent1"},
					},
				},
				CSld: parts.CommonSlideData{
					SpTree: parts.ShapeTree{
						Shapes: []parts.Shape{{
							NvSpPr: parts.NonVisualShapeProps{
								CNvPr: parts.CnvPr{ID: 2, Name: "Layout Body"},
								NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "body", Idx: 1}},
							},
							TxBody: &parts.TextBody{
								ListStyle: &parts.DMLTextListStyle{
									Lvl1PPr: &parts.DMLTextStyleLevel{
										DefRPr: &parts.DMLRunPr{SchemeColor: &drawingxml.SchemeClr{Val: "tx1"}},
									},
								},
							},
						}},
					},
				},
			},
		}},
		Themes: map[string]*parts.Theme{
			"ppt/theme/theme1.xml": {
				Elements: parts.ThemeElements{
					ClrScheme: parts.ColorScheme{
						DK1:     &parts.ThemeColor{SrgbClr: &parts.SrgbColor{Val: "000000"}},
						Accent1: &parts.ThemeColor{SrgbClr: &parts.SrgbColor{Val: "00FF00"}},
					},
				},
			},
		},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	run := pres.Slides[0].Shapes[0].Paragraphs[0].Runs[0]
	if run.Color != "00FF00" {
		t.Fatalf("run color = %q, want 00FF00", run.Color)
	}
}

func TestResolvePackageAppliesSlideColorMapOverrideToRunColor(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000}},
		Slides: []*parts.SlideEntry{{
			Slide: &parts.Slide{
				ClrMapOvr: &parts.ColorMapOverride{
					OverrideClrMapping: &parts.OverrideColorMapping{
						ColorMap: parts.ColorMap{Tx1: "accent2"},
					},
				},
				CSld: parts.CommonSlideData{
					SpTree: parts.ShapeTree{
						Shapes: []parts.Shape{{
							NvSpPr: parts.NonVisualShapeProps{CNvPr: parts.CnvPr{ID: 1, Name: "Text"}},
							TxBody: &parts.TextBody{
								Paragraphs: []parts.DMLParagraph{{
									Runs: []parts.DMLRun{{
										Text:       "Hello",
										Properties: &parts.DMLRunPr{SchemeColor: &drawingxml.SchemeClr{Val: "tx1"}},
									}},
								}},
							},
						}},
					},
				},
			},
		}},
		Themes: map[string]*parts.Theme{
			"ppt/theme/theme1.xml": {
				Elements: parts.ThemeElements{
					ClrScheme: parts.ColorScheme{
						DK1:     &parts.ThemeColor{SrgbClr: &parts.SrgbColor{Val: "000000"}},
						Accent2: &parts.ThemeColor{SrgbClr: &parts.SrgbColor{Val: "FF8800"}},
					},
				},
			},
		},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	run := pres.Slides[0].Shapes[0].Paragraphs[0].Runs[0]
	if run.Color != "FF8800" {
		t.Fatalf("run color = %q, want FF8800", run.Color)
	}
}

func TestResolvePackageAppliesSlideThemeOverrideToSchemeColor(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000}},
		Slides: []*parts.SlideEntry{{
			Slide: &parts.Slide{
				ThemeOverride: &parts.ThemeOverride{
					ClrScheme: &parts.ColorScheme{
						Accent1: &parts.ThemeColor{SrgbClr: &parts.SrgbColor{Val: "00FF00"}},
					},
				},
				CSld: parts.CommonSlideData{
					SpTree: parts.ShapeTree{
						Shapes: []parts.Shape{{
							NvSpPr: parts.NonVisualShapeProps{CNvPr: parts.CnvPr{ID: 1, Name: "Text"}},
							TxBody: &parts.TextBody{
								Paragraphs: []parts.DMLParagraph{{
									Runs: []parts.DMLRun{{
										Text:       "Hello",
										Properties: &parts.DMLRunPr{SchemeColor: &drawingxml.SchemeClr{Val: "accent1"}},
									}},
								}},
							},
						}},
					},
				},
			},
		}},
		Themes: map[string]*parts.Theme{
			"ppt/theme/theme1.xml": {
				Elements: parts.ThemeElements{
					ClrScheme: parts.ColorScheme{
						Accent1: &parts.ThemeColor{SrgbClr: &parts.SrgbColor{Val: "4472C4"}},
					},
				},
			},
		},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	run := pres.Slides[0].Shapes[0].Paragraphs[0].Runs[0]
	if run.Color != "00FF00" {
		t.Fatalf("run color = %q, want 00FF00", run.Color)
	}
}

func TestResolvePackageAppliesLayoutThemeOverrideToFontAndFillRefs(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000}},
		Slides: []*parts.SlideEntry{{
			Layout: &parts.SlideLayout{
				ThemeOverride: &parts.ThemeOverride{
					FontScheme: &parts.FontScheme{
						MajorFont: parts.FontGroup{Latin: parts.FontRef{Typeface: "Aptos Display"}},
						MinorFont: parts.FontGroup{Latin: parts.FontRef{Typeface: "Fira Sans"}},
					},
					FmtScheme: &parts.FormatScheme{
						FillStyleList: []parts.ThemeFormatFillStyle{
							{SolidFill: &parts.ColorRef{Val: "ABCDEF"}},
						},
					},
				},
			},
			Slide: &parts.Slide{
				CSld: parts.CommonSlideData{
					SpTree: parts.ShapeTree{
						Shapes: []parts.Shape{{
							NvSpPr: parts.NonVisualShapeProps{CNvPr: parts.CnvPr{ID: 1, Name: "Styled"}},
							Style: &parts.ShapeStyle{
								FillRef: &parts.ShapeStyleRef{Idx: 1},
								FontRef: &parts.ShapeFontRef{Idx: "minor"},
							},
							TxBody: &parts.TextBody{
								Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{{Text: "Hi"}}}},
							},
						}},
					},
				},
			},
		}},
		Themes: map[string]*parts.Theme{
			"ppt/theme/theme1.xml": {
				Name: "Base Theme",
				Elements: parts.ThemeElements{
					ClrScheme: parts.ColorScheme{
						Accent1: &parts.ThemeColor{SrgbClr: &parts.SrgbColor{Val: "4472C4"}},
					},
					FontScheme: parts.FontScheme{
						MajorFont: parts.FontGroup{Latin: parts.FontRef{Typeface: "Calibri"}},
						MinorFont: parts.FontGroup{Latin: parts.FontRef{Typeface: "Arial"}},
					},
					FmtScheme: parts.FormatScheme{
						FillStyleList: []parts.ThemeFormatFillStyle{
							{SolidFill: &parts.ColorRef{Val: "FF0000"}},
						},
					},
				},
			},
		},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	shape := pres.Slides[0].Shapes[0]
	if shape.FillColor != "ABCDEF" {
		t.Fatalf("fillColor = %q, want ABCDEF", shape.FillColor)
	}
	run := shape.Paragraphs[0].Runs[0]
	if run.FontFamily != "Fira Sans" {
		t.Fatalf("fontFamily = %q, want Fira Sans", run.FontFamily)
	}
}

func TestResolvePackageAppliesNotesThemeOverrideToRunColor(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000}},
		Slides: []*parts.SlideEntry{{
			Slide: &parts.Slide{
				CSld: parts.CommonSlideData{
					SpTree: parts.ShapeTree{
						Shapes: []parts.Shape{{
							NvSpPr: parts.NonVisualShapeProps{CNvPr: parts.CnvPr{ID: 1, Name: "Main"}},
							TxBody: &parts.TextBody{Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{{Text: "Slide"}}}}},
						}},
					},
				},
			},
			Notes: &parts.NotesSlide{
				ThemeOverride: &parts.ThemeOverride{
					ClrScheme: &parts.ColorScheme{
						Accent1: &parts.ThemeColor{SrgbClr: &parts.SrgbColor{Val: "00FF00"}},
					},
				},
				CSld: parts.CommonSlideData{
					SpTree: parts.ShapeTree{
						Shapes: []parts.Shape{{
							NvSpPr: parts.NonVisualShapeProps{CNvPr: parts.CnvPr{ID: 2, Name: "Notes"}},
							TxBody: &parts.TextBody{
								Paragraphs: []parts.DMLParagraph{{
									Runs: []parts.DMLRun{{
										Text:       "Speaker notes",
										Properties: &parts.DMLRunPr{SchemeColor: &drawingxml.SchemeClr{Val: "accent1"}},
									}},
								}},
							},
						}},
					},
				},
			},
		}},
		Themes: map[string]*parts.Theme{
			"ppt/theme/theme1.xml": {
				Elements: parts.ThemeElements{
					ClrScheme: parts.ColorScheme{
						Accent1: &parts.ThemeColor{SrgbClr: &parts.SrgbColor{Val: "4472C4"}},
					},
				},
			},
		},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if pres.Slides[0].Notes == nil {
		t.Fatal("notes = nil, want resolved notes slide")
	}
	run := pres.Slides[0].Notes.Shapes[0].Paragraphs[0].Runs[0]
	if run.Color != "00FF00" {
		t.Fatalf("notes run color = %q, want 00FF00", run.Color)
	}
}

func TestResolvePackageParsesOLEObjectShapes(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		Slides: []*parts.SlideEntry{
			{
				Slide: &parts.Slide{
					CSld: parts.CommonSlideData{
						SpTree: parts.ShapeTree{
							Items: []parts.ShapeTreeItem{
								{Kind: parts.ShapeTreeItemOleObject, Index: 0},
							},
							OleObjectFrames: []parts.OleObjectFrame{
								{
									ID:         4,
									Name:       "Object",
									Xfrm:       &parts.Transform2D{Off: parts.Offset{X: 1000000, Y: 1000000}, Ext: parts.Extents{CX: 2000000, CY: 1500000}},
									RelID:      "rId3",
									Embed:      true,
									ShowAsIcon: true,
									ProgID:     "Word.Document.12",
									ObjectName: "Document",
									IconRelID:  "rId4",
								},
							},
						},
					},
				},
				OleObjectParts: map[string]media.Part{
					"rId3": {Name: "ppt/embeddings/oleObject1.bin", ContentType: "application/vnd.openxmlformats-officedocument.oleObject", Data: []byte("OLE payload")},
				},
				MediaParts: map[string]media.Part{
					"rId4": {Name: "ppt/media/image1.png", ContentType: "image/png", Data: []byte("PNG icon")},
				},
			},
		},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(pres.Slides) != 1 || len(pres.Slides[0].Shapes) != 1 {
		t.Fatalf("unexpected shape count: %#v", pres.Slides[0].Shapes)
	}
	shape := pres.Slides[0].Shapes[0]
	if shape.Kind != resolve.ShapeOleObject {
		t.Fatalf("shape.Kind = %v, want ShapeOleObject", shape.Kind)
	}
	if shape.Bounds.X != 1000000 || shape.Bounds.Y != 1000000 || shape.Bounds.W != 2000000 || shape.Bounds.H != 1500000 {
		t.Fatalf("unexpected bounds: %+v", shape.Bounds)
	}
	if shape.OLEObjectName != "Document" {
		t.Errorf("OLEObjectName = %q, want Document", shape.OLEObjectName)
	}
	if shape.OLEObjectProgID != "Word.Document.12" {
		t.Errorf("OLEObjectProgID = %q, want Word.Document.12", shape.OLEObjectProgID)
	}
	if string(shape.OLEObjectData) != "OLE payload" {
		t.Errorf("OLEObjectData = %q, want OLE payload", string(shape.OLEObjectData))
	}
	if string(shape.OLEIconData) != "PNG icon" {
		t.Errorf("OLEIconData = %q, want PNG icon", string(shape.OLEIconData))
	}
}

func TestResolvePackagePrefersMasterPlaceholderFontOverMasterTextStyleFallback(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		Slides: []*parts.SlideEntry{
			{
				Slide: &parts.Slide{
					CSld: parts.CommonSlideData{
						SpTree: parts.ShapeTree{
							Shapes: []parts.Shape{
								{
									NvSpPr: parts.NonVisualShapeProps{
										CNvPr: parts.CnvPr{ID: 1, Name: "Title"},
										NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "title"}},
									},
									TxBody: &parts.TextBody{
										Paragraphs: []parts.DMLParagraph{
											{Runs: []parts.DMLRun{{Text: "Title text", Properties: &parts.DMLRunPr{Lang: "en"}}}},
										},
									},
								},
							},
						},
					},
				},
				Layout: &parts.SlideLayout{
					CSld: parts.CommonSlideData{
						SpTree: parts.ShapeTree{
							Shapes: []parts.Shape{
								{
									NvSpPr: parts.NonVisualShapeProps{
										CNvPr: parts.CnvPr{ID: 2, Name: "Layout Title"},
										NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "title"}},
									},
									TxBody: &parts.TextBody{
										ListStyle: &parts.DMLTextListStyle{
											Lvl1PPr: &parts.DMLTextStyleLevel{
												DefRPr: &parts.DMLRunPr{FontSize: 3600, SchemeColor: &drawingxml.SchemeClr{Val: "lt1"}},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		SlideMasters: []*parts.SlideMasterEntry{
			{
				Master: &parts.SlideMaster{
					CSld: parts.CommonSlideData{
						SpTree: parts.ShapeTree{
							Shapes: []parts.Shape{
								{
									NvSpPr: parts.NonVisualShapeProps{
										CNvPr: parts.CnvPr{ID: 3, Name: "Master Title"},
										NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "title"}},
									},
									TxBody: &parts.TextBody{
										ListStyle: &parts.DMLTextListStyle{
											Lvl1PPr: &parts.DMLTextStyleLevel{
												DefRPr: &parts.DMLRunPr{Latin: &parts.FontRef{Typeface: "Oswald"}},
											},
										},
									},
								},
							},
						},
					},
					TxStyles: &parts.MasterTxStyles{
						TitleStyle: &parts.DMLTextListStyle{
							Lvl1PPr: &parts.DMLTextStyleLevel{
								DefRPr: &parts.DMLRunPr{Latin: &parts.FontRef{Typeface: "Arial"}},
							},
						},
					},
				},
			},
		},
		Themes: map[string]*parts.Theme{
			"ppt/theme/theme1.xml": {
				Name: "Test Theme",
				Elements: parts.ThemeElements{
					ClrScheme: parts.ColorScheme{
						LT1: &parts.ThemeColor{SrgbClr: &parts.SrgbColor{Val: "FFFFFF"}},
					},
					FontScheme: parts.FontScheme{
						MajorFont: parts.FontGroup{Latin: parts.FontRef{Typeface: "Roboto"}},
						MinorFont: parts.FontGroup{Latin: parts.FontRef{Typeface: "Source Code Pro"}},
					},
				},
			},
		},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	run := pres.Slides[0].Shapes[0].Paragraphs[0].Runs[0]
	if run.FontFamily != "Oswald" {
		t.Fatalf("font family = %q, want Oswald", run.FontFamily)
	}
}

func TestResolvePackageAppliesPptxTableStyleInheritance(t *testing.T) {
	var theme parts.Theme
	if err := xml.Unmarshal([]byte(`<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:themeElements>
			<a:clrScheme name="Fixture Colors">
				<a:lt1><a:srgbClr val="FFFFFF"/></a:lt1>
				<a:accent1><a:srgbClr val="336699"/></a:accent1>
				<a:accent2><a:srgbClr val="CCE5FF"/></a:accent2>
				<a:accent3><a:srgbClr val="E2F0D9"/></a:accent3>
				<a:accent4><a:srgbClr val="CC6600"/></a:accent4>
			</a:clrScheme>
			<a:fontScheme name="Fixture Fonts">
				<a:minorFont><a:latin typeface="Aptos"/></a:minorFont>
			</a:fontScheme>
			<a:fmtScheme name="Fixture Matrix">
				<a:lnStyleLst>
					<a:ln w="12700">
						<a:solidFill>
							<a:schemeClr val="phClr">
								<a:lumMod val="50000"/>
							</a:schemeClr>
						</a:solidFill>
					</a:ln>
				</a:lnStyleLst>
				<a:fillStyleLst>
					<a:solidFill>
						<a:schemeClr val="phClr">
							<a:lumMod val="50000"/>
							<a:lumOff val="50000"/>
						</a:schemeClr>
					</a:solidFill>
				</a:fillStyleLst>
			</a:fmtScheme>
		</a:themeElements>
	</a:theme>`), &theme); err != nil {
		t.Fatalf("unmarshal theme: %v", err)
	}

	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		Slides: []*parts.SlideEntry{{
			Slide: &parts.Slide{CSld: parts.CommonSlideData{SpTree: parts.ShapeTree{Tables: []parts.TableFrame{{
				ID:   7,
				Name: "Styled Table",
				Xfrm: &parts.Transform2D{Off: parts.Offset{X: 1000, Y: 1000}, Ext: parts.Extents{CX: 4000, CY: 3000}},
				Table: &parts.PresentationTable{
					Props: &parts.PresentationTableProps{TableStyleID: "{STYLE-ID}", FirstRow: true, BandRow: true},
					Grid:  parts.PresentationTableGrid{Columns: []parts.PresentationTableColumn{{W: 2000}, {W: 2000}}},
					Rows: []parts.PresentationTableRow{{
						Cells: []parts.PresentationTableCell{{
							TextBody: &parts.TextBody{Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{{Text: "Header"}}}}},
						}, {
							Props:    &parts.PresentationTableCellProps{MarL: 120},
							TextBody: &parts.TextBody{Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{{Text: "Head 2"}}}}},
						}},
					}, {
						Cells: []parts.PresentationTableCell{{
							TextBody: &parts.TextBody{Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{{Text: "Band 1"}}}}},
						}, {
							TextBody: &parts.TextBody{Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{{Text: "Band 1b"}}}}},
						}},
					}, {
						Cells: []parts.PresentationTableCell{{
							TextBody: &parts.TextBody{Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{{Text: "Band 2"}}}}},
						}, {
							TextBody: &parts.TextBody{Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{{Text: "Band 2b"}}}}},
						}},
					}},
				},
			}}}}},
		}},
		TableStyles: &parts.TableStyleList{Default: "{STYLE-ID}", Styles: []parts.DrawingTableStyle{{
			StyleID:  "{STYLE-ID}",
			WholeTbl: &parts.TableStyleRegion{CellStyle: &parts.DrawingTableCellStyle{FillRef: &parts.DrawingTableFillRef{Idx: 1, SchemeClr: &drawingxml.SchemeClr{Val: "accent1"}}}},
			FirstRow: &parts.TableStyleRegion{
				CellStyle: &parts.DrawingTableCellStyle{Borders: &parts.DrawingTableCellBorders{Bottom: &parts.DrawingTableBorder{LineRef: &parts.DrawingTableLineRef{Idx: 1, SchemeClr: &drawingxml.SchemeClr{Val: "accent4"}}}}},
				TextStyle: &parts.DrawingTableCellTextStyle{Bold: "on", FontRef: &parts.DrawingTableFontRef{Idx: "minor", Color: &parts.ColorRef{Val: "FF00FF"}}},
			},
			Band1H: &parts.TableStyleRegion{CellStyle: &parts.DrawingTableCellStyle{FillScheme: &drawingxml.SchemeClr{Val: "accent2"}}},
			Band2H: &parts.TableStyleRegion{CellStyle: &parts.DrawingTableCellStyle{FillScheme: &drawingxml.SchemeClr{Val: "accent3"}}},
		}}},
		Themes: map[string]*parts.Theme{
			"ppt/theme/theme1.xml": {
				Elements: theme.Elements,
			},
		},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	shape := pres.Slides[0].Shapes[0]
	if shape.Kind != resolve.ShapeTable {
		t.Fatalf("shape kind = %v, want table", shape.Kind)
	}
	headerRun := shape.Table.Rows[0].Cells[0].Paragraphs[0].Runs[0]
	if !headerRun.Bold || headerRun.Color != "FF00FF" || headerRun.FontFamily != "Aptos" {
		t.Fatalf("header run = %#v, want bold Aptos magenta text", headerRun)
	}
	if got := shape.Table.Rows[0].Cells[1].FillColor; got != "99B3CC" {
		t.Fatalf("header fill = %q, want 99B3CC", got)
	}
	if shape.Table.Rows[0].Cells[0].BorderBottom == nil || shape.Table.Rows[0].Cells[0].BorderBottom.Color != "663300" {
		t.Fatalf("header bottom border = %#v, want color 663300", shape.Table.Rows[0].Cells[0].BorderBottom)
	}
	if got := shape.Table.Rows[1].Cells[0].FillColor; got != "CCE5FF" {
		t.Fatalf("band1 fill = %q, want CCE5FF", got)
	}
	if got := shape.Table.Rows[2].Cells[0].FillColor; got != "E2F0D9" {
		t.Fatalf("band2 fill = %q, want E2F0D9", got)
	}
}

func TestResolvePackageAppliesFullDrawingMLTransformsToPptxTableStyles(t *testing.T) {
	var theme parts.Theme
	if err := xml.Unmarshal([]byte(`<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:themeElements>
			<a:clrScheme name="Fixture Colors">
				<a:accent1><a:srgbClr val="FF0000"/></a:accent1>
				<a:accent2><a:srgbClr val="00FF00"/></a:accent2>
			</a:clrScheme>
			<a:fmtScheme name="Fixture Matrix">
				<a:lnStyleLst>
					<a:ln w="12700">
						<a:solidFill>
							<a:schemeClr val="phClr">
								<a:red val="50000"/>
							</a:schemeClr>
						</a:solidFill>
					</a:ln>
				</a:lnStyleLst>
			</a:fmtScheme>
		</a:themeElements>
	</a:theme>`), &theme); err != nil {
		t.Fatalf("unmarshal theme: %v", err)
	}

	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000}},
		Slides: []*parts.SlideEntry{{
			Slide: &parts.Slide{CSld: parts.CommonSlideData{SpTree: parts.ShapeTree{Tables: []parts.TableFrame{{
				ID:   9,
				Name: "Transform Table",
				Table: &parts.PresentationTable{
					Props: &parts.PresentationTableProps{TableStyleID: "{STYLE-ID}", FirstRow: true},
					Grid:  parts.PresentationTableGrid{Columns: []parts.PresentationTableColumn{{W: 2000}}},
					Rows: []parts.PresentationTableRow{{
						Cells: []parts.PresentationTableCell{{
							TextBody: &parts.TextBody{Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{{Text: "Header"}}}}},
						}},
					}},
				},
			}}}}},
		}},
		TableStyles: &parts.TableStyleList{Default: "{STYLE-ID}", Styles: []parts.DrawingTableStyle{{
			StyleID: "{STYLE-ID}",
			WholeTbl: &parts.TableStyleRegion{CellStyle: &parts.DrawingTableCellStyle{
				FillScheme: &drawingxml.SchemeClr{Val: "accent1", Gray: &struct{}{}},
			}},
			FirstRow: &parts.TableStyleRegion{CellStyle: &parts.DrawingTableCellStyle{
				Borders: &parts.DrawingTableCellBorders{
					Bottom: &parts.DrawingTableBorder{LineRef: &parts.DrawingTableLineRef{Idx: 1, SchemeClr: &drawingxml.SchemeClr{Val: "accent2"}}},
				},
			}},
		}}},
		Themes: map[string]*parts.Theme{"ppt/theme/theme1.xml": {Elements: theme.Elements}},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	cell := pres.Slides[0].Shapes[0].Table.Rows[0].Cells[0]
	if got := cell.FillColor; got != "4C4C4C" {
		t.Fatalf("fill color = %q, want 4C4C4C", got)
	}
	if cell.BorderBottom == nil || cell.BorderBottom.Color != "80FF00" {
		t.Fatalf("bottom border = %#v, want color 80FF00", cell.BorderBottom)
	}
}

func TestResolvePackageAppliesPptxTableStyleMatrixGradientAndEffects(t *testing.T) {
	var theme parts.Theme
	if err := xml.Unmarshal([]byte(`<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:themeElements>
			<a:clrScheme name="Fixture Colors">
				<a:lt1><a:srgbClr val="FFFFFF"/></a:lt1>
				<a:accent1><a:srgbClr val="336699"/></a:accent1>
				<a:accent4><a:srgbClr val="CC6600"/></a:accent4>
			</a:clrScheme>
			<a:fmtScheme name="Fixture Matrix">
				<a:fillStyleLst>
					<a:gradFill>
						<a:gsLst>
							<a:gs pos="0"><a:schemeClr val="phClr"/></a:gs>
							<a:gs pos="100000"><a:schemeClr val="lt1"/></a:gs>
						</a:gsLst>
						<a:lin ang="5400000"/>
					</a:gradFill>
				</a:fillStyleLst>
				<a:effectStyleLst>
					<a:effectStyle>
						<a:effectLst>
							<a:outerShdw blurRad="12700" dist="6350" dir="5400000">
								<a:schemeClr val="phClr"><a:alpha val="60000"/></a:schemeClr>
							</a:outerShdw>
						</a:effectLst>
					</a:effectStyle>
				</a:effectStyleLst>
			</a:fmtScheme>
		</a:themeElements>
	</a:theme>`), &theme); err != nil {
		t.Fatalf("unmarshal theme: %v", err)
	}

	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000}},
		Slides: []*parts.SlideEntry{{
			Slide: &parts.Slide{CSld: parts.CommonSlideData{SpTree: parts.ShapeTree{Tables: []parts.TableFrame{{
				ID:   8,
				Name: "Matrix Table",
				Table: &parts.PresentationTable{
					Props: &parts.PresentationTableProps{TableStyleID: "{STYLE-ID}", FirstRow: true},
					Grid:  parts.PresentationTableGrid{Columns: []parts.PresentationTableColumn{{W: 2000}}},
					Rows: []parts.PresentationTableRow{{
						Cells: []parts.PresentationTableCell{{
							TextBody: &parts.TextBody{Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{{Text: "Header"}}}}},
						}},
					}},
				},
			}}}}},
		}},
		TableStyles: &parts.TableStyleList{Default: "{STYLE-ID}", Styles: []parts.DrawingTableStyle{{
			StyleID:  "{STYLE-ID}",
			WholeTbl: &parts.TableStyleRegion{CellStyle: &parts.DrawingTableCellStyle{FillRef: &parts.DrawingTableFillRef{Idx: 1, SchemeClr: &drawingxml.SchemeClr{Val: "accent1"}}}},
			FirstRow: &parts.TableStyleRegion{CellStyle: &parts.DrawingTableCellStyle{EffectRef: &parts.DrawingTableEffectRef{Idx: 1, SchemeClr: &drawingxml.SchemeClr{Val: "accent4"}}}},
		}}},
		Themes: map[string]*parts.Theme{"ppt/theme/theme1.xml": {Elements: theme.Elements}},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	cell := pres.Slides[0].Shapes[0].Table.Rows[0].Cells[0]
	if got := cell.FillColor; got != "336699" {
		t.Fatalf("fill color = %q, want 336699", got)
	}
	gradient, ok := cell.Fill.(drawing.GradientFill)
	if !ok {
		t.Fatalf("fill = %#v, want drawing.GradientFill", cell.Fill)
	}
	if len(gradient.Stops) != 2 || gradient.Stops[0].Color != "336699" || gradient.Stops[1].Color != "FFFFFF" {
		t.Fatalf("gradient stops = %#v, want accent1 to lt1", gradient.Stops)
	}
	if len(cell.Effects.Effects) != 1 {
		t.Fatalf("effects = %#v, want one effect", cell.Effects)
	}
	shadow, ok := cell.Effects.Effects[0].(drawing.Shadow)
	if !ok {
		t.Fatalf("effect = %#v, want drawing.Shadow", cell.Effects.Effects[0])
	}
	if shadow.Color != "CC6600" || shadow.Alpha != 0.6 {
		t.Fatalf("shadow = %#v, want accent4 with alpha 0.6", shadow)
	}
}

func TestResolvePackageResolvesThemeTypefaceTokensAndStrike(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		Slides: []*parts.SlideEntry{
			{
				Slide: &parts.Slide{
					CSld: parts.CommonSlideData{
						SpTree: parts.ShapeTree{
							Shapes: []parts.Shape{
								{
									NvSpPr: parts.NonVisualShapeProps{CNvPr: parts.CnvPr{ID: 1, Name: "Fonts"}},
									TxBody: &parts.TextBody{
										Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{
											{Text: "Major", Properties: &parts.DMLRunPr{Latin: &parts.FontRef{Typeface: "+mj-lt"}, Strike: "sngStrike"}},
											{Text: "EA", Properties: &parts.DMLRunPr{EA: &parts.FontRef{Typeface: "+mn-ea"}}},
										}}},
									},
								},
							},
						},
					},
				},
			},
		},
		Themes: map[string]*parts.Theme{
			"ppt/theme/theme1.xml": {
				Elements: parts.ThemeElements{
					FontScheme: parts.FontScheme{
						MajorFont: parts.FontGroup{
							Latin: parts.FontRef{Typeface: "Aptos Display"},
							EA:    parts.FontRef{Typeface: "MS Gothic"},
						},
						MinorFont: parts.FontGroup{
							Latin: parts.FontRef{Typeface: "Aptos"},
							EA:    parts.FontRef{Typeface: "Yu Gothic"},
						},
					},
				},
			},
		},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	runs := pres.Slides[0].Shapes[0].Paragraphs[0].Runs
	if len(runs) != 2 {
		t.Fatalf("run count = %d, want 2", len(runs))
	}
	if runs[0].FontFamily != "Aptos Display" {
		t.Fatalf("run[0] font family = %q, want Aptos Display", runs[0].FontFamily)
	}
	if !runs[0].Strike {
		t.Fatal("run[0] strike = false, want true")
	}
	if runs[1].FontFamily != "Yu Gothic" {
		t.Fatalf("run[1] font family = %q, want Yu Gothic", runs[1].FontFamily)
	}
}

func TestResolvePackageInheritsTextBodyAndParagraphSpacing(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		Slides: []*parts.SlideEntry{
			{
				Slide: &parts.Slide{
					CSld: parts.CommonSlideData{
						SpTree: parts.ShapeTree{
							Shapes: []parts.Shape{
								{
									NvSpPr: parts.NonVisualShapeProps{
										CNvPr: parts.CnvPr{ID: 1, Name: "Subtitle"},
										NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "subTitle", Idx: 1}},
									},
									TxBody: &parts.TextBody{
										Paragraphs: []parts.DMLParagraph{
											{Runs: []parts.DMLRun{{Text: "Digital experience designer"}}},
										},
									},
								},
							},
						},
					},
				},
				Layout: &parts.SlideLayout{
					CSld: parts.CommonSlideData{
						SpTree: parts.ShapeTree{
							Shapes: []parts.Shape{
								{
									NvSpPr: parts.NonVisualShapeProps{
										CNvPr: parts.CnvPr{ID: 2, Name: "Layout Subtitle"},
										NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "subTitle", Idx: 1}},
									},
									TxBody: &parts.TextBody{
										BodyPr: &parts.DMLBodyPr{
											Anchor: "ctr",
											LIns:   int64Ptr(91425),
											RIns:   int64Ptr(91425),
											TIns:   int64Ptr(91425),
											BIns:   int64Ptr(91425),
										},
										ListStyle: &parts.DMLTextListStyle{
											Lvl1PPr: &parts.DMLTextStyleLevel{
												SpcBef: &parts.DMLSpacing{SpcPts: &parts.SpacingPts{Val: 0}},
												SpcAft: &parts.DMLSpacing{SpcPts: &parts.SpacingPts{Val: 0}},
												LnSpc:  &parts.DMLSpacing{SpcPct: &parts.SpacingPct{Val: 100000}},
												DefRPr: &parts.DMLRunPr{FontSize: 3600},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	shape := pres.Slides[0].Shapes[0]
	if shape.TextAnchor != "ctr" {
		t.Fatalf("text anchor = %q, want ctr", shape.TextAnchor)
	}
	if shape.TextInsetLeftEMU != 91425 || shape.TextInsetBottomEMU != 91425 {
		t.Fatalf("unexpected text insets: left=%d bottom=%d", shape.TextInsetLeftEMU, shape.TextInsetBottomEMU)
	}
	p := shape.Paragraphs[0]
	if !p.HasSpaceBefore || p.SpaceBefore != 0 || !p.HasSpaceAfter || p.SpaceAfter != 0 {
		t.Fatalf("unexpected paragraph spacing: before=%d/%v after=%d/%v", p.SpaceBefore, p.HasSpaceBefore, p.SpaceAfter, p.HasSpaceAfter)
	}
	if !p.HasLineSpacing || p.LineSpacingPct != 100000 {
		t.Fatalf("unexpected line spacing: %d/%v", p.LineSpacingPct, p.HasLineSpacing)
	}
}

func TestResolvePackageMapsSectionsToSlides(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
			SlideIDs: []parts.IDRef{
				{ID: 256, RelID: "rId1"},
				{ID: 257, RelID: "rId2"},
				{ID: 258, RelID: "rId3"},
			},
			Sections: []parts.Section{
				{Name: "Intro", SlideIDs: []parts.SectionSlideID{{ID: 256}, {ID: 257}}},
				{Name: "Conclusion", SlideIDs: []parts.SectionSlideID{{ID: 258}}},
			},
		},
		Slides: []*parts.SlideEntry{
			{Slide: &parts.Slide{}},
			{Slide: &parts.Slide{}},
			{Slide: &parts.Slide{}},
		},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(pres.Sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(pres.Sections))
	}
	if pres.Sections[0] != "Intro" || pres.Sections[1] != "Conclusion" {
		t.Fatalf("unexpected sections: %v", pres.Sections)
	}
	if pres.Slides[0].SectionName != "Intro" {
		t.Fatalf("slide[0].SectionName = %q, want Intro", pres.Slides[0].SectionName)
	}
	if pres.Slides[1].SectionName != "Intro" {
		t.Fatalf("slide[1].SectionName = %q, want Intro", pres.Slides[1].SectionName)
	}
	if pres.Slides[2].SectionName != "Conclusion" {
		t.Fatalf("slide[2].SectionName = %q, want Conclusion", pres.Slides[2].SectionName)
	}
}

func TestResolvePackageConvertsBreaksToNewlineRuns(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		Slides: []*parts.SlideEntry{{
			Slide: &parts.Slide{
				CSld: parts.CommonSlideData{
					SpTree: parts.ShapeTree{Shapes: []parts.Shape{{
						NvSpPr: parts.NonVisualShapeProps{CNvPr: parts.CnvPr{ID: 1, Name: "Body"}},
						TxBody: &parts.TextBody{
							Paragraphs: []parts.DMLParagraph{{
								Runs: []parts.DMLRun{{Text: "Line1"}},
								Breaks: []parts.DMLBreak{{
									Properties: &parts.DMLRunPr{FontSize: 2400, Bold: boolPtr(true)},
								}},
								Inlines: []parts.DMLParaInline{
									{Kind: parts.DMLParaInlineRun, Run: &parts.DMLRun{Text: "Line1"}},
									{Kind: parts.DMLParaInlineBreak, Break: &parts.DMLBreak{Properties: &parts.DMLRunPr{FontSize: 2400, Bold: boolPtr(true)}}},
								},
							}},
						},
					}}},
				},
			},
		}},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(pres.Slides) != 1 || len(pres.Slides[0].Shapes) != 1 {
		t.Fatalf("expected 1 slide with 1 shape")
	}
	paragraphs := pres.Slides[0].Shapes[0].Paragraphs
	if len(paragraphs) != 1 {
		t.Fatalf("expected 1 paragraph, got %d", len(paragraphs))
	}
	runs := paragraphs[0].Runs
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(runs))
	}
	if runs[0].Text != "Line1" {
		t.Fatalf("run[0].Text = %q, want Line1", runs[0].Text)
	}
	if runs[1].Text != "\n" {
		t.Fatalf("run[1].Text = %q, want newline", runs[1].Text)
	}
	if !runs[1].Bold {
		t.Fatalf("run[1].Bold = %v, want true", runs[1].Bold)
	}
	if runs[1].FontSize != 2400 {
		t.Fatalf("run[1].FontSize = %d, want 2400", runs[1].FontSize)
	}
}

func TestResolvePackagePreservesNoAutofitOverflow(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		Slides: []*parts.SlideEntry{{
			Slide: &parts.Slide{
				CSld: parts.CommonSlideData{
					SpTree: parts.ShapeTree{Shapes: []parts.Shape{{
						NvSpPr: parts.NonVisualShapeProps{
							CNvPr: parts.CnvPr{ID: 1, Name: "Body"},
						},
						SpPr: parts.ShapeProperties{
							Xfrm: &parts.Transform2D{
								Off: parts.Offset{X: 1000, Y: 2000},
								Ext: parts.Extents{CX: 3000, CY: 4000},
							},
						},
						TxBody: &parts.TextBody{
							BodyPr: &parts.DMLBodyPr{
								Anchor:    "b",
								NoAutofit: &struct{}{},
							},
							Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{{Text: "Project name"}}}},
						},
					}}},
				},
			},
		}},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	shape := pres.Slides[0].Shapes[0]
	if !shape.TextAllowOverflow {
		t.Fatalf("TextAllowOverflow = false, want true")
	}
	if shape.TextAnchor != "b" {
		t.Fatalf("TextAnchor = %q, want %q", shape.TextAnchor, "b")
	}
}

func TestResolvePackagePreservesInheritedNormAutofit(t *testing.T) {
	fontScale := 92000
	lineReduction := 18000
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		Slides: []*parts.SlideEntry{{
			Slide: &parts.Slide{
				CSld: parts.CommonSlideData{
					SpTree: parts.ShapeTree{Shapes: []parts.Shape{{
						NvSpPr: parts.NonVisualShapeProps{
							CNvPr: parts.CnvPr{ID: 1, Name: "Body"},
							NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "body", Idx: 1}},
						},
						TxBody: &parts.TextBody{
							Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{{Text: "Inherited autofit"}}}},
						},
					}}},
				},
			},
			Layout: &parts.SlideLayout{
				CSld: parts.CommonSlideData{
					SpTree: parts.ShapeTree{Shapes: []parts.Shape{{
						NvSpPr: parts.NonVisualShapeProps{
							CNvPr: parts.CnvPr{ID: 2, Name: "Layout Body"},
							NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "body", Idx: 1}},
						},
						TxBody: &parts.TextBody{
							BodyPr: &parts.DMLBodyPr{
								Anchor: "ctr",
								NormAutofit: &parts.DMLNormAutofit{
									FontScale:      &fontScale,
									LnSpcReduction: &lineReduction,
								},
							},
						},
					}}},
				},
			},
		}},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	shape := pres.Slides[0].Shapes[0]
	if shape.TextAllowOverflow {
		t.Fatalf("TextAllowOverflow = true, want false")
	}
	if shape.TextAnchor != "ctr" {
		t.Fatalf("TextAnchor = %q, want %q", shape.TextAnchor, "ctr")
	}
	if shape.TextBodyProps.AutofitType != "normAutofit" {
		t.Fatalf("AutofitType = %q, want %q", shape.TextBodyProps.AutofitType, "normAutofit")
	}
	if shape.TextBodyProps.AutofitFontScale != fontScale {
		t.Fatalf("AutofitFontScale = %d, want %d", shape.TextBodyProps.AutofitFontScale, fontScale)
	}
	if shape.TextBodyProps.AutofitLineSpaceReduction != lineReduction {
		t.Fatalf("AutofitLineSpaceReduction = %d, want %d", shape.TextBodyProps.AutofitLineSpaceReduction, lineReduction)
	}
}

func TestResolvePackageSkipsSentinelPlaceholderInheritance(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		Slides: []*parts.SlideEntry{{
			Slide: &parts.Slide{
				CSld: parts.CommonSlideData{
					SpTree: parts.ShapeTree{Shapes: []parts.Shape{{
						NvSpPr: parts.NonVisualShapeProps{
							CNvPr: parts.CnvPr{ID: 1, Name: "Textbox"},
							NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "body", Idx: int(^uint32(0))}},
						},
						TxBody: &parts.TextBody{
							Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{{Text: "Text box body"}}}},
						},
					}}},
				},
			},
			Layout: &parts.SlideLayout{
				CSld: parts.CommonSlideData{
					SpTree: parts.ShapeTree{Shapes: []parts.Shape{{
						NvSpPr: parts.NonVisualShapeProps{
							CNvPr: parts.CnvPr{ID: 2, Name: "Layout Body"},
							NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "body", Idx: 1}},
						},
						TxBody: &parts.TextBody{
							ListStyle: &parts.DMLTextListStyle{
								Lvl1PPr: &parts.DMLTextStyleLevel{
									DefRPr: &parts.DMLRunPr{Latin: &parts.FontRef{Typeface: "Source Code Pro"}},
								},
							},
						},
					}}},
				},
			},
		}},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	run := pres.Slides[0].Shapes[0].Paragraphs[0].Runs[0]
	if run.FontFamily == "Source Code Pro" {
		t.Fatalf("font family = %q, want sentinel placeholder to skip inherited body font", run.FontFamily)
	}
}

func TestResolvePackageInheritsAndOverridesDefaultRunStyles(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		Slides: []*parts.SlideEntry{{
			Slide: &parts.Slide{CSld: parts.CommonSlideData{SpTree: parts.ShapeTree{Shapes: []parts.Shape{{
				NvSpPr: parts.NonVisualShapeProps{CNvPr: parts.CnvPr{ID: 1, Name: "Body"}},
				TxBody: &parts.TextBody{
					ListStyle: &parts.DMLTextListStyle{Lvl1PPr: &parts.DMLTextStyleLevel{
						DefRPr: &parts.DMLRunPr{Bold: boolPtr(true), Italic: boolPtr(true), Underline: "sng", Strike: "sngStrike", Cap: "small", Baseline: intPtr(30000), FontSize: 2800},
					}},
					Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{
						{Text: "Inherited"},
						{Text: "Override", Properties: &parts.DMLRunPr{Bold: boolPtr(false), Italic: boolPtr(false), Underline: "none", Strike: "noStrike", Cap: "all", Baseline: intPtr(-25000)}},
					}}},
				},
			}}}}},
		}},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	runs := pres.Slides[0].Shapes[0].Paragraphs[0].Runs
	if len(runs) != 2 {
		t.Fatalf("run count = %d, want 2", len(runs))
	}
	if !runs[0].Bold || !runs[0].Italic || !runs[0].Underline || !runs[0].Strike {
		t.Fatalf("run[0] missing inherited emphasis: %+v", runs[0])
	}
	if !runs[0].SmallCaps || !runs[0].Superscript || runs[0].Subscript {
		t.Fatalf("run[0] missing inherited caps/baseline styling: %+v", runs[0])
	}
	if runs[1].Bold || runs[1].Italic || runs[1].Underline || runs[1].Strike {
		t.Fatalf("run[1] explicit overrides were ignored: %+v", runs[1])
	}
	if !runs[1].AllCaps || runs[1].SmallCaps || runs[1].Superscript || !runs[1].Subscript {
		t.Fatalf("run[1] caps/baseline overrides were ignored: %+v", runs[1])
	}
}

func TestResolvePackageParagraphIndentOverridesLayoutDefaults(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		Slides: []*parts.SlideEntry{
			{
				Slide: &parts.Slide{
					CSld: parts.CommonSlideData{
						SpTree: parts.ShapeTree{
							Shapes: []parts.Shape{
								{
									NvSpPr: parts.NonVisualShapeProps{
										CNvPr: parts.CnvPr{ID: 1, Name: "Body"},
										NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "body", Idx: 1}},
									},
									TxBody: &parts.TextBody{
										Paragraphs: []parts.DMLParagraph{
											{
												Properties: &parts.DMLParaProps{MarL: intPtr(0), Indent: intPtr(0)},
												Runs:       []parts.DMLRun{{Text: "Paragraph"}},
											},
										},
									},
								},
							},
						},
					},
				},
				Layout: &parts.SlideLayout{
					CSld: parts.CommonSlideData{
						SpTree: parts.ShapeTree{
							Shapes: []parts.Shape{
								{
									NvSpPr: parts.NonVisualShapeProps{
										CNvPr: parts.CnvPr{ID: 2, Name: "Layout Body"},
										NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "body", Idx: 1}},
									},
									TxBody: &parts.TextBody{
										ListStyle: &parts.DMLTextListStyle{
											Lvl1PPr: &parts.DMLTextStyleLevel{MarL: intPtr(457200), Indent: intPtr(-228600)},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	p := pres.Slides[0].Shapes[0].Paragraphs[0]
	if !p.HasMarginLeft || !p.HasTextIndent {
		t.Fatalf("expected resolved paragraph indent flags: %+v", p)
	}
	if p.MarginLeftEMU != 0 || p.TextIndentEMU != 0 {
		t.Fatalf("expected explicit zero paragraph indent to override layout defaults, got margin=%d indent=%d", p.MarginLeftEMU, p.TextIndentEMU)
	}
}
func int64Ptr(v int64) *int64 { return &v }

func intPtr(v int) *int { return &v }

func boolPtr(v bool) *bool { return &v }

func TestResolvePackagePrefersLayoutBuNoneOverMasterListDefaults(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		Slides: []*parts.SlideEntry{
			{
				Slide: &parts.Slide{
					CSld: parts.CommonSlideData{
						SpTree: parts.ShapeTree{
							Shapes: []parts.Shape{
								{
									NvSpPr: parts.NonVisualShapeProps{
										CNvPr: parts.CnvPr{ID: 1, Name: "Body"},
										NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "body", Idx: 1}},
									},
									TxBody: &parts.TextBody{
										Paragraphs: []parts.DMLParagraph{
											{
												Runs: []parts.DMLRun{
													{Text: "Plain paragraph"},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				Layout: &parts.SlideLayout{
					CSld: parts.CommonSlideData{
						SpTree: parts.ShapeTree{
							Shapes: []parts.Shape{
								{
									NvSpPr: parts.NonVisualShapeProps{
										CNvPr: parts.CnvPr{ID: 2, Name: "Layout Body"},
										NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "body", Idx: 1}},
									},
									TxBody: &parts.TextBody{
										ListStyle: &parts.DMLTextListStyle{
											Lvl1PPr: &parts.DMLTextStyleLevel{
												BuNone: &struct{}{},
												DefRPr: &parts.DMLRunPr{FontSize: 2400},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		SlideMasters: []*parts.SlideMasterEntry{
			{
				Master: &parts.SlideMaster{
					TxStyles: &parts.MasterTxStyles{
						BodyStyle: &parts.DMLTextListStyle{
							Lvl1PPr: &parts.DMLTextStyleLevel{
								BuChar: &parts.BuChar{Char: "•"},
								DefRPr: &parts.DMLRunPr{FontSize: 2400},
							},
						},
					},
				},
			},
		},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	p := pres.Slides[0].Shapes[0].Paragraphs[0]
	if p.BulletChar != "" || p.AutoNum {
		t.Fatalf("expected no inherited list marker, got bullet=%q autonum=%v", p.BulletChar, p.AutoNum)
	}
}

func TestResolvePackageKeepsConnectorGeometry(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		Slides: []*parts.SlideEntry{
			{
				Slide: &parts.Slide{
					CSld: parts.CommonSlideData{
						SpTree: parts.ShapeTree{
							Shapes: []parts.Shape{
								{
									NvSpPr: parts.NonVisualShapeProps{
										CNvPr: parts.CnvPr{ID: 1, Name: "Source"},
									},
									SpPr: parts.ShapeProperties{
										Xfrm: &parts.Transform2D{
											Off: parts.Offset{X: 100, Y: 200},
											Ext: parts.Extents{CX: 300, CY: 400},
										},
									},
									TxBody: &parts.TextBody{
										Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{{Text: "A"}}}},
									},
								},
								{
									NvSpPr: parts.NonVisualShapeProps{
										CNvPr: parts.CnvPr{ID: 2, Name: "Target"},
									},
									SpPr: parts.ShapeProperties{
										Xfrm: &parts.Transform2D{
											Off: parts.Offset{X: 500, Y: 700},
											Ext: parts.Extents{CX: 300, CY: 400},
										},
									},
									TxBody: &parts.TextBody{
										Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{{Text: "B"}}}},
									},
								},
							},
							Connectors: []parts.Connector{
								{
									NvCxnSpPr: parts.NonVisualCxnSpProps{
										CNvPr: parts.CnvPr{ID: 7, Name: "Connector"},
										CNvCxnSpPr: &parts.ConnectorConnectionProps{
											StCxn:  &parts.ConnectionSiteRef{ID: 1, Idx: 2},
											EndCxn: &parts.ConnectionSiteRef{ID: 2, Idx: 0},
										},
									},
									SpPr: parts.ConnectorProperties{
										Xfrm: &parts.Transform2D{
											Off:   parts.Offset{X: 100, Y: 200},
											Ext:   parts.Extents{CX: 300, CY: 400},
											FlipH: true,
										},
										PrstGeom: &parts.PresetGeometry{
											Prst: "bentConnector3",
											AvLst: []parts.GeometryGuide{
												{Name: "adj1", Fmla: "val 100000"},
											},
										},
										Line: &parts.ConnectorLine{
											W:         28575,
											SolidFill: &drawingxml.SolidFill{SRGBClr: &drawingxml.SRGBClr{Val: "595959"}},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(pres.Slides) != 1 || len(pres.Slides[0].Shapes) != 3 {
		t.Fatalf("unexpected shape count: %#v", pres.Slides)
	}
	shape := pres.Slides[0].Shapes[2]
	if shape.ConnectorType != "bentConnector3" {
		t.Fatalf("connector type = %q, want bentConnector3", shape.ConnectorType)
	}
	if !shape.ConnectorFlipH || shape.ConnectorFlipV {
		t.Fatalf("unexpected connector flips: h=%v v=%v", shape.ConnectorFlipH, shape.ConnectorFlipV)
	}
	if len(shape.ConnectorAdjustments) < 1 || shape.ConnectorAdjustments[0] != 100000 {
		t.Fatalf("connector adjustments = %v, want [100000]", shape.ConnectorAdjustments)
	}
	if shape.ConnectorStart == nil || shape.ConnectorStart.X != 250 || shape.ConnectorStart.Y != 600 {
		t.Fatalf("unexpected connector start: %#v", shape.ConnectorStart)
	}
	if shape.ConnectorEnd == nil || shape.ConnectorEnd.X != 650 || shape.ConnectorEnd.Y != 700 {
		t.Fatalf("unexpected connector end: %#v", shape.ConnectorEnd)
	}
}

func TestResolvePackagePreservesSlideShapeOrder(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		Slides: []*parts.SlideEntry{
			{
				Slide: &parts.Slide{
					CSld: parts.CommonSlideData{
						SpTree: parts.ShapeTree{
							Shapes: []parts.Shape{
								{
									NvSpPr: parts.NonVisualShapeProps{CNvPr: parts.CnvPr{ID: 1, Name: "Circle"}},
									SpPr: parts.ShapeProperties{
										Xfrm:     &parts.Transform2D{Off: parts.Offset{X: 100, Y: 100}, Ext: parts.Extents{CX: 200, CY: 200}},
										PrstGeom: &parts.PresetGeometry{Prst: "ellipse"},
									},
									TxBody: &parts.TextBody{Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{{Text: ""}}}}},
								},
							},
							Connectors: []parts.Connector{
								{
									NvCxnSpPr: parts.NonVisualCxnSpProps{CNvPr: parts.CnvPr{ID: 2, Name: "Line"}},
									SpPr: parts.ConnectorProperties{
										Xfrm:     &parts.Transform2D{Off: parts.Offset{X: 0, Y: 200}, Ext: parts.Extents{CX: 400, CY: 0}},
										PrstGeom: &parts.PresetGeometry{Prst: "straightConnector1"},
										Line:     &parts.ConnectorLine{SolidFill: &drawingxml.SolidFill{SRGBClr: &drawingxml.SRGBClr{Val: "000000"}}},
									},
								},
							},
							Items: []parts.ShapeTreeItem{
								{Kind: parts.ShapeTreeItemConnector, Index: 0},
								{Kind: parts.ShapeTreeItemShape, Index: 0},
							},
						},
					},
				},
			},
		},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(pres.Slides[0].Shapes) != 2 {
		t.Fatalf("shape count = %d, want 2", len(pres.Slides[0].Shapes))
	}
	if pres.Slides[0].Shapes[0].Kind != resolve.ShapeConnector || pres.Slides[0].Shapes[1].Kind != resolve.ShapeText {
		t.Fatalf("unexpected shape order: %#v", pres.Slides[0].Shapes)
	}
}

func TestResolvePackageKeepsShapeRotation(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		Slides: []*parts.SlideEntry{
			{
				Slide: &parts.Slide{
					CSld: parts.CommonSlideData{
						SpTree: parts.ShapeTree{
							Shapes: []parts.Shape{
								{
									NvSpPr: parts.NonVisualShapeProps{
										CNvPr: parts.CnvPr{ID: 1, Name: "Triangle"},
									},
									SpPr: parts.ShapeProperties{
										Xfrm: &parts.Transform2D{
											Off: parts.Offset{X: 100, Y: 200},
											Ext: parts.Extents{CX: 300, CY: 400},
											Rot: 10800000,
										},
										PrstGeom: &parts.PresetGeometry{Prst: "triangle"},
										SolidFill: &parts.ColorRef{
											Val: "E91D63",
										},
									},
									TxBody: &parts.TextBody{
										Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{{Text: ""}}}},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	shape := pres.Slides[0].Shapes[0]
	if shape.Geometry != "triangle" {
		t.Fatalf("geometry = %q, want triangle", shape.Geometry)
	}
	if shape.RotationDeg != 180 {
		t.Fatalf("rotation = %.2f, want 180", shape.RotationDeg)
	}
	if shape.Fill == nil {
		t.Fatal("Fill = nil, want shared fill preserved")
	}
}

func TestResolvePackagePreservesCustomGeometry(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000}},
		Slides: []*parts.SlideEntry{{
			Slide: &parts.Slide{CSld: parts.CommonSlideData{SpTree: parts.ShapeTree{Shapes: []parts.Shape{{
				NvSpPr: parts.NonVisualShapeProps{CNvPr: parts.CnvPr{ID: 1, Name: "Custom"}},
				SpPr: parts.ShapeProperties{
					Xfrm: &parts.Transform2D{Off: parts.Offset{X: 100, Y: 200}, Ext: parts.Extents{CX: 300, CY: 400}},
					CustGeom: &parts.CustomGeometry{Paths: []parts.CustomPath{{
						W: 100,
						H: 100,
						Commands: []parts.CustomPathCommand{
							{Type: "M", Points: []parts.FormulaPoint{{X: "0", Y: "100"}}},
							{Type: "L", Points: []parts.FormulaPoint{{X: "50", Y: "0"}}},
							{Type: "L", Points: []parts.FormulaPoint{{X: "100", Y: "100"}}},
							{Type: "Z"},
						},
					}}},
				},
				TxBody: &parts.TextBody{Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{{Text: ""}}}}},
			}}}}},
		}},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	shape := pres.Slides[0].Shapes[0]
	if shape.GeometryData == nil {
		t.Fatal("GeometryData = nil, want custom geometry preserved")
	}
	if shape.GeometryData.Type != drawing.GeometryTypeCustom {
		t.Fatalf("geometry type = %q, want custom", shape.GeometryData.Type)
	}
	if len(shape.GeometryData.Paths) != 1 {
		t.Fatalf("path count = %d, want 1", len(shape.GeometryData.Paths))
	}
}

func TestResolvePackageMaterializesPictureFillOnAutoShape(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000}},
		Slides: []*parts.SlideEntry{
			{
				MediaParts: map[string]media.Part{
					"rId7": {ContentType: "image/png", Data: []byte{0x89, 0x50, 0x4E, 0x47}},
				},
				Slide: &parts.Slide{CSld: parts.CommonSlideData{SpTree: parts.ShapeTree{
					Shapes: []parts.Shape{
						{
							NvSpPr: parts.NonVisualShapeProps{CNvPr: parts.CnvPr{ID: 1, Name: "Filled"}},
							SpPr: parts.ShapeProperties{
								Xfrm:     &parts.Transform2D{Off: parts.Offset{X: 100, Y: 200}, Ext: parts.Extents{CX: 300, CY: 400}},
								BlipFill: &parts.BlipFill{Blip: &parts.Blip{Embed: "rId7"}, SrcRect: &parts.SrcRect{L: 10000, R: 20000}},
							},
						},
					},
				}}},
			},
		},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	fill, ok := pres.Slides[0].Shapes[0].Fill.(drawing.PictureFill)
	if !ok {
		t.Fatalf("fill type = %T, want drawing.PictureFill", pres.Slides[0].Shapes[0].Fill)
	}
	if fill.EmbedID != "rId7" || fill.ContentType != "image/png" || len(fill.ImageData) != 4 {
		t.Fatalf("unexpected picture fill materialization = %#v", fill)
	}
	if fill.Crop == nil || fill.Crop.X != 0.1 || fill.Crop.Width != 0.7 {
		t.Fatalf("unexpected picture fill crop = %#v", fill.Crop)
	}
}

func TestResolvePackagePreservesSharedLineStyle(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		Slides: []*parts.SlideEntry{
			{
				Slide: &parts.Slide{
					CSld: parts.CommonSlideData{
						SpTree: parts.ShapeTree{
							Connectors: []parts.Connector{
								{
									NvCxnSpPr: parts.NonVisualCxnSpProps{
										CNvPr: parts.CnvPr{ID: 7, Name: "Connector"},
									},
									SpPr: parts.ConnectorProperties{
										Xfrm:     &parts.Transform2D{Off: parts.Offset{X: 100, Y: 200}, Ext: parts.Extents{CX: 300, CY: 0}},
										PrstGeom: &parts.PresetGeometry{Prst: "straightConnector1"},
										Line: &parts.ConnectorLine{
											W:        28575,
											Cap:      "round",
											PrstDash: &parts.PresetDash{Val: "dashDot"},
											Bevel:    &struct{}{},
											SolidFill: &drawingxml.SolidFill{
												SRGBClr: &drawingxml.SRGBClr{Val: "595959"},
											},
										},
									},
								},
							},
							Items: []parts.ShapeTreeItem{
								{Kind: parts.ShapeTreeItemConnector, Index: 0},
							},
						},
					},
				},
			},
		},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	shape := pres.Slides[0].Shapes[0]
	if shape.LineStyle == nil {
		t.Fatal("LineStyle = nil, want shared line style")
	}
	if shape.LineStyle.Cap != drawing.LineCapRound || shape.LineStyle.Join != drawing.LineJoinBevel || shape.LineStyle.Dash != drawing.DashPatternDashDot {
		t.Fatalf("unexpected line style = %#v", shape.LineStyle)
	}
}

func TestResolvePackagePreservesSharedEffects(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		Slides: []*parts.SlideEntry{
			{
				Slide: &parts.Slide{
					CSld: parts.CommonSlideData{
						SpTree: parts.ShapeTree{
							Shapes: []parts.Shape{
								{
									NvSpPr: parts.NonVisualShapeProps{
										CNvPr: parts.CnvPr{ID: 1, Name: "Glow"},
									},
									SpPr: parts.ShapeProperties{
										Xfrm: &parts.Transform2D{
											Off: parts.Offset{X: 100, Y: 200},
											Ext: parts.Extents{CX: 300, CY: 400},
										},
										PrstGeom: &parts.PresetGeometry{Prst: "ellipse"},
										Glow: &parts.GlowEffect{
											Rad:       50800,
											SchemeClr: &parts.EffectSchemeColor{Val: "accent2", Alpha: &parts.EffectAlpha{Val: 60000}},
										},
									},
									TxBody: &parts.TextBody{
										Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{{Text: ""}}}},
									},
								},
							},
						},
					},
				},
			},
		},
		Themes: map[string]*parts.Theme{
			"ppt/theme/theme1.xml": {
				Elements: parts.ThemeElements{
					ClrScheme: parts.ColorScheme{
						Accent2: &parts.ThemeColor{SrgbClr: &parts.SrgbColor{Val: "673AB7"}},
					},
				},
			},
		},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	shape := pres.Slides[0].Shapes[0]
	if len(shape.Effects.Effects) != 1 {
		t.Fatalf("effect count = %d, want 1", len(shape.Effects.Effects))
	}
	glow, ok := shape.Effects.Effects[0].(drawing.Glow)
	if !ok {
		t.Fatalf("effect type = %T, want drawing.Glow", shape.Effects.Effects[0])
	}
	if glow.Color != "673AB7" || glow.Alpha != 0.6 || glow.Radius != 4.0 {
		t.Fatalf("unexpected glow = %#v", glow)
	}
}

func TestPptxCorpusResolve(t *testing.T) {
	fixturesDir := repoPath(t, "testdata", "pptx")
	entries, err := os.ReadDir(fixturesDir)
	if err != nil {
		t.Fatalf("read fixtures dir: %v", err)
	}

	var count int
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
			if pres == nil {
				t.Fatal("presentation is nil")
			}
			if len(pres.Slides) == 0 {
				t.Fatal("no slides resolved")
			}
		})
	}

	if count == 0 {
		t.Fatal("no pptx fixtures found")
	}
}

func repoPath(t *testing.T, elems ...string) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", ".."))
	return filepath.Join(append([]string{root}, elems...)...)
}

func isPptxFixtureName(name string) bool {
	return strings.EqualFold(filepath.Ext(name), ".pptx") && !strings.HasPrefix(name, "~$")
}

func TestResolvePackageAppliesGroupTransformToChildShapes(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		Slides: []*parts.SlideEntry{
			{
				Slide: &parts.Slide{
					CSld: parts.CommonSlideData{
						SpTree: parts.ShapeTree{
							GroupShapes: []parts.GroupShape{
								{
									NvGrpSpPr: &parts.NonVisualGroupShapeProps{
										CNvPr: parts.CnvPr{ID: 100, Name: "Group 1"},
									},
									GrpSpPr: &parts.GroupShapeProperties{
										Xfrm: &parts.Transform2D{
											Off: parts.Offset{X: 1000, Y: 2000},
											Ext: parts.Extents{CX: 5000, CY: 3000},
										},
									},
									Shapes: []parts.Shape{
										{
											NvSpPr: parts.NonVisualShapeProps{
												CNvPr: parts.CnvPr{ID: 1, Name: "Child Shape"},
											},
											SpPr: parts.ShapeProperties{
												Xfrm: &parts.Transform2D{
													Off: parts.Offset{X: 500, Y: 500},
													Ext: parts.Extents{CX: 1000, CY: 1000},
												},
												PrstGeom: &parts.PresetGeometry{Prst: "rect"},
											},
											TxBody: &parts.TextBody{
												Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{{Text: "Child"}}}},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(pres.Slides) != 1 || len(pres.Slides[0].Shapes) != 1 {
		t.Fatalf("unexpected shape count: slides=%d shapes=%d", len(pres.Slides), len(pres.Slides[0].Shapes))
	}

	groupShape := pres.Slides[0].Shapes[0]
	if groupShape.Kind != resolve.ShapeGroup {
		t.Fatalf("expected group shape, got kind: %v", groupShape.Kind)
	}
	if len(groupShape.Children) != 1 {
		t.Fatalf("expected 1 child shape, got: %d", len(groupShape.Children))
	}

	childShape := groupShape.Children[0]
	// Child shape should have its bounds transformed by group offset
	// Original child bounds: X=500, Y=500, W=1000, H=1000
	// Group offset: X=1000, Y=2000
	// Expected transformed bounds: X=1500, Y=2500, W=1000, H=1000
	if childShape.Bounds.X != 1500 {
		t.Errorf("child bounds X = %d, want 1500", childShape.Bounds.X)
	}
	if childShape.Bounds.Y != 2500 {
		t.Errorf("child bounds Y = %d, want 2500", childShape.Bounds.Y)
	}
	if childShape.Bounds.W != 1000 {
		t.Errorf("child bounds W = %d, want 1000", childShape.Bounds.W)
	}
	if childShape.Bounds.H != 1000 {
		t.Errorf("child bounds H = %d, want 1000", childShape.Bounds.H)
	}
}

func TestResolveFieldSlideNumKind(t *testing.T) {
	// A shape with an <a:fld type="slidenum"> field in its paragraph should
	// produce a run with FieldKind == "slidenum".
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		Slides: []*parts.SlideEntry{{
			Slide: &parts.Slide{
				CSld: parts.CommonSlideData{
					SpTree: parts.ShapeTree{
						Shapes: []parts.Shape{{
							NvSpPr: parts.NonVisualShapeProps{
								CNvPr: parts.CnvPr{ID: 1, Name: "field-shape"},
							},
							SpPr: parts.ShapeProperties{
								Xfrm: &parts.Transform2D{
									Off: parts.Offset{X: 0, Y: 0},
									Ext: parts.Extents{CX: 914400, CY: 685800},
								},
							},
							TxBody: &parts.TextBody{
								Paragraphs: []parts.DMLParagraph{{
									Fields: []parts.DMLField{{
										Type: "slidenum",
										Text: "<slide#>",
									}},
								}},
							},
						}},
					},
				},
			},
		}},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}
	if len(pres.Slides) == 0 {
		t.Fatal("expected at least one slide")
	}
	slide := pres.Slides[0]
	if len(slide.Shapes) == 0 {
		t.Fatal("expected at least one shape")
	}
	shape := slide.Shapes[0]
	if len(shape.Paragraphs) == 0 {
		t.Fatal("expected at least one paragraph")
	}
	para := shape.Paragraphs[0]
	if len(para.Runs) == 0 {
		t.Fatal("expected at least one run from field")
	}
	run := para.Runs[0]
	if run.FieldKind != "slidenum" {
		t.Errorf("FieldKind = %q, want %q", run.FieldKind, "slidenum")
	}
}

func TestResolvePackageInheritsLayoutSlideNumberPlaceholder(t *testing.T) {
	layout := &parts.SlideLayout{
		CSld: parts.CommonSlideData{
			SpTree: parts.ShapeTree{
				Shapes: []parts.Shape{{
					NvSpPr: parts.NonVisualShapeProps{
						CNvPr: parts.CnvPr{ID: 7, Name: "Slide Number"},
						NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "sldNum", Idx: 9}},
					},
					SpPr: parts.ShapeProperties{
						Xfrm: &parts.Transform2D{
							Off: parts.Offset{X: 1000, Y: 2000},
							Ext: parts.Extents{CX: 3000, CY: 4000},
						},
					},
					TxBody: &parts.TextBody{
						Paragraphs: []parts.DMLParagraph{{}},
					},
				}},
			},
		},
	}
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		Slides: []*parts.SlideEntry{{
			Slide:  &parts.Slide{CSld: parts.CommonSlideData{SpTree: parts.ShapeTree{}}},
			Layout: layout,
		}},
		SlideMasters: []*parts.SlideMasterEntry{{
			Master: &parts.SlideMaster{
				HF: &parts.HeaderFooter{SlideNumber: boolPtr(true)},
			},
			Layouts: []*parts.SlideLayout{layout},
		}},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}
	slide := pres.Slides[0]
	if slide.SlideNumber == nil {
		t.Fatal("expected inherited slide-number placeholder")
	}
	if slide.SlideNumber.Bounds.X != 1000 || slide.SlideNumber.Bounds.Y != 2000 {
		t.Fatalf("unexpected inherited bounds: %+v", slide.SlideNumber.Bounds)
	}
	if len(slide.SlideNumber.Paragraphs) != 1 || len(slide.SlideNumber.Paragraphs[0].Runs) != 1 {
		t.Fatalf("expected synthesized slide-number paragraph/run, got %+v", slide.SlideNumber.Paragraphs)
	}
	if got := slide.SlideNumber.Paragraphs[0].Runs[0].FieldKind; got != "slidenum" {
		t.Fatalf("FieldKind = %q, want %q", got, "slidenum")
	}
}

func TestResolvePackageSkipsDisabledMasterSlideNumberPlaceholder(t *testing.T) {
	master := &parts.SlideMaster{
		HF: &parts.HeaderFooter{SlideNumber: boolPtr(false)},
		CSld: parts.CommonSlideData{
			SpTree: parts.ShapeTree{
				Shapes: []parts.Shape{{
					NvSpPr: parts.NonVisualShapeProps{
						CNvPr: parts.CnvPr{ID: 8, Name: "Master Slide Number"},
						NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "sldNum", Idx: 9}},
					},
					SpPr: parts.ShapeProperties{
						Xfrm: &parts.Transform2D{
							Off: parts.Offset{X: 1000, Y: 2000},
							Ext: parts.Extents{CX: 3000, CY: 4000},
						},
					},
					TxBody: &parts.TextBody{
						Paragraphs: []parts.DMLParagraph{{}},
					},
				}},
			},
		},
	}
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		Slides: []*parts.SlideEntry{{
			Slide: &parts.Slide{CSld: parts.CommonSlideData{SpTree: parts.ShapeTree{}}},
		}},
		SlideMasters: []*parts.SlideMasterEntry{{
			Master: master,
		}},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}
	if slide := pres.Slides[0]; slide.SlideNumber != nil {
		t.Fatalf("expected disabled master slide-number placeholder to be skipped, got %+v", slide.SlideNumber)
	}
}

func TestResolveFieldUnknownTypePreservesText(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		Slides: []*parts.SlideEntry{{
			Slide: &parts.Slide{
				CSld: parts.CommonSlideData{
					SpTree: parts.ShapeTree{
						Shapes: []parts.Shape{{
							NvSpPr: parts.NonVisualShapeProps{
								CNvPr: parts.CnvPr{ID: 1, Name: "field-shape"},
							},
							SpPr: parts.ShapeProperties{
								Xfrm: &parts.Transform2D{
									Off: parts.Offset{X: 0, Y: 0},
									Ext: parts.Extents{CX: 914400, CY: 685800},
								},
							},
							TxBody: &parts.TextBody{
								Paragraphs: []parts.DMLParagraph{{
									Fields: []parts.DMLField{{
										Type: "datetime1",
										Text: "1/1/2024",
									}},
								}},
							},
						}},
					},
				},
			},
		}},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}
	run := pres.Slides[0].Shapes[0].Paragraphs[0].Runs[0]
	if run.FieldKind != "datetime1" {
		t.Errorf("FieldKind = %q, want %q", run.FieldKind, "datetime1")
	}
	if run.Text != "1/1/2024" {
		t.Errorf("Text = %q, want %q", run.Text, "1/1/2024")
	}
}

func TestResolvePackageUsesPerSlideMasterFromLayout(t *testing.T) {
	// POI XSLF parity: each slide should use the master linked by its layout,
	// not a presentation-wide default master.
	masterA := &parts.SlideMaster{
		CSld: parts.CommonSlideData{
			Bg: &parts.SlideBg{BgPrSrgb: &drawingxml.SRGBClr{Val: "FF0000"}},
		},
	}
	masterB := &parts.SlideMaster{
		CSld: parts.CommonSlideData{
			Bg: &parts.SlideBg{BgPrSrgb: &drawingxml.SRGBClr{Val: "00FF00"}},
		},
	}
	layoutA := &parts.SlideLayout{PartName: "ppt/slideLayouts/slideLayout1.xml"}
	layoutB := &parts.SlideLayout{PartName: "ppt/slideLayouts/slideLayout2.xml"}

	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		SlideMasters: []*parts.SlideMasterEntry{
			{PartName: "ppt/slideMasters/slideMaster1.xml", Master: masterA, Layouts: []*parts.SlideLayout{layoutA}},
			{PartName: "ppt/slideMasters/slideMaster2.xml", Master: masterB, Layouts: []*parts.SlideLayout{layoutB}},
		},
		Slides: []*parts.SlideEntry{
			{
				Slide: &parts.Slide{
					CSld: parts.CommonSlideData{SpTree: parts.ShapeTree{Shapes: []parts.Shape{}}},
				},
				Layout:         layoutA,
				MasterPartName: "ppt/slideMasters/slideMaster1.xml",
			},
			{
				Slide: &parts.Slide{
					CSld: parts.CommonSlideData{SpTree: parts.ShapeTree{Shapes: []parts.Shape{}}},
				},
				Layout:         layoutB,
				MasterPartName: "ppt/slideMasters/slideMaster2.xml",
			},
		},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(pres.Slides) != 2 {
		t.Fatalf("slides = %d, want 2", len(pres.Slides))
	}

	if pres.Slides[0].BgColor != "FF0000" {
		t.Fatalf("slide0 bg = %q, want FF0000 (masterA)", pres.Slides[0].BgColor)
	}
	if pres.Slides[1].BgColor != "00FF00" {
		t.Fatalf("slide1 bg = %q, want 00FF00 (masterB)", pres.Slides[1].BgColor)
	}
}

func TestResolvePackageFallsBackToLayoutMembershipForMaster(t *testing.T) {
	// When MasterPartName is absent, the resolver should fall back to finding
	// the master that owns the slide's layout via PartName matching.
	masterA := &parts.SlideMaster{
		CSld: parts.CommonSlideData{
			Bg: &parts.SlideBg{BgPrSrgb: &drawingxml.SRGBClr{Val: "0000FF"}},
		},
	}
	layoutA := &parts.SlideLayout{PartName: "ppt/slideLayouts/slideLayout1.xml"}

	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		SlideMasters: []*parts.SlideMasterEntry{
			{PartName: "ppt/slideMasters/slideMaster1.xml", Master: masterA, Layouts: []*parts.SlideLayout{layoutA}},
		},
		Slides: []*parts.SlideEntry{
			{
				Slide: &parts.Slide{
					CSld: parts.CommonSlideData{SpTree: parts.ShapeTree{Shapes: []parts.Shape{}}},
				},
				Layout: layoutA,
				// MasterPartName intentionally left empty
			},
		},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(pres.Slides) != 1 {
		t.Fatalf("slides = %d, want 1", len(pres.Slides))
	}
	if pres.Slides[0].BgColor != "0000FF" {
		t.Fatalf("bg = %q, want 0000FF (master fallback via layout membership)", pres.Slides[0].BgColor)
	}
}

func TestResolvePackagePrependsMasterDecorativeShapes(t *testing.T) {
	master := &parts.SlideMaster{
		CSld: parts.CommonSlideData{
			SpTree: parts.ShapeTree{
				Shapes: []parts.Shape{
					{
						NvSpPr: parts.NonVisualShapeProps{
							CNvPr: parts.CnvPr{ID: 10, Name: "Master Decoration"},
						},
						SpPr: parts.ShapeProperties{
							Xfrm: &parts.Transform2D{
								Off: parts.Offset{X: 50, Y: 60},
								Ext: parts.Extents{CX: 70, CY: 80},
							},
						},
						TxBody: &parts.TextBody{
							Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{{Text: "background"}}}},
						},
					},
					{
						NvSpPr: parts.NonVisualShapeProps{
							CNvPr: parts.CnvPr{ID: 11, Name: "Master Title Placeholder"},
							NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "title"}},
						},
						TxBody: &parts.TextBody{
							Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{{Text: "skip me"}}}},
						},
					},
				},
				Items: []parts.ShapeTreeItem{
					{Kind: parts.ShapeTreeItemShape, Index: 0},
					{Kind: parts.ShapeTreeItemShape, Index: 1},
				},
			},
		},
	}
	slideSp := parts.Shape{
		NvSpPr: parts.NonVisualShapeProps{
			CNvPr: parts.CnvPr{ID: 1, Name: "Slide Shape"},
		},
		TxBody: &parts.TextBody{
			Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{{Text: "foreground"}}}},
		},
	}

	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000}},
		SlideMasters: []*parts.SlideMasterEntry{{
			PartName: "ppt/slideMasters/slideMaster1.xml",
			Master:   master,
		}},
		Slides: []*parts.SlideEntry{{
			Slide: &parts.Slide{
				CSld: parts.CommonSlideData{
					SpTree: parts.ShapeTree{Shapes: []parts.Shape{slideSp}},
				},
			},
			Layout:         &parts.SlideLayout{},
			MasterPartName: "ppt/slideMasters/slideMaster1.xml",
		}},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(pres.Slides) != 1 || len(pres.Slides[0].Shapes) != 2 {
		t.Fatalf("resolved shapes = %#v", pres.Slides)
	}
	if pres.Slides[0].Shapes[0].Name != "Master Decoration" {
		t.Fatalf("first shape = %q, want master decorative shape first", pres.Slides[0].Shapes[0].Name)
	}
	if pres.Slides[0].Shapes[1].Name != "Slide Shape" {
		t.Fatalf("second shape = %q, want slide shape", pres.Slides[0].Shapes[1].Name)
	}
}

// --- POI XSLF shape property inheritance regressions ---

func TestResolveShapeInheritsTransformFromLayoutPlaceholder(t *testing.T) {
	// Slide placeholder has no xfrm; layout placeholder provides bounds.
	slideSp := &parts.Shape{
		NvSpPr: parts.NonVisualShapeProps{
			CNvPr: parts.CnvPr{ID: 1, Name: "Title"},
			NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "title"}},
		},
		SpPr: parts.ShapeProperties{}, // no xfrm
		TxBody: &parts.TextBody{
			Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{{Text: "Hello"}}}},
		},
	}
	layoutSp := &parts.Shape{
		NvSpPr: parts.NonVisualShapeProps{
			CNvPr: parts.CnvPr{ID: 2, Name: "Layout Title"},
			NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "title"}},
		},
		SpPr: parts.ShapeProperties{
			Xfrm: &parts.Transform2D{
				Off: parts.Offset{X: 100, Y: 200},
				Ext: parts.Extents{CX: 300, CY: 400},
			},
		},
	}

	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000}},
		Slides: []*parts.SlideEntry{{
			Slide: &parts.Slide{
				CSld: parts.CommonSlideData{
					SpTree: parts.ShapeTree{Shapes: []parts.Shape{*slideSp}},
				},
			},
			Layout: &parts.SlideLayout{
				CSld: parts.CommonSlideData{
					SpTree: parts.ShapeTree{Shapes: []parts.Shape{*layoutSp}},
				},
			},
		}},
	}
	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(pres.Slides) != 1 || len(pres.Slides[0].Shapes) != 1 {
		t.Fatalf("expected 1 slide with 1 shape")
	}
	shape := pres.Slides[0].Shapes[0]
	if shape.Bounds.X != 100 || shape.Bounds.Y != 200 || shape.Bounds.W != 300 || shape.Bounds.H != 400 {
		t.Fatalf("bounds = %+v, want {100,200,300,400}", shape.Bounds)
	}
}

func TestResolveShapeInheritsFillFromMasterPlaceholder(t *testing.T) {
	// Slide placeholder has no fill; master placeholder provides solid fill.
	slideSp := &parts.Shape{
		NvSpPr: parts.NonVisualShapeProps{
			CNvPr: parts.CnvPr{ID: 1, Name: "Body"},
			NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "body"}},
		},
		SpPr: parts.ShapeProperties{}, // no fill
		TxBody: &parts.TextBody{
			Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{{Text: "Body text"}}}},
		},
	}
	masterSp := &parts.Shape{
		NvSpPr: parts.NonVisualShapeProps{
			CNvPr: parts.CnvPr{ID: 2, Name: "Master Body"},
			NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "body"}},
		},
		SpPr: parts.ShapeProperties{
			SolidFill: &parts.ColorRef{Val: "FF0000"},
		},
	}
	master := &parts.SlideMaster{
		CSld: parts.CommonSlideData{
			SpTree: parts.ShapeTree{Shapes: []parts.Shape{*masterSp}},
		},
	}

	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000}},
		SlideMasters: []*parts.SlideMasterEntry{{Master: master}},
		Slides: []*parts.SlideEntry{{
			Slide: &parts.Slide{
				CSld: parts.CommonSlideData{
					SpTree: parts.ShapeTree{Shapes: []parts.Shape{*slideSp}},
				},
			},
			Layout: &parts.SlideLayout{},
		}},
	}
	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	shape := pres.Slides[0].Shapes[0]
	if shape.FillColor != "FF0000" {
		t.Fatalf("fill color = %q, want FF0000", shape.FillColor)
	}
}

func TestResolveShapeInheritsLineFromLayoutPlaceholder(t *testing.T) {
	// Slide placeholder has no line; layout placeholder provides line.
	slideSp := &parts.Shape{
		NvSpPr: parts.NonVisualShapeProps{
			CNvPr: parts.CnvPr{ID: 1, Name: "Title"},
			NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "title"}},
		},
		SpPr: parts.ShapeProperties{}, // no line
		TxBody: &parts.TextBody{
			Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{{Text: "Title"}}}},
		},
	}
	layoutSp := &parts.Shape{
		NvSpPr: parts.NonVisualShapeProps{
			CNvPr: parts.CnvPr{ID: 2, Name: "Layout Title"},
			NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "title"}},
		},
		SpPr: parts.ShapeProperties{
			Line: &parts.ConnectorLine{
				SolidFill: &drawingxml.SolidFill{SRGBClr: &drawingxml.SRGBClr{Val: "0000FF"}},
				W:         12700, // 1pt in EMU
			},
		},
	}

	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000}},
		Slides: []*parts.SlideEntry{{
			Slide: &parts.Slide{
				CSld: parts.CommonSlideData{
					SpTree: parts.ShapeTree{Shapes: []parts.Shape{*slideSp}},
				},
			},
			Layout: &parts.SlideLayout{
				CSld: parts.CommonSlideData{
					SpTree: parts.ShapeTree{Shapes: []parts.Shape{*layoutSp}},
				},
			},
		}},
	}
	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	shape := pres.Slides[0].Shapes[0]
	if shape.LineColor != "0000FF" {
		t.Fatalf("line color = %q, want 0000FF", shape.LineColor)
	}
	if shape.LineWidthPt != 1 {
		t.Fatalf("line width = %v, want 1", shape.LineWidthPt)
	}
}

func TestResolveShapeExplicitNoFillBlocksInheritance(t *testing.T) {
	// Slide placeholder explicitly sets NoFill; should NOT inherit fill from master.
	slideSp := &parts.Shape{
		NvSpPr: parts.NonVisualShapeProps{
			CNvPr: parts.CnvPr{ID: 1, Name: "Body"},
			NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "body"}},
		},
		SpPr: parts.ShapeProperties{
			NoFill: &struct{}{}, // explicit no fill
		},
		TxBody: &parts.TextBody{
			Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{{Text: "Body text"}}}},
		},
	}
	masterSp := &parts.Shape{
		NvSpPr: parts.NonVisualShapeProps{
			CNvPr: parts.CnvPr{ID: 2, Name: "Master Body"},
			NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "body"}},
		},
		SpPr: parts.ShapeProperties{
			SolidFill: &parts.ColorRef{Val: "FF0000"},
		},
	}
	master := &parts.SlideMaster{
		CSld: parts.CommonSlideData{
			SpTree: parts.ShapeTree{Shapes: []parts.Shape{*masterSp}},
		},
	}

	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000}},
		SlideMasters: []*parts.SlideMasterEntry{{Master: master}},
		Slides: []*parts.SlideEntry{{
			Slide: &parts.Slide{
				CSld: parts.CommonSlideData{
					SpTree: parts.ShapeTree{Shapes: []parts.Shape{*slideSp}},
				},
			},
			Layout: &parts.SlideLayout{},
		}},
	}
	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	shape := pres.Slides[0].Shapes[0]
	if shape.FillColor != "" {
		t.Fatalf("fill color = %q, want empty (explicit noFill blocks inheritance)", shape.FillColor)
	}
}

func TestResolveShapeOwnPropertiesTakePrecedence(t *testing.T) {
	// Slide placeholder has its own xfrm, fill, and line — should not inherit.
	slideSp := &parts.Shape{
		NvSpPr: parts.NonVisualShapeProps{
			CNvPr: parts.CnvPr{ID: 1, Name: "Title"},
			NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "title"}},
		},
		SpPr: parts.ShapeProperties{
			Xfrm: &parts.Transform2D{
				Off: parts.Offset{X: 10, Y: 20},
				Ext: parts.Extents{CX: 50, CY: 60},
			},
			SolidFill: &parts.ColorRef{Val: "00FF00"},
			Line: &parts.ConnectorLine{
				SolidFill: &drawingxml.SolidFill{SRGBClr: &drawingxml.SRGBClr{Val: "000000"}},
				W:         25400,
			},
		},
		TxBody: &parts.TextBody{
			Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{{Text: "Title"}}}},
		},
	}
	layoutSp := &parts.Shape{
		NvSpPr: parts.NonVisualShapeProps{
			CNvPr: parts.CnvPr{ID: 2, Name: "Layout Title"},
			NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "title"}},
		},
		SpPr: parts.ShapeProperties{
			Xfrm: &parts.Transform2D{
				Off: parts.Offset{X: 100, Y: 200},
				Ext: parts.Extents{CX: 300, CY: 400},
			},
			SolidFill: &parts.ColorRef{Val: "FF0000"},
			Line: &parts.ConnectorLine{
				SolidFill: &drawingxml.SolidFill{SRGBClr: &drawingxml.SRGBClr{Val: "0000FF"}},
				W:         12700,
			},
		},
	}

	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000}},
		Slides: []*parts.SlideEntry{{
			Slide: &parts.Slide{
				CSld: parts.CommonSlideData{
					SpTree: parts.ShapeTree{Shapes: []parts.Shape{*slideSp}},
				},
			},
			Layout: &parts.SlideLayout{
				CSld: parts.CommonSlideData{
					SpTree: parts.ShapeTree{Shapes: []parts.Shape{*layoutSp}},
				},
			},
		}},
	}
	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	shape := pres.Slides[0].Shapes[0]
	if shape.Bounds.W != 50 {
		t.Fatalf("bounds.W = %d, want 50 (own xfrm takes precedence)", shape.Bounds.W)
	}
	if shape.FillColor != "00FF00" {
		t.Fatalf("fill = %q, want 00FF00 (own fill takes precedence)", shape.FillColor)
	}
	if shape.LineColor != "000000" {
		t.Fatalf("line = %q, want 000000 (own line takes precedence)", shape.LineColor)
	}
	if shape.LineWidthPt != 2 {
		t.Fatalf("line width = %v, want 2 (own line takes precedence)", shape.LineWidthPt)
	}
}

func TestResolveShapeInheritsPictureFillFromLayoutMediaParts(t *testing.T) {
	slideSp := &parts.Shape{
		NvSpPr: parts.NonVisualShapeProps{
			CNvPr: parts.CnvPr{ID: 1, Name: "Title"},
			NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "title"}},
		},
		SpPr: parts.ShapeProperties{},
		TxBody: &parts.TextBody{
			Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{{Text: "Title"}}}},
		},
	}
	layoutSp := &parts.Shape{
		NvSpPr: parts.NonVisualShapeProps{
			CNvPr: parts.CnvPr{ID: 2, Name: "Layout Title"},
			NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "title"}},
		},
		SpPr: parts.ShapeProperties{
			BlipFill: &parts.BlipFill{
				Blip: &parts.Blip{Embed: "rIdLayoutImg"},
			},
		},
	}

	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000}},
		Slides: []*parts.SlideEntry{{
			Slide: &parts.Slide{
				CSld: parts.CommonSlideData{
					SpTree: parts.ShapeTree{Shapes: []parts.Shape{*slideSp}},
				},
			},
			Layout: &parts.SlideLayout{
				CSld: parts.CommonSlideData{
					SpTree: parts.ShapeTree{Shapes: []parts.Shape{*layoutSp}},
				},
			},
			LayoutMediaParts: map[string]media.Part{
				"rIdLayoutImg": {
					Name:        "ppt/media/layout-fill.png",
					ContentType: "image/png",
					Data:        []byte{0x89, 0x50, 0x4E, 0x47},
				},
			},
		}},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	fill, ok := pres.Slides[0].Shapes[0].Fill.(drawing.PictureFill)
	if !ok {
		t.Fatalf("fill type = %T, want drawing.PictureFill", pres.Slides[0].Shapes[0].Fill)
	}
	if fill.ContentType != "image/png" {
		t.Fatalf("content type = %q, want image/png", fill.ContentType)
	}
	if string(fill.ImageData) != string([]byte{0x89, 0x50, 0x4E, 0x47}) {
		t.Fatalf("image data = %v, want inherited layout media payload", fill.ImageData)
	}
}

func TestResolvePackageRendersEmbeddedVideoPicture(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000}},
		Slides: []*parts.SlideEntry{{
			Slide: &parts.Slide{
				CSld: parts.CommonSlideData{
					SpTree: parts.ShapeTree{
						Pictures: []parts.Picture{{
							NvPicPr: parts.NonVisualPicProps{
								CNvPr: parts.CnvPr{ID: 2, Name: "Video 1"},
								NvPr: parts.ApplicationNonVisualDrawingProps{
									VideoFile: &parts.MediaReference{Link: "rIdVideo"},
									Media:     &parts.MediaReference{Embed: "rIdVideo"},
								},
							},
							BlipFill: parts.BlipFill{
								Blip: &parts.Blip{Embed: "rIdPoster"},
							},
							SpPr: parts.ShapeProperties{
								Xfrm: &parts.Transform2D{
									Off: parts.Offset{X: 100, Y: 200},
									Ext: parts.Extents{CX: 300, CY: 400},
								},
							},
						}},
						Items: []parts.ShapeTreeItem{
							{Kind: parts.ShapeTreeItemPicture, Index: 0},
						},
					},
				},
			},
			Layout: &parts.SlideLayout{},
			MediaParts: map[string]media.Part{
				"rIdVideo": {
					Name:        "ppt/media/movie.mp4",
					ContentType: "video/mp4",
					Data:        []byte("mp4-bytes"),
				},
				"rIdPoster": {
					Name:        "ppt/media/poster.png",
					ContentType: "image/png",
					Data:        []byte{0x89, 0x50, 0x4E, 0x47},
				},
			},
		}},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(pres.Slides) != 1 || len(pres.Slides[0].Shapes) != 1 {
		t.Fatalf("resolved slides = %#v", pres.Slides)
	}
	shape := pres.Slides[0].Shapes[0]
	if shape.Kind != resolve.ShapeMedia {
		t.Fatalf("shape kind = %v, want ShapeMedia", shape.Kind)
	}
	if shape.MediaKind != "video" {
		t.Fatalf("media kind = %q, want video", shape.MediaKind)
	}
	if shape.MediaMIME != "video/mp4" {
		t.Fatalf("media mime = %q, want video/mp4", shape.MediaMIME)
	}
	if string(shape.MediaData) != "mp4-bytes" {
		t.Fatalf("media data = %q, want mp4-bytes", string(shape.MediaData))
	}
	if shape.MediaPosterMIME != "image/png" {
		t.Fatalf("poster mime = %q, want image/png", shape.MediaPosterMIME)
	}
	if string(shape.MediaPosterData) != string([]byte{0x89, 0x50, 0x4E, 0x47}) {
		t.Fatalf("poster data = %v, want preview payload", shape.MediaPosterData)
	}
}

func TestResolvePackageParagraphPropertiesExtended(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		Slides: []*parts.SlideEntry{
			{
				Slide: &parts.Slide{
					CSld: parts.CommonSlideData{
						SpTree: parts.ShapeTree{
							Shapes: []parts.Shape{
								{
									NvSpPr: parts.NonVisualShapeProps{
										CNvPr: parts.CnvPr{ID: 1, Name: "Body"},
										NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "body", Idx: 1}},
									},
									TxBody: &parts.TextBody{
										Paragraphs: []parts.DMLParagraph{
											{
												Properties: &parts.DMLParaProps{
													MarR:     intPtr(457200),
													DefTabSz: intPtr(914400),
													FontAlgn: "b",
													BuSzPct:  &parts.BuSzPct{Val: 125000},
													TabStops: []parts.TabStop{{Pos: 457200, Algn: "l"}, {Pos: 914400, Algn: "ctr"}},
												},
												Runs: []parts.DMLRun{{Text: "Hello"}},
											},
										},
									},
								},
							},
						},
					},
				},
				Layout: &parts.SlideLayout{
					CSld: parts.CommonSlideData{
						SpTree: parts.ShapeTree{
							Shapes: []parts.Shape{
								{
									NvSpPr: parts.NonVisualShapeProps{
										CNvPr: parts.CnvPr{ID: 2, Name: "Layout Body"},
										NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "body", Idx: 1}},
									},
									TxBody: &parts.TextBody{
										ListStyle: &parts.DMLTextListStyle{
											Lvl1PPr: &parts.DMLTextStyleLevel{
												MarR:     intPtr(228600),
												DefTabSz: intPtr(457200),
												FontAlgn: "t",
												BuSzPts:  &parts.BuSzPts{Val: 2000},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	p := pres.Slides[0].Shapes[0].Paragraphs[0]

	// Shape-level overrides should win over layout defaults.
	if !p.HasMarginRight || p.MarginRightEMU != 457200 {
		t.Fatalf("marginRight = %d (has=%v), want 457200 true", p.MarginRightEMU, p.HasMarginRight)
	}
	if p.DefaultTabSizeEMU != 914400 {
		t.Fatalf("defaultTabSize = %d, want 914400", p.DefaultTabSizeEMU)
	}
	if p.FontAlign != "b" {
		t.Fatalf("fontAlign = %q, want b", p.FontAlign)
	}
	// buSzPct on shape should take precedence; buSzPts from layout should be ignored.
	if p.BulletSizePct != 125000 {
		t.Fatalf("bulletSizePct = %d, want 125000", p.BulletSizePct)
	}
	if p.BulletSize != 0 {
		t.Fatalf("bulletSize = %d, want 0 (pct takes precedence)", p.BulletSize)
	}
	if len(p.TabStops) != 2 {
		t.Fatalf("tabStops count = %d, want 2", len(p.TabStops))
	}
	if p.TabStops[0].Pos != 457200 || p.TabStops[0].Algn != "l" {
		t.Fatalf("tabStop[0] = %+v, want {Pos:457200 Algn:l}", p.TabStops[0])
	}
}

func TestResolvePackageParagraphPropertiesInheritFromLayout(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		Slides: []*parts.SlideEntry{
			{
				Slide: &parts.Slide{
					CSld: parts.CommonSlideData{
						SpTree: parts.ShapeTree{
							Shapes: []parts.Shape{
								{
									NvSpPr: parts.NonVisualShapeProps{
										CNvPr: parts.CnvPr{ID: 1, Name: "Body"},
										NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "body", Idx: 1}},
									},
									TxBody: &parts.TextBody{
										Paragraphs: []parts.DMLParagraph{
											{Runs: []parts.DMLRun{{Text: "Hello"}}},
										},
									},
								},
							},
						},
					},
				},
				Layout: &parts.SlideLayout{
					CSld: parts.CommonSlideData{
						SpTree: parts.ShapeTree{
							Shapes: []parts.Shape{
								{
									NvSpPr: parts.NonVisualShapeProps{
										CNvPr: parts.CnvPr{ID: 2, Name: "Layout Body"},
										NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "body", Idx: 1}},
									},
									TxBody: &parts.TextBody{
										ListStyle: &parts.DMLTextListStyle{
											Lvl1PPr: &parts.DMLTextStyleLevel{
												MarR:     intPtr(228600),
												DefTabSz: intPtr(457200),
												FontAlgn: "t",
												BuSzPct:  &parts.BuSzPct{Val: 150000},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	p := pres.Slides[0].Shapes[0].Paragraphs[0]

	if !p.HasMarginRight || p.MarginRightEMU != 228600 {
		t.Fatalf("marginRight = %d (has=%v), want 228600 true", p.MarginRightEMU, p.HasMarginRight)
	}
	if p.DefaultTabSizeEMU != 457200 {
		t.Fatalf("defaultTabSize = %d, want 457200", p.DefaultTabSizeEMU)
	}
	if p.FontAlign != "t" {
		t.Fatalf("fontAlign = %q, want t", p.FontAlign)
	}
	if p.BulletSizePct != 150000 {
		t.Fatalf("bulletSizePct = %d, want 150000", p.BulletSizePct)
	}
}

func TestResolvePackageEndParaRPrOverridesMasterDefaults(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		SlideMasters: []*parts.SlideMasterEntry{
			{
				Master: &parts.SlideMaster{
					TxStyles: &parts.MasterTxStyles{
						BodyStyle: &parts.DMLTextListStyle{
							Lvl1PPr: &parts.DMLTextStyleLevel{
								DefRPr: &parts.DMLRunPr{FontSize: 1800},
							},
						},
					},
				},
			},
		},
		Slides: []*parts.SlideEntry{
			{
				Slide: &parts.Slide{
					CSld: parts.CommonSlideData{
						SpTree: parts.ShapeTree{
							Shapes: []parts.Shape{
								{
									NvSpPr: parts.NonVisualShapeProps{
										CNvPr: parts.CnvPr{ID: 1, Name: "Body"},
										NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "body"}},
									},
									TxBody: &parts.TextBody{
										Paragraphs: []parts.DMLParagraph{
											{
												// Master says 18pt, endParaRPr says 24pt
												Runs:       []parts.DMLRun{{Text: "Hello"}},
												EndParaRPr: &parts.DMLRunPr{FontSize: 2400},
											},
										},
									},
								},
							},
						},
					},
				},
				MasterPartName: "ppt/slideMasters/slideMaster1.xml",
			},
		},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(pres.Slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(pres.Slides))
	}
	para := pres.Slides[0].Shapes[0].Paragraphs[0]
	// endParaRPr should override master defRPr for the paragraph's default font size
	if para.Runs[0].FontSize != 2400 {
		t.Fatalf("run font size = %d, want 2400 (from endParaRPr)", para.Runs[0].FontSize)
	}
}

func TestResolvePackageEndParaRPrInheritsWhenEmpty(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		SlideMasters: []*parts.SlideMasterEntry{
			{
				Master: &parts.SlideMaster{},
			},
		},
		Slides: []*parts.SlideEntry{
			{
				Slide: &parts.Slide{
					CSld: parts.CommonSlideData{
						SpTree: parts.ShapeTree{
							Shapes: []parts.Shape{
								{
									NvSpPr: parts.NonVisualShapeProps{
										CNvPr: parts.CnvPr{ID: 1, Name: "Body"},
										NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "body"}},
									},
									TxBody: &parts.TextBody{
										Paragraphs: []parts.DMLParagraph{
											{
												// No runs, just endParaRPr with font size
												EndParaRPr: &parts.DMLRunPr{FontSize: 3200},
											},
										},
									},
								},
							},
						},
					},
				},
				MasterPartName: "ppt/slideMasters/slideMaster1.xml",
			},
		},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(pres.Slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(pres.Slides))
	}
	para := pres.Slides[0].Shapes[0].Paragraphs[0]
	if para.Runs != nil && len(para.Runs) > 0 {
		t.Fatalf("expected no runs, got %d", len(para.Runs))
	}
	// The empty paragraph should still carry endParaRPr info for potential rendering
	if para.Runs == nil {
		t.Logf("Empty paragraph preserved (no runs)")
	}
}

func TestResolvePackageBodyPrRotWrapAnchorCtr(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		Slides: []*parts.SlideEntry{{
			Slide: &parts.Slide{
				CSld: parts.CommonSlideData{
					SpTree: parts.ShapeTree{
						Shapes: []parts.Shape{{
							NvSpPr: parts.NonVisualShapeProps{
								CNvPr: parts.CnvPr{ID: 1, Name: "Box"},
							},
							TxBody: &parts.TextBody{
								BodyPr: &parts.DMLBodyPr{
									Rot:       900000,
									Wrap:      "none",
									Anchor:    "ctr",
									AnchorCtr: boolPtr(true),
								},
								Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{{Text: "Hi"}}}},
							},
						}},
					},
				},
			},
		}},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	props := pres.Slides[0].Shapes[0].TextBodyProps
	if props.Rot != 900000 {
		t.Fatalf("rot = %d, want 900000", props.Rot)
	}
	if props.Wrap != "none" {
		t.Fatalf("wrap = %q, want none", props.Wrap)
	}
	if !props.AnchorCtr {
		t.Fatal("expected anchorCtr=true")
	}
}

func TestResolvePackageRunHighlight(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		Slides: []*parts.SlideEntry{{
			Slide: &parts.Slide{
				CSld: parts.CommonSlideData{
					SpTree: parts.ShapeTree{
						Shapes: []parts.Shape{{
							NvSpPr: parts.NonVisualShapeProps{
								CNvPr: parts.CnvPr{ID: 1, Name: "Box"},
							},
							TxBody: &parts.TextBody{
								Paragraphs: []parts.DMLParagraph{{
									Runs: []parts.DMLRun{{
										Text: "Hi",
										Properties: &parts.DMLRunPr{
											Highlight: &parts.ColorRef{Val: "FFFF00"},
										},
									}},
								}},
							},
						}},
					},
				},
			},
		}},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	run := pres.Slides[0].Shapes[0].Paragraphs[0].Runs[0]
	if run.Highlight != "FFFF00" {
		t.Fatalf("highlight = %q, want FFFF00", run.Highlight)
	}
}

func TestResolvePackageShapeStyleFillRef(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		Themes: map[string]*parts.Theme{
			"ppt/theme/theme1.xml": {
				Name: "Test Theme",
				Elements: parts.ThemeElements{
					ClrScheme: parts.ColorScheme{
						Accent1: &parts.ThemeColor{SrgbClr: &parts.SrgbColor{Val: "4472C4"}},
					},
					FontScheme: parts.FontScheme{
						MajorFont: parts.FontGroup{Latin: parts.FontRef{Typeface: "Calibri"}},
						MinorFont: parts.FontGroup{Latin: parts.FontRef{Typeface: "Arial"}},
					},
					FmtScheme: parts.FormatScheme{
						FillStyleList: []parts.ThemeFormatFillStyle{
							{SolidFill: &parts.ColorRef{Val: "FF0000"}},
							{SolidFill: &parts.ColorRef{Val: "00FF00"}},
						},
						LineStyleList: []parts.ConnectorLine{
							{W: 12700, SolidFill: &drawingxml.SolidFill{SRGBClr: &drawingxml.SRGBClr{Val: "000000"}}},
						},
					},
				},
			},
		},
		Slides: []*parts.SlideEntry{{
			Slide: &parts.Slide{
				CSld: parts.CommonSlideData{
					SpTree: parts.ShapeTree{
						Shapes: []parts.Shape{{
							NvSpPr: parts.NonVisualShapeProps{
								CNvPr: parts.CnvPr{ID: 1, Name: "Styled"},
							},
							SpPr: parts.ShapeProperties{}, // no explicit fill
							Style: &parts.ShapeStyle{
								FillRef: &parts.ShapeStyleRef{Idx: 2}, // points to 00FF00
								LnRef:   &parts.ShapeStyleRef{Idx: 1},
							},
							TxBody: &parts.TextBody{
								Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{{Text: "Hi"}}}},
							},
						}},
					},
				},
			},
		}},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	shape := pres.Slides[0].Shapes[0]
	if shape.FillColor != "00FF00" {
		t.Fatalf("fillColor = %q, want 00FF00", shape.FillColor)
	}
	if shape.LineWidthPt == 0 {
		t.Fatal("expected line style from lnRef")
	}
}

func TestResolvePackageShapeStyleFontRef(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		Themes: map[string]*parts.Theme{
			"ppt/theme/theme1.xml": {
				Name: "Test Theme",
				Elements: parts.ThemeElements{
					ClrScheme: parts.ColorScheme{},
					FontScheme: parts.FontScheme{
						MajorFont: parts.FontGroup{Latin: parts.FontRef{Typeface: "Calibri"}},
						MinorFont: parts.FontGroup{Latin: parts.FontRef{Typeface: "Arial"}},
					},
					FmtScheme: parts.FormatScheme{},
				},
			},
		},
		Slides: []*parts.SlideEntry{{
			Slide: &parts.Slide{
				CSld: parts.CommonSlideData{
					SpTree: parts.ShapeTree{
						Shapes: []parts.Shape{{
							NvSpPr: parts.NonVisualShapeProps{
								CNvPr: parts.CnvPr{ID: 1, Name: "Styled"},
							},
							Style: &parts.ShapeStyle{
								FontRef: &parts.ShapeFontRef{Idx: "minor"},
							},
							TxBody: &parts.TextBody{
								Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{{Text: "Hi"}}}},
							},
						}},
					},
				},
			},
		}},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	run := pres.Slides[0].Shapes[0].Paragraphs[0].Runs[0]
	if run.FontFamily != "Arial" {
		t.Fatalf("fontFamily = %q, want Arial", run.FontFamily)
	}
}

func TestResolvePackageAppliesEmphasisEffects(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		Slides: []*parts.SlideEntry{{
			Slide: &parts.Slide{
				CSld: parts.CommonSlideData{
					SpTree: parts.ShapeTree{
						Shapes: []parts.Shape{{
							NvSpPr: parts.NonVisualShapeProps{
								CNvPr: parts.CnvPr{ID: 1, Name: "Box"},
							},
							TxBody: &parts.TextBody{
								Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{{Text: "Hi"}}}},
							},
						}},
					},
				},
				EmphasisEffects: map[int][]string{
					1: {"spin", "growShrink"},
				},
			},
		}},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	shape := pres.Slides[0].Shapes[0]
	if len(shape.EmphasisEffects) != 2 || shape.EmphasisEffects[0] != "spin" || shape.EmphasisEffects[1] != "growShrink" {
		t.Fatalf("emphasis effects = %v, want [spin growShrink]", shape.EmphasisEffects)
	}
}

func TestResolveChartPreservesGraphicFrameBounds(t *testing.T) {
	path := repoPath(t, "testdata", "pptx", "bar-chart.pptx")
	pkg, err := opc.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	pptPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(pres.Slides) == 0 {
		t.Fatal("no slides resolved")
	}

	var chart *resolve.Shape
	for _, sh := range pres.Slides[0].Shapes {
		if sh.Kind == resolve.ShapeChart {
			chart = sh
			break
		}
	}
	if chart == nil {
		t.Fatal("no chart shape resolved on slide 0")
	}

	want := resolve.Rect{X: 1524000, Y: 1397000, W: 6096000, H: 4064000}
	if chart.Bounds != want {
		t.Errorf("chart bounds = %+v, want %+v", chart.Bounds, want)
	}
}

func TestResolvePackageBackgroundsPictureTileFill(t *testing.T) {
	path := repoPath(t, "testdata", "pptx", "backgrounds.pptx")
	pkg, err := opc.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	pptPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(pres.Slides) < 4 {
		t.Fatalf("expected at least 4 slides, got %d", len(pres.Slides))
	}
	slide := pres.Slides[3]
	pf, ok := slide.BgFill.(drawing.PictureFill)
	if !ok {
		t.Fatalf("slide 4 BgFill is %T, want PictureFill", slide.BgFill)
	}
	if !pf.Tile {
		t.Errorf("expected tiled background")
	}
	if len(pf.ImageData) == 0 {
		t.Errorf("expected background image data to be materialized")
	}
}

func TestResolvePreservesBackgroundPictureTileMetadata(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000}},
		Slides: []*parts.SlideEntry{{
			Slide: &parts.Slide{CSld: parts.CommonSlideData{
				Bg: &parts.SlideBg{BgPrBlipFill: &parts.BlipFill{
					Blip: &parts.Blip{Embed: "rIdTile"},
					Tile: &drawingxml.Tile{Tx: 12700, Ty: 25400, Sx: 50000, Sy: 25000, Flip: "x", Algn: "ctr"},
				}},
				SpTree: parts.ShapeTree{},
			}},
			MediaParts: map[string]media.Part{"rIdTile": {ContentType: "image/png", Data: []byte("tile")}},
		}},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	tile := pres.Slides[0].BgPictureTile
	if tile == nil {
		t.Fatal("expected background picture tile metadata")
	}
	if tile.Tx != 12700 || tile.Ty != 25400 || tile.Sx != 50000 || tile.Sy != 25000 || tile.Flip != "x" || tile.Alignment != "ctr" {
		t.Fatalf("unexpected tile metadata: %+v", tile)
	}
}

func TestResolveShapeInheritsPictureTileMetadataFromLayout(t *testing.T) {
	slideShape := parts.Shape{
		NvSpPr: parts.NonVisualShapeProps{CNvPr: parts.CnvPr{ID: 1, Name: "Title"}, NvPr: parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "title"}}},
		TxBody: &parts.TextBody{Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{{Text: "Title"}}}}},
	}
	layoutShape := parts.Shape{
		NvSpPr: parts.NonVisualShapeProps{CNvPr: parts.CnvPr{ID: 2, Name: "Layout Title"}, NvPr: parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "title"}}},
		SpPr: parts.ShapeProperties{BlipFill: &parts.BlipFill{
			Blip: &parts.Blip{Embed: "rIdLayoutTile"},
			Tile: &drawingxml.Tile{Sx: 40000, Sy: 60000, Algn: "br", Flip: "xy"},
		}},
	}
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000}},
		Slides: []*parts.SlideEntry{{
			Slide:            &parts.Slide{CSld: parts.CommonSlideData{SpTree: parts.ShapeTree{Shapes: []parts.Shape{slideShape}}}},
			Layout:           &parts.SlideLayout{CSld: parts.CommonSlideData{SpTree: parts.ShapeTree{Shapes: []parts.Shape{layoutShape}}}},
			LayoutMediaParts: map[string]media.Part{"rIdLayoutTile": {ContentType: "image/png", Data: []byte("tile")}},
		}},
	}
	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	tile := pres.Slides[0].Shapes[0].PictureTile
	if tile == nil || tile.Sx != 40000 || tile.Sy != 60000 || tile.Alignment != "br" || tile.Flip != "xy" {
		t.Fatalf("unexpected inherited tile metadata: %+v", tile)
	}
}

func TestResolvePackageShapeStyleEffectDagRef(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000}},
		Themes: map[string]*parts.Theme{
			"ppt/theme/theme1.xml": {
				Name: "Effect DAG Theme",
				Elements: parts.ThemeElements{
					ClrScheme: parts.ColorScheme{Accent1: &parts.ThemeColor{SrgbClr: &parts.SrgbColor{Val: "4472C4"}}},
					FmtScheme: parts.FormatScheme{EffectStyleList: []parts.ThemeFormatEffectStyle{{
						EffectDag: &parts.EffectDag{Items: []parts.EffectDagItem{
							{Glow: &parts.GlowEffect{Rad: 25400, SchemeClr: &parts.EffectSchemeColor{Val: "accent1"}}},
							{PresetShadow: &parts.PresetShadowEffect{Prst: "shdw8", Dist: 25400, Dir: 2700000, SchemeClr: &parts.EffectSchemeColor{Val: "accent1"}}},
							{FillOverlay: &parts.FillOverlayEffect{Blend: "screen", SolidFill: &parts.ColorRef{Val: "FF0000"}}},
							{Container: &parts.EffectDag{Type: "tree", Items: []parts.EffectDagItem{{Blur: &parts.BlurEffect{Rad: 12700, Grow: func() *bool { v := true; return &v }()}}}}},
						}},
					}}},
				},
			},
		},
		Slides: []*parts.SlideEntry{{
			Slide: &parts.Slide{CSld: parts.CommonSlideData{SpTree: parts.ShapeTree{Shapes: []parts.Shape{{
				NvSpPr: parts.NonVisualShapeProps{CNvPr: parts.CnvPr{ID: 1, Name: "Styled"}},
				Style:  &parts.ShapeStyle{EffectRef: &parts.ShapeStyleRef{Idx: 1}},
				TxBody: &parts.TextBody{Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{{Text: "Hi"}}}}},
			}}}}},
		}},
	}

	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	effects := pres.Slides[0].Shapes[0].Effects.Effects
	if len(effects) != 4 {
		t.Fatalf("effect count = %d, want 4", len(effects))
	}
	glow, ok := effects[0].(drawing.Glow)
	if !ok || glow.Color != "4472C4" || glow.Radius != 2.0 {
		t.Fatalf("unexpected glow = %#v", effects[0])
	}
	shadow, ok := effects[1].(drawing.Shadow)
	if !ok || shadow.Color != "4472C4" || shadow.Distance != 2.0 || shadow.Angle != 45.0 {
		t.Fatalf("unexpected preset shadow = %#v", effects[1])
	}
	overlay, ok := effects[2].(drawing.FillOverlay)
	if !ok || overlay.BlendMode != "screen" {
		t.Fatalf("unexpected fill overlay = %#v", effects[2])
	}
	blur, ok := effects[3].(drawing.Blur)
	if !ok || blur.Radius != 1.0 || !blur.Grow || blur.ContainerType != "tree" {
		t.Fatalf("unexpected blur = %#v", effects[3])
	}
}

func TestResolvePictureUsesBlipDPIForMissingExtents(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 200, 100))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	rot := false
	pkg := &parts.Package{
		Presentation: &parts.Presentation{SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000}},
		Slides: []*parts.SlideEntry{{
			Slide: &parts.Slide{CSld: parts.CommonSlideData{SpTree: parts.ShapeTree{Pictures: []parts.Picture{{
				NvPicPr:  parts.NonVisualPicProps{CNvPr: parts.CnvPr{ID: 7, Name: "Intrinsic"}},
				BlipFill: parts.BlipFill{Dpi: 200, RotWithShape: &rot, Blip: &parts.Blip{Embed: "rId1"}},
				SpPr:     parts.ShapeProperties{Xfrm: &parts.Transform2D{}},
			}}}}},
			MediaParts: map[string]media.Part{"rId1": {Data: buf.Bytes(), ContentType: "image/png"}},
		}},
	}
	pres, err := resolve.ResolvePackage(pkg)
	if err != nil {
		t.Fatal(err)
	}
	sh := pres.Slides[0].Shapes[0]
	if sh.ImageDPI != 200 || !sh.ImageRotWithShapeSet || sh.ImageRotWithShape {
		t.Fatalf("metadata = dpi %d rotSet %v rot %v", sh.ImageDPI, sh.ImageRotWithShapeSet, sh.ImageRotWithShape)
	}
	if sh.Bounds.W != 914400 || sh.Bounds.H != 457200 {
		t.Fatalf("intrinsic bounds = %+v", sh.Bounds)
	}
}

func TestResolvePicturePreservesStretchFillRect(t *testing.T) {
	pkg := &parts.Package{
		Presentation: &parts.Presentation{SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000}},
		Slides: []*parts.SlideEntry{{
			Slide: &parts.Slide{CSld: parts.CommonSlideData{SpTree: parts.ShapeTree{Pictures: []parts.Picture{{
				NvPicPr:  parts.NonVisualPicProps{CNvPr: parts.CnvPr{ID: 9, Name: "Inset"}},
				BlipFill: parts.BlipFill{Blip: &parts.Blip{Embed: "rId1"}, Stretch: &drawingxml.Stretch{FillRect: &drawingxml.FillRect{L: 10000, T: 20000, R: 30000, B: 10000}}},
				SpPr:     parts.ShapeProperties{Xfrm: &parts.Transform2D{Ext: parts.Extents{CX: 914400, CY: 914400}}},
			}}}}},
			MediaParts: map[string]media.Part{"rId1": {Data: []byte("img"), ContentType: "image/png"}},
		}},
	}
	pres, err := resolve.ResolvePackage(pkg)
	if err != nil {
		t.Fatal(err)
	}
	sh := pres.Slides[0].Shapes[0]
	if sh.ImageFillLeft != 0.1 || sh.ImageFillTop != 0.2 || sh.ImageFillRight != 0.3 || sh.ImageFillBottom != 0.1 {
		t.Fatalf("fill rect = %.2f %.2f %.2f %.2f", sh.ImageFillLeft, sh.ImageFillTop, sh.ImageFillRight, sh.ImageFillBottom)
	}
}
