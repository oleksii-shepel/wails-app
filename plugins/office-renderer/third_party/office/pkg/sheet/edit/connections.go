package edit

import (
	"fmt"
	"strings"

	sheetir "office/pkg/sheet/ir"
)

// Connections returns workbook external-data connection definitions.
func (w *Workbook) Connections() []*sheetir.WorkbookConnection {
	if w == nil || w.ir == nil {
		return nil
	}
	return append([]*sheetir.WorkbookConnection(nil), w.ir.Connections...)
}

// Connection finds a workbook connection by id or name. A positive id takes precedence.
func (w *Workbook) Connection(id int, name string) (*sheetir.WorkbookConnection, bool) {
	if w == nil || w.ir == nil {
		return nil, false
	}
	for _, connection := range w.ir.Connections {
		if connection == nil {
			continue
		}
		if id > 0 && connection.ID == id {
			return connection, true
		}
		if id <= 0 && strings.EqualFold(strings.TrimSpace(connection.Name), strings.TrimSpace(name)) {
			return connection, true
		}
	}
	return nil, false
}

// SetConnection creates or replaces a workbook connection. When ID is zero a new ID is assigned.
func (w *Workbook) SetConnection(connection sheetir.WorkbookConnection) (*sheetir.WorkbookConnection, error) {
	if w == nil || w.ir == nil {
		return nil, fmt.Errorf("sheet edit: nil workbook")
	}
	connection.Name = strings.TrimSpace(connection.Name)
	if connection.Name == "" {
		return nil, fmt.Errorf("sheet edit: connection name is empty")
	}
	if connection.ID <= 0 {
		connection.ID = w.nextConnectionID()
	}
	for i, existing := range w.ir.Connections {
		if existing == nil {
			continue
		}
		if existing.ID == connection.ID || strings.EqualFold(strings.TrimSpace(existing.Name), connection.Name) {
			clone := connection
			w.ir.Connections[i] = &clone
			return w.ir.Connections[i], nil
		}
	}
	clone := connection
	w.ir.Connections = append(w.ir.Connections, &clone)
	return w.ir.Connections[len(w.ir.Connections)-1], nil
}

// RemoveConnection removes a workbook connection by id or name.
func (w *Workbook) RemoveConnection(id int, name string) bool {
	if w == nil || w.ir == nil {
		return false
	}
	for i, connection := range w.ir.Connections {
		if connection == nil {
			continue
		}
		match := id > 0 && connection.ID == id
		if id <= 0 {
			match = strings.EqualFold(strings.TrimSpace(connection.Name), strings.TrimSpace(name))
		}
		if match {
			w.ir.Connections = append(w.ir.Connections[:i], w.ir.Connections[i+1:]...)
			return true
		}
	}
	return false
}

func (w *Workbook) nextConnectionID() int {
	maxID := 0
	for _, connection := range w.ir.Connections {
		if connection != nil && connection.ID > maxID {
			maxID = connection.ID
		}
	}
	return maxID + 1
}
