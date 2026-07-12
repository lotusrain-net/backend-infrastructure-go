package apperror

import (
	"errors"
	"fmt"
	"net/http"
)

// Error is the stable public representation of an application failure.
type Error struct {
	Code       int    `json:"code"`
	Message    string `json:"msg"`
	HTTPStatus int    `json:"-"`
	Data       any    `json:"data"`
	cause      error
}

func New(code int, message string, status int, cause error) *Error {
	return &Error{Code: code, Message: message, HTTPStatus: status, cause: cause}
}

func (e *Error) Error() string {
	if e.cause == nil {
		return e.Message
	}
	return fmt.Sprintf("%s: %v", e.Message, e.cause)
}

func (e *Error) Unwrap() error { return e.cause }

func From(err error) *Error {
	var appError *Error
	if errors.As(err, &appError) {
		return appError
	}
	return New(http.StatusInternalServerError, "internal server error", http.StatusInternalServerError, err)
}

func Validation(details map[string]string) *Error {
	return &Error{
		Code:       http.StatusUnprocessableEntity,
		Message:    "validation failed",
		HTTPStatus: http.StatusUnprocessableEntity,
		Data:       details,
	}
}

func NotFound(resource string) *Error {
	return New(http.StatusNotFound, resource+" not found", http.StatusNotFound, nil)
}

func ServiceUnavailable(message string, cause error) *Error {
	return New(http.StatusServiceUnavailable, message, http.StatusServiceUnavailable, cause)
}

func TooManyRequests() *Error {
	return New(http.StatusTooManyRequests, "too many requests", http.StatusTooManyRequests, nil)
}
