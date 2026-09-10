package docformat

// MaterializedField is a format-neutral recursive field tree. R is the
// semantic run carrier owned by the parser that materialized the field.
// Keeping the field grammar independent from a specific binary Word version
// lets canonical Word97 and compatibility parsers share the same field shape
// without making the shared model depend on either parser package.
type MaterializedField[R any] struct {
	Kind      string
	Instr     string
	Arg       string
	Anchor    string
	Target    string
	Children  []R
	InstrRuns []R
}
