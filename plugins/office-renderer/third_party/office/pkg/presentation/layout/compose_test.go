package layout_test

import (
	"testing"

	"office/pkg/ooxml/drawingxml"
	"office/pkg/presentation/layout"
	"office/pkg/presentation/parsers/ooxml/parts"
)

func TestComposePackageBasic(t *testing.T) {
	// Create a minimal package with master, layout, and slide
	pkg := &parts.Package{
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
										NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "title", Idx: 0}},
									},
									SpPr: parts.ShapeProperties{
										Xfrm: &parts.Transform2D{
											Off: parts.Offset{X: 100000, Y: 100000},
											Ext: parts.Extents{CX: 500000, CY: 200000},
										},
									},
									TxBody: &parts.TextBody{
										Paragraphs: []parts.DMLParagraph{
											{
												Runs: []parts.DMLRun{
													{Text: "Slide Title"},
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
										CNvPr: parts.CnvPr{ID: 2, Name: "Layout Title"},
										NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "title", Idx: 0}},
									},
									TxBody: &parts.TextBody{
										ListStyle: &parts.DMLTextListStyle{
											Lvl1PPr: &parts.DMLTextStyleLevel{
												DefRPr: &parts.DMLRunPr{FontSize: 4400, SchemeColor: &drawingxml.SchemeClr{Val: "dk1"}},
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
										NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "title", Idx: 0}},
									},
									TxBody: &parts.TextBody{
										ListStyle: &parts.DMLTextListStyle{
											Lvl1PPr: &parts.DMLTextStyleLevel{
												DefRPr: &parts.DMLRunPr{FontSize: 4800, SchemeColor: &drawingxml.SchemeClr{Val: "dk1"}},
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
								DefRPr: &parts.DMLRunPr{FontSize: 4400},
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
						DK1: &parts.ThemeColor{SrgbClr: &parts.SrgbColor{Val: "000000"}},
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

	// Compose the scene
	scene, err := layout.ComposePackage(pkg, layout.ComposeOptions{})
	if err != nil {
		t.Fatalf("ComposePackage failed: %v", err)
	}

	// Verify basic structure
	if scene == nil {
		t.Fatal("Expected scene, got nil")
	}
	if len(scene.Slides) != 1 {
		t.Fatalf("Expected 1 slide, got %d", len(scene.Slides))
	}
	if scene.SlideWidthEMU != 9144000 {
		t.Errorf("Expected slide width 9144000, got %d", scene.SlideWidthEMU)
	}
	if scene.SlideHeightEMU != 6858000 {
		t.Errorf("Expected slide height 6858000, got %d", scene.SlideHeightEMU)
	}

	// Verify slide has shapes
	slide := scene.Slides[0]
	if len(slide.Shapes) == 0 {
		t.Fatal("Expected shapes in slide, got none")
	}

	// Verify shape inheritance
	foundTitle := false
	for _, cs := range slide.Shapes {
		if cs.Shape.Name == "Title" {
			foundTitle = true
			// Check that shape has inherited properties
			if cs.Shape.Placeholder != 1 { // PlaceholderTitle = 1
				t.Errorf("Expected placeholder type Title (1), got %d", cs.Shape.Placeholder)
			}
			// Check that shape has text
			if len(cs.Shape.Paragraphs) == 0 {
				t.Error("Expected paragraphs in title shape")
			}
			break
		}
	}
	if !foundTitle {
		t.Error("Did not find title shape in composed slide")
	}
}

func TestComposePackageTreeOrder(t *testing.T) {
	// Test that shapes are composed in correct tree order
	pkg := &parts.Package{
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
										CNvPr: parts.CnvPr{ID: 1, Name: "Slide Shape 1"},
									},
									SpPr: parts.ShapeProperties{
										Xfrm: &parts.Transform2D{
											Off: parts.Offset{X: 100000, Y: 100000},
											Ext: parts.Extents{CX: 100000, CY: 100000},
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
						DK1: &parts.ThemeColor{SrgbClr: &parts.SrgbColor{Val: "000000"}},
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

	scene, err := layout.ComposePackage(pkg, layout.ComposeOptions{})
	if err != nil {
		t.Fatalf("ComposePackage failed: %v", err)
	}

	if len(scene.Slides) != 1 {
		t.Fatalf("Expected 1 slide, got %d", len(scene.Slides))
	}

	slide := scene.Slides[0]
	if len(slide.Shapes) != 1 {
		t.Fatalf("Expected 1 shape, got %d", len(slide.Shapes))
	}

	// Verify shape source
	cs := slide.Shapes[0]
	if cs.Source != layout.SourceSlide {
		t.Errorf("Expected shape source SourceSlide (%d), got %d", layout.SourceSlide, cs.Source)
	}
	if cs.Shape.Name != "Slide Shape 1" {
		t.Errorf("Expected shape name 'Slide Shape 1', got %s", cs.Shape.Name)
	}
}