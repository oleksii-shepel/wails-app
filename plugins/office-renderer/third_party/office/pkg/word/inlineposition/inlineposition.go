package inlineposition

import (
	"math"
	"strconv"
	"strings"

	resolve "office/pkg/word/ir"
)

const DefaultTabStopTwips = 720
const twipsPerPoint = 20.0

type TabStop struct {
	PositionPt float64
	Alignment  string
	Leader     string
}

func NextTabStop(stops []resolve.TabStop, currentXPt float64, defaultTabTwips int) TabStop {
	if defaultTabTwips <= 0 {
		defaultTabTwips = DefaultTabStopTwips
	}
	best := TabStop{PositionPt: math.Inf(1)}
	for _, stop := range stops {
		if stop.Pos <= 0 || stop.Algn == "bar" {
			continue
		}
		pos := float64(stop.Pos) / twipsPerPoint
		if pos > currentXPt+0.01 && pos < best.PositionPt {
			best = TabStop{PositionPt: pos, Alignment: stop.Algn, Leader: stop.Leader}
		}
	}
	if !math.IsInf(best.PositionPt, 1) {
		if best.Alignment == "" {
			best.Alignment = "left"
		}
		return best
	}
	step := float64(defaultTabTwips) / twipsPerPoint
	return TabStop{PositionPt: (math.Floor(currentXPt/step) + 1) * step, Alignment: "left"}
}

func AlignedTabAdvance(stop TabStop, currentXPt, fieldWidthPt, decimalPrefixPt float64) float64 {
	target := stop.PositionPt
	switch stop.Alignment {
	case "center":
		target -= fieldWidthPt / 2
	case "right":
		target -= fieldWidthPt
	case "decimal":
		target -= decimalPrefixPt
	}
	if target <= currentXPt {
		return 0
	}
	return target - currentXPt
}

type AdvanceOffsets struct {
	Left, Right, Up, Down float64
	AbsX, AbsY            float64
	HasAbsX, HasAbsY      bool
}

type AdvanceState struct {
	HorizontalPt float64
	VerticalPt   float64
	InlineXPt    float64
}

func (s *AdvanceState) Reset(inlineXPt float64) {
	s.HorizontalPt = 0
	s.VerticalPt = 0
	s.InlineXPt = inlineXPt
}
func (s *AdvanceState) Apply(o AdvanceOffsets) {
	if o.HasAbsX {
		s.HorizontalPt = o.AbsX - s.InlineXPt
		s.InlineXPt = o.AbsX
	} else {
		d := o.Right - o.Left
		s.HorizontalPt += d
		s.InlineXPt += d
	}
	if o.HasAbsY {
		s.VerticalPt = -o.AbsY
	} else {
		s.VerticalPt += o.Up - o.Down
	}
}

func ParseAdvance(instr string) AdvanceOffsets {
	var out AdvanceOffsets
	fields := strings.Fields(strings.TrimSpace(instr))
	if len(fields) == 0 || !strings.EqualFold(fields[0], "ADVANCE") {
		return out
	}
	for i := 1; i < len(fields); i++ {
		sw := fields[i]
		if !strings.HasPrefix(sw, `\`) || len(sw) < 2 || i+1 >= len(fields) {
			continue
		}
		v, err := strconv.ParseFloat(strings.Trim(fields[i+1], `"`), 64)
		if err != nil {
			continue
		}
		i++
		switch strings.ToUpper(strings.TrimPrefix(sw, `\`)) {
		case "L":
			out.Left = v
		case "R":
			out.Right = v
		case "U":
			out.Up = v
		case "D":
			out.Down = v
		case "X":
			out.HasAbsX = true
			out.AbsX = v
		case "Y":
			out.HasAbsY = true
			out.AbsY = v
		}
	}
	return out
}
