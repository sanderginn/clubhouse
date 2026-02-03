package models

import (
	"time"

	"github.com/google/uuid"
)

type SavedRecipe struct {
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"user_id"`
	PostID    uuid.UUID  `json:"post_id"`
	Category  string     `json:"category"`
	CreatedAt time.Time  `json:"created_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

type RecipeCategory struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Name      string    `json:"name"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
}

// PostSaveInfo represents save tooltip data for a post.
type PostSaveInfo struct {
	SaveCount      int            `json:"save_count"`
	Users          []ReactionUser `json:"users"`
	ViewerSaved    bool           `json:"viewer_saved"`
	ViewerCategory *string        `json:"viewer_category,omitempty"`
}

type SavedRecipeCategory struct {
	Name    string                `json:"name"`
	Recipes []SavedRecipeWithPost `json:"recipes"`
}

type SavedRecipeWithPost struct {
	SavedRecipe
	Post *Post `json:"post,omitempty"`
}

// CreateSavedRecipeRequest represents the request body for saving a recipe.
type CreateSavedRecipeRequest struct {
	PostID   string  `json:"post_id"`
	Category *string `json:"category,omitempty"`
}

// CreateSavedRecipeResponse represents the response for saving a recipe.
type CreateSavedRecipeResponse struct {
	SavedRecipe SavedRecipe `json:"saved_recipe"`
}

// DeleteSavedRecipeResponse represents the response for removing a saved recipe.
type DeleteSavedRecipeResponse struct {
	SavedRecipe *SavedRecipe `json:"saved_recipe"`
	Message     string       `json:"message"`
}

// ListSavedRecipesResponse represents the response for listing saved recipes grouped by category.
type ListSavedRecipesResponse struct {
	Categories []SavedRecipeCategory `json:"categories"`
}

// GetPostSaveInfoResponse represents the response for post save tooltip data.
type GetPostSaveInfoResponse struct {
	SaveInfo PostSaveInfo `json:"save_info"`
}

// CreateRecipeCategoryRequest represents the request body for creating a recipe category.
type CreateRecipeCategoryRequest struct {
	Name     string `json:"name"`
	Position *int   `json:"position,omitempty"`
}

// UpdateRecipeCategoryRequest represents the request body for updating a recipe category.
type UpdateRecipeCategoryRequest struct {
	Name     *string `json:"name,omitempty"`
	Position *int    `json:"position,omitempty"`
}

// CreateRecipeCategoryResponse represents the response for creating a recipe category.
type CreateRecipeCategoryResponse struct {
	Category RecipeCategory `json:"category"`
}

// UpdateRecipeCategoryResponse represents the response for updating a recipe category.
type UpdateRecipeCategoryResponse struct {
	Category RecipeCategory `json:"category"`
}

// DeleteRecipeCategoryResponse represents the response for deleting a recipe category.
type DeleteRecipeCategoryResponse struct {
	ID      uuid.UUID `json:"id"`
	Message string    `json:"message"`
}

// ListRecipeCategoriesResponse represents the response for listing recipe categories.
type ListRecipeCategoriesResponse struct {
	Categories []RecipeCategory `json:"categories"`
}
