package edit

import (
	presir "office/pkg/presentation/ir"
)

// Shape wraps an IR shape for editing.
type Shape struct {
	slide *Slide
	ir    *presir.Shape
}

// Kind returns the shape kind.
func (sh *Shape) Kind() presir.ShapeKind {
	if sh == nil || sh.ir == nil {
		return presir.ShapeOther
	}
	return sh.ir.Kind
}

// Name returns the shape name.
func (sh *Shape) Name() string {
	if sh == nil || sh.ir == nil {
		return ""
	}
	return sh.ir.Name
}

// SetName sets the shape name.
func (sh *Shape) SetName(name string) {
	if sh == nil || sh.ir == nil {
		return
	}
	sh.ir.Name = name
}

// ID returns the non-visual shape id.
func (sh *Shape) ID() int {
	if sh == nil || sh.ir == nil {
		return 0
	}
	return sh.ir.ID
}

// SetID sets the non-visual shape id.
func (sh *Shape) SetID(id int) {
	if sh == nil || sh.ir == nil {
		return
	}
	sh.ir.ID = id
}

// Bounds returns the shape bounds in EMU.
func (sh *Shape) Bounds() (x, y, w, h int64) {
	if sh == nil || sh.ir == nil {
		return 0, 0, 0, 0
	}
	return sh.ir.Bounds.X, sh.ir.Bounds.Y, sh.ir.Bounds.W, sh.ir.Bounds.H
}

// SetBounds sets the shape bounds in EMU.
func (sh *Shape) SetBounds(x, y, w, h int64) {
	if sh == nil || sh.ir == nil {
		return
	}
	sh.ir.Bounds = presir.Rect{X: x, Y: y, W: w, H: h}
}

// SetFillSolid sets a solid fill color as a 6-digit hex string.
func (sh *Shape) SetFillSolid(color string) {
	if sh == nil || sh.ir == nil {
		return
	}
	sh.ir.FillColor = color
}

// FillColor returns the solid fill color hex string.
func (sh *Shape) FillColor() string {
	if sh == nil || sh.ir == nil {
		return ""
	}
	return sh.ir.FillColor
}

// SetLine sets the shape outline color and width in points.
func (sh *Shape) SetLine(color string, widthPt float64) {
	if sh == nil || sh.ir == nil {
		return
	}
	sh.ir.LineColor = color
	sh.ir.LineWidthPt = widthPt
}

// Line returns the outline color and width in points.
func (sh *Shape) Line() (color string, widthPt float64) {
	if sh == nil || sh.ir == nil {
		return "", 0
	}
	return sh.ir.LineColor, sh.ir.LineWidthPt
}

// Paragraphs returns all paragraphs in the shape.
func (sh *Shape) Paragraphs() []*Paragraph {
	if sh == nil || sh.ir == nil {
		return nil
	}
	out := make([]*Paragraph, len(sh.ir.Paragraphs))
	for i, p := range sh.ir.Paragraphs {
		out[i] = &Paragraph{shape: sh, ir: p}
	}
	return out
}

// AddParagraph adds a new paragraph to the shape and returns it.
func (sh *Shape) AddParagraph() *Paragraph {
	if sh == nil || sh.ir == nil {
		return nil
	}
	p := &presir.Paragraph{Runs: []*presir.Run{}}
	sh.ir.Paragraphs = append(sh.ir.Paragraphs, p)
	return &Paragraph{shape: sh, ir: p}
}

// AddText is a convenience helper that adds a single paragraph with one run.
func (sh *Shape) AddText(text string) *Paragraph {
	p := sh.AddParagraph()
	if p == nil {
		return nil
	}
	p.AddRun(text)
	return p
}

// SetText replaces all paragraphs with a single paragraph containing one run.
func (sh *Shape) SetText(text string) {
	if sh == nil || sh.ir == nil {
		return
	}
	sh.ir.Paragraphs = []*presir.Paragraph{{
		Runs: []*presir.Run{{Text: text}},
	}}
}

