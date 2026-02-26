package services

import (
	"github.com/sanderginn/clubhouse/internal/observability"
	"go.opentelemetry.io/otel/trace"
)

func recordSpanError(span trace.Span, err error) {
	if err == nil {
		return
	}

	span.RecordError(err)
	observability.RecordServiceError("service_operation")
}
