package response

import (
	"net/http"

	publicresponse "github.com/jyysy/backend-infrastructure-go/pkg/apikit"
)

type Envelope[T any] = publicresponse.Envelope[T]
type ErrorEnvelope = publicresponse.ErrorEnvelope

func Write(writer http.ResponseWriter, status int, data any) {
	publicresponse.Write(writer, status, data)
}
func WriteError(writer http.ResponseWriter, err error) { publicresponse.WriteError(writer, err) }
