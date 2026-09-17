package layout

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func TestPhysicalCompositionHasSingleMaterializer(t *testing.T) {
	data, err := os.ReadFile("physical_visual.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	if !strings.Contains(src, "composition := materializePhysicalCompositionPlan(shape, paint, transform, visual.Effects)") {
		t.Fatal("physical visual tree must materialize shared composition once from already-resolved state")
	}

	f, err := parser.ParseFile(token.NewFileSet(), "physical_visual.go", data, 0)
	if err != nil {
		t.Fatal(err)
	}
	populatedConstructors := 0
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		id, ok := lit.Type.(*ast.Ident)
		if ok && id.Name == "PhysicalCompositionPlan" && len(lit.Elts) != 0 {
			populatedConstructors++
		}
		return true
	})
	// Empty zero-value returns are harmless. Exactly one populated literal is
	// allowed: the authoritative constructor in materializePhysicalCompositionPlan.
	if populatedConstructors != 1 {
		t.Fatalf("physical_visual.go has %d populated PhysicalCompositionPlan literals; want exactly one canonical materializer", populatedConstructors)
	}
}

func TestSpecializedPlansUseCanonicalCompositionResolver(t *testing.T) {
	for _, name := range []string{"physical_visual.go", "text_visual.go", "shape_visual.go"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		src := string(data)
		if strings.Contains(src, ".PhysicalCompositionPlan = PhysicalCompositionPlan{") || strings.Contains(src, "Composition: PhysicalCompositionPlan{") {
			t.Fatalf("%s constructs physical composition outside canonical materializer", name)
		}
	}
}
