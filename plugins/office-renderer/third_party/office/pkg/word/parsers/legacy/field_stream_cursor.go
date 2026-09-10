package legacy

import "strings"

// legacyFieldRunCursor consumes DOC complex-field control characters
// (0x13 begin, 0x14 separator, 0x15 end) incrementally. Instruction text is
// withheld from visible output; cached-result runs are annotated with the same
// semantic field metadata used by the mature parser.
type legacyFieldRunCursor struct {
	inner legacyRunCursor
	stack []legacyStreamFieldFrame
	queue []binaryWordRun
}

type legacyStreamFieldFrame struct {
	instr    strings.Builder
	parsed   legacyDocField
	inResult bool
}

func newLegacyFieldRunCursor(inner legacyRunCursor) legacyRunCursor {
	if inner == nil {
		return nil
	}
	return &legacyFieldRunCursor{inner: inner}
}

func (c *legacyFieldRunCursor) Next() (binaryWordRun, bool, error) {
	if c == nil {
		return binaryWordRun{}, false, nil
	}
	for {
		if len(c.queue) > 0 {
			run := c.queue[0]
			c.queue = c.queue[1:]
			return run, true, nil
		}
		run, ok, err := c.inner.Next()
		if err != nil || !ok {
			return run, ok, err
		}
		c.consume(run)
	}
}

func (c *legacyFieldRunCursor) consume(run binaryWordRun) {
	if run.Kind != "" || !strings.ContainsAny(run.Text, "\x13\x14\x15") {
		c.consumeTextRun(run)
		return
	}
	text := run.Text
	cp := run.StartCP
	for len(text) > 0 {
		idx := strings.IndexAny(text, "\x13\x14\x15")
		if idx < 0 {
			part := run
			part.Text = text
			part.StartCP = cp
			c.consumeTextRun(part)
			return
		}
		if idx > 0 {
			part := run
			part.Text = text[:idx]
			part.StartCP = cp
			c.consumeTextRun(part)
			cp += len([]rune(text[:idx]))
		}
		switch text[idx] {
		case 0x13:
			c.stack = append(c.stack, legacyStreamFieldFrame{})
		case 0x14:
			if len(c.stack) > 0 {
				top := &c.stack[len(c.stack)-1]
				instr := strings.TrimSpace(top.instr.String())
				top.parsed = parseDocFieldInstruction(instr)
				top.parsed.Instr = instr
				// The mature parser historically represents HYPERLINK primarily by
				// Target, but the streaming IR also carries a field kind. Preserve it
				// explicitly so field semantics are not lost at the cursor boundary.
				if top.parsed.Kind == "" && strings.HasPrefix(strings.ToUpper(instr), "HYPERLINK") {
					top.parsed.Kind = "HYPERLINK"
				}
				top.inResult = true
			}
		case 0x15:
			if len(c.stack) > 0 {
				c.stack = c.stack[:len(c.stack)-1]
			}
		}
		cp++
		text = text[idx+1:]
	}
}

func (c *legacyFieldRunCursor) consumeTextRun(run binaryWordRun) {
	if run.Text == "" && run.Kind == "" {
		return
	}
	if len(c.stack) == 0 {
		c.queue = append(c.queue, run)
		return
	}
	top := &c.stack[len(c.stack)-1]
	if !top.inResult {
		top.instr.WriteString(run.Text)
		return
	}
	run.FieldKind = top.parsed.Kind
	run.FieldInstr = top.parsed.Instr
	run.FieldArg = top.parsed.Arg
	run.Anchor = top.parsed.Anchor
	run.Target = top.parsed.Target
	if run.Target == "" && top.parsed.Kind == "HYPERLINK" {
		run.Target = parseHyperlinkFieldInstr(top.parsed.Instr)
	}
	c.queue = append(c.queue, run)
}
