package parts_test

import (
	"encoding/xml"
	"testing"

	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/drawingxml"
	"office/pkg/presentation/parsers/ooxml/parts"
)

func TestShapeTreePreservesXMLChildOrder(t *testing.T) {
	const src = `
<spTree xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <nvGrpSpPr/>
  <grpSpPr/>
  <cxnSp><nvCxnSpPr><cNvPr id="1" name="line"/></nvCxnSpPr><spPr/></cxnSp>
  <sp><nvSpPr><cNvPr id="2" name="shape"/></nvSpPr><spPr/></sp>
  <pic><nvPicPr><cNvPr id="3" name="pic"/></nvPicPr><blipFill/><spPr/></pic>
</spTree>`

	var tree parts.ShapeTree
	if err := xml.Unmarshal([]byte(src), &tree); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(tree.Items) != 3 {
		t.Fatalf("item count = %d, want 3", len(tree.Items))
	}
	if tree.Items[0].Kind != parts.ShapeTreeItemConnector || tree.Items[1].Kind != parts.ShapeTreeItemShape || tree.Items[2].Kind != parts.ShapeTreeItemPicture {
		t.Fatalf("unexpected order: %#v", tree.Items)
	}
}

func TestPresentationParsesColorMapsAndOverrides(t *testing.T) {
	const masterXML = `
<p:sldMaster xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
             xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:clrMap bg1="lt1" tx1="accent2" bg2="lt2" tx2="dk2" accent1="accent3" hlink="accent4" followedHlink="accent5"/>
  <p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld>
</p:sldMaster>`
	var master parts.SlideMaster
	if err := xml.Unmarshal([]byte(masterXML), &master); err != nil {
		t.Fatalf("unmarshal master: %v", err)
	}
	if master.ClrMap == nil || master.ClrMap.Tx1 != "accent2" || master.ClrMap.FollowedHlink != "accent5" {
		t.Fatalf("master clrMap = %#v, want parsed attributes", master.ClrMap)
	}

	const layoutXML = `
<p:sldLayout xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
             xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:clrMapOvr>
    <a:overrideClrMapping bg1="dk1" tx1="accent6" accent1="accent4"/>
  </p:clrMapOvr>
  <p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld>
</p:sldLayout>`
	var layout parts.SlideLayout
	if err := xml.Unmarshal([]byte(layoutXML), &layout); err != nil {
		t.Fatalf("unmarshal layout: %v", err)
	}
	if layout.ClrMapOvr == nil || layout.ClrMapOvr.OverrideClrMapping == nil {
		t.Fatalf("layout clrMapOvr = %#v, want override mapping", layout.ClrMapOvr)
	}
	if layout.ClrMapOvr.OverrideClrMapping.Tx1 != "accent6" {
		t.Fatalf("layout tx1 = %q, want accent6", layout.ClrMapOvr.OverrideClrMapping.Tx1)
	}

	const slideXML = `
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
       xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>
  <p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld>
</p:sld>`
	var slide parts.Slide
	if err := xml.Unmarshal([]byte(slideXML), &slide); err != nil {
		t.Fatalf("unmarshal slide: %v", err)
	}
	if slide.ClrMapOvr == nil || slide.ClrMapOvr.MasterClrMapping == nil {
		t.Fatalf("slide clrMapOvr = %#v, want master mapping", slide.ClrMapOvr)
	}
}

func TestPresetGeometryConvertsToSharedDrawingGeometry(t *testing.T) {
	geom := (&parts.PresetGeometry{
		Prst: "roundRect",
		AvLst: []parts.GeometryGuide{
			{Name: "adj", Fmla: "val 25000"},
		},
	}).DrawingPresetGeometry()
	if geom == nil {
		t.Fatal("DrawingPresetGeometry returned nil")
	}
	if geom.Name != "roundRect" {
		t.Fatalf("name = %q, want roundRect", geom.Name)
	}
	if len(geom.Adjustments) != 1 || geom.Adjustments[0] != 0.25 {
		t.Fatalf("adjustments = %#v, want [0.25]", geom.Adjustments)
	}

	full := (&parts.PresetGeometry{
		Prst: "triangle",
		AvLst: []parts.GeometryGuide{
			{Name: "adj1", Fmla: "val 50000"},
			{Name: "adj2", Fmla: "wd/2"},
		},
	}).DrawingGeometry()
	if full == nil {
		t.Fatal("DrawingGeometry returned nil")
	}
	if full.Type != drawing.GeometryTypePreset || full.Name != "triangle" {
		t.Fatalf("unexpected geometry = %#v", full)
	}
	if len(full.Guides) != 2 || full.Guides[0].Value != 50000 || full.Guides[1].Formula != "wd/2" {
		t.Fatalf("unexpected guides = %#v", full.Guides)
	}
}

func TestPresetGeometryOrdersNamedAdjustmentsByAdjSlot(t *testing.T) {
	geom := (&parts.PresetGeometry{
		Prst: "rightArrow",
		AvLst: []parts.GeometryGuide{
			{Name: "adj2", Fmla: "val 25000"},
			{Name: "adj1", Fmla: "val 75000"},
		},
	}).DrawingPresetGeometry()
	if geom == nil {
		t.Fatal("DrawingPresetGeometry returned nil")
	}
	if len(geom.Adjustments) != 2 {
		t.Fatalf("adjustment count = %d, want 2", len(geom.Adjustments))
	}
	if geom.Adjustments[0] != 0.75 || geom.Adjustments[1] != 0.25 {
		t.Fatalf("adjustments = %#v, want [0.75 0.25]", geom.Adjustments)
	}
}

