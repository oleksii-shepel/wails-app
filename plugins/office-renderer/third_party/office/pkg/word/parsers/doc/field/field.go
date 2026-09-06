package field

import "strings"

// Instruction is the format-neutral semantic projection of one binary Word
// complex-field instruction. Raw text is retained in Instr for lossless or
// unsupported field forms.
type Instruction struct {
	Kind   string
	Instr  string
	Arg    string
	Target string
	Anchor string
}

// ParseInstruction decodes field instruction forms shared by Word 6/95 and
// Word97+. Unknown instructions keep their normalized kind and raw instruction
// without inventing additional semantics.
func ParseInstruction(instr string) Instruction {
	trimmed := strings.TrimSpace(instr)
	if trimmed == "" {
		return Instruction{}
	}
	tokens := splitTokens(trimmed)
	if len(tokens) == 0 {
		return Instruction{Instr: trimmed}
	}
	kind := strings.ToUpper(tokens[0])
	out := Instruction{Kind: kind, Instr: trimmed}
	switch kind {
	case "HYPERLINK":
		if len(tokens) > 1 {
			out.Target = tokens[1]
		}
	case "REF", "PAGEREF":
		if len(tokens) > 1 {
			out.Anchor = tokens[1]
		}
	case "STYLEREF", "DOCPROPERTY", "DOCVARIABLE":
		if len(tokens) > 1 {
			out.Arg = tokens[1]
		}
	case "FILLIN":
		if len(tokens) > 2 {
			out.Arg = tokens[2]
		}
	}
	return out
}

func splitTokens(input string) []string {
	var out []string
	var b strings.Builder
	quoted := false
	escaped := false
	flush := func() {
		if b.Len() == 0 {
			return
		}
		out = append(out, b.String())
		b.Reset()
	}
	for _, r := range input {
		if escaped {
			b.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			if quoted {
				escaped = true
			} else {
				flush()
				b.WriteRune(r)
			}
			continue
		}
		if r == '"' {
			quoted = !quoted
			if !quoted {
				flush()
			}
			continue
		}
		if !quoted && (r == ' ' || r == '\t' || r == '\r' || r == '\n') {
			flush()
			continue
		}
		b.WriteRune(r)
	}
	flush()
	return out
}
