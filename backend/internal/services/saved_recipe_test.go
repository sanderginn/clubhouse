package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/testutil"
)

func TestSaveRecipeIdempotentAndAudit(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "saverecipeuser", "saverecipe@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Recipes", "recipe")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Recipe post")

	service := NewSavedRecipeService(db)
	_, err := service.CreateCategory(context.Background(), uuid.MustParse(userID), "Favorites")
	if err != nil {
		t.Fatalf("CreateCategory failed: %v", err)
	}

	_, err = service.SaveRecipe(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), []string{"Favorites"})
	if err != nil {
		t.Fatalf("SaveRecipe failed: %v", err)
	}

	_, err = service.SaveRecipe(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), []string{"Favorites"})
	if err != nil {
		t.Fatalf("SaveRecipe duplicate failed: %v", err)
	}

	var savedCount int
	if err := db.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM saved_recipes WHERE user_id = $1 AND post_id = $2 AND deleted_at IS NULL",
		uuid.MustParse(userID), uuid.MustParse(postID),
	).Scan(&savedCount); err != nil {
		t.Fatalf("failed to count saved recipes: %v", err)
	}
	if savedCount != 1 {
		t.Fatalf("expected 1 saved recipe, got %d", savedCount)
	}

	var metadataBytes []byte
	query := `
		SELECT metadata
		FROM audit_logs
		WHERE action = 'save_recipe' AND target_user_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`
	if err := db.QueryRowContext(context.Background(), query, uuid.MustParse(userID)).Scan(&metadataBytes); err != nil {
		t.Fatalf("failed to query audit log: %v", err)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}

	if metadata["post_id"] != postID {
		t.Errorf("expected post_id %s, got %v", postID, metadata["post_id"])
	}
	if categories, ok := metadata["categories"].([]interface{}); !ok || len(categories) != 1 || categories[0] != "Favorites" {
		t.Errorf("expected categories [Favorites], got %v", metadata["categories"])
	}
}

func TestUnsaveRecipeAllCategories(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "unsaverecipeuser", "unsaverecipe@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Recipes", "recipe")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Recipe post")

	service := NewSavedRecipeService(db)
	_, err := service.CreateCategory(context.Background(), uuid.MustParse(userID), "Favorites")
	if err != nil {
		t.Fatalf("CreateCategory failed: %v", err)
	}

	_, err = service.SaveRecipe(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), []string{"Favorites"})
	if err != nil {
		t.Fatalf("SaveRecipe failed: %v", err)
	}
	_, err = service.SaveRecipe(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), []string{""})
	if err != nil {
		t.Fatalf("SaveRecipe uncategorized failed: %v", err)
	}

	if err := service.UnsaveRecipe(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), nil); err != nil {
		t.Fatalf("UnsaveRecipe failed: %v", err)
	}

	var remaining int
	if err := db.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM saved_recipes WHERE user_id = $1 AND post_id = $2 AND deleted_at IS NULL",
		uuid.MustParse(userID), uuid.MustParse(postID),
	).Scan(&remaining); err != nil {
		t.Fatalf("failed to count remaining saves: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("expected no active saves, got %d", remaining)
	}

	var metadataBytes []byte
	query := `
		SELECT metadata
		FROM audit_logs
		WHERE action = 'unsave_recipe' AND target_user_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`
	if err := db.QueryRowContext(context.Background(), query, uuid.MustParse(userID)).Scan(&metadataBytes); err != nil {
		t.Fatalf("failed to query audit log: %v", err)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}
	if metadata["post_id"] != postID {
		t.Errorf("expected post_id %s, got %v", postID, metadata["post_id"])
	}
	if _, ok := metadata["category"]; !ok {
		t.Errorf("expected category to be present in metadata")
	}
}

func TestGetPostSavesIncludesViewer(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userA := testutil.CreateTestUser(t, db, "saveviewer", "saveviewer@test.com", false, true)
	userB := testutil.CreateTestUser(t, db, "saveviewerb", "saveviewerb@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Recipes", "recipe")
	postID := testutil.CreateTestPost(t, db, userA, sectionID, "Recipe post")

	service := NewSavedRecipeService(db)
	_, err := service.SaveRecipe(context.Background(), uuid.MustParse(userA), uuid.MustParse(postID), nil)
	if err != nil {
		t.Fatalf("SaveRecipe userA failed: %v", err)
	}
	_, err = service.SaveRecipe(context.Background(), uuid.MustParse(userB), uuid.MustParse(postID), nil)
	if err != nil {
		t.Fatalf("SaveRecipe userB failed: %v", err)
	}

	info, err := service.GetPostSaves(context.Background(), uuid.MustParse(postID), ptrUUID(uuid.MustParse(userA)))
	if err != nil {
		t.Fatalf("GetPostSaves failed: %v", err)
	}

	if info.SaveCount != 2 {
		t.Fatalf("expected save count 2, got %d", info.SaveCount)
	}
	if len(info.Users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(info.Users))
	}
	if !info.ViewerSaved {
		t.Fatalf("expected viewer_saved true")
	}
	if len(info.ViewerCategories) != 1 || info.ViewerCategories[0] != defaultRecipeCategory {
		t.Fatalf("expected viewer category %q, got %v", defaultRecipeCategory, info.ViewerCategories)
	}
}

