package sax

import (
	"fmt"
	"strings"

	"office/pkg/ooxml/drawing"
	"office/pkg/presentation/ir"
	"office/pkg/render/metafile"
)

// BuildContextualSlidesPath builds slide IR directly from the forward-only SAX
// stream. Package relationships and theme colors are resolved as immutable
// context before each slide's authored visual events are consumed.
func BuildContextualSlidesPath(filePath string, opts Options) ([]*ir.Slide, error) {
	b := newIRBuilder()
	if err := WalkPath(filePath, b, opts); err != nil {
		return nil, err
	}
	return b.slides, nil
}

// BuildContextualSlidesBytes is the in-memory counterpart of BuildContextualSlidesPath.
func BuildContextualSlidesBytes(data []byte, opts Options) ([]*ir.Slide, error) {
	b := newIRBuilder()
	if err := WalkBytes(data, b, opts); err != nil {
		return nil, err
	}
	return b.slides, nil
}

// BuildContextualPresentationBytes builds the presentation-level canonical IR directly
// from SAX events. It is the production-oriented boundary used while the old
// package composer is being dismantled.
func BuildContextualPresentationBytes(data []byte, opts Options) (*ir.Presentation, error) {
	b := newIRBuilder()
	if err := WalkBytes(data, b, opts); err != nil {
		return nil, err
	}
	b.presentation.Slides = b.slides
	b.finalizeCanonicalTheme()
	return b.presentation, nil
}

// BuildAuthoredSlidesPath is retained as a compatibility name while callers
// migrate to the contextual SAX builder.
func BuildAuthoredSlidesPath(filePath string, opts Options) ([]*ir.Slide, error) {
	return BuildContextualSlidesPath(filePath, opts)
}

// BuildAuthoredSlidesBytes is retained as a compatibility name.
func BuildAuthoredSlidesBytes(data []byte, opts Options) ([]*ir.Slide, error) {
	return BuildContextualSlidesBytes(data, opts)
}

type irBuilder struct {
	presentation            *ir.Presentation
	slides                  []*ir.Slide
	slide                   *ir.Slide
	shapes                  []*ir.Shape
	paragraph               *ir.Paragraph
	runProps                *RunProperties
	pendingRun              *ir.Run
	themeColors             map[string]string
	themeLineStyles         map[int]LineProperties
	themeFillStyles         map[int]ThemeFillStyle
	themeEffectStyles       map[int]ThemeEffectStyle
	masterColorMap          map[string]string
	layoutColorMap          map[string]string
	slideColorMap           map[string]string
	activeColorScope        string
	themeName               string
	masterShapes            []*ir.Shape
	layoutShapes            []*ir.Shape
	masterPart              string
	layoutPart              string
	masterTextStyles        map[string]map[int]textStyleLevel
	defaultTextStyles       map[int]textStyleLevel
	themeFonts              map[string]string
	relationships           map[int]map[string]relationshipTarget
	payloads                map[int]map[string]relationshipPayload
	olePreviews             map[int]map[string]relationshipPayload
	partRelationships       map[string]map[string]relationshipTarget
	partPayloads            map[string]map[string]relationshipPayload
	tableRow                *ir.TableRow
	tableCell               *ir.TableCell
	tableActive             bool
	connectorRefs           map[*ir.Shape]connectorRefs
	inNotes                 bool
	showMasterShapes        bool
	layoutShowsMasterShapes bool
}

type connectorRef struct {
	shapeID, siteIdx int
	set              bool
}
type connectorRefs struct{ start, end connectorRef }

type relationshipTarget struct {
	Target   string
	External bool
}

type relationshipPayload struct {
	Data        []byte
	ContentType string
	RelType     string
}

type textStyleLevel struct {
	Paragraph ParagraphProperties
	Run       RunProperties
}

func newIRBuilder() *irBuilder {
	return &irBuilder{presentation: &ir.Presentation{}, themeColors: make(map[string]string), themeLineStyles: make(map[int]LineProperties), themeFillStyles: make(map[int]ThemeFillStyle), themeEffectStyles: make(map[int]ThemeEffectStyle), masterTextStyles: make(map[string]map[int]textStyleLevel), defaultTextStyles: make(map[int]textStyleLevel), themeFonts: make(map[string]string), connectorRefs: make(map[*ir.Shape]connectorRefs), relationships: make(map[int]map[string]relationshipTarget), payloads: make(map[int]map[string]relationshipPayload), olePreviews: make(map[int]map[string]relationshipPayload), partRelationships: make(map[string]map[string]relationshipTarget), partPayloads: make(map[string]map[string]relationshipPayload)}
}

