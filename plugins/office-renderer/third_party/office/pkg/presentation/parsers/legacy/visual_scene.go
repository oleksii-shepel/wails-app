package legacy

import (
	"fmt"
	"math"
	"sort"

	officeartir "office/pkg/officeart/ir"
	officeartlegacy "office/pkg/officeart/legacy"
	"office/pkg/ooxml/drawing"
	presentationir "office/pkg/presentation/ir"
)

// legacyPptVisualScene is the semantic boundary for one PPT OfficeArt drawing.
// It owns the single collected OfficeArt event sequence and all shape-family
// projections derived from it. Production callers must not independently walk
// the binary drawing for text, tables, pictures, connectors, or groups.
type legacyPptVisualScene struct {
	dispatch legacyPptSlideVisualDispatch
}

func collectLegacyPptVisualScene(payload []byte) legacyPptVisualScene {
	dispatch := collectLegacyPptSlideVisualDispatch(payload)
	return legacyPptVisualScene{dispatch: dispatch}
}

func (s legacyPptVisualScene) empty() bool {
	return len(s.dispatch.containers.officeArtEvents()) == 0 && len(s.dispatch.rules) == 0
}

func (s legacyPptVisualScene) textMetas(fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string) []legacyPptTextShape {
	// BIG38 keeps text as an event projection because damaged/encrypted legacy
	// presentations can retain usable ClientTextbox semantics even when the
	// structural ShapeNode scene is incomplete. This is still a single-pass
	// semantic consumer: it reads the scene-owned event sequence and never
	// reopens the persisted stream.
	var metas []legacyPptTextShape
	collectLegacyPptTextShapesFromEvents(s.dispatch.containers.officeArtEvents(), legacyPptGroupFrame{}, fontTable, colorScheme, exHyperlinks, &metas)
	return metas
}

func (s legacyPptVisualScene) filterTextMetasOwnedByGroupPaths(metas []legacyPptTextShape, ownedPaths [][]int) []legacyPptTextShape {
	if len(metas) == 0 || len(ownedPaths) == 0 {
		return metas
	}
	ownedIDs := make(map[uint32]struct{})
	for _, container := range s.dispatch.containers.shapeContainers() {
		if container.ShapeID == 0 || container.IsGroupRoot {
			continue
		}
		for _, path := range ownedPaths {
			if legacyPptGroupPathHasPrefix(container.GroupPath, path) {
				ownedIDs[container.ShapeID] = struct{}{}
				break
			}
		}
	}
	if len(ownedIDs) == 0 {
		return metas
	}
	out := metas[:0]
	for _, meta := range metas {
		if _, owned := ownedIDs[meta.shapeID]; owned {
			continue
		}
		out = append(out, meta)
	}
	return out
}

// legacyPptVisualProjection carries the complete local semantic result and its
// ownership ledger. Reconciliation and completeness accounting share one boundary.
type legacyPptVisualProjection struct {
	Shapes    []*presentationir.Shape
	Ownership []legacyPptContainerOwnership
	// TextRoles preserves the authored TextHeaderAtom role only until the legacy
	// master-text cascade has resolved effective paragraph/run semantics. It is
	// parser-local metadata and never leaks into canonical presentation IR.
	TextRoles map[uint32]int
}

func (p legacyPptVisualProjection) unclaimed() []legacyPptContainerOwnership {
	var out []legacyPptContainerOwnership
	for _, entry := range p.Ownership {
		if !entry.Claimed {
			out = append(out, entry)
		}
	}
	return out
}

// completeProjection is the parser-completeness boundary for one OfficeArt
// scene. Projection and ownership accounting are deliberately inseparable: any
// SpContainer that fails to reach canonical IR remains observable as unclaimed.
func (s legacyPptVisualScene) completeProjection(
	slideText legacySlideText,
	fontTable map[uint16]string,
	colorScheme map[uint8]string,
	placeholderTemplates map[presentationir.PlaceholderKind]legacyPptTextShape,
	exHyperlinks map[uint32]string,
	pictureStore map[uint32]*legacyPptPicture,
	externalObjects map[uint32]legacyPptExternalObject,
	canvas legacyPptCanvas,
) legacyPptVisualProjection {
	// Table ownership is outcome-based. Project tables first so text can be
	// suppressed only for group paths that actually materialized as canonical
	// ShapeTable. The previous text-first path used a permissive table recovery
	// guess inside the event walker and could erase every label in an ordinary
	// diagram when the stricter table projector later rejected that group.
	var tables []*presentationir.Shape
	var tableOwnedPaths [][]int
	collectLegacyPptTableShapesFromSceneForSlideWithOwnership(s.dispatch.containers, legacyPptGroupFrame{}, slideText, fontTable, colorScheme, exHyperlinks, &tables, &tableOwnedPaths)

	metas := s.textMetas(fontTable, colorScheme, exHyperlinks)
	metas = s.filterTextMetasOwnedByGroupPaths(metas, tableOwnedPaths)
	metas = inheritLegacyPptMasterPlaceholderTemplates(metas, placeholderTemplates)
	textRoles := make(map[uint32]int)
	for _, meta := range metas {
		if meta.shapeID != 0 && meta.textType != pptTextTypeUnknown {
			textRoles[meta.shapeID] = meta.textType
		}
	}
	if len(textRoles) == 0 {
		textRoles = nil
	}
	shapes := buildLegacyPptTextShapesFromMetas(metas, slideText, canvas)

	tablesByScene := map[int][]*presentationir.Shape{1: tables}
	textByScene := map[int][]*presentationir.Shape{1: shapes}
	tablesByScene = filterLegacyDuplicateProjectedTables(textByScene, tablesByScene)
	shapes = append(shapes, tablesByScene[1]...)

	// Picture shape ownership comes from the OfficeArt scene, not from successful
	// BStore decoding. Always project picture nodes so a missing/damaged Pictures
	// stream cannot silently turn authored pictures into generic AutoShapes.
	var pictures []*presentationir.Shape
	collectLegacyPptImageShapesFromEvents(s.dispatch.containers.officeArtEvents(), legacyPptGroupFrame{}, colorScheme, pictureStore, externalObjects, &pictures)
	shapes = append(shapes, pictures...)

	// Semantic-family ownership is outcome based, but family precedence is
	// authoritative. Build a provisional generic projection only as an endpoint
	// index for connector-rule resolution; do not commit it yet. This lets a
	// connector bind to an ordinary AutoShape that has no richer family owner
	// without allowing that provisional generic connector to steal the connector
	// SpContainer itself.
	provisionalGeneric := s.genericShapes(shapes, colorScheme)
	endpointCandidates := append(append([]*presentationir.Shape(nil), shapes...), provisionalGeneric...)
	shapeIndex := legacyPptShapeIndexByOriginalID(endpointCandidates)
	var connectors []*presentationir.Shape
	collectLegacyPptConnectorShapesFromEvents(s.dispatch.containers.officeArtEvents(), legacyPptGroupFrame{}, colorScheme, s.dispatch.rules, shapeIndex, &connectors)
	shapes = append(shapes, connectors...)

	// Commit generic OfficeArt only after every richer leaf family has had an
	// opportunity to claim its ShapeID. genericShapes is the completeness
	// fallback: text/table/picture/media/OLE/connector ownership wins when it
	// materializes; otherwise a valid authored OfficeArt container survives as a
	// canonical visual instead of disappearing because a specialized decoder
	// declined it.
	shapes = append(shapes, s.genericShapes(shapes, colorScheme)...)

	groupSource := legacyPptDiscoveredShapeSource(legacyPptShapeIndexByOriginalID(shapes))
	groups := buildLegacyPptGroupShapesFromSceneWithOwnership(s.dispatch.containers, groupSource, fontTable, colorScheme, exHyperlinks, &tableOwnedPaths)
	if len(groups) != 0 {
		groupMap := map[int][]*presentationir.Shape{1: groups}
		flatMap := map[int][]*presentationir.Shape{1: shapes}
		flatMap = filterLegacyDuplicateProjectedTables(groupMap, flatMap)
		flatMap = filterLegacyGroupedShapeDuplicates(flatMap, groupMap)
		shapes = append(flatMap[1], groups...)
	}
	shapes = s.sourceOrdered(shapes)
	resolveLegacyPptShapePictureFills(shapes, pictureStore)
	return legacyPptVisualProjection{Shapes: shapes, Ownership: s.ownershipAuditWithTablePaths(shapes, tableOwnedPaths), TextRoles: textRoles}
}

