package smartart

// LayoutDefinition is the parsed dgm:layoutDef algorithm tree.
type LayoutDefinition struct {
	UniqueID string
	Root     *LayoutNode
}

// LayoutNode represents a single dgm:layoutNode in the layout definition.
type LayoutNode struct {
	Name        string
	StyleLbl    string
	Algorithm   *Algorithm
	Shape       *LayoutShape
	PresOf      *PresentationOf
	Constraints []Constraint
	Rules       []Rule
	Variables   map[string]string
	Children    []*LayoutNode
	ForEach     *ForEachSpec
	Choose      *ChooseSpec
}

// Algorithm describes the layout algorithm applied by a layout node.
type Algorithm struct {
	Type   string
	Params map[string]string
}

// LayoutShape describes the shape geometry requested by a layout node.
type LayoutShape struct {
	Type string
	Rot  float64
	Blip string
}

// PresentationOf maps a layout node to a subset of the data-model points.
type PresentationOf struct {
	Axis   string
	PtType string
	Cnt    int
	Start  int
	Step   int
}

// Constraint is a dgm:constr element that sizes or positions layout nodes.
type Constraint struct {
	Type       string
	For        string
	ForName    string
	RefType    string
	RefFor     string
	RefForName string
	Op         string
	Val        float64
	Fact       float64
}

// Rule is a dgm:rule element used to limit font sizes or spacing.
type Rule struct {
	Type    string
	For     string
	ForName string
	PtType  string
	Val     float64
	Fact    float64
	Max     float64
}

// ForEachSpec describes a dgm:forEach expansion over an axis of data points.
type ForEachSpec struct {
	Name          string
	Axis          string
	PtType        string
	Cnt           int
	Start         int
	Step          int
	HideLastTrans bool
	Ref           string
	Children      []*LayoutNode
}

// ChooseSpec describes a dgm:choose branch in the layout tree.
type ChooseSpec struct {
	Name    string
	Cases   []*ChooseCase
	Default *ChooseCase
}

// ChooseCase is a single dgm:if or dgm:else branch.
type ChooseCase struct {
	Name        string
	Func        string
	Arg         string
	Op          string
	Val         string
	Axis        string
	PtType      string
	Constraints []Constraint
	Algorithm   *Algorithm
	Children    []*LayoutNode
}
