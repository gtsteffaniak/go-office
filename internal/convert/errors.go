package convert

import (
	"context"
	"errors"
	"strings"
)

// Converter error codes aligned with ONLYOFFICE Document Server /converter responses.
const (
	ErrCodeTimeout      = -2
	ErrCodeConversion   = -3
	ErrCodeDownload     = -4
	ErrCodeInput        = -7
	ErrCodeInvalidToken = -8
	ErrCodeSizeLimit    = -10
)

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

// ConverterErrorCode maps an error to the ONLYOFFICE /converter error field.
func ConverterErrorCode(err error) int {
	if err == nil {
		return 0
	}
	msg := err.Error()
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) ||
		strings.Contains(msg, "context deadline exceeded") {
		return ErrCodeTimeout
	}
	if strings.Contains(msg, "missing jwt") || strings.Contains(msg, "invalid jwt") ||
		strings.Contains(msg, "jwt key mismatch") || strings.Contains(msg, "jwt url mismatch") {
		return ErrCodeInvalidToken
	}
	if strings.Contains(msg, "download") || strings.Contains(msg, "upstream") ||
		strings.Contains(msg, "status 4") || strings.Contains(msg, "status 5") {
		return ErrCodeDownload
	}
	if strings.Contains(msg, "unsupported output") || strings.Contains(msg, "output type is required") ||
		strings.Contains(msg, "parse") || strings.Contains(msg, "malformed") {
		return ErrCodeInput
	}
	if strings.Contains(msg, "size limit") || strings.Contains(msg, "too large") {
		return ErrCodeSizeLimit
	}
	if strings.Contains(msg, "x2t") || strings.Contains(msg, "convert:") {
		return ErrCodeConversion
	}
	return ErrCodeDownload
}
