package render

import (
	"encoding/xml"
	"math"
	"testing"

	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/smartart"
)

func TestProcessDiagramWithDrawingPositions(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram">
		<dgm:ptLst>
			<dgm:pt id="dataRoot" type="doc"/>
			<dgm:pt id="dataChild"/>
		</dgm:ptLst>
		<dgm:cxnLst>
			<dgm:cxn id="cxn1" type="presOf" srcId="dataRoot" destId="presRoot"/>
			<dgm:cxn id="cxn2" type="presOf" srcId="dataChild" destId="presChild"/>
			<dgm:cxn id="cxn3" srcId="dataRoot" destId="dataChild"/>
		</dgm:cxnLst>
	</dgm:dataModel>`)

	drawingXML := []byte(`<dsp:drawing xmlns:dsp="http://schemas.microsoft.com/office/drawing/2008/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dsp:spTree>
			<dsp:sp modelId="presRoot">
				<dsp:spPr>
					<a:xfrm><a:off x="0" y="0"/><a:ext cx="100000" cy="100000"/></a:xfrm>
					<a:prstGeom prst="ellipse"/>
					<a:solidFill><a:srgbClr val="FF0000"/></a:solidFill>
				</dsp:spPr>
			</dsp:sp>
			<dsp:sp modelId="presChild">
				<dsp:spPr>
					<a:xfrm><a:off x="200000" y="300000"/><a:ext cx="100000" cy="100000"/></a:xfrm>
					<a:prstGeom prst="rect"/>
					<a:solidFill><a:srgbClr val="00FF00"/></a:solidFill>
				</dsp:spPr>
			</dsp:sp>
		</dsp:spTree>
	</dsp:drawing>`)

	// Frame: 2 inches square at (1in, 1in) => 1828800 EMU per inch.
	frame := &FrameBounds{
		X: 1828800,
		Y: 1828800,
		W: 3657600,
		H: 3657600,
	}

	shapes, err := ProcessDiagramWithOptionsAndFrame(dataModelXML, nil, nil, nil, drawingXML, frame, nil)
	if err != nil {
		t.Fatalf("ProcessDiagramWithOptionsAndFrame: %v", err)
	}

	if len(shapes) != 2 {
		t.Fatalf("shapes = %d, want 2", len(shapes))
	}

	// Drawing bounds: 0..300000 x 0..400000 EMU => 300k x 400k.
	// Frame is 2in x 2in => 3657600 EMU. Aspect ratio of drawing is 0.75,
	// frame is 1.0, so scale = 3657600/400000 = 9.144. Drawing is centered
	// horizontally with offset (3657600 - 300000*9.144)/2 = 0? Actually
	// 300000*9.144 = 2743200, frameW - that = 914400, /2 = 457200.
	// Frame origin in points: 1828800/12700 = 144 pt.
	// Root at drawing (0,0) => frame origin + offset = 144 + 36 = 180 pt.
	// We just verify positions are non-zero and child is offset from root.
	root := shapes[0]
	child := shapes[1]
	if root.X == 0 || root.Y == 0 {
		t.Errorf("root position zero: %v", root)
	}
	if child.X <= root.X || child.Y <= root.Y {
		t.Errorf("child not offset from root: root=%v child=%v", root, child)
	}
	if root.Type != "ellipse" {
		t.Errorf("root geometry = %q, want ellipse", root.Type)
	}
	if child.Type != "rect" {
		t.Errorf("child geometry = %q, want rect", child.Type)
	}
	// Pre-rendered drawing coordinates are EMU, but SmartArtShape coordinates
	// are points. This exact assertion prevents a regression where the EMU
	// drawing envelope was divided into a point frame without unit conversion,
	// collapsing nodes to ~1/12700 of their authored size.
	if math.Abs(root.X-180) > 0.01 || math.Abs(root.Y-144) > 0.01 ||
		math.Abs(root.Width-72) > 0.01 || math.Abs(root.Height-72) > 0.01 {
		t.Fatalf("root fitted geometry = (%.3f, %.3f, %.3f, %.3f), want (180, 144, 72, 72)", root.X, root.Y, root.Width, root.Height)
	}
	if math.Abs(child.X-324) > 0.01 || math.Abs(child.Y-360) > 0.01 {
		t.Fatalf("child fitted origin = (%.3f, %.3f), want (324, 360)", child.X, child.Y)
	}
}

func TestProcessDiagramWithDrawingNoFrame(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram">
		<dgm:ptLst>
			<dgm:pt id="dataRoot" type="doc"/>
		</dgm:ptLst>
		<dgm:cxnLst>
			<dgm:cxn type="presOf" srcId="dataRoot" destId="presRoot"/>
		</dgm:cxnLst>
	</dgm:dataModel>`)

	drawingXML := []byte(`<dsp:drawing xmlns:dsp="http://schemas.microsoft.com/office/drawing/2008/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dsp:spTree>
			<dsp:sp modelId="presRoot">
				<dsp:spPr>
					<a:xfrm><a:off x="127000" y="254000"/><a:ext cx="127000" cy="127000"/></a:xfrm>
				</dsp:spPr>
			</dsp:sp>
		</dsp:spTree>
	</dsp:drawing>`)

	shapes, err := ProcessDiagramWithOptionsAndFrame(dataModelXML, nil, nil, nil, drawingXML, nil, nil)
	if err != nil {
		t.Fatalf("ProcessDiagramWithOptionsAndFrame: %v", err)
	}
	if len(shapes) != 1 {
		t.Fatalf("shapes = %d, want 1", len(shapes))
	}
	if shapes[0].X != 10.0 || shapes[0].Y != 20.0 {
		t.Errorf("position = (%.4f, %.4f), want (10, 20)", shapes[0].X, shapes[0].Y)
	}
	if shapes[0].Width != 10.0 || shapes[0].Height != 10.0 {
		t.Errorf("size = (%.4f, %.4f), want (10, 10)", shapes[0].Width, shapes[0].Height)
	}
}

