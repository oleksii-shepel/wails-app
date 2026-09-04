package edit

import (
	"fmt"
	"strconv"
	"strings"

	sheetir "office/pkg/sheet/ir"
)

// VMLObjects returns generic worksheet VML objects in storage order.
func (s *Sheet) VMLObjects() []*sheetir.VMLObject {
	if s == nil || s.sheet == nil || len(s.sheet.VMLObjects) == 0 {
		return nil
	}
	out := make([]*sheetir.VMLObject, 0, len(s.sheet.VMLObjects))
	for _, obj := range s.sheet.VMLObjects {
		if obj == nil {
			continue
		}
		clone := *obj
		out = append(out, &clone)
	}
	return out
}

// VMLObject looks up a generic worksheet VML object by id.
func (s *Sheet) VMLObject(id string) (*sheetir.VMLObject, bool) {
	if s == nil || s.sheet == nil {
		return nil, false
	}
	id = strings.TrimSpace(id)
	for _, obj := range s.sheet.VMLObjects {
		if obj != nil && strings.EqualFold(strings.TrimSpace(obj.ID), id) {
			clone := *obj
			return &clone, true
		}
	}
	return nil, false
}

// AddVMLObject appends a generic VML-backed worksheet shape.
func (s *Sheet) AddVMLObject(id, shapeType, style string) (*sheetir.VMLObject, error) {
	if s == nil || s.sheet == nil {
		return nil, fmt.Errorf("sheet edit: nil sheet")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		id = s.nextVMLObjectID()
	}
	if _, ok := s.VMLObject(id); ok {
		return nil, fmt.Errorf("sheet edit: VML object %q already exists", id)
	}
	obj := &sheetir.VMLObject{ID: id, Type: strings.TrimSpace(shapeType), Style: strings.TrimSpace(style)}
	s.sheet.VMLObjects = append(s.sheet.VMLObjects, obj)
	s.markFormControlsDirty()
	return obj, nil
}

// SetVMLObject replaces editable properties of a generic VML object matched by id.
func (s *Sheet) SetVMLObject(obj *sheetir.VMLObject) error {
	if s == nil || s.sheet == nil {
		return fmt.Errorf("sheet edit: nil sheet")
	}
	if obj == nil {
		return fmt.Errorf("sheet edit: nil VML object")
	}
	id := strings.TrimSpace(obj.ID)
	if id == "" {
		return fmt.Errorf("sheet edit: VML object id is empty")
	}
	for _, existing := range s.sheet.VMLObjects {
		if existing == nil || !strings.EqualFold(strings.TrimSpace(existing.ID), id) {
			continue
		}
		existing.ID = id
		existing.Type = strings.TrimSpace(obj.Type)
		existing.Style = strings.TrimSpace(obj.Style)
		existing.FillColor = strings.TrimSpace(obj.FillColor)
		existing.StrokeColor = strings.TrimSpace(obj.StrokeColor)
		existing.Text = obj.Text
		existing.Anchor = strings.TrimSpace(obj.Anchor)
		s.markFormControlsDirty()
		return nil
	}
	return fmt.Errorf("sheet edit: VML object %q not found", id)
}

// RemoveVMLObject removes a generic VML object by id.
func (s *Sheet) RemoveVMLObject(id string) bool {
	if s == nil || s.sheet == nil {
		return false
	}
	id = strings.TrimSpace(id)
	for i, obj := range s.sheet.VMLObjects {
		if obj != nil && strings.EqualFold(strings.TrimSpace(obj.ID), id) {
			s.sheet.VMLObjects = append(s.sheet.VMLObjects[:i], s.sheet.VMLObjects[i+1:]...)
			s.markFormControlsDirty()
			return true
		}
	}
	return false
}

// ClearVMLObjects removes all generic VML objects without touching controls/comments.
func (s *Sheet) ClearVMLObjects() int {
	if s == nil || s.sheet == nil {
		return 0
	}
	n := len(s.sheet.VMLObjects)
	if n == 0 {
		return 0
	}
	s.sheet.VMLObjects = nil
	s.markFormControlsDirty()
	return n
}

func (s *Sheet) nextVMLObjectID() string {
	for i := 1025; ; i++ {
		id := fmt.Sprintf("_x0000_s%d", i)
		if _, exists := s.VMLObject(id); !exists {
			return id
		}
	}
}

// FormControls returns worksheet VML-based form controls in storage order.
func (s *Sheet) FormControls() []*sheetir.FormControl {
	if s == nil || s.sheet == nil || len(s.sheet.FormControls) == 0 {
		return nil
	}
	out := make([]*sheetir.FormControl, 0, len(s.sheet.FormControls))
	for _, control := range s.sheet.FormControls {
		if control == nil {
			continue
		}
		clone := *control
		clone.Selected = append([]int(nil), control.Selected...)
		out = append(out, &clone)
	}
	return out
}

