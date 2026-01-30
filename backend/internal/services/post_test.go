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

func TestCreatePostWithoutLinks(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	disableLinkMetadata(t)

	userID := testutil.CreateTestUser(t, db, "postuser", "postuser@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Test Section", "general")

	service := NewPostService(db)
	req := &models.CreatePostRequest{
		SectionID: sectionID,
		Content:   "Hello world",
	}

	post, err := service.CreatePost(context.Background(), req, uuid.MustParse(userID))
	if err != nil {
		t.Fatalf("CreatePost failed: %v", err)
	}

	if post.Content != "Hello world" {
		t.Errorf("expected content 'Hello world', got %s", post.Content)
	}
	if post.SectionID.String() != sectionID {
		t.Errorf("expected section_id %s, got %s", sectionID, post.SectionID.String())
	}
	if len(post.Links) != 0 {
		t.Errorf("expected no links, got %d", len(post.Links))
	}
}

func TestCreatePostWithLinks(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	disableLinkMetadata(t)

	userID := testutil.CreateTestUser(t, db, "postlinkuser", "postlinkuser@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Links Section", "general")

	service := NewPostService(db)
	req := &models.CreatePostRequest{
		SectionID: sectionID,
		Content:   "Check this out",
		Links: []models.LinkRequest{
			{URL: "https://example.com"},
		},
	}

	post, err := service.CreatePost(context.Background(), req, uuid.MustParse(userID))
	if err != nil {
		t.Fatalf("CreatePost failed: %v", err)
	}

	if len(post.Links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(post.Links))
	}
	if post.Links[0].URL != "https://example.com" {
		t.Errorf("expected link url https://example.com, got %s", post.Links[0].URL)
	}

	var metadataIsNull bool
	err = db.QueryRow(`SELECT metadata IS NULL FROM links WHERE post_id = $1`, post.ID).Scan(&metadataIsNull)
	if err != nil {
		t.Fatalf("failed to query link metadata: %v", err)
	}
	if !metadataIsNull {
		t.Errorf("expected metadata to be NULL when link metadata is disabled")
	}
}

func TestCreatePostWithLinksNoContent(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	disableLinkMetadata(t)

	userID := testutil.CreateTestUser(t, db, "postlinknocontent", "postlinknocontent@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Links Only Section", "general")

	service := NewPostService(db)
	req := &models.CreatePostRequest{
		SectionID: sectionID,
		Content:   "   ",
		Links: []models.LinkRequest{
			{URL: "https://example.com"},
		},
	}

	post, err := service.CreatePost(context.Background(), req, uuid.MustParse(userID))
	if err != nil {
		t.Fatalf("CreatePost failed: %v", err)
	}

	if post.Content != "" {
		t.Errorf("expected empty content, got %q", post.Content)
	}
	if len(post.Links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(post.Links))
	}
}

func TestCreatePostRequiresContentOrLinks(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	disableLinkMetadata(t)

	userID := testutil.CreateTestUser(t, db, "postempty", "postempty@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Empty Section", "general")

	service := NewPostService(db)
	req := &models.CreatePostRequest{
		SectionID: sectionID,
		Content:   "   ",
	}

	_, err := service.CreatePost(context.Background(), req, uuid.MustParse(userID))
	if err == nil {
		t.Fatalf("expected error for empty content without links")
	}
	if err.Error() != "content is required" {
		t.Fatalf("expected error %q, got %q", "content is required", err.Error())
	}
}

func TestDeletePostOwner(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "deleteowner", "deleteowner@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Delete Section", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Owner post")

	service := NewPostService(db)
	post, err := service.DeletePost(context.Background(), uuid.MustParse(postID), uuid.MustParse(userID), false)
	if err != nil {
		t.Fatalf("DeletePost failed: %v", err)
	}

	if post.DeletedAt == nil {
		t.Fatalf("expected deleted_at to be set")
	}
	if post.DeletedByUserID == nil || post.DeletedByUserID.String() != userID {
		t.Errorf("expected deleted_by_user_id %s, got %v", userID, post.DeletedByUserID)
	}
}

func TestDeletePostAdmin(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "deleteuser", "deleteuser@test.com", false, true)
	adminID := testutil.CreateTestUser(t, db, "deleteadmin", "deleteadmin@test.com", true, true)
	sectionID := testutil.CreateTestSection(t, db, "Admin Delete Section", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Admin delete post")

	service := NewPostService(db)
	post, err := service.DeletePost(context.Background(), uuid.MustParse(postID), uuid.MustParse(adminID), true)
	if err != nil {
		t.Fatalf("DeletePost failed: %v", err)
	}

	if post.DeletedAt == nil {
		t.Fatalf("expected deleted_at to be set")
	}
	if post.DeletedByUserID == nil || post.DeletedByUserID.String() != adminID {
		t.Errorf("expected deleted_by_user_id %s, got %v", adminID, post.DeletedByUserID)
	}
}

func TestRestorePostOwner(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "restoreowner", "restoreowner@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Restore Section", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Restore post")

	service := NewPostService(db)
	_, err := service.DeletePost(context.Background(), uuid.MustParse(postID), uuid.MustParse(userID), false)
	if err != nil {
		t.Fatalf("DeletePost failed: %v", err)
	}

	post, err := service.RestorePost(context.Background(), uuid.MustParse(postID), uuid.MustParse(userID), false)
	if err != nil {
		t.Fatalf("RestorePost failed: %v", err)
	}

	if post.DeletedAt != nil {
		t.Fatalf("expected deleted_at to be cleared")
	}
	if post.DeletedByUserID != nil {
		t.Fatalf("expected deleted_by_user_id to be cleared")
	}
}

