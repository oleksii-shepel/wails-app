package word97

import (
	"strings"

	docfield "office/pkg/word/parsers/doc/field"
)

// Field is the shared binary Word field-instruction semantic projection.
type Field = docfield.Instruction

type FieldParser func(string) Field

type fieldFrame struct {
	instr    strings.Builder
	parsed   Field
	inResult bool
}

type fieldCursor struct {
	inner Cursor
	parse FieldParser
	stack []fieldFrame
	queue []Run
}

// NewFieldCursor consumes Word complex-field control characters (0x13 begin,
// 0x14 separator, 0x15 end) incrementally. Instruction text is suppressed and
// visible cached-result runs receive the parsed field metadata.
func NewFieldCursor(inner Cursor, parse FieldParser) Cursor {
	if inner == nil { return nil }
	if parse == nil { parse = func(instr string) Field { return Field{Instr: strings.TrimSpace(instr)} } }
	return &fieldCursor{inner: inner, parse: parse}
}

func (c *fieldCursor) Next() (Run, bool, error) {
	if c == nil { return Run{}, false, nil }
	for {
		if len(c.queue) > 0 { run:=c.queue[0]; c.queue=c.queue[1:]; return run,true,nil }
		run,ok,err:=c.inner.Next(); if err!=nil||!ok{return run,ok,err}
		c.consume(run)
	}
}

func (c *fieldCursor) consume(run Run) {
	if run.Kind != "" || !strings.ContainsAny(run.Text, "\x13\x14\x15") { c.consumeTextRun(run); return }
	text:=run.Text
	cp:=run.StartCP
	for len(text)>0 {
		idx:=strings.IndexAny(text,"\x13\x14\x15")
		if idx<0 { part:=run; part.Text=text; part.StartCP=cp; c.consumeTextRun(part); return }
		if idx>0 { part:=run; part.Text=text[:idx]; part.StartCP=cp; c.consumeTextRun(part); cp+=len([]rune(text[:idx])) }
		switch text[idx] {
		case 0x13:
			c.stack=append(c.stack,fieldFrame{})
		case 0x14:
			if len(c.stack)>0 { top:=&c.stack[len(c.stack)-1]; instr:=strings.TrimSpace(top.instr.String()); top.parsed=c.parse(instr); if top.parsed.Instr==""{top.parsed.Instr=instr}; top.inResult=true }
		case 0x15:
			if len(c.stack)>0 { c.stack=c.stack[:len(c.stack)-1] }
		}
		cp++
		text=text[idx+1:]
	}
}

func (c *fieldCursor) consumeTextRun(run Run) {
	if run.Text=="" && run.Kind=="" { return }
	if len(c.stack)==0 { c.queue=append(c.queue,run); return }
	top:=&c.stack[len(c.stack)-1]
	if !top.inResult { top.instr.WriteString(run.Text); return }
	run.FieldKind=top.parsed.Kind
	run.FieldInstr=top.parsed.Instr
	run.FieldArg=top.parsed.Arg
	run.Target=top.parsed.Target
	run.Anchor=top.parsed.Anchor
	c.queue=append(c.queue,run)
}
