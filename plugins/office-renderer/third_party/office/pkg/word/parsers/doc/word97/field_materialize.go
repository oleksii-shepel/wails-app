package word97

import (
	"strings"

	wordir "office/pkg/word/ir"
	docformat "office/pkg/word/parsers/doc/format"
)

// MaterializedField is retained as a source-compatible name for callers in
// the canonical Word97 package. The recursive field shape itself is owned by
// the version-neutral DOC format package.
type MaterializedField = docformat.MaterializedField[Run]

type materializedFieldFrame struct {
	instr     strings.Builder
	instrRuns []Run
	result    []Run
	parsed    Field
	inResult  bool
}

// MaterializeFields is the Word97 compatibility-semantic field pass. It owns
// complex-field nesting and FFData before Word97 runs cross into legacy builder
// types. Genuine pre-97 keeps its independent compatibility materializer.
func MaterializeFields(runs []Run, dataStream []byte) []Run {
	const begin = '\x13'
	const sep = '\x14'
	const end = '\x15'
	out := make([]Run, 0, len(runs))
	stack := make([]materializedFieldFrame, 0, 4)
	inResult := func() bool { return len(stack) > 0 && stack[len(stack)-1].inResult }
	inField := func() bool { return len(stack) > 0 }
	emitText := func(text string, r Run) {
		if text == "" {
			return
		}
		nr := r
		nr.Text = text
		nr.Fields = cloneMaterializedFields(r.Fields)
		if inResult() {
			top := &stack[len(stack)-1]
			applyMaterializedFieldMetadata(&nr, top.parsed)
			top.result = append(top.result, nr)
			return
		}
		if inField() {
			top := &stack[len(stack)-1]
			top.instr.WriteString(text)
			top.instrRuns = append(top.instrRuns, nr)
			return
		}
		out = append(out, nr)
	}
	flush := func() *MaterializedField {
		top := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		return &MaterializedField{Kind: top.parsed.Kind, Instr: top.parsed.Instr, Arg: top.parsed.Arg, Anchor: top.parsed.Anchor, Target: top.parsed.Target, Children: top.result, InstrRuns: top.instrRuns}
	}
	var handle func(*MaterializedField)
	handle = func(f *MaterializedField) {
		if len(stack) > 0 {
			parent := &stack[len(stack)-1]
			if parent.inResult {
				rr := f.Children
				if len(rr) == 0 {
					rr = []Run{{}}
				}
				applyMaterializedFieldMetadata(&rr[0], Field{Kind: f.Kind, Instr: f.Instr, Arg: f.Arg, Anchor: f.Anchor, Target: f.Target})
				rr[0].Fields = append(rr[0].Fields, f)
				parent.result = append(parent.result, rr...)
				return
			}
			display := joinCanonicalRunText(f.Children)
			parent.instr.WriteString(display)
			parent.instrRuns = append(parent.instrRuns, Run{Text: display, Fields: []*MaterializedField{f}})
			return
		}
		if len(f.Children) > 0 {
			applyMaterializedFieldMetadata(&f.Children[0], Field{Kind: f.Kind, Instr: f.Instr, Arg: f.Arg, Anchor: f.Anchor, Target: f.Target})
		}
		if isFormFieldKind(f.Kind) {
			out = append(out, materializeFormField(f, dataStream))
			return
		}
		if len(f.Children) > 0 {
			out = append(out, f.Children...)
			return
		}
		out = append(out, Run{FieldKind: f.Kind, FieldInstr: f.Instr, FieldArg: f.Arg, Anchor: f.Anchor, Fields: []*MaterializedField{f}})
	}
	for _, r := range runs {
		if r.Kind != "" {
			if inResult() {
				top := &stack[len(stack)-1]
				applyMaterializedFieldMetadata(&r, top.parsed)
				top.result = append(top.result, r)
			} else if inField() {
				stack[len(stack)-1].instrRuns = append(stack[len(stack)-1].instrRuns, r)
			} else {
				out = append(out, r)
			}
			continue
		}
		if !strings.ContainsAny(r.Text, "\x13\x14\x15") {
			if inField() && !inResult() {
				top := &stack[len(stack)-1]
				top.instr.WriteString(r.Text)
				top.instrRuns = append(top.instrRuns, r)
			} else if inResult() {
				emitText(r.Text, r)
			} else {
				out = append(out, r)
			}
			continue
		}
		var b strings.Builder
		cp := r.StartCP
		flushSeg := func() {
			text := b.String()
			if text != "" {
				seg := r
				seg.StartCP = cp - len([]rune(text))
				emitText(text, seg)
			}
			b.Reset()
		}
		for _, ch := range r.Text {
			switch ch {
			case begin:
				flushSeg()
				stack = append(stack, materializedFieldFrame{})
			case sep:
				flushSeg()
				if inField() {
					top := &stack[len(stack)-1]
					instr := strings.TrimSpace(top.instr.String())
					top.parsed = parseMaterializedFieldInstruction(instr)
					top.inResult = true
				}
			case end:
				flushSeg()
				if inField() {
					handle(flush())
				}
			default:
				b.WriteRune(ch)
			}
			cp++
		}
		flushSeg()
	}
	for len(stack) > 0 {
		handle(flush())
	}
	return out
}