func TestAdminRestorePostCreatesAuditLog(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "adminrestoreuser", "adminrestoreuser@test.com", false, true)
	adminID := testutil.CreateTestUser(t, db, "adminrestore", "adminrestore@test.com", true, true)
	sectionID := testutil.CreateTestSection(t, db, "Admin Restore Section", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Admin restore post")

	service := NewPostService(db)
	_, err := service.DeletePost(context.Background(), uuid.MustParse(postID), uuid.MustParse(userID), false)
	if err != nil {
		t.Fatalf("DeletePost failed: %v", err)
	}

	_, err = service.AdminRestorePost(context.Background(), uuid.MustParse(postID), uuid.MustParse(adminID))
	if err != nil {
		t.Fatalf("AdminRestorePost failed: %v", err)
	}

	var count int
	var metadataBytes []byte
	err = db.QueryRow(`
		SELECT COUNT(*), metadata
		FROM audit_logs
		WHERE admin_user_id = $1 AND action = 'restore_post' AND related_post_id = $2
		GROUP BY metadata
	`, adminID, postID).Scan(&count, &metadataBytes)
	if err != nil {
		t.Fatalf("failed to query audit log: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 audit log entry, got %d", count)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}
	if metadata["restored_by_user_id"] != adminID {
		t.Errorf("expected restored_by_user_id %s, got %v", adminID, metadata["restored_by_user_id"])
	}
	if metadata["post_id"] != postID {
		t.Errorf("expected post_id %s, got %v", postID, metadata["post_id"])
	}
}

func TestHardDeletePostCreatesAuditLog(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "harddeleteuser", "harddeleteuser@test.com", false, true)
	adminID := testutil.CreateTestUser(t, db, "harddeleteadmin", "harddeleteadmin@test.com", true, true)
	sectionID := testutil.CreateTestSection(t, db, "Hard Delete Section", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Hard delete post")

	service := NewPostService(db)
	if err := service.HardDeletePost(context.Background(), uuid.MustParse(postID), uuid.MustParse(adminID)); err != nil {
		t.Fatalf("HardDeletePost failed: %v", err)
	}

	var postCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM posts WHERE id = $1`, postID).Scan(&postCount); err != nil {
		t.Fatalf("failed to query post: %v", err)
	}
	if postCount != 0 {
		t.Errorf("expected post to be deleted, found %d rows", postCount)
	}

	var auditCount int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM audit_logs
		WHERE admin_user_id = $1 AND action = 'hard_delete_post'
	`, adminID).Scan(&auditCount); err != nil {
		t.Fatalf("failed to query audit log: %v", err)
	}
	if auditCount != 1 {
		t.Errorf("expected 1 audit log entry, got %d", auditCount)
	}
}

func TestAdminDeletePostCreatesAuditLogWithMetadata(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "moderateduser", "moderateduser@test.com", false, true)
	adminID := testutil.CreateTestUser(t, db, "moderatoradmin", "moderatoradmin@test.com", true, true)
	sectionID := testutil.CreateTestSection(t, db, "Moderation Section", "general")
	content := strings.Repeat("a", 150)
	postID := testutil.CreateTestPost(t, db, userID, sectionID, content)

	service := NewPostService(db)
	_, err := service.DeletePost(context.Background(), uuid.MustParse(postID), uuid.MustParse(adminID), true)
	if err != nil {
		t.Fatalf("DeletePost failed: %v", err)
	}

	var relatedPostID uuid.UUID
	var metadataBytes []byte
	err = db.QueryRow(`
		SELECT related_post_id, metadata
		FROM audit_logs
		WHERE admin_user_id = $1 AND action = 'delete_post'
	`, adminID).Scan(&relatedPostID, &metadataBytes)
	if err != nil {
		t.Fatalf("failed to query audit log: %v", err)
	}
	if relatedPostID.String() != postID {
		t.Errorf("expected related_post_id %s, got %s", postID, relatedPostID.String())
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}
	if metadata["post_id"] != postID {
		t.Errorf("expected post_id %s, got %v", postID, metadata["post_id"])
	}
	if metadata["section_id"] != sectionID {
		t.Errorf("expected section_id %s, got %v", sectionID, metadata["section_id"])
	}
	if metadata["deleted_by_user_id"] != adminID {
		t.Errorf("expected deleted_by_user_id %s, got %v", adminID, metadata["deleted_by_user_id"])
	}
	excerpt, ok := metadata["content_excerpt"].(string)
	if !ok {
		t.Fatalf("expected content_excerpt to be string, got %T", metadata["content_excerpt"])
	}
	if len([]rune(excerpt)) != auditExcerptLimit {
		t.Errorf("expected content_excerpt length %d, got %d", auditExcerptLimit, len([]rune(excerpt)))
	}
}

func disableLinkMetadata(t *testing.T) {
	t.Helper()
	config := GetConfigService()
	current := config.GetConfig().LinkMetadataEnabled
	disabled := false
	if _, err := config.UpdateConfig(context.Background(), &disabled, nil); err != nil {
		t.Fatalf("failed to disable link metadata: %v", err)
	}
	t.Cleanup(func() {
		if _, err := config.UpdateConfig(context.Background(), &current, nil); err != nil {
			t.Fatalf("failed to restore link metadata: %v", err)
		}
	})
}
