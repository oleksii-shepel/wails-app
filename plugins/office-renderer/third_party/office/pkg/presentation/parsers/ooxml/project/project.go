// Package project owns the format-boundary projection from parsed PresentationML
// parts into canonical presentation IR. Visual composition (master/layout/slide
// inheritance, geometry and styling) remains the responsibility of layout.
package project

import (
	"strings"

	"office/pkg/ooxml/media"
	"office/pkg/presentation/ir"
	"office/pkg/presentation/parsers/ooxml/parts"
)

// NewPresentation projects presentation-level semantics that do not require
// visual composition.
func NewPresentation(pkg *parts.Package) *ir.Presentation {
	if pkg == nil {
		return nil
	}
	out := &ir.Presentation{Diagnostics: pkg.Diagnostics}
	if pkg.Presentation != nil && pkg.Presentation.SlideSize != nil {
		out.SlideWidthEMU = pkg.Presentation.SlideSize.CX
		out.SlideHeightEMU = pkg.Presentation.SlideSize.CY
	}
	out.EmbeddedFonts = EmbeddedFonts(pkg.EmbeddedFonts)
	return out
}

// EmbeddedFonts projects parsed embedded-font parts into canonical IR.
func EmbeddedFonts(fonts []*parts.EmbeddedFontData) []*ir.EmbeddedFont {
	out := make([]*ir.EmbeddedFont, 0, len(fonts))
	for _, font := range fonts {
		if font == nil || strings.TrimSpace(font.Typeface) == "" {
			continue
		}
		resolved := &ir.EmbeddedFont{Family: strings.TrimSpace(font.Typeface)}
		appendFace := func(part *parts.FontPart, weight int, style string) {
			if part == nil || len(part.Data) == 0 {
				return
			}
			resolved.Faces = append(resolved.Faces, &ir.EmbeddedFontFace{Family: resolved.Family, Weight: weight, Style: style, ContentType: part.ContentType, Name: part.Name, Data: part.Data})
		}
		appendFace(font.Regular, 400, "normal")
		appendFace(font.Bold, 700, "normal")
		appendFace(font.Italic, 400, "italic")
		appendFace(font.BoldItalic, 700, "italic")
		if len(resolved.Faces) != 0 {
			out = append(out, resolved)
		}
	}
	return out
}

// MasterIDsByPart projects presentation master references into a stable part→ID map.
func MasterIDsByPart(pkg *parts.Package) map[string]int {
	if pkg == nil || pkg.Presentation == nil {
		return nil
	}
	partByRelID := make(map[string]string, len(pkg.SlideMasters))
	for _, me := range pkg.SlideMasters {
		if me != nil && me.RelID != "" {
			partByRelID[me.RelID] = me.PartName
		}
	}
	out := make(map[string]int, len(pkg.Presentation.SlideMasterIDs))
	for _, ref := range pkg.Presentation.SlideMasterIDs {
		if ref.ID != 0 {
			if part := partByRelID[ref.RelID]; part != "" {
				out[part] = ref.ID
			}
		}
	}
	return out
}

// ApplySections projects section membership after slides have been composed.
func ApplySections(raw *parts.Presentation, out *ir.Presentation) {
	if raw == nil || out == nil || len(raw.Sections) == 0 {
		return
	}
	slideIDToIndex := make(map[int]int, len(raw.SlideIDs))
	for i, ref := range raw.SlideIDs {
		if ref.ID != 0 {
			slideIDToIndex[ref.ID] = i
		}
	}
	for _, sec := range raw.Sections {
		if sec.Name == "" {
			continue
		}
		out.Sections = append(out.Sections, sec.Name)
		for _, ref := range sec.SlideIDs {
			if i, ok := slideIDToIndex[ref.ID]; ok && i >= 0 && i < len(out.Slides) {
				out.Slides[i].SectionName = sec.Name
			}
		}
	}
}

// Comments projects OOXML comment/author records into format-independent IR.
func Comments(raw []*parts.PptxComment, authors map[int]parts.CommentAuthor) []*ir.Comment {
	if len(raw) == 0 {
		return nil
	}
	out := make([]*ir.Comment, 0, len(raw))
	for _, c := range raw {
		if c == nil {
			continue
		}
		item := &ir.Comment{DateTime: c.DateTime, Text: c.Text, X: c.X, Y: c.Y}
		if a, ok := authors[c.AuthorID]; ok {
			item.AuthorName, item.Initials = a.Name, a.Initials
		}
		out = append(out, item)
	}
	return out
}

// Transition projects parsed slide-change metadata into canonical IR.
func Transition(raw *parts.SlideTransition) *ir.SlideTransition {
	if raw == nil {
		return nil
	}
	return &ir.SlideTransition{Type: raw.Type, Spd: raw.Spd, AdvClick: raw.AdvClick, AdvTmMs: raw.AdvTmMs}
}

