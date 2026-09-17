package layout

import (
	"math"
	"sort"
	"strings"

	"office/pkg/ooxml/drawing"
	"office/pkg/presentation/ir"
)

// PhysicalVisual is the format-neutral render contract produced by layout.
// Canonical hierarchy has already been flattened and all group transforms have
// been composed before a renderer receives this value.
type PhysicalClipPlan struct {
	Kind        string
	CSSPath     string
	ClipContent bool
}

// Physical paint bounds are resolved by layout so renderers do not reinterpret
// canonical geometry or effect extents.
type PhysicalPaintBoundsPlan struct {
	LeftPt   float64
	TopPt    float64
	WidthPt  float64
	HeightPt float64
}

func (p PhysicalPaintBoundsPlan) Empty() bool {
	return p.WidthPt <= 0 || p.HeightPt <= 0
}

type PhysicalSVGPaintPlan struct {
	OverflowVisible bool
	FilterXPercent  float64
	FilterYPercent  float64
	FilterWPercent  float64
	FilterHPercent  float64
}

type PhysicalAncestorPaintPlan struct {
	Bounds       PhysicalPaintBoundsPlan
	Intersects   bool
	ContainsBase bool
}

type PhysicalClipPoint struct {
	XPt float64
	YPt float64
}

type PhysicalAncestorClipPlan struct {
	Bounds      PhysicalPaintBoundsPlan
	Kind        string
	RotationDeg float64
	FlipH       bool
	FlipV       bool
	Polygon     []PhysicalClipPoint
	Intersects  bool
}

// Group ancestry is part of PhysicalPaintPlan: ancestor envelopes and clips
// describe how this leaf participates in paint composition, so they must not
// be duplicated on PhysicalVisual as a second contract.
type PhysicalPaintPlan struct {
	ZIndex          int
	OverflowVisible bool
	Overflow        PhysicalOverflowPlan
	Bounds          PhysicalPaintBoundsPlan
	Clip            PhysicalClipPlan
	SVG             PhysicalSVGPaintPlan
	Ancestors       []PhysicalAncestorPaintPlan
	AncestorClips   []PhysicalAncestorClipPlan
	ClippedBounds   PhysicalPaintBoundsPlan
	PaintClipped    bool
}

// PhysicalShapePlan is the layout-owned composition contract shared by ordinary
// drawing shapes. It keeps frame, transform, vector paint, clipping and paint
// order together so renderers do not have to assemble a generic shape from
// independent canonical properties. Specialized plans may add content-specific
// state, but this plan remains the authoritative base drawing composition.

// PhysicalCompositionPlan is the common outer-frame contract for specialized
// presentation visuals. It owns placement, transform, resolved effects and
// paint participation; specialized plans embed it and add only content-specific
// state.
type PhysicalCompositionPlan struct {
	FrameBounds PhysicalPaintBoundsPlan
	Paint       PhysicalPaintPlan
	Transform   PhysicalTransformPlan
	Effects     EffectVisualPlan
}

// materializePhysicalCompositionPlan is the single constructor for outer visual
// composition. Callers that already resolved transform/effects pass them through
// so a physical visual tree never recomputes composition state per plan family.
func materializePhysicalCompositionPlan(shape *ir.Shape, paint PhysicalPaintPlan, transform PhysicalTransformPlan, effects EffectVisualPlan) PhysicalCompositionPlan {
	if shape == nil {
		return PhysicalCompositionPlan{}
	}
	return PhysicalCompositionPlan{
		FrameBounds: physicalBoundsFromRect(shape.Bounds),
		Transform:   transform,
		Effects:     effects,
		Paint:       paint,
	}
}

func resolvePhysicalCompositionPlan(shape *ir.Shape) PhysicalCompositionPlan {
	if shape == nil {
		return PhysicalCompositionPlan{}
	}
	return materializePhysicalCompositionPlan(shape, PhysicalPaintPlan{}, ResolvePhysicalTransformPlan(shape), ResolveEffectVisualPlan(shape.Effects))
}

type PhysicalShapePlan struct {
	PhysicalCompositionPlan
	Visual ShapeVisualPlan
}

type PhysicalVisual struct {
	// Kind is the layout-projected render classification. Renderers dispatch on
	// this physical value rather than reopening canonical Shape.Kind.
	Kind ir.ShapeKind
	// SourceShape preserves the canonical semantic/content carrier used by renderers.
	// Shape is the layout-owned slide-space clone and must not be consumed by rendering.
	SourceShape      *ir.Shape
	Shape            *ir.Shape
	Drawing          *PhysicalShapePlan
	Text             *TextVisualPlan
	Image            *ImageVisualPlan
	Connector        *ConnectorVisualPlan
	FreeConnector    *FreeConnectorVisualPlan
	Table            *PhysicalTablePlan
	Chart            *PhysicalChartPlan
	SmartArt         *PhysicalSmartArtPlan
	Media            *PhysicalMediaPlan
	OLE              *PhysicalOLEPlan
	PlaceholderClass string
}

// MaterializePhysicalVisualTree closes the presentation layout boundary in one
// operation: hierarchy -> slide-space leaves -> physical drawing plans. The
// returned order is the authored paint order and the canonical IR is untouched.
func MaterializePhysicalVisualTree(shapes []*ir.Shape) []PhysicalVisual {
	leaves := materializeVisualTreeWithAncestry(shapes)
	out := make([]PhysicalVisual, 0, len(leaves))
	paintIndex := 0
	for _, leaf := range leaves {
		shape := leaf.Shape
		if shape == nil || suppressEmptyPlaceholderPaint(shape) || suppressDegenerateShapePaint(shape) {
			continue
		}
		visual := ResolveShapeVisualPlan(shape)
		transform := ResolvePhysicalTransformPlan(shape)
		materializePhysicalGradientOrientation(&visual, transform, shape)
		clip := resolvePhysicalClipPlan(visual)
		overflow := resolvePhysicalOverflowPlan(shape, visual.Effects)
		svgPaint := resolvePhysicalSVGPaintPlan(shape, overflow, visual.Effects)
		paintBounds := resolvePhysicalPaintBounds(shape, overflow, transform)
		paintBounds = resolvePhysicalVectorPaintBounds(shape, paintBounds, overflow)
		ancestorClips := resolvePhysicalAncestorClipPlans(paintBounds, leaf.AncestorClips)
		clippedBounds, paintClipped := resolveClippedPhysicalPaintBounds(paintBounds, ancestorClips)
		// Fully clipped leaves remain in the physical scene. PaintClipped plus an
		// empty ClippedBounds is the authoritative visibility result; retaining the
		// leaf preserves authored scene identity/order for animation, interaction
		// and inspection without asking renderers to recompute clip geometry.
		paintIndex++
		ancestorPaint := resolvePhysicalAncestorPaintPlans(shape, paintBounds, leaf.Ancestors)
		paint := PhysicalPaintPlan{
			ZIndex:          paintIndex,
			OverflowVisible: visual.Effects.NeedsOverflow || !overflow.Empty(),
			Overflow:        overflow,
			Bounds:          paintBounds,
			Clip:            clip,
			SVG:             svgPaint,
			Ancestors:       ancestorPaint,
			AncestorClips:   ancestorClips,
			ClippedBounds:   clippedBounds,
			PaintClipped:    paintClipped,
		}
		composition := materializePhysicalCompositionPlan(shape, paint, transform, visual.Effects)
		textPlan := resolveTextVisualPlanWithShapeVisualAndComposition(shape, visual, composition)
		drawingPlan := PhysicalShapePlan{
			PhysicalCompositionPlan: composition,
			Visual:                  visual,
		}
		item := PhysicalVisual{
			Kind:             shape.Kind,
			SourceShape:      leaf.Source,
			Shape:            shape,
			Drawing:          &drawingPlan,
			Text:             &textPlan,
			PlaceholderClass: ResolvePhysicalPlaceholderClass(shape),
		}
		switch shape.Kind {
		case ir.ShapeImage:
			plan := resolveImageVisualPlanWithComposition(shape, composition)
			item.Image = &plan
		case ir.ShapeMedia:
			item.Media = resolvePhysicalMediaPlanWithComposition(shape, composition)
		case ir.ShapeOleObject:
			item.OLE = resolvePhysicalOLEPlanWithComposition(shape, composition)
		case ir.ShapeConnector:
			if shape.ConnectorStart != nil && shape.ConnectorEnd != nil {
				plan := resolveFreeConnectorVisualPlanWithComposition(shape, composition)
				item.FreeConnector = &plan
			} else {
				plan := resolveConnectorVisualPlanWithComposition(shape, composition)
				item.Connector = &plan
			}
		case ir.ShapeSmartArtConnector:
			// Resolve the authored connector representation at the layout boundary.
			// Renderers must not inspect canonical connector type/bounds to choose a
			// materialization path.
			if shape.ConnectorType != "" && shape.Bounds.W > 0 && shape.Bounds.H >= 0 {
				plan := resolveConnectorVisualPlanWithComposition(shape, composition)
				item.Connector = &plan
			} else {
				plan := resolveFreeConnectorVisualPlanWithComposition(shape, composition)
				item.FreeConnector = &plan
			}
		case ir.ShapeTable:
			item.Table = resolvePhysicalTablePlanWithComposition(shape, composition)
		case ir.ShapeChart:
			item.Chart = resolvePhysicalChartPlanWithComposition(shape, composition)
		case ir.ShapeSmartArt:
			item.SmartArt = resolvePhysicalSmartArtPlanWithComposition(shape, composition)
		}
		out = append(out, item)
	}
	return out
}

