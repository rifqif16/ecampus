package apperr

import (
	"fmt"
	"time"
)

type Category string

const (
	CategoryValidation   Category = "VALIDATION_ERROR"
	CategoryUnauthorized Category = "UNAUTHORIZED"
	CategoryForbidden    Category = "FORBIDDEN"
	CategoryNotFound     Category = "NOT_FOUND"
	CategoryConflict     Category = "CONFLICT"
	CategoryRateLimited  Category = "RATE_LIMITED"
	CategoryBusinessRule Category = "BUSINESS_RULE_VIOLATION"
	CategoryInternal     Category = "INTERNAL_ERROR"
)

type Detail struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

type Error struct {
	Category Category
	Code     string
	Message  string
	Details  []Detail
	Malformed  bool
	RetryAfter time.Duration
	Cause error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Cause }

func (e *Error) WithCause(cause error) *Error {
	clone := *e
	clone.Cause = cause
	return &clone
}

func (e *Error) WithDetails(details ...Detail) *Error {
	clone := *e
	clone.Details = details
	return &clone
}

func New(category Category, code, message string) *Error {
	return &Error{Category: category, Code: code, Message: message}
}

func Validation(code, message string, details ...Detail) *Error {
	return &Error{Category: CategoryValidation, Code: code, Message: message, Details: details}
}

func Malformed(code, message string, details ...Detail) *Error {
	err := Validation(code, message, details...)
	err.Malformed = true
	return err
}

func Unauthorized(code, message string) *Error {
	return New(CategoryUnauthorized, code, message)
}

func Forbidden(code, message string) *Error {
	return New(CategoryForbidden, code, message)
}

func NotFound(code, message string) *Error {
	return New(CategoryNotFound, code, message)
}

func Conflict(code, message string) *Error {
	return New(CategoryConflict, code, message)
}

func BusinessRule(code, message string) *Error {
	return New(CategoryBusinessRule, code, message)
}

func RateLimited(code, message string, retryAfter time.Duration) *Error {
	err := New(CategoryRateLimited, code, message)
	err.RetryAfter = retryAfter
	return err
}

func Internal(cause error) *Error {
	return &Error{Category: CategoryInternal, Code: "INTERNAL_ERROR", Message: "internal error", Cause: cause}
}
