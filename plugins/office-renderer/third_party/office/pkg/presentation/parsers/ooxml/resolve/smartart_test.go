package resolve

import (
	"testing"

	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/media"
	"office/pkg/presentation/parsers/ooxml/parts"
)

func TestResolveSmartArtConnectors(t *testing.T) {
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
								<dgm:pt id="root" type="doc"><dgm:prSet><dgm:fld>Root</dgm:fld></dgm:prSet></dgm:pt>
								<dgm:pt id="a"><dgm:prSet><dgm:fld>A</dgm:fld></dgm:prSet></dgm:pt>
								<dgm:pt id="b"><dgm:prSet><dgm:fld>B</dgm:fld></dgm:prSet></dgm:pt>
							</dgm:ptLst>
							<dgm:cxnLst>
								<dgm:cxn srcId="root" destId="a"/>
								<dgm:cxn srcId="root" destId="b"/>
							</dgm:cxnLst>
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

	var nodes, connectors int
	for _, sh := range pres.Slides[0].Shapes {
		switch sh.Kind {
		case ShapeSmartArt:
			nodes++
		case ShapeSmartArtConnector:
			connectors++
			if sh.ConnectorStart == nil || sh.ConnectorEnd == nil {
				t.Fatal("SmartArt connector missing endpoints")
			}
			if sh.LineColor == "" {
				t.Fatal("SmartArt connector missing line color")
			}
		}
	}

	if nodes != 3 {
		t.Fatalf("expected 3 SmartArt nodes, got %d", nodes)
	}
	if connectors != 2 {
		t.Fatalf("expected 2 SmartArt connectors, got %d", connectors)
	}
}

func TestResolveSmartArtNodeIDsAndConnections(t *testing.T) {
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
								<dgm:pt id="1" type="doc"/>
								<dgm:pt id="2"/>
							</dgm:ptLst>
							<dgm:cxnLst>
								<dgm:cxn srcId="1" destId="2"/>
							</dgm:cxnLst>
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
	if len(pres.Slides) != 1 || len(pres.Slides[0].Shapes) != 3 {
		t.Fatalf("expected 1 slide with 3 shapes, got %d shapes", len(pres.Slides[0].Shapes))
	}

	var root, child *Shape
	for _, sh := range pres.Slides[0].Shapes {
		if sh.Kind != ShapeSmartArt {
			continue
		}
		if sh.SmartArtNodeID == "1" {
			root = sh
		} else if sh.SmartArtNodeID == "2" {
			child = sh
		}
	}
	if root == nil {
		t.Fatal("root node missing")
	}
	if child == nil {
		t.Fatal("child node missing")
	}
	if len(root.SmartArtConnections) != 1 || root.SmartArtConnections[0] != "2" {
		t.Fatalf("root connections = %v, want [2]", root.SmartArtConnections)
	}
}

func TestResolveSmartArtHierarchyConnectorPath(t *testing.T) {
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
								<dgm:pt id="root" type="doc"><dgm:prSet><dgm:fld>Root</dgm:fld></dgm:prSet></dgm:pt>
								<dgm:pt id="a"><dgm:prSet><dgm:fld>A</dgm:fld></dgm:prSet></dgm:pt>
							</dgm:ptLst>
							<dgm:cxnLst>
								<dgm:cxn srcId="root" destId="a"/>
							</dgm:cxnLst>
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

	var conn *Shape
	for _, sh := range pres.Slides[0].Shapes {
		if sh.Kind == ShapeSmartArtConnector {
			conn = sh
			break
		}
	}
	if conn == nil {
		t.Fatal("expected a SmartArt connector")
	}
	if len(conn.ConnectorPath) != 4 {
		t.Fatalf("ConnectorPath = %d points, want 4", len(conn.ConnectorPath))
	}
	// For a top-down hierarchy the connector should start at the bottom center
	// of the root and end at the top center of the child (Y increases downward).
	if conn.ConnectorStart.Y >= conn.ConnectorEnd.Y {
		t.Errorf("connector should run downward, startY=%d endY=%d", conn.ConnectorStart.Y, conn.ConnectorEnd.Y)
	}
}

