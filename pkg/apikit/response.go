package apikit

import (
	"bytes"
	"encoding/json"
	"net/http"
)

// Envelope is the stable wire representation of a successful response.
type Envelope[T any] struct {
	// Code is the application or HTTP status code.
	Code int `json:"code"`
	// Msg is the stable human-readable response message.
	Msg string `json:"msg"`
	// Data is the response payload.
	Data T `json:"data"`
}

// ErrorEnvelope is the stable wire representation of a failed response.
type ErrorEnvelope struct {
	// Code is the stable application error code.
	Code int `json:"code"`
	// Msg is the stable public error message.
	Msg string `json:"msg"`
	// Data contains optional structured error details.
	Data any `json:"data"`
}

// Write writes a success envelope with the supplied HTTP status and data.
func Write(writer http.ResponseWriter, status int, data any) {
	writeJSON(writer, status, Envelope[any]{Code: status, Msg: "success", Data: data})
}

// WriteError maps err to Error and writes its stable error envelope.
func WriteError(writer http.ResponseWriter, err error) {
	appError := From(err)
	writeJSON(writer, appError.HTTPStatus, ErrorEnvelope{
		Code: appError.Code,
		Msg:  appError.Message,
		Data: appError.Data,
	})
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	var buffer bytes.Buffer
	if err := json.NewEncoder(&buffer).Encode(value); err != nil {
		status = http.StatusInternalServerError
		buffer.Reset()
		_ = json.NewEncoder(&buffer).Encode(ErrorEnvelope{
			Code: http.StatusInternalServerError,
			Msg:  "internal server error",
			Data: nil,
		})
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_, _ = writer.Write(buffer.Bytes())
}
