package services

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestAuthFailureTrackerEscalatesLockout(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	tracker := &AuthFailureTracker{
		redis: client,
		config: AuthFailureConfig{
			Threshold:   2,
			Window:      time.Hour,
			BaseLockout: time.Second,
			MaxLockout:  10 * time.Second,
		},
		now: time.Now,
	}

	ctx := context.Background()
	identifier := []string{"alice"}
	ip := "127.0.0.1"

	locked, retryAfter, err := tracker.RegisterFailure(ctx, ip, identifier)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if locked || retryAfter != 0 {
		t.Fatalf("expected first failure to avoid lockout, got locked=%v retry=%v", locked, retryAfter)
	}

	locked, retryAfter, err = tracker.RegisterFailure(ctx, ip, identifier)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !locked || retryAfter != time.Second {
		t.Fatalf("expected lockout of 1s, got locked=%v retry=%v", locked, retryAfter)
	}

	redisServer.FastForward(time.Second)

	locked, retryAfter, err = tracker.RegisterFailure(ctx, ip, identifier)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !locked || retryAfter != 2*time.Second {
		t.Fatalf("expected escalated lockout of 2s, got locked=%v retry=%v", locked, retryAfter)
	}
}

func TestAuthFailureTrackerResetClearsState(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	tracker := &AuthFailureTracker{
		redis: client,
		config: AuthFailureConfig{
			Threshold:   1,
			Window:      time.Hour,
			BaseLockout: time.Second,
			MaxLockout:  10 * time.Second,
		},
		now: time.Now,
	}

	ctx := context.Background()
	identifier := []string{"bob"}
	ip := "127.0.0.1"

	locked, retryAfter, err := tracker.RegisterFailure(ctx, ip, identifier)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !locked || retryAfter == 0 {
		t.Fatalf("expected lockout before reset")
	}

	if err := tracker.Reset(ctx, ip, identifier); err != nil {
		t.Fatalf("unexpected error resetting tracker: %v", err)
	}

	locked, retryAfter, err = tracker.IsLocked(ctx, ip, identifier)
	if err != nil {
		t.Fatalf("unexpected error checking lockout: %v", err)
	}
	if locked || retryAfter != 0 {
		t.Fatalf("expected lockout cleared after reset")
	}

	locked, retryAfter, err = tracker.RegisterFailure(ctx, ip, identifier)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !locked || retryAfter == 0 {
		t.Fatalf("expected lockout after reset to behave like first failure")
	}
}
