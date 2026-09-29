package httpx_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rifqif16/ecampus/backend/internal/platform/apperr"
	"github.com/rifqif16/ecampus/backend/internal/platform/httpx"
	"github.com/rifqif16/ecampus/backend/internal/platform/logger"
)

const testRequestID = "req-123"

type errorEnvelope struct {
	Error struct {
		Category  string          `json:"category"`
		Code      string          `json:"code"`
		Message   string          `json:"message"`
		Details   []apperr.Detail `json:"details"`
		RequestID string          `json:"request_id"`
	} `json:"error"`
}

func newErrorWriter(t *testing.T) (*httpx.ErrorWriter, *bytes.Buffer) {
	t.Helper()

	var logs bytes.Buffer
	log, err := logger.New(&logs, "debug")
	if err != nil {
		t.Fatalf("logger: %v", err)
	}
	return httpx.NewErrorWriter(log), &logs
}

func write(t *testing.T, ew *httpx.ErrorWriter, err error) (*httptest.ResponseRecorder, errorEnvelope) {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/x", nil)
	req = req.WithContext(httpx.WithRequestID(req.Context(), testRequestID))
	rec := httptest.NewRecorder()

	ew.Write(rec, req, err)

	var env errorEnvelope
	if decodeErr := json.Unmarshal(rec.Body.Bytes(), &env); decodeErr != nil {
		t.Fatalf("body is not valid JSON: %v (%q)", decodeErr, rec.Body.String())
	}
	return rec, env
}

func TestErrorWriter_MapsAppErrorCategories(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCat    string
		wantCode   string
	}{
		{"validation values", apperr.Validation("VALIDATION_ERROR", "Input tidak valid."), 422, "VALIDATION_ERROR", "VALIDATION_ERROR"},
		{"validation shape", apperr.Malformed("VALIDATION_ERROR", "Bentuk request salah."), 400, "VALIDATION_ERROR", "VALIDATION_ERROR"},
		{"unauthorized", apperr.Unauthorized("AUTH_INVALID_CREDENTIALS", "Salah."), 401, "UNAUTHORIZED", "AUTH_INVALID_CREDENTIALS"},
		{"forbidden", apperr.Forbidden("AUTH_FORBIDDEN", "Dilarang."), 403, "FORBIDDEN", "AUTH_FORBIDDEN"},
		{"not found", apperr.NotFound("NOT_FOUND", "Tidak ada."), 404, "NOT_FOUND", "NOT_FOUND"},
		{"conflict", apperr.Conflict("PAYMENT_ALREADY_PAID", "Lunas."), 409, "CONFLICT", "PAYMENT_ALREADY_PAID"},
		{"business rule", apperr.BusinessRule("KRS_CLASS_FULL", "Penuh."), 422, "BUSINESS_RULE_VIOLATION", "KRS_CLASS_FULL"},
		{"rate limited", apperr.RateLimited("RATE_LIMITED", "Pelan.", time.Second), 429, "RATE_LIMITED", "RATE_LIMITED"},
		{"wrapped app error", fmt.Errorf("enroll: %w", apperr.BusinessRule("KRS_SKS_EXCEEDED", "Lebih.")), 422, "BUSINESS_RULE_VIOLATION", "KRS_SKS_EXCEEDED"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ew, _ := newErrorWriter(t)

			rec, env := write(t, ew, tt.err)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if env.Error.Category != tt.wantCat || env.Error.Code != tt.wantCode {
				t.Fatalf("category/code = %s/%s, want %s/%s", env.Error.Category, env.Error.Code, tt.wantCat, tt.wantCode)
			}
			if env.Error.RequestID != testRequestID {
				t.Fatalf("request_id = %q, want %q", env.Error.RequestID, testRequestID)
			}
			if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
				t.Fatalf("content-type = %q", ct)
			}
		})
	}
}

func TestErrorWriter_IncludesDetails(t *testing.T) {
	ew, _ := newErrorWriter(t)
	err := apperr.Validation("VALIDATION_ERROR", "Input tidak valid.",
		apperr.Detail{Field: "class_id", Reason: "required"})

	_, env := write(t, ew, err)

	if len(env.Error.Details) != 1 || env.Error.Details[0].Field != "class_id" {
		t.Fatalf("details = %+v", env.Error.Details)
	}
}

