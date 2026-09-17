package project

import (
	"office/pkg/ooxml/media"
	"testing"

	"office/pkg/presentation/ir"
	"office/pkg/presentation/parsers/ooxml/parts"
)

func TestNewPresentationProjectsCanvasAndSectionsWithoutVisualResolution(t *testing.T) {
	pkg := &parts.Package{Presentation: &parts.Presentation{
		SlideSize: &parts.SlideSize{CX: 12192000, CY: 6858000},
		SlideIDs:  []parts.IDRef{{ID: 256}, {ID: 257}},
		Sections:  []parts.Section{{Name: "Intro", SlideIDs: []parts.SectionSlideID{{ID: 256}}}},
	}}
	got := NewPresentation(pkg)
	if got.SlideWidthEMU != 12192000 || got.SlideHeightEMU != 6858000 {
		t.Fatalf("canvas = %dx%d", got.SlideWidthEMU, got.SlideHeightEMU)
	}
	// Section projection is intentionally a second step because slides are composed by layout.
	got.Slides = make([]*ir.Slide, 2)
	for i := range got.Slides {
		got.Slides[i] = &ir.Slide{}
	}
	ApplySections(pkg.Presentation, got)
	if len(got.Sections) != 1 || got.Sections[0] != "Intro" || got.Slides[0].SectionName != "Intro" {
		t.Fatalf("sections = %#v, first slide = %#v", got.Sections, got.Slides[0])
	}
}

func TestCommentsProjectAuthorSemantics(t *testing.T) {
	got := Comments([]*parts.PptxComment{{AuthorID: 7, Text: "review", X: 10, Y: 20}}, map[int]parts.CommentAuthor{7: {ID: 7, Name: "Ada", Initials: "AL"}})
	if len(got) != 1 || got[0].AuthorName != "Ada" || got[0].Initials != "AL" || got[0].Text != "review" {
		t.Fatalf("comments = %#v", got)
	}
}

func TestApplySlideMetadataProjectsTransitionAndTiming(t *testing.T) {
	raw := &parts.Slide{
		Transition:      &parts.SlideTransition{Type: "fade", Spd: "slow", AdvClick: true, AdvTmMs: 1250},
		AnimationEvents: []parts.AnimationEvent{{ShapeID: 7, NodeType: "clickEffect", Behavior: "animEffect", Effect: "fade", DelayMs: 20, DurationMs: 300}},
		AnimSteps:       map[int]int{7: 2},
	}
	out := &ir.Slide{}
	ApplySlideMetadata(raw, out)
	if out.Transition == nil || out.Transition.Type != "fade" || out.Transition.AdvTmMs != 1250 {
		t.Fatalf("transition = %#v", out.Transition)
	}
	if len(out.AnimationTimeline) != 1 || out.AnimationTimeline[0].ShapeID != 7 || out.AnimationTimeline[0].Step != 2 {
		t.Fatalf("timeline = %#v", out.AnimationTimeline)
	}
}

func TestResourcesProjectsSlideRelationshipPayloads(t *testing.T) {
	entry := &parts.SlideEntry{
		MediaParts:     map[string]media.Part{"rIdMedia": {Data: []byte("video"), ContentType: "video/mp4"}},
		OleObjectParts: map[string]media.Part{"rIdOle": {Data: []byte("ole"), ContentType: "application/vnd.openxmlformats-officedocument.oleObject"}},
		HlinkURLs:      map[string]string{"rIdLink": "https://example.test"},
		OleObjectLinks: map[string]string{"rIdOleLink": "file:///example.xlsx"},
	}
	got := Resources(entry)
	if r := got.Media["rIdMedia"]; string(r.Data) != "video" || r.MIME != "video/mp4" {
		t.Fatalf("media resource = %#v", r)
	}
	if r := got.OLE["rIdOle"]; string(r.Data) != "ole" || r.MIME == "" {
		t.Fatalf("OLE resource = %#v", r)
	}
	if got.Hyperlinks["rIdLink"] != "https://example.test" {
		t.Fatalf("hyperlink = %q", got.Hyperlinks["rIdLink"])
	}
	if got.OLELinks["rIdOleLink"] != "file:///example.xlsx" {
		t.Fatalf("OLE link = %q", got.OLELinks["rIdOleLink"])
	}
}

func TestSlideResourcesNormalizePictureMediaAndHyperlinks(t *testing.T) {
	entry := &parts.SlideEntry{
		MediaParts: map[string]media.Part{
			" rId5 ": {Data: []byte("image"), ContentType: "image/png"},
		},
		HlinkURLs: map[string]string{" rId7 ": "https://example.test/"},
	}
	resources := Resources(entry)
	got, ok := resources.Lookup("rId5")
	if !ok || string(got.Data) != "image" || got.MIME != "image/png" {
		t.Fatalf("picture resource = %#v, %v", got, ok)
	}
	if got := resources.Hyperlink("rId7"); got != "https://example.test/" {
		t.Fatalf("hyperlink = %q", got)
	}
}

func TestApplyAnimationMetadataProjectsShapeTimingWithoutVisualResolver(t *testing.T) {
	raw := &parts.Slide{
		AnimSteps:         map[int]int{7: 2},
		EmphasisEffects:   map[int][]string{7: {"spin"}},
		MotionPaths:       map[int]parts.AnimMotionPath{7: {Path: "M 0 0 L 1 1", Dur: 500, Origin: "layout"}},
		AnimationTriggers: map[int][]string{7: {"onClick"}},
	}
	out := &ir.Slide{Shapes: []*ir.Shape{{ID: 1, Children: []*ir.Shape{{ID: 7}}}}}
	ApplyAnimationMetadata(raw, out)
	shape := out.Shapes[0].Children[0]
	if shape.AnimStep != 2 || len(shape.EmphasisEffects) != 1 || shape.EmphasisEffects[0] != "spin" {
		t.Fatalf("animation projection = %#v", shape)
	}
	if shape.MotionPath == nil || shape.MotionPath.Path != "M 0 0 L 1 1" || shape.MotionPath.Dur != 500 {
		t.Fatalf("motion path = %#v", shape.MotionPath)
	}
	if len(shape.AnimationTriggers) != 1 || shape.AnimationTriggers[0] != "onClick" {
		t.Fatalf("triggers = %v", shape.AnimationTriggers)
	}
}
