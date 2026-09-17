package layout

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestPhysicalShapePlanUsesSharedCompositionContract(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "physical_visual.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var st *ast.StructType
	ast.Inspect(f, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok || ts.Name.Name != "PhysicalShapePlan" {
			return true
		}
		st, _ = ts.Type.(*ast.StructType)
		return false
	})
	if st == nil {
		t.Fatal("PhysicalShapePlan declaration not found")
	}
	forbidden := map[string]bool{"FrameBounds": true, "Transform": true, "Effects": true, "Paint": true, "Clip": true}
	hasComposition := false
	hasVisual := false
	for _, field := range st.Fields.List {
		if len(field.Names) == 0 {
			if id, ok := field.Type.(*ast.Ident); ok && id.Name == "PhysicalCompositionPlan" {
				hasComposition = true
			}
			continue
		}
		for _, name := range field.Names {
			if forbidden[name.Name] {
				t.Fatalf("PhysicalShapePlan retained duplicated composition field %q", name.Name)
			}
			if name.Name == "Visual" {
				hasVisual = true
			}
		}
	}
	if !hasComposition {
		t.Fatal("PhysicalShapePlan must embed PhysicalCompositionPlan")
	}
	if !hasVisual {
		t.Fatal("PhysicalShapePlan must retain shape-specific Visual payload")
	}
}