// Text returns the concatenated plain text of all paragraphs.
func (sh *Shape) Text() string {
	if sh == nil || sh.ir == nil {
		return ""
	}
	var out string
	for _, p := range sh.ir.Paragraphs {
		for _, r := range p.Runs {
			out += r.Text
		}
	}
	return out
}

// SetOLEObject updates the embedded OLE or package payload on this shape.
func (sh *Shape) SetOLEObject(data []byte, mimeType, progID, objectName string) {
	if sh == nil || sh.ir == nil {
		return
	}
	sh.ir.Kind = presir.ShapeOleObject
	sh.ir.OLEObjectData = append([]byte(nil), data...)
	sh.ir.OLEObjectMIME = mimeType
	sh.ir.OLEObjectLinkURL = ""
	sh.ir.OLEObjectProgID = progID
	sh.ir.OLEObjectName = objectName
	if sh.ir.OLEObjectName == "" {
		sh.ir.OLEObjectName = "Object"
	}
}

// SetOLEObjectLink converts this shape to an externally linked OLE object.
func (sh *Shape) SetOLEObjectLink(url, progID, objectName string) {
	if sh == nil || sh.ir == nil {
		return
	}
	sh.ir.Kind = presir.ShapeOleObject
	sh.ir.OLEObjectData = nil
	sh.ir.OLEObjectMIME = ""
	sh.ir.OLEObjectLinkURL = url
	sh.ir.OLEObjectProgID = progID
	sh.ir.OLEObjectName = objectName
	if sh.ir.OLEObjectName == "" {
		sh.ir.OLEObjectName = "Object"
	}
}

// OLEObjectLinkURL returns the external target of a linked OLE object.
func (sh *Shape) OLEObjectLinkURL() string {
	if sh == nil || sh.ir == nil {
		return ""
	}
	return sh.ir.OLEObjectLinkURL
}

// OLEObjectData returns a copy of the embedded OLE or package payload.
func (sh *Shape) OLEObjectData() []byte {
	if sh == nil || sh.ir == nil {
		return nil
	}
	return append([]byte(nil), sh.ir.OLEObjectData...)
}

// OLEObjectMIME returns the content type of the embedded OLE or package payload.
func (sh *Shape) OLEObjectMIME() string {
	if sh == nil || sh.ir == nil {
		return ""
	}
	return sh.ir.OLEObjectMIME
}

// OLEObjectName returns the display name of the embedded OLE object.
func (sh *Shape) OLEObjectName() string {
	if sh == nil || sh.ir == nil {
		return ""
	}
	return sh.ir.OLEObjectName
}

// OLEObjectProgID returns the OLE ProgID (for example, Word.Document.12).
func (sh *Shape) OLEObjectProgID() string {
	if sh == nil || sh.ir == nil {
		return ""
	}
	return sh.ir.OLEObjectProgID
}

// SetOLEIcon sets or replaces the icon image used for this embedded object.
func (sh *Shape) SetOLEIcon(data []byte, mimeType string) {
	if sh == nil || sh.ir == nil {
		return
	}
	sh.ir.OLEIconData = append([]byte(nil), data...)
	sh.ir.OLEIconMIME = mimeType
}

// OLEIcon returns a copy of the embedded object's icon image and its content type.
func (sh *Shape) OLEIcon() ([]byte, string) {
	if sh == nil || sh.ir == nil {
		return nil, ""
	}
	return append([]byte(nil), sh.ir.OLEIconData...), sh.ir.OLEIconMIME
}

// ClearOLEIcon removes the icon image for this embedded object.
func (sh *Shape) ClearOLEIcon() {
	if sh == nil || sh.ir == nil {
		return
	}
	sh.ir.OLEIconData = nil
	sh.ir.OLEIconMIME = ""
}

