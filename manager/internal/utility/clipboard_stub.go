//go:build !windows

package utility

import "errors"

var errClipboardNotSupported = errors.New("clipboard: only supported on Windows")

func GetFilePaths() ([]string, error) {
	return nil, errClipboardNotSupported
}

func ReadText() (string, error) {
	return "", errClipboardNotSupported
}