func (b *irBuilder) HandleEvent(ev Event) error {
	switch ev.Type {
	case EventPresentationProperties:
		b.presentation.SlideWidthEMU, b.presentation.SlideHeightEMU = ev.CanvasWidthEMU, ev.CanvasHeightEMU
	case EventBackgroundStyleRef:
		style, ok := b.themeFillStyles[ev.StyleIndex]
		if ok && style.Kind != "" && b.slide != nil {
			tmp := &ir.Shape{}
			b.applyThemeFillStyle(tmp, style, ev.Color)
			if tmp.Fill != nil {
				b.slide.BgFill, b.slide.BgColor = tmp.Fill, tmp.FillColor
			}
		}
	case EventBackgroundPicture:
		if b.slide != nil {
			payload := b.relationshipPayloadForEvent(ev)
			if len(payload.Data) != 0 {
				fill := drawing.PictureFill{EmbedID: ev.RelID, ImageData: append([]byte(nil), payload.Data...), ContentType: payload.ContentType, Stretch: true, Alpha: 1}
				if ev.CropLeft != 0 || ev.CropTop != 0 || ev.CropRight != 0 || ev.CropBottom != 0 {
					fill.Crop = &drawing.Rect{X: float64(ev.CropLeft) / 100000, Y: float64(ev.CropTop) / 100000, Width: 1 - float64(ev.CropLeft+ev.CropRight)/100000, Height: 1 - float64(ev.CropTop+ev.CropBottom)/100000}
				}
				if ev.PictureFillLeft != 0 || ev.PictureFillTop != 0 || ev.PictureFillRight != 0 || ev.PictureFillBottom != 0 {
					fill.FillRect = &drawing.Rect{X: float64(ev.PictureFillLeft) / 100000, Y: float64(ev.PictureFillTop) / 100000, Width: 1 - float64(ev.PictureFillLeft+ev.PictureFillRight)/100000, Height: 1 - float64(ev.PictureFillTop+ev.PictureFillBottom)/100000}
				}
				if ev.PictureTileSx != 0 || ev.PictureTileSy != 0 || ev.PictureTileTx != 0 || ev.PictureTileTy != 0 || ev.PictureTileFlip != "" || ev.PictureTileAlign != "" {
					fill.Tile, fill.Stretch = true, false
					fill.TileOffsetX, fill.TileOffsetY = ev.PictureTileTx, ev.PictureTileTy
					fill.TileScaleX, fill.TileScaleY = ev.PictureTileSx, ev.PictureTileSy
					fill.TileFlip, fill.TileAlign = ev.PictureTileFlip, ev.PictureTileAlign
				}
				b.slide.BgFill = fill
			}
		}
	case EventSlideBackground:
		if ev.SlideIndex >= 0 && ev.SlideIndex < len(b.slides) {
			color := b.resolveVisualColor(ev.Color)
			b.slides[ev.SlideIndex].BgColor = color
			b.slides[ev.SlideIndex].BgFill = drawing.SolidFill{Color: color, Alpha: 1}
		} else if b.slide != nil {
			color := b.resolveVisualColor(ev.Color)
			b.slide.BgColor = color
			b.slide.BgFill = drawing.SolidFill{Color: color, Alpha: 1}
		}
	case EventSlideMasterVisibility:
		if ev.ShowMasterShapesSet && b.slide != nil && b.slide.Index == ev.SlideIndex {
			if ev.Scope == "layout" {
				b.layoutShowsMasterShapes = ev.ShowMasterShapes
			} else {
				b.showMasterShapes = ev.ShowMasterShapes
			}
		}
	case EventSlideTransition:
		tr := &ir.SlideTransition{Type: ev.TransitionType, Spd: ev.TransitionSpeed, AdvClick: ev.TransitionAdvClick, AdvTmMs: ev.TransitionAdvTmMs}
		if b.slide != nil && b.slide.Index == ev.SlideIndex {
			b.slide.Transition = tr
		} else if ev.SlideIndex >= 0 && ev.SlideIndex < len(b.slides) {
			b.slides[ev.SlideIndex].Transition = tr
		}
	case EventStartComment:
		comment := &ir.Comment{AuthorName: ev.Author, Initials: ev.Initials, DateTime: ev.Date, Text: ev.Text, X: ev.CommentX, Y: ev.CommentY}
		if b.slide != nil && b.slide.Index == ev.SlideIndex {
			b.slide.Comments = append(b.slide.Comments, comment)
		} else if ev.SlideIndex >= 0 && ev.SlideIndex < len(b.slides) {
			b.slides[ev.SlideIndex].Comments = append(b.slides[ev.SlideIndex].Comments, comment)
		}
	case EventEndComment:
	case EventStartNotes:
		if ev.SlideIndex >= 0 && ev.SlideIndex < len(b.slides) {
			b.slide = b.slides[ev.SlideIndex]
			b.slide.Notes = &ir.NotesSlide{}
			b.inNotes = true
		}
	case EventEndNotes:
		b.inNotes = false
		b.slide = nil
		b.shapes = nil
		b.paragraph = nil
	case EventStartSlide:
		if b.slide != nil {
			return fmt.Errorf("sax ir builder: nested slide")
		}
		b.slide = &ir.Slide{Index: ev.SlideIndex}
		// PresentationML defaults showMasterSp to true when the attribute is absent,
		// both on the slide and on its layout. The two visibility scopes compose:
		// either one can suppress inherited master artwork without removing the
		// master template graph used for placeholder/style inheritance.
		b.showMasterShapes = true
		b.layoutShowsMasterShapes = true
		// Theme and color-map state is slide-contextual. Do not let a previous
		// master/layout chain leak into the next slide.
		b.themeColors = make(map[string]string)
		b.themeLineStyles = make(map[int]LineProperties)
		b.themeFillStyles = make(map[int]ThemeFillStyle)
		b.themeEffectStyles = make(map[int]ThemeEffectStyle)
		b.masterColorMap = make(map[string]string)
		b.layoutColorMap = make(map[string]string)
		b.slideColorMap = make(map[string]string)
		b.activeColorScope = "slide"
	case EventRelationship:
		rels := b.relationships[ev.SlideIndex]
		if rels == nil {
			rels = make(map[string]relationshipTarget)
			b.relationships[ev.SlideIndex] = rels
		}
		// Keep the slide-index view for compatibility with slide actions, while
		// also preserving OOXML's actual ownership rule: rIds are local to a part.
		if ev.Scope == "slide" {
			rels[ev.RelID] = relationshipTarget{Target: ev.TargetPart, External: ev.External}
		}
		partRels := b.partRelationships[ev.PartName]
		if partRels == nil {
			partRels = make(map[string]relationshipTarget)
			b.partRelationships[ev.PartName] = partRels
		}
		partRels[ev.RelID] = relationshipTarget{Target: ev.TargetPart, External: ev.External}
	case EventRelationshipPayload:
		payload := relationshipPayload{Data: append([]byte(nil), ev.Payload...), ContentType: ev.ContentType, RelType: ev.RelType}
		partPayloads := b.partPayloads[ev.PartName]
		if partPayloads == nil {
			partPayloads = make(map[string]relationshipPayload)
			b.partPayloads[ev.PartName] = partPayloads
		}
		partPayloads[ev.RelID] = payload
		if ev.Scope == "slide" {
			payloads := b.payloads[ev.SlideIndex]
			if payloads == nil {
				payloads = make(map[string]relationshipPayload)
				b.payloads[ev.SlideIndex] = payloads
			}
			payloads[ev.RelID] = payload
		}
	case EventOLEPreview:
		if ev.OLEShapeRef == "" || len(ev.Payload) == 0 {
			return nil
		}
		previews := b.olePreviews[ev.SlideIndex]
		if previews == nil {
			previews = make(map[string]relationshipPayload)
			b.olePreviews[ev.SlideIndex] = previews
		}
		previews[ev.OLEShapeRef] = relationshipPayload{Data: append([]byte(nil), ev.Payload...), ContentType: ev.ContentType, RelType: ev.RelType}
	case EventStartMaster:
		b.masterShapes = nil
		b.masterColorMap = make(map[string]string)
		b.activeColorScope = "master"
		b.masterTextStyles = make(map[string]map[int]textStyleLevel)
		b.masterPart = ev.PartName
	case EventStartLayout:
		b.layoutShapes = nil
		b.layoutPart = ev.PartName
		b.layoutColorMap = cloneStringMap(b.masterColorMap)
		b.activeColorScope = "layout"
	case EventTextStyle:
		if ev.Scope == "presentation" && ev.TextStyleKind == "default" {
			b.defaultTextStyles[ev.TextStyleLevel] = textStyleLevel{Paragraph: ev.Paragraph, Run: ev.Run}
		} else if ev.Scope == "master" && ev.TextStyleKind != "" {
			levels := b.masterTextStyles[ev.TextStyleKind]
			if levels == nil {
				levels = make(map[int]textStyleLevel)
				b.masterTextStyles[ev.TextStyleKind] = levels
			}
			levels[ev.TextStyleLevel] = textStyleLevel{Paragraph: ev.Paragraph, Run: ev.Run}
		}
	case EventEndMaster:
		b.activeColorScope = "slide"
		if len(b.shapes) != 0 {
			return fmt.Errorf("sax ir builder: context ended with %d open shapes", len(b.shapes))
		}
		b.paragraph = nil
	case EventEndLayout:
		b.slideColorMap = cloneStringMap(b.layoutColorMap)
		b.activeColorScope = "slide"
		if len(b.shapes) != 0 {
			return fmt.Errorf("sax ir builder: context ended with %d open shapes", len(b.shapes))
		}
		b.paragraph = nil
	case EventEndSlide:
		if b.slide == nil {
			return fmt.Errorf("sax ir builder: slide end without slide")
		}
		if len(b.shapes) != 0 {
			return fmt.Errorf("sax ir builder: slide ended with %d open shapes", len(b.shapes))
		}
		b.applyContextualInheritance()
		b.resolveConnectorEndpoints()
		b.slides = append(b.slides, b.slide)
		b.slide = nil
		b.paragraph = nil
	case EventStartShape:
		if b.slide == nil {
			return nil
		} // notes use the same low-level walker
		s := &ir.Shape{ID: ev.ShapeID, Name: ev.ShapeName, Kind: shapeKind(ev.ShapeKind)}
		if len(b.shapes) == 0 {
			if b.inNotes && b.slide.Notes != nil {
				b.slide.Notes.Shapes = append(b.slide.Notes.Shapes, s)
			} else {
				switch ev.Scope {
				case "master":
					b.masterShapes = append(b.masterShapes, s)
				case "layout":
					b.layoutShapes = append(b.layoutShapes, s)
				default:
					b.slide.Shapes = append(b.slide.Shapes, s)
				}
			}
		} else {
			b.shapes[len(b.shapes)-1].Children = append(b.shapes[len(b.shapes)-1].Children, s)
		}
		b.shapes = append(b.shapes, s)
	case EventEndShape:
		if b.slide == nil {
			return nil
		}
		if len(b.shapes) == 0 {
			return fmt.Errorf("sax ir builder: shape end without shape")
		}
		s := b.shapes[len(b.shapes)-1]
		if ev.ShapeID != 0 && s.ID != ev.ShapeID {
			return fmt.Errorf("sax ir builder: shape end id %d, want %d", ev.ShapeID, s.ID)
		}
		b.shapes = b.shapes[:len(b.shapes)-1]
		b.paragraph = nil
	case EventConnectorEndpoint:
		if s := b.currentShape(); s != nil && s.Kind == ir.ShapeConnector {
			r := b.connectorRefs[s]
			ref := connectorRef{shapeID: ev.ConnectorShapeID, siteIdx: ev.ConnectorSiteIdx, set: true}
			if ev.ConnectorEndpoint == "stCxn" {
				r.start = ref
			} else {
				r.end = ref
			}
			b.connectorRefs[s] = r
		}
	case EventTransform:
		if s := b.currentShape(); s != nil {
			s.HasTransform = true
			x := ev.Transform
			if x.HasOffset {
				s.Bounds.X, s.Bounds.Y = x.X, x.Y
			}
			if x.HasExtents {
				s.Bounds.W, s.Bounds.H = x.CX, x.CY
			}
			s.RotationDeg = float64(x.Rotation) / 60000.0
			s.FlipH, s.FlipV = x.FlipH, x.FlipV
			if s.Kind == ir.ShapeGroup {
				s.GroupTransform = &ir.VisualTransform{Offset: ir.Point{X: x.X, Y: x.Y}, Extent: ir.VisualSize{W: x.CX, H: x.CY}, ChildOffset: ir.Point{X: x.ChildX, Y: x.ChildY}, ChildExtent: ir.VisualSize{W: x.ChildCX, H: x.ChildCY}, Rotation: float64(x.Rotation) / 60000.0, FlipH: x.FlipH, FlipV: x.FlipV}
			}
		}
	case EventPlaceholder:
		if s := b.currentShape(); s != nil {
			s.Placeholder = placeholderKind(ev.Placeholder, ev.PlaceholderIdx)
			s.PlaceholderIdx = ev.PlaceholderIdx
			s.PlaceholderHasCustomPrompt = ev.PlaceholderHasCustomPrompt
			s.PlaceholderOrient = ev.PlaceholderOrient
			s.PlaceholderSize = ev.PlaceholderSize
		}
	case EventGeometry:
		if s := b.currentShape(); s != nil {
			s.Geometry = ev.Geometry
			if ev.GeometryData != nil {
				s.GeometryData = ev.GeometryData
			}
			if s.Kind == ir.ShapeConnector {
				s.ConnectorType = ev.Geometry
			}
		}
	case EventThemeColor:
		if ev.ThemeName != "" {
			b.themeName = ev.ThemeName
		}
		if ev.ThemeSlot != "" && ev.Color.Value != "" {
			b.themeColors[ev.ThemeSlot] = ev.Color.Value
		}
	case EventColorMap:
		target := b.slideColorMap
		switch ev.Scope {
		case "master":
			target = b.masterColorMap
		case "layout":
			target = b.layoutColorMap
		}
		if target == nil {
			target = make(map[string]string)
			switch ev.Scope {
			case "master":
				b.masterColorMap = target
			case "layout":
				b.layoutColorMap = target
			default:
				b.slideColorMap = target
			}
		}
		for k, v := range ev.ColorMap {
			target[k] = v
		}
	case EventThemeFont:
		if ev.ThemeFontKind != "" && ev.ThemeFontFamily != "" {
			b.themeFonts[ev.ThemeFontKind] = ev.ThemeFontFamily
		}
	case EventTextBodyProps:
		if sh := b.currentShape(); sh != nil {
			applyTextBodyProps(sh, ev.TextBody)
		}
	case EventFill:
		if s := b.currentShape(); s != nil {
			color := b.resolveVisualColor(ev.Color)
			s.FillColor = color
			alpha := 1.0
			if ev.Color.HasAlpha {
				alpha = float64(ev.Color.Alpha) / 100000.0
			}
			fill := drawing.SolidFill{Color: color, Alpha: alpha, ThemeColor: func() string {
				if ev.Color.Kind == "schemeClr" {
					return ev.Color.Value
				}
				return ""
			}()}
			if ev.PaintTarget == "tableCell" && b.tableCell != nil {
				b.tableCell.Fill, b.tableCell.FillColor = fill, color
			} else {
				s.FillColor, s.Fill = color, fill
			}
		}
	case EventNoFill:
		if s := b.currentShape(); s != nil {
			if ev.PaintTarget == "tableCell" && b.tableCell != nil {
				b.tableCell.FillColor = ""
				b.tableCell.Fill = drawing.NoFill{}
			} else {
				s.FillColor = ""
				s.Fill = drawing.NoFill{}
			}
		}
	case EventGradientFill:
		if s := b.currentShape(); s != nil {
			g := drawing.GradientFill{GradientType: drawing.GradientTypeLinear, Angle: float64(ev.Gradient.Angle) / 60000.0, Flip: ev.Gradient.Flip, RotateWithShape: ev.Gradient.RotateWithShape}
			if ev.Gradient.Path != "" {
				g.GradientType = drawing.GradientTypePath
			}
			for _, st := range ev.Gradient.Stops {
				a := 1.0
				if st.Color.HasAlpha {
					a = float64(st.Color.Alpha) / 100000.0
				}
				g.Stops = append(g.Stops, drawing.GradientStop{Position: float64(st.Position) / 100000.0, Color: b.resolveVisualColor(st.Color), Alpha: a})
			}
			if ev.PaintTarget == "tableCell" && b.tableCell != nil {
				b.tableCell.Fill, b.tableCell.FillColor = g, ""
			} else {
				s.Fill, s.FillColor = g, ""
			}
		}
	case EventPatternFill:
		if s := b.currentShape(); s != nil {
			fa, ba := 1.0, 1.0
			if ev.Pattern.Foreground.HasAlpha {
				fa = float64(ev.Pattern.Foreground.Alpha) / 100000.0
			}
			if ev.Pattern.Background.HasAlpha {
				ba = float64(ev.Pattern.Background.Alpha) / 100000.0
			}
			pf := drawing.PatternFill{PatternType: drawing.PatternType(ev.Pattern.Preset), Foreground: b.resolveVisualColor(ev.Pattern.Foreground), Background: b.resolveVisualColor(ev.Pattern.Background), ForeAlpha: fa, BackAlpha: ba}
			if ev.PaintTarget == "tableCell" && b.tableCell != nil {
				b.tableCell.Fill, b.tableCell.FillColor = pf, ""
			} else {
				s.Fill, s.FillColor = pf, ""
			}
		}
	case EventThemeLineStyle:
		b.themeLineStyles[ev.StyleIndex] = ev.Line
	case EventThemeFillStyle:
		b.themeFillStyles[ev.StyleIndex] = ev.ThemeFill
	case EventThemeEffectStyle:
		b.themeEffectStyles[ev.StyleIndex] = ev.ThemeEffect
	case EventLineStyleRef:
		if s := b.currentShape(); s != nil && s.LineStyle == nil && s.LineColor == "" {
			line := b.themeLineStyles[ev.StyleIndex]
			colorToken := ev.Color
			if colorToken.Value == "" || colorToken.Value == "phClr" {
				colorToken = line.Color
			}
			color := b.resolveVisualColor(colorToken)
			s.LineColor = color
			if line.WidthEMU != 0 || line.Cap != "" || line.Join != "" || line.Dash != "" || line.Compound != "" || color != "" {
				ls := drawing.DefaultLineStyle()
				if line.WidthEMU != 0 {
					ls.Width = float64(line.WidthEMU) / 12700.0
				}
				if color != "" {
					ls.Color = color
				}
				if line.Cap != "" {
					ls.Cap = drawing.LineCap(line.Cap)
				}
				if line.Join != "" {
					ls.Join = drawing.LineJoin(line.Join)
				}
				if line.Dash != "" {
					ls.Dash = drawing.DashPattern(line.Dash)
				}
				ls.CompoundType = line.Compound
				if line.MiterLimit != 0 {
					ls.MiterLimit = float64(line.MiterLimit) / 100000.0
				}
				ls.HeadEnd = saxArrowhead(line.HeadType, line.HeadWidth, line.HeadLength)
				ls.TailEnd = saxArrowhead(line.TailType, line.TailWidth, line.TailLength)
				s.ArrowHead, s.ArrowHeadWidth, s.ArrowHeadLength = line.HeadType, line.HeadWidth, line.HeadLength
				s.ArrowTail, s.ArrowTailWidth, s.ArrowTailLength = line.TailType, line.TailWidth, line.TailLength
				s.LineStyle = &ls
				s.LineWidthPt = ls.Width
			}
		}
	case EventFillStyleRef:
		if s := b.currentShape(); s != nil && s.Fill == nil && s.FillColor == "" {
			style, ok := b.themeFillStyles[ev.StyleIndex]
			if ok && style.Kind != "" {
				b.applyThemeFillStyle(s, style, ev.Color)
			} else if color := b.resolveVisualColor(ev.Color); color != "" {
				// Preserve the authored placeholder color when the referenced
				// fmtScheme fill is unavailable. A missing theme style must not
				// erase otherwise valid fillRef semantics.
				s.FillColor = color
			}
		}
	case EventEffectStyleRef:
		if s := b.currentShape(); s != nil && len(s.Effects.Effects) == 0 {
			style := b.themeEffectStyles[ev.StyleIndex]
			for _, source := range style.Effects {
				fxp := source
				if fxp.Color.Value == "phClr" || fxp.Color.Value == "" {
					fxp.Color = ev.Color
				}
				if fx := b.materializeEffect(fxp); fx != nil {
					s.Effects.Effects = append(s.Effects.Effects, fx)
				}
			}
		}
	case EventLine:
		if s := b.currentShape(); s != nil {
			color := b.resolveVisualColor(ev.Line.Color)
			if color == "" {
				color = b.resolveVisualColor(ev.Color)
			}
			s.LineColor = color
			s.LineWidthPt = float64(ev.Line.WidthEMU) / 12700.0
			if ev.Line.WidthEMU == 0 {
				s.LineWidthPt = float64(ev.LineWidthEMU) / 12700.0
			}
			ls := drawing.DefaultLineStyle()
			ls.Width, ls.Color = s.LineWidthPt, color
			if ev.Line.Cap != "" {
				ls.Cap = drawing.LineCap(ev.Line.Cap)
			}
			if ev.Line.Join != "" {
				ls.Join = drawing.LineJoin(ev.Line.Join)
			}
			if ev.Line.Dash != "" {
				ls.Dash = drawing.DashPattern(ev.Line.Dash)
			}
			ls.CompoundType = ev.Line.Compound
			if ev.Line.MiterLimit != 0 {
				ls.MiterLimit = float64(ev.Line.MiterLimit) / 100000.0
			}
			ls.HeadEnd = saxArrowhead(ev.Line.HeadType, ev.Line.HeadWidth, ev.Line.HeadLength)
			ls.TailEnd = saxArrowhead(ev.Line.TailType, ev.Line.TailWidth, ev.Line.TailLength)
			s.ArrowHead, s.ArrowHeadWidth, s.ArrowHeadLength = ev.Line.HeadType, ev.Line.HeadWidth, ev.Line.HeadLength
			s.ArrowTail, s.ArrowTailWidth, s.ArrowTailLength = ev.Line.TailType, ev.Line.TailWidth, ev.Line.TailLength
			if ev.Line.Color.HasAlpha {
				ls.Alpha = float64(ev.Line.Color.Alpha) / 100000.0
			}
			if b.tableCell != nil && strings.HasPrefix(ev.PaintTarget, "border") {
				switch ev.PaintTarget {
				case "borderLeft":
					b.tableCell.BorderLeft = &ls
				case "borderRight":
					b.tableCell.BorderRight = &ls
				case "borderTop":
					b.tableCell.BorderTop = &ls
				case "borderBottom":
					b.tableCell.BorderBottom = &ls
				}
			} else {
				s.LineStyle = &ls
			}
		}
	case EventEffect:
		if s := b.currentShape(); s != nil {
			if fx := b.materializeEffect(ev.Effect); fx != nil {
				s.Effects.Effects = append(s.Effects.Effects, fx)
			}
		}
	case EventGraphicObject:
		if s := b.currentShape(); s != nil {
			payload := b.relationshipPayloadForEvent(ev)
			switch ev.GraphicObjectKind {
			case "chart":
				s.Kind = ir.ShapeChart
				s.ImageRelID = ev.RelID
				s.ChartData = append([]byte(nil), payload.Data...)
			case "smartArt":
				s.Kind = ir.ShapeSmartArt
				s.ImageRelID = ev.RelID
			case "video", "audio":
				s.Kind = ir.ShapeMedia
				s.MediaKind = ev.GraphicObjectKind
				s.MediaRelID = ev.RelID
				s.MediaData = append([]byte(nil), payload.Data...)
				s.MediaMIME = payload.ContentType
			case "ole":
				s.Kind = ir.ShapeOleObject
				s.ImageRelID = ev.RelID
				s.OLEObjectName = ev.OLEName
				s.OLEObjectProgID = ev.OLEProgID
				s.OLEObjectData = append([]byte(nil), payload.Data...)
				s.OLEObjectMIME = payload.ContentType
				if rel := b.relationshipForEvent(ev); rel.External {
					s.OLEObjectLinkURL = rel.Target
				}
				if ev.OLEShapeRef != "" {
					if previews := b.olePreviews[ev.SlideIndex]; previews != nil {
						if preview := previews[ev.OLEShapeRef]; len(preview.Data) > 0 {
							s.OLEIconData = append([]byte(nil), preview.Data...)
							s.OLEIconMIME = preview.ContentType
						}
					}
				}
			}
		}
	case EventShapeAction:
		if s := b.currentShape(); s != nil {
			if ev.ActionRelID != "" && b.slide != nil {
				if rel := b.relationships[b.slide.Index][ev.ActionRelID]; rel.Target != "" {
					if rel.External {
						s.HyperlinkURL = rel.Target
					}
				}
			}
			if ev.Action != "" {
				s.ActionLink = normalizeActionLink(ev.Action)
			}
		}
	case EventStartTable:
		b.tableActive = true
		if s := b.currentShape(); s != nil {
			s.Kind = ir.ShapeTable
			s.Table = &ir.Table{}
		}
	case EventTableColumn:
		if s := b.currentShape(); s != nil && s.Table != nil {
			s.Table.ColumnWidthsEMU = append(s.Table.ColumnWidthsEMU, ev.TableValue)
		}
	case EventStartTableRow:
		if s := b.currentShape(); s != nil && s.Table != nil {
			b.tableRow = &ir.TableRow{HeightEMU: ev.TableValue}
			s.Table.Rows = append(s.Table.Rows, b.tableRow)
		}
	case EventEndTableRow:
		b.tableRow = nil
	case EventStartTableCell:
		if b.tableRow != nil {
			span := ev.TableGridSpan
			if span == 0 {
				span = int(ev.TableValue)
			}
			if span == 0 {
				span = 1
			}
			b.tableCell = &ir.TableCell{
				GridSpan: span, RowSpan: ev.TableRowSpan, HMerge: ev.TableHMerge, VMerge: ev.TableVMerge,
				MarginLeftEMU: 91440, MarginRightEMU: 91440, MarginTopEMU: 45720, MarginBottomEMU: 45720,
			}
			b.tableRow.Cells = append(b.tableRow.Cells, b.tableCell)
		}
	case EventTableCellProps:
		if b.tableCell != nil {
			b.tableCell.TextAnchor = ev.TableAnchor
			if ev.TableMarginLeftSet || ev.TableMarginLeft != 0 {
				b.tableCell.MarginLeftEMU = ev.TableMarginLeft
			}
			if ev.TableMarginRightSet || ev.TableMarginRight != 0 {
				b.tableCell.MarginRightEMU = ev.TableMarginRight
			}
			if ev.TableMarginTopSet || ev.TableMarginTop != 0 {
				b.tableCell.MarginTopEMU = ev.TableMarginTop
			}
			if ev.TableMarginBottomSet || ev.TableMarginBottom != 0 {
				b.tableCell.MarginBottomEMU = ev.TableMarginBottom
			}
		}
	case EventEndTableCell:
		b.tableCell = nil
		b.paragraph = nil
	case EventEndTable:
		b.tableActive = false
		b.tableRow = nil
		b.tableCell = nil
		b.paragraph = nil
	case EventBlip:
		if s := b.currentShape(); s != nil {
			if ev.RelID != "" {
				s.ImageRelID = ev.RelID
				payload := b.relationshipPayloadForEvent(ev)
				if svg, _, _, ok := metafile.ToSVG(payload.Data, payload.ContentType); ok {
					payload.Data = svg
					payload.ContentType = "image/svg+xml"
				}
				if len(payload.Data) != 0 {
					if s.Kind == ir.ShapeOleObject {
						s.OLEIconData = append([]byte(nil), payload.Data...)
						s.OLEIconMIME = payload.ContentType
					} else {
						s.ImageData = append([]byte(nil), payload.Data...)
						s.ImageMIME = payload.ContentType
					}
				}
			}
			s.ImageCropLeft = float64(ev.CropLeft) / 100000
			s.ImageCropTop = float64(ev.CropTop) / 100000
			s.ImageCropRight = float64(ev.CropRight) / 100000
			s.ImageCropBottom = float64(ev.CropBottom) / 100000
			if ev.PictureTileSx != 0 || ev.PictureTileSy != 0 || ev.PictureTileTx != 0 || ev.PictureTileTy != 0 || ev.PictureTileFlip != "" || ev.PictureTileAlign != "" {
				s.PictureTile = &ir.PictureTile{Tx: ev.PictureTileTx, Ty: ev.PictureTileTy, Sx: ev.PictureTileSx, Sy: ev.PictureTileSy, Flip: ev.PictureTileFlip, Alignment: ev.PictureTileAlign}
			}
			if ev.PictureFillLeft != 0 || ev.PictureFillTop != 0 || ev.PictureFillRight != 0 || ev.PictureFillBottom != 0 {
				s.ImageFillLeft, s.ImageFillTop = float64(ev.PictureFillLeft)/100000, float64(ev.PictureFillTop)/100000
				s.ImageFillRight, s.ImageFillBottom = float64(ev.PictureFillRight)/100000, float64(ev.PictureFillBottom)/100000
			}
		}
	case EventStartParagraph:
		if s := b.currentShape(); s != nil {
			b.paragraph = &ir.Paragraph{}
			if b.tableCell != nil {
				b.tableCell.Paragraphs = append(b.tableCell.Paragraphs, b.paragraph)
			} else {
				s.Paragraphs = append(s.Paragraphs, b.paragraph)
			}
			b.runProps = nil
		}
	case EventParagraphProps:
		if b.paragraph != nil {
			pp := ev.Paragraph
			if color := b.resolveVisualColor(pp.BulletColor); color != "" {
				pp.BulletColor = VisualColor{Kind: "srgbClr", Value: color}
			}
			applyParagraphProps(b.paragraph, pp)
		}
	case EventStartRun:
		b.pendingRun = &ir.Run{FieldKind: ev.FieldType}
		b.runProps = nil
	case EventRunProps:
		rp := ev.Run
		if rp.Color.Kind == "schemeClr" {
			rp.Color = VisualColor{Kind: "srgbClr", Value: b.resolveVisualColor(rp.Color)}
		}
		if rp.Highlight.Kind == "schemeClr" {
			rp.Highlight = VisualColor{Kind: "srgbClr", Value: b.resolveVisualColor(rp.Highlight)}
		}
		b.runProps = &rp
		if b.pendingRun != nil {
			b.applyPendingRunProps()
		}
	case EventEndRun:
		b.finishPendingRun()
	case EventEndParagraph:
		sh := b.currentShape()
		if sh != nil && sh.Placeholder != ir.PlaceholderNone && b.paragraph != nil && len(b.paragraph.Runs) == 0 && b.runProps != nil {
			r := &ir.Run{}
			applyRunProps(r, *b.runProps)
			b.paragraph.Runs = append(b.paragraph.Runs, r)
		}
		b.paragraph = nil
		b.runProps = nil
	case EventText:
		if b.tableActive && b.tableCell == nil {
			return nil
		}
		if b.pendingRun != nil {
			b.pendingRun.Text += ev.Text
			if b.pendingRun.FieldKind == "" {
				b.pendingRun.FieldKind = ev.FieldType
			}
		} else {
			b.appendText(ev.Text, ev.FieldType)
		}
	case EventBreak:
		b.appendText("\n", "")
	case EventTab:
		b.appendText("	", "")
	case EventField:
		b.appendText("", ev.FieldType)
	}
	return nil
}

