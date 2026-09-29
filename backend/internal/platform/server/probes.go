package server

import (
	"context"

	"github.com/rifqif16/ecampus/backend/internal/api"
	"github.com/rifqif16/ecampus/backend/internal/platform/health"
)

const (
	statusOK          = "ok"
	statusUnavailable = "unavailable"
)

type Probes struct {
	checker *health.Checker
}

func NewProbes(checker *health.Checker) Probes {
	return Probes{checker: checker}
}

func (Probes) Liveness(context.Context, api.LivenessRequestObject) (api.LivenessResponseObject, error) {
	return api.Liveness200JSONResponse{
		Data: api.LivenessStatus{Status: api.LivenessStatusStatus(statusOK)},
	}, nil
}

func (p Probes) Readiness(ctx context.Context, _ api.ReadinessRequestObject) (api.ReadinessResponseObject, error) {
	report := p.checker.Run(ctx)
	if !report.Ready {
		return api.Readiness503JSONResponse(readinessBody(report, statusUnavailable)), nil
	}
	return api.Readiness200JSONResponse(readinessBody(report, statusOK)), nil
}

func readinessBody(report health.Report, status string) api.ReadinessResponse {
	return api.ReadinessResponse{
		Data: api.ReadinessStatus{
			Status: api.ReadinessStatusStatus(status),
			Checks: report.Checks,
		},
	}
}
