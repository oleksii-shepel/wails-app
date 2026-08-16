//go:build windows

package platform

import (
	"fmt"
	"syscall"

	contracts "wails-app/internal/contracts"
)

func GetPathAttributes(path string) (contracts.PathAttributes, error) {
	ptr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return contracts.PathAttributes{}, err
	}
	attrs, err := syscall.GetFileAttributes(ptr)
	if err != nil {
		return contracts.PathAttributes{}, err
	}
	return contracts.PathAttributes{
		ReadOnly: attrs&syscall.FILE_ATTRIBUTE_READONLY != 0,
		Hidden:   attrs&syscall.FILE_ATTRIBUTE_HIDDEN != 0,
		System:   attrs&syscall.FILE_ATTRIBUTE_SYSTEM != 0,
		Archive:  attrs&syscall.FILE_ATTRIBUTE_ARCHIVE != 0,
	}, nil
}

func SetPathAttributes(path string, update contracts.PathAttributesUpdate) error {
	ptr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	attrs, err := syscall.GetFileAttributes(ptr)
	if err != nil {
		return err
	}
	apply := func(value *bool, flag uint32) {
		if value == nil {
			return
		}
		if *value {
			attrs |= flag
		} else {
			attrs &^= flag
		}
	}
	apply(update.ReadOnly, syscall.FILE_ATTRIBUTE_READONLY)
	apply(update.Hidden, syscall.FILE_ATTRIBUTE_HIDDEN)
	apply(update.System, syscall.FILE_ATTRIBUTE_SYSTEM)
	apply(update.Archive, syscall.FILE_ATTRIBUTE_ARCHIVE)
	if attrs == 0 {
		attrs = syscall.FILE_ATTRIBUTE_NORMAL
	}
	if err := syscall.SetFileAttributes(ptr, attrs); err != nil {
		return fmt.Errorf("attributes: %w", err)
	}
	return nil
}
