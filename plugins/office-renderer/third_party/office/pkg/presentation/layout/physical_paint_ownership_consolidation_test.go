package layout

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestPhysicalVisualPaintIsOwnedByDrawingComposition(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "physical_visual.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name.Name != "PhysicalVisual" {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				t.Fatal("PhysicalVisual is not a struct")
			}
			for _, field := range st.Fields.List {
				for _, name := range field.Names {
					if name.Name == "Paint" {
						t.Fatal("PhysicalVisual retained duplicate top-level Paint; use Drawing.PhysicalCompositionPlan.Paint")
					}
				}
			}
			return
		}
	}
	t.Fatal("PhysicalVisual declaration not found")
}
