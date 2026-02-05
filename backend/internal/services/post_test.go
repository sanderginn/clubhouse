package services

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/testutil"
)

func TestCreatePostWithoutLinks(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	disableLinkMetadata(t)

	userID := testutil.CreateTestUser(t, db, "postuser", "postuser@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Test Section", "general")

	service := NewPostService(db)
	req := &models.CreatePostRequest{
		SectionID: sectionID,
		Content:   "Hello world",
	}

	post, err := service.CreatePost(context.Background(), req, uuid.MustParse(userID))
	if err != nil {
		t.Fatalf("CreatePost failed: %v", err)
	}

	if post.Content != "Hello world" {
		t.Errorf("expected content 'Hello world', got %s", post.Content)
	}
	if post.SectionID.String() != sectionID {
		t.Errorf("expected section_id %s, got %s", sectionID, post.SectionID.String())
	}
	if len(post.Links) != 0 {
		t.Errorf("expected no links, got %d", len(post.Links))
	}
}

func TestCreatePostWithLinks(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	disableLinkMetadata(t)

	userID := testutil.CreateTestUser(t, db, "postlinkuser", "postlinkuser@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Links Section", "general")

	service := NewPostService(db)
	req := &models.CreatePostRequest{
		SectionID: sectionID,
		Content:   "Check this out",
		Links: []models.LinkRequest{
			{URL: "https://example.com"},
		},
	}

	post, err := service.CreatePost(context.Background(), req, uuid.MustParse(userID))
	if err != nil {
		t.Fatalf("CreatePost failed: %v", err)
	}

	if len(post.Links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(post.Links))
	}
	if post.Links[0].URL != "https://example.com" {
		t.Errorf("expected link url https://example.com, got %s", post.Links[0].URL)
	}

	var metadataIsNull bool
	err = db.QueryRow(`SELECT metadata IS NULL FROM links WHERE post_id = $1`, post.ID).Scan(&metadataIsNull)
	if err != nil {
		t.Fatalf("failed to query link metadata: %v", err)
	}
	if !metadataIsNull {
		t.Errorf("expected metadata to be NULL when link metadata is disabled")
	}
}

func TestCreatePost_EnqueuesMetadataJob(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	config := GetConfigService()
	current := config.GetConfig().LinkMetadataEnabled
	enabled := true
	if _, err := config.UpdateConfig(context.Background(), &enabled, nil, nil); err != nil {
		t.Fatalf("failed to enable link metadata: %v", err)
	}
	t.Cleanup(func() {
		if _, err := config.UpdateConfig(context.Background(), &current, nil, nil); err != nil {
			t.Fatalf("failed to restore link metadata: %v", err)
		}
	})

	rdb := setupMetadataQueueTestRedis(t)

	userID := testutil.CreateTestUser(t, db, "youtubepost", "youtubepost@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Video Section", "general")

	service := NewPostServiceWithRedis(db, rdb)
	req := &models.CreatePostRequest{
		SectionID: sectionID,
		Content:   "Watch this",
		Links: []models.LinkRequest{
			{URL: "https://www.youtube.com/watch?v=dQw4w9WgXcQ"},
		},
	}

	post, err := service.CreatePost(context.Background(), req, uuid.MustParse(userID))
	if err != nil {
		t.Fatalf("CreatePost failed: %v", err)
	}

	var metadataIsNull bool
	if err := db.QueryRow(`SELECT metadata IS NULL FROM links WHERE post_id = $1`, post.ID).Scan(&metadataIsNull); err != nil {
		t.Fatalf("failed to query link metadata: %v", err)
	}
	if !metadataIsNull {
		t.Fatalf("expected stored link metadata to be NULL")
	}

	length, err := GetQueueLength(context.Background(), rdb)
	if err != nil {
		t.Fatalf("failed to get queue length: %v", err)
	}
	if length != 1 {
		t.Fatalf("expected 1 metadata job, got %d", length)
	}

	job, err := DequeueMetadataJob(context.Background(), rdb, 1*time.Second)
	if err != nil {
		t.Fatalf("failed to dequeue metadata job: %v", err)
	}
	if job == nil {
		t.Fatalf("expected metadata job")
	}
	if job.PostID != post.ID {
		t.Fatalf("job.PostID = %s, want %s", job.PostID, post.ID)
	}
	if job.URL != req.Links[0].URL {
		t.Fatalf("job.URL = %s, want %s", job.URL, req.Links[0].URL)
	}
	if len(post.Links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(post.Links))
	}
	if job.LinkID != post.Links[0].ID {
		t.Fatalf("job.LinkID = %s, want %s", job.LinkID, post.Links[0].ID)
	}
}

