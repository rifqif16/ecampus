package httpx_test

import (
	"context"
	"testing"

	"github.com/rifqif16/ecampus/backend/internal/platform/httpx"
)

func TestRequestID_RoundTrip(t *testing.T) {
	ctx := httpx.WithRequestID(context.Background(), "abc")

	if got := httpx.RequestID(ctx); got != "abc" {
		t.Fatalf("RequestID = %q, want abc", got)
	}
}

func TestRequestID_AbsentReturnsEmpty(t *testing.T) {
	if got := httpx.RequestID(context.Background()); got != "" {
		t.Fatalf("RequestID = %q, want empty", got)
	}
}
