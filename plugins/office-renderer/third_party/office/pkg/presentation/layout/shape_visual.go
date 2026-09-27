package layout

import (
	"math"
	"office/pkg/ooxml/drawing"
	"office/pkg/presentation/ir"
	"office/pkg/render/metafile"
	"sort"
	"strconv"
	"strings"
)

// ShapeVisualPlan is the format-independent materialization plan for a shape's
// canonical geometry, fill and stroke. Renderers consume this plan instead of
// independently deciding whether DrawingML visuals require vector geometry.
type GradientStopVisualPlan struct {
	Position float64
	Color    string
	Alpha    float64
}

type GradientVisualPlan struct {
	Kind                 drawing.GradientType
	AngleDeg             float64
	Stops                []GradientStopVisualPlan
	FlipH                bool
	FlipV                bool
	RotateWithShape      bool
	ScaleAngleWithAspect bool
	LegacyShadeType      string
	CenterX              float64
	CenterY              float64
	CenterWidth          float64
	CenterHeight         float64
	HasCenter            bool
	// Linear endpoints are materialized by physical layout after authored flip,
	// shadeScale/aspect and shape transform semantics have been resolved. Renderers
	// serialize these coordinates and do not reinterpret gradient orientation.
	LinearX1        float64
	LinearY1        float64
	LinearX2        float64
	LinearY2        float64
	HasLinearVector bool
	// PathBoundary records the preset boundary used by an OfficeArt ShadeShape fill.
	// SVG can exactly materialize ellipse-boundary path gradients; arbitrary shape
	// boundaries require a richer path-gradient backend.
	PathBoundary string
	// PathBoundaryPath carries the materialized shape outline for OfficeArt
	// ShadeShape fills whose boundary is neither a rectangle nor an ellipse.
	// Coordinates are physical shape-local points and remain renderer-agnostic.
	PathBoundaryPath     *drawing.Path
	PathBoundaryWidthPt  float64
	PathBoundaryHeightPt float64
}

type FillVisualPlan struct {
	Fill            drawing.Fill
	FallbackColor   string
	Kind            drawing.FillType
	Visible         bool
	NeedsDefinition bool
	HasTransparency bool
	PictureTile     *ir.PictureTile
	Gradient        *GradientVisualPlan
}

type EffectOverlayPlan struct {
	Fill            drawing.Fill
	BlendMode       string
	NeedsDefinition bool
}

type GeometryVisualPlan struct {
	Preset             string
	Custom             bool
	Adjustments        []float64
	ClipKind           string
	ContentInsetTop    float64
	ContentInsetRight  float64
	ContentInsetBottom float64
	ContentInsetLeft   float64
}

type StrokeVisualPlan struct {
	Color          string
	WidthPt        float64
	CSSStyle       string
	SVGLineCap     string
	SVGLineJoin    string
	MiterLimit     float64
	DashArray      string
	Compound       bool
	CompoundType   string
	CompoundWidths []float64
	HasArrowheads  bool
	NeedsVector    bool
	Alpha          float64
	AlphaSet       bool
	Fill           drawing.Fill
}

type EffectTransformStep struct {
	Kind     string
	X        float64
	Y        float64
	ScaleX   float64
	ScaleY   float64
	SkewXDeg float64
	SkewYDeg float64
}

type ReflectionVisualPlan struct {
	DirectionDeg    float64
	DistancePt      float64
	BlurPt          float64
	Size            float64
	ScaleX          float64
	ScaleY          float64
	SkewXDeg        float64
	SkewYDeg        float64
	RotateWithShape bool
	Alpha           float64
	StartAlpha      float64
	EndAlpha        float64
	StartPosition   float64
	EndPosition     float64
}

type ThreeDVisualPlan struct {
	CameraFOV           float64
	CameraZoom          float64
	RotationXDeg        float64
	RotationYDeg        float64
	RotationZDeg        float64
	LightRotationXDeg   float64
	LightRotationYDeg   float64
	LightRotationZDeg   float64
	Material            string
	CameraPreset        string
	LightRig            string
	LightDirection      string
	ExtrusionDepthPt    float64
	ContourWidthPt      float64
	BevelTopWidthPt     float64
	BevelTopHeightPt    float64
	BevelTopPreset      string
	BevelBottomWidthPt  float64
	BevelBottomHeightPt float64
	BevelBottomPreset   string
	ContourColor        string
	ExtrusionColor      string
	LightingOverlays    []EffectOverlayPlan
}

type EffectCompositionPlan struct {
	Transforms []EffectTransformStep
	BlendMode  string
	Reflection *ReflectionVisualPlan
	ThreeD     *ThreeDVisualPlan
}

type ShadowVisualPlan struct {
	Inner           bool
	Color           string
	Alpha           float64
	BlurPt          float64
	OffsetXPt       float64
	OffsetYPt       float64
	RotateWithShape bool
}

type GlowVisualPlan struct {
	Color           string
	Alpha, RadiusPt float64
}
type BlurVisualPlan struct {
	RadiusPt float64
	Grow     bool
}
type SoftEdgeVisualPlan struct{ RadiusPt float64 }

type EffectVisualPlan struct {
	HasEffects     bool
	HasShadow      bool
	HasInnerShadow bool
	HasGlow        bool
	HasSoftEdge    bool
	HasReflection  bool
	HasBlur        bool
	HasThreeD      bool
	NeedsOverflow  bool
	Composition    EffectCompositionPlan
	Overlays       []EffectOverlayPlan
	Shadows        []ShadowVisualPlan
	Glows          []GlowVisualPlan
	Blurs          []BlurVisualPlan
	SoftEdges      []SoftEdgeVisualPlan
}

// ConnectorVisualPlan is the format-independent physical/materialization plan
// for connector geometry and stroke. It keeps connector construction out of
// HTML renderers while preserving DrawingML's shared primitive builder.
type VectorPrimitivePlan struct {
	Primitive drawing.VectorPrimitive
	Stroke    StrokeVisualPlan
}

func resolveVectorPrimitivePlans(primitives []drawing.VectorPrimitive) []VectorPrimitivePlan {
	out := make([]VectorPrimitivePlan, 0, len(primitives))
	for _, primitive := range primitives {
		out = append(out, VectorPrimitivePlan{
			Primitive: primitive,
			Stroke:    ResolveStrokeVisualPlan(primitive.Style.Stroke, primitive.Style.StrokeColor, primitive.Style.StrokeWidth),
		})
	}
	return out
}

type ConnectorVisualPlan struct {
	Composition PhysicalCompositionPlan
	BoundsXPt   float64
	BoundsYPt   float64
	BoundsWPt   float64
	BoundsHPt   float64
	Type        string
	Adjustments []int
	FlipH       bool
	FlipV       bool
	ArrowHead   string
	ArrowTail   string
	Stroke      StrokeVisualPlan
	Primitives  []VectorPrimitivePlan
}

// ResolveConnectorVisualPlan projects canonical connector state into physical
// point-space once. Renderers consume Spec/primitives and materialize SVG only.
func resolveConnectorVisualPlanWithComposition(shape *ir.Shape, composition PhysicalCompositionPlan) ConnectorVisualPlan {
	plan := ResolveConnectorVisualPlan(shape)
	plan.Composition = composition
	return plan
}

