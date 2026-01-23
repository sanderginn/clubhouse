package services

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	authRateLimitIPMaxEnv            = "AUTH_RATE_LIMIT_IP_MAX"
	authRateLimitIPWindowEnv         = "AUTH_RATE_LIMIT_IP_WINDOW"
	authRateLimitIdentifierMaxEnv    = "AUTH_RATE_LIMIT_IDENTIFIER_MAX"
	authRateLimitIdentifierWindowEnv = "AUTH_RATE_LIMIT_IDENTIFIER_WINDOW"
)

const (
	defaultAuthRateLimitIPMax         = 5
	defaultAuthRateLimitIdentifierMax = 10
)

var defaultAuthRateLimitWindow = time.Minute

// RateLimitConfig defines a simple fixed-window limit.
type RateLimitConfig struct {
	Limit  int
	Window time.Duration
}

// AuthRateLimitConfig defines limits for auth endpoints.
type AuthRateLimitConfig struct {
	IP         RateLimitConfig
	Identifier RateLimitConfig
}

// RateLimiter uses Redis to enforce a fixed-window rate limit.
type RateLimiter struct {
	redis  *redis.Client
	prefix string
	limit  int
	window time.Duration
	script *redis.Script
}

// NewRateLimiter creates a Redis-backed rate limiter.
func NewRateLimiter(redisClient *redis.Client, prefix string, config RateLimitConfig) *RateLimiter {
	return &RateLimiter{
		redis:  redisClient,
		prefix: prefix,
		limit:  config.Limit,
		window: config.Window,
		script: redis.NewScript(`
local current = redis.call("INCR", KEYS[1])
if current == 1 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return current
`),
	}
}

// Allow reports whether the key is within the rate limit.
func (l *RateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return true, nil
	}
	if l.limit <= 0 || l.window <= 0 {
		return true, nil
	}

	redisKey := l.prefix + key
	current, err := l.script.Run(ctx, l.redis, []string{redisKey}, l.window.Milliseconds()).Int()
	if err != nil {
		return false, err
	}

	return current <= l.limit, nil
}

// AuthRateLimiter enforces rate limits for auth endpoints.
type AuthRateLimiter struct {
	ipLimiter         *RateLimiter
	identifierLimiter *RateLimiter
}

// NewAuthRateLimiter creates a new auth rate limiter using environment configuration.
func NewAuthRateLimiter(redis *redis.Client) *AuthRateLimiter {
	config := loadAuthRateLimitConfig()
	return &AuthRateLimiter{
		ipLimiter:         NewRateLimiter(redis, "rate:auth:ip:", config.IP),
		identifierLimiter: NewRateLimiter(redis, "rate:auth:identifier:", config.Identifier),
	}
}

// Allow checks the IP and identifier rate limits.
func (l *AuthRateLimiter) Allow(ctx context.Context, ip string, identifiers []string) (bool, error) {
	if l == nil {
		return true, nil
	}

	if allowed, err := l.ipLimiter.Allow(ctx, ip); err != nil || !allowed {
		return allowed, err
	}

	for _, identifier := range identifiers {
		normalized := normalizeIdentifier(identifier)
		if normalized == "" {
			continue
		}
		allowed, err := l.identifierLimiter.Allow(ctx, normalized)
		if err != nil || !allowed {
			return allowed, err
		}
	}

	return true, nil
}

func normalizeIdentifier(identifier string) string {
	return strings.ToLower(strings.TrimSpace(identifier))
}

func loadAuthRateLimitConfig() AuthRateLimitConfig {
	return AuthRateLimitConfig{
		IP: RateLimitConfig{
			Limit:  readIntEnv(authRateLimitIPMaxEnv, defaultAuthRateLimitIPMax),
			Window: readDurationEnv(authRateLimitIPWindowEnv, defaultAuthRateLimitWindow),
		},
		Identifier: RateLimitConfig{
			Limit:  readIntEnv(authRateLimitIdentifierMaxEnv, defaultAuthRateLimitIdentifierMax),
			Window: readDurationEnv(authRateLimitIdentifierWindowEnv, defaultAuthRateLimitWindow),
		},
	}
}

func readIntEnv(key string, defaultValue int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}

	return parsed
}

func readDurationEnv(key string, defaultValue time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return defaultValue
	}

	return parsed
}
