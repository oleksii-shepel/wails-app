package smartart

import (
	"math"
	"testing"

	"office/pkg/ooxml/drawing"
)

func TestParseDiagramWithOptionsLayout(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawing/2006/diagram">
		<dgm:ptLst>
			<dgm:pt id="1" type="doc">
				<dgm:prSet>
					<dgm:fld>Root</dgm:fld>
				</dgm:prSet>
			</dgm:pt>
			<dgm:pt id="2" type="node">
				<dgm:prSet>
					<dgm:fld>Child</dgm:fld>
				</dgm:prSet>
			</dgm:pt>
		</dgm:ptLst>
		<dgm:cxnLst>
			<dgm:cxn srcId="1" destId="2"/>
		</dgm:cxnLst>
	</dgm:dataModel>`)

	layoutXML := []byte(`<dgm:layoutDef xmlns:dgm="http://schemas.openxmlformats.org/drawing/2006/diagram" dir="lrtb" typeUri="urn:microsoft:office:process">
		<dgm:varLst>
			<dgm:dir val="lrtb"/>
		</dgm:varLst>
	</dgm:layoutDef>`)

	diagram, err := ParseDiagramWithOptions(dataModelXML, layoutXML, nil, nil)
	if err != nil {
		t.Fatalf("ParseDiagramWithOptions: %v", err)
	}
	if diagram.Layout.Type != "process" {
		t.Errorf("Layout.Type = %q, want process", diagram.Layout.Type)
	}
	if diagram.Layout.Direction != "leftRight" {
		t.Errorf("Layout.Direction = %q, want leftRight", diagram.Layout.Direction)
	}
}

func TestDetectDiagramType(t *testing.T) {
	tests := []struct {
		name string
		xml  string
		want string
	}{
		{"orgChart", "<layoutDef typeUri=\"orgChart\">", "hierarchy"},
		{"hierarchy", "<layoutDef typeUri=\"hierarchy\">", "hierarchy"},
		{"process", "<layoutDef typeUri=\"process\">", "process"},
		{"flowChart", "<layoutDef typeUri=\"flowChart\">", "process"},
		{"cycle", "<layoutDef typeUri=\"cycle\">", "cycle"},
		{"radial", "<layoutDef typeUri=\"radial\">", "radial"},
		{"matrix", "<layoutDef typeUri=\"matrix\">", "matrix"},
		{"pyramid", "<layoutDef typeUri=\"pyramid\">", "pyramid"},
		{"list", "<layoutDef typeUri=\"list\">", "list"},
		{"bullet", "<layoutDef typeUri=\"bullet\">", "bullet"},
		{"unknown", "<layoutDef typeUri=\"custom\">", "hierarchy"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectDiagramType([]byte(tt.xml))
			if got != tt.want {
				t.Errorf("DetectDiagramType(%q) = %q, want %q", tt.xml, got, tt.want)
			}
		})
	}
}

func TestParseDiagramWithOptionsColorPalette(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawing/2006/diagram">
		<dgm:ptLst>
			<dgm:pt id="1" type="doc">
				<dgm:prSet><dgm:fld>A</dgm:fld></dgm:prSet>
			</dgm:pt>
		</dgm:ptLst>
	</dgm:dataModel>`)

	colorXML := []byte(`<cs:colorsDef xmlns:cs="http://schemas.openxmlformats.org/drawing/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<cs:styleLbl name="node1">
			<cs:fillClrLst>
				<a:srgbClr val="FF0000"/>
				<a:srgbClr val="00FF00"/>
				<a:srgbClr val="0000FF"/>
			</cs:fillClrLst>
		</cs:styleLbl>
	</cs:colorsDef>`)

	diagram, err := ParseDiagramWithOptions(dataModelXML, nil, nil, colorXML)
	if err != nil {
		t.Fatalf("ParseDiagramWithOptions: %v", err)
	}
	want := []string{"FF0000", "00FF00", "0000FF"}
	if len(diagram.Style.ColorPalette) != len(want) {
		t.Fatalf("ColorPalette = %v, want %v", diagram.Style.ColorPalette, want)
	}
	for i, c := range want {
		if diagram.Style.ColorPalette[i] != c {
			t.Errorf("ColorPalette[%d] = %q, want %q", i, diagram.Style.ColorPalette[i], c)
		}
	}
}

func TestParseDiagramWithOptionsStyleShapeType(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawing/2006/diagram">
		<dgm:ptLst>
			<dgm:pt id="1" type="doc">
				<dgm:prSet><dgm:fld>A</dgm:fld></dgm:prSet>
			</dgm:pt>
		</dgm:ptLst>
	</dgm:dataModel>`)

	styleXML := []byte(`<qs:styleDef xmlns:qs="http://schemas.openxmlformats.org/drawing/2006/diagram">
		<qs:styleLbl name="node1">
			<qs:style>
				<qs:fillRef idx="1">
					<qs:schemeClr val="accent1"/>
				</qs:fillRef>
				<qs:effectRef idx="0">
					<qs:schemeClr val="accent1"/>
				</qs:effectRef>
			</qs:style>
			<qs:scene3d/>
		</qs:styleLbl>
	</qs:styleDef>`)

	diagram, err := ParseDiagramWithOptions(dataModelXML, nil, styleXML, nil)
	if err != nil {
		t.Fatalf("ParseDiagramWithOptions: %v", err)
	}
	if diagram.Style.ShapeType != "rectangle" {
		t.Errorf("ShapeType = %q, want rectangle (default when no hint found)", diagram.Style.ShapeType)
	}

	// Now test with an ellipse hint
	styleXML = []byte(`<qs:styleDef xmlns:qs="http://schemas.openxmlformats.org/drawing/2006/diagram">
		<qs:styleLbl name="node1">
			<qs:style>
				<qs:prstGeom prst="ellipse"/>
			</qs:style>
		</qs:styleLbl>
	</qs:styleDef>`)

	diagram, err = ParseDiagramWithOptions(dataModelXML, nil, styleXML, nil)
	if err != nil {
		t.Fatalf("ParseDiagramWithOptions: %v", err)
	}
	if diagram.Style.ShapeType != "ellipse" {
		t.Errorf("ShapeType = %q, want ellipse", diagram.Style.ShapeType)
	}
}

func TestParseDiagramRealSmartArtStructure(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dgm:ptLst>
			<dgm:pt modelId="{ROOT}" type="doc"/>
			<dgm:pt modelId="{A}">
				<dgm:t>
					<a:p>
						<a:r><a:t>Alpha</a:t></a:r>
					</a:p>
				</dgm:t>
			</dgm:pt>
			<dgm:pt modelId="{B}">
				<dgm:t>
					<a:p>
						<a:r><a:t>Beta</a:t></a:r>
					</a:p>
				</dgm:t>
			</dgm:pt>
			<dgm:pt modelId="{PAR}" type="parTrans" cxnId="{C1}"/>
			<dgm:pt modelId="{SIB}" type="sibTrans" cxnId="{C1}"/>
			<dgm:pt modelId="{PRES}" type="pres"/>
		</dgm:ptLst>
		<dgm:cxnLst>
			<dgm:cxn modelId="{C1}" srcId="{ROOT}" destId="{A}" parTransId="{PAR}" sibTransId="{SIB}"/>
			<dgm:cxn modelId="{C2}" srcId="{ROOT}" destId="{B}"/>
			<dgm:cxn modelId="{C3}" type="presOf" srcId="{A}" destId="{PRES}"/>
		</dgm:cxnLst>
	</dgm:dataModel>`)

	diagram, err := ParseDiagram(dataModelXML)
	if err != nil {
		t.Fatalf("ParseDiagram: %v", err)
	}

	if len(diagram.Nodes) != 3 {
		t.Fatalf("visible nodes = %d, want 3", len(diagram.Nodes))
	}

	texts := map[string]string{}
	for _, n := range diagram.Nodes {
		texts[n.ID] = n.Text
	}
	if got, want := texts["{A}"], "Alpha"; got != want {
		t.Errorf("node A text = %q, want %q", got, want)
	}
	if got, want := texts["{B}"], "Beta"; got != want {
		t.Errorf("node B text = %q, want %q", got, want)
	}
	if got, want := texts["{ROOT}"], ""; got != want {
		t.Errorf("root text = %q, want %q", got, want)
	}

	root := findNodeByID(diagram, "{ROOT}")
	if root == nil {
		t.Fatal("root node missing")
	}
	if root.Level != 0 {
		t.Errorf("root level = %d, want 0", root.Level)
	}
	if len(root.Children) != 2 {
		t.Fatalf("root children = %d, want 2", len(root.Children))
	}

	childA := findNodeByID(diagram, "{A}")
	if childA == nil {
		t.Fatal("child A missing")
	}
	if childA.Level != 1 {
		t.Errorf("child A level = %d, want 1", childA.Level)
	}
	if childA.Parent != root {
		t.Error("child A parent mismatch")
	}
}

