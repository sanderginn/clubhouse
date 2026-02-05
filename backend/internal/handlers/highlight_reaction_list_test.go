package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/testutil"
	"github.com/stretchr/testify/require"
)

func TestGetHighlightReactions(t *testing.T) {
	db := testutil.RequireTestDB(t)
	handler := NewHighlightReactionHandler(db, nil)

	userID := testutil.CreateTestUser(t, db, "highlightreact", "highlightreact@test.com", false, true)
	reactorID := testutil.CreateTestUser(t, db, "reactor", "reactor@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Highlight Section", "music")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Highlight post")
	linkID := createTestLink(t, db, postID, "https://example.com/test")

	highlight := models.Highlight{Timestamp: 30, Label: "Drop"}
	metadata := map[string]interface{}{"highlights": []models.Highlight{highlight}}
	payload, err := json.Marshal(metadata)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE links SET metadata = $1 WHERE id = $2`, payload, linkID)
	require.NoError(t, err)

	highlightID, err := models.EncodeHighlightID(uuid.MustParse(linkID), highlight)
	require.NoError(t, err)

	if _, err = db.Exec(`
		INSERT INTO highlight_reactions (id, user_id, link_id, highlight_id, created_at)
		VALUES (gen_random_uuid(), $1, $2, $3, now())
	`, reactorID, linkID, highlightID); err != nil {
		t.Fatalf("failed to insert highlight reaction: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/posts/"+postID+"/highlights/"+highlightID+"/reactions", nil)
	w := httptest.NewRecorder()

	handler.GetHighlightReactions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var response models.GetReactionsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Reactions) != 1 {
		t.Fatalf("expected 1 reaction group, got %d", len(response.Reactions))
	}

	group := response.Reactions[0]
	if group.Emoji != "❤️" {
		t.Fatalf("expected heart emoji, got %q", group.Emoji)
	}
	if len(group.Users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(group.Users))
	}
	if group.Users[0].ID.String() != reactorID {
		t.Fatalf("expected user %s, got %s", reactorID, group.Users[0].ID.String())
	}
}

func createTestLink(t *testing.T, db *sql.DB, postID, url string) string {
	t.Helper()
	var id string
	query := `INSERT INTO links (id, post_id, url, created_at) VALUES (gen_random_uuid(), $1, $2, now()) RETURNING id`
	err := db.QueryRow(query, postID, url).Scan(&id)
	require.NoError(t, err)
	return id
}
