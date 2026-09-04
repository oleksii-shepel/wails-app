package edit

import (
	"fmt"

	presir "office/pkg/presentation/ir"
)

// Slide wraps an IR slide for editing.
type Slide struct {
	pres  *Presentation
	ir    *presir.Slide
	index int
}

// Index returns the 0-based slide position.
func (s *Slide) Index() int {
	if s == nil {
		return -1
	}
	return s.ir.Index
}

// Name returns the slide's section/name, if any.
func (s *Slide) Name() string {
	if s == nil || s.ir == nil {
		return ""
	}
	return s.ir.SectionName
}

// SetName sets the slide's section/name.
func (s *Slide) SetName(name string) {
	if s == nil || s.ir == nil {
		return
	}
	s.ir.SectionName = name
}

// LayoutType returns the slide layout type.
func (s *Slide) LayoutType() string {
	if s == nil || s.ir == nil {
		return ""
	}
	return s.ir.LayoutType
}

// SetLayoutType sets the slide layout type.
func (s *Slide) SetLayoutType(layoutType string) {
	if s == nil || s.ir == nil {
		return
	}
	s.ir.LayoutType = layoutType
}

// SetLayout assigns the slide to a specific layout.
func (s *Slide) SetLayout(layout *SlideLayout) {
	if s == nil || s.ir == nil || layout == nil || layout.ir == nil {
		return
	}
	s.ir.Layout = layout.ir
	s.ir.LayoutType = layout.ir.Type
	if layout.master != nil && layout.master.ir != nil {
		s.ir.Master = layout.master.ir
	}
}

// Layout returns the layout assigned to this slide, if any.
func (s *Slide) Layout() *SlideLayout {
	if s == nil || s.ir == nil || s.ir.Layout == nil {
		return nil
	}
	return &SlideLayout{ir: s.ir.Layout}
}

// SetMaster assigns the slide to a specific master.
func (s *Slide) SetMaster(master *SlideMaster) {
	if s == nil || s.ir == nil || master == nil || master.ir == nil {
		return
	}
	s.ir.Master = master.ir
}

// Master returns the master assigned to this slide, if any.
func (s *Slide) Master() *SlideMaster {
	if s == nil || s.ir == nil || s.ir.Master == nil {
		return nil
	}
	return &SlideMaster{ir: s.ir.Master}
}

// Shapes returns all shapes on the slide.
func (s *Slide) Shapes() []*Shape {
	if s == nil || s.ir == nil {
		return nil
	}
	out := make([]*Shape, len(s.ir.Shapes))
	for i, sh := range s.ir.Shapes {
		out[i] = &Shape{slide: s, ir: sh}
	}
	return out
}

// AddTextBox adds a new text box shape and returns it.
func (s *Slide) AddTextBox(x, y, w, h int64) *Shape {
	return s.addShape(presir.ShapeText, x, y, w, h)
}

// AddShape adds a new shape with the given geometry and returns it.
func (s *Slide) AddShape(geometry string, x, y, w, h int64) *Shape {
	shape := s.addShape(presir.ShapeText, x, y, w, h)
	if shape != nil {
		shape.ir.Geometry = geometry
	}
	return shape
}

// AddImage adds an image from raw bytes and returns the image shape.
func (s *Slide) AddImage(data []byte, mimeType string, x, y, w, h int64) *Shape {
	if s == nil || s.ir == nil {
		return nil
	}
	shape := &presir.Shape{
		Kind:      presir.ShapeImage,
		Name:      nextShapeName(s.ir.Shapes, "Picture"),
		Bounds:    presir.Rect{X: x, Y: y, W: w, H: h},
		ImageData: data,
		ImageMIME: mimeType,
	}
	s.ir.Shapes = append(s.ir.Shapes, shape)
	return &Shape{slide: s, ir: shape}
}

// AddImageFromFile reads an image file and adds it to the slide.
func (s *Slide) AddImageFromFile(path string, x, y, w, h int64) (*Shape, error) {
	data, mimeType, err := readImageFile(path)
	if err != nil {
		return nil, err
	}
	return s.AddImage(data, mimeType, x, y, w, h), nil
}