func TestProcessDiagramWithDrawingSchemeClr(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram">
		<dgm:ptLst>
			<dgm:pt id="dataRoot" type="doc"/>
		</dgm:ptLst>
		<dgm:cxnLst>
			<dgm:cxn type="presOf" srcId="dataRoot" destId="presRoot"/>
		</dgm:cxnLst>
	</dgm:dataModel>`)

	drawingXML := []byte(`<dsp:drawing xmlns:dsp="http://schemas.microsoft.com/office/drawing/2008/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dsp:spTree>
			<dsp:sp modelId="presRoot">
				<dsp:spPr>
					<a:xfrm><a:off x="0" y="0"/><a:ext cx="100000" cy="100000"/></a:xfrm>
					<a:prstGeom prst="rect"/>
					<a:solidFill>
						<a:schemeClr val="accent1">
							<a:alpha val="50000"/>
						</a:schemeClr>
					</a:solidFill>
					<a:ln w="12700">
						<a:solidFill>
							<a:schemeClr val="lt1"/>
						</a:solidFill>
					</a:ln>
				</dsp:spPr>
			</dsp:sp>
		</dsp:spTree>
	</dsp:drawing>`)

	themeColors := map[string]string{
		"accent1": "123456",
		"lt1":     "ABCDEF",
	}

	shapes, err := ProcessDiagramWithOptionsAndFrame(dataModelXML, nil, nil, nil, drawingXML, nil, themeColors)
	if err != nil {
		t.Fatalf("ProcessDiagramWithOptionsAndFrame: %v", err)
	}
	if len(shapes) != 1 {
		t.Fatalf("shapes = %d, want 1", len(shapes))
	}

	s := shapes[0]
	fill, ok := s.Fill.(drawing.SolidFill)
	if !ok {
		t.Fatalf("fill type = %T, want SolidFill", s.Fill)
	}
	if fill.Color != "123456" {
		t.Errorf("fill color = %q, want 123456", fill.Color)
	}
	if fill.Alpha != 0.5 {
		t.Errorf("fill alpha = %v, want 0.5", fill.Alpha)
	}
	if s.Line == nil {
		t.Fatalf("line not parsed")
	}
	if s.Line.Color != "ABCDEF" {
		t.Errorf("line color = %q, want ABCDEF", s.Line.Color)
	}
	if s.Line.Width != 1.0 {
		t.Errorf("line width = %v, want 1.0", s.Line.Width)
	}
}

func TestProcessDiagramWithDrawingGradientFill(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram">
		<dgm:ptLst>
			<dgm:pt id="dataRoot" type="doc"/>
		</dgm:ptLst>
		<dgm:cxnLst>
			<dgm:cxn type="presOf" srcId="dataRoot" destId="presRoot"/>
		</dgm:cxnLst>
	</dgm:dataModel>`)

	drawingXML := []byte(`<dsp:drawing xmlns:dsp="http://schemas.microsoft.com/office/drawing/2008/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dsp:spTree>
			<dsp:sp modelId="presRoot">
				<dsp:spPr>
					<a:xfrm><a:off x="0" y="0"/><a:ext cx="100000" cy="100000"/></a:xfrm>
					<a:prstGeom prst="rect"/>
					<a:gradFill flip="none" rotWithShape="1">
						<a:gsLst>
							<a:gs pos="0">
								<a:srgbClr val="FF0000"/>
							</a:gs>
							<a:gs pos="100000">
								<a:schemeClr val="accent1">
									<a:alpha val="50000"/>
								</a:schemeClr>
							</a:gs>
						</a:gsLst>
						<a:lin ang="2700000" scaled="1"/>
					</a:gradFill>
				</dsp:spPr>
			</dsp:sp>
		</dsp:spTree>
	</dsp:drawing>`)

	themeColors := map[string]string{
		"accent1": "0000FF",
	}

	shapes, err := ProcessDiagramWithOptionsAndFrame(dataModelXML, nil, nil, nil, drawingXML, nil, themeColors)
	if err != nil {
		t.Fatalf("ProcessDiagramWithOptionsAndFrame: %v", err)
	}
	if len(shapes) != 1 {
		t.Fatalf("shapes = %d, want 1", len(shapes))
	}

	grad, ok := shapes[0].Fill.(drawing.GradientFill)
	if !ok {
		t.Fatalf("fill type = %T, want GradientFill", shapes[0].Fill)
	}
	if grad.Angle != 45.0 {
		t.Errorf("angle = %v, want 45.0", grad.Angle)
	}
	if len(grad.Stops) != 2 {
		t.Fatalf("stops = %d, want 2", len(grad.Stops))
	}
	if grad.Stops[0].Color != "FF0000" || grad.Stops[0].Position != 0.0 {
		t.Errorf("stop0 = %+v, want FF0000 at 0.0", grad.Stops[0])
	}
	if grad.Stops[1].Color != "0000FF" || grad.Stops[1].Position != 1.0 || grad.Stops[1].Alpha != 0.5 {
		t.Errorf("stop1 = %+v, want 0000FF at 1.0 alpha 0.5", grad.Stops[1])
	}
}

