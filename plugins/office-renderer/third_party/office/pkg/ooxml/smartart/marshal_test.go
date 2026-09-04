package smartart

import (
	"bytes"
	"strings"
	"testing"

	"office/pkg/ooxml/drawing"
)

func TestParsePreservesTopLevelRawElements(t *testing.T) {
	xml := `<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawing/2006/diagram"
					xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dgm:ptLst>
			<dgm:pt modelId="1" type="root"><dgm:t><a:p><a:r><a:t>Root</a:t></a:r></a:p></dgm:t></dgm:pt>
		</dgm:ptLst>
		<dgm:bg><a:bgClr><a:srgbClr val="FF0000"/></a:bgClr></dgm:bg>
		<dgm:whole><a:scene3d/></dgm:whole>
		<dgm:extLst><dgm:ext uri="{XXXX}"><m:mathPr xmlns:m="http://..."/></dgm:ext></dgm:extLst>
	</dgm:dataModel>`

	d, err := ParseDiagram([]byte(xml))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if !bytes.Contains(d.Bg, []byte("<a:bgClr>")) {
		t.Errorf("bg not preserved: %s", d.Bg)
	}
	if !bytes.Contains(d.Whole, []byte("<a:scene3d")) {
		t.Errorf("whole not preserved: %s", d.Whole)
	}
	if !bytes.Contains(d.ExtLst, []byte(`uri="{XXXX}"`)) {
		t.Errorf("extLst not preserved: %s", d.ExtLst)
	}
	if !strings.Contains(d.RootAttrs, `xmlns:dgm="http://schemas.openxmlformats.org/drawing/2006/diagram"`) {
		t.Errorf("root attrs not captured: %s", d.RootAttrs)
	}
}

func TestParsePreservesPointExtLst(t *testing.T) {
	xml := `<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawing/2006/diagram"
					xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dgm:ptLst>
			<dgm:pt modelId="1" type="root">
				<dgm:extLst><dgm:ext uri="{pt-ext}"><custom/></dgm:ext></dgm:extLst>
				<dgm:t><a:p><a:r><a:t>Root</a:t></a:r></a:p></dgm:t>
			</dgm:pt>
		</dgm:ptLst>
	</dgm:dataModel>`

	d, err := ParseDiagram([]byte(xml))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(d.Nodes) != 1 {
		t.Fatalf("got %d nodes, want 1", len(d.Nodes))
	}
	if !bytes.Contains(d.Nodes[0].ExtLst, []byte(`uri="{pt-ext}"`)) {
		t.Errorf("point extLst not preserved: %s", d.Nodes[0].ExtLst)
	}
}

func TestParseRetainsHelperPoints(t *testing.T) {
	xml := `<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawing/2006/diagram">
		<dgm:ptLst>
			<dgm:pt modelId="1" type="root"><dgm:t><a:p><a:r><a:t>Root</a:t></a:r></a:p></dgm:t></dgm:pt>
			<dgm:pt modelId="2" type="pres"><dgm:prSet phldr="1"/><dgm:t><a:p><a:r><a:t>Helper</a:t></a:r></a:p></dgm:t></dgm:pt>
			<dgm:pt modelId="3" type="parTrans"><dgm:t><a:p><a:r><a:t>Trans</a:t></a:r></a:p></dgm:t></dgm:pt>
		</dgm:ptLst>
	</dgm:dataModel>`

	d, err := ParseDiagram([]byte(xml))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(d.Nodes) != 1 {
		t.Fatalf("got %d visible nodes, want 1", len(d.Nodes))
	}
	if len(d.HelperPoints) != 2 {
		t.Fatalf("got %d helper points, want 2", len(d.HelperPoints))
	}
	ids := map[string]bool{}
	for _, hp := range d.HelperPoints {
		ids[hp.ModelID] = true
		if hp.Type == "" {
			t.Errorf("helper point missing type")
		}
		if len(hp.RawXML) == 0 {
			t.Errorf("helper point raw XML empty")
		}
	}
	if !ids["2"] || !ids["3"] {
		t.Errorf("unexpected helper ids: %v", ids)
	}
}