// AddTable adds a table with the given number of rows and columns.
func (s *Slide) AddTable(rows, cols int, x, y, w, h int64) *Table {
	if s == nil || s.ir == nil || rows <= 0 || cols <= 0 {
		return nil
	}
	colWidth := w / int64(cols)
	rowHeight := h / int64(rows)

	table := &presir.Table{
		ColumnWidthsEMU: make([]int64, cols),
		Rows:            make([]*presir.TableRow, rows),
	}
	for c := 0; c < cols; c++ {
		table.ColumnWidthsEMU[c] = colWidth
	}
	for r := 0; r < rows; r++ {
		row := &presir.TableRow{
			HeightEMU: rowHeight,
			Cells:     make([]*presir.TableCell, cols),
		}
		for c := 0; c < cols; c++ {
			row.Cells[c] = &presir.TableCell{
				Paragraphs: []*presir.Paragraph{},
			}
		}
		table.Rows[r] = row
	}

	shape := &presir.Shape{
		Kind:   presir.ShapeTable,
		Name:   nextShapeName(s.ir.Shapes, "Table"),
		Bounds: presir.Rect{X: x, Y: y, W: w, H: h},
		Table:  table,
	}
	s.ir.Shapes = append(s.ir.Shapes, shape)
	return &Table{shape: &Shape{slide: s, ir: shape}, ir: table}
}

// AddPlaceholder adds a placeholder shape with the given kind and returns it.
func (s *Slide) AddPlaceholder(kind presir.PlaceholderKind, x, y, w, h int64) *Shape {
	shape := s.addShape(presir.ShapeText, x, y, w, h)
	if shape == nil {
		return nil
	}
	shape.ir.Placeholder = kind
	shape.ir.Name = placeholderDefaultName(kind)
	return shape
}

// AddSmartArt adds a SmartArt diagram to the slide and returns it.
func (s *Slide) AddSmartArt(art *SmartArt) *SmartArt {
	if s == nil || s.ir == nil || art == nil || art.ir == nil {
		return nil
	}
	art.slide = s
	s.ir.SmartArt = append(s.ir.SmartArt, art.ir)
	return art
}

// SmartArts returns editable SmartArt diagrams on the slide.
func (s *Slide) SmartArts() []*SmartArt {
	if s == nil || s.ir == nil {
		return nil
	}
	out := make([]*SmartArt, len(s.ir.SmartArt))
	for i, art := range s.ir.SmartArt {
		out[i] = &SmartArt{slide: s, ir: art}
	}
	return out
}

// AddMedia adds an audio or video shape and returns it.
func (s *Slide) AddMedia(data []byte, mimeType, kind string, x, y, w, h int64) *Shape {
	if s == nil || s.ir == nil {
		return nil
	}
	shape := &presir.Shape{
		Kind:      presir.ShapeMedia,
		Name:      nextShapeName(s.ir.Shapes, "Media"),
		Bounds:    presir.Rect{X: x, Y: y, W: w, H: h},
		MediaData: data,
		MediaMIME: mimeType,
		MediaKind: kind,
	}
	s.ir.Shapes = append(s.ir.Shapes, shape)
	return &Shape{slide: s, ir: shape}
}

// AddOLEObject adds an embedded OLE or package object and returns the shape.
func (s *Slide) AddOLEObject(data []byte, mimeType, progID, objectName string, x, y, w, h int64) *Shape {
	if s == nil || s.ir == nil {
		return nil
	}
	shape := &presir.Shape{
		Kind:            presir.ShapeOleObject,
		Name:            nextShapeName(s.ir.Shapes, "Object"),
		Bounds:          presir.Rect{X: x, Y: y, W: w, H: h},
		OLEObjectData:   append([]byte(nil), data...),
		OLEObjectMIME:   mimeType,
		OLEObjectProgID: progID,
		OLEObjectName:   objectName,
	}
	if shape.OLEObjectName == "" {
		shape.OLEObjectName = "Object"
	}
	s.ir.Shapes = append(s.ir.Shapes, shape)
	return &Shape{slide: s, ir: shape}
}

