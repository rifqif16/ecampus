package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/rifqif16/ecampus/backend/internal/platform/httpx"
)

var generatedID = regexp.MustCompile(`^[0-9a-f]{32}$`)

func serveWithRequestID(t *testing.T, incoming string) (headerID, ctxID string) {
	t.Helper()

	handler := httpx.RequestIDMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		ctxID = httpx.RequestID(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if incoming != "" {
		req.Header.Set(httpx.RequestIDHeader, incoming)
	}
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	return rec.Header().Get(httpx.RequestIDHeader), ctxID
}

func TestRequestIDMiddleware_GeneratesWhenAbsent(t *testing.T) {
	headerID, ctxID := serveWithRequestID(t, "")

	if !generatedID.MatchString(headerID) {
		t.Fatalf("generated id = %q, want 32 hex chars", headerID)
	}
	if headerID != ctxID {
		t.Fatalf("header id %q != context id %q", headerID, ctxID)
	}
}

func TestRequestIDMiddleware_GeneratedIDsAreUnique(t *testing.T) {
	first, _ := serveWithRequestID(t, "")
	second, _ := serveWithRequestID(t, "")

	if first == second {
		t.Fatalf("two requests got the same id %q", first)
	}
}

func TestRequestIDMiddleware_ReusesValidIncomingID(t *testing.T) {
	headerID, ctxID := serveWithRequestID(t, "proxy-abc_123.4")

	if headerID != "proxy-abc_123.4" || ctxID != "proxy-abc_123.4" {
		t.Fatalf("header=%q ctx=%q, want incoming id reused", headerID, ctxID)
	}
}

func TestRequestIDMiddleware_ReplacesInvalidIncomingID(t *testing.T) {
	tests := []struct {
		name     string
		incoming string
	}{
		{"contains space", "bad id"},
		{"contains markup", "<script>x</script>"},
		{"too long", strings.Repeat("a", 65)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headerID, ctxID := serveWithRequestID(t, tt.incoming)

			if !generatedID.MatchString(headerID) || headerID != ctxID {
				t.Fatalf("header=%q ctx=%q, want a freshly generated id", headerID, ctxID)
			}
		})
	}
}

func TestRequestIDMiddleware_AcceptsMaxLengthID(t *testing.T) {
	id := strings.Repeat("a", 64)

	headerID, _ := serveWithRequestID(t, id)

	if headerID != id {
		t.Fatalf("64-char id was replaced: %q", headerID)
	}
}

func chain(handler http.HandlerFunc, ew *httpx.ErrorWriter) http.Handler {
	return httpx.RequestIDMiddleware(httpx.Recover(ew)(handler))
}

func TestRecover_PanicBecomesInternalError(t *testing.T) {
	ew, logs := newErrorWriter(t)
	handler := chain(func(http.ResponseWriter, *http.Request) { panic("kaboom-secret") }, ew)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "kaboom-secret") {
		t.Fatalf("response leaks panic value: %s", rec.Body.String())
	}

	var env errorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if env.Error.Category != "INTERNAL_ERROR" || env.Error.RequestID != rec.Header().Get(httpx.RequestIDHeader) {
		t.Fatalf("unexpected error body: %+v", env.Error)
	}
	if !strings.Contains(logs.String(), "kaboom-secret") || !strings.Contains(logs.String(), "goroutine") {
		t.Fatalf("logs must hold panic value and stack: %s", logs.String())
	}
}

func TestRecover_HandlesErrorPanicValue(t *testing.T) {
	ew, _ := newErrorWriter(t)
	handler := chain(func(http.ResponseWriter, *http.Request) { panic(http.ErrNotSupported) }, ew)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestRecover_PassesThroughWhenNoPanic(t *testing.T) {
	ew, logs := newErrorWriter(t)
	handler := chain(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }, ew)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if logs.Len() != 0 {
		t.Fatalf("unexpected logs: %s", logs.String())
	}
}

func TestRecover_ServerKeepsServingAfterPanic(t *testing.T) {
	ew, _ := newErrorWriter(t)
	handler := chain(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/boom" {
			panic("boom")
		}
		w.WriteHeader(http.StatusOK)
	}, ew)

	boom := httptest.NewRecorder()
	handler.ServeHTTP(boom, httptest.NewRequest(http.MethodGet, "/boom", nil))
	ok := httptest.NewRecorder()
	handler.ServeHTTP(ok, httptest.NewRequest(http.MethodGet, "/fine", nil))

	if boom.Code != 500 || ok.Code != 200 {
		t.Fatalf("statuses = %d, %d; want 500, 200", boom.Code, ok.Code)
	}
}

func TestRecover_RepanicsOnAbortHandler(t *testing.T) {
	ew, _ := newErrorWriter(t)
	handler := chain(func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) }, ew)

	defer func() {
		if got := recover(); got != http.ErrAbortHandler {
			t.Fatalf("recovered %v, want http.ErrAbortHandler", got)
		}
	}()
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	t.Fatal("expected re-panic")
}
