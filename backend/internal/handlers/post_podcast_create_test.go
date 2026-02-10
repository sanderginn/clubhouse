package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/services"
	"github.com/sanderginn/clubhouse/internal/testutil"
)

func TestCreatePostHandlerPodcastShowWithHighlightedEpisodes(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })
	disableLinkMetadataForCreatePostPodcastTests(t)

	userID := testutil.CreateTestUser(t, db, "podcastcreateshow", "podcastcreateshow@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Podcasts", "podcast")
	note := "Start here"

	reqBody := models.CreatePostRequest{
		SectionID: sectionID,
		Content:   "Podcast show post",
		Links: []models.LinkRequest{
			{
				URL: "https://example.com/show",
				Podcast: &models.PodcastMetadata{
					Kind: "show",
					HighlightEpisodes: []models.PodcastHighlightEpisode{
						{
							Title: "Episode 1",
							URL:   "https://example.com/show/episode-1",
							Note:  &note,
						},
					},
				},
			},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}

	handler := newPostHandlerForPodcastCreateTests(db)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "podcastcreateshow", false))
	rr := httptest.NewRecorder()

	handler.CreatePost(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}

	var response models.CreatePostResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(response.Post.Links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(response.Post.Links))
	}
	if response.Post.Links[0].Podcast == nil {
		t.Fatal("expected podcast metadata in response link")
	}
	if response.Post.Links[0].Podcast.Kind != "show" {
		t.Fatalf("expected podcast kind show, got %q", response.Post.Links[0].Podcast.Kind)
	}
	if len(response.Post.Links[0].Podcast.HighlightEpisodes) != 1 {
		t.Fatalf("expected 1 highlighted episode, got %d", len(response.Post.Links[0].Podcast.HighlightEpisodes))
	}

	var metadataBytes []byte
	if err := db.QueryRow(`SELECT metadata FROM links WHERE post_id = $1`, response.Post.ID).Scan(&metadataBytes); err != nil {
		t.Fatalf("failed to query stored link metadata: %v", err)
	}

	var metadata map[string]any
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}

	rawPodcast, ok := metadata["podcast"]
	if !ok {
		t.Fatal("expected podcast metadata in stored payload")
	}
	encodedPodcast, err := json.Marshal(rawPodcast)
	if err != nil {
		t.Fatalf("failed to marshal stored podcast metadata: %v", err)
	}
	var storedPodcast models.PodcastMetadata
	if err := json.Unmarshal(encodedPodcast, &storedPodcast); err != nil {
		t.Fatalf("failed to unmarshal stored podcast metadata: %v", err)
	}
	if storedPodcast.Kind != "show" {
		t.Fatalf("expected stored podcast kind show, got %q", storedPodcast.Kind)
	}
	if len(storedPodcast.HighlightEpisodes) != 1 {
		t.Fatalf("expected 1 stored highlighted episode, got %d", len(storedPodcast.HighlightEpisodes))
	}
}

func TestCreatePostHandlerPodcastEpisode(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })
	disableLinkMetadataForCreatePostPodcastTests(t)

	userID := testutil.CreateTestUser(t, db, "podcastcreateepisode", "podcastcreateepisode@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Podcasts", "podcast")

	reqBody := models.CreatePostRequest{
		SectionID: sectionID,
		Content:   "Podcast episode post",
		Links: []models.LinkRequest{
			{
				URL: "https://example.com/episode",
				Podcast: &models.PodcastMetadata{
					Kind: "episode",
				},
			},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}

	handler := newPostHandlerForPodcastCreateTests(db)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "podcastcreateepisode", false))
	rr := httptest.NewRecorder()

	handler.CreatePost(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}

	var response models.CreatePostResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(response.Post.Links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(response.Post.Links))
	}
	if response.Post.Links[0].Podcast == nil {
		t.Fatal("expected podcast metadata in response link")
	}
	if response.Post.Links[0].Podcast.Kind != "episode" {
		t.Fatalf("expected podcast kind episode, got %q", response.Post.Links[0].Podcast.Kind)
	}
	if len(response.Post.Links[0].Podcast.HighlightEpisodes) != 0 {
		t.Fatalf("expected no highlighted episodes, got %d", len(response.Post.Links[0].Podcast.HighlightEpisodes))
	}
}

func TestCreatePostHandlerPodcastKindSelectionRequired(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })
	disableLinkMetadataForCreatePostPodcastTests(t)

	userID := testutil.CreateTestUser(t, db, "podcastcreateuncertain", "podcastcreateuncertain@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Podcasts", "podcast")

	reqBody := models.CreatePostRequest{
		SectionID: sectionID,
		Content:   "Podcast uncertain kind",
		Links: []models.LinkRequest{
			{
				URL: "https://example.com/listen",
				Podcast: &models.PodcastMetadata{
					Kind: "",
				},
			},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}

	handler := newPostHandlerForPodcastCreateTests(db)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "podcastcreateuncertain", false))
	rr := httptest.NewRecorder()

	handler.CreatePost(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if response.Code != "PODCAST_KIND_SELECTION_REQUIRED" {
		t.Fatalf("expected code PODCAST_KIND_SELECTION_REQUIRED, got %q", response.Code)
	}
}

func disableLinkMetadataForCreatePostPodcastTests(t *testing.T) {
	t.Helper()

	config := services.GetConfigService()
	current := config.GetConfig().LinkMetadataEnabled
	disabled := false
	if _, err := config.UpdateConfig(context.Background(), &disabled, nil, nil); err != nil {
		t.Fatalf("failed to disable link metadata: %v", err)
	}
	t.Cleanup(func() {
		if _, err := config.UpdateConfig(context.Background(), &current, nil, nil); err != nil {
			t.Fatalf("failed to restore link metadata config: %v", err)
		}
	})
}

func newPostHandlerForPodcastCreateTests(db *sql.DB) *PostHandler {
	handler := NewPostHandler(db, nil, nil)
	handler.rateLimiter = &stubContentRateLimiter{allowed: true}
	return handler
}
