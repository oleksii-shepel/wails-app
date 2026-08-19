package contracts

type FilterType string

const (
	FilterName      FilterType = "name"
	FilterExtension FilterType = "ext"
	FilterType_     FilterType = "type"
	FilterSize      FilterType = "size"
	FilterModified  FilterType = "modified"
	FilterGlob      FilterType = "glob"
)

type FilterCriteria struct {
	Type     FilterType `json:"type"`
	Value    string     `json:"value"`
	Operator string     `json:"operator,omitempty"`
}

type FilterQuery struct {
	Text          string           `json:"text,omitempty"`
	Criteria      []FilterCriteria `json:"criteria,omitempty"`
	CaseSensitive bool             `json:"caseSensitive"`
}