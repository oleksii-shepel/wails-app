package stream

// appendUniqueString appends value only when it is not already present.
// Keep this helper in a neutral stream file: both DOC and DOCX normalization
// use it, so its ownership must not live in either format-specific bridge.
func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
