package parts

import (
	"strings"
	"testing"
)

func TestLegacyDrawingVMLObjectsParseAndMergePreservesUnknownContent(t *testing.T) {
	raw := []byte(`<xml xmlns:v="urn:schemas-microsoft-com:vml" xmlns:o="urn:schemas-microsoft-com:office:office" xmlns:x="urn:schemas-microsoft-com:office:excel">
      <v:shape id="Custom1" type="#custom" style="position:absolute;width:80pt;height:30pt" fillcolor="yellow" strokecolor="black">
        <o:lock ext="edit" rotation="t"/><v:textbox><div>Original</div></v:textbox>
      </v:shape>
      <v:shape id="Control1" type="#_x0000_t201"><x:ClientData ObjectType="Button"><x:Anchor>0,0,0,0,1,0,1,0</x:Anchor></x:ClientData></v:shape>
    </xml>`)

	objects := ParseLegacyDrawingVMLObjects(raw)
	if len(objects) != 1 {
		t.Fatalf("objects = %#v, want one generic object", objects)
	}
	if got := objects[0]; got.ID != "Custom1" || got.Type != "#custom" || got.Text != "Original" || got.FillColor != "yellow" {
		t.Fatalf("object = %#v", got)
	}
	objects[0].FillColor = "#00ff00"
	objects[0].Text = "Updated"
	merged := string(MergeLegacyDrawingVMLObjects(raw, objects))
	for _, want := range []string{`id="Custom1"`, `fillcolor="#00ff00"`, `Updated`, `rotation="t"`, `id="Control1"`} {
		if !strings.Contains(merged, want) {
			t.Fatalf("merged VML missing %q: %s", want, merged)
		}
	}
}

func TestLegacyDrawingVMLObjectsCanAddAndRemoveObjects(t *testing.T) {
	raw := []byte(`<xml xmlns:v="urn:schemas-microsoft-com:vml"><v:shape id="Old" style="width:10pt"/></xml>`)
	merged := MergeLegacyDrawingVMLObjects(raw, []VMLObject{{ID: "New", Style: "width:20pt", FillColor: "red", Text: "Hello"}})
	text := string(merged)
	if strings.Contains(text, `id="Old"`) || !strings.Contains(text, `id="New"`) || !strings.Contains(text, `Hello`) {
		t.Fatalf("unexpected merged VML: %s", text)
	}
}
