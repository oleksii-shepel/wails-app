package legacy

import "testing"

func TestResolveCanonicalWord97RunRevisionsResolvesClonedTableProps(t *testing.T) {
	props := &legacyTableRowProps{propertyRevision: &legacyPendingRevisionMark{Present: true, Ibst: 1, HasIbst: true}}
	runs := []docRun{{
		Revision:     "inserted",
		IbstRMark:    0,
		HasIbstRMark: true,
		TableProps:   props.clone(),
	}}

	resolveCanonicalWord97RunRevisions(runs, []string{"Alice", "Bob"})

	if runs[0].RevisionAuthor != "Alice" {
		t.Fatalf("revision author = %q, want Alice", runs[0].RevisionAuthor)
	}
	if runs[0].TableProps == nil || runs[0].TableProps.resolvedPropertyRevision == nil {
		t.Fatalf("table property revision was not resolved: %#v", runs[0].TableProps)
	}
	if runs[0].TableProps.resolvedPropertyRevision.Author != "Bob" {
		t.Fatalf("table property revision author = %q, want Bob", runs[0].TableProps.resolvedPropertyRevision.Author)
	}
	if props.resolvedPropertyRevision != nil {
		t.Fatal("resolution mutated the source PAPX table properties")
	}
}