func findNodeByID(d *Diagram, id string) *Node {
	for _, n := range d.Nodes {
		if n.ID == id {
			return n
		}
	}
	return nil
}

func TestParseDiagramWithOptionsColorPaletteSchemeClr(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram">
		<dgm:ptLst>
			<dgm:pt id="1" type="doc">
				<dgm:prSet><dgm:fld>A</dgm:fld></dgm:prSet>
			</dgm:pt>
		</dgm:ptLst>
	</dgm:dataModel>`)

	colorXML := []byte(`<cs:colorsDef xmlns:cs="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<cs:styleLbl name="node1">
			<cs:fillClrLst>
				<a:schemeClr val="accent1"/>
				<a:srgbClr val="00FF00"/>
			</cs:fillClrLst>
		</cs:styleLbl>
	</cs:colorsDef>`)

	themeColors := map[string]string{
		"accent1": "FF0000",
	}

	diagram, err := ParseDiagramWithOptionsAndTheme(dataModelXML, nil, nil, colorXML, themeColors)
	if err != nil {
		t.Fatalf("ParseDiagramWithOptionsAndTheme: %v", err)
	}
	want := []string{"FF0000", "00FF00"}
	if len(diagram.Style.ColorPalette) != len(want) {
		t.Fatalf("ColorPalette = %v, want %v", diagram.Style.ColorPalette, want)
	}
	for i, c := range want {
		if diagram.Style.ColorPalette[i] != c {
			t.Errorf("ColorPalette[%d] = %q, want %q", i, diagram.Style.ColorPalette[i], c)
		}
	}
}

func TestParseDiagramWithOptionsStyleShapeTypeFallback(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram">
		<dgm:ptLst>
			<dgm:pt id="1" type="doc">
				<dgm:prSet><dgm:fld>A</dgm:fld></dgm:prSet>
			</dgm:pt>
		</dgm:ptLst>
	</dgm:dataModel>`)

	for _, tc := range []struct {
		hint, want string
	}{
		{"hexagon", "hexagon"},
		{"diamond", "diamond"},
		{"triangle", "triangle"},
		{"pentagon", "pentagon"},
		{"roundRect", "roundRect"},
		{"ellipse", "ellipse"},
	} {
		styleXML := []byte(`<qs:styleDef xmlns:qs="http://schemas.openxmlformats.org/drawingml/2006/diagram">
			<qs:styleLbl name="node1">
				<qs:style>
					<qs:prstGeom prst="rectangle"/>
				</qs:style>
			</qs:styleLbl>
			<qs:other hint="` + tc.hint + `"/>
		</qs:styleDef>`)

		diagram, err := ParseDiagramWithOptions(dataModelXML, nil, styleXML, nil)
		if err != nil {
			t.Fatalf("ParseDiagramWithOptions %s: %v", tc.hint, err)
		}
		if diagram.Style.ShapeType != tc.want {
			t.Errorf("ShapeType %s = %q, want %q", tc.hint, diagram.Style.ShapeType, tc.want)
		}
	}
}

func TestParseDiagramWithOptionsStyleLineAndText(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram">
		<dgm:ptLst>
			<dgm:pt id="1" type="doc">
				<dgm:prSet><dgm:fld>A</dgm:fld></dgm:prSet>
			</dgm:pt>
		</dgm:ptLst>
	</dgm:dataModel>`)

	styleXML := []byte(`<qs:styleDef xmlns:qs="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<qs:styleLbl name="node1">
			<qs:style>
				<qs:ln w="12700">
					<a:solidFill>
						<a:srgbClr val="FF0000"/>
					</a:solidFill>
				</qs:ln>
				<qs:fontRef idx="minorFont">
					<a:srgbClr val="00FF00"/>
				</qs:fontRef>
			</qs:style>
		</qs:styleLbl>
	</qs:styleDef>`)

	diagram, err := ParseDiagramWithOptions(dataModelXML, nil, styleXML, nil)
	if err != nil {
		t.Fatalf("ParseDiagramWithOptions: %v", err)
	}
	if diagram.Style.Line == nil {
		t.Fatalf("Line style not parsed")
	}
	if diagram.Style.Line.Color != "FF0000" {
		t.Errorf("Line.Color = %q, want FF0000", diagram.Style.Line.Color)
	}
	if diagram.Style.Line.Width != 1.0 {
		t.Errorf("Line.Width = %v, want 1.0", diagram.Style.Line.Width)
	}
	if diagram.Style.TextColor != "00FF00" {
		t.Errorf("TextColor = %q, want 00FF00", diagram.Style.TextColor)
	}
}

func TestParseDiagramWithOptionsStyleLineAndTextTheme(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram">
		<dgm:ptLst>
			<dgm:pt id="1" type="doc">
				<dgm:prSet><dgm:fld>A</dgm:fld></dgm:prSet>
			</dgm:pt>
		</dgm:ptLst>
	</dgm:dataModel>`)

	styleXML := []byte(`<qs:styleDef xmlns:qs="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<qs:styleLbl name="node1">
			<qs:style>
				<qs:ln w="25400">
					<a:solidFill>
						<a:schemeClr val="accent1"/>
					</a:solidFill>
				</qs:ln>
				<qs:fontRef idx="minorFont">
					<a:schemeClr val="tx1"/>
				</qs:fontRef>
			</qs:style>
		</qs:styleLbl>
	</qs:styleDef>`)

	themeColors := map[string]string{
		"accent1": "123456",
		"tx1":     "ABCDEF",
	}

	diagram, err := ParseDiagramWithOptionsAndTheme(dataModelXML, nil, styleXML, nil, themeColors)
	if err != nil {
		t.Fatalf("ParseDiagramWithOptionsAndTheme: %v", err)
	}
	if diagram.Style.Line == nil {
		t.Fatalf("Line style not parsed")
	}
	if diagram.Style.Line.Color != "123456" {
		t.Errorf("Line.Color = %q, want 123456", diagram.Style.Line.Color)
	}
	if diagram.Style.Line.Width != 2.0 {
		t.Errorf("Line.Width = %v, want 2.0", diagram.Style.Line.Width)
	}
	if diagram.Style.TextColor != "ABCDEF" {
		t.Errorf("TextColor = %q, want ABCDEF", diagram.Style.TextColor)
	}
}

