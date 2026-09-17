package layout

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestPhysicalVisualDoesNotDuplicatePaintAncestry(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "physical_visual.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok || ts.Name.Name != "PhysicalVisual" {
			return true
		}
		found = true
		st, ok := ts.Type.(*ast.StructType)
		if !ok {
			t.Fatal("PhysicalVisual is not a struct")
		}
		for _, field := range st.Fields.List {
			for _, name := range field.Names {
				if name.Name == "Ancestry" {
					t.Fatal("PhysicalVisual retained duplicate Ancestry contract; use Paint.Ancestors/AncestorClips")
				}
			}
		}
		return false
	})
	if !found {
		t.Fatal("PhysicalVisual declaration not found")
	}
}