// SetRotation sets the clockwise rotation in degrees.
func (sh *Shape) SetRotation(deg float64) {
	if sh == nil || sh.ir == nil {
		return
	}
	sh.ir.RotationDeg = deg
}

// Rotation returns the clockwise rotation in degrees.
func (sh *Shape) Rotation() float64 {
	if sh == nil || sh.ir == nil {
		return 0
	}
	return sh.ir.RotationDeg
}

// SetPlaceholder sets the placeholder kind for the shape.
func (sh *Shape) SetPlaceholder(kind presir.PlaceholderKind) {
	if sh == nil || sh.ir == nil {
		return
	}
	sh.ir.Placeholder = kind
}

// Placeholder returns the placeholder kind, or PlaceholderNone if not a placeholder.
func (sh *Shape) Placeholder() presir.PlaceholderKind {
	if sh == nil || sh.ir == nil {
		return presir.PlaceholderNone
	}
	return sh.ir.Placeholder
}

// PlaceholderIdx returns the placeholder index used to match placeholders
// across master, layout, and slide.
func (sh *Shape) PlaceholderIdx() int {
	if sh == nil || sh.ir == nil {
		return 0
	}
	return sh.ir.PlaceholderIdx
}

// SetPlaceholderIdx sets the placeholder index.
func (sh *Shape) SetPlaceholderIdx(idx int) {
	if sh == nil || sh.ir == nil {
		return
	}
	sh.ir.PlaceholderIdx = idx
}

// PlaceholderOrient returns the placeholder orientation ("horz" or "vert").
func (sh *Shape) PlaceholderOrient() string {
	if sh == nil || sh.ir == nil {
		return ""
	}
	return sh.ir.PlaceholderOrient
}

// SetPlaceholderOrient sets the placeholder orientation.
func (sh *Shape) SetPlaceholderOrient(orient string) {
	if sh == nil || sh.ir == nil {
		return
	}
	sh.ir.PlaceholderOrient = orient
}

// PlaceholderSize returns the placeholder size ("full", "half", or "quarter").
func (sh *Shape) PlaceholderSize() string {
	if sh == nil || sh.ir == nil {
		return ""
	}
	return sh.ir.PlaceholderSize
}

// SetPlaceholderSize sets the placeholder size.
func (sh *Shape) SetPlaceholderSize(size string) {
	if sh == nil || sh.ir == nil {
		return
	}
	sh.ir.PlaceholderSize = size
}

// PlaceholderHasCustomPrompt reports whether the placeholder shows a custom
// prompt instead of the default.
func (sh *Shape) PlaceholderHasCustomPrompt() bool {
	if sh == nil || sh.ir == nil {
		return false
	}
	return sh.ir.PlaceholderHasCustomPrompt
}

// SetPlaceholderHasCustomPrompt sets the custom prompt flag.
func (sh *Shape) SetPlaceholderHasCustomPrompt(v bool) {
	if sh == nil || sh.ir == nil {
		return
	}
	sh.ir.PlaceholderHasCustomPrompt = v
}

// SetAnimationStep sets the 1-based click step at which this shape enters.
// Use 0 to make the shape always visible (no entrance animation).
func (sh *Shape) SetAnimationStep(step int) {
	if sh == nil || sh.ir == nil {
		return
	}
	sh.ir.AnimStep = step
}

// AnimationStep returns the shape's entrance animation click step, or 0 if none.
func (sh *Shape) AnimationStep() int {
	if sh == nil || sh.ir == nil {
		return 0
	}
	return sh.ir.AnimStep
}

// AddEmphasisEffect appends an emphasis animation effect name to this shape.
// Supported values mirror common POI/OOXML emphasis types such as "spin",
// "growShrink", "pulse", and "colorPulse".
func (sh *Shape) AddEmphasisEffect(effect string) {
	if sh == nil || sh.ir == nil || effect == "" {
		return
	}
	sh.ir.EmphasisEffects = append(sh.ir.EmphasisEffects, effect)
}

