package layout

import (
	"strings"

	resolve "office/pkg/word/ir"
)

// paragraphFlowRole describes what a paragraph contributes to text flow.
// Drawing-host paragraphs retain an authoritative paragraph Y for drawing
// placement without manufacturing an ordinary empty text line.
type paragraphFlowRole uint8

const (
	paragraphFlowRoleText paragraphFlowRole = iota
	paragraphFlowRoleEmpty
	paragraphFlowRoleDrawingHost
	paragraphFlowRoleMixed
)

func resolveParagraphFlowRole(p *resolve.Paragraph) paragraphFlowRole {
	if p == nil {
		return paragraphFlowRoleEmpty
	}

	hasText := false
	hasDrawing := false
	for i := range p.Runs {
		run := &p.Runs[i]
		if drawingFromRun(run) != nil {
			hasDrawing = true
		}
		if strings.TrimSpace(run.Text) != "" || run.Kind == "hyperlink" || run.Kind == "break" {
			hasText = true
		}
	}

	switch {
	case hasDrawing && !hasText:
		return paragraphFlowRoleDrawingHost
	case hasDrawing && hasText:
		return paragraphFlowRoleMixed
	case hasText:
		return paragraphFlowRoleText
	default:
		return paragraphFlowRoleEmpty
	}
}

func paragraphIsDrawingHost(p *resolve.Paragraph) bool {
	return resolveParagraphFlowRole(p) == paragraphFlowRoleDrawingHost
}
