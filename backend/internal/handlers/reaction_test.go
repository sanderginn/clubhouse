package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/sanderginn/clubhouse/internal/testutil"
)

func subscribeTestChannel(t *testing.T, client *redis.Client, channel string) *redis.PubSub {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)

	pubsub := client.Subscribe(ctx, channel)
	t.Cleanup(func() {
		_ = pubsub.Close()
	})

	if _, err := pubsub.Receive(ctx); err != nil {
		t.Fatalf("failed to subscribe to channel %s: %v", channel, err)
	}

	return pubsub
}

func receiveEvent(t *testing.T, pubsub *redis.PubSub) wsEvent {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)

	msg, err := pubsub.ReceiveMessage(ctx)
	if err != nil {
		t.Fatalf("failed to receive message: %v", err)
	}

	var event wsEvent
	if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
		t.Fatalf("failed to unmarshal event: %v", err)
	}

	return event
}

func TestRemoveReaction(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	// Create test data
	userID := testutil.CreateTestUser(t, db, "reactionhandleruser", "reactionhandler@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Test Section", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Test post")

	// Add a reaction first
	_, err := db.Exec(`
		INSERT INTO reactions (id, user_id, post_id, emoji, created_at)
		VALUES ($1, $2, $3, '👍', now())
	`, uuid.New(), userID, postID)
	if err != nil {
		t.Fatalf("failed to create reaction: %v", err)
	}

	handler := NewReactionHandler(db, nil, nil)

	// Test remove reaction
	req := httptest.NewRequest("DELETE", "/api/v1/posts/"+postID+"/reactions/👍", nil)
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "reactionhandleruser", false))
	w := httptest.NewRecorder()

	handler.RemoveReactionFromPost(w, req)

	if w.Code != http.StatusNoContent && w.Code != http.StatusOK {
		t.Errorf("expected status 204 or 200, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestAddReactionToPostPublishesSectionEvent(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })
	testutil.CleanupRedis(t)

	redisClient := testutil.GetTestRedis(t)

	userID := testutil.CreateTestUser(t, db, "reactionpostuser", "reactionpost@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Test Section", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Test post")

	channel := formatChannel(sectionPrefix, sectionID)
	pubsub := subscribeTestChannel(t, redisClient, channel)

	body := bytes.NewBufferString(`{"emoji":"🔥"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts/"+postID+"/reactions", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "reactionpostuser", false))
	w := httptest.NewRecorder()

	handler := NewReactionHandler(db, redisClient, nil)
	handler.AddReactionToPost(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	event := receiveEvent(t, pubsub)
	if event.Type != "reaction_added" {
		t.Fatalf("expected reaction_added event, got %s", event.Type)
	}

	dataBytes, err := json.Marshal(event.Data)
	if err != nil {
		t.Fatalf("failed to marshal event data: %v", err)
	}

	var payload reactionEventData
	if err := json.Unmarshal(dataBytes, &payload); err != nil {
		t.Fatalf("failed to unmarshal reaction payload: %v", err)
	}

	if payload.PostID == nil || payload.PostID.String() != postID {
		t.Fatalf("expected post_id %s, got %v", postID, payload.PostID)
	}
	if payload.CommentID != nil {
		t.Fatalf("expected no comment_id, got %v", payload.CommentID)
	}
}

func TestAddReactionToCommentPublishesSectionEvent(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })
	testutil.CleanupRedis(t)

	redisClient := testutil.GetTestRedis(t)

	userID := testutil.CreateTestUser(t, db, "reactioncommentuser", "reactioncomment@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Test Section", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Test post")
	commentID := testutil.CreateTestComment(t, db, userID, postID, "Test comment")

	channel := formatChannel(sectionPrefix, sectionID)
	pubsub := subscribeTestChannel(t, redisClient, channel)

	body := bytes.NewBufferString(`{"emoji":"🎉"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/comments/"+commentID+"/reactions", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "reactioncommentuser", false))
	w := httptest.NewRecorder()

	handler := NewReactionHandler(db, redisClient, nil)
	handler.AddReactionToComment(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	event := receiveEvent(t, pubsub)
	if event.Type != "reaction_added" {
		t.Fatalf("expected reaction_added event, got %s", event.Type)
	}

	dataBytes, err := json.Marshal(event.Data)
	if err != nil {
		t.Fatalf("failed to marshal event data: %v", err)
	}

	var payload reactionEventData
	if err := json.Unmarshal(dataBytes, &payload); err != nil {
		t.Fatalf("failed to unmarshal reaction payload: %v", err)
	}

	if payload.PostID == nil || payload.PostID.String() != postID {
		t.Fatalf("expected post_id %s, got %v", postID, payload.PostID)
	}
	if payload.CommentID == nil || payload.CommentID.String() != commentID {
		t.Fatalf("expected comment_id %s, got %v", commentID, payload.CommentID)
	}
}
