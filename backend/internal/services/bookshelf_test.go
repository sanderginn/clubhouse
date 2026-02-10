package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/testutil"
)

func TestBookshelfAddRemoveAndAudit(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.MustParse(testutil.CreateTestUser(t, db, "bookshelfaddremove", "bookshelfaddremove@test.com", false, true))
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postID := uuid.MustParse(testutil.CreateTestPost(t, db, userID.String(), sectionID, "Book post"))

	service := NewBookshelfService(db)
	if err := service.AddToBookshelf(context.Background(), userID, postID, []string{"Favorites"}); err != nil {
		t.Fatalf("AddToBookshelf failed: %v", err)
	}

	if err := service.RemoveFromBookshelf(context.Background(), userID, postID); err != nil {
		t.Fatalf("RemoveFromBookshelf failed: %v", err)
	}

	var activeCount int
	if err := db.QueryRowContext(context.Background(), `
		SELECT COUNT(*)
		FROM bookshelf_items
		WHERE user_id = $1 AND post_id = $2 AND deleted_at IS NULL
	`, userID, postID).Scan(&activeCount); err != nil {
		t.Fatalf("failed to query active bookshelf items: %v", err)
	}
	if activeCount != 0 {
		t.Fatalf("expected 0 active bookshelf items, got %d", activeCount)
	}

	addMetadata := mustQueryAuditMetadata(t, db, "add_to_bookshelf", userID)
	if addMetadata["post_id"] != postID.String() {
		t.Fatalf("expected add_to_bookshelf post_id %s, got %v", postID.String(), addMetadata["post_id"])
	}

	removeMetadata := mustQueryAuditMetadata(t, db, "remove_from_bookshelf", userID)
	if removeMetadata["post_id"] != postID.String() {
		t.Fatalf("expected remove_from_bookshelf post_id %s, got %v", postID.String(), removeMetadata["post_id"])
	}
}

func TestBookshelfAddIsIdempotentForActiveItemRetry(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.MustParse(testutil.CreateTestUser(t, db, "bookshelfretry", "bookshelfretry@test.com", false, true))
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postID := uuid.MustParse(testutil.CreateTestPost(t, db, userID.String(), sectionID, "Book post"))

	service := NewBookshelfService(db)
	if err := service.AddToBookshelf(context.Background(), userID, postID, []string{"Retry Shelf"}); err != nil {
		t.Fatalf("first AddToBookshelf failed: %v", err)
	}
	if err := service.AddToBookshelf(context.Background(), userID, postID, []string{"Retry Shelf"}); err != nil {
		t.Fatalf("second AddToBookshelf retry failed: %v", err)
	}

	var activeCount int
	if err := db.QueryRowContext(context.Background(), `
		SELECT COUNT(*)
		FROM bookshelf_items
		WHERE user_id = $1 AND post_id = $2 AND deleted_at IS NULL
	`, userID, postID).Scan(&activeCount); err != nil {
		t.Fatalf("failed to query active bookshelf item count: %v", err)
	}
	if activeCount != 1 {
		t.Fatalf("expected 1 active bookshelf item after retry, got %d", activeCount)
	}

	var addAuditCount int
	if err := db.QueryRowContext(context.Background(), `
		SELECT COUNT(*)
		FROM audit_logs
		WHERE action = 'add_to_bookshelf' AND target_user_id = $1
	`, userID).Scan(&addAuditCount); err != nil {
		t.Fatalf("failed to query add_to_bookshelf audit count: %v", err)
	}
	if addAuditCount != 1 {
		t.Fatalf("expected 1 add_to_bookshelf audit entry after idempotent retry, got %d", addAuditCount)
	}
}