// resolveLegacyPptShapePictureFills materializes authored texture/picture fills
// against the presentation Pictures store. The shared OfficeArt decoder owns the
// fill semantics but cannot reach the store, so it carries the BLIP index and
// this boundary turns it into image bytes. WordArt text is the visible case:
// 41246-2 slide 16's "Fin" is painted with the deck's crumpled-paper texture.
func resolveLegacyPptShapePictureFills(shapes []*presentationir.Shape, pictureStore map[uint32]*legacyPptPicture) {
	if len(pictureStore) == 0 {
		return
	}
	var walk func(*presentationir.Shape)
	walk = func(shape *presentationir.Shape) {
		if shape == nil {
			return
		}
		resolveLegacyPptShapePictureFill(shape, pictureStore)
		for _, child := range shape.Children {
			walk(child)
		}
	}
	for _, shape := range shapes {
		walk(shape)
	}
}

func resolveLegacyPptShapePictureFill(shape *presentationir.Shape, pictureStore map[uint32]*legacyPptPicture) {
	fill, ok := shape.Fill.(drawing.PictureFill)
	if !ok || len(fill.ImageData) > 0 || fill.ImageURL != "" {
		return
	}
	if shape.OfficeArt == nil || shape.OfficeArt.Fill.Picture == nil {
		return
	}
	picture := pictureStore[shape.OfficeArt.Fill.Picture.BlipRef]
	if picture == nil || len(picture.Data) == 0 {
		return
	}
	fill.ImageData = append([]byte(nil), picture.Data...)
	fill.ContentType = picture.MIME
	shape.Fill = fill
}

// sourceOrdered restores authored OfficeArt z/source order after the semantic
// families have been projected independently. Family ownership must not become
// paint order: text, tables, pictures, generic shapes, connectors and groups all
// originate from one ordered SpContainer stream. Unknown/synthesized shapes keep
// their relative order after authored shapes.
func (s legacyPptVisualScene) sourceOrdered(shapes []*presentationir.Shape) []*presentationir.Shape {
	if len(shapes) < 2 {
		return shapes
	}
	order := make(map[uint32]int)
	for _, container := range s.dispatch.containers.shapeContainers() {
		if container.ShapeID == 0 {
			continue
		}
		if _, exists := order[container.ShapeID]; !exists {
			order[container.ShapeID] = len(order)
		}
	}
	if len(order) == 0 {
		return shapes
	}
	type orderedShape struct {
		shape    *presentationir.Shape
		position int
		authored bool
		stable   int
	}
	items := make([]orderedShape, 0, len(shapes))
	for i, shape := range shapes {
		item := orderedShape{shape: shape, stable: i}
		if shape != nil && shape.OriginalShapeID != 0 {
			item.position, item.authored = order[shape.OriginalShapeID]
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].authored != items[j].authored {
			return items[i].authored
		}
		if !items[i].authored {
			return items[i].stable < items[j].stable
		}
		return items[i].position < items[j].position
	})
	out := make([]*presentationir.Shape, 0, len(items))
	for _, item := range items {
		out = append(out, item.shape)
	}
	return out
}

