package resolve

import (
	"testing"

	"office/pkg/ooxml/drawingxml"
	"office/pkg/presentation/parsers/ooxml/parts"
)

func TestResolveFixturePackageAppliesNotesMasterDefaults(t *testing.T) {
	pptPkg := &parts.Package{
		Presentation: &parts.Presentation{SlideSize: &parts.SlideSize{CX: 9144000, CY: 6858000}},
		NotesMaster: &parts.NotesMaster{
			CSld: parts.CommonSlideData{Bg: &parts.SlideBg{BgPrSrgb: &drawingxml.SRGBClr{Val: "FFF2CC"}}},
			TxStyles: &parts.MasterTxStyles{
				BodyStyle: &parts.DMLTextListStyle{Lvl1PPr: &parts.DMLTextStyleLevel{
					DefRPr: &parts.DMLRunPr{FontSize: 1800},
					SpcAft: &parts.DMLSpacing{SpcPts: &parts.SpacingPts{Val: 600}},
				}},
			},
		},
		Slides: []*parts.SlideEntry{{
			Slide: &parts.Slide{},
			Notes: &parts.NotesSlide{CSld: parts.CommonSlideData{SpTree: parts.ShapeTree{Shapes: []parts.Shape{{
				NvSpPr: parts.NonVisualShapeProps{
					CNvPr: parts.CnvPr{ID: 7, Name: "Notes Body"},
					NvPr:  parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: "body"}},
				},
				SpPr: parts.ShapeProperties{
					Xfrm: &parts.Transform2D{
						Off: parts.Offset{X: 0, Y: 0},
						Ext: parts.Extents{CX: 1000, CY: 1000},
					},
				},
				TxBody: &parts.TextBody{Paragraphs: []parts.DMLParagraph{{Runs: []parts.DMLRun{{Text: "Speaker note"}}}}},
			}}}}},
		}},
	}

	pres, err := resolveFixturePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolveFixturePackage: %v", err)
	}
	if len(pres.Slides) != 1 || pres.Slides[0].Notes == nil {
		t.Fatalf("resolved notes missing: %#v", pres.Slides)
	}
	notes := pres.Slides[0].Notes
	if notes.BgColor != "FFF2CC" {
		t.Fatalf("notes background = %q, want %q", notes.BgColor, "FFF2CC")
	}
	if len(notes.Shapes) != 1 || len(notes.Shapes[0].Paragraphs) != 1 || len(notes.Shapes[0].Paragraphs[0].Runs) != 1 {
		t.Fatalf("resolved notes shapes = %#v", notes.Shapes)
	}
	run := notes.Shapes[0].Paragraphs[0].Runs[0]
	if run.FontSize != 1800 {
		t.Fatalf("notes run font size = %d, want 1800", run.FontSize)
	}
	if !notes.Shapes[0].Paragraphs[0].HasSpaceAfter || notes.Shapes[0].Paragraphs[0].SpaceAfter != 600 {
		t.Fatalf("notes paragraph spacing after = %d/%v, want 600/true", notes.Shapes[0].Paragraphs[0].SpaceAfter, notes.Shapes[0].Paragraphs[0].HasSpaceAfter)
	}
}
