package layout

import (
	"office/pkg/diag"
	"office/pkg/presentation/parsers/ooxml/resolve"
)

// Scene represents a fully composed presentation with resolved inheritance.
type Scene struct {
	// SlideWidthEMU and SlideHeightEMU are the canvas dimensions.
	SlideWidthEMU  int64
	SlideHeightEMU int64
	Slides         []*SlideScene
	// EmbeddedFonts are presentation-level font faces that can be emitted into HTML.
	EmbeddedFonts []*resolve.EmbeddedFont
	// Theme is the first resolved theme, if any (presentation-level).
	Theme       *resolve.Theme
	Diagnostics diag.List
}

// SlideScene is a fully composed slide with inherited properties.
type SlideScene struct {
	// Index is the 0-based slide position in the deck.
	Index int
	// BgColor is the slide background as sRGB hex (e.g. "FFFFFF"), or "" for white/default.
	BgColor string
	// Shapes holds all content shapes on the slide in exact tree order.
	Shapes []*ComposedShape
	// InheritanceTrace provides debugging information about property inheritance.
	InheritanceTrace *InheritanceTrace
}

// ComposedShape is a shape with fully resolved inherited properties.
type ComposedShape struct {
	// Source identifies where this shape originated (master, layout, or slide).
	Source ShapeSource
	// Shape is the resolved shape with inherited properties applied.
	Shape *resolve.Shape
	// Children holds nested shapes for Group shapes.
	Children []*ComposedShape
}

// ShapeSource identifies the origin of a shape in the inheritance hierarchy.
type ShapeSource int

const (
	SourceMaster ShapeSource = iota
	SourceLayout
	SourceSlide
)

// InheritanceTrace records how properties were inherited for debugging.
type InheritanceTrace struct {
	// MasterProperties records which properties came from the master.
	MasterProperties map[string]interface{}
	// LayoutProperties records which properties came from the layout.
	LayoutProperties map[string]interface{}
	// SlideProperties records which properties came from the slide.
	SlideProperties map[string]interface{}
	// OverriddenProperties records which properties were overridden at each level.
	OverriddenProperties map[string][]string
}

// ComposeOptions configures the scene composition process.
type ComposeOptions struct {
	// EnableTracing enables detailed inheritance tracing for debugging.
	EnableTracing bool
	// StrictMode enables strict validation of inheritance rules.
	StrictMode bool
}