func TestErrorWriter_RetryAfterHeader(t *testing.T) {
	tests := []struct {
		name  string
		after time.Duration
		want  string
	}{
		{"whole seconds", 30 * time.Second, "30"},
		{"rounded up", 1500 * time.Millisecond, "2"},
		{"unset", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ew, _ := newErrorWriter(t)

			rec, _ := write(t, ew, apperr.RateLimited("RATE_LIMITED", "Pelan.", tt.after))

			if got := rec.Header().Get("Retry-After"); got != tt.want {
				t.Fatalf("Retry-After = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestErrorWriter_MapsPostgresErrors(t *testing.T) {
	tests := []struct {
		name       string
		pgCode     string
		wantStatus int
		wantCat    string
	}{
		{"unique violation", "23505", 409, "CONFLICT"},
		{"exclusion violation", "23P01", 409, "CONFLICT"},
		{"check violation", "23514", 422, "VALIDATION_ERROR"},
		{"not null violation", "23502", 422, "VALIDATION_ERROR"},
		{"serialization failure", "40001", 409, "CONFLICT"},
		{"deadlock", "40P01", 409, "CONFLICT"},
		{"undefined table", "42P01", 500, "INTERNAL_ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ew, logs := newErrorWriter(t)
			pgErr := &pgconn.PgError{Code: tt.pgCode, TableName: "students_secret", ConstraintName: "uq_nim_secret"}

			rec, env := write(t, ew, fmt.Errorf("insert: %w", pgErr))

			if rec.Code != tt.wantStatus || env.Error.Category != tt.wantCat {
				t.Fatalf("status/category = %d/%s, want %d/%s", rec.Code, env.Error.Category, tt.wantStatus, tt.wantCat)
			}
			if strings.Contains(rec.Body.String(), "secret") {
				t.Fatalf("response leaks database internals: %s", rec.Body.String())
			}
			if !strings.Contains(logs.String(), tt.pgCode) {
				t.Fatalf("root cause missing from logs: %s", logs.String())
			}
		})
	}
}

func TestErrorWriter_UnknownErrorIsSanitizedButLogged(t *testing.T) {
	ew, logs := newErrorWriter(t)

	rec, env := write(t, ew, errors.New("dial tcp 10.0.0.5: secret-host refused"))

	if rec.Code != http.StatusInternalServerError || env.Error.Category != "INTERNAL_ERROR" {
		t.Fatalf("status/category = %d/%s", rec.Code, env.Error.Category)
	}
	if strings.Contains(rec.Body.String(), "secret-host") {
		t.Fatalf("response leaks internal error: %s", rec.Body.String())
	}
	if !strings.Contains(logs.String(), "secret-host") || !strings.Contains(logs.String(), testRequestID) {
		t.Fatalf("logs must hold cause and request id: %s", logs.String())
	}
}

func TestErrorWriter_InternalAppErrorHidesCustomMessage(t *testing.T) {
	ew, _ := newErrorWriter(t)
	err := apperr.Internal(errors.New("boom"))
	err.Message = "leaked detail"

	rec, env := write(t, ew, err)

	if rec.Code != 500 || strings.Contains(rec.Body.String(), "leaked detail") {
		t.Fatalf("internal message leaked: %d %s", rec.Code, rec.Body.String())
	}
	if env.Error.Code != "INTERNAL_ERROR" {
		t.Fatalf("code = %s", env.Error.Code)
	}
}

func TestErrorWriter_NilErrorBecomesInternal(t *testing.T) {
	ew, _ := newErrorWriter(t)

	rec, env := write(t, ew, nil)

	if rec.Code != 500 || env.Error.Category != "INTERNAL_ERROR" {
		t.Fatalf("status/category = %d/%s", rec.Code, env.Error.Category)
	}
}

func TestErrorWriter_MissingRequestIDYieldsEmptyField(t *testing.T) {
	ew, _ := newErrorWriter(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	ew.Write(rec, req, apperr.NotFound("NOT_FOUND", "Tidak ada."))

	var env errorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if env.Error.RequestID != "" {
		t.Fatalf("request_id = %q, want empty", env.Error.RequestID)
	}
}
