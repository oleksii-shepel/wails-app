package contracts

// SystemSurfaceRecord is the common wire model for synthetic system resources.
// Fields intentionally stay open-ended: presentation columns are contributed by
// the provider and the core file row contract does not grow per system domain.
type SystemSurfaceRecord struct {
	ID     string         `json:"id"`
	Name   string         `json:"name"`
	Fields map[string]any `json:"fields,omitempty"`
}