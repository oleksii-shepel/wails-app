package render

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"office/pkg/diag"
	"office/pkg/ooxml/smartart"
)

// layoutContext carries the data-model nodes associated with a layout node.
type layoutContext struct {
	// nodes are the data-model points that the current layout node positions.
	nodes []*smartart.Node
	// vars holds layout variables inherited from ancestor varLst elements.
	vars map[string]string
	// position/total describe the current item when executing a forEach. Positions
	// are 1-based to match DiagramML's pos/revPos semantics.
	position int
	total    int
	// named remembers which data nodes were associated with named layout nodes,
	// allowing forName/refForName constraints to target the intended set.
	named map[string][]*smartart.Node
}

// runLayoutDefinition executes the parsed dgm:layoutDef algorithm tree. It
// returns true when it was able to position the diagram's nodes.
func (e *LayoutEngine) runLayoutDefinition() bool {
	if e.Diagram == nil || e.Diagram.Layout == nil || e.Diagram.Layout.Definition == nil {
		return false
	}

	root := firstRoot(e.Diagram.Nodes)
	var nodes []*smartart.Node
	if root != nil {
		nodes = root.Children
	} else if len(e.Diagram.Nodes) > 0 {
		nodes = e.Diagram.Nodes
	}
	if len(nodes) == 0 {
		return false
	}

	ctx := &layoutContext{nodes: nodes, vars: map[string]string{"dir": "norm"}, total: len(nodes), named: map[string][]*smartart.Node{}}
	return e.visitLayoutNode(e.Diagram.Layout.Definition.Root, ctx)
}

func firstRoot(nodes []*smartart.Node) *smartart.Node {
	for _, n := range nodes {
		if n.Parent == nil {
			return n
		}
	}
	return nil
}

func (e *LayoutEngine) visitLayoutNode(ln *smartart.LayoutNode, ctx *layoutContext) bool {
	if ln == nil {
		return true
	}

	// Merge layout variables from this node.
	childVars := mergeVars(ctx.vars, ln.Variables)
	childCtx := &layoutContext{nodes: ctx.nodes, vars: childVars, position: ctx.position, total: ctx.total, named: ctx.named}
	if childCtx.named == nil {
		childCtx.named = map[string][]*smartart.Node{}
	}
	if ln.PresOf != nil {
		childCtx.nodes = e.presentationOfNodes(ln.PresOf, childCtx)
		childCtx.total = len(childCtx.nodes)
	}
	e.applyLayoutShape(ln.Shape, childCtx)
	if name := strings.TrimSpace(ln.Name); name != "" {
		childCtx.named[strings.ToLower(name)] = append([]*smartart.Node(nil), childCtx.nodes...)
	}

	// Apply geometry constraints and rules before algorithms so sizing participates in layout.
	e.applySizeConstraints(ln.Constraints, childCtx)
	e.applyRules(ln.Rules, childCtx)

	// Evaluate a choose branch first; it may supply an algorithm and children.
	if ln.Choose != nil {
		chosen := e.chooseCase(ln.Choose, childCtx)
		if chosen != nil {
			e.applySizeConstraints(chosen.Constraints, childCtx)
			if chosen.Algorithm != nil {
				if !e.runAlgorithm(chosen.Algorithm, childCtx) {
					return false
				}
			}
			for _, c := range chosen.Children {
				if !e.visitLayoutNode(c, childCtx) {
					return false
				}
			}
			e.applyPositionConstraints(chosen.Constraints, childCtx)
			e.applyPositionConstraints(ln.Constraints, childCtx)
			return true
		}
	}

	// If this layout node has its own algorithm, run it against the current
	// data-node context.
	if ln.Algorithm != nil {
		if !e.runAlgorithm(ln.Algorithm, childCtx) {
			return false
		}
	}

	// Process forEach expansions.
	if ln.ForEach != nil {
		ok := e.runForEach(ln.ForEach, childCtx)
		if ok {
			e.applyPositionConstraints(ln.Constraints, childCtx)
		}
		return ok
	}

	// Recurse into ordinary children.
	for _, c := range ln.Children {
		if !e.visitLayoutNode(c, childCtx) {
			return false
		}
	}
	e.applyPositionConstraints(ln.Constraints, childCtx)
	return true
}

