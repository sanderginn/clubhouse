package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/middleware"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/services"
)

// SearchHandler handles search endpoints.
type SearchHandler struct {
	searchService *services.SearchService
}

// NewSearchHandler creates a new search handler.
func NewSearchHandler(db *sql.DB) *SearchHandler {
	return &SearchHandler{
		searchService: services.NewSearchService(db),
	}
}

// Search handles GET /api/v1/search?q=query&scope=global.
func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeError(w, http.StatusBadRequest, "QUERY_REQUIRED", "Query is required")
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
				writeError(w, http.StatusBadRequest, "SECTION_ID_REQUIRED", "Section ID is required for section scope")
				return
			}
		}

		if sectionIDStr != "" {
			parsedID, err := uuid.Parse(sectionIDStr)
			if err != nil {
				writeError(w, http.StatusBadRequest, "INVALID_SECTION_ID", "Invalid section ID format")
				return
			}

			sectionID = &parsedID
		}
	} else if scope != "global" {
		writeError(w, http.StatusBadRequest, "INVALID_SCOPE", "Scope must be 'section' or 'global'")
		return
	}

	limit := 20
	if limitStr := strings.TrimSpace(r.URL.Query().Get("limit")); limitStr != "" {
		parsedLimit, err := parseIntParam(limitStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_LIMIT", "Limit must be a number")
			return
		}
		limit = parsedLimit
	}

	results, err := h.searchService.Search(r.Context(), q, scope, sectionID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SEARCH_FAILED", "Failed to search")
		return
	}

	response := models.SearchResponse{Results: results}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
