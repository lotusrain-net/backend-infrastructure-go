package response

import (
	"bytes"
	"encoding/json"
	"net/http"

	"backend-infrastructure-go/internal/shared/apperror"
)

type Envelope struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

type ErrorEnvelope struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

func Write(writer http.ResponseWriter, status int, data any) {
	writeJSON(writer, status, Envelope{Code: status, Msg: "success", Data: data})
}

func WriteError(writer http.ResponseWriter, err error) {
	appError := apperror.From(err)
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
