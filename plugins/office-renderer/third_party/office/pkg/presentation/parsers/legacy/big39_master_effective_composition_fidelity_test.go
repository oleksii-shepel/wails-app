package legacy

import (
	"testing"

	officeartir "office/pkg/officeart/ir"
	"office/pkg/ooxml/drawing"
	presentationir "office/pkg/presentation/ir"
)

func TestBIG39MasterPlaceholderCarriesEffectiveTextBodySemantics(t *testing.T) {
	inherited := legacyPptShapeTransform{
		HasTextAnchor: true, TextAnchor: "ctr",
		HasTextInsets: true, TextInsetLeft: 11, TextInsetTop: 22, TextInsetRight: 33, TextInsetBottom: 44,
		HasAllowOverflow: true, AllowOverflow: true,
		HasAutoGrow: true, AutoGrow: true,
	}
	got := mergeLegacyPptPlaceholderTransform(legacyPptShapeTransform{}, inherited)
	if !got.HasTextAnchor || got.TextAnchor != "ctr" {
		t.Fatalf("text anchor = %#v", got)
	}
	if !got.HasTextInsets || got.TextInsetLeft != 11 || got.TextInsetTop != 22 || got.TextInsetRight != 33 || got.TextInsetBottom != 44 {
		t.Fatalf("text insets = %#v", got)
	}
	if !got.HasAllowOverflow || !got.AllowOverflow || !got.HasAutoGrow || !got.AutoGrow {
		t.Fatalf("text fitting = %#v", got)
	}
}

func TestBIG39SlidePlaceholderOverridesMasterTextBodySemantics(t *testing.T) {
	local := legacyPptShapeTransform{HasTextAnchor: true, TextAnchor: "b", HasTextInsets: true, TextInsetLeft: 1, TextInsetTop: 2, TextInsetRight: 3, TextInsetBottom: 4, HasAutoGrow: true, AutoGrow: false}
	inherited := legacyPptShapeTransform{HasTextAnchor: true, TextAnchor: "ctr", HasTextInsets: true, TextInsetLeft: 11, TextInsetTop: 22, TextInsetRight: 33, TextInsetBottom: 44, HasAutoGrow: true, AutoGrow: true}
	got := mergeLegacyPptPlaceholderTransform(local, inherited)
	if got.TextAnchor != "b" || got.TextInsetLeft != 1 || got.TextInsetTop != 2 || got.TextInsetRight != 3 || got.TextInsetBottom != 4 || got.AutoGrow {
		t.Fatalf("local placeholder semantics were overwritten: %#v", got)
	}
}

func TestBIG39MasterArtworkIsInstancedPerSlide(t *testing.T) {
	masterChild := &presentationir.Shape{Kind: presentationir.ShapeText, Name: "child"}
	master := &presentationir.Shape{Kind: presentationir.ShapeGroup, Name: "master", Children: []*presentationir.Shape{masterChild}}
	slides := []legacySlideText{{PersistID: 1, MasterID: 10, MasterPersistID: 110}, {PersistID: 2, MasterID: 10, MasterPersistID: 110}}
	got := composeLegacyPptMasterVisuals(slides, nil, map[int][]*presentationir.Shape{110: {master}})
	if len(got[1]) != 1 || len(got[2]) != 1 {
		t.Fatalf("master composition = %#v", got)
	}
	if got[1][0] == got[2][0] || got[1][0] == master {
		t.Fatal("master root reused across slide instances")
	}
	if len(got[1][0].Children) != 1 || len(got[2][0].Children) != 1 || got[1][0].Children[0] == got[2][0].Children[0] {
		t.Fatal("master child reused across slide instances")
	}
}