func ResolveConnectorVisualPlan(shape *ir.Shape) ConnectorVisualPlan {
	if shape == nil {
		return ConnectorVisualPlan{}
	}
	bx := drawing.EMUToPoints(shape.Bounds.X)
	by := drawing.EMUToPoints(shape.Bounds.Y)
	bw := drawing.EMUToPoints(shape.Bounds.W)
	bh := drawing.EMUToPoints(shape.Bounds.H)
	stroke := ResolveStrokeVisualPlan(shape.LineStyle, shape.LineColor, shape.LineWidthPt)
	connectorType := shape.ConnectorType
	// Legacy PPT line/connector records can arrive without an OOXML preset name.
	// They are still authored connectors: their bounding box is the physical
	// start/end vector. Treat the missing preset as the DrawingML straight
	// connector instead of materializing an empty SVG layer.
	if connectorType == "" && (shape.Bounds.W != 0 || shape.Bounds.H != 0) {
		connectorType = "straightConnector1"
	}
	spec := drawing.ConnectorSpec{
		Type: connectorType, BoundsX: bx, BoundsY: by, BoundsW: bw, BoundsH: bh,
		Adjustments: append([]int(nil), shape.ConnectorAdjustments...),
		FlipH:       shape.ConnectorFlipH, FlipV: shape.ConnectorFlipV,
		LineStyle: shape.LineStyle, LineColor: stroke.Color, LineWidthPt: stroke.WidthPt,
		ArrowHead: shape.ArrowHead, ArrowHeadWidth: shape.ArrowHeadWidth, ArrowHeadLength: shape.ArrowHeadLength,
		ArrowTail: shape.ArrowTail, ArrowTailWidth: shape.ArrowTailWidth, ArrowTailLength: shape.ArrowTailLength,
	}
	primitives := drawing.BuildConnectorPrimitives(spec)
	return ConnectorVisualPlan{
		BoundsXPt: bx, BoundsYPt: by, BoundsWPt: bw, BoundsHPt: bh,
		Composition: resolvePhysicalCompositionPlan(shape), Type: connectorType,
		Adjustments: append([]int(nil), shape.ConnectorAdjustments...),
		FlipH:       shape.ConnectorFlipH, FlipV: shape.ConnectorFlipV,
		ArrowHead: shape.ArrowHead, ArrowTail: shape.ArrowTail,
		Stroke: stroke, Primitives: resolveVectorPrimitivePlans(primitives),
	}
}

// FreeConnectorVisualPlan is the fully materialized slide-space plan for a
// connector whose authored representation is an endpoint/path sequence rather
// than a bounded preset connector. Layout owns endpoint trimming, curve
// tangents, and arrowhead geometry so renderers only serialize primitives.
type CubicConnectorSegment struct {
	Start, Control1, Control2, End drawing.Point
}

type FreeConnectorVisualPlan struct {
	Composition   PhysicalCompositionPlan
	Points        []drawing.Point
	Curved        bool
	Control1      drawing.Point
	Control2      drawing.Point
	CurveSegments []CubicConnectorSegment
	Stroke        StrokeVisualPlan
	StartArrow    *drawing.ArrowGeometry
	EndArrow      *drawing.ArrowGeometry
}

// ResolveFreeConnectorVisualPlan projects canonical connector endpoints/path
// into physical point-space and resolves all geometry affected by arrowheads.
func resolveFreeConnectorVisualPlanWithComposition(shape *ir.Shape, composition PhysicalCompositionPlan) FreeConnectorVisualPlan {
	plan := ResolveFreeConnectorVisualPlan(shape)
	plan.Composition = composition
	return plan
}

func ResolveFreeConnectorVisualPlan(shape *ir.Shape) FreeConnectorVisualPlan {
	if shape == nil || shape.ConnectorStart == nil || shape.ConnectorEnd == nil {
		return FreeConnectorVisualPlan{}
	}
	stroke := ResolveStrokeVisualPlan(shape.LineStyle, shape.LineColor, shape.LineWidthPt)
	if stroke.Color == "" {
		stroke.Color = "999999"
	}
	if stroke.WidthPt == 0 {
		stroke.WidthPt = 1
	}
	startPt := drawing.Point{X: drawing.EMUToPoints(shape.ConnectorStart.X), Y: drawing.EMUToPoints(shape.ConnectorStart.Y)}
	endPt := drawing.Point{X: drawing.EMUToPoints(shape.ConnectorEnd.X), Y: drawing.EMUToPoints(shape.ConnectorEnd.Y)}
	pts := []drawing.Point{startPt}
	if len(shape.ConnectorPath) > 2 {
		for _, p := range shape.ConnectorPath[1 : len(shape.ConnectorPath)-1] {
			pts = append(pts, drawing.Point{X: drawing.EMUToPoints(p.X), Y: drawing.EMUToPoints(p.Y)})
		}
	} else if strings.HasPrefix(shape.ConnectorType, "bentConnector") || strings.HasPrefix(shape.ConnectorType, "curvedConnector") {
		// Solver-connected legacy PPT connectors carry resolved attachment points,
		// but their authored preset still owns the elbow/curve route. Re-materialize
		// that route in the endpoint envelope instead of degrading it to a straight
		// free connector. The solver points are already in slide space, so endpoint
		// direction supplies the preset flips.
		left, right := math.Min(startPt.X, endPt.X), math.Max(startPt.X, endPt.X)
		top, bottom := math.Min(startPt.Y, endPt.Y), math.Max(startPt.Y, endPt.Y)
		routed := drawing.ConnectorWaypoints(shape.ConnectorType, left, top, right-left, bottom-top, shape.ConnectorAdjustments, endPt.X < startPt.X, endPt.Y < startPt.Y)
		if len(routed) >= 2 {
			pts = routed[:len(routed)-1]
		}
	}
	pts = append(pts, endPt)
	originalStart, originalEnd := pts[0], pts[len(pts)-1]

	// Curves need a different ownership rule from polylines. Marker trimming is
	// distance along the authored Bezier, not a straight chord to the first
	// control point. Build the untrimmed curve first, retain its endpoint
	// tangents for marker orientation, then consume marker depth by arc length.
	var originalCurves []CubicConnectorSegment
	if (shape.ConnectorType == "curved" || shape.ConnectorType == "longCurve") && len(pts) == 2 {
		c1, c2 := connectorCurveControlPoints(originalStart, originalEnd, shape.ConnectorType == "longCurve")
		originalCurves = []CubicConnectorSegment{{Start: originalStart, Control1: c1, Control2: c2, End: originalEnd}}
	} else if strings.HasPrefix(shape.ConnectorType, "curvedConnector") && len(pts) >= 4 {
		originalCurves = materializePresetCurveSegments(pts)
	}
	if len(originalCurves) > 0 {
		curves := append([]CubicConnectorSegment(nil), originalCurves...)
		if shape.ArrowHead != "" {
			curves = trimCubicConnectorSegments(curves, drawing.ArrowTrimDistanceForStroke(shape.ArrowHead, shape.ArrowHeadLength, stroke.WidthPt), true)
		}
		if shape.ArrowTail != "" {
			curves = trimCubicConnectorSegments(curves, drawing.ArrowTrimDistanceForStroke(shape.ArrowTail, shape.ArrowTailLength, stroke.WidthPt), false)
		}
		plan := FreeConnectorVisualPlan{Composition: resolvePhysicalCompositionPlan(shape), Stroke: stroke, Curved: true, CurveSegments: curves}
		if len(curves) > 0 {
			plan.Control1 = curves[0].Control1
			plan.Control2 = curves[len(curves)-1].Control2
			plan.Points = make([]drawing.Point, 0, len(curves)+1)
			plan.Points = append(plan.Points, curves[0].Start)
			for _, seg := range curves {
				plan.Points = append(plan.Points, seg.End)
			}
		} else {
			// Two very large endpoint markers may consume the whole visible shaft.
			// Marker paint still belongs to the authored tips even when there is no
			// remaining Bezier segment to serialize.
			plan.Points = []drawing.Point{originalStart, originalEnd}
		}
		startRef := originalCurves[0].Control1
		endRef := originalCurves[len(originalCurves)-1].Control2
		plan.StartArrow = drawing.ComputeArrowGeometryForStroke(shape.ArrowHead, shape.ArrowHeadWidth, shape.ArrowHeadLength, stroke.WidthPt, originalStart, startRef)
		plan.EndArrow = drawing.ComputeArrowGeometryForStroke(shape.ArrowTail, shape.ArrowTailWidth, shape.ArrowTailLength, stroke.WidthPt, originalEnd, endRef)
		return plan
	}

	// Marker orientation belongs to the authored path tangent, not to the shaft
	// after endpoint trimming. On short bidirectional-arrow lines both trims can
	// consume the entire shaft; retaining the original adjacent points keeps both
	// marker geometries materializable even when the visible shaft collapses.
	originalStartRef, originalEndRef := pts[1], pts[len(pts)-2]
	if shape.ArrowHead != "" {
		pts = trimConnectorEndpointAcrossPolyline(pts, drawing.ArrowTrimDistanceForStroke(shape.ArrowHead, shape.ArrowHeadLength, stroke.WidthPt), true)
	}
	if shape.ArrowTail != "" {
		pts = trimConnectorEndpointAcrossPolyline(pts, drawing.ArrowTrimDistanceForStroke(shape.ArrowTail, shape.ArrowTailLength, stroke.WidthPt), false)
	}
	plan := FreeConnectorVisualPlan{Composition: resolvePhysicalCompositionPlan(shape), Points: pts, Stroke: stroke}
	plan.StartArrow = drawing.ComputeArrowGeometryForStroke(shape.ArrowHead, shape.ArrowHeadWidth, shape.ArrowHeadLength, stroke.WidthPt, originalStart, originalStartRef)
	plan.EndArrow = drawing.ComputeArrowGeometryForStroke(shape.ArrowTail, shape.ArrowTailWidth, shape.ArrowTailLength, stroke.WidthPt, originalEnd, originalEndRef)
	return plan
}