func TestResolveSmartArtBlipFill(t *testing.T) {
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
						DataModelXML: []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
							<dgm:ptLst>
								<dgm:pt id="1" type="doc">
									<dgm:spPr>
										<a:blipFill>
											<a:blip r:embed="rIdImage"/>
											<a:stretch/>
										</a:blipFill>
									</dgm:spPr>
									<dgm:t><a:p><a:r><a:t>Pic</a:t></a:r></a:p></dgm:t>
								</dgm:pt>
							</dgm:ptLst>
						</dgm:dataModel>`),
						MediaParts: map[string]media.Part{
							"rIdImage": {
								Name:        "/ppt/media/image1.png",
								ContentType: "image/png",
								Data:        []byte{0x89, 0x50, 0x4E, 0x47},
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
	if len(pres.Slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(pres.Slides))
	}

	var node *Shape
	for _, sh := range pres.Slides[0].Shapes {
		if sh.Kind == ShapeSmartArt {
			node = sh
			break
		}
	}
	if node == nil {
		t.Fatal("expected a SmartArt node")
	}
	pf, ok := node.Fill.(*drawing.PictureFill)
	if !ok {
		t.Fatalf("fill type = %T, want *drawing.PictureFill", node.Fill)
	}
	if pf.EmbedID != "rIdImage" {
		t.Errorf("embed id = %q, want rIdImage", pf.EmbedID)
	}
	if string(pf.ImageData) != string([]byte{0x89, 0x50, 0x4E, 0x47}) {
		t.Errorf("image data not resolved")
	}
	if pf.ContentType != "image/png" {
		t.Errorf("content type = %q, want image/png", pf.ContentType)
	}
}

func TestResolveSmartArtParagraphTabStops(t *testing.T) {
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
						DataModelXML: []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
							<dgm:ptLst>
								<dgm:pt id="root" type="doc">
									<dgm:t>
										<a:p>
											<a:pPr defTabSz="914400">
												<a:tabLst>
													<a:tab pos="457200" algn="l"/>
													<a:tab pos="914400" algn="ctr"/>
												</a:tabLst>
											</a:pPr>
											<a:r><a:t>left	center</a:t></a:r>
										</a:p>
									</dgm:t>
								</dgm:pt>
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
		t.Fatalf("slides = %d, want 1", len(pres.Slides))
	}
	var node *Shape
	for _, sh := range pres.Slides[0].Shapes {
		if sh.Kind == ShapeSmartArt {
			node = sh
			break
		}
	}
	if node == nil || len(node.Paragraphs) != 1 {
		t.Fatalf("SmartArt paragraph missing: %#v", pres.Slides[0].Shapes)
	}
	para := node.Paragraphs[0]
	if para.DefaultTabSizeEMU != 914400 {
		t.Fatalf("default tab size = %d, want 914400", para.DefaultTabSizeEMU)
	}
	if len(para.TabStops) != 2 {
		t.Fatalf("tab stops = %d, want 2", len(para.TabStops))
	}
	if para.TabStops[0] != (TabStop{Pos: 457200, Algn: "l"}) {
		t.Fatalf("tab stop 0 = %+v", para.TabStops[0])
	}
	if para.TabStops[1] != (TabStop{Pos: 914400, Algn: "ctr"}) {
		t.Fatalf("tab stop 1 = %+v", para.TabStops[1])
	}
}

func TestResolveSmartArtPlaceholderText(t *testing.T) {
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
								<dgm:pt id="1" type="doc">
									<dgm:prSet phldr="1" phldrT="[Text]"/>
								</dgm:pt>
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
	if len(pres.Slides) != 1 || len(pres.Slides[0].Shapes) != 1 {
		t.Fatalf("expected 1 slide with 1 shape, got %d shapes", len(pres.Slides[0].Shapes))
	}
	node := pres.Slides[0].Shapes[0]
	if len(node.Paragraphs) != 1 || len(node.Paragraphs[0].Runs) != 1 {
		t.Fatalf("expected one paragraph with one run, got %+v", node.Paragraphs)
	}
	if node.Paragraphs[0].Runs[0].Text != "[Text]" {
		t.Errorf("text = %q, want [Text]", node.Paragraphs[0].Runs[0].Text)
	}
}

func TestResolveSmartArtConnectorDirections(t *testing.T) {
	for _, tc := range []struct {
		name      string
		dirAttr   string
		wantStart string // "top", "bottom", "left", or "right"
	}{
		{"topDown", "tblr", "bottom"},
		{"bottomUp", "btlr", "top"},
		{"leftRight", "lrtb", "right"},
		{"rightLeft", "rltb", "left"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			layoutXML := `<dgm:layoutDef xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" dir="` + tc.dirAttr + `" uniqueId="hierarchy"/>`
			pptPkg := &parts.Package{
				Presentation: &parts.Presentation{
					SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
				},
				Slides: []*parts.SlideEntry{
					{
						Slide: &parts.Slide{
							CSld: parts.CommonSlideData{
								SpTree: parts.ShapeTree{Shapes: []parts.Shape{}},
							},
						},
						SmartArtParts: []*parts.SmartArtPart{
							{
								DataModelXML: []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram">
									<dgm:ptLst>
										<dgm:pt id="root" type="doc"><dgm:prSet><dgm:fld>Root</dgm:fld></dgm:prSet></dgm:pt>
										<dgm:pt id="a"><dgm:prSet><dgm:fld>A</dgm:fld></dgm:prSet></dgm:pt>
									</dgm:ptLst>
									<dgm:cxnLst>
										<dgm:cxn srcId="root" destId="a"/>
									</dgm:cxnLst>
								</dgm:dataModel>`),
								LayoutXML: []byte(layoutXML),
							},
						},
					},
				},
			}

			pres, err := resolveFixturePackage(pptPkg)
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}

			var conn *Shape
			for _, sh := range pres.Slides[0].Shapes {
				if sh.Kind == ShapeSmartArtConnector {
					conn = sh
					break
				}
			}
			if conn == nil {
				t.Fatal("expected a SmartArt connector")
			}
			if len(conn.ConnectorPath) != 4 {
				t.Fatalf("ConnectorPath = %d points, want 4", len(conn.ConnectorPath))
			}

			start := conn.ConnectorStart
			end := conn.ConnectorEnd
			switch tc.wantStart {
			case "bottom":
				if start.Y >= end.Y {
					t.Errorf("expected downward connector, startY=%v endY=%v", start.Y, end.Y)
				}
			case "top":
				if start.Y <= end.Y {
					t.Errorf("expected upward connector, startY=%v endY=%v", start.Y, end.Y)
				}
			case "right":
				if start.X >= end.X {
					t.Errorf("expected rightward connector, startX=%v endX=%v", start.X, end.X)
				}
			case "left":
				if start.X <= end.X {
					t.Errorf("expected leftward connector, startX=%v endX=%v", start.X, end.X)
				}
			}
		})
	}
}

func TestResolveSmartArtConnectorFromStyleDef(t *testing.T) {
	styleXML := []byte(`<dgm:styleDef xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dgm:styleLbl name="lnNode1">
			<dgm:style>
				<a:ln w="19050">
					<a:solidFill><a:srgbClr val="FF0000"/></a:solidFill>
					<a:headEnd type="triangle" w="med" len="med"/>
				</a:ln>
			</dgm:style>
		</dgm:styleLbl>
	</dgm:styleDef>`)

	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		Slides: []*parts.SlideEntry{
			{
				Slide: &parts.Slide{
					CSld: parts.CommonSlideData{
						SpTree: parts.ShapeTree{Shapes: []parts.Shape{}},
					},
				},
				SmartArtParts: []*parts.SmartArtPart{
					{
						DataModelXML: []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram">
							<dgm:ptLst>
								<dgm:pt id="root" type="doc"><dgm:prSet><dgm:fld>Root</dgm:fld></dgm:prSet></dgm:pt>
								<dgm:pt id="a"><dgm:prSet><dgm:fld>A</dgm:fld></dgm:prSet></dgm:pt>
							</dgm:ptLst>
							<dgm:cxnLst>
								<dgm:cxn srcId="root" destId="a"/>
							</dgm:cxnLst>
						</dgm:dataModel>`),
						StyleXML: styleXML,
					},
				},
			},
		},
	}

	pres, err := resolveFixturePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	var conn *Shape
	for _, sh := range pres.Slides[0].Shapes {
		if sh.Kind == ShapeSmartArtConnector {
			conn = sh
			break
		}
	}
	if conn == nil {
		t.Fatal("expected a SmartArt connector")
	}
	if conn.LineColor != "FF0000" {
		t.Errorf("connector color = %q, want FF0000", conn.LineColor)
	}
	if conn.LineWidthPt != 1.5 {
		t.Errorf("connector width = %v, want 1.5", conn.LineWidthPt)
	}
	if conn.ArrowHead != string(drawing.ArrowheadTriangle) {
		t.Errorf("connector arrow head = %q, want triangle", conn.ArrowHead)
	}
}

