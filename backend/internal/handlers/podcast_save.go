package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/middleware"
	"github.com/sanderginn/clubhouse/internal/observability"
	"github.com/sanderginn/clubhouse/internal/services"
)

const (
	defaultPodcastSavedListLimit = 20
	maxPodcastSavedListLimit     = 100
)

// PodcastSaveHandler handles podcast save-for-later endpoints.
type PodcastSaveHandler struct {
	podcastSaveService *services.PodcastSaveService
}

// NewPodcastSaveHandler creates a new podcast save handler.
func NewPodcastSaveHandler(db *sql.DB) *PodcastSaveHandler {
	return &PodcastSaveHandler{
		podcastSaveService: services.NewPodcastSaveService(db),
	}
}

// SavePodcast handles POST /api/v1/posts/{postID}/podcast-save.
func (h *PodcastSaveHandler) SavePodcast(w http.ResponseWriter, r *http.Request) {
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

	save, err := h.podcastSaveService.SavePodcast(r.Context(), userID, postID)
	if err != nil {
		switch err.Error() {
		case "podcast post not found":
			writeError(r.Context(), w, http.StatusNotFound, "POST_NOT_FOUND", "Post not found")
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "SAVE_PODCAST_FAILED", "Failed to save podcast")
		}
		return
	}

	observability.LogInfo(r.Context(), "podcast saved",
		"user_id", userID.String(),
		"post_id", postID.String(),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(save); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode save podcast response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// UnsavePodcast handles DELETE /api/v1/posts/{postID}/podcast-save.
func (h *PodcastSaveHandler) UnsavePodcast(w http.ResponseWriter, r *http.Request) {
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

	if err := h.podcastSaveService.UnsavePodcast(r.Context(), userID, postID); err != nil {
		switch err.Error() {
		case "podcast post not found":
			writeError(r.Context(), w, http.StatusNotFound, "POST_NOT_FOUND", "Post not found")
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "UNSAVE_PODCAST_FAILED", "Failed to unsave podcast")
		}
		return
	}

	observability.LogInfo(r.Context(), "podcast unsaved",
		"user_id", userID.String(),
		"post_id", postID.String(),
	)

	w.WriteHeader(http.StatusNoContent)
}

// GetPostPodcastSaveInfo handles GET /api/v1/posts/{postID}/podcast-save-info.
func (h *PodcastSaveHandler) GetPostPodcastSaveInfo(w http.ResponseWriter, r *http.Request) {
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

	info, err := h.podcastSaveService.GetPostPodcastSaveInfo(r.Context(), postID, &userID)
	if err != nil {
		if err.Error() == "podcast post not found" {
			writeError(r.Context(), w, http.StatusNotFound, "POST_NOT_FOUND", "Post not found")
			return
		}
		writeError(r.Context(), w, http.StatusInternalServerError, "GET_PODCAST_SAVE_INFO_FAILED", "Failed to get podcast save info")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(info); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode post podcast save info response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// ListSectionSavedPodcastPosts handles GET /api/v1/sections/{sectionID}/podcast-saved.
func (h *PodcastSaveHandler) ListSectionSavedPodcastPosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	sectionID, err := extractSectionIDFromPath(r.URL.Path)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_SECTION_ID", "Invalid section ID format")
		return
	}

	cursor, limit, err := parsePodcastSavedListQuery(r)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_LIMIT", err.Error())
		return
	}

	feed, err := h.podcastSaveService.ListSectionSavedPodcastPosts(r.Context(), sectionID, userID, cursor, limit)
	if err != nil {
		switch err.Error() {
		case "section not found":
			writeError(r.Context(), w, http.StatusNotFound, "SECTION_NOT_FOUND", "Section not found")
		case "section is not podcast":
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_SECTION_TYPE", "Section must be a podcast section")
		case "invalid cursor":
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_CURSOR", "Invalid cursor format")
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "GET_PODCAST_SAVED_FAILED", "Failed to get saved podcasts")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(feed); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode section saved podcast response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

func parsePodcastSavedListQuery(r *http.Request) (*string, int, error) {
	limit := defaultPodcastSavedListLimit
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			return nil, 0, errors.New("limit must be a positive integer")
		}
		limit = parsed
	}
	if limit > maxPodcastSavedListLimit {
		limit = maxPodcastSavedListLimit
	}

	var cursor *string
	if cursorParam := strings.TrimSpace(r.URL.Query().Get("cursor")); cursorParam != "" {
		cursor = &cursorParam
	}

	return cursor, limit, nil
}

func extractSectionIDFromPath(path string) (uuid.UUID, error) {
	pathParts := strings.Split(path, "/")
	for i, part := range pathParts {
		if part == "sections" && i+1 < len(pathParts) {
			return uuid.Parse(pathParts[i+1])
		}
	}
	return uuid.Nil, errors.New("section ID not found in path")
}