// suppressEmptyPlaceholderPaint keeps placeholder templates in canonical IR while
// preventing an unpopulated slide placeholder from becoming visible artwork.
// PowerPoint uses empty title/subtitle/body/etc. placeholders as editing affordances;
// their inherited fill/line geometry is not slide-show paint until the placeholder
// carries actual content. Keeping this decision at the physical-layout boundary
// preserves editability and keeps every renderer on the same visibility contract.
func suppressDegenerateShapePaint(shape *ir.Shape) bool {
	if shape == nil {
		return true
	}
	// Connectors may legitimately have zero width or height. Area-bearing
	// shapes cannot paint a frame/effect when either authored extent is zero.
	if shape.Kind == ir.ShapeConnector || shape.Kind == ir.ShapeSmartArtConnector {
		return false
	}
	return shape.Bounds.W <= 0 || shape.Bounds.H <= 0
}

func suppressEmptyPlaceholderPaint(shape *ir.Shape) bool {
	if shape == nil || shape.Placeholder == ir.PlaceholderNone || shape.Kind != ir.ShapeText {
		return false
	}
	for _, paragraph := range shape.Paragraphs {
		if paragraph == nil {
			continue
		}
		for _, run := range paragraph.Runs {
			if run == nil {
				continue
			}
			if strings.TrimSpace(run.Text) != "" || run.MathML != "" || run.FieldKind != "" {
				return false
			}
		}
	}
	return true
}

func resolvePhysicalClipPlan(visual ShapeVisualPlan) PhysicalClipPlan {
	kind := visual.Geometry.ClipKind
	plan := PhysicalClipPlan{Kind: kind, ClipContent: visual.ClipContent}
	switch kind {
	case "ellipse":
		plan.CSSPath = "ellipse(50% 50% at 50% 50%)"
	case "roundRect", "roundrect":
		plan.CSSPath = "inset(0 round 8%)"
	}
	return plan
}

// PhysicalOverflowPlan describes how far a visual may paint outside its
// canonical bounds after layout has resolved effects. Values are points in
// slide space and are deliberately renderer-neutral.
type PhysicalOverflowPlan struct {
	LeftPt   float64
	TopPt    float64
	RightPt  float64
	BottomPt float64
}

func (p PhysicalOverflowPlan) Empty() bool {
	return p.LeftPt <= 0 && p.TopPt <= 0 && p.RightPt <= 0 && p.BottomPt <= 0
}

// resolvePhysicalOverflowPlan materializes effect paint extents once. Inner
// effects and soft edges do not expand the outer paint bounds. Reflection
// overflow is resolved from its authored distance, direction, blur, scale,
// skew, size and rotWithShape semantics so clipping/culling share one physical
// envelope with the renderer.
func resolvePhysicalOverflowPlan(shape *ir.Shape, effects EffectVisualPlan) PhysicalOverflowPlan {
	var out PhysicalOverflowPlan
	expand := func(left, top, right, bottom float64) {
		if left > out.LeftPt {
			out.LeftPt = left
		}
		if top > out.TopPt {
			out.TopPt = top
		}
		if right > out.RightPt {
			out.RightPt = right
		}
		if bottom > out.BottomPt {
			out.BottomPt = bottom
		}
	}
	for _, shadow := range effects.Shadows {
		if shadow.Inner {
			continue
		}
		spread := shadow.BlurPt * 2
		expand(spread-maxFloat64(0, shadow.OffsetXPt), spread-maxFloat64(0, shadow.OffsetYPt), spread+maxFloat64(0, shadow.OffsetXPt), spread+maxFloat64(0, shadow.OffsetYPt))
	}
	for _, glow := range effects.Glows {
		expand(glow.RadiusPt, glow.RadiusPt, glow.RadiusPt, glow.RadiusPt)
	}
	for _, blur := range effects.Blurs {
		if !blur.Grow {
			continue
		}
		expand(blur.RadiusPt, blur.RadiusPt, blur.RadiusPt, blur.RadiusPt)
	}
	if effects.Composition.Reflection != nil && shape != nil {
		r := *effects.Composition.Reflection
		w := math.Abs(float64(shape.Bounds.W) / 12700.0)
		h := math.Abs(float64(shape.Bounds.H) / 12700.0)
		sx, sy := math.Abs(r.ScaleX), math.Abs(r.ScaleY)
		if sx == 0 {
			sx = 1
		}
		if sy == 0 {
			sy = 1
		}
		// DrawingML size is a fractional reflection-size hint. Preserve the
		// authored scale when present, otherwise the reflection is full-size.
		if r.Size > 0 {
			sy *= r.Size
		}
		rw, rh := w*sx, h*sy
		// Skew can move the far edge outside the scaled rectangle. Account for
		// that here so ancestor clipping and culling see the same paint envelope.
		rw += math.Abs(math.Tan(r.SkewXDeg*math.Pi/180)) * rh
		rh += math.Abs(math.Tan(r.SkewYDeg*math.Pi/180)) * rw
		direction := r.DirectionDeg
		if r.RotateWithShape {
			direction += shape.RotationDeg
		}
		a := math.Mod(direction, 360)
		if a < 0 {
			a += 360
		}
		blur := math.Max(0, r.BlurPt) * 2
		distance := math.Max(0, r.DistancePt)
		switch {
		case a >= 45 && a < 135:
			expand(0, blur, distance+rw+blur, blur)
		case a >= 135 && a < 225:
			expand(blur, distance+rh+blur, blur, 0)
		case a >= 225 && a < 315:
			expand(distance+rw+blur, blur, 0, blur)
		default:
			expand(blur, 0, blur, distance+rh+blur)
		}
	} else if effects.HasReflection && shape != nil {
		// Compatibility for manually constructed EffectVisualPlans in callers.
		h := math.Abs(float64(shape.Bounds.H) / 12700.0)
		expand(0, 0, 0, h)
	}
	return out
}

func resolvePhysicalAncestorClipPlans(paint PhysicalPaintBoundsPlan, clips []materializedAncestorClip) []PhysicalAncestorClipPlan {
	if len(clips) == 0 {
		return nil
	}
	out := make([]PhysicalAncestorClipPlan, 0, len(clips))
	for _, clip := range clips {
		bounds := physicalBoundsFromRect(clip.Bounds)
		polygon := physicalAncestorClipPolygon(bounds, clip.Kind, clip.RotationDeg, clip.FlipH, clip.FlipV)
		physicalBounds := bounds
		if len(polygon) >= 3 {
			physicalBounds = physicalClipPolygonBounds(polygon)
		}
		out = append(out, PhysicalAncestorClipPlan{
			Bounds:      bounds,
			Kind:        clip.Kind,
			RotationDeg: clip.RotationDeg,
			FlipH:       clip.FlipH,
			FlipV:       clip.FlipV,
			Polygon:     polygon,
			Intersects:  physicalBoundsIntersect(physicalBounds, paint),
		})
	}
	return out
}

