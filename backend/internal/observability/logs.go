package observability

import (
	"context"
	"runtime/debug"
	"strings"
	"time"

	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/trace"
)

type ErrorLog struct {
	Message    string
	Code       string
	StatusCode int
	UserID     string
	Err        error
}

func LogError(ctx context.Context, entry ErrorLog) {
	logger := global.Logger("clubhouse")

	var record log.Record
	record.SetTimestamp(time.Now())
	record.SetSeverity(log.SeverityError)
	record.SetSeverityText("ERROR")
	record.SetBody(log.StringValue(entry.Message))

	attrs := []log.KeyValue{
		log.String("error.code", entry.Code),
		log.Int("http.status_code", entry.StatusCode),
	}
	if entry.UserID != "" {
		attrs = append(attrs, log.String("user_id", entry.UserID))
	}
	if entry.Err != nil {
		attrs = append(attrs, log.String("error.message", entry.Err.Error()))
	}
	stack := strings.TrimSpace(string(debug.Stack()))
	if stack != "" {
		attrs = append(attrs, log.String("error.stack", stack))
	}
	if spanCtx := trace.SpanContextFromContext(ctx); spanCtx.IsValid() {
		attrs = append(attrs,
			log.String("trace_id", spanCtx.TraceID().String()),
			log.String("span_id", spanCtx.SpanID().String()),
		)
	}

	record.AddAttributes(attrs...)
	logger.Emit(ctx, record)
}
