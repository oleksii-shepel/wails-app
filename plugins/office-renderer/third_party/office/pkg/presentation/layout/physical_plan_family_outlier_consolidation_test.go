package layout

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestImageAndConnectorPlansUseCommonCompositionContract(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "shape_visual.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	targets := map[string]bool{"ImageVisualPlan": false, "ConnectorVisualPlan": false, "FreeConnectorVisualPlan": false}
	forbidden := map[string]bool{"Paint": true, "ContainerBounds": true, "Transform": true, "ShapeEffects": true, "RotationDeg": true}
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if _, ok := targets[ts.Name.Name]; !ok {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				t.Fatalf("%s is not a struct", ts.Name.Name)
			}
			has := false
			for _, field := range st.Fields.List {
				for _, name := range field.Names {
					if name.Name == "Composition" {
						has = true
					}
					if forbidden[name.Name] {
						t.Fatalf("%s retained superseded composition field %q", ts.Name.Name, name.Name)
					}
				}
			}
			if !has {
				t.Fatalf("%s does not own PhysicalCompositionPlan through Composition", ts.Name.Name)
			}
			targets[ts.Name.Name] = true
		}
	}
	for name, found := range targets {
		if !found {
			t.Fatalf("missing %s", name)
		}
	}
}