func TestBookshelfCategoryCRUDWithDeleteRecategorization(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.MustParse(testutil.CreateTestUser(t, db, "bookshelfcategory", "bookshelfcategory@test.com", false, true))
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postID := uuid.MustParse(testutil.CreateTestPost(t, db, userID.String(), sectionID, "Book post"))

	service := NewBookshelfService(db)
	category, err := service.CreateCategory(context.Background(), userID, "To Read")
	if err != nil {
		t.Fatalf("CreateCategory failed: %v", err)
	}

	updated, err := service.UpdateCategory(context.Background(), userID, category.ID, models.UpdateBookshelfCategoryRequest{
		Name:     "Shelf A",
		Position: 3,
	})
	if err != nil {
		t.Fatalf("UpdateCategory failed: %v", err)
	}
	if updated.Name != "Shelf A" || updated.Position != 3 {
		t.Fatalf("unexpected updated category: %+v", updated)
	}

	if err := service.AddToBookshelf(context.Background(), userID, postID, []string{"Shelf A"}); err != nil {
		t.Fatalf("AddToBookshelf failed: %v", err)
	}

	if err := service.DeleteCategory(context.Background(), userID, category.ID); err != nil {
		t.Fatalf("DeleteCategory failed: %v", err)
	}

	var categoryID uuid.NullUUID
	if err := db.QueryRowContext(context.Background(), `
		SELECT category_id
		FROM bookshelf_items
		WHERE user_id = $1 AND post_id = $2 AND deleted_at IS NULL
	`, userID, postID).Scan(&categoryID); err != nil {
		t.Fatalf("failed to query bookshelf item category: %v", err)
	}
	if categoryID.Valid {
		t.Fatalf("expected category_id NULL after delete, got %s", categoryID.UUID.String())
	}

	createMetadata := mustQueryAuditMetadata(t, db, "create_bookshelf_category", userID)
	if createMetadata["category_name"] != "To Read" {
		t.Fatalf("expected create category_name To Read, got %v", createMetadata["category_name"])
	}

	updateMetadata := mustQueryAuditMetadata(t, db, "update_bookshelf_category", userID)
	changesRaw, ok := updateMetadata["changes"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected update changes object, got %T", updateMetadata["changes"])
	}
	if changesRaw["name"] != "Shelf A" {
		t.Fatalf("expected updated name Shelf A, got %v", changesRaw["name"])
	}

	deleteMetadata := mustQueryAuditMetadata(t, db, "delete_bookshelf_category", userID)
	if deleteMetadata["category_id"] != category.ID.String() {
		t.Fatalf("expected delete category_id %s, got %v", category.ID.String(), deleteMetadata["category_id"])
	}
	if deleteMetadata["category_name"] != "Shelf A" {
		t.Fatalf("expected delete category_name Shelf A, got %v", deleteMetadata["category_name"])
	}
}

func TestBookshelfReorderCategories(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.MustParse(testutil.CreateTestUser(t, db, "bookshelfreorder", "bookshelfreorder@test.com", false, true))
	service := NewBookshelfService(db)

	categoryOne, err := service.CreateCategory(context.Background(), userID, "One")
	if err != nil {
		t.Fatalf("CreateCategory One failed: %v", err)
	}
	categoryTwo, err := service.CreateCategory(context.Background(), userID, "Two")
	if err != nil {
		t.Fatalf("CreateCategory Two failed: %v", err)
	}
	categoryThree, err := service.CreateCategory(context.Background(), userID, "Three")
	if err != nil {
		t.Fatalf("CreateCategory Three failed: %v", err)
	}

	if err := service.ReorderCategories(context.Background(), userID, []uuid.UUID{
		categoryThree.ID,
		categoryOne.ID,
		categoryTwo.ID,
	}); err != nil {
		t.Fatalf("ReorderCategories failed: %v", err)
	}

	categories, err := service.GetCategories(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetCategories failed: %v", err)
	}
	if len(categories) != 3 {
		t.Fatalf("expected 3 categories, got %d", len(categories))
	}
	if categories[0].ID != categoryThree.ID || categories[0].Position != 0 {
		t.Fatalf("unexpected first category after reorder: %+v", categories[0])
	}
	if categories[1].ID != categoryOne.ID || categories[1].Position != 1 {
		t.Fatalf("unexpected second category after reorder: %+v", categories[1])
	}
	if categories[2].ID != categoryTwo.ID || categories[2].Position != 2 {
		t.Fatalf("unexpected third category after reorder: %+v", categories[2])
	}
}

func TestBookshelfAutoCreateCategoriesOnAdd(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.MustParse(testutil.CreateTestUser(t, db, "bookshelfautocreate", "bookshelfautocreate@test.com", false, true))
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postID := uuid.MustParse(testutil.CreateTestPost(t, db, userID.String(), sectionID, "Book post"))

	service := NewBookshelfService(db)
	if err := service.AddToBookshelf(context.Background(), userID, postID, []string{"Auto Shelf"}); err != nil {
		t.Fatalf("AddToBookshelf failed: %v", err)
	}

	var (
		categoryID   uuid.UUID
		categoryName string
	)
	if err := db.QueryRowContext(context.Background(), `
		SELECT bc.id, bc.name
		FROM bookshelf_items bi
		JOIN bookshelf_categories bc ON bc.id = bi.category_id
		WHERE bi.user_id = $1 AND bi.post_id = $2 AND bi.deleted_at IS NULL
	`, userID, postID).Scan(&categoryID, &categoryName); err != nil {
		t.Fatalf("failed to query auto-created category mapping: %v", err)
	}
	if categoryName != "Auto Shelf" {
		t.Fatalf("expected Auto Shelf category, got %s", categoryName)
	}

	var categoryCount int
	if err := db.QueryRowContext(context.Background(), `
		SELECT COUNT(*) FROM bookshelf_categories WHERE user_id = $1
	`, userID).Scan(&categoryCount); err != nil {
		t.Fatalf("failed to query category count: %v", err)
	}
	if categoryCount != 1 {
		t.Fatalf("expected 1 auto-created category, got %d", categoryCount)
	}

	metadata := mustQueryAuditMetadata(t, db, "add_to_bookshelf", userID)
	autoCreated, ok := metadata["auto_created_categories"].([]interface{})
	if !ok || len(autoCreated) != 1 || autoCreated[0] != "Auto Shelf" {
		t.Fatalf("expected auto_created_categories [Auto Shelf], got %v", metadata["auto_created_categories"])
	}
}