func TestParseDiagramWithOptionsRealStyleXML(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram">
		<dgm:ptLst>
			<dgm:pt id="1" type="doc">
				<dgm:prSet><dgm:fld>A</dgm:fld></dgm:prSet>
			</dgm:pt>
		</dgm:ptLst>
	</dgm:dataModel>`)

	// Real-world styleDef uses the dgm namespace and a:lnRef / a:fontRef.
	styleXML := []byte(`<dgm:styleDef xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dgm:styleLbl name="node0">
			<dgm:style>
				<a:lnRef idx="2">
					<a:scrgbClr r="0" g="0" b="0"/>
				</a:lnRef>
				<a:fillRef idx="1">
					<a:scrgbClr r="0" g="0" b="0"/>
				</a:fillRef>
				<a:effectRef idx="0">
					<a:scrgbClr r="0" g="0" b="0"/>
				</a:effectRef>
				<a:fontRef idx="minor">
					<a:schemeClr val="tx1"/>
				</a:fontRef>
			</dgm:style>
		</dgm:styleLbl>
	</dgm:styleDef>`)

	themeColors := map[string]string{
		"tx1": "1F2F3F",
	}
	fmtScheme := &ThemeFormatScheme{
		FillStyles: []ThemeFillStyle{
			{Fill: drawing.SolidFill{Color: "FFFFFF", Alpha: 1.0}},
		},
		LineStyles: []ThemeLineStyle{
			{},
			{Line: &drawing.LineStyle{Color: "000000", Width: 1.0, Alpha: 1.0}},
		},
	}

	diagram, err := ParseDiagramWithOptionsAndThemeFormat(dataModelXML, nil, styleXML, nil, themeColors, fmtScheme)
	if err != nil {
		t.Fatalf("ParseDiagramWithOptionsAndThemeFormat: %v", err)
	}
	if diagram.Style.Line == nil {
		t.Fatalf("Line style not parsed from real style XML")
	}
	if diagram.Style.Line.Color != "000000" {
		t.Errorf("Line.Color = %q, want 000000", diagram.Style.Line.Color)
	}
	if diagram.Style.TextColor != "1F2F3F" {
		t.Errorf("TextColor = %q, want 1F2F3F", diagram.Style.TextColor)
	}
}

func TestParseDiagramWithOptionsLnRefNoLine(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram">
		<dgm:ptLst>
			<dgm:pt id="1" type="doc">
				<dgm:prSet><dgm:fld>A</dgm:fld></dgm:prSet>
			</dgm:pt>
		</dgm:ptLst>
	</dgm:dataModel>`)

	styleXML := []byte(`<dgm:styleDef xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dgm:styleLbl name="node0">
			<dgm:style>
				<a:lnRef idx="0"/>
				<a:fontRef idx="minor">
					<a:srgbClr val="FF00FF"/>
				</a:fontRef>
			</dgm:style>
		</dgm:styleLbl>
	</dgm:styleDef>`)

	diagram, err := ParseDiagramWithOptions(dataModelXML, nil, styleXML, nil)
	if err != nil {
		t.Fatalf("ParseDiagramWithOptions: %v", err)
	}
	if diagram.Style.Line != nil {
		t.Errorf("Line style should be nil for idx=0, got %+v", diagram.Style.Line)
	}
	if diagram.Style.TextColor != "FF00FF" {
		t.Errorf("TextColor = %q, want FF00FF", diagram.Style.TextColor)
	}
}

func TestParseDiagramWithOptionsLinePalette(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram">
		<dgm:ptLst>
			<dgm:pt id="1" type="doc">
				<dgm:prSet><dgm:fld>A</dgm:fld></dgm:prSet>
			</dgm:pt>
		</dgm:ptLst>
	</dgm:dataModel>`)

	colorXML := []byte(`<dgm:colorsDef xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dgm:styleLbl name="node0">
			<dgm:fillClrLst>
				<a:srgbClr val="FF0000"/>
			</dgm:fillClrLst>
			<dgm:linClrLst>
				<a:srgbClr val="000000"/>
				<a:srgbClr val="FFFFFF"/>
			</dgm:linClrLst>
		</dgm:styleLbl>
	</dgm:colorsDef>`)

	diagram, err := ParseDiagramWithOptions(dataModelXML, nil, nil, colorXML)
	if err != nil {
		t.Fatalf("ParseDiagramWithOptions: %v", err)
	}
	if len(diagram.Style.ColorPalette) != 1 || diagram.Style.ColorPalette[0] != "FF0000" {
		t.Errorf("ColorPalette = %v, want [FF0000]", diagram.Style.ColorPalette)
	}
	want := []string{"000000", "FFFFFF"}
	if len(diagram.Style.LinePalette) != len(want) {
		t.Fatalf("LinePalette = %v, want %v", diagram.Style.LinePalette, want)
	}
	for i, c := range want {
		if diagram.Style.LinePalette[i] != c {
			t.Errorf("LinePalette[%d] = %q, want %q", i, diagram.Style.LinePalette[i], c)
		}
	}
}

func TestParseDiagramWithOptionsNodeSpPr(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dgm:ptLst>
			<dgm:pt modelId="node1" type="doc">
				<dgm:spPr>
					<a:solidFill>
						<a:srgbClr val="FF0000"/>
					</a:solidFill>
					<a:ln w="12700">
						<a:solidFill>
							<a:schemeClr val="accent1"/>
						</a:solidFill>
					</a:ln>
				</dgm:spPr>
				<dgm:prSet><dgm:fld>A</dgm:fld></dgm:prSet>
			</dgm:pt>
			<dgm:pt modelId="node2">
				<dgm:spPr>
					<a:gradFill>
						<a:gsLst>
							<a:gs pos="0"><a:srgbClr val="00FF00"/></a:gs>
							<a:gs pos="100000"><a:srgbClr val="0000FF"/></a:gs>
						</a:gsLst>
					</a:gradFill>
				</dgm:spPr>
				<dgm:prSet><dgm:fld>B</dgm:fld></dgm:prSet>
			</dgm:pt>
		</dgm:ptLst>
	</dgm:dataModel>`)

	themeColors := map[string]string{
		"accent1": "123456",
	}

	diagram, err := ParseDiagramWithOptionsAndTheme(dataModelXML, nil, nil, nil, themeColors)
	if err != nil {
		t.Fatalf("ParseDiagramWithOptionsAndTheme: %v", err)
	}
	if len(diagram.Nodes) != 2 {
		t.Fatalf("nodes = %d, want 2", len(diagram.Nodes))
	}

	node1 := diagram.Nodes[0]
	fill1, ok := node1.Fill.(drawing.SolidFill)
	if !ok || fill1.Color != "FF0000" {
		t.Errorf("node1 fill = %+v, want SolidFill FF0000", node1.Fill)
	}
	if node1.Line == nil || node1.Line.Color != "123456" || node1.Line.Width != 1.0 {
		t.Errorf("node1 line = %+v, want color 123456 width 1.0", node1.Line)
	}

	node2 := diagram.Nodes[1]
	grad, ok := node2.Fill.(drawing.GradientFill)
	if !ok {
		t.Fatalf("node2 fill type = %T, want GradientFill", node2.Fill)
	}
	if len(grad.Stops) != 2 || grad.Stops[0].Color != "00FF00" || grad.Stops[1].Color != "0000FF" {
		t.Errorf("node2 gradient stops = %+v", grad.Stops)
	}
}