func (e *LayoutEngine) chooseCase(ch *smartart.ChooseSpec, ctx *layoutContext) *smartart.ChooseCase {
	if ch == nil {
		return nil
	}
	for _, c := range ch.Cases {
		if e.evaluateCase(c, ctx) {
			return c
		}
	}
	return ch.Default
}

func (e *LayoutEngine) evaluateCase(c *smartart.ChooseCase, ctx *layoutContext) bool {
	if c == nil {
		return false
	}
	fn := strings.ToLower(strings.TrimSpace(c.Func))
	arg := strings.ToLower(strings.TrimSpace(c.Arg))
	op := strings.ToLower(strings.TrimSpace(c.Op))
	val := strings.TrimSpace(c.Val)

	selected := e.selectAxisNodes(c.Axis, c.PtType, ctx)
	if strings.TrimSpace(c.Axis) == "" {
		selected = ctx.nodes
	}
	switch fn {
	case "var":
		got := strings.ToLower(strings.TrimSpace(ctx.vars[arg]))
		return compareString(got, strings.ToLower(strings.TrimSpace(val)), op)
	case "cnt":
		want, err := strconv.Atoi(val)
		return err == nil && compareInt(len(selected), want, op)
	case "pos":
		want, err := strconv.Atoi(val)
		return err == nil && compareInt(contextPosition(ctx), want, op)
	case "revpos":
		want, err := strconv.Atoi(val)
		return err == nil && compareInt(contextReversePosition(ctx), want, op)
	case "poseven":
		return compareBool(contextPosition(ctx)%2 == 0, val, op)
	case "posodd":
		return compareBool(contextPosition(ctx)%2 == 1, val, op)
	case "depth":
		want, err := strconv.Atoi(val)
		return err == nil && compareInt(contextDepth(selected), want, op)
	case "maxdepth":
		want, err := strconv.Atoi(val)
		return err == nil && compareInt(contextMaxDepth(selected), want, op)
	}
	return false
}

func compareString(got, want, op string) bool {
	switch op {
	case "equ", "==", "=":
		return got == want
	case "neq":
		return got != want
	}
	return false
}

func compareInt(got, want int, op string) bool {
	switch op {
	case "equ", "==", "=":
		return got == want
	case "neq":
		return got != want
	case "gt":
		return got > want
	case "gte":
		return got >= want
	case "lt":
		return got < want
	case "lte":
		return got <= want
	}
	return false
}

func compareBool(got bool, raw, op string) bool {
	want := true
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "0", "false", "f", "no":
		want = false
	case "", "1", "true", "t", "yes":
	default:
		if v, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil {
			want = v != 0
		}
	}
	if strings.EqualFold(strings.TrimSpace(op), "neq") {
		return got != want
	}
	return got == want
}

func contextPosition(ctx *layoutContext) int {
	if ctx != nil && ctx.position > 0 {
		return ctx.position
	}
	return 1
}

func contextReversePosition(ctx *layoutContext) int {
	if ctx == nil {
		return 1
	}
	total := ctx.total
	if total <= 0 {
		total = len(ctx.nodes)
	}
	pos := contextPosition(ctx)
	if total < pos {
		return 1
	}
	return total - pos + 1
}

func contextDepth(nodes []*smartart.Node) int {
	if len(nodes) == 0 || nodes[0] == nil {
		return 0
	}
	return nodes[0].Level
}

