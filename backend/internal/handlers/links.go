package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/sanderginn/clubhouse/internal/models"
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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

	metadata, err := linkmeta.FetchMetadata(r.Context(), trimmedURL)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadGateway, "LINK_METADATA_FETCH_FAILED", "Failed to fetch link metadata")
		return
	}

	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	if _, ok := metadata["url"]; !ok {
		metadata["url"] = trimmedURL
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(models.LinkPreviewResponse{Metadata: metadata})
}