func TestParseDiagramWithOptionsTextFormatting(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dgm:ptLst>
			<dgm:pt modelId="1" type="doc">
				<dgm:t>
					<a:bodyPr/>
					<a:lstStyle/>
					<a:p>
						<a:pPr algn="ctr"/>
						<a:r>
							<a:rPr b="1" sz="1400">
								<a:solidFill>
									<a:srgbClr val="FF0000"/>
								</a:solidFill>
							</a:rPr>
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
				</dgm:t>
			</dgm:pt>
		</dgm:ptLst>
	</dgm:dataModel>`)

	themeColors := map[string]string{
		"accent1": "0000FF",
	}

	diagram, err := ParseDiagramWithOptionsAndTheme(dataModelXML, nil, nil, nil, themeColors)
	if err != nil {
		t.Fatalf("ParseDiagramWithOptionsAndTheme: %v", err)
	}
	if len(diagram.Nodes) != 1 {
		t.Fatalf("nodes = %d, want 1", len(diagram.Nodes))
	}
	node := diagram.Nodes[0]
	if node.Text != "bolditalic" {
		t.Errorf("plain text = %q, want bolditalic", node.Text)
	}
	if len(node.Paragraphs) != 1 {
		t.Fatalf("paragraphs = %d, want 1", len(node.Paragraphs))
	}
	if node.Paragraphs[0].Align != "ctr" {
		t.Errorf("align = %q, want ctr", node.Paragraphs[0].Align)
	}
	runs := node.Paragraphs[0].Runs
	if len(runs) != 2 {
		t.Fatalf("runs = %d, want 2", len(runs))
	}
	if !runs[0].Bold || runs[0].Italic || runs[0].FontSize != 1400 || runs[0].Color != "FF0000" || runs[0].Text != "bold" {
		t.Errorf("run0 = %+v", runs[0])
	}
	if runs[1].Bold || !runs[1].Italic || !runs[1].Underline || runs[1].Color != "0000FF" || runs[1].Text != "italic" {
		t.Errorf("run1 = %+v", runs[1])
	}
}

func TestParseDiagramWithOptionsNodeBlipFill(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dgm:ptLst>
			<dgm:pt modelId="1" type="doc">
				<dgm:spPr>
					<a:blipFill dpi="144" rotWithShape="0">
						<a:blip r:embed="rId2" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"/>
						<a:srcRect l="10000" r="20000"/>
						<a:stretch><a:fillRect t="10000" b="20000"/></a:stretch>
					</a:blipFill>
				</dgm:spPr>
				<dgm:t>
					<a:p><a:r><a:t>Image</a:t></a:r></a:p>
				</dgm:t>
			</dgm:pt>
		</dgm:ptLst>
	</dgm:dataModel>`)

	diagram, err := ParseDiagramWithOptionsAndTheme(dataModelXML, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("ParseDiagramWithOptionsAndTheme: %v", err)
	}
	if len(diagram.Nodes) != 1 {
		t.Fatalf("nodes = %d, want 1", len(diagram.Nodes))
	}
	node := diagram.Nodes[0]
	pf, ok := node.Fill.(*drawing.PictureFill)
	if !ok {
		t.Fatalf("fill type = %T, want *drawing.PictureFill", node.Fill)
	}
	if pf.EmbedID != "rId2" {
		t.Errorf("embed id = %q, want rId2", pf.EmbedID)
	}
	if !pf.Stretch {
		t.Errorf("stretch = false, want true")
	}
	if pf.DPI != 144 || !pf.RotWithShapeSet || pf.RotWithShape {
		t.Errorf("geometry metadata = dpi=%d rotSet=%v rot=%v", pf.DPI, pf.RotWithShapeSet, pf.RotWithShape)
	}
	if pf.Crop == nil || math.Abs(pf.Crop.X-0.1) > 1e-9 || math.Abs(pf.Crop.Width-0.7) > 1e-9 {
		t.Errorf("crop = %#v", pf.Crop)
	}
	if pf.FillRect == nil || math.Abs(pf.FillRect.Y-0.1) > 1e-9 || math.Abs(pf.FillRect.Height-0.7) > 1e-9 {
		t.Errorf("fill rect = %#v", pf.FillRect)
	}
}

func TestParseDiagramWithOptionsBodyPr(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dgm:ptLst>
			<dgm:pt modelId="1" type="doc">
				<dgm:t>
					<a:bodyPr anchor="b" anchorCtr="1" rot="5400000" wrap="none" vert="vert" lIns="100000" rIns="200000" tIns="300000" bIns="400000"/>
					<a:lstStyle/>
					<a:p><a:r><a:t>text</a:t></a:r></a:p>
				</dgm:t>
			</dgm:pt>
		</dgm:ptLst>
	</dgm:dataModel>`)

	diagram, err := ParseDiagramWithOptionsAndTheme(dataModelXML, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("ParseDiagramWithOptionsAndTheme: %v", err)
	}
	if len(diagram.Nodes) != 1 {
		t.Fatalf("nodes = %d, want 1", len(diagram.Nodes))
	}
	bp := diagram.Nodes[0].TextBodyProps
	if bp.Anchor != "b" || !bp.AnchorCtr || bp.Rot != 5400000 || bp.Wrap != "none" || bp.Vert != "vert" {
		t.Errorf("body pr = %+v", bp)
	}
	if bp.InsetL != 100000 || bp.InsetR != 200000 || bp.InsetT != 300000 || bp.InsetB != 400000 {
		t.Errorf("insets = %+v", bp)
	}
}

func TestParseDiagramSemanticBackgroundWholeAndTextEffects(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dgm:ptLst>
			<dgm:pt modelId="1" type="doc">
				<dgm:t>
					<a:p><a:r>
						<a:rPr><a:effectLst><a:glow rad="12700"><a:srgbClr val="00FF00"/></a:glow></a:effectLst></a:rPr>
						<a:t>Glow</a:t>
					</a:r></a:p>
				</dgm:t>
			</dgm:pt>
		</dgm:ptLst>
		<dgm:bg><a:bgClr><a:srgbClr val="FF0000"/></a:bgClr></dgm:bg>
		<dgm:whole><a:effectLst><a:outerShdw blurRad="12700" dist="12700" dir="0"><a:srgbClr val="000000"/></a:outerShdw></a:effectLst></dgm:whole>
	</dgm:dataModel>`)

	diagram, err := ParseDiagramWithOptionsAndTheme(dataModelXML, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("ParseDiagramWithOptionsAndTheme: %v", err)
	}
	fill, ok := diagram.BackgroundFill.(drawing.SolidFill)
	if !ok || fill.Color != "FF0000" {
		t.Fatalf("background fill = %#v, want red solid fill", diagram.BackgroundFill)
	}
	if len(diagram.WholeEffect.Effects) != 1 || diagram.WholeEffect.Effects[0].Type() != drawing.EffectTypeShadow {
		t.Fatalf("whole effects = %#v", diagram.WholeEffect)
	}
	if len(diagram.Nodes) != 1 || len(diagram.Nodes[0].Paragraphs) != 1 || len(diagram.Nodes[0].Paragraphs[0].Runs) != 1 {
		t.Fatalf("text runs = %#v", diagram.Nodes)
	}
	run := diagram.Nodes[0].Paragraphs[0].Runs[0]
	if len(run.Effects.Effects) != 1 || run.Effects.Effects[0].Type() != drawing.EffectTypeGlow {
		t.Fatalf("run effects = %#v", run.Effects)
	}
}