// AddLinkedOLEObject adds an externally linked OLE object and returns the shape.
func (s *Slide) AddLinkedOLEObject(url, progID, objectName string, x, y, w, h int64) *Shape {
	if s == nil || s.ir == nil {
		return nil
	}
	shape := &presir.Shape{
		Kind:             presir.ShapeOleObject,
		Name:             nextShapeName(s.ir.Shapes, "Object"),
		Bounds:           presir.Rect{X: x, Y: y, W: w, H: h},
		OLEObjectProgID:  progID,
		OLEObjectName:    objectName,
		OLEObjectLinkURL: url,
	}
	if shape.OLEObjectName == "" {
		shape.OLEObjectName = "Object"
	}
	s.ir.Shapes = append(s.ir.Shapes, shape)
	return &Shape{slide: s, ir: shape}
}

// AddChart adds a chart shape from raw chart XML and returns it.
func (s *Slide) AddChart(chartData []byte, x, y, w, h int64) *Shape {
	if s == nil || s.ir == nil {
		return nil
	}
	shape := &presir.Shape{
		Kind:      presir.ShapeChart,
		Name:      nextShapeName(s.ir.Shapes, "Chart"),
		Bounds:    presir.Rect{X: x, Y: y, W: w, H: h},
		ChartData: chartData,
	}
	s.ir.Shapes = append(s.ir.Shapes, shape)
	return &Shape{slide: s, ir: shape}
}

// AddConnector adds a connector line shape and returns it.
func (s *Slide) AddConnector(x, y, w, h int64) *Shape {
	if s == nil || s.ir == nil {
		return nil
	}
	shape := &presir.Shape{
		Kind:        presir.ShapeConnector,
		Name:        nextShapeName(s.ir.Shapes, "Connector"),
		Bounds:      presir.Rect{X: x, Y: y, W: w, H: h},
		LineColor:   "000000",
		LineWidthPt: 1,
	}
	s.ir.Shapes = append(s.ir.Shapes, shape)
	return &Shape{slide: s, ir: shape}
}

// AddGroup adds a new group shape and returns it.
func (s *Slide) AddGroup(x, y, w, h int64) *Group {
	if s == nil || s.ir == nil {
		return nil
	}
	shape := &presir.Shape{
		Kind:   presir.ShapeGroup,
		Name:   nextShapeName(s.ir.Shapes, "Group"),
		Bounds: presir.Rect{X: x, Y: y, W: w, H: h},
	}
	s.ir.Shapes = append(s.ir.Shapes, shape)
	return &Group{shape: &Shape{slide: s, ir: shape}, ir: shape}
}

// SetTransition sets the slide transition effect.
func (s *Slide) SetTransition(t *presir.SlideTransition) {
	if s == nil || s.ir == nil {
		return
	}
	s.ir.Transition = t
}

// Transition returns the slide transition effect, if any.
func (s *Slide) Transition() *presir.SlideTransition {
	if s == nil || s.ir == nil {
		return nil
	}
	return s.ir.Transition
}

// RemoveShape removes the given shape from the slide. Returns true if it was
// present and removed.
func (s *Slide) RemoveShape(shape *Shape) bool {
	if s == nil || s.ir == nil || shape == nil || shape.ir == nil {
		return false
	}
	return removeShapeFromList(&s.ir.Shapes, shape.ir)
}

// RemoveShapeByName removes the first direct child shape with the given name.
// Returns true if a shape was removed.
func (s *Slide) RemoveShapeByName(name string) bool {
	if s == nil || s.ir == nil {
		return false
	}
	return removeShapeByNameFromList(&s.ir.Shapes, name)
}

// DuplicateShape creates a deep copy of shape and appends it to the slide. The
// clone is given a fresh name based on the original name to avoid duplicates.
func (s *Slide) DuplicateShape(shape *Shape) *Shape {
	if s == nil || s.ir == nil || shape == nil || shape.ir == nil {
		return nil
	}
	clone := cloneShape(shape.ir)
	clone.Name = uniqueShapeName(s.ir.Shapes, clone.Name)
	s.ir.Shapes = append(s.ir.Shapes, clone)
	return &Shape{slide: s, ir: clone}
}

