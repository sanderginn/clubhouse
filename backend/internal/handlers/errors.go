package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/sanderginn/clubhouse/internal/middleware"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/observability"
)

func writeError(ctx context.Context, w http.ResponseWriter, statusCode int, code string, message string) {
	userID := ""
	if id, err := middleware.GetUserIDFromContext(ctx); err == nil {
		userID = id.String()
	}
	observability.LogError(ctx, observability.ErrorLog{
		Message:    message,
		Code:       code,
		StatusCode: statusCode,
		UserID:     userID,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(models.ErrorResponse{
		Error: message,
		Code:  code,
	}); err != nil {
		observability.LogError(ctx, observability.ErrorLog{
			Message:    "failed to encode error response",
			Code:       "ENCODE_FAILED",
			StatusCode: statusCode,
			UserID:     userID,
			Err:        err,
		})
	}
}

func writeErrorWithMFARequired(ctx context.Context, w http.ResponseWriter, statusCode int, code string, message string) {
	userID := ""
	if id, err := middleware.GetUserIDFromContext(ctx); err == nil {
		userID = id.String()
	}
	observability.LogError(ctx, observability.ErrorLog{
		Message:    message,
		Code:       code,
		StatusCode: statusCode,
		UserID:     userID,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(models.ErrorResponse{
		Error:       message,
		Code:        code,
		MFARequired: true,
	}); err != nil {
		observability.LogError(ctx, observability.ErrorLog{
			Message:    "failed to encode error response",
			Code:       "ENCODE_FAILED",
			StatusCode: statusCode,
			UserID:     userID,
			Err:        err,
		})
	}
}