func TestCreatePost_MultipleLinks_EnqueuesAllJobs(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	config := GetConfigService()
	current := config.GetConfig().LinkMetadataEnabled
	enabled := true
	if _, err := config.UpdateConfig(context.Background(), &enabled, nil, nil); err != nil {
		t.Fatalf("failed to enable link metadata: %v", err)
	}
	t.Cleanup(func() {
		if _, err := config.UpdateConfig(context.Background(), &current, nil, nil); err != nil {
			t.Fatalf("failed to restore link metadata: %v", err)
		}
	})

	rdb := setupMetadataQueueTestRedis(t)

	userID := testutil.CreateTestUser(t, db, "multilinkpost", "multilinkpost@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Multi Link Section", "general")

	service := NewPostServiceWithRedis(db, rdb)
	req := &models.CreatePostRequest{
		SectionID: sectionID,
		Content:   "Multiple links",
		Links: []models.LinkRequest{
			{URL: "https://youtube.com/watch?v=abc"},
			{URL: "https://spotify.com/track/xyz"},
			{URL: "https://bandcamp.com/album/test"},
		},
	}

	post, err := service.CreatePost(context.Background(), req, uuid.MustParse(userID))
	if err != nil {
		t.Fatalf("CreatePost failed: %v", err)
	}
	if post == nil {
		t.Fatalf("expected post")
	}

	length, err := GetQueueLength(context.Background(), rdb)
	if err != nil {
		t.Fatalf("failed to get queue length: %v", err)
	}
	if length != int64(len(req.Links)) {
		t.Fatalf("expected %d metadata jobs, got %d", len(req.Links), length)
	}
}

func TestCreatePostWithHighlightsStoresSortedMetadata(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	disableLinkMetadata(t)

	userID := testutil.CreateTestUser(t, db, "highlightuser", "highlightuser@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Music Section", "music")

	service := NewPostService(db)
	req := &models.CreatePostRequest{
		SectionID: sectionID,
		Content:   "Highlights",
		Links: []models.LinkRequest{
			{
				URL: "https://example.com/track",
				Highlights: []models.Highlight{
					{Timestamp: 45, Label: "Chorus"},
					{Timestamp: 10, Label: "Intro"},
				},
			},
		},
	}

	post, err := service.CreatePost(context.Background(), req, uuid.MustParse(userID))
	if err != nil {
		t.Fatalf("CreatePost failed: %v", err)
	}

	if len(post.Links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(post.Links))
	}
	if len(post.Links[0].Highlights) != 2 {
		t.Fatalf("expected 2 highlights, got %d", len(post.Links[0].Highlights))
	}
	if post.Links[0].Highlights[0].Timestamp != 10 || post.Links[0].Highlights[1].Timestamp != 45 {
		t.Errorf("expected highlights sorted by timestamp, got %+v", post.Links[0].Highlights)
	}

	var metadataBytes []byte
	if err := db.QueryRow(`SELECT metadata FROM links WHERE post_id = $1`, post.ID).Scan(&metadataBytes); err != nil {
		t.Fatalf("failed to query link metadata: %v", err)
	}
	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatalf("failed to unmarshal link metadata: %v", err)
	}
	rawHighlights, ok := metadata["highlights"]
	if !ok {
		t.Fatalf("expected highlights in metadata")
	}
	encoded, err := json.Marshal(rawHighlights)
	if err != nil {
		t.Fatalf("failed to marshal highlights: %v", err)
	}
	var storedHighlights []models.Highlight
	if err := json.Unmarshal(encoded, &storedHighlights); err != nil {
		t.Fatalf("failed to unmarshal highlights: %v", err)
	}
	if len(storedHighlights) != 2 || storedHighlights[0].Timestamp != 10 || storedHighlights[1].Timestamp != 45 {
		t.Errorf("expected stored highlights sorted, got %+v", storedHighlights)
	}
}

func TestCreatePostRejectsHighlightsForNonMusicSection(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	disableLinkMetadata(t)

	userID := testutil.CreateTestUser(t, db, "highlightreject", "highlightreject@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "General Section", "general")

	service := NewPostService(db)
	req := &models.CreatePostRequest{
		SectionID: sectionID,
		Content:   "Highlights",
		Links: []models.LinkRequest{
			{
				URL: "https://example.com/track",
				Highlights: []models.Highlight{
					{Timestamp: 5, Label: "Intro"},
				},
			},
		},
	}

	_, err := service.CreatePost(context.Background(), req, uuid.MustParse(userID))
	if err == nil {
		t.Fatalf("expected error for highlights in non-music section")
	}
	if !strings.Contains(err.Error(), "highlights are not allowed") {
		t.Fatalf("expected highlights validation error, got %v", err)
	}
}

func TestCreatePostWithLinksNoContent(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	disableLinkMetadata(t)

	userID := testutil.CreateTestUser(t, db, "postlinknocontent", "postlinknocontent@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Links Only Section", "general")

	service := NewPostService(db)
	req := &models.CreatePostRequest{
		SectionID: sectionID,
		Content:   "   ",
		Links: []models.LinkRequest{
			{URL: "https://example.com"},
		},
	}

	post, err := service.CreatePost(context.Background(), req, uuid.MustParse(userID))
	if err != nil {
		t.Fatalf("CreatePost failed: %v", err)
	}

	if post.Content != "" {
		t.Errorf("expected empty content, got %q", post.Content)
	}
	if len(post.Links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(post.Links))
	}
}

