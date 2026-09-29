package health_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rifqif16/ecampus/backend/internal/platform/health"
	"github.com/rifqif16/ecampus/backend/internal/platform/httpx"
	"github.com/rifqif16/ecampus/backend/internal/platform/logger"
)

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

func newChecker(t *testing.T, timeout time.Duration, checks ...health.Check) (*health.Checker, *bytes.Buffer) {
	t.Helper()

	var logs bytes.Buffer
	log, err := logger.New(&logs, "debug")
	if err != nil {
		t.Fatalf("logger: %v", err)
	}
	return health.NewChecker(log, timeout, checks...), &logs
}

func okCheck(name string) health.Check {
	return health.Check{Name: name, Run: func(context.Context) error { return nil }}
}

func failCheck(name string, err error) health.Check {
	return health.Check{Name: name, Run: func(context.Context) error { return err }}
}

func TestRun_AllChecksPass(t *testing.T) {
	c, _ := newChecker(t, time.Second, okCheck("database"), okCheck("storage"))

	report := c.Run(context.Background())

	if !report.Ready {
		t.Fatal("report must be ready")
	}
	if report.Checks["database"] != "ok" || report.Checks["storage"] != "ok" {
		t.Fatalf("checks = %v", report.Checks)
	}
}

func TestRun_NoChecksIsReady(t *testing.T) {
	c, _ := newChecker(t, time.Second)

	report := c.Run(context.Background())

	if !report.Ready || len(report.Checks) != 0 {
		t.Fatalf("report = %+v, want ready with no checks", report)
	}
}

func TestRun_FailureIsReportedWithoutCauseAndLogged(t *testing.T) {
	cause := errors.New("dial tcp 10.0.0.5:5432: secret-host refused")
	c, logs := newChecker(t, time.Second, failCheck("database", cause), okCheck("storage"))
	ctx := httpx.WithRequestID(context.Background(), "req-9")

	report := c.Run(ctx)

	if report.Ready {
		t.Fatal("report must not be ready")
	}
	if report.Checks["database"] != "fail" || report.Checks["storage"] != "ok" {
		t.Fatalf("checks = %v", report.Checks)
	}
	for name, status := range report.Checks {
		if strings.Contains(name+status, "secret-host") {
			t.Fatalf("report leaks cause: %v", report.Checks)
		}
	}
	if !strings.Contains(logs.String(), "secret-host") || !strings.Contains(logs.String(), "req-9") {
		t.Fatalf("logs must hold cause and request id: %s", logs.String())
	}
}

func TestRun_SlowCheckIsCutOffByTimeout(t *testing.T) {
	blocking := health.Check{Name: "database", Run: func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}}
	c, _ := newChecker(t, 50*time.Millisecond, blocking)

	start := time.Now()
	report := c.Run(context.Background())

	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("Run took %v, timeout not applied", elapsed)
	}
	if report.Ready || report.Checks["database"] != "fail" {
		t.Fatalf("report = %+v", report)
	}
}

func TestNewChecker_NonPositiveTimeoutUsesDefault(t *testing.T) {
	var gotDeadline time.Time
	probe := health.Check{Name: "db", Run: func(ctx context.Context) error {
		gotDeadline, _ = ctx.Deadline()
		return nil
	}}
	c, _ := newChecker(t, 0, probe)

	c.Run(context.Background())

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