func TestMarshalDataModelRoundTrip(t *testing.T) {
	xml := `<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawing/2006/diagram"
					xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<dgm:ptLst>
			<dgm:pt modelId="orig1" type="root"><dgm:t><a:p><a:r><a:t>Root</a:t></a:r></a:p></dgm:t></dgm:pt>
			<dgm:pt modelId="orig2" type="node"><dgm:t><a:p><a:r><a:t>Child</a:t></a:r></a:p></dgm:t></dgm:pt>
			<dgm:pt modelId="orig3" type="pres"><dgm:t><a:p><a:r><a:t>Pres</a:t></a:r></a:p></dgm:t></dgm:pt>
		</dgm:ptLst>
		<dgm:cxnLst>
			<dgm:cxn modelId="c1" srcId="orig1" destId="orig2" type="parCh"/>
		</dgm:cxnLst>
		<dgm:bg><a:bgClr><a:srgbClr val="00FF00"/></a:bgClr></dgm:bg>
		<dgm:whole><a:scene3d/></dgm:whole>
		<dgm:extLst><dgm:ext uri="{top}"><custom/></dgm:ext></dgm:extLst>
	</dgm:dataModel>`

	d, err := ParseDiagram([]byte(xml))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	out := MarshalDataModel(d)
	if !bytes.Contains(out, []byte("<dgm:ptLst>")) {
		t.Errorf("output missing ptLst: %s", out)
	}
	if !bytes.Contains(out, []byte("<dgm:cxnLst>")) {
		t.Errorf("output missing cxnLst: %s", out)
	}
	if !bytes.Contains(out, []byte("<a:bgClr>")) {
		t.Errorf("output missing bg: %s", out)
	}
	if !bytes.Contains(out, []byte("<dgm:extLst>")) {
		t.Errorf("output missing extLst: %s", out)
	}
	if !bytes.Contains(out, []byte("<dgm:whole>")) {
		t.Errorf("output missing whole: %s", out)
	}
	if !bytes.Contains(out, []byte("Root")) || !bytes.Contains(out, []byte("Child")) {
		t.Errorf("output missing node text: %s", out)
	}
	if !bytes.Contains(out, []byte("Pres")) {
		t.Errorf("output missing helper text: %s", out)
	}
	if strings.Count(string(out), "<dgm:dataModel") != 1 {
		t.Errorf("unexpected root element count: %s", out)
	}
}

func TestMarshalPreservesNodeIDsForConnections(t *testing.T) {
	d := &Diagram{
		Nodes: []*Node{
			{ID: "root", Type: "root", Text: "Root"},
			{ID: "child", Type: "node", Text: "Child"},
		},
		Connections: []Connection{{SrcID: "root", DestID: "child"}},
	}

	out := MarshalDataModel(d)
	if !bytes.Contains(out, []byte(`modelId="root"`)) {
		t.Fatalf("root modelId not preserved: %s", out)
	}
	if !bytes.Contains(out, []byte(`modelId="child"`)) {
		t.Fatalf("child modelId not preserved: %s", out)
	}
	if !bytes.Contains(out, []byte(`srcId="root"`)) || !bytes.Contains(out, []byte(`destId="child"`)) {
		t.Fatalf("connection IDs not preserved: %s", out)
	}

	parsed, err := ParseDiagram(out)
	if err != nil {
		t.Fatalf("parse marshaled data model: %v", err)
	}
	if len(parsed.Nodes) != 2 {
		t.Fatalf("nodes = %d, want 2", len(parsed.Nodes))
	}
	var root *Node
	for _, node := range parsed.Nodes {
		if node.ID == "root" {
			root = node
		}
	}
	if root == nil {
		t.Fatal("root node missing after parse")
	}
	if len(root.Children) != 1 || root.Children[0].ID != "child" {
		t.Fatalf("root children = %+v, want child", root.Children)
	}
}

func TestMarshalDataModelTextRunFormattingRoundTrip(t *testing.T) {
	d := &Diagram{
		Nodes: []*Node{
			{
				ID:   "n1",
				Type: "root",
				Paragraphs: []*TextParagraph{
					{
						Runs: []TextRun{
							{
								Text:         "Styled",
								Bold:         true,
								Italic:       true,
								Underline:    true,
								FontSize:     1800,
								FontFamily:   "Aptos",
								FontFamilyEA: "Yu Gothic",
								FontFamilyCS: "Arial",
								Color:        "#336699",
							},
						},
					},
				},
			},
		},
	}

	out := MarshalDataModel(d)
	for _, want := range []string{
		`<a:solidFill><a:srgbClr val="336699"/></a:solidFill>`,
		`<a:latin typeface="Aptos"/>`,
		`<a:ea typeface="Yu Gothic"/>`,
		`<a:cs typeface="Arial"/>`,
	} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("marshaled XML missing %q: %s", want, out)
		}
	}

	parsed, err := ParseDiagram(out)
	if err != nil {
		t.Fatalf("parse marshaled data model: %v", err)
	}
	if len(parsed.Nodes) != 1 || len(parsed.Nodes[0].Paragraphs) != 1 || len(parsed.Nodes[0].Paragraphs[0].Runs) != 1 {
		t.Fatalf("unexpected parsed text structure: %+v", parsed.Nodes)
	}
	run := parsed.Nodes[0].Paragraphs[0].Runs[0]
	if run.Text != "Styled" || !run.Bold || !run.Italic || !run.Underline || run.FontSize != 1800 || run.Color != "336699" {
		t.Fatalf("run formatting mismatch: %+v", run)
	}
	if run.FontFamily != "Aptos" || run.FontFamilyEA != "Yu Gothic" || run.FontFamilyCS != "Arial" {
		t.Fatalf("run fonts mismatch: %+v", run)
	}
}

