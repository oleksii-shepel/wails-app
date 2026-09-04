package legacy

import (
	"testing"

	officeartescher "office/pkg/officeart/legacy/escher"
)

func TestParseLegacyPptConnectorAdjustmentsPropertiesMatchesLegacyUnits(t *testing.T) {
	props := []officeartescher.Property{
		{ID: legacyOfficeArtPropAdjustValue, Value: uint32(10800)},
		{ID: legacyOfficeArtPropAdjustValue + 2, Value: uint32(21600)},
	}
	got := parseLegacyPptConnectorAdjustmentsProperties(props)
	if len(got) != 3 {
		t.Fatalf("len = %d, got %#v", len(got), got)
	}
	if got[0] != 50000 || got[1] != 0 || got[2] != 100000 {
		t.Fatalf("got %#v", got)
	}
}
