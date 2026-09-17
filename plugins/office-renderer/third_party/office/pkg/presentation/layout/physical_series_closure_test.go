package layout

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// TestPhysicalSeriesClosureRejectsCompatibilityAliases freezes the final A17-A45
// text-flow contract: physical axes and point-valued indentation are authoritative.
func TestPhysicalSeriesClosureRejectsCompatibilityAliases(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "text_flow.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name.Name != "TextFlowMetrics" {
				continue
			}
			st := ts.Type.(*ast.StructType)
			for _, field := range st.Fields.List {
				for _, name := range field.Names {
					if name.Name == "PhysicalLineWidth" {
						t.Fatal("TextFlowMetrics retained compatibility alias PhysicalLineWidth")
					}
				}
			}
		}
	}
}
