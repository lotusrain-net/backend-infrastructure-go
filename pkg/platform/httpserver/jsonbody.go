package httpserver

import (
	"net/http"

	"github.com/lotusrain-net/backend-infrastructure-go/pkg/httpkit"
)

func DecodeJSON(w http.ResponseWriter, r *http.Request, limit int64, value any) error {
	return httpkit.DecodeJSON(w, r, limit, value)
}
