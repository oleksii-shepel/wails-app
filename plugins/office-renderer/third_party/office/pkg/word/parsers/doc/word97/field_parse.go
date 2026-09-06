package word97

import docfield "office/pkg/word/parsers/doc/field"

// ParseFieldInstruction decodes binary Word field instruction forms through
// the shared DOC grammar used by both Word97+ and pre-97 readers.
func ParseFieldInstruction(instr string) Field {
	return docfield.ParseInstruction(instr)
}
