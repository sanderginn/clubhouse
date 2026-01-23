package services

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRateLimiterAllowsWithinLimit(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
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
