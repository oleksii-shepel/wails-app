package render

import (
	resolve "office/pkg/word/ir"
	"strings"
	"testing"
)

func TestPictureFrameGeometryStylesPreserveCircularFrame(t *testing.T) {
	img := &resolve.Image{FrameGeometry: "flowChartConnector"}
	styles := strings.Join(pictureFrameGeometryStyles(img), ";")
	if !strings.Contains(styles, "border-radius:50%") {
		t.Fatalf("styles = %q", styles)
	}
	if strings.Contains(styles, "border:") {
		t.Fatalf("frame geometry manufactured border: %q", styles)
	}
}
