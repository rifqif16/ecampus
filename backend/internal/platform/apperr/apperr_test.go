package apperr_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rifqif16/ecampus/backend/internal/platform/apperr"
)

func TestConstructors_SetCategory(t *testing.T) {
	tests := []struct {
		name string
		err  *apperr.Error
		want apperr.Category
	}{
		{"validation", apperr.Validation("C", "m"), apperr.CategoryValidation},
		{"malformed", apperr.Malformed("C", "m"), apperr.CategoryValidation},
		{"unauthorized", apperr.Unauthorized("C", "m"), apperr.CategoryUnauthorized},
		{"forbidden", apperr.Forbidden("C", "m"), apperr.CategoryForbidden},
		{"not found", apperr.NotFound("C", "m"), apperr.CategoryNotFound},
		{"conflict", apperr.Conflict("C", "m"), apperr.CategoryConflict},
		{"business rule", apperr.BusinessRule("C", "m"), apperr.CategoryBusinessRule},
		{"rate limited", apperr.RateLimited("C", "m", time.Second), apperr.CategoryRateLimited},
		{"internal", apperr.Internal(errors.New("boom")), apperr.CategoryInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Category != tt.want {
				t.Fatalf("category = %s, want %s", tt.err.Category, tt.want)
			}
		})
	}
}

func TestMalformed_SetsFlagOnly(t *testing.T) {
	if !apperr.Malformed("C", "m").Malformed {
		t.Fatal("Malformed() must set Malformed flag")
	}
	if apperr.Validation("C", "m").Malformed {
		t.Fatal("Validation() must not set Malformed flag")
	}
}

func TestError_StringIncludesCauseForLogs(t *testing.T) {
	err := apperr.Internal(errors.New("connection refused"))

	if got := err.Error(); !strings.Contains(got, "connection refused") {
		t.Fatalf("Error() = %q, want it to include the cause", got)
	}
}

func TestWithCause_DoesNotMutateShared(t *testing.T) {
	shared := apperr.BusinessRule("KRS_CLASS_FULL", "Kapasitas kelas penuh.")
	cause := errors.New("row lock")

	derived := shared.WithCause(cause)

	if shared.Cause != nil {
		t.Fatal("shared error was mutated")
	}
	if !errors.Is(derived, cause) {
		t.Fatal("derived error must unwrap to cause")
	}
}

func TestWithDetails_DoesNotMutateShared(t *testing.T) {
	shared := apperr.Validation("VALIDATION_ERROR", "Input tidak valid.")

	derived := shared.WithDetails(apperr.Detail{Field: "class_id", Reason: "required"})

	if len(shared.Details) != 0 {
		t.Fatal("shared error was mutated")
	}
	if len(derived.Details) != 1 {
		t.Fatalf("details = %d, want 1", len(derived.Details))
	}
}

func TestErrorsAs_ThroughWrapping(t *testing.T) {
	base := apperr.Conflict("PAYMENT_ALREADY_PAID", "Tagihan sudah lunas.")
	wrapped := fmt.Errorf("pay invoice: %w", base)

	var got *apperr.Error
	if !errors.As(wrapped, &got) {
		t.Fatal("errors.As failed through wrapping")
	}
	if got.Code != "PAYMENT_ALREADY_PAID" {
		t.Fatalf("code = %s", got.Code)
	}
}

func TestRateLimited_CarriesRetryAfter(t *testing.T) {
	err := apperr.RateLimited("RATE_LIMITED", "Terlalu banyak permintaan.", 30*time.Second)

	if err.RetryAfter != 30*time.Second {
		t.Fatalf("RetryAfter = %v", err.RetryAfter)
	}
}
