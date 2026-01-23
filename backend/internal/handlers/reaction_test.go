package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/testutil"
)

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
