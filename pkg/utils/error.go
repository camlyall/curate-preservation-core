package utils

import "strings"

// TruncateError returns a truncated error message if it exceeds the given maxLength.
func TruncateError(errMsg string, maxLength int) string {
	// Remove line breaks for cleaner tag display
	errMsg = strings.ReplaceAll(errMsg, "\n", " ")

	if len(errMsg) <= maxLength {
		return errMsg
	}
	return errMsg[:maxLength-3] + "..."
}