func TestParseDiagramWithOptionsParagraphTabStops(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dgm:ptLst>
			<dgm:pt modelId="1" type="doc">
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
	</dgm:dataModel>`)

	diagram, err := ParseDiagramWithOptionsAndTheme(dataModelXML, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("ParseDiagramWithOptionsAndTheme: %v", err)
	}
	if len(diagram.Nodes) != 1 || len(diagram.Nodes[0].Paragraphs) != 1 {
		t.Fatalf("paragraphs = %#v", diagram.Nodes)
	}
	para := diagram.Nodes[0].Paragraphs[0]
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

func TestParseDiagramWithOptionsPropertySet(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram">
		<dgm:ptLst>
			<dgm:pt modelId="1" type="doc">
				<dgm:prSet loTypeId="node1" qsTypeId="fgAcc1" csTypeId="acc1" phldr="1" phldrT="[Text]" custAng="45"/>
			</dgm:pt>
		</dgm:ptLst>
	</dgm:dataModel>`)

	diagram, err := ParseDiagramWithOptionsAndTheme(dataModelXML, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("ParseDiagramWithOptionsAndTheme: %v", err)
	}
	if len(diagram.Nodes) != 1 {
		t.Fatalf("nodes = %d, want 1", len(diagram.Nodes))
	}
	ps := diagram.Nodes[0].PropertySet
	if ps.LayoutTypeID != "node1" || ps.QuickStyleTypeID != "fgAcc1" || ps.ColorTypeID != "acc1" {
		t.Errorf("type ids = %+v", ps)
	}
	if !ps.Placeholder || ps.PlaceholderText != "[Text]" || ps.CustomAngle != 45 {
		t.Errorf("placeholder/angle = %+v", ps)
	}
}

func TestParseDiagramWithOptionsStyleMatrixRefs(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram">
		<dgm:ptLst>
			<dgm:pt id="1" type="doc">
				<dgm:prSet><dgm:fld>A</dgm:fld></dgm:prSet>
			</dgm:pt>
		</dgm:ptLst>
	</dgm:dataModel>`)

	styleXML := []byte(`<dgm:styleDef xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dgm:styleLbl name="node0">
			<dgm:style>
				<a:fillRef idx="1">
					<a:srgbClr val="FF0000"/>
				</a:fillRef>
				<a:lnRef idx="2">
					<a:srgbClr val="00FF00"/>
				</a:lnRef>
				<a:fontRef idx="minor">
					<a:srgbClr val="0000FF"/>
				</a:fontRef>
			</dgm:style>
		</dgm:styleLbl>
		<dgm:styleLbl name="lnNode1">
			<dgm:style>
				<a:lnRef idx="3">
					<a:srgbClr val="FFFF00"/>
				</a:lnRef>
			</dgm:style>
		</dgm:styleLbl>
	</dgm:styleDef>`)

	fmtScheme := &ThemeFormatScheme{
		FillStyles: []ThemeFillStyle{
			{Fill: drawing.SolidFill{Color: "phClrPlaceholder", Alpha: 1.0}, UsesPlaceholder: true},
		},
		LineStyles: []ThemeLineStyle{
			{},
			{Line: &drawing.LineStyle{Color: "phClrPlaceholder", Width: 2.0, Alpha: 1.0}, UsesPlaceholder: true},
			{Line: &drawing.LineStyle{Color: "000000", Width: 3.0, Alpha: 1.0}, UsesPlaceholder: true},
		},
	}

	diagram, err := ParseDiagramWithOptionsAndThemeFormat(dataModelXML, nil, styleXML, nil, nil, fmtScheme)
	if err != nil {
		t.Fatalf("ParseDiagramWithOptionsAndThemeFormat: %v", err)
	}

	if sf, ok := diagram.Style.Fill.(drawing.SolidFill); !ok || sf.Color != "FF0000" {
		t.Errorf("fill = %+v, want solid FF0000", diagram.Style.Fill)
	}
	if diagram.Style.Line == nil || diagram.Style.Line.Color != "00FF00" || diagram.Style.Line.Width != 2.0 {
		t.Errorf("line = %+v, want color 00FF00 width 2pt", diagram.Style.Line)
	}
	if diagram.Style.TextColor != "0000FF" {
		t.Errorf("text color = %q, want 0000FF", diagram.Style.TextColor)
	}
	if diagram.Style.ConnectorLine == nil || diagram.Style.ConnectorLine.Color != "FFFF00" || diagram.Style.ConnectorLine.Width != 3.0 {
		t.Errorf("connector line = %+v, want color FFFF00 width 3pt", diagram.Style.ConnectorLine)
	}
}

func TestParseDiagramWithOptionsFillRefNoFill(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram">
		<dgm:ptLst>
			<dgm:pt id="1" type="doc">
				<dgm:prSet><dgm:fld>A</dgm:fld></dgm:prSet>
			</dgm:pt>
		</dgm:ptLst>
	</dgm:dataModel>`)

	styleXML := []byte(`<dgm:styleDef xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dgm:styleLbl name="node0">
			<dgm:style>
				<a:fillRef idx="0"/>
				<a:lnRef idx="0"/>
			</dgm:style>
		</dgm:styleLbl>
	</dgm:styleDef>`)

	fmtScheme := &ThemeFormatScheme{
		FillStyles: []ThemeFillStyle{{Fill: drawing.SolidFill{Color: "FFFFFF", Alpha: 1.0}}},
		LineStyles: []ThemeLineStyle{{Line: &drawing.LineStyle{Color: "000000", Width: 1.0, Alpha: 1.0}}},
	}

	diagram, err := ParseDiagramWithOptionsAndThemeFormat(dataModelXML, nil, styleXML, nil, nil, fmtScheme)
	if err != nil {
		t.Fatalf("ParseDiagramWithOptionsAndThemeFormat: %v", err)
	}
	if _, ok := diagram.Style.Fill.(drawing.NoFill); !ok {
		t.Errorf("fill = %+v, want NoFill for idx=0", diagram.Style.Fill)
	}
	if diagram.Style.Line != nil {
		t.Errorf("line = %+v, want nil for idx=0", diagram.Style.Line)
	}
}

