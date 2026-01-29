package services

import (
	"context"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func TestCreateSessionTracksUserSession(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	service := NewSessionService(client)

	ctx := context.Background()
	userID := uuid.New()

	session, err := service.CreateSession(ctx, userID, "tester", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	userKey := UserSessionSetPrefix + userID.String()
	isMember, err := client.SIsMember(ctx, userKey, session.ID).Result()
	if err != nil {
		t.Fatalf("unexpected redis error: %v", err)
	}
	if !isMember {
		t.Fatalf("expected session to be tracked in user session set")
	}
}

func TestDeleteSessionRemovesFromUserSet(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	service := NewSessionService(client)

	ctx := context.Background()
	userID := uuid.New()

	session, err := service.CreateSession(ctx, userID, "tester", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := service.DeleteSession(ctx, session.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	userKey := UserSessionSetPrefix + userID.String()
	isMember, err := client.SIsMember(ctx, userKey, session.ID).Result()
	if err != nil {
		t.Fatalf("unexpected redis error: %v", err)
	}
	if isMember {
		t.Fatalf("expected session to be removed from user session set")
	}
}

func TestDeleteAllSessionsForUser(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	service := NewSessionService(client)

	ctx := context.Background()
	userID := uuid.New()
	otherUserID := uuid.New()

	session1, err := service.CreateSession(ctx, userID, "tester", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	session2, err := service.CreateSession(ctx, userID, "tester", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	otherSession, err := service.CreateSession(ctx, otherUserID, "other", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := service.DeleteAllSessionsForUser(ctx, userID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := service.GetSession(ctx, session1.ID); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected session1 to be deleted, got %v", err)
	}
	if _, err := service.GetSession(ctx, session2.ID); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected session2 to be deleted, got %v", err)
	}
	if _, err := service.GetSession(ctx, otherSession.ID); err != nil {
		t.Fatalf("expected other session to remain, got %v", err)
	}

	userKey := UserSessionSetPrefix + userID.String()
	if exists, err := client.Exists(ctx, userKey).Result(); err != nil {
		t.Fatalf("unexpected redis error: %v", err)
	} else if exists != 0 {
		t.Fatalf("expected user session set to be deleted")
	}
}

func TestUpdateUserAdminStatusUpdatesSessions(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	service := NewSessionService(client)

	ctx := context.Background()
	userID := uuid.New()
	otherUserID := uuid.New()

	session, err := service.CreateSession(ctx, userID, "member", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	otherSession, err := service.CreateSession(ctx, otherUserID, "other", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := service.UpdateUserAdminStatus(ctx, userID, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated, err := service.GetSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated.IsAdmin {
		t.Fatalf("expected session to be updated to admin")
	}

	otherUpdated, err := service.GetSession(ctx, otherSession.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if otherUpdated.IsAdmin {
		t.Fatalf("expected other session to remain non-admin")
	}
}
