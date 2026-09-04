package render

import (
	"testing"

	"office/pkg/ooxml/smartart"
)

func testNode(id, typ string, level int) *smartart.Node {
	return &smartart.Node{ID: id, Type: typ, Level: level, Assistant: typ == "asst"}
}

func TestForEachHonorsStartStepCountAndPointType(t *testing.T) {
	nodes := []*smartart.Node{
		testNode("1", "node", 1), testNode("2", "node", 1), testNode("a", "asst", 1),
		testNode("3", "node", 1), testNode("4", "node", 1), testNode("5", "node", 1),
	}
	e := &LayoutEngine{}
	ctx := &layoutContext{nodes: nodes, vars: map[string]string{}, named: map[string][]*smartart.Node{}}
	fe := &smartart.ForEachSpec{Axis: "self", PtType: "node", Start: 2, Step: 2, Cnt: 2, Children: []*smartart.LayoutNode{{Constraints: []smartart.Constraint{{Type: "w", For: "self", Val: 222}}}}}
	if !e.runForEach(fe, ctx) {
		t.Fatal("runForEach failed")
	}
	// Point-type filtering yields normal nodes [1,2,3,4,5], then st=2/step=2 => 2 and 4.
	if nodes[1].Position.Width != 222 || nodes[4].Position.Width != 222 {
		t.Fatalf("selected widths = %v, %v; want 222,222", nodes[1].Position.Width, nodes[4].Position.Width)
	}
	if nodes[0].Position.Width == 222 || nodes[2].Position.Width == 222 || nodes[3].Position.Width == 222 || nodes[5].Position.Width == 222 {
		t.Fatalf("unexpected node selected by forEach")
	}
}

func TestChooseFunctionsUseIterationAndDepthContext(t *testing.T) {
	n := testNode("n", "node", 3)
	child := testNode("c", "node", 5)
	child.Parent = n
	n.Children = []*smartart.Node{child}
	e := &LayoutEngine{}
	ctx := &layoutContext{nodes: []*smartart.Node{n}, vars: map[string]string{}, position: 2, total: 5}
	cases := []smartart.ChooseCase{
		{Func: "pos", Op: "equ", Val: "2"},
		{Func: "revPos", Op: "equ", Val: "4"},
		{Func: "posEven", Op: "equ", Val: "1"},
		{Func: "posOdd", Op: "equ", Val: "0"},
		{Func: "depth", Op: "equ", Val: "3"},
		{Func: "maxDepth", Op: "equ", Val: "5"},
	}
	for _, c := range cases {
		if !e.evaluateCase(&c, ctx) {
			t.Errorf("case %+v did not match", c)
		}
	}
}

func TestRulesAndNamedConstraintTargetsAffectGeometry(t *testing.T) {
	a := testNode("a", "node", 1)
	b := testNode("b", "node", 1)
	e := &LayoutEngine{}
	ctx := &layoutContext{nodes: []*smartart.Node{b}, vars: map[string]string{}, named: map[string][]*smartart.Node{"target": {a}}}
	e.applyRules([]smartart.Rule{{Type: "w", ForName: "target", Val: 200, Fact: 0.5, Max: 80}}, ctx)
	if a.Position.Width != 80 {
		t.Fatalf("rule width=%v, want 80", a.Position.Width)
	}
	e.applySizeConstraints([]smartart.Constraint{{Type: "h", ForName: "target", Val: 75}}, ctx)
	if a.Position.Height != 75 {
		t.Fatalf("named constraint height=%v, want 75", a.Position.Height)
	}
}

func TestConnectorAlgorithmPreservesRoutingParameters(t *testing.T) {
	root := &smartart.Node{ID: "root"}
	a := &smartart.Node{ID: "a", Parent: root}
	b := &smartart.Node{ID: "b", Parent: root}
	root.Children = []*smartart.Node{a, b}
	d := &smartart.Diagram{
		Nodes: []*smartart.Node{root, a, b},
		Layout: &smartart.Layout{Definition: &smartart.LayoutDefinition{Root: &smartart.LayoutNode{
			Algorithm: &smartart.Algorithm{Type: "conn", Params: map[string]string{
				"connRout": "curve", "bendPt": "end", "begPts": "midR", "endPts": "midL",
				"dim": "2D", "begSty": "arr", "endSty": "noArr",
			}},
		}}},
	}
	NewLayoutEngine(d).CalculatePositions()
	got := d.Layout
	if got.ConnectorRouting != "curve" || got.ConnectorBendPoint != "end" {
		t.Fatalf("routing parameters not preserved: %+v", got)
	}
	if got.ConnectorBeginPts != "midR" || got.ConnectorEndPts != "midL" || got.ConnectorDimension != "2D" {
		t.Fatalf("endpoint/dimension parameters not preserved: %+v", got)
	}
	if got.ConnectorBeginStyle != "arr" || got.ConnectorEndStyle != "noArr" {
		t.Fatalf("arrow style parameters not preserved: %+v", got)
	}
}