func (b *irBuilder) relationshipPayloadForEvent(ev Event) relationshipPayload {
	if ev.RelID == "" {
		return relationshipPayload{}
	}
	if payload := b.partPayloads[ev.PartName][ev.RelID]; len(payload.Data) != 0 || payload.ContentType != "" || payload.RelType != "" {
		return payload
	}
	// OOXML relationship identifiers are local to their owning package part.
	// Never let an unresolved master/layout rId borrow a same-named slide
	// payload: that silently paints the wrong image/media and is worse than
	// preserving an unresolved canonical resource. The slide-index map remains
	// only as a compatibility view for legacy/part-less slide events.
	if ev.Scope == "slide" || ev.PartName == "" {
		return b.relationshipPayload(ev.SlideIndex, ev.RelID)
	}
	return relationshipPayload{}
}

func (b *irBuilder) relationshipForEvent(ev Event) relationshipTarget {
	if ev.RelID == "" {
		return relationshipTarget{}
	}
	if rel, ok := b.partRelationships[ev.PartName][ev.RelID]; ok {
		return rel
	}
	// As with payloads, a relationship target is meaningful only in the part
	// that owns its rId. Restrict the slide-index fallback to slide/part-less
	// compatibility events so inherited artwork cannot resolve through an
	// unrelated slide relationship with the same identifier.
	if ev.Scope == "slide" || ev.PartName == "" {
		return b.relationships[ev.SlideIndex][ev.RelID]
	}
	return relationshipTarget{}
}

