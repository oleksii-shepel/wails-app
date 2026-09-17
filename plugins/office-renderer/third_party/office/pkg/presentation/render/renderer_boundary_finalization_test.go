package render

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRendererConsumesCanonicalIRThroughSemanticSourceOnly closes the renderer
// boundary: the slide-space Shape clone belongs to layout. Renderers may read
// canonical semantic/content state only through SourceShape; geometry,
// composition, paint, transforms and effects come from physical plans.
func TestRendererConsumesCanonicalIRThroughSemanticSourceOnly(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(".", name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Shape" {
				return true
			}
			id, ok := sel.X.(*ast.Ident)
			if ok && id.Name == "item" {
				t.Errorf("%s:%d renderer consumes layout-owned item.Shape; use item.SourceShape for semantic content and physical plans for composition", name, fset.Position(sel.Pos()).Line)
			}
			return true
		})
	}
}

func TestRendererCanonicalShapeSelectorsAreSemanticOnly(t *testing.T) {
	allowed := map[string]bool{
		"ActionLink": true, "AnimStep": true, "AnimationTriggers": true,
		"EmphasisEffects": true, "HyperlinkURL": true, "ID": true,
		"ImageData": true, "ImageMIME": true,
		"MediaData": true, "MediaKind": true, "MediaLinkURL": true,
		"MediaMIME": true, "MediaPosterData": true, "MediaPosterMIME": true,
		"MotionPath": true, "Name": true, "OLEIconData": true,
		"OLEIconMIME": true, "OLEObjectLinkURL": true, "OLEObjectName": true,
		"OLEObjectProgID": true, "Paragraphs": true,
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(".", name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			id, ok := sel.X.(*ast.Ident)
			if !ok || id.Name != "shape" {
				return true
			}
			if !allowed[sel.Sel.Name] {
				t.Errorf("%s:%d renderer canonical shape selector %q is not classified as semantic/content", name, fset.Position(sel.Pos()).Line, sel.Sel.Name)
			}
			return true
		})
	}
}


func TestClass16TableAndChartRenderersUsePhysicalSemanticPlans(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil { t.Fatal(err) }
	body := string(data)
	functionBody := func(name string) string {
		start := strings.Index(body, "func "+name)
		if start < 0 { return "" }
		open := strings.Index(body[start:], "{")
		if open < 0 { return "" }
		open += start
		depth := 0
		for i := open; i < len(body); i++ {
			switch body[i] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 { return body[start:i+1] }
			}
		}
		return ""
	}
	for _, name := range []string{"renderTableShape", "renderChartShape"} {
		fn := functionBody(name)
		if fn == "" { t.Fatalf("could not isolate %s", name) }
		for _, forbidden := range []string{"shape.Table", "shape.ChartData", "shape.ImageData"} {
			if strings.Contains(fn, forbidden) {
				t.Fatalf("%s re-derives canonical semantic data via %q", name, forbidden)
			}
		}
	}
}


func TestClass17SlideRendererConsumesUnifiedPhysicalPaintOrder(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil { t.Fatal(err) }
	body := string(data)
	if !strings.Contains(body, "renderPhysicalSlidePaintOrder(b, physicalSlide") {
		t.Fatal("slide renderer does not delegate to unified physical paint-order serializer")
	}
	start := strings.Index(body, "func renderPhysicalSlidePaintOrder")
	if start < 0 { t.Fatal("paint-order serializer not found") }
	open := strings.Index(body[start:], "{")
	if open < 0 { t.Fatal("paint-order serializer body not found") }
	open += start
	depth := 0
	end := -1
	for i := open; i < len(body); i++ {
		switch body[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 { end = i + 1; break }
		}
		if end >= 0 { break }
	}
	if end < 0 { t.Fatal("paint-order serializer end not found") }
	fn := body[start:end]
	if !strings.Contains(fn, "slide.PaintOrder") {
		t.Fatal("serializer does not iterate unified physical paint order")
	}
	if strings.Contains(fn, "slide.Content") || strings.Contains(fn, "slide.Decorations") {
		t.Fatal("serializer regroups content/decorations")
	}
}


func TestClass18RendererDoesNotRecomputePhysicalClipVisibility(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil { t.Fatal(err) }
	body := string(data)
	for _, forbidden := range []string{
		"resolveClippedPhysicalPaintBounds",
		"physicalPolygonIntersectsBounds",
		"intersectPhysicalPaintBounds",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("renderer recomputes layout-owned clip visibility via %q", forbidden)
		}
	}
}


func TestClass18RendererConsumesPhysicalFullyClippedDecision(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	start := strings.Index(body, "func renderPhysicalVisual(")
	if start < 0 {
		t.Fatal("renderPhysicalVisual not found")
	}
	open := strings.Index(body[start:], "{")
	if open < 0 {
		t.Fatal("renderPhysicalVisual body not found")
	}
	open += start
	depth := 0
	end := -1
	for i := open; i < len(body); i++ {
		switch body[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = i + 1
				i = len(body)
			}
		}
	}
	if end < 0 {
		t.Fatal("renderPhysicalVisual body is incomplete")
	}
	fn := body[start:end]
	if !strings.Contains(fn, "item.Drawing.Paint.PaintClipped") ||
		!strings.Contains(fn, "item.Drawing.Paint.ClippedBounds.Empty()") {
		t.Fatal("renderer does not consume layout-owned fully-clipped visibility decision")
	}
}


func TestClass19RendererDispatchesOnPhysicalKind(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	start := strings.Index(body, "func renderShapeContent(")
	if start < 0 {
		t.Fatal("renderShapeContent not found")
	}
	open := strings.Index(body[start:], "{")
	if open < 0 {
		t.Fatal("renderShapeContent body not found")
	}
	open += start
	depth := 0
	end := -1
	for i := open; i < len(body); i++ {
		switch body[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = i + 1
				i = len(body)
			}
		}
	}
	if end < 0 {
		t.Fatal("renderShapeContent body is incomplete")
	}
	fn := body[start:end]
	if !strings.Contains(fn, "switch item.Kind") {
		t.Fatal("renderer does not dispatch on physical visual kind")
	}
	if strings.Contains(fn, "shape.Kind") || strings.Contains(fn, "item.SourceShape.Kind") {
		t.Fatal("renderer reopens canonical shape kind for dispatch")
	}
}
