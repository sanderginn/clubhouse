package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/middleware"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/services"
)

// ReactionHandler handles reaction endpoints
type ReactionHandler struct {
	reactionService *services.ReactionService
}

// NewReactionHandler creates a new reaction handler
func NewReactionHandler(db *sql.DB) *ReactionHandler {
	return &ReactionHandler{
		reactionService: services.NewReactionService(db),
	}
}

// AddReactionToPost handles POST /api/v1/posts/{postId}/reactions
func (h *ReactionHandler) AddReactionToPost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	postID, err := extractPostIDFromPath(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_POST_ID", "Invalid post ID format")
		return
	}

	var req models.CreateReactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	reaction, err := h.reactionService.AddReactionToPost(r.Context(), postID, userID, req.Emoji)
	if err != nil {
		switch err.Error() {
		case "emoji is required":
			writeError(w, http.StatusBadRequest, "EMOJI_REQUIRED", err.Error())
		case "emoji must be 10 characters or less":
			writeError(w, http.StatusBadRequest, "EMOJI_TOO_LONG", err.Error())
		case "post not found":
			writeError(w, http.StatusNotFound, "POST_NOT_FOUND", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "REACTION_CREATION_FAILED", "Failed to add reaction")
		}
		return
	}

	response := models.CreateReactionResponse{
		Reaction: *reaction,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func extractPostIDFromPath(path string) (uuid.UUID, error) {
	pathParts := strings.Split(path, "/")
	for i, part := range pathParts {
		if part == "posts" && i+1 < len(pathParts) {
			return uuid.Parse(pathParts[i+1])
		}
	}
	return uuid.Nil, errors.New("post ID not found in path")
}
