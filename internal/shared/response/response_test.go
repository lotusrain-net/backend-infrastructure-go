package response_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"backend-infrastructure-go/internal/shared/apperror"
	"backend-infrastructure-go/internal/shared/response"
)

func TestWriteEncodesStableEnvelope(t *testing.T) {
	recorder := httptest.NewRecorder()
	response.Write(recorder, http.StatusCreated, map[string]string{"id": "42"})

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("content type = %q", got)
	}
	var body struct {
		Code int               `json:"code"`
		Msg  string            `json:"msg"`
		Data map[string]string `json:"data"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Code != http.StatusCreated || body.Msg != "success" || body.Data["id"] != "42" {
		t.Fatalf("unexpected envelope: %#v", body)
	}
}

func TestWriteErrorUsesPublicApplicationError(t *testing.T) {
	recorder := httptest.NewRecorder()
	response.WriteError(recorder, apperror.NotFound("user"))

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d", recorder.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["code"] != float64(http.StatusNotFound) || body["msg"] != "user not found" {
		t.Fatalf("unexpected error envelope: %#v", body)
	}
	if _, exists := body["data"]; !exists {
		t.Fatalf("error envelope must preserve code/msg/data shape: %#v", body)
	}
}

func TestWriteValidationErrorPlacesDetailsInData(t *testing.T) {
	recorder := httptest.NewRecorder()
	response.WriteError(recorder, apperror.Validation(map[string]string{"email": "is required"}))

	var body struct {
		Data map[string]string `json:"data"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Data["email"] != "is required" {
		t.Fatalf("data = %#v", body.Data)
	}
}

func TestWriteErrorKeepsCodeNumericForFrontendCompatibility(t *testing.T) {
	recorder := httptest.NewRecorder()
	response.WriteError(recorder, apperror.NotFound("user"))

	var body struct {
		Code int `json:"code"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("error code must be a JSON integer: %v", err)
	}
	if body.Code != http.StatusNotFound {
		t.Fatalf("code = %d", body.Code)
	}
}

func TestWriteBuffersBeforeCommittingStatusAndFallsBackOnEncodingFailure(t *testing.T) {
	recorder := httptest.NewRecorder()
	response.Write(recorder, http.StatusCreated, make(chan int))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", recorder.Code)
	}
	if got := recorder.Body.String(); !contains(got, `"code":500`) || contains(got, `"code":201`) {
		t.Fatalf("body = %q", got)
	}
}

func TestWriteErrorDropsUnencodableErrorData(t *testing.T) {
	recorder := httptest.NewRecorder()
	err := apperror.New(51000, "unsafe payload", http.StatusBadGateway, nil)
	err.Data = make(chan int)

	response.WriteError(recorder, err)

	if recorder.Code != http.StatusInternalServerError || !contains(recorder.Body.String(), `"code":500`) {
		t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}

func TestWriteErrorDoesNotLeakUnknownError(t *testing.T) {
	recorder := httptest.NewRecorder()
	response.WriteError(recorder, errors.New("secret database message"))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", recorder.Code)
	}
	if got := recorder.Body.String(); got == "" || contains(got, "secret database message") {
		t.Fatalf("unexpected body: %q", got)
	}
}

func contains(value, substring string) bool {
	for index := 0; index+len(substring) <= len(value); index++ {
		if value[index:index+len(substring)] == substring {
			return true
		}
	}
	return false
}
