package httpx

import (
	"context"
	"net/http"
	"time"

	"github.com/rifqif16/ecampus/backend/internal/platform/apperr"
)

func Timeout(d time.Duration) func(http.Handler) http.Handler {
	if d <= 0 {
		panic("httpx.Timeout: duration must be positive")
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func MaxBodyBytes(ew *ErrorWriter, limit int64) func(http.Handler) http.Handler {
	if limit <= 0 {
		panic("httpx.MaxBodyBytes: limit must be positive")
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > limit {
				ew.Write(w, r, apperr.PayloadTooLarge(payloadTooLargeMessage))
				return
			}

			r.Body = http.MaxBytesReader(w, r.Body, limit)
			next.ServeHTTP(w, r)
		})
	}
}
