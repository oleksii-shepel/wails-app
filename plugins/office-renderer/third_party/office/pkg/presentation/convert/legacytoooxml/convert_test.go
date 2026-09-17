package legacytoooxml

import (
	"testing"

	presentationir "office/pkg/presentation/ir"
	presparts "office/pkg/presentation/parsers/ooxml/parts"
)

func TestConvertNilPresentation(t *testing.T) {
	_, err := Convert(nil)
	if err == nil {
		t.Fatal("expected error for nil presentation")
	}
}

func TestConvertEmptyPresentation(t *testing.T) {
	pres := &presentationir.Presentation{}
	pkg, err := Convert(pres)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg == nil {
		t.Fatal("expected non-nil package")
	}
	if pkg.Presentation == nil {
		t.Fatal("expected non-nil presentation part")
	}
}

func TestConvertSlideDimensions(t *testing.T) {
	pres := &presentationir.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
	}
	pkg, err := Convert(pres)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.Presentation.SlideSize == nil {
		t.Fatal("expected slide size")
	}
	if pkg.Presentation.SlideSize.CX != 9144000 {
		t.Fatalf("expected width 9144000, got %d", pkg.Presentation.SlideSize.CX)
	}
	if pkg.Presentation.SlideSize.CY != 6858000 {
		t.Fatalf("expected height 6858000, got %d", pkg.Presentation.SlideSize.CY)
	}
}

func TestConvertSlideWithText(t *testing.T) {
	pres := &presentationir.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*presentationir.Slide{
			{
				Index: 0,
				Shapes: []*presentationir.Shape{
					{
						ID:       1,
						Name:     "Title",
						Kind:     presentationir.ShapeText,
						Bounds:   presentationir.Rect{X: 1000000, Y: 500000, W: 7000000, H: 1000000},
						Geometry: "rect",
						Paragraphs: []*presentationir.Paragraph{
							{
								Align: "ctr",
								Runs: []*presentationir.Run{
									{Text: "Hello World", Bold: true, FontSize: 18},
								},
							},
						},
					},
				},
			},
		},
	}
	pkg, err := Convert(pres)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkg.Slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(pkg.Slides))
	}
	slide := pkg.Slides[0].Slide
	if len(slide.CSld.SpTree.Shapes) != 1 {
		t.Fatalf("expected 1 shape, got %d", len(slide.CSld.SpTree.Shapes))
	}
	shape := slide.CSld.SpTree.Shapes[0]
	if shape.TxBody == nil {
		t.Fatal("expected text body")
	}
	if len(shape.TxBody.Paragraphs) != 1 {
		t.Fatalf("expected 1 paragraph, got %d", len(shape.TxBody.Paragraphs))
	}
	para := shape.TxBody.Paragraphs[0]
	if len(para.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(para.Runs))
	}
	if para.Runs[0].Text != "Hello World" {
		t.Fatalf("expected Hello World, got %q", para.Runs[0].Text)
	}
}

func TestConvertSlideWithImage(t *testing.T) {
	pres := &presentationir.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*presentationir.Slide{
			{
				Index: 0,
				Shapes: []*presentationir.Shape{
					{
						ID:        1,
						Name:      "Picture",
						Kind:      presentationir.ShapeImage,
						Bounds:    presentationir.Rect{X: 1000000, Y: 1000000, W: 2000000, H: 1500000},
						ImageData: []byte{0x89, 0x50, 0x4E, 0x47},
						ImageMIME: "image/png",
					},
				},
			},
		},
	}
	pkg, err := Convert(pres)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkg.Slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(pkg.Slides))
	}
	slide := pkg.Slides[0].Slide
	if len(slide.CSld.SpTree.Pictures) != 1 {
		t.Fatalf("expected 1 picture, got %d", len(slide.CSld.SpTree.Pictures))
	}
	if len(pkg.MediaParts) != 1 {
		t.Fatalf("expected 1 media part, got %d", len(pkg.MediaParts))
	}
}

func TestConvertSlideWithBackground(t *testing.T) {
	pres := &presentationir.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*presentationir.Slide{
			{
				Index:   0,
				BgColor: "FF0000",
			},
		},
	}
	pkg, err := Convert(pres)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	slide := pkg.Slides[0].Slide
	if slide.CSld.Bg == nil {
		t.Fatal("expected background")
	}
	if slide.CSld.Bg.BgPrSrgb == nil || slide.CSld.Bg.BgPrSrgb.Val != "FF0000" {
		t.Fatal("expected red background")
	}
}

func TestConvertSlideWithNotes(t *testing.T) {
	pres := &presentationir.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*presentationir.Slide{
			{
				Index: 0,
				Notes: &presentationir.NotesSlide{
					Shapes: []*presentationir.Shape{
						{
							ID:     1,
							Name:   "Notes",
							Kind:   presentationir.ShapeText,
							Bounds: presentationir.Rect{X: 1000000, Y: 500000, W: 7000000, H: 1000000},
							Paragraphs: []*presentationir.Paragraph{
								{Runs: []*presentationir.Run{{Text: "Note text"}}},
							},
						},
					},
				},
			},
		},
	}
	pkg, err := Convert(pres)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.Slides[0].Notes == nil {
		t.Fatal("expected notes slide")
	}
}

