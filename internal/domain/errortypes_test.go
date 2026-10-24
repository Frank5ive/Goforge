package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestErrorTypesReportAndUnwrapTheirCause(t *testing.T) {
	cause := errors.New("disk full")

	tests := []struct {
		name    string
		err     error
		message string // what the error says, with the cause
		bare    string // what it says without one
	}{
		{"DomainError", &DomainError{Code: "E1", Message: "bad", Err: cause}, "E1: bad: disk full", "E1: bad"},
		{"TemplateError", NewTemplateError("model.tmpl", "failed", cause), "template error [model.tmpl]: failed: disk full", "template error [model.tmpl]: failed"},
		{"FileSystemError", NewFileSystemError("write", "/x", "failed", cause), "filesystem error [write] at /x: failed: disk full", "filesystem error [write] at /x: failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.message {
				t.Errorf("Error() = %q, want %q", got, tt.message)
			}
			if !errors.Is(tt.err, cause) {
				t.Errorf("errors.Is(err, cause) = false; Unwrap should return the cause")
			}
		})
	}

	bare := []error{&DomainError{Code: "E1", Message: "bad"}, NewTemplateError("model.tmpl", "failed", nil), NewFileSystemError("write", "/x", "failed", nil)}
	for i, err := range bare {
		if got := err.Error(); got != tests[i].bare {
			t.Errorf("%s without a cause = %q, want %q", tests[i].name, got, tests[i].bare)
		}
		if errors.Unwrap(err) != nil {
			t.Errorf("%s without a cause unwraps to %v, want nil", tests[i].name, errors.Unwrap(err))
		}
	}
}

func TestValidationErrorMentionsItsField(t *testing.T) {
	err := NewValidationErrorWithField("name", "is required")
	if got := err.Error(); !strings.Contains(got, "name") || !strings.Contains(got, "is required") {
		t.Errorf("Error() = %q, want the field and the message", got)
	}
}