// FormControl looks up a worksheet VML form control by alt text.
func (s *Sheet) FormControl(altText string) (*sheetir.FormControl, bool) {
	if s == nil || s.sheet == nil {
		return nil, false
	}
	altText = strings.TrimSpace(altText)
	for _, control := range s.sheet.FormControls {
		if control != nil && strings.EqualFold(strings.TrimSpace(control.AltText), altText) {
			clone := *control
			clone.Selected = append([]int(nil), control.Selected...)
			return &clone, true
		}
	}
	return nil, false
}

// AddFormControl appends a VML-based worksheet form control.
func (s *Sheet) AddFormControl(controlType, altText, anchor string) (*sheetir.FormControl, error) {
	if s == nil || s.sheet == nil {
		return nil, fmt.Errorf("sheet edit: nil sheet")
	}
	controlType, err := normalizeFormControlType(controlType)
	if err != nil {
		return nil, err
	}
	anchor, err = normalizeFormControlAnchor(anchor)
	if err != nil {
		return nil, err
	}
	altText = strings.TrimSpace(altText)
	if altText == "" {
		altText = s.nextFormControlName(controlType)
	}
	control := &sheetir.FormControl{
		Type:    controlType,
		AltText: altText,
		Anchor:  anchor,
	}
	s.sheet.FormControls = append(s.sheet.FormControls, control)
	s.markFormControlsDirty()
	return control, nil
}

// SetFormControl replaces a worksheet VML form control matched by alt text.
func (s *Sheet) SetFormControl(control *sheetir.FormControl) error {
	if s == nil || s.sheet == nil {
		return fmt.Errorf("sheet edit: nil sheet")
	}
	if control == nil {
		return fmt.Errorf("sheet edit: nil form control")
	}
	controlType, err := normalizeFormControlType(control.Type)
	if err != nil {
		return err
	}
	anchor, err := normalizeFormControlAnchor(control.Anchor)
	if err != nil {
		return err
	}
	altText := strings.TrimSpace(control.AltText)
	if altText == "" {
		return fmt.Errorf("sheet edit: form control alt text is empty")
	}
	for _, existing := range s.sheet.FormControls {
		if existing == nil || !strings.EqualFold(strings.TrimSpace(existing.AltText), altText) {
			continue
		}
		existing.Type = controlType
		existing.AltText = altText
		existing.Anchor = anchor
		existing.Checked = control.Checked
		existing.Mixed = control.Mixed && controlType == "Checkbox"
		existing.FirstButton = control.FirstButton && controlType == "Radio"
		existing.FmlaLink = strings.TrimSpace(control.FmlaLink)
		existing.FmlaGroup = strings.TrimSpace(control.FmlaGroup)
		existing.FmlaRange = strings.TrimSpace(control.FmlaRange)
		existing.SelectionType = strings.TrimSpace(strings.ToLower(control.SelectionType))
		existing.Selected = append([]int(nil), control.Selected...)
		existing.Min = control.Min
		existing.Max = control.Max
		existing.Val = control.Val
		existing.Increment = control.Increment
		existing.PageStep = control.PageStep
		existing.Horizontal = control.Horizontal && controlType == "Scroll"
		existing.DropLines = control.DropLines
		s.markFormControlsDirty()
		return nil
	}
	return fmt.Errorf("sheet edit: form control %q not found", altText)
}

// SetFormControlValue updates a Spin/Scroll control value with range validation.
func (s *Sheet) SetFormControlValue(altText string, value int) error {
	control, ok := s.FormControl(altText)
	if !ok {
		return fmt.Errorf("sheet edit: form control %q not found", altText)
	}
	if control.Type != "Spin" && control.Type != "Scroll" {
		return fmt.Errorf("sheet edit: form control %q is not scrollable", altText)
	}
	if control.Max >= control.Min && (value < control.Min || value > control.Max) {
		return fmt.Errorf("sheet edit: form control value %d outside [%d,%d]", value, control.Min, control.Max)
	}
	control.Val = value
	return s.SetFormControl(control)
}

// SetFormControlSelection updates List/Drop selected item indexes.
func (s *Sheet) SetFormControlSelection(altText string, selected ...int) error {
	control, ok := s.FormControl(altText)
	if !ok {
		return fmt.Errorf("sheet edit: form control %q not found", altText)
	}
	if control.Type != "List" && control.Type != "Drop" {
		return fmt.Errorf("sheet edit: form control %q is not a list control", altText)
	}
	for _, idx := range selected {
		if idx < 1 {
			return fmt.Errorf("sheet edit: selected item indexes must be >= 1")
		}
	}
	if control.Type == "Drop" || control.SelectionType == "single" || control.SelectionType == "" {
		if len(selected) > 1 {
			return fmt.Errorf("sheet edit: single-select control accepts at most one selected item")
		}
	}
	control.Selected = append([]int(nil), selected...)
	control.Val = 0
	if len(selected) > 0 {
		control.Val = selected[0]
	}
	return s.SetFormControl(control)
}