func TestCreatePostWithImages(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "postimageuser", "postimage@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Images Section", "general")

	service := NewPostService(db)
	req := &models.CreatePostRequest{
		SectionID: sectionID,
		Content:   "   ",
		Images: []models.PostImageRequest{
			{URL: "https://example.com/a.jpg", Caption: stringPtr("First"), AltText: stringPtr("Alt A")},
			{URL: "https://example.com/b.jpg"},
		},
	}

	post, err := service.CreatePost(context.Background(), req, uuid.MustParse(userID))
	if err != nil {
		t.Fatalf("CreatePost failed: %v", err)
	}

	if post.Content != "" {
		t.Errorf("expected empty content, got %q", post.Content)
	}
	if len(post.Images) != 2 {
		t.Fatalf("expected 2 images, got %d", len(post.Images))
	}
	if post.Images[0].URL != "https://example.com/a.jpg" || post.Images[0].Position != 0 {
		t.Errorf("unexpected first image: %+v", post.Images[0])
	}
	if post.Images[0].Caption == nil || *post.Images[0].Caption != "First" {
		t.Errorf("expected caption 'First', got %v", post.Images[0].Caption)
	}
	if post.Images[1].URL != "https://example.com/b.jpg" || post.Images[1].Position != 1 {
		t.Errorf("unexpected second image: %+v", post.Images[1])
	}

	rows, err := db.Query(`SELECT position FROM post_images WHERE post_id = $1 ORDER BY position ASC`, post.ID)
	if err != nil {
		t.Fatalf("failed to query post images: %v", err)
	}
	defer rows.Close()

	var positions []int
	for rows.Next() {
		var position int
		if err := rows.Scan(&position); err != nil {
			t.Fatalf("failed to scan post images: %v", err)
		}
		positions = append(positions, position)
	}
	if len(positions) != 2 || positions[0] != 0 || positions[1] != 1 {
		t.Fatalf("expected positions [0 1], got %v", positions)
	}
}

func TestGetPostByIDIncludesRecipeStats(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	viewerID := testutil.CreateTestUser(t, db, "recipestatsviewer", "recipestatsviewer@test.com", false, true)
	otherID := testutil.CreateTestUser(t, db, "recipestatsother", "recipestatsother@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Recipes", "recipe")
	postID := testutil.CreateTestPost(t, db, viewerID, sectionID, "Recipe content")

	_, err := db.ExecContext(context.Background(), `
		INSERT INTO saved_recipes (id, user_id, post_id, category, created_at)
		VALUES (gen_random_uuid(), $1, $2, $3, now())
	`, uuid.MustParse(viewerID), uuid.MustParse(postID), "Dinner")
	if err != nil {
		t.Fatalf("failed to insert saved recipe: %v", err)
	}
	_, err = db.ExecContext(context.Background(), `
		INSERT INTO saved_recipes (id, user_id, post_id, category, created_at)
		VALUES (gen_random_uuid(), $1, $2, $3, now())
	`, uuid.MustParse(viewerID), uuid.MustParse(postID), "Favorites")
	if err != nil {
		t.Fatalf("failed to insert saved recipe: %v", err)
	}
	_, err = db.ExecContext(context.Background(), `
		INSERT INTO saved_recipes (id, user_id, post_id, category, created_at)
		VALUES (gen_random_uuid(), $1, $2, $3, now())
	`, uuid.MustParse(otherID), uuid.MustParse(postID), "Dessert")
	if err != nil {
		t.Fatalf("failed to insert saved recipe: %v", err)
	}
	_, err = db.ExecContext(context.Background(), `
		INSERT INTO cook_logs (id, user_id, post_id, rating, created_at)
		VALUES (gen_random_uuid(), $1, $2, $3, now())
	`, uuid.MustParse(viewerID), uuid.MustParse(postID), 4)
	if err != nil {
		t.Fatalf("failed to insert cook log: %v", err)
	}
	_, err = db.ExecContext(context.Background(), `
		INSERT INTO cook_logs (id, user_id, post_id, rating, created_at)
		VALUES (gen_random_uuid(), $1, $2, $3, now())
	`, uuid.MustParse(otherID), uuid.MustParse(postID), 5)
	if err != nil {
		t.Fatalf("failed to insert cook log: %v", err)
	}

	service := NewPostService(db)
	post, err := service.GetPostByID(context.Background(), uuid.MustParse(postID), uuid.MustParse(viewerID))
	if err != nil {
		t.Fatalf("GetPostByID failed: %v", err)
	}

	if post.RecipeStats == nil {
		t.Fatalf("expected recipe stats to be populated")
	}
	if post.RecipeStats.SaveCount != 3 {
		t.Fatalf("expected save count 3, got %d", post.RecipeStats.SaveCount)
	}
	if post.RecipeStats.CookCount != 2 {
		t.Fatalf("expected cook count 2, got %d", post.RecipeStats.CookCount)
	}
	if post.RecipeStats.AvgRating == nil || *post.RecipeStats.AvgRating != 4.5 {
		t.Fatalf("expected avg rating 4.5, got %v", post.RecipeStats.AvgRating)
	}
	if !post.RecipeStats.ViewerSaved {
		t.Fatalf("expected viewer_saved true")
	}
	if !post.RecipeStats.ViewerCooked {
		t.Fatalf("expected viewer_cooked true")
	}
	if len(post.RecipeStats.ViewerCategories) != 2 {
		t.Fatalf("expected 2 viewer categories, got %d", len(post.RecipeStats.ViewerCategories))
	}
	if post.RecipeStats.ViewerCategories[0] != "Dinner" || post.RecipeStats.ViewerCategories[1] != "Favorites" {
		t.Fatalf("expected viewer categories [Dinner Favorites], got %v", post.RecipeStats.ViewerCategories)
	}
}