func TestMarshalDataModelParagraphTabStopsRoundTrip(t *testing.T) {
	d := &Diagram{
		Nodes: []*Node{
			{
				ID:   "n1",
				Type: "doc",
				Paragraphs: []*TextParagraph{
					{
						DefaultTabSizeEMU: 914400,
						TabStops: []TabStop{
							{Pos: 457200, Algn: "l"},
							{Pos: 914400, Algn: "ctr"},
						},
						Runs: []TextRun{{Text: "left\tcenter"}},
					},
				},
			},
		},
	}

	out := MarshalDataModel(d)
	if !bytes.Contains(out, []byte(`defTabSz="914400"`)) {
		t.Fatalf("expected defTabSz in output: %s", out)
	}
	if !bytes.Contains(out, []byte(`<a:tab pos="457200" algn="l"/>`)) {
		t.Fatalf("expected left tab stop in output: %s", out)
	}
	if !bytes.Contains(out, []byte(`<a:tab pos="914400" algn="ctr"/>`)) {
		t.Fatalf("expected center tab stop in output: %s", out)
	}

	parsed, err := ParseDiagram(out)
	if err != nil {
		t.Fatalf("parse marshaled data model: %v", err)
	}
	if len(parsed.Nodes) != 1 || len(parsed.Nodes[0].Paragraphs) != 1 {
		t.Fatalf("unexpected parsed text structure: %+v", parsed.Nodes)
	}
	para := parsed.Nodes[0].Paragraphs[0]
	if para.DefaultTabSizeEMU != 914400 {
		t.Fatalf("default tab size = %d, want 914400", para.DefaultTabSizeEMU)
	}
	if len(para.TabStops) != 2 {
		t.Fatalf("tab stops = %d, want 2", len(para.TabStops))
	}
}

func TestMarshalDataModelNodeFormattingRoundTrip(t *testing.T) {
	d := &Diagram{
		Nodes: []*Node{
			{
				ID:        "n1",
				Type:      "doc",
				Text:      "Formatted",
				Fill:      drawing.SolidFill{Color: "ABCDEF", Alpha: 1},
				TextColor: "102030",
			},
		},
	}

	out := MarshalDataModel(d)
	if !bytes.Contains(out, []byte(`<dgm:spPr><a:solidFill><a:srgbClr val="ABCDEF"/></a:solidFill></dgm:spPr>`)) {
		t.Fatalf("expected node fill in output: %s", out)
	}
	if !bytes.Contains(out, []byte(`<a:solidFill><a:srgbClr val="102030"/></a:solidFill>`)) {
		t.Fatalf("expected text color in output: %s", out)
	}
	parsed, err := ParseDiagram(out)
	if err != nil {
		t.Fatalf("ParseDiagram: %v", err)
	}
	if len(parsed.Nodes) != 1 {
		t.Fatalf("nodes = %d, want 1", len(parsed.Nodes))
	}
	if sf, ok := parsed.Nodes[0].Fill.(drawing.SolidFill); !ok || sf.Color != "ABCDEF" {
		t.Fatalf("fill = %#v", parsed.Nodes[0].Fill)
	}
	if len(parsed.Nodes[0].Paragraphs) != 1 || len(parsed.Nodes[0].Paragraphs[0].Runs) != 1 || parsed.Nodes[0].Paragraphs[0].Runs[0].Color != "102030" {
		t.Fatalf("text color not preserved: %#v", parsed.Nodes[0].Paragraphs)
	}
}

func TestMarshalDefaultRootAttrs(t *testing.T) {
	d := &Diagram{
		Nodes: []*Node{{ID: "1", Type: "root", Text: "A"}},
	}
	out := MarshalDataModel(d)
	if !bytes.Contains(out, []byte(`xmlns:dgm="http://schemas.openxmlformats.org/drawing/2006/diagram"`)) {
		t.Errorf("default namespaces missing: %s", out)
	}
}
