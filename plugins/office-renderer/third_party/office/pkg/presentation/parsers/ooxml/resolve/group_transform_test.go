package resolve

import (
	"testing"

	"office/pkg/ooxml/drawing"
	playout "office/pkg/presentation/layout"
	"office/pkg/presentation/parsers/ooxml/parts"
)

func TestGroupTransformScaling(t *testing.T) {
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
											Off:   parts.Offset{X: 1000, Y: 2000},
											Ext:   parts.Extents{CX: 4000, CY: 6000},
											ChOff: parts.Offset{X: 0, Y: 0},
											ChExt: parts.Extents{CX: 2000, CY: 3000},
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

	pres, err := resolveFixturePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(pres.Slides) != 1 || len(pres.Slides[0].Shapes) != 1 {
		t.Fatalf("unexpected shape count")
	}

	groupShape := pres.Slides[0].Shapes[0]
	if len(groupShape.Children) != 1 {
		t.Fatalf("expected 1 child shape, got: %d", len(groupShape.Children))
	}

	childShape := playout.MaterializeGroupChildren(groupShape)[0]
	// scaleX = 4000/2000 = 2, scaleY = 6000/3000 = 2
	// slideX = 1000 + (500 - 0) * 2 = 2000
	// slideY = 2000 + (500 - 0) * 2 = 3000
	// slideW = 1000 * 2 = 2000
	// slideH = 1000 * 2 = 2000
	if childShape.Bounds.X != 2000 {
		t.Errorf("child bounds X = %d, want 2000", childShape.Bounds.X)
	}
	if childShape.Bounds.Y != 3000 {
		t.Errorf("child bounds Y = %d, want 3000", childShape.Bounds.Y)
	}
	if childShape.Bounds.W != 2000 {
		t.Errorf("child bounds W = %d, want 2000", childShape.Bounds.W)
	}
	if childShape.Bounds.H != 2000 {
		t.Errorf("child bounds H = %d, want 2000", childShape.Bounds.H)
	}
}

