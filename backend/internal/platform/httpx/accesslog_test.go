package httpx_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rifqif16/ecampus/backend/internal/platform/httpx"
	"github.com/rifqif16/ecampus/backend/internal/platform/logger"
)

func fixedRoute(route string) func(*http.Request) string {
	return func(*http.Request) string { return route }
}

func logLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()

	var lines []map[string]any
	for _, raw := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if raw == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(raw), &entry); err != nil {
			t.Fatalf("invalid log line %q: %v", raw, err)
		}
		lines = append(lines, entry)
	}
	return lines
}

func serveLogged(t *testing.T, level string, route string, quiet []string, handler http.HandlerFunc, target string) []map[string]any {
	t.Helper()

	var buf bytes.Buffer
	log, err := logger.New(&buf, level)
	if err != nil {
		t.Fatalf("logger: %v", err)
	}
	chain := httpx.RequestIDMiddleware(
		httpx.ClientIPMiddleware(httpx.ClientIPHeader)(
			httpx.AccessLog(log, fixedRoute(route), quiet...)(handler)))
	req := httptest.NewRequest(http.MethodPost, target, nil)
	req.Header.Set(httpx.ClientIPHeader, "203.0.113.7")

	chain.ServeHTTP(httptest.NewRecorder(), req)
	return logLines(t, &buf)
}

func TestAccessLog_RecordsRequestFields(t *testing.T) {
	handler := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("hello"))
	}

	lines := serveLogged(t, "info", "/api/v1/things/{id}", nil, handler, "/api/v1/things/42")

	if len(lines) != 1 {
		t.Fatalf("got %d log lines, want 1", len(lines))
	}
	entry := lines[0]
	if entry["msg"] != "http request" || entry["level"] != "INFO" {
		t.Fatalf("unexpected entry: %v", entry)
	}
	if entry["method"] != "POST" || entry["route"] != "/api/v1/things/{id}" || entry["client_ip"] != "203.0.113.7" {
		t.Fatalf("unexpected entry: %v", entry)
	}
	if entry["status"] != float64(201) || entry["bytes"] != float64(5) {
		t.Fatalf("status/bytes = %v/%v, want 201/5", entry["status"], entry["bytes"])
	}
	if id, _ := entry["request_id"].(string); id == "" {
		t.Fatal("request_id missing")
	}
	if _, ok := entry["duration_ms"].(float64); !ok {
		t.Fatalf("duration_ms missing: %v", entry)
	}
}

func TestAccessLog_DefaultsToStatus200WhenHandlerWritesNothing(t *testing.T) {
	lines := serveLogged(t, "info", "/x", nil, func(http.ResponseWriter, *http.Request) {}, "/x")

	if lines[0]["status"] != float64(200) {
		t.Fatalf("status = %v, want 200", lines[0]["status"])
	}
}

func TestAccessLog_KeepsFirstStatusOnDoubleWriteHeader(t *testing.T) {
	handler := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.WriteHeader(http.StatusTeapot)
	}

	lines := serveLogged(t, "info", "/x", nil, handler, "/x")

	if lines[0]["status"] != float64(202) {
		t.Fatalf("status = %v, want 202", lines[0]["status"])
	}
}

func TestAccessLog_ServerErrorsLogAtErrorLevel(t *testing.T) {
	handler := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) }

	lines := serveLogged(t, "info", "/x", nil, handler, "/x")

	if lines[0]["level"] != "ERROR" {
		t.Fatalf("level = %v, want ERROR", lines[0]["level"])
	}
}

func TestAccessLog_ClientErrorsStayAtInfo(t *testing.T) {
	handler := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) }

	lines := serveLogged(t, "info", "/x", nil, handler, "/x")

	if lines[0]["level"] != "INFO" {
		t.Fatalf("level = %v, want INFO", lines[0]["level"])
	}
}

func TestAccessLog_QuietRoutesAreDebugOnly(t *testing.T) {
	ok := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }

	if lines := serveLogged(t, "info", "/healthz", []string{"/healthz"}, ok, "/healthz"); len(lines) != 0 {
		t.Fatalf("quiet route logged at info level: %v", lines)
	}
	lines := serveLogged(t, "debug", "/healthz", []string{"/healthz"}, ok, "/healthz")
	if len(lines) != 1 || lines[0]["level"] != "DEBUG" {
		t.Fatalf("quiet route at debug level: %v", lines)
	}
}

func TestAccessLog_QuietRouteFailuresStillSurface(t *testing.T) {
	fail := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }

	lines := serveLogged(t, "info", "/readyz", []string{"/readyz"}, fail, "/readyz")

	if len(lines) != 1 || lines[0]["status"] != float64(503) {
		t.Fatalf("lines = %v, want one line with status 503", lines)
	}
}

func TestAccessLog_UnmatchedRouteLabel(t *testing.T) {
	lines := serveLogged(t, "info", "", nil, func(http.ResponseWriter, *http.Request) {}, "/whatever")

	if lines[0]["route"] != "unmatched" {
		t.Fatalf("route = %v, want unmatched", lines[0]["route"])
	}
}

func TestAccessLog_NeverLogsRawPathOrQuery(t *testing.T) {
	var buf bytes.Buffer
	log, err := logger.New(&buf, "info")
	if err != nil {
		t.Fatalf("logger: %v", err)
	}
	handler := httpx.AccessLog(log, fixedRoute("/reset"))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/reset/secret-id?token=secret-token", nil))

	if strings.Contains(buf.String(), "secret") {
		t.Fatalf("log leaks path or query: %s", buf.String())
	}
}

func TestAccessLog_LogsPanickingRequestAsServerErrorAndRepanics(t *testing.T) {
	var buf bytes.Buffer
	log, err := logger.New(&buf, "info")
	if err != nil {
		t.Fatalf("logger: %v", err)
	}
	handler := httpx.AccessLog(log, fixedRoute("/boom"))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("kaboom")
	}))

	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("panic was swallowed by AccessLog")
			}
		}()
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/boom", nil))
	}()

	lines := logLines(t, &buf)
	if len(lines) != 1 || lines[0]["status"] != float64(500) || lines[0]["level"] != "ERROR" {
		t.Fatalf("lines = %v, want one ERROR line with status 500", lines)
	}
}

func TestAccessLog_PreservesResponseWriterUnwrap(t *testing.T) {
	var buf bytes.Buffer
	log, err := logger.New(&buf, "info")
	if err != nil {
		t.Fatalf("logger: %v", err)
	}
	flushed := false
	handler := httpx.AccessLog(log, fixedRoute("/stream"))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := http.NewResponseController(w).Flush(); err == nil {
			flushed = true
		}
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/stream", nil))

	if !flushed {
		t.Fatal("http.ResponseController could not reach the underlying Flusher")
	}
}