// AnimationTimeline projects parsed timing events into canonical IR while
// attaching the parser-computed entrance step for each target shape.
func AnimationTimeline(events []parts.AnimationEvent, steps map[int]int) []ir.AnimationEvent {
	if len(events) == 0 {
		return nil
	}
	out := make([]ir.AnimationEvent, 0, len(events))
	for _, ev := range events {
		out = append(out, ir.AnimationEvent{
			ShapeID: ev.ShapeID, Step: steps[ev.ShapeID], NodeType: ev.NodeType,
			Behavior: ev.Behavior, Effect: ev.Effect, Trigger: ev.Trigger,
			DelayMs: ev.DelayMs, DurationMs: ev.DurationMs, RepeatCount: ev.RepeatCount,
			RepeatDurMs: ev.RepeatDurMs, AutoReverse: ev.AutoReverse, Fill: ev.Fill,
			MotionPath: ev.MotionPath, MotionOrigin: ev.MotionOrigin,
		})
	}
	return out
}

// ApplySlideMetadata projects slide-level semantics that are independent of
// visual composition. Keeping this operation here prevents the visual resolver
// from reinterpreting PresentationML timing/transition records.
func ApplySlideMetadata(raw *parts.Slide, out *ir.Slide) {
	if raw == nil || out == nil {
		return
	}
	out.Transition = Transition(raw.Transition)
	out.AnimationTimeline = AnimationTimeline(raw.AnimationEvents, raw.AnimSteps)
}

// ApplyAnimationMetadata projects all parsed p:timing semantics onto an already
// authored shape tree. This belongs at the OOXML project boundary rather than in
// the visual resolver: timing targets stable shape IDs and does not require
// layout/master composition.
func ApplyAnimationMetadata(raw *parts.Slide, out *ir.Slide) {
	if raw == nil || out == nil {
		return
	}
	ApplySlideMetadata(raw, out)
	applyAnimationToShapes(out.Shapes, raw)
}

func applyAnimationToShapes(shapes []*ir.Shape, raw *parts.Slide) {
	for _, shape := range shapes {
		if shape == nil {
			continue
		}
		if step, ok := raw.AnimSteps[shape.ID]; ok {
			shape.AnimStep = step
		}
		if fx, ok := raw.EmphasisEffects[shape.ID]; ok {
			shape.EmphasisEffects = append([]string(nil), fx...)
		}
		if mp, ok := raw.MotionPaths[shape.ID]; ok {
			shape.MotionPath = &ir.AnimMotionPath{Path: mp.Path, Dur: mp.Dur, Origin: mp.Origin}
		}
		if triggers, ok := raw.AnimationTriggers[shape.ID]; ok {
			shape.AnimationTriggers = append([]string(nil), triggers...)
		}
		applyAnimationToShapes(shape.Children, raw)
	}
}

// Resource is a canonical relationship-backed payload or external target.
// It deliberately omits OPC part names and relationship implementation details.
type Resource struct {
	Data []byte
	MIME string
	URL  string
}

// SlideResources projects slide-local relationship resources needed during
// visual composition. Relationship IDs remain stable logical keys, while OPC
// media.Part and relationship records do not cross this boundary.
type SlideResources struct {
	Media      map[string]Resource
	OLE        map[string]Resource
	Hyperlinks map[string]string
	OLELinks   map[string]string
}

// Resources projects a parsed slide entry's relationship-backed resources into
// a format-independent lookup context for composition.
func Resources(entry *parts.SlideEntry) SlideResources {
	var out SlideResources
	if entry == nil {
		return out
	}
	out.Media = MediaResources(entry.MediaParts)
	out.OLE = MediaResources(entry.OleObjectParts)
	out.Hyperlinks = normalizedStrings(entry.HlinkURLs)
	out.OLELinks = normalizedStrings(entry.OleObjectLinks)
	return out
}

// MediaResources projects an OOXML media-part lookup into canonical resources.
func MediaResources(src map[string]media.Part) map[string]Resource {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]Resource, len(src))
	for id, part := range src {
		out[normalizeResourceID(id)] = Resource{Data: part.Data, MIME: part.ContentType}
	}
	return out
}

// Lookup returns a normalized relationship-backed resource.
func (r SlideResources) Lookup(id string) (Resource, bool) {
	item, ok := r.Media[normalizeResourceID(id)]
	return item, ok
}

// Hyperlink returns the external hyperlink target for a relationship id.
func (r SlideResources) Hyperlink(id string) string {
	return r.Hyperlinks[normalizeResourceID(id)]
}

// WithMedia constructs a resource context for non-slide visual trees such as
// layouts and masters. No OPC media.Part value escapes the projection boundary.
func WithMedia(src map[string]media.Part) SlideResources {
	return SlideResources{Media: MediaResources(src)}
}

func normalizedStrings(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]string, len(src))
	for id, value := range src {
		out[normalizeResourceID(id)] = value
	}
	return out
}

func normalizeResourceID(id string) string { return strings.TrimSpace(id) }