func TestBookshelfSoftDeleteAndReAddRestoresItem(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.MustParse(testutil.CreateTestUser(t, db, "bookshelfrestore", "bookshelfrestore@test.com", false, true))
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postID := uuid.MustParse(testutil.CreateTestPost(t, db, userID.String(), sectionID, "Book post"))

	service := NewBookshelfService(db)
	if err := service.AddToBookshelf(context.Background(), userID, postID, nil); err != nil {
		t.Fatalf("initial AddToBookshelf failed: %v", err)
	}
	if err := service.RemoveFromBookshelf(context.Background(), userID, postID); err != nil {
		t.Fatalf("RemoveFromBookshelf failed: %v", err)
	}

	var deletedItemID uuid.UUID
	if err := db.QueryRowContext(context.Background(), `
		SELECT id
		FROM bookshelf_items
		WHERE user_id = $1 AND post_id = $2 AND deleted_at IS NOT NULL
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`, userID, postID).Scan(&deletedItemID); err != nil {
		t.Fatalf("failed to query deleted bookshelf item: %v", err)
	}

	if err := service.AddToBookshelf(context.Background(), userID, postID, []string{"Restored Shelf"}); err != nil {
		t.Fatalf("re-add AddToBookshelf failed: %v", err)
	}

	var (
		activeItemID    uuid.UUID
		activeDeletedAt sql.NullTime
	)
	if err := db.QueryRowContext(context.Background(), `
		SELECT id, deleted_at
		FROM bookshelf_items
		WHERE user_id = $1 AND post_id = $2
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`, userID, postID).Scan(&activeItemID, &activeDeletedAt); err != nil {
		t.Fatalf("failed to query restored bookshelf item: %v", err)
	}
	if activeItemID != deletedItemID {
		t.Fatalf("expected restored item ID %s, got %s", deletedItemID, activeItemID)
	}
	if activeDeletedAt.Valid {
		t.Fatalf("expected restored item deleted_at to be NULL")
	}
}

func TestGetBookshelfStatsForPostsAggregation(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userA := uuid.MustParse(testutil.CreateTestUser(t, db, "bookshelfstatsa", "bookshelfstatsa@test.com", false, true))
	userB := uuid.MustParse(testutil.CreateTestUser(t, db, "bookshelfstatsb", "bookshelfstatsb@test.com", false, true))
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postOne := uuid.MustParse(testutil.CreateTestPost(t, db, userA.String(), sectionID, "Book one"))
	postTwo := uuid.MustParse(testutil.CreateTestPost(t, db, userA.String(), sectionID, "Book two"))

	service := NewBookshelfService(db)
	if err := service.AddToBookshelf(context.Background(), userA, postOne, []string{"Favorites"}); err != nil {
		t.Fatalf("AddToBookshelf userA postOne failed: %v", err)
	}
	if err := service.AddToBookshelf(context.Background(), userB, postOne, nil); err != nil {
		t.Fatalf("AddToBookshelf userB postOne failed: %v", err)
	}
	if err := service.AddToBookshelf(context.Background(), userA, postTwo, nil); err != nil {
		t.Fatalf("AddToBookshelf userA postTwo failed: %v", err)
	}

	stats, err := service.GetBookshelfStatsForPosts(context.Background(), []uuid.UUID{postOne, postTwo}, &userA)
	if err != nil {
		t.Fatalf("GetBookshelfStatsForPosts failed: %v", err)
	}

	postOneStats := stats[postOne]
	if postOneStats == nil {
		t.Fatalf("expected stats for post one")
	}
	if postOneStats.SaveCount != 2 {
		t.Fatalf("expected post one save_count 2, got %d", postOneStats.SaveCount)
	}
	if !postOneStats.ViewerSaved {
		t.Fatalf("expected post one ViewerSaved true")
	}
	if len(postOneStats.ViewerCategories) != 1 || postOneStats.ViewerCategories[0] != "Favorites" {
		t.Fatalf("expected post one viewer categories [Favorites], got %v", postOneStats.ViewerCategories)
	}

	postTwoStats := stats[postTwo]
	if postTwoStats == nil {
		t.Fatalf("expected stats for post two")
	}
	if postTwoStats.SaveCount != 1 {
		t.Fatalf("expected post two save_count 1, got %d", postTwoStats.SaveCount)
	}
	if !postTwoStats.ViewerSaved {
		t.Fatalf("expected post two ViewerSaved true")
	}
	if len(postTwoStats.ViewerCategories) != 1 || postTwoStats.ViewerCategories[0] != defaultBookshelfCategoryName {
		t.Fatalf("expected post two viewer categories [%s], got %v", defaultBookshelfCategoryName, postTwoStats.ViewerCategories)
	}
}

