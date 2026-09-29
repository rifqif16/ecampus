package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rifqif16/ecampus/backend/internal/platform/httpx"
)

func rateLimitChain(t *testing.T, requests int, exempt ...string) http.Handler {
	t.Helper()

	ew, _ := newErrorWriter(t)
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	return httpx.RequestIDMiddleware(
		httpx.ClientIPMiddleware(httpx.ClientIPHeader)(
			httpx.RateLimitByIP(ew, requests, time.Minute, exempt...)(ok)))
}

func hit(handler http.Handler, path, clientIP, remoteAddr string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if clientIP != "" {
		req.Header.Set(httpx.ClientIPHeader, clientIP)
	}
	if remoteAddr != "" {
		req.RemoteAddr = remoteAddr
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestRateLimitByIP_BlocksAfterLimitWithStandardError(t *testing.T) {
	handler := rateLimitChain(t, 3)

	for i := 1; i <= 3; i++ {
		if rec := hit(handler, "/x", "203.0.113.7", ""); rec.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, want 200", i, rec.Code)
		}
	}
	rec := hit(handler, "/x", "203.0.113.7", "")

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"category":"RATE_LIMITED"`) || !strings.Contains(body, `"code":"RATE_LIMITED"`) {
		t.Fatalf("body = %s", body)
	}
	if !strings.Contains(body, rec.Header().Get(httpx.RequestIDHeader)) {
		t.Fatalf("error body must carry the request id: %s", body)
	}
	seconds, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil || seconds < 1 || seconds > 60 {
		t.Fatalf("Retry-After = %q, want 1..60 seconds", rec.Header().Get("Retry-After"))
	}
}

func TestRateLimitByIP_SeparateBucketPerClient(t *testing.T) {
	handler := rateLimitChain(t, 1)

	first := hit(handler, "/x", "203.0.113.1", "")
	second := hit(handler, "/x", "203.0.113.2", "")
	firstAgain := hit(handler, "/x", "203.0.113.1", "")

	if first.Code != 200 || second.Code != 200 {
		t.Fatalf("distinct clients got %d and %d, want 200 and 200", first.Code, second.Code)
	}
	if firstAgain.Code != http.StatusTooManyRequests {
		t.Fatalf("repeat client status = %d, want 429", firstAgain.Code)
	}
}

func TestRateLimitByIP_ExemptPathsDoNotConsumeBudget(t *testing.T) {
	handler := rateLimitChain(t, 1, "/healthz")

	for i := 0; i < 10; i++ {
		if rec := hit(handler, "/healthz", "203.0.113.7", ""); rec.Code != http.StatusOK {
			t.Fatalf("exempt request %d status = %d, want 200", i, rec.Code)
		}
	}

	if rec := hit(handler, "/x", "203.0.113.7", ""); rec.Code != http.StatusOK {
		t.Fatalf("first counted request status = %d, want 200 (exempt calls must not use budget)", rec.Code)
	}
	if rec := hit(handler, "/x", "203.0.113.7", ""); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second counted request status = %d, want 429", rec.Code)
	}
}

func TestRateLimitByIP_UnresolvableClientsShareOneBucket(t *testing.T) {
	handler := rateLimitChain(t, 1)

	first := hit(handler, "/x", "", "garbage-one")
	second := hit(handler, "/x", "", "garbage-two")

	if first.Code != http.StatusOK || second.Code != http.StatusTooManyRequests {
		t.Fatalf("statuses = %d, %d; want 200 then 429 (shared unknown bucket)", first.Code, second.Code)
	}
}

func TestRateLimitByIP_IgnoresSpoofedForwardedFor(t *testing.T) {
	handler := rateLimitChain(t, 1)

	for _, spoof := range []string{"198.51.100.1", "198.51.100.2"} {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Header.Set("X-Forwarded-For", spoof)
		req.RemoteAddr = "192.0.2.50:1111"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if spoof == "198.51.100.2" && rec.Code != http.StatusTooManyRequests {
			t.Fatalf("rotating X-Forwarded-For bypassed the limit: status = %d", rec.Code)
		}
	}
}
