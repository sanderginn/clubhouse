package handlers

import (
	"math"
	"net/http"
	"strings"

	"github.com/sanderginn/clubhouse/internal/observability"
)

const maxFrontendMetricsPerRequest = 50
const maxFrontendMetricTagLength = 128

type MetricsHandler struct{}

func NewMetricsHandler() *MetricsHandler {
	return &MetricsHandler{}
}

type frontendMetricsRequest struct {
	Metrics []frontendMetric `json:"metrics"`
}

type frontendMetric struct {
	Type           string   `json:"type"`
	Name           string   `json:"name,omitempty"`
	Value          *float64 `json:"value,omitempty"`
	Unit           string   `json:"unit,omitempty"`
	Rating         string   `json:"rating,omitempty"`
	Delta          *float64 `json:"delta,omitempty"`
	ID             string   `json:"id,omitempty"`
	NavigationType string   `json:"navigationType,omitempty"`
	Endpoint       string   `json:"endpoint,omitempty"`
	Method         string   `json:"method,omitempty"`
	Status         *int     `json:"status,omitempty"`
	DurationMs     *float64 `json:"durationMs,omitempty"`
	ResourceType   string   `json:"resourceType,omitempty"`
	Component      string   `json:"component,omitempty"`
	Outcome        string   `json:"outcome,omitempty"`
}

func (h *MetricsHandler) RecordFrontendMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST requests are allowed")
		return
	}

	var req frontendMetricsRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if len(req.Metrics) == 0 {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "No metrics provided")
		return
	}
	if len(req.Metrics) > maxFrontendMetricsPerRequest {
		writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Too many metrics in one request")
		return
	}

	for _, metric := range req.Metrics {
		switch strings.TrimSpace(metric.Type) {
		case "web_vital":
			handleWebVitalMetric(r, metric)
		case "api_timing":
			handleApiTimingMetric(r, metric)
		case "websocket_connect":
			handleWebsocketMetric(r, metric)
		case "asset_load":
			handleAssetMetric(r, metric)
		case "component_render":
			handleComponentMetric(r, metric)
		default:
			observability.LogWarn(r.Context(), "unknown frontend metric type", "metric_type", metric.Type)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func handleWebVitalMetric(r *http.Request, metric frontendMetric) {
	name := sanitizeMetricValue(metric.Name)
	if name == "" || metric.Value == nil {
		return
	}
	value := *metric.Value
	if isInvalidMetricNumber(value) {
		return
	}
	observability.RecordFrontendWebVital(
		r.Context(),
		name,
		value,
		sanitizeMetricValue(metric.Rating),
		sanitizeMetricValue(metric.NavigationType),
		sanitizeMetricValue(metric.Unit),
	)
}

func handleApiTimingMetric(r *http.Request, metric frontendMetric) {
	endpoint := sanitizeMetricValue(metric.Endpoint)
	method := sanitizeMetricValue(strings.ToUpper(metric.Method))
	if endpoint == "" || method == "" || metric.DurationMs == nil {
		return
	}
	duration := *metric.DurationMs
	if isInvalidMetricNumber(duration) {
		return
	}
	status := 0
	if metric.Status != nil {
		status = *metric.Status
	}
	observability.RecordFrontendAPIDuration(r.Context(), endpoint, method, status, duration)
}

func handleWebsocketMetric(r *http.Request, metric frontendMetric) {
	if metric.DurationMs == nil {
		return
	}
	duration := *metric.DurationMs
	if isInvalidMetricNumber(duration) {
		return
	}
	observability.RecordFrontendWebsocketConnect(r.Context(), sanitizeMetricValue(metric.Outcome), duration)
}

func handleAssetMetric(r *http.Request, metric frontendMetric) {
	if metric.DurationMs == nil {
		return
	}
	duration := *metric.DurationMs
	if isInvalidMetricNumber(duration) {
		return
	}
	observability.RecordFrontendAssetLoad(
		r.Context(),
		sanitizeMetricValue(metric.ResourceType),
		sanitizeMetricValue(metric.Name),
		duration,
	)
}

func handleComponentMetric(r *http.Request, metric frontendMetric) {
	if metric.DurationMs == nil {
		return
	}
	duration := *metric.DurationMs
	if isInvalidMetricNumber(duration) {
		return
	}
	observability.RecordFrontendComponentRender(r.Context(), sanitizeMetricValue(metric.Component), duration)
}

func sanitizeMetricValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) > maxFrontendMetricTagLength {
		return value[:maxFrontendMetricTagLength]
	}
	return value
}

func isInvalidMetricNumber(value float64) bool {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return true
	}
	if value < 0 {
		return true
	}
	return false
}
