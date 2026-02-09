package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/testutil"
)

func TestAddToWatchlistIdempotentAndAudit(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "watchlistuser", "watchlist@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Movies", "movie")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Movie post")

	service := NewWatchlistService(db)
	_, err := service.CreateCategory(context.Background(), uuid.MustParse(userID), "Favorites")
	if err != nil {
		t.Fatalf("CreateCategory failed: %v", err)
	}

	_, err = service.AddToWatchlist(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), []string{"Favorites", "Favorites"})
	if err != nil {
		t.Fatalf("AddToWatchlist failed: %v", err)
	}

	_, err = service.AddToWatchlist(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), []string{"Favorites"})
	if err != nil {
		t.Fatalf("AddToWatchlist duplicate failed: %v", err)
	}

	var activeCount int
	if err := db.QueryRowContext(
		context.Background(),
		"SELECT COUNT(*) FROM watchlist_items WHERE user_id = $1 AND post_id = $2 AND deleted_at IS NULL",
		uuid.MustParse(userID),
		uuid.MustParse(postID),
	).Scan(&activeCount); err != nil {
		t.Fatalf("failed to count watchlist items: %v", err)
	}
	if activeCount != 1 {
		t.Fatalf("expected 1 active watchlist item, got %d", activeCount)
	}

	var metadataBytes []byte
	query := `
		SELECT metadata
		FROM audit_logs
		WHERE action = 'add_to_watchlist' AND target_user_id = $1
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

func TestAddToWatchlistRejectsNonMovieAndSeriesPosts(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "watchlistinvalid", "watchlistinvalid@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Recipes", "recipe")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Recipe post")

	service := NewWatchlistService(db)
	_, err := service.AddToWatchlist(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), nil)
	if err == nil {
		t.Fatalf("expected AddToWatchlist to fail for non-movie post")
	}
}

func TestRemoveFromWatchlistAllCategories(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "watchlistremove", "watchlistremove@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Movies", "movie")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Movie post")

	service := NewWatchlistService(db)
	_, err := service.CreateCategory(context.Background(), uuid.MustParse(userID), "Favorites")
	if err != nil {
		t.Fatalf("CreateCategory failed: %v", err)
	}

	_, err = service.AddToWatchlist(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), []string{"Favorites"})
	if err != nil {
		t.Fatalf("AddToWatchlist favorites failed: %v", err)
	}
	_, err = service.AddToWatchlist(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), nil)
	if err != nil {
		t.Fatalf("AddToWatchlist uncategorized failed: %v", err)
	}

	if err := service.RemoveFromWatchlist(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), nil); err != nil {
		t.Fatalf("RemoveFromWatchlist failed: %v", err)
	}

	var remaining int
	if err := db.QueryRowContext(
		context.Background(),
		"SELECT COUNT(*) FROM watchlist_items WHERE user_id = $1 AND post_id = $2 AND deleted_at IS NULL",
		uuid.MustParse(userID),
		uuid.MustParse(postID),
	).Scan(&remaining); err != nil {
		t.Fatalf("failed to count remaining watchlist items: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("expected no active watchlist items, got %d", remaining)
	}

	var metadataBytes []byte
	query := `
		SELECT metadata
		FROM audit_logs
		WHERE action = 'remove_from_watchlist' AND target_user_id = $1
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
		t.Errorf("expected category key in audit metadata")
	}
}