func TestProcessDiagramWithDrawingTextFormatting(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram">
		<dgm:ptLst>
			<dgm:pt id="dataRoot" type="doc"/>
		</dgm:ptLst>
		<dgm:cxnLst>
			<dgm:cxn type="presOf" srcId="dataRoot" destId="presRoot"/>
		</dgm:cxnLst>
	</dgm:dataModel>`)

	drawingXML := []byte(`<dsp:drawing xmlns:dsp="http://schemas.microsoft.com/office/drawing/2008/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dsp:spTree>
			<dsp:sp modelId="presRoot">
				<dsp:spPr>
					<a:xfrm><a:off x="0" y="0"/><a:ext cx="100000" cy="100000"/></a:xfrm>
					<a:prstGeom prst="rect"/>
				</dsp:spPr>
				<dsp:txBody>
					<a:bodyPr/>
					<a:lstStyle/>
					<a:p>
						<a:pPr algn="l"/>
						<a:r>
							<a:rPr b="1" sz="1100"/>
							<a:t>bold</a:t>
						</a:r>
						<a:r>
							<a:rPr i="1" u="sng">
								<a:solidFill>
									<a:schemeClr val="accent1"/>
								</a:solidFill>
							</a:rPr>
							<a:t>italic</a:t>
						</a:r>
					</a:p>
				</dsp:txBody>
			</dsp:sp>
		</dsp:spTree>
	</dsp:drawing>`)

	themeColors := map[string]string{
		"accent1": "FF0000",
	}

	shapes, err := ProcessDiagramWithOptionsAndFrame(dataModelXML, nil, nil, nil, drawingXML, nil, themeColors)
	if err != nil {
		t.Fatalf("ProcessDiagramWithOptionsAndFrame: %v", err)
	}
	if len(shapes) != 1 {
		t.Fatalf("shapes = %d, want 1", len(shapes))
	}

	s := shapes[0]
	if len(s.Paragraphs) != 1 {
		t.Fatalf("paragraphs = %d, want 1", len(s.Paragraphs))
	}
	if s.Paragraphs[0].Align != "l" {
		t.Errorf("align = %q, want l", s.Paragraphs[0].Align)
	}
	runs := s.Paragraphs[0].Runs
	if len(runs) != 2 {
		t.Fatalf("runs = %d, want 2", len(runs))
	}
	if !runs[0].Bold || runs[0].Italic || runs[0].FontSize != 1100 || runs[0].Text != "bold" {
		t.Errorf("run0 = %+v", runs[0])
	}
	if runs[1].Bold || !runs[1].Italic || !runs[1].Underline || runs[1].Color != "FF0000" || runs[1].Text != "italic" {
		t.Errorf("run1 = %+v", runs[1])
	}
	if s.Text != "bolditalic" {
		t.Errorf("plain Text = %q, want bolditalic", s.Text)
	}
}

func TestProcessDiagramWithDrawingBlipFill(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram">
		<dgm:ptLst>
			<dgm:pt id="dataRoot" type="doc"/>
		</dgm:ptLst>
		<dgm:cxnLst>
			<dgm:cxn type="presOf" srcId="dataRoot" destId="presRoot"/>
		</dgm:cxnLst>
	</dgm:dataModel>`)

	drawingXML := []byte(`<dsp:drawing xmlns:dsp="http://schemas.microsoft.com/office/drawing/2008/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
		<dsp:spTree>
			<dsp:sp modelId="presRoot">
				<dsp:spPr>
					<a:xfrm><a:off x="0" y="0"/><a:ext cx="100000" cy="100000"/></a:xfrm>
					<a:prstGeom prst="rect"/>
					<a:blipFill>
						<a:blip r:embed="rIdImage"/>
						<a:stretch/>
					</a:blipFill>
				</dsp:spPr>
			</dsp:sp>
		</dsp:spTree>
	</dsp:drawing>`)

	shapes, err := ProcessDiagramWithOptionsAndFrame(dataModelXML, nil, nil, nil, drawingXML, nil, nil)
	if err != nil {
		t.Fatalf("ProcessDiagramWithOptionsAndFrame: %v", err)
	}
	if len(shapes) != 1 {
		t.Fatalf("shapes = %d, want 1", len(shapes))
	}
	pf, ok := shapes[0].Fill.(*drawing.PictureFill)
	if !ok {
		t.Fatalf("fill type = %T, want *drawing.PictureFill", shapes[0].Fill)
	}
	if pf.EmbedID != "rIdImage" {
		t.Errorf("embed id = %q, want rIdImage", pf.EmbedID)
	}
	if !pf.Stretch {
		t.Errorf("stretch = false, want true")
	}
}

