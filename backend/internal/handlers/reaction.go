package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/sanderginn/clubhouse/internal/middleware"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/observability"
	"github.com/sanderginn/clubhouse/internal/services"
)

// ReactionHandler handles reaction endpoints
type ReactionHandler struct {
	reactionService *services.ReactionService
	notify          *services.NotificationService
	redis           *redis.Client
	postService     *services.PostService
	commentService  *services.CommentService
}

// NewReactionHandler creates a new reaction handler
func NewReactionHandler(db *sql.DB, redisClient *redis.Client, pushService *services.PushService) *ReactionHandler {
	return &ReactionHandler{
		reactionService: services.NewReactionService(db),
		notify:          services.NewNotificationService(db, redisClient, pushService),
		redis:           redisClient,
		postService:     services.NewPostService(db),
		commentService:  services.NewCommentService(db),
	}
}

// AddReactionToPost handles POST /api/v1/posts/{postId}/reactions
func (h *ReactionHandler) AddReactionToPost(w http.ResponseWriter, r *http.Request) {
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

	var req models.CreateReactionRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	reaction, err := h.reactionService.AddReactionToPost(r.Context(), postID, userID, req.Emoji)
	if err != nil {
		switch err.Error() {
		case "emoji is required":
			writeError(r.Context(), w, http.StatusBadRequest, "EMOJI_REQUIRED", err.Error())
		case "emoji must be 10 characters or less":
			writeError(r.Context(), w, http.StatusBadRequest, "EMOJI_TOO_LONG", err.Error())
		case "post not found":
			writeError(r.Context(), w, http.StatusNotFound, "POST_NOT_FOUND", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "REACTION_CREATION_FAILED", "Failed to add reaction")
		}
		return
	}

	response := models.CreateReactionResponse{
		Reaction: *reaction,
	}

	publishCtx, cancel := publishContext()
	_ = h.notify.CreateNotificationForPostReaction(publishCtx, postID, reaction.UserID)
	_ = publishEvent(publishCtx, h.redis, formatChannel(postPrefix, postID), "reaction_added", reactionEventData{
		PostID: &postID,
		UserID: reaction.UserID,
		Emoji:  reaction.Emoji,
	})
	if sectionID, err := h.postService.GetSectionIDByPostID(publishCtx, postID); err == nil {
		_ = publishEvent(publishCtx, h.redis, formatChannel(sectionPrefix, sectionID), "reaction_added", reactionEventData{
			PostID: &postID,
			UserID: reaction.UserID,
			Emoji:  reaction.Emoji,
		})
	}
	observability.RecordReactionAdded(publishCtx, reaction.Emoji)
	cancel()

	observability.LogInfo(r.Context(), "reaction added",
		"reaction_id", reaction.ID.String(),
		"user_id", userID.String(),
		"post_id", postID.String(),
		"emoji", reaction.Emoji,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode create reaction response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusCreated,
			Err:        err,
		})
	}
}

// GetPostReactions handles GET /api/v1/posts/{postId}/reactions
func (h *ReactionHandler) GetPostReactions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	postID, err := extractPostIDFromPath(r.URL.Path)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_POST_ID", "Invalid post ID format")
		return
	}

	reactions, err := h.reactionService.GetPostReactions(r.Context(), postID)
	if err != nil {
		if err.Error() == "post not found" {
			writeError(r.Context(), w, http.StatusNotFound, "POST_NOT_FOUND", "Post not found")
			return
		}
		writeError(r.Context(), w, http.StatusInternalServerError, "GET_REACTIONS_FAILED", "Failed to get reactions")
		return
	}

	response := models.GetReactionsResponse{
		Reactions: reactions,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode get post reactions response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// RemoveReactionFromPost handles DELETE /api/v1/posts/{postId}/reactions/{emoji}
func (h *ReactionHandler) RemoveReactionFromPost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only DELETE requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 7 {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Post ID and emoji are required")
		return
	}

	postIDStr := pathParts[4]
	postID, err := uuid.Parse(postIDStr)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_POST_ID", "Invalid post ID format")
		return
	}

	emoji, err := url.PathUnescape(pathParts[6])
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_EMOJI", "Invalid emoji format")
		return
	}

	if emoji == "" {
		writeError(r.Context(), w, http.StatusBadRequest, "EMOJI_REQUIRED", "Emoji is required")
		return
	}

	err = h.reactionService.RemoveReactionFromPost(r.Context(), postID, emoji, userID)
	if err != nil {
		if err.Error() == "reaction not found" {
			// Idempotent: return 204 even if not found
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeError(r.Context(), w, http.StatusInternalServerError, "REMOVE_REACTION_FAILED", "Failed to remove reaction")
		return
	}

	publishCtx, cancel := publishContext()
	_ = publishEvent(publishCtx, h.redis, formatChannel(postPrefix, postID), "reaction_removed", reactionEventData{
		PostID: &postID,
		UserID: userID,
		Emoji:  emoji,
	})
	if sectionID, err := h.postService.GetSectionIDByPostID(publishCtx, postID); err == nil {
		_ = publishEvent(publishCtx, h.redis, formatChannel(sectionPrefix, sectionID), "reaction_removed", reactionEventData{
			PostID: &postID,
			UserID: userID,
			Emoji:  emoji,
		})
	}
	observability.RecordReactionRemoved(publishCtx, emoji)
	cancel()

	observability.LogInfo(r.Context(), "reaction removed",
		"user_id", userID.String(),
		"post_id", postID.String(),
		"emoji", emoji,
	)

	w.WriteHeader(http.StatusNoContent)
}

