package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rifqif16/ecampus/backend/internal/platform/health"
	"github.com/rifqif16/ecampus/backend/internal/platform/logger"
	"github.com/rifqif16/ecampus/backend/internal/platform/server"
)

func newRouter(t *testing.T, checks ...health.Check) http.Handler {
	t.Helper()

	var logs bytes.Buffer
	log, err := logger.New(&logs, "debug")
	if err != nil {
		t.Fatalf("logger: %v", err)
	}
	return server.NewRouter(server.Deps{
		Log:    log,
		Health: health.NewHandler(log, time.Second, checks...),
	})
}

func do(t *testing.T, h http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func TestRouter_Healthz(t *testing.T) {
	rec := do(t, newRouter(t), http.MethodGet, "/healthz")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Header().Get("X-Request-ID") == "" {
		t.Fatal("X-Request-ID header missing")
	}
}

func TestRouter_ReadyzReflectsChecks(t *testing.T) {
	failing := health.Check{Name: "database", Run: func(context.Context) error { return errors.New("down") }}
	passing := health.Check{Name: "database", Run: func(context.Context) error { return nil }}

	if rec := do(t, newRouter(t, passing), http.MethodGet, "/readyz"); rec.Code != http.StatusOK {
		t.Fatalf("passing readyz status = %d, want 200", rec.Code)
	}
	if rec := do(t, newRouter(t, failing), http.MethodGet, "/readyz"); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("failing readyz status = %d, want 503", rec.Code)
	}
}

func TestRouter_UnknownPathUsesErrorEnvelope(t *testing.T) {
	rec := do(t, newRouter(t), http.MethodGet, "/api/v1/nope")

	assertRouteNotFound(t, rec)
}

func TestRouter_WrongMethodUsesErrorEnvelope(t *testing.T) {
	rec := do(t, newRouter(t), http.MethodPost, "/healthz")

	assertRouteNotFound(t, rec)
}

func assertRouteNotFound(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if rec.Header().Get("X-Request-ID") == "" {
		t.Fatal("X-Request-ID header missing on error response")
	}

	var body struct {
		Error struct {
			Category  string `json:"category"`
			Code      string `json:"code"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Error.Category != "NOT_FOUND" || body.Error.Code != "ROUTE_NOT_FOUND" {
		t.Fatalf("error = %+v", body.Error)
	}
	if body.Error.RequestID != rec.Header().Get("X-Request-ID") {
		t.Fatalf("request_id %q != header %q", body.Error.RequestID, rec.Header().Get("X-Request-ID"))
	}
}
