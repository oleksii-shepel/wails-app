package word97

import wordir "office/pkg/word/ir"

// Drawing is one already-resolved Word97 drawing payload keyed by its anchor CP.
// Binary OfficeArt decoding is deliberately separate from this stream state.
type Drawing struct {
	Image *wordir.Image
	Shape *wordir.Shape
}

// UnsupportedError reports a construct whose sequential representation would
// otherwise lose semantics. The outer DOC parser translates this at its API boundary.
type UnsupportedError struct{ Feature string }

func (e UnsupportedError) Error() string { return "word97 stream: unsupported " + e.Feature }

type drawingCursor struct {
	inner                Cursor
	drawings             map[int]Drawing
	inlineImages         []*wordir.Image
	inlineIndex          int
	inlineByPicLocation  map[int]*wordir.Image
	pictureData          []byte
	oleObjects           []wordir.Run
	oleIndex             int
	oleByCP              map[int]*wordir.Run
	queue                []Run
	pendingErr           error
	compatibilityAnchors bool
}

func NewDrawingCursor(inner Cursor, drawings map[int]Drawing, inlineImages []*wordir.Image, oleObjects []wordir.Run, oleByCP map[int]*wordir.Run) Cursor {
	return NewDrawingCursorWithPictureLocations(inner, drawings, inlineImages, nil, oleObjects, oleByCP)
}

// NewDrawingCursorWithPictureLocations consumes a pre-resolved picture map.
// It is retained for focused tests and callers that already own that map.
func NewDrawingCursorWithPictureLocations(inner Cursor, drawings map[int]Drawing, inlineImages []*wordir.Image, inlineByPicLocation map[int]*wordir.Image, oleObjects []wordir.Run, oleByCP map[int]*wordir.Run) Cursor {
	if inner == nil {
		return nil
	}
	return &drawingCursor{inner: inner, drawings: drawings, inlineImages: inlineImages, inlineByPicLocation: inlineByPicLocation, oleObjects: oleObjects, oleByCP: oleByCP}
}

// NewDrawingCursorWithPictureData resolves fixed pictures lazily from the exact
// sprmCPicLocation carried by each emitted run. PICF decoding remains owned by
// the canonical picture decoder; this cursor only consumes its semantic state.
// NewCompatibilityDrawingCursorWithPictureData is the canonical projection used
// by the legacy public DOC adapter. Old/synthetic Word97 producers do not always
// carry sprmCPicLocation on 0x01 anchors, so this mode preserves the mature
// adapter semantics without weakening the normal OpenReaderAt pipeline.
func NewCompatibilityDrawingCursorWithPictureData(inner Cursor, drawings map[int]Drawing, inlineImages []*wordir.Image, pictureData []byte, oleObjects []wordir.Run, oleByCP map[int]*wordir.Run) Cursor {
	if inner == nil {
		return nil
	}
	return &drawingCursor{
		inner:                inner,
		drawings:             drawings,
		inlineImages:         inlineImages,
		inlineByPicLocation:  make(map[int]*wordir.Image),
		pictureData:          pictureData,
		oleObjects:           oleObjects,
		oleByCP:              oleByCP,
		compatibilityAnchors: true,
	}
}

func NewDrawingCursorWithPictureData(inner Cursor, drawings map[int]Drawing, inlineImages []*wordir.Image, pictureData []byte, oleObjects []wordir.Run, oleByCP map[int]*wordir.Run) Cursor {
	if inner == nil {
		return nil
	}
	return &drawingCursor{
		inner:               inner,
		drawings:            drawings,
		inlineImages:        inlineImages,
		inlineByPicLocation: make(map[int]*wordir.Image),
		pictureData:         pictureData,
		oleObjects:          oleObjects,
		oleByCP:             oleByCP,
	}
}

func (c *drawingCursor) Next() (Run, bool, error) {
	for {
		if len(c.queue) > 0 {
			r := c.queue[0]
			c.queue = c.queue[1:]
			return r, true, nil
		}
		if c.pendingErr != nil {
			err := c.pendingErr
			c.pendingErr = nil
			return Run{}, false, err
		}
		run, ok, err := c.inner.Next()
		if err != nil || !ok {
			return run, ok, err
		}
		c.expand(run)
	}
}

func (c *drawingCursor) nextInlineImage() (*wordir.Image, bool) {
	if c.inlineIndex >= len(c.inlineImages) {
		return nil, false
	}
	image := c.inlineImages[c.inlineIndex]
	c.inlineIndex++
	return image, image != nil
}