func TestGetPostByIDNonRecipeOmitsRecipeStats(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "nonrecipeuser", "nonrecipeuser@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "General", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "General content")

	service := NewPostService(db)
	post, err := service.GetPostByID(context.Background(), uuid.MustParse(postID), uuid.MustParse(userID))
	if err != nil {
		t.Fatalf("GetPostByID failed: %v", err)
	}

	if post.RecipeStats != nil {
		t.Fatalf("expected recipe stats to be omitted for non-recipe posts")
	}
}

func TestGetFeedIncludesRecipeStatsForRecipeSection(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	viewerID := testutil.CreateTestUser(t, db, "feedrecipeviewer", "feedrecipeviewer@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Recipes", "recipe")
	postIDWithStats := testutil.CreateTestPost(t, db, viewerID, sectionID, "Recipe with stats")
	postIDNoStats := testutil.CreateTestPost(t, db, viewerID, sectionID, "Recipe without stats")

	_, err := db.ExecContext(context.Background(), `
		INSERT INTO saved_recipes (id, user_id, post_id, category, created_at)
		VALUES (gen_random_uuid(), $1, $2, $3, now())
	`, uuid.MustParse(viewerID), uuid.MustParse(postIDWithStats), "Quick")
	if err != nil {
		t.Fatalf("failed to insert saved recipe: %v", err)
	}
	_, err = db.ExecContext(context.Background(), `
		INSERT INTO cook_logs (id, user_id, post_id, rating, created_at)
		VALUES (gen_random_uuid(), $1, $2, $3, now())
	`, uuid.MustParse(viewerID), uuid.MustParse(postIDWithStats), 5)
	if err != nil {
		t.Fatalf("failed to insert cook log: %v", err)
	}

	service := NewPostService(db)
	feed, err := service.GetFeed(context.Background(), uuid.MustParse(sectionID), nil, 10, uuid.MustParse(viewerID))
	if err != nil {
		t.Fatalf("GetFeed failed: %v", err)
	}

	postByID := make(map[string]*models.Post)
	for _, post := range feed.Posts {
		postByID[post.ID.String()] = post
	}

	recipeWithStats := postByID[postIDWithStats]
	if recipeWithStats == nil || recipeWithStats.RecipeStats == nil {
		t.Fatalf("expected recipe stats for post with stats")
	}
	if recipeWithStats.RecipeStats.SaveCount != 1 {
		t.Fatalf("expected save count 1, got %d", recipeWithStats.RecipeStats.SaveCount)
	}
	if recipeWithStats.RecipeStats.CookCount != 1 {
		t.Fatalf("expected cook count 1, got %d", recipeWithStats.RecipeStats.CookCount)
	}
	if recipeWithStats.RecipeStats.AvgRating == nil || *recipeWithStats.RecipeStats.AvgRating != 5 {
		t.Fatalf("expected avg rating 5, got %v", recipeWithStats.RecipeStats.AvgRating)
	}
	if len(recipeWithStats.RecipeStats.ViewerCategories) != 1 || recipeWithStats.RecipeStats.ViewerCategories[0] != "Quick" {
		t.Fatalf("expected viewer categories [Quick], got %v", recipeWithStats.RecipeStats.ViewerCategories)
	}

	recipeNoStats := postByID[postIDNoStats]
	if recipeNoStats == nil || recipeNoStats.RecipeStats == nil {
		t.Fatalf("expected recipe stats for post without stats")
	}
	if recipeNoStats.RecipeStats.SaveCount != 0 || recipeNoStats.RecipeStats.CookCount != 0 {
		t.Fatalf("expected zero stats for post without stats, got %+v", recipeNoStats.RecipeStats)
	}
	if recipeNoStats.RecipeStats.AvgRating != nil {
		t.Fatalf("expected nil avg rating for post without stats, got %v", recipeNoStats.RecipeStats.AvgRating)
	}
}

func TestCreatePostRequiresContentOrLinks(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	disableLinkMetadata(t)

	userID := testutil.CreateTestUser(t, db, "postempty", "postempty@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Empty Section", "general")

	service := NewPostService(db)
	req := &models.CreatePostRequest{
		SectionID: sectionID,
		Content:   "   ",
	}

	_, err := service.CreatePost(context.Background(), req, uuid.MustParse(userID))
	if err == nil {
		t.Fatalf("expected error for empty content without links")
	}
	if err.Error() != "content is required" {
		t.Fatalf("expected error %q, got %q", "content is required", err.Error())
	}
}

