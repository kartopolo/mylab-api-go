package billing

import "fmt"

type ValidationError struct {
	Message string
	Errors  map[string]string
}

func (e *ValidationError) Error() string {
	if e == nil {
		return "validation error"
	}
	if e.Message != "" {
		return e.Message
	}
	return "validation error"
}

func NewValidationError(message string, errors map[string]string) *ValidationError {
	if errors == nil {
		errors = map[string]string{}
	}
	return &ValidationError{Message: message, Errors: errors}
}

type NotFoundError struct {
	Message string
}

func (e *NotFoundError) Error() string {
	if e == nil {
		return "not found"
	}
	if e.Message != "" {
		return e.Message
	}
	return "not found"
}

func NewNotFoundError(format string, args ...any) *NotFoundError {
	return &NotFoundError{Message: fmt.Sprintf(format, args...)}
}
