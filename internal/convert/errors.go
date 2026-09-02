package convert

import "strings"

// NonRecoverableConvertError reports x2t failures that will not succeed on retry
// (unsupported output format such as binary .doc, corrupt input, missing Editor.bin).
// Callers clear pending changes instead of leaving the cache dirty for repeated flush-on-open.
func NonRecoverableConvertError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "exit status") ||
		strings.Contains(msg, "produced no output") ||
		strings.Contains(msg, "Editor.bin missing")
}
