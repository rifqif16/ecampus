package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func getFrom(router http.Handler, path, clientIP string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("X-Client-IP", clientIP)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestRouter_RateLimitsGeneralAPIPerClientIP(t *testing.T) {
	router := newRouter(t)

	for i := 1; i <= 300; i++ {
		if rec := getFrom(router, "/api/v1/anything", "203.0.113.7"); rec.Code == http.StatusTooManyRequests {
			t.Fatalf("request %d was limited, budget is 300 per minute", i)
		}
	}

	limited := getFrom(router, "/api/v1/anything", "203.0.113.7")
	if limited.Code != http.StatusTooManyRequests {
		t.Fatalf("request 301 status = %d, want 429", limited.Code)
	}
	if !strings.Contains(limited.Body.String(), "RATE_LIMITED") || limited.Header().Get("Retry-After") == "" {
		t.Fatalf("429 must carry RATE_LIMITED body and Retry-After: %v %s", limited.Header(), limited.Body.String())
	}

	if other := getFrom(router, "/api/v1/anything", "203.0.113.8"); other.Code == http.StatusTooManyRequests {
		t.Fatal("a different client must have its own budget")
	}
}

func TestRouter_ProbesAreNeverRateLimited(t *testing.T) {
	router := newRouter(t)
	for i := 0; i < 301; i++ {
		getFrom(router, "/api/v1/anything", "203.0.113.9")
	}

	for _, path := range []string{"/healthz", "/readyz"} {
		if rec := getFrom(router, path, "203.0.113.9"); rec.Code == http.StatusTooManyRequests {
			t.Fatalf("%s was rate limited", path)
		}
	}
}