func (b *irBuilder) relationshipPayload(slideIndex int, relID string) relationshipPayload {
	if relID == "" {
		return relationshipPayload{}
	}
	return b.payloads[slideIndex][relID]
}

func (b *irBuilder) currentShape() *ir.Shape {
	if len(b.shapes) == 0 {
		return nil
	}
	return b.shapes[len(b.shapes)-1]
}
func (b *irBuilder) applyPendingRunProps() {
	if b.pendingRun == nil || b.runProps == nil {
		return
	}
	applyRunProps(b.pendingRun, *b.runProps)
	if b.runProps.HyperlinkRelID != "" && b.slide != nil {
		if rel := b.relationships[b.slide.Index][b.runProps.HyperlinkRelID]; rel.External {
			b.pendingRun.HyperlinkURL = rel.Target
		}
	}
}

func (b *irBuilder) finishPendingRun() {
	if b.pendingRun == nil {
		return
	}
	b.applyPendingRunProps()
	if b.paragraph != nil {
		b.paragraph.Runs = append(b.paragraph.Runs, b.pendingRun)
	}
	b.pendingRun = nil
	b.runProps = nil
}

func (b *irBuilder) appendText(text, field string) {
	if b.paragraph == nil {
		if s := b.currentShape(); s != nil {
			b.paragraph = &ir.Paragraph{}
			s.Paragraphs = append(s.Paragraphs, b.paragraph)
		}
	}
	if b.paragraph != nil {
		r := &ir.Run{Text: text, FieldKind: field}
		if b.runProps != nil {
			applyRunProps(r, *b.runProps)
			if b.runProps.HyperlinkRelID != "" && b.slide != nil {
				if rel := b.relationships[b.slide.Index][b.runProps.HyperlinkRelID]; rel.External {
					r.HyperlinkURL = rel.Target
				}
			}
			b.runProps = nil
		}
		b.paragraph.Runs = append(b.paragraph.Runs, r)
	}
}

func applyParagraphProps(p *ir.Paragraph, v ParagraphProperties) {
	if v.HasLevel {
		p.Level = v.Level
	}
	if v.Align != "" {
		p.Align = v.Align
	}
	if v.HasMarginLeft {
		p.MarginLeftEMU, p.HasMarginLeft = v.MarginLeftEMU, true
	}
	if v.HasMarginRight {
		p.MarginRightEMU, p.HasMarginRight = v.MarginRightEMU, true
	}
	if v.HasTextIndent {
		p.TextIndentEMU, p.HasTextIndent = v.TextIndentEMU, true
	}
	if v.HasDefaultTabSize {
		p.DefaultTabSizeEMU = v.DefaultTabSizeEMU
	}
	if v.FontAlign != "" {
		p.FontAlign = v.FontAlign
	}
	if v.TextDirection != "" {
		p.TextDirection = v.TextDirection
	}
	for _, tab := range v.TabStops {
		p.TabStops = append(p.TabStops, ir.TabStop{Pos: tab.Pos, Algn: tab.Align})
	}
	if v.HasBullet {
		p.BulletChar = v.BulletChar
		p.BulletNone = v.BulletChar == "" && !v.AutoNum
	}
	if v.HasAutoNum {
		p.AutoNum, p.AutoNumType, p.AutoNumStart = v.AutoNum, v.AutoNumType, v.AutoNumStart
		if v.AutoNum {
			// buAutoNum is the marker definition at this cascade level; it replaces
			// an inherited buChar rather than coexisting with it.
			p.BulletChar = ""
			p.BulletNone = false
		}
	}
	if v.BulletFontTx {
		p.BulletFontTx = true
		p.BulletFontFamily = ""
	} else if v.BulletFontFamily != "" {
		p.BulletFontTx = false
		p.BulletFontFamily = v.BulletFontFamily
	}
	if v.BulletSize != 0 {
		p.BulletSize = v.BulletSize
	}
	if v.BulletSizePct != 0 {
		p.BulletSizePct = v.BulletSizePct
	}
	if v.BulletColor.Value != "" {
		p.BulletColor = v.BulletColor.Value
	}
	if v.HasSpaceBefore {
		p.SpaceBefore, p.SpaceBeforePct, p.HasSpaceBefore = v.SpaceBefore, v.SpaceBeforePct, true
	}
	if v.HasSpaceAfter {
		p.SpaceAfter, p.SpaceAfterPct, p.HasSpaceAfter = v.SpaceAfter, v.SpaceAfterPct, true
	}
	if v.HasLineSpacing {
		p.LineSpacingPct, p.LineSpacingPt, p.HasLineSpacing = v.LineSpacingPct, v.LineSpacingPt, true
	}
}

func applyRunProps(r *ir.Run, v RunProperties) {
	if v.BoldSet {
		r.Bold, r.BoldSet = v.Bold, true
	}
	if v.ItalicSet {
		r.Italic, r.ItalicSet = v.Italic, true
	}
	if v.UnderlineSet {
		r.Underline, r.UnderlineSet = v.Underline, true
	}
	if v.StrikeSet {
		r.Strike, r.StrikeSet = v.Strike, true
	}
	if v.FontSize != 0 {
		r.FontSize = v.FontSize
	}
	if v.FontFamily != "" {
		r.FontFamily = v.FontFamily
	}
	if v.CharacterSpacing != 0 {
		r.CharSpacing = v.CharacterSpacing
	}
	if v.Kerning != 0 {
		r.Kern = v.Kerning
	}
	switch v.Color.Kind {
	case "srgbClr":
		r.Color = v.Color.Value
	}
	switch v.Highlight.Kind {
	case "srgbClr":
		r.Highlight = v.Highlight.Value
	}
	if v.Baseline > 0 {
		r.Superscript = true
	} else if v.Baseline < 0 {
		r.Subscript = true
	}
	r.AllCaps = v.Caps == "all"
	r.SmallCaps = v.Caps == "small"
}

func shapeKind(kind string) ir.ShapeKind {
	switch kind {
	case "pic":
		return ir.ShapeImage
	case "grpSp":
		return ir.ShapeGroup
	case "cxnSp":
		return ir.ShapeConnector
	case "graphicFrame":
		return ir.ShapeOther
	default:
		return ir.ShapeText
	}
}
func placeholderKind(v string, idx ...int) ir.PlaceholderKind {
	switch v {
	case "title", "ctrTitle":
		return ir.PlaceholderTitle
	case "body":
		return ir.PlaceholderBody
	case "subTitle":
		return ir.PlaceholderSubtitle
	case "dt":
		return ir.PlaceholderDateTime
	case "sldNum":
		return ir.PlaceholderSlideNumber
	case "ftr":
		return ir.PlaceholderFooter
	case "hdr":
		return ir.PlaceholderHeader
	case "obj":
		return ir.PlaceholderContent
	case "chart":
		return ir.PlaceholderChart
	case "tbl":
		return ir.PlaceholderTable
	case "pic":
		return ir.PlaceholderPicture
	case "media":
		return ir.PlaceholderMedia
	case "clipArt":
		return ir.PlaceholderClipArt
	case "dgm":
		return ir.PlaceholderDiagram
	case "sldImg":
		return ir.PlaceholderSlideImage
	case "":
		// Match the shared OOXML cascade resolver: an authored p:ph with an
		// omitted type behaves as title for the default slot, and as body for
		// indexed placeholders. This keeps SAX placeholder inheritance aligned
		// with old PowerPoint decks that omit p:ph@type in layouts/masters.
		if len(idx) == 0 || idx[0] == 0 {
			return ir.PlaceholderTitle
		}
		return ir.PlaceholderBody
	default:
		return ir.PlaceholderOther
	}
}

// applyContextualInheritance resolves the visual subset whose semantics are
// unambiguous at the SAX boundary. It deliberately leaves text-style/theme
// cascade to later SP passes. Matching follows PresentationML placeholder
// identity: explicit idx first, semantic placeholder kind second.
func (b *irBuilder) applyContextualInheritance() {
	if b.slide == nil {
		return
	}
	master := &ir.SlideMaster{Shapes: cloneShapeSlice(b.masterShapes)}
	layout := &ir.SlideLayout{Master: master, Shapes: cloneShapeSlice(b.layoutShapes)}
	master.Layouts = []*ir.SlideLayout{layout}
	b.slide.Master, b.slide.Layout = master, layout
	// Resolve authored slide shapes against placeholder templates before adding
	// inherited artwork. Layout/master placeholders are templates only; non-
	// placeholder shapes are real paint content and must participate in the
	// effective slide scene in master -> layout -> slide order.
	for _, s := range b.slide.Shapes {
		inheritShapeTree(s, b.layoutShapes, b.masterShapes)
		b.applyMasterTextStyleTree(s)
		b.applyDefaultTextStyleTree(s)
		b.resolveThemeFontsTree(s)
	}
	authored := b.slide.Shapes
	effective := make([]*ir.Shape, 0, len(b.masterShapes)+len(b.layoutShapes)+len(authored))
	if b.showMasterShapes && b.layoutShowsMasterShapes {
		effective = appendDecorativeShapes(effective, b.masterShapes)
	}
	effective = appendDecorativeShapes(effective, b.layoutShapes)
	effective = append(effective, authored...)
	b.slide.Shapes = effective
}

// appendDecorativeShapes materializes inherited paint content while keeping
// placeholder shapes as inheritance templates. Each inherited shape is cloned
// because a master/layout can feed multiple slides and later physical/layout
// passes are allowed to annotate a slide-local shape graph.
func appendDecorativeShapes(dst []*ir.Shape, source []*ir.Shape) []*ir.Shape {
	for _, s := range source {
		if inherited := cloneInheritedPaintShape(s); inherited != nil {
			dst = append(dst, inherited)
		}
	}
	return dst
}

// cloneInheritedPaintShape materializes only paintable master/layout content.
// Placeholder/template semantics stay in the master/layout graph and are
// inherited by authored slide placeholders, but they must never become paint
// merely because they are nested inside a group. PowerPoint producers can also
// leave the built-in master editing prompts inside grouped artwork, so filtering
// only the top-level shape is insufficient.
func cloneInheritedPaintShape(s *ir.Shape) *ir.Shape {
	if s == nil || isMasterEditingPromptShape(s) {
		return nil
	}

	// A placeholder shape is an inheritance template, not paint. A group is
	// different: some producers put p:ph on the group/container while keeping
	// ordinary decorative pictures/shapes below it. Dropping the group before
	// inspecting its children loses real layout artwork (and, critically, its
	// group transform). Keep such a group only as a transform container, clear
	// its template identity, and recursively retain paintable descendants.
	if s.Placeholder != ir.PlaceholderNone && s.Kind != ir.ShapeGroup {
		return nil
	}

	cloned := cloneShape(s)
	if cloned == nil {
		return nil
	}
	if s.Placeholder != ir.PlaceholderNone {
		cloned.Placeholder = ir.PlaceholderNone
		cloned.PlaceholderIdx = 0
		cloned.Paragraphs = nil
	}
	if len(cloned.Children) == 0 {
		if cloned.Kind == ir.ShapeGroup {
			return nil
		}
		return cloned
	}

	children := make([]*ir.Shape, 0, len(cloned.Children))
	for _, child := range cloned.Children {
		if paintChild := cloneInheritedPaintShape(child); paintChild != nil {
			children = append(children, paintChild)
		}
	}
	cloned.Children = children

	// A group is only a coordinate/transform container. Once all of its
	// template children are removed it has no independent paint contribution.
	if cloned.Kind == ir.ShapeGroup && len(cloned.Children) == 0 {
		return nil
	}
	return cloned
}

// isMasterEditingPromptShape recognizes PowerPoint's built-in master editing
// instructions when they survive in a master/layout visual tree without a p:ph
// marker. These strings are authoring UI, not slide-show artwork. Some older or
// producer-generated files serialize the prompt as an ordinary text shape, so
// placeholder-only filtering is insufficient and leaks the template instructions
// onto every effective slide. Keep the recognition deliberately narrow: an
// inherited shape must contain the canonical master prompt itself.
func isMasterEditingPromptShape(s *ir.Shape) bool {
	if s == nil || s.Kind != ir.ShapeText {
		return false
	}
	for _, p := range s.Paragraphs {
		if p == nil {
			continue
		}
		for _, r := range p.Runs {
			if r == nil {
				continue
			}
			text := strings.TrimSpace(strings.ToLower(r.Text))
			if text == "click to edit master text styles" ||
				text == "click to edit master title style" ||
				text == "click to edit master subtitle style" {
				return true
			}
		}
	}
	return false
}

func applyTextBodyProps(s *ir.Shape, v TextBodyProperties) {
	p := s.TextBodyProps
	if v.Wrap != "" {
		p.Wrap = v.Wrap
	}
	if v.Anchor != "" {
		p.Anchor = v.Anchor
		s.TextAnchor = v.Anchor
	}
	if v.Vert != "" {
		p.Vert = v.Vert
	}
	if v.HasAnchorCtr {
		p.AnchorCtr = v.AnchorCtr
		p.HasAnchorCtr = true
	}
	if v.HasRot {
		p.Rot = v.Rot
		p.HasRot = true
	}
	// a:bodyPr owns numCol at this level. DrawingML defaults numCol to one
	// when the attribute is omitted; omission on an authored bodyPr must not
	// accidentally inherit a multi-column value from a layout/master
	// placeholder. That halves the physical line extent and causes premature
	// wrapping (for example two intended title lines becoming four).
	if v.HasColumns {
		p.Columns = v.Columns
	} else {
		p.Columns = 1
	}
	if v.ColumnSpacingEMU != 0 {
		p.ColumnSpacingEMU = v.ColumnSpacingEMU
	}
	if v.HasInsetT {
		p.InsetT = v.InsetT
	}
	if v.HasInsetB {
		p.InsetB = v.InsetB
	}
	if v.HasInsetL {
		p.InsetL = v.InsetL
	}
	if v.HasInsetR {
		p.InsetR = v.InsetR
	}
	if v.HasInsetT || v.HasInsetB || v.HasInsetL || v.HasInsetR {
		// An authored bodyPr may specify only some inset edges.  The other
		// edges still have DrawingML's physical text-frame defaults; zero is
		// not the omission default.  Materialize the complete rectangle here
		// so HasTextInsets can safely mean that all four canonical edges are
		// resolved (and an explicitly authored zero remains distinguishable).
		if !v.HasInsetT {
			p.InsetT = 45720
		}
		if !v.HasInsetB {
			p.InsetB = 45720
		}
		if !v.HasInsetL {
			p.InsetL = 91440
		}
		if !v.HasInsetR {
			p.InsetR = 91440
		}
		s.HasTextInsets = true
		s.TextInsetTopEMU, s.TextInsetBottomEMU, s.TextInsetLeftEMU, s.TextInsetRightEMU = p.InsetT, p.InsetB, p.InsetL, p.InsetR
	}
	if v.AutofitType != "" {
		p.AutofitType = v.AutofitType
		s.TextAutoGrow = v.AutofitType == "spAutoFit"
		s.TextAllowOverflow = v.AutofitType == "noAutofit"
	}
	if v.AutofitFontScale != 0 {
		p.AutofitFontScale = v.AutofitFontScale
	}
	if v.AutofitLineSpaceReduction != 0 {
		p.AutofitLineSpaceReduction = v.AutofitLineSpaceReduction
	}
	s.TextBodyProps = p
}

