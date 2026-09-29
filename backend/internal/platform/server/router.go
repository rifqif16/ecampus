package server

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/rifqif16/ecampus/backend/internal/platform/apperr"
	"github.com/rifqif16/ecampus/backend/internal/platform/health"
	"github.com/rifqif16/ecampus/backend/internal/platform/httpx"
)

var errRouteNotFound = apperr.NotFound("ROUTE_NOT_FOUND", "Endpoint tidak ditemukan.")

type Deps struct {
	Log    *slog.Logger
	Health *health.Handler
}

func NewRouter(deps Deps) http.Handler {
	errWriter := httpx.NewErrorWriter(deps.Log)
	notFound := func(w http.ResponseWriter, r *http.Request) {
		errWriter.Write(w, r, errRouteNotFound)
	}

	r := chi.NewRouter()
	r.Use(httpx.RequestIDMiddleware)
	r.Use(httpx.Recover(errWriter))
	r.NotFound(notFound)
	r.MethodNotAllowed(notFound)

	r.Get("/healthz", deps.Health.Liveness)
	r.Get("/readyz", deps.Health.Readiness)
	return r
}
