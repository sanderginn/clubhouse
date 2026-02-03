package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/sanderginn/clubhouse/internal/middleware"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/observability"
	"github.com/sanderginn/clubhouse/internal/services"
)

// SavedRecipeHandler handles saved recipe endpoints.
type SavedRecipeHandler struct {
	savedRecipeService *services.SavedRecipeService
	postService        *services.PostService
	userService        *services.UserService
	redis              *redis.Client
}

// NewSavedRecipeHandler creates a new saved recipe handler.
func NewSavedRecipeHandler(db *sql.DB, redisClient *redis.Client) *SavedRecipeHandler {
	return &SavedRecipeHandler{
		savedRecipeService: services.NewSavedRecipeService(db),
		postService:        services.NewPostService(db),
		userService:        services.NewUserService(db),
		redis:              redisClient,
	}
}

// SaveRecipe handles POST /api/v1/posts/{postId}/save
func (h *SavedRecipeHandler) SaveRecipe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	postID, err := extractPostIDFromPath(r.URL.Path)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_POST_ID", "Invalid post ID format")
		return
	}

	var req models.CreateSavedRecipeRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	savedRecipes, err := h.savedRecipeService.SaveRecipe(r.Context(), userID, postID, req.Categories)
	if err != nil {
		switch err.Error() {
		case "recipe post not found":
			writeError(r.Context(), w, http.StatusNotFound, "POST_NOT_FOUND", "Post not found")
		case "category not found":
			writeError(r.Context(), w, http.StatusNotFound, "CATEGORY_NOT_FOUND", "Category not found")
		case "category name must be 100 characters or less":
			writeError(r.Context(), w, http.StatusBadRequest, "CATEGORY_NAME_TOO_LONG", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "SAVE_RECIPE_FAILED", "Failed to save recipe")
		}
		return
	}

	response := models.CreateSavedRecipeResponse{
		SavedRecipes: savedRecipes,
	}

	categories := uniqueRecipeCategories(savedRecipes)
	publishCtx, cancel := publishContext()
	username := ""
	if user, err := h.userService.GetUserByID(publishCtx, userID); err == nil {
		username = user.Username
	} else {
		observability.LogWarn(publishCtx, "failed to load user for recipe_saved event",
			"user_id", userID.String(),
			"post_id", postID.String(),
			"error", err.Error(),
		)
	}
	eventData := recipeSavedEventData{
		PostID:     postID,
		UserID:     userID,
		Username:   username,
		Categories: categories,
	}
	_ = publishEvent(publishCtx, h.redis, formatChannel(postPrefix, postID), "recipe_saved", eventData)
	if sectionID, err := h.postService.GetSectionIDByPostID(publishCtx, postID); err == nil {
		_ = publishEvent(publishCtx, h.redis, formatChannel(sectionPrefix, sectionID), "recipe_saved", eventData)
	}
	cancel()

	observability.LogInfo(r.Context(), "recipe saved",
		"user_id", userID.String(),
		"post_id", postID.String(),
		"category_count", strconv.Itoa(len(savedRecipes)),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode save recipe response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusCreated,
			Err:        err,
		})
	}
}

// UnsaveRecipe handles DELETE /api/v1/posts/{postId}/save
func (h *SavedRecipeHandler) UnsaveRecipe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only DELETE requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	postID, err := extractPostIDFromPath(r.URL.Path)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_POST_ID", "Invalid post ID format")
		return
	}

	var category *string
	if categoryParam := strings.TrimSpace(r.URL.Query().Get("category")); categoryParam != "" {
		category = &categoryParam
	}

	if err := h.savedRecipeService.UnsaveRecipe(r.Context(), userID, postID, category); err != nil {
		switch err.Error() {
		case "recipe post not found":
			writeError(r.Context(), w, http.StatusNotFound, "POST_NOT_FOUND", "Post not found")
		case "category name must be 100 characters or less":
			writeError(r.Context(), w, http.StatusBadRequest, "CATEGORY_NAME_TOO_LONG", err.Error())
		case "saved recipe not found":
			w.WriteHeader(http.StatusNoContent)
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "UNSAVE_RECIPE_FAILED", "Failed to unsave recipe")
		}
		return
	}

	observability.LogInfo(r.Context(), "recipe unsaved",
		"user_id", userID.String(),
		"post_id", postID.String(),
	)

	publishCtx, cancel := publishContext()
	eventData := recipeUnsavedEventData{
		PostID: postID,
		UserID: userID,
	}
	_ = publishEvent(publishCtx, h.redis, formatChannel(postPrefix, postID), "recipe_unsaved", eventData)
	if sectionID, err := h.postService.GetSectionIDByPostID(publishCtx, postID); err == nil {
		_ = publishEvent(publishCtx, h.redis, formatChannel(sectionPrefix, sectionID), "recipe_unsaved", eventData)
	}
	cancel()

	w.WriteHeader(http.StatusNoContent)
}

