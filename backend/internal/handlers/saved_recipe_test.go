package handlers

import (
	"bytes"
	"context"
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

func TestSaveRecipeHandlerSuccess(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "savehandleruser", "savehandler@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Recipes", "recipe")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Recipe post")

	service := services.NewSavedRecipeService(db)
	if _, err := service.CreateCategory(reqContext(), uuid.MustParse(userID), "Favorites"); err != nil {
		t.Fatalf("CreateCategory Favorites failed: %v", err)
	}
	if _, err := service.CreateCategory(reqContext(), uuid.MustParse(userID), "Weeknight Dinners"); err != nil {
		t.Fatalf("CreateCategory Weeknight Dinners failed: %v", err)
	}

	handler := NewSavedRecipeHandler(db, nil)

	body := bytes.NewBufferString(`{"categories":["Favorites","Weeknight Dinners"]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts/"+postID+"/save", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "savehandleruser", false))
	rr := httptest.NewRecorder()

	handler.SaveRecipe(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var response models.CreateSavedRecipeResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.SavedRecipes) != 2 {
		t.Fatalf("expected 2 saved recipes, got %d", len(response.SavedRecipes))
	}

	gotCategories := []string{response.SavedRecipes[0].Category, response.SavedRecipes[1].Category}
	sort.Strings(gotCategories)
	if gotCategories[0] != "Favorites" || gotCategories[1] != "Weeknight Dinners" {
		t.Fatalf("unexpected categories: %v", gotCategories)
	}
}

func TestSaveRecipePublishesSectionEvent(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })
	testutil.CleanupRedis(t)

	redisClient := testutil.GetTestRedis(t)

	userID := testutil.CreateTestUser(t, db, "saverecipeevent", "saverecipeevent@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Recipes", "recipe")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Recipe post")

	service := services.NewSavedRecipeService(db)
	if _, err := service.CreateCategory(reqContext(), uuid.MustParse(userID), "Favorites"); err != nil {
		t.Fatalf("CreateCategory Favorites failed: %v", err)
	}

	channel := formatChannel(sectionPrefix, sectionID)
	pubsub := subscribeTestChannel(t, redisClient, channel)

	handler := NewSavedRecipeHandler(db, redisClient)

	body := bytes.NewBufferString(`{"categories":["Favorites"]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts/"+postID+"/save", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "saverecipeevent", false))
	rr := httptest.NewRecorder()

	handler.SaveRecipe(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	event := receiveEvent(t, pubsub)
	if event.Type != "recipe_saved" {
		t.Fatalf("expected recipe_saved event, got %s", event.Type)
	}

	dataBytes, err := json.Marshal(event.Data)
	if err != nil {
		t.Fatalf("failed to marshal event data: %v", err)
	}

	var payload recipeSavedEventData
	if err := json.Unmarshal(dataBytes, &payload); err != nil {
		t.Fatalf("failed to unmarshal recipe saved payload: %v", err)
	}

	if payload.PostID.String() != postID {
		t.Fatalf("expected post_id %s, got %s", postID, payload.PostID.String())
	}
	if payload.UserID.String() != userID {
		t.Fatalf("expected user_id %s, got %s", userID, payload.UserID.String())
	}
	if payload.Username != "saverecipeevent" {
		t.Fatalf("expected username saverecipeevent, got %s", payload.Username)
	}
	if len(payload.Categories) != 1 || payload.Categories[0] != "Favorites" {
		t.Fatalf("expected categories [Favorites], got %v", payload.Categories)
	}
}

func TestUnsaveRecipePublishesSectionEvent(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })
	testutil.CleanupRedis(t)

	redisClient := testutil.GetTestRedis(t)

	userID := testutil.CreateTestUser(t, db, "unsaverecipeevent", "unsaverecipeevent@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Recipes", "recipe")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Recipe post")

	service := services.NewSavedRecipeService(db)
	if _, err := service.SaveRecipe(reqContext(), uuid.MustParse(userID), uuid.MustParse(postID), nil); err != nil {
		t.Fatalf("SaveRecipe failed: %v", err)
	}

	channel := formatChannel(sectionPrefix, sectionID)
	pubsub := subscribeTestChannel(t, redisClient, channel)

	handler := NewSavedRecipeHandler(db, redisClient)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/posts/"+postID+"/save", nil)
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "unsaverecipeevent", false))
	rr := httptest.NewRecorder()

	handler.UnsaveRecipe(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	event := receiveEvent(t, pubsub)
	if event.Type != "recipe_unsaved" {
		t.Fatalf("expected recipe_unsaved event, got %s", event.Type)
	}

	dataBytes, err := json.Marshal(event.Data)
	if err != nil {
		t.Fatalf("failed to marshal event data: %v", err)
	}

	var payload recipeUnsavedEventData
	if err := json.Unmarshal(dataBytes, &payload); err != nil {
		t.Fatalf("failed to unmarshal recipe unsaved payload: %v", err)
	}

	if payload.PostID.String() != postID {
		t.Fatalf("expected post_id %s, got %s", postID, payload.PostID.String())
	}
	if payload.UserID.String() != userID {
		t.Fatalf("expected user_id %s, got %s", userID, payload.UserID.String())
	}
}

func TestSaveRecipeHandlerMissingUser(t *testing.T) {
	handler := &SavedRecipeHandler{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts/"+uuid.New().String()+"/save", bytes.NewBufferString(`{"categories":["Favorites"]}`))
	rr := httptest.NewRecorder()

	handler.SaveRecipe(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rr.Code)
	}
}

