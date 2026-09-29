package httpx_test

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/rifqif16/ecampus/backend/internal/platform/apperr"
)

func TestErrorWriter_MaxBytesErrorBecomesPayloadTooLarge(t *testing.T) {
	ew, logs := newErrorWriter(t)

	rec, env := write(t, ew, &http.MaxBytesError{Limit: 1024})

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
	if env.Error.Category != "VALIDATION_ERROR" || env.Error.Code != "PAYLOAD_TOO_LARGE" {
		t.Fatalf("category/code = %s/%s", env.Error.Category, env.Error.Code)
	}
	if !strings.Contains(logs.String(), "PAYLOAD_TOO_LARGE") {
		t.Fatalf("expected a log line for the rejected request: %s", logs.String())
	}
}

func TestErrorWriter_MaxBytesErrorWrappedInsideAppErrorCause(t *testing.T) {
	ew, _ := newErrorWriter(t)
	wrapped := apperr.Malformed("VALIDATION_ERROR", "Permintaan tidak valid.").
		WithCause(fmt.Errorf("decode body: %w", &http.MaxBytesError{Limit: 10}))

	rec, env := write(t, ew, wrapped)

	if rec.Code != http.StatusRequestEntityTooLarge || env.Error.Code != "PAYLOAD_TOO_LARGE" {
		t.Fatalf("status/code = %d/%s, want 413/PAYLOAD_TOO_LARGE", rec.Code, env.Error.Code)
	}
}

func TestErrorWriter_PayloadTooLargeAppErrorIs413(t *testing.T) {
	ew, _ := newErrorWriter(t)

	rec, env := write(t, ew, apperr.PayloadTooLarge("Berkas terlalu besar."))

	if rec.Code != http.StatusRequestEntityTooLarge || env.Error.Message != "Berkas terlalu besar." {
		t.Fatalf("status/message = %d/%q", rec.Code, env.Error.Message)
	}
}

func TestErrorWriter_UnsupportedMediaTypeIs415(t *testing.T) {
	ew, _ := newErrorWriter(t)

	rec, env := write(t, ew, apperr.UnsupportedMediaType("Tipe berkas tidak didukung."))

	if rec.Code != http.StatusUnsupportedMediaType || env.Error.Code != "UNSUPPORTED_MEDIA_TYPE" {
		t.Fatalf("status/code = %d/%s", rec.Code, env.Error.Code)
	}
	if env.Error.Category != "VALIDATION_ERROR" {
		t.Fatalf("category = %s, want VALIDATION_ERROR", env.Error.Category)
	}
}

func TestErrorWriter_PlainMalformedStays400(t *testing.T) {
	ew, _ := newErrorWriter(t)

	rec, _ := write(t, ew, errors.Join(apperr.Malformed("VALIDATION_ERROR", "Bentuk salah.")))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