func TestConnectorAlgorithmResolvesNamedSourceAndDestinationSets(t *testing.T) {
	a := &smartart.Node{ID: "a"}
	b := &smartart.Node{ID: "b"}
	d := &smartart.Diagram{Layout: &smartart.Layout{}}
	e := NewLayoutEngine(d)
	ctx := &layoutContext{
		nodes: []*smartart.Node{a, b},
		named: map[string][]*smartart.Node{"source": {a}, "target": {b}},
	}
	if !e.runAlgorithm(&smartart.Algorithm{Type: "conn", Params: map[string]string{"srcNode": "source", "dstNode": "target"}}, ctx) {
		t.Fatal("connector algorithm failed")
	}
	if len(d.Layout.ConnectorSourceIDs) != 1 || d.Layout.ConnectorSourceIDs[0] != "a" || len(d.Layout.ConnectorDestIDs) != 1 || d.Layout.ConnectorDestIDs[0] != "b" {
		t.Fatalf("named connector sets not resolved: src=%v dst=%v", d.Layout.ConnectorSourceIDs, d.Layout.ConnectorDestIDs)
	}
}

func TestPresentationOfHonorsStartStepCountAndPointType(t *testing.T) {
	nodes := []*smartart.Node{
		testNode("1", "node", 1), testNode("a", "asst", 1), testNode("2", "node", 1),
		testNode("3", "node", 1), testNode("4", "node", 1), testNode("5", "node", 1),
	}
	e := &LayoutEngine{}
	ctx := &layoutContext{nodes: nodes, vars: map[string]string{}, named: map[string][]*smartart.Node{}}
	got := e.presentationOfNodes(&smartart.PresentationOf{Axis: "self", PtType: "node", Start: 2, Step: 2, Cnt: 2}, ctx)
	if len(got) != 2 || got[0].ID != "2" || got[1].ID != "4" {
		t.Fatalf("presentation selection = %#v; want nodes 2 and 4", got)
	}
}

func TestTextSizeRulesApplyPrimaryAndSecondaryParagraphs(t *testing.T) {
	n := testNode("n", "node", 1)
	n.Paragraphs = []*smartart.TextParagraph{
		{Level: 0, Runs: []smartart.TextRun{{Text: "primary", FontSize: 1800}}},
		{Level: 1, Runs: []smartart.TextRun{{Text: "secondary", FontSize: 1600}}},
	}
	e := &LayoutEngine{}
	ctx := &layoutContext{nodes: []*smartart.Node{n}, vars: map[string]string{}, named: map[string][]*smartart.Node{}}
	e.applyRules([]smartart.Rule{{Type: "primFontSz", Val: 24}, {Type: "secFontSz", Val: 14}}, ctx)
	if n.Paragraphs[0].Runs[0].FontSize != 2400 {
		t.Fatalf("primary font size=%d, want 2400", n.Paragraphs[0].Runs[0].FontSize)
	}
	if n.Paragraphs[1].Runs[0].FontSize != 1400 {
		t.Fatalf("secondary font size=%d, want 1400", n.Paragraphs[1].Runs[0].FontSize)
	}
}

func TestLayoutShapeAppliesGeometryAndDrawingRotation(t *testing.T) {
	n := testNode("n", "node", 1)
	e := &LayoutEngine{}
	e.applyLayoutShape(&smartart.LayoutShape{Type: "ellipse", Rot: 5400000}, &layoutContext{nodes: []*smartart.Node{n}})
	if n.Geometry != "ellipse" {
		t.Fatalf("geometry=%q, want ellipse", n.Geometry)
	}
	if n.RotationDeg != 90 {
		t.Fatalf("rotation=%v, want 90", n.RotationDeg)
	}
}

func TestUnsupportedLayoutAlgorithmProducesDiagnosticAndFallback(t *testing.T) {
	root := testNode("root", "root", 0)
	a := testNode("a", "node", 1)
	a.Parent = root
	root.Children = []*smartart.Node{a}
	d := &smartart.Diagram{
		Nodes: []*smartart.Node{root, a},
		Layout: &smartart.Layout{Type: "hierarchy", Definition: &smartart.LayoutDefinition{Root: &smartart.LayoutNode{
			Algorithm: &smartart.Algorithm{Type: "futureAlgorithm"},
		}}},
	}
	NewLayoutEngine(d).CalculatePositions()
	if len(d.Diagnostics) == 0 || d.Diagnostics[0].Code != "smartart.unsupported-layout-algorithm" {
		t.Fatalf("diagnostics=%#v, want unsupported algorithm diagnostic", d.Diagnostics)
	}
	if a.Position.Width <= 0 || a.Position.Height <= 0 {
		t.Fatal("expected heuristic fallback to size and position node")
	}
}
