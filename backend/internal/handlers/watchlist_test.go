package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/services"
	"github.com/sanderginn/clubhouse/internal/testutil"
)

func TestAddToWatchlistHandlerSuccess(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "watchlisthandleruser", "watchlisthandler@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Movies", "movie")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Movie post")

	service := services.NewWatchlistService(db)
	if _, err := service.CreateCategory(reqContext(), uuid.MustParse(userID), "Favorites"); err != nil {
		t.Fatalf("CreateCategory failed: %v", err)
	}

	handler := NewWatchlistHandler(db, nil)

	body := bytes.NewBufferString(`{"categories":["Favorites"]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts/"+postID+"/watchlist", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "watchlisthandleruser", false))
	rr := httptest.NewRecorder()

	handler.AddToWatchlist(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var response models.AddToWatchlistResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.WatchlistItems) != 1 {
		t.Fatalf("expected 1 watchlist item, got %d", len(response.WatchlistItems))
	}
	if response.WatchlistItems[0].Category != "Favorites" {
		t.Fatalf("expected Favorites category, got %s", response.WatchlistItems[0].Category)
	}
}

func TestAddToWatchlistPublishesSectionEvent(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })
	testutil.CleanupRedis(t)

	redisClient := testutil.GetTestRedis(t)

	userID := testutil.CreateTestUser(t, db, "watchlisteventuser", "watchlistevent@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Movies", "movie")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Movie post")

	service := services.NewWatchlistService(db)
	if _, err := service.CreateCategory(reqContext(), uuid.MustParse(userID), "Sci-Fi"); err != nil {
		t.Fatalf("CreateCategory failed: %v", err)
	}

	channel := formatChannel(sectionPrefix, sectionID)
	pubsub := subscribeTestChannel(t, redisClient, channel)

	handler := NewWatchlistHandler(db, redisClient)
	body := bytes.NewBufferString(`{"categories":["Sci-Fi"]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts/"+postID+"/watchlist", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "watchlisteventuser", false))
	rr := httptest.NewRecorder()

	handler.AddToWatchlist(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	event := receiveEvent(t, pubsub)
	if event.Type != "movie_watchlisted" {
		t.Fatalf("expected movie_watchlisted event, got %s", event.Type)
	}

	dataBytes, err := json.Marshal(event.Data)
	if err != nil {
		t.Fatalf("failed to marshal event data: %v", err)
	}

	var payload movieWatchlistedEventData
	if err := json.Unmarshal(dataBytes, &payload); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	if payload.PostID.String() != postID {
		t.Fatalf("expected post_id %s, got %s", postID, payload.PostID.String())
	}
	if payload.UserID.String() != userID {
		t.Fatalf("expected user_id %s, got %s", userID, payload.UserID.String())
	}
	if payload.Username != "watchlisteventuser" {
		t.Fatalf("expected username watchlisteventuser, got %s", payload.Username)
	}
	if len(payload.Categories) != 1 || payload.Categories[0] != "Sci-Fi" {
		t.Fatalf("expected categories [Sci-Fi], got %v", payload.Categories)
	}
}

func TestRemoveFromWatchlistPublishesSectionEvent(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })
	testutil.CleanupRedis(t)

	redisClient := testutil.GetTestRedis(t)

	userID := testutil.CreateTestUser(t, db, "unwatchlisteventuser", "unwatchlistevent@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Movies", "movie")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Movie post")

	service := services.NewWatchlistService(db)
	if _, err := service.AddToWatchlist(reqContext(), uuid.MustParse(userID), uuid.MustParse(postID), nil); err != nil {
		t.Fatalf("AddToWatchlist failed: %v", err)
	}

	channel := formatChannel(sectionPrefix, sectionID)
	pubsub := subscribeTestChannel(t, redisClient, channel)

	handler := NewWatchlistHandler(db, redisClient)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/posts/"+postID+"/watchlist", nil)
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "unwatchlisteventuser", false))
	rr := httptest.NewRecorder()

	handler.RemoveFromWatchlist(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	event := receiveEvent(t, pubsub)
	if event.Type != "movie_unwatchlisted" {
		t.Fatalf("expected movie_unwatchlisted event, got %s", event.Type)
	}

	dataBytes, err := json.Marshal(event.Data)
	if err != nil {
		t.Fatalf("failed to marshal event data: %v", err)
	}

	var payload movieUnwatchlistedEventData
	if err := json.Unmarshal(dataBytes, &payload); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	if payload.PostID.String() != postID {
		t.Fatalf("expected post_id %s, got %s", postID, payload.PostID.String())
	}
	if payload.UserID.String() != userID {
		t.Fatalf("expected user_id %s, got %s", userID, payload.UserID.String())
	}
}

