package word97

// mainStoryCursor restricts the concatenated Word PieceTable stream to ccpText.
// Word97 stores footnotes, headers, comments, endnotes and textbox stories in
// the same logical CP space after the main story; those stories have separate
// materializers and must never leak through the document-body cursor.
func mainStoryCursor(inner Cursor, textCount int) Cursor {
	if inner == nil || textCount <= 0 {
		return inner
	}
	return &rangeCursor{inner: inner, startCP: 0, endCP: textCount}
}