func (b *irBuilder) applyDefaultTextStyleTree(s *ir.Shape) {
	if s == nil {
		return
	}
	for _, p := range s.Paragraphs {
		if p == nil {
			continue
		}
		style, ok := b.defaultTextStyles[p.Level]
		if !ok {
			style, ok = b.defaultTextStyles[0]
		}
		if ok {
			b.inheritParagraphStyle(p, style.Paragraph)
			for _, r := range p.Runs {
				b.inheritRunStyle(r, style.Run)
			}
		}
	}
	// Table text has no placeholder chain, but it still participates in the
	// presentation text-default cascade. PowerPoint also uses 18pt as the
	// application default table run size when the package supplies no explicit
	// size at any level. Materialize that here so layout and HTML consume the
	// same physical typography instead of falling back to the renderer's 12px.
	if s.Table != nil {
		for _, row := range s.Table.Rows {
			if row == nil {
				continue
			}
			for _, cell := range row.Cells {
				if cell == nil {
					continue
				}
				for _, p := range cell.Paragraphs {
					if p == nil {
						continue
					}
					style, ok := b.defaultTextStyles[p.Level]
					if !ok {
						style, ok = b.defaultTextStyles[0]
					}
					if ok {
						b.inheritParagraphStyle(p, style.Paragraph)
					}
					for _, r := range p.Runs {
						if r == nil {
							continue
						}
						if ok {
							b.inheritRunStyle(r, style.Run)
						}
						if r.FontSize == 0 {
							r.FontSize = 1800
						}
					}
				}
			}
		}
	}
	for _, child := range s.Children {
		b.applyDefaultTextStyleTree(child)
	}
}

func (b *irBuilder) resolveThemeFontsTree(s *ir.Shape) {
	if s == nil {
		return
	}
	for _, p := range s.Paragraphs {
		if p != nil {
			for _, r := range p.Runs {
				if r != nil {
					r.FontFamily = b.resolveThemeTypeface(r.FontFamily, s.Placeholder)
				}
			}
			p.BulletFontFamily = b.resolveThemeTypeface(p.BulletFontFamily, s.Placeholder)
		}
	}
	if s.Table != nil {
		for _, row := range s.Table.Rows {
			if row == nil {
				continue
			}
			for _, cell := range row.Cells {
				if cell == nil {
					continue
				}
				for _, p := range cell.Paragraphs {
					if p == nil {
						continue
					}
					for _, r := range p.Runs {
						if r != nil {
							r.FontFamily = b.resolveThemeTypeface(r.FontFamily, ir.PlaceholderNone)
						}
					}
					p.BulletFontFamily = b.resolveThemeTypeface(p.BulletFontFamily, ir.PlaceholderNone)
				}
			}
		}
	}
	for _, child := range s.Children {
		b.resolveThemeFontsTree(child)
	}
}
func (b *irBuilder) resolveThemeTypeface(v string, ph ir.PlaceholderKind) string {
	if v == "+mj-lt" {
		return b.themeFonts["majorFont:latin"]
	}
	if v == "+mn-lt" {
		return b.themeFonts["minorFont:latin"]
	}
	if v != "" {
		return v
	}
	if ph == ir.PlaceholderTitle {
		return b.themeFonts["majorFont:latin"]
	}
	return b.themeFonts["minorFont:latin"]
}

func (b *irBuilder) applyMasterTextStyleTree(s *ir.Shape) {
	if s == nil {
		return
	}
	if s.Placeholder == ir.PlaceholderNone {
		for _, child := range s.Children {
			b.applyMasterTextStyleTree(child)
		}
		return
	}
	kind := "otherStyle"
	switch s.Placeholder {
	case ir.PlaceholderTitle:
		kind = "titleStyle"
	case ir.PlaceholderSubtitle:
		kind = "bodyStyle"
	case ir.PlaceholderBody, ir.PlaceholderContent, ir.PlaceholderChart, ir.PlaceholderTable, ir.PlaceholderPicture, ir.PlaceholderMedia, ir.PlaceholderClipArt, ir.PlaceholderDiagram:
		kind = "bodyStyle"
	}
	levels := b.masterTextStyles[kind]
	for _, p := range s.Paragraphs {
		if p == nil {
			continue
		}
		level := p.Level
		style, ok := levels[level]
		if !ok {
			style, ok = levels[0]
		}
		if !ok {
			continue
		}
		b.inheritParagraphStyle(p, style.Paragraph)
		for _, r := range p.Runs {
			b.inheritRunStyle(r, style.Run)
		}
	}
	for _, child := range s.Children {
		b.applyMasterTextStyleTree(child)
	}
}

func (b *irBuilder) inheritParagraphStyle(p *ir.Paragraph, v ParagraphProperties) {
	if p == nil {
		return
	}
	if p.Align == "" {
		p.Align = v.Align
	}
	if !p.HasMarginLeft && v.HasMarginLeft {
		p.MarginLeftEMU, p.HasMarginLeft = v.MarginLeftEMU, true
	}
	if !p.HasMarginRight && v.HasMarginRight {
		p.MarginRightEMU, p.HasMarginRight = v.MarginRightEMU, true
	}
	if !p.HasTextIndent && v.HasTextIndent {
		p.TextIndentEMU, p.HasTextIndent = v.TextIndentEMU, true
	}
	if p.DefaultTabSizeEMU == 0 && v.HasDefaultTabSize {
		p.DefaultTabSizeEMU = v.DefaultTabSizeEMU
	}
	if p.FontAlign == "" {
		p.FontAlign = v.FontAlign
	}
	if p.BulletChar == "" && !p.AutoNum && !p.BulletNone {
		if v.HasBullet {
			p.BulletChar = v.BulletChar
			p.BulletNone = v.BulletChar == "" && !v.AutoNum
		}
		if v.HasAutoNum {
			p.AutoNum, p.AutoNumType, p.AutoNumStart = v.AutoNum, v.AutoNumType, v.AutoNumStart
			if v.AutoNum {
				p.BulletNone = false
			}
		}
	}
	if !p.BulletFontTx && p.BulletFontFamily == "" {
		if v.BulletFontTx {
			p.BulletFontTx = true
		} else {
			p.BulletFontFamily = v.BulletFontFamily
		}
	}
	if p.BulletSize == 0 {
		p.BulletSize = v.BulletSize
	}
	if p.BulletSizePct == 0 {
		p.BulletSizePct = v.BulletSizePct
	}
	if p.BulletColor == "" {
		p.BulletColor = b.resolveVisualColor(v.BulletColor)
	}
	if !p.HasSpaceBefore && v.HasSpaceBefore {
		p.SpaceBefore, p.SpaceBeforePct, p.HasSpaceBefore = v.SpaceBefore, v.SpaceBeforePct, true
	}
	if !p.HasSpaceAfter && v.HasSpaceAfter {
		p.SpaceAfter, p.SpaceAfterPct, p.HasSpaceAfter = v.SpaceAfter, v.SpaceAfterPct, true
	}
	if !p.HasLineSpacing && v.HasLineSpacing {
		p.LineSpacingPct, p.LineSpacingPt, p.HasLineSpacing = v.LineSpacingPct, v.LineSpacingPt, true
	}
}

func (b *irBuilder) inheritRunStyle(r *ir.Run, v RunProperties) {
	if r == nil {
		return
	}
	if !r.BoldSet && v.BoldSet {
		r.Bold, r.BoldSet = v.Bold, true
	}
	if !r.ItalicSet && v.ItalicSet {
		r.Italic, r.ItalicSet = v.Italic, true
	}
	if !r.UnderlineSet && v.UnderlineSet {
		r.Underline, r.UnderlineSet = v.Underline, true
	}
	if !r.StrikeSet && v.StrikeSet {
		r.Strike, r.StrikeSet = v.Strike, true
	}
	if r.FontSize == 0 {
		r.FontSize = v.FontSize
	}
	if r.FontFamily == "" {
		r.FontFamily = v.FontFamily
	}
	if r.Color == "" {
		r.Color = b.resolveVisualColor(v.Color)
	}
	if r.Highlight == "" {
		r.Highlight = b.resolveVisualColor(v.Highlight)
	}
}

func inheritShapeTree(s *ir.Shape, layout, master []*ir.Shape) {
	if s == nil {
		return
	}
	if s.Placeholder != ir.PlaceholderNone {
		if l := findPlaceholder(layout, s); l != nil {
			inheritVisualFacts(s, l)
		}
		if m := findPlaceholder(master, s); m != nil {
			inheritVisualFacts(s, m)
		}
	}
	for _, child := range s.Children {
		inheritShapeTree(child, layout, master)
	}
}

func findPlaceholder(shapes []*ir.Shape, target *ir.Shape) *ir.Shape {
	var kindMatch *ir.Shape
	var walk func([]*ir.Shape)
	walk = func(items []*ir.Shape) {
		for _, candidate := range items {
			if candidate == nil {
				continue
			}
			if target.PlaceholderIdx != 0 && candidate.PlaceholderIdx == target.PlaceholderIdx && placeholderKindsCompatible(target.Placeholder, candidate.Placeholder) {
				kindMatch = candidate
				return
			}
			if kindMatch == nil && placeholderKindsCompatible(target.Placeholder, candidate.Placeholder) {
				kindMatch = candidate
			}
			if kindMatch == nil {
				walk(candidate.Children)
			}
			if kindMatch != nil && target.PlaceholderIdx != 0 && kindMatch.PlaceholderIdx == target.PlaceholderIdx {
				return
			}
		}
	}
	walk(shapes)
	return kindMatch
}

func placeholderKindsCompatible(target, candidate ir.PlaceholderKind) bool {
	if target == ir.PlaceholderNone || candidate == ir.PlaceholderNone {
		return false
	}
	if target == candidate {
		return true
	}
	// Generic object/content placeholders and body placeholders share the
	// content text/style role. Subtitle placeholders also share body text style
	// inheritance when a master has no dedicated subtitle placeholder. They must
	// never match date/footer/slide-number placeholders merely because p:ph@idx
	// collides across layout/master.
	contentLike := func(k ir.PlaceholderKind) bool {
		return k == ir.PlaceholderContent || k == ir.PlaceholderBody || k == ir.PlaceholderSubtitle
	}
	return contentLike(target) && contentLike(candidate)
}

