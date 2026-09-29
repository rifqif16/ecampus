package apperr_test

import (
	"testing"

	"github.com/rifqif16/ecampus/backend/internal/platform/apperr"
)

func TestPayloadTooLarge(t *testing.T) {
	err := apperr.PayloadTooLarge("Terlalu besar.")

	if err.Category != apperr.CategoryValidation || err.Code != apperr.CodePayloadTooLarge || !err.Malformed {
		t.Fatalf("unexpected error: %+v", err)
	}
}

func TestUnsupportedMediaType(t *testing.T) {
	err := apperr.UnsupportedMediaType("Tipe tidak didukung.")

	if err.Category != apperr.CategoryValidation || err.Code != apperr.CodeUnsupportedMediaType || !err.Malformed {
		t.Fatalf("unexpected error: %+v", err)
	}
}
