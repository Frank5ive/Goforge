package domain

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrorCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"attached", WithCode(CodeMarkerMissing, errors.New("x")), CodeMarkerMissing},
		{"attached deeper", fmt.Errorf("context: %w", WithCode(CodeNotAProject, errors.New("x"))), CodeNotAProject},
		{"outermost wins", WithCode(CodeAlreadyExists, WithCode(CodeInternal, errors.New("x"))), CodeAlreadyExists},
		{"validation error", NewValidationError("bad"), CodeInvalidInput},
		{"validation step", NewGenerationError("validation", "bad", nil), CodeInvalidInput},
		{"filesystem error", NewFileSystemError("write", "/x", "failed", nil), CodeWriteFailed},
		{"anything else", errors.New("x"), CodeInternal},
	}
	for _, tt := range tests {
		if got := ErrorCode(tt.err); got != tt.want {
			t.Errorf("%s: ErrorCode() = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestCodedErrorKeepsMessageAndCause(t *testing.T) {
	cause := errors.New("disk full")
	err := WithHint(CodeWriteFailed, "free some space", cause)

	if err.Error() != "disk full" || !errors.Is(err, cause) {
		t.Errorf("err = %v, want the cause's message and chain", err)
	}
	if ErrorHint(err) != "free some space" || ErrorHint(cause) != "" {
		t.Errorf("ErrorHint() = %q", ErrorHint(err))
	}
}
