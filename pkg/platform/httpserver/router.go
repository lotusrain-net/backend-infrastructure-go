package httpserver

import (
	"github.com/go-chi/chi/v5"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/httpkit"
)

type ReadinessChecker = httpkit.ReadinessChecker
type RouterOptions = httpkit.RouterOptions

func NewRouter(options RouterOptions) (chi.Router, error) { return httpkit.NewRouter(options) }