// genericShapes projects authored OfficeArt shapes that are not owned by a richer
// PPT semantic family. It deliberately consumes ShapeNode from the already-decoded
// container scene: no payload recursion and no second Escher scan is permitted.
func (s legacyPptVisualScene) genericShapes(owned []*presentationir.Shape, colorScheme map[uint8]string) []*presentationir.Shape {
	ownedIDs := legacyPptShapeIndexByOriginalID(owned)
	if ownedIDs == nil {
		ownedIDs = make(map[uint32]*presentationir.Shape)
	}
	projectedTables := make(map[uint32]struct{})
	for _, shape := range owned {
		if shape != nil && shape.Kind == presentationir.ShapeTable && shape.OriginalShapeID != 0 {
			projectedTables[shape.OriginalShapeID] = struct{}{}
		}
	}
	var out []*presentationir.Shape
	for _, container := range s.dispatch.containers.shapeContainers() {
		if container.Node == nil || container.IsGroupRoot || container.ShapeID == 0 {
			continue
		}
		if _, exists := ownedIDs[container.ShapeID]; exists {
			continue
		}
		// Grid/border lines authored inside an explicit or already-materialized
		// table are structural topology, not generic fallback shapes. This guard is
		// essential now that placement preserves zero-axis anchors: otherwise table
		// borders would reappear as duplicate standalone connectors.
		if s.tableTopologyOwnsLine(container, projectedTables, nil) {
			continue
		}
		node := container.Node
		// A richer family projection wins when it actually materializes this ID.
		// Do not reserve picture-frame SpContainers merely because of ShapeType:
		// damaged/unusual decks can retain a perfectly valid OfficeArt frame while
		// its BLIP/ExObj relationship is absent. In that case the complete OfficeArt
		// visual is still authored content and must reach canonical IR rather than
		// becoming an unclaimed container. The owned-ID check above prevents a
		// duplicate whenever picture/media/OLE projection did succeed.
		// ClientTextbox is only a possible text payload. Empty textbox-bearing
		// decorative shapes still own their authored OfficeArt visual.
		if node.HasClientTextbox {
			if _, textOwned := ownedIDs[container.ShapeID]; textOwned {
				continue
			}
		}
		placement := resolveLegacyPptShapePlacement(node, s.frameForContainer(container))
		if !placement.valid() {
			continue
		}
		bounds := placement.Bounds

		shared := officeartlegacy.ShapeFromEscherWithColorResolver(node, func(raw uint32) string {
			return resolveLegacyPptOfficeArtColorRef(raw, colorScheme)
		})
		// A ClientTextbox with no recovered text and no authored OfficeArt paint is a
		// structural text-run fragment, not a drawable shape. Binary PPT frequently
		// persists these narrow spacer boxes inside dense grouped diagrams; OOXML
		// drops them because they have neither semantic text nor visual paint. Keep
		// the completeness fallback for genuinely decorative empty textboxes, but do
		// not promote an invisible container into canonical presentation IR.
		if container.GroupDepth > 1 && node.HasClientTextbox && legacyPptOfficeArtShapeIsEmptyTextbox(shared) {
			continue
		}
		var transform legacyPptShapeTransform
		transform.FlipH = node.Flags&legacyOfficeArtSpFlagFlipH != 0
		transform.FlipV = node.Flags&legacyOfficeArtSpFlagFlipV != 0
		parseLegacyPptFOPTPropertiesTransform(node.Properties, colorScheme, &transform)

		kind := presentationir.ShapeText
		if (bounds.W == 0 || bounds.H == 0) && shared != nil && shared.Line.Visible && !node.HasClientTextbox {
			// A non-primitive/freeform OfficeArt shape can author a stroked zero-axis
			// anchor without using an MSOSPT connector type. Materialize that surface
			// as a canonical connector so the stroke has physical endpoints instead of
			// disappearing inside a zero-sized generic shape box.
			kind = presentationir.ShapeConnector
		}
		shape := &presentationir.Shape{
			Kind:            kind,
			OriginalShapeID: container.ShapeID,
			Name:            fmt.Sprintf("Legacy Shape %d", container.ShapeID),
			Bounds:          bounds,
			OfficeArt:       shared,
		}
		if kind == presentationir.ShapeConnector {
			// Degenerate freeform strokes and authored line shapes share the same
			// canonical connector geometry once their zero-axis anchor is known.
			// Normalizing the geometry here keeps PPT and PPTX from carrying two
			// representations of the same physical line into layout.
			shape.Geometry = "line"
			shape.ConnectorType = "straightConnector1"
			shape.ConnectorFlipH = transform.FlipH
			shape.ConnectorFlipV = transform.FlipV
			shape.ConnectorStart, shape.ConnectorEnd = legacyPptConnectorEndpoints(bounds, transform.FlipH, transform.FlipV)
		}
		applyLegacyPptOfficeArtSurface(shape, shared, transform)
		// ShapeFromEscherWithColorResolver already resolves the authored OfficeArt
		// surface with contextual COLORREF semantics (including fSysIndex references
		// to this shape's fill/line colors). Do not re-decode an explicit textbox
		// fill with the simpler PPT resolver: doing so can replace the resolved
		// authored color (for example the yellow pulse rectangles) with black.
		// Keep the legacy helper only as a fallback for records the shared decoder
		// could not materialize.
		if shape.FillColor == "" {
			if fill, valid := parseLegacyPptFOPTPropertiesTextFillColor(node.Properties, colorScheme); valid {
				shape.FillColor = fill
				shape.Fill = &drawing.SolidFill{Color: fill, Alpha: 1}
			}
		}
		out = append(out, shape)
		ownedIDs[container.ShapeID] = shape
	}
	return out
}

func legacyPptOfficeArtShapeIsEmptyTextbox(shape *officeartir.Shape) bool {
	if shape == nil {
		return true
	}
	if shape.Fill.Kind != officeartir.FillUnset && shape.Fill.Kind != officeartir.FillNone && shape.Fill.Alpha != 0 {
		return false
	}
	if shape.Line.Visible && shape.Line.Alpha != 0 {
		return false
	}
	if shape.Effects.Shadow != nil || shape.Effects.Glow != nil || shape.Effects.Blur != nil || shape.Image != nil || shape.GeometryText != nil {
		return false
	}
	if shape.Text != nil && len(shape.Text.Paragraphs) != 0 {
		return false
	}
	return true
}