func TestUpdatePostCreatesAuditLogWithMetadata(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "updatepostuser", "updatepost@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Update Post Section", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Original post content")

	service := NewPostService(db)
	req := &models.UpdatePostRequest{
		Content: "Updated post content",
	}

	_, err := service.UpdatePost(context.Background(), uuid.MustParse(postID), uuid.MustParse(userID), req)
	if err != nil {
		t.Fatalf("UpdatePost failed: %v", err)
	}

	var metadataBytes []byte
	err = db.QueryRow(`
		SELECT metadata
		FROM audit_logs
		WHERE admin_user_id = $1 AND action = 'update_post'
	`, userID).Scan(&metadataBytes)
	if err != nil {
		t.Fatalf("failed to query audit log: %v", err)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}
	if metadata["post_id"] != postID {
		t.Errorf("expected post_id %s, got %v", postID, metadata["post_id"])
	}
	if metadata["section_id"] != sectionID {
		t.Errorf("expected section_id %s, got %v", sectionID, metadata["section_id"])
	}
	if metadata["previous_content"] != "Original post content" {
		t.Errorf("expected previous_content %q, got %v", "Original post content", metadata["previous_content"])
	}
	if metadata["content_excerpt"] != "Updated post content" {
		t.Errorf("expected content_excerpt %q, got %v", "Updated post content", metadata["content_excerpt"])
	}
	linksChanged, ok := metadata["links_changed"].(bool)
	if !ok {
		t.Fatalf("expected links_changed to be bool, got %T", metadata["links_changed"])
	}
	if linksChanged {
		t.Errorf("expected links_changed false, got %v", linksChanged)
	}
	linksProvided, ok := metadata["links_provided"].(bool)
	if !ok {
		t.Fatalf("expected links_provided to be bool, got %T", metadata["links_provided"])
	}
	if linksProvided {
		t.Errorf("expected links_provided false, got %v", linksProvided)
	}
	imagesChanged, ok := metadata["images_changed"].(bool)
	if !ok {
		t.Fatalf("expected images_changed to be bool, got %T", metadata["images_changed"])
	}
	if imagesChanged {
		t.Errorf("expected images_changed false, got %v", imagesChanged)
	}
	imagesProvided, ok := metadata["images_provided"].(bool)
	if !ok {
		t.Fatalf("expected images_provided to be bool, got %T", metadata["images_provided"])
	}
	if imagesProvided {
		t.Errorf("expected images_provided false, got %v", imagesProvided)
	}
}

