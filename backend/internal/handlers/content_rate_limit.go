package handlers

import (
	"context"
	"net/http"

	"github.com/sanderginn/clubhouse/internal/observability"
)

type contentRateLimiter interface {
	Allow(ctx context.Context, key string) (bool, error)
}

func checkContentRateLimit(ctx context.Context, w http.ResponseWriter, limiter contentRateLimiter, key string) bool {
	if limiter == nil {
		return true
	}

	allowed, err := limiter.Allow(ctx, key)
	if err != nil {
		observability.LogError(ctx, observability.ErrorLog{
			Message:    "content rate limit check failed",
			Code:       "RATE_LIMIT_CHECK_FAILED",
			StatusCode: http.StatusInternalServerError,
			Err:        err,
		})
		return true
	}

	if !allowed {
		writeError(ctx, w, http.StatusTooManyRequests, "RATE_LIMITED", "Too many requests. Please try again later.")
		return false
	}

	return true
}
