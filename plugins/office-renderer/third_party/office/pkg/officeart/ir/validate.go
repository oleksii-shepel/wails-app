package ir

import (
	"fmt"
	"strings"
)

func (s *Shape) Validate() error {
	if s == nil {
		return fmt.Errorf("officeart: nil shape")
	}
	if s.Transform.Rect.W < 0 || s.Transform.Rect.H < 0 {
		return fmt.Errorf("officeart: shape %d has negative bounds", s.ID)
	}
	if err := validateColor("fill", s.Fill.Color); err != nil {
		return fmt.Errorf("officeart: shape %d: %w", s.ID, err)
	}
	if err := validateColor("line", s.Line.Color); err != nil {
		return fmt.Errorf("officeart: shape %d: %w", s.ID, err)
	}
	for _, child := range s.Children {
		if err := child.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func validateColor(name, value string) error {
	if value == "" {
		return nil
	}
	v := strings.TrimPrefix(value, "#")
	if len(v) != 6 && len(v) != 8 {
		return fmt.Errorf("%s color %q must be RRGGBB or AARRGGBB", name, value)
	}
	for _, r := range v {
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return fmt.Errorf("%s color %q is not hexadecimal", name, value)
		}
	}
	return nil
}
