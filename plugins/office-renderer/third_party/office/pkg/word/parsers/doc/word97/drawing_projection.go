package word97

import (
	"io"
	"sort"

	"office/pkg/ooxml/cfb"
	wordir "office/pkg/word/ir"
	docbinary "office/pkg/word/parsers/doc/binary"
	docofficeart "office/pkg/word/parsers/doc/officeart"
)

// DrawingProjection is the complete canonical drawing/OLE state consumed by
// the sequential Word97 drawing cursor.
type DrawingProjection struct {
	Drawings          map[int]Drawing
	InlineImages      []*wordir.Image
	TextBoxes         []*wordir.TextBox
	OLEObjects        []wordir.Run
	OLEByCP           map[int]*wordir.Run
	ExpectedTextboxes int
	ResolvedTextboxes int
}

// ResolveDrawingProjection owns normal Word97 drawing assembly: OfficeArt
// visuals, FSPA placement, textbox ownership/materialization, inline media and
// OLE previews. No legacy parser state is required.
func ResolveDrawingProjection(word io.ReaderAt, wordSize int64, f *cfb.File, opened *OpenCore) DrawingProjection {
	projection := DrawingProjection{OLEByCP: make(map[int]*wordir.Run)}
	if word == nil || opened == nil {
		return projection
	}

	projection.InlineImages = LoadInlineImages(f)
	projection.OLEObjects = LoadOLEObjects(f)
	if meta := LoadPreservation(f); meta != nil {
		if objects := meta.OLERuns(); len(objects) > 0 {
			projection.OLEObjects = objects
		}
	}

	var dataStream []byte
	if f != nil {
		if data, ok := f.Stream("Data"); ok && len(data) > 0 {
			dataStream = data
		} else if data, ok = f.Stream("DataStream"); ok && len(data) > 0 {
			dataStream = data
		}
	}
	docofficeart.AttachOLEPreviews(projection.OLEObjects, dataStream)

	metadata := opened.Drawings
	anchors := append([]docbinary.FloatingAnchor(nil), metadata.MainAnchors...)
	sort.SliceStable(anchors, func(i, j int) bool { return anchors[i].CP < anchors[j].CP })

	officeArtStreams := [][]byte{metadata.DggInfo, dataStream}
	// An FSPA anchor's shape ID names a drawing-layer shape, and that layer is
	// FibDggInfo. The Data stream also stores OfficeArt picture frames, but those
	// belong to inline pictures (paired with their PICF block) and reuse the same
	// ID space, so a Data record must never shadow the anchored shape that
	// carries the picture's rotation and corrections.
	catalog := docofficeart.DecodeStreams(metadata.DggInfo).MergeMissing(docofficeart.DecodeStreams(dataStream))
	images := docofficeart.ProjectImages(officeArtStreams...)

	// DggInfo/Data are authoritative. Some older producers (including historical
	// versions of this package's writer) appended valid OfficeArt records to
	// WordDocument instead. Read the whole WordDocument only when an anchored
	// shape ID is genuinely absent, and merge only those missing entries. Never
	// replace a canonical DggInfo/Data shape with whole-stream recovery.
	var recoveredWord []byte
	if catalogMissingAnchoredShapes(catalog, anchors) {
		recoveredWord = readWholeReaderAt(word, wordSize)
		if len(recoveredWord) > 0 {
			catalog = catalog.MergeMissing(docofficeart.DecodeStreams(recoveredWord))
		}
	}

	// BLIP payloads can likewise be split across DggInfo/Data and WordDocument.
	// Image recovery may scan all three byte sources, but shape semantics remain
	// in the catalog assembled above.
	if len(images) < projectedImageDemand(catalog, anchors) {
		if len(recoveredWord) == 0 {
			recoveredWord = readWholeReaderAt(word, wordSize)
		}
		if len(recoveredWord) > 0 {
			recoveredStreams := append(append([][]byte(nil), officeArtStreams...), recoveredWord)
			if recoveredImages := docofficeart.ProjectImages(recoveredStreams...); len(recoveredImages) > len(images) {
				images = recoveredImages
			}
		}
	}

	resolved := make(map[int]Drawing, len(anchors))
	byShape := make(map[uint32]*wordir.Shape, len(anchors)+len(metadata.HeaderAnchors))
	allAnchors := append(append([]docbinary.FloatingAnchor(nil), anchors...), metadata.HeaderAnchors...)
	anchorByShape := docbinary.AnchorMapByShapeID(allAnchors)

	imageIndex := 0
	reuseSingle := len(images) == 1
	for _, anchor := range anchors {
		sourceShape, hasSourceShape := catalog.Find(anchor.Shape)
		shapeHasSemanticContent := hasSourceShape && sourceShape != nil &&
			(sourceShape.Text != nil || sourceShape.GeometryText != nil || len(sourceShape.Children) > 0)
		shapeOwnsImage := hasSourceShape && sourceShape != nil && (sourceShape.Image != nil || sourceShape.Kind == "picture")
		// Picture frames and structurally-unclassified anchors consume sequential
		// image payloads. Textbox/group shapes keep their semantic ownership even
		// when unrelated BLIPs exist in the same OfficeArt stream.
		if shapeOwnsImage || !hasSourceShape || !shapeHasSemanticContent {
			template := selectProjectedImage(images, imageIndex, reuseSingle)
			if template != nil {
				image := *template
				if docbinary.ApplyAnchorToImage(&image, anchor) {
					catalog.ApplyImageVisual(anchor.Shape, &image)
					resolved[anchor.CP] = Drawing{Image: &image}
					if !reuseSingle {
						imageIndex++
					}
					continue
				}
			}
		}
		if shape := catalog.ProjectAnchoredShape(anchor); shape != nil {
			resolved[anchor.CP] = Drawing{Shape: shape}
			if anchor.Shape != 0 {
				byShape[anchor.Shape] = shape
			}
		}
	}

	projection.ExpectedTextboxes = len(metadata.Textboxes)
	textboxes := MaterializeTextboxes(metadata.Textboxes, anchorByShape, func(startCP, endCP int) wordir.Story {
		return MaterializeStoryRange(word, wordSize, opened, StoryRange{StartCP: startCP, EndCP: endCP})
	})
	if len(textboxes) > 0 {
		projection.TextBoxes = make([]*wordir.TextBox, 0, len(textboxes))
	}
	textFlows := catalog.TextFlows()
	for _, owned := range textboxes {
		tb := owned.TextBox
		if tb == nil {
			continue
		}
		shapeID := owned.Range.ShapeID
		if tb.Vert == "" {
			if flow := textFlows[shapeID]; flow != "" {
				tb.Vert = flow
				tb.HasVertSetting = true
			}
		}
		projection.TextBoxes = append(projection.TextBoxes, tb)
		if shape := byShape[shapeID]; shape != nil && shape.TextBox != nil {
			docofficeart.MergeTextBoxVisual(tb, shape.TextBox)
		}
		if shape := byShape[shapeID]; shape != nil {
			shape.TextBox = tb
			projection.ResolvedTextboxes++
			continue
		}
		if anchor, ok := anchorByShape[shapeID]; ok {
			if shape := catalog.ProjectAnchoredShape(anchor); shape != nil {
				if shape.TextBox != nil {
					docofficeart.MergeTextBoxVisual(tb, shape.TextBox)
				}
				shape.TextBox = tb
				byShape[shapeID] = shape
				if _, exists := resolved[anchor.CP]; !exists {
					resolved[anchor.CP] = Drawing{Shape: shape}
				}
				projection.ResolvedTextboxes++
			}
		}
	}

	if len(resolved) > 0 {
		projection.Drawings = resolved
	}
	return projection
}