func TestProcessDiagramWithDrawingBodyPr(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram">
		<dgm:ptLst>
			<dgm:pt id="dataRoot" type="doc"/>
		</dgm:ptLst>
		<dgm:cxnLst>
			<dgm:cxn type="presOf" srcId="dataRoot" destId="presRoot"/>
		</dgm:cxnLst>
	</dgm:dataModel>`)

	drawingXML := []byte(`<dsp:drawing xmlns:dsp="http://schemas.microsoft.com/office/drawing/2008/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dsp:spTree>
			<dsp:sp modelId="presRoot">
				<dsp:spPr>
					<a:xfrm><a:off x="0" y="0"/><a:ext cx="100000" cy="100000"/></a:xfrm>
					<a:prstGeom prst="rect"/>
				</dsp:spPr>
				<dsp:txBody>
					<a:bodyPr anchor="t" anchorCtr="0" rot="900000" wrap="square" vert="horz" lIns="0" rIns="0" tIns="0" bIns="0"/>
					<a:lstStyle/>
					<a:p><a:r><a:t>top</a:t></a:r></a:p>
				</dsp:txBody>
			</dsp:sp>
		</dsp:spTree>
	</dsp:drawing>`)

	shapes, err := ProcessDiagramWithOptionsAndFrame(dataModelXML, nil, nil, nil, drawingXML, nil, nil)
	if err != nil {
		t.Fatalf("ProcessDiagramWithOptionsAndFrame: %v", err)
	}
	if len(shapes) != 1 {
		t.Fatalf("shapes = %d, want 1", len(shapes))
	}
	bp := shapes[0].TextBodyProps
	if bp.Anchor != "t" || bp.AnchorCtr || bp.Rot != 900000 || bp.Wrap != "square" || bp.Vert != "horz" {
		t.Errorf("body pr = %+v", bp)
	}
	if bp.InsetL != 0 || bp.InsetR != 0 || bp.InsetT != 0 || bp.InsetB != 0 {
		t.Errorf("insets = %+v", bp)
	}
}

func TestProcessDiagramWithDrawingParagraphTabStops(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram">
		<dgm:ptLst>
			<dgm:pt id="dataRoot" type="doc"/>
		</dgm:ptLst>
		<dgm:cxnLst>
			<dgm:cxn type="presOf" srcId="dataRoot" destId="presRoot"/>
		</dgm:cxnLst>
	</dgm:dataModel>`)

	drawingXML := []byte(`<dsp:drawing xmlns:dsp="http://schemas.microsoft.com/office/drawing/2008/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dsp:spTree>
			<dsp:sp modelId="presRoot">
				<dsp:spPr>
					<a:xfrm><a:off x="0" y="0"/><a:ext cx="100000" cy="100000"/></a:xfrm>
					<a:prstGeom prst="rect"/>
				</dsp:spPr>
				<dsp:txBody>
					<a:bodyPr/>
					<a:lstStyle/>
					<a:p>
						<a:pPr defTabSz="914400">
							<a:tabLst>
								<a:tab pos="457200" algn="l"/>
								<a:tab pos="914400" algn="ctr"/>
							</a:tabLst>
						</a:pPr>
						<a:r><a:t>left	center</a:t></a:r>
					</a:p>
				</dsp:txBody>
			</dsp:sp>
		</dsp:spTree>
	</dsp:drawing>`)

	shapes, err := ProcessDiagramWithOptionsAndFrame(dataModelXML, nil, nil, nil, drawingXML, nil, nil)
	if err != nil {
		t.Fatalf("ProcessDiagramWithOptionsAndFrame: %v", err)
	}
	if len(shapes) != 1 || len(shapes[0].Paragraphs) != 1 {
		t.Fatalf("paragraphs = %#v", shapes)
	}
	para := shapes[0].Paragraphs[0]
	if para.DefaultTabSizeEMU != 914400 {
		t.Fatalf("default tab size = %d, want 914400", para.DefaultTabSizeEMU)
	}
	if len(para.TabStops) != 2 {
		t.Fatalf("tab stops = %d, want 2", len(para.TabStops))
	}
	if para.TabStops[0] != (smartart.TabStop{Pos: 457200, Algn: "l"}) {
		t.Fatalf("tab stop 0 = %+v", para.TabStops[0])
	}
	if para.TabStops[1] != (smartart.TabStop{Pos: 914400, Algn: "ctr"}) {
		t.Fatalf("tab stop 1 = %+v", para.TabStops[1])
	}
}

