package parts

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"
)

func TestSharedItemsMixedTypesRoundTrip(t *testing.T) {
	var items SharedItems
	if err := xml.Unmarshal([]byte(`<sharedItems count="4"><s v="East"/><n v="42"/><b v="1"/><m/></sharedItems>`), &items); err != nil {
		t.Fatal(err)
	}
	if len(items.Items) != 4 || items.Items[0].Type != "s" || items.Items[1].Value != "42" || items.Items[3].Type != "m" {
		t.Fatalf("items = %#v", items.Items)
	}
	var out bytes.Buffer
	if err := xml.NewEncoder(&out).Encode(items); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`<s v="East"></s>`, `<n v="42"></n>`, `<b v="1"></b>`, `<m></m>`} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in %s", want, out.String())
		}
	}
}
