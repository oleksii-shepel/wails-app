package smartart

import (
	"testing"
)

func TestParseDiagramRepairsDuplicateNodeIDs(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram">
		<dgm:ptLst>
			<dgm:pt modelId="1" id="p1"><dgm:prSet><dgm:fld>A</dgm:fld></dgm:prSet></dgm:pt>
			<dgm:pt modelId="1" id="p2"><dgm:prSet><dgm:fld>B</dgm:fld></dgm:prSet></dgm:pt>
			<dgm:pt modelId="3" id="p3"><dgm:prSet><dgm:fld>C</dgm:fld></dgm:prSet></dgm:pt>
		</dgm:ptLst>
		<dgm:cxnLst>
			<dgm:cxn srcId="1" destId="3"/>
		</dgm:cxnLst>
	</dgm:dataModel>`)

	diagram, err := ParseDiagram(dataModelXML)
	if err != nil {
		t.Fatalf("ParseDiagram: %v", err)
	}
	if len(diagram.Nodes) != 3 {
		t.Fatalf("nodes = %d, want 3", len(diagram.Nodes))
	}
	ids := make(map[string]int)
	for _, n := range diagram.Nodes {
		ids[n.ID]++
	}
	for id, count := range ids {
		if count > 1 {
			t.Errorf("duplicate node id %q still present", id)
		}
	}
	if diagram.Nodes[1].ID != "1-dup1" {
		t.Errorf("second duplicate id = %q, want 1-dup1", diagram.Nodes[1].ID)
	}
	if len(diagram.Diagnostics) == 0 {
		t.Errorf("expected diagnostic for duplicate id repair")
	}
}

func TestParseDiagramOrdersChildrenBySrcOrd(t *testing.T) {
	dataModelXML := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram">
		<dgm:ptLst>
			<dgm:pt modelId="root" id="r"><dgm:prSet><dgm:fld>Root</dgm:fld></dgm:prSet></dgm:pt>
			<dgm:pt modelId="c1" id="c1"><dgm:prSet><dgm:fld>C1</dgm:fld></dgm:prSet></dgm:pt>
			<dgm:pt modelId="c2" id="c2"><dgm:prSet><dgm:fld>C2</dgm:fld></dgm:prSet></dgm:pt>
			<dgm:pt modelId="c3" id="c3"><dgm:prSet><dgm:fld>C3</dgm:fld></dgm:prSet></dgm:pt>
		</dgm:ptLst>
		<dgm:cxnLst>
			<dgm:cxn srcId="root" destId="c2" srcOrd="1"/>
			<dgm:cxn srcId="root" destId="c3" srcOrd="2"/>
			<dgm:cxn srcId="root" destId="c1" srcOrd="0"/>
		</dgm:cxnLst>
	</dgm:dataModel>`)

	diagram, err := ParseDiagram(dataModelXML)
	if err != nil {
		t.Fatalf("ParseDiagram: %v", err)
	}
	root := diagram.Nodes[0]
	if len(root.Children) != 3 {
		t.Fatalf("root children = %d, want 3", len(root.Children))
	}
	want := []string{"c1", "c2", "c3"}
	for i, child := range root.Children {
		if child.ID != want[i] {
			t.Errorf("child[%d] = %q, want %q", i, child.ID, want[i])
		}
	}
}