// CloneSlide returns a deep copy of this slide that is not yet attached to the
// presentation. Use DuplicateSlide or InsertSlide to add it to a deck.
func (s *Slide) CloneSlide() *presir.Slide {
	if s == nil || s.ir == nil {
		return nil
	}
	return cloneSlide(s.ir)
}

// AddComment adds a comment to the slide at the given anchor position in EMU.
func (s *Slide) AddComment(text, authorName, initials string, x, y int64) {
	if s == nil || s.ir == nil {
		return
	}
	s.ir.Comments = append(s.ir.Comments, &presir.Comment{
		AuthorName: authorName,
		Initials:   initials,
		Text:       text,
		X:          x,
		Y:          y,
	})
}

// Comments returns all comments on the slide.
func (s *Slide) Comments() []*presir.Comment {
	if s == nil || s.ir == nil {
		return nil
	}
	return s.ir.Comments
}

// RemoveComment removes a specific comment from the slide.
func (s *Slide) RemoveComment(comment *presir.Comment) bool {
	if s == nil || s.ir == nil || comment == nil {
		return false
	}
	for i, existing := range s.ir.Comments {
		if existing == comment {
			s.ir.Comments = append(s.ir.Comments[:i], s.ir.Comments[i+1:]...)
			return true
		}
	}
	return false
}

// ClearComments removes all comments from the slide.
func (s *Slide) ClearComments() {
	if s == nil || s.ir == nil {
		return
	}
	s.ir.Comments = nil
}

// SetNotesText replaces the slide's notes with a single text box containing text.
// RemoveNotes removes the slide notes entirely.
func (s *Slide) RemoveNotes() {
	if s == nil || s.ir == nil {
		return
	}
	s.ir.Notes = nil
}

func (s *Slide) SetNotesText(text string) {
	if s == nil || s.ir == nil {
		return
	}
	s.ir.Notes = &presir.NotesSlide{
		Shapes: []*presir.Shape{{
			Kind: presir.ShapeText,
			Name: "Notes Text",
			Bounds: presir.Rect{
				X: 0, Y: 0,
				W: s.pres.IR.SlideWidthEMU,
				H: s.pres.IR.SlideHeightEMU,
			},
			Paragraphs: []*presir.Paragraph{{
				Runs: []*presir.Run{{Text: text}},
			}},
		}},
	}
}

// NotesText returns the plain text of the first notes shape, if any.
func (s *Slide) NotesText() string {
	if s == nil || s.ir == nil || s.ir.Notes == nil || len(s.ir.Notes.Shapes) == 0 {
		return ""
	}
	var out string
	for _, p := range s.ir.Notes.Shapes[0].Paragraphs {
		for _, r := range p.Runs {
			out += r.Text
		}
	}
	return out
}

// SetBackgroundColor sets the slide background as a 6-digit hex string.
func (s *Slide) SetBackgroundColor(hex string) {
	if s == nil || s.ir == nil {
		return
	}
	s.ir.BgColor = hex
}

// BackgroundColor returns the slide background color hex string.
func (s *Slide) BackgroundColor() string {
	if s == nil || s.ir == nil {
		return ""
	}
	return s.ir.BgColor
}

func (s *Slide) addShape(kind presir.ShapeKind, x, y, w, h int64) *Shape {
	if s == nil || s.ir == nil {
		return nil
	}
	shape := &presir.Shape{
		Kind:       kind,
		Name:       nextShapeName(s.ir.Shapes, "Shape"),
		Bounds:     presir.Rect{X: x, Y: y, W: w, H: h},
		Paragraphs: []*presir.Paragraph{},
	}
	s.ir.Shapes = append(s.ir.Shapes, shape)
	return &Shape{slide: s, ir: shape}
}

