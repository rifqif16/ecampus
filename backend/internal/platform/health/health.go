package health

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/rifqif16/ecampus/backend/internal/platform/httpx"
)

const DefaultReadinessTimeout = 2 * time.Second

const (
	statusOK          = "ok"
	statusFail        = "fail"
	statusUnavailable = "unavailable"
)

type Check struct {
	Name string
	Run  func(ctx context.Context) error
}

type Pinger interface {
	Ping(ctx context.Context) error
}

func PingCheck(name string, pinger Pinger) Check {
	return Check{Name: name, Run: pinger.Ping}
}

type Handler struct {
	log     *slog.Logger
	timeout time.Duration
	checks  []Check
}

func NewHandler(log *slog.Logger, timeout time.Duration, checks ...Check) *Handler {
	if timeout <= 0 {
		timeout = DefaultReadinessTimeout
	}
	return &Handler{log: log, timeout: timeout, checks: checks}
}

type liveness struct {
	Status string `json:"status"`
}

type readiness struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

func (h *Handler) Liveness(w http.ResponseWriter, r *http.Request) {
	h.write(r.Context(), w, http.StatusOK, liveness{Status: statusOK})
}

func (h *Handler) Readiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	results, ready := h.runChecks(ctx)
	if !ready {
		h.write(ctx, w, http.StatusServiceUnavailable, readiness{Status: statusUnavailable, Checks: results})
		return
	}
	h.write(ctx, w, http.StatusOK, readiness{Status: statusOK, Checks: results})
}

func (h *Handler) runChecks(ctx context.Context) (map[string]string, bool) {
	results := make(map[string]string, len(h.checks))
	ready := true

	for _, check := range h.checks {
		if err := check.Run(ctx); err != nil {
			ready = false
			results[check.Name] = statusFail
			h.log.WarnContext(ctx, "readiness check failed",
				"request_id", httpx.RequestID(ctx), "check", check.Name, "error", err)
			continue
		}
		results[check.Name] = statusOK
	}
	return results, ready
}

func (h *Handler) write(ctx context.Context, w http.ResponseWriter, status int, body any) {
	if err := httpx.WriteData(w, status, body, nil); err != nil {
		h.log.ErrorContext(ctx, "write probe response", "request_id", httpx.RequestID(ctx), "error", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
