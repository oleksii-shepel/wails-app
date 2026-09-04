package edit

import "fmt"

// Comments returns worksheet comments keyed by normalized A1 reference.
func (s *Sheet) Comments() map[string]string {
	if s == nil || s.sheet == nil || len(s.sheet.Comments) == 0 {
		return nil
	}
	out := make(map[string]string, len(s.sheet.Comments))
	for ref, text := range s.sheet.Comments {
		out[ref] = text
	}
	return out
}

// Comment looks up a worksheet comment by A1-style cell reference.
func (s *Sheet) Comment(ref string) (string, bool) {
	if s == nil || s.sheet == nil {
		return "", false
	}
	normalized, err := normalizeCommentRef(ref)
	if err != nil {
		return "", false
	}
	text, ok := s.sheet.Comments[normalized]
	return text, ok
}

// SetComment stores or clears a worksheet comment by A1-style cell reference.
func (s *Sheet) SetComment(ref, text string) error {
	if s == nil || s.sheet == nil {
		return fmt.Errorf("sheet edit: nil sheet")
	}
	normalized, err := normalizeCommentRef(ref)
	if err != nil {
		return err
	}
	if s.sheet.Comments == nil {
		s.sheet.Comments = make(map[string]string)
	}
	if text == "" {
		delete(s.sheet.Comments, normalized)
		return nil
	}
	s.sheet.Comments[normalized] = text
	return nil
}

// RemoveComment removes a worksheet comment by A1-style cell reference.
func (s *Sheet) RemoveComment(ref string) bool {
	if s == nil || s.sheet == nil || len(s.sheet.Comments) == 0 {
		return false
	}
	normalized, err := normalizeCommentRef(ref)
	if err != nil {
		return false
	}
	if _, ok := s.sheet.Comments[normalized]; !ok {
		return false
	}
	delete(s.sheet.Comments, normalized)
	return true
}

// ClearComments removes all worksheet comments and returns the number removed.
func (s *Sheet) ClearComments() int {
	if s == nil || s.sheet == nil {
		return 0
	}
	removed := len(s.sheet.Comments)
	s.sheet.Comments = make(map[string]string)
	return removed
}

func normalizeCommentRef(ref string) (string, error) {
	row, col, err := parseCellRef(ref)
	if err != nil {
		return "", err
	}
	return cellRef(row, col), nil
}