func physicalAncestorClipPolygon(bounds PhysicalPaintBoundsPlan, kind string, rotationDeg float64, flipH, flipV bool) []PhysicalClipPoint {
	if bounds.Empty() {
		return nil
	}
	// Rectangular axis-aligned clips can stay compact. Ellipses always need a
	// physical polygon for layout-owned visibility testing, even when they have
	// no rotation or flips; their rectangular envelope is not the clip itself.
	if strings.ToLower(kind) != "ellipse" && math.Abs(rotationDeg) < 0.0001 && !flipH && !flipV {
		return nil
	}
	cx := bounds.LeftPt + bounds.WidthPt/2
	cy := bounds.TopPt + bounds.HeightPt/2
	rx := bounds.WidthPt / 2
	ry := bounds.HeightPt / 2
	rotate := func(x, y float64) PhysicalClipPoint {
		if flipH {
			x = -x
		}
		if flipV {
			y = -y
		}
		a := rotationDeg * math.Pi / 180
		ca, sa := math.Cos(a), math.Sin(a)
		return PhysicalClipPoint{XPt: cx + x*ca - y*sa, YPt: cy + x*sa + y*ca}
	}
	switch strings.ToLower(kind) {
	case "ellipse":
		const segments = 32
		out := make([]PhysicalClipPoint, 0, segments)
		for i := 0; i < segments; i++ {
			a := 2 * math.Pi * float64(i) / segments
			out = append(out, rotate(rx*math.Cos(a), ry*math.Sin(a)))
		}
		return out
	default:
		return []PhysicalClipPoint{rotate(-rx, -ry), rotate(rx, -ry), rotate(rx, ry), rotate(-rx, ry)}
	}
}

func physicalClipPolygonBounds(points []PhysicalClipPoint) PhysicalPaintBoundsPlan {
	if len(points) == 0 {
		return PhysicalPaintBoundsPlan{}
	}
	minX, maxX := points[0].XPt, points[0].XPt
	minY, maxY := points[0].YPt, points[0].YPt
	for _, point := range points[1:] {
		minX = math.Min(minX, point.XPt)
		maxX = math.Max(maxX, point.XPt)
		minY = math.Min(minY, point.YPt)
		maxY = math.Max(maxY, point.YPt)
	}
	return PhysicalPaintBoundsPlan{LeftPt: minX, TopPt: minY, WidthPt: maxX - minX, HeightPt: maxY - minY}
}

func physicalTransformedBounds(bounds PhysicalPaintBoundsPlan, rotationDeg float64) PhysicalPaintBoundsPlan {
	if bounds.Empty() || math.Abs(rotationDeg) < 0.0001 {
		return bounds
	}
	cx := bounds.LeftPt + bounds.WidthPt/2
	cy := bounds.TopPt + bounds.HeightPt/2
	rx := bounds.WidthPt / 2
	ry := bounds.HeightPt / 2
	a := rotationDeg * math.Pi / 180
	ca, sa := math.Cos(a), math.Sin(a)
	points := make([]PhysicalClipPoint, 0, 4)
	for _, p := range [][2]float64{{-rx, -ry}, {rx, -ry}, {rx, ry}, {-rx, ry}} {
		points = append(points, PhysicalClipPoint{XPt: cx + p[0]*ca - p[1]*sa, YPt: cy + p[0]*sa + p[1]*ca})
	}
	return physicalClipPolygonBounds(points)
}

// resolveClippedPhysicalPaintBounds computes the conservative paint envelope
// after explicit ancestor clips. Ordinary group envelopes never enter this
// function: only authored ClipContent ancestors are supplied. Non-rectangular
// clips conservatively use their physical bounding box here; the clip kind is
// retained separately for SVG/CSS serializers.
func resolveClippedPhysicalPaintBounds(paint PhysicalPaintBoundsPlan, clips []PhysicalAncestorClipPlan) (PhysicalPaintBoundsPlan, bool) {
	if paint.Empty() || len(clips) == 0 {
		return paint, false
	}
	out := paint
	clipped := false
	for _, clip := range clips {
		if clip.Bounds.Empty() {
			continue
		}
		clipBounds := clip.Bounds
		if len(clip.Polygon) >= 3 {
			// Bounding-box intersection alone is insufficient for rotated and
			// elliptical clips: a leaf can overlap the polygon envelope while
			// remaining completely outside the authored clip. Resolve visibility
			// against the physical polygon before reducing the paint envelope.
			if !physicalPolygonIntersectsBounds(clip.Polygon, out) {
				return PhysicalPaintBoundsPlan{}, true
			}
			clipBounds = physicalClipPolygonBounds(clip.Polygon)
		}
		next, ok := intersectPhysicalPaintBounds(out, clipBounds)
		if !ok {
			return PhysicalPaintBoundsPlan{}, true
		}
		if !physicalBoundsEqual(next, out) {
			clipped = true
		}
		out = next
	}
	return out, clipped
}

// physicalPolygonIntersectsBounds performs the visibility half of clipping in
// physical slide space. It deliberately does not attempt polygon clipping: the
// returned ClippedBounds remains a conservative rectangular paint envelope,
// while fully clipped leaves can be rejected without asking a renderer to
// reconstruct group geometry.
func physicalPolygonIntersectsBounds(polygon []PhysicalClipPoint, bounds PhysicalPaintBoundsPlan) bool {
	if len(polygon) < 3 || bounds.Empty() {
		return false
	}
	left, top := bounds.LeftPt, bounds.TopPt
	right, bottom := left+bounds.WidthPt, top+bounds.HeightPt
	insideRect := func(p PhysicalClipPoint) bool {
		return p.XPt >= left && p.XPt <= right && p.YPt >= top && p.YPt <= bottom
	}
	for _, p := range polygon {
		if insideRect(p) {
			return true
		}
	}
	corners := []PhysicalClipPoint{{left, top}, {right, top}, {right, bottom}, {left, bottom}}
	for _, p := range corners {
		if physicalPointInPolygon(p, polygon) {
			return true
		}
	}
	rectEdges := [][2]PhysicalClipPoint{{corners[0], corners[1]}, {corners[1], corners[2]}, {corners[2], corners[3]}, {corners[3], corners[0]}}
	for i := range polygon {
		a := polygon[i]
		b := polygon[(i+1)%len(polygon)]
		for _, edge := range rectEdges {
			if physicalSegmentsIntersect(a, b, edge[0], edge[1]) {
				return true
			}
		}
	}
	return false
}

func physicalPointInPolygon(point PhysicalClipPoint, polygon []PhysicalClipPoint) bool {
	inside := false
	for i, j := 0, len(polygon)-1; i < len(polygon); j, i = i, i+1 {
		pi, pj := polygon[i], polygon[j]
		if ((pi.YPt > point.YPt) != (pj.YPt > point.YPt)) &&
			(point.XPt < (pj.XPt-pi.XPt)*(point.YPt-pi.YPt)/(pj.YPt-pi.YPt)+pi.XPt) {
			inside = !inside
		}
	}
	return inside
}

func physicalSegmentsIntersect(a, b, c, d PhysicalClipPoint) bool {
	cross := func(p, q, r PhysicalClipPoint) float64 {
		return (q.XPt-p.XPt)*(r.YPt-p.YPt) - (q.YPt-p.YPt)*(r.XPt-p.XPt)
	}
	const epsilon = 1e-9
	abC, abD := cross(a, b, c), cross(a, b, d)
	cdA, cdB := cross(c, d, a), cross(c, d, b)
	return (abC*abD <= epsilon) && (cdA*cdB <= epsilon)
}

func intersectPhysicalPaintBounds(a, b PhysicalPaintBoundsPlan) (PhysicalPaintBoundsPlan, bool) {
	if a.Empty() || b.Empty() {
		return PhysicalPaintBoundsPlan{}, false
	}
	left := math.Max(a.LeftPt, b.LeftPt)
	top := math.Max(a.TopPt, b.TopPt)
	right := math.Min(a.LeftPt+a.WidthPt, b.LeftPt+b.WidthPt)
	bottom := math.Min(a.TopPt+a.HeightPt, b.TopPt+b.HeightPt)
	if right <= left || bottom <= top {
		return PhysicalPaintBoundsPlan{}, false
	}
	return PhysicalPaintBoundsPlan{LeftPt: left, TopPt: top, WidthPt: right - left, HeightPt: bottom - top}, true
}