// EmphasisEffects returns a copy of the emphasis animation effect names for
// this shape.
func (sh *Shape) EmphasisEffects() []string {
	if sh == nil || sh.ir == nil {
		return nil
	}
	return append([]string(nil), sh.ir.EmphasisEffects...)
}

// SetEmphasisEffects replaces all emphasis animation effect names for this
// shape.
func (sh *Shape) SetEmphasisEffects(effects []string) {
	if sh == nil || sh.ir == nil {
		return
	}
	sh.ir.EmphasisEffects = append([]string(nil), effects...)
}

// ClearEmphasisEffects removes all emphasis animation effects from this shape.
func (sh *Shape) ClearEmphasisEffects() {
	if sh == nil || sh.ir == nil {
		return
	}
	sh.ir.EmphasisEffects = nil
}

// SetMotionPath sets a motion path animation for this shape using SVG-like
// path data (e.g. "M 0 0 L 100000 0"). A default duration of 500 ms and
// origin "layout" are used.
func (sh *Shape) SetMotionPath(path string) {
	sh.SetMotionPathWithOptions(path, 500, "layout")
}

// SetMotionPathWithDur sets a motion path animation with a custom duration in
// milliseconds.
func (sh *Shape) SetMotionPathWithDur(path string, durMs int) {
	sh.SetMotionPathWithOptions(path, durMs, "layout")
}

// SetMotionPathWithOptions sets a motion path animation with a custom duration
// and origin. Origin should be "parent" or "layout".
func (sh *Shape) SetMotionPathWithOptions(path string, durMs int, origin string) {
	if sh == nil || sh.ir == nil || path == "" {
		return
	}
	if origin == "" {
		origin = "layout"
	}
	sh.ir.MotionPath = &presir.AnimMotionPath{
		Path:   path,
		Dur:    durMs,
		Origin: origin,
	}
}

// MotionPath returns the motion path animation details for this shape. The
// final boolean is false when no motion path is set.
func (sh *Shape) MotionPath() (path string, durMs int, origin string, ok bool) {
	if sh == nil || sh.ir == nil || sh.ir.MotionPath == nil {
		return "", 0, "", false
	}
	mp := sh.ir.MotionPath
	return mp.Path, mp.Dur, mp.Origin, true
}

// ClearMotionPath removes the motion path animation from this shape.
func (sh *Shape) ClearMotionPath() {
	if sh == nil || sh.ir == nil {
		return
	}
	sh.ir.MotionPath = nil
}

// AddAnimationTrigger appends a trigger event name for this shape's animation.
// Common values are "onClick" and "onHover".
func (sh *Shape) AddAnimationTrigger(event string) {
	if sh == nil || sh.ir == nil || event == "" {
		return
	}
	sh.ir.AnimationTriggers = append(sh.ir.AnimationTriggers, event)
}

// AnimationTriggers returns a copy of the animation trigger events for this
// shape.
func (sh *Shape) AnimationTriggers() []string {
	if sh == nil || sh.ir == nil {
		return nil
	}
	return append([]string(nil), sh.ir.AnimationTriggers...)
}

// SetAnimationTriggers replaces all animation trigger events for this shape.
func (sh *Shape) SetAnimationTriggers(events []string) {
	if sh == nil || sh.ir == nil {
		return
	}
	sh.ir.AnimationTriggers = append([]string(nil), events...)
}

// ClearAnimationTriggers removes all animation triggers from this shape.
func (sh *Shape) ClearAnimationTriggers() {
	if sh == nil || sh.ir == nil {
		return
	}
	sh.ir.AnimationTriggers = nil
}

// SetFlip sets horizontal and vertical flipping.
func (sh *Shape) SetFlip(h, v bool) {
	if sh == nil || sh.ir == nil {
		return
	}
	sh.ir.FlipH = h
	sh.ir.FlipV = v
}