func TestCategoryCRUDWithAudit(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "categoryuser", "categoryuser@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Recipes", "recipe")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Recipe post")

	service := NewSavedRecipeService(db)
	category, err := service.CreateCategory(context.Background(), uuid.MustParse(userID), "Desserts")
	if err != nil {
		t.Fatalf("CreateCategory failed: %v", err)
	}

	_, err = service.SaveRecipe(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), []string{"Desserts"})
	if err != nil {
		t.Fatalf("SaveRecipe failed: %v", err)
	}

	newName := "Quick Desserts"
	newPosition := 2
	if err := service.UpdateCategory(context.Background(), uuid.MustParse(userID), category.ID, &newName, &newPosition); err != nil {
		t.Fatalf("UpdateCategory failed: %v", err)
	}

	if err := service.DeleteCategory(context.Background(), uuid.MustParse(userID), category.ID); err != nil {
		t.Fatalf("DeleteCategory failed: %v", err)
	}

	var updatedCategory string
	if err := db.QueryRowContext(
		context.Background(),
		"SELECT category FROM saved_recipes WHERE user_id = $1 AND post_id = $2 AND deleted_at IS NULL",
		uuid.MustParse(userID),
		uuid.MustParse(postID),
	).Scan(&updatedCategory); err != nil {
		t.Fatalf("failed to check recategorized recipe: %v", err)
	}
	if updatedCategory != defaultRecipeCategory {
		t.Fatalf("expected recategorized recipe to be %q, got %q", defaultRecipeCategory, updatedCategory)
	}

	var deleteMetadata []byte
	if err := db.QueryRowContext(
		context.Background(),
		"SELECT metadata FROM audit_logs WHERE action = 'delete_recipe_category' AND target_user_id = $1 ORDER BY created_at DESC LIMIT 1",
		uuid.MustParse(userID),
	).Scan(&deleteMetadata); err != nil {
		t.Fatalf("failed to query delete audit log: %v", err)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(deleteMetadata, &metadata); err != nil {
		t.Fatalf("failed to unmarshal delete audit metadata: %v", err)
	}
	if metadata["category_id"] != category.ID.String() {
		t.Errorf("expected category_id %s, got %v", category.ID.String(), metadata["category_id"])
	}
	if metadata["category_name"] != "Quick Desserts" {
		t.Errorf("expected category_name Quick Desserts, got %v", metadata["category_name"])
	}
}

func TestGetUserSavedRecipesGroupsByCategory(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "savedrecipelistuser", "savedrecipelist@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Recipes", "recipe")
	postA := testutil.CreateTestPost(t, db, userID, sectionID, "Recipe A")
	postB := testutil.CreateTestPost(t, db, userID, sectionID, "Recipe B")

	service := NewSavedRecipeService(db)
	_, err := service.CreateCategory(context.Background(), uuid.MustParse(userID), "Favorites")
	if err != nil {
		t.Fatalf("CreateCategory failed: %v", err)
	}

	_, err = service.SaveRecipe(context.Background(), uuid.MustParse(userID), uuid.MustParse(postA), []string{"Favorites"})
	if err != nil {
		t.Fatalf("SaveRecipe favorites failed: %v", err)
	}
	_, err = service.SaveRecipe(context.Background(), uuid.MustParse(userID), uuid.MustParse(postB), nil)
	if err != nil {
		t.Fatalf("SaveRecipe uncategorized failed: %v", err)
	}

	categories, err := service.GetUserSavedRecipes(context.Background(), uuid.MustParse(userID))
	if err != nil {
		t.Fatalf("GetUserSavedRecipes failed: %v", err)
	}

	if len(categories) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(categories))
	}
}

func ptrUUID(id uuid.UUID) *uuid.UUID {
	return &id
}
