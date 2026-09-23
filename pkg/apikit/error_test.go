package apikit_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/lotusrain-net/backend-infrastructure-go/pkg/apikit"
)

func TestErrorPreservesStableCodeStatusAndCause(t *testing.T) {
	cause := errors.New("database unavailable")
	err := apikit.New(51003, "service temporarily unavailable", http.StatusServiceUnavailable, cause)

	if err.Code != 51003 || err.Message != "service temporarily unavailable" {
		t.Fatalf("unexpected public error: %#v", err)
	}
	if err.HTTPStatus != http.StatusServiceUnavailable {
		t.Fatalf("HTTPStatus = %d, want %d", err.HTTPStatus, http.StatusServiceUnavailable)
	}
	if !errors.Is(err, cause) {
		t.Fatal("wrapped cause is not discoverable with errors.Is")
	}
}

func TestFromMapsUnknownErrorsToInternalError(t *testing.T) {
	mapped := apikit.From(errors.New("sensitive detail"))

	if mapped.Code != http.StatusInternalServerError || mapped.HTTPStatus != http.StatusInternalServerError {
		t.Fatalf("unexpected mapping: %#v", mapped)
	}
	if mapped.Message != "internal server error" {
		t.Fatalf("message = %q", mapped.Message)
	}
}

func TestValidationCarriesFieldDetails(t *testing.T) {
	err := apikit.Validation(map[string]string{"email": "is required"})

	if err.Code != http.StatusUnprocessableEntity || err.HTTPStatus != http.StatusUnprocessableEntity {
		t.Fatalf("unexpected validation error: %#v", err)
	}
	details := err.Data.(map[string]string)
	if got := details["email"]; got != "is required" {
		t.Fatalf("detail = %q", got)
	}
}

func TestConvenienceErrorsUseStableStatuses(t *testing.T) {
	cause := errors.New("constraint")
	for _, test := range []struct {
		name   string
		err    *apikit.Error
		status int
	}{
		{name: "not found", err: apikit.NotFound("user"), status: http.StatusNotFound},
		{name: "conflict", err: apikit.Conflict("duplicate", cause), status: http.StatusConflict},
		{name: "method not allowed", err: apikit.MethodNotAllowed(), status: http.StatusMethodNotAllowed},
		{name: "internal", err: apikit.Internal(cause), status: http.StatusInternalServerError},
		{name: "unavailable", err: apikit.ServiceUnavailable("database unavailable", cause), status: http.StatusServiceUnavailable},
		{name: "limited", err: apikit.TooManyRequests(), status: http.StatusTooManyRequests},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.err.Code != test.status || test.err.HTTPStatus != test.status {
				t.Fatalf("error = %#v", test.err)
			}
		})
	}
}