func TestBIG39MasterArtworkDeepClonesSharedOfficeArtGraph(t *testing.T) {
	sharedChild := &officeartir.Shape{ID: 8, Geometry: officeartir.Geometry{Adjustments: []float64{0.25}}}
	shared := &officeartir.Shape{
		ID:       7,
		Geometry: officeartir.Geometry{Paths: []officeartir.Path{{Commands: []officeartir.PathCommand{{Op: "M", Points: []officeartir.PointF{{X: 1, Y: 2}}}}}}},
		Fill:     officeartir.Fill{Gradient: &officeartir.GradientFill{Stops: []officeartir.GradientStop{{Position: 0.5, Color: "112233"}}}},
		Image:    &officeartir.Image{Data: []byte{1, 2, 3}},
		Children: []*officeartir.Shape{sharedChild},
	}
	master := &presentationir.Shape{Kind: presentationir.ShapeGroup, OfficeArt: shared}
	got := composeLegacyPptMasterVisuals(
		[]legacySlideText{{PersistID: 1, MasterID: 10, MasterPersistID: 110}, {PersistID: 2, MasterID: 10, MasterPersistID: 110}},
		nil,
		map[int][]*presentationir.Shape{110: {master}},
	)
	left, right := got[1][0].OfficeArt, got[2][0].OfficeArt
	if left == shared || right == shared || left == right {
		t.Fatal("shared OfficeArt root reused across master instances")
	}
	if left.Children[0] == sharedChild || right.Children[0] == sharedChild || left.Children[0] == right.Children[0] {
		t.Fatal("shared OfficeArt child reused across master instances")
	}
	left.Geometry.Paths[0].Commands[0].Points[0].X = 99
	left.Fill.Gradient.Stops[0].Color = "FFFFFF"
	left.Image.Data[0] = 9
	left.Children[0].Geometry.Adjustments[0] = 0.75
	if right.Geometry.Paths[0].Commands[0].Points[0].X != 1 || right.Fill.Gradient.Stops[0].Color != "112233" || right.Image.Data[0] != 1 || right.Children[0].Geometry.Adjustments[0] != 0.25 {
		t.Fatal("mutation leaked between per-slide OfficeArt instances")
	}
}

func TestBIG39MasterConnectorEndpointsAndPathsAreIndependent(t *testing.T) {
	start := &presentationir.Point{X: 10, Y: 20}
	master := &presentationir.Shape{
		Kind:                 presentationir.ShapeConnector,
		ConnectorStart:       start,
		ConnectorPath:        []presentationir.Point{{X: 1, Y: 2}, {X: 3, Y: 4}},
		ConnectorAdjustments: []int{25000},
	}
	got := composeLegacyPptMasterVisuals(
		[]legacySlideText{{PersistID: 1, MasterID: 10, MasterPersistID: 110}, {PersistID: 2, MasterID: 10, MasterPersistID: 110}}, nil,
		map[int][]*presentationir.Shape{110: {master}},
	)
	got[1][0].ConnectorStart.X = 999
	got[1][0].ConnectorPath[0].X = 999
	got[1][0].ConnectorAdjustments[0] = 999
	if got[2][0].ConnectorStart.X != 10 || got[2][0].ConnectorPath[0].X != 1 || got[2][0].ConnectorAdjustments[0] != 25000 {
		t.Fatal("connector composition state leaked between master instances")
	}
}

func TestBIG39FinalMasterInstanceOwnsPresentationMutableGraph(t *testing.T) {
	master := &presentationir.Shape{
		Kind:                presentationir.ShapeGroup,
		GeometryData:        &drawing.Geometry{Paths: []drawing.Path{{Commands: []drawing.PathCommand{{Type: "M", Points: []drawing.Point{{X: 1, Y: 2}}}}}}},
		GeometryAdjustments: []float64{0.25},
		PictureTile:         &presentationir.PictureTile{Tx: 10},
		GroupTransform:      &presentationir.VisualTransform{Rotation: 12},
		LineStyle:           &drawing.LineStyle{CompoundWidths: []float64{0.2, 0.8}},
		Paragraphs:          []*presentationir.Paragraph{{TabStops: []presentationir.TabStop{{Pos: 42}}, Runs: []*presentationir.Run{{Text: "master"}}}},
	}
	got := composeLegacyPptMasterVisuals(
		[]legacySlideText{{PersistID: 1, MasterID: 10, MasterPersistID: 110}, {PersistID: 2, MasterID: 10, MasterPersistID: 110}}, nil,
		map[int][]*presentationir.Shape{110: {master}},
	)
	left, right := got[1][0], got[2][0]
	left.GeometryData.Paths[0].Commands[0].Points[0].X = 99
	left.GeometryAdjustments[0] = 0.75
	left.PictureTile.Tx = 99
	left.GroupTransform.Rotation = 90
	left.LineStyle.CompoundWidths[0] = 0.9
	left.Paragraphs[0].TabStops[0].Pos = 99
	left.Paragraphs[0].Runs[0].Text = "changed"
	if right.GeometryData.Paths[0].Commands[0].Points[0].X != 1 || right.GeometryAdjustments[0] != 0.25 || right.PictureTile.Tx != 10 || right.GroupTransform.Rotation != 12 || right.LineStyle.CompoundWidths[0] != 0.2 || right.Paragraphs[0].TabStops[0].Pos != 42 || right.Paragraphs[0].Runs[0].Text != "master" {
		t.Fatal("presentation-side master mutation leaked between slide instances")
	}
}
