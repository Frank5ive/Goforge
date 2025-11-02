package domain

import "fmt"

// DomainError represents a base error type for domain-specific errors.
type DomainError struct {
	Code    string
	Message string
	Err     error
}

func (e *DomainError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *DomainError) Unwrap() error {
	return e.Err
}

// ValidationError represents validation failures in the domain layer.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("validation error [%s]: %s", e.Field, e.Message)
	}
	return fmt.Sprintf("validation error: %s", e.Message)
}

// NewValidationError creates a new validation error.
func NewValidationError(message string) *ValidationError {
	return &ValidationError{
		Message: message,
	}
}

// NewValidationErrorWithField creates a new validation error with a field name.
func NewValidationErrorWithField(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}

// GenerationError represents errors during server generation.
type GenerationError struct {
	Step    string
	Message string
	Err     error
}

func (e *GenerationError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("generation error at step [%s]: %s: %v", e.Step, e.Message, e.Err)
	}
	return fmt.Sprintf("generation error at step [%s]: %s", e.Step, e.Message)
}

func (e *GenerationError) Unwrap() error {
	return e.Err
}

// NewGenerationError creates a new generation error.
func NewGenerationError(step, message string, err error) *GenerationError {
	return &GenerationError{
		Step:    step,
		Message: message,
		Err:     err,
	}
}

// TemplateError represents errors during template rendering.
type TemplateError struct {
	TemplateName string
	Message      string
	Err          error
}

func (e *TemplateError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("template error [%s]: %s: %v", e.TemplateName, e.Message, e.Err)
	}
	return fmt.Sprintf("template error [%s]: %s", e.TemplateName, e.Message)
}

func (e *TemplateError) Unwrap() error {
	return e.Err
}

// NewTemplateError creates a new template error.
func NewTemplateError(templateName, message string, err error) *TemplateError {
	return &TemplateError{
		TemplateName: templateName,
		Message:      message,
		Err:          err,
	}
}

// FileSystemError represents errors during filesystem operations.
type FileSystemError struct {
	Operation string
	Path      string
	Message   string
	Err       error
}

func (e *FileSystemError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("filesystem error [%s] at %s: %s: %v", e.Operation, e.Path, e.Message, e.Err)
	}
	return fmt.Sprintf("filesystem error [%s] at %s: %s", e.Operation, e.Path, e.Message)
}

func (e *FileSystemError) Unwrap() error {
	return e.Err
}

// NewFileSystemError creates a new filesystem error.
func NewFileSystemError(operation, path, message string, err error) *FileSystemError {
	return &FileSystemError{
		Operation: operation,
		Path:      path,
		Message:   message,
		Err:       err,
	}
}
