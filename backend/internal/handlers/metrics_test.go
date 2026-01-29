package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type metricRequest struct {
	Metrics []map[string]any `json:"metrics"`
}

func TestRecordFrontendMetricsMethodNotAllowed(t *testing.T) {
	handler := NewMetricsHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics/vitals", nil)
	recorder := httptest.NewRecorder()

	handler.RecordFrontendMetrics(recorder, req)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d", recorder.Code)
	}
}

func TestRecordFrontendMetricsInvalidBody(t *testing.T) {
	handler := NewMetricsHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/metrics/vitals", bytes.NewBufferString("{"))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.RecordFrontendMetrics(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}

func TestRecordFrontendMetricsSuccess(t *testing.T) {
	handler := NewMetricsHandler()
	payload := metricRequest{
		Metrics: []map[string]any{
			{
				"type":           "web_vital",
				"name":           "LCP",
				"value":          1200.5,
				"unit":           "ms",
				"rating":         "good",
				"navigationType": "navigate",
			},
			{
				"type":       "api_timing",
				"endpoint":   "/api/v1/posts/:id",
				"method":     "GET",
				"status":     200,
				"durationMs": 95.2,
			},
			{
				"type":       "websocket_connect",
				"outcome":    "success",
				"durationMs": 40.3,
			},
			{
				"type":         "asset_load",
				"name":         "/assets/app.css",
				"resourceType": "css",
				"durationMs":   32.1,
			},
			{
				"type":       "component_render",
				"component":  "PostCard",
				"durationMs": 6.8,
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/metrics/vitals", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.RecordFrontendMetrics(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", recorder.Code)
	}
}