func TestParseDiagramWithOptionsLayoutDefinition(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram">
		<dgm:ptLst>
			<dgm:pt id="1" type="doc"><dgm:prSet><dgm:fld>A</dgm:fld></dgm:prSet></dgm:pt>
		</dgm:ptLst>
	</dgm:dataModel>`)

	layoutXML := []byte(`<dgm:layoutDef xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" uniqueId="urn:test" dir="tB">
		<dgm:layoutNode name="diagram">
			<dgm:varLst>
				<dgm:dir val="norm"/>
			</dgm:varLst>
			<dgm:choose name="pickAlg">
				<dgm:if name="horizontal" func="var" arg="dir" op="equ" val="norm">
					<dgm:alg type="snake">
						<dgm:param type="flowDir" val="row"/>
					</dgm:alg>
				</dgm:if>
				<dgm:else name="vertical">
					<dgm:alg type="lin">
						<dgm:param type="linDir" val="fromT"/>
					</dgm:alg>
				</dgm:else>
			</dgm:choose>
			<dgm:forEach name="nodes" axis="ch" ptType="node">
				<dgm:layoutNode name="node">
					<dgm:alg type="tx"/>
					<dgm:shape type="rect"/>
					<dgm:presOf axis="self" ptType="node"/>
				</dgm:layoutNode>
			</dgm:forEach>
		</dgm:layoutNode>
	</dgm:layoutDef>`)

	diagram, err := ParseDiagramWithOptionsAndTheme(dataModelXML, layoutXML, nil, nil, nil)
	if err != nil {
		t.Fatalf("ParseDiagramWithOptionsAndTheme: %v", err)
	}
	if diagram.Layout == nil {
		t.Fatal("Layout is nil")
	}
	if diagram.Layout.Direction != "topDown" {
		t.Errorf("direction = %q, want topDown", diagram.Layout.Direction)
	}
	if diagram.Layout.Definition == nil {
		t.Fatal("Layout.Definition is nil")
	}
	if diagram.Layout.Definition.UniqueID != "urn:test" {
		t.Errorf("uniqueId = %q, want urn:test", diagram.Layout.Definition.UniqueID)
	}
	root := diagram.Layout.Definition.Root
	if root == nil || root.Name != "diagram" {
		t.Fatalf("root = %+v, want name=diagram", root)
	}
	if root.Choose == nil || len(root.Choose.Cases) != 1 {
		t.Fatalf("choose cases = %d, want 1", len(root.Choose.Cases))
	}
	c := root.Choose.Cases[0]
	if c.Func != "var" || c.Arg != "dir" || c.Val != "norm" {
		t.Errorf("choose case = %+v", c)
	}
	if c.Algorithm == nil || c.Algorithm.Type != "snake" || c.Algorithm.Params["flowDir"] != "row" {
		t.Errorf("choose algorithm = %+v", c.Algorithm)
	}
	if root.ForEach == nil || root.ForEach.Axis != "ch" || root.ForEach.PtType != "node" {
		t.Errorf("forEach = %+v", root.ForEach)
	}
	if len(root.Variables) == 0 || root.Variables["dir"] != "norm" {
		t.Errorf("variables = %+v", root.Variables)
	}
}

func TestParseDiagramWithOptionsBullets(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dgm:ptLst>
			<dgm:pt id="1" type="doc">
				<dgm:t xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
					<a:bodyPr/>
					<a:p>
						<a:pPr lvl="1">
							<a:buChar char="•"/>
							<a:buClr><a:srgbClr val="FF0000"/></a:buClr>
							<a:buFont typeface="Arial"/>
							<a:buSzPts val="1400"/>
						</a:pPr>
						<a:r><a:t>First</a:t></a:r>
					</a:p>
					<a:p>
						<a:pPr>
							<a:buAutoNum type="arabicPeriod" startAt="2"/>
							<a:buFontTx/>
							<a:buSzPct val="75000"/>
						</a:pPr>
						<a:r><a:t>Second</a:t></a:r>
					</a:p>
					<a:p>
						<a:pPr><a:buNone/></a:pPr>
						<a:r><a:t>No bullet</a:t></a:r>
					</a:p>
				</dgm:t>
			</dgm:pt>
		</dgm:ptLst>
	</dgm:dataModel>`)

	diagram, err := ParseDiagramWithOptionsAndTheme(dataModelXML, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("ParseDiagramWithOptionsAndTheme: %v", err)
	}
	if len(diagram.Nodes) != 1 {
		t.Fatalf("nodes = %d, want 1", len(diagram.Nodes))
	}
	pars := diagram.Nodes[0].Paragraphs
	if len(pars) != 3 {
		t.Fatalf("paragraphs = %d, want 3", len(pars))
	}

	p0 := pars[0]
	if p0.Level != 1 || p0.BulletChar != "•" || p0.BulletColor != "FF0000" || p0.BulletFontFamily != "Arial" || p0.BulletSize != 1400 {
		t.Errorf("paragraph 0 = %+v", p0)
	}

	p1 := pars[1]
	if !p1.AutoNum || p1.AutoNumType != "arabicPeriod" || p1.AutoNumStart != 2 || !p1.BulletFontTx || p1.BulletSizePct != 75000 {
		t.Errorf("paragraph 1 = %+v", p1)
	}

	p2 := pars[2]
	if p2.BulletChar != "" || p2.AutoNum {
		t.Errorf("paragraph 2 should have no bullet, got = %+v", p2)
	}
}