func TestCustomGeometryConvertsToSharedDrawingGeometry(t *testing.T) {
	const src = `
<spPr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <a:custGeom>
    <a:gdLst>
      <a:gd name="adj1" fmla="val 50000"/>
    </a:gdLst>
    <a:pathLst>
      <a:path w="100" h="50">
        <a:moveTo><a:pt x="0" y="50"/></a:moveTo>
        <a:lnTo><a:pt x="50" y="0"/></a:lnTo>
        <a:lnTo><a:pt x="100" y="50"/></a:lnTo>
        <a:close/>
      </a:path>
    </a:pathLst>
  </a:custGeom>
</spPr>`

	var sp parts.ShapeProperties
	if err := xml.Unmarshal([]byte(src), &sp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	geom := sp.DrawingGeometry()
	if geom == nil {
		t.Fatal("DrawingGeometry returned nil")
	}
	if geom.Type != drawing.GeometryTypeCustom {
		t.Fatalf("geometry type = %q, want custom", geom.Type)
	}
	if len(geom.Guides) != 1 || geom.Guides[0].Name != "adj1" {
		t.Fatalf("unexpected guides = %#v", geom.Guides)
	}
	if len(geom.Paths) != 1 || len(geom.Paths[0].Commands) != 4 {
		t.Fatalf("unexpected paths = %#v", geom.Paths)
	}
	if geom.Paths[0].Commands[1].Points[0] != (drawing.Point{X: 0.5, Y: 0}) {
		t.Fatalf("unexpected normalized point = %#v", geom.Paths[0].Commands[1].Points[0])
	}
}

func TestPictureBlipFillParsesSourceRect(t *testing.T) {
	const src = `
<pic xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
	 xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
	 xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <nvPicPr><cNvPr id="1" name="Photo"/></nvPicPr>
  <blipFill>
    <a:blip r:embed="rId5"/>
    <a:srcRect l="12500" t="5000" r="25000" b="10000"/>
    <a:stretch><a:fillRect/></a:stretch>
  </blipFill>
  <spPr/>
</pic>`

	var pic parts.Picture
	if err := xml.Unmarshal([]byte(src), &pic); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if pic.BlipFill.Blip == nil || pic.BlipFill.Blip.Embed != "rId5" {
		t.Fatalf("unexpected embed = %#v", pic.BlipFill.Blip)
	}
	if pic.BlipFill.SrcRect == nil {
		t.Fatal("expected srcRect to be parsed")
	}
	if pic.BlipFill.SrcRect.L != 12500 || pic.BlipFill.SrcRect.T != 5000 || pic.BlipFill.SrcRect.R != 25000 || pic.BlipFill.SrcRect.B != 10000 {
		t.Fatalf("unexpected srcRect = %#v", pic.BlipFill.SrcRect)
	}
}

func TestShapePropertiesDrawingFillUsesSharedTypes(t *testing.T) {
	fill := (&parts.ShapeProperties{
		FillScheme: &drawingxml.SchemeClr{Val: "accent2"},
	}).DrawingFill(map[string]string{"accent2": "673AB7"})

	solid, ok := fill.(drawing.SolidFill)
	if !ok {
		t.Fatalf("fill type = %T, want drawing.SolidFill", fill)
	}
	if solid.Color != "673AB7" || solid.ThemeColor != "accent2" {
		t.Fatalf("unexpected fill = %#v", solid)
	}

	noFill := (&parts.ShapeProperties{NoFill: &struct{}{}}).DrawingFill(nil)
	if _, ok := noFill.(drawing.NoFill); !ok {
		t.Fatalf("noFill type = %T, want drawing.NoFill", noFill)
	}
}

func TestConnectorLineDrawingLineStyleUsesSharedTypes(t *testing.T) {
	style := (&parts.ConnectorLine{
		W:         25400,
		Cap:       "round",
		Cmpd:      "dbl",
		SolidFill: &drawingxml.SolidFill{SchemeClr: &drawingxml.SchemeClr{Val: "dk2"}},
		PrstDash:  &parts.PresetDash{Val: "dash"},
		Round:     &struct{}{},
	}).DrawingLineStyle(map[string]string{"dk2": "424242"})
	if style == nil {
		t.Fatal("DrawingLineStyle returned nil")
	}
	if style.Width != 2.0 || style.Color != "424242" || style.Cap != drawing.LineCapRound || style.Join != drawing.LineJoinRound || style.Dash != drawing.DashPatternDash || !style.Compound {
		t.Fatalf("unexpected style = %#v", style)
	}
}

func TestFillConversionParityWithDrawingXML(t *testing.T) {
	themeColors := map[string]string{"accent1": "4472C4", "dk1": "2F2F2F"}

	// Solid fill parity
	dxSolid := drawingxml.ResolveSolidFill(&drawingxml.SolidFill{SchemeClr: &drawingxml.SchemeClr{Val: "accent1"}}, themeColors)
	presSolid := (&parts.ShapeProperties{FillScheme: &drawingxml.SchemeClr{Val: "accent1"}}).DrawingFill(themeColors)
	if dxSolid == nil || presSolid == nil {
		t.Fatal("solid fill conversion returned nil")
	}
	if dxSolid.Color != presSolid.(drawing.SolidFill).Color {
		t.Errorf("solid fill color mismatch: drawingxml=%q presentation=%q", dxSolid.Color, presSolid.(drawing.SolidFill).Color)
	}

	// Gradient fill parity
	const gradXML = `<a:gradFill xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:gsLst>
			<a:gs pos="0"><a:srgbClr val="FF0000"/></a:gs>
			<a:gs pos="100000"><a:schemeClr val="accent1"/></a:gs>
		</a:gsLst>
		<a:lin ang="5400000"/>
	</a:gradFill>`
	var dxGrad drawingxml.GradientFill
	if err := xml.Unmarshal([]byte(gradXML), &dxGrad); err != nil {
		t.Fatalf("unmarshal drawingxml gradient: %v", err)
	}
	var presGrad parts.GradFill
	if err := xml.Unmarshal([]byte(gradXML), &presGrad); err != nil {
		t.Fatalf("unmarshal presentation gradient: %v", err)
	}
	dxGradResult := drawingxml.ResolveGradientFill(&dxGrad, themeColors)
	presGradResult := (&parts.ShapeProperties{GradFill: &presGrad}).DrawingFill(themeColors)
	if dxGradResult == nil || presGradResult == nil {
		t.Fatal("gradient fill conversion returned nil")
	}
	dxGF := *dxGradResult
	presGF := presGradResult.(drawing.GradientFill)
	if len(dxGF.Stops) != len(presGF.Stops) {
		t.Fatalf("gradient stop count mismatch: drawingxml=%d presentation=%d", len(dxGF.Stops), len(presGF.Stops))
	}
	for i := range dxGF.Stops {
		if dxGF.Stops[i].Color != presGF.Stops[i].Color {
			t.Errorf("gradient stop %d color mismatch: drawingxml=%q presentation=%q", i, dxGF.Stops[i].Color, presGF.Stops[i].Color)
		}
	}
	if dxGF.Angle != presGF.Angle {
		t.Errorf("gradient angle mismatch: drawingxml=%v presentation=%v", dxGF.Angle, presGF.Angle)
	}

	// Pattern fill parity
	const pattXML = `<a:pattFill xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" prst="pct5">
		<a:fgClr><a:srgbClr val="4472C4"/></a:fgClr>
		<a:bgClr><a:schemeClr val="dk1"/></a:bgClr>
	</a:pattFill>`
	var dxPatt drawingxml.PatternFill
	if err := xml.Unmarshal([]byte(pattXML), &dxPatt); err != nil {
		t.Fatalf("unmarshal drawingxml pattern: %v", err)
	}
	var presPatt parts.PattFill
	if err := xml.Unmarshal([]byte(pattXML), &presPatt); err != nil {
		t.Fatalf("unmarshal presentation pattern: %v", err)
	}
	dxPattResult := drawingxml.ResolvePatternFill(&dxPatt, themeColors)
	presPattResult := (&parts.ShapeProperties{PattFill: &presPatt}).DrawingFill(themeColors)
	if dxPattResult == nil || presPattResult == nil {
		t.Fatal("pattern fill conversion returned nil")
	}
	if dxPattResult.Foreground != presPattResult.(drawing.PatternFill).Foreground {
		t.Errorf("pattern foreground mismatch: drawingxml=%q presentation=%q", dxPattResult.Foreground, presPattResult.(drawing.PatternFill).Foreground)
	}
}

func TestShapePropertiesDrawingEffectListUsesSharedTypes(t *testing.T) {
	effects := (&parts.ShapeProperties{
		Scene3D: &parts.Scene3D{
			Camera: &parts.SceneCamera{
				Prst: "legacyPerspectiveTopLeft", FOV: 2700000, Zoom: 125000,
				Rot: &parts.SceneRotation{Lat: -900000, Lon: -600000},
			},
			LightRig: &parts.SceneLightRig{
				Rig: "threePt", Dir: "tl", Rot: &parts.SceneRotation{Lat: 300000, Lon: 600000, Rev: 900000},
			},
		},
		Shape3D: &parts.Shape3D{
			ExtrusionH:   76200,
			ContourW:     25400,
			PrstMaterial: "metal",
			BevelT:       &parts.Shape3DBevel{W: 38100, H: 12700, Prst: "relaxedInset"},
			BevelB:       &parts.Shape3DBevel{W: 12700, H: 6350, Prst: "angle"},
			ContourClr:   &parts.ColorRef{Val: "D9D9D9"},
			ExtrusionClr: &parts.ColorRef{Val: "595959"},
		},
		OuterShadow: &parts.OuterShadowEffect{
			BlurRad:   38100,
			Dist:      25400,
			Dir:       2700000,
			SchemeClr: &parts.EffectSchemeColor{Val: "accent1", Alpha: &parts.EffectAlpha{Val: 50000}},
		},
		Glow: &parts.GlowEffect{
			Rad:     50800,
			SrgbClr: &parts.EffectColorRef{Val: "FF0000"},
		},
		SoftEdge: &parts.SoftEdgeEffect{Rad: 12700},
		Reflection: &parts.ReflectionEffect{
			BlurRad: 25400,
			Dist:    38100,
			Dir:     5400000,
			StA:     25000,
		},
	}).DrawingEffectList(map[string]string{"accent1": "673AB7"})

	if len(effects.Effects) != 5 {
		t.Fatalf("effect count = %d, want 5", len(effects.Effects))
	}

	threeD, ok := effects.Effects[0].(drawing.ThreeD)
	if !ok {
		t.Fatalf("3D type = %T, want drawing.ThreeD", effects.Effects[0])
	}
	if threeD.CameraPreset != "legacyPerspectiveTopLeft" || threeD.LightDirection != "tl" || threeD.Material != "metal" {
		t.Fatalf("unexpected 3D effect = %#v", threeD)
	}
	if threeD.CameraFOV != 45 || threeD.CameraZoom != 1.25 {
		t.Fatalf("unexpected 3D camera metadata = %#v", threeD)
	}
	if threeD.LightRotationX != 5 || threeD.LightRotationY != 10 || threeD.LightRotationZ != 15 {
		t.Fatalf("unexpected 3D light rotation = %#v", threeD)
	}
	if threeD.ExtrusionDepth != 6.0 || threeD.ContourWidth != 2.0 || threeD.BevelTopHeight != 1.0 {
		t.Fatalf("unexpected 3D dimensions = %#v", threeD)
	}
	if threeD.BevelTopPreset != "relaxedInset" || threeD.BevelBottomPreset != "angle" {
		t.Fatalf("unexpected 3D bevel presets = %#v", threeD)
	}

	shadow, ok := effects.Effects[1].(drawing.Shadow)
	if !ok {
		t.Fatalf("shadow type = %T, want drawing.Shadow", effects.Effects[1])
	}
	if shadow.Color != "673AB7" || shadow.Alpha != 0.5 || shadow.Blur != 3.0 || shadow.Distance != 2.0 || shadow.Angle != 45.0 {
		t.Fatalf("unexpected shadow = %#v", shadow)
	}

	glow, ok := effects.Effects[2].(drawing.Glow)
	if !ok {
		t.Fatalf("glow type = %T, want drawing.Glow", effects.Effects[2])
	}
	if glow.Color != "FF0000" || glow.Radius != 4.0 {
		t.Fatalf("unexpected glow = %#v", glow)
	}

	softEdge, ok := effects.Effects[3].(drawing.SoftEdge)
	if !ok {
		t.Fatalf("soft edge type = %T, want drawing.SoftEdge", effects.Effects[3])
	}
	if softEdge.Radius != 1.0 {
		t.Fatalf("unexpected soft edge = %#v", softEdge)
	}

	reflection, ok := effects.Effects[4].(drawing.Reflection)
	if !ok {
		t.Fatalf("reflection type = %T, want drawing.Reflection", effects.Effects[4])
	}
	if reflection.Alpha != 0.25 || reflection.Blur != 2.0 || reflection.Distance != 3.0 || reflection.Direction != 90.0 {
		t.Fatalf("unexpected reflection = %#v", reflection)
	}
}

func TestParagraphParsesEndParaRPr(t *testing.T) {
	const src = `
<a:p xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <a:endParaRPr sz="2400" b="1">
    <a:latin typeface="Arial"/>
  </a:endParaRPr>
</a:p>`

	var para parts.DMLParagraph
	if err := xml.Unmarshal([]byte(src), &para); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if para.EndParaRPr == nil {
		t.Fatal("expected EndParaRPr to be parsed")
	}
	if para.EndParaRPr.FontSize != 2400 {
		t.Fatalf("font size = %d, want 2400", para.EndParaRPr.FontSize)
	}
	if para.EndParaRPr.Bold == nil || !*para.EndParaRPr.Bold {
		t.Fatal("expected bold to be true")
	}
	if para.EndParaRPr.Latin == nil || para.EndParaRPr.Latin.Typeface != "Arial" {
		t.Fatalf("latin typeface = %v, want Arial", para.EndParaRPr.Latin)
	}
}

func TestParaPropsParsesMarginRightAndTabStops(t *testing.T) {
	const src = `
<a:pPr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
       marR="457200" defTabSz="914400" fontAlgn="ctr">
  <a:buSzPct val="125000"/>
  <a:tabLst>
    <a:tab pos="457200" algn="l"/>
    <a:tab pos="914400" algn="ctr"/>
  </a:tabLst>
</a:pPr>`

	var props parts.DMLParaProps
	if err := xml.Unmarshal([]byte(src), &props); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if props.MarR == nil || *props.MarR != 457200 {
		t.Fatalf("marR = %v, want 457200", props.MarR)
	}
	if props.DefTabSz == nil || *props.DefTabSz != 914400 {
		t.Fatalf("defTabSz = %v, want 914400", props.DefTabSz)
	}
	if props.FontAlgn != "ctr" {
		t.Fatalf("fontAlgn = %q, want ctr", props.FontAlgn)
	}
	if props.BuSzPct == nil || props.BuSzPct.Val != 125000 {
		t.Fatalf("buSzPct = %v, want 125000", props.BuSzPct)
	}
	if len(props.TabStops) != 2 {
		t.Fatalf("tabStops count = %d, want 2", len(props.TabStops))
	}
	if props.TabStops[0].Pos != 457200 || props.TabStops[0].Algn != "l" {
		t.Fatalf("tabStop[0] = %+v, want {Pos:457200 Algn:l}", props.TabStops[0])
	}
	if props.TabStops[1].Pos != 914400 || props.TabStops[1].Algn != "ctr" {
		t.Fatalf("tabStop[1] = %+v, want {Pos:914400 Algn:ctr}", props.TabStops[1])
	}
}

func TestShapePropertiesInnerShadowAndBlur(t *testing.T) {
	effects := (&parts.ShapeProperties{
		InnerShadow: &parts.InnerShadowEffect{
			BlurRad: 38100,
			Dist:    25400,
			Dir:     5400000,
			SrgbClr: &parts.EffectColorRef{Val: "112233"},
		},
		Blur: &parts.BlurEffect{Rad: 76200},
	}).DrawingEffectList(nil)

	if len(effects.Effects) != 2 {
		t.Fatalf("effect count = %d, want 2", len(effects.Effects))
	}

	inner, ok := effects.Effects[0].(drawing.InnerShadow)
	if !ok {
		t.Fatalf("type = %T, want drawing.InnerShadow", effects.Effects[0])
	}
	if inner.Color != "112233" || inner.Blur != 3.0 || inner.Distance != 2.0 || inner.Angle != 90.0 {
		t.Fatalf("unexpected inner shadow = %#v", inner)
	}

	blur, ok := effects.Effects[1].(drawing.Blur)
	if !ok {
		t.Fatalf("type = %T, want drawing.Blur", effects.Effects[1])
	}
	if blur.Radius != 6.0 {
		t.Fatalf("unexpected blur radius = %v", blur.Radius)
	}
}

func TestBodyPrParsesRotWrapAnchorCtr(t *testing.T) {
	const src = `
<a:bodyPr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
  rot="5400000" wrap="none" anchor="ctr" anchorCtr="1"/>`

	var bp parts.DMLBodyPr
	if err := xml.Unmarshal([]byte(src), &bp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if bp.Rot != 5400000 {
		t.Fatalf("rot = %d, want 5400000", bp.Rot)
	}
	if bp.Wrap != "none" {
		t.Fatalf("wrap = %q, want none", bp.Wrap)
	}
	if bp.Anchor != "ctr" {
		t.Fatalf("anchor = %q, want ctr", bp.Anchor)
	}
	if bp.AnchorCtr == nil || !*bp.AnchorCtr {
		t.Fatal("expected anchorCtr=true")
	}
}

func TestBodyPrConvertsRotWrapAnchorCtr(t *testing.T) {
	trueVal := true
	bp := &parts.DMLBodyPr{
		Rot:       900000,
		Wrap:      "none",
		Anchor:    "t",
		AnchorCtr: &trueVal,
	}
	props := bp.DrawingTextBodyProperties()
	if props.Rot != 900000 {
		t.Fatalf("rot = %d, want 900000", props.Rot)
	}
	if props.Wrap != "none" {
		t.Fatalf("wrap = %q, want none", props.Wrap)
	}
	if !props.AnchorCtr {
		t.Fatal("expected AnchorCtr=true")
	}
}

func TestRunPrParsesHighlight(t *testing.T) {
	const src = `
<a:rPr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <a:highlight><a:srgbClr val="FFFF00"/></a:highlight>
</a:rPr>`

	var rpr parts.DMLRunPr
	if err := xml.Unmarshal([]byte(src), &rpr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if rpr.Highlight == nil || rpr.Highlight.Val != "FFFF00" {
		t.Fatalf("highlight = %v, want FFFF00", rpr.Highlight)
	}
}

func TestRunPrParsesHighlightScheme(t *testing.T) {
	const src = `
<a:rPr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <a:highlight><a:schemeClr val="accent1"/></a:highlight>
</a:rPr>`

	var rpr parts.DMLRunPr
	if err := xml.Unmarshal([]byte(src), &rpr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if rpr.HighlightScheme == nil || rpr.HighlightScheme.Val != "accent1" {
		t.Fatalf("highlightScheme = %v, want accent1", rpr.HighlightScheme)
	}
}

func TestShapeParsesStyle(t *testing.T) {
	const src = `
<p:sp xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
      xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:nvSpPr><p:cNvPr id="1" name="Rect"/></p:nvSpPr>
  <p:spPr/>
  <p:style>
    <a:lnRef idx="1"><a:schemeClr val="accent1"/></a:lnRef>
    <a:fillRef idx="2"><a:srgbClr val="FF0000"/></a:fillRef>
    <a:effectRef idx="0"/>
    <a:fontRef idx="minor"><a:schemeClr val="lt1"/></a:fontRef>
  </p:style>
</p:sp>`

	var sp parts.Shape
	if err := xml.Unmarshal([]byte(src), &sp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if sp.Style == nil {
		t.Fatal("expected Style to be parsed")
	}
	if sp.Style.LnRef == nil || sp.Style.LnRef.Idx != 1 {
		t.Fatalf("lnRef idx = %v, want 1", sp.Style.LnRef)
	}
	if sp.Style.FillRef == nil || sp.Style.FillRef.Idx != 2 {
		t.Fatalf("fillRef idx = %v, want 2", sp.Style.FillRef)
	}
	if sp.Style.EffectRef == nil || sp.Style.EffectRef.Idx != 0 {
		t.Fatalf("effectRef idx = %v, want 0", sp.Style.EffectRef)
	}
	if sp.Style.FontRef == nil || sp.Style.FontRef.Idx != "minor" {
		t.Fatalf("fontRef idx = %v, want minor", sp.Style.FontRef)
	}
}

func TestParseTimingEmphasisEffects(t *testing.T) {
	const src = `
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
       xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:cSld>
    <p:spTree>
      <p:sp><p:nvSpPr><p:cNvPr id="1" name="shape"/></p:nvSpPr><p:spPr/></p:sp>
    </p:spTree>
  </p:cSld>
  <p:timing>
    <p:tnLst>
      <p:par>
        <p:cTn nodeType="mainSeq">
          <p:childTnLst>
            <p:par>
              <p:cTn nodeType="clickEffect">
                <p:childTnLst>
                  <p:set><p:cBhvr><p:tgtEl><p:spTgt spid="2"/></p:tgtEl></p:cBhvr></p:set>
                </p:childTnLst>
              </p:cTn>
            </p:par>
            <p:par>
              <p:cTn nodeType="emphEffect">
                <p:childTnLst>
                  <p:animRot><p:cBhvr><p:tgtEl><p:spTgt spid="3"/></p:tgtEl></p:cBhvr></p:animRot>
                </p:childTnLst>
              </p:cTn>
            </p:par>
            <p:par>
              <p:cTn nodeType="emphEffect">
                <p:childTnLst>
                  <p:animScale><p:cBhvr><p:tgtEl><p:spTgt spid="3"/></p:tgtEl></p:cBhvr></p:animScale>
                </p:childTnLst>
              </p:cTn>
            </p:par>
          </p:childTnLst>
        </p:cTn>
      </p:par>
    </p:tnLst>
  </p:timing>
</p:sld>`

	var slide parts.Slide
	if err := xml.Unmarshal([]byte(src), &slide); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if slide.AnimSteps == nil || slide.AnimSteps[2] != 1 {
		t.Fatalf("expected entrance step 1 for shape 2, got %v", slide.AnimSteps)
	}
	fx := slide.EmphasisEffects[3]
	if len(fx) != 2 || fx[0] != "spin" || fx[1] != "growShrink" {
		t.Fatalf("expected emphasis [spin growShrink] for shape 3, got %v", fx)
	}
}

func TestTimingRoundTrip(t *testing.T) {
	s := parts.Slide{
		AnimSteps:         map[int]int{1: 1, 2: 2},
		EmphasisEffects:   map[int][]string{3: {"spin", "growShrink"}},
		MotionPaths:       map[int]parts.AnimMotionPath{4: {Path: "M 0 0 L 100000 0", Dur: 600, Origin: "layout"}},
		AnimationTriggers: map[int][]string{5: {"onClick"}},
	}

	b, err := xml.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var loaded parts.Slide
	if err := xml.Unmarshal(b, &loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if loaded.AnimSteps[1] != 1 || loaded.AnimSteps[2] != 2 {
		t.Fatalf("AnimSteps lost: %v", loaded.AnimSteps)
	}
	fx := loaded.EmphasisEffects[3]
	if len(fx) != 2 || fx[0] != "spin" || fx[1] != "growShrink" {
		t.Fatalf("EmphasisEffects lost: %v", loaded.EmphasisEffects)
	}
	mp := loaded.MotionPaths[4]
	if mp.Path != "M 0 0 L 100000 0" || mp.Dur != 600 || mp.Origin != "layout" {
		t.Fatalf("MotionPaths lost: %v", loaded.MotionPaths)
	}
	tr := loaded.AnimationTriggers[5]
	if len(tr) != 1 || tr[0] != "onClick" {
		t.Fatalf("AnimationTriggers lost: %v", loaded.AnimationTriggers)
	}
}

func TestCustomGeometryPolyLineTo(t *testing.T) {
	const src = `
<spPr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <a:custGeom>
    <a:pathLst>
      <a:path w="100" h="100">
        <a:moveTo><a:pt x="0" y="0"/></a:moveTo>
        <a:polyLineTo>
          <a:pt x="10" y="10"/>
          <a:pt x="20" y="5"/>
          <a:pt x="30" y="15"/>
        </a:polyLineTo>
        <a:close/>
      </a:path>
    </a:pathLst>
  </a:custGeom>
</spPr>`

	var sp parts.ShapeProperties
	if err := xml.Unmarshal([]byte(src), &sp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	geom := sp.DrawingGeometry()
	if geom == nil {
		t.Fatal("DrawingGeometry returned nil")
	}
	if len(geom.Paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(geom.Paths))
	}
	cmds := geom.Paths[0].Commands
	// moveTo + 3 polyLineTo points + close = 5 commands
	if len(cmds) != 5 {
		t.Fatalf("expected 5 commands, got %d: %#v", len(cmds), cmds)
	}
	if cmds[0].Type != "M" {
		t.Errorf("cmd[0].Type = %q, want M", cmds[0].Type)
	}
	for i := 1; i <= 3; i++ {
		if cmds[i].Type != "L" {
			t.Errorf("cmd[%d].Type = %q, want L", i, cmds[i].Type)
		}
	}
	if cmds[4].Type != "Z" {
		t.Errorf("cmd[4].Type = %q, want Z", cmds[4].Type)
	}
	// Check normalized points (divided by path w/h = 100)
	if cmds[1].Points[0] != (drawing.Point{X: 0.1, Y: 0.1}) {
		t.Errorf("cmd[1].Points[0] = %#v, want {0.1, 0.1}", cmds[1].Points[0])
	}
	if cmds[3].Points[0] != (drawing.Point{X: 0.3, Y: 0.15}) {
		t.Errorf("cmd[3].Points[0] = %#v, want {0.3, 0.15}", cmds[3].Points[0])
	}
}

func TestShapePropertiesParsesAndFlattensEffectDag(t *testing.T) {
	const src = `<a:spPr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <a:effectDag name="root" type="sib">
    <a:glow rad="25400"><a:schemeClr val="accent1"><a:alpha val="50000"/></a:schemeClr></a:glow>
    <a:cont name="nested" type="tree">
      <a:outerShdw blurRad="38100" dist="25400" dir="2700000"><a:srgbClr val="112233"/></a:outerShdw>
      <a:blur rad="12700"/>
    </a:cont>
  </a:effectDag>
</a:spPr>`

	var sp parts.ShapeProperties
	if err := xml.Unmarshal([]byte(src), &sp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if sp.EffectDag == nil {
		t.Fatal("expected effectDag")
	}
	if sp.EffectDag.Name != "root" || sp.EffectDag.Type != "sib" || len(sp.EffectDag.Items) != 2 {
		t.Fatalf("unexpected root effectDag = %#v", sp.EffectDag)
	}
	if sp.EffectDag.Items[1].Container == nil || sp.EffectDag.Items[1].Container.Name != "nested" || sp.EffectDag.Items[1].Container.Type != "tree" {
		t.Fatalf("unexpected nested container = %#v", sp.EffectDag.Items[1].Container)
	}

	effects := sp.DrawingEffectList(map[string]string{"accent1": "4472C4"})
	if len(effects.Effects) != 3 {
		t.Fatalf("effect count = %d, want 3", len(effects.Effects))
	}
	glow, ok := effects.Effects[0].(drawing.Glow)
	if !ok || glow.Color != "4472C4" || glow.Alpha != 0.5 || glow.Radius != 2.0 {
		t.Fatalf("unexpected glow = %#v", effects.Effects[0])
	}
	shadow, ok := effects.Effects[1].(drawing.Shadow)
	if !ok || shadow.Color != "112233" || shadow.Blur != 3.0 || shadow.Distance != 2.0 || shadow.Angle != 45.0 {
		t.Fatalf("unexpected shadow = %#v", effects.Effects[1])
	}
	blur, ok := effects.Effects[2].(drawing.Blur)
	if !ok || blur.Radius != 1.0 {
		t.Fatalf("unexpected blur = %#v", effects.Effects[2])
	}
}

func TestEffectDagMarshalRoundTripPreservesContainers(t *testing.T) {
	dag := parts.EffectDag{
		Name: "root",
		Type: "sib",
		Items: []parts.EffectDagItem{
			{Glow: &parts.GlowEffect{Rad: 25400, SrgbClr: &parts.EffectColorRef{Val: "FF0000"}}},
			{Container: &parts.EffectDag{Name: "nested", Type: "tree", Items: []parts.EffectDagItem{
				{SoftEdge: &parts.SoftEdgeEffect{Rad: 12700}},
			}}},
		},
	}
	data, err := xml.Marshal(dag)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var loaded parts.EffectDag
	if err := xml.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal round trip: %v", err)
	}
	if loaded.Name != "root" || loaded.Type != "sib" || len(loaded.Items) != 2 {
		t.Fatalf("unexpected round-trip dag = %#v", loaded)
	}
	if loaded.Items[0].Glow == nil || loaded.Items[0].Glow.Rad != 25400 {
		t.Fatalf("unexpected round-trip glow = %#v", loaded.Items[0].Glow)
	}
	if loaded.Items[1].Container == nil || loaded.Items[1].Container.Name != "nested" || len(loaded.Items[1].Container.Items) != 1 {
		t.Fatalf("unexpected round-trip nested container = %#v", loaded.Items[1].Container)
	}
}

func TestShapePropertiesParsesPresetShadowFromEffectListAndDag(t *testing.T) {
	const src = `<a:spPr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <a:effectLst>
    <a:prstShdw prst="shdw14" dist="25400" dir="2700000">
      <a:schemeClr val="accent1"><a:alpha val="50000"/></a:schemeClr>
    </a:prstShdw>
  </a:effectLst>
  <a:effectDag name="dag">
    <a:prstShdw prst="shdw3" dist="38100" dir="5400000">
      <a:srgbClr val="112233"/>
    </a:prstShdw>
  </a:effectDag>
</a:spPr>`

	var sp parts.ShapeProperties
	if err := xml.Unmarshal([]byte(src), &sp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if sp.PresetShadow == nil || sp.PresetShadow.Prst != "shdw14" {
		t.Fatalf("unexpected effect-list preset shadow = %#v", sp.PresetShadow)
	}
	if sp.EffectDag == nil || len(sp.EffectDag.Items) != 1 || sp.EffectDag.Items[0].PresetShadow == nil {
		t.Fatalf("unexpected effectDag preset shadow = %#v", sp.EffectDag)
	}
	if sp.EffectDag.Items[0].PresetShadow.Prst != "shdw3" {
		t.Fatalf("dag preset = %q, want shdw3", sp.EffectDag.Items[0].PresetShadow.Prst)
	}

	effects := sp.DrawingEffectList(map[string]string{"accent1": "4472C4"})
	if len(effects.Effects) != 2 {
		t.Fatalf("effect count = %d, want 2", len(effects.Effects))
	}
	first, ok := effects.Effects[0].(drawing.Shadow)
	if !ok || first.Color != "4472C4" || first.Alpha != 0.5 || first.Distance != 2.0 || first.Angle != 45.0 {
		t.Fatalf("unexpected first preset shadow = %#v", effects.Effects[0])
	}
	second, ok := effects.Effects[1].(drawing.Shadow)
	if !ok || second.Color != "112233" || second.Distance != 3.0 || second.Angle != 90.0 {
		t.Fatalf("unexpected dag preset shadow = %#v", effects.Effects[1])
	}
}

func TestPresetShadowMarshalRoundTripPreservesPreset(t *testing.T) {
	dag := parts.EffectDag{Items: []parts.EffectDagItem{{PresetShadow: &parts.PresetShadowEffect{
		Prst: "shdw20", Dist: 25400, Dir: 2700000, SrgbClr: &parts.EffectColorRef{Val: "010203"},
	}}}}
	data, err := xml.Marshal(dag)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var loaded parts.EffectDag
	if err := xml.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(loaded.Items) != 1 || loaded.Items[0].PresetShadow == nil || loaded.Items[0].PresetShadow.Prst != "shdw20" {
		t.Fatalf("unexpected round-trip preset shadow = %#v", loaded.Items)
	}
}

func TestShapePropertiesDrawingEffectsPreserveRichMetadata(t *testing.T) {
	grow := true
	rot := true
	effects := (&parts.ShapeProperties{
		OuterShadow: &parts.OuterShadowEffect{
			BlurRad: 38100, Dist: 25400, Dir: 2700000,
			Sx: 125000, Sy: 75000, Kx: 600000, Ky: -300000,
			Algn: "br", RotWithShape: &rot,
			SrgbClr: &parts.EffectColorRef{Val: "112233"},
		},
		FillOverlay: &parts.FillOverlayEffect{
			Blend: "mult", SolidFill: &parts.ColorRef{Val: "FF0000"},
		},
		Reflection: &parts.ReflectionEffect{
			BlurRad: 12700, StA: 60000, StPos: 10000, EndA: 5000, EndPos: 90000,
			Dist: 25400, Dir: 5400000, FadeDir: 5400000,
			Sx: 90000, Sy: 60000, Kx: 300000, Ky: -300000, Algn: "b", RotWithShape: &rot,
		},
		Blur: &parts.BlurEffect{Rad: 76200, Grow: &grow},
	}).DrawingEffectList(nil)

	if len(effects.Effects) != 4 {
		t.Fatalf("effect count = %d, want 4", len(effects.Effects))
	}
	shadow, ok := effects.Effects[0].(drawing.Shadow)
	if !ok || shadow.ScaleX != 1.25 || shadow.ScaleY != 0.75 || shadow.SkewX != 10 || shadow.SkewY != -5 || shadow.Alignment != "br" || !shadow.RotateWithShape {
		t.Fatalf("unexpected rich shadow = %#v", effects.Effects[0])
	}
	overlay, ok := effects.Effects[1].(drawing.FillOverlay)
	if !ok || overlay.BlendMode != "mult" {
		t.Fatalf("unexpected fill overlay = %#v", effects.Effects[1])
	}
	if solid, ok := overlay.Fill.(drawing.SolidFill); !ok || solid.Color != "FF0000" {
		t.Fatalf("unexpected overlay fill = %#v", overlay.Fill)
	}
	reflection, ok := effects.Effects[2].(drawing.Reflection)
	if !ok || reflection.StartAlpha != 60000 || reflection.EndAlpha != 5000 || reflection.StartPosition != 10000 || reflection.EndPosition != 90000 || reflection.ScaleX != 0.9 || reflection.ScaleY != 0.6 || reflection.SkewX != 5 || reflection.SkewY != -5 || reflection.Alignment != "b" || !reflection.RotateWithShape {
		t.Fatalf("unexpected rich reflection = %#v", effects.Effects[2])
	}
	blur, ok := effects.Effects[3].(drawing.Blur)
	if !ok || !blur.Grow || blur.Radius != 6 {
		t.Fatalf("unexpected rich blur = %#v", effects.Effects[3])
	}
}

func TestEffectDagCarriesContainerTypeAndFillOverlay(t *testing.T) {
	const src = `<a:effectDag xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" type="tree"><a:cont type="sib"><a:fillOverlay blend="screen"><a:solidFill><a:srgbClr val="00FF00"/></a:solidFill></a:fillOverlay><a:blur rad="25400" grow="1"/></a:cont></a:effectDag>`
	var dag parts.EffectDag
	if err := xml.Unmarshal([]byte(src), &dag); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	effects := (&parts.ShapeProperties{EffectDag: &dag}).DrawingEffectList(nil)
	if len(effects.Effects) != 2 {
		t.Fatalf("effect count = %d, want 2", len(effects.Effects))
	}
	overlay, ok := effects.Effects[0].(drawing.FillOverlay)
	if !ok || overlay.ContainerType != "sib" || overlay.BlendMode != "screen" {
		t.Fatalf("unexpected dag overlay = %#v", effects.Effects[0])
	}
	blur, ok := effects.Effects[1].(drawing.Blur)
	if !ok || blur.ContainerType != "sib" || !blur.Grow {
		t.Fatalf("unexpected dag blur = %#v", effects.Effects[1])
	}
}

func TestBlipParsesAndResolvesPictureEffects(t *testing.T) {
	const src = `
<a:blip xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
        xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
        r:embed="rId7">
  <a:grayscl/>
  <a:biLevel thresh="42000"/>
  <a:duotone>
    <a:srgbClr val="112233"/>
    <a:schemeClr val="accent1"/>
  </a:duotone>
  <a:lum bright="10000" contrast="-20000"/>
  <a:hsl hue="5400000" sat="75000" lum="120000"/>
  <a:alphaMod amt="80000"/>
  <a:alphaModFix amt="70000"/>
  <a:alphaOff amt="-10000"/>
  <a:alphaRepl a="60000"/>
  <a:clrRepl><a:srgbClr val="ABCDEF"/></a:clrRepl>
</a:blip>`

	var blip parts.Blip
	if err := xml.Unmarshal([]byte(src), &blip); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if blip.Embed != "rId7" {
		t.Fatalf("embed = %q, want rId7", blip.Embed)
	}
	effects := blip.DrawingEffects(map[string]string{"accent1": "445566"})
	if len(effects) != 10 {
		t.Fatalf("effect count = %d, want 10 (%#v)", len(effects), effects)
	}
	if _, ok := effects[0].(drawing.Grayscale); !ok {
		t.Fatalf("effect[0] = %T, want Grayscale", effects[0])
	}
	if bi, ok := effects[1].(drawing.BiLevel); !ok || bi.Threshold != 42000 {
		t.Fatalf("effect[1] = %#v, want BiLevel threshold 42000", effects[1])
	}
	if duo, ok := effects[2].(drawing.Duotone); !ok || duo.Color1 != "112233" || duo.Color2 != "445566" {
		t.Fatalf("effect[2] = %#v, want resolved Duotone", effects[2])
	}
	if lum, ok := effects[3].(drawing.Lum); !ok || lum.Brightness != 10000 || lum.Contrast != -20000 {
		t.Fatalf("effect[3] = %#v, want Lum", effects[3])
	}
	if hsl, ok := effects[4].(drawing.HSLEffect); !ok || hsl.Hue != 5400000 || hsl.Sat != 75000 || hsl.Lum != 120000 {
		t.Fatalf("effect[4] = %#v, want HSLEffect", effects[4])
	}
	if repl, ok := effects[9].(drawing.ClrRepl); !ok || repl.Color != "ABCDEF" {
		t.Fatalf("effect[9] = %#v, want ClrRepl ABCDEF", effects[9])
	}
}

func TestDrawingFillCarriesBlipEffects(t *testing.T) {
	sp := &parts.ShapeProperties{BlipFill: &parts.BlipFill{
		Blip: &parts.Blip{Embed: "rId1", Grayscl: &struct{}{}, Lum: &parts.BlipLum{Bright: 15000}},
	}}
	fill, ok := sp.DrawingFill(nil).(drawing.PictureFill)
	if !ok {
		t.Fatalf("fill = %T, want drawing.PictureFill", sp.DrawingFill(nil))
	}
	if fill.EmbedID != "rId1" {
		t.Fatalf("embed = %q, want rId1", fill.EmbedID)
	}
	if len(fill.BlipEffects) != 2 {
		t.Fatalf("blip effects = %d, want 2", len(fill.BlipEffects))
	}
}

func TestBlipFillPreservesDPIAndExplicitRotationFlag(t *testing.T) {
	var fill parts.BlipFill
	if err := xml.Unmarshal([]byte(`<blipFill dpi="220" rotWithShape="0"><blip embed="rId1"/><stretch><fillRect/></stretch></blipFill>`), &fill); err != nil {
		t.Fatal(err)
	}
	if fill.Dpi != 220 || fill.RotWithShape == nil || *fill.RotWithShape {
		t.Fatalf("blip fill metadata = dpi %d rot %#v", fill.Dpi, fill.RotWithShape)
	}
}

func TestBlipAlphaModFixWithoutAmountPreservesPictureOpacity(t *testing.T) {
	const src = `<a:blip xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:alphaModFix/></a:blip>`
	var blip parts.Blip
	if err := xml.Unmarshal([]byte(src), &blip); err != nil {
		t.Fatal(err)
	}
	effects := blip.DrawingEffects(nil)
	if len(effects) != 1 {
		t.Fatalf("effects = %d, want 1: %#v", len(effects), effects)
	}
	alpha, ok := effects[0].(drawing.AlphaModFix)
	if !ok || alpha.Amount != 100000 {
		t.Fatalf("alphaModFix = %#v, want 100000 (opaque no-op)", effects[0])
	}
}

func TestBlipAlphaModFixPreservesExplicitZero(t *testing.T) {
	const src = `<a:blip xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:alphaModFix amt="0"/></a:blip>`
	var blip parts.Blip
	if err := xml.Unmarshal([]byte(src), &blip); err != nil {
		t.Fatal(err)
	}
	effects := blip.DrawingEffects(nil)
	alpha, ok := effects[0].(drawing.AlphaModFix)
	if !ok || alpha.Amount != 0 {
		t.Fatalf("alphaModFix = %#v, want authored zero", effects[0])
	}
}

func TestBlipParsesExtendedAlphaAndColorChangeEffects(t *testing.T) {
	const src = `<blip xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" embed="rId1"><a:alphaBiLevel thresh="40000"/><a:alphaOutset rad="12700"/><a:alphaInv/><a:alphaCeiling/><a:alphaFloor/><a:clrChange useA="1"><a:clrFrom><a:srgbClr val="FF0000"><a:alpha val="50000"/></a:srgbClr></a:clrFrom><a:clrTo><a:srgbClr val="00FF00"><a:alpha val="75000"/></a:srgbClr></a:clrTo></a:clrChange></blip>`
	var b parts.Blip
	if err := xml.Unmarshal([]byte(src), &b); err != nil {
		t.Fatal(err)
	}
	effects := b.DrawingEffects(nil)
	if len(effects) != 6 {
		t.Fatalf("effects=%d want 6: %#v", len(effects), effects)
	}
	if _, ok := effects[0].(drawing.AlphaBiLevel); !ok {
		t.Fatalf("effect0=%T", effects[0])
	}
	if _, ok := effects[1].(drawing.AlphaOutset); !ok {
		t.Fatalf("effect1=%T", effects[1])
	}
	if _, ok := effects[2].(drawing.AlphaInv); !ok {
		t.Fatalf("effect2=%T", effects[2])
	}
	if _, ok := effects[3].(drawing.AlphaCeiling); !ok {
		t.Fatalf("effect3=%T", effects[3])
	}
	if _, ok := effects[4].(drawing.AlphaFloor); !ok {
		t.Fatalf("effect4=%T", effects[4])
	}
	cc, ok := effects[5].(drawing.ClrChange)
	if !ok || cc.From != "FF0000" || cc.To != "00FF00" || !cc.UseAlpha || cc.FromAlpha != .5 || cc.ToAlpha != .75 {
		t.Fatalf("clrChange=%#v", effects[5])
	}
}

func TestShapePropertiesThreeDRoundTripPreservesCameraLightAndBevelMetadata(t *testing.T) {
	const src = `<spPr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:scene3d>
			<a:camera prst="perspectiveRelaxed" fov="3000000" zoom="120000">
				<a:rot lat="600000" lon="-1200000" rev="300000"/>
			</a:camera>
			<a:lightRig rig="balanced" dir="br">
				<a:rot lat="120000" lon="240000" rev="360000"/>
			</a:lightRig>
		</a:scene3d>
		<a:sp3d extrusionH="914400" contourW="12700" prstMaterial="plastic">
			<a:bevelT w="25400" h="38100" prst="softRound"/>
			<a:bevelB w="12700" h="19050" prst="angle"/>
			<a:extrusionClr><a:srgbClr val="334455"/></a:extrusionClr>
			<a:contourClr><a:srgbClr val="AABBCC"/></a:contourClr>
		</a:sp3d>
	</spPr>`
	var props parts.ShapeProperties
	if err := xml.Unmarshal([]byte(src), &props); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if props.Scene3D == nil || props.Scene3D.Camera == nil || props.Scene3D.LightRig == nil || props.Shape3D == nil {
		t.Fatalf("missing 3D structures: %#v", props)
	}
	if props.Scene3D.Camera.FOV != 3000000 || props.Scene3D.Camera.Zoom != 120000 {
		t.Fatalf("camera metadata lost: %#v", props.Scene3D.Camera)
	}
	if props.Scene3D.LightRig.Rot == nil || props.Scene3D.LightRig.Rot.Rev != 360000 {
		t.Fatalf("light rotation lost: %#v", props.Scene3D.LightRig)
	}
	if props.Shape3D.BevelT == nil || props.Shape3D.BevelT.Prst != "softRound" || props.Shape3D.BevelB == nil || props.Shape3D.BevelB.Prst != "angle" {
		t.Fatalf("bevel metadata lost: %#v", props.Shape3D)
	}
	data, err := xml.Marshal(&props)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got parts.ShapeProperties
	if err := xml.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal round-trip: %v", err)
	}
	if got.Scene3D == nil || got.Scene3D.Camera == nil || got.Scene3D.Camera.FOV != 3000000 || got.Scene3D.Camera.Zoom != 120000 {
		t.Fatalf("camera did not round-trip: %#v", got.Scene3D)
	}
	if got.Scene3D.LightRig == nil || got.Scene3D.LightRig.Rot == nil || got.Scene3D.LightRig.Rot.Lat != 120000 {
		t.Fatalf("light rig did not round-trip: %#v", got.Scene3D)
	}
	if got.Shape3D == nil || got.Shape3D.BevelT == nil || got.Shape3D.BevelT.Prst != "softRound" || got.Shape3D.BevelB == nil || got.Shape3D.BevelB.Prst != "angle" {
		t.Fatalf("shape 3D did not round-trip: %#v", got.Shape3D)
	}
}

func TestPresentationParsesDefaultTextStyle(t *testing.T) {
	const src = `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:defaultTextStyle><a:lvl1pPr><a:defRPr sz="1800"/></a:lvl1pPr></p:defaultTextStyle></p:presentation>`
	var pres parts.Presentation
	if err := xml.Unmarshal([]byte(src), &pres); err != nil {
		t.Fatalf("unmarshal presentation: %v", err)
	}
	if pres.DefaultTextStyle == nil || pres.DefaultTextStyle.Lvl1PPr == nil || pres.DefaultTextStyle.Lvl1PPr.DefRPr == nil {
		t.Fatalf("default text style = %#v, want lvl1 defRPr", pres.DefaultTextStyle)
	}
	if got := pres.DefaultTextStyle.Lvl1PPr.DefRPr.FontSize; got != 1800 {
		t.Fatalf("default font size = %d, want 1800", got)
	}
}
