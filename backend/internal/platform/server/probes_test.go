package server_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/rifqif16/ecampus/backend/internal/api"
	"github.com/rifqif16/ecampus/backend/internal/platform/health"
	"github.com/rifqif16/ecampus/backend/internal/platform/logger"
	"github.com/rifqif16/ecampus/backend/internal/platform/server"
)

func newProbes(t *testing.T, checks ...health.Check) server.Probes {
	t.Helper()

	var logs bytes.Buffer
	log, err := logger.New(&logs, "debug")
	if err != nil {
		t.Fatalf("logger: %v", err)
	}
	return server.NewProbes(health.NewChecker(log, time.Second, checks...))
}

func TestProbes_LivenessNeverRunsChecks(t *testing.T) {
	called := false
	spy := health.Check{Name: "db", Run: func(context.Context) error { called = true; return nil }}

	resp, err := newProbes(t, spy).Liveness(context.Background(), api.LivenessRequestObject{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := resp.(api.Liveness200JSONResponse); !ok {
		t.Fatalf("response type = %T, want Liveness200JSONResponse", resp)
	}
	if called {
		t.Fatal("liveness must not run readiness checks")
	}
}

func TestProbes_ReadinessSelectsResponseByReport(t *testing.T) {
	tests := []struct {
		name   string
		checks []health.Check
		want   string
	}{
		{"ready", []health.Check{passing("database")}, "200"},
		{"not ready", []health.Check{failing("database")}, "503"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := newProbes(t, tt.checks...).Readiness(context.Background(), api.ReadinessRequestObject{})

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			switch typed := resp.(type) {
			case api.Readiness200JSONResponse:
				if tt.want != "200" || typed.Data.Checks["database"] != "ok" {
					t.Fatalf("unexpected 200 response: %+v", typed)
				}
			case api.Readiness503JSONResponse:
				if tt.want != "503" || typed.Data.Checks["database"] != "fail" {
					t.Fatalf("unexpected 503 response: %+v", typed)
				}
			default:
				t.Fatalf("unexpected response type %T", resp)
			}
		})
	}
}
