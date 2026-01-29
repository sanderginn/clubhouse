package services

import (
	"context"
	"testing"
	"time"

	"github.com/sanderginn/clubhouse/internal/testutil"
)

func TestRateLimiterAllowsWithinLimit(t *testing.T) {
	client := testutil.GetTestRedis(t)
	defer testutil.CleanupRedis(t)
	redisServer := testutil.GetMiniredisServer(t)
	limiter := NewRateLimiter(client, "rate:test:", RateLimitConfig{Limit: 2, Window: time.Second})

	ctx := context.Background()
	for i := 0; i < 2; i++ {
		allowed, err := limiter.Allow(ctx, "ip:127.0.0.1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !allowed {
			t.Fatalf("expected request %d to be allowed", i+1)
		}
	}

	allowed, err := limiter.Allow(ctx, "ip:127.0.0.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Fatalf("expected request to be rate limited")
	}

	redisServer.FastForward(time.Second)

	allowed, err = limiter.Allow(ctx, "ip:127.0.0.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatalf("expected request to be allowed after window reset")
	}
}