func catalogMissingAnchoredShapes(catalog docofficeart.Catalog, anchors []docbinary.FloatingAnchor) bool {
	for _, anchor := range anchors {
		if anchor.Shape == 0 {
			continue
		}
		if _, ok := catalog.Find(anchor.Shape); !ok {
			return true
		}
	}
	return false
}

func projectedImageDemand(catalog docofficeart.Catalog, anchors []docbinary.FloatingAnchor) int {
	demand := 0
	for _, anchor := range anchors {
		sourceShape, hasSourceShape := catalog.Find(anchor.Shape)
		shapeHasSemanticContent := hasSourceShape && sourceShape != nil &&
			(sourceShape.Text != nil || sourceShape.GeometryText != nil || len(sourceShape.Children) > 0)
		shapeOwnsImage := hasSourceShape && sourceShape != nil && (sourceShape.Image != nil || sourceShape.Kind == "picture")
		if shapeOwnsImage || !hasSourceShape || !shapeHasSemanticContent {
			demand++
		}
	}
	return demand
}

func readWholeReaderAt(reader io.ReaderAt, size int64) []byte {
	if reader == nil || size <= 0 || size > int64(^uint(0)>>1) {
		return nil
	}
	buf := make([]byte, int(size))
	n, err := reader.ReadAt(buf, 0)
	if err != nil && err != io.EOF {
		return nil
	}
	if n <= 0 {
		return nil
	}
	return buf[:n]
}

func selectProjectedImage(images []*wordir.Image, index int, reuseSingle bool) *wordir.Image {
	if len(images) == 0 {
		return nil
	}
	if reuseSingle {
		return images[0]
	}
	if index < 0 || index >= len(images) {
		return nil
	}
	return images[index]
}
