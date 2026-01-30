package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sanderginn/clubhouse/internal/services"
)

func TestGetPublicConfig(t *testing.T) {
	services.ResetConfigServiceForTests()
	configService := services.GetConfigService()
	t.Cleanup(func() { services.ResetConfigServiceForTests() })

	timezone := "America/Los_Angeles"
	if _, err := configService.UpdateConfig(context.Background(), nil, nil, &timezone); err != nil {
		t.Fatalf("failed to set display timezone: %v", err)
	}

	handler := NewConfigHandler()
	req := httptest.NewRequest("GET", "/api/v1/config", nil)
	w := httptest.NewRecorder()

	handler.GetPublicConfig(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response struct {
		Config struct {
			DisplayTimezone string `json:"displayTimezone"`
		} `json:"config"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Config.DisplayTimezone != timezone {
		t.Fatalf("expected displayTimezone %s, got %s", timezone, response.Config.DisplayTimezone)
	}
}
