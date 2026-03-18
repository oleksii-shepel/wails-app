package elevation

import (
	"os"
	"strings"
)

func IsPermissionDenied(err error) bool {
	if err == nil {
		return false
	}
	if os.IsPermission(err) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "permission denied") ||
		strings.Contains(msg, "access is denied") ||
		strings.Contains(msg, "operation not permitted") ||
		strings.Contains(msg, "eacces") ||
		strings.Contains(msg, "eperm")
}

func ShouldRetryDeleteWithElevation(err error) bool {
	if IsPermissionDenied(err) {
		return true
	}
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "shfileoperationw returned error code 0x78")
}
