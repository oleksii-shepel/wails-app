package smartart

import "testing"

func TestLayoutForEachPreservesIterationControls(t *testing.T) {
	layoutXML := []byte(`<dgm:layoutDef xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" uniqueId="u1">
		<dgm:layoutNode name="root">
			<dgm:forEach name="items" axis="ch" ptType="node" cnt="3" st="2" step="2" hideLastTrans="1" ref="shared">
				<dgm:layoutNode name="item"/>
			</dgm:forEach>
		</dgm:layoutNode>
	</dgm:layoutDef>`)
	layout := &Layout{}
	parseSmartArtLayout(layoutXML, layout)
	if layout.Definition == nil || layout.Definition.Root == nil || layout.Definition.Root.ForEach == nil {
		t.Fatal("expected parsed forEach")
	}
	fe := layout.Definition.Root.ForEach
	if fe.Cnt != 3 || fe.Start != 2 || fe.Step != 2 || !fe.HideLastTrans || fe.Ref != "shared" {
		t.Fatalf("unexpected forEach = %#v", fe)
	}
}

func TestLayoutRulesPreserveTargetMetadata(t *testing.T) {
	layoutXML := []byte(`<dgm:layoutDef xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram">
		<dgm:layoutNode name="root">
			<dgm:ruleLst><dgm:rule type="w" for="ch" forName="items" ptType="node" val="200" fact="0.5" max="80"/></dgm:ruleLst>
		</dgm:layoutNode>
	</dgm:layoutDef>`)
	layout := &Layout{}
	parseSmartArtLayout(layoutXML, layout)
	rules := layout.Definition.Root.Rules
	if len(rules) != 1 {
		t.Fatalf("rules=%d, want 1", len(rules))
	}
	r := rules[0]
	if r.For != "ch" || r.ForName != "items" || r.PtType != "node" || r.Val != 200 || r.Fact != 0.5 || r.Max != 80 {
		t.Fatalf("unexpected rule = %#v", r)
	}
}

func TestPresentationOfPreservesIterationControls(t *testing.T) {
	layoutXML := []byte(`<dgm:layoutDef xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram">
		<dgm:layoutNode name="root">
			<dgm:presOf axis="desOrSelf" ptType="node" cnt="2" st="2" step="3"/>
		</dgm:layoutNode>
	</dgm:layoutDef>`)
	layout := &Layout{}
	parseSmartArtLayout(layoutXML, layout)
	if layout.Definition == nil || layout.Definition.Root == nil || layout.Definition.Root.PresOf == nil {
		t.Fatal("expected parsed presOf")
	}
	p := layout.Definition.Root.PresOf
	if p.Axis != "desOrSelf" || p.PtType != "node" || p.Cnt != 2 || p.Start != 2 || p.Step != 3 {
		t.Fatalf("unexpected presOf = %#v", p)
	}
}
