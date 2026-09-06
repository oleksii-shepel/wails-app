package docformat

import (
	"office/pkg/diag"
	wordir "office/pkg/word/ir"
)

// PendingRevisionMark is unresolved legacy revision attribution carried by
// character, paragraph, section, and table property SPRMs. Resolution against
// SttbfRMark belongs to the higher semantic layer.
type PendingRevisionMark struct {
	Present bool
	Ibst    uint16
	HasIbst bool
	Dttm    uint32
	HasDttm bool
}

// CHPXEntry is one FC-indexed character-formatting record from a ChpxFkp page.
// FC is the raw FcCompressed value of the first character to which Format applies.
type CHPXEntry struct {
	FC                    uint32
	Format                wordir.RunFormat
	Revision              string
	Spec                  bool
	Data                  bool
	Obj                   bool
	IbstRMark             uint16
	HasIbstRMark          bool
	DttmRMark             uint32
	HasDttmRMark          bool
	PropertyRevision      *PendingRevisionMark
	DisplayFieldRevision  *PendingRevisionMark
	Diagnostics           diag.List
	Sprms                 []byte
}
