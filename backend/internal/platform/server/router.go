package server

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/rifqif16/ecampus/backend/internal/api"
	"github.com/rifqif16/ecampus/backend/internal/platform/apperr"
	"github.com/rifqif16/ecampus/backend/internal/platform/health"
	"github.com/rifqif16/ecampus/backend/internal/platform/httpx"
)

var (
	errRouteNotFound    = apperr.NotFound("ROUTE_NOT_FOUND", "Endpoint tidak ditemukan.")
	errMalformedRequest = apperr.Malformed("VALIDATION_ERROR", "Permintaan tidak valid.")
)

var quietRoutes = []string{"/healthz", "/readyz"}

type Deps struct {
	Log    *slog.Logger
	Health *health.Checker
}

func NewRouter(deps Deps) http.Handler {
	errWriter := httpx.NewErrorWriter(deps.Log)

	r := chi.NewRouter()
	r.Use(httpx.RequestIDMiddleware)
	r.Use(httpx.Recover(errWriter))
	r.Use(httpx.ClientIPMiddleware(httpx.ClientIPHeader))
	r.Use(httpx.AccessLog(deps.Log, chiRoutePattern, quietRoutes...))
	r.NotFound(routeNotFound(errWriter))
	r.MethodNotAllowed(routeNotFound(errWriter))

	strict := api.NewStrictHandlerWithOptions(
		API{Probes: NewProbes(deps.Health)},
		nil,
		api.StrictHTTPServerOptions{
			RequestErrorHandlerFunc:  malformedRequest(errWriter),
			ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) { errWriter.Write(w, r, err) },
		},
	)

	return api.HandlerWithOptions(strict, api.ChiServerOptions{
		BaseRouter:       r,
		ErrorHandlerFunc: malformedRequest(errWriter),
	})
}

func chiRoutePattern(r *http.Request) string {
	if rc := chi.RouteContext(r.Context()); rc != nil {
		return rc.RoutePattern()
	}
	return ""
}

func routeNotFound(ew *httpx.ErrorWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ew.Write(w, r, errRouteNotFound)
	}
}

func malformedRequest(ew *httpx.ErrorWriter) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		ew.Write(w, r, errMalformedRequest.WithCause(err))
	}
}