func TestResolveSmartArtConnectorDrawingOverridesStyle(t *testing.T) {
	styleXML := []byte(`<dgm:styleDef xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dgm:styleLbl name="lnNode1">
			<dgm:style>
				<a:ln w="19050">
					<a:solidFill><a:srgbClr val="FF0000"/></a:solidFill>
				</a:ln>
			</dgm:style>
		</dgm:styleLbl>
	</dgm:styleDef>`)

	drawingXML := []byte(`<dsp:drawing xmlns:dsp="http://schemas.microsoft.com/office/drawing/2008/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dsp:spTree>
			<dsp:sp modelId="presRoot">
				<dsp:spPr>
					<a:xfrm><a:off x="0" y="0"/><a:ext cx="100000" cy="100000"/></a:xfrm>
					<a:prstGeom prst="rect"/>
				</dsp:spPr>
			</dsp:sp>
			<dsp:sp modelId="presA">
				<dsp:spPr>
					<a:xfrm><a:off x="200000" y="0"/><a:ext cx="100000" cy="100000"/></a:xfrm>
					<a:prstGeom prst="rect"/>
				</dsp:spPr>
			</dsp:sp>
			<dsp:cxnSp>
				<dsp:spPr>
					<a:ln w="25400">
						<a:solidFill><a:srgbClr val="00FF00"/></a:solidFill>
					</a:ln>
				</dsp:spPr>
			</dsp:cxnSp>
		</dsp:spTree>
	</dsp:drawing>`)

	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		Slides: []*parts.SlideEntry{
			{
				Slide: &parts.Slide{
					CSld: parts.CommonSlideData{
						SpTree: parts.ShapeTree{Shapes: []parts.Shape{}},
					},
				},
				SmartArtParts: []*parts.SmartArtPart{
					{
						DataModelXML: []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram">
							<dgm:ptLst>
								<dgm:pt id="root" type="doc"/>
								<dgm:pt id="a"/>
							</dgm:ptLst>
							<dgm:cxnLst>
								<dgm:cxn type="presOf" srcId="root" destId="presRoot"/>
								<dgm:cxn type="presOf" srcId="a" destId="presA"/>
								<dgm:cxn srcId="root" destId="a"/>
							</dgm:cxnLst>
						</dgm:dataModel>`),
						StyleXML:   styleXML,
						DrawingXML: drawingXML,
					},
				},
			},
		},
	}

	pres, err := resolveFixturePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	var conn *Shape
	for _, sh := range pres.Slides[0].Shapes {
		if sh.Kind == ShapeSmartArtConnector {
			conn = sh
			break
		}
	}
	if conn == nil {
		t.Fatal("expected a SmartArt connector")
	}
	if conn.LineColor != "00FF00" {
		t.Errorf("connector color = %q, want 00FF00 (drawing overrides style)", conn.LineColor)
	}
	if conn.LineWidthPt != 2.0 {
		t.Errorf("connector width = %v, want 2.0", conn.LineWidthPt)
	}
}

func TestResolveSmartArtPreRenderedConnectorGeometry(t *testing.T) {
	drawingXML := []byte(`<dsp:drawing xmlns:dsp="http://schemas.microsoft.com/office/drawing/2008/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dsp:spTree>
			<dsp:sp modelId="presRoot">
				<dsp:spPr>
					<a:xfrm><a:off x="0" y="0"/><a:ext cx="100000" cy="100000"/></a:xfrm>
					<a:prstGeom prst="rect"/>
				</dsp:spPr>
			</dsp:sp>
			<dsp:sp modelId="presA">
				<dsp:spPr>
					<a:xfrm><a:off x="200000" y="0"/><a:ext cx="100000" cy="100000"/></a:xfrm>
					<a:prstGeom prst="rect"/>
				</dsp:spPr>
			</dsp:sp>
			<dsp:cxnSp>
				<dsp:spPr>
					<a:xfrm><a:off x="100000" y="50000"/><a:ext cx="100000" cy="0"/></a:xfrm>
					<a:prstGeom prst="curvedConnector2">
						<a:avLst><a:gd name="adj1" fmla="val 50000"/></a:avLst>
					</a:prstGeom>
					<a:ln w="19050"><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></a:ln>
				</dsp:spPr>
			</dsp:cxnSp>
		</dsp:spTree>
	</dsp:drawing>`)

	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{
			SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000},
		},
		Slides: []*parts.SlideEntry{
			{
				Slide: &parts.Slide{
					CSld: parts.CommonSlideData{
						SpTree: parts.ShapeTree{
							Shapes:         []parts.Shape{},
							SmartArtFrames: []parts.SmartArtFrame{{Xfrm: &parts.Transform2D{Off: parts.Offset{X: 0, Y: 0}, Ext: parts.Extents{CX: 400000, CY: 200000}}, RelIDs: map[string]string{"dm": "rId1"}}},
						},
					},
				},
				SmartArtParts: []*parts.SmartArtPart{
					{
						RelID: "rId1",
						DataModelXML: []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram">
							<dgm:ptLst>
								<dgm:pt id="root" type="doc"/>
								<dgm:pt id="a"/>
							</dgm:ptLst>
							<dgm:cxnLst>
								<dgm:cxn type="presOf" srcId="root" destId="presRoot"/>
								<dgm:cxn type="presOf" srcId="a" destId="presA"/>
								<dgm:cxn srcId="root" destId="a"/>
							</dgm:cxnLst>
						</dgm:dataModel>`),
						DrawingXML: drawingXML,
					},
				},
			},
		},
	}

	pres, err := resolveFixturePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	var conn *Shape
	for _, sh := range pres.Slides[0].Shapes {
		if sh.Kind == ShapeSmartArtConnector {
			conn = sh
			break
		}
	}
	if conn == nil {
		t.Fatal("expected a SmartArt connector")
	}
	if conn.ConnectorType != "curvedConnector2" {
		t.Errorf("connector type = %q, want curvedConnector2", conn.ConnectorType)
	}
	if conn.Bounds.W <= 0 || conn.Bounds.H < 0 {
		t.Errorf("connector bounds = %+v, want positive width and non-negative height", conn.Bounds)
	}
	if len(conn.ConnectorAdjustments) != 1 || conn.ConnectorAdjustments[0] != 50000 {
		t.Errorf("connector adjustments = %v, want [50000]", conn.ConnectorAdjustments)
	}
	if conn.LineColor != "FF0000" {
		t.Errorf("connector color = %q, want FF0000", conn.LineColor)
	}
	if conn.LineWidthPt != 1.5 {
		t.Errorf("connector width = %v, want 1.5", conn.LineWidthPt)
	}
	if conn.DrawingOrder == 0 {
		t.Error("expected connector DrawingOrder to be set")
	}
}