func contextMaxDepth(nodes []*smartart.Node) int {
	maxDepth := 0
	var walk func(*smartart.Node)
	walk = func(n *smartart.Node) {
		if n == nil {
			return
		}
		if n.Level > maxDepth {
			maxDepth = n.Level
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	for _, n := range nodes {
		walk(n)
	}
	return maxDepth
}

func mergeVars(parent map[string]string, child map[string]string) map[string]string {
	out := make(map[string]string, len(parent)+len(child))
	for k, v := range parent {
		out[k] = v
	}
	for k, v := range child {
		out[k] = v
	}
	return out
}

func (e *LayoutEngine) runForEach(fe *smartart.ForEachSpec, ctx *layoutContext) bool {
	if fe == nil {
		return true
	}
	nodes := e.selectAxisNodes(fe.Axis, fe.PtType, ctx)
	if len(nodes) == 0 {
		return true
	}
	start := fe.Start
	if start <= 0 {
		start = 1
	}
	step := fe.Step
	if step == 0 {
		step = 1
	}
	indexes := make([]int, 0, len(nodes))
	if step > 0 {
		for i := start - 1; i < len(nodes); i += step {
			indexes = append(indexes, i)
		}
	} else {
		i := start - 1
		if i < 0 || i >= len(nodes) {
			i = len(nodes) - 1
		}
		for ; i >= 0; i += step {
			indexes = append(indexes, i)
		}
	}
	if fe.Cnt > 0 && len(indexes) > fe.Cnt {
		indexes = indexes[:fe.Cnt]
	}
	for iter, idx := range indexes {
		childCtx := &layoutContext{nodes: []*smartart.Node{nodes[idx]}, vars: ctx.vars, position: iter + 1, total: len(indexes), named: ctx.named}
		for _, c := range fe.Children {
			if !e.visitLayoutNode(c, childCtx) {
				return false
			}
		}
	}
	return true
}

func (e *LayoutEngine) selectAxisNodes(axis, ptType string, ctx *layoutContext) []*smartart.Node {
	axis = strings.ToLower(strings.TrimSpace(axis))
	var out []*smartart.Node
	switch axis {
	case "", "self":
		out = ctx.nodes
	case "ch":
		for _, n := range ctx.nodes {
			if n != nil {
				out = append(out, n.Children...)
			}
		}
	case "des", "desorself":
		var walk func(*smartart.Node)
		walk = func(n *smartart.Node) {
			if n == nil {
				return
			}
			for _, c := range n.Children {
				out = append(out, c)
				walk(c)
			}
		}
		if axis == "desorself" {
			out = append(out, ctx.nodes...)
		}
		for _, n := range ctx.nodes {
			walk(n)
		}
	case "par":
		out = uniqueParents(ctx.nodes)
	case "ancst", "ancstorself":
		seen := map[*smartart.Node]bool{}
		if axis == "ancstorself" {
			for _, n := range ctx.nodes {
				if n != nil && !seen[n] {
					seen[n] = true
					out = append(out, n)
				}
			}
		}
		for _, n := range ctx.nodes {
			if n == nil {
				continue
			}
			for p := n.Parent; p != nil; p = p.Parent {
				if !seen[p] {
					seen[p] = true
					out = append(out, p)
				}
			}
		}
	case "followsib", "followingsib":
		out = siblingAxisNodes(ctx.nodes, true)
	case "precedsib", "precedingsib":
		out = siblingAxisNodes(ctx.nodes, false)
	default:
		out = ctx.nodes
	}
	return filterPointType(out, ptType)
}

func filterPointType(nodes []*smartart.Node, ptType string) []*smartart.Node {
	pt := strings.ToLower(strings.TrimSpace(ptType))
	if pt == "" || pt == "all" {
		return nodes
	}
	out := make([]*smartart.Node, 0, len(nodes))
	for _, n := range nodes {
		if n == nil {
			continue
		}
		t := strings.ToLower(strings.TrimSpace(n.Type))
		match := t == pt
		if pt == "node" {
			match = !n.Assistant && (t == "" || t == "node" || t == "root" || t == "doc")
		}
		if pt == "asst" || pt == "assistant" {
			match = n.Assistant || t == "asst" || t == "assistant"
		}
		if match {
			out = append(out, n)
		}
	}
	return out
}

func uniqueParents(nodes []*smartart.Node) []*smartart.Node {
	var out []*smartart.Node
	seen := map[*smartart.Node]bool{}
	for _, n := range nodes {
		if n == nil || n.Parent == nil || seen[n.Parent] {
			continue
		}
		seen[n.Parent] = true
		out = append(out, n.Parent)
	}
	return out
}

func siblingAxisNodes(nodes []*smartart.Node, following bool) []*smartart.Node {
	var out []*smartart.Node
	seen := map[*smartart.Node]bool{}
	for _, n := range nodes {
		if n == nil || n.Parent == nil {
			continue
		}
		siblings := n.Parent.Children
		idx := -1
		for i, s := range siblings {
			if s == n {
				idx = i
				break
			}
		}
		if idx < 0 {
			continue
		}
		start, end := 0, idx
		if following {
			start, end = idx+1, len(siblings)
		}
		for _, s := range siblings[start:end] {
			if !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	return out
}

func (e *LayoutEngine) runAlgorithm(alg *smartart.Algorithm, ctx *layoutContext) bool {
	if alg == nil || len(ctx.nodes) == 0 {
		return true
	}
	typeStr := strings.ToLower(strings.TrimSpace(alg.Type))
	switch typeStr {
	case "lin":
		dir := strings.ToLower(strings.TrimSpace(alg.Params["linDir"]))
		e.layoutLinear(ctx.nodes, dir, alg.Params)
	case "snake":
		e.layoutSnake(ctx.nodes, alg.Params)
	case "cycle":
		e.layoutCycleNodes(ctx.nodes, alg.Params)
	case "pyra":
		e.layoutPyramidNodes(ctx.nodes, alg.Params)
	case "hierroot", "hierchild":
		// Hierarchy root/child algorithms are complementary in DiagramML. Apply
		// the common child-direction parameter before executing the semantic tree.
		if e.Diagram.Layout != nil {
			switch strings.ToLower(strings.TrimSpace(alg.Params["chDir"])) {
			case "horz":
				e.Diagram.Layout.Direction = "leftRight"
			case "vert":
				e.Diagram.Layout.Direction = "topDown"
			}
		}
		e.calculateHierarchyPositions()
	case "conn":
		// Connector geometry is materialized after nodes become presentation
		// shapes. Preserve the active DiagramML connector parameters on the
		// semantic layout so the presentation resolver can synthesize the same
		// routing when no pre-rendered dsp:cxnSp is available.
		if e.Diagram != nil && e.Diagram.Layout != nil {
			l := e.Diagram.Layout
			l.ConnectorRouting = strings.TrimSpace(alg.Params["connRout"])
			l.ConnectorBendPoint = strings.TrimSpace(alg.Params["bendPt"])
			l.ConnectorBeginPts = strings.TrimSpace(alg.Params["begPts"])
			l.ConnectorEndPts = strings.TrimSpace(alg.Params["endPts"])
			l.ConnectorSource = strings.TrimSpace(alg.Params["srcNode"])
			l.ConnectorDest = strings.TrimSpace(alg.Params["dstNode"])
			l.ConnectorDimension = strings.TrimSpace(alg.Params["dim"])
			l.ConnectorBeginStyle = strings.TrimSpace(alg.Params["begSty"])
			l.ConnectorEndStyle = strings.TrimSpace(alg.Params["endSty"])
			l.ConnectorSourceIDs = nodeIDsForNamedLayout(ctx, l.ConnectorSource)
			l.ConnectorDestIDs = nodeIDsForNamedLayout(ctx, l.ConnectorDest)
		}
		return true
	case "tx", "sp", "composite", "none":
		// Text-only, spacer, or composite container: no positioning here.
		return true
	default:
		// Unsupported algorithm: record why the layout-definition executor fell
		// back instead of silently degrading to a heuristic layout.
		e.addLayoutDiagnostic("smartart.unsupported-layout-algorithm", fmt.Sprintf("Unsupported SmartArt layout algorithm %q; using fallback layout", alg.Type))
		return false
	}
	return true
}

func nodeIDsForNamedLayout(ctx *layoutContext, name string) []string {
	name = strings.ToLower(strings.TrimSpace(name))
	if ctx == nil || name == "" || ctx.named == nil {
		return nil
	}
	nodes := ctx.named[name]
	if len(nodes) == 0 {
		return nil
	}
	out := make([]string, 0, len(nodes))
	seen := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		if n == nil || strings.TrimSpace(n.ID) == "" || seen[n.ID] {
			continue
		}
		seen[n.ID] = true
		out = append(out, n.ID)
	}
	return out
}

// layoutBox holds a node and its resolved size while an algorithm runs.
type layoutBox struct {
	node       *smartart.Node
	x, y, w, h float64
}

// defaultBox creates a box with a default size for a data-model node.
func (e *LayoutEngine) defaultBox(n *smartart.Node) layoutBox {
	w, h := 150.0, 60.0
	if n != nil {
		if n.Position.Width > 0 {
			w = n.Position.Width
		}
		if n.Position.Height > 0 {
			h = n.Position.Height
		}
	}
	return layoutBox{node: n, w: w, h: h}
}

func (e *LayoutEngine) layoutLinear(nodes []*smartart.Node, dir string, params map[string]string) {
	if len(nodes) == 0 {
		return
	}
	boxes := make([]layoutBox, len(nodes))
	maxW, maxH := 0.0, 0.0
	for i, n := range nodes {
		boxes[i] = e.defaultBox(n)
		maxW = math.Max(maxW, boxes[i].w)
		maxH = math.Max(maxH, boxes[i].h)
	}

	spacing := paramFloat(params, "sp", 30.0)
	startX, startY := 50.0, 50.0
	hAlign := strings.ToLower(strings.TrimSpace(params["horzAlign"]))
	vAlign := strings.ToLower(strings.TrimSpace(params["vertAlign"]))
	isHorizontal := dir == "froml" || dir == "fromleft" || dir == "fromr" || dir == "fromright"

	if isHorizontal {
		x := startX
		order := make([]int, len(boxes))
		for i := range order {
			order[i] = i
		}
		if dir == "fromr" || dir == "fromright" {
			for i := range order {
				order[i] = len(order) - 1 - i
			}
		}
		for _, idx := range order {
			b := &boxes[idx]
			b.x = x
			switch vAlign {
			case "mid", "ctr":
				b.y = startY + (maxH-b.h)/2
			case "b":
				b.y = startY + maxH - b.h
			default:
				b.y = startY
			}
			x += b.w + spacing
		}
	} else {
		y := startY
		order := make([]int, len(boxes))
		for i := range order {
			order[i] = i
		}
		if dir == "fromb" || dir == "frombottom" {
			for i := range order {
				order[i] = len(order) - 1 - i
			}
		}
		for _, idx := range order {
			b := &boxes[idx]
			b.y = y
			switch hAlign {
			case "ctr", "mid":
				b.x = startX + (maxW-b.w)/2
			case "r":
				b.x = startX + maxW - b.w
			default:
				b.x = startX
			}
			y += b.h + spacing
		}
	}

	for _, b := range boxes {
		b.node.Position = smartart.Position{X: b.x, Y: b.y, Width: b.w, Height: b.h}
		b.node.HasPosition = true
	}
}

func (e *LayoutEngine) layoutSnake(nodes []*smartart.Node, params map[string]string) {
	if len(nodes) == 0 {
		return
	}
	boxes := make([]layoutBox, len(nodes))
	for i, n := range nodes {
		boxes[i] = e.defaultBox(n)
	}
	flowDir := strings.ToLower(strings.TrimSpace(params["flowDir"]))
	spacing := paramFloat(params, "sp", 30.0)
	startX, startY := 50.0, 50.0
	limit := 0
	if strings.EqualFold(strings.TrimSpace(params["bkpt"]), "fixed") {
		if v, err := strconv.Atoi(strings.TrimSpace(params["bkPtFixedVal"])); err == nil && v > 0 {
			limit = v
		}
	}
	if limit <= 0 {
		if flowDir == "col" {
			limit = 4
		} else {
			limit = 4
		}
	}
	reverseAlternate := strings.EqualFold(strings.TrimSpace(params["contDir"]), "revDir")
	grow := strings.ToLower(strings.TrimSpace(params["grDir"]))

	if flowDir == "col" {
		for i := range boxes {
			col, row := i/limit, i%limit
			visualRow := row
			if reverseAlternate && col%2 == 1 {
				visualRow = limit - 1 - row
			}
			boxes[i].x = startX + float64(col)*(boxes[i].w+spacing)
			boxes[i].y = startY + float64(visualRow)*(boxes[i].h+spacing)
		}
	} else {
		for i := range boxes {
			row, col := i/limit, i%limit
			visualCol := col
			if reverseAlternate && row%2 == 1 {
				visualCol = limit - 1 - col
			}
			boxes[i].x = startX + float64(visualCol)*(boxes[i].w+spacing)
			boxes[i].y = startY + float64(row)*(boxes[i].h+spacing)
		}
	}
	// Mirror from the requested growth corner after laying out from top-left.
	minX, minY, maxX, maxY := boxBounds(boxes)
	for i := range boxes {
		if strings.Contains(grow, "r") {
			boxes[i].x = minX + maxX - (boxes[i].x + boxes[i].w)
		}
		if strings.Contains(grow, "b") {
			boxes[i].y = minY + maxY - (boxes[i].y + boxes[i].h)
		}
	}
	for _, b := range boxes {
		b.node.Position = smartart.Position{X: b.x, Y: b.y, Width: b.w, Height: b.h}
		b.node.HasPosition = true
	}
}

func (e *LayoutEngine) layoutCycleNodes(nodes []*smartart.Node, params map[string]string) {
	if len(nodes) == 0 {
		return
	}
	centerX, centerY := 400.0, 300.0
	radius := paramFloat(params, "rad", 200.0)
	startDeg := drawingAngleDegrees(params["stAng"], -90.0)
	spanDeg := drawingAngleDegrees(params["spanAng"], 360.0)
	start := startDeg * math.Pi / 180
	span := spanDeg * math.Pi / 180
	step := 0.0
	if len(nodes) > 1 {
		if math.Abs(math.Abs(spanDeg)-360) < 1e-6 {
			step = span / float64(len(nodes))
		} else {
			step = span / float64(len(nodes)-1)
		}
	}
	for i, n := range nodes {
		b := e.defaultBox(n)
		angle := start + float64(i)*step
		n.Position = smartart.Position{X: centerX + radius*math.Cos(angle) - b.w/2, Y: centerY + radius*math.Sin(angle) - b.h/2, Width: b.w, Height: b.h}
		n.HasPosition = true
	}
}

func (e *LayoutEngine) layoutPyramidNodes(nodes []*smartart.Node, params map[string]string) {
	if len(nodes) == 0 {
		return
	}
	const (
		centerX = 400.0
		startY  = 50.0
		baseW   = 600.0
		rowH    = 70.0
	)
	for i, n := range nodes {
		frac := float64(i+1) / float64(len(nodes))
		w := math.Max(120, baseW*frac)
		n.Position = smartart.Position{
			X:     centerX - w/2,
			Y:     startY + float64(i)*rowH,
			Width: w, Height: 60,
		}
		n.HasPosition = true
	}
}

func paramFloat(params map[string]string, key string, fallback float64) float64 {
	if params == nil {
		return fallback
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(params[key]), 64)
	if err != nil {
		return fallback
	}
	return v
}

func drawingAngleDegrees(raw string, fallback float64) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return fallback
	}
	// DiagramML angle-valued parameters use the DrawingML 1/60000 degree unit.
	if math.Abs(v) > 360 {
		return v / 60000.0
	}
	return v
}

func boxBounds(boxes []layoutBox) (minX, minY, maxX, maxY float64) {
	if len(boxes) == 0 {
		return
	}
	minX, minY = boxes[0].x, boxes[0].y
	maxX, maxY = boxes[0].x+boxes[0].w, boxes[0].y+boxes[0].h
	for _, b := range boxes[1:] {
		minX = math.Min(minX, b.x)
		minY = math.Min(minY, b.y)
		maxX = math.Max(maxX, b.x+b.w)
		maxY = math.Max(maxY, b.y+b.h)
	}
	return
}

func (e *LayoutEngine) applySizeConstraints(constraints []smartart.Constraint, ctx *layoutContext) {
	for _, c := range constraints {
		typ := strings.ToLower(strings.TrimSpace(c.Type))
		if typ != "w" && typ != "h" && typ != "ar" && typ != "woff" && typ != "hoff" {
			continue
		}
		for _, n := range e.constraintTargets(c.For, c.ForName, ctx) {
			if n == nil {
				continue
			}
			if n.Position.Width <= 0 {
				n.Position.Width = 150
			}
			if n.Position.Height <= 0 {
				n.Position.Height = 60
			}
			value, ok := e.constraintValue(c, ctx)
			if !ok {
				continue
			}
			switch typ {
			case "w":
				n.Position.Width = applyNumericConstraint(n.Position.Width, value, c.Op)
			case "h":
				n.Position.Height = applyNumericConstraint(n.Position.Height, value, c.Op)
			case "ar":
				if value > 0 {
					current := n.Position.Width / n.Position.Height
					if current > value {
						n.Position.Width = n.Position.Height * value
					} else {
						n.Position.Height = n.Position.Width / value
					}
				}
			case "woff":
				n.Position.Width = math.Max(0, n.Position.Width+value)
			case "hoff":
				n.Position.Height = math.Max(0, n.Position.Height+value)
			}
			n.HasPosition = true
		}
	}
}

func (e *LayoutEngine) applyPositionConstraints(constraints []smartart.Constraint, ctx *layoutContext) {
	for _, c := range constraints {
		typ := strings.ToLower(strings.TrimSpace(c.Type))
		if typ != "l" && typ != "t" && typ != "loff" && typ != "toff" && typ != "r" && typ != "b" {
			continue
		}
		for _, n := range e.constraintTargets(c.For, c.ForName, ctx) {
			if n == nil {
				continue
			}
			value, ok := e.constraintValue(c, ctx)
			if !ok {
				continue
			}
			switch typ {
			case "l":
				n.Position.X = applyNumericConstraint(n.Position.X, value, c.Op)
			case "t":
				n.Position.Y = applyNumericConstraint(n.Position.Y, value, c.Op)
			case "loff":
				n.Position.X += value
			case "toff":
				n.Position.Y += value
			case "r":
				n.Position.X = applyNumericConstraint(n.Position.X+n.Position.Width, value, c.Op) - n.Position.Width
			case "b":
				n.Position.Y = applyNumericConstraint(n.Position.Y+n.Position.Height, value, c.Op) - n.Position.Height
			}
		}
	}
}

func (e *LayoutEngine) constraintTargets(rel, name string, ctx *layoutContext) []*smartart.Node {
	if key := strings.ToLower(strings.TrimSpace(name)); key != "" && ctx != nil && ctx.named != nil {
		if nodes, ok := ctx.named[key]; ok {
			return nodes
		}
	}
	if strings.TrimSpace(rel) == "" {
		rel = "self"
	}
	return e.selectAxisNodes(rel, "", ctx)
}

func (e *LayoutEngine) constraintValue(c smartart.Constraint, ctx *layoutContext) (float64, bool) {
	if strings.TrimSpace(c.RefType) == "" {
		return c.Val, true
	}
	refRel := c.RefFor
	if strings.TrimSpace(refRel) == "" {
		refRel = "self"
	}
	var refs []*smartart.Node
	if key := strings.ToLower(strings.TrimSpace(c.RefForName)); key != "" && ctx != nil && ctx.named != nil {
		refs = ctx.named[key]
	}
	if len(refs) == 0 {
		refs = e.selectAxisNodes(refRel, "", ctx)
	}
	if len(refs) == 0 || refs[0] == nil {
		return 0, false
	}
	ref := refs[0]
	var v float64
	switch strings.ToLower(strings.TrimSpace(c.RefType)) {
	case "w":
		v = ref.Position.Width
	case "h":
		v = ref.Position.Height
	case "l":
		v = ref.Position.X
	case "t":
		v = ref.Position.Y
	case "r":
		v = ref.Position.X + ref.Position.Width
	case "b":
		v = ref.Position.Y + ref.Position.Height
	default:
		return 0, false
	}
	fact := c.Fact
	if fact == 0 {
		fact = 1
	}
	return v * fact, true
}

func (e *LayoutEngine) applyRules(rules []smartart.Rule, ctx *layoutContext) {
	for _, r := range rules {
		typ := strings.ToLower(strings.TrimSpace(r.Type))
		if typ != "w" && typ != "h" && typ != "woff" && typ != "hoff" && typ != "loff" && typ != "toff" && typ != "primfontsz" && typ != "secfontsz" {
			continue
		}
		value := r.Val
		if !math.IsNaN(r.Fact) && r.Fact != 0 {
			value *= r.Fact
		}
		if !math.IsNaN(r.Max) && r.Max > 0 && value > r.Max {
			value = r.Max
		}
		targets := filterPointType(e.constraintTargets(r.For, r.ForName, ctx), r.PtType)
		for _, n := range targets {
			if n == nil {
				continue
			}
			switch typ {
			case "w":
				n.Position.Width = value
			case "h":
				n.Position.Height = value
			case "woff":
				n.Position.Width = math.Max(0, n.Position.Width+value)
			case "hoff":
				n.Position.Height = math.Max(0, n.Position.Height+value)
			case "loff":
				n.Position.X += value
			case "toff":
				n.Position.Y += value
			case "primfontsz":
				applyTextSizeRule(n, 0, value)
			case "secfontsz":
				applyTextSizeRule(n, 1, value)
			}
			n.HasPosition = true
		}
	}
}

func (e *LayoutEngine) presentationOfNodes(p *smartart.PresentationOf, ctx *layoutContext) []*smartart.Node {
	if p == nil {
		return ctx.nodes
	}
	nodes := e.selectAxisNodes(p.Axis, p.PtType, ctx)
	if len(nodes) == 0 {
		return nil
	}
	start := p.Start
	if start <= 0 {
		start = 1
	}
	step := p.Step
	if step == 0 {
		step = 1
	}
	indexes := make([]int, 0, len(nodes))
	if step > 0 {
		for i := start - 1; i < len(nodes); i += step {
			indexes = append(indexes, i)
		}
	} else {
		i := start - 1
		if i < 0 || i >= len(nodes) {
			i = len(nodes) - 1
		}
		for ; i >= 0; i += step {
			indexes = append(indexes, i)
		}
	}
	if p.Cnt > 0 && len(indexes) > p.Cnt {
		indexes = indexes[:p.Cnt]
	}
	out := make([]*smartart.Node, 0, len(indexes))
	for _, idx := range indexes {
		out = append(out, nodes[idx])
	}
	return out
}

func (e *LayoutEngine) applyLayoutShape(shape *smartart.LayoutShape, ctx *layoutContext) {
	if shape == nil || ctx == nil {
		return
	}
	geometry := strings.TrimSpace(shape.Type)
	rotation := shape.Rot
	if math.Abs(rotation) > 360 {
		rotation /= 60000.0
	}
	for _, n := range ctx.nodes {
		if n == nil {
			continue
		}
		if geometry != "" && !strings.EqualFold(geometry, "none") && strings.TrimSpace(n.Geometry) == "" {
			n.Geometry = geometry
		}
		if rotation != 0 && n.RotationDeg == 0 {
			n.RotationDeg = rotation
		}
	}
}

func applyTextSizeRule(n *smartart.Node, minLevel int, points float64) {
	if n == nil || points <= 0 || math.IsNaN(points) {
		return
	}
	size := int(math.Round(points * 100))
	for _, p := range n.Paragraphs {
		if p == nil {
			continue
		}
		isPrimary := p.Level <= 0
		if minLevel == 0 && !isPrimary {
			continue
		}
		if minLevel > 0 && isPrimary {
			continue
		}
		for i := range p.Runs {
			p.Runs[i].FontSize = size
		}
	}
}

func (e *LayoutEngine) addLayoutDiagnostic(code, message string) {
	if e == nil || e.Diagram == nil {
		return
	}
	e.Diagram.Diagnostics.Add(diag.Diagnostic{
		Severity: diag.SeverityInfo,
		Code:     code,
		Message:  message,
	})
}

func applyNumericConstraint(current, value float64, op string) float64 {
	switch strings.ToLower(strings.TrimSpace(op)) {
	case "gte":
		return math.Max(current, value)
	case "lte":
		return math.Min(current, value)
	default:
		return value
	}
}