func TestConvertSlideWithComments(t *testing.T) {
	pres := &presentationir.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*presentationir.Slide{
			{
				Index: 0,
				Comments: []*presentationir.Comment{
					{AuthorName: "Alice", Text: "Great slide!"},
				},
			},
		},
	}
	pkg, err := Convert(pres)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkg.Slides[0].Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(pkg.Slides[0].Comments))
	}
	if len(pkg.CommentAuthors) != 1 {
		t.Fatalf("expected 1 comment author, got %d", len(pkg.CommentAuthors))
	}
}

func TestConvertTheme(t *testing.T) {
	pres := &presentationir.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Theme: &presentationir.Theme{
			Name: "Office Theme",
			ColorScheme: map[string]string{
				"dk1":     "000000",
				"lt1":     "FFFFFF",
				"accent1": "4472C4",
			},
		},
	}
	pkg, err := Convert(pres)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkg.Themes) != 1 {
		t.Fatalf("expected 1 theme, got %d", len(pkg.Themes))
	}
}

func TestConvertGroupAnimations(t *testing.T) {
	pres := &presentationir.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*presentationir.Slide{
			{
				Index: 0,
				Shapes: []*presentationir.Shape{
					{
						Kind:   presentationir.ShapeGroup,
						Name:   "Group",
						Bounds: presentationir.Rect{X: 0, Y: 0, W: 1000000, H: 1000000},
						Children: []*presentationir.Shape{
							{
								Kind:            presentationir.ShapeText,
								Name:            "Child",
								Bounds:          presentationir.Rect{X: 0, Y: 0, W: 500000, H: 500000},
								AnimStep:        1,
								EmphasisEffects: []string{"spin"},
								MotionPath: &presentationir.AnimMotionPath{
									Path:   "M 0 0 L 100000 0",
									Dur:    600,
									Origin: "layout",
								},
								AnimationTriggers: []string{"onClick"},
							},
						},
					},
				},
			},
		},
	}
	pkg, err := Convert(pres)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkg.Slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(pkg.Slides))
	}
	slide := pkg.Slides[0].Slide
	if len(slide.AnimSteps) != 1 {
		t.Fatalf("expected 1 animated entrance shape, got %v", slide.AnimSteps)
	}
	if len(slide.EmphasisEffects) != 1 {
		t.Fatalf("expected 1 emphasis shape, got %v", slide.EmphasisEffects)
	}
	if len(slide.MotionPaths) != 1 {
		t.Fatalf("expected 1 motion path shape, got %v", slide.MotionPaths)
	}
	if len(slide.AnimationTriggers) != 1 {
		t.Fatalf("expected 1 trigger shape, got %v", slide.AnimationTriggers)
	}
}
func TestLayoutForSlideDoesNotBorrowFirstMasterLayout(t *testing.T) {
	masterA := &presentationir.SlideMaster{Name: "A"}
	layoutA := &presentationir.SlideLayout{Type: "blank", Master: masterA}
	masterA.Layouts = []*presentationir.SlideLayout{layoutA}
	masterB := &presentationir.SlideMaster{Name: "B"}
	layoutB := &presentationir.SlideLayout{Type: "blank", Master: masterB}
	masterB.Layouts = []*presentationir.SlideLayout{layoutB}

	c := &converter{pkg: &presparts.Package{}}
	c.convertSlideMasters([]*presentationir.SlideMaster{masterA, masterB})
	if got := c.layoutForSlide(&presentationir.Slide{LayoutType: "blank"}); got != nil {
		t.Fatalf("unowned slide borrowed layout %q", got.PartName)
	}
	if got := c.layoutForSlide(&presentationir.Slide{Master: masterB, LayoutType: "blank"}); got != c.layoutParts[layoutB] {
		t.Fatalf("master-owned slide layout = %p, want master B layout %p", got, c.layoutParts[layoutB])
	}
}

func TestLayoutForSlideUsesExactLayoutOwnershipBeforeType(t *testing.T) {
	master := &presentationir.SlideMaster{Name: "M"}
	first := &presentationir.SlideLayout{Type: "blank", Name: "first", Master: master}
	second := &presentationir.SlideLayout{Type: "blank", Name: "second", Master: master}
	master.Layouts = []*presentationir.SlideLayout{first, second}
	c := &converter{pkg: &presparts.Package{}}
	c.convertSlideMasters([]*presentationir.SlideMaster{master})
	got := c.layoutForSlide(&presentationir.Slide{Master: master, Layout: second, LayoutType: "blank"})
	if got != c.layoutParts[second] {
		t.Fatalf("layout = %p, want exact second layout %p", got, c.layoutParts[second])
	}
}
