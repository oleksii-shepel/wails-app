package parts_test

import (
	"encoding/xml"
	"testing"

	"office/pkg/presentation/parsers/ooxml/parts"
)

const slideWithFadeTransition = `
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld>
    <p:spTree>
      <p:nvGrpSpPr/>
      <p:grpSpPr/>
    </p:spTree>
  </p:cSld>
  <p:transition spd="med" advClick="1" advTm="3000">
    <p:fade/>
  </p:transition>
</p:sld>`

const slideWithPushTransition = `
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld>
    <p:spTree>
      <p:nvGrpSpPr/>
      <p:grpSpPr/>
    </p:spTree>
  </p:cSld>
  <p:transition spd="slow" advClick="0">
    <p:push dir="l"/>
  </p:transition>
</p:sld>`

const slideNoTransition = `
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld>
    <p:spTree>
      <p:nvGrpSpPr/>
      <p:grpSpPr/>
    </p:spTree>
  </p:cSld>
</p:sld>`

const slideWithTiming = `
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld>
    <p:spTree>
      <p:nvGrpSpPr/>
      <p:grpSpPr/>
    </p:spTree>
  </p:cSld>
  <p:timing>
    <p:tnLst>
      <p:par>
        <p:cTn id="1" dur="indefinite">
          <p:tnLst>
            <p:par>
              <p:cTn id="2" fill="hold">
                <p:tnLst>
                  <p:par>
                    <p:cTn id="3" presetClass="entr" nodeType="clickEffect" grpId="0" fill="hold">
                      <p:tnLst>
                        <p:par>
                          <p:cTn id="4">
                            <p:childTnLst>
                              <p:animEffect transition="in" filter="fade">
                                <p:tgtEl><p:spTgt spid="10"/></p:tgtEl>
                              </p:animEffect>
                            </p:childTnLst>
                          </p:cTn>
                        </p:par>
                      </p:tnLst>
                    </p:cTn>
                  </p:par>
                  <p:par>
                    <p:cTn id="5" presetClass="entr" nodeType="clickEffect" grpId="1" fill="hold">
                      <p:tnLst>
                        <p:par>
                          <p:cTn id="6">
                            <p:childTnLst>
                              <p:animEffect transition="in" filter="fade">
                                <p:tgtEl><p:spTgt spid="20"/></p:tgtEl>
                              </p:animEffect>
                            </p:childTnLst>
                          </p:cTn>
                        </p:par>
                      </p:tnLst>
                    </p:cTn>
                  </p:par>
                  <p:par>
                    <p:cTn id="7" presetClass="entr" nodeType="withEffect" grpId="1" fill="hold">
                      <p:tnLst>
                        <p:par>
                          <p:cTn id="8">
                            <p:childTnLst>
                              <p:animEffect transition="in" filter="fade">
                                <p:tgtEl><p:spTgt spid="30"/></p:tgtEl>
                              </p:animEffect>
                            </p:childTnLst>
                          </p:cTn>
                        </p:par>
                      </p:tnLst>
                    </p:cTn>
                  </p:par>
                </p:tnLst>
              </p:cTn>
            </p:par>
          </p:tnLst>
        </p:cTn>
      </p:par>
    </p:tnLst>
  </p:timing>
</p:sld>`

