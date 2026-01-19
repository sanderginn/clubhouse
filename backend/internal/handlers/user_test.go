package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
)

// TestGetProfileSuccess tests successfully retrieving a user profile
func TestGetProfileSuccess(t *testing.T) {
	db, err := getTestDB()
	if err != nil {
		t.Fatalf("failed to get test DB: %v", err)
	}
	if db == nil {
		t.Skip("test database not configured")
	}
	defer db.Close()

	userID := uuid.New()
	testUsername := "profileuser"
	testEmail := "profile@example.com"
	testHash := "$2a$12$test"

	query := `
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, $2, $3, $4, false, now(), now())
	`
	_, err = db.Exec(query, userID, testUsername, testEmail, testHash)
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
	db, err := getTestDB()
	if err != nil {
		t.Fatalf("failed to get test DB: %v", err)
	}
	if db == nil {
		t.Skip("test database not configured")
	}
	defer db.Close()

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
	db, err := getTestDB()
	if err != nil {
		t.Fatalf("failed to get test DB: %v", err)
	}
	if db == nil {
		t.Skip("test database not configured")
	}
	defer db.Close()

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

// TestGetProfileMethodNotAllowed tests with non-GET method
func TestGetProfileMethodNotAllowed(t *testing.T) {
	db, err := getTestDB()
	if err != nil {
		t.Fatalf("failed to get test DB: %v", err)
	}
	if db == nil {
		t.Skip("test database not configured")
	}
	defer db.Close()

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
	db, err := getTestDB()
	if err != nil {
		t.Fatalf("failed to get test DB: %v", err)
	}
	if db == nil {
		t.Skip("test database not configured")
	}
	defer db.Close()

	userID := uuid.New()
	testUsername := "deleteduser"
	testEmail := "deleted@example.com"
	testHash := "$2a$12$test"

	query := `
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at, deleted_at)
		VALUES ($1, $2, $3, $4, false, now(), now(), now())
	`
	_, err = db.Exec(query, userID, testUsername, testEmail, testHash)
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
	db, err := getTestDB()
	if err != nil {
		t.Fatalf("failed to get test DB: %v", err)
	}
	if db == nil {
		t.Skip("test database not configured")
	}
	defer db.Close()

	userID := uuid.New()
	testUsername := "unapproveduser"
	testEmail := "unapproved@example.com"
	testHash := "$2a$12$test"

	query := `
		INSERT INTO users (id, username, email, password_hash, is_admin, created_at)
		VALUES ($1, $2, $3, $4, false, now())
	`
	_, err = db.Exec(query, userID, testUsername, testEmail, testHash)
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
	db, err := getTestDB()
	if err != nil {
		t.Fatalf("failed to get test DB: %v", err)
	}
	if db == nil {
		t.Skip("test database not configured")
	}
	defer db.Close()

	// Create test user
	userID := uuid.New()
	testUsername := "postuser"
	testEmail := "postuser@example.com"
	testHash := "$2a$12$test"

	userQuery := `
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, $2, $3, $4, false, now(), now())
	`
	_, err = db.Exec(userQuery, userID, testUsername, testEmail, testHash)
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
	postQuery := `INSERT INTO posts (id, user_id, section_id, content, created_at) VALUES ($1, $2, $3, $4, now())`
	_, err = db.Exec(postQuery, postID1, userID, sectionID, "Test post 1")
	if err != nil {
		t.Fatalf("failed to create test post 1: %v", err)
	}
	_, err = db.Exec(postQuery, postID2, userID, sectionID, "Test post 2")
	if err != nil {
		t.Fatalf("failed to create test post 2: %v", err)
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
}

// TestGetUserPostsEmptyResult tests that non-existent user returns empty posts list
func TestGetUserPostsEmptyResult(t *testing.T) {
	db, err := getTestDB()
	if err != nil {
		t.Fatalf("failed to get test DB: %v", err)
	}
	if db == nil {
		t.Skip("test database not configured")
	}
	defer db.Close()

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
	db, err := getTestDB()
	if err != nil {
		t.Fatalf("failed to get test DB: %v", err)
	}
	if db == nil {
		t.Skip("test database not configured")
	}
	defer db.Close()

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
	db, err := getTestDB()
	if err != nil {
		t.Fatalf("failed to get test DB: %v", err)
	}
	if db == nil {
		t.Skip("test database not configured")
	}
	defer db.Close()

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
	db, err := getTestDB()
	if err != nil {
		t.Fatalf("failed to get test DB: %v", err)
	}
	if db == nil {
		t.Skip("test database not configured")
	}
	defer db.Close()

	// Create test user
	userID := uuid.New()
	testUsername := "softdeletepostuser"
	testEmail := "softdeletepost@example.com"
	testHash := "$2a$12$test"

	userQuery := `
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, $2, $3, $4, false, now(), now())
	`
	_, err = db.Exec(userQuery, userID, testUsername, testEmail, testHash)
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

	if len(response.Posts) != 1 {
		t.Errorf("expected 1 post (soft-deleted excluded), got %d", len(response.Posts))
	}

	if len(response.Posts) > 0 && response.Posts[0].ID != normalPostID {
		t.Errorf("expected normal post ID %s, got %s", normalPostID, response.Posts[0].ID)
	}
}

// TestGetUserPostsPagination tests cursor-based pagination
func TestGetUserPostsPagination(t *testing.T) {
	db, err := getTestDB()
	if err != nil {
		t.Fatalf("failed to get test DB: %v", err)
	}
	if db == nil {
		t.Skip("test database not configured")
	}
	defer db.Close()

	// Create test user
	userID := uuid.New()
	testUsername := "paginationuser"
	testEmail := "pagination@example.com"
	testHash := "$2a$12$test"

	userQuery := `
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, $2, $3, $4, false, now(), now())
	`
	_, err = db.Exec(userQuery, userID, testUsername, testEmail, testHash)
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

	// Create 3 posts
	for i := 0; i < 3; i++ {
		postID := uuid.New()
		postQuery := `INSERT INTO posts (id, user_id, section_id, content, created_at) VALUES ($1, $2, $3, $4, now())`
		_, err = db.Exec(postQuery, postID, userID, sectionID, "Test post")
		if err != nil {
			t.Fatalf("failed to create test post: %v", err)
		}
	}

	handler := NewUserHandler(db)

	// Request with limit=2
	req := httptest.NewRequest("GET", "/api/v1/users/"+userID.String()+"/posts?limit=2", nil)
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

	if !response.HasMore {
		t.Errorf("expected has_more to be true")
	}

	if response.NextCursor == nil {
		t.Errorf("expected next_cursor to be set")
	}
}
