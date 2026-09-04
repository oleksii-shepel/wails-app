package render

import "testing"

func TestParseDiagramDrawingGroupTransform(t *testing.T) {
	drawingXML := []byte(`<dsp:drawing xmlns:dsp="http://schemas.microsoft.com/office/drawing/2008/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dsp:spTree>
			<dsp:grpSp>
				<dsp:grpSpPr>
					<a:xfrm>
						<a:off x="100000" y="200000"/>
						<a:ext cx="200000" cy="200000"/>
						<a:chOff x="0" y="0"/>
						<a:chExt cx="100000" cy="100000"/>
					</a:xfrm>
				</dsp:grpSpPr>
				<dsp:sp modelId="presChild">
					<dsp:spPr>
						<a:xfrm>
							<a:off x="10000" y="10000"/>
							<a:ext cx="50000" cy="50000"/>
						</a:xfrm>
						<a:prstGeom prst="rect"/>
					</dsp:spPr>
				</dsp:sp>
			</dsp:grpSp>
		</dsp:spTree>
	</dsp:drawing>`)

	shapes, _, err := parseDiagramDrawing(drawingXML, nil)
	if err != nil {
		t.Fatalf("parseDiagramDrawing: %v", err)
	}

	s, ok := shapes["presChild"]
	if !ok {
		t.Fatalf("child shape not found")
	}

	// Scale = ext/chExt = 2 in each dimension.
	// Transformed position = off + (child.off - chOff) * scale.
	if s.X != 120000 || s.Y != 220000 {
		t.Errorf("position = (%.0f, %.0f), want (120000, 220000)", s.X, s.Y)
	}
	if s.W != 100000 || s.H != 100000 {
		t.Errorf("size = (%.0f, %.0f), want (100000, 100000)", s.W, s.H)
	}
}

func TestParseDiagramDrawingNestedGroup(t *testing.T) {
	drawingXML := []byte(`<dsp:drawing xmlns:dsp="http://schemas.microsoft.com/office/drawing/2008/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dsp:spTree>
			<dsp:grpSp>
				<dsp:grpSpPr>
					<a:xfrm>
						<a:off x="100000" y="100000"/>
						<a:ext cx="200000" cy="200000"/>
						<a:chOff x="0" y="0"/>
						<a:chExt cx="100000" cy="100000"/>
					</a:xfrm>
				</dsp:grpSpPr>
				<dsp:grpSp>
					<dsp:grpSpPr>
						<a:xfrm>
							<a:off x="50000" y="50000"/>
							<a:ext cx="100000" cy="100000"/>
							<a:chOff x="0" y="0"/>
							<a:chExt cx="100000" cy="100000"/>
						</a:xfrm>
					</dsp:grpSpPr>
					<dsp:sp modelId="presNested">
						<dsp:spPr>
							<a:xfrm>
								<a:off x="0" y="0"/>
								<a:ext cx="50000" cy="50000"/>
							</a:xfrm>
							<a:prstGeom prst="rect"/>
						</dsp:spPr>
					</dsp:sp>
				</dsp:grpSp>
			</dsp:grpSp>
		</dsp:spTree>
	</dsp:drawing>`)

	shapes, _, err := parseDiagramDrawing(drawingXML, nil)
	if err != nil {
		t.Fatalf("parseDiagramDrawing: %v", err)
	}

	s, ok := shapes["presNested"]
	if !ok {
		t.Fatalf("nested shape not found")
	}

	// Outer scale = 2. Inner transform offset = 100000 + 50000*2 = 200000.
	// Child at (0,0) size 50000 -> (200000,200000) size 100000.
	if s.X != 200000 || s.Y != 200000 {
		t.Errorf("position = (%.0f, %.0f), want (200000, 200000)", s.X, s.Y)
	}
	if s.W != 100000 || s.H != 100000 {
		t.Errorf("size = (%.0f, %.0f), want (100000, 100000)", s.W, s.H)
	}
}

func TestParseDiagramDrawingGroupFlipRotate(t *testing.T) {
	drawingXML := []byte(`<dsp:drawing xmlns:dsp="http://schemas.microsoft.com/office/drawing/2008/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dsp:spTree>
			<dsp:grpSp>
				<dsp:grpSpPr>
					<a:xfrm rot="5400000" flipH="1">
						<a:off x="100000" y="100000"/>
						<a:ext cx="100000" cy="100000"/>
						<a:chOff x="0" y="0"/>
						<a:chExt cx="100000" cy="100000"/>
					</a:xfrm>
				</dsp:grpSpPr>
				<dsp:sp modelId="presChild">
					<dsp:spPr>
						<a:xfrm>
							<a:off x="0" y="0"/>
							<a:ext cx="50000" cy="50000"/>
						</a:xfrm>
						<a:prstGeom prst="rect"/>
					</dsp:spPr>
				</dsp:sp>
			</dsp:grpSp>
		</dsp:spTree>
	</dsp:drawing>`)

	shapes, _, err := parseDiagramDrawing(drawingXML, nil)
	if err != nil {
		t.Fatalf("parseDiagramDrawing: %v", err)
	}

	s, ok := shapes["presChild"]
	if !ok {
		t.Fatalf("child shape not found")
	}

	// No scale, child offset at group offset; rotation/flips accumulate.
	if s.X != 100000 || s.Y != 100000 {
		t.Errorf("position = (%.0f, %.0f), want (100000, 100000)", s.X, s.Y)
	}
	if s.RotationDeg != 90.0 {
		t.Errorf("rotation = %v, want 90", s.RotationDeg)
	}
	if !s.FlipH {
		t.Errorf("flipH = false, want true")
	}
	if s.FlipV {
		t.Errorf("flipV = true, want false")
	}
}

