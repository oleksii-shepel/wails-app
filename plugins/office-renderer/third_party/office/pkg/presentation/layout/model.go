package layout

import ir "office/pkg/presentation/ir"

// Scene is the canonical resolved presentation scene.
//
// Layout deliberately does not define a second presentation model. Parser and
// format-specific resolver stages project into presentation/ir, and every
// downstream layout/rendering stage consumes that same canonical model.
type Scene = ir.Presentation

// SlideScene is the canonical resolved slide scene.
type SlideScene = ir.Slide
