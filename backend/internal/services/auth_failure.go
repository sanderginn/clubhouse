package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	authFailedLoginThresholdEnv   = "AUTH_FAILED_LOGIN_THRESHOLD"
	authFailedLoginWindowEnv      = "AUTH_FAILED_LOGIN_WINDOW"
	authFailedLoginBaseLockoutEnv = "AUTH_FAILED_LOGIN_BASE_LOCKOUT"
	authFailedLoginMaxLockoutEnv  = "AUTH_FAILED_LOGIN_MAX_LOCKOUT"
)

const (
	defaultAuthFailedLoginThreshold = 5
)

var (
	defaultAuthFailedLoginWindow      = 30 * time.Minute
	defaultAuthFailedLoginBaseLockout = 30 * time.Second
	defaultAuthFailedLoginMaxLockout  = 15 * time.Minute
)

// AuthFailureConfig controls failed login tracking behavior.
type AuthFailureConfig struct {
	Threshold   int
	Window      time.Duration
	BaseLockout time.Duration
	MaxLockout  time.Duration
}

// AuthFailureTracker tracks failed login attempts and enforces lockouts.
type AuthFailureTracker struct {
	redis  *redis.Client
	config AuthFailureConfig
	now    func() time.Time
}

// NewAuthFailureTracker creates a new failed-login tracker using environment configuration.
func NewAuthFailureTracker(redis *redis.Client) *AuthFailureTracker {
	return &AuthFailureTracker{
		redis:  redis,
		config: loadAuthFailureConfig(),
		now:    time.Now,
	}
}

// IsLocked reports whether the identifier/IP pair is currently locked out.
func (t *AuthFailureTracker) IsLocked(ctx context.Context, ip string, identifiers []string) (bool, time.Duration, error) {
	if t == nil || t.redis == nil {
		return false, 0, nil
	}

	normalizedIP := normalizeIP(ip)
	if normalizedIP == "" {
		return false, 0, nil
	}

	locked := false
	var maxRetry time.Duration
	for _, identifier := range identifiers {
		key := t.lockoutKey(identifier, normalizedIP)
		if key == "" {
			continue
		}
		ttl, err := t.redis.PTTL(ctx, key).Result()
		if err != nil {
			return false, 0, err
		}
		if ttl == time.Duration(-1) {
			locked = true
			continue
		}
		if ttl > 0 {
			locked = true
			if ttl > maxRetry {
				maxRetry = ttl
			}
		}
	}

	return locked, maxRetry, nil
}

// RegisterFailure increments the failure count and applies a lockout when needed.
func (t *AuthFailureTracker) RegisterFailure(ctx context.Context, ip string, identifiers []string) (bool, time.Duration, error) {
	if t == nil || t.redis == nil {
		return false, 0, nil
	}

	normalizedIP := normalizeIP(ip)
	if normalizedIP == "" {
		return false, 0, nil
	}

	var locked bool
	var maxRetry time.Duration
	for _, identifier := range identifiers {
		countKey := t.countKey(identifier, normalizedIP)
		lockoutKey := t.lockoutKey(identifier, normalizedIP)
		if countKey == "" || lockoutKey == "" {
			continue
		}

		count, err := t.redis.Incr(ctx, countKey).Result()
		if err != nil {
			return false, 0, err
		}

		if t.config.Window > 0 {
			if err := t.redis.Expire(ctx, countKey, t.config.Window).Err(); err != nil {
				return false, 0, err
			}
		}

		if int(count) < t.config.Threshold || t.config.Threshold <= 0 {
			continue
		}

		lockoutDuration := t.lockoutDuration(int(count))
		if lockoutDuration <= 0 {
			continue
		}

		if err := t.redis.Set(ctx, lockoutKey, t.now().Unix(), lockoutDuration).Err(); err != nil {
			return false, 0, err
		}

		locked = true
		if lockoutDuration > maxRetry {
			maxRetry = lockoutDuration
		}
	}

	return locked, maxRetry, nil
}

// Reset clears failure counts and lockouts for the identifier/IP pair.
func (t *AuthFailureTracker) Reset(ctx context.Context, ip string, identifiers []string) error {
	if t == nil || t.redis == nil {
		return nil
	}

	normalizedIP := normalizeIP(ip)
	if normalizedIP == "" {
		return nil
	}

	keys := make([]string, 0, len(identifiers)*2)
	for _, identifier := range identifiers {
		countKey := t.countKey(identifier, normalizedIP)
		lockoutKey := t.lockoutKey(identifier, normalizedIP)
		if countKey == "" || lockoutKey == "" {
			continue
		}
		keys = append(keys, countKey, lockoutKey)
	}

	if len(keys) == 0 {
		return nil
	}

	return t.redis.Del(ctx, keys...).Err()
}

func (t *AuthFailureTracker) countKey(identifier, ip string) string {
	normalized := normalizeIdentifier(identifier)
	if normalized == "" || ip == "" {
		return ""
	}
	return fmt.Sprintf("auth:failed:%s:%s", normalized, ip)
}

func (t *AuthFailureTracker) lockoutKey(identifier, ip string) string {
	normalized := normalizeIdentifier(identifier)
	if normalized == "" || ip == "" {
		return ""
	}
	return fmt.Sprintf("auth:lockout:%s:%s", normalized, ip)
}

func (t *AuthFailureTracker) lockoutDuration(failureCount int) time.Duration {
	if t.config.Threshold <= 0 || failureCount < t.config.Threshold {
		return 0
	}

	base := t.config.BaseLockout
	if base <= 0 {
		return 0
	}

	steps := failureCount - t.config.Threshold
	lockout := base
	for i := 0; i < steps; i++ {
		lockout *= 2
		if t.config.MaxLockout > 0 && lockout >= t.config.MaxLockout {
			return t.config.MaxLockout
		}
	}

	if t.config.MaxLockout > 0 && lockout > t.config.MaxLockout {
		return t.config.MaxLockout
	}

	return lockout
}

func normalizeIP(ip string) string {
	return strings.TrimSpace(ip)
}

func loadAuthFailureConfig() AuthFailureConfig {
	return AuthFailureConfig{
		Threshold:   readIntEnv(authFailedLoginThresholdEnv, defaultAuthFailedLoginThreshold),
		Window:      readDurationEnv(authFailedLoginWindowEnv, defaultAuthFailedLoginWindow),
		BaseLockout: readDurationEnv(authFailedLoginBaseLockoutEnv, defaultAuthFailedLoginBaseLockout),
		MaxLockout:  readDurationEnv(authFailedLoginMaxLockoutEnv, defaultAuthFailedLoginMaxLockout),
	}
}
