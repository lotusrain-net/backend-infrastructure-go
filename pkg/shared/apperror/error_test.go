package apperror_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/lotusrain-net/backend-infrastructure-go/pkg/shared/apperror"
)

func TestErrorPreservesStableCodeStatusAndCause(t *testing.T) {
	cause := errors.New("database unavailable")
	err := apperror.New(51003, "service temporarily unavailable", http.StatusServiceUnavailable, cause)

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
	mapped := apperror.From(errors.New("sensitive detail"))

	if mapped.Code != http.StatusInternalServerError || mapped.HTTPStatus != http.StatusInternalServerError {
		t.Fatalf("unexpected mapping: %#v", mapped)
	}
	if mapped.Message != "internal server error" {
		t.Fatalf("message = %q", mapped.Message)
	}
}

func TestValidationCarriesFieldDetails(t *testing.T) {
	err := apperror.Validation(map[string]string{"email": "is required"})

	if err.Code != http.StatusUnprocessableEntity || err.HTTPStatus != http.StatusUnprocessableEntity {
		t.Fatalf("unexpected validation error: %#v", err)
	}
	details := err.Data.(map[string]string)
	if got := details["email"]; got != "is required" {
		t.Fatalf("detail = %q", got)
	}
}