func physicalBoundsEqual(a, b PhysicalPaintBoundsPlan) bool {
	const epsilon = 1e-6
	return math.Abs(a.LeftPt-b.LeftPt) <= epsilon && math.Abs(a.TopPt-b.TopPt) <= epsilon &&
		math.Abs(a.WidthPt-b.WidthPt) <= epsilon && math.Abs(a.HeightPt-b.HeightPt) <= epsilon
}

func resolvePhysicalAncestorPaintPlans(shape *ir.Shape, paint PhysicalPaintBoundsPlan, ancestors []materializedAncestorPaint) []PhysicalAncestorPaintPlan {
	if len(ancestors) == 0 {
		return nil
	}
	out := make([]PhysicalAncestorPaintPlan, 0, len(ancestors))
	base := physicalBoundsFromRect(ir.Rect{})
	if shape != nil {
		base = physicalBoundsFromRect(shape.Bounds)
	}
	for _, ancestor := range ancestors {
		bounds := physicalTransformedBounds(physicalBoundsFromRect(ancestor.Bounds), ancestor.RotationDeg)
		out = append(out, PhysicalAncestorPaintPlan{
			Bounds:       bounds,
			Intersects:   physicalBoundsIntersect(bounds, paint),
			ContainsBase: physicalBoundsContains(bounds, base),
		})
	}
	return out
}

func physicalBoundsFromRect(rect ir.Rect) PhysicalPaintBoundsPlan {
	if rect.W <= 0 || rect.H <= 0 {
		return PhysicalPaintBoundsPlan{}
	}
	const emuPerPt = 12700.0
	return PhysicalPaintBoundsPlan{
		LeftPt:   float64(rect.X) / emuPerPt,
		TopPt:    float64(rect.Y) / emuPerPt,
		WidthPt:  float64(rect.W) / emuPerPt,
		HeightPt: float64(rect.H) / emuPerPt,
	}
}

func physicalBoundsIntersect(a, b PhysicalPaintBoundsPlan) bool {
	if a.Empty() || b.Empty() {
		return false
	}
	return a.LeftPt < b.LeftPt+b.WidthPt && b.LeftPt < a.LeftPt+a.WidthPt &&
		a.TopPt < b.TopPt+b.HeightPt && b.TopPt < a.TopPt+a.HeightPt
}

func physicalBoundsContains(outer, inner PhysicalPaintBoundsPlan) bool {
	if outer.Empty() || inner.Empty() {
		return false
	}
	const epsilon = 1e-6
	return inner.LeftPt+epsilon >= outer.LeftPt && inner.TopPt+epsilon >= outer.TopPt &&
		inner.LeftPt+inner.WidthPt <= outer.LeftPt+outer.WidthPt+epsilon &&
		inner.TopPt+inner.HeightPt <= outer.TopPt+outer.HeightPt+epsilon
}

func maxFloat64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// resolvePhysicalPaintBounds materializes the axis-aligned slide-space paint
// bounds after effect overflow and the final shape rotation have been applied.
// This keeps effect-aware hit/clip/paint composition out of renderers.
func resolvePhysicalPaintBounds(shape *ir.Shape, overflow PhysicalOverflowPlan, transform PhysicalTransformPlan) PhysicalPaintBoundsPlan {
	if shape == nil || shape.Bounds.W <= 0 || shape.Bounds.H <= 0 {
		return PhysicalPaintBoundsPlan{}
	}
	const emuPerPt = 12700.0
	x := float64(shape.Bounds.X) / emuPerPt
	y := float64(shape.Bounds.Y) / emuPerPt
	w := float64(shape.Bounds.W) / emuPerPt
	h := float64(shape.Bounds.H) / emuPerPt
	left := x - overflow.LeftPt
	top := y - overflow.TopPt
	right := x + w + overflow.RightPt
	bottom := y + h + overflow.BottomPt

	angle := math.Mod(transform.RotationDeg, 360)
	if math.Abs(angle) < 1e-9 {
		return PhysicalPaintBoundsPlan{LeftPt: left, TopPt: top, WidthPt: right - left, HeightPt: bottom - top}
	}

	cx := x + w/2
	cy := y + h/2
	rad := angle * math.Pi / 180
	cosA, sinA := math.Cos(rad), math.Sin(rad)
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, point := range [][2]float64{{left, top}, {right, top}, {right, bottom}, {left, bottom}} {
		dx, dy := point[0]-cx, point[1]-cy
		rx := cx + dx*cosA - dy*sinA
		ry := cy + dx*sinA + dy*cosA
		minX = math.Min(minX, rx)
		minY = math.Min(minY, ry)
		maxX = math.Max(maxX, rx)
		maxY = math.Max(maxY, ry)
	}
	return PhysicalPaintBoundsPlan{LeftPt: minX, TopPt: minY, WidthPt: maxX - minX, HeightPt: maxY - minY}
}

// resolvePhysicalSVGPaintPlan converts layout-owned effect overflow into an
// SVG paint contract. Inline SVG establishes its own viewport and can clip
// shadows/glow even when the containing HTML shape allows overflow, so layout
// explicitly tells serializers when the vector viewport must remain open. The
// filter region is expressed as percentages relative to the canonical shape
// bounds for renderers that materialize SVG filters.

// resolvePhysicalVectorPaintBounds closes the vector/media paint envelope at
// layout time. Canonical Bounds describe the authored frame, not necessarily
// every painted pixel: strokes extend outside it and endpoint/path connectors
// may not have a usable frame at all. Renderers and ancestor clipping must see
// the final slide-space envelope rather than rediscovering that geometry.
func resolvePhysicalVectorPaintBounds(shape *ir.Shape, base PhysicalPaintBoundsPlan, overflow PhysicalOverflowPlan) PhysicalPaintBoundsPlan {
	if shape == nil {
		return base
	}
	strokePad := math.Max(0, shape.LineWidthPt/2)
	arrowPad := 0.0
	if shape.Kind == ir.ShapeConnector || shape.Kind == ir.ShapeSmartArtConnector {
		for _, spec := range [][3]string{{shape.ArrowHead, shape.ArrowHeadWidth, shape.ArrowHeadLength}, {shape.ArrowTail, shape.ArrowTailWidth, shape.ArrowTailLength}} {
			if spec[0] == "" || spec[0] == "none" {
				continue
			}
			length, width := drawing.ArrowDimensions(spec[0], spec[1], spec[2])
			arrowPad = math.Max(arrowPad, math.Max(length, width)/2)
		}
	}
	pad := math.Max(strokePad, arrowPad)

	// Endpoint/path connectors are already in slide space and frequently carry
	// an empty canonical Bounds rectangle. Materialize their actual path envelope.
	if (shape.Kind == ir.ShapeConnector || shape.Kind == ir.ShapeSmartArtConnector) && shape.ConnectorStart != nil && shape.ConnectorEnd != nil {
		points := make([]ir.Point, 0, len(shape.ConnectorPath)+2)
		points = append(points, *shape.ConnectorStart)
		if len(shape.ConnectorPath) > 2 {
			points = append(points, shape.ConnectorPath[1:len(shape.ConnectorPath)-1]...)
		}
		points = append(points, *shape.ConnectorEnd)
		minX, minY := math.Inf(1), math.Inf(1)
		maxX, maxY := math.Inf(-1), math.Inf(-1)
		for _, pt := range points {
			x, y := drawing.EMUToPoints(pt.X), drawing.EMUToPoints(pt.Y)
			minX, minY = math.Min(minX, x), math.Min(minY, y)
			maxX, maxY = math.Max(maxX, x), math.Max(maxY, y)
		}
		if !math.IsInf(minX, 0) {
			pathBounds := PhysicalPaintBoundsPlan{
				LeftPt:   minX - pad - overflow.LeftPt,
				TopPt:    minY - pad - overflow.TopPt,
				WidthPt:  maxX - minX + 2*pad + overflow.LeftPt + overflow.RightPt,
				HeightPt: maxY - minY + 2*pad + overflow.TopPt + overflow.BottomPt,
			}
			// A connector may also carry a non-empty authored frame. Keep the
			// effect-aware frame envelope as well as the actual endpoint/path
			// envelope instead of allowing one representation to discard the other.
			return unionPhysicalPaintBounds(base, pathBounds)
		}
	}
	if base.Empty() || pad <= 0 {
		return base
	}
	return PhysicalPaintBoundsPlan{LeftPt: base.LeftPt - pad, TopPt: base.TopPt - pad, WidthPt: base.WidthPt + 2*pad, HeightPt: base.HeightPt + 2*pad}
}

