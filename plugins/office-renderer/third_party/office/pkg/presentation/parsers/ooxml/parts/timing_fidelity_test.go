package parts

import (
	"encoding/xml"
	"testing"
)

func TestTimingParsesSchedulerEvents(t *testing.T) {
	src := `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree/></p:cSld><p:timing><p:tnLst><p:seq><p:cTn id="1"><p:childTnLst><p:par><p:cTn id="2" nodeType="clickEffect" dur="600" repeatCount="2" autoRev="1" fill="hold"><p:stCondLst><p:cond delay="150"/></p:stCondLst><p:childTnLst><p:animScale><p:cBhvr><p:tgtEl><p:spTgt spid="4"/></p:tgtEl></p:cBhvr></p:animScale></p:childTnLst></p:cTn></p:par></p:childTnLst></p:cTn></p:seq></p:tnLst></p:timing></p:sld>`
	var slide Slide
	if err := xml.Unmarshal([]byte(src), &slide); err != nil {
		t.Fatal(err)
	}
	if len(slide.AnimationEvents) != 1 {
		t.Fatalf("events=%d want 1: %#v", len(slide.AnimationEvents), slide.AnimationEvents)
	}
	ev := slide.AnimationEvents[0]
	if ev.ShapeID != 4 || ev.NodeType != "clickEffect" || ev.Behavior != "animScale" {
		t.Fatalf("unexpected event: %#v", ev)
	}
	if ev.DelayMs != 150 || ev.DurationMs != 600 || ev.RepeatCount != 2 || !ev.AutoReverse || ev.Fill != "hold" {
		t.Fatalf("timing metadata lost: %#v", ev)
	}
	if ev.Trigger != "onClick" {
		t.Fatalf("trigger=%q want onClick", ev.Trigger)
	}
	if len(slide.TimingRawXML) == 0 {
		t.Fatal("raw timing XML must remain preserved")
	}
}

func TestTimingParsesMotionAndAfterPrevious(t *testing.T) {
	src := `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree/></p:cSld><p:timing><p:tnLst><p:par><p:cTn id="8" nodeType="afterEffect" dur="1200" repeatDur="2400"><p:stCondLst><p:cond delay="300"/></p:stCondLst><p:childTnLst><p:animMotion path="M 0 0 L 1 1" origin="layout"><p:cBhvr><p:tgtEl><p:spTgt spid="9"/></p:tgtEl></p:cBhvr></p:animMotion></p:childTnLst></p:cTn></p:par></p:tnLst></p:timing></p:sld>`
	var slide Slide
	if err := xml.Unmarshal([]byte(src), &slide); err != nil {
		t.Fatal(err)
	}
	if len(slide.AnimationEvents) != 1 {
		t.Fatalf("events=%d", len(slide.AnimationEvents))
	}
	ev := slide.AnimationEvents[0]
	if ev.Trigger != "afterPrevious" || ev.MotionPath != "M 0 0 L 1 1" || ev.MotionOrigin != "layout" || ev.RepeatDurMs != 2400 {
		t.Fatalf("unexpected event: %#v", ev)
	}
}