// applyLegacyPptOfficeArtSurface is the single PPT adapter boundary for an
// authored OfficeArt shape surface. The shared OfficeArt IR owns geometry,
// fills, strokes, effects and image adjustments; PPT's transform keeps only
// application-specific compatibility such as arrow enum mapping and text-body
// behavior. Every semantic shape family must pass through this boundary so a
// picture, connector, textbox or grouped child cannot silently lose styling.
func applyLegacyPptOfficeArtSurface(shape *presentationir.Shape, shared *officeartir.Shape, transform legacyPptShapeTransform) {
	if shape == nil {
		return
	}
	applyLegacyPptOfficeArtVisual(shape, shared)
	// Geometry comes last: a rotated OfficeArt anchor is stored in its
	// orientation quadrant, so the canonical bounds only exist after the
	// transform step transposes them, and geometry scaled into the box (the
	// binary bent arrow) must use the canonical frame.
	applyLegacyPptShapeTransform(shape, transform)
	applyLegacyPptOfficeArtGeometry(shape, shared)
}

// applyLegacyPptOfficeArtGeometry bridges the shared OfficeArt geometry model to
// presentation canonical geometry. Preset names alone are insufficient for real
// legacy decks: freeforms and edited AutoShapes carry authored vertices/segments
// that must survive into layout's physical vector pipeline. The shared OfficeArt
// decoder remains the single owner of Escher geometry interpretation; this adapter
// only translates its normalized representation.
func applyLegacyPptOfficeArtGeometry(shape *presentationir.Shape, shared *officeartir.Shape) {
	if shape == nil || shared == nil {
		return
	}
	geometry := shared.Geometry
	if geometry.Kind == officeartir.GeometryCustom && len(geometry.Paths) != 0 {
		// A zero-axis OfficeArt freeform made only from open M/L segments is the
		// legacy representation of a straight connector. The scene classifier has
		// already established connector ownership from the authored anchor/stroke;
		// do not overwrite that canonical line geometry with syntax-level custom
		// geometry during the OfficeArt -> presentation bridge. This mirrors the
		// DrawingML boundary, which collapses the same degenerate open freeform.
		if shape.Kind == presentationir.ShapeConnector && legacyPptCustomGeometryIsZeroAxisLine(shape, geometry) {
			shape.Geometry = "line"
			shape.GeometryData = nil
			shape.GeometryAdjustments = nil
			if shape.ConnectorType == "" {
				shape.ConnectorType = "straightConnector1"
			}
			return
		}
		converted := &drawing.Geometry{Type: drawing.GeometryTypeCustom}
		for _, sourcePath := range geometry.Paths {
			path := drawing.Path{FillRule: sourcePath.FillRule, PaintFlagsSet: true, FillEnabled: sourcePath.Fill, StrokeEnabled: sourcePath.Stroke}
			for _, sourceCommand := range sourcePath.Commands {
				command := drawing.PathCommand{Type: sourceCommand.Op}
				for _, point := range sourceCommand.Points {
					command.Points = append(command.Points, drawing.Point{X: point.X, Y: point.Y})
				}
				path.Commands = append(path.Commands, command)
			}
			converted.Paths = append(converted.Paths, path)
		}
		shape.Geometry = ""
		shape.GeometryData = converted
		shape.GeometryAdjustments = append([]float64(nil), geometry.Adjustments...)
		return
	}
	if geometry.Preset != "" {
		shape.Geometry = geometry.Preset
	}
	if geometry.Preset == "bentArrow" && len(geometry.Adjustments) == 0 {
		// OfficeArt's bent arrow is not DrawingML's: its bend is a pair of
		// authored ellipses rather than the preset's quarter arc, so the
		// DrawingML formulas cannot reproduce it at any handle value.
		// PowerPoint's own conversion of such a shape materializes the binary
		// geometry as a custom path in a 21600 box; use it verbatim instead of
		// approximating the preset.
		shape.Geometry = ""
		shape.GeometryData = legacyPptOfficeArtBentArrowGeometry()
		return
	}
	if geometry.Preset == "rightArrow" {
		// OfficeArt and DrawingML expose the same two visual handles through
		// different coordinate systems and in a different order. Materialize
		// the effective DrawingML-style values even when the legacy slots were
		// omitted, because the OfficeArt defaults are aspect-ratio dependent
		// after conversion to DrawingML.
		shape.GeometryAdjustments = normalizeLegacyPptRightArrowAdjustments(geometry.Adjustments, geometry.AdjustmentPresent, shape.Bounds)
	} else if len(geometry.Adjustments) != 0 {
		shape.GeometryAdjustments = normalizeLegacyPptPresetAdjustments(geometry.Preset, geometry.Adjustments)
	} else if defaults := legacyPptDefaultPresetAdjustments(geometry.Preset); len(defaults) != 0 {
		shape.GeometryAdjustments = defaults
	}
}

// legacyPptOfficeArtBentArrowGeometry returns the binary format's bent-arrow
// outline with the control points PowerPoint itself writes when it converts
// such a shape to a custom path (verified against the OOXML copy of 41246-2) —
// the only faithful source for a shape whose binary geometry predates the
// DrawingML preset. Custom geometry is stored in normalized 0..1 coordinates,
// so the authored 21600-unit box becomes the unit square.
func legacyPptOfficeArtBentArrowGeometry() *drawing.Geometry {
	const box = 21600.0
	pt := func(x, y float64) drawing.Point { return drawing.Point{X: x / box, Y: y / box} }
	path := drawing.Path{FillRule: "nonzero", PaintFlagsSet: true, FillEnabled: true, StrokeEnabled: true}
	add := func(kind string, points ...drawing.Point) {
		path.Commands = append(path.Commands, drawing.PathCommand{Type: kind, Points: points})
	}
	add("M", pt(21600, 6079))
	add("L", pt(15126, 0))
	add("L", pt(15126, 2912))
	add("L", pt(12427, 2912))
	add("C", pt(5564, 2912), pt(0, 7052), pt(0, 12158))
	add("L", pt(0, 21600))
	add("L", pt(6474, 21600))
	add("L", pt(6474, 12158))
	add("C", pt(6474, 10550), pt(9139, 9246), pt(12427, 9246))
	add("L", pt(15126, 9246))
	add("L", pt(15126, 12158))
	add("Z")
	return &drawing.Geometry{Type: drawing.GeometryTypeCustom, Paths: []drawing.Path{path}}
}

