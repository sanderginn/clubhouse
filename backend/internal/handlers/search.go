package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/middleware"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/observability"
	"github.com/sanderginn/clubhouse/internal/services"
)

// SearchHandler handles search endpoints.
type SearchHandler struct {
	searchService *services.SearchService
}

const maxSearchQueryLength = 512

// NewSearchHandler creates a new search handler.
func NewSearchHandler(db *sql.DB) *SearchHandler {
	return &SearchHandler{
		searchService: services.NewSearchService(db),
	}
}

// Search handles GET /api/v1/search?q=query&scope=global.
func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeError(r.Context(), w, http.StatusBadRequest, "QUERY_REQUIRED", "Query is required")
		return
	}
	if len(q) > maxSearchQueryLength {
		writeError(r.Context(), w, http.StatusBadRequest, "QUERY_TOO_LONG", "Query is too long")
		return
	}

	scope := strings.TrimSpace(r.URL.Query().Get("scope"))
	if scope == "" {
		scope = "section"
	}

	var sectionID *uuid.UUID
	if scope == "section" {
		sectionIDStr := strings.TrimSpace(r.URL.Query().Get("section_id"))
		if sectionIDStr == "" {
			if contextSectionID, err := middleware.GetSectionIDFromContext(r.Context()); err == nil {
				sectionID = &contextSectionID
			} else {
				writeError(r.Context(), w, http.StatusBadRequest, "SECTION_ID_REQUIRED", "Section ID is required for section scope")
				return
			}
		}

		if sectionIDStr != "" {
			parsedID, err := uuid.Parse(sectionIDStr)
			if err != nil {
				writeError(r.Context(), w, http.StatusBadRequest, "INVALID_SECTION_ID", "Invalid section ID format")
				return
			}

			sectionID = &parsedID
		}
	} else if scope != "global" {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_SCOPE", "Scope must be 'section' or 'global'")
		return
	}

	limit := 20
	if limitStr := strings.TrimSpace(r.URL.Query().Get("limit")); limitStr != "" {
		parsedLimit, err := parseIntParam(limitStr)
		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_LIMIT", "Limit must be a number")
			return
		}
		limit = parsedLimit
	}

	searchStart := time.Now()
	meaningful, err := h.searchService.IsQueryMeaningful(r.Context(), q)
	if err != nil {
		writeError(r.Context(), w, http.StatusInternalServerError, "SEARCH_FAILED", "Failed to search")
		return
	}
	if !meaningful {
		writeError(r.Context(), w, http.StatusBadRequest, "QUERY_INVALID", "Query is invalid")
		return
	}

	// Get the current user ID for reaction state (optional - uuid.Nil if not authenticated)
	userID, _ := middleware.GetUserIDFromContext(r.Context())

	results, err := h.searchService.Search(r.Context(), q, scope, sectionID, limit, userID)
	if err != nil {
		writeError(r.Context(), w, http.StatusInternalServerError, "SEARCH_FAILED", "Failed to search")
		return
	}
	observability.RecordSearchQuery(r.Context(), scope, len(results), time.Since(searchStart))

	response := models.SearchResponse{Results: results}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode search response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}