func TestProcessDiagramRendersBackgroundAndWholeEffects(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dgm:ptLst>
			<dgm:pt id="root" type="doc"><dgm:t><a:p><a:r><a:t>Root</a:t></a:r></a:p></dgm:t></dgm:pt>
		</dgm:ptLst>
		<dgm:bg><a:bgClr><a:srgbClr val="FF0000"/></a:bgClr></dgm:bg>
		<dgm:whole><a:effectLst><a:glow rad="12700"><a:srgbClr val="00FF00"/></a:glow></a:effectLst></dgm:whole>
	</dgm:dataModel>`)

	shapes, err := ProcessDiagramWithOptionsAndFrame(dataModelXML, nil, nil, nil, nil, &FrameBounds{X: 0, Y: 0, W: 1270000, H: 635000}, nil)
	if err != nil {
		t.Fatalf("ProcessDiagramWithOptionsAndFrame: %v", err)
	}
	if len(shapes) != 2 {
		t.Fatalf("shapes = %d, want background + node", len(shapes))
	}
	if shapes[0].ID != "__background" {
		t.Fatalf("first shape ID = %q, want background", shapes[0].ID)
	}
	fill, ok := shapes[0].Fill.(drawing.SolidFill)
	if !ok || fill.Color != "FF0000" {
		t.Fatalf("background fill = %#v", shapes[0].Fill)
	}
	if len(shapes[1].Effect.Effects) != 1 || shapes[1].Effect.Effects[0].Type() != drawing.EffectTypeGlow {
		t.Fatalf("node whole effects = %#v", shapes[1].Effect)
	}
}

func TestParseDiagramDrawingConnectorStyle(t *testing.T) {
	drawingXML := []byte(`<dsp:drawing xmlns:dsp="http://schemas.microsoft.com/office/drawing/2008/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dsp:spTree>
			<dsp:cxnSp>
				<dsp:spPr>
					<a:ln w="19050" cap="flat" cmpd="sng">
						<a:solidFill><a:srgbClr val="FF0000"/></a:solidFill>
						<a:prstDash val="dash"/>
						<a:headEnd type="triangle" w="med" len="med"/>
						<a:tailEnd type="arrow" w="lg" len="lg"/>
					</a:ln>
				</dsp:spPr>
			</dsp:cxnSp>
		</dsp:spTree>
	</dsp:drawing>`)

	_, connStyle, err := parseDiagramDrawing(drawingXML, nil)
	if err != nil {
		t.Fatalf("parseDiagramDrawing: %v", err)
	}
	if connStyle == nil {
		t.Fatal("expected connector style")
	}
	if connStyle.Color != "FF0000" || connStyle.Width != 1.5 {
		t.Errorf("line = %+v", connStyle)
	}
	if connStyle.Dash != drawing.DashPatternDash {
		t.Errorf("dash = %v, want Dash", connStyle.Dash)
	}
	if connStyle.HeadEnd == nil || connStyle.HeadEnd.Type != drawing.ArrowheadTriangle {
		t.Errorf("head end = %+v", connStyle.HeadEnd)
	}
	if connStyle.TailEnd == nil || connStyle.TailEnd.Type != drawing.ArrowheadArrow {
		t.Errorf("tail end = %+v", connStyle.TailEnd)
	}
}

func TestParseDiagramDrawingPatternFill(t *testing.T) {
	drawingXML := []byte(`<dsp:drawing xmlns:dsp="http://schemas.microsoft.com/office/drawing/2008/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dsp:spTree>
			<dsp:sp modelId="p1">
				<dsp:spPr>
					<a:xfrm><a:off x="0" y="0"/><a:ext cx="100000" cy="100000"/></a:xfrm>
					<a:prstGeom prst="rect"/>
					<a:pattFill prst="diagBrick">
						<a:fgClr><a:srgbClr val="FF0000"><a:alphaMod val="50000"/></a:srgbClr></a:fgClr>
						<a:bgClr><a:schemeClr val="accent1"/></a:bgClr>
					</a:pattFill>
				</dsp:spPr>
			</dsp:sp>
		</dsp:spTree>
	</dsp:drawing>`)

	shapes, _, err := parseDiagramDrawing(drawingXML, map[string]string{"accent1": "00FF00"})
	if err != nil {
		t.Fatalf("parseDiagramDrawing: %v", err)
	}
	info, ok := shapes["p1"]
	if !ok {
		t.Fatalf("missing shape p1")
	}
	pf, ok := info.Fill.(drawing.PatternFill)
	if !ok {
		t.Fatalf("fill = %T, want PatternFill", info.Fill)
	}
	if string(pf.PatternType) != "diagBrick" {
		t.Errorf("pattern type = %q, want diagBrick", pf.PatternType)
	}
	if pf.Foreground != "FF0000" || pf.ForeAlpha != 0.5 {
		t.Errorf("foreground = %q alpha=%v, want FF0000/0.5", pf.Foreground, pf.ForeAlpha)
	}
	if pf.Background != "00FF00" {
		t.Errorf("background = %q, want 00FF00", pf.Background)
	}
}

func TestParseDiagramDrawingAlphaModOff(t *testing.T) {
	drawingXML := []byte(`<dsp:drawing xmlns:dsp="http://schemas.microsoft.com/office/drawing/2008/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dsp:spTree>
			<dsp:sp modelId="p2">
				<dsp:spPr>
					<a:xfrm><a:off x="0" y="0"/><a:ext cx="100000" cy="100000"/></a:xfrm>
					<a:prstGeom prst="rect"/>
					<a:solidFill><a:schemeClr val="accent1"><a:alpha val="80000"/><a:alphaMod val="50000"/><a:alphaOff val="10000"/></a:schemeClr></a:solidFill>
				</dsp:spPr>
			</dsp:sp>
		</dsp:spTree>
	</dsp:drawing>`)

	shapes, _, err := parseDiagramDrawing(drawingXML, map[string]string{"accent1": "0000FF"})
	if err != nil {
		t.Fatalf("parseDiagramDrawing: %v", err)
	}
	sf, ok := shapes["p2"].Fill.(drawing.SolidFill)
	if !ok {
		t.Fatalf("fill = %T, want SolidFill", shapes["p2"].Fill)
	}
	if sf.Alpha != 0.5 {
		t.Errorf("alpha = %v, want 0.5", sf.Alpha)
	}
}

func TestParseDiagramDrawingEffectList(t *testing.T) {
	drawingXML := []byte(`<dsp:drawing xmlns:dsp="http://schemas.microsoft.com/office/drawing/2008/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dsp:spTree>
			<dsp:sp modelId="p1">
				<dsp:spPr>
					<a:xfrm><a:off x="0" y="0"/><a:ext cx="100000" cy="100000"/></a:xfrm>
					<a:prstGeom prst="rect"/>
					<a:effectLst>
						<a:outerShdw blurRad="19050" dist="19050" dir="2700000">
							<a:srgbClr val="FF0000"><a:alpha val="50000"/></a:srgbClr>
						</a:outerShdw>
						<a:glow rad="12700"><a:srgbClr val="00FF00"/></a:glow>
						<a:softEdge rad="6350"/>
					</a:effectLst>
				</dsp:spPr>
			</dsp:sp>
		</dsp:spTree>
	</dsp:drawing>`)

	shapes, _, err := parseDiagramDrawing(drawingXML, nil)
	if err != nil {
		t.Fatalf("parseDiagramDrawing: %v", err)
	}
	info, ok := shapes["p1"]
	if !ok {
		t.Fatalf("missing shape p1")
	}
	if len(info.Effect.Effects) != 3 {
		t.Fatalf("effects = %d, want 3", len(info.Effect.Effects))
	}
	shadow, ok := info.Effect.Effects[0].(drawing.Shadow)
	if !ok {
		t.Fatalf("first effect = %T, want Shadow", info.Effect.Effects[0])
	}
	if shadow.Color != "FF0000" || shadow.Alpha != 0.5 || shadow.Distance != 1.5 {
		t.Errorf("shadow = %+v", shadow)
	}
	glow, ok := info.Effect.Effects[1].(drawing.Glow)
	if !ok || glow.Color != "00FF00" || glow.Radius != 1.0 {
		t.Errorf("glow = %+v", info.Effect.Effects[1])
	}
	soft, ok := info.Effect.Effects[2].(drawing.SoftEdge)
	if !ok || soft.Radius != 0.5 {
		t.Errorf("softEdge = %+v", info.Effect.Effects[2])
	}
}

func TestParseDiagramDrawingThreeD(t *testing.T) {
	drawingXML := []byte(`<dsp:drawing xmlns:dsp="http://schemas.microsoft.com/office/drawing/2008/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dsp:spTree>
			<dsp:sp modelId="p1">
				<dsp:spPr>
					<a:xfrm><a:off x="0" y="0"/><a:ext cx="100000" cy="100000"/></a:xfrm>
					<a:prstGeom prst="rect"/>
					<a:scene3d>
						<a:camera prst="perspectiveRelaxed" fov="2700000" zoom="125000">
							<a:rot lat="300000" lon="600000" rev="0"/>
						</a:camera>
						<a:lightRig rig="threePt" dir="t"><a:rot lat="120000" lon="240000" rev="360000"/></a:lightRig>
					</a:scene3d>
					<a:sp3d extrusionH="914400" prstMaterial="plastic">
						<a:bevelT w="19050" h="19050" prst="softRound"/>
					</a:sp3d>
				</dsp:spPr>
			</dsp:sp>
		</dsp:spTree>
	</dsp:drawing>`)

	shapes, _, err := parseDiagramDrawing(drawingXML, nil)
	if err != nil {
		t.Fatalf("parseDiagramDrawing: %v", err)
	}
	info := shapes["p1"]
	threeD, ok := info.Effect.Effects[0].(drawing.ThreeD)
	if !ok {
		t.Fatalf("effect = %T, want ThreeD", info.Effect.Effects[0])
	}
	if threeD.CameraPreset != "perspectiveRelaxed" {
		t.Errorf("camera = %q, want perspectiveRelaxed", threeD.CameraPreset)
	}
	if threeD.LightRig != "threePt" {
		t.Errorf("light rig = %q, want threePt", threeD.LightRig)
	}
	if threeD.CameraFOV != 45 || threeD.CameraZoom != 1.25 {
		t.Errorf("camera metadata = (fov=%v zoom=%v), want (45, 1.25)", threeD.CameraFOV, threeD.CameraZoom)
	}
	if threeD.LightRotationX != 2 || threeD.LightRotationY != 4 || threeD.LightRotationZ != 6 {
		t.Errorf("light rotation = (%v,%v,%v), want (2,4,6)", threeD.LightRotationX, threeD.LightRotationY, threeD.LightRotationZ)
	}
	if threeD.RotationX != 5.0 || threeD.RotationY != 10.0 {
		t.Errorf("rotation = (%v, %v), want (5, 10)", threeD.RotationX, threeD.RotationY)
	}
	if threeD.ExtrusionDepth != 72.0 {
		t.Errorf("extrusion = %v, want 72", threeD.ExtrusionDepth)
	}
	if threeD.BevelTopWidth != 1.5 || threeD.BevelTopHeight != 1.5 {
		t.Errorf("bevel = (%v, %v), want (1.5, 1.5)", threeD.BevelTopWidth, threeD.BevelTopHeight)
	}
	if threeD.BevelTopPreset != "softRound" {
		t.Errorf("bevel preset = %q, want softRound", threeD.BevelTopPreset)
	}
}

func TestParseDiagramDrawingGradientLine(t *testing.T) {
	drawingXML := []byte(`<dsp:drawing xmlns:dsp="http://schemas.microsoft.com/office/drawing/2008/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dsp:spTree>
			<dsp:sp modelId="p1">
				<dsp:spPr>
					<a:xfrm><a:off x="0" y="0"/><a:ext cx="100000" cy="100000"/></a:xfrm>
					<a:prstGeom prst="rect"/>
					<a:ln w="19050">
						<a:gradFill>
							<a:gsLst>
								<a:gs pos="0"><a:srgbClr val="FF0000"/></a:gs>
								<a:gs pos="100000"><a:srgbClr val="0000FF"/></a:gs>
							</a:gsLst>
						</a:gradFill>
					</a:ln>
				</dsp:spPr>
			</dsp:sp>
		</dsp:spTree>
	</dsp:drawing>`)

	shapes, _, err := parseDiagramDrawing(drawingXML, nil)
	if err != nil {
		t.Fatalf("parseDiagramDrawing: %v", err)
	}
	info := shapes["p1"]
	if info.Line == nil {
		t.Fatalf("no line parsed")
	}
	if info.Line.Fill == nil {
		t.Fatalf("line.Fill = nil, want GradientFill")
	}
	gf, ok := info.Line.Fill.(drawing.GradientFill)
	if !ok {
		t.Fatalf("line.Fill = %T, want GradientFill", info.Line.Fill)
	}
	if len(gf.Stops) != 2 || gf.Stops[0].Color != "FF0000" || gf.Stops[1].Color != "0000FF" {
		t.Errorf("stops = %+v", gf.Stops)
	}
}

func TestProcessDiagramWithCustomGeometry(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram">
		<dgm:ptLst>
			<dgm:pt id="p1" type="doc"/>
		</dgm:ptLst>
		<dgm:cxnLst>
			<dgm:cxn id="c1" type="presOf" srcId="p1" destId="p1"/>
		</dgm:cxnLst>
	</dgm:dataModel>`)

	drawingXML := []byte(`<dsp:drawing xmlns:dsp="http://schemas.microsoft.com/office/drawing/2008/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dsp:spTree>
			<dsp:sp modelId="p1">
				<dsp:spPr>
					<a:xfrm><a:off x="0" y="0"/><a:ext cx="100000" cy="100000"/></a:xfrm>
					<a:custGeom>
						<a:pathLst>
							<a:path w="100000" h="100000">
								<a:moveTo><a:pt x="0" y="100000"/></a:moveTo>
								<a:lnTo><a:pt x="50000" y="0"/></a:lnTo>
								<a:lnTo><a:pt x="100000" y="100000"/></a:lnTo>
								<a:close/>
							</a:path>
						</a:pathLst>
					</a:custGeom>
				</dsp:spPr>
			</dsp:sp>
		</dsp:spTree>
	</dsp:drawing>`)

	shapes, err := ProcessDiagramWithOptionsAndFrame(dataModelXML, nil, nil, nil, drawingXML, nil, nil)
	if err != nil {
		t.Fatalf("ProcessDiagramWithOptionsAndFrame: %v", err)
	}
	if len(shapes) != 1 {
		t.Fatalf("shapes = %d, want 1", len(shapes))
	}
	cg := shapes[0].CustomGeometry
	if cg == nil {
		t.Fatalf("custom geometry not parsed")
	}
	if cg.Type != drawing.GeometryTypeCustom {
		t.Errorf("geometry type = %v, want custom", cg.Type)
	}
	if len(cg.Paths) != 1 {
		t.Errorf("paths = %d, want 1", len(cg.Paths))
	}
}

