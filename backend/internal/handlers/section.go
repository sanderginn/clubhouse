package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/observability"
	"github.com/sanderginn/clubhouse/internal/services"
)

type SectionHandler struct {
	sectionService *services.SectionService
}

func NewSectionHandler(db *sql.DB) *SectionHandler {
	return &SectionHandler{
		sectionService: services.NewSectionService(db),
	}
}

func (h *SectionHandler) ListSections(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	sections, err := h.sectionService.ListSections(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusInternalServerError, "LIST_SECTIONS_FAILED", "Failed to list sections")
		return
	}

	response := models.ListSectionsResponse{
		Sections: sections,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode list sections response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

func (h *SectionHandler) GetSection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Section ID is required")
		return
	}

	sectionIDStr := pathParts[4]
	sectionID, err := uuid.Parse(sectionIDStr)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_SECTION_ID", "Invalid section ID format")
		return
	}

	section, err := h.sectionService.GetSectionByID(r.Context(), sectionID)
	if err != nil {
		if err.Error() == "section not found" {
			writeError(r.Context(), w, http.StatusNotFound, "SECTION_NOT_FOUND", "Section not found")
			return
		}
		writeError(r.Context(), w, http.StatusInternalServerError, "GET_SECTION_FAILED", "Failed to get section")
		return
	}

	response := models.GetSectionResponse{
		Section: *section,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode get section response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}
