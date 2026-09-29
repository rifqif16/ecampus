package httpx_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rifqif16/ecampus/backend/internal/platform/httpx"
)

func mustPanic(t *testing.T, fn func()) {
	t.Helper()

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic, got none")
		}
	}()
	fn()
}

func TestTimeout_SetsDeadlineOnRequestContext(t *testing.T) {
	var deadline time.Time
	var hasDeadline bool
	handler := httpx.Timeout(time.Minute)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		deadline, hasDeadline = r.Context().Deadline()
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if !hasDeadline {
		t.Fatal("request context has no deadline")
	}
	if remaining := time.Until(deadline); remaining <= 0 || remaining > time.Minute {
		t.Fatalf("deadline remaining = %v, want within (0, 1m]", remaining)
	}
}

func TestTimeout_ContextExpiresForSlowHandler(t *testing.T) {
	var got error
	handler := httpx.Timeout(30 * time.Millisecond)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			got = r.Context().Err()
		case <-time.After(2 * time.Second):
		}
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if !errors.Is(got, context.DeadlineExceeded) {
		t.Fatalf("ctx error = %v, want DeadlineExceeded", got)
	}
}

func TestTimeout_ParentCancellationPropagates(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	var got error
	handler := httpx.Timeout(time.Minute)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		cancel()
		got = r.Context().Err()
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(parent)

	handler.ServeHTTP(httptest.NewRecorder(), req)

	if !errors.Is(got, context.Canceled) {
		t.Fatalf("ctx error = %v, want Canceled", got)
	}
}

func TestTimeout_RejectsNonPositiveDuration(t *testing.T) {
	mustPanic(t, func() { httpx.Timeout(0) })
	mustPanic(t, func() { httpx.Timeout(-time.Second) })
}

func TestMaxBodyBytes_RejectsNonPositiveLimit(t *testing.T) {
	ew, _ := newErrorWriter(t)

	mustPanic(t, func() { httpx.MaxBodyBytes(ew, 0) })
	mustPanic(t, func() { httpx.MaxBodyBytes(ew, -1) })
}

func bodyLimitChain(t *testing.T, limit int64, handler http.HandlerFunc) http.Handler {
	t.Helper()

	ew, _ := newErrorWriter(t)
	return httpx.RequestIDMiddleware(httpx.MaxBodyBytes(ew, limit)(handler))
}

func TestMaxBodyBytes_AllowsBodyWithinLimit(t *testing.T) {
	var read int
	handler := bodyLimitChain(t, 10, func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read within limit failed: %v", err)
		}
		read = len(data)
		w.WriteHeader(http.StatusOK)
	})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("0123456789")))

	if rec.Code != http.StatusOK || read != 10 {
		t.Fatalf("status=%d read=%d, want 200/10 (limit is inclusive)", rec.Code, read)
	}
}

func TestMaxBodyBytes_RejectsDeclaredOversizeBeforeHandler(t *testing.T) {
	called := false
	handler := bodyLimitChain(t, 10, func(http.ResponseWriter, *http.Request) { called = true })
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("01234567890")))

	if called {
		t.Fatal("handler must not run when Content-Length exceeds the limit")
	}
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "PAYLOAD_TOO_LARGE") {
		t.Fatalf("body = %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), rec.Header().Get(httpx.RequestIDHeader)) {
		t.Fatalf("error body must carry the request id: %s", rec.Body.String())
	}
}

func TestMaxBodyBytes_CutsOffBodyOfUnknownLength(t *testing.T) {
	ew, _ := newErrorWriter(t)
	handler := httpx.RequestIDMiddleware(httpx.MaxBodyBytes(ew, 10)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			ew.Write(w, r, err)
			return
		}
		w.WriteHeader(http.StatusOK)
	})))
	req := httptest.NewRequest(http.MethodPost, "/", io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("a"), 100))))
	req.ContentLength = -1
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413 for a chunked oversize body", rec.Code)
	}
}

func TestMaxBodyBytes_RequestWithoutBodyPasses(t *testing.T) {
	handler := bodyLimitChain(t, 10, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}
