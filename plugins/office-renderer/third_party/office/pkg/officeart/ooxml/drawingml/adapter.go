package drawingml

import (
	"math"

	"office/pkg/officeart/ir"
	dml "office/pkg/ooxml/drawing"
)

// ShapeSpec is the format-neutral OOXML drawing input used by DOCX/XLSX/PPTX
// adapters before conversion into the shared OfficeArt IR.
type ShapeSpec struct {
	ID     uint32
	Name   string
	Kind   ir.ShapeKind
	Bounds ir.Rect

	RotationDeg float64
	FlipH       bool
	FlipV       bool

	Geometry *dml.Geometry
	Preset   *dml.PresetGeometry
	Fill     dml.Fill
	Line     *dml.LineStyle
	Effects  dml.EffectList
	Text     *dml.TextBody
	Image    *dml.PictureFill

	Connector *ConnectorSpec

	Children []*ShapeSpec
	Source   ir.SourceRef
}

type ConnectorSpec struct {
	Type        string
	Adjustments []int
	Start       *ir.Point
	End         *ir.Point
}

func Shape(spec ShapeSpec) *ir.Shape {
	kind := spec.Kind
	if kind == "" {
		kind = inferKind(spec)
	}

	out := &ir.Shape{
		ID:   spec.ID,
		Name: spec.Name,
		Kind: kind,
		Transform: ir.Transform{
			Rect:        spec.Bounds,
			RotationDeg: spec.RotationDeg,
			FlipH:       spec.FlipH,
			FlipV:       spec.FlipV,
		},
		Geometry: Geometry(spec.Geometry, spec.Preset),
		Fill:     Fill(spec.Fill),
		Effects:  Effects(spec.Effects),
		Source:   spec.Source,
	}
	if spec.Line != nil {
		out.Line = Line(*spec.Line)
	}
	if spec.Text != nil {
		out.Text = TextBody(*spec.Text)
	}
	if spec.Image != nil {
		out.Image = Picture(*spec.Image)
	}
	if spec.Connector != nil {
		out.Connector = Connector(*spec.Connector, spec.Line)
		if out.Kind == ir.ShapeUnknown || out.Kind == ir.ShapeAuto {
			out.Kind = ir.ShapeConnector
		}
	}
	if len(spec.Children) > 0 {
		out.Children = make([]*ir.Shape, 0, len(spec.Children))
		for _, child := range spec.Children {
			if child == nil {
				continue
			}
			out.Children = append(out.Children, Shape(*child))
		}
		if out.Kind == ir.ShapeUnknown || out.Kind == ir.ShapeAuto {
			out.Kind = ir.ShapeGroup
		}
	}
	return out
}

func inferKind(spec ShapeSpec) ir.ShapeKind {
	if len(spec.Children) > 0 {
		return ir.ShapeGroup
	}
	if spec.Connector != nil {
		return ir.ShapeConnector
	}
	if spec.Image != nil {
		return ir.ShapePicture
	}
	if spec.Text != nil && spec.Geometry == nil && spec.Preset == nil {
		return ir.ShapeTextBox
	}
	return ir.ShapeAuto
}

func Fill(fill dml.Fill) ir.Fill {
	if fill == nil {
		return ir.Fill{}
	}
	switch f := fill.(type) {
	case dml.NoFill:
		return ir.Fill{Kind: ir.FillNone}
	case dml.SolidFill:
		alpha := f.ResolvedAlpha()
		if alpha == 0 {
			alpha = 1
		}
		return ir.Fill{
			Kind:  ir.FillSolid,
			Color: f.ApplyColorTransforms(f.Color),
			Alpha: alpha,
		}
	case dml.GradientFill:
		stops := make([]ir.GradientStop, 0, len(f.Stops))
		for _, stop := range f.Stops {
			stops = append(stops, ir.GradientStop{
				Position: stop.Position,
				Color:    stop.Color,
				Alpha:    normalizedAlpha(stop.Alpha),
			})
		}
		return ir.Fill{
			Kind: ir.FillGradient,
			Gradient: &ir.GradientFill{
				Type:  string(f.GradientType),
				Angle: f.Angle,
				Stops: stops,
			},
		}
	case dml.PatternFill:
		return ir.Fill{
			Kind: ir.FillPattern,
			Pattern: &ir.PatternFill{
				Preset:     string(f.PatternType),
				Foreground: f.Foreground,
				Background: f.Background,
			},
		}
	case dml.PictureFill:
		picture := Picture(f)
		pf := &ir.PictureFill{Image: *picture}
		if f.Tile {
			pf.Tile = &ir.Tile{
				OffsetXEMU: f.TileOffsetX,
				OffsetYEMU: f.TileOffsetY,
				ScaleX:     percent100000(f.TileScaleX),
				ScaleY:     percent100000(f.TileScaleY),
			}
		}
		return ir.Fill{Kind: ir.FillPicture, Picture: pf}
	default:
		return ir.Fill{}
	}
}

