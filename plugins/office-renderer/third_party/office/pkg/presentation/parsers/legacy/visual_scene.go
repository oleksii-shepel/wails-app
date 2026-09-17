package legacy

import (
	"fmt"
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

// project materializes the complete local visual scene from one OfficeArt event
// sequence. Reconciliation (table-vs-text ownership, connector endpoint binding,
// and grouped-child suppression) happens here instead of in parsePptFile.
func (s legacyPptVisualScene) project(
	slideText legacySlideText,
	fontTable map[uint16]string,
	colorScheme map[uint8]string,
	placeholderTemplates map[presentationir.PlaceholderKind]legacyPptTextShape,
	exHyperlinks map[uint32]string,
	pictureStore map[uint32]*legacyPptPicture,
	externalObjects map[uint32]legacyPptExternalObject,
) []*presentationir.Shape {
	metas := s.textMetas(fontTable, colorScheme, exHyperlinks)
	metas = inheritLegacyPptMasterPlaceholderTemplates(metas, placeholderTemplates)
	shapes := buildLegacyPptTextShapesFromMetas(metas, slideText)

	var tables []*presentationir.Shape
	collectLegacyPptTableShapesFromSceneForSlide(s.dispatch.containers, legacyPptGroupFrame{}, slideText, fontTable, colorScheme, exHyperlinks, &tables)
	tablesByScene := map[int][]*presentationir.Shape{1: tables}
	textByScene := map[int][]*presentationir.Shape{1: shapes}
	tablesByScene = filterLegacyDuplicateProjectedTables(textByScene, tablesByScene)
	shapes = append(shapes, tablesByScene[1]...)

	// Picture shape ownership comes from the OfficeArt scene, not from successful
	// BStore decoding. Always project picture nodes so a missing/damaged Pictures
	// stream cannot silently turn authored pictures into generic AutoShapes.
	var pictures []*presentationir.Shape
	collectLegacyPptImageShapesFromEvents(s.dispatch.containers.officeArtEvents(), legacyPptGroupFrame{}, pictureStore, externalObjects, &pictures)
	shapes = append(shapes, pictures...)

	// BIG37: materialize ordinary OfficeArt AutoShapes from the same semantic
	// container scene before connector/group reconciliation. Historically the
	// PPT adapter projected only family-specific text/table/picture shapes, which
	// meant master artwork such as rectangles, callouts and decorative shapes
	// silently disappeared. Generic shapes now participate in the same scene and
	// are suppressed whenever a richer family projection already owns the ID.
	shapes = append(shapes, s.genericShapes(shapes, colorScheme)...)

	shapeIndex := legacyPptShapeIndexByOriginalID(shapes)
	var connectors []*presentationir.Shape
	collectLegacyPptConnectorShapesFromEvents(s.dispatch.containers.officeArtEvents(), legacyPptGroupFrame{}, colorScheme, s.dispatch.rules, shapeIndex, &connectors)
	shapes = append(shapes, connectors...)

	groupSource := legacyPptDiscoveredShapeSource(legacyPptShapeIndexByOriginalID(shapes))
	groups := buildLegacyPptGroupShapesFromScene(s.dispatch.containers, groupSource, fontTable, colorScheme, exHyperlinks)
	if len(groups) != 0 {
		groupMap := map[int][]*presentationir.Shape{1: groups}
		flatMap := map[int][]*presentationir.Shape{1: shapes}
		flatMap = filterLegacyDuplicateProjectedTables(groupMap, flatMap)
		flatMap = filterLegacyGroupedShapeDuplicates(flatMap, groupMap)
		shapes = append(flatMap[1], groups...)
	}
	return s.sourceOrdered(shapes)
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
	var out []*presentationir.Shape
	for _, container := range s.dispatch.containers.shapeContainers() {
		if container.Node == nil || container.IsGroupRoot || container.ShapeID == 0 {
			continue
		}
		if _, exists := ownedIDs[container.ShapeID]; exists {
			continue
		}
		node := container.Node
		// Rich semantic families own their authored SpContainer even when payload
		// resolution is incomplete. Falling back to a generic AutoShape for a
		// picture or ClientTextbox creates a second visible object and makes a
		// damaged relationship look like authored artwork. Keep ownership at the
		// semantic boundary; family projectors may retain a placeholder/payload-less
		// node for diagnostics, but generic projection must never compete for it.
		if node.ShapeType == legacyOfficeArtShapePicture || node.HasClientTextbox {
			continue
		}
		var bounds presentationir.Rect
		var ok bool
		if node.Flags&legacyOfficeArtSpFlagChild != 0 && len(node.ChildAnchor) != 0 {
			if local, valid := parseLegacyPptChildAnchor(node.ChildAnchor); valid {
				// A shape belongs to the deepest authored group whose path is an
				// ancestor of the container. Exact matching loses nested-group
				// transforms and leaves ChildAnchor coordinates in group space.
				frame := legacyPptGroupFrame{}
				bestDepth := -1
				for _, group := range s.dispatch.containers.shapeGroups() {
					if legacyPptGroupPathHasPrefix(container.GroupPath, group.Path) && len(group.Path) > bestDepth {
						frame = group.Frame
						bestDepth = len(group.Path)
					}
				}
				bounds, ok = mapLegacyPptRectThroughGroupFrame(local, frame)
			}
		}
		if !ok && len(node.ClientAnchor) != 0 {
			bounds, ok = parseLegacyPptClientAnchor(node.ClientAnchor)
		}
		// ChildAnchor coordinates are meaningful only inside an authored group
		// coordinate system.  Treating an unresolved child anchor as slide-space
		// geometry leaks raw Escher group units into canonical bounds (often many
		// times the slide width).  Non-child shapes may still carry a ChildAnchor
		// as their only placement, but FLAG_CHILD shapes require a resolved frame.
		if !ok && node.Flags&legacyOfficeArtSpFlagChild == 0 && len(node.ChildAnchor) != 0 {
			bounds, ok = parseLegacyPptChildAnchor(node.ChildAnchor)
		}
		if !ok || bounds.W <= 0 || bounds.H <= 0 {
			continue
		}

		shared := officeartlegacy.ShapeFromEscherWithColorResolver(node, func(raw uint32) string {
			return resolveLegacyPptOfficeArtColorRef(raw, colorScheme)
		})
		shape := &presentationir.Shape{
			Kind:            presentationir.ShapeText,
			OriginalShapeID: container.ShapeID,
			Name:            fmt.Sprintf("Legacy Shape %d", container.ShapeID),
			Bounds:          bounds,
			OfficeArt:       shared,
		}
		applyLegacyPptOfficeArtGeometry(shape, shared)
		applyLegacyPptOfficeArtVisual(shape, shared)
		var transform legacyPptShapeTransform
		parseLegacyPptFOPTPropertiesTransform(node.Properties, colorScheme, &transform)
		if fill, valid := parseLegacyPptFOPTPropertiesTextFillColor(node.Properties, colorScheme); valid {
			shape.FillColor = fill
			shape.Fill = &drawing.SolidFill{Color: fill, Alpha: 1}
		}
		applyLegacyPptShapeTransform(shape, transform)
		out = append(out, shape)
		ownedIDs[container.ShapeID] = shape
	}
	return out
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
		converted := &drawing.Geometry{Type: drawing.GeometryTypeCustom}
		for _, sourcePath := range geometry.Paths {
			path := drawing.Path{FillRule: sourcePath.FillRule}
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
	if len(geometry.Adjustments) != 0 {
		shape.GeometryAdjustments = normalizeLegacyPptPresetAdjustments(geometry.Preset, geometry.Adjustments)
	}
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
			g := drawing.GradientFill{GradientType: drawing.GradientType(shared.Fill.Gradient.Type), Angle: shared.Fill.Gradient.Angle, RotateWithShape: true, ScaleAngleWithAspect: shared.Fill.Gradient.ScaleAngleWithAspect, LegacyShadeType: shared.Fill.Gradient.LegacyShadeType}
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

	if shared.Effects.Shadow != nil {
		shadow := shared.Effects.Shadow
		shape.Effects.Effects = append(shape.Effects.Effects, drawing.Shadow{Color: shadow.Color, Alpha: shadow.Alpha, Blur: shadow.BlurRadius, Distance: shadow.Distance, Angle: shadow.AngleDeg})
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

func (s legacyPptVisualScene) pictures(pictureStore map[uint32]*legacyPptPicture, externalObjects map[uint32]legacyPptExternalObject) []*presentationir.Shape {
	var pictures []*presentationir.Shape
	collectLegacyPptImageShapesFromEvents(s.dispatch.containers.officeArtEvents(), legacyPptGroupFrame{}, pictureStore, externalObjects, &pictures)
	return pictures
}

// textShapes projects text directly from the scene's already collected event stream.
func (s legacyPptVisualScene) textShapes(slideText legacySlideText, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string) []*presentationir.Shape {
	return buildLegacyPptTextShapesFromMetas(s.textMetas(fontTable, colorScheme, exHyperlinks), slideText)
}

func (s legacyPptVisualScene) tables(fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string) []*presentationir.Shape {
	var shapes []*presentationir.Shape
	collectLegacyPptTableShapesFromScene(s.dispatch.containers, legacyPptGroupFrame{}, fontTable, colorScheme, exHyperlinks, &shapes)
	return shapes
}

func (s legacyPptVisualScene) connectors(colorScheme map[uint8]string, shapeIndex map[uint32]*presentationir.Shape) []*presentationir.Shape {
	var shapes []*presentationir.Shape
	collectLegacyPptConnectorShapesFromEvents(s.dispatch.containers.officeArtEvents(), legacyPptGroupFrame{}, colorScheme, s.dispatch.rules, shapeIndex, &shapes)
	return shapes
}

func (s legacyPptVisualScene) groups(childSource legacyPptGroupChildSource, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string) []*presentationir.Shape {
	return buildLegacyPptGroupShapesFromScene(s.dispatch.containers, childSource, fontTable, colorScheme, exHyperlinks)
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