// AddReactionToComment handles POST /api/v1/comments/{commentId}/reactions
func (h *ReactionHandler) AddReactionToComment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	commentID, err := extractCommentIDFromPath(r.URL.Path)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_COMMENT_ID", "Invalid comment ID format")
		return
	}

	var req models.CreateReactionRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	reaction, err := h.reactionService.AddReactionToComment(r.Context(), commentID, userID, req.Emoji)
	if err != nil {
		switch err.Error() {
		case "emoji is required":
			writeError(r.Context(), w, http.StatusBadRequest, "EMOJI_REQUIRED", err.Error())
		case "emoji must be 10 characters or less":
			writeError(r.Context(), w, http.StatusBadRequest, "EMOJI_TOO_LONG", err.Error())
		case "comment not found":
			writeError(r.Context(), w, http.StatusNotFound, "COMMENT_NOT_FOUND", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "REACTION_CREATION_FAILED", "Failed to add reaction")
		}
		return
	}

	response := models.CreateReactionResponse{
		Reaction: *reaction,
	}

	publishCtx, cancel := publishContext()
	_ = h.notify.CreateNotificationForCommentReaction(publishCtx, commentID, reaction.UserID)
	_ = publishEvent(publishCtx, h.redis, formatChannel(commentPrefix, commentID), "reaction_added", reactionEventData{
		CommentID: &commentID,
		UserID:    reaction.UserID,
		Emoji:     reaction.Emoji,
	})
	if postID, sectionID, err := h.commentService.GetCommentContext(publishCtx, commentID); err == nil {
		_ = publishEvent(publishCtx, h.redis, formatChannel(sectionPrefix, sectionID), "reaction_added", reactionEventData{
			PostID:    &postID,
			CommentID: &commentID,
			UserID:    reaction.UserID,
			Emoji:     reaction.Emoji,
		})
	}
	observability.RecordReactionAdded(publishCtx, reaction.Emoji)
	cancel()

	observability.LogInfo(r.Context(), "reaction added",
		"reaction_id", reaction.ID.String(),
		"user_id", userID.String(),
		"comment_id", commentID.String(),
		"emoji", reaction.Emoji,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode create reaction response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusCreated,
			Err:        err,
		})
	}
}

// GetCommentReactions handles GET /api/v1/comments/{commentId}/reactions
func (h *ReactionHandler) GetCommentReactions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	commentID, err := extractCommentIDFromPath(r.URL.Path)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_COMMENT_ID", "Invalid comment ID format")
		return
	}

	reactions, err := h.reactionService.GetCommentReactions(r.Context(), commentID)
	if err != nil {
		if err.Error() == "comment not found" {
			writeError(r.Context(), w, http.StatusNotFound, "COMMENT_NOT_FOUND", "Comment not found")
			return
		}
		writeError(r.Context(), w, http.StatusInternalServerError, "GET_REACTIONS_FAILED", "Failed to get reactions")
		return
	}

	response := models.GetReactionsResponse{
		Reactions: reactions,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode get comment reactions response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// RemoveReactionFromComment handles DELETE /api/v1/comments/{commentId}/reactions/{emoji}
func (h *ReactionHandler) RemoveReactionFromComment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only DELETE requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 7 {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Comment ID and emoji are required")
		return
	}

	commentIDStr := pathParts[4]
	commentID, err := uuid.Parse(commentIDStr)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_COMMENT_ID", "Invalid comment ID format")
		return
	}

	emoji, err := url.PathUnescape(pathParts[6])
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_EMOJI", "Invalid emoji format")
		return
	}

	if emoji == "" {
		writeError(r.Context(), w, http.StatusBadRequest, "EMOJI_REQUIRED", "Emoji is required")
		return
	}

	err = h.reactionService.RemoveReactionFromComment(r.Context(), commentID, emoji, userID)
	if err != nil {
		if err.Error() == "reaction not found" {
			// Idempotent: return 204 even if not found
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeError(r.Context(), w, http.StatusInternalServerError, "REMOVE_REACTION_FAILED", "Failed to remove reaction")
		return
	}

	publishCtx, cancel := publishContext()
	_ = publishEvent(publishCtx, h.redis, formatChannel(commentPrefix, commentID), "reaction_removed", reactionEventData{
		CommentID: &commentID,
		UserID:    userID,
		Emoji:     emoji,
	})
	if postID, sectionID, err := h.commentService.GetCommentContext(publishCtx, commentID); err == nil {
		_ = publishEvent(publishCtx, h.redis, formatChannel(sectionPrefix, sectionID), "reaction_removed", reactionEventData{
			PostID:    &postID,
			CommentID: &commentID,
			UserID:    userID,
			Emoji:     emoji,
		})
	}
	observability.RecordReactionRemoved(publishCtx, emoji)
	cancel()

	observability.LogInfo(r.Context(), "reaction removed",
		"user_id", userID.String(),
		"comment_id", commentID.String(),
		"emoji", emoji,
	)

	w.WriteHeader(http.StatusNoContent)
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

func extractCommentIDFromPath(path string) (uuid.UUID, error) {
	pathParts := strings.Split(path, "/")
	for i, part := range pathParts {
		if part == "comments" && i+1 < len(pathParts) {
			return uuid.Parse(pathParts[i+1])
		}
	}
	return uuid.Nil, errors.New("comment ID not found in path")
}
