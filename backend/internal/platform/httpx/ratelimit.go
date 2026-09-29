package httpx

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/httprate"

	"github.com/rifqif16/ecampus/backend/internal/platform/apperr"
)

const (
	rateLimitedCode    = "RATE_LIMITED"
	rateLimitedMessage = "Terlalu banyak permintaan. Coba lagi nanti."

	unknownClientKey = "unknown"
)

func RateLimitByIP(ew *ErrorWriter, requests int, window time.Duration, exemptPaths ...string) func(http.Handler) http.Handler {
	exempt := make(map[string]struct{}, len(exemptPaths))
	for _, path := range exemptPaths {
		exempt[path] = struct{}{}
	}

	limiter := httprate.NewRateLimiter(requests, window,
		httprate.WithKeyFuncs(rateLimitKey),
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			retryAfter := retryAfterFrom(w, window)
			ew.Write(w, r, apperr.RateLimited(rateLimitedCode, rateLimitedMessage, retryAfter))
		}),
	)

	return func(next http.Handler) http.Handler {
		limited := limiter.Handler(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := exempt[r.URL.Path]; ok {
				next.ServeHTTP(w, r)
				return
			}
			limited.ServeHTTP(w, r)
		})
	}
}

func rateLimitKey(r *http.Request) (string, error) {
	if ip := ClientIP(r.Context()); ip != "" {
		return ip, nil
	}
	return unknownClientKey, nil
}

func retryAfterFrom(w http.ResponseWriter, fallback time.Duration) time.Duration {
	if seconds, err := strconv.Atoi(w.Header().Get("Retry-After")); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return fallback
}
