package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/services"
	"github.com/sanderginn/clubhouse/internal/testutil"
)

func TestBookshelfCategoryHandlersSuccess(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "bookshelfcategoryhandler", "bookshelfcategoryhandler@test.com", false, true)
	handler := NewBookshelfHandler(services.NewBookshelfService(db))

	createReqA := httptest.NewRequest(http.MethodPost, "/api/v1/bookshelf/categories", bytes.NewBufferString(`{"name":"Favorites"}`))
	createReqA.Header.Set("Content-Type", "application/json")
	createReqA = createReqA.WithContext(createTestUserContext(createReqA.Context(), uuid.MustParse(userID), "bookshelfcategoryhandler", false))
	createRRA := httptest.NewRecorder()
	handler.CreateCategory(createRRA, createReqA)

	if createRRA.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d. Body: %s", createRRA.Code, createRRA.Body.String())
	}

	var createRespA models.CreateBookshelfCategoryResponse
	if err := json.NewDecoder(createRRA.Body).Decode(&createRespA); err != nil {
		t.Fatalf("failed to decode create category response: %v", err)
	}

	createReqB := httptest.NewRequest(http.MethodPost, "/api/v1/bookshelf/categories", bytes.NewBufferString(`{"name":"Sci-Fi"}`))
	createReqB.Header.Set("Content-Type", "application/json")
	createReqB = createReqB.WithContext(createTestUserContext(createReqB.Context(), uuid.MustParse(userID), "bookshelfcategoryhandler", false))
	createRRB := httptest.NewRecorder()
	handler.CreateCategory(createRRB, createReqB)

	if createRRB.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d. Body: %s", createRRB.Code, createRRB.Body.String())
	}

	var createRespB models.CreateBookshelfCategoryResponse
	if err := json.NewDecoder(createRRB.Body).Decode(&createRespB); err != nil {
		t.Fatalf("failed to decode second create category response: %v", err)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/bookshelf/categories", nil)
	listReq = listReq.WithContext(createTestUserContext(listReq.Context(), uuid.MustParse(userID), "bookshelfcategoryhandler", false))
	listRR := httptest.NewRecorder()
	handler.ListCategories(listRR, listReq)

	if listRR.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", listRR.Code, listRR.Body.String())
	}

	var listResp models.ListBookshelfCategoriesResponse
	if err := json.NewDecoder(listRR.Body).Decode(&listResp); err != nil {
		t.Fatalf("failed to decode list categories response: %v", err)
	}
	if len(listResp.Categories) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(listResp.Categories))
	}

	updateReq := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/bookshelf/categories/"+createRespA.Category.ID.String(),
		bytes.NewBufferString(`{"name":"Classics","position":1}`),
	)
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq = updateReq.WithContext(createTestUserContext(updateReq.Context(), uuid.MustParse(userID), "bookshelfcategoryhandler", false))
	updateRR := httptest.NewRecorder()
	handler.UpdateCategory(updateRR, updateReq)

	if updateRR.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", updateRR.Code, updateRR.Body.String())
	}

	var updateResp models.UpdateBookshelfCategoryResponse
	if err := json.NewDecoder(updateRR.Body).Decode(&updateResp); err != nil {
		t.Fatalf("failed to decode update category response: %v", err)
	}
	if updateResp.Category.Name != "Classics" {
		t.Fatalf("expected updated category name Classics, got %s", updateResp.Category.Name)
	}

	reorderReq := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/bookshelf/categories/reorder",
		bytes.NewBufferString(`{"category_ids":["`+createRespB.Category.ID.String()+`","`+createRespA.Category.ID.String()+`"]}`),
	)
	reorderReq.Header.Set("Content-Type", "application/json")
	reorderReq = reorderReq.WithContext(createTestUserContext(reorderReq.Context(), uuid.MustParse(userID), "bookshelfcategoryhandler", false))
	reorderRR := httptest.NewRecorder()
	handler.ReorderCategories(reorderRR, reorderReq)

	if reorderRR.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d. Body: %s", reorderRR.Code, reorderRR.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/bookshelf/categories/"+createRespA.Category.ID.String(), nil)
	deleteReq = deleteReq.WithContext(createTestUserContext(deleteReq.Context(), uuid.MustParse(userID), "bookshelfcategoryhandler", false))
	deleteRR := httptest.NewRecorder()
	handler.DeleteCategory(deleteRR, deleteReq)

	if deleteRR.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d. Body: %s", deleteRR.Code, deleteRR.Body.String())
	}
}