func unionPhysicalPaintBounds(a, b PhysicalPaintBoundsPlan) PhysicalPaintBoundsPlan {
	if a.Empty() {
		return b
	}
	if b.Empty() {
		return a
	}
	left := math.Min(a.LeftPt, b.LeftPt)
	top := math.Min(a.TopPt, b.TopPt)
	right := math.Max(a.LeftPt+a.WidthPt, b.LeftPt+b.WidthPt)
	bottom := math.Max(a.TopPt+a.HeightPt, b.TopPt+b.HeightPt)
	return PhysicalPaintBoundsPlan{LeftPt: left, TopPt: top, WidthPt: right - left, HeightPt: bottom - top}
}

func resolvePhysicalSVGPaintPlan(shape *ir.Shape, overflow PhysicalOverflowPlan, effects EffectVisualPlan) PhysicalSVGPaintPlan {
	plan := PhysicalSVGPaintPlan{FilterXPercent: 0, FilterYPercent: 0, FilterWPercent: 100, FilterHPercent: 100}
	if shape == nil || shape.Bounds.W <= 0 || shape.Bounds.H <= 0 {
		return plan
	}
	if overflow.Empty() && !effects.NeedsOverflow {
		return plan
	}
	const emuPerPt = 12700.0
	w := float64(shape.Bounds.W) / emuPerPt
	h := float64(shape.Bounds.H) / emuPerPt
	if w <= 0 || h <= 0 {
		return plan
	}
	left := overflow.LeftPt / w * 100
	right := overflow.RightPt / w * 100
	top := overflow.TopPt / h * 100
	bottom := overflow.BottomPt / h * 100
	plan.OverflowVisible = true
	plan.FilterXPercent = -left
	plan.FilterYPercent = -top
	plan.FilterWPercent = 100 + left + right
	plan.FilterHPercent = 100 + top + bottom
	return plan
}

// materializePhysicalGradientOrientation resolves the one gradient transform
// that cannot be left to the rotated shape container: gradFill rotWithShape=0.
// Once a visual reaches the renderer its gradient angle is already expressed in
// the shape's physical SVG coordinate system.
func materializePhysicalGradientOrientation(visual *ShapeVisualPlan, transform PhysicalTransformPlan, shape *ir.Shape) {
	if visual == nil || visual.Fill.Gradient == nil {
		return
	}
	gradient := *visual.Fill.Gradient
	gradient.Stops = append([]GradientStopVisualPlan(nil), visual.Fill.Gradient.Stops...)
	if gradient.ScaleAngleWithAspect && shape != nil && shape.Bounds.W > 0 && shape.Bounds.H > 0 {
		// msofillShadeScale scales the authored direction by the painted aspect
		// ratio.  Transform the unit direction into the rectangular shape space;
		// square shapes therefore preserve the original angle exactly.
		rad := gradient.AngleDeg * math.Pi / 180
		w, h := float64(shape.Bounds.W), float64(shape.Bounds.H)
		gradient.AngleDeg = math.Atan2(math.Sin(rad)*h, math.Cos(rad)*w) * 180 / math.Pi
	}
	if !gradient.RotateWithShape {
		gradient.AngleDeg -= transform.RotationDeg
	}
	// Reflections change the handedness of the local gradient coordinate system.
	if transform.FlipH != transform.FlipV {
		gradient.AngleDeg = -gradient.AngleDeg
	}
	for gradient.AngleDeg < 0 {
		gradient.AngleDeg += 360
	}
	for gradient.AngleDeg >= 360 {
		gradient.AngleDeg -= 360
	}
	if gradient.Kind == drawing.GradientTypeLinear {
		// Resolve the complete authored linear-gradient coordinate system here.
		// Flip belongs to the gradient coordinate space, while transform flips
		// above affect the physical angle. Keeping both concerns here prevents
		// SVG/CSS renderers from becoming a second gradient geometry engine.
		rad := gradient.AngleDeg * math.Pi / 180
		dx, dy := math.Cos(rad), math.Sin(rad)
		x1, y1 := .5-.5*dx, .5-.5*dy
		x2, y2 := .5+.5*dx, .5+.5*dy
		if gradient.FlipH {
			x1, x2 = 1-x1, 1-x2
		}
		if gradient.FlipV {
			y1, y2 = 1-y1, 1-y2
		}
		gradient.LinearX1, gradient.LinearY1 = x1, y1
		gradient.LinearX2, gradient.LinearY2 = x2, y2
		gradient.HasLinearVector = true
	}
	visual.Fill.Gradient = &gradient
}

// resolvePhysicalImageFrameBounds materializes fillRect into slide-space
// physical bounds and then applies the final picture rotation. Source cropping
// deliberately does not change this destination frame: it changes which source
// pixels occupy the frame, not where the frame paints.
func resolvePhysicalImageFrameBounds(shape *ir.Shape, image ImageVisualPlan) PhysicalPaintBoundsPlan {
	if shape == nil || shape.Bounds.W <= 0 || shape.Bounds.H <= 0 || image.FrameHidden {
		return PhysicalPaintBoundsPlan{}
	}
	left := image.FrameLeft / 100
	top := image.FrameTop / 100
	right := image.FrameRight / 100
	bottom := image.FrameBottom / 100
	wScale := 1 - left - right
	hScale := 1 - top - bottom
	if wScale <= 0 || hScale <= 0 {
		return PhysicalPaintBoundsPlan{}
	}

	// fillRect is a destination rectangle inside the picture shape. When the
	// picture rotates, that rectangle rotates around the authored picture
	// center, not around the fillRect's own center. Resolving a cloned inset
	// shape would silently change the transform origin for asymmetric fillRect.
	const emuPerPt = 12700.0
	x := float64(shape.Bounds.X) / emuPerPt
	y := float64(shape.Bounds.Y) / emuPerPt
	w := float64(shape.Bounds.W) / emuPerPt
	h := float64(shape.Bounds.H) / emuPerPt
	frameLeft := x + w*left
	frameTop := y + h*top
	frameRight := x + w*(1-right)
	frameBottom := y + h*(1-bottom)
	angle := math.Mod(image.Composition.Transform.RotationDeg, 360)
	if math.Abs(angle) < 1e-9 {
		return PhysicalPaintBoundsPlan{LeftPt: frameLeft, TopPt: frameTop, WidthPt: frameRight - frameLeft, HeightPt: frameBottom - frameTop}
	}
	cx, cy := x+w/2, y+h/2
	rad := angle * math.Pi / 180
	cosA, sinA := math.Cos(rad), math.Sin(rad)
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, point := range [][2]float64{{frameLeft, frameTop}, {frameRight, frameTop}, {frameRight, frameBottom}, {frameLeft, frameBottom}} {
		dx, dy := point[0]-cx, point[1]-cy
		rx := cx + dx*cosA - dy*sinA
		ry := cy + dx*sinA + dy*cosA
		minX, minY = math.Min(minX, rx), math.Min(minY, ry)
		maxX, maxY = math.Max(maxX, rx), math.Max(maxY, ry)
	}
	return PhysicalPaintBoundsPlan{LeftPt: minX, TopPt: minY, WidthPt: maxX - minX, HeightPt: maxY - minY}
}

// PhysicalSmartArtPlan captures SmartArt node content that remains after the
// diagram resolver has expanded SmartArt into canonical node shapes. Geometry,
// paint, text-frame metrics, effects and transforms are already carried by the
// ordinary PhysicalVisual plans; this plan owns the remaining node content so
// serializers do not return to canonical SmartArt fields.
type PhysicalSmartArtPlan struct {
	PhysicalCompositionPlan
	// FrameBounds, Transform, Effects and Paint close SmartArt node participation
	// in slide composition. Node geometry/fill/stroke/text remain in the shared
	// physical visual plans, while serializers must not re-read canonical frame
	// geometry or effects merely to place the SmartArt node.
	Paragraphs []*ir.Paragraph
}