func TestGetPostWatchlistInfoHandler(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userA := testutil.CreateTestUser(t, db, "watchlistinfoa", "watchlistinfoa@test.com", false, true)
	userB := testutil.CreateTestUser(t, db, "watchlistinfob", "watchlistinfob@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Movies", "movie")
	postID := testutil.CreateTestPost(t, db, userA, sectionID, "Movie post")

	service := services.NewWatchlistService(db)
	if _, err := service.AddToWatchlist(reqContext(), uuid.MustParse(userA), uuid.MustParse(postID), nil); err != nil {
		t.Fatalf("AddToWatchlist userA failed: %v", err)
	}
	if _, err := service.AddToWatchlist(reqContext(), uuid.MustParse(userB), uuid.MustParse(postID), nil); err != nil {
		t.Fatalf("AddToWatchlist userB failed: %v", err)
	}

	handler := NewWatchlistHandler(db, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/posts/"+postID+"/watchlist-info", nil)
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userA), "watchlistinfoa", false))
	rr := httptest.NewRecorder()

	handler.GetPostWatchlistInfo(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var response models.PostWatchlistInfo
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.SaveCount != 2 {
		t.Fatalf("expected save_count 2, got %d", response.SaveCount)
	}
	if !response.ViewerSaved {
		t.Fatalf("expected viewer_saved true")
	}
	if len(response.ViewerCategories) != 1 || response.ViewerCategories[0] != "Uncategorized" {
		t.Fatalf("expected viewer_categories [Uncategorized], got %v", response.ViewerCategories)
	}
}

func TestListWatchlistHandler(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "listwatchlistuser", "listwatchlist@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Movies", "movie")
	postA := testutil.CreateTestPost(t, db, userID, sectionID, "Movie A")
	postB := testutil.CreateTestPost(t, db, userID, sectionID, "Movie B")

	service := services.NewWatchlistService(db)
	if _, err := service.CreateCategory(reqContext(), uuid.MustParse(userID), "Favorites"); err != nil {
		t.Fatalf("CreateCategory failed: %v", err)
	}
	if _, err := service.AddToWatchlist(reqContext(), uuid.MustParse(userID), uuid.MustParse(postA), []string{"Favorites"}); err != nil {
		t.Fatalf("AddToWatchlist favorites failed: %v", err)
	}
	if _, err := service.AddToWatchlist(reqContext(), uuid.MustParse(userID), uuid.MustParse(postB), nil); err != nil {
		t.Fatalf("AddToWatchlist uncategorized failed: %v", err)
	}

	handler := NewWatchlistHandler(db, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/watchlist", nil)
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "listwatchlistuser", false))
	rr := httptest.NewRecorder()

	handler.ListWatchlist(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var response models.WatchlistResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Categories) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(response.Categories))
	}

	gotNames := []string{response.Categories[0].Name, response.Categories[1].Name}
	sort.Strings(gotNames)
	if gotNames[0] != "Favorites" || gotNames[1] != "Uncategorized" {
		t.Fatalf("unexpected category names: %v", gotNames)
	}
}

func TestWatchlistCategoryCRUDAndListHandlers(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "watchlistcategoryhandler", "watchlistcategoryhandler@test.com", false, true)
	handler := NewWatchlistHandler(db, nil)

	createBody := bytes.NewBufferString(`{"name":"To Watch"}`)
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/me/watchlist-categories", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq = createReq.WithContext(createTestUserContext(createReq.Context(), uuid.MustParse(userID), "watchlistcategoryhandler", false))
	createRR := httptest.NewRecorder()

	handler.CreateWatchlistCategory(createRR, createReq)

	if createRR.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d. Body: %s", createRR.Code, createRR.Body.String())
	}

	var createResp models.CreateWatchlistCategoryResponse
	if err := json.NewDecoder(createRR.Body).Decode(&createResp); err != nil {
		t.Fatalf("failed to decode create response: %v", err)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/me/watchlist-categories", nil)
	listReq = listReq.WithContext(createTestUserContext(listReq.Context(), uuid.MustParse(userID), "watchlistcategoryhandler", false))
	listRR := httptest.NewRecorder()

	handler.ListWatchlistCategories(listRR, listReq)

	if listRR.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", listRR.Code, listRR.Body.String())
	}

	var listResp models.ListWatchlistCategoriesResponse
	if err := json.NewDecoder(listRR.Body).Decode(&listResp); err != nil {
		t.Fatalf("failed to decode list response: %v", err)
	}
	if len(listResp.Categories) != 1 || listResp.Categories[0].ID != createResp.Category.ID {
		t.Fatalf("expected single created category, got %+v", listResp.Categories)
	}

	updateBody := bytes.NewBufferString(`{"name":"Sci-Fi","position":2}`)
	updateReq := httptest.NewRequest(http.MethodPatch, "/api/v1/me/watchlist-categories/"+createResp.Category.ID.String(), updateBody)
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq = updateReq.WithContext(createTestUserContext(updateReq.Context(), uuid.MustParse(userID), "watchlistcategoryhandler", false))
	updateRR := httptest.NewRecorder()

	handler.UpdateWatchlistCategory(updateRR, updateReq)

	if updateRR.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", updateRR.Code, updateRR.Body.String())
	}

	var updateResp models.UpdateWatchlistCategoryResponse
	if err := json.NewDecoder(updateRR.Body).Decode(&updateResp); err != nil {
		t.Fatalf("failed to decode update response: %v", err)
	}
	if updateResp.Category.Name != "Sci-Fi" {
		t.Fatalf("expected updated name Sci-Fi, got %s", updateResp.Category.Name)
	}
	if updateResp.Category.Position != 2 {
		t.Fatalf("expected updated position 2, got %d", updateResp.Category.Position)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/me/watchlist-categories/"+createResp.Category.ID.String(), nil)
	deleteReq = deleteReq.WithContext(createTestUserContext(deleteReq.Context(), uuid.MustParse(userID), "watchlistcategoryhandler", false))
	deleteRR := httptest.NewRecorder()

	handler.DeleteWatchlistCategory(deleteRR, deleteReq)

	if deleteRR.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", deleteRR.Code)
	}
}

func TestWatchlistHandlersRequireAuth(t *testing.T) {
	handler := &WatchlistHandler{}
	categoryID := uuid.New().String()
	postID := uuid.New().String()

	tests := []struct {
		name     string
		fn       func(http.ResponseWriter, *http.Request)
		method   string
		path     string
		body     string
		wantCode int
	}{
		{
			name:     "add to watchlist",
			fn:       handler.AddToWatchlist,
			method:   http.MethodPost,
			path:     "/api/v1/posts/" + postID + "/watchlist",
			body:     `{"categories":["Favorites"]}`,
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "remove from watchlist",
			fn:       handler.RemoveFromWatchlist,
			method:   http.MethodDelete,
			path:     "/api/v1/posts/" + postID + "/watchlist",
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "get watchlist info",
			fn:       handler.GetPostWatchlistInfo,
			method:   http.MethodGet,
			path:     "/api/v1/posts/" + postID + "/watchlist-info",
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "list watchlist",
			fn:       handler.ListWatchlist,
			method:   http.MethodGet,
			path:     "/api/v1/me/watchlist",
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "list watchlist categories",
			fn:       handler.ListWatchlistCategories,
			method:   http.MethodGet,
			path:     "/api/v1/me/watchlist-categories",
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "create watchlist category",
			fn:       handler.CreateWatchlistCategory,
			method:   http.MethodPost,
			path:     "/api/v1/me/watchlist-categories",
			body:     `{"name":"Category"}`,
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "update watchlist category",
			fn:       handler.UpdateWatchlistCategory,
			method:   http.MethodPatch,
			path:     "/api/v1/me/watchlist-categories/" + categoryID,
			body:     `{"name":"Updated"}`,
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "delete watchlist category",
			fn:       handler.DeleteWatchlistCategory,
			method:   http.MethodDelete,
			path:     "/api/v1/me/watchlist-categories/" + categoryID,
			wantCode: http.StatusUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var body *bytes.Buffer
			if tc.body != "" {
				body = bytes.NewBufferString(tc.body)
			} else {
				body = bytes.NewBuffer(nil)
			}
			req := httptest.NewRequest(tc.method, tc.path, body)
			rr := httptest.NewRecorder()

			tc.fn(rr, req)

			if rr.Code != tc.wantCode {
				t.Fatalf("expected status %d, got %d. Body: %s", tc.wantCode, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestAddToWatchlistValidationErrors(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "watchlistvalidation", "watchlistvalidation@test.com", false, true)
	recipeSectionID := testutil.CreateTestSection(t, db, "Recipes", "recipe")
	recipePostID := testutil.CreateTestPost(t, db, userID, recipeSectionID, "Recipe post")

	handler := NewWatchlistHandler(db, nil)
	ctx := createTestUserContext(reqContext(), uuid.MustParse(userID), "watchlistvalidation", false)

	invalidReq := httptest.NewRequest(http.MethodPost, "/api/v1/posts/not-a-uuid/watchlist", bytes.NewBufferString(`{"categories":["Favorites"]}`))
	invalidReq.Header.Set("Content-Type", "application/json")
	invalidReq = invalidReq.WithContext(ctx)
	invalidRR := httptest.NewRecorder()

	handler.AddToWatchlist(invalidRR, invalidReq)

	if invalidRR.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid post id status 400, got %d. Body: %s", invalidRR.Code, invalidRR.Body.String())
	}

	var invalidResp models.ErrorResponse
	if err := json.NewDecoder(invalidRR.Body).Decode(&invalidResp); err != nil {
		t.Fatalf("failed to decode invalid post id response: %v", err)
	}
	if invalidResp.Code != "INVALID_POST_ID" {
		t.Fatalf("expected INVALID_POST_ID, got %s", invalidResp.Code)
	}

	nonMovieReq := httptest.NewRequest(http.MethodPost, "/api/v1/posts/"+recipePostID+"/watchlist", bytes.NewBufferString(`{"categories":["Favorites"]}`))
	nonMovieReq.Header.Set("Content-Type", "application/json")
	nonMovieReq = nonMovieReq.WithContext(ctx)
	nonMovieRR := httptest.NewRecorder()

	handler.AddToWatchlist(nonMovieRR, nonMovieReq)

	if nonMovieRR.Code != http.StatusNotFound {
		t.Fatalf("expected non-movie status 404, got %d. Body: %s", nonMovieRR.Code, nonMovieRR.Body.String())
	}

	var nonMovieResp models.ErrorResponse
	if err := json.NewDecoder(nonMovieRR.Body).Decode(&nonMovieResp); err != nil {
		t.Fatalf("failed to decode non-movie response: %v", err)
	}
	if nonMovieResp.Code != "POST_NOT_FOUND" {
		t.Fatalf("expected POST_NOT_FOUND, got %s", nonMovieResp.Code)
	}
}
