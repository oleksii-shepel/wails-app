// Package resolve preserves the former presentation IR type names while the
// materialized PresentationML resolver remains retired. It contains no parser
// or package-resolution entry point; new code should import presentation/ir.
package resolve

import ir "office/pkg/presentation/ir"

type (
	Presentation     = ir.Presentation
	Slide            = ir.Slide
	SlideTransition  = ir.SlideTransition
	NotesSlide       = ir.NotesSlide
	Comment          = ir.Comment
	Shape            = ir.Shape
	ShapeKind        = ir.ShapeKind
	SmartArtShape    = ir.SmartArtShape
	PlaceholderKind  = ir.PlaceholderKind
	Rect             = ir.Rect
	Point            = ir.Point
	Table            = ir.Table
	TableRow         = ir.TableRow
	TableCell        = ir.TableCell
	Paragraph        = ir.Paragraph
	Run              = ir.Run
	Theme            = ir.Theme
	EmbeddedFont     = ir.EmbeddedFont
	EmbeddedFontFace = ir.EmbeddedFontFace
	TabStop          = ir.TabStop
	AnimMotionPath   = ir.AnimMotionPath
	AnimationEvent   = ir.AnimationEvent
	PictureTile      = ir.PictureTile
	EffectOverlay    = ir.EffectOverlay
)

const (
	ShapeText              = ir.ShapeText
	ShapeImage             = ir.ShapeImage
	ShapeGroup             = ir.ShapeGroup
	ShapeConnector         = ir.ShapeConnector
	ShapeSmartArtConnector = ir.ShapeSmartArtConnector
	ShapeSmartArt          = ir.ShapeSmartArt
	ShapeChart             = ir.ShapeChart
	ShapeTable             = ir.ShapeTable
	ShapeMedia             = ir.ShapeMedia
	ShapeOleObject         = ir.ShapeOleObject
	ShapeOther             = ir.ShapeOther

	PlaceholderNone        = ir.PlaceholderNone
	PlaceholderTitle       = ir.PlaceholderTitle
	PlaceholderBody        = ir.PlaceholderBody
	PlaceholderSubtitle    = ir.PlaceholderSubtitle
	PlaceholderDateTime    = ir.PlaceholderDateTime
	PlaceholderSlideNumber = ir.PlaceholderSlideNumber
	PlaceholderFooter      = ir.PlaceholderFooter
	PlaceholderHeader      = ir.PlaceholderHeader
	PlaceholderContent     = ir.PlaceholderContent
	PlaceholderChart       = ir.PlaceholderChart
	PlaceholderTable       = ir.PlaceholderTable
	PlaceholderPicture     = ir.PlaceholderPicture
	PlaceholderMedia       = ir.PlaceholderMedia
	PlaceholderClipArt     = ir.PlaceholderClipArt
	PlaceholderDiagram     = ir.PlaceholderDiagram
	PlaceholderSlideImage  = ir.PlaceholderSlideImage
	PlaceholderOther       = ir.PlaceholderOther
)
