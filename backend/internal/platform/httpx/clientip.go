package httpx

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

const ClientIPHeader = "X-Client-IP"

type clientIPKey struct{}

func WithClientIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, clientIPKey{}, ip)
}

func ClientIP(ctx context.Context) string {
	ip, _ := ctx.Value(clientIPKey{}).(string)
	return ip
}

func ClientIPMiddleware(header string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := resolveClientIP(r, header)
			next.ServeHTTP(w, r.WithContext(WithClientIP(r.Context(), ip)))
		})
	}
}

func resolveClientIP(r *http.Request, header string) string {
	if ip, ok := normalizeIP(r.Header.Get(header)); ok {
		return ip
	}

	host := r.RemoteAddr
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	ip, _ := normalizeIP(host)
	return ip
}

func normalizeIP(raw string) (string, bool) {
	addr, err := netip.ParseAddr(strings.TrimSpace(raw))
	if err != nil {
		return "", false
	}
	return addr.Unmap().String(), true
}