func TestGetPostWatchlistInfoIncludesViewer(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userA := testutil.CreateTestUser(t, db, "watchlistviewer", "watchlistviewer@test.com", false, true)
	userB := testutil.CreateTestUser(t, db, "watchlistviewerb", "watchlistviewerb@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Movies", "movie")
	postID := testutil.CreateTestPost(t, db, userA, sectionID, "Movie post")

	service := NewWatchlistService(db)
	_, err := service.AddToWatchlist(context.Background(), uuid.MustParse(userA), uuid.MustParse(postID), nil)
	if err != nil {
		t.Fatalf("AddToWatchlist userA failed: %v", err)
	}
	_, err = service.AddToWatchlist(context.Background(), uuid.MustParse(userB), uuid.MustParse(postID), nil)
	if err != nil {
		t.Fatalf("AddToWatchlist userB failed: %v", err)
	}

	info, err := service.GetPostWatchlistInfo(context.Background(), uuid.MustParse(postID), ptrUUIDWatchlist(uuid.MustParse(userA)))
	if err != nil {
		t.Fatalf("GetPostWatchlistInfo failed: %v", err)
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
	if len(info.ViewerCategories) != 1 || info.ViewerCategories[0] != defaultWatchlistCategory {
		t.Fatalf("expected viewer categories [%s], got %v", defaultWatchlistCategory, info.ViewerCategories)
	}
}

func TestWatchlistCategoryCRUDWithAudit(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "watchlistcategory", "watchlistcategory@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Movies", "movie")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Movie post")

	service := NewWatchlistService(db)
	category, err := service.CreateCategory(context.Background(), uuid.MustParse(userID), "To Watch")
	if err != nil {
		t.Fatalf("CreateCategory failed: %v", err)
	}

	categories, err := service.GetUserWatchlistCategories(context.Background(), uuid.MustParse(userID))
	if err != nil {
		t.Fatalf("GetUserWatchlistCategories failed: %v", err)
	}
	if len(categories) != 1 || categories[0].ID != category.ID {
		t.Fatalf("expected one category with id %s, got %+v", category.ID, categories)
	}

	_, err = service.AddToWatchlist(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), []string{"To Watch"})
	if err != nil {
		t.Fatalf("AddToWatchlist failed: %v", err)
	}

	newName := "Top Picks"
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
		"SELECT category FROM watchlist_items WHERE user_id = $1 AND post_id = $2 AND deleted_at IS NULL",
		uuid.MustParse(userID),
		uuid.MustParse(postID),
	).Scan(&updatedCategory); err != nil {
		t.Fatalf("failed to verify recategorized watchlist item: %v", err)
	}
	if updatedCategory != defaultWatchlistCategory {
		t.Fatalf("expected category %q after delete, got %q", defaultWatchlistCategory, updatedCategory)
	}

	var deleteMetadata []byte
	if err := db.QueryRowContext(
		context.Background(),
		"SELECT metadata FROM audit_logs WHERE action = 'delete_watchlist_category' AND target_user_id = $1 ORDER BY created_at DESC LIMIT 1",
		uuid.MustParse(userID),
	).Scan(&deleteMetadata); err != nil {
		t.Fatalf("failed to query delete audit log: %v", err)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(deleteMetadata, &metadata); err != nil {
		t.Fatalf("failed to unmarshal delete metadata: %v", err)
	}
	if metadata["category_id"] != category.ID.String() {
		t.Errorf("expected category_id %s, got %v", category.ID.String(), metadata["category_id"])
	}
	if metadata["category_name"] != newName {
		t.Errorf("expected category_name %q, got %v", newName, metadata["category_name"])
	}
}

func TestDeleteCategoryHandlesOverlappingUncategorizedItems(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "watchlistoverlap", "watchlistoverlap@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Movies", "movie")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Movie post")

	service := NewWatchlistService(db)
	category, err := service.CreateCategory(context.Background(), uuid.MustParse(userID), "Favorites")
	if err != nil {
		t.Fatalf("CreateCategory failed: %v", err)
	}

	_, err = service.AddToWatchlist(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), []string{"Favorites"})
	if err != nil {
		t.Fatalf("AddToWatchlist favorites failed: %v", err)
	}
	_, err = service.AddToWatchlist(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), nil)
	if err != nil {
		t.Fatalf("AddToWatchlist uncategorized failed: %v", err)
	}

	if err := service.DeleteCategory(context.Background(), uuid.MustParse(userID), category.ID); err != nil {
		t.Fatalf("DeleteCategory failed: %v", err)
	}

	var remaining int
	if err := db.QueryRowContext(
		context.Background(),
		`SELECT COUNT(*) FROM watchlist_items
		WHERE user_id = $1 AND post_id = $2 AND category = $3 AND deleted_at IS NULL`,
		uuid.MustParse(userID),
		uuid.MustParse(postID),
		defaultWatchlistCategory,
	).Scan(&remaining); err != nil {
		t.Fatalf("failed to count remaining uncategorized rows: %v", err)
	}
	if remaining != 1 {
		t.Fatalf("expected one remaining uncategorized row, got %d", remaining)
	}
}

func TestUpdateCategoryRenameToUncategorizedHandlesOverlap(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "watchlistrenameoverlap", "watchlistrenameoverlap@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Movies", "movie")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Movie post")

	service := NewWatchlistService(db)
	category, err := service.CreateCategory(context.Background(), uuid.MustParse(userID), "To Watch")
	if err != nil {
		t.Fatalf("CreateCategory failed: %v", err)
	}

	_, err = service.AddToWatchlist(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), []string{"To Watch"})
	if err != nil {
		t.Fatalf("AddToWatchlist To Watch failed: %v", err)
	}
	_, err = service.AddToWatchlist(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), nil)
	if err != nil {
		t.Fatalf("AddToWatchlist uncategorized failed: %v", err)
	}

	newName := defaultWatchlistCategory
	if err := service.UpdateCategory(context.Background(), uuid.MustParse(userID), category.ID, &newName, nil); err != nil {
		t.Fatalf("UpdateCategory failed: %v", err)
	}

	var remaining int
	if err := db.QueryRowContext(
		context.Background(),
		`SELECT COUNT(*) FROM watchlist_items
		WHERE user_id = $1 AND post_id = $2 AND category = $3 AND deleted_at IS NULL`,
		uuid.MustParse(userID),
		uuid.MustParse(postID),
		defaultWatchlistCategory,
	).Scan(&remaining); err != nil {
		t.Fatalf("failed to count remaining uncategorized rows: %v", err)
	}
	if remaining != 1 {
		t.Fatalf("expected one remaining uncategorized row, got %d", remaining)
	}
}