func parseMaterializedFieldInstruction(instr string) Field {
	f := ParseFieldInstruction(instr)
	f.Instr = strings.TrimSpace(instr)
	if f.Kind == "SEQ" && f.Arg == "" {
		f.Arg = fieldLeadingArg(instr, "SEQ")
	}
	switch f.Kind {
	case "TOC", "INDEX", "XE", "TC":
		f.Arg = strings.TrimSpace(instr)
	}
	return f
}
func fieldLeadingArg(instr, kind string) string {
	rest := strings.TrimSpace(instr[len(kind):])
	if rest == "" {
		return ""
	}
	if rest[0] == '"' {
		if i := strings.IndexByte(rest[1:], '"'); i >= 0 {
			return rest[1 : i+1]
		}
	}
	if i := strings.IndexAny(rest, " \t\\"); i >= 0 {
		return rest[:i]
	}
	return rest
}
func applyMaterializedFieldMetadata(r *Run, f Field) {
	if f.Target != "" {
		r.Target = f.Target
	}
	if f.Instr != "" {
		r.FieldInstr = f.Instr
	}
	if f.Kind != "" {
		r.FieldKind = f.Kind
		r.FieldArg = f.Arg
		r.Anchor = f.Anchor
	}
	if f.Kind == "HYPERLINK" && f.Target != "" {
		r.FieldKind = ""
		r.FieldArg = ""
	}
}
func joinCanonicalRunText(r []Run) string {
	var b strings.Builder
	for _, x := range r {
		b.WriteString(x.Text)
	}
	return b.String()
}
func cloneMaterializedFields(src []*MaterializedField) []*MaterializedField {
	if len(src) == 0 {
		return nil
	}
	return append([]*MaterializedField(nil), src...)
}
func isFormFieldKind(k string) bool {
	return k == "FORMTEXT" || k == "FORMCHECKBOX" || k == "FORMDROPDOWN"
}
func materializeFormField(f *MaterializedField, data []byte) Run {
	base := Run{}
	if len(f.Children) > 0 {
		base = f.Children[0]
	}
	ff := &wordir.FormField{}
	display := joinCanonicalRunText(f.Children)
	if data != nil && base.Format.PicLocation > 0 {
		if decoded := readFFData(data, base.Format.PicLocation); decoded != nil {
			ff = decoded
			if ff.Type == "text" && ff.DefaultText == "" {
				ff.DefaultText = display
			}
			if ff.Type == "dropdown" && len(ff.Items) == 0 && strings.TrimSpace(display) != "" {
				ff.Items = []string{display}
				ff.DefaultIdx = 0
			}
			goto done
		}
	}
	switch f.Kind {
	case "FORMTEXT":
		ff.Type = "text"
		ff.DefaultText = display
	case "FORMCHECKBOX":
		ff.Type = "checkbox"
		ff.Checked = formCheckboxChecked(display)
	case "FORMDROPDOWN":
		ff.Type = "dropdown"
		if strings.TrimSpace(display) != "" {
			ff.Items = []string{display}
			ff.DefaultIdx = 0
		}
	}
done:
	base.Text = ""
	base.Kind = "formField"
	base.FormField = ff
	base.FieldKind = f.Kind
	base.FieldInstr = f.Instr
	base.FieldArg = f.Arg
	base.Anchor = f.Anchor
	base.Target = f.Target
	base.Image = nil
	base.Shape = nil
	return base
}
func formCheckboxChecked(s string) bool {
	switch strings.TrimSpace(strings.ToUpper(s)) {
	case "1", "TRUE", "X", "YES", "ON", "☑", "☒":
		return true
	}
	return false
}