func TestSlideTransitionFade(t *testing.T) {
	var s parts.Slide
	if err := xml.Unmarshal([]byte(slideWithFadeTransition), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.Transition == nil {
		t.Fatal("Transition is nil, want non-nil")
	}
	if s.Transition.Type != "fade" {
		t.Errorf("Type = %q, want %q", s.Transition.Type, "fade")
	}
	if s.Transition.Spd != "med" {
		t.Errorf("Spd = %q, want %q", s.Transition.Spd, "med")
	}
	if !s.Transition.AdvClick {
		t.Error("AdvClick = false, want true")
	}
	if s.Transition.AdvTmMs != 3000 {
		t.Errorf("AdvTmMs = %d, want 3000", s.Transition.AdvTmMs)
	}
}

func TestSlideTransitionPushNoAutoAdvance(t *testing.T) {
	var s parts.Slide
	if err := xml.Unmarshal([]byte(slideWithPushTransition), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.Transition == nil {
		t.Fatal("Transition is nil, want non-nil")
	}
	if s.Transition.Type != "push" {
		t.Errorf("Type = %q, want %q", s.Transition.Type, "push")
	}
	if s.Transition.Spd != "slow" {
		t.Errorf("Spd = %q, want %q", s.Transition.Spd, "slow")
	}
	if s.Transition.AdvClick {
		t.Error("AdvClick = true, want false (advClick=\"0\")")
	}
	if s.Transition.AdvTmMs != 0 {
		t.Errorf("AdvTmMs = %d, want 0", s.Transition.AdvTmMs)
	}
}

func TestSlideNoTransition(t *testing.T) {
	var s parts.Slide
	if err := xml.Unmarshal([]byte(slideNoTransition), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.Transition != nil {
		t.Errorf("Transition = %+v, want nil", s.Transition)
	}
}

func TestSlideTimingAnimSteps(t *testing.T) {
	var s parts.Slide
	if err := xml.Unmarshal([]byte(slideWithTiming), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(s.AnimSteps) == 0 {
		t.Fatal("AnimSteps is empty, want entries for shapes 10, 20, 30")
	}
	// Shape 10 has grpId=0 → click step 1
	if got := s.AnimSteps[10]; got != 1 {
		t.Errorf("AnimSteps[10] = %d, want 1", got)
	}
	// Shape 20 has grpId=1 → click step 2
	if got := s.AnimSteps[20]; got != 2 {
		t.Errorf("AnimSteps[20] = %d, want 2", got)
	}
	// Shape 30 has grpId=1 (withEffect) → same click step 2
	if got := s.AnimSteps[30]; got != 2 {
		t.Errorf("AnimSteps[30] = %d, want 2", got)
	}
}

func TestSlideTransitionMarshalRoundTrip(t *testing.T) {
	original := &parts.SlideTransition{
		Type:     "wipe",
		Spd:      "fast",
		AdvClick: false,
		AdvTmMs:  5000,
	}
	data, err := xml.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got parts.SlideTransition
	if err := xml.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Type != "wipe" {
		t.Errorf("Type = %q, want wipe", got.Type)
	}
	if got.Spd != "fast" {
		t.Errorf("Spd = %q, want fast", got.Spd)
	}
	if got.AdvClick {
		t.Error("AdvClick = true, want false")
	}
	if got.AdvTmMs != 5000 {
		t.Errorf("AdvTmMs = %d, want 5000", got.AdvTmMs)
	}
}

func TestApplicationNonVisualDrawingPropsMarshalXML(t *testing.T) {
	props := parts.ApplicationNonVisualDrawingProps{
		Ph: &parts.PlaceholderInfo{Type: "title"},
		VideoFile: &parts.MediaReference{Embed: "rId2"},
		AudioFile: &parts.MediaReference{Link: "rId3"},
	}
	data, err := xml.Marshal(props)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got parts.ApplicationNonVisualDrawingProps
	if err := xml.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Ph == nil || got.Ph.Type != "title" {
		t.Errorf("ph = %+v, want title", got.Ph)
	}
	if got.VideoFile == nil || got.VideoFile.Embed != "rId2" {
		t.Errorf("videoFile = %+v, want embed=rId2", got.VideoFile)
	}
	if got.AudioFile == nil || got.AudioFile.Link != "rId3" {
		t.Errorf("audioFile = %+v, want link=rId3", got.AudioFile)
	}
}

func TestMediaReferenceFromStartBareAttributes(t *testing.T) {
	// Verify that media references with bare (non-namespaced) embed/link attributes
	// are parsed correctly, matching the output of our custom MarshalXML.
	const xmlData = `<nvPr><videoFile embed="rId1"/><audioFile link="rId2"/></nvPr>`
	var got parts.ApplicationNonVisualDrawingProps
	if err := xml.Unmarshal([]byte(xmlData), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.VideoFile == nil || got.VideoFile.Embed != "rId1" {
		t.Errorf("videoFile embed = %q, want rId1", got.VideoFile.Embed)
	}
	if got.AudioFile == nil || got.AudioFile.Link != "rId2" {
		t.Errorf("audioFile link = %q, want rId2", got.AudioFile.Link)
	}
}

func TestSlideTransitionWithDirSoundDur(t *testing.T) {
	const xmlData = `<p:transition xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" spd="slow" advClick="0">
  <p:push dir="l" dur="750"/>
  <p:snd embed="rIdSound1"/>
</p:transition>`
	var tr parts.SlideTransition
	if err := xml.Unmarshal([]byte(xmlData), &tr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if tr.Type != "push" {
		t.Errorf("Type = %q, want push", tr.Type)
	}
	if tr.Dir != "l" {
		t.Errorf("Dir = %q, want l", tr.Dir)
	}
	if tr.Dur != 750 {
		t.Errorf("Dur = %d, want 750", tr.Dur)
	}
	if tr.Sound != "rIdSound1" {
		t.Errorf("Sound = %q, want rIdSound1", tr.Sound)
	}

	// Round-trip.
	data, err := xml.Marshal(&tr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got parts.SlideTransition
	if err := xml.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal round-trip: %v", err)
	}
	if got.Dir != "l" {
		t.Errorf("round-trip Dir = %q, want l", got.Dir)
	}
	if got.Dur != 750 {
		t.Errorf("round-trip Dur = %d, want 750", got.Dur)
	}
	if got.Sound != "rIdSound1" {
		t.Errorf("round-trip Sound = %q, want rIdSound1", got.Sound)
	}
}