func TestGetUserWatchlistGroupsByCategoryAndIncludesPost(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "watchlistgroup", "watchlistgroup@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Movies", "movie")
	postA := testutil.CreateTestPost(t, db, userID, sectionID, "Movie A")
	postB := testutil.CreateTestPost(t, db, userID, sectionID, "Movie B")

	service := NewWatchlistService(db)
	_, err := service.CreateCategory(context.Background(), uuid.MustParse(userID), "Favorites")
	if err != nil {
		t.Fatalf("CreateCategory failed: %v", err)
	}

	_, err = service.AddToWatchlist(context.Background(), uuid.MustParse(userID), uuid.MustParse(postA), []string{"Favorites"})
	if err != nil {
		t.Fatalf("AddToWatchlist favorites failed: %v", err)
	}
	_, err = service.AddToWatchlist(context.Background(), uuid.MustParse(userID), uuid.MustParse(postB), nil)
	if err != nil {
		t.Fatalf("AddToWatchlist uncategorized failed: %v", err)
	}

	grouped, err := service.GetUserWatchlist(context.Background(), uuid.MustParse(userID), nil)
	if err != nil {
		t.Fatalf("GetUserWatchlist failed: %v", err)
	}

	if len(grouped) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(grouped))
	}

	favorites := grouped["Favorites"]
	if len(favorites) != 1 {
		t.Fatalf("expected 1 Favorites item, got %d", len(favorites))
	}
	if favorites[0].Post == nil || favorites[0].Post.ID != uuid.MustParse(postA) {
		t.Fatalf("expected Favorites item post %s, got %+v", postA, favorites[0].Post)
	}

	uncategorized := grouped[defaultWatchlistCategory]
	if len(uncategorized) != 1 {
		t.Fatalf("expected 1 Uncategorized item, got %d", len(uncategorized))
	}
	if uncategorized[0].Post == nil || uncategorized[0].Post.ID != uuid.MustParse(postB) {
		t.Fatalf("expected Uncategorized item post %s, got %+v", postB, uncategorized[0].Post)
	}
}

func TestGetUserWatchlistFiltersBySectionType(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "watchlistsectionfilter", "watchlistsectionfilter@test.com", false, true)
	movieSectionID := testutil.CreateTestSection(t, db, "Movies", "movie")
	seriesSectionID := testutil.CreateTestSection(t, db, "Series", "series")
	moviePostID := testutil.CreateTestPost(t, db, userID, movieSectionID, "Movie post")
	seriesPostID := testutil.CreateTestPost(t, db, userID, seriesSectionID, "Series post")

	service := NewWatchlistService(db)
	_, err := service.CreateCategory(context.Background(), uuid.MustParse(userID), "Favorites")
	if err != nil {
		t.Fatalf("CreateCategory failed: %v", err)
	}

	_, err = service.AddToWatchlist(context.Background(), uuid.MustParse(userID), uuid.MustParse(moviePostID), []string{"Favorites"})
	if err != nil {
		t.Fatalf("AddToWatchlist movie failed: %v", err)
	}
	_, err = service.AddToWatchlist(context.Background(), uuid.MustParse(userID), uuid.MustParse(seriesPostID), []string{"Favorites"})
	if err != nil {
		t.Fatalf("AddToWatchlist series failed: %v", err)
	}

	movieType := "movie"
	movieOnly, err := service.GetUserWatchlist(context.Background(), uuid.MustParse(userID), &movieType)
	if err != nil {
		t.Fatalf("GetUserWatchlist movie filter failed: %v", err)
	}
	if len(movieOnly["Favorites"]) != 1 {
		t.Fatalf("expected 1 movie item, got %d", len(movieOnly["Favorites"]))
	}
	if movieOnly["Favorites"][0].PostID != uuid.MustParse(moviePostID) {
		t.Fatalf("expected movie post %s, got %s", moviePostID, movieOnly["Favorites"][0].PostID)
	}

	seriesType := "series"
	seriesOnly, err := service.GetUserWatchlist(context.Background(), uuid.MustParse(userID), &seriesType)
	if err != nil {
		t.Fatalf("GetUserWatchlist series filter failed: %v", err)
	}
	if len(seriesOnly["Favorites"]) != 1 {
		t.Fatalf("expected 1 series item, got %d", len(seriesOnly["Favorites"]))
	}
	if seriesOnly["Favorites"][0].PostID != uuid.MustParse(seriesPostID) {
		t.Fatalf("expected series post %s, got %s", seriesPostID, seriesOnly["Favorites"][0].PostID)
	}
}

func ptrUUIDWatchlist(id uuid.UUID) *uuid.UUID {
	return &id
}