func TestProcessDiagramWithPictureFillCropAndEffects(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram">
		<dgm:ptLst>
			<dgm:pt id="p1" type="doc"/>
		</dgm:ptLst>
		<dgm:cxnLst>
			<dgm:cxn id="c1" type="presOf" srcId="p1" destId="p1"/>
		</dgm:cxnLst>
	</dgm:dataModel>`)

	drawingXML := []byte(`<dsp:drawing xmlns:dsp="http://schemas.microsoft.com/office/drawing/2008/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
		<dsp:spTree>
			<dsp:sp modelId="p1">
				<dsp:spPr>
					<a:xfrm><a:off x="0" y="0"/><a:ext cx="100000" cy="100000"/></a:xfrm>
					<a:blipFill dpi="192" rotWithShape="0">
						<a:blip r:embed="rId1">
							<a:grayscl/>
							<a:lum bright="50000" contrast="10000"/>
						</a:blip>
						<a:srcRect l="10000" t="20000" r="30000" b="40000"/>
						<a:stretch><a:fillRect l="5000" t="10000" r="15000" b="20000"/></a:stretch>
					</a:blipFill>
				</dsp:spPr>
			</dsp:sp>
		</dsp:spTree>
	</dsp:drawing>`)

	shapes, err := ProcessDiagramWithOptionsAndFrame(dataModelXML, nil, nil, nil, drawingXML, nil, nil)
	if err != nil {
		t.Fatalf("ProcessDiagramWithOptionsAndFrame: %v", err)
	}
	if len(shapes) != 1 {
		t.Fatalf("shapes = %d, want 1", len(shapes))
	}
	pf, ok := shapes[0].Fill.(*drawing.PictureFill)
	if !ok {
		t.Fatalf("fill type = %T, want PictureFill", shapes[0].Fill)
	}
	if pf.EmbedID != "rId1" {
		t.Errorf("embed id = %q, want rId1", pf.EmbedID)
	}
	if pf.DPI != 192 || !pf.RotWithShapeSet || pf.RotWithShape {
		t.Fatalf("geometry metadata = dpi=%d rotSet=%v rot=%v", pf.DPI, pf.RotWithShapeSet, pf.RotWithShape)
	}
	if pf.FillRect == nil || math.Abs(pf.FillRect.X-0.05) > 1e-9 || math.Abs(pf.FillRect.Y-0.1) > 1e-9 || math.Abs(pf.FillRect.Width-0.8) > 1e-9 || math.Abs(pf.FillRect.Height-0.7) > 1e-9 {
		t.Fatalf("fill rect = %#v", pf.FillRect)
	}
	if pf.Crop == nil {
		t.Fatal("crop not parsed")
	}
	if math.Abs(pf.Crop.X-0.1) > 1e-9 || math.Abs(pf.Crop.Y-0.2) > 1e-9 || math.Abs(pf.Crop.Width-0.6) > 1e-9 || math.Abs(pf.Crop.Height-0.4) > 1e-9 {
		t.Errorf("crop = %+v, want left=0.1 top=0.2 width=0.6 height=0.4", pf.Crop)
	}
	if len(pf.BlipEffects) != 2 {
		t.Fatalf("effects = %d, want 2", len(pf.BlipEffects))
	}
	if pf.BlipEffects[0].Type() != drawing.EffectTypeGrayscale {
		t.Errorf("first effect = %v, want grayscale", pf.BlipEffects[0].Type())
	}
	lum, ok := pf.BlipEffects[1].(drawing.Lum)
	if !ok {
		t.Fatalf("second effect = %T, want Lum", pf.BlipEffects[1])
	}
	if lum.Brightness != 50000 || lum.Contrast != 10000 {
		t.Errorf("lum = %+v", lum)
	}
}

func TestDrawingEffectListUsesRichSharedParser(t *testing.T) {
	const src = `<effectLst xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:outerShdw blurRad="12700" dist="25400" dir="5400000" sx="120000" sy="80000" algn="br" rotWithShape="1"><a:schemeClr val="accent1"/></a:outerShdw>
		<a:blur rad="25400" grow="1"/>
		<a:fillOverlay blend="screen"><a:solidFill><a:schemeClr val="accent2"/></a:solidFill></a:fillOverlay>
	</effectLst>`
	var parsed drawingEffectList
	if err := xml.Unmarshal([]byte(src), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	effects := drawingEffectListToEffects(&parsed, map[string]string{"accent1": "123456", "accent2": "ABCDEF"})
	if len(effects.Effects) != 3 {
		t.Fatalf("effects = %d, want 3", len(effects.Effects))
	}
	shadow, ok := effects.Effects[0].(drawing.Shadow)
	if !ok || shadow.Color != "123456" || shadow.ScaleX != 1.2 || shadow.ScaleY != 0.8 || shadow.Alignment != "br" || !shadow.RotateWithShape {
		t.Fatalf("shadow = %#v", effects.Effects[0])
	}
	blur, ok := effects.Effects[1].(drawing.Blur)
	if !ok || !blur.Grow || blur.Radius != 2 {
		t.Fatalf("blur = %#v", effects.Effects[1])
	}
	overlay, ok := effects.Effects[2].(drawing.FillOverlay)
	if !ok || overlay.BlendMode != "screen" {
		t.Fatalf("overlay = %#v", effects.Effects[2])
	}
	solid, ok := overlay.Fill.(drawing.SolidFill)
	if !ok || solid.Color != "ABCDEF" {
		t.Fatalf("overlay fill = %#v", overlay.Fill)
	}
}

func TestProcessDiagramPreservesExtendedPictureBlipEffects(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram"><dgm:ptLst><dgm:pt id="p1" type="doc"/></dgm:ptLst><dgm:cxnLst><dgm:cxn id="c1" type="presOf" srcId="p1" destId="p1"/></dgm:cxnLst></dgm:dataModel>`)
	drawingXML := []byte(`<dsp:drawing xmlns:dsp="http://schemas.microsoft.com/office/drawing/2008/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><dsp:spTree><dsp:sp modelId="p1"><dsp:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="100000" cy="100000"/></a:xfrm><a:blipFill><a:blip r:embed="rId1"><a:alphaMod amt="80000"/><a:alphaOff amt="-10000"/><a:alphaRepl a="60000"/><a:hsl hue="5400000" sat="75000" lum="120000"/><a:clrRepl><a:srgbClr val="336699"/></a:clrRepl></a:blip></a:blipFill></dsp:spPr></dsp:sp></dsp:spTree></dsp:drawing>`)
	shapes, err := ProcessDiagramWithOptionsAndFrame(dataModelXML, nil, nil, nil, drawingXML, nil, nil)
	if err != nil {
		t.Fatalf("ProcessDiagramWithOptionsAndFrame: %v", err)
	}
	if len(shapes) != 1 {
		t.Fatalf("shapes = %d, want 1", len(shapes))
	}
	pf, ok := shapes[0].Fill.(*drawing.PictureFill)
	if !ok {
		t.Fatalf("fill = %T, want PictureFill", shapes[0].Fill)
	}
	if len(pf.BlipEffects) != 5 {
		t.Fatalf("effects = %d, want 5 (%#v)", len(pf.BlipEffects), pf.BlipEffects)
	}
	if _, ok := pf.BlipEffects[0].(drawing.AlphaMod); !ok {
		t.Fatalf("effect[0] = %T, want AlphaMod", pf.BlipEffects[0])
	}
	if _, ok := pf.BlipEffects[3].(drawing.HSLEffect); !ok {
		t.Fatalf("effect[3] = %T, want HSLEffect", pf.BlipEffects[3])
	}
	if repl, ok := pf.BlipEffects[4].(drawing.ClrRepl); !ok || repl.Color != "336699" {
		t.Fatalf("effect[4] = %#v, want ClrRepl", pf.BlipEffects[4])
	}
}