// legacyPptDefaultPresetAdjustments returns the OfficeArt default handles for
// presets whose binary-format defaults differ from the DrawingML ones. Binary
// PowerPoint applies these when a shape authors no adjust properties, and its
// own OfficeArt -> DrawingML conversion materializes them: the converted
// copy of 41246-2 carries adj1=20000/adj2=40000/adj3=33333 for the curved
// right arrow and adj1=36667/adj2=73333/adj3=33333 for the curved left arrow,
// and the exported PDF draws that geometry. DrawingML's own defaults (25000 /
// 50000 / 25000) drew a visibly thinner, wrong-proportioned arrow.
func legacyPptDefaultPresetAdjustments(preset string) []float64 {
	switch preset {
	case "curvedRightArrow":
		return []float64{0.2, 0.4, 0.33333}
	case "curvedLeftArrow":
		return []float64{0.36667, 0.73333, 0.33333}
	default:
		return nil
	}
}

func legacyPptCustomGeometryIsZeroAxisLine(shape *presentationir.Shape, geometry officeartir.Geometry) bool {
	if shape == nil || (shape.Bounds.W == 0) == (shape.Bounds.H == 0) {
		return false
	}
	hasSegment := false
	for _, path := range geometry.Paths {
		for _, command := range path.Commands {
			switch command.Op {
			case "M":
			case "L":
				hasSegment = true
			default:
				return false
			}
		}
	}
	return hasSegment
}

// normalizeLegacyPptRightArrowAdjustments maps the two OfficeArt/VML right-arrow
// handles to DrawingML's canonical rightArrow adjustments. In OfficeArt the
// first handle is the x coordinate where the arrow head begins (default 16200
// in a 21600-wide geometry box) and the second is the shaft's top y coordinate
// (default 5400). DrawingML instead stores shaft thickness as adj1 and head
// length, measured in units of the short side, as adj2.
func normalizeLegacyPptRightArrowAdjustments(values []float64, present []bool, bounds presentationir.Rect) []float64 {
	const (
		legacyCoord      = 21600.0
		defaultHeadStart = 16200.0
		defaultShaftTop  = 5400.0
	)
	headStart := defaultHeadStart
	shaftTop := defaultShaftTop
	if len(values) > 0 && len(present) > 0 && present[0] {
		headStart = values[0]
	}
	if len(values) > 1 && len(present) > 1 && present[1] {
		shaftTop = values[1]
	}

	// DrawingML adj1 is the full shaft-height fraction. OfficeArt stores the
	// distance from the top edge to that shaft, hence 1 - 2*y/h.
	body := 1.0 - 2.0*(shaftTop/legacyCoord)
	if body < 0 {
		body = 0
	} else if body > 1 {
		body = 1
	}

	w := math.Abs(float64(bounds.W))
	h := math.Abs(float64(bounds.H))
	ss := math.Min(w, h)
	if ss == 0 {
		// With no physical bounds available, preserve the square-shape meaning.
		w, ss = 1, 1
	}
	head := (1.0 - headStart/legacyCoord) * w / ss
	maxHead := w / ss
	if head < 0 {
		head = 0
	} else if head > maxHead {
		head = maxHead
	}
	return []float64{body, head}
}

// normalizeLegacyPptPresetAdjustments converts shape-specific legacy OfficeArt
// adjustment units into the canonical units consumed by drawing PresetGeometry.
// Custom/freeform geometry deliberately keeps its raw adjustment values above
// because its authored path is already authoritative.
func normalizeLegacyPptPresetAdjustments(preset string, values []float64) []float64 {
	if len(values) == 0 {
		return nil
	}
	out := make([]float64, len(values))
	for i, value := range values {
		out[i] = normalizeLegacyPptPresetAdjustment(preset, i, value)
	}
	return out
}

// normalizeLegacyPptPresetAdjustment owns the OfficeArt -> canonical unit
// conversion for preset handles. OfficeArt adjustment values are shape-specific:
// ordinary Cartesian handles use the 21600 geometry space, while the angular
// handles used by arc-family presets are 16.16-degree values. Canonical drawing
// geometry consumes both as normalized fractions/turns, so converting every
// adjustment with /21600 corrupts authored angles.
func normalizeLegacyPptPresetAdjustment(preset string, slot int, value float64) float64 {
	switch preset {
	case "arc", "pie", "chord":
		// Legacy arc-family shapes have angular handles. One full turn is 360 degrees in
		// OfficeArt's 16.16 fixed-point angle representation.
		if slot < 2 {
			return value / (360.0 * 65536.0)
		}
	case "blockArc":
		// blockArc's first two handles are start/end angles; the third is
		// radial thickness in the ordinary 21600 geometry coordinate space.
		if slot < 2 {
			return value / (360.0 * 65536.0)
		}
	}
	return value / 21600.0
}

