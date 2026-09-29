package health

import (
	"context"
	"log/slog"
	"time"

	"github.com/rifqif16/ecampus/backend/internal/platform/httpx"
)

const DefaultReadinessTimeout = 2 * time.Second

const (
	StatusOK   = "ok"
	StatusFail = "fail"
)

// Check is one readiness dependency (database, storage, job runner, ...).
type Check struct {
	Name string
	Run  func(ctx context.Context) error
}

type Pinger interface {
	Ping(ctx context.Context) error
}

// PingCheck adapts anything with Ping (e.g. *pgxpool.Pool) to a Check.
func PingCheck(name string, pinger Pinger) Check {
	return Check{Name: name, Run: pinger.Ping}
}

// Report is safe to expose: it only names which check failed, never why.
type Report struct {
	Ready  bool
	Checks map[string]string
}

type Checker struct {
	log     *slog.Logger
	timeout time.Duration
	checks  []Check
}

// NewChecker builds a Checker. A non-positive timeout falls back to
// DefaultReadinessTimeout. The timeout is shared by all checks of one run.
func NewChecker(log *slog.Logger, timeout time.Duration, checks ...Check) *Checker {
	if timeout <= 0 {
		timeout = DefaultReadinessTimeout
	}
	return &Checker{log: log, timeout: timeout, checks: checks}
}

// Run executes every check in order. Failure causes go to the log only.
func (c *Checker) Run(ctx context.Context) Report {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	report := Report{Ready: true, Checks: make(map[string]string, len(c.checks))}
	for _, check := range c.checks {
		if err := check.Run(ctx); err != nil {
			report.Ready = false
			report.Checks[check.Name] = StatusFail
			c.log.WarnContext(ctx, "readiness check failed",
				"request_id", httpx.RequestID(ctx), "check", check.Name, "error", err)
			continue
		}
		report.Checks[check.Name] = StatusOK
	}
	return report
}
