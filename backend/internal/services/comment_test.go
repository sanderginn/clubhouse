package services

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/testutil"
)

func TestCreateComment(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	// Create test user and section
	userID := testutil.CreateTestUser(t, db, "commentuser", "comment@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Test Section", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Test post content")

	service := NewCommentService(db)

	req := &models.CreateCommentRequest{
		PostID:  postID,
		Content: "Test comment",
	}

	comment, err := service.CreateComment(context.Background(), req, uuid.MustParse(userID))
	if err != nil {
		t.Fatalf("CreateComment failed: %v", err)
	}

	if comment.Content != "Test comment" {
		t.Errorf("expected content 'Test comment', got %s", comment.Content)
	}
	if comment.User == nil {
		t.Fatalf("expected comment user to be populated")
	}
	if comment.User.Username != "commentuser" {
		t.Errorf("expected username 'commentuser', got %s", comment.User.Username)
	}
}

func TestGetCommentByID(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	// Create test data
	userID := testutil.CreateTestUser(t, db, "getcommentuser", "getcomment@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Test Section", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Test post")
	commentID := testutil.CreateTestComment(t, db, userID, postID, "Test comment content")

	service := NewCommentService(db)

	comment, err := service.GetCommentByID(context.Background(), uuid.MustParse(commentID), uuid.MustParse(userID))
	if err != nil {
		t.Fatalf("GetCommentByID failed: %v", err)
	}

	if comment.Content != "Test comment content" {
		t.Errorf("expected content 'Test comment content', got %s", comment.Content)
	}
}

func TestValidateCreateCommentInput(t *testing.T) {
	tests := []struct {
		name    string
		req     *models.CreateCommentRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid comment",
			req: &models.CreateCommentRequest{
				PostID:  uuid.New().String(),
				Content: "This is a comment",
			},
			wantErr: false,
		},
		{
			name: "missing post_id",
			req: &models.CreateCommentRequest{
				Content: "This is a comment",
			},
			wantErr: true,
			errMsg:  "post_id is required",
		},
		{
			name: "empty content",
			req: &models.CreateCommentRequest{
				PostID:  uuid.New().String(),
				Content: "",
			},
			wantErr: true,
			errMsg:  "content is required",
		},
		{
			name: "content too long",
			req: &models.CreateCommentRequest{
				PostID:  uuid.New().String(),
				Content: string(make([]byte, 5001)),
			},
			wantErr: true,
			errMsg:  "content must be less than 5000 characters",
		},
		{
			name: "empty link url",
			req: &models.CreateCommentRequest{
				PostID:  uuid.New().String(),
				Content: "Check this out",
				Links: []models.LinkRequest{
					{URL: ""},
				},
			},
			wantErr: true,
			errMsg:  "link url cannot be empty",
		},
		{
			name: "link url too long",
			req: &models.CreateCommentRequest{
				PostID:  uuid.New().String(),
				Content: "Check this out",
				Links: []models.LinkRequest{
					{URL: string(make([]byte, 2049))},
				},
			},
			wantErr: true,
			errMsg:  "link url must be less than 2048 characters",
		},
		{
			name: "valid comment with optional parent",
			req: &models.CreateCommentRequest{
				PostID:          uuid.New().String(),
				ParentCommentID: stringPtr(uuid.New().String()),
				Content:         "Reply to comment",
			},
			wantErr: false,
		},
		{
			name: "valid comment with links",
			req: &models.CreateCommentRequest{
				PostID:  uuid.New().String(),
				Content: "Check this out",
				Links: []models.LinkRequest{
					{URL: "https://example.com"},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCreateCommentInput(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateCreateCommentInput() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err.Error() != tt.errMsg {
				t.Errorf("validateCreateCommentInput() error message = %q, want %q", err.Error(), tt.errMsg)
			}
		})
	}
}

func stringPtr(s string) *string {
	return &s
}
