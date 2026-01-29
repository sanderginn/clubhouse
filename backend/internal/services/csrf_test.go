package services

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/sanderginn/clubhouse/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRedis(t *testing.T) *redis.Client {
	client := testutil.GetTestRedis(t)

	// Clean up test data after each test
	t.Cleanup(func() {
		testutil.CleanupRedis(t)
	})

	return client
}

func TestGenerateToken(t *testing.T) {
	client := setupTestRedis(t)
	service := NewCSRFService(client)
	ctx := context.Background()

	sessionID := uuid.New().String()
	userID := uuid.New()

	token, err := service.GenerateToken(ctx, sessionID, userID)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Verify token is stored in Redis
	key := CSRFKeyPrefix + token
	value, err := client.Get(ctx, key).Result()
	require.NoError(t, err)
	expectedValue := sessionID + ":" + userID.String()
	assert.Equal(t, expectedValue, value)

	// Verify TTL is set
	ttl, err := client.TTL(ctx, key).Result()
	require.NoError(t, err)
	assert.Greater(t, ttl, time.Duration(0))
	assert.LessOrEqual(t, ttl, CSRFTokenDuration)
}

func TestValidateToken_Success(t *testing.T) {
	client := setupTestRedis(t)
	service := NewCSRFService(client)
	ctx := context.Background()

	sessionID := uuid.New().String()
	userID := uuid.New()

	// Generate token
	token, err := service.GenerateToken(ctx, sessionID, userID)
	require.NoError(t, err)

	// Validate with correct session and user
	err = service.ValidateToken(ctx, token, sessionID, userID)
	assert.NoError(t, err)
}

func TestValidateToken_EmptyToken(t *testing.T) {
	client := setupTestRedis(t)
	service := NewCSRFService(client)
	ctx := context.Background()

	sessionID := uuid.New().String()
	userID := uuid.New()

	err := service.ValidateToken(ctx, "", sessionID, userID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "csrf token is required")
}

func TestValidateToken_NotFound(t *testing.T) {
	client := setupTestRedis(t)
	service := NewCSRFService(client)
	ctx := context.Background()

	sessionID := uuid.New().String()
	userID := uuid.New()
	invalidToken := "invalid-token-that-does-not-exist"

	err := service.ValidateToken(ctx, invalidToken, sessionID, userID)
	assert.ErrorIs(t, err, ErrCSRFTokenNotFound)
}

func TestValidateToken_MismatchedSession(t *testing.T) {
	client := setupTestRedis(t)
	service := NewCSRFService(client)
	ctx := context.Background()

	sessionID := uuid.New().String()
	userID := uuid.New()

	// Generate token
	token, err := service.GenerateToken(ctx, sessionID, userID)
	require.NoError(t, err)

	// Validate with different session
	differentSessionID := uuid.New().String()
	err = service.ValidateToken(ctx, token, differentSessionID, userID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "csrf token does not match session")
}

func TestValidateToken_MismatchedUser(t *testing.T) {
	client := setupTestRedis(t)
	service := NewCSRFService(client)
	ctx := context.Background()

	sessionID := uuid.New().String()
	userID := uuid.New()

	// Generate token
	token, err := service.GenerateToken(ctx, sessionID, userID)
	require.NoError(t, err)

	// Validate with different user
	differentUserID := uuid.New()
	err = service.ValidateToken(ctx, token, sessionID, differentUserID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "csrf token does not match session")
}

func TestDeleteToken(t *testing.T) {
	client := setupTestRedis(t)
	service := NewCSRFService(client)
	ctx := context.Background()

	sessionID := uuid.New().String()
	userID := uuid.New()

	// Generate token
	token, err := service.GenerateToken(ctx, sessionID, userID)
	require.NoError(t, err)

	// Delete token
	err = service.DeleteToken(ctx, token)
	require.NoError(t, err)

	// Verify token is deleted
	key := CSRFKeyPrefix + token
	_, err = client.Get(ctx, key).Result()
	assert.Equal(t, redis.Nil, err)
}

func TestValidateToken_Expired(t *testing.T) {
	client := setupTestRedis(t)
	service := NewCSRFService(client)
	ctx := context.Background()

	sessionID := uuid.New().String()
	userID := uuid.New()

	// Generate token
	token, err := service.GenerateToken(ctx, sessionID, userID)
	require.NoError(t, err)

	// Manually delete the token to simulate expiration
	key := CSRFKeyPrefix + token
	client.Del(ctx, key)

	// Validate should fail
	err = service.ValidateToken(ctx, token, sessionID, userID)
	assert.ErrorIs(t, err, ErrCSRFTokenNotFound)
}
