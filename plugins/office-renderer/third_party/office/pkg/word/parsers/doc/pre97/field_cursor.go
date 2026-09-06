package pre97

import (
	"strings"

	docfield "office/pkg/word/parsers/doc/field"
)

type fieldFrame struct {
	instr    strings.Builder
	field    docfield.Instruction
	inResult bool
}

type fieldCursor struct {
	inner Cursor
	stack []fieldFrame
	queue []Run
}

func NewFieldCursor(inner Cursor) Cursor {
	if inner == nil { return nil }
	return &fieldCursor{inner: inner}
}

func (c *fieldCursor) Next() (Run, bool, error) {
	if c == nil { return Run{}, false, nil }
	for {
		if len(c.queue) > 0 {
			r := c.queue[0]; c.queue = c.queue[1:]; return r, true, nil
		}
		run, ok, err := c.inner.Next()
		if err != nil || !ok { return run, ok, err }
		c.consume(run)
	}
}

func (c *fieldCursor) consume(run Run) {
	if run.Kind != "" || !strings.ContainsAny(run.Text, "\x13\x14\x15") {
		c.consumeText(run); return
	}
	text, cp := run.Text, run.StartCP
	for len(text) > 0 {
		idx := strings.IndexAny(text, "\x13\x14\x15")
		if idx < 0 {
			part := run; part.Text = text; part.StartCP = cp; c.consumeText(part); return
		}
		if idx > 0 {
			part := run; part.Text = text[:idx]; part.StartCP = cp; c.consumeText(part)
			cp += len([]rune(text[:idx]))
		}
		switch text[idx] {
		case 0x13:
			c.stack = append(c.stack, fieldFrame{})
		case 0x14:
			if len(c.stack) > 0 {
				top := &c.stack[len(c.stack)-1]
				instr := strings.TrimSpace(top.instr.String())
				top.field = docfield.ParseInstruction(instr)
				top.inResult = true
			}
		case 0x15:
			if len(c.stack) > 0 { c.stack = c.stack[:len(c.stack)-1] }
		}
		cp++
		text = text[idx+1:]
	}
}

func (c *fieldCursor) consumeText(run Run) {
	if run.Text == "" && run.Kind == "" { return }
	if len(c.stack) == 0 { c.queue = append(c.queue, run); return }
	top := &c.stack[len(c.stack)-1]
	if !top.inResult { top.instr.WriteString(run.Text); return }
	run.FieldKind, run.FieldInstr, run.FieldArg = top.field.Kind, top.field.Instr, top.field.Arg
	run.Target, run.Anchor = top.field.Target, top.field.Anchor
	c.queue = append(c.queue, run)
}
