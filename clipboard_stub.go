//go:build !windows

package main

import "errors"

var errClipboardNotSupported = errors.New("clipboard: only supported on Windows")

func GetClipboardFilePaths() ([]string, error) {
	return nil, errClipboardNotSupported
}

func ReadClipboardText() (string, error) {
	return "", errClipboardNotSupported
}
