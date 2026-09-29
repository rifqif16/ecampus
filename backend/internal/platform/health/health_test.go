package health_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rifqif16/ecampus/backend/internal/platform/health"
	"github.com/rifqif16/ecampus/backend/internal/platform/logger"
)

type probeBody struct {
	Data struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	} `json:"data"`
}

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

func newHandler(t *testing.T, timeout time.Duration, checks ...health.Check) (*health.Handler, *bytes.Buffer) {
	t.Helper()

	var logs bytes.Buffer
	log, err := logger.New(&logs, "debug")
	if err != nil {
		t.Fatalf("logger: %v", err)
	}
	return health.NewHandler(log, timeout, checks...), &logs
}

func okCheck(name string) health.Check {
	return health.Check{Name: name, Run: func(context.Context) error { return nil }}
}

func failCheck(name string, err error) health.Check {
	return health.Check{Name: name, Run: func(context.Context) error { return err }}
}

func serve(t *testing.T, fn http.HandlerFunc) (*httptest.ResponseRecorder, probeBody) {
	t.Helper()

	rec := httptest.NewRecorder()
	fn(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	var body probeBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v (%q)", err, rec.Body.String())
	}
	return rec, body
}

func TestLiveness_NeverRunsChecks(t *testing.T) {
	called := false
	spy := health.Check{Name: "db", Run: func(context.Context) error { called = true; return errors.New("down") }}
	h, _ := newHandler(t, time.Second, spy)

	rec, body := serve(t, h.Liveness)

	if rec.Code != http.StatusOK || body.Data.Status != "ok" {
		t.Fatalf("status=%d body=%+v", rec.Code, body)
	}
	if called {
		t.Fatal("liveness must not touch dependencies")
	}
}

func TestReadiness_AllChecksPass(t *testing.T) {
	h, _ := newHandler(t, time.Second, okCheck("database"), okCheck("storage"))

	rec, body := serve(t, h.Readiness)

	if rec.Code != http.StatusOK || body.Data.Status != "ok" {
		t.Fatalf("status=%d body=%+v", rec.Code, body)
	}
	if body.Data.Checks["database"] != "ok" || body.Data.Checks["storage"] != "ok" {
		t.Fatalf("checks = %v", body.Data.Checks)
	}
}

func TestReadiness_NoChecksIsReady(t *testing.T) {
	h, _ := newHandler(t, time.Second)

	rec, body := serve(t, h.Readiness)

	if rec.Code != http.StatusOK || body.Data.Status != "ok" {
		t.Fatalf("status=%d body=%+v", rec.Code, body)
	}
}

func TestReadiness_FailingCheckReturns503WithoutLeakingCause(t *testing.T) {
	cause := errors.New("dial tcp 10.0.0.5:5432: secret-host refused")
	h, logs := newHandler(t, time.Second, failCheck("database", cause), okCheck("storage"))

	rec, body := serve(t, h.Readiness)

	if rec.Code != http.StatusServiceUnavailable || body.Data.Status != "unavailable" {
		t.Fatalf("status=%d body=%+v", rec.Code, body)
	}
	if body.Data.Checks["database"] != "fail" || body.Data.Checks["storage"] != "ok" {
		t.Fatalf("checks = %v", body.Data.Checks)
	}
	if strings.Contains(rec.Body.String(), "secret-host") {
		t.Fatalf("response leaks cause: %s", rec.Body.String())
	}
	if !strings.Contains(logs.String(), "secret-host") {
		t.Fatalf("cause missing from logs: %s", logs.String())
	}
}

func TestReadiness_SlowCheckIsCutOffByTimeout(t *testing.T) {
	blocking := health.Check{Name: "database", Run: func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}}
	h, _ := newHandler(t, 50*time.Millisecond, blocking)

	start := time.Now()
	rec, body := serve(t, h.Readiness)

	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("readiness took %v, timeout not applied", elapsed)
	}
	if rec.Code != http.StatusServiceUnavailable || body.Data.Checks["database"] != "fail" {
		t.Fatalf("status=%d body=%+v", rec.Code, body)
	}
}

func TestNewHandler_NonPositiveTimeoutUsesDefault(t *testing.T) {
	var gotDeadline time.Time
	probe := health.Check{Name: "db", Run: func(ctx context.Context) error {
		gotDeadline, _ = ctx.Deadline()
		return nil
	}}
	h, _ := newHandler(t, 0, probe)

	serve(t, h.Readiness)

	if remaining := time.Until(gotDeadline); remaining <= 0 || remaining > health.DefaultReadinessTimeout {
		t.Fatalf("deadline remaining = %v, want within (0, %v]", remaining, health.DefaultReadinessTimeout)
	}
}

func TestPingCheck(t *testing.T) {
	tests := []struct {
		name    string
		pinger  fakePinger
		wantErr bool
	}{
		{"healthy", fakePinger{}, false},
		{"unhealthy", fakePinger{err: errors.New("down")}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := health.PingCheck("database", tt.pinger)

			err := check.Run(context.Background())

			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if check.Name != "database" {
				t.Fatalf("name = %q", check.Name)
			}
		})
	}
}
