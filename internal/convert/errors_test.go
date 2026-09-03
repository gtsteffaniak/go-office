package convert

import (
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