// applyLegacyPptOfficeArtVisual carries the shared OfficeArt visual model into
// presentation IR before PPT-specific palette resolution is applied. This keeps
// the Escher decoder authoritative for visual semantics while allowing the PPT
// adapter to override scheme-indexed colors with its effective master/slide
// palette. The bridge intentionally covers the whole ordinary AutoShape visual
// surface in one place rather than growing property-by-property adapters.
func applyLegacyPptOfficeArtVisual(shape *presentationir.Shape, shared *officeartir.Shape) {
	if shape == nil || shared == nil {
		return
	}
	if shared.Transform.RotationDeg != 0 {
		shape.RotationDeg = shared.Transform.RotationDeg
	}
	shape.FlipH = shape.FlipH || shared.Transform.FlipH
	shape.FlipV = shape.FlipV || shared.Transform.FlipV

	switch shared.Fill.Kind {
	case officeartir.FillNone:
		shape.FillColor = ""
		shape.Fill = drawing.NoFill{}
	case officeartir.FillSolid:
		if shared.Fill.Color != "" {
			alpha := shared.Fill.Alpha
			if alpha == 0 {
				alpha = 1
			}
			shape.FillColor = shared.Fill.Color
			shape.Fill = drawing.SolidFill{Color: shared.Fill.Color, Alpha: alpha}
		}
	case officeartir.FillGradient:
		if shared.Fill.Gradient != nil {
			// OfficeArt stores the fill angle in the painted (screen) frame: PowerPoint's own
			// conversion of a 90-degree-rotated edge rule keeps ang="0" and still draws the
			// gradient along the rule. Rotating the fill with the shape turned those edge
			// gradients across the rule instead of along it.
			g := drawing.GradientFill{GradientType: drawing.GradientType(shared.Fill.Gradient.Type), Angle: shared.Fill.Gradient.Angle, RotateWithShape: false, ScaleAngleWithAspect: shared.Fill.Gradient.ScaleAngleWithAspect, LegacyShadeType: shared.Fill.Gradient.LegacyShadeType}
			if shared.Fill.Gradient.HasFillTo {
				left, top := shared.Fill.Gradient.FillToLeft, shared.Fill.Gradient.FillToTop
				right, bottom := shared.Fill.Gradient.FillToRight, shared.Fill.Gradient.FillToBottom
				g.FillToRect = &drawing.Rect{X: left, Y: top, Width: max(0, 1-left-right), Height: max(0, 1-top-bottom)}
			}
			for _, stop := range shared.Fill.Gradient.Stops {
				g.Stops = append(g.Stops, drawing.GradientStop{Position: stop.Position, Color: stop.Color, Alpha: stop.Alpha})
			}
			shape.Fill = g
		}
	case officeartir.FillPattern:
		if shared.Fill.Pattern != nil {
			shape.Fill = drawing.PatternFill{PatternType: drawing.PatternType(shared.Fill.Pattern.Preset), Foreground: shared.Fill.Pattern.Foreground, Background: shared.Fill.Pattern.Background, ForeAlpha: 1, BackAlpha: 1}
		}
	case officeartir.FillPicture:
		if shared.Fill.Picture != nil {
			picture := shared.Fill.Picture
			alpha := shared.Fill.Alpha
			if alpha == 0 {
				alpha = 1
			}
			fill := drawing.PictureFill{
				ImageURL:    picture.Image.URL,
				EmbedID:     picture.Image.RelID,
				ImageData:   append([]byte(nil), picture.Image.Data...),
				ContentType: picture.Image.MIMEType,
				DPI:         picture.Image.DPI,
				Alpha:       alpha,
				Stretch:     picture.Tile == nil,
			}
			crop := picture.Image.Crop
			if crop.Left != 0 || crop.Top != 0 || crop.Right != 0 || crop.Bottom != 0 {
				fill.Crop = &drawing.Rect{X: crop.Left, Y: crop.Top, Width: max(0, 1-crop.Left-crop.Right), Height: max(0, 1-crop.Top-crop.Bottom)}
			}
			if picture.Tile != nil {
				fill.Tile = true
				fill.TileOffsetX = picture.Tile.OffsetXEMU
				fill.TileOffsetY = picture.Tile.OffsetYEMU
				fill.TileScaleX = int(picture.Tile.ScaleX * 100000)
				fill.TileScaleY = int(picture.Tile.ScaleY * 100000)
			}
			shape.Fill = fill
		}
	}

	if shared.Line.Visible {
		ls := drawing.DefaultLineStyle()
		if shared.Line.Color != "" {
			ls.Color = shared.Line.Color
			shape.LineColor = shared.Line.Color
		}
		// OfficeArt IR owns line opacity. Preserve zero: it is a valid authored
		// fully-transparent stroke, not an absent/default value.
		ls.Alpha = shared.Line.Alpha
		if shared.Line.WidthPt > 0 {
			ls.Width = shared.Line.WidthPt
			shape.LineWidthPt = shared.Line.WidthPt
		}
		if shared.Line.Dash != "" {
			ls.Dash = drawing.DashPattern(shared.Line.Dash)
		}
		if shared.Line.Cap != "" {
			ls.Cap = drawing.LineCap(shared.Line.Cap)
		}
		if shared.Line.Join != "" {
			ls.Join = drawing.LineJoin(shared.Line.Join)
		}
		shape.LineStyle = &ls
	} else if shared.Line.Color != "" || shared.Line.WidthPt > 0 || shared.Line.Dash != "" {
		// A decoded line with authored properties but explicit visibility=false is
		// authoritative: do not let renderer defaults resurrect the stroke.
		shape.LineColor = ""
		shape.LineWidthPt = 0
		shape.LineStyle = nil
	}

	// Endpoint decorations are part of the shared OfficeArt line surface, not a
	// connector-only concern. Canonical semantics match DrawingML and the vector
	// builder: StartArrow -> ArrowHead (first point), EndArrow -> ArrowTail (last
	// point). An explicitly invisible line owns no endpoint paint.
	if shared.Line.Visible {
		if shared.Line.StartArrow.Type != "" {
			shape.ArrowHead = shared.Line.StartArrow.Type
			shape.ArrowHeadWidth = shared.Line.StartArrow.Width
			shape.ArrowHeadLength = shared.Line.StartArrow.Length
		}
		if shared.Line.EndArrow.Type != "" {
			shape.ArrowTail = shared.Line.EndArrow.Type
			shape.ArrowTailWidth = shared.Line.EndArrow.Width
			shape.ArrowTailLength = shared.Line.EndArrow.Length
		}
		shape.ArrowHead, shape.ArrowHeadWidth, shape.ArrowHeadLength = presentationir.CanonicalArrowEnd(shape.ArrowHead, shape.ArrowHeadWidth, shape.ArrowHeadLength)
		shape.ArrowTail, shape.ArrowTailWidth, shape.ArrowTailLength = presentationir.CanonicalArrowEnd(shape.ArrowTail, shape.ArrowTailWidth, shape.ArrowTailLength)
	}

	if shared.Effects.Shadow != nil {
		shadow := shared.Effects.Shadow
		if shadow.Inner {
			shape.Effects.Effects = append(shape.Effects.Effects, drawing.InnerShadow{Color: shadow.Color, Alpha: shadow.Alpha, Blur: shadow.BlurRadius, Distance: shadow.Distance, Angle: shadow.AngleDeg})
		} else {
			shape.Effects.Effects = append(shape.Effects.Effects, drawing.Shadow{Color: shadow.Color, Alpha: shadow.Alpha, Blur: shadow.BlurRadius, Distance: shadow.Distance, Angle: shadow.AngleDeg})
		}
	}
	if shared.Effects.Glow != nil {
		glow := shared.Effects.Glow
		shape.Effects.Effects = append(shape.Effects.Effects, drawing.Glow{Color: glow.Color, Alpha: glow.Alpha, Radius: glow.RadiusPt})
	}
	if shared.Effects.Blur != nil {
		blur := shared.Effects.Blur
		shape.Effects.Effects = append(shape.Effects.Effects, drawing.Blur{Radius: blur.RadiusPt})
	}

	if shared.Image != nil {
		shape.ImageCropLeft = shared.Image.Crop.Left
		shape.ImageCropTop = shared.Image.Crop.Top
		shape.ImageCropRight = shared.Image.Crop.Right
		shape.ImageCropBottom = shared.Image.Crop.Bottom
		if shared.Image.DPI > 0 {
			shape.ImageDPI = shared.Image.DPI
		}

		// OfficeArt decodes legacy picture adjustments into format-independent
		// image semantics. Preserve them on presentation IR instead of dropping
		// them at the PPT adapter boundary. The shared physical image pipeline
		// already owns these DrawingML-equivalent effects for PPTX, so legacy PPT
		// pictures must enter that same path.
		brightness := shared.Image.Brightness
		contrast := shared.Image.Contrast
		if brightness != 0 || contrast != 0 {
			if brightness == 0 {
				brightness = 1
			}
			if contrast == 0 {
				contrast = 1
			}
			if brightness != 1 || contrast != 1 {
				shape.ImageEffects = append(shape.ImageEffects, drawing.Lum{
					Brightness: int((brightness - 1) * 100000),
					Contrast:   int((contrast - 1) * 100000),
				})
			}
		}
		if shared.Image.Grayscale {
			shape.ImageEffects = append(shape.ImageEffects, drawing.Grayscale{})
		}
		if shared.Image.BiLevel {
			// OfficeArt's fBiLevel flag has no authored threshold. PowerPoint uses
			// the midpoint for the equivalent black/white picture adjustment.
			shape.ImageEffects = append(shape.ImageEffects, drawing.BiLevel{Threshold: 50000})
		}
	}
}