// GetPostSaves handles GET /api/v1/posts/{postId}/saves
func (h *SavedRecipeHandler) GetPostSaves(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	postID, err := extractPostIDFromPath(r.URL.Path)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_POST_ID", "Invalid post ID format")
		return
	}

	info, err := h.savedRecipeService.GetPostSaves(r.Context(), postID, &userID)
	if err != nil {
		if err.Error() == "recipe post not found" {
			writeError(r.Context(), w, http.StatusNotFound, "POST_NOT_FOUND", "Post not found")
			return
		}
		writeError(r.Context(), w, http.StatusInternalServerError, "GET_POST_SAVES_FAILED", "Failed to get post saves")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(info); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode post saves response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// ListSavedRecipes handles GET /api/v1/me/saved-recipes
func (h *SavedRecipeHandler) ListSavedRecipes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	categories, err := h.savedRecipeService.GetUserSavedRecipes(r.Context(), userID)
	if err != nil {
		writeError(r.Context(), w, http.StatusInternalServerError, "GET_SAVED_RECIPES_FAILED", "Failed to get saved recipes")
		return
	}

	response := models.ListSavedRecipesResponse{
		Categories: categories,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode saved recipes response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// ListRecipeCategories handles GET /api/v1/me/recipe-categories
func (h *SavedRecipeHandler) ListRecipeCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	categories, err := h.savedRecipeService.GetUserCategories(r.Context(), userID)
	if err != nil {
		writeError(r.Context(), w, http.StatusInternalServerError, "GET_RECIPE_CATEGORIES_FAILED", "Failed to get recipe categories")
		return
	}

	response := models.ListRecipeCategoriesResponse{
		Categories: categories,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode recipe categories response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

func uniqueRecipeCategories(savedRecipes []models.SavedRecipe) []string {
	if len(savedRecipes) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(savedRecipes))
	categories := make([]string, 0, len(savedRecipes))
	for _, savedRecipe := range savedRecipes {
		category := savedRecipe.Category
		if _, ok := seen[category]; ok {
			continue
		}
		seen[category] = struct{}{}
		categories = append(categories, category)
	}
	return categories
}

// CreateRecipeCategory handles POST /api/v1/me/recipe-categories
func (h *SavedRecipeHandler) CreateRecipeCategory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	var req models.CreateRecipeCategoryRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		writeError(r.Context(), w, http.StatusBadRequest, "CATEGORY_NAME_REQUIRED", "Category name is required")
		return
	}

	category, err := h.savedRecipeService.CreateCategory(r.Context(), userID, req.Name)
	if err != nil {
		switch err.Error() {
		case "category name must be 100 characters or less":
			writeError(r.Context(), w, http.StatusBadRequest, "CATEGORY_NAME_TOO_LONG", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "CREATE_CATEGORY_FAILED", "Failed to create category")
		}
		return
	}

	response := models.CreateRecipeCategoryResponse{
		Category: *category,
	}

	observability.LogInfo(r.Context(), "recipe category created",
		"user_id", userID.String(),
		"category_id", category.ID.String(),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode create recipe category response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusCreated,
			Err:        err,
		})
	}
}

// UpdateRecipeCategory handles PATCH /api/v1/me/recipe-categories/{id}
func (h *SavedRecipeHandler) UpdateRecipeCategory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only PATCH requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	categoryID, err := extractCategoryIDFromPath(r.URL.Path)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_CATEGORY_ID", "Invalid category ID format")
		return
	}

	var req models.UpdateRecipeCategoryRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if req.Name != nil && strings.TrimSpace(*req.Name) == "" {
		writeError(r.Context(), w, http.StatusBadRequest, "CATEGORY_NAME_REQUIRED", "Category name is required")
		return
	}

	if err := h.savedRecipeService.UpdateCategory(r.Context(), userID, categoryID, req.Name, req.Position); err != nil {
		switch err.Error() {
		case "category not found":
			writeError(r.Context(), w, http.StatusNotFound, "CATEGORY_NOT_FOUND", "Category not found")
		case "category name must be 100 characters or less":
			writeError(r.Context(), w, http.StatusBadRequest, "CATEGORY_NAME_TOO_LONG", err.Error())
		case "no updates provided":
			writeError(r.Context(), w, http.StatusBadRequest, "NO_UPDATES", "No updates provided")
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "UPDATE_CATEGORY_FAILED", "Failed to update category")
		}
		return
	}

	updated, err := h.fetchUserCategory(r.Context(), userID, categoryID)
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(r.Context(), w, http.StatusNotFound, "CATEGORY_NOT_FOUND", "Category not found")
			return
		}
		writeError(r.Context(), w, http.StatusInternalServerError, "UPDATE_CATEGORY_FAILED", "Failed to load updated category")
		return
	}

	response := models.UpdateRecipeCategoryResponse{
		Category: *updated,
	}

	observability.LogInfo(r.Context(), "recipe category updated",
		"user_id", userID.String(),
		"category_id", categoryID.String(),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode update recipe category response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// DeleteRecipeCategory handles DELETE /api/v1/me/recipe-categories/{id}
func (h *SavedRecipeHandler) DeleteRecipeCategory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only DELETE requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	categoryID, err := extractCategoryIDFromPath(r.URL.Path)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_CATEGORY_ID", "Invalid category ID format")
		return
	}

	if err := h.savedRecipeService.DeleteCategory(r.Context(), userID, categoryID); err != nil {
		if err.Error() == "category not found" {
			writeError(r.Context(), w, http.StatusNotFound, "CATEGORY_NOT_FOUND", "Category not found")
			return
		}
		writeError(r.Context(), w, http.StatusInternalServerError, "DELETE_CATEGORY_FAILED", "Failed to delete category")
		return
	}

	observability.LogInfo(r.Context(), "recipe category deleted",
		"user_id", userID.String(),
		"category_id", categoryID.String(),
	)

	w.WriteHeader(http.StatusNoContent)
}

func (h *SavedRecipeHandler) fetchUserCategory(ctx context.Context, userID, categoryID uuid.UUID) (*models.RecipeCategory, error) {
	categories, err := h.savedRecipeService.GetUserCategories(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, category := range categories {
		if category.ID == categoryID {
			return &category, nil
		}
	}
	return nil, sql.ErrNoRows
}

func extractCategoryIDFromPath(path string) (uuid.UUID, error) {
	pathParts := strings.Split(path, "/")
	for i, part := range pathParts {
		if part == "recipe-categories" && i+1 < len(pathParts) {
			return uuid.Parse(pathParts[i+1])
		}
	}
	return uuid.Nil, sql.ErrNoRows
}
