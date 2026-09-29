package server_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRouter_RejectsOversizedBodyBeforeRouting(t *testing.T) {
	router := newRouter(t)
	body := bytes.NewReader(bytes.Repeat([]byte("a"), (1<<20)+1))
	req := httptest.NewRequest(http.MethodPost, "/healthz", body)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "PAYLOAD_TOO_LARGE") {
		t.Fatalf("body = %s", rec.Body.String())
	}
	if rec.Header().Get("X-Request-ID") == "" {
		t.Fatal("X-Request-ID header missing on rejected request")
	}
}

func TestRouter_AcceptsBodyAtLimit(t *testing.T) {
	router := newRouter(t)
	body := bytes.NewReader(bytes.Repeat([]byte("a"), 1<<20))
	req := httptest.NewRequest(http.MethodPost, "/healthz", body)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (POST /healthz is not routed, but the body limit must let it through)", rec.Code)
	}
}