func TestGroupTransformRotationComposition(t *testing.T) {
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
											Off: parts.Offset{X: 0, Y: 0},
											Ext: parts.Extents{CX: 1000, CY: 1000},
											Rot: 5400000, // 90 degrees
										},
									},
									Shapes: []parts.Shape{
										{
											NvSpPr: parts.NonVisualShapeProps{
												CNvPr: parts.CnvPr{ID: 1, Name: "Child Shape"},
											},
											SpPr: parts.ShapeProperties{
												Xfrm: &parts.Transform2D{
													Off: parts.Offset{X: 0, Y: 0},
													Ext: parts.Extents{CX: 100, CY: 100},
													Rot: 5400000, // 90 degrees
												},
												PrstGeom: &parts.PresetGeometry{Prst: "rect"},
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

	pres, err := resolveFixturePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	childShape := playout.MaterializeGroupChildren(pres.Slides[0].Shapes[0])[0]
	// Child has 90 deg, group has 90 deg, total = 180 deg
	if childShape.RotationDeg != 180.0 {
		t.Errorf("child rotation = %f, want 180.0", childShape.RotationDeg)
	}
}

func TestGroupTransformFlipComposition(t *testing.T) {
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
											Off:   parts.Offset{X: 0, Y: 0},
											Ext:   parts.Extents{CX: 1000, CY: 1000},
											FlipH: true,
										},
									},
									Shapes: []parts.Shape{
										{
											NvSpPr: parts.NonVisualShapeProps{
												CNvPr: parts.CnvPr{ID: 1, Name: "Child Shape"},
											},
											SpPr: parts.ShapeProperties{
												Xfrm: &parts.Transform2D{
													Off:   parts.Offset{X: 0, Y: 0},
													Ext:   parts.Extents{CX: 100, CY: 100},
													FlipH: true,
												},
												PrstGeom: &parts.PresetGeometry{Prst: "rect"},
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

	pres, err := resolveFixturePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	childShape := playout.MaterializeGroupChildren(pres.Slides[0].Shapes[0])[0]
	// Child has FlipH=true, group has FlipH=true, XOR => false
	if childShape.FlipH {
		t.Errorf("child FlipH = true, want false")
	}
}

func TestGroupTransformNestedGroups(t *testing.T) {
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
										CNvPr: parts.CnvPr{ID: 100, Name: "Outer Group"},
									},
									GrpSpPr: &parts.GroupShapeProperties{
										Xfrm: &parts.Transform2D{
											Off:   parts.Offset{X: 100, Y: 200},
											Ext:   parts.Extents{CX: 2000, CY: 2000},
											ChOff: parts.Offset{X: 0, Y: 0},
											ChExt: parts.Extents{CX: 1000, CY: 1000},
										},
									},
									GroupShapes: []parts.GroupShape{
										{
											NvGrpSpPr: &parts.NonVisualGroupShapeProps{
												CNvPr: parts.CnvPr{ID: 101, Name: "Inner Group"},
											},
											GrpSpPr: &parts.GroupShapeProperties{
												Xfrm: &parts.Transform2D{
													Off:   parts.Offset{X: 0, Y: 0},
													Ext:   parts.Extents{CX: 500, CY: 500},
													ChOff: parts.Offset{X: 0, Y: 0},
													ChExt: parts.Extents{CX: 500, CY: 500},
												},
											},
											Shapes: []parts.Shape{
												{
													NvSpPr: parts.NonVisualShapeProps{
														CNvPr: parts.CnvPr{ID: 1, Name: "Deep Child"},
													},
													SpPr: parts.ShapeProperties{
														Xfrm: &parts.Transform2D{
															Off: parts.Offset{X: 50, Y: 50},
															Ext: parts.Extents{CX: 100, CY: 100},
														},
														PrstGeom: &parts.PresetGeometry{Prst: "rect"},
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
			},
		},
	}

	pres, err := resolveFixturePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	outer := pres.Slides[0].Shapes[0]
	if len(outer.Children) != 1 {
		t.Fatalf("expected 1 inner group, got %d", len(outer.Children))
	}
	inner := playout.MaterializeGroupChildren(outer)[0]
	if len(inner.Children) != 1 {
		t.Fatalf("expected 1 deep child, got %d", len(inner.Children))
	}
	deep := inner.Children[0]
	// Outer scale = 2000/1000 = 2; inner scale = 500/500 = 1
	// deep slide X = 100 + (50 - 0) * 2 = 200
	// deep slide Y = 200 + (50 - 0) * 2 = 300
	// deep slide W = 100 * 2 = 200
	// deep slide H = 100 * 2 = 200
	if deep.Bounds.X != 200 {
		t.Errorf("deep bounds X = %d, want 200", deep.Bounds.X)
	}
	if deep.Bounds.Y != 300 {
		t.Errorf("deep bounds Y = %d, want 300", deep.Bounds.Y)
	}
	if deep.Bounds.W != 200 {
		t.Errorf("deep bounds W = %d, want 200", deep.Bounds.W)
	}
	if deep.Bounds.H != 200 {
		t.Errorf("deep bounds H = %d, want 200", deep.Bounds.H)
	}
}

func TestGroupTransformZeroChExt(t *testing.T) {
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
											Off:   parts.Offset{X: 100, Y: 200},
											Ext:   parts.Extents{CX: 1000, CY: 1000},
											ChOff: parts.Offset{X: 0, Y: 0},
											ChExt: parts.Extents{CX: 0, CY: 0},
										},
									},
									Shapes: []parts.Shape{
										{
											NvSpPr: parts.NonVisualShapeProps{
												CNvPr: parts.CnvPr{ID: 1, Name: "Child Shape"},
											},
											SpPr: parts.ShapeProperties{
												Xfrm: &parts.Transform2D{
													Off: parts.Offset{X: 50, Y: 50},
													Ext: parts.Extents{CX: 100, CY: 100},
												},
												PrstGeom: &parts.PresetGeometry{Prst: "rect"},
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

	pres, err := resolveFixturePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	childShape := playout.MaterializeGroupChildren(pres.Slides[0].Shapes[0])[0]
	// scaleX = 1 (ChExt.CX == 0), scaleY = 1 (ChExt.CY == 0)
	// slideX = 100 + (50 - 0) * 1 = 150
	// slideY = 200 + (50 - 0) * 1 = 250
	// slideW = 100 * 1 = 100
	// slideH = 100 * 1 = 100
	if childShape.Bounds.X != 150 {
		t.Errorf("child bounds X = %d, want 150", childShape.Bounds.X)
	}
	if childShape.Bounds.Y != 250 {
		t.Errorf("child bounds Y = %d, want 250", childShape.Bounds.Y)
	}
	if childShape.Bounds.W != 100 {
		t.Errorf("child bounds W = %d, want 100", childShape.Bounds.W)
	}
	if childShape.Bounds.H != 100 {
		t.Errorf("child bounds H = %d, want 100", childShape.Bounds.H)
	}
}

func TestGroupTransformChOffTranslation(t *testing.T) {
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
											Off:   parts.Offset{X: 1000, Y: 1000},
											Ext:   parts.Extents{CX: 2000, CY: 2000},
											ChOff: parts.Offset{X: 100, Y: 100},
											ChExt: parts.Extents{CX: 1000, CY: 1000},
										},
									},
									Shapes: []parts.Shape{
										{
											NvSpPr: parts.NonVisualShapeProps{
												CNvPr: parts.CnvPr{ID: 1, Name: "Child Shape"},
											},
											SpPr: parts.ShapeProperties{
												Xfrm: &parts.Transform2D{
													Off: parts.Offset{X: 200, Y: 200},
													Ext: parts.Extents{CX: 500, CY: 500},
												},
												PrstGeom: &parts.PresetGeometry{Prst: "rect"},
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

	pres, err := resolveFixturePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	childShape := playout.MaterializeGroupChildren(pres.Slides[0].Shapes[0])[0]
	// scaleX = 2000/1000 = 2, scaleY = 2000/1000 = 2
	// slideX = 1000 + (200 - 100) * 2 = 1200
	// slideY = 1000 + (200 - 100) * 2 = 1200
	// slideW = 500 * 2 = 1000
	// slideH = 500 * 2 = 1000
	if childShape.Bounds.X != 1200 {
		t.Errorf("child bounds X = %d, want 1200", childShape.Bounds.X)
	}
	if childShape.Bounds.Y != 1200 {
		t.Errorf("child bounds Y = %d, want 1200", childShape.Bounds.Y)
	}
	if childShape.Bounds.W != 1000 {
		t.Errorf("child bounds W = %d, want 1000", childShape.Bounds.W)
	}
	if childShape.Bounds.H != 1000 {
		t.Errorf("child bounds H = %d, want 1000", childShape.Bounds.H)
	}
}

func TestResolveSmartArtSharedTypes(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		Slides: []*parts.SlideEntry{
			{
				Slide: &parts.Slide{
					CSld: parts.CommonSlideData{
						SpTree: parts.ShapeTree{
							Shapes: []parts.Shape{},
						},
					},
				},
				SmartArtParts: []*parts.SmartArtPart{
					{
						DataModelXML: []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram">
							<dgm:ptLst>
								<dgm:pt id="1" type="root"><dgm:prSet><dgm:fld>CEO</dgm:fld></dgm:prSet></dgm:pt>
							</dgm:ptLst>
						</dgm:dataModel>`),
					},
				},
			},
		},
	}

	pres, err := resolveFixturePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(pres.Slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(pres.Slides))
	}
	if len(pres.Slides[0].Shapes) != 1 {
		t.Fatalf("expected 1 SmartArt shape, got %d", len(pres.Slides[0].Shapes))
	}
	shape := pres.Slides[0].Shapes[0]
	if shape.Kind != ShapeSmartArt {
		t.Fatalf("expected SmartArt kind, got %v", shape.Kind)
	}
	// Verify shared DrawingML types are populated.
	if shape.Fill == nil {
		t.Fatalf("SmartArt shape Fill is nil")
	}
	fill, ok := shape.Fill.(drawing.SolidFill)
	if !ok {
		t.Fatalf("SmartArt fill type = %T, want SolidFill", shape.Fill)
	}
	if fill.Color == "" && fill.ThemeColor == "" {
		t.Fatalf("SmartArt fill has no color")
	}
	if shape.LineStyle == nil {
		t.Fatalf("SmartArt shape LineStyle is nil")
	}
	if shape.LineStyle.Color == "" && shape.LineStyle.ThemeColor == "" {
		t.Fatalf("SmartArt line has no color")
	}
}