func TestPreRenderedSmartArtWithoutConnectorShapesDoesNotSynthesizeLogicalEdges(t *testing.T) {
	// A dsp:drawing is an authoritative visual scene. Logical data-model edges
	// must not turn into painted diagonal connectors when the drawing itself has
	// no dsp:cxnSp records.
	part := &parts.SmartArtPart{
		RelID:        "rId1",
		DataModelXML: []byte(`<?xml version="1.0"?><dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram"><dgm:ptLst><dgm:pt modelId="a"/><dgm:pt modelId="b"/></dgm:ptLst><dgm:cxnLst><dgm:cxn modelId="c" srcId="a" destId="b"/></dgm:cxnLst></dgm:dataModel>`),
		DrawingXML:   []byte(`<?xml version="1.0"?><dsp:drawing xmlns:dsp="http://schemas.microsoft.com/office/drawing/2008/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><dsp:spTree><dsp:sp><dsp:nvSpPr><dsp:cNvPr id="1" name=""/><dsp:cNvSpPr/><dsp:nvPr><dsp:extLst><a:ext uri="x"><dsp:dataModelExt relId="" minVer=""/></a:ext></dsp:extLst></dsp:nvPr></dsp:nvSpPr><dsp:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="1000000" cy="500000"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></dsp:spPr></dsp:sp></dsp:spTree></dsp:drawing>`),
	}
	shapes, _ := ResolveSmartArtPart(part, nil, nil, nil, nil)
	for _, shape := range shapes {
		if shape != nil && shape.Kind == ShapeSmartArtConnector {
			t.Fatalf("pre-rendered drawing without cxnSp synthesized connector: %#v", shape)
		}
	}
}
