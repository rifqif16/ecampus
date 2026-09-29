package httpx

import (
	"log/slog"
	"net/http"
	"time"
)

const unmatchedRoute = "unmatched"

type statusRecorder struct {
	http.ResponseWriter
	status      int
	bytes       int64
	wroteHeader bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.wroteHeader {
		s.status = code
		s.wroteHeader = true
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if !s.wroteHeader {
		s.status = http.StatusOK
		s.wroteHeader = true
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += int64(n)
	return n, err
}

func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

func AccessLog(log *slog.Logger, routePattern func(*http.Request) string, quietRoutes ...string) func(http.Handler) http.Handler {
	quiet := make(map[string]struct{}, len(quietRoutes))
	for _, route := range quietRoutes {
		quiet[route] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			completed := false

			defer func() {
				route := routePattern(r)
				if route == "" {
					route = unmatchedRoute
				}
				status := responseStatus(rec, completed)
				log.Log(r.Context(), levelFor(route, status, quiet), "http request",
					"request_id", RequestID(r.Context()),
					"client_ip", ClientIP(r.Context()),
					"method", r.Method,
					"route", route,
					"status", status,
					"duration_ms", float64(time.Since(start).Microseconds())/1000,
					"bytes", rec.bytes,
				)
			}()

			next.ServeHTTP(rec, r)
			completed = true
		})
	}
}

func responseStatus(rec *statusRecorder, completed bool) int {
	switch {
	case rec.wroteHeader:
		return rec.status
	case !completed:
		return http.StatusInternalServerError
	default:
		return http.StatusOK
	}
}

func levelFor(route string, status int, quiet map[string]struct{}) slog.Level {
	if status >= http.StatusInternalServerError {
		return slog.LevelError
	}
	if _, ok := quiet[route]; ok {
		return slog.LevelDebug
	}
	return slog.LevelInfo
}