func TestBookshelfAddAndRemoveHandlersSuccess(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "bookshelfaddhandler", "bookshelfaddhandler@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Book post")
	handler := NewBookshelfHandler(services.NewBookshelfService(db))

	addReq := httptest.NewRequest(http.MethodPost, "/api/v1/posts/"+postID+"/bookshelf", bytes.NewBufferString(`{"categories":["Favorites"]}`))
	addReq.Header.Set("Content-Type", "application/json")
	addReq = addReq.WithContext(createTestUserContext(addReq.Context(), uuid.MustParse(userID), "bookshelfaddhandler", false))
	addRR := httptest.NewRecorder()
	handler.AddToBookshelf(addRR, addReq)

	if addRR.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d. Body: %s", addRR.Code, addRR.Body.String())
	}

	var activeCount int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM bookshelf_items
		WHERE user_id = $1 AND post_id = $2 AND deleted_at IS NULL
	`, userID, postID).Scan(&activeCount); err != nil {
		t.Fatalf("failed to query bookshelf item count: %v", err)
	}
	if activeCount != 1 {
		t.Fatalf("expected 1 active bookshelf item, got %d", activeCount)
	}

	removeReq := httptest.NewRequest(http.MethodDelete, "/api/v1/posts/"+postID+"/bookshelf", nil)
	removeReq = removeReq.WithContext(createTestUserContext(removeReq.Context(), uuid.MustParse(userID), "bookshelfaddhandler", false))
	removeRR := httptest.NewRecorder()
	handler.RemoveFromBookshelf(removeRR, removeReq)

	if removeRR.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d. Body: %s", removeRR.Code, removeRR.Body.String())
	}

	var deletedAt sql.NullTime
	if err := db.QueryRow(`
		SELECT deleted_at
		FROM bookshelf_items
		WHERE user_id = $1 AND post_id = $2
	`, userID, postID).Scan(&deletedAt); err != nil {
		t.Fatalf("failed to query deleted bookshelf item: %v", err)
	}
	if !deletedAt.Valid {
		t.Fatal("expected deleted_at to be set after remove")
	}
}

func TestBookshelfListHandlersSuccess(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userA := uuid.MustParse(testutil.CreateTestUser(t, db, "bookshelflisthandlera", "bookshelflisthandlera@test.com", false, true))
	userB := uuid.MustParse(testutil.CreateTestUser(t, db, "bookshelflisthandlerb", "bookshelflisthandlerb@test.com", false, true))
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postA := uuid.MustParse(testutil.CreateTestPost(t, db, userA.String(), sectionID, "Book A"))
	postB := uuid.MustParse(testutil.CreateTestPost(t, db, userA.String(), sectionID, "Book B"))
	postC := uuid.MustParse(testutil.CreateTestPost(t, db, userB.String(), sectionID, "Book C"))

	service := services.NewBookshelfService(db)
	if err := service.AddToBookshelf(reqContext(), userA, postA, []string{"Favorites"}); err != nil {
		t.Fatalf("AddToBookshelf postA failed: %v", err)
	}
	if err := service.AddToBookshelf(reqContext(), userA, postB, nil); err != nil {
		t.Fatalf("AddToBookshelf postB failed: %v", err)
	}
	if err := service.AddToBookshelf(reqContext(), userB, postC, []string{"Favorites"}); err != nil {
		t.Fatalf("AddToBookshelf postC failed: %v", err)
	}

	handler := NewBookshelfHandler(service)

	myReq := httptest.NewRequest(http.MethodGet, "/api/v1/bookshelf?limit=1", nil)
	myReq = myReq.WithContext(createTestUserContext(myReq.Context(), userA, "bookshelflisthandlera", false))
	myRR := httptest.NewRecorder()
	handler.GetMyBookshelf(myRR, myReq)

	if myRR.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", myRR.Code, myRR.Body.String())
	}

	var myResp models.ListBookshelfItemsResponse
	if err := json.NewDecoder(myRR.Body).Decode(&myResp); err != nil {
		t.Fatalf("failed to decode my bookshelf response: %v", err)
	}
	if len(myResp.BookshelfItems) != 1 {
		t.Fatalf("expected 1 item in first page, got %d", len(myResp.BookshelfItems))
	}
	if myResp.NextCursor == nil || *myResp.NextCursor == "" {
		t.Fatal("expected next_cursor for first page")
	}

	pageTwoReq := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/bookshelf?limit=1&cursor="+url.QueryEscape(*myResp.NextCursor),
		nil,
	)
	pageTwoReq = pageTwoReq.WithContext(createTestUserContext(pageTwoReq.Context(), userA, "bookshelflisthandlera", false))
	pageTwoRR := httptest.NewRecorder()
	handler.GetMyBookshelf(pageTwoRR, pageTwoReq)

	if pageTwoRR.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", pageTwoRR.Code, pageTwoRR.Body.String())
	}

	var pageTwoResp models.ListBookshelfItemsResponse
	if err := json.NewDecoder(pageTwoRR.Body).Decode(&pageTwoResp); err != nil {
		t.Fatalf("failed to decode second page response: %v", err)
	}
	if len(pageTwoResp.BookshelfItems) != 1 {
		t.Fatalf("expected 1 item in second page, got %d", len(pageTwoResp.BookshelfItems))
	}

	allReq := httptest.NewRequest(http.MethodGet, "/api/v1/bookshelf/all?category=Favorites&limit=10", nil)
	allReq = allReq.WithContext(createTestUserContext(allReq.Context(), userA, "bookshelflisthandlera", false))
	allRR := httptest.NewRecorder()
	handler.GetAllBookshelf(allRR, allReq)

	if allRR.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", allRR.Code, allRR.Body.String())
	}

	var allResp models.ListBookshelfItemsResponse
	if err := json.NewDecoder(allRR.Body).Decode(&allResp); err != nil {
		t.Fatalf("failed to decode all bookshelf response: %v", err)
	}
	if len(allResp.BookshelfItems) != 2 {
		t.Fatalf("expected 2 favorite items across users, got %d", len(allResp.BookshelfItems))
	}
}

func TestBookshelfHandlersRequireAuth(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	handler := NewBookshelfHandler(services.NewBookshelfService(db))
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
			name:     "create category",
			fn:       handler.CreateCategory,
			method:   http.MethodPost,
			path:     "/api/v1/bookshelf/categories",
			body:     `{"name":"Favorites"}`,
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "list categories",
			fn:       handler.ListCategories,
			method:   http.MethodGet,
			path:     "/api/v1/bookshelf/categories",
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "update category",
			fn:       handler.UpdateCategory,
			method:   http.MethodPut,
			path:     "/api/v1/bookshelf/categories/" + categoryID,
			body:     `{"name":"Updated","position":0}`,
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "delete category",
			fn:       handler.DeleteCategory,
			method:   http.MethodDelete,
			path:     "/api/v1/bookshelf/categories/" + categoryID,
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "reorder categories",
			fn:       handler.ReorderCategories,
			method:   http.MethodPost,
			path:     "/api/v1/bookshelf/categories/reorder",
			body:     `{"category_ids":["` + categoryID + `"]}`,
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "add to bookshelf",
			fn:       handler.AddToBookshelf,
			method:   http.MethodPost,
			path:     "/api/v1/posts/" + postID + "/bookshelf",
			body:     `{"categories":["Favorites"]}`,
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "remove from bookshelf",
			fn:       handler.RemoveFromBookshelf,
			method:   http.MethodDelete,
			path:     "/api/v1/posts/" + postID + "/bookshelf",
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "get my bookshelf",
			fn:       handler.GetMyBookshelf,
			method:   http.MethodGet,
			path:     "/api/v1/bookshelf",
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "get all bookshelf",
			fn:       handler.GetAllBookshelf,
			method:   http.MethodGet,
			path:     "/api/v1/bookshelf/all",
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

func TestBookshelfHandlersValidationErrors(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.MustParse(testutil.CreateTestUser(t, db, "bookshelfvalidation", "bookshelfvalidation@test.com", false, true))
	handler := NewBookshelfHandler(services.NewBookshelfService(db))
	ctx := createTestUserContext(reqContext(), userID, "bookshelfvalidation", false)

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/bookshelf/categories", bytes.NewBufferString(`{"name":"   "}`))
	createReq.Header.Set("Content-Type", "application/json")
	createReq = createReq.WithContext(ctx)
	createRR := httptest.NewRecorder()
	handler.CreateCategory(createRR, createReq)
	assertErrorCode(t, createRR, http.StatusBadRequest, "CATEGORY_NAME_REQUIRED")

	updateReq := httptest.NewRequest(http.MethodPut, "/api/v1/bookshelf/categories/not-a-uuid", bytes.NewBufferString(`{"name":"Shelf","position":0}`))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq = updateReq.WithContext(ctx)
	updateRR := httptest.NewRecorder()
	handler.UpdateCategory(updateRR, updateReq)
	assertErrorCode(t, updateRR, http.StatusBadRequest, "INVALID_CATEGORY_ID")

	reorderReq := httptest.NewRequest(http.MethodPost, "/api/v1/bookshelf/categories/reorder", bytes.NewBufferString(`{"category_ids":[]}`))
	reorderReq.Header.Set("Content-Type", "application/json")
	reorderReq = reorderReq.WithContext(ctx)
	reorderRR := httptest.NewRecorder()
	handler.ReorderCategories(reorderRR, reorderReq)
	assertErrorCode(t, reorderRR, http.StatusBadRequest, "CATEGORY_IDS_REQUIRED")

	addReq := httptest.NewRequest(http.MethodPost, "/api/v1/posts/not-a-uuid/bookshelf", bytes.NewBufferString(`{"categories":["Favorites"]}`))
	addReq.Header.Set("Content-Type", "application/json")
	addReq = addReq.WithContext(ctx)
	addRR := httptest.NewRecorder()
	handler.AddToBookshelf(addRR, addReq)
	assertErrorCode(t, addRR, http.StatusBadRequest, "INVALID_POST_ID")

	invalidCursorReq := httptest.NewRequest(http.MethodGet, "/api/v1/bookshelf?cursor=invalid-cursor", nil)
	invalidCursorReq = invalidCursorReq.WithContext(ctx)
	invalidCursorRR := httptest.NewRecorder()
	handler.GetMyBookshelf(invalidCursorRR, invalidCursorReq)
	assertErrorCode(t, invalidCursorRR, http.StatusBadRequest, "INVALID_CURSOR")

	invalidLimitReq := httptest.NewRequest(http.MethodGet, "/api/v1/bookshelf/all?limit=abc", nil)
	invalidLimitReq = invalidLimitReq.WithContext(ctx)
	invalidLimitRR := httptest.NewRecorder()
	handler.GetAllBookshelf(invalidLimitRR, invalidLimitReq)
	assertErrorCode(t, invalidLimitRR, http.StatusBadRequest, "INVALID_LIMIT")
}

func TestBookshelfHandlersNotFound(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.MustParse(testutil.CreateTestUser(t, db, "bookshelfnotfound", "bookshelfnotfound@test.com", false, true))
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	recipeSectionID := testutil.CreateTestSection(t, db, "Recipes", "recipe")
	bookPostID := testutil.CreateTestPost(t, db, userID.String(), sectionID, "Book post")
	recipePostID := testutil.CreateTestPost(t, db, userID.String(), recipeSectionID, "Recipe post")
	handler := NewBookshelfHandler(services.NewBookshelfService(db))
	ctx := createTestUserContext(reqContext(), userID, "bookshelfnotfound", false)

	updateReq := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/bookshelf/categories/"+uuid.New().String(),
		bytes.NewBufferString(`{"name":"Missing","position":0}`),
	)
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq = updateReq.WithContext(ctx)
	updateRR := httptest.NewRecorder()
	handler.UpdateCategory(updateRR, updateReq)
	assertErrorCode(t, updateRR, http.StatusNotFound, "CATEGORY_NOT_FOUND")

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/bookshelf/categories/"+uuid.New().String(), nil)
	deleteReq = deleteReq.WithContext(ctx)
	deleteRR := httptest.NewRecorder()
	handler.DeleteCategory(deleteRR, deleteReq)
	assertErrorCode(t, deleteRR, http.StatusNotFound, "CATEGORY_NOT_FOUND")

	addReq := httptest.NewRequest(http.MethodPost, "/api/v1/posts/"+recipePostID+"/bookshelf", bytes.NewBufferString(`{"categories":["Favorites"]}`))
	addReq.Header.Set("Content-Type", "application/json")
	addReq = addReq.WithContext(ctx)
	addRR := httptest.NewRecorder()
	handler.AddToBookshelf(addRR, addReq)
	assertErrorCode(t, addRR, http.StatusNotFound, "POST_NOT_FOUND")

	removeReq := httptest.NewRequest(http.MethodDelete, "/api/v1/posts/"+bookPostID+"/bookshelf", nil)
	removeReq = removeReq.WithContext(ctx)
	removeRR := httptest.NewRecorder()
	handler.RemoveFromBookshelf(removeRR, removeReq)
	assertErrorCode(t, removeRR, http.StatusNotFound, "BOOKSHELF_ITEM_NOT_FOUND")
}

func assertErrorCode(t *testing.T, rr *httptest.ResponseRecorder, wantStatus int, wantCode string) {
	t.Helper()

	if rr.Code != wantStatus {
		t.Fatalf("expected status %d, got %d. Body: %s", wantStatus, rr.Code, rr.Body.String())
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if response.Code != wantCode {
		t.Fatalf("expected code %s, got %s", wantCode, response.Code)
	}
}