func inheritVisualFacts(dst, src *ir.Shape) {
	if dst == nil || src == nil {
		return
	}
	// PresentationML placeholder transforms inherit as a unit. An absent a:xfrm
	// means "use the template transform"; an authored a:xfrm, even when its
	// values are zero, is an override. Preserve that distinction explicitly so
	// rotation/flips are not lost after bounds are copied.
	if !dst.HasTransform && src.HasTransform {
		dst.Bounds = src.Bounds
		dst.RotationDeg, dst.FlipH, dst.FlipV = src.RotationDeg, src.FlipH, src.FlipV
		dst.HasTransform = true
	} else if !dst.HasTransform && rectEmpty(dst.Bounds) {
		// Compatibility for canonical templates produced by older callers that
		// predate transform provenance.
		dst.Bounds = src.Bounds
		dst.RotationDeg, dst.FlipH, dst.FlipV = src.RotationDeg, src.FlipH, src.FlipV
	}
	if dst.Geometry == "" {
		dst.Geometry = src.Geometry
	}
	if dst.FillColor == "" {
		dst.FillColor = src.FillColor
	}
	if dst.LineColor == "" {
		dst.LineColor = src.LineColor
	}
	if dst.LineWidthPt == 0 {
		dst.LineWidthPt = src.LineWidthPt
	}
	if dst.PlaceholderOrient == "" {
		dst.PlaceholderOrient = src.PlaceholderOrient
	}
	if dst.PlaceholderSize == "" {
		dst.PlaceholderSize = src.PlaceholderSize
	}
	inheritTextBodyFacts(dst, src)
	inheritTextFacts(dst, src)
}

func inheritTextBodyFacts(dst, src *ir.Shape) {
	if dst == nil || src == nil {
		return
	}
	d, x := &dst.TextBodyProps, src.TextBodyProps
	if d.Wrap == "" {
		d.Wrap = x.Wrap
	}
	if d.Anchor == "" {
		d.Anchor = x.Anchor
		if dst.TextAnchor == "" {
			dst.TextAnchor = src.TextAnchor
		}
	}
	if d.Vert == "" {
		d.Vert = x.Vert
	}
	if !d.HasAnchorCtr && x.HasAnchorCtr {
		d.AnchorCtr = x.AnchorCtr
		d.HasAnchorCtr = true
	}
	if !d.HasRot && x.HasRot {
		d.Rot = x.Rot
		d.HasRot = true
	}
	if d.Columns == 0 {
		d.Columns = x.Columns
	}
	if d.ColumnSpacingEMU == 0 {
		d.ColumnSpacingEMU = x.ColumnSpacingEMU
	}
	if !dst.HasTextInsets && src.HasTextInsets {
		dst.HasTextInsets = true
		d.InsetT, d.InsetB, d.InsetL, d.InsetR = x.InsetT, x.InsetB, x.InsetL, x.InsetR
		dst.TextInsetTopEMU, dst.TextInsetBottomEMU, dst.TextInsetLeftEMU, dst.TextInsetRightEMU = src.TextInsetTopEMU, src.TextInsetBottomEMU, src.TextInsetLeftEMU, src.TextInsetRightEMU
	}
	if d.AutofitType == "" {
		d.AutofitType = x.AutofitType
		d.AutofitFontScale = x.AutofitFontScale
		d.AutofitLineSpaceReduction = x.AutofitLineSpaceReduction
		dst.TextAutoGrow = src.TextAutoGrow
		dst.TextAllowOverflow = src.TextAllowOverflow
	}
}

func inheritTextFacts(dst, src *ir.Shape) {
	if dst == nil || src == nil || len(dst.Paragraphs) == 0 || len(src.Paragraphs) == 0 {
		return
	}
	for i, dp := range dst.Paragraphs {
		if dp == nil {
			continue
		}
		sp := src.Paragraphs[0]
		if i < len(src.Paragraphs) && src.Paragraphs[i] != nil {
			sp = src.Paragraphs[i]
		}
		if sp == nil {
			continue
		}
		// Placeholder list styles are part of the placeholder's effective text
		// style, not just a source for font/color. Carry the complete paragraph
		// contract before the master/default text-style fallback runs. In
		// particular, line spacing is visually significant: sample-3's subtitle
		// inherits 115% from the master's body placeholder while the authored
		// slide paragraph only overrides its 16pt space-after. Dropping the 115%
		// here makes every downstream physical line and vertical anchor wrong.
		inheritParagraphFacts(dp, sp)
		var sr *ir.Run
		if len(sp.Runs) > 0 {
			sr = sp.Runs[0]
		}
		if sr == nil {
			continue
		}
		for _, dr := range dp.Runs {
			inheritRunFacts(dr, sr)
		}
	}
}

func inheritParagraphFacts(dst, src *ir.Paragraph) {
	if dst == nil || src == nil {
		return
	}
	if dst.Align == "" {
		dst.Align = src.Align
	}
	if dst.TextDirection == "" {
		dst.TextDirection = src.TextDirection
	}
	if !dst.HasMarginLeft && src.HasMarginLeft {
		dst.MarginLeftEMU, dst.HasMarginLeft = src.MarginLeftEMU, true
	}
	if !dst.HasMarginRight && src.HasMarginRight {
		dst.MarginRightEMU, dst.HasMarginRight = src.MarginRightEMU, true
	}
	if !dst.HasTextIndent && src.HasTextIndent {
		dst.TextIndentEMU, dst.HasTextIndent = src.TextIndentEMU, true
	}
	if dst.DefaultTabSizeEMU == 0 {
		dst.DefaultTabSizeEMU = src.DefaultTabSizeEMU
	}
	if len(dst.TabStops) == 0 && len(src.TabStops) != 0 {
		dst.TabStops = append([]ir.TabStop(nil), src.TabStops...)
	}
	if dst.FontAlign == "" {
		dst.FontAlign = src.FontAlign
	}

	// Explicit buNone is an override and must survive all later cascade stages.
	if src.BulletNone && dst.BulletChar == "" && !dst.AutoNum && !dst.BulletNone {
		dst.BulletNone = true
		dst.BulletChar = ""
		dst.AutoNum = false
		dst.AutoNumType = ""
		dst.AutoNumStart = 0
	} else if dst.BulletChar == "" && !dst.AutoNum && !dst.BulletNone {
		if src.AutoNum {
			dst.AutoNum, dst.AutoNumType, dst.AutoNumStart = true, src.AutoNumType, src.AutoNumStart
		} else if src.BulletChar != "" {
			dst.BulletChar = src.BulletChar
		}
	}
	if !dst.BulletFontTx && dst.BulletFontFamily == "" {
		if src.BulletFontTx {
			dst.BulletFontTx = true
		} else {
			dst.BulletFontFamily = src.BulletFontFamily
		}
	}
	if dst.BulletColor == "" {
		dst.BulletColor = src.BulletColor
	}
	if dst.BulletSize == 0 {
		dst.BulletSize = src.BulletSize
	}
	if dst.BulletSizePct == 0 {
		dst.BulletSizePct = src.BulletSizePct
	}

	if !dst.HasSpaceBefore && src.HasSpaceBefore {
		dst.SpaceBefore, dst.SpaceBeforePct, dst.HasSpaceBefore = src.SpaceBefore, src.SpaceBeforePct, true
	} else if !dst.HasSpaceBefore && dst.SpaceBeforePct == 0 && src.SpaceBeforePct != 0 {
		dst.SpaceBeforePct = src.SpaceBeforePct
	}
	if !dst.HasSpaceAfter && src.HasSpaceAfter {
		dst.SpaceAfter, dst.SpaceAfterPct, dst.HasSpaceAfter = src.SpaceAfter, src.SpaceAfterPct, true
	} else if !dst.HasSpaceAfter && dst.SpaceAfterPct == 0 && src.SpaceAfterPct != 0 {
		dst.SpaceAfterPct = src.SpaceAfterPct
	}
	if !dst.HasLineSpacing && src.HasLineSpacing {
		dst.LineSpacingPct, dst.LineSpacingPt, dst.HasLineSpacing = src.LineSpacingPct, src.LineSpacingPt, true
	}
}

func inheritRunFacts(dst, src *ir.Run) {
	if dst == nil || src == nil {
		return
	}
	if !dst.BoldSet && src.BoldSet {
		dst.Bold, dst.BoldSet = src.Bold, true
	}
	if !dst.ItalicSet && src.ItalicSet {
		dst.Italic, dst.ItalicSet = src.Italic, true
	}
	if !dst.UnderlineSet && src.UnderlineSet {
		dst.Underline, dst.UnderlineSet = src.Underline, true
	}
	if !dst.StrikeSet && src.StrikeSet {
		dst.Strike, dst.StrikeSet = src.Strike, true
	}
	if dst.FontSize == 0 {
		dst.FontSize = src.FontSize
	}
	if dst.FontFamily == "" {
		dst.FontFamily = src.FontFamily
	}
	if dst.Color == "" {
		dst.Color = src.Color
	}
	if dst.Highlight == "" {
		dst.Highlight = src.Highlight
	}
}

func rectEmpty(r ir.Rect) bool { return r.X == 0 && r.Y == 0 && r.W == 0 && r.H == 0 }

func cloneShapeSlice(in []*ir.Shape) []*ir.Shape {
	out := make([]*ir.Shape, 0, len(in))
	for _, s := range in {
		out = append(out, cloneShape(s))
	}
	return out
}
func cloneShape(s *ir.Shape) *ir.Shape {
	if s == nil {
		return nil
	}
	c := *s
	c.Children = cloneShapeSlice(s.Children)
	c.Paragraphs = cloneParagraphs(s.Paragraphs)
	return &c
}

func cloneParagraphs(in []*ir.Paragraph) []*ir.Paragraph {
	out := make([]*ir.Paragraph, 0, len(in))
	for _, p := range in {
		if p == nil {
			out = append(out, nil)
			continue
		}
		cp := *p
		cp.Runs = make([]*ir.Run, 0, len(p.Runs))
		for _, r := range p.Runs {
			if r == nil {
				cp.Runs = append(cp.Runs, nil)
			} else {
				cr := *r
				cp.Runs = append(cp.Runs, &cr)
			}
		}
		out = append(out, &cp)
	}
	return out
}

