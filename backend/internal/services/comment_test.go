package services

import (
	"context"
	"encoding/json"
	"strings"
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

func TestCreateCommentWithImageID(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "commentimageuser", "commentimage@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Image Section", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Post with image")

	imageID := uuid.New()
	_, err := db.Exec(`
		INSERT INTO post_images (id, post_id, image_url, position, caption, alt_text, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, now())
	`, imageID, uuid.MustParse(postID), "https://example.com/test.jpg", 1, "Caption", "Alt text")
	if err != nil {
		t.Fatalf("failed to create post image: %v", err)
	}

	service := NewCommentService(db)
	req := &models.CreateCommentRequest{
		PostID:  postID,
		Content: "Comment with image",
		ImageID: stringPtr(imageID.String()),
	}

	comment, err := service.CreateComment(context.Background(), req, uuid.MustParse(userID))
	if err != nil {
		t.Fatalf("CreateComment failed: %v", err)
	}

	if comment.ImageID == nil {
		t.Fatalf("expected image_id to be set")
	}
	if comment.ImageID.String() != imageID.String() {
		t.Fatalf("expected image_id %s, got %s", imageID.String(), comment.ImageID.String())
	}
}

func TestCreateCommentCreatesAuditLog(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "createcommentuser", "createcomment@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Create Comment Section", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Post for create comment audit")

	imageID := uuid.New()
	_, err := db.Exec(`
		INSERT INTO post_images (id, post_id, image_url, position, caption, alt_text, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, now())
	`, imageID, uuid.MustParse(postID), "https://example.com/audit.jpg", 1, "Caption", "Alt text")
	if err != nil {
		t.Fatalf("failed to create post image: %v", err)
	}

	service := NewCommentService(db)
	req := &models.CreateCommentRequest{
		PostID:  postID,
		Content: "Create comment audit log",
		ImageID: stringPtr(imageID.String()),
	}

	comment, err := service.CreateComment(context.Background(), req, uuid.MustParse(userID))
	if err != nil {
		t.Fatalf("CreateComment failed: %v", err)
	}

	var metadataBytes []byte
	err = db.QueryRow(`
		SELECT metadata
		FROM audit_logs
		WHERE admin_user_id = $1 AND action = 'create_comment'
	`, userID).Scan(&metadataBytes)
	if err != nil {
		t.Fatalf("failed to query audit log: %v", err)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}
	if metadata["comment_id"] != comment.ID.String() {
		t.Errorf("expected comment_id %s, got %v", comment.ID.String(), metadata["comment_id"])
	}
	if metadata["post_id"] != postID {
		t.Errorf("expected post_id %s, got %v", postID, metadata["post_id"])
	}
	if metadata["section_id"] != sectionID {
		t.Errorf("expected section_id %s, got %v", sectionID, metadata["section_id"])
	}
	if metadata["image_id"] != imageID.String() {
		t.Errorf("expected image_id %s, got %v", imageID.String(), metadata["image_id"])
	}
	if metadata["content_excerpt"] != "Create comment audit log" {
		t.Errorf("expected content_excerpt %q, got %v", "Create comment audit log", metadata["content_excerpt"])
	}
}

func TestCreateCommentInvalidImageID(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "commentinvalidimage", "commentinvalidimage@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Invalid Image Section", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Post for invalid image test")

	service := NewCommentService(db)
	req := &models.CreateCommentRequest{
		PostID:  postID,
		Content: "Comment with invalid image id",
		ImageID: stringPtr("not-a-uuid"),
	}

	_, err := service.CreateComment(context.Background(), req, uuid.MustParse(userID))
	if err == nil {
		t.Fatalf("expected error for invalid image id")
	}
	if err.Error() != "invalid image id" {
		t.Fatalf("expected error %q, got %q", "invalid image id", err.Error())
	}
}

func TestCreateCommentImageNotFound(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "commentimage404", "commentimage404@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Image 404 Section", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Post for missing image test")

	service := NewCommentService(db)
	req := &models.CreateCommentRequest{
		PostID:  postID,
		Content: "Comment with missing image",
		ImageID: stringPtr(uuid.New().String()),
	}

	_, err := service.CreateComment(context.Background(), req, uuid.MustParse(userID))
	if err == nil {
		t.Fatalf("expected error for missing image")
	}
	if err.Error() != "image not found" {
		t.Fatalf("expected error %q, got %q", "image not found", err.Error())
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

func TestUpdateCommentCreatesAuditLogWithMetadata(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "updatecommentuser", "updatecomment@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Update Comment Section", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Post for comment update")
	commentID := testutil.CreateTestComment(t, db, userID, postID, "Original comment content")

	service := NewCommentService(db)
	req := &models.UpdateCommentRequest{
		Content: "Updated comment content",
	}

	_, err := service.UpdateComment(context.Background(), uuid.MustParse(commentID), uuid.MustParse(userID), req)
	if err != nil {
		t.Fatalf("UpdateComment failed: %v", err)
	}

	var metadataBytes []byte
	err = db.QueryRow(`
		SELECT metadata
		FROM audit_logs
		WHERE admin_user_id = $1 AND action = 'update_comment'
	`, userID).Scan(&metadataBytes)
	if err != nil {
		t.Fatalf("failed to query audit log: %v", err)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}
	if metadata["comment_id"] != commentID {
		t.Errorf("expected comment_id %s, got %v", commentID, metadata["comment_id"])
	}
	if metadata["post_id"] != postID {
		t.Errorf("expected post_id %s, got %v", postID, metadata["post_id"])
	}
	if metadata["section_id"] != sectionID {
		t.Errorf("expected section_id %s, got %v", sectionID, metadata["section_id"])
	}
	if metadata["previous_content"] != "Original comment content" {
		t.Errorf("expected previous_content %q, got %v", "Original comment content", metadata["previous_content"])
	}
	if metadata["content_excerpt"] != "Updated comment content" {
		t.Errorf("expected content_excerpt %q, got %v", "Updated comment content", metadata["content_excerpt"])
	}
	linksChanged, ok := metadata["links_changed"].(bool)
	if !ok {
		t.Fatalf("expected links_changed to be bool, got %T", metadata["links_changed"])
	}
	if linksChanged {
		t.Errorf("expected links_changed false, got %v", linksChanged)
	}
	linksProvided, ok := metadata["links_provided"].(bool)
	if !ok {
		t.Fatalf("expected links_provided to be bool, got %T", metadata["links_provided"])
	}
	if linksProvided {
		t.Errorf("expected links_provided false, got %v", linksProvided)
	}
}

func TestAdminDeleteCommentCreatesAuditLogWithMetadata(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "commentmoduser", "commentmoduser@test.com", false, true)
	adminID := testutil.CreateTestUser(t, db, "commentmodadmin", "commentmodadmin@test.com", true, true)
	sectionID := testutil.CreateTestSection(t, db, "Comment Section", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Post for comment moderation")
	content := strings.Repeat("c", 140)
	commentID := testutil.CreateTestComment(t, db, userID, postID, content)

	service := NewCommentService(db)
	_, err := service.DeleteComment(context.Background(), uuid.MustParse(commentID), uuid.MustParse(adminID), true)
	if err != nil {
		t.Fatalf("DeleteComment failed: %v", err)
	}

	var relatedCommentID uuid.UUID
	var metadataBytes []byte
	err = db.QueryRow(`
		SELECT related_comment_id, metadata
		FROM audit_logs
		WHERE admin_user_id = $1 AND action = 'delete_comment'
	`, adminID).Scan(&relatedCommentID, &metadataBytes)
	if err != nil {
		t.Fatalf("failed to query audit log: %v", err)
	}
	if relatedCommentID.String() != commentID {
		t.Errorf("expected related_comment_id %s, got %s", commentID, relatedCommentID.String())
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}
	if metadata["comment_id"] != commentID {
		t.Errorf("expected comment_id %s, got %v", commentID, metadata["comment_id"])
	}
	if metadata["post_id"] != postID {
		t.Errorf("expected post_id %s, got %v", postID, metadata["post_id"])
	}
	if metadata["deleted_by_user_id"] != adminID {
		t.Errorf("expected deleted_by_user_id %s, got %v", adminID, metadata["deleted_by_user_id"])
	}
	isSelfDelete, ok := metadata["is_self_delete"].(bool)
	if !ok {
		t.Fatalf("expected is_self_delete to be bool, got %T", metadata["is_self_delete"])
	}
	if isSelfDelete {
		t.Errorf("expected is_self_delete false, got %v", metadata["is_self_delete"])
	}
	deletedByAdmin, ok := metadata["deleted_by_admin"].(bool)
	if !ok {
		t.Fatalf("expected deleted_by_admin to be bool, got %T", metadata["deleted_by_admin"])
	}
	if !deletedByAdmin {
		t.Errorf("expected deleted_by_admin true, got %v", metadata["deleted_by_admin"])
	}
	excerpt, ok := metadata["content_excerpt"].(string)
	if !ok {
		t.Fatalf("expected content_excerpt to be string, got %T", metadata["content_excerpt"])
	}
	if len([]rune(excerpt)) != auditExcerptLimit {
		t.Errorf("expected content_excerpt length %d, got %d", auditExcerptLimit, len([]rune(excerpt)))
	}
}

func TestDeleteCommentOwnerCreatesAuditLogWithMetadata(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "commentowner", "commentowner@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Self Delete Comment Section", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Post for comment self delete")
	commentID := testutil.CreateTestComment(t, db, userID, postID, "Owner delete comment")

	service := NewCommentService(db)
	_, err := service.DeleteComment(context.Background(), uuid.MustParse(commentID), uuid.MustParse(userID), false)
	if err != nil {
		t.Fatalf("DeleteComment failed: %v", err)
	}

	var metadataBytes []byte
	err = db.QueryRow(`
		SELECT metadata
		FROM audit_logs
		WHERE admin_user_id = $1 AND action = 'delete_comment' AND related_comment_id = $2
	`, userID, commentID).Scan(&metadataBytes)
	if err != nil {
		t.Fatalf("failed to query audit log: %v", err)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}
	if metadata["comment_id"] != commentID {
		t.Errorf("expected comment_id %s, got %v", commentID, metadata["comment_id"])
	}
	if metadata["post_id"] != postID {
		t.Errorf("expected post_id %s, got %v", postID, metadata["post_id"])
	}
	if metadata["deleted_by_user_id"] != userID {
		t.Errorf("expected deleted_by_user_id %s, got %v", userID, metadata["deleted_by_user_id"])
	}
	isSelfDelete, ok := metadata["is_self_delete"].(bool)
	if !ok {
		t.Fatalf("expected is_self_delete to be bool, got %T", metadata["is_self_delete"])
	}
	if !isSelfDelete {
		t.Errorf("expected is_self_delete true, got %v", metadata["is_self_delete"])
	}
}

func TestAdminRestoreCommentCreatesAuditLogWithMetadata(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "commentrestoreuser", "commentrestoreuser@test.com", false, true)
	adminID := testutil.CreateTestUser(t, db, "commentrestoreadmin", "commentrestoreadmin@test.com", true, true)
	sectionID := testutil.CreateTestSection(t, db, "Restore Comment Section", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Post for comment restore")
	commentID := testutil.CreateTestComment(t, db, userID, postID, "Comment to restore")

	service := NewCommentService(db)
	_, err := service.DeleteComment(context.Background(), uuid.MustParse(commentID), uuid.MustParse(userID), false)
	if err != nil {
		t.Fatalf("DeleteComment failed: %v", err)
	}

	_, err = service.AdminRestoreComment(context.Background(), uuid.MustParse(commentID), uuid.MustParse(adminID))
	if err != nil {
		t.Fatalf("AdminRestoreComment failed: %v", err)
	}

	var metadataBytes []byte
	err = db.QueryRow(`
		SELECT metadata
		FROM audit_logs
		WHERE admin_user_id = $1 AND action = 'restore_comment' AND related_comment_id = $2
	`, adminID, commentID).Scan(&metadataBytes)
	if err != nil {
		t.Fatalf("failed to query audit log: %v", err)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}
	if metadata["comment_id"] != commentID {
		t.Errorf("expected comment_id %s, got %v", commentID, metadata["comment_id"])
	}
	if metadata["restored_by_user_id"] != adminID {
		t.Errorf("expected restored_by_user_id %s, got %v", adminID, metadata["restored_by_user_id"])
	}
}

func stringPtr(s string) *string {
	return &s
}