func TestUpdatePostRemovesLinkMetadata(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "updatelinkremove", "updatelinkremove@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Update Link Section", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Original content")

	linkID := uuid.New()
	metadata := models.JSONMap{
		"title": "Example",
		"type":  "article",
	}
	_, err := db.Exec(`
		INSERT INTO links (id, post_id, url, metadata, created_at)
		VALUES ($1, $2, $3, $4, now())
	`, linkID, postID, "https://example.com", metadata)
	if err != nil {
		t.Fatalf("failed to insert link metadata: %v", err)
	}

	service := NewPostService(db)
	req := &models.UpdatePostRequest{
		Content:            "Updated content",
		RemoveLinkMetadata: true,
	}

	_, err = service.UpdatePost(context.Background(), uuid.MustParse(postID), uuid.MustParse(userID), req)
	if err != nil {
		t.Fatalf("UpdatePost failed: %v", err)
	}

	var linkCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM links WHERE post_id = $1`, postID).Scan(&linkCount); err != nil {
		t.Fatalf("failed to query links: %v", err)
	}
	if linkCount != 0 {
		t.Fatalf("expected links to be removed, found %d", linkCount)
	}

	var action string
	if err := db.QueryRow(`
		SELECT action
		FROM audit_logs
		WHERE admin_user_id = $1 AND action = 'remove_link_metadata'
	`, userID).Scan(&action); err != nil {
		t.Fatalf("expected removal audit log: %v", err)
	}
}

func TestUpdatePostImages(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "updatepostimages", "updatepostimages@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Update Images Section", "general")

	service := NewPostService(db)
	createReq := &models.CreatePostRequest{
		SectionID: sectionID,
		Content:   "Post with images",
		Images: []models.PostImageRequest{
			{URL: "https://example.com/one.jpg"},
			{URL: "https://example.com/two.jpg"},
		},
	}

	post, err := service.CreatePost(context.Background(), createReq, uuid.MustParse(userID))
	if err != nil {
		t.Fatalf("CreatePost failed: %v", err)
	}

	updateReq := &models.UpdatePostRequest{
		Content: "Post with images",
		Images: &[]models.PostImageRequest{
			{URL: "https://example.com/two.jpg", Caption: stringPtr("Second")},
		},
	}

	updated, err := service.UpdatePost(context.Background(), post.ID, uuid.MustParse(userID), updateReq)
	if err != nil {
		t.Fatalf("UpdatePost failed: %v", err)
	}

	if len(updated.Images) != 1 {
		t.Fatalf("expected 1 image after update, got %d", len(updated.Images))
	}
	if updated.Images[0].URL != "https://example.com/two.jpg" || updated.Images[0].Position != 0 {
		t.Errorf("unexpected updated image: %+v", updated.Images[0])
	}
	if updated.Images[0].Caption == nil || *updated.Images[0].Caption != "Second" {
		t.Errorf("expected caption 'Second', got %v", updated.Images[0].Caption)
	}

	var metadataBytes []byte
	err = db.QueryRow(`
		SELECT metadata
		FROM audit_logs
		WHERE admin_user_id = $1 AND action = 'update_post'
		ORDER BY created_at DESC
		LIMIT 1
	`, userID).Scan(&metadataBytes)
	if err != nil {
		t.Fatalf("failed to query audit log: %v", err)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}
	imagesChanged, ok := metadata["images_changed"].(bool)
	if !ok {
		t.Fatalf("expected images_changed to be bool, got %T", metadata["images_changed"])
	}
	if !imagesChanged {
		t.Errorf("expected images_changed true, got %v", imagesChanged)
	}
	imageCount, ok := metadata["image_count"].(float64)
	if !ok {
		t.Fatalf("expected image_count to be number, got %T", metadata["image_count"])
	}
	if int(imageCount) != 1 {
		t.Errorf("expected image_count 1, got %v", imageCount)
	}
}

func TestUpdatePostHighlights(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	disableLinkMetadata(t)

	userID := testutil.CreateTestUser(t, db, "updatehighlights", "updatehighlights@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Update Music Section", "music")

	service := NewPostService(db)
	createReq := &models.CreatePostRequest{
		SectionID: sectionID,
		Content:   "Post with link",
		Links: []models.LinkRequest{
			{URL: "https://example.com/track"},
		},
	}

	post, err := service.CreatePost(context.Background(), createReq, uuid.MustParse(userID))
	if err != nil {
		t.Fatalf("CreatePost failed: %v", err)
	}

	updateReq := &models.UpdatePostRequest{
		Content: "Post with link",
		Links: &[]models.LinkRequest{
			{
				URL: "https://example.com/track",
				Highlights: []models.Highlight{
					{Timestamp: 30, Label: "Verse"},
					{Timestamp: 12, Label: "Intro"},
				},
			},
		},
	}

	updated, err := service.UpdatePost(context.Background(), post.ID, uuid.MustParse(userID), updateReq)
	if err != nil {
		t.Fatalf("UpdatePost failed: %v", err)
	}

	if len(updated.Links) != 1 {
		t.Fatalf("expected 1 link after update, got %d", len(updated.Links))
	}
	if len(updated.Links[0].Highlights) != 2 {
		t.Fatalf("expected 2 highlights, got %d", len(updated.Links[0].Highlights))
	}
	if updated.Links[0].Highlights[0].Timestamp != 12 || updated.Links[0].Highlights[1].Timestamp != 30 {
		t.Errorf("expected highlights sorted, got %+v", updated.Links[0].Highlights)
	}

	var metadataBytes []byte
	if err := db.QueryRow(`SELECT metadata FROM links WHERE post_id = $1`, post.ID).Scan(&metadataBytes); err != nil {
		t.Fatalf("failed to query link metadata: %v", err)
	}
	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatalf("failed to unmarshal link metadata: %v", err)
	}
	rawHighlights, ok := metadata["highlights"]
	if !ok {
		t.Fatalf("expected highlights in metadata")
	}
	encoded, err := json.Marshal(rawHighlights)
	if err != nil {
		t.Fatalf("failed to marshal highlights: %v", err)
	}
	var storedHighlights []models.Highlight
	if err := json.Unmarshal(encoded, &storedHighlights); err != nil {
		t.Fatalf("failed to unmarshal highlights: %v", err)
	}
	if len(storedHighlights) != 2 || storedHighlights[0].Timestamp != 12 || storedHighlights[1].Timestamp != 30 {
		t.Errorf("expected stored highlights sorted, got %+v", storedHighlights)
	}
}

func TestDeletePostOwner(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "deleteowner", "deleteowner@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Delete Section", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Owner post")

	service := NewPostService(db)
	post, err := service.DeletePost(context.Background(), uuid.MustParse(postID), uuid.MustParse(userID), false)
	if err != nil {
		t.Fatalf("DeletePost failed: %v", err)
	}

	if post.DeletedAt == nil {
		t.Fatalf("expected deleted_at to be set")
	}
	if post.DeletedByUserID == nil || post.DeletedByUserID.String() != userID {
		t.Errorf("expected deleted_by_user_id %s, got %v", userID, post.DeletedByUserID)
	}

	var metadataBytes []byte
	err = db.QueryRow(`
		SELECT metadata
		FROM audit_logs
		WHERE admin_user_id = $1 AND action = 'delete_post' AND related_post_id = $2
	`, userID, postID).Scan(&metadataBytes)
	if err != nil {
		t.Fatalf("failed to query audit log: %v", err)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}
	if metadata["post_id"] != postID {
		t.Errorf("expected post_id %s, got %v", postID, metadata["post_id"])
	}
	if metadata["section_id"] != sectionID {
		t.Errorf("expected section_id %s, got %v", sectionID, metadata["section_id"])
	}
	if metadata["deleted_by_user_id"] != userID {
		t.Errorf("expected deleted_by_user_id %s, got %v", userID, metadata["deleted_by_user_id"])
	}
	isSelfDelete, ok := metadata["is_self_delete"].(bool)
	if !ok {
		t.Fatalf("expected is_self_delete to be bool, got %T", metadata["is_self_delete"])
	}
	if !isSelfDelete {
		t.Errorf("expected is_self_delete true, got %v", metadata["is_self_delete"])
	}
}

func TestDeletePostAdmin(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "deleteuser", "deleteuser@test.com", false, true)
	adminID := testutil.CreateTestUser(t, db, "deleteadmin", "deleteadmin@test.com", true, true)
	sectionID := testutil.CreateTestSection(t, db, "Admin Delete Section", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Admin delete post")

	service := NewPostService(db)
	post, err := service.DeletePost(context.Background(), uuid.MustParse(postID), uuid.MustParse(adminID), true)
	if err != nil {
		t.Fatalf("DeletePost failed: %v", err)
	}

	if post.DeletedAt == nil {
		t.Fatalf("expected deleted_at to be set")
	}
	if post.DeletedByUserID == nil || post.DeletedByUserID.String() != adminID {
		t.Errorf("expected deleted_by_user_id %s, got %v", adminID, post.DeletedByUserID)
	}
}

func TestRestorePostOwner(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "restoreowner", "restoreowner@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Restore Section", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Restore post")

	service := NewPostService(db)
	_, err := service.DeletePost(context.Background(), uuid.MustParse(postID), uuid.MustParse(userID), false)
	if err != nil {
		t.Fatalf("DeletePost failed: %v", err)
	}

	post, err := service.RestorePost(context.Background(), uuid.MustParse(postID), uuid.MustParse(userID), false)
	if err != nil {
		t.Fatalf("RestorePost failed: %v", err)
	}

	if post.DeletedAt != nil {
		t.Fatalf("expected deleted_at to be cleared")
	}
	if post.DeletedByUserID != nil {
		t.Fatalf("expected deleted_by_user_id to be cleared")
	}
}

func TestAdminRestorePostCreatesAuditLog(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "adminrestoreuser", "adminrestoreuser@test.com", false, true)
	adminID := testutil.CreateTestUser(t, db, "adminrestore", "adminrestore@test.com", true, true)
	sectionID := testutil.CreateTestSection(t, db, "Admin Restore Section", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Admin restore post")

	service := NewPostService(db)
	_, err := service.DeletePost(context.Background(), uuid.MustParse(postID), uuid.MustParse(userID), false)
	if err != nil {
		t.Fatalf("DeletePost failed: %v", err)
	}

	_, err = service.AdminRestorePost(context.Background(), uuid.MustParse(postID), uuid.MustParse(adminID))
	if err != nil {
		t.Fatalf("AdminRestorePost failed: %v", err)
	}

	var count int
	var metadataBytes []byte
	err = db.QueryRow(`
		SELECT COUNT(*), metadata
		FROM audit_logs
		WHERE admin_user_id = $1 AND action = 'restore_post' AND related_post_id = $2
		GROUP BY metadata
	`, adminID, postID).Scan(&count, &metadataBytes)
	if err != nil {
		t.Fatalf("failed to query audit log: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 audit log entry, got %d", count)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}
	if metadata["restored_by_user_id"] != adminID {
		t.Errorf("expected restored_by_user_id %s, got %v", adminID, metadata["restored_by_user_id"])
	}
	if metadata["post_id"] != postID {
		t.Errorf("expected post_id %s, got %v", postID, metadata["post_id"])
	}
}

func TestHardDeletePostCreatesAuditLog(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "harddeleteuser", "harddeleteuser@test.com", false, true)
	adminID := testutil.CreateTestUser(t, db, "harddeleteadmin", "harddeleteadmin@test.com", true, true)
	sectionID := testutil.CreateTestSection(t, db, "Hard Delete Section", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Hard delete post")

	service := NewPostService(db)
	if err := service.HardDeletePost(context.Background(), uuid.MustParse(postID), uuid.MustParse(adminID)); err != nil {
		t.Fatalf("HardDeletePost failed: %v", err)
	}

	var postCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM posts WHERE id = $1`, postID).Scan(&postCount); err != nil {
		t.Fatalf("failed to query post: %v", err)
	}
	if postCount != 0 {
		t.Errorf("expected post to be deleted, found %d rows", postCount)
	}

	var auditCount int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM audit_logs
		WHERE admin_user_id = $1 AND action = 'hard_delete_post'
	`, adminID).Scan(&auditCount); err != nil {
		t.Fatalf("failed to query audit log: %v", err)
	}
	if auditCount != 1 {
		t.Errorf("expected 1 audit log entry, got %d", auditCount)
	}
}

func TestAdminDeletePostCreatesAuditLogWithMetadata(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "moderateduser", "moderateduser@test.com", false, true)
	adminID := testutil.CreateTestUser(t, db, "moderatoradmin", "moderatoradmin@test.com", true, true)
	sectionID := testutil.CreateTestSection(t, db, "Moderation Section", "general")
	content := strings.Repeat("a", 150)
	postID := testutil.CreateTestPost(t, db, userID, sectionID, content)

	service := NewPostService(db)
	_, err := service.DeletePost(context.Background(), uuid.MustParse(postID), uuid.MustParse(adminID), true)
	if err != nil {
		t.Fatalf("DeletePost failed: %v", err)
	}

	var relatedPostID uuid.UUID
	var metadataBytes []byte
	err = db.QueryRow(`
		SELECT related_post_id, metadata
		FROM audit_logs
		WHERE admin_user_id = $1 AND action = 'delete_post'
	`, adminID).Scan(&relatedPostID, &metadataBytes)
	if err != nil {
		t.Fatalf("failed to query audit log: %v", err)
	}
	if relatedPostID.String() != postID {
		t.Errorf("expected related_post_id %s, got %s", postID, relatedPostID.String())
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}
	if metadata["post_id"] != postID {
		t.Errorf("expected post_id %s, got %v", postID, metadata["post_id"])
	}
	if metadata["section_id"] != sectionID {
		t.Errorf("expected section_id %s, got %v", sectionID, metadata["section_id"])
	}
	if metadata["deleted_by_user_id"] != adminID {
		t.Errorf("expected deleted_by_user_id %s, got %v", adminID, metadata["deleted_by_user_id"])
	}
	isSelfDelete, ok := metadata["is_self_delete"].(bool)
	if !ok {
		t.Fatalf("expected is_self_delete to be bool, got %T", metadata["is_self_delete"])
	}
	if isSelfDelete {
		t.Errorf("expected is_self_delete false, got %v", metadata["is_self_delete"])
	}
	deletedByAdmin, ok := metadata["deleted_by_admin"].(bool)
	if !ok {
		t.Fatalf("expected deleted_by_admin to be bool, got %T", metadata["deleted_by_admin"])
	}
	if !deletedByAdmin {
		t.Errorf("expected deleted_by_admin true, got %v", metadata["deleted_by_admin"])
	}
	excerpt, ok := metadata["content_excerpt"].(string)
	if !ok {
		t.Fatalf("expected content_excerpt to be string, got %T", metadata["content_excerpt"])
	}
	if len([]rune(excerpt)) != auditExcerptLimit {
		t.Errorf("expected content_excerpt length %d, got %d", auditExcerptLimit, len([]rune(excerpt)))
	}
}

func disableLinkMetadata(t *testing.T) {
	t.Helper()
	config := GetConfigService()
	current := config.GetConfig().LinkMetadataEnabled
	disabled := false
	if _, err := config.UpdateConfig(context.Background(), &disabled, nil, nil); err != nil {
		t.Fatalf("failed to disable link metadata: %v", err)
	}
	t.Cleanup(func() {
		if _, err := config.UpdateConfig(context.Background(), &current, nil, nil); err != nil {
			t.Fatalf("failed to restore link metadata: %v", err)
		}
	})
}

func newFailingRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	return redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:1",
		DialTimeout:  10 * time.Millisecond,
		ReadTimeout:  10 * time.Millisecond,
		WriteTimeout: 10 * time.Millisecond,
	})
}

func TestCreatePost_QueueFailure_DoesNotFailPost(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	config := GetConfigService()
	current := config.GetConfig().LinkMetadataEnabled
	enabled := true
	if _, err := config.UpdateConfig(context.Background(), &enabled, nil, nil); err != nil {
		t.Fatalf("failed to enable link metadata: %v", err)
	}
	t.Cleanup(func() {
		if _, err := config.UpdateConfig(context.Background(), &current, nil, nil); err != nil {
			t.Fatalf("failed to restore link metadata: %v", err)
		}
	})

	rdb := newFailingRedisClient(t)
	t.Cleanup(func() { _ = rdb.Close() })

	userID := testutil.CreateTestUser(t, db, "queuefailpost", "queuefailpost@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Queue Fail Section", "general")

	service := NewPostServiceWithRedis(db, rdb)
	req := &models.CreatePostRequest{
		SectionID: sectionID,
		Content:   "Queue should not fail post creation",
		Links: []models.LinkRequest{
			{URL: "https://example.com"},
		},
	}

	post, err := service.CreatePost(context.Background(), req, uuid.MustParse(userID))
	if err != nil {
		t.Fatalf("CreatePost failed: %v", err)
	}
	if post == nil {
		t.Fatalf("expected post")
	}
}
