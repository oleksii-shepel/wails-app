package diag

import "strings"

// Severity classifies a diagnostic.
type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

// Location identifies where a diagnostic originated.
//
// Part is the preferred field going forward. Path is kept as a compatibility
// alias because a number of older tests and callers still read it directly.
type Location struct {
	Part string
	Path string
}

// NormalizedPart returns the preferred location value.
func (l Location) NormalizedPart() string {
	if strings.TrimSpace(l.Part) != "" {
		return strings.TrimSpace(l.Part)
	}
	return strings.TrimSpace(l.Path)
}

// Diagnostic is one structured parser/resolver message.
type Diagnostic struct {
	Severity Severity
	Code     string
	Message  string
	Location Location
	Cause    error
}

// Error renders the diagnostic as an error string.
func (d Diagnostic) Error() string {
	var b strings.Builder
	severity := strings.TrimSpace(string(d.Severity))
	if severity == "" {
		severity = string(SeverityError)
	}
	b.WriteString(severity)
	b.WriteString(": ")
	b.WriteString(strings.TrimSpace(d.Message))
	if code := strings.TrimSpace(d.Code); code != "" {
		b.WriteString(" (")
		b.WriteString(code)
		b.WriteString(")")
	}
	if part := d.Location.NormalizedPart(); part != "" {
		b.WriteString(" [")
		b.WriteString(part)
		b.WriteString("]")
	}
	if d.Cause != nil {
		b.WriteString(": ")
		b.WriteString(d.Cause.Error())
	}
	return b.String()
}

// List is an ordered collection of diagnostics.
type List []Diagnostic

// Add appends a diagnostic to the list and normalizes legacy location fields.
func (l *List) Add(d Diagnostic) {
	if strings.TrimSpace(d.Location.Part) == "" && strings.TrimSpace(d.Location.Path) != "" {
		d.Location.Part = strings.TrimSpace(d.Location.Path)
	}
	if strings.TrimSpace(d.Location.Path) == "" && strings.TrimSpace(d.Location.Part) != "" {
		d.Location.Path = strings.TrimSpace(d.Location.Part)
	}
	*l = append(*l, d)
}

// HasErrors reports whether the list contains at least one error-severity diagnostic.
func (l List) HasErrors() bool {
	for _, diagnostic := range l {
		if diagnostic.Severity == SeverityError {
			return true
		}
	}
	return false
}
