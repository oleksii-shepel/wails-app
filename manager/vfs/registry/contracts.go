package registry

import (
	contracts "wails-app/internal/contracts"
	"wails-app/vfs"
)

type Metadata interface {
	RegistryKind() string
	RegistryValueType() string
	RegistryPreview() string
}

func ApplyFileInfoMetadata(file *contracts.FileInfo, entry vfs.FileInfo) {
	if file == nil {
		return
	}
	metadata, ok := entry.(Metadata)
	if !ok {
		return
	}
	file.RegistryKind = metadata.RegistryKind()
	file.RegistryValueType = metadata.RegistryValueType()
	file.RegistryPreview = metadata.RegistryPreview()
}