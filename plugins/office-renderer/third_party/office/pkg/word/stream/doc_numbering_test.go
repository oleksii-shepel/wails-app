package stream

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestNumberingFromReferencePromotesBinaryListReference(t *testing.T) {
	ref := &wordir.NumberingReference{NumID: 9, Level: 2}
	numbering := numberingFromReference(ref)
	if numbering == nil || numbering.NumID != ref.NumID || numbering.Level != ref.Level {
		t.Fatalf("numbering = %#v, want numId=%d level=%d", numbering, ref.NumID, ref.Level)
	}
	if got := numberingFromReference(nil); got != nil {
		t.Fatalf("nil reference numbering = %#v, want nil", got)
	}
}
