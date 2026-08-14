// Package apperror is a compatibility adapter for pkg/apikit.
package apperror

import "github.com/jyysy/backend-infrastructure-go/pkg/apikit"

type Error = apikit.Error

func New(code int, message string, status int, cause error) *Error {
	return apikit.New(code, message, status, cause)
}
func From(err error) *Error                       { return apikit.From(err) }
func Validation(details map[string]string) *Error { return apikit.Validation(details) }
func NotFound(resource string) *Error             { return apikit.NotFound(resource) }
func Conflict(message string, cause error) *Error { return apikit.Conflict(message, cause) }
func MethodNotAllowed() *Error                    { return apikit.MethodNotAllowed() }
func Internal(cause error) *Error                 { return apikit.Internal(cause) }
func ServiceUnavailable(message string, cause error) *Error {
	return apikit.ServiceUnavailable(message, cause)
}
func TooManyRequests() *Error { return apikit.TooManyRequests() }
