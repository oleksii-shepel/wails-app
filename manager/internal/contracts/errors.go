package contracts

import (
	"errors"
	"io/fs"
	"os"
	"strings"
)

// AppErrorCode is a stable machine-readable string sent with every BatchItemError.
type AppErrorCode = string

const (
	ErrCodePermissionDenied AppErrorCode = "permission_denied"
	ErrCodeNotFound         AppErrorCode = "not_found"
	ErrCodeDiskFull         AppErrorCode = "disk_full"
	ErrCodeReadOnly         AppErrorCode = "read_only"
	ErrCodeAlreadyExists    AppErrorCode = "already_exists"
	ErrCodeInvalidPath      AppErrorCode = "invalid_path"
	ErrCodeCancelled        AppErrorCode = "cancelled"
	ErrCodeUnknown          AppErrorCode = "unknown"
)

type BatchItemError struct {
	Source      string       `json:"source"`
	Destination string       `json:"destination,omitempty"`
	Error       string       `json:"error"`
	Code        AppErrorCode `json:"code"`
}

func ClassifyError(err error) AppErrorCode {
	if err == nil {
		return ErrCodeUnknown
	}

	message := err.Error()
	switch {
	case errors.Is(err, os.ErrNotExist), errors.Is(err, fs.ErrNotExist):
		return ErrCodeNotFound
	case errors.Is(err, os.ErrPermission), errors.Is(err, fs.ErrPermission):
		return ErrCodePermissionDenied
	case errors.Is(err, os.ErrExist), errors.Is(err, fs.ErrExist):
		return ErrCodeAlreadyExists
	case errors.Is(err, os.ErrInvalid), errors.Is(err, fs.ErrInvalid):
		return ErrCodeInvalidPath
	}

	for _, sub := range []string{"no space left", "disk full", "not enough space"} {
		if containsFold(message, sub) {
			return ErrCodeDiskFull
		}
	}
	for _, sub := range []string{"read-only", "read only", "write-protected"} {
		if containsFold(message, sub) {
			return ErrCodeReadOnly
		}
	}
	for _, sub := range []string{"cancelled", "canceled"} {
		if containsFold(message, sub) {
			return ErrCodeCancelled
		}
	}
	return ErrCodeUnknown
}

func containsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}