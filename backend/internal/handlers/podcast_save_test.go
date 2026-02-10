package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/services"
	"github.com/sanderginn/clubhouse/internal/testutil"
)

func TestSavePodcastHandlerSuccess(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "savepodcasthandler", "savepodcasthandler@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Podcasts", "podcast")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Podcast post")

	handler := NewPodcastSaveHandler(db)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts/"+postID+"/podcast-save", nil)
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "savepodcasthandler", false))
	rr := httptest.NewRecorder()

	handler.SavePodcast(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var response models.PodcastSave
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.PostID.String() != postID {
		t.Fatalf("expected post_id %s, got %s", postID, response.PostID.String())
	}
	if response.UserID.String() != userID {
		t.Fatalf("expected user_id %s, got %s", userID, response.UserID.String())
	}
}

func TestUnsavePodcastHandlerNoContent(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "unsavepodcasthandler", "unsavepodcasthandler@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Podcasts", "podcast")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Podcast post")

	service := services.NewPodcastSaveService(db)
	if _, err := service.SavePodcast(reqContext(), uuid.MustParse(userID), uuid.MustParse(postID)); err != nil {
		t.Fatalf("SavePodcast failed: %v", err)
	}

	handler := NewPodcastSaveHandler(db)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/posts/"+postID+"/podcast-save", nil)
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "unsavepodcasthandler", false))
	rr := httptest.NewRecorder()

	handler.UnsavePodcast(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestGetPostPodcastSaveInfoHandler(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userA := testutil.CreateTestUser(t, db, "podcastsaveinfoa", "podcastsaveinfoa@test.com", false, true)
	userB := testutil.CreateTestUser(t, db, "podcastsaveinfob", "podcastsaveinfob@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Podcasts", "podcast")
	postID := testutil.CreateTestPost(t, db, userA, sectionID, "Podcast post")

	service := services.NewPodcastSaveService(db)
	if _, err := service.SavePodcast(reqContext(), uuid.MustParse(userA), uuid.MustParse(postID)); err != nil {
		t.Fatalf("SavePodcast userA failed: %v", err)
	}
	if _, err := service.SavePodcast(reqContext(), uuid.MustParse(userB), uuid.MustParse(postID)); err != nil {
		t.Fatalf("SavePodcast userB failed: %v", err)
	}

	handler := NewPodcastSaveHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/posts/"+postID+"/podcast-save-info", nil)
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userA), "podcastsaveinfoa", false))
	rr := httptest.NewRecorder()

	handler.GetPostPodcastSaveInfo(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var response models.PostPodcastSaveInfo
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.SaveCount != 2 {
		t.Fatalf("expected save_count 2, got %d", response.SaveCount)
	}
	if !response.ViewerSaved {
		t.Fatalf("expected viewer_saved true")
	}
}

func TestListSectionSavedPodcastPostsHandler(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "listsavedpodcasthandler", "listsavedpodcasthandler@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Podcasts", "podcast")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Podcast post")

	service := services.NewPodcastSaveService(db)
	if _, err := service.SavePodcast(reqContext(), uuid.MustParse(userID), uuid.MustParse(postID)); err != nil {
		t.Fatalf("SavePodcast failed: %v", err)
	}

	handler := NewPodcastSaveHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sections/"+sectionID+"/podcast-saved", nil)
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "listsavedpodcasthandler", false))
	rr := httptest.NewRecorder()

	handler.ListSectionSavedPodcastPosts(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var response models.FeedResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(response.Posts) != 1 {
		t.Fatalf("expected 1 post, got %d", len(response.Posts))
	}
	if response.Posts[0].ID.String() != postID {
		t.Fatalf("expected post_id %s, got %s", postID, response.Posts[0].ID.String())
	}
}

func TestPodcastSaveHandlersRequireAuth(t *testing.T) {
	handler := &PodcastSaveHandler{}
	postID := uuid.New().String()
	sectionID := uuid.New().String()

	tests := []struct {
		name     string
		fn       func(http.ResponseWriter, *http.Request)
		method   string
		path     string
		wantCode int
	}{
		{
			name:     "save podcast",
			fn:       handler.SavePodcast,
			method:   http.MethodPost,
			path:     "/api/v1/posts/" + postID + "/podcast-save",
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "unsave podcast",
			fn:       handler.UnsavePodcast,
			method:   http.MethodDelete,
			path:     "/api/v1/posts/" + postID + "/podcast-save",
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "get post podcast save info",
			fn:       handler.GetPostPodcastSaveInfo,
			method:   http.MethodGet,
			path:     "/api/v1/posts/" + postID + "/podcast-save-info",
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "list section saved podcasts",
			fn:       handler.ListSectionSavedPodcastPosts,
			method:   http.MethodGet,
			path:     "/api/v1/sections/" + sectionID + "/podcast-saved",
			wantCode: http.StatusUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rr := httptest.NewRecorder()

			tc.fn(rr, req)

			if rr.Code != tc.wantCode {
				t.Fatalf("expected status %d, got %d. Body: %s", tc.wantCode, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestPodcastSaveHandlersMethodValidation(t *testing.T) {
	handler := &PodcastSaveHandler{}
	postID := uuid.New().String()
	sectionID := uuid.New().String()

	tests := []struct {
		name   string
		fn     func(http.ResponseWriter, *http.Request)
		method string
		path   string
	}{
		{
			name:   "save podcast",
			fn:     handler.SavePodcast,
			method: http.MethodGet,
			path:   "/api/v1/posts/" + postID + "/podcast-save",
		},
		{
			name:   "unsave podcast",
			fn:     handler.UnsavePodcast,
			method: http.MethodPost,
			path:   "/api/v1/posts/" + postID + "/podcast-save",
		},
		{
			name:   "get post podcast save info",
			fn:     handler.GetPostPodcastSaveInfo,
			method: http.MethodPost,
			path:   "/api/v1/posts/" + postID + "/podcast-save-info",
		},
		{
			name:   "list section saved podcasts",
			fn:     handler.ListSectionSavedPodcastPosts,
			method: http.MethodPost,
			path:   "/api/v1/sections/" + sectionID + "/podcast-saved",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rr := httptest.NewRecorder()

			tc.fn(rr, req)

			if rr.Code != http.StatusMethodNotAllowed {
				t.Fatalf("expected status 405, got %d. Body: %s", rr.Code, rr.Body.String())
			}

			var response models.ErrorResponse
			if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if response.Code != "METHOD_NOT_ALLOWED" {
				t.Fatalf("expected METHOD_NOT_ALLOWED, got %s", response.Code)
			}
		})
	}
}

func TestPodcastSaveValidationErrors(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "podcastsavevalidation", "podcastsavevalidation@test.com", false, true)
	recipeSectionID := testutil.CreateTestSection(t, db, "Recipes", "recipe")
	recipePostID := testutil.CreateTestPost(t, db, userID, recipeSectionID, "Recipe post")
	podcastSectionID := testutil.CreateTestSection(t, db, "Podcasts", "podcast")

	handler := NewPodcastSaveHandler(db)
	ctx := createTestUserContext(reqContext(), uuid.MustParse(userID), "podcastsavevalidation", false)

	invalidPostReq := httptest.NewRequest(http.MethodPost, "/api/v1/posts/not-a-uuid/podcast-save", nil)
	invalidPostReq = invalidPostReq.WithContext(ctx)
	invalidPostRR := httptest.NewRecorder()
	handler.SavePodcast(invalidPostRR, invalidPostReq)

	if invalidPostRR.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid post id status 400, got %d. Body: %s", invalidPostRR.Code, invalidPostRR.Body.String())
	}

	var invalidPostResp models.ErrorResponse
	if err := json.NewDecoder(invalidPostRR.Body).Decode(&invalidPostResp); err != nil {
		t.Fatalf("failed to decode invalid post id response: %v", err)
	}
	if invalidPostResp.Code != "INVALID_POST_ID" {
		t.Fatalf("expected INVALID_POST_ID, got %s", invalidPostResp.Code)
	}

	nonPodcastReq := httptest.NewRequest(http.MethodPost, "/api/v1/posts/"+recipePostID+"/podcast-save", nil)
	nonPodcastReq = nonPodcastReq.WithContext(ctx)
	nonPodcastRR := httptest.NewRecorder()
	handler.SavePodcast(nonPodcastRR, nonPodcastReq)

	if nonPodcastRR.Code != http.StatusNotFound {
		t.Fatalf("expected non-podcast status 404, got %d. Body: %s", nonPodcastRR.Code, nonPodcastRR.Body.String())
	}

	var nonPodcastResp models.ErrorResponse
	if err := json.NewDecoder(nonPodcastRR.Body).Decode(&nonPodcastResp); err != nil {
		t.Fatalf("failed to decode non-podcast response: %v", err)
	}
	if nonPodcastResp.Code != "POST_NOT_FOUND" {
		t.Fatalf("expected POST_NOT_FOUND, got %s", nonPodcastResp.Code)
	}

	invalidSectionReq := httptest.NewRequest(http.MethodGet, "/api/v1/sections/not-a-uuid/podcast-saved", nil)
	invalidSectionReq = invalidSectionReq.WithContext(ctx)
	invalidSectionRR := httptest.NewRecorder()
	handler.ListSectionSavedPodcastPosts(invalidSectionRR, invalidSectionReq)

	if invalidSectionRR.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid section id status 400, got %d. Body: %s", invalidSectionRR.Code, invalidSectionRR.Body.String())
	}

	var invalidSectionResp models.ErrorResponse
	if err := json.NewDecoder(invalidSectionRR.Body).Decode(&invalidSectionResp); err != nil {
		t.Fatalf("failed to decode invalid section id response: %v", err)
	}
	if invalidSectionResp.Code != "INVALID_SECTION_ID" {
		t.Fatalf("expected INVALID_SECTION_ID, got %s", invalidSectionResp.Code)
	}

	nonPodcastSectionReq := httptest.NewRequest(http.MethodGet, "/api/v1/sections/"+recipeSectionID+"/podcast-saved", nil)
	nonPodcastSectionReq = nonPodcastSectionReq.WithContext(ctx)
	nonPodcastSectionRR := httptest.NewRecorder()
	handler.ListSectionSavedPodcastPosts(nonPodcastSectionRR, nonPodcastSectionReq)

	if nonPodcastSectionRR.Code != http.StatusBadRequest {
		t.Fatalf("expected non-podcast section status 400, got %d. Body: %s", nonPodcastSectionRR.Code, nonPodcastSectionRR.Body.String())
	}

	var nonPodcastSectionResp models.ErrorResponse
	if err := json.NewDecoder(nonPodcastSectionRR.Body).Decode(&nonPodcastSectionResp); err != nil {
		t.Fatalf("failed to decode non-podcast section response: %v", err)
	}
	if nonPodcastSectionResp.Code != "INVALID_SECTION_TYPE" {
		t.Fatalf("expected INVALID_SECTION_TYPE, got %s", nonPodcastSectionResp.Code)
	}

	badCursorReq := httptest.NewRequest(http.MethodGet, "/api/v1/sections/"+podcastSectionID+"/podcast-saved?cursor=bad-cursor", nil)
	badCursorReq = badCursorReq.WithContext(ctx)
	badCursorRR := httptest.NewRecorder()
	handler.ListSectionSavedPodcastPosts(badCursorRR, badCursorReq)

	if badCursorRR.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid cursor status 400, got %d. Body: %s", badCursorRR.Code, badCursorRR.Body.String())
	}

	var badCursorResp models.ErrorResponse
	if err := json.NewDecoder(badCursorRR.Body).Decode(&badCursorResp); err != nil {
		t.Fatalf("failed to decode invalid cursor response: %v", err)
	}
	if badCursorResp.Code != "INVALID_CURSOR" {
		t.Fatalf("expected INVALID_CURSOR, got %s", badCursorResp.Code)
	}
}