func ResolvePhysicalSmartArtPlan(shape *ir.Shape) *PhysicalSmartArtPlan {
	return resolvePhysicalSmartArtPlanWithComposition(shape, resolvePhysicalCompositionPlan(shape))
}

func resolvePhysicalSmartArtPlanWithComposition(shape *ir.Shape, composition PhysicalCompositionPlan) *PhysicalSmartArtPlan {
	if shape == nil || shape.Kind != ir.ShapeSmartArt {
		return nil
	}
	paragraphs := append([]*ir.Paragraph(nil), shape.Paragraphs...)
	return &PhysicalSmartArtPlan{
		PhysicalCompositionPlan: composition,
		Paragraphs:              paragraphs,
	}
}

// PhysicalTablePlan is the renderer-neutral materialization of presentation
// table geometry and cell paint. The renderer may still serialize canonical
// paragraph content, but it must not recalculate grid, merge, margin, border,
// fill, or cell-effect semantics from ir.Table.
type PhysicalTablePlan struct {
	PhysicalCompositionPlan
	// FrameBounds, Transform, Effects and Paint close table participation in
	// slide composition. Serializers consume these values and must not re-read
	// canonical shape geometry/effects to place the table frame.
	ColumnWidthsPercent []float64
	Rows                []PhysicalTableRowPlan
	// Edges is the fully materialized table grid paint plan. Every physical
	// grid segment has exactly one winning stroke, including boundaries touched
	// by merged cells. Renderers must not delegate border conflict resolution
	// to CSS table layout.
	Edges []PhysicalTableEdgePlan
}

type PhysicalTableEdgePlan struct {
	X1Pct, Y1Pct float64
	X2Pct, Y2Pct float64
	Stroke       StrokeVisualPlan
}

type PhysicalTableRowPlan struct {
	HeightPt float64
	Cells    []PhysicalTableCellPlan
}

type PhysicalTableCellPlan struct {
	Skip            bool
	ColumnSpan      int
	RowSpan         int
	Fill            FillVisualPlan
	Effects         EffectVisualPlan
	VerticalAlign   string
	JustifyContent  string
	CellHeightPt    float64
	ContentHeightPt float64
	ClipContent     bool
	PaddingTopPt    float64
	PaddingRightPt  float64
	PaddingBottomPt float64
	PaddingLeftPt   float64
	BorderLeft      StrokeVisualPlan
	BorderRight     StrokeVisualPlan
	BorderTop       StrokeVisualPlan
	BorderBottom    StrokeVisualPlan
	Paragraphs      []*ir.Paragraph
	ParagraphPlans  []ParagraphVisualPlan
}

// PhysicalMediaPlan closes media frame composition at layout. The renderer may
// still choose browser/sidecar serialization for the payload, but placement,
// transform, effects and paint ordering are no longer inferred from canonical IR.
type PhysicalMediaPlan struct {
	PhysicalCompositionPlan
}

func ResolvePhysicalMediaPlan(shape *ir.Shape) *PhysicalMediaPlan {
	return resolvePhysicalMediaPlanWithComposition(shape, resolvePhysicalCompositionPlan(shape))
}

func resolvePhysicalMediaPlanWithComposition(shape *ir.Shape, composition PhysicalCompositionPlan) *PhysicalMediaPlan {
	if shape == nil || shape.Kind != ir.ShapeMedia {
		return nil
	}
	return &PhysicalMediaPlan{PhysicalCompositionPlan: composition}
}

// PhysicalOLEPlan closes OLE frame composition at the layout boundary. The
// embedded payload and link remain content concerns; placement, transform and
// drawing effects are physical presentation concerns.
type PhysicalOLEPlan struct {
	PhysicalCompositionPlan
	Name       string
	ProgID     string
	LinkURL    string
	ObjectMIME string
	ObjectData []byte
	IconMIME   string
	IconData   []byte
}

func ResolvePhysicalOLEPlan(shape *ir.Shape) *PhysicalOLEPlan {
	return resolvePhysicalOLEPlanWithComposition(shape, resolvePhysicalCompositionPlan(shape))
}

func resolvePhysicalOLEPlanWithComposition(shape *ir.Shape, composition PhysicalCompositionPlan) *PhysicalOLEPlan {
	if shape == nil || shape.Kind != ir.ShapeOleObject {
		return nil
	}
	return &PhysicalOLEPlan{
		PhysicalCompositionPlan: composition,
		Name: shape.OLEObjectName, ProgID: shape.OLEObjectProgID,
		LinkURL: shape.OLEObjectLinkURL, ObjectMIME: shape.OLEObjectMIME,
		ObjectData: append([]byte(nil), shape.OLEObjectData...),
		IconMIME: shape.OLEIconMIME, IconData: append([]byte(nil), shape.OLEIconData...),
	}
}

type PhysicalChartPlan struct {
	PhysicalCompositionPlan
	// FrameBounds is the authored slide-space chart frame. Rotation/reflection are
	// carried separately in Transform so renderers can serialize CSS transforms
	// without re-reading canonical EMU geometry.
	MIME        string
	Data        []byte
	Alt         string
	WidthPt     float64
	HeightPt    float64
	ObjectFit   string
	ClipContent bool
}

func ResolvePhysicalTablePlan(shape *ir.Shape) *PhysicalTablePlan {
	return resolvePhysicalTablePlanWithComposition(shape, resolvePhysicalCompositionPlan(shape))
}

func resolvePhysicalTablePlanWithComposition(shape *ir.Shape, composition PhysicalCompositionPlan) *PhysicalTablePlan {
	if shape == nil || shape.Table == nil || len(shape.Table.Rows) == 0 {
		return nil
	}
	plan := &PhysicalTablePlan{PhysicalCompositionPlan: composition}
	var total int64
	for _, w := range shape.Table.ColumnWidthsEMU {
		total += w
	}
	if total > 0 {
		plan.ColumnWidthsPercent = make([]float64, len(shape.Table.ColumnWidthsEMU))
		for i, w := range shape.Table.ColumnWidthsEMU {
			plan.ColumnWidthsPercent[i] = float64(w) * 100 / float64(total)
		}
	}
	for rowIndex, row := range shape.Table.Rows {
		rp := PhysicalTableRowPlan{}
		if row == nil {
			plan.Rows = append(plan.Rows, rp)
			continue
		}
		rp.HeightPt = float64(row.HeightEMU) / 12700.0
		for _, cell := range row.Cells {
			cp := PhysicalTableCellPlan{}
			if cell == nil {
				cp.Skip = true
				rp.Cells = append(rp.Cells, cp)
				continue
			}
			cp.Skip = cell.HMerge || cell.VMerge
			cp.ColumnSpan, cp.RowSpan = cell.GridSpan, cell.RowSpan
			cp.Fill = ResolveFillVisualPlan(cell.Fill, cell.FillColor, nil)
			cp.Effects = ResolveEffectVisualPlan(cell.Effects)
			cp.PaddingLeftPt = float64(cell.MarginLeftEMU) / 12700.0
			cp.PaddingRightPt = float64(cell.MarginRightEMU) / 12700.0
			cp.PaddingTopPt = float64(cell.MarginTopEMU) / 12700.0
			cp.PaddingBottomPt = float64(cell.MarginBottomEMU) / 12700.0
			switch cell.TextAnchor {
			case "ctr":
				cp.VerticalAlign = "middle"
				cp.JustifyContent = "center"
			case "b":
				cp.VerticalAlign = "bottom"
				cp.JustifyContent = "flex-end"
			default:
				cp.VerticalAlign = "top"
				cp.JustifyContent = "flex-start"
			}
			cp.CellHeightPt = physicalTableCellHeightPt(shape.Table.Rows, rowIndex, cp.RowSpan)
			cp.ContentHeightPt = cp.CellHeightPt - cp.PaddingTopPt - cp.PaddingBottomPt
			if cp.ContentHeightPt < 0 {
				cp.ContentHeightPt = 0
			}
			cp.ClipContent = cp.CellHeightPt > 0
			cp.BorderLeft = resolveTableBorderStroke(cell.BorderLeft)
			cp.BorderRight = resolveTableBorderStroke(cell.BorderRight)
			cp.BorderTop = resolveTableBorderStroke(cell.BorderTop)
			cp.BorderBottom = resolveTableBorderStroke(cell.BorderBottom)
			cp.Paragraphs = append([]*ir.Paragraph(nil), cell.Paragraphs...)
			// Table-cell text participates in the same physical paragraph pipeline as
			// ordinary slide text. The cell's resolved grid width minus physical
			// margins is the wrapping extent; serializers must not re-enter layout.
			cellWidthPt := physicalTableCellWidthPt(shape.Table.ColumnWidthsEMU, len(rp.Cells), cp.ColumnSpan)
			cellWidthPt -= cp.PaddingLeftPt + cp.PaddingRightPt
			if cellWidthPt < 0 {
				cellWidthPt = 0
			}
			cp.ParagraphPlans = resolveParagraphPlansForExtent(cp.Paragraphs, cellWidthPt, 1, 1)
			rp.Cells = append(rp.Cells, cp)
		}
		plan.Rows = append(plan.Rows, rp)
	}
	resolvePhysicalTableGridEdges(plan)
	return plan
}