func TestParseDiagramWithOptionsNodeStyleRefs(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dgm:ptLst>
			<dgm:pt modelId="node1" type="doc">
				<dgm:spPr>
					<a:style>
						<a:fillRef idx="1"/>
						<a:lnRef idx="2">
							<a:schemeClr val="accent1"/>
						</a:lnRef>
					</a:style>
				</dgm:spPr>
				<dgm:prSet><dgm:fld>A</dgm:fld></dgm:prSet>
			</dgm:pt>
			<dgm:pt modelId="node2">
				<dgm:spPr>
					<a:style>
						<a:fillRef idx="1"/>
					</a:style>
					<a:solidFill>
						<a:srgbClr val="00FF00"/>
					</a:solidFill>
				</dgm:spPr>
				<dgm:prSet><dgm:fld>B</dgm:fld></dgm:prSet>
			</dgm:pt>
		</dgm:ptLst>
	</dgm:dataModel>`)

	themeColors := map[string]string{
		"accent1": "123456",
	}
	fmtScheme := &ThemeFormatScheme{
		FillStyles: []ThemeFillStyle{
			{Fill: drawing.SolidFill{Color: "FF0000", Alpha: 1.0}},
		},
		LineStyles: []ThemeLineStyle{
			{}, // idx 1
			{Line: &drawing.LineStyle{Color: "000000", Width: 2.0, Alpha: 1.0}, UsesPlaceholder: true},
		},
	}

	diagram, err := ParseDiagramWithOptionsAndThemeFormat(dataModelXML, nil, nil, nil, themeColors, fmtScheme)
	if err != nil {
		t.Fatalf("ParseDiagramWithOptionsAndThemeFormat: %v", err)
	}
	if len(diagram.Nodes) != 2 {
		t.Fatalf("nodes = %d, want 2", len(diagram.Nodes))
	}

	node1 := diagram.Nodes[0]
	fill1, ok := node1.Fill.(drawing.SolidFill)
	if !ok || fill1.Color != "FF0000" {
		t.Errorf("node1 fill = %+v, want SolidFill FF0000", node1.Fill)
	}
	if node1.Line == nil || node1.Line.Color != "123456" || node1.Line.Width != 2.0 {
		t.Errorf("node1 line = %+v, want color 123456 width 2.0", node1.Line)
	}

	node2 := diagram.Nodes[1]
	fill2, ok := node2.Fill.(drawing.SolidFill)
	if !ok || fill2.Color != "00FF00" {
		t.Errorf("node2 fill = %+v, want direct solid fill 00FF00", node2.Fill)
	}
}

func TestParseDiagramWithOptionsParagraphSpacing(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dgm:ptLst>
			<dgm:pt modelId="1" type="doc">
				<dgm:t>
					<a:bodyPr/>
					<a:lstStyle/>
					<a:p>
						<a:pPr>
							<a:spcBef><a:spcPts val="1000"/></a:spcBef>
							<a:spcAft><a:spcPct val="200000"/></a:spcAft>
							<a:lnSpc><a:spcPct val="120000"/></a:lnSpc>
						</a:pPr>
						<a:r><a:t>text</a:t></a:r>
					</a:p>
				</dgm:t>
			</dgm:pt>
		</dgm:ptLst>
	</dgm:dataModel>`)

	diagram, err := ParseDiagramWithOptionsAndTheme(dataModelXML, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("ParseDiagramWithOptionsAndTheme: %v", err)
	}
	if len(diagram.Nodes) != 1 {
		t.Fatalf("nodes = %d, want 1", len(diagram.Nodes))
	}
	node := diagram.Nodes[0]
	if len(node.Paragraphs) != 1 {
		t.Fatalf("paragraphs = %d, want 1", len(node.Paragraphs))
	}
	p := node.Paragraphs[0]
	if p.SpaceBefore != 1000 {
		t.Errorf("space before = %d, want 1000", p.SpaceBefore)
	}
	if p.SpaceAfterPct != 200000 {
		t.Errorf("space after pct = %d, want 200000", p.SpaceAfterPct)
	}
	if p.LineSpacingPct != 120000 {
		t.Errorf("line spacing pct = %d, want 120000", p.LineSpacingPct)
	}
}

func TestParseDataModelNodeStylesPatternFill(t *testing.T) {
	xmlData := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawing/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dgm:ptLst>
			<dgm:pt id="n1" type="node">
				<dgm:spPr>
					<a:pattFill prst="smGrid">
						<a:fgClr><a:srgbClr val="FF0000"><a:alphaMod val="50000"/></a:srgbClr></a:fgClr>
						<a:bgClr><a:srgbClr val="FFFFFF"/></a:bgClr>
					</a:pattFill>
				</dgm:spPr>
			</dgm:pt>
		</dgm:ptLst>
	</dgm:dataModel>`)

	styles := parseDataModelNodeStyles(xmlData, nil, nil)
	style, ok := styles["n1"]
	if !ok {
		t.Fatalf("missing style for n1")
	}
	pf, ok := style.Fill.(drawing.PatternFill)
	if !ok {
		t.Fatalf("fill = %T, want PatternFill", style.Fill)
	}
	if string(pf.PatternType) != "smGrid" {
		t.Errorf("pattern type = %q, want smGrid", pf.PatternType)
	}
	if pf.Foreground != "FF0000" {
		t.Errorf("foreground = %q, want FF0000", pf.Foreground)
	}
	if pf.Background != "FFFFFF" {
		t.Errorf("background = %q, want FFFFFF", pf.Background)
	}
	if pf.ForeAlpha != 0.5 {
		t.Errorf("foreground alpha = %v, want 0.5", pf.ForeAlpha)
	}
}

func TestParseDataModelNodeStylesAlphaModOff(t *testing.T) {
	xmlData := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawing/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dgm:ptLst>
			<dgm:pt id="n2" type="node">
				<dgm:spPr>
					<a:solidFill><a:srgbClr val="00FF00"><a:alpha val="80000"/><a:alphaMod val="50000"/><a:alphaOff val="10000"/></a:srgbClr></a:solidFill>
				</dgm:spPr>
			</dgm:pt>
		</dgm:ptLst>
	</dgm:dataModel>`)

	styles := parseDataModelNodeStyles(xmlData, nil, nil)
	style := styles["n2"]
	sf, ok := style.Fill.(drawing.SolidFill)
	if !ok {
		t.Fatalf("fill = %T, want SolidFill", style.Fill)
	}
	// 0.8 * 0.5 + 0.1 = 0.5
	if sf.Alpha != 0.5 {
		t.Errorf("alpha = %v, want 0.5", sf.Alpha)
	}
}

