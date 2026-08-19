package app

import (
	"fmt"

	contracts "wails-app/internal/contracts"
)

type registryValueWriter interface {
	WriteRegistryValue(
		path string,
		valueType string,
		data string,
	) (*contracts.OperationResult, error)
}

func (a *App) WriteRegistryValue(
	path string,
	valueType string,
	data string,
) (*contracts.OperationResult, error) {
	writer, ok := a.Filesystem.(registryValueWriter)
	if !ok {
		return nil, fmt.Errorf("registry value writing is unavailable")
	}
	return writer.WriteRegistryValue(path, valueType, data)
}
