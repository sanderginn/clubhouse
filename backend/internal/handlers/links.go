package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/observability"
	"github.com/sanderginn/clubhouse/internal/services"
	linkmeta "github.com/sanderginn/clubhouse/internal/services/links"
)

// LinkHandler handles link-related endpoints.
type LinkHandler struct{}

// NewLinkHandler creates a new link handler.
func NewLinkHandler() *LinkHandler {
	return &LinkHandler{}
}

// PreviewLink handles POST /api/v1/links/preview.
func (h *LinkHandler) PreviewLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST requests are allowed")
		return
	}

	if !services.GetConfigService().IsLinkMetadataEnabled() {
		writeError(r.Context(), w, http.StatusForbidden, "LINK_METADATA_DISABLED", "Link previews are disabled")
		return
	}

	var req models.LinkPreviewRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	trimmedURL := strings.TrimSpace(req.URL)
	if trimmedURL == "" {
		writeError(r.Context(), w, http.StatusBadRequest, "URL_REQUIRED", "URL is required")
		return
	}

	if len(trimmedURL) > 2048 {
		writeError(r.Context(), w, http.StatusBadRequest, "URL_TOO_LONG", "Link URL must be less than 2048 characters")
		return
	}

	observability.RecordLinkMetadataFetchAttempt(r.Context(), 1)
	start := time.Now()
	embed, _ := linkmeta.ExtractEmbed(r.Context(), trimmedURL)
	metadata, err := linkmeta.FetchMetadata(r.Context(), trimmedURL)
	observability.RecordLinkMetadataFetchDuration(r.Context(), time.Since(start))
	domain := linkmeta.ExtractDomain(trimmedURL)
	if err != nil {
		errorType := linkmeta.ClassifyFetchError(err)
		observability.RecordLinkMetadataFetchFailure(r.Context(), 1, domain, errorType)
		observability.LogWarn(r.Context(), "link metadata fetch failed", "link_url", trimmedURL, "link_domain", domain, "error_type", errorType, "error", err.Error())
		if embed == nil {
			metadata = map[string]interface{}{}
		}
	}
	if len(metadata) == 0 {
		observability.RecordLinkMetadataFetchFailure(r.Context(), 1, domain, "empty_metadata")
		observability.LogWarn(r.Context(), "link metadata fetch empty", "link_url", trimmedURL, "link_domain", domain, "error_type", "empty_metadata")
	} else {
		observability.RecordLinkMetadataFetchSuccess(r.Context(), 1)
	}

	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	metadata = linkmeta.ApplyEmbedMetadata(metadata, embed)
	if _, ok := metadata["url"]; !ok {
		metadata["url"] = trimmedURL
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(models.LinkPreviewResponse{Metadata: metadata}); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode link preview response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// ParseRecipe handles POST /api/v1/links/parse-recipe.
func (h *LinkHandler) ParseRecipe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST requests are allowed")
		return
	}

	if !services.GetConfigService().IsLinkMetadataEnabled() {
		writeError(r.Context(), w, http.StatusForbidden, "LINK_METADATA_DISABLED", "Link previews are disabled")
		return
	}

	var req models.LinkPreviewRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	trimmedURL := strings.TrimSpace(req.URL)
	if trimmedURL == "" {
		writeError(r.Context(), w, http.StatusBadRequest, "URL_REQUIRED", "URL is required")
		return
	}

	if len(trimmedURL) > 2048 {
		writeError(r.Context(), w, http.StatusBadRequest, "URL_TOO_LONG", "Link URL must be less than 2048 characters")
		return
	}

	observability.RecordLinkMetadataFetchAttempt(r.Context(), 1)
	start := time.Now()
	embed, _ := linkmeta.ExtractEmbed(r.Context(), trimmedURL)
	metadata, err := linkmeta.FetchMetadata(r.Context(), trimmedURL)
	observability.RecordLinkMetadataFetchDuration(r.Context(), time.Since(start))
	domain := linkmeta.ExtractDomain(trimmedURL)
	if err != nil {
		errorType := linkmeta.ClassifyFetchError(err)
		observability.RecordLinkMetadataFetchFailure(r.Context(), 1, domain, errorType)
		observability.LogWarn(r.Context(), "recipe metadata fetch failed", "link_url", trimmedURL, "link_domain", domain, "error_type", errorType, "error", err.Error())
		writeError(r.Context(), w, http.StatusBadGateway, "RECIPE_PARSE_FAILED", "Failed to parse recipe")
		return
	}

	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	metadata = linkmeta.ApplyEmbedMetadata(metadata, embed)
	if _, ok := metadata["url"]; !ok {
		metadata["url"] = trimmedURL
	}

	recipe, ok := metadata["recipe"]
	if !ok || recipe == nil {
		observability.RecordLinkMetadataFetchFailure(r.Context(), 1, domain, "recipe_not_found")
		observability.LogWarn(r.Context(), "recipe metadata not found", "link_url", trimmedURL, "link_domain", domain)
		writeError(r.Context(), w, http.StatusUnprocessableEntity, "RECIPE_NOT_FOUND", "No recipe data found")
		return
	}

	observability.RecordLinkMetadataFetchSuccess(r.Context(), 1)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(models.LinkPreviewResponse{Metadata: metadata}); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode recipe parse response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}