func (b *irBuilder) resolveVisualColor(c VisualColor) string {
	color := c.Value
	if c.Kind == "schemeClr" {
		slot := c.Value
		var aliases map[string]string
		switch b.activeColorScope {
		case "master":
			aliases = b.masterColorMap
		case "layout":
			aliases = b.layoutColorMap
		default:
			aliases = b.slideColorMap
		}
		if mapped := aliases[slot]; mapped != "" {
			slot = mapped
		}
		color = b.themeColors[slot]
	}
	if color == "" {
		return ""
	}

	// Apply transforms after scheme/clrMap resolution. This is especially
	// important for fmtScheme entries: the same phClr base is intentionally
	// reused with tint/shade/luminance transforms to produce distinct artwork.
	f := drawing.SolidFill{Color: color}
	if c.HasShade {
		f.Shade, f.HasShade = float64(c.Shade)/100000.0, true
	}
	if c.HasTint {
		f.Tint, f.HasTint = float64(c.Tint)/100000.0, true
	}
	if c.HasLumMod {
		f.LumMod = float64(c.LumMod) / 100000.0
	}
	if c.HasLumOff {
		f.LumOff = float64(c.LumOff) / 100000.0
	}
	if c.HasSatMod {
		f.SatMod = float64(c.SatMod) / 100000.0
	}
	if c.HasSatOff {
		f.SatOff = float64(c.SatOff) / 100000.0
	}
	return f.ApplyColorTransforms(color)
}

func cloneStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func saxArrowhead(kind, width, length string) *drawing.Arrowhead {
	if kind == "" || kind == "none" {
		return nil
	}
	a := &drawing.Arrowhead{Type: drawing.ArrowheadType(kind), Size: drawing.ArrowheadSize(width), Length: drawing.ArrowheadSize(length)}
	if a.Size == "" {
		a.Size = drawing.ArrowheadSizeMedium
	}
	if a.Length == "" {
		a.Length = drawing.ArrowheadSizeMedium
	}
	return a
}

func (b *irBuilder) materializeEffect(v EffectProperties) drawing.Effect {
	color := b.resolveVisualColor(v.Color)
	alpha := 1.0
	if v.Color.HasAlpha {
		alpha = float64(v.Color.Alpha) / 100000.0
	}
	pt := func(x int64) float64 { return float64(x) / 12700.0 }
	deg := func(x int64) float64 { return float64(x) / 60000.0 }
	switch v.Kind {
	case "outerShdw":
		return drawing.Shadow{Color: color, Alpha: alpha, Blur: pt(v.BlurRad), Distance: pt(v.Dist), Angle: deg(v.Dir), ScaleX: float64(v.SX) / 100000.0, ScaleY: float64(v.SY) / 100000.0, SkewX: float64(v.KX) / 60000.0, SkewY: float64(v.KY) / 60000.0, Alignment: v.Align, Align: v.Align != "", RotateWithShape: v.RotateWithShape}
	case "innerShdw":
		return drawing.InnerShadow{Color: color, Alpha: alpha, Blur: pt(v.BlurRad), Distance: pt(v.Dist), Angle: deg(v.Dir)}
	case "glow":
		return drawing.Glow{Color: color, Alpha: alpha, Radius: pt(v.Rad)}
	case "softEdge":
		return drawing.SoftEdge{Radius: pt(v.Rad)}
	case "reflection":
		return drawing.Reflection{Alpha: alpha, Blur: pt(v.BlurRad), Distance: pt(v.Dist), Direction: deg(v.Dir), StartAlpha: v.StartAlpha, StartPosition: v.StartPos, EndAlpha: v.EndAlpha, EndPosition: v.EndPos, FadeDirection: float64(v.FadeDir) / 60000.0, ScaleX: float64(v.SX) / 100000.0, ScaleY: float64(v.SY) / 100000.0, SkewX: float64(v.KX) / 60000.0, SkewY: float64(v.KY) / 60000.0, Alignment: v.Align, RotateWithShape: v.RotateWithShape}
	case "blur":
		return drawing.Blur{Radius: pt(v.Rad), Grow: v.Grow}
	}
	return nil
}

// finalizeCanonicalTheme materializes the presentation-level theme directly from
// facts streamed out of theme XML. This removes the old composer as the owner of
// the canonical color/font theme for SAX-capable presentations.
func (b *irBuilder) finalizeCanonicalTheme() {
	if len(b.themeColors) == 0 && len(b.themeFonts) == 0 {
		return
	}
	colors := make(map[string]string, len(b.themeColors))
	for k, v := range b.themeColors {
		colors[k] = v
	}
	b.presentation.Theme = &ir.Theme{
		Name:        b.themeName,
		ColorScheme: colors,
		MajorFont:   b.themeFonts["majorFont:latin"],
		MinorFont:   b.themeFonts["minorFont:latin"],
		MajorEAFont: b.themeFonts["majorFont:ea"],
		MinorEAFont: b.themeFonts["minorFont:ea"],
		MajorCSFont: b.themeFonts["majorFont:cs"],
		MinorCSFont: b.themeFonts["minorFont:cs"],
	}
}

func (b *irBuilder) resolveConnectorEndpoints() {
	if b.slide == nil || len(b.connectorRefs) == 0 {
		return
	}
	byID := map[int]*ir.Shape{}
	var visit func([]*ir.Shape)
	visit = func(shapes []*ir.Shape) {
		for _, s := range shapes {
			if s == nil {
				continue
			}
			byID[s.ID] = s
			visit(s.Children)
		}
	}
	visit(b.slide.Shapes)
	point := func(ref connectorRef) *ir.Point {
		if !ref.set {
			return nil
		}
		t := byID[ref.shapeID]
		if t == nil {
			return nil
		}
		var p ir.Point
		switch ref.siteIdx {
		case 0:
			p = ir.Point{X: t.Bounds.X + t.Bounds.W/2, Y: t.Bounds.Y}
		case 1:
			p = ir.Point{X: t.Bounds.X + t.Bounds.W, Y: t.Bounds.Y + t.Bounds.H/2}
		case 2:
			p = ir.Point{X: t.Bounds.X + t.Bounds.W/2, Y: t.Bounds.Y + t.Bounds.H}
		case 3:
			p = ir.Point{X: t.Bounds.X, Y: t.Bounds.Y + t.Bounds.H/2}
		default:
			p = ir.Point{X: t.Bounds.X + t.Bounds.W/2, Y: t.Bounds.Y + t.Bounds.H/2}
		}
		return &p
	}
	for s, refs := range b.connectorRefs {
		if refs.start.set {
			s.ConnectorStart = point(refs.start)
		}
		if refs.end.set {
			s.ConnectorEnd = point(refs.end)
		}
	}
}

func normalizeActionLink(action string) string {
	const prefix = "ppaction://"
	if !strings.HasPrefix(action, prefix) {
		return action
	}
	v := strings.TrimPrefix(action, prefix)
	if i := strings.Index(v, "jump="); i >= 0 {
		j := v[i+5:]
		if k := strings.IndexByte(j, '&'); k >= 0 {
			j = j[:k]
		}
		switch j {
		case "firstSlide", "lastSlide", "nextSlide", "prevSlide", "endShow":
			return j
		}
	}
	if i := strings.Index(v, "slidenum="); i >= 0 {
		n := v[i+9:]
		if k := strings.IndexByte(n, '&'); k >= 0 {
			n = n[:k]
		}
		if n != "" {
			return "slide" + n
		}
	}
	return action
}

func (b *irBuilder) themeStyleColor(token, placeholder VisualColor) VisualColor {
	if token.Value != "phClr" {
		return token
	}

	// phClr supplies the base color for an fmtScheme entry; it does not replace
	// the transforms authored on that entry. For example, Office themes commonly
	// define every stop of a background gradient as phClr plus a different
	// tint/shade/saturation transform. Replacing the whole token collapses those
	// stops to one flat color. Preserve the style token and substitute only its
	// color identity.
	out := token
	out.Kind = placeholder.Kind
	out.Value = placeholder.Value
	if !out.HasAlpha && placeholder.HasAlpha {
		out.Alpha, out.HasAlpha = placeholder.Alpha, true
	}
	return out
}

func (b *irBuilder) applyThemeFillStyle(s *ir.Shape, style ThemeFillStyle, placeholder VisualColor) {
	switch style.Kind {
	case "none":
		s.Fill = drawing.NoFill{}
		s.FillColor = ""
	case "solid":
		token := b.themeStyleColor(style.Color, placeholder)
		color := b.resolveVisualColor(token)
		if color == "" {
			return
		}
		alpha := 1.0
		if token.HasAlpha {
			alpha = float64(token.Alpha) / 100000.0
		}
		s.FillColor = color
		s.Fill = drawing.SolidFill{Color: color, Alpha: alpha}
	case "gradient":
		g := drawing.GradientFill{GradientType: drawing.GradientTypeLinear, Angle: float64(style.Gradient.Angle) / 60000.0, Flip: style.Gradient.Flip, RotateWithShape: style.Gradient.RotateWithShape}
		if style.Gradient.Path != "" {
			g.GradientType = drawing.GradientTypePath
		}
		if style.Gradient.HasFillToRect {
			l := float64(style.Gradient.FillToLeft) / 100000.0
			t := float64(style.Gradient.FillToTop) / 100000.0
			r := float64(style.Gradient.FillToRight) / 100000.0
			btm := float64(style.Gradient.FillToBottom) / 100000.0
			g.FillToRect = &drawing.Rect{X: l, Y: t, Width: 1 - l - r, Height: 1 - t - btm}
		}
		for _, st := range style.Gradient.Stops {
			token := b.themeStyleColor(st.Color, placeholder)
			alpha := 1.0
			if token.HasAlpha {
				alpha = float64(token.Alpha) / 100000.0
			}
			g.Stops = append(g.Stops, drawing.GradientStop{Position: float64(st.Position) / 100000.0, Color: b.resolveVisualColor(token), Alpha: alpha})
		}
		s.Fill = g
		s.FillColor = ""
	case "pattern":
		fg := b.themeStyleColor(style.Pattern.Foreground, placeholder)
		bg := b.themeStyleColor(style.Pattern.Background, placeholder)
		fa, ba := 1.0, 1.0
		if fg.HasAlpha {
			fa = float64(fg.Alpha) / 100000.0
		}
		if bg.HasAlpha {
			ba = float64(bg.Alpha) / 100000.0
		}
		s.Fill = drawing.PatternFill{PatternType: drawing.PatternType(style.Pattern.Preset), Foreground: b.resolveVisualColor(fg), Background: b.resolveVisualColor(bg), ForeAlpha: fa, BackAlpha: ba}
		s.FillColor = ""
	}
}
