package apikit

import (
	"errors"
	"fmt"
	"net/http"
)

// Error is the stable public representation of an application failure.
type Error struct {
	// Code is the stable application error code.
	Code int `json:"code"`
	// Message is the stable public error message.
	Message string `json:"msg"`
	// HTTPStatus is the status written to the HTTP response.
	HTTPStatus int `json:"-"`
	// Data contains optional structured error details.
	Data  any `json:"data"`
	cause error
}

// New creates an application error with independent application and HTTP codes.
func New(code int, message string, status int, cause error) *Error {
	return &Error{Code: code, Message: message, HTTPStatus: status, cause: cause}
}

// Error returns a diagnostic message including the cause when present.
func (e *Error) Error() string {
	if e.cause == nil {
		return e.Message
	}
	return fmt.Sprintf("%s: %v", e.Message, e.cause)
}

// Unwrap returns the underlying cause, when one is available.
func (e *Error) Unwrap() error { return e.cause }

// From converts arbitrary errors to a stable internal-server-error response.
func From(err error) *Error {
	var appError *Error
	if errors.As(err, &appError) {
		return appError
	}
	return New(http.StatusInternalServerError, "internal server error", http.StatusInternalServerError, err)
}

// Validation creates a 422 validation error with field-level details.
func Validation(details map[string]string) *Error {
	return &Error{
		Code:       http.StatusUnprocessableEntity,
		Message:    "validation failed",
		HTTPStatus: http.StatusUnprocessableEntity,
		Data:       details,
	}
}

// NotFound creates a 404 error for resource.
func NotFound(resource string) *Error {
	return New(http.StatusNotFound, resource+" not found", http.StatusNotFound, nil)
}

// Conflict creates a 409 conflict error.
func Conflict(message string, cause error) *Error {
	if message == "" {
		message = "conflict"
	}
	return New(http.StatusConflict, message, http.StatusConflict, cause)
}

// MethodNotAllowed creates a 405 error for an unsupported HTTP method.
func MethodNotAllowed() *Error {
	return New(http.StatusMethodNotAllowed, "method not allowed", http.StatusMethodNotAllowed, nil)
}

// Internal creates a 500 error without exposing its cause to clients.
func Internal(cause error) *Error {
	return New(http.StatusInternalServerError, "internal server error", http.StatusInternalServerError, cause)
}

// ServiceUnavailable creates a 503 dependency or availability error.
func ServiceUnavailable(message string, cause error) *Error {
	return New(http.StatusServiceUnavailable, message, http.StatusServiceUnavailable, cause)
}

// TooManyRequests creates a 429 rate-limit error.
func TooManyRequests() *Error {
	return New(http.StatusTooManyRequests, "too many requests", http.StatusTooManyRequests, nil)
}