// legacyPptBackgroundFill resolves any OfficeArt fBackground surface into the
// canonical DrawingML fill model. Background containers commonly carry gradients
// and patterns without a usable client anchor, so they must not depend on ordinary
// shape projection/placement.
func legacyPptBackgroundFill(scene legacyPptVisualScene, colorScheme map[uint8]string) drawing.Fill {
	const spFlagBackground uint32 = 0x00000400
	for _, event := range scene.dispatch.containers.officeArtEvents() {
		node := event.ShapeNode
		if !event.ShapeStart || node == nil || node.Flags&spFlagBackground == 0 {
			continue
		}
		shared := officeartlegacy.ShapeFromEscherWithColorResolver(node, func(raw uint32) string {
			return resolveLegacyPptOfficeArtColorRef(raw, colorScheme)
		})
		if shared == nil {
			continue
		}
		tmp := &presentationir.Shape{}
		applyLegacyPptOfficeArtVisual(tmp, shared)
		switch tmp.Fill.(type) {
		case drawing.NoFill:
			return tmp.Fill
		default:
			if tmp.Fill != nil {
				return tmp.Fill
			}
		}
	}
	return nil
}

// legacyPptPictureBackgroundFill resolves an OfficeArt fBackground shape into
// canonical slide background paint. Background shapes normally have no client
// anchor, so projecting them as ordinary picture shapes loses the authored
// full-slide image.
func legacyPptPictureBackgroundFill(scene legacyPptVisualScene, pictureStore map[uint32]*legacyPptPicture) drawing.Fill {
	const (
		spFlagBackground uint32 = 0x00000400
		propFillBlip     uint16 = 0x0186
	)
	if len(pictureStore) == 0 {
		return nil
	}
	for _, event := range scene.dispatch.containers.officeArtEvents() {
		node := event.ShapeNode
		if !event.ShapeStart || node == nil || node.Flags&spFlagBackground == 0 {
			continue
		}
		var ref uint32
		for _, prop := range node.Properties {
			if prop.ID == propFillBlip && !prop.IsComplex {
				ref = prop.Value
				break
			}
		}
		picture := pictureStore[ref]
		if ref == 0 || picture == nil || len(picture.Data) == 0 {
			continue
		}
		return drawing.PictureFill{
			ImageData:   append([]byte(nil), picture.Data...),
			ContentType: picture.MIME,
			Stretch:     true,
			Alpha:       1,
		}
	}
	return nil
}

// legacyPptContainerOwnership makes the completeness contract explicit for every
// authored SpContainer. A container is either represented by canonical IR or is
// intentionally structural/background-only. Anything else is unclaimed and is
// therefore a parser-completeness gap rather than a silently ignored record.
type legacyPptContainerOwnership struct {
	ShapeID uint32
	Reason  string
	Claimed bool
}

func (s legacyPptVisualScene) ownershipAudit(projected []*presentationir.Shape) []legacyPptContainerOwnership {
	return s.ownershipAuditWithTablePaths(projected, nil)
}

