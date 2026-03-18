package contracts

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
