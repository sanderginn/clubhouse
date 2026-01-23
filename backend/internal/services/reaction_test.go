package services

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/testutil"
)

func TestAddReactionToPost(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "reactionuser", "reaction@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Test Section", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Test post")

	service := NewReactionService(db)

	_, err := service.AddReactionToPost(context.Background(), uuid.MustParse(postID), uuid.MustParse(userID), "👍")
	if err != nil {
		t.Fatalf("AddReactionToPost failed: %v", err)
	}

	// Adding same reaction again should not error (upsert)
	_, err = service.AddReactionToPost(context.Background(), uuid.MustParse(postID), uuid.MustParse(userID), "👍")
	if err != nil {
		t.Fatalf("AddReactionToPost (duplicate) failed: %v", err)
	}
}

func TestRemoveReaction(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "removereactionuser", "removereaction@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Test Section", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Test post")

	service := NewReactionService(db)

	// Add then remove
	_, err := service.AddReactionToPost(context.Background(), uuid.MustParse(postID), uuid.MustParse(userID), "👍")
	if err != nil {
		t.Fatalf("AddReactionToPost failed: %v", err)
	}

	err = service.RemoveReactionFromPost(context.Background(), uuid.MustParse(postID), "👍", uuid.MustParse(userID))
	if err != nil {
		t.Fatalf("RemoveReactionFromPost failed: %v", err)
	}
}

func TestValidateEmoji(t *testing.T) {
	tests := []struct {
		name    string
		emoji   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid emoji",
			emoji:   "👍",
			wantErr: false,
		},
		{
			name:    "valid text emoji",
			emoji:   ":thumbsup:",
			wantErr: false,
		},
		{
			name:    "empty emoji",
			emoji:   "",
			wantErr: true,
			errMsg:  "emoji is required",
		},
		{
			name:    "whitespace only",
			emoji:   "   ",
			wantErr: true,
			errMsg:  "emoji is required",
		},
		{
			name:    "emoji too long",
			emoji:   "12345678901",
			wantErr: true,
			errMsg:  "emoji must be 10 characters or less",
		},
		{
			name:    "emoji at max length",
			emoji:   "1234567890",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEmoji(tt.emoji)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateEmoji() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err.Error() != tt.errMsg {
				t.Errorf("validateEmoji() error message = %q, want %q", err.Error(), tt.errMsg)
			}
		})
	}
}