func placeholderDefaultName(kind presir.PlaceholderKind) string {
	switch kind {
	case presir.PlaceholderTitle:
		return "Title 1"
	case presir.PlaceholderBody:
		return "Content Placeholder 2"
	case presir.PlaceholderSubtitle:
		return "Subtitle 2"
	case presir.PlaceholderDateTime:
		return "Date Placeholder 3"
	case presir.PlaceholderSlideNumber:
		return "Slide Number Placeholder 4"
	case presir.PlaceholderFooter:
		return "Footer Placeholder 5"
	case presir.PlaceholderHeader:
		return "Header Placeholder 6"
	case presir.PlaceholderContent:
		return "Content Placeholder 7"
	case presir.PlaceholderChart:
		return "Chart Placeholder 8"
	case presir.PlaceholderTable:
		return "Table Placeholder 9"
	case presir.PlaceholderPicture:
		return "Picture Placeholder 10"
	case presir.PlaceholderMedia:
		return "Media Placeholder 11"
	case presir.PlaceholderClipArt:
		return "ClipArt Placeholder 12"
	case presir.PlaceholderDiagram:
		return "Diagram Placeholder 13"
	case presir.PlaceholderSlideImage:
		return "Slide Image Placeholder 14"
	default:
		return "Text Placeholder"
	}
}

func nextShapeName(shapes []*presir.Shape, prefix string) string {
	max := 0
	for _, sh := range shapes {
		if sh == nil {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(sh.Name, prefix+" %d", &n); err == nil && n > max {
			max = n
		}
	}
	return fmt.Sprintf("%s %d", prefix, max+1)
}

func removeShapeFromList(shapes *[]*presir.Shape, target *presir.Shape) bool {
	for i, sh := range *shapes {
		if sh == target {
			*shapes = append((*shapes)[:i], (*shapes)[i+1:]...)
			return true
		}
	}
	return false
}

func removeShapeByNameFromList(shapes *[]*presir.Shape, name string) bool {
	for i, sh := range *shapes {
		if sh != nil && sh.Name == name {
			*shapes = append((*shapes)[:i], (*shapes)[i+1:]...)
			return true
		}
	}
	return false
}

func uniqueShapeName(shapes []*presir.Shape, name string) string {
	if findShapeInList(shapes, name) == nil {
		return name
	}
	base := name
	// Strip an existing " Copy N" suffix to avoid "Copy Copy" stacking.
	for i := len(base) - 1; i > 0; i-- {
		if base[i] == ' ' {
			maybe := base[i+1:]
			if _, err := fmt.Sscanf(maybe, "%d", new(int)); err == nil {
				base = base[:i]
				break
			}
			break
		}
	}
	for n := 1; ; n++ {
		candidate := fmt.Sprintf("%s Copy %d", base, n)
		if findShapeInList(shapes, candidate) == nil {
			return candidate
		}
	}
}

func cloneSlide(src *presir.Slide) *presir.Slide {
	if src == nil {
		return nil
	}
	return &presir.Slide{
		Index:       src.Index,
		SectionName: src.SectionName,
		MasterID:    src.MasterID,
		LayoutType:  src.LayoutType,
		BgColor:     src.BgColor,
		Shapes:      cloneShapes(src.Shapes),
		Notes:       cloneNotesSlide(src.Notes),
		Header:      cloneShape(src.Header),
		Footer:      cloneShape(src.Footer),
		DateTime:    cloneShape(src.DateTime),
		SlideNumber: cloneShape(src.SlideNumber),
		Transition:  cloneTransition(src.Transition),
		Master:      src.Master,
		Layout:      src.Layout,
		Comments:    cloneComments(src.Comments),
	}
}

func cloneNotesSlide(src *presir.NotesSlide) *presir.NotesSlide {
	if src == nil {
		return nil
	}
	return &presir.NotesSlide{
		BgColor: src.BgColor,
		Shapes:  cloneShapes(src.Shapes),
	}
}

func cloneTransition(src *presir.SlideTransition) *presir.SlideTransition {
	if src == nil {
		return nil
	}
	return &presir.SlideTransition{
		Type:     src.Type,
		Spd:      src.Spd,
		AdvClick: src.AdvClick,
		AdvTmMs:  src.AdvTmMs,
	}
}

func cloneComments(src []*presir.Comment) []*presir.Comment {
	if src == nil {
		return nil
	}
	dst := make([]*presir.Comment, len(src))
	for i, c := range src {
		if c == nil {
			continue
		}
		dst[i] = &presir.Comment{
			AuthorName: c.AuthorName,
			Initials:   c.Initials,
			DateTime:   c.DateTime,
			Text:       c.Text,
			X:          c.X,
			Y:          c.Y,
		}
	}
	return dst
}