func Line(line dml.LineStyle) ir.Line {
	return ir.Line{
		Visible:    true,
		Color:      line.Color,
		Alpha:      normalizedAlpha(line.Alpha),
		WidthPt:    line.Width,
		Dash:       string(line.Dash),
		Cap:        string(line.Cap),
		Join:       string(line.Join),
		StartArrow: Arrow(line.HeadEnd),
		EndArrow:   Arrow(line.TailEnd),
	}
}

func Arrow(arrow *dml.Arrowhead) ir.Arrow {
	if arrow == nil {
		return ir.Arrow{}
	}
	return ir.Arrow{
		Type:   string(arrow.Type),
		Width:  string(arrow.Size),
		Length: string(arrow.Length),
	}
}

func Picture(pic dml.PictureFill) *ir.Image {
	out := &ir.Image{
		Data:     append([]byte(nil), pic.ImageData...),
		MIMEType: pic.ContentType,
		RelID:    pic.EmbedID,
		URL:      pic.ImageURL,
		DPI:      pic.DPI,
	}
	if pic.Crop != nil {
		out.Crop = ir.Crop{
			Left:   clamp01(pic.Crop.X),
			Top:    clamp01(pic.Crop.Y),
			Right:  clamp01(1 - (pic.Crop.X + pic.Crop.Width)),
			Bottom: clamp01(1 - (pic.Crop.Y + pic.Crop.Height)),
		}
	}
	return out
}

func Geometry(geometry *dml.Geometry, preset *dml.PresetGeometry) ir.Geometry {
	if preset != nil {
		return ir.Geometry{
			Kind:        ir.GeometryPreset,
			Preset:      preset.Name,
			Adjustments: append([]float64(nil), preset.Adjustments...),
		}
	}
	if geometry == nil {
		return ir.Geometry{}
	}
	if geometry.Type == dml.GeometryTypePreset {
		return ir.Geometry{
			Kind:   ir.GeometryPreset,
			Preset: geometry.Name,
		}
	}

	out := ir.Geometry{
		Kind:   ir.GeometryCustom,
		Preset: geometry.Name,
	}
	for _, path := range geometry.Paths {
		p := ir.Path{
			FillRule: path.FillRule,
			Fill:     true,
			Stroke:   true,
		}
		for _, cmd := range path.Commands {
			// The shared IR intentionally uses SVG-like commands. DrawingML arc
			// commands are retained as "A" until the renderer's arc normalizer
			// converts them to cubic segments.
			c := ir.PathCommand{Op: cmd.Type}
			for _, point := range cmd.Points {
				c.Points = append(c.Points, ir.PointF{X: point.X, Y: point.Y})
			}
			p.Commands = append(p.Commands, c)
		}
		out.Paths = append(out.Paths, p)
	}
	return out
}