func TestUnsaveRecipeHandlerNoContent(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "unsavehandleruser", "unsavehandler@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Recipes", "recipe")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Recipe post")

	service := services.NewSavedRecipeService(db)
	if _, err := service.CreateCategory(reqContext(), uuid.MustParse(userID), "Favorites"); err != nil {
		t.Fatalf("CreateCategory failed: %v", err)
	}
	if _, err := service.SaveRecipe(reqContext(), uuid.MustParse(userID), uuid.MustParse(postID), []string{"Favorites"}); err != nil {
		t.Fatalf("SaveRecipe failed: %v", err)
	}

	handler := NewSavedRecipeHandler(db, nil)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/posts/"+postID+"/save", nil)
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "unsavehandleruser", false))
	rr := httptest.NewRecorder()

	handler.UnsaveRecipe(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", rr.Code)
	}
}

func TestGetPostSavesHandler(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userA := testutil.CreateTestUser(t, db, "saveinfoa", "saveinfoa@test.com", false, true)
	userB := testutil.CreateTestUser(t, db, "saveinfob", "saveinfob@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Recipes", "recipe")
	postID := testutil.CreateTestPost(t, db, userA, sectionID, "Recipe post")

	service := services.NewSavedRecipeService(db)
	if _, err := service.SaveRecipe(reqContext(), uuid.MustParse(userA), uuid.MustParse(postID), nil); err != nil {
		t.Fatalf("SaveRecipe userA failed: %v", err)
	}
	if _, err := service.SaveRecipe(reqContext(), uuid.MustParse(userB), uuid.MustParse(postID), nil); err != nil {
		t.Fatalf("SaveRecipe userB failed: %v", err)
	}

	handler := NewSavedRecipeHandler(db, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/posts/"+postID+"/saves", nil)
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userA), "saveinfoa", false))
	rr := httptest.NewRecorder()

	handler.GetPostSaves(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var response models.PostSaveInfo
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.SaveCount != 2 {
		t.Fatalf("expected save_count 2, got %d", response.SaveCount)
	}
	if !response.ViewerSaved {
		t.Fatalf("expected viewer_saved true")
	}
	if len(response.ViewerCategories) != 1 {
		t.Fatalf("expected 1 viewer category, got %d", len(response.ViewerCategories))
	}
}

func TestListSavedRecipesHandler(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "listsaveduser", "listsaved@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Recipes", "recipe")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Recipe post")

	service := services.NewSavedRecipeService(db)
	if _, err := service.CreateCategory(reqContext(), uuid.MustParse(userID), "Favorites"); err != nil {
		t.Fatalf("CreateCategory failed: %v", err)
	}
	if _, err := service.SaveRecipe(reqContext(), uuid.MustParse(userID), uuid.MustParse(postID), []string{"Favorites"}); err != nil {
		t.Fatalf("SaveRecipe failed: %v", err)
	}

	handler := NewSavedRecipeHandler(db, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/saved-recipes", nil)
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "listsaveduser", false))
	rr := httptest.NewRecorder()

	handler.ListSavedRecipes(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var response models.ListSavedRecipesResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(response.Categories) != 1 {
		t.Fatalf("expected 1 category, got %d", len(response.Categories))
	}
}

func TestRecipeCategoryCRUDHandlers(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "categoryhandleruser", "categoryhandler@test.com", false, true)
	handler := NewSavedRecipeHandler(db, nil)

	createBody := bytes.NewBufferString(`{"name":"Desserts"}`)
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/me/recipe-categories", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq = createReq.WithContext(createTestUserContext(createReq.Context(), uuid.MustParse(userID), "categoryhandleruser", false))
	createRR := httptest.NewRecorder()

	handler.CreateRecipeCategory(createRR, createReq)

	if createRR.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d. Body: %s", createRR.Code, createRR.Body.String())
	}

	var createResp models.CreateRecipeCategoryResponse
	if err := json.NewDecoder(createRR.Body).Decode(&createResp); err != nil {
		t.Fatalf("failed to decode create response: %v", err)
	}

	updateBody := bytes.NewBufferString(`{"name":"Quick Desserts","position":2}`)
	updateReq := httptest.NewRequest(http.MethodPatch, "/api/v1/me/recipe-categories/"+createResp.Category.ID.String(), updateBody)
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq = updateReq.WithContext(createTestUserContext(updateReq.Context(), uuid.MustParse(userID), "categoryhandleruser", false))
	updateRR := httptest.NewRecorder()

	handler.UpdateRecipeCategory(updateRR, updateReq)

	if updateRR.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", updateRR.Code, updateRR.Body.String())
	}

	var updateResp models.UpdateRecipeCategoryResponse
	if err := json.NewDecoder(updateRR.Body).Decode(&updateResp); err != nil {
		t.Fatalf("failed to decode update response: %v", err)
	}
	if updateResp.Category.Name != "Quick Desserts" {
		t.Fatalf("expected updated name, got %s", updateResp.Category.Name)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/me/recipe-categories/"+createResp.Category.ID.String(), nil)
	deleteReq = deleteReq.WithContext(createTestUserContext(deleteReq.Context(), uuid.MustParse(userID), "categoryhandleruser", false))
	deleteRR := httptest.NewRecorder()

	handler.DeleteRecipeCategory(deleteRR, deleteReq)

	if deleteRR.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", deleteRR.Code)
	}
}

func TestUpdateRecipeCategoryHandlerNoUpdates(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "noupdatesuser", "noupdates@test.com", false, true)
	service := services.NewSavedRecipeService(db)
	category, err := service.CreateCategory(reqContext(), uuid.MustParse(userID), "Meals")
	if err != nil {
		t.Fatalf("CreateCategory failed: %v", err)
	}

	handler := NewSavedRecipeHandler(db, nil)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/me/recipe-categories/"+category.ID.String(), bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "noupdatesuser", false))
	rr := httptest.NewRecorder()

	handler.UpdateRecipeCategory(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}

func reqContext() context.Context {
	return context.Background()
}
