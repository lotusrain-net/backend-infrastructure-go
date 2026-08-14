package httpkit

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// DecodeJSON strictly decodes one JSON value with a bounded request body.
func DecodeJSON(w http.ResponseWriter, r *http.Request, limit int64, value any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}