func TextBody(body dml.TextBody) *ir.TextBody {
	props := body.BodyProps
	out := &ir.TextBody{
		Insets: ir.Insets{
			Left:   props.InsetL,
			Top:    props.InsetT,
			Right:  props.InsetR,
			Bottom: props.InsetB,
		},
		Wrap:         textWrap(props.Wrap),
		Vert:         textVertical(props.Vert),
		Anchor:       textAnchor(props.Anchor),
		AnchorCenter: props.AnchorCtr,
		RotationDeg:  float64(props.Rot) / 60000.0,
		AutoFit:      textAutoFit(props.AutofitType),
		Columns:      props.Columns,
		ColumnGap:    props.ColumnSpacingEMU,
	}
	for _, paragraph := range body.Paragraphs {
		p := ir.Paragraph{
			Align:         paragraph.Props.Align,
			SpaceBeforePt: float64(paragraph.Props.SpaceBefore) / 100.0,
			SpaceAfterPt:  float64(paragraph.Props.SpaceAfter) / 100.0,
		}
		if paragraph.Props.LineSpacing > 0 {
			p.LineHeightPt = float64(paragraph.Props.LineSpacing) / 100.0
		}
		for _, run := range paragraph.Runs {
			p.Runs = append(p.Runs, ir.TextRun{
				Text:       run.Text,
				FontFamily: run.Props.FontFamily,
				FontSizePt: run.Props.FontSizePt,
				Bold:       run.Props.Bold,
				Italic:     run.Props.Italic,
				Underline:  run.Props.Underline,
				Color:      run.Props.Color,
			})
		}
		out.Paragraphs = append(out.Paragraphs, p)
	}
	return out
}

func Effects(list dml.EffectList) ir.Effects {
	var out ir.Effects
	for _, effect := range list.Effects {
		switch e := effect.(type) {
		case dml.Shadow:
			if out.Shadow == nil {
				out.Shadow = &ir.Shadow{
					Color:      e.Color,
					Alpha:      normalizedAlpha(e.Alpha),
					BlurRadius: e.Blur,
					Distance:   e.Distance,
					AngleDeg:   e.Angle,
				}
			}
		case dml.InnerShadow:
			if out.Shadow == nil {
				out.Shadow = &ir.Shadow{
					Color:      e.Color,
					Alpha:      normalizedAlpha(e.Alpha),
					BlurRadius: e.Blur,
					Distance:   e.Distance,
					AngleDeg:   e.Angle,
					Inner:      true,
				}
			}
		case dml.Glow:
			if out.Glow == nil {
				out.Glow = &ir.Glow{
					Color:    e.Color,
					Alpha:    normalizedAlpha(e.Alpha),
					RadiusPt: e.Radius,
				}
			}
		case dml.Blur:
			if out.Blur == nil {
				out.Blur = &ir.Blur{RadiusPt: e.Radius}
			}
		case dml.SoftEdge:
			if out.Blur == nil {
				out.Blur = &ir.Blur{RadiusPt: e.Radius}
			}
		}
	}
	return out
}

func Connector(spec ConnectorSpec, line *dml.LineStyle) *ir.Connector {
	out := &ir.Connector{
		Type:  spec.Type,
		Start: spec.Start,
		End:   spec.End,
	}
	if line != nil {
		out.StartArrow = Arrow(line.HeadEnd)
		out.EndArrow = Arrow(line.TailEnd)
	}
	if len(spec.Adjustments) > 0 {
		out.Adjustments = make([]float64, len(spec.Adjustments))
		for i, adjustment := range spec.Adjustments {
			out.Adjustments[i] = float64(adjustment) / 100000.0
		}
	}
	return out
}

func textWrap(value string) ir.TextWrap {
	if value == "none" {
		return ir.TextWrapNone
	}
	return ir.TextWrapSquare
}

func textVertical(value string) ir.TextVertical {
	switch value {
	case "vert":
		return ir.TextVerticalRL
	case "vert270":
		return ir.TextVertical270
	case "wordArtVert":
		return ir.TextWordArtVertical
	default:
		return ir.TextHorizontal
	}
}

func textAnchor(value string) ir.TextAnchor {
	switch value {
	case "ctr":
		return ir.TextAnchorCenter
	case "b":
		return ir.TextAnchorBottom
	default:
		return ir.TextAnchorTop
	}
}

func textAutoFit(value string) ir.AutoFit {
	switch value {
	case "noAutofit":
		return ir.AutoFitNone
	case "spAutoFit":
		return ir.AutoFitShape
	case "normAutofit":
		return ir.AutoFitNormal
	default:
		return ir.AutoFitDefault
	}
}

func normalizedAlpha(value float64) float64 {
	if value <= 0 {
		return 1
	}
	if value > 1 {
		return 1
	}
	return value
}

func percent100000(value int) float64 {
	if value == 0 {
		return 1
	}
	return float64(value) / 100000.0
}

func clamp01(value float64) float64 {
	return math.Max(0, math.Min(1, value))
}
