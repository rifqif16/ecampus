package httpx_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rifqif16/ecampus/backend/internal/platform/httpx"
)

func resolveIP(t *testing.T, remoteAddr, headerValue string) string {
	t.Helper()

	var got string
	handler := httpx.ClientIPMiddleware(httpx.ClientIPHeader)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = httpx.ClientIP(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = remoteAddr
	if headerValue != "" {
		req.Header.Set(httpx.ClientIPHeader, headerValue)
	}

	handler.ServeHTTP(httptest.NewRecorder(), req)
	return got
}

func TestClientIPMiddleware(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		header     string
		want       string
	}{
		{"valid ipv4 header wins", "10.0.0.2:5555", "203.0.113.7", "203.0.113.7"},
		{"valid ipv6 header", "10.0.0.2:5555", "2001:db8::1", "2001:db8::1"},
		{"header is trimmed", "10.0.0.2:5555", "  203.0.113.7 ", "203.0.113.7"},
		{"ipv4-mapped ipv6 is unmapped", "10.0.0.2:5555", "::ffff:203.0.113.7", "203.0.113.7"},
		{"invalid header falls back to peer", "192.0.2.10:4321", "not-an-ip", "192.0.2.10"},
		{"forwarded list is rejected", "192.0.2.10:4321", "1.2.3.4, 5.6.7.8", "192.0.2.10"},
		{"missing header uses peer ipv4", "192.0.2.10:4321", "", "192.0.2.10"},
		{"missing header uses peer ipv6", "[2001:db8::9]:4321", "", "2001:db8::9"},
		{"peer without port", "192.0.2.10", "", "192.0.2.10"},
		{"nothing usable", "garbage", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveIP(t, tt.remoteAddr, tt.header); got != tt.want {
				t.Fatalf("client ip = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClientIP_AbsentReturnsEmpty(t *testing.T) {
	if got := httpx.ClientIP(context.Background()); got != "" {
		t.Fatalf("ClientIP = %q, want empty", got)
	}
}