func materializePresetCurveSegments(points []drawing.Point) []CubicConnectorSegment {
	if len(points) < 2 {
		return nil
	}
	// curvedConnector2 is authored directly as one cubic: endpoint, two
	// controls, endpoint. Preserve those controls exactly.
	if len(points) == 4 {
		return []CubicConnectorSegment{{Start: points[0], Control1: points[1], Control2: points[2], End: points[3]}}
	}
	// Higher-order legacy curved presets expose a sequence of guide points.
	// Materialize a C1-continuous cubic spline through them rather than
	// serializing the guides as a visibly broken polyline. Catmull-Rom to
	// cubic conversion keeps endpoint ownership in layout and gives arrows
	// the actual first/last curve tangents.
	out := make([]CubicConnectorSegment, 0, len(points)-1)
	for i := 0; i < len(points)-1; i++ {
		p0 := points[maxInt(0, i-1)]
		p1 := points[i]
		p2 := points[i+1]
		p3 := points[minInt(len(points)-1, i+2)]
		c1 := drawing.Point{X: p1.X + (p2.X-p0.X)/6, Y: p1.Y + (p2.Y-p0.Y)/6}
		c2 := drawing.Point{X: p2.X - (p3.X-p1.X)/6, Y: p2.Y - (p3.Y-p1.Y)/6}
		out = append(out, CubicConnectorSegment{Start: p1, Control1: c1, Control2: c2, End: p2})
	}
	return out
}

// trimCubicConnectorSegments consumes marker depth by arc length while
// preserving the authored Bezier shape. It may remove whole short cubic
// segments before splitting the first/last surviving segment.
func trimCubicConnectorSegments(in []CubicConnectorSegment, trim float64, fromStart bool) []CubicConnectorSegment {
	out := append([]CubicConnectorSegment(nil), in...)
	if len(out) == 0 || trim <= 0 {
		return out
	}
	if fromStart {
		for len(out) > 0 && trim > 0 {
			segLen := cubicConnectorArcLength(out[0], 1)
			if segLen <= 1e-9 {
				out = out[1:]
				continue
			}
			if trim >= segLen-1e-9 {
				trim -= segLen
				out = out[1:]
				continue
			}
			t := cubicConnectorParameterAtArcLength(out[0], trim, true)
			_, right := splitCubicConnectorSegment(out[0], t)
			out[0] = right
			return out
		}
		return out
	}
	for len(out) > 0 && trim > 0 {
		i := len(out) - 1
		segLen := cubicConnectorArcLength(out[i], 1)
		if segLen <= 1e-9 {
			out = out[:i]
			continue
		}
		if trim >= segLen-1e-9 {
			trim -= segLen
			out = out[:i]
			continue
		}
		t := cubicConnectorParameterAtArcLength(out[i], trim, false)
		left, _ := splitCubicConnectorSegment(out[i], t)
		out[i] = left
		return out
	}
	return out
}

// cubicConnectorParameterAtArcLength returns t such that trim points of arc
// length are removed from the selected endpoint. Binary search uses a stable
// polyline approximation; connector marker geometry does not require a costly
// exact elliptic integral and this is sub-pixel at presentation scale.
func cubicConnectorParameterAtArcLength(seg CubicConnectorSegment, trim float64, fromStart bool) float64 {
	full := cubicConnectorArcLength(seg, 1)
	if trim <= 0 || full <= 0 {
		if fromStart {
			return 0
		}
		return 1
	}
	if trim >= full {
		if fromStart {
			return 1
		}
		return 0
	}
	lo, hi := 0.0, 1.0
	for i := 0; i < 28; i++ {
		mid := (lo + hi) / 2
		var got float64
		if fromStart {
			got = cubicConnectorArcLength(seg, mid)
		} else {
			_, right := splitCubicConnectorSegment(seg, mid)
			got = cubicConnectorArcLength(right, 1)
		}
		if got < trim {
			if fromStart {
				lo = mid
			} else {
				hi = mid
			}
		} else {
			if fromStart {
				hi = mid
			} else {
				lo = mid
			}
		}
	}
	return (lo + hi) / 2
}

func cubicConnectorArcLength(seg CubicConnectorSegment, t float64) float64 {
	if t <= 0 {
		return 0
	}
	if t > 1 {
		t = 1
	}
	// 64 chords are cheap for connector-sized curves and sufficiently accurate
	// for marker trimming, including large arrowheads on short curves.
	const steps = 64
	prev := seg.Start
	total := 0.0
	for i := 1; i <= steps; i++ {
		u := t * float64(i) / steps
		p := cubicConnectorPoint(seg, u)
		total += math.Hypot(p.X-prev.X, p.Y-prev.Y)
		prev = p
	}
	return total
}

func cubicConnectorPoint(seg CubicConnectorSegment, t float64) drawing.Point {
	u := 1 - t
	b0 := u * u * u
	b1 := 3 * u * u * t
	b2 := 3 * u * t * t
	b3 := t * t * t
	return drawing.Point{
		X: b0*seg.Start.X + b1*seg.Control1.X + b2*seg.Control2.X + b3*seg.End.X,
		Y: b0*seg.Start.Y + b1*seg.Control1.Y + b2*seg.Control2.Y + b3*seg.End.Y,
	}
}