func TestGetUserAndAllBookshelfItemsPagination(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userA := uuid.MustParse(testutil.CreateTestUser(t, db, "bookshelflista", "bookshelflista@test.com", false, true))
	userB := uuid.MustParse(testutil.CreateTestUser(t, db, "bookshelflistb", "bookshelflistb@test.com", false, true))
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postOne := uuid.MustParse(testutil.CreateTestPost(t, db, userA.String(), sectionID, "Book one"))
	postTwo := uuid.MustParse(testutil.CreateTestPost(t, db, userA.String(), sectionID, "Book two"))
	postThree := uuid.MustParse(testutil.CreateTestPost(t, db, userA.String(), sectionID, "Book three"))

	service := NewBookshelfService(db)
	if err := service.AddToBookshelf(context.Background(), userA, postOne, []string{"Favorites"}); err != nil {
		t.Fatalf("AddToBookshelf postOne failed: %v", err)
	}
	if err := service.AddToBookshelf(context.Background(), userA, postTwo, nil); err != nil {
		t.Fatalf("AddToBookshelf postTwo failed: %v", err)
	}
	if err := service.AddToBookshelf(context.Background(), userB, postThree, []string{"Favorites"}); err != nil {
		t.Fatalf("AddToBookshelf postThree failed: %v", err)
	}

	pageOne, nextCursor, err := service.GetUserBookshelf(context.Background(), userA, nil, nil, 1)
	if err != nil {
		t.Fatalf("GetUserBookshelf page one failed: %v", err)
	}
	if len(pageOne) != 1 {
		t.Fatalf("expected 1 item on page one, got %d", len(pageOne))
	}
	if nextCursor == nil || *nextCursor == "" {
		t.Fatalf("expected next cursor on page one")
	}

	pageTwo, finalCursor, err := service.GetUserBookshelf(context.Background(), userA, nil, nextCursor, 1)
	if err != nil {
		t.Fatalf("GetUserBookshelf page two failed: %v", err)
	}
	if len(pageTwo) != 1 {
		t.Fatalf("expected 1 item on page two, got %d", len(pageTwo))
	}
	if finalCursor != nil {
		t.Fatalf("expected no cursor on final page")
	}

	favorites := "Favorites"
	allFavorites, allCursor, err := service.GetAllBookshelfItems(context.Background(), &favorites, nil, 10)
	if err != nil {
		t.Fatalf("GetAllBookshelfItems failed: %v", err)
	}
	if len(allFavorites) != 2 {
		t.Fatalf("expected 2 favorites entries across users, got %d", len(allFavorites))
	}
	if allCursor != nil {
		t.Fatalf("expected no next cursor for full favorites page")
	}

	uncategorized := defaultBookshelfCategoryName
	userUncategorized, _, err := service.GetUserBookshelf(context.Background(), userA, &uncategorized, nil, 10)
	if err != nil {
		t.Fatalf("GetUserBookshelf uncategorized filter failed: %v", err)
	}
	if len(userUncategorized) != 1 || userUncategorized[0].PostID != postTwo {
		t.Fatalf("expected uncategorized result for post %s, got %+v", postTwo.String(), userUncategorized)
	}
}

func mustQueryAuditMetadata(t *testing.T, db *sql.DB, action string, userID uuid.UUID) map[string]interface{} {
	t.Helper()

	var raw []byte
	if err := db.QueryRowContext(context.Background(), `
		SELECT metadata
		FROM audit_logs
		WHERE action = $1 AND target_user_id = $2
		ORDER BY created_at DESC
		LIMIT 1
	`, action, userID).Scan(&raw); err != nil {
		t.Fatalf("failed to query audit log for action %s: %v", action, err)
	}

	metadata := make(map[string]interface{})
	if err := json.Unmarshal(raw, &metadata); err != nil {
		t.Fatalf("failed to unmarshal audit metadata for action %s: %v", action, err)
	}

	return metadata
}
