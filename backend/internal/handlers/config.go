package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/sanderginn/clubhouse/internal/observability"
	"github.com/sanderginn/clubhouse/internal/services"
)

// PublicConfigResponse wraps public config for the frontend.
type PublicConfigResponse struct {
	Config PublicConfig `json:"config"`
}

// PublicConfig represents publicly available configuration values.
type PublicConfig struct {
	DisplayTimezone string `json:"displayTimezone"`
}

// ConfigHandler handles public configuration endpoints.
type ConfigHandler struct{}

// NewConfigHandler creates a new ConfigHandler.
func NewConfigHandler() *ConfigHandler {
	return &ConfigHandler{}
}

// GetPublicConfig returns the public configuration.
func (h *ConfigHandler) GetPublicConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	config := services.GetConfigService().GetConfig()
	response := PublicConfigResponse{
		Config: PublicConfig{
			DisplayTimezone: config.DisplayTimezone,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode public config response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}
