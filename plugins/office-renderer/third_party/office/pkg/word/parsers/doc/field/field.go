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
		out.Target, out.Anchor = parseHyperlink(tokens)
	case "REF":
		if len(tokens) > 1 {
			out.Anchor = tokens[1]
		}
	case "PAGEREF":
		if len(tokens) > 1 {
			out.Anchor = tokens[1]
		}
	case "STYLEREF", "DOCPROPERTY", "DOCVARIABLE":
		if len(tokens) > 1 {
			out.Arg = tokens[1]
		}
	case "FILLIN":
		// Word commonly stores a prompt followed by the default value.
		if len(tokens) > 2 {
			out.Arg = tokens[2]
		} else if len(tokens) > 1 {
			out.Arg = tokens[1]
		}
	case "ASK", "MERGEFIELD", "AUTOTEXT", "SET":
		if len(tokens) > 1 {
			out.Arg = tokens[1]
		}
	case "SEQ":
		// Keep the shared parser deliberately conservative. The mature document
		// materializer derives sequence identity/control from the raw instruction;
		// publishing the leading token as generic Arg would change the established
		// shared grammar contract.
	case "QUOTE", "EMBED", "LINK", "IF", "COMPARE":
		// These instructions have meaningful internal grammar of their own.
		// Preserve the normalized raw instruction as the semantic argument until
		// the dedicated resolver interprets it.
		out.Arg = trimmed
	}

	return out
}

func parseHyperlink(tokens []string) (target, anchor string) {
	for i := 1; i < len(tokens); i++ {
		token := tokens[i]
		switch strings.ToLower(token) {
		case `\l`:
			if i+1 < len(tokens) {
				anchor = tokens[i+1]
				i++
			}
		case `\m`, `\n`, `\o`, `\t`:
			// Switches with an argument. They do not change the navigation
			// target represented by this format-neutral instruction.
			if i+1 < len(tokens) {
				i++
			}
		default:
			if strings.HasPrefix(token, `\`) {
				continue
			}
			if target == "" {
				target = token
			}
		}
	}
	if anchor != "" {
		if target != "" {
			target += "#" + anchor
		} else {
			target = "#" + anchor
		}
	}
	return target, anchor
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
