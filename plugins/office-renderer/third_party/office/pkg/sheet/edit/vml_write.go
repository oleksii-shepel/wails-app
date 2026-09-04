package edit

import (
	sheetir "office/pkg/sheet/ir"
	sheetparts "office/pkg/sheet/parsers/ooxml/parts"
)

func (w *Workbook) applyLegacyDrawingVML(pkg *sheetparts.Package) {
	if w == nil || pkg == nil {
		return
	}
	for i, entry := range pkg.Worksheets {
		if entry == nil || i >= len(w.ir.Sheets) {
			continue
		}
		sheet := w.ir.Sheets[i]
		if sheet == nil {
			continue
		}
		raw := w.legacyDrawingVML[sheet.ID]
		if !w.vmlDirty[sheet.ID] {
			if len(raw) > 0 {
				entry.LegacyDrawingVML = append([]byte(nil), raw...)
			} else if len(sheet.FormControls) > 0 || len(sheet.VMLObjects) > 0 {
				entry.LegacyDrawingVML = mergeLegacyDrawingVML(nil, sheet.FormControls, sheet.VMLObjects)
			}
			continue
		}
		entry.LegacyDrawingVML = mergeLegacyDrawingVML(raw, sheet.FormControls, sheet.VMLObjects)
	}
}

func mergeLegacyDrawingVML(raw []byte, controls []*sheetir.FormControl, objects []*sheetir.VMLObject) []byte {
	merged := sheetparts.MergeLegacyDrawingVML(raw, mapFormControlsForVML(controls))
	return sheetparts.MergeLegacyDrawingVMLObjects(merged, mapVMLObjectsForWrite(objects))
}

func mapVMLObjectsForWrite(objects []*sheetir.VMLObject) []sheetparts.VMLObject {
	if len(objects) == 0 {
		return nil
	}
	out := make([]sheetparts.VMLObject, 0, len(objects))
	for _, obj := range objects {
		if obj == nil {
			continue
		}
		out = append(out, sheetparts.VMLObject{ID: obj.ID, Type: obj.Type, Style: obj.Style, FillColor: obj.FillColor, StrokeColor: obj.StrokeColor, Text: obj.Text, Anchor: obj.Anchor})
	}
	return out
}

func mapFormControlsForVML(controls []*sheetir.FormControl) []sheetparts.FormControl {
	if len(controls) == 0 {
		return nil
	}
	out := make([]sheetparts.FormControl, 0, len(controls))
	for _, control := range controls {
		if control == nil {
			continue
		}
		out = append(out, sheetparts.FormControl{
			Type:          control.Type,
			AltText:       control.AltText,
			Anchor:        control.Anchor,
			Checked:       control.Checked,
			Mixed:         control.Mixed,
			FirstButton:   control.FirstButton,
			FmlaLink:      control.FmlaLink,
			FmlaGroup:     control.FmlaGroup,
			FmlaRange:     control.FmlaRange,
			SelectionType: control.SelectionType,
			Selected:      append([]int(nil), control.Selected...),
			Min:           control.Min,
			Max:           control.Max,
			Val:           control.Val,
			Increment:     control.Increment,
			PageStep:      control.PageStep,
			Horizontal:    control.Horizontal,
			DropLines:     control.DropLines,
		})
	}
	return out
}
