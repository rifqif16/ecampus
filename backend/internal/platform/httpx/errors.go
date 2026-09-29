package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rifqif16/ecampus/backend/internal/platform/apperr"
)

const (
	internalMessage        = "Terjadi kesalahan pada server."
	payloadTooLargeMessage = "Ukuran permintaan melebihi batas."

	pgUniqueViolation     = "23505"
	pgExclusionViolation  = "23P01"
	pgCheckViolation      = "23514"
	pgNotNullViolation    = "23502"
	pgSerializationFailed = "40001"
	pgDeadlockDetected    = "40P01"
)

type errorBody struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Category  apperr.Category `json:"category"`
	Code      string          `json:"code"`
	Message   string          `json:"message"`
	Details   []apperr.Detail `json:"details,omitempty"`
	RequestID string          `json:"request_id"`
}

type ErrorWriter struct {
	log *slog.Logger
}

func NewErrorWriter(log *slog.Logger) *ErrorWriter {
	return &ErrorWriter{log: log}
}

func (ew *ErrorWriter) Write(w http.ResponseWriter, r *http.Request, err error) {
	appErr := classify(err)
	requestID := RequestID(r.Context())
	ew.logCause(r.Context(), appErr, requestID)

	if appErr.Category == apperr.CategoryInternal {
		appErr = apperr.Internal(appErr.Cause)
		appErr.Message = internalMessage
	}

	body, marshalErr := json.Marshal(errorBody{Error: errorPayload{
		Category:  appErr.Category,
		Code:      appErr.Code,
		Message:   appErr.Message,
		Details:   appErr.Details,
		RequestID: requestID,
	}})
	if marshalErr != nil {
		http.Error(w, internalMessage, http.StatusInternalServerError)
		return
	}

	if appErr.Category == apperr.CategoryRateLimited && appErr.RetryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(appErr.RetryAfter.Seconds()))))
	}
	writeJSON(w, statusFor(appErr), body)
}

func (ew *ErrorWriter) logCause(ctx context.Context, appErr *apperr.Error, requestID string) {
	if appErr.Cause == nil {
		return
	}

	level := slog.LevelWarn
	if appErr.Category == apperr.CategoryInternal {
		level = slog.LevelError
	}
	ew.log.Log(ctx, level, "request failed",
		"request_id", requestID,
		"category", appErr.Category,
		"code", appErr.Code,
		"error", appErr.Cause,
	)
}

func classify(err error) *apperr.Error {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return apperr.PayloadTooLarge(payloadTooLargeMessage).WithCause(err)
	}

	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		return appErr
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return mapPgError(pgErr)
	}

	if err == nil {
		err = errors.New("nil error passed to ErrorWriter")
	}
	return apperr.Internal(err)
}

func mapPgError(pgErr *pgconn.PgError) *apperr.Error {
	switch pgErr.Code {
	case pgUniqueViolation, pgExclusionViolation:
		return apperr.Conflict("CONFLICT", "Data bertentangan dengan data yang sudah ada.").WithCause(pgErr)
	case pgCheckViolation, pgNotNullViolation:
		return apperr.Validation("VALIDATION_ERROR", "Data tidak valid.").WithCause(pgErr)
	case pgSerializationFailed, pgDeadlockDetected:
		return apperr.Conflict("CONFLICT", "Permintaan bertabrakan dengan proses lain. Silakan coba lagi.").WithCause(pgErr)
	default:
		return apperr.Internal(pgErr)
	}
}

func statusFor(appErr *apperr.Error) int {
	switch appErr.Category {
	case apperr.CategoryValidation:
		return validationStatus(appErr)
	case apperr.CategoryUnauthorized:
		return http.StatusUnauthorized
	case apperr.CategoryForbidden:
		return http.StatusForbidden
	case apperr.CategoryNotFound:
		return http.StatusNotFound
	case apperr.CategoryConflict:
		return http.StatusConflict
	case apperr.CategoryRateLimited:
		return http.StatusTooManyRequests
	case apperr.CategoryBusinessRule:
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}

func validationStatus(appErr *apperr.Error) int {
	switch {
	case appErr.Code == apperr.CodePayloadTooLarge:
		return http.StatusRequestEntityTooLarge
	case appErr.Code == apperr.CodeUnsupportedMediaType:
		return http.StatusUnsupportedMediaType
	case appErr.Malformed:
		return http.StatusBadRequest
	default:
		return http.StatusUnprocessableEntity
	}
}
