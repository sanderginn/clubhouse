package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/sanderginn/clubhouse/internal/models"
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
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	sections, err := h.sectionService.ListSections(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "LIST_SECTIONS_FAILED", "Failed to list sections")
		return
	}

	response := models.ListSectionsResponse{
		Sections: sections,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
