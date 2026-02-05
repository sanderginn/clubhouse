package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/sanderginn/clubhouse/internal/middleware"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/observability"
	"github.com/sanderginn/clubhouse/internal/services"
)

type HighlightReactionHandler struct {
	service     *services.HighlightReactionService
	postService *services.PostService
	redis       *redis.Client
}

func NewHighlightReactionHandler(db *sql.DB, redisClient *redis.Client) *HighlightReactionHandler {
	return &HighlightReactionHandler{
		service:     services.NewHighlightReactionService(db),
		postService: services.NewPostService(db),
		redis:       redisClient,
	}
}

// AddHighlightReaction handles POST /api/v1/posts/{postId}/highlights/{highlightId}/reactions
func (h *HighlightReactionHandler) AddHighlightReaction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	postID, highlightID, err := extractHighlightReactionPath(r.URL.Path)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid highlight reaction path")
		return
	}

	response, created, err := h.service.AddReaction(r.Context(), postID, highlightID, userID)
	if err != nil {
		switch err.Error() {
		case "invalid highlight id":
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_HIGHLIGHT_ID", err.Error())
		case "highlight not found":
			writeError(r.Context(), w, http.StatusNotFound, "HIGHLIGHT_NOT_FOUND", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "HIGHLIGHT_REACTION_FAILED", "Failed to add highlight reaction")
		}
		return
	}

	if created {
		publishCtx, cancel := publishContext()
		linkID, _, decodeErr := models.DecodeHighlightID(highlightID)
		if decodeErr == nil {
			_ = publishEvent(publishCtx, h.redis, formatChannel(postPrefix, postID), "highlight_reaction_added", highlightReactionEventData{
				PostID:      postID,
				LinkID:      linkID,
				HighlightID: highlightID,
				UserID:      userID,
			})
			if sectionID, err := h.postService.GetSectionIDByPostID(publishCtx, postID); err == nil {
				_ = publishEvent(publishCtx, h.redis, formatChannel(sectionPrefix, sectionID), "highlight_reaction_added", highlightReactionEventData{
					PostID:      postID,
					LinkID:      linkID,
					HighlightID: highlightID,
					UserID:      userID,
				})
			}
		}
		observability.RecordReactionAdded(publishCtx, "❤️")
		cancel()
	}

	observability.LogInfo(r.Context(), "highlight reaction added",
		"post_id", postID.String(),
		"highlight_id", highlightID,
		"user_id", userID.String(),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode highlight reaction response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusCreated,
			Err:        err,
		})
	}
}

// GetHighlightReactions handles GET /api/v1/posts/{postId}/highlights/{highlightId}/reactions
func (h *HighlightReactionHandler) GetHighlightReactions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	postID, highlightID, err := extractHighlightReactionPath(r.URL.Path)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid highlight reaction path")
		return
	}

	reactions, err := h.service.GetReactions(r.Context(), postID, highlightID)
	if err != nil {
		switch err.Error() {
		case "invalid highlight id":
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_HIGHLIGHT_ID", err.Error())
		case "highlight not found":
			writeError(r.Context(), w, http.StatusNotFound, "HIGHLIGHT_NOT_FOUND", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "HIGHLIGHT_REACTION_FAILED", "Failed to fetch highlight reactions")
		}
		return
	}

	response := models.GetReactionsResponse{
		Reactions: reactions,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode highlight reactions response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// RemoveHighlightReaction handles DELETE /api/v1/posts/{postId}/highlights/{highlightId}/reactions
func (h *HighlightReactionHandler) RemoveHighlightReaction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only DELETE requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	postID, highlightID, err := extractHighlightReactionPath(r.URL.Path)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid highlight reaction path")
		return
	}

	response, err := h.service.RemoveReaction(r.Context(), postID, highlightID, userID)
	if err != nil {
		switch err.Error() {
		case "invalid highlight id":
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_HIGHLIGHT_ID", err.Error())
			return
		case "highlight not found":
			writeError(r.Context(), w, http.StatusNotFound, "HIGHLIGHT_NOT_FOUND", err.Error())
			return
		case "reaction not found":
			w.WriteHeader(http.StatusNoContent)
			return
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "HIGHLIGHT_REACTION_FAILED", "Failed to remove highlight reaction")
			return
		}
	}

	publishCtx, cancel := publishContext()
	linkID, _, decodeErr := models.DecodeHighlightID(highlightID)
	if decodeErr == nil {
		_ = publishEvent(publishCtx, h.redis, formatChannel(postPrefix, postID), "highlight_reaction_removed", highlightReactionEventData{
			PostID:      postID,
			LinkID:      linkID,
			HighlightID: highlightID,
			UserID:      userID,
		})
		if sectionID, err := h.postService.GetSectionIDByPostID(publishCtx, postID); err == nil {
			_ = publishEvent(publishCtx, h.redis, formatChannel(sectionPrefix, sectionID), "highlight_reaction_removed", highlightReactionEventData{
				PostID:      postID,
				LinkID:      linkID,
				HighlightID: highlightID,
				UserID:      userID,
			})
		}
	}
	observability.RecordReactionRemoved(publishCtx, "❤️")
	cancel()

	observability.LogInfo(r.Context(), "highlight reaction removed",
		"post_id", postID.String(),
		"highlight_id", highlightID,
		"user_id", userID.String(),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode highlight reaction response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

func extractHighlightReactionPath(path string) (uuid.UUID, string, error) {
	postID, err := extractPostIDFromPath(path)
	if err != nil {
		return uuid.UUID{}, "", err
	}
	parts := strings.Split(strings.TrimSuffix(path, "/"), "/")
	for i, part := range parts {
		if part == "highlights" && i+1 < len(parts) {
			return postID, parts[i+1], nil
		}
	}
	return uuid.UUID{}, "", errors.New("highlight ID not found in path")
}
