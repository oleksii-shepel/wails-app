package layout

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestPhysicalVisualDoesNotRetainSupersededCompositionAliases(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "physical_visual.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	var physicalVisual *ast.StructType
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name.Name != "PhysicalVisual" {
				continue
			}
			physicalVisual, _ = typeSpec.Type.(*ast.StructType)
		}
	}
	if physicalVisual == nil {
		t.Fatal("PhysicalVisual declaration not found")
	}

	fields := make(map[string]struct{})
	for _, field := range physicalVisual.Fields.List {
		for _, name := range field.Names {
			fields[name.Name] = struct{}{}
		}
	}

	for _, forbidden := range []string{
		"PaintIndex",
		"Transform",
		"Visual",
		"ClipKind",
		"Clip",
		"OverflowVisible",
		"Overflow",
		"Ancestry",
		"Paint",
	} {
		if _, ok := fields[forbidden]; ok {
			t.Fatalf("PhysicalVisual retained superseded composition alias %q", forbidden)
		}
	}
	for _, required := range []string{"Drawing"} {
		if _, ok := fields[required]; !ok {
			t.Fatalf("PhysicalVisual lost authoritative composition contract %q", required)
		}
	}
}
