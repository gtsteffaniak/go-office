package convert

import (
	"context"
	"fmt"
	"testing"
)

func TestNonRecoverableConvertError(t *testing.T) {
	if NonRecoverableConvertError(nil) {
		t.Fatal("nil should not be non-recoverable")
	}
	if !NonRecoverableConvertError(fmt.Errorf("convert: x2t docx→doc: exit status 80")) {
		t.Fatal("exit status should be non-recoverable")
	}
	if !NonRecoverableConvertError(fmt.Errorf("convert: reverse x2t produced no output")) {
		t.Fatal("no output should be non-recoverable")
	}
}

func TestConverterErrorCode(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{nil, 0},
		{context.DeadlineExceeded, ErrCodeTimeout},
		{fmt.Errorf("converter: missing jwt"), ErrCodeInvalidToken},
		{fmt.Errorf("converter: download: status 404"), ErrCodeDownload},
		{fmt.Errorf("unsupported output type %q", "doc"), ErrCodeInput},
		{fmt.Errorf("convert: x2t failed"), ErrCodeConversion},
	}
	for _, tc := range cases {
		if got := ConverterErrorCode(tc.err); got != tc.want {
			t.Fatalf("ConverterErrorCode(%v) = %d, want %d", tc.err, got, tc.want)
		}
	}
}
