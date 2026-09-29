package httpx

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"regexp"
	"runtime/debug"

	"github.com/rifqif16/ecampus/backend/internal/platform/apperr"
)

const requestIDBytes = 16

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if !validRequestID.MatchString(id) {
			id = newRequestID()
		}

		w.Header().Set(RequestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(WithRequestID(r.Context(), id)))
	})
}

func newRequestID() string {
	buf := make([]byte, requestIDBytes)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

func Recover(ew *ErrorWriter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				ew.Write(w, r, apperr.Internal(fmt.Errorf("panic: %v\n%s", rec, debug.Stack())))
			}()

			next.ServeHTTP(w, r)
		})
	}
}
