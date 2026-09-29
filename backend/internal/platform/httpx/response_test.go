package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rifqif16/ecampus/backend/internal/platform/httpx"
)

func TestWriteData_DataOnly(t *testing.T) {
	rec := httptest.NewRecorder()

	err := httpx.WriteData(rec, http.StatusOK, map[string]string{"id": "1"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, hasMeta := body["meta"]; hasMeta {
		t.Fatalf("meta must be omitted when nil: %s", rec.Body.String())
	}
	if body["data"].(map[string]any)["id"] != "1" {
		t.Fatalf("unexpected data: %s", rec.Body.String())
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content-type = %q", ct)
	}
}

func TestWriteData_WithMetaAndCustomStatus(t *testing.T) {
	rec := httptest.NewRecorder()

	err := httpx.WriteData(rec, http.StatusCreated, []int{1, 2}, map[string]int{"total": 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var body struct {
		Data []int          `json:"data"`
		Meta map[string]int `json:"meta"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if rec.Code != http.StatusCreated || len(body.Data) != 2 || body.Meta["total"] != 2 {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWriteData_NilDataSerializesAsNull(t *testing.T) {
	rec := httptest.NewRecorder()

	if err := httpx.WriteData(rec, http.StatusOK, nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"data":null}` {
		t.Fatalf("body = %s", got)
	}
}

func TestWriteData_UnmarshalableDataWritesNothing(t *testing.T) {
	rec := httptest.NewRecorder()

	err := httpx.WriteData(rec, http.StatusOK, make(chan int), nil)

	if err == nil {
		t.Fatal("expected marshal error, got nil")
	}
	if rec.Body.Len() != 0 || rec.Header().Get("Content-Type") != "" {
		t.Fatal("nothing must be written when marshaling fails")
	}
}
