package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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
		Health: health.NewChecker(log, time.Second, checks...),
	})
}

func do(t *testing.T, h http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func passing(name string) health.Check {
	return health.Check{Name: name, Run: func(context.Context) error { return nil }}
}

func failing(name string) health.Check {
	return health.Check{Name: name, Run: func(context.Context) error { return errors.New("secret-host down") }}
}

func TestRouter_HealthzShapeAndHeaders(t *testing.T) {
	rec := do(t, newRouter(t), http.MethodGet, "/healthz")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Header().Get("X-Request-ID") == "" {
		t.Fatal("X-Request-ID header missing")
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content-type = %q", ct)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"data":{"status":"ok"}}` {
		t.Fatalf("body = %s", got)
	}
}

func TestRouter_HealthzIgnoresFailingDependencies(t *testing.T) {
	rec := do(t, newRouter(t, failing("database")), http.MethodGet, "/healthz")

	if rec.Code != http.StatusOK {
		t.Fatalf("liveness status = %d, want 200 even when a dependency is down", rec.Code)
	}
}

type readyzBody struct {
	Data struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	} `json:"data"`
}

func decodeReadyz(t *testing.T, rec *httptest.ResponseRecorder) readyzBody {
	t.Helper()

	var body readyzBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v (%q)", err, rec.Body.String())
	}
	return body
}

func TestRouter_ReadyzWhenAllChecksPass(t *testing.T) {
	rec := do(t, newRouter(t, passing("database")), http.MethodGet, "/readyz")

	body := decodeReadyz(t, rec)
	if rec.Code != http.StatusOK || body.Data.Status != "ok" || body.Data.Checks["database"] != "ok" {
		t.Fatalf("status=%d body=%+v", rec.Code, body)
	}
}

func TestRouter_ReadyzWhenCheckFailsDoesNotLeakCause(t *testing.T) {
	rec := do(t, newRouter(t, failing("database"), passing("storage")), http.MethodGet, "/readyz")

	body := decodeReadyz(t, rec)
	if rec.Code != http.StatusServiceUnavailable || body.Data.Status != "unavailable" {
		t.Fatalf("status=%d body=%+v", rec.Code, body)
	}
	if body.Data.Checks["database"] != "fail" || body.Data.Checks["storage"] != "ok" {
		t.Fatalf("checks = %v", body.Data.Checks)
	}
	if strings.Contains(rec.Body.String(), "secret-host") {
		t.Fatalf("response leaks cause: %s", rec.Body.String())
	}
}

func TestRouter_ReadyzWithNoChecksIsReady(t *testing.T) {
	rec := do(t, newRouter(t), http.MethodGet, "/readyz")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
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
