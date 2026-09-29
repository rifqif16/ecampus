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

func newRouterWithLogs(t *testing.T, checks ...health.Check) (http.Handler, *bytes.Buffer) {
	t.Helper()

	var logs bytes.Buffer
	log, err := logger.New(&logs, "debug")
	if err != nil {
		t.Fatalf("logger: %v", err)
	}
	handler := server.NewRouter(server.Deps{
		Log:    log,
		Health: health.NewChecker(log, time.Second, checks...),
	})
	return handler, &logs
}

func newRouter(t *testing.T, checks ...health.Check) http.Handler {
	t.Helper()

	handler, _ := newRouterWithLogs(t, checks...)
	return handler
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

func accessLogEntries(t *testing.T, logs *bytes.Buffer) []map[string]any {
	t.Helper()

	var entries []map[string]any
	for _, raw := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		var entry map[string]any
		if err := json.Unmarshal([]byte(raw), &entry); err != nil {
			t.Fatalf("invalid log line %q: %v", raw, err)
		}
		if entry["msg"] == "http request" {
			entries = append(entries, entry)
		}
	}
	return entries
}

func TestRouter_AccessLogUsesRoutePatternAndClientIP(t *testing.T) {
	router, logs := newRouterWithLogs(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz?probe=1", nil)
	req.Header.Set("X-Client-IP", "203.0.113.7")

	router.ServeHTTP(httptest.NewRecorder(), req)

	entries := accessLogEntries(t, logs)
	if len(entries) != 1 {
		t.Fatalf("got %d access log entries, want 1", len(entries))
	}
	entry := entries[0]
	if entry["route"] != "/healthz" || entry["client_ip"] != "203.0.113.7" || entry["status"] != float64(200) {
		t.Fatalf("unexpected entry: %v", entry)
	}
	if entry["level"] != "DEBUG" {
		t.Fatalf("probe level = %v, want DEBUG", entry["level"])
	}
}

func TestRouter_AccessLogLabelsUnknownPathsUnmatched(t *testing.T) {
	router, logs := newRouterWithLogs(t)

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/secret-id-123", nil))

	entries := accessLogEntries(t, logs)
	if len(entries) != 1 || entries[0]["route"] != "unmatched" || entries[0]["status"] != float64(404) {
		t.Fatalf("entries = %v", entries)
	}
	if strings.Contains(logs.String(), "secret-id-123") {
		t.Fatalf("log leaks raw path: %s", logs.String())
	}
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