// Group returns the group editor if the shape is a group, otherwise nil.
func (sh *Shape) Group() *Group {
	if sh == nil || sh.ir == nil || sh.ir.Kind != presir.ShapeGroup {
		return nil
	}
	return &Group{shape: sh, ir: sh.ir}
}

// Table returns the table data if the shape is a table, otherwise nil.
func (sh *Shape) Table() *Table {
	if sh == nil || sh.ir == nil || sh.ir.Table == nil {
		return nil
	}
	return &Table{shape: sh, ir: sh.ir.Table}
}

// SetConnectorType sets the connector geometry preset (e.g. "straight", "bent", "curved").
func (sh *Shape) SetConnectorType(connectorType string) {
	if sh == nil || sh.ir == nil {
		return
	}
	sh.ir.ConnectorType = connectorType
}

// ConnectorType returns the connector geometry preset.
func (sh *Shape) ConnectorType() string {
	if sh == nil || sh.ir == nil {
		return ""
	}
	return sh.ir.ConnectorType
}

// SetArrowHead sets the arrowhead style for connectors ("none", "triangle", "arrow", "stealth").
func (sh *Shape) SetArrowHead(style string) {
	if sh == nil || sh.ir == nil {
		return
	}
	sh.ir.ArrowHead = style
}

// ArrowHead returns the connector arrowhead style.
func (sh *Shape) ArrowHead() string {
	if sh == nil || sh.ir == nil {
		return ""
	}
	return sh.ir.ArrowHead
}

// SetArrowTail sets the arrow tail style for connectors.
func (sh *Shape) SetArrowTail(style string) {
	if sh == nil || sh.ir == nil {
		return
	}
	sh.ir.ArrowTail = style
}

// ArrowTail returns the connector arrow tail style.
func (sh *Shape) ArrowTail() string {
	if sh == nil || sh.ir == nil {
		return ""
	}
	return sh.ir.ArrowTail
}

// SetHyperlinkURL sets an external URL that is navigated to when the shape is
// clicked. Pass an empty string to remove the hyperlink.
func (sh *Shape) SetHyperlinkURL(url string) {
	if sh == nil || sh.ir == nil {
		return
	}
	sh.ir.HyperlinkURL = url
	if url != "" {
		sh.ir.ActionLink = ""
	}
}

// HyperlinkURL returns the shape's external hyperlink URL, if any.
func (sh *Shape) HyperlinkURL() string {
	if sh == nil || sh.ir == nil {
		return ""
	}
	return sh.ir.HyperlinkURL
}

// SetActionLink sets a built-in navigation target for the shape. Supported
// values mirror POI conventions: "firstSlide", "lastSlide", "nextSlide",
// "prevSlide", "endShow", or "slideN" where N is a 1-based slide number.
// Pass an empty string to remove the action.
func (sh *Shape) SetActionLink(action string) {
	if sh == nil || sh.ir == nil {
		return
	}
	sh.ir.ActionLink = action
	if action != "" {
		sh.ir.HyperlinkURL = ""
	}
}

// ActionLink returns the shape's built-in navigation target, if any.
func (sh *Shape) ActionLink() string {
	if sh == nil || sh.ir == nil {
		return ""
	}
	return sh.ir.ActionLink
}

// RemoveHyperlink clears both external URL and action link from the shape.
func (sh *Shape) RemoveHyperlink() {
	if sh == nil || sh.ir == nil {
		return
	}
	sh.ir.HyperlinkURL = ""
	sh.ir.ActionLink = ""
}

// Clone returns a deep copy of this shape. The returned shape is not attached
// to any slide or group; use Slide.DuplicateShape or Group.DuplicateShape to
// add it to a container.
func (sh *Shape) Clone() *Shape {
	if sh == nil || sh.ir == nil {
		return nil
	}
	return &Shape{ir: cloneShape(sh.ir)}
}
