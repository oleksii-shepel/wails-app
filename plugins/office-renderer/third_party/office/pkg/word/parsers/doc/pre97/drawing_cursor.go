package pre97

import wordir "office/pkg/word/ir"

const DrawingAnchorChar = '\x08'

type Drawing struct {
	Image *wordir.Image
	Shape *wordir.Shape
}

type DrawingProjection struct {
	Drawings     map[int]Drawing
	InlineImages []*wordir.Image
	OLEObjects   []wordir.Run
	OLEByCP      map[int]*wordir.Run
}

type drawingCursor struct {
	inner       Cursor
	projection  DrawingProjection
	inlineIndex int
	oleIndex    int
	queue       []Run
	pendingErr  error
}

func NewDrawingCursor(inner Cursor, projection DrawingProjection) Cursor {
	if inner == nil {
		return nil
	}
	return &drawingCursor{inner: inner, projection: projection}
}

func (c *drawingCursor) Next() (Run, bool, error) {
	for {
		if len(c.queue) > 0 {
			run := c.queue[0]
			c.queue = c.queue[1:]
			return run, true, nil
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
		var kind string

		switch {
		case ch == DrawingAnchorChar && run.Spec:
			resolved, ok = c.projection.Drawings[cp]
			if !ok {
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
			if c.oleIndex < len(c.projection.OLEObjects) {
				obj = c.projection.OLEObjects[c.oleIndex]
				c.oleIndex++
			}
			copyObj := obj
			if c.projection.OLEByCP != nil {
				c.projection.OLEByCP[cp] = &copyObj
			}
			ok = true
			kind = "olePlaceholder"
		case ch == '\x01' && run.Data:
			if c.inlineIndex >= len(c.projection.InlineImages) {
				c.pendingErr = UnsupportedError{Feature: "picture/data anchor"}
				return
			}
			resolved = Drawing{Image: c.projection.InlineImages[c.inlineIndex]}
			c.inlineIndex++
			ok = true
		default:
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
		projected := cloneRunWithText(run, "", cp)
		projected.Kind = kind
		projected.Image = resolved.Image
		projected.Shape = resolved.Shape
		projected.Spec = false
		projected.Data = false
		projected.Obj = false
		c.queue = append(c.queue, projected)
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

func cloneRunWithText(run Run, text string, startCP int) Run {
	run.Text = text
	run.StartCP = startCP
	return run
}
