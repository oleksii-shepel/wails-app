package render

import (
	"os"
	"strings"
	"testing"
)

func TestOLERendererConsumesPhysicalFrameTransformEffectsAndPaint(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	body := functionBody(t, string(data), "renderOleObjectShape", "oleObjectIconSource")
	for _, forbidden := range []string{
		"boundsPercent(shape.Bounds", "shape.RotationDeg", "shape.FlipH", "shape.FlipV",
		"effectStyleProps(ctx, shape)", "appendPhysicalPaintStyles(styles, ctx, shape)",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("OLE renderer retained canonical composition %q", forbidden)
		}
	}
	for _, required := range []string{
		"item.OLE", "physicalBoundsPercent(ole.FrameBounds",
		"ole.Transform.RotationDeg", "ole.Transform.FlipH", "presentationEffectPlanStyles(ole.Effects)",
		"appendOLEPhysicalPaintStyles",
	} {
		if !strings.Contains(body, required) {
			t.Fatalf("OLE renderer must consume physical composition %q", required)
		}
	}
}


func TestClass15OLERendererConsumesPhysicalPayload(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil { t.Fatal(err) }
	body := string(data)
	for _, forbidden := range []string{
		"shape.OLEIconData", "shape.OLEIconMIME", "shape.OLEObjectLinkURL",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("OLE renderer still reads canonical payload %q", forbidden)
		}
	}
}