// resolvePhysicalTableGridEdges resolves the complete table border graph into
// unit grid segments. Long sides of merged cells are split at row/column grid
// boundaries, so every shared segment can choose one physical winner without
// relying on CSS border-collapse semantics.
func resolvePhysicalTableGridEdges(plan *PhysicalTablePlan) {
	if plan == nil || len(plan.Rows) == 0 {
		return
	}
	cols := len(plan.ColumnWidthsPercent)
	if cols == 0 {
		for _, row := range plan.Rows {
			if len(row.Cells) > cols {
				cols = len(row.Cells)
			}
		}
	}
	if cols == 0 {
		return
	}
	colPos := make([]float64, cols+1)
	if len(plan.ColumnWidthsPercent) == cols {
		for i, w := range plan.ColumnWidthsPercent {
			colPos[i+1] = colPos[i] + w
		}
	} else {
		for i := 0; i <= cols; i++ {
			colPos[i] = float64(i) * 100 / float64(cols)
		}
	}
	rowPos := make([]float64, len(plan.Rows)+1)
	// Edge coordinates live in the table shape's coordinate system, not in a
	// normalized "sum of rows" box. DrawingML permits the graphic-frame
	// height to exceed the sum of a:tr/@h values (sample-9 is 216pt high while
	// its three rows total 162pt). Normalizing row heights to 100% moved the
	// first-row bottom edge from 54pt to 72pt and painted it through row two.
	// Materialize row boundaries against the authored frame height so cells,
	// rows and the overlay edge layer share one physical coordinate system.
	frameHeightPt := plan.FrameBounds.HeightPt
	if frameHeightPt > 0 {
		for i, row := range plan.Rows {
			rowPos[i+1] = rowPos[i] + row.HeightPt*100/frameHeightPt
		}
	} else {
		var totalH float64
		for _, row := range plan.Rows {
			totalH += row.HeightPt
		}
		if totalH > 0 {
			for i, row := range plan.Rows {
				rowPos[i+1] = rowPos[i] + row.HeightPt*100/totalH
			}
		} else {
			for i := range rowPos {
				rowPos[i] = float64(i) * 100 / float64(len(plan.Rows))
			}
		}
	}
	type key struct {
		axis        byte
		fixed, unit int
	}
	edges := map[key]StrokeVisualPlan{}
	put := func(k key, stroke StrokeVisualPlan) { edges[k] = winningTableStroke(edges[k], stroke) }
	for r := range plan.Rows {
		row := &plan.Rows[r]
		for c := range row.Cells {
			cell := &row.Cells[c]
			if cell.Skip {
				continue
			}
			cs, rs := cell.ColumnSpan, cell.RowSpan
			if cs <= 0 {
				cs = 1
			}
			if rs <= 0 {
				rs = 1
			}
			endC, endR := c+cs, r+rs
			if endC > cols {
				endC = cols
			}
			if endR > len(plan.Rows) {
				endR = len(plan.Rows)
			}
			for u := c; u < endC; u++ {
				put(key{'h', r, u}, cell.BorderTop)
				put(key{'h', endR, u}, cell.BorderBottom)
			}
			for u := r; u < endR; u++ {
				put(key{'v', c, u}, cell.BorderLeft)
				put(key{'v', endC, u}, cell.BorderRight)
			}
			cell.BorderLeft, cell.BorderRight = StrokeVisualPlan{}, StrokeVisualPlan{}
			cell.BorderTop, cell.BorderBottom = StrokeVisualPlan{}, StrokeVisualPlan{}
		}
	}
	for k, stroke := range edges {
		if stroke.WidthPt <= 0 || stroke.Color == "" {
			continue
		}
		e := PhysicalTableEdgePlan{Stroke: stroke}
		if k.axis == 'h' {
			if k.fixed < 0 || k.fixed >= len(rowPos) || k.unit < 0 || k.unit+1 >= len(colPos) {
				continue
			}
			e.X1Pct, e.X2Pct, e.Y1Pct, e.Y2Pct = colPos[k.unit], colPos[k.unit+1], rowPos[k.fixed], rowPos[k.fixed]
		} else {
			if k.fixed < 0 || k.fixed >= len(colPos) || k.unit < 0 || k.unit+1 >= len(rowPos) {
				continue
			}
			e.X1Pct, e.X2Pct, e.Y1Pct, e.Y2Pct = colPos[k.fixed], colPos[k.fixed], rowPos[k.unit], rowPos[k.unit+1]
		}
		plan.Edges = append(plan.Edges, e)
	}
	sort.SliceStable(plan.Edges, func(i, j int) bool {
		a, b := plan.Edges[i], plan.Edges[j]
		if a.Y1Pct != b.Y1Pct {
			return a.Y1Pct < b.Y1Pct
		}
		if a.X1Pct != b.X1Pct {
			return a.X1Pct < b.X1Pct
		}
		if a.Y2Pct != b.Y2Pct {
			return a.Y2Pct < b.Y2Pct
		}
		return a.X2Pct < b.X2Pct
	})
}

func winningTableStroke(a, b StrokeVisualPlan) StrokeVisualPlan {
	if a.WidthPt <= 0 || a.Color == "" {
		return b
	}
	if b.WidthPt <= 0 || b.Color == "" {
		return a
	}
	if b.WidthPt > a.WidthPt {
		return b
	}
	return a
}

func physicalTableCellHeightPt(rows []*ir.TableRow, start, span int) float64 {
	if span <= 0 {
		span = 1
	}
	if start < 0 || start >= len(rows) {
		return 0
	}
	end := start + span
	if end > len(rows) {
		end = len(rows)
	}
	var emu int64
	for _, row := range rows[start:end] {
		if row != nil {
			emu += row.HeightEMU
		}
	}
	return float64(emu) / 12700.0
}

func physicalTableCellWidthPt(widths []int64, start, span int) float64 {
	if span <= 0 {
		span = 1
	}
	end := start + span
	if start < 0 || start >= len(widths) {
		return 0
	}
	if end > len(widths) {
		end = len(widths)
	}
	var emu int64
	for _, w := range widths[start:end] {
		emu += w
	}
	return float64(emu) / 12700.0
}

func resolveParagraphPlansForExtent(paragraphs []*ir.Paragraph, extent, fontScale, lineScale float64) []ParagraphVisualPlan {
	plans := make([]ParagraphVisualPlan, 0, len(paragraphs))
	counters := map[string]int{}
	for _, p := range paragraphs {
		pp := ResolveParagraphVisualPlan(p, extent, fontScale, lineScale)
		if pp.Marker.Kind == "auto" {
			if counters[pp.Marker.CounterKey] == 0 {
				counters[pp.Marker.CounterKey] = pp.Marker.Start
			}
			pp.Marker.Text = formatAutoNumForLayout(pp.Marker.NumberingType, counters[pp.Marker.CounterKey])
			counters[pp.Marker.CounterKey]++
			if CanLayoutParagraphLines(p, pp.Metrics, fontScale) {
				lines := layoutParagraphLinesWithMarker(p, pp.Metrics, fontScale, pp.Marker.Text, pp.Marker.FontSizePt, pp.Marker.FontFamily, pp.Marker.GapPt)
				pp.Lines = append([]PhysicalTextLine(nil), lines.Lines...)
				pp.LinesMeasured = true
			}
		} else {
			clear(counters)
		}
		plans = append(plans, pp)
	}
	return plans
}

