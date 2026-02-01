package observability

import (
	"testing"
	"time"

	"go.opentelemetry.io/otel/log"
)

func TestBuildStderrPayloadExtractsFields(t *testing.T) {
	attrs := []log.KeyValue{
		log.String("trace_id", "trace-123"),
		log.String("span_id", "span-456"),
		log.String("user_id", "user-789"),
		log.String("error.code", "POST_CREATE_FAILED"),
		log.String("error.message", "boom"),
		log.String("error.stack", "stack"),
		log.Int("http.status_code", 500),
		log.String("section_id", "section-123"),
	}

	payload := buildStderrPayload("INFO", "hello", attrs)

	if payload.Level != "info" {
		t.Fatalf("expected level info, got %q", payload.Level)
	}
	if payload.Message != "hello" {
		t.Fatalf("expected message hello, got %q", payload.Message)
	}
	if payload.TraceID != "trace-123" || payload.SpanID != "span-456" || payload.UserID != "user-789" {
		t.Fatalf("expected trace/span/user IDs to be set, got %+v", payload)
	}
	if payload.Error == nil {
		t.Fatalf("expected error payload to be set")
	}
	if payload.Error.Code != "POST_CREATE_FAILED" || payload.Error.Message != "boom" || payload.Error.Stack != "stack" {
		t.Fatalf("unexpected error payload: %+v", payload.Error)
	}
	if payload.Error.StatusCode != 500 {
		t.Fatalf("expected status code 500, got %d", payload.Error.StatusCode)
	}
	if payload.Fields == nil || payload.Fields["section_id"] != "section-123" {
		t.Fatalf("expected fields to include section_id, got %+v", payload.Fields)
	}
	if payload.Fields["trace_id"] != nil || payload.Fields["span_id"] != nil || payload.Fields["user_id"] != nil {
		t.Fatalf("expected trace/span/user IDs to be removed from fields, got %+v", payload.Fields)
	}
	if _, err := time.Parse(time.RFC3339Nano, payload.Timestamp); err != nil {
		t.Fatalf("expected valid timestamp, got %q", payload.Timestamp)
	}
}

func TestBuildStderrPayloadEmptyFields(t *testing.T) {
	payload := buildStderrPayload("WARN", "empty", nil)

	if payload.Level != "warn" {
		t.Fatalf("expected level warn, got %q", payload.Level)
	}
	if payload.Fields != nil {
		t.Fatalf("expected fields to be nil, got %+v", payload.Fields)
	}
	if payload.Error != nil {
		t.Fatalf("expected error to be nil, got %+v", payload.Error)
	}
}