func TestParseDiagramDrawingGroupConnectorStyle(t *testing.T) {
	drawingXML := []byte(`<dsp:drawing xmlns:dsp="http://schemas.microsoft.com/office/drawing/2008/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dsp:spTree>
			<dsp:grpSp>
				<dsp:grpSpPr>
					<a:xfrm>
						<a:off x="0" y="0"/>
						<a:ext cx="100000" cy="100000"/>
						<a:chOff x="0" y="0"/>
						<a:chExt cx="100000" cy="100000"/>
					</a:xfrm>
				</dsp:grpSpPr>
				<dsp:cxnSp>
					<dsp:spPr>
						<a:ln w="12700">
							<a:solidFill><a:srgbClr val="00FF00"/></a:solidFill>
						</a:ln>
					</dsp:spPr>
				</dsp:cxnSp>
				<dsp:sp modelId="presChild">
					<dsp:spPr>
						<a:xfrm>
							<a:off x="0" y="0"/>
							<a:ext cx="50000" cy="50000"/>
						</a:xfrm>
						<a:prstGeom prst="rect"/>
					</dsp:spPr>
				</dsp:sp>
			</dsp:grpSp>
		</dsp:spTree>
	</dsp:drawing>`)

	_, connStyle, err := parseDiagramDrawing(drawingXML, nil)
	if err != nil {
		t.Fatalf("parseDiagramDrawing: %v", err)
	}
	if connStyle == nil {
		t.Fatal("expected connector style from group")
	}
	if connStyle.Color != "00FF00" || connStyle.Width != 1.0 {
		t.Errorf("connector style = %+v", connStyle)
	}
}

func TestParseDiagramDrawingAdjustments(t *testing.T) {
	drawingXML := []byte(`<dsp:drawing xmlns:dsp="http://schemas.microsoft.com/office/drawing/2008/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dsp:spTree>
			<dsp:sp modelId="presRoot">
				<dsp:spPr>
					<a:xfrm><a:off x="0" y="0"/><a:ext cx="100000" cy="100000"/></a:xfrm>
					<a:prstGeom prst="roundRect">
						<a:avLst>
							<a:gd name="adj" fmla="val 50000"/>
						</a:avLst>
					</a:prstGeom>
				</dsp:spPr>
			</dsp:sp>
		</dsp:spTree>
	</dsp:drawing>`)

	shapes, _, err := parseDiagramDrawing(drawingXML, nil)
	if err != nil {
		t.Fatalf("parseDiagramDrawing: %v", err)
	}
	s, ok := shapes["presRoot"]
	if !ok {
		t.Fatalf("shape not found")
	}
	if len(s.Adjustments) != 1 || s.Adjustments[0] != 0.5 {
		t.Errorf("adjustments = %v, want [0.5]", s.Adjustments)
	}
}

func TestParseDiagramDrawingOrder(t *testing.T) {
	drawingXML := []byte(`<dsp:drawing xmlns:dsp="http://schemas.microsoft.com/office/drawing/2008/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dsp:spTree>
			<dsp:sp modelId="second">
				<dsp:spPr>
					<a:xfrm><a:off x="0" y="0"/><a:ext cx="100000" cy="100000"/></a:xfrm>
					<a:prstGeom prst="rect"/>
				</dsp:spPr>
			</dsp:sp>
			<dsp:sp modelId="first">
				<dsp:spPr>
					<a:xfrm><a:off x="0" y="0"/><a:ext cx="100000" cy="100000"/></a:xfrm>
					<a:prstGeom prst="rect"/>
				</dsp:spPr>
			</dsp:sp>
		</dsp:spTree>
	</dsp:drawing>`)

	shapes, _, err := parseDiagramDrawing(drawingXML, nil)
	if err != nil {
		t.Fatalf("parseDiagramDrawing: %v", err)
	}
	if shapes["second"].DrawingOrder != 0 {
		t.Errorf("second order = %d, want 0", shapes["second"].DrawingOrder)
	}
	if shapes["first"].DrawingOrder != 1 {
		t.Errorf("first order = %d, want 1", shapes["first"].DrawingOrder)
	}
}