func resolveTableBorderStroke(line *drawing.LineStyle) StrokeVisualPlan {
	if line == nil {
		return StrokeVisualPlan{}
	}
	// Table borders are ordinary DrawingML lines. Materialize them through the
	// same stroke pipeline as shapes/connectors so dash, alpha, compound style,
	// caps/joins and theme-resolved colour are not silently discarded.
	plan := ResolveStrokeVisualPlan(line, "000000", 0.75)
	if plan.Alpha <= 0 {
		plan.Alpha = 1
	}
	return plan
}

func ResolvePhysicalChartPlan(shape *ir.Shape) *PhysicalChartPlan {
	return resolvePhysicalChartPlanWithComposition(shape, resolvePhysicalCompositionPlan(shape))
}

func resolvePhysicalChartPlanWithComposition(shape *ir.Shape, composition PhysicalCompositionPlan) *PhysicalChartPlan {
	if shape == nil || shape.Kind != ir.ShapeChart || len(shape.ImageData) == 0 {
		return nil
	}
	data := append([]byte(nil), shape.ImageData...)
	return &PhysicalChartPlan{
		PhysicalCompositionPlan: composition,
		MIME:                    shape.ImageMIME, Data: data, Alt: shape.Name,
		WidthPt: drawing.EMUToPoints(shape.Bounds.W), HeightPt: drawing.EMUToPoints(shape.Bounds.H),
		ObjectFit: "fill", ClipContent: true,
	}
}

// PhysicalSlideBackgroundPlan is the layout-owned slide canvas background.
// Rendering serializes this plan and does not inspect slide background fields.
type PhysicalSlideBackgroundPlan struct {
	Color       string
	Fill        drawing.Fill
	PictureTile *ir.PictureTile
	ZIndex      int
}

// PhysicalSlidePlan is the final physical composition of a slide. Content and
// slide decorations share one paint-order materialization so z-order is stable
// across the composition boundary.
type PhysicalSlidePlaybackPlan struct {
	TransitionType          string
	TransitionSpeed         string
	AdvanceClick            bool
	AdvanceTimeMs           int
	MaxAnimationStep        int
	AnimationTimeline       []ir.AnimationEvent
	AnimationTargetShapeIDs map[int]struct{}
	HasInteractiveBehavior  bool
}

func (p PhysicalSlidePlaybackPlan) HasAnimationTimeline() bool { return len(p.AnimationTimeline) > 0 }

type PhysicalNotesPlan struct {
	BackgroundColor string
	Content         []PhysicalVisual
}

type PhysicalCommentPlan struct {
	AuthorName string
	Initials   string
	DateTime   string
	Text       string
	AnchorXEMU int64
	AnchorYEMU int64
}

type PhysicalSlideAuxiliaryPlan struct {
	Notes    *PhysicalNotesPlan
	Comments []PhysicalCommentPlan
}

type PhysicalSlidePlan struct {
	Index       int
	SectionName string
	Background  PhysicalSlideBackgroundPlan
	Playback    PhysicalSlidePlaybackPlan
	// PaintOrder is the single layout-owned slide paint sequence. Content,
	// decorations and slide-number projections remain available for semantic
	// consumers, but renderers must serialize PaintOrder to preserve interleaving.
	PaintOrder  []PhysicalVisual
	Content     []PhysicalVisual
	Decorations []PhysicalVisual
	SlideNumber *PhysicalVisual
	Auxiliary   PhysicalSlideAuxiliaryPlan
}

// MaterializePhysicalSlide closes slide-level composition before rendering.
func MaterializePhysicalSlide(slide *ir.Slide) PhysicalSlidePlan {
	if slide == nil {
		return PhysicalSlidePlan{}
	}
	shapes := make([]*ir.Shape, 0, len(slide.Shapes)+4)
	shapes = append(shapes, slide.Shapes...)
	for _, shape := range []*ir.Shape{slide.Header, slide.Footer, slide.DateTime, slide.SlideNumber} {
		if shape != nil {
			shapes = append(shapes, shape)
		}
	}
	all := MaterializePhysicalVisualTree(shapes)
	contentSet := make(map[*ir.Shape]struct{}, len(slide.Shapes))
	for _, shape := range slide.Shapes {
		contentSet[shape] = struct{}{}
	}
	playback := PhysicalSlidePlaybackPlan{
		AdvanceClick:            true,
		MaxAnimationStep:        physicalMaxAnimationStep(slide.Shapes),
		AnimationTimeline:       append([]ir.AnimationEvent(nil), slide.AnimationTimeline...),
		AnimationTargetShapeIDs: physicalAnimationTargetShapeIDs(slide.AnimationTimeline),
		HasInteractiveBehavior:  physicalShapesHaveInteractiveBehavior(slide.Shapes),
	}
	if slide.Transition != nil {
		playback.TransitionType = slide.Transition.Type
		playback.TransitionSpeed = slide.Transition.Spd
		playback.AdvanceClick = slide.Transition.AdvClick
		playback.AdvanceTimeMs = slide.Transition.AdvTmMs
	}
	plan := PhysicalSlidePlan{Index: slide.Index, SectionName: slide.SectionName, Background: PhysicalSlideBackgroundPlan{
		Color: slide.BgColor, Fill: slide.BgFill, PictureTile: slide.BgPictureTile, ZIndex: 0,
	}, Playback: playback, Auxiliary: materializePhysicalSlideAuxiliary(slide)}
	plan.PaintOrder = append(plan.PaintOrder, all...)
	for i := range all {
		item := all[i]
		if item.SourceShape == slide.SlideNumber {
			copy := item
			plan.SlideNumber = &copy
			continue
		}
		if _, ok := contentSet[item.SourceShape]; ok {
			plan.Content = append(plan.Content, item)
		} else {
			plan.Decorations = append(plan.Decorations, item)
		}
	}
	return plan
}

func physicalShapesHaveInteractiveBehavior(shapes []*ir.Shape) bool {
	for _, shape := range shapes {
		if shape == nil {
			continue
		}
		if len(shape.EmphasisEffects) > 0 || shape.MotionPath != nil || len(shape.AnimationTriggers) > 0 || shape.ActionLink != "" {
			return true
		}
		if physicalShapesHaveInteractiveBehavior(shape.Children) {
			return true
		}
	}
	return false
}

func physicalMaxAnimationStep(shapes []*ir.Shape) int {
	maxStep := 0
	for _, shape := range shapes {
		if shape == nil {
			continue
		}
		if shape.AnimStep > maxStep {
			maxStep = shape.AnimStep
		}
		if child := physicalMaxAnimationStep(shape.Children); child > maxStep {
			maxStep = child
		}
	}
	return maxStep
}

func physicalAnimationTargetShapeIDs(events []ir.AnimationEvent) map[int]struct{} {
	if len(events) == 0 {
		return nil
	}
	ids := make(map[int]struct{}, len(events))
	for _, event := range events {
		if event.ShapeID > 0 {
			ids[event.ShapeID] = struct{}{}
		}
	}
	if len(ids) == 0 {
		return nil
	}
	return ids
}

func materializePhysicalSlideAuxiliary(slide *ir.Slide) PhysicalSlideAuxiliaryPlan {
	if slide == nil {
		return PhysicalSlideAuxiliaryPlan{}
	}
	plan := PhysicalSlideAuxiliaryPlan{}
	if slide.Notes != nil {
		plan.Notes = &PhysicalNotesPlan{
			BackgroundColor: slide.Notes.BgColor,
			Content:         MaterializePhysicalVisualTree(slide.Notes.Shapes),
		}
	}
	if len(slide.Comments) > 0 {
		plan.Comments = make([]PhysicalCommentPlan, 0, len(slide.Comments))
		for _, comment := range slide.Comments {
			if comment == nil {
				continue
			}
			plan.Comments = append(plan.Comments, PhysicalCommentPlan{
				AuthorName: comment.AuthorName, Initials: comment.Initials, DateTime: comment.DateTime, Text: comment.Text,
				AnchorXEMU: comment.X, AnchorYEMU: comment.Y,
			})
		}
	}
	return plan
}