// ownershipAuditWithTablePaths consumes semantic ownership emitted by the table
// projector itself. A recovered legacy table can have an unavailable/unstable
// group-root ShapeID even though its group topology materialized successfully, so
// group path is the authoritative ownership key for table descendants.
func (s legacyPptVisualScene) ownershipAuditWithTablePaths(projected []*presentationir.Shape, tableOwnedPaths [][]int) []legacyPptContainerOwnership {
	claimed := make(map[uint32]struct{})
	projectedTables := make(map[uint32]struct{})
	var collect func(*presentationir.Shape)
	collect = func(shape *presentationir.Shape) {
		if shape == nil {
			return
		}
		if shape.OriginalShapeID != 0 {
			claimed[shape.OriginalShapeID] = struct{}{}
			if shape.Kind == presentationir.ShapeTable {
				projectedTables[shape.OriginalShapeID] = struct{}{}
			}
		}
		for _, child := range shape.Children {
			collect(child)
		}
	}
	for _, shape := range projected {
		collect(shape)
	}

	const spFlagBackground uint32 = 0x00000400
	out := make([]legacyPptContainerOwnership, 0, len(s.dispatch.containers.shapeContainers()))
	for _, container := range s.dispatch.containers.shapeContainers() {
		entry := legacyPptContainerOwnership{ShapeID: container.ShapeID}
		switch {
		case container.ShapeID == 0:
			// Shape identity comes from the decoded FSP record. If decoding did not
			// yield a ShapeID there is no authored visual identity that canonical IR
			// could preserve. Treat this as structural/no-ID ownership before looking
			// at Node so malformed scaffolding does not become a false corpus gap.
			entry.Claimed, entry.Reason = true, "structural: no authored ShapeID"
		case container.Node == nil:
			// This branch is intentionally defensive: current scene decoding derives
			// ShapeID from Node, so Node==nil with a non-zero ShapeID should be
			// impossible. If a future decoder preserves identity across a partial
			// decode, keep it visible as a genuine completeness failure.
			entry.Reason = "unclaimed: missing decoded ShapeNode"
		case container.IsGroupRoot:
			entry.Claimed, entry.Reason = true, "structural: group root"
		case container.Node.Flags&spFlagBackground != 0:
			entry.Claimed, entry.Reason = true, "background paint"
		default:
			_, entry.Claimed = claimed[container.ShapeID]
			if entry.Claimed {
				entry.Reason = "canonical IR"
			} else if s.tableTopologyOwnsLine(container, projectedTables, tableOwnedPaths) {
				// Explicit PowerPoint tables author their borders/grid as child line
				// SpContainers. Once the containing group has successfully materialized
				// as canonical ShapeTable, those line containers are semantic table
				// topology rather than independent drawable shapes. Keep them out of
				// generic fallback and account for them explicitly in the ownership
				// ledger so zero-axis table borders do not look like parser gaps.
				entry.Claimed = true
				entry.Reason = "table topology"
			} else {
				placement := resolveLegacyPptShapePlacement(container.Node, s.frameForContainer(container))
				if !placement.valid() {
					entry.Reason = "unclaimed: placement: " + placement.Reason
				} else if placement.Bounds.W <= 0 || placement.Bounds.H <= 0 {
					entry.Reason = "unclaimed: placement: degenerate bounds"
				} else {
					entry.Reason = "unclaimed: no canonical IR owner"
				}
			}
		}
		out = append(out, entry)
	}

	// Grid lines are structural children of explicit PPT table groups. Audit the
	// semantic table once per group instead: if the table failed to materialize,
	// expose one group-level completeness gap rather than one gap per border line.
	for _, group := range s.dispatch.containers.shapeGroups() {
		if !legacyPptGroupContainerIsTable(group.Event.Payload) {
			continue
		}
		materialized := legacyPptOwnedGroupPath(tableOwnedPaths, group.Path)
		rootID, hasRootID := legacyPptGroupRootShapeID(group.Event.Payload)
		if !materialized && hasRootID && rootID != 0 {
			_, materialized = projectedTables[rootID]
		}
		if materialized {
			continue
		}
		out = append(out, legacyPptContainerOwnership{
			ShapeID: rootID,
			Reason:  "unclaimed: explicit table group did not materialize",
			Claimed: false,
		})
	}
	return out
}

func (s legacyPptVisualScene) tableTopologyOwnsLine(container legacyPptShapeContainer, projectedTables map[uint32]struct{}, tableOwnedPaths [][]int) bool {
	if container.Node == nil || legacyPptConnectorType(container.Node.ShapeType) == "" {
		return false
	}

	// An explicit PowerPoint table owns its grid/border SpContainers as structural
	// topology regardless of whether the higher-level table object can be rebuilt.
	// Materialization failure is audited once at the table-group boundary instead
	// of surfacing hundreds of child line failures.
	for _, group := range s.dispatch.containers.shapeGroups() {
		if !legacyPptGroupContainerIsTable(group.Event.Payload) {
			continue
		}
		if legacyPptGroupPathHasPrefix(container.GroupPath, group.Path) {
			return true
		}
	}

	// Inferred (non-explicit) tables still require successful semantic
	// materialization before their lines are consumed as table topology.
	for _, path := range tableOwnedPaths {
		if legacyPptGroupPathHasPrefix(container.GroupPath, path) {
			return true
		}
	}
	if len(projectedTables) == 0 {
		return false
	}
	for _, group := range s.dispatch.containers.shapeGroups() {
		if !legacyPptGroupPathHasPrefix(container.GroupPath, group.Path) {
			continue
		}
		rootID, ok := legacyPptGroupRootShapeID(group.Event.Payload)
		if !ok || rootID == 0 {
			continue
		}
		if _, ok := projectedTables[rootID]; ok {
			return true
		}
	}
	return false
}

func appendLegacyPptOwnedGroupPath(paths [][]int, path []int) [][]int {
	if len(path) == 0 {
		return paths
	}
	for _, existing := range paths {
		if legacyPptGroupPathEqual(existing, path) {
			return paths
		}
	}
	return append(paths, append([]int(nil), path...))
}

func legacyPptOwnedGroupPath(paths [][]int, path []int) bool {
	for _, existing := range paths {
		if legacyPptGroupPathEqual(existing, path) {
			return true
		}
	}
	return false
}
