package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/testutil"
)

func TestGetPostReactions(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "reactionlistuser1", "reactionlist1@test.com", false, true)
	user2ID := testutil.CreateTestUser(t, db, "reactionlistuser2", "reactionlist2@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Reactions Section", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Test post")

	_, err := db.Exec(`
		INSERT INTO reactions (id, user_id, post_id, emoji, created_at)
		VALUES ($1, $2, $3, $4, now()), ($5, $6, $7, $8, now())
	`,
		uuid.New(), userID, postID, "👍",
		uuid.New(), user2ID, postID, "👍",
	)
	if err != nil {
		t.Fatalf("failed to create reactions: %v", err)
	}

	handler := NewReactionHandler(db, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/posts/"+postID+"/reactions", nil)
	w := httptest.NewRecorder()

	handler.GetPostReactions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response models.GetReactionsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Reactions) != 1 {
		t.Fatalf("expected 1 reaction group, got %d", len(response.Reactions))
	}

	group := response.Reactions[0]
	if group.Emoji != "👍" {
		t.Fatalf("expected emoji 👍, got %s", group.Emoji)
	}
	if len(group.Users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(group.Users))
	}
}

func TestGetCommentReactions(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "reactionlistuser3", "reactionlist3@test.com", false, true)
	user2ID := testutil.CreateTestUser(t, db, "reactionlistuser4", "reactionlist4@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Reactions Section 2", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Test post")
	commentID := testutil.CreateTestComment(t, db, userID, postID, "Test comment")

	_, err := db.Exec(`
		INSERT INTO reactions (id, user_id, comment_id, emoji, created_at)
		VALUES ($1, $2, $3, $4, now()), ($5, $6, $7, $8, now())
	`,
		uuid.New(), userID, commentID, "🔥",
		uuid.New(), user2ID, commentID, "🔥",
	)
	if err != nil {
		t.Fatalf("failed to create reactions: %v", err)
	}

	handler := NewReactionHandler(db, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/comments/"+commentID+"/reactions", nil)
	w := httptest.NewRecorder()

	handler.GetCommentReactions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response models.GetReactionsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Reactions) != 1 {
		t.Fatalf("expected 1 reaction group, got %d", len(response.Reactions))
	}

	group := response.Reactions[0]
	if group.Emoji != "🔥" {
		t.Fatalf("expected emoji 🔥, got %s", group.Emoji)
	}
	if len(group.Users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(group.Users))
	}
}
