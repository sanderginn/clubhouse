package handlers

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"github.com/sanderginn/clubhouse/internal/middleware"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/services"
	"github.com/sanderginn/clubhouse/internal/testutil"
)

// createTestUserContext creates a context with user session for testing
func createTestUserContext(ctx context.Context, userID uuid.UUID, username string, isAdmin bool) context.Context {
	session := &services.Session{
		UserID:   userID,
		Username: username,
		IsAdmin:  isAdmin,
	}
	return context.WithValue(ctx, middleware.UserContextKey, session)
}

// TestGetProfileSuccess tests successfully retrieving a user profile
func TestGetProfileSuccess(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.New()
	testUsername := "profileuser"
	testEmail := "profile@example.com"
	testHash := "$2a$12$test"

	query := `
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, $2, $3, $4, false, now(), now())
	`
	_, err := db.Exec(query, userID, testUsername, testEmail, testHash)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	handler := NewUserHandler(db)

	req := httptest.NewRequest("GET", "/api/v1/users/"+userID.String(), nil)
	w := httptest.NewRecorder()

	handler.GetProfile(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response models.UserProfileResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.ID != userID {
		t.Errorf("expected user ID %s, got %s", userID, response.ID)
	}

	if response.Username != testUsername {
		t.Errorf("expected username %s, got %s", testUsername, response.Username)
	}

	if response.Stats.PostCount != 0 {
		t.Errorf("expected post count 0, got %d", response.Stats.PostCount)
	}

	if response.Stats.CommentCount != 0 {
		t.Errorf("expected comment count 0, got %d", response.Stats.CommentCount)
	}
}

// TestGetProfileNotFound tests 404 for non-existent user
func TestGetProfileNotFound(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	handler := NewUserHandler(db)
	randomID := uuid.New()

	req := httptest.NewRequest("GET", "/api/v1/users/"+randomID.String(), nil)
	w := httptest.NewRecorder()

	handler.GetProfile(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.Code != "USER_NOT_FOUND" {
		t.Errorf("expected code USER_NOT_FOUND, got %s", response.Code)
	}
}

// TestGetProfileInvalidID tests with invalid user ID format
func TestGetProfileInvalidID(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	handler := NewUserHandler(db)

	req := httptest.NewRequest("GET", "/api/v1/users/not-a-uuid", nil)
	w := httptest.NewRecorder()

	handler.GetProfile(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.Code != "INVALID_USER_ID" {
		t.Errorf("expected code INVALID_USER_ID, got %s", response.Code)
	}
}

func TestAutocompleteUsers(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	testutil.CreateTestUser(t, db, "alice", "alice@example.com", false, true)
	testutil.CreateTestUser(t, db, "alex", "alex@example.com", false, true)
	testutil.CreateTestUser(t, db, "bob", "bob@example.com", false, true)
	testutil.CreateTestUser(t, db, "pendinguser", "pending@example.com", false, false)

	handler := NewUserHandler(db)
	req := httptest.NewRequest("GET", "/api/v1/users/autocomplete?q=al&limit=5", nil)
	w := httptest.NewRecorder()

	handler.AutocompleteUsers(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response models.UserAutocompleteResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Users) < 2 {
		t.Fatalf("expected at least 2 users, got %d", len(response.Users))
	}

	found := map[string]bool{}
	for _, user := range response.Users {
		found[user.Username] = true
	}
	if !found["alice"] || !found["alex"] {
		t.Fatalf("expected users alice and alex, got %+v", response.Users)
	}
}

func TestLookupUserByUsername(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.MustParse(testutil.CreateTestUser(t, db, "Sander", "sander@example.com", false, true))
	handler := NewUserHandler(db)

	req := httptest.NewRequest("GET", "/api/v1/users/lookup?username=sander", nil)
	w := httptest.NewRecorder()

	handler.LookupUserByUsername(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response models.UserLookupResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.User.ID != userID {
		t.Errorf("expected user ID %s, got %s", userID, response.User.ID)
	}
	if response.User.Username != "Sander" {
		t.Errorf("expected username Sander, got %s", response.User.Username)
	}
}

// TestGetProfileMethodNotAllowed tests with non-GET method
func TestGetProfileMethodNotAllowed(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	handler := NewUserHandler(db)
	userID := uuid.New()

	req := httptest.NewRequest("POST", "/api/v1/users/"+userID.String(), nil)
	w := httptest.NewRecorder()

	handler.GetProfile(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.Code != "METHOD_NOT_ALLOWED" {
		t.Errorf("expected code METHOD_NOT_ALLOWED, got %s", response.Code)
	}
}

// TestGetProfileSoftDeletedUser tests that soft-deleted users are hidden
func TestGetProfileSoftDeletedUser(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.New()
	testUsername := "deleteduser"
	testEmail := "deleted@example.com"
	testHash := "$2a$12$test"

	query := `
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at, deleted_at)
		VALUES ($1, $2, $3, $4, false, now(), now(), now())
	`
	_, err := db.Exec(query, userID, testUsername, testEmail, testHash)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	handler := NewUserHandler(db)

	req := httptest.NewRequest("GET", "/api/v1/users/"+userID.String(), nil)
	w := httptest.NewRecorder()

	handler.GetProfile(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.Code != "USER_NOT_FOUND" {
		t.Errorf("expected code USER_NOT_FOUND, got %s", response.Code)
	}
}

// TestGetProfileUnapprovedUser tests that unapproved users are hidden
func TestGetProfileUnapprovedUser(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.New()
	testUsername := "unapproveduser"
	testEmail := "unapproved@example.com"
	testHash := "$2a$12$test"

	query := `
		INSERT INTO users (id, username, email, password_hash, is_admin, created_at)
		VALUES ($1, $2, $3, $4, false, now())
	`
	_, err := db.Exec(query, userID, testUsername, testEmail, testHash)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	handler := NewUserHandler(db)

	req := httptest.NewRequest("GET", "/api/v1/users/"+userID.String(), nil)
	w := httptest.NewRecorder()

	handler.GetProfile(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// TestGetUserPostsSuccess tests successfully retrieving a user's posts
func TestGetUserPostsSuccess(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	// Create test user
	userID := uuid.New()
	testUsername := "postuser"
	testEmail := "postuser@example.com"
	testHash := "$2a$12$test"

	userQuery := `
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, $2, $3, $4, false, now(), now())
	`
	_, err := db.Exec(userQuery, userID, testUsername, testEmail, testHash)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Create test section
	sectionID := uuid.New()
	sectionQuery := `INSERT INTO sections (id, name, type, created_at) VALUES ($1, 'Test Section', 'general', now())`
	_, err = db.Exec(sectionQuery, sectionID)
	if err != nil {
		t.Fatalf("failed to create test section: %v", err)
	}

	// Create test posts
	postID1 := uuid.New()
	postID2 := uuid.New()
	postQuery := `INSERT INTO posts (id, user_id, section_id, content, created_at) VALUES ($1, $2, $3, $4, $5)`
	_, err = db.Exec(postQuery, postID1, userID, sectionID, "Test post 1", time.Now().Add(-2*time.Minute))
	if err != nil {
		t.Fatalf("failed to create test post 1: %v", err)
	}
	_, err = db.Exec(postQuery, postID2, userID, sectionID, "Test post 2", time.Now().Add(-1*time.Minute))
	if err != nil {
		t.Fatalf("failed to create test post 2: %v", err)
	}

	imageID := uuid.New()
	imageQuery := `
		INSERT INTO post_images (id, post_id, image_url, position, caption, alt_text, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, now())
	`
	_, err = db.Exec(imageQuery, imageID, postID1, "https://example.com/test-image.jpg", 1, "Caption", "Alt text")
	if err != nil {
		t.Fatalf("failed to create test post image: %v", err)
	}

	handler := NewUserHandler(db)

	req := httptest.NewRequest("GET", "/api/v1/users/"+userID.String()+"/posts", nil)
	w := httptest.NewRecorder()

	handler.GetUserPosts(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response models.FeedResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if len(response.Posts) != 2 {
		t.Errorf("expected 2 posts, got %d", len(response.Posts))
	}

	if response.HasMore {
		t.Errorf("expected has_more to be false")
	}

	postImages := map[uuid.UUID][]models.PostImage{}
	for _, post := range response.Posts {
		postImages[post.ID] = post.Images
	}

	if images := postImages[postID1]; len(images) != 1 {
		t.Errorf("expected 1 image for post %s, got %d", postID1, len(images))
	}

	if images := postImages[postID2]; len(images) != 0 {
		t.Errorf("expected 0 images for post %s, got %d", postID2, len(images))
	}
}

// TestGetUserPostsEmptyResult tests that non-existent user returns empty posts list
func TestGetUserPostsEmptyResult(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	handler := NewUserHandler(db)
	randomID := uuid.New()

	req := httptest.NewRequest("GET", "/api/v1/users/"+randomID.String()+"/posts", nil)
	w := httptest.NewRecorder()

	handler.GetUserPosts(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response models.FeedResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if len(response.Posts) != 0 {
		t.Errorf("expected 0 posts, got %d", len(response.Posts))
	}
}

// TestGetUserPostsInvalidID tests with invalid user ID format
func TestGetUserPostsInvalidID(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	handler := NewUserHandler(db)

	req := httptest.NewRequest("GET", "/api/v1/users/not-a-uuid/posts", nil)
	w := httptest.NewRecorder()

	handler.GetUserPosts(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.Code != "INVALID_USER_ID" {
		t.Errorf("expected code INVALID_USER_ID, got %s", response.Code)
	}
}

// TestGetUserPostsMethodNotAllowed tests with non-GET method
func TestGetUserPostsMethodNotAllowed(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	handler := NewUserHandler(db)
	userID := uuid.New()

	req := httptest.NewRequest("POST", "/api/v1/users/"+userID.String()+"/posts", nil)
	w := httptest.NewRecorder()

	handler.GetUserPosts(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.Code != "METHOD_NOT_ALLOWED" {
		t.Errorf("expected code METHOD_NOT_ALLOWED, got %s", response.Code)
	}
}

// TestGetUserPostsExcludesSoftDeleted tests that soft-deleted posts are excluded
func TestGetUserPostsExcludesSoftDeleted(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	// Create test user
	userID := uuid.New()
	testUsername := "softdeletepostuser"
	testEmail := "softdeletepost@example.com"
	testHash := "$2a$12$test"

	userQuery := `
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, $2, $3, $4, false, now(), now())
	`
	_, err := db.Exec(userQuery, userID, testUsername, testEmail, testHash)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Create test section
	sectionID := uuid.New()
	sectionQuery := `INSERT INTO sections (id, name, type, created_at) VALUES ($1, 'Test Section', 'general', now())`
	_, err = db.Exec(sectionQuery, sectionID)
	if err != nil {
		t.Fatalf("failed to create test section: %v", err)
	}

	// Create normal post
	normalPostID := uuid.New()
	postQuery := `INSERT INTO posts (id, user_id, section_id, content, created_at) VALUES ($1, $2, $3, $4, now())`
	_, err = db.Exec(postQuery, normalPostID, userID, sectionID, "Normal post")
	if err != nil {
		t.Fatalf("failed to create normal post: %v", err)
	}

	// Create soft-deleted post
	deletedPostID := uuid.New()
	deletedPostQuery := `INSERT INTO posts (id, user_id, section_id, content, created_at, deleted_at) VALUES ($1, $2, $3, $4, now(), now())`
	_, err = db.Exec(deletedPostQuery, deletedPostID, userID, sectionID, "Deleted post")
	if err != nil {
		t.Fatalf("failed to create deleted post: %v", err)
	}

	handler := NewUserHandler(db)

	req := httptest.NewRequest("GET", "/api/v1/users/"+userID.String()+"/posts", nil)
	w := httptest.NewRecorder()

	handler.GetUserPosts(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response models.FeedResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	// Only the non-deleted post should be returned
	if len(response.Posts) != 1 {
		t.Errorf("expected 1 post (excluding soft-deleted), got %d", len(response.Posts))
	}

	if response.Posts[0].ID != normalPostID {
		t.Errorf("expected normal post ID %s, got %s", normalPostID, response.Posts[0].ID)
	}
}

// TestGetUserCommentsSuccess tests successfully retrieving user comments
func TestGetUserCommentsSuccess(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	// Create test user
	userID := uuid.New()
	testUsername := "commentuser"
	testEmail := "commentuser@example.com"
	testHash := "$2a$12$test"

	userQuery := `
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, $2, $3, $4, false, now(), now())
	`
	_, err := db.Exec(userQuery, userID, testUsername, testEmail, testHash)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Create a section and post for the comments
	sectionID := uuid.New()
	_, err = db.Exec(`INSERT INTO sections (id, name, type, created_at) VALUES ($1, 'Test', 'general', now())`, sectionID)
	if err != nil {
		t.Fatalf("failed to create test section: %v", err)
	}

	postID := uuid.New()
	_, err = db.Exec(`INSERT INTO posts (id, user_id, section_id, content, created_at) VALUES ($1, $2, $3, 'Test post', now())`, postID, userID, sectionID)
	if err != nil {
		t.Fatalf("failed to create test post: %v", err)
	}

	// Create test comments
	commentID1 := uuid.New()
	commentID2 := uuid.New()
	_, err = db.Exec(`INSERT INTO comments (id, user_id, post_id, content, created_at) VALUES ($1, $2, $3, 'Comment 1', now())`, commentID1, userID, postID)
	if err != nil {
		t.Fatalf("failed to create test comment 1: %v", err)
	}
	_, err = db.Exec(`INSERT INTO comments (id, user_id, post_id, content, created_at) VALUES ($1, $2, $3, 'Comment 2', now())`, commentID2, userID, postID)
	if err != nil {
		t.Fatalf("failed to create test comment 2: %v", err)
	}

	handler := NewUserHandler(db)

	req := httptest.NewRequest("GET", "/api/v1/users/"+userID.String()+"/comments", nil)
	w := httptest.NewRecorder()

	handler.GetUserComments(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response models.GetThreadResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if len(response.Comments) != 2 {
		t.Errorf("expected 2 comments, got %d", len(response.Comments))
	}

	if response.Meta.HasMore {
		t.Errorf("expected has_more to be false, got true")
	}
}

// TestGetUserCommentsNotFound tests 404 for non-existent user
func TestGetUserCommentsNotFound(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	handler := NewUserHandler(db)
	randomID := uuid.New()

	req := httptest.NewRequest("GET", "/api/v1/users/"+randomID.String()+"/comments", nil)
	w := httptest.NewRecorder()

	handler.GetUserComments(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.Code != "USER_NOT_FOUND" {
		t.Errorf("expected code USER_NOT_FOUND, got %s", response.Code)
	}
}

// TestGetUserCommentsInvalidID tests with invalid user ID format
func TestGetUserCommentsInvalidID(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	handler := NewUserHandler(db)

	req := httptest.NewRequest("GET", "/api/v1/users/not-a-uuid/comments", nil)
	w := httptest.NewRecorder()

	handler.GetUserComments(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.Code != "INVALID_USER_ID" {
		t.Errorf("expected code INVALID_USER_ID, got %s", response.Code)
	}
}

// TestGetUserCommentsMethodNotAllowed tests with non-GET method
func TestGetUserCommentsMethodNotAllowed(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	handler := NewUserHandler(db)
	userID := uuid.New()

	req := httptest.NewRequest("POST", "/api/v1/users/"+userID.String()+"/comments", nil)
	w := httptest.NewRecorder()

	handler.GetUserComments(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.Code != "METHOD_NOT_ALLOWED" {
		t.Errorf("expected code METHOD_NOT_ALLOWED, got %s", response.Code)
	}
}

// TestGetUserCommentsExcludesSoftDeleted tests that soft-deleted comments are excluded
func TestGetUserCommentsExcludesSoftDeleted(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	// Create test user
	userID := uuid.New()
	testUsername := "softdeleteuser"
	testEmail := "softdeleteuser@example.com"
	testHash := "$2a$12$test"

	userQuery := `
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, $2, $3, $4, false, now(), now())
	`
	_, err := db.Exec(userQuery, userID, testUsername, testEmail, testHash)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Create a section and post for the comments
	sectionID := uuid.New()
	_, err = db.Exec(`INSERT INTO sections (id, name, type, created_at) VALUES ($1, 'Test', 'general', now())`, sectionID)
	if err != nil {
		t.Fatalf("failed to create test section: %v", err)
	}

	postID := uuid.New()
	_, err = db.Exec(`INSERT INTO posts (id, user_id, section_id, content, created_at) VALUES ($1, $2, $3, 'Test post', now())`, postID, userID, sectionID)
	if err != nil {
		t.Fatalf("failed to create test post: %v", err)
	}

	// Create active comment
	activeCommentID := uuid.New()
	_, err = db.Exec(`INSERT INTO comments (id, user_id, post_id, content, created_at) VALUES ($1, $2, $3, 'Active comment', now())`, activeCommentID, userID, postID)
	if err != nil {
		t.Fatalf("failed to create active comment: %v", err)
	}

	// Create soft-deleted comment
	deletedCommentID := uuid.New()
	_, err = db.Exec(`INSERT INTO comments (id, user_id, post_id, content, created_at, deleted_at) VALUES ($1, $2, $3, 'Deleted comment', now(), now())`, deletedCommentID, userID, postID)
	if err != nil {
		t.Fatalf("failed to create deleted comment: %v", err)
	}

	handler := NewUserHandler(db)

	req := httptest.NewRequest("GET", "/api/v1/users/"+userID.String()+"/comments", nil)
	w := httptest.NewRecorder()

	handler.GetUserComments(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response models.GetThreadResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	// Only the active comment should be returned
	if len(response.Comments) != 1 {
		t.Errorf("expected 1 comment (excluding soft-deleted), got %d", len(response.Comments))
	}

	if response.Comments[0].ID != activeCommentID {
		t.Errorf("expected active comment ID %s, got %s", activeCommentID, response.Comments[0].ID)
	}
}

// TestUpdateMeSuccess tests successfully updating user profile
func TestUpdateMeSuccess(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.New()
	testUsername := "updatemeuser"
	testEmail := "updateme@example.com"
	testHash := "$2a$12$test"

	query := `
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, $2, $3, $4, false, now(), now())
	`
	_, err := db.Exec(query, userID, testUsername, testEmail, testHash)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	handler := NewUserHandler(db)

	// Create request with bio and profile picture URL
	reqBody := `{"bio": "My new bio", "profile_picture_url": "https://example.com/image.png"}`
	req := httptest.NewRequest("PATCH", "/api/v1/users/me", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	// Add user context (simulating auth middleware)
	ctx := createTestUserContext(req.Context(), userID, testUsername, false)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.UpdateMe(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response models.UpdateUserResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.ID != userID {
		t.Errorf("expected user ID %s, got %s", userID, response.ID)
	}

	if response.Bio == nil || *response.Bio != "My new bio" {
		t.Errorf("expected bio 'My new bio', got %v", response.Bio)
	}

	if response.ProfilePictureUrl == nil || *response.ProfilePictureUrl != "https://example.com/image.png" {
		t.Errorf("expected profile_picture_url 'https://example.com/image.png', got %v", response.ProfilePictureUrl)
	}

	// Verify changes in database
	var bio, profilePictureUrl sql.NullString
	err = db.QueryRow("SELECT bio, profile_picture_url FROM users WHERE id = $1", userID).Scan(&bio, &profilePictureUrl)
	if err != nil {
		t.Fatalf("failed to query user: %v", err)
	}

	if !bio.Valid || bio.String != "My new bio" {
		t.Errorf("expected bio 'My new bio' in DB, got %v", bio)
	}

	if !profilePictureUrl.Valid || profilePictureUrl.String != "https://example.com/image.png" {
		t.Errorf("expected profile_picture_url 'https://example.com/image.png' in DB, got %v", profilePictureUrl)
	}
}

// TestUpdateMeBioOnly tests updating only the bio
func TestUpdateMeBioOnly(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.New()
	testUsername := "bioonlyuser"
	testEmail := "bioonly@example.com"
	testHash := "$2a$12$test"

	query := `
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, $2, $3, $4, false, now(), now())
	`
	_, err := db.Exec(query, userID, testUsername, testEmail, testHash)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	handler := NewUserHandler(db)

	reqBody := `{"bio": "Only bio update"}`
	req := httptest.NewRequest("PATCH", "/api/v1/users/me", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	ctx := createTestUserContext(req.Context(), userID, testUsername, false)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.UpdateMe(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response models.UpdateUserResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.Bio == nil || *response.Bio != "Only bio update" {
		t.Errorf("expected bio 'Only bio update', got %v", response.Bio)
	}
}

// TestUpdateMeInvalidURL tests updating with invalid profile picture URL
func TestUpdateMeInvalidURL(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.New()
	testUsername := "invalidurluser"
	testEmail := "invalidurl@example.com"
	testHash := "$2a$12$test"

	query := `
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, $2, $3, $4, false, now(), now())
	`
	_, err := db.Exec(query, userID, testUsername, testEmail, testHash)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	handler := NewUserHandler(db)

	reqBody := `{"profile_picture_url": "not-a-valid-url"}`
	req := httptest.NewRequest("PATCH", "/api/v1/users/me", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	ctx := createTestUserContext(req.Context(), userID, testUsername, false)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.UpdateMe(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.Code != "INVALID_URL_SCHEME" {
		t.Errorf("expected code INVALID_URL_SCHEME, got %s", response.Code)
	}
}

// TestUpdateMeEmptyBody tests updating with empty request body
func TestUpdateMeEmptyBody(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.New()
	testUsername := "emptybodyuser"
	testEmail := "emptybody@example.com"
	testHash := "$2a$12$test"

	query := `
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, $2, $3, $4, false, now(), now())
	`
	_, err := db.Exec(query, userID, testUsername, testEmail, testHash)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	handler := NewUserHandler(db)

	reqBody := `{}`
	req := httptest.NewRequest("PATCH", "/api/v1/users/me", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	ctx := createTestUserContext(req.Context(), userID, testUsername, false)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.UpdateMe(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.Code != "INVALID_REQUEST" {
		t.Errorf("expected code INVALID_REQUEST, got %s", response.Code)
	}
}

// TestUpdateMeMethodNotAllowed tests with non-PATCH method
func TestUpdateMeMethodNotAllowed(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.New()

	handler := NewUserHandler(db)

	req := httptest.NewRequest("GET", "/api/v1/users/me", nil)
	ctx := createTestUserContext(req.Context(), userID, "testuser", false)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.UpdateMe(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.Code != "METHOD_NOT_ALLOWED" {
		t.Errorf("expected code METHOD_NOT_ALLOWED, got %s", response.Code)
	}
}

// TestUpdateMeNoAuth tests UpdateMe without authentication
func TestUpdateMeNoAuth(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	handler := NewUserHandler(db)

	reqBody := `{"bio": "Test bio"}`
	req := httptest.NewRequest("PATCH", "/api/v1/users/me", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	// No context with user

	w := httptest.NewRecorder()
	handler.UpdateMe(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusUnauthorized, w.Code, w.Body.String())
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.Code != "UNAUTHORIZED" {
		t.Errorf("expected code UNAUTHORIZED, got %s", response.Code)
	}
}

func TestGetMySectionSubscriptionsSuccess(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.New()
	sectionID := uuid.New()

	_, err := db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'sectionuser', 'sectionuser@example.com', '$2a$12$test', false, now(), now())
	`, userID)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO sections (id, name, type, created_at)
		VALUES ($1, 'Test Section', 'general', now())
	`, sectionID)
	if err != nil {
		t.Fatalf("failed to create test section: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO section_subscriptions (user_id, section_id, opted_out_at)
		VALUES ($1, $2, now())
	`, userID, sectionID)
	if err != nil {
		t.Fatalf("failed to create section subscription: %v", err)
	}

	handler := NewUserHandler(db)

	req := httptest.NewRequest("GET", "/api/v1/users/me/section-subscriptions", nil)
	ctx := createTestUserContext(req.Context(), userID, "sectionuser", false)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.GetMySectionSubscriptions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response models.GetSectionSubscriptionsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if len(response.SectionSubscriptions) != 1 {
		t.Fatalf("expected 1 section subscription, got %d", len(response.SectionSubscriptions))
	}

	if response.SectionSubscriptions[0].SectionID != sectionID {
		t.Errorf("expected section ID %s, got %s", sectionID, response.SectionSubscriptions[0].SectionID)
	}
}

func TestUpdateMySectionSubscriptionOptOut(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.New()
	sectionID := uuid.New()

	_, err := db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'optoutuser', 'optoutuser@example.com', '$2a$12$test', false, now(), now())
	`, userID)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO sections (id, name, type, created_at)
		VALUES ($1, 'OptOut Section', 'general', now())
	`, sectionID)
	if err != nil {
		t.Fatalf("failed to create test section: %v", err)
	}

	handler := NewUserHandler(db)

	reqBody := `{"opted_out": true}`
	req := httptest.NewRequest("PATCH", "/api/v1/users/me/section-subscriptions/"+sectionID.String(), strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	ctx := createTestUserContext(req.Context(), userID, "optoutuser", false)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.UpdateMySectionSubscription(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response models.UpdateSectionSubscriptionResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.SectionID != sectionID {
		t.Errorf("expected section ID %s, got %s", sectionID, response.SectionID)
	}
	if !response.OptedOut || response.OptedOutAt == nil {
		t.Errorf("expected opted_out true with timestamp, got opted_out=%v opted_out_at=%v", response.OptedOut, response.OptedOutAt)
	}

	var exists bool
	if err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM section_subscriptions WHERE user_id = $1 AND section_id = $2
		)
	`, userID, sectionID).Scan(&exists); err != nil {
		t.Fatalf("failed to check section subscription: %v", err)
	}
	if !exists {
		t.Fatalf("expected section subscription to exist")
	}
}

func TestUpdateMySectionSubscriptionOptIn(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.New()
	sectionID := uuid.New()

	_, err := db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'optinuser', 'optinuser@example.com', '$2a$12$test', false, now(), now())
	`, userID)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO sections (id, name, type, created_at)
		VALUES ($1, 'OptIn Section', 'general', now())
	`, sectionID)
	if err != nil {
		t.Fatalf("failed to create test section: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO section_subscriptions (user_id, section_id, opted_out_at)
		VALUES ($1, $2, now())
	`, userID, sectionID)
	if err != nil {
		t.Fatalf("failed to create section subscription: %v", err)
	}

	handler := NewUserHandler(db)

	reqBody := `{"opted_out": false}`
	req := httptest.NewRequest("PATCH", "/api/v1/users/me/section-subscriptions/"+sectionID.String(), strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	ctx := createTestUserContext(req.Context(), userID, "optinuser", false)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.UpdateMySectionSubscription(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response models.UpdateSectionSubscriptionResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.OptedOut {
		t.Errorf("expected opted_out false, got true")
	}
	if response.OptedOutAt != nil {
		t.Errorf("expected opted_out_at nil, got %v", response.OptedOutAt)
	}

	var exists bool
	if err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM section_subscriptions WHERE user_id = $1 AND section_id = $2
		)
	`, userID, sectionID).Scan(&exists); err != nil {
		t.Fatalf("failed to check section subscription: %v", err)
	}
	if exists {
		t.Fatalf("expected section subscription to be removed")
	}
}

func TestUpdateMySectionSubscriptionMissingOptedOut(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.New()
	sectionID := uuid.New()

	_, err := db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'missingoptuser', 'missingoptuser@example.com', '$2a$12$test', false, now(), now())
	`, userID)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO sections (id, name, type, created_at)
		VALUES ($1, 'Missing Opt Section', 'general', now())
	`, sectionID)
	if err != nil {
		t.Fatalf("failed to create test section: %v", err)
	}

	handler := NewUserHandler(db)

	reqBody := `{}`
	req := httptest.NewRequest("PATCH", "/api/v1/users/me/section-subscriptions/"+sectionID.String(), strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	ctx := createTestUserContext(req.Context(), userID, "missingoptuser", false)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.UpdateMySectionSubscription(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.Code != "INVALID_REQUEST" {
		t.Errorf("expected code INVALID_REQUEST, got %s", response.Code)
	}
}

func TestUserMFAEnrollVerifyDisable(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	keyBytes := make([]byte, 32)
	for i := range keyBytes {
		keyBytes[i] = byte(i + 1)
	}
	t.Setenv("CLUBHOUSE_TOTP_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(keyBytes))

	userID := uuid.New()
	_, err := db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'totpuser', 'totpuser@example.com', '$2a$12$test', false, now(), now())
	`, userID)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	handler := NewUserHandler(db)

	enrollReq := httptest.NewRequest(http.MethodPost, "/api/v1/users/me/mfa/enable", nil)
	enrollReq = enrollReq.WithContext(createTestUserContext(enrollReq.Context(), userID, "totpuser", false))
	enrollRes := httptest.NewRecorder()

	handler.EnrollMFA(enrollRes, enrollReq)

	if enrollRes.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusOK, enrollRes.Code, enrollRes.Body.String())
	}

	var enrollBody models.TOTPEnrollResponse
	if err := json.NewDecoder(enrollRes.Body).Decode(&enrollBody); err != nil {
		t.Fatalf("failed to decode enroll response: %v", err)
	}

	if enrollBody.Secret == "" || enrollBody.OtpAuthURL == "" {
		t.Fatalf("expected secret and otpauth url to be set")
	}

	var enrollMetadataBytes []byte
	if err := db.QueryRow(`
		SELECT metadata
		FROM audit_logs
		WHERE admin_user_id = $1 AND action = 'enroll_mfa'
		ORDER BY created_at DESC
		LIMIT 1
	`, userID).Scan(&enrollMetadataBytes); err != nil {
		t.Fatalf("failed to query enroll audit log: %v", err)
	}

	var enrollMetadata map[string]interface{}
	if err := json.Unmarshal(enrollMetadataBytes, &enrollMetadata); err != nil {
		t.Fatalf("failed to unmarshal enroll metadata: %v", err)
	}
	if enrollMetadata["method"] != "totp" {
		t.Errorf("expected enroll method 'totp', got %v", enrollMetadata["method"])
	}

	code, err := totp.GenerateCode(enrollBody.Secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("failed to generate totp code: %v", err)
	}

	verifyPayload, err := json.Marshal(models.TOTPVerifyRequest{Code: code})
	if err != nil {
		t.Fatalf("failed to marshal verify payload: %v", err)
	}

	verifyReq := httptest.NewRequest(http.MethodPost, "/api/v1/users/me/mfa/verify", strings.NewReader(string(verifyPayload)))
	verifyReq = verifyReq.WithContext(createTestUserContext(verifyReq.Context(), userID, "totpuser", false))
	verifyRes := httptest.NewRecorder()

	handler.VerifyMFA(verifyRes, verifyReq)

	if verifyRes.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusOK, verifyRes.Code, verifyRes.Body.String())
	}

	var verifyBody models.TOTPVerifyResponse
	if err := json.NewDecoder(verifyRes.Body).Decode(&verifyBody); err != nil {
		t.Fatalf("failed to decode verify response: %v", err)
	}
	if verifyBody.Message == "" {
		t.Fatalf("expected verify message to be set")
	}
	if len(verifyBody.BackupCodes) == 0 {
		t.Fatalf("expected backup codes to be returned")
	}

	var backupCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mfa_backup_codes WHERE user_id = $1`, userID).Scan(&backupCount); err != nil {
		t.Fatalf("failed to query backup codes: %v", err)
	}
	if backupCount != len(verifyBody.BackupCodes) {
		t.Fatalf("expected %d backup codes, got %d", len(verifyBody.BackupCodes), backupCount)
	}

	var enabled bool
	var secret []byte
	if err := db.QueryRow("SELECT totp_enabled, totp_secret_encrypted FROM users WHERE id = $1", userID).Scan(&enabled, &secret); err != nil {
		t.Fatalf("failed to query totp settings: %v", err)
	}
	if !enabled {
		t.Fatalf("expected totp_enabled to be true")
	}
	if len(secret) == 0 {
		t.Fatalf("expected totp secret to be stored")
	}

	var enableMetadataBytes []byte
	if err := db.QueryRow(`
		SELECT metadata
		FROM audit_logs
		WHERE admin_user_id = $1 AND action = 'enable_mfa'
		ORDER BY created_at DESC
		LIMIT 1
	`, userID).Scan(&enableMetadataBytes); err != nil {
		t.Fatalf("failed to query enable audit log: %v", err)
	}

	var enableMetadata map[string]interface{}
	if err := json.Unmarshal(enableMetadataBytes, &enableMetadata); err != nil {
		t.Fatalf("failed to unmarshal enable metadata: %v", err)
	}
	if enableMetadata["method"] != "totp" {
		t.Errorf("expected enable method 'totp', got %v", enableMetadata["method"])
	}

	disableCode, err := totp.GenerateCode(enrollBody.Secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("failed to generate totp code: %v", err)
	}

	disablePayload, err := json.Marshal(models.TOTPVerifyRequest{Code: disableCode})
	if err != nil {
		t.Fatalf("failed to marshal disable payload: %v", err)
	}

	disableReq := httptest.NewRequest(http.MethodPost, "/api/v1/users/me/mfa/disable", strings.NewReader(string(disablePayload)))
	disableReq = disableReq.WithContext(createTestUserContext(disableReq.Context(), userID, "totpuser", false))
	disableRes := httptest.NewRecorder()

	handler.DisableMFA(disableRes, disableReq)

	if disableRes.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusOK, disableRes.Code, disableRes.Body.String())
	}

	if err := db.QueryRow("SELECT totp_enabled, totp_secret_encrypted FROM users WHERE id = $1", userID).Scan(&enabled, &secret); err != nil {
		t.Fatalf("failed to query totp settings: %v", err)
	}
	if enabled {
		t.Fatalf("expected totp_enabled to be false")
	}
	if len(secret) != 0 {
		t.Fatalf("expected totp secret to be cleared")
	}

	var disableMetadataBytes []byte
	if err := db.QueryRow(`
		SELECT metadata
		FROM audit_logs
		WHERE admin_user_id = $1 AND action = 'disable_mfa'
		ORDER BY created_at DESC
		LIMIT 1
	`, userID).Scan(&disableMetadataBytes); err != nil {
		t.Fatalf("failed to query disable audit log: %v", err)
	}

	var disableMetadata map[string]interface{}
	if err := json.Unmarshal(disableMetadataBytes, &disableMetadata); err != nil {
		t.Fatalf("failed to unmarshal disable metadata: %v", err)
	}
	if disableMetadata["method"] != "totp" {
		t.Errorf("expected disable method 'totp', got %v", disableMetadata["method"])
	}
}