func TestParseDiagramExtendedColorPalettes(t *testing.T) {
	colorXML := []byte(`<cs:colorDef xmlns:cs="http://schemas.microsoft.com/office/drawing/2007/8/2/colors" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<cs:fillClrLst><a:srgbClr val="111111"/></cs:fillClrLst>
		<cs:linClrLst><a:srgbClr val="222222"/></cs:linClrLst>
		<cs:txFillClrLst><a:srgbClr val="333333"/></cs:txFillClrLst>
		<cs:txLinClrLst><a:srgbClr val="444444"/></cs:txLinClrLst>
		<cs:effectClrLst><a:srgbClr val="555555"/></cs:effectClrLst>
	</cs:colorDef>`)
	xmlData := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawing/2006/diagram">
		<dgm:ptLst>
			<dgm:pt id="1" type="doc"><dgm:prSet><dgm:fld>Root</dgm:fld></dgm:prSet></dgm:pt>
		</dgm:ptLst>
	</dgm:dataModel>`)

	diagram, err := ParseDiagramWithOptionsAndTheme(xmlData, nil, nil, colorXML, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(diagram.Style.ColorPalette) != 1 || diagram.Style.ColorPalette[0] != "111111" {
		t.Errorf("ColorPalette = %v, want [111111]", diagram.Style.ColorPalette)
	}
	if len(diagram.Style.LinePalette) != 1 || diagram.Style.LinePalette[0] != "222222" {
		t.Errorf("LinePalette = %v, want [222222]", diagram.Style.LinePalette)
	}
	if len(diagram.Style.TextFillPalette) != 1 || diagram.Style.TextFillPalette[0] != "333333" {
		t.Errorf("TextFillPalette = %v, want [333333]", diagram.Style.TextFillPalette)
	}
	if len(diagram.Style.TextLinePalette) != 1 || diagram.Style.TextLinePalette[0] != "444444" {
		t.Errorf("TextLinePalette = %v, want [444444]", diagram.Style.TextLinePalette)
	}
	if len(diagram.Style.EffectPalette) != 1 || diagram.Style.EffectPalette[0] != "555555" {
		t.Errorf("EffectPalette = %v, want [555555]", diagram.Style.EffectPalette)
	}
}

func TestParseSmartArtStyleDirectEffect(t *testing.T) {
	styleXML := []byte(`<dgm:styleDef xmlns:dgm="http://schemas.openxmlformats.org/drawing/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dgm:styleLbl name="node1">
			<a:style>
				<a:effectLst>
					<a:outerShdw blurRad="19050" dist="19050" dir="2700000">
						<a:srgbClr val="FF0000"/>
					</a:outerShdw>
					<a:softEdge rad="6350"/>
				</a:effectLst>
			</a:style>
		</dgm:styleLbl>
	</dgm:styleDef>`)

	fill, line, connLine, effect, textColor, _ := parseSmartArtStyle(styleXML, nil, nil)
	if fill != nil || line != nil || connLine != nil || textColor != "" {
		t.Fatalf("unexpected non-empty values")
	}
	if len(effect.Effects) != 2 {
		t.Fatalf("effects = %d, want 2", len(effect.Effects))
	}
	if shadow, ok := effect.Effects[0].(drawing.Shadow); !ok || shadow.Color != "FF0000" {
		t.Errorf("first effect = %+v", effect.Effects[0])
	}
	if soft, ok := effect.Effects[1].(drawing.SoftEdge); !ok || soft.Radius != 0.5 {
		t.Errorf("second effect = %+v", effect.Effects[1])
	}
}

func TestParseDataModelNodeStyleGradientLine(t *testing.T) {
	xmlData := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawing/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dgm:ptLst>
			<dgm:pt id="n1" type="node">
				<dgm:spPr>
					<a:ln w="12700">
						<a:gradFill>
							<a:gsLst>
								<a:gs pos="0"><a:srgbClr val="FF0000"/></a:gs>
								<a:gs pos="100000"><a:srgbClr val="0000FF"/></a:gs>
							</a:gsLst>
						</a:gradFill>
					</a:ln>
				</dgm:spPr>
			</dgm:pt>
		</dgm:ptLst>
	</dgm:dataModel>`)

	styles := parseDataModelNodeStyles(xmlData, nil, nil)
	style := styles["n1"]
	if style == nil || style.Line == nil {
		t.Fatalf("missing line style")
	}
	gf, ok := style.Line.Fill.(drawing.GradientFill)
	if !ok {
		t.Fatalf("line.Fill = %T, want GradientFill", style.Line.Fill)
	}
	if len(gf.Stops) != 2 || gf.Stops[0].Color != "FF0000" || gf.Stops[1].Color != "0000FF" {
		t.Errorf("stops = %+v", gf.Stops)
	}
}

func TestParseEffectListInnerXMLWithThemeRichEffects(t *testing.T) {
	inner := `<a:outerShdw xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" blurRad="25400" dist="12700" dir="5400000" sx="125000" sy="75000" kx="600000" ky="-300000" algn="br" rotWithShape="1"><a:schemeClr val="accent1"><a:alpha val="50000"/></a:schemeClr></a:outerShdw>
<a:blur xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" rad="38100" grow="1"/>
<a:reflection xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" blurRad="12700" dist="25400" dir="5400000" stA="60000" stPos="10000" endA="5000" endPos="90000" fadeDir="5400000" sx="90000" sy="80000" kx="300000" ky="-300000" algn="b" rotWithShape="1"/>
<a:fillOverlay xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" blend="mult"><a:solidFill><a:schemeClr val="accent2"><a:alpha val="40000"/></a:schemeClr></a:solidFill></a:fillOverlay>
<a:alphaModFix xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" amt="80000"/>
<a:lum xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" bright="10000" contrast="20000"/>`

	effects := ParseEffectListInnerXMLWithTheme(inner, map[string]string{"accent1": "112233", "accent2": "445566"})
	if len(effects.Effects) != 6 {
		t.Fatalf("effects = %d, want 6", len(effects.Effects))
	}
	shadow, ok := effects.Effects[0].(drawing.Shadow)
	if !ok {
		t.Fatalf("effect[0] = %T, want Shadow", effects.Effects[0])
	}
	if shadow.Color != "112233" || shadow.Alpha != 0.5 || shadow.ScaleX != 1.25 || shadow.ScaleY != 0.75 || shadow.SkewX != 10 || shadow.SkewY != -5 || shadow.Alignment != "br" || !shadow.RotateWithShape {
		t.Fatalf("shadow = %#v", shadow)
	}
	blur, ok := effects.Effects[1].(drawing.Blur)
	if !ok || !blur.Grow || blur.Radius != 3 {
		t.Fatalf("blur = %#v", effects.Effects[1])
	}
	reflection, ok := effects.Effects[2].(drawing.Reflection)
	if !ok || reflection.StartAlpha != 60000 || reflection.EndAlpha != 5000 || reflection.StartPosition != 10000 || reflection.EndPosition != 90000 || reflection.ScaleX != 0.9 || reflection.ScaleY != 0.8 || reflection.Alignment != "b" || !reflection.RotateWithShape {
		t.Fatalf("reflection = %#v", effects.Effects[2])
	}
	overlay, ok := effects.Effects[3].(drawing.FillOverlay)
	if !ok || overlay.BlendMode != "mult" {
		t.Fatalf("overlay = %#v", effects.Effects[3])
	}
	solid, ok := overlay.Fill.(drawing.SolidFill)
	if !ok || solid.Color != "445566" || solid.Alpha != 0.4 {
		t.Fatalf("overlay fill = %#v", overlay.Fill)
	}
	if mod, ok := effects.Effects[4].(drawing.AlphaModFix); !ok || mod.Amount != 80000 {
		t.Fatalf("alphaModFix = %#v", effects.Effects[4])
	}
	if lum, ok := effects.Effects[5].(drawing.Lum); !ok || lum.Brightness != 10000 || lum.Contrast != 20000 {
		t.Fatalf("lum = %#v", effects.Effects[5])
	}
}

func TestParseSmartArtStyleResolvesEffectSchemeColor(t *testing.T) {
	styleXML := []byte(`<dgm:styleDef xmlns:dgm="http://schemas.openxmlformats.org/drawing/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><dgm:styleLbl name="node1"><a:style><a:effectLst><a:glow rad="12700"><a:schemeClr val="accent3"/></a:glow></a:effectLst></a:style></dgm:styleLbl></dgm:styleDef>`)
	_, _, _, effects, _, _ := parseSmartArtStyle(styleXML, map[string]string{"accent3": "ABCDEF"}, nil)
	if len(effects.Effects) != 1 {
		t.Fatalf("effects = %d, want 1", len(effects.Effects))
	}
	glow, ok := effects.Effects[0].(drawing.Glow)
	if !ok || glow.Color != "ABCDEF" {
		t.Fatalf("glow = %#v", effects.Effects[0])
	}
}