// SetFormControlChecked updates a Checkbox or Radio state. Checking a radio
// button also clears the other buttons in the same linked/FirstButton group.
func (s *Sheet) SetFormControlChecked(altText string, checked bool) error {
	control, ok := s.FormControl(altText)
	if !ok {
		return fmt.Errorf("sheet edit: form control %q not found", altText)
	}
	if control.Type != "Checkbox" && control.Type != "Radio" {
		return fmt.Errorf("sheet edit: form control %q is not checkable", altText)
	}
	if control.Type == "Checkbox" {
		control.Checked = checked
		if checked {
			control.Mixed = false
		}
		return s.SetFormControl(control)
	}

	if !checked {
		control.Checked = false
		return s.SetFormControl(control)
	}

	return s.selectRadioButton(altText)
}

// SelectRadioButton selects one radio button and clears its group peers.
func (s *Sheet) SelectRadioButton(altText string) error {
	return s.selectRadioButton(altText)
}

func (s *Sheet) selectRadioButton(altText string) error {
	if s == nil || s.sheet == nil {
		return fmt.Errorf("sheet edit: nil sheet")
	}
	target := -1
	for i, control := range s.sheet.FormControls {
		if control != nil && strings.EqualFold(strings.TrimSpace(control.AltText), strings.TrimSpace(altText)) {
			target = i
			break
		}
	}
	if target < 0 {
		return fmt.Errorf("sheet edit: form control %q not found", altText)
	}
	selected := s.sheet.FormControls[target]
	if selected == nil || selected.Type != "Radio" {
		return fmt.Errorf("sheet edit: form control %q is not a radio button", altText)
	}

	link := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(selected.FmlaLink), "$", ""))
	if link != "" {
		for _, control := range s.sheet.FormControls {
			if control == nil || control.Type != "Radio" {
				continue
			}
			peerLink := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(control.FmlaLink), "$", ""))
			if peerLink == link {
				control.Checked = control == selected
			}
		}
	} else {
		start := target
		for start > 0 {
			prev := s.sheet.FormControls[start-1]
			if prev == nil || prev.Type != "Radio" || selected.FirstButton {
				break
			}
			start--
			if s.sheet.FormControls[start].FirstButton {
				break
			}
		}
		end := target + 1
		for end < len(s.sheet.FormControls) {
			next := s.sheet.FormControls[end]
			if next == nil || next.Type != "Radio" || next.FirstButton {
				break
			}
			end++
		}
		for i := start; i < end; i++ {
			if control := s.sheet.FormControls[i]; control != nil && control.Type == "Radio" {
				control.Checked = i == target
			}
		}
	}
	s.markFormControlsDirty()
	return nil
}

// RemoveFormControl removes a worksheet VML form control by alt text.
func (s *Sheet) RemoveFormControl(altText string) bool {
	if s == nil || s.sheet == nil {
		return false
	}
	altText = strings.TrimSpace(altText)
	for i, control := range s.sheet.FormControls {
		if control != nil && strings.EqualFold(strings.TrimSpace(control.AltText), altText) {
			s.sheet.FormControls = append(s.sheet.FormControls[:i], s.sheet.FormControls[i+1:]...)
			s.markFormControlsDirty()
			return true
		}
	}
	return false
}

// ClearFormControls removes all worksheet VML form controls.
func (s *Sheet) ClearFormControls() int {
	if s == nil || s.sheet == nil {
		return 0
	}
	removed := len(s.sheet.FormControls)
	if removed == 0 {
		return 0
	}
	s.sheet.FormControls = nil
	s.markFormControlsDirty()
	return removed
}

func (s *Sheet) nextFormControlName(controlType string) string {
	for i := 1; ; i++ {
		name := fmt.Sprintf("%s %d", controlType, i)
		found := false
		for _, control := range s.sheet.FormControls {
			if control != nil && strings.EqualFold(strings.TrimSpace(control.AltText), name) {
				found = true
				break
			}
		}
		if !found {
			return name
		}
	}
}

func (s *Sheet) markFormControlsDirty() {
	if s == nil || s.sheet == nil || s.wb == nil {
		return
	}
	s.wb.markVMLDirty(s.sheet.ID)
}

func normalizeFormControlType(controlType string) (string, error) {
	switch strings.TrimSpace(controlType) {
	case "Button", "Checkbox", "Radio", "List", "Drop", "Spin", "Scroll", "GBox", "Label", "Edit":
		return strings.TrimSpace(controlType), nil
	default:
		return "", fmt.Errorf("sheet edit: unsupported form control type %q", controlType)
	}
}

func normalizeFormControlAnchor(anchor string) (string, error) {
	parts := strings.Split(anchor, ",")
	if len(parts) != 8 {
		return "", fmt.Errorf("sheet edit: form control anchor must have 8 comma-separated integers")
	}
	normalized := make([]string, 0, 8)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if _, err := strconv.Atoi(part); err != nil {
			return "", fmt.Errorf("sheet edit: invalid form control anchor component %q", part)
		}
		normalized = append(normalized, part)
	}
	return strings.Join(normalized, ", "), nil
}