func splitCubicConnectorSegment(seg CubicConnectorSegment, t float64) (CubicConnectorSegment, CubicConnectorSegment) {
	lerp := func(a, b drawing.Point) drawing.Point {
		return drawing.Point{X: a.X + (b.X-a.X)*t, Y: a.Y + (b.Y-a.Y)*t}
	}
	p01 := lerp(seg.Start, seg.Control1)
	p12 := lerp(seg.Control1, seg.Control2)
	p23 := lerp(seg.Control2, seg.End)
	p012 := lerp(p01, p12)
	p123 := lerp(p12, p23)
	p := lerp(p012, p123)
	return CubicConnectorSegment{Start: seg.Start, Control1: p01, Control2: p012, End: p}, CubicConnectorSegment{Start: p, Control1: p123, Control2: p23, End: seg.End}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// trimConnectorEndpointAcrossPolyline removes marker depth along the actual
// polyline, consuming short endpoint segments instead of clamping to the first
// vertex. This keeps the visible shaft behind the marker for thick strokes and
// large arrow presets.
func trimConnectorEndpointAcrossPolyline(pts []drawing.Point, trim float64, tail bool) []drawing.Point {
	out := append([]drawing.Point(nil), pts...)
	if len(out) < 2 || trim <= 0 {
		return out
	}
	if tail {
		for len(out) >= 2 && trim > 0 {
			a, b := out[0], out[1]
			seg := math.Hypot(b.X-a.X, b.Y-a.Y)
			if seg == 0 {
				out = out[1:]
				continue
			}
			if trim < seg {
				out[0] = drawing.Point{X: a.X + (b.X-a.X)*trim/seg, Y: a.Y + (b.Y-a.Y)*trim/seg}
				return out
			}
			trim -= seg
			out = out[1:]
		}
		return out
	}
	for len(out) >= 2 && trim > 0 {
		i := len(out) - 1
		a, b := out[i], out[i-1]
		seg := math.Hypot(b.X-a.X, b.Y-a.Y)
		if seg == 0 {
			out = out[:i]
			continue
		}
		if trim < seg {
			out[i] = drawing.Point{X: a.X + (b.X-a.X)*trim/seg, Y: a.Y + (b.Y-a.Y)*trim/seg}
			return out
		}
		trim -= seg
		out = out[:i]
	}
	return out
}

func trimConnectorEndpoint(pts []drawing.Point, trim float64, tail bool) []drawing.Point {
	out := append([]drawing.Point(nil), pts...)
	if len(out) < 2 || trim <= 0 {
		return out
	}
	var a, b drawing.Point
	if tail {
		a, b = out[0], out[1]
	} else {
		a, b = out[len(out)-1], out[len(out)-2]
	}
	dx, dy := b.X-a.X, b.Y-a.Y
	mag := math.Hypot(dx, dy)
	if mag == 0 {
		return out
	}
	d := math.Min(trim, mag)
	np := drawing.Point{X: a.X + dx*d/mag, Y: a.Y + dy*d/mag}
	if tail {
		out[0] = np
	} else {
		out[len(out)-1] = np
	}
	return out
}

func connectorCurveControlPoints(start, end drawing.Point, long bool) (drawing.Point, drawing.Point) {
	dx, dy := end.X-start.X, end.Y-start.Y
	f1, f2 := 0.35, 0.65
	if long {
		f1, f2 = 0.18, 0.82
	}
	if math.Abs(dx) >= math.Abs(dy) {
		return drawing.Point{X: start.X + dx*f1, Y: start.Y}, drawing.Point{X: start.X + dx*f2, Y: end.Y}
	}
	return drawing.Point{X: start.X, Y: start.Y + dy*f1}, drawing.Point{X: end.X, Y: start.Y + dy*f2}
}

// PhysicalTransformPlan is the format-independent physical transform applied to
// a materialized visual. Group transforms have already been composed by layout;
// this plan represents only the final slide-space rotation/reflection.
type PhysicalTransformPlan struct {
	RotationDeg float64
	FlipH       bool
	FlipV       bool
}

// ImageVisualPlan materializes picture placement semantics before rendering.
// Crop describes the visible source rectangle. Fill describes the destination
// frame rectangle. Values are percentages ready for physical rendering.
type ImageEffectVisualPlan struct {
	Type   string
	Int1   int
	Int2   int
	Int3   int
	Float1 float64
	Float2 float64
	Bool1  bool
	Color1 string
	Color2 string
}

func ResolveImageEffectVisualPlans(effects []drawing.Effect) []ImageEffectVisualPlan {
	if len(effects) == 0 {
		return nil
	}
	out := make([]ImageEffectVisualPlan, 0, len(effects))
	for _, effect := range effects {
		if effect == nil {
			continue
		}
		plan := ImageEffectVisualPlan{Type: string(effect.Type())}
		switch e := effect.(type) {
		case drawing.BiLevel:
			plan.Int1 = e.Threshold
		case *drawing.BiLevel:
			if e != nil {
				plan.Int1 = e.Threshold
			}
		case drawing.Duotone:
			plan.Color1, plan.Color2 = e.Color1, e.Color2
		case *drawing.Duotone:
			if e != nil {
				plan.Color1, plan.Color2 = e.Color1, e.Color2
			}
		case drawing.Lum:
			plan.Int1, plan.Int2 = e.Brightness, e.Contrast
		case *drawing.Lum:
			if e != nil {
				plan.Int1, plan.Int2 = e.Brightness, e.Contrast
			}
		case drawing.HSLEffect:
			plan.Int1, plan.Int2, plan.Int3 = e.Hue, e.Sat, e.Lum
		case *drawing.HSLEffect:
			if e != nil {
				plan.Int1, plan.Int2, plan.Int3 = e.Hue, e.Sat, e.Lum
			}
		case drawing.DeSatMod:
			plan.Int1 = e.Amount
		case *drawing.DeSatMod:
			if e != nil {
				plan.Int1 = e.Amount
			}
		case drawing.AlphaMod:
			plan.Int1 = e.Amount
		case *drawing.AlphaMod:
			if e != nil {
				plan.Int1 = e.Amount
			}
		case drawing.AlphaModFix:
			plan.Int1 = e.Amount
		case *drawing.AlphaModFix:
			if e != nil {
				plan.Int1 = e.Amount
			}
		case drawing.AlphaOff:
			plan.Int1 = e.Amount
		case *drawing.AlphaOff:
			if e != nil {
				plan.Int1 = e.Amount
			}
		case drawing.AlphaRepl:
			plan.Int1 = e.Alpha
		case *drawing.AlphaRepl:
			if e != nil {
				plan.Int1 = e.Alpha
			}
		case drawing.AlphaBiLevel:
			plan.Int1 = e.Threshold
		case *drawing.AlphaBiLevel:
			if e != nil {
				plan.Int1 = e.Threshold
			}
		case drawing.AlphaOutset:
			plan.Float1 = e.Radius
		case *drawing.AlphaOutset:
			if e != nil {
				plan.Float1 = e.Radius
			}
		case drawing.ClrRepl:
			plan.Color1 = e.Color
		case *drawing.ClrRepl:
			if e != nil {
				plan.Color1 = e.Color
			}
		case drawing.ClrChange:
			plan.Color1, plan.Color2, plan.Float1, plan.Float2, plan.Bool1 = e.From, e.To, e.FromAlpha, e.ToAlpha, e.UseAlpha
		case *drawing.ClrChange:
			if e != nil {
				plan.Color1, plan.Color2, plan.Float1, plan.Float2, plan.Bool1 = e.From, e.To, e.FromAlpha, e.ToAlpha, e.UseAlpha
			}
		}
		out = append(out, plan)
	}
	return out
}

type ImageSourceRectPlan struct {
	Left   float64
	Top    float64
	Right  float64
	Bottom float64
}

// ImageVisualPlan is the complete physical picture-composition contract. In
// addition to renderer-ready CSS percentages it preserves the authored source
// crop rectangle and the final destination frame so crop, clipping and
// transforms can be reasoned about without returning to canonical IR.
type ImageVisualPlan struct {
	// Renderer-ready image artifact. Browser-unsupported Office metafiles are
	// normalized at the shared physical boundary so HTML and other serializers do
	// not need parser-specific WMF/EMF handling.
	MIME string
	Data []byte
	Alt  string

	// Composition owns the authored picture frame, transform, shape effects and paint.
	// FrameBounds below remains the fillRect destination frame inside it.
	Composition  PhysicalCompositionPlan
	DPI          int
	RotWithShape bool
	FrameLeft    float64
	FrameTop     float64
	FrameRight   float64
	FrameBottom  float64
	FrameHidden  bool
	SourceRect   ImageSourceRectPlan

	ImageWidth  float64
	ImageHeight float64
	ImageLeft   float64
	ImageTop    float64
	ImageHidden bool

	CounterRotationDeg     float64
	CounterRotationOriginX float64
	CounterRotationOriginY float64
	ClipKind               string
	ClipPath               string
	FrameBounds            PhysicalPaintBoundsPlan
	Effects                []ImageEffectVisualPlan
	// FilterRegion is the renderer-ready SVG filter viewport for blip effects.
	// It is expressed in object-bounding-box percentages and expands only as
	// far as image effects require, so alphaOutset is not clipped by a fixed
	// serializer heuristic.
	FilterXPct      float64
	FilterYPct      float64
	FilterWidthPct  float64
	FilterHeightPct float64
}

func ResolvePhysicalTransformPlan(shape *ir.Shape) PhysicalTransformPlan {
	if shape == nil {
		return PhysicalTransformPlan{}
	}
	return PhysicalTransformPlan{RotationDeg: shape.RotationDeg, FlipH: shape.FlipH, FlipV: shape.FlipV}
}

// ResolveImageVisualPlan owns picture crop/fill-rect materialization. Renderers
// should not reinterpret ImageCrop*/ImageFill* independently.
func resolveImageVisualPlanWithComposition(shape *ir.Shape, composition PhysicalCompositionPlan) ImageVisualPlan {
	return resolveImageVisualPlanWithCompositionAndOptions(shape, composition, PhysicalMaterializeOptions{})
}

func resolveImageVisualPlanWithCompositionAndOptions(shape *ir.Shape, composition PhysicalCompositionPlan, opts PhysicalMaterializeOptions) ImageVisualPlan {
	plan := ResolveImageVisualPlan(shape)
	plan.Composition = composition
	if shape != nil {
		plan.Data, plan.MIME = physicalPresentationImageWithOptions(shape.ImageData, shape.ImageMIME, append([]metafile.FontFace(nil), opts.MetafileFonts...))
		plan.Alt = shape.Name
	}
	return plan
}

func ResolveImageVisualPlan(shape *ir.Shape) ImageVisualPlan {
	plan := ImageVisualPlan{ImageWidth: 100, ImageHeight: 100, RotWithShape: true, Composition: resolvePhysicalCompositionPlan(shape)}
	if shape == nil {
		return plan
	}
	plan.DPI = shape.ImageDPI
	if shape.ImageRotWithShapeSet {
		plan.RotWithShape = shape.ImageRotWithShape
	}
	geometry := ResolveGeometryVisualPlan(shape)
	plan.ClipKind = geometry.ClipKind
	switch geometry.ClipKind {
	case "ellipse":
		plan.ClipPath = "ellipse(50% 50% at 50% 50%)"
	case "roundRect", "roundrect":
		plan.ClipPath = "inset(0 round 8%)"
	}
	plan.FrameLeft = shape.ImageFillLeft * 100
	plan.FrameTop = shape.ImageFillTop * 100
	plan.FrameRight = shape.ImageFillRight * 100
	plan.FrameBottom = shape.ImageFillBottom * 100
	plan.FrameHidden = shape.ImageFillLeft+shape.ImageFillRight >= 1 || shape.ImageFillTop+shape.ImageFillBottom >= 1
	plan.SourceRect = ImageSourceRectPlan{
		Left: shape.ImageCropLeft, Top: shape.ImageCropTop,
		Right: shape.ImageCropRight, Bottom: shape.ImageCropBottom,
	}
	plan.FrameBounds = resolvePhysicalImageFrameBounds(shape, plan)
	plan.Effects = ResolveImageEffectVisualPlans(shape.ImageEffects)
	plan.FilterWidthPct, plan.FilterHeightPct = 100, 100

	visibleWidth := 1.0 - shape.ImageCropLeft - shape.ImageCropRight
	visibleHeight := 1.0 - shape.ImageCropTop - shape.ImageCropBottom
	if visibleWidth <= 0 || visibleHeight <= 0 {
		plan.ImageHidden = true
		return plan
	}
	plan.ImageWidth = 100.0 / visibleWidth
	plan.ImageHeight = 100.0 / visibleHeight
	plan.ImageLeft = -100.0 * shape.ImageCropLeft / visibleWidth
	plan.ImageTop = -100.0 * shape.ImageCropTop / visibleHeight

	// Materialize the blip-effect filter viewport from the final physical image
	// element, not from a fixed HTML/SVG margin. AlphaOutset is specified in
	// points, while the filter is attached to the cropped/expanded <img>.
	maxOutsetPt := 0.0
	for _, effect := range plan.Effects {
		if effect.Type == string(drawing.EffectTypeAlphaOutset) && effect.Float1 > maxOutsetPt {
			maxOutsetPt = effect.Float1
		}
	}
	if maxOutsetPt > 0 {
		frameW := 1 - shape.ImageFillLeft - shape.ImageFillRight
		frameH := 1 - shape.ImageFillTop - shape.ImageFillBottom
		imageWidthPt := float64(shape.Bounds.W) / 12700.0 * frameW * (plan.ImageWidth / 100)
		imageHeightPt := float64(shape.Bounds.H) / 12700.0 * frameH * (plan.ImageHeight / 100)
		if imageWidthPt > 0 {
			x := maxOutsetPt / imageWidthPt * 100
			plan.FilterXPct = -x
			plan.FilterWidthPct = 100 + 2*x
		}
		if imageHeightPt > 0 {
			y := maxOutsetPt / imageHeightPt * 100
			plan.FilterYPct = -y
			plan.FilterHeightPct = 100 + 2*y
		}
	}
	if shape.ImageRotWithShapeSet && !shape.ImageRotWithShape && shape.RotationDeg != 0 {
		// A single reflection reverses rotational handedness. Counter-rotation
		// therefore changes sign when the picture transform contains exactly one
		// flip; otherwise the ordinary inverse rotation is correct.
		plan.CounterRotationDeg = -shape.RotationDeg
		if shape.FlipH != shape.FlipV {
			plan.CounterRotationDeg = shape.RotationDeg
		}

		// The blip is cropped/expanded inside fillRect, so its own CSS center is
		// generally not the authored picture center. Materialize the picture-center
		// pivot in image-local percentages; serializers must not guess it.
		frameW := 1 - shape.ImageFillLeft - shape.ImageFillRight
		frameH := 1 - shape.ImageFillTop - shape.ImageFillBottom
		imageX := shape.ImageFillLeft + frameW*(plan.ImageLeft/100)
		imageY := shape.ImageFillTop + frameH*(plan.ImageTop/100)
		imageW := frameW * (plan.ImageWidth / 100)
		imageH := frameH * (plan.ImageHeight / 100)
		if imageW != 0 {
			plan.CounterRotationOriginX = (0.5 - imageX) / imageW * 100
		}
		if imageH != 0 {
			plan.CounterRotationOriginY = (0.5 - imageY) / imageH * 100
		}
	}
	return plan
}

type ShapeVisualPlan struct {
	WidthPt             float64
	HeightPt            float64
	UseVector           bool
	HasGeometry         bool
	HasVisibleFill      bool
	HasVisibleStroke    bool
	FillNeedsDefinition bool
	ClipContent         bool
	HasEffects          bool
	HasThreeD           bool
	Fill                FillVisualPlan
	Geometry            GeometryVisualPlan
	Stroke              StrokeVisualPlan
	Effects             EffectVisualPlan
	Primitives          []VectorPrimitivePlan
}

// ResolveShapeVisualPlan converts canonical shape visual state into physical
// geometry ready for a renderer. It deliberately knows nothing about PPT/PPTX
// package parts or PresentationML inheritance.
func ResolveShapeVisualPlan(shape *ir.Shape) ShapeVisualPlan {
	if shape == nil {
		return ShapeVisualPlan{}
	}
	width := drawing.EMUToPoints(shape.Bounds.W)
	height := drawing.EMUToPoints(shape.Bounds.H)
	preset := shape.Geometry
	if preset == "" {
		preset = "rect"
	}
	hasGeometry := shape.GeometryData != nil || shape.Geometry != ""
	fill := ResolveFillVisualPlan(shape.Fill, shape.FillColor, shape.PictureTile)
	if fill.Gradient != nil && fill.Gradient.Kind == drawing.GradientTypePath {
		fill.Gradient.PathBoundary = preset
	}
	fillNeedsDefinition := fill.NeedsDefinition
	geometry := ResolveGeometryVisualPlan(shape)
	stroke := ResolveStrokeVisualPlan(shape.LineStyle, shape.LineColor, shape.LineWidthPt)
	effects := ResolveEffectVisualPlan(shape.Effects)
	useVector := shape.GeometryData != nil || (shape.Geometry != "" && shape.Geometry != "rect") || fillNeedsDefinition || stroke.NeedsVector || shape.ArrowHead != "" || shape.ArrowTail != ""
	spec := drawing.ShapeGeometrySpec{
		Geometry: shape.GeometryData, Preset: &drawing.PresetGeometry{Name: preset, Adjustments: shape.GeometryAdjustments},
		Width: width, Height: height, Fill: shape.Fill, FillColor: shape.FillColor,
		Stroke: shape.LineStyle, StrokeColor: stroke.Color, StrokeWidth: stroke.WidthPt,
		ArrowHead: shape.ArrowHead, ArrowHeadWidth: shape.ArrowHeadWidth, ArrowHeadLength: shape.ArrowHeadLength,
		ArrowTail: shape.ArrowTail, ArrowTailWidth: shape.ArrowTailWidth, ArrowTailLength: shape.ArrowTailLength,
	}
	primitives, _ := drawing.BuildShapePrimitives(spec)
	if fill.Gradient != nil && fill.Gradient.Kind == drawing.GradientTypePath && !strings.EqualFold(preset, "rect") && !strings.EqualFold(preset, "ellipse") {
		for i := range primitives {
			if primitives[i].Kind == drawing.PrimitivePath && primitives[i].Path != nil {
				pathCopy := *primitives[i].Path
				pathCopy.Commands = append([]drawing.PathCommand(nil), primitives[i].Path.Commands...)
				fill.Gradient.PathBoundaryPath = &pathCopy
				fill.Gradient.PathBoundaryWidthPt = width
				fill.Gradient.PathBoundaryHeightPt = height
				break
			}
		}
	}
	return ShapeVisualPlan{
		WidthPt:             width,
		HeightPt:            height,
		UseVector:           useVector,
		HasGeometry:         hasGeometry,
		HasVisibleFill:      fill.Visible,
		HasVisibleStroke:    stroke.Color != "" && stroke.WidthPt > 0,
		FillNeedsDefinition: fillNeedsDefinition,
		ClipContent:         shape.Geometry == "ellipse",
		HasEffects:          effects.HasEffects,
		HasThreeD:           effects.HasThreeD,
		Fill:                fill,
		Geometry:            geometry,
		Stroke:              stroke,
		Effects:             effects,
		Primitives:          resolveVectorPrimitivePlans(primitives),
	}
}

// ResolveFillVisualPlan classifies canonical fill semantics once for all renderers.
// It keeps the canonical fill itself so HTML/SVG materializers do not need to
// rediscover fill kind, definition requirements, transparency, or tile state.
func ResolveFillVisualPlan(fill drawing.Fill, fallbackColor string, tile *ir.PictureTile) FillVisualPlan {
	plan := FillVisualPlan{Fill: fill, FallbackColor: fallbackColor, PictureTile: tile}
	if fill == nil {
		return plan
	}
	plan.Kind = fill.Type()
	switch fill.(type) {
	case drawing.NoFill, *drawing.NoFill:
		plan.Visible = false
	default:
		plan.Visible = true
	}
	plan.NeedsDefinition = shapeFillNeedsVectorMaterialization(fill)
	switch f := fill.(type) {
	case drawing.SolidFill:
		plan.HasTransparency = f.Alpha > 0 && f.Alpha < 1
	case *drawing.SolidFill:
		plan.HasTransparency = f != nil && f.Alpha > 0 && f.Alpha < 1
	case drawing.PictureFill:
		plan.HasTransparency = f.Alpha > 0 && f.Alpha < 1
	case *drawing.PictureFill:
		plan.HasTransparency = f != nil && f.Alpha > 0 && f.Alpha < 1
	case drawing.GradientFill:
		plan.Gradient = resolveGradientVisualPlan(f)
		for _, stop := range f.Stops {
			if stop.Alpha > 0 && stop.Alpha < 1 {
				plan.HasTransparency = true
				break
			}
		}
	case *drawing.GradientFill:
		if f != nil {
			plan.Gradient = resolveGradientVisualPlan(*f)
			for _, stop := range f.Stops {
				if stop.Alpha > 0 && stop.Alpha < 1 {
					plan.HasTransparency = true
					break
				}
			}
		}
	case drawing.PatternFill:
		plan.HasTransparency = (f.ForeAlpha > 0 && f.ForeAlpha < 1) || (f.BackAlpha > 0 && f.BackAlpha < 1)
	case *drawing.PatternFill:
		plan.HasTransparency = f != nil && ((f.ForeAlpha > 0 && f.ForeAlpha < 1) || (f.BackAlpha > 0 && f.BackAlpha < 1))
	}
	return plan
}

func resolveGradientVisualPlan(fill drawing.GradientFill) *GradientVisualPlan {
	plan := &GradientVisualPlan{Kind: fill.GradientType, AngleDeg: fill.Angle, RotateWithShape: fill.RotateWithShape, ScaleAngleWithAspect: fill.ScaleAngleWithAspect, LegacyShadeType: fill.LegacyShadeType}
	if fill.FillToRect != nil {
		// fillTo is a normalized rectangle. Malformed legacy values must not
		// escape the shape box or produce a negative inner extent. Canonicalize
		// that geometry once here so path/radial renderers share the same bounds.
		left := math.Max(0, math.Min(1, fill.FillToRect.X))
		top := math.Max(0, math.Min(1, fill.FillToRect.Y))
		right := math.Max(left, math.Min(1, fill.FillToRect.X+math.Max(0, fill.FillToRect.Width)))
		bottom := math.Max(top, math.Min(1, fill.FillToRect.Y+math.Max(0, fill.FillToRect.Height)))
		plan.CenterX = (left + right) / 2
		plan.CenterY = (top + bottom) / 2
		plan.CenterWidth = right - left
		plan.CenterHeight = bottom - top
		plan.HasCenter = true
	}
	switch strings.ToLower(strings.TrimSpace(fill.Flip)) {
	case "x":
		plan.FlipH = true
	case "y":
		plan.FlipV = true
	case "xy", "yx":
		plan.FlipH, plan.FlipV = true, true
	}
	plan.Stops = make([]GradientStopVisualPlan, 0, len(fill.Stops))
	// Alpha zero is a real gradient stop value (fully transparent), not an
	// implicit default. Older hand-built IR sometimes omitted alpha on every
	// stop, so retain that compatibility only when the entire gradient has zero
	// alpha. Parsed PPT/PPTX gradients materialize opaque stops as Alpha=1.
	hasAuthoredAlpha := false
	for _, stop := range fill.Stops {
		if stop.Alpha != 0 {
			hasAuthoredAlpha = true
			break
		}
	}
	for _, stop := range fill.Stops {
		pos := math.Max(0, math.Min(1, stop.Position))
		alpha := stop.Alpha
		if !hasAuthoredAlpha {
			alpha = 1
		} else {
			alpha = math.Max(0, math.Min(1, alpha))
		}
		plan.Stops = append(plan.Stops, GradientStopVisualPlan{Position: pos, Color: stop.Color, Alpha: alpha})
	}
	// SVG requires monotonically increasing offsets. Canonicalize once in
	// layout so every renderer sees the same authored gradient semantics.
	sort.SliceStable(plan.Stops, func(i, j int) bool {
		return plan.Stops[i].Position < plan.Stops[j].Position
	})
	return plan
}

// ResolveGeometryVisualPlan owns geometry-specific content constraints. The
// renderer materializes these physical ratios but does not reinterpret the
// authored preset geometry.
func ResolveGeometryVisualPlan(shape *ir.Shape) GeometryVisualPlan {
	if shape == nil {
		return GeometryVisualPlan{}
	}
	preset := shape.Geometry
	if preset == "" {
		preset = "rect"
	}
	plan := GeometryVisualPlan{
		Preset:      preset,
		Custom:      shape.GeometryData != nil,
		Adjustments: append([]float64(nil), shape.GeometryAdjustments...),
	}
	switch shape.Geometry {
	case "ellipse":
		plan.ClipKind = "ellipse"
		plan.ContentInsetTop = 0.16
		plan.ContentInsetRight = 0.18
		plan.ContentInsetBottom = 0.16
		plan.ContentInsetLeft = 0.18
	}
	return plan
}

// shapeFillNeedsVectorMaterialization classifies canonical drawing fills that
// cannot be faithfully represented by the renderer's ordinary CSS fill path.
// Keeping this predicate here avoids making presentation layout depend on the
// renderer-oriented drawingcss package.
func shapeFillNeedsVectorMaterialization(fill drawing.Fill) bool {
	switch fill.(type) {
	case drawing.GradientFill, *drawing.GradientFill,
		drawing.PatternFill, *drawing.PatternFill,
		drawing.PictureFill, *drawing.PictureFill:
		return true
	default:
		return false
	}
}

func ResolveStrokeVisualPlan(style *drawing.LineStyle, fallbackColor string, fallbackWidth float64) StrokeVisualPlan {
	color := fallbackColor
	width := fallbackWidth
	plan := StrokeVisualPlan{CSSStyle: "solid"}
	if style != nil {
		if style.Color != "" {
			color = style.Color
		}
		if style.Width > 0 {
			width = style.Width
		}
		plan.Alpha = style.Alpha
		plan.AlphaSet = true
		plan.Fill = style.Fill
		plan.Compound = style.Compound || (style.CompoundType != "" && style.CompoundType != "sng")
		plan.CompoundType = style.CompoundType
		plan.CompoundWidths = resolveCompoundStrokeWidths(style)
		plan.HasArrowheads = style.HeadEnd != nil || style.TailEnd != nil
		plan.MiterLimit = style.MiterLimit
		switch style.Cap {
		case drawing.LineCapRound:
			plan.SVGLineCap = "round"
		case drawing.LineCapSquare:
			plan.SVGLineCap = "square"
		case drawing.LineCapFlat:
			plan.SVGLineCap = "butt"
		}
		switch style.Join {
		case drawing.LineJoinRound:
			plan.SVGLineJoin = "round"
		case drawing.LineJoinBevel:
			plan.SVGLineJoin = "bevel"
		case drawing.LineJoinMiter:
			plan.SVGLineJoin = "miter"
		}
		plan.DashArray = strokeDashArray(style.Dash, width)
		if style.Dash != drawing.DashPatternSolid && style.Dash != "" {
			plan.CSSStyle = "dashed"
		}
		if plan.Compound {
			plan.CSSStyle = "double"
		}
		preciseDash := style.Dash != "" && style.Dash != drawing.DashPatternSolid
		preciseCapJoin := style.Cap != "" && style.Cap != drawing.LineCapFlat || style.Join != "" && style.Join != drawing.LineJoinMiter
		partialAlpha := style.Alpha >= 0 && style.Alpha < 1
		plan.NeedsVector = style.Fill != nil || plan.HasArrowheads || preciseDash || preciseCapJoin || partialAlpha || (plan.Compound && style.CompoundType != "dbl")
	}
	plan.Color, plan.WidthPt = color, width
	return plan
}

func resolveCompoundStrokeWidths(style *drawing.LineStyle) []float64 {
	if style == nil {
		return nil
	}
	if len(style.CompoundWidths) != 0 {
		return append([]float64(nil), style.CompoundWidths...)
	}
	switch style.CompoundType {
	case "dbl":
		return []float64{1.0 / 3.0, 1.0 / 3.0}
	case "thickThin":
		return []float64{2.0 / 3.0, 1.0 / 3.0}
	case "thinThick":
		return []float64{1.0 / 3.0, 2.0 / 3.0}
	case "tri":
		return []float64{1.0 / 3.0, 1.0 / 3.0, 1.0 / 3.0}
	default:
		return nil
	}
}

func strokeDashArray(dash drawing.DashPattern, widthPt float64) string {
	if widthPt <= 0 {
		widthPt = 1
	}
	var units []float64
	switch dash {
	case drawing.DashPatternDash, drawing.DashPatternSystemDash:
		units = []float64{3, 3}
	case drawing.DashPatternDot, drawing.DashPatternSystemDot:
		units = []float64{1, 3}
	case drawing.DashPatternDashDot, drawing.DashPatternSystemDashDot:
		units = []float64{3, 3, 1, 3}
	case drawing.DashPatternLongDash:
		units = []float64{5, 3}
	case drawing.DashPatternLongDashDot:
		units = []float64{5, 3, 1, 3}
	case drawing.DashPatternLongDashDotDot:
		units = []float64{5, 3, 1, 3, 1, 3}
	case drawing.DashPatternSystemDashDotDot:
		units = []float64{3, 3, 1, 3, 1, 3}
	default:
		return ""
	}
	parts := make([]string, len(units))
	for i, unit := range units {
		parts[i] = formatStrokeMetric(unit * widthPt)
	}
	return strings.Join(parts, ", ")
}

func formatStrokeMetric(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func ResolveEffectVisualPlan(effects drawing.EffectList) EffectVisualPlan {
	plan := EffectVisualPlan{HasEffects: len(effects.Effects) > 0}
	plan.Composition = ResolveEffectCompositionPlan(effects)
	for _, effect := range effects.Effects {
		switch e := effect.(type) {
		case drawing.Shadow:
			plan.HasShadow = true
			plan.NeedsOverflow = true
			plan.Shadows = append(plan.Shadows, resolveShadowVisualPlan(e, false))
		case *drawing.Shadow:
			plan.HasShadow = true
			plan.NeedsOverflow = true
			if e != nil {
				plan.Shadows = append(plan.Shadows, resolveShadowVisualPlan(*e, false))
			}
		case drawing.InnerShadow:
			plan.HasInnerShadow = true
			plan.Shadows = append(plan.Shadows, resolveInnerShadowVisualPlan(e))
		case *drawing.InnerShadow:
			plan.HasInnerShadow = true
			if e != nil {
				plan.Shadows = append(plan.Shadows, resolveInnerShadowVisualPlan(*e))
			}
		case drawing.Glow:
			plan.HasGlow = true
			plan.NeedsOverflow = true
			plan.Glows = append(plan.Glows, GlowVisualPlan{Color: e.Color, Alpha: e.Alpha, RadiusPt: e.Radius})
		case *drawing.Glow:
			plan.HasGlow = true
			plan.NeedsOverflow = true
			if e != nil {
				plan.Glows = append(plan.Glows, GlowVisualPlan{Color: e.Color, Alpha: e.Alpha, RadiusPt: e.Radius})
			}
		case drawing.SoftEdge:
			plan.HasSoftEdge = true
			plan.SoftEdges = append(plan.SoftEdges, SoftEdgeVisualPlan{RadiusPt: e.Radius})
		case *drawing.SoftEdge:
			plan.HasSoftEdge = true
			if e != nil {
				plan.SoftEdges = append(plan.SoftEdges, SoftEdgeVisualPlan{RadiusPt: e.Radius})
			}
		case drawing.Reflection, *drawing.Reflection:
			plan.HasReflection = true
			plan.NeedsOverflow = true
		case drawing.Blur:
			plan.HasBlur = true
			plan.NeedsOverflow = true
			plan.Blurs = append(plan.Blurs, BlurVisualPlan{RadiusPt: e.Radius, Grow: e.Grow})
		case *drawing.Blur:
			plan.HasBlur = true
			plan.NeedsOverflow = true
			if e != nil {
				plan.Blurs = append(plan.Blurs, BlurVisualPlan{RadiusPt: e.Radius, Grow: e.Grow})
			}
		case drawing.ThreeD, *drawing.ThreeD:
			plan.HasThreeD = true
			plan.NeedsOverflow = true
		case drawing.FillOverlay:
			plan.Overlays = append(plan.Overlays, EffectOverlayPlan{Fill: e.Fill, BlendMode: e.BlendMode, NeedsDefinition: shapeFillNeedsVectorMaterialization(e.Fill)})
		case *drawing.FillOverlay:
			if e != nil {
				plan.Overlays = append(plan.Overlays, EffectOverlayPlan{Fill: e.Fill, BlendMode: e.BlendMode, NeedsDefinition: shapeFillNeedsVectorMaterialization(e.Fill)})
			}
		}
	}
	return plan
}

func resolveShadowVisualPlan(s drawing.Shadow, inner bool) ShadowVisualPlan {
	rad := s.Angle * math.Pi / 180
	return ShadowVisualPlan{Inner: inner, Color: s.Color, Alpha: s.Alpha, BlurPt: s.Blur, OffsetXPt: s.Distance * math.Cos(rad), OffsetYPt: -s.Distance * math.Sin(rad), RotateWithShape: s.RotateWithShape}
}
func resolveInnerShadowVisualPlan(s drawing.InnerShadow) ShadowVisualPlan {
	rad := s.Angle * math.Pi / 180
	return ShadowVisualPlan{Inner: true, Color: s.Color, Alpha: s.Alpha, BlurPt: s.Blur, OffsetXPt: s.Distance * math.Cos(rad), OffsetYPt: -s.Distance * math.Sin(rad)}
}

// ResolveEffectCompositionPlan preserves the authored effect-composition order
// while projecting renderer-independent transform, blend, reflection and 3-D
// semantics into layout-owned state.
func ResolveEffectCompositionPlan(effects drawing.EffectList) EffectCompositionPlan {
	var plan EffectCompositionPlan
	for _, effect := range effects.Effects {
		switch e := effect.(type) {
		case drawing.RelativeOffset:
			plan.Transforms = append(plan.Transforms, EffectTransformStep{Kind: "offset", X: e.X, Y: e.Y})
		case *drawing.RelativeOffset:
			if e != nil {
				plan.Transforms = append(plan.Transforms, EffectTransformStep{Kind: "offset", X: e.X, Y: e.Y})
			}
		case drawing.TransformEffect:
			plan.Transforms = append(plan.Transforms, effectTransformStep(e))
		case *drawing.TransformEffect:
			if e != nil {
				plan.Transforms = append(plan.Transforms, effectTransformStep(*e))
			}
		case drawing.BlendMode:
			plan.BlendMode = e.Mode
		case *drawing.BlendMode:
			if e != nil {
				plan.BlendMode = e.Mode
			}
		case drawing.Reflection:
			v := reflectionVisualPlan(e)
			plan.Reflection = &v
		case *drawing.Reflection:
			if e != nil {
				v := reflectionVisualPlan(*e)
				plan.Reflection = &v
			}
		case drawing.ThreeD:
			v := threeDVisualPlan(e)
			plan.ThreeD = &v
		case *drawing.ThreeD:
			if e != nil {
				v := threeDVisualPlan(*e)
				plan.ThreeD = &v
			}
		}
	}
	return plan
}

func effectTransformStep(e drawing.TransformEffect) EffectTransformStep {
	sx, sy := e.ScaleX, e.ScaleY
	if sx == 0 {
		sx = 1
	}
	if sy == 0 {
		sy = 1
	}
	return EffectTransformStep{Kind: "transform", X: e.TranslateX, Y: e.TranslateY, ScaleX: sx, ScaleY: sy, SkewXDeg: e.SkewX, SkewYDeg: e.SkewY}
}

func reflectionVisualPlan(r drawing.Reflection) ReflectionVisualPlan {
	return ReflectionVisualPlan{
		DirectionDeg: r.Direction, DistancePt: r.Distance, BlurPt: r.Blur, Size: r.Size,
		ScaleX: r.ScaleX, ScaleY: r.ScaleY, SkewXDeg: r.SkewX, SkewYDeg: r.SkewY,
		RotateWithShape: r.RotateWithShape, Alpha: r.Alpha,
		StartAlpha: float64(r.StartAlpha) / 100000.0, EndAlpha: float64(r.EndAlpha) / 100000.0,
		StartPosition: float64(r.StartPosition) / 1000.0, EndPosition: float64(r.EndPosition) / 1000.0,
	}
}

func threeDVisualPlan(t drawing.ThreeD) ThreeDVisualPlan {
	plan := ThreeDVisualPlan{
		CameraFOV: t.CameraFOV, CameraZoom: t.CameraZoom,
		RotationXDeg: t.RotationX, RotationYDeg: t.RotationY, RotationZDeg: t.RotationZ,
		LightRotationXDeg: t.LightRotationX, LightRotationYDeg: t.LightRotationY, LightRotationZDeg: t.LightRotationZ,
		Material: t.Material, CameraPreset: t.CameraPreset, LightRig: t.LightRig, LightDirection: t.LightDirection,
		ExtrusionDepthPt: t.ExtrusionDepth, ContourWidthPt: t.ContourWidth,
		BevelTopWidthPt: t.BevelTopWidth, BevelTopHeightPt: t.BevelTopHeight, BevelTopPreset: t.BevelTopPreset,
		BevelBottomWidthPt: t.BevelBottomWidth, BevelBottomHeightPt: t.BevelBottomHeight, BevelBottomPreset: t.BevelBottomPreset,
		ContourColor: t.ContourColor, ExtrusionColor: t.ExtrusionColor,
	}
	plan.LightingOverlays = threeDLightingOverlayPlans(plan)
	return plan
}

func threeDLightingOverlayPlans(t ThreeDVisualPlan) []EffectOverlayPlan {
	if t.ExtrusionDepthPt <= 0 && t.ContourWidthPt <= 0 && t.BevelTopHeightPt <= 0 && t.BevelBottomHeightPt <= 0 && t.Material == "" {
		return nil
	}
	angle := threeDLightAngle(t.LightDirection)
	highlight, shade := threeDLightOpacities(t)
	highlightColor := strings.TrimSpace(t.ContourColor)
	if highlightColor == "" {
		highlightColor = "FFFFFF"
	}
	shadeColor := strings.TrimSpace(t.ExtrusionColor)
	if shadeColor == "" {
		shadeColor = "000000"
	}
	return []EffectOverlayPlan{
		{BlendMode: "screen", NeedsDefinition: true, Fill: drawing.GradientFill{GradientType: drawing.GradientTypeLinear, Angle: angle, Stops: []drawing.GradientStop{{Position: 0, Color: highlightColor, Alpha: highlight}, {Position: 0.35, Color: highlightColor, Alpha: highlight * 0.35}, {Position: 1, Color: highlightColor, Alpha: 0}}}},
		{BlendMode: "mult", NeedsDefinition: true, Fill: drawing.GradientFill{GradientType: drawing.GradientTypeLinear, Angle: math.Mod(angle+180, 360), Stops: []drawing.GradientStop{{Position: 0, Color: shadeColor, Alpha: shade}, {Position: 0.45, Color: shadeColor, Alpha: shade * 0.4}, {Position: 1, Color: shadeColor, Alpha: 0}}}},
	}
}

func threeDLightAngle(direction string) float64 {
	switch strings.ToLower(strings.TrimSpace(direction)) {
	case "t":
		return 90
	case "tr":
		return 135
	case "r":
		return 180
	case "br":
		return 225
	case "b":
		return 270
	case "bl":
		return 315
	case "l":
		return 0
	case "tl":
		return 45
	default:
		return 45
	}
}

func threeDLightOpacities(t ThreeDVisualPlan) (float64, float64) {
	highlight, shade := 0.18, 0.14
	switch strings.ToLower(strings.TrimSpace(t.Material)) {
	case "metal", "metal2", "legacymetal":
		highlight, shade = 0.30, 0.18
	case "matte", "plastic":
		highlight, shade = 0.20, 0.12
	}
	switch strings.ToLower(strings.TrimSpace(t.LightRig)) {
	case "threept":
		highlight, shade = highlight+0.06, shade+0.03
	case "balanced", "soft":
		highlight += 0.03
	}
	if t.ContourWidthPt > 0 || t.BevelTopHeightPt > 0 || t.BevelBottomHeightPt > 0 {
		highlight += 0.02
	}
	if t.ExtrusionDepthPt > 0 {
		shade += 0.02
	}
	if highlight > 0.45 {
		highlight = 0.45
	}
	if shade > 0.30 {
		shade = 0.30
	}
	return highlight, shade
}
