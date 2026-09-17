package render

import (
	"os"
	"strings"
	"testing"
)

func functionBody(t *testing.T, src, name, next string) string {
	t.Helper()
	start := strings.Index(src, "func "+name+"(")
	if start < 0 {
		t.Fatalf("%s not found", name)
	}
	end := strings.Index(src[start:], "\nfunc "+next+"(")
	if end < 0 {
		t.Fatalf("end of %s not found", name)
	}
	return src[start : start+end]
}

func TestImageRendererConsumesPhysicalContainerTransformEffectsAndPaint(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	body := functionBody(t, string(data), "renderImageShape", "renderMediaShape")
	for _, forbidden := range []string{"boundsPercent(shape.Bounds", "shape.RotationDeg", "shape.FlipH", "shape.FlipV", "effectStyleProps(ctx, shape)", "appendPhysicalPaintStyles(styles, ctx, shape)"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("image renderer retained canonical composition %q", forbidden)
		}
	}
	for _, required := range []string{"item.Image", "physicalBoundsPercent(imageVisual.Composition.FrameBounds", "imageVisual.Composition.Transform.RotationDeg", "imageVisual.Composition.Transform.FlipH", "presentationPictureEffectPlanStyles(imageVisual.Composition.Effects)", "appendImagePhysicalPaintStyles(styles, imageVisual)"} {
		if !strings.Contains(body, required) {
			t.Fatalf("image renderer must consume physical composition %q", required)
		}
	}
}

func TestMediaRendererConsumesPhysicalFrameTransformEffectsAndPaint(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	body := functionBody(t, string(data), "renderMediaShape", "renderOleObjectShape")
	for _, forbidden := range []string{"boundsPercent(shape.Bounds", "shape.RotationDeg", "shape.FlipH", "shape.FlipV", "effectStyleProps(ctx, shape)", "appendPhysicalPaintStyles(styles, ctx, shape)"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("media renderer retained canonical composition %q", forbidden)
		}
	}
	for _, required := range []string{"item.Media", "physicalBoundsPercent(media.FrameBounds", "media.Transform.RotationDeg", "media.Transform.FlipH", "presentationEffectPlanStyles(media.Effects)", "appendMediaPhysicalPaintStyles(styles, media)"} {
		if !strings.Contains(body, required) {
			t.Fatalf("media renderer must consume physical composition %q", required)
		}
	}
}