func (c *drawingCursor) inlinePicture(run Run) (*wordir.Image, inlinePictureState) {
	if !run.HasPicLocation {
		if image, ok := c.nextInlineImage(); ok {
			return image, inlinePictureRenderable
		}
		return nil, inlinePictureInvalid
	}

	if image := c.inlineByPicLocation[run.PicLocation]; image != nil {
		return image, inlinePictureRenderable
	}

	state := inlinePictureInvalid
	if len(c.pictureData) > 0 {
		if image, resolvedState := resolveInlinePicture(c.pictureData, run.PicLocation); image != nil && resolvedState == inlinePictureRenderable {
			if c.inlineByPicLocation == nil {
				c.inlineByPicLocation = make(map[int]*wordir.Image)
			}
			c.inlineByPicLocation[run.PicLocation] = image
			return image, inlinePictureRenderable
		} else {
			state = resolvedState
		}
	}

	// Some valid Word97 producers keep only a recognized PICF at fcPic while
	// storing renderable media elsewhere in the same Data stream. The canonical
	// snapshot scans that stream in source order, so use it only after exact
	// fcPic resolution has failed.
	if image, ok := c.nextInlineImage(); ok {
		if c.inlineByPicLocation == nil {
			c.inlineByPicLocation = make(map[int]*wordir.Image)
		}
		c.inlineByPicLocation[run.PicLocation] = image
		return image, inlinePictureRenderable
	}
	return nil, state
}

func (c *drawingCursor) expand(run Run) {
	if run.Kind != "" || run.Text == "" {
		c.queue = append(c.queue, run)
		return
	}
	runes := []rune(run.Text)
	segmentStart := 0
	segmentCP := run.StartCP
	handled := false
	for i, ch := range runes {
		cp := run.StartCP + i
		var resolved Drawing
		var ok bool
		kind := ""
		switch {
		case ch == '\x08' && run.Spec:
			resolved, ok = c.drawings[cp]
			if !ok {
				// fSpec 0x08 is an invisible object anchor marker. If OfficeArt
				// resolution cannot map it, consume the marker but preserve text.
				handled = true
				if i > segmentStart {
					c.queue = append(c.queue, cloneRunWithText(run, string(runes[segmentStart:i]), segmentCP))
				}
				segmentStart = i + 1
				segmentCP = cp + 1
				continue
			}
		case ch == '\x01' && run.Obj:
			obj := wordir.Run{Kind: "olePlaceholder", Text: "Embedded Object", OLEMIME: "application/octet-stream", OLEType: "Embed"}
			if c.oleIndex < len(c.oleObjects) {
				obj = c.oleObjects[c.oleIndex]
				c.oleIndex++
			}
			copyObj := obj
			if c.oleByCP != nil {
				c.oleByCP[cp] = &copyObj
			}
			ok = true
			kind = "olePlaceholder"
		case ch == '\x01' && (c.compatibilityAnchors || (!run.Obj && run.Spec && !run.Data && run.HasPicLocation)):
			image, state := c.inlinePicture(run)
			switch state {
			case inlinePictureRenderable:
				resolved = Drawing{Image: image}
				ok = true
			case inlinePicturePlaceholder:
				if c.compatibilityAnchors {
					// Mature public-DOC behavior: an unresolved compatibility anchor
					// is not silently consumed; it is an embedded-object placeholder.
					ok = true
					kind = "olePlaceholder"
					resolved = Drawing{}
					break
				}
				handled = true
				if i > segmentStart {
					c.queue = append(c.queue, cloneRunWithText(run, string(runes[segmentStart:i]), segmentCP))
				}
				segmentStart = i + 1
				segmentCP = cp + 1
				continue
			default:
				if c.compatibilityAnchors {
					// Preserve the mature public-DOC adapter behavior: an unresolved
					// compatibility 0x01 anchor is an embedded-object placeholder.
					// The strict canonical streaming path still reports unsupported
					// picture/data anchors instead of guessing.
					ok = true
					kind = "olePlaceholder"
					break
				}
				c.pendingErr = UnsupportedError{Feature: "picture/data anchor"}
				return
			}
		default:
			// fData is not a Word97 picture discriminator. It is used by other
			// data-bearing constructs (notably fields/forms), so leave those runs
			// untouched for later semantic stages. Any unclaimed 0x01 is removed
			// by the final control-text normalizer rather than becoming visible.
			continue
		}
		if !ok {
			continue
		}
		handled = true
		if i > segmentStart {
			c.queue = append(c.queue, cloneRunWithText(run, string(runes[segmentStart:i]), segmentCP))
		}
		if kind == "" {
			kind = "shape"
			if resolved.Image != nil {
				kind = "image"
			}
		}
		emitted := Run{Kind: kind, Image: resolved.Image, Shape: resolved.Shape, Format: run.Format, ParagraphFormat: run.ParagraphFormat, StartCP: cp, Revision: run.Revision}
		if kind == "olePlaceholder" {
			obj := wordir.Run{Kind: "olePlaceholder", Text: "Embedded Object", OLEMIME: "application/octet-stream", OLEType: "Embed"}
			emitted.EmbeddedRun = &obj
		}
		c.queue = append(c.queue, emitted)
		segmentStart = i + 1
		segmentCP = cp + 1
	}
	if !handled {
		c.queue = append(c.queue, run)
		return
	}
	if segmentStart < len(runes) {
		c.queue = append(c.queue, cloneRunWithText(run, string(runes[segmentStart:]), segmentCP))
	}
}
