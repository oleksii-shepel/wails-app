// Package ir defines the stable public intermediate representation (IR) for
// presentations (.pptx / PresentationML).
//
// # Stability guarantee
//
// All exported types and fields in this package are considered stable public
// API. Additions are non-breaking. Fields will not be removed or renamed
// without a major version bump. Internal OOXML parsing details (parts,
// resolve) are NOT part of this contract.
//
// # Overview
//
// The IR is produced by
// the presentation SAX/package projection pipeline and consumed
// by the layout and render layers. It can also be consumed directly by callers
// who need structured access to slide content without going through the
// rendering pipeline.
//
// # Type hierarchy
//
//	Presentation
//	├── Slides []*Slide
//	│   ├── Shapes []*Shape       — all content objects in tree order
//	│   │   ├── Paragraphs        — text content for ShapeText shapes
//	│   │   │   └── Runs []*Run   — styled text runs
//	│   │   └── Children []*Shape — nested shapes for ShapeGroup
//	│   ├── Notes *NotesSlide     — notes slide content, if present
//	│   ├── Header / Footer / DateTime / SlideNumber placeholders
//	│   └── Transition            — slide-change animation metadata
//	├── Theme *Theme              — presentation-level color and font scheme
//	└── EmbeddedFonts            — font faces available for CSS @font-face emission
//
// # Shape kinds
//
// [Shape.Kind] classifies each shape:
//   - [ShapeText]      — text box or placeholder (has Paragraphs)
//   - [ShapeImage]     — picture (has ImageData / ImageRelID)
//   - [ShapeGroup]     — group container (has Children)
//   - [ShapeConnector] — connector line
//   - [ShapeSmartArt]  — SmartArt converted to primitive shapes
//   - [ShapeChart]     — embedded chart converted to SVG
//   - [ShapeOther]     — unrecognised object
//
// # Placeholder roles
//
// [Shape.Placeholder] identifies semantic placeholder slots:
//   - [PlaceholderTitle], [PlaceholderSubtitle], [PlaceholderBody]
//   - [PlaceholderDateTime], [PlaceholderSlideNumber], [PlaceholderFooter]
//   - [PlaceholderOther] — custom or unrecognised placeholder type
//
// # Coordinates and units
//
// All positional values ([Rect], [Point]) use EMU (914400 EMU = 1 inch).
// [Shape.Bounds] is the bounding box of the shape relative to the slide canvas.
// [Presentation.SlideWidthEMU] and [Presentation.SlideHeightEMU] give the
// canvas size. Use [office/pkg/ooxml/units] for conversion helpers.
//
// # Text formatting
//
// [Run] fields are fully resolved: bold, italic, underline, strike, font size
// (hundredths of a point), font family, and color (sRGB hex). Paragraph-level
// alignment and spacing are in [Paragraph]. Inheritance from slide layout,
// master, and theme has already been applied.
package ir
