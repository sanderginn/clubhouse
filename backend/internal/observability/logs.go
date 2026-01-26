package observability

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"runtime/debug"
	"strings"
	"time"

	stdlog "log"

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

var stderrLogger = stdlog.New(os.Stderr, "", 0)

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
	emitStderr("ERROR", entry.Message, attrs)
}

// LogInfo logs an informational message with optional key-value pairs
func LogInfo(ctx context.Context, message string, kvPairs ...string) {
	logger := global.Logger("clubhouse")

	var record log.Record
	record.SetTimestamp(time.Now())
	record.SetSeverity(log.SeverityInfo)
	record.SetSeverityText("INFO")
	record.SetBody(log.StringValue(message))

	attrs := make([]log.KeyValue, 0)

	// Convert key-value pairs to attributes
	for i := 0; i < len(kvPairs)-1; i += 2 {
		attrs = append(attrs, log.String(kvPairs[i], kvPairs[i+1]))
	}

	if spanCtx := trace.SpanContextFromContext(ctx); spanCtx.IsValid() {
		attrs = append(attrs,
			log.String("trace_id", spanCtx.TraceID().String()),
			log.String("span_id", spanCtx.SpanID().String()),
		)
	}

	record.AddAttributes(attrs...)
	logger.Emit(ctx, record)
	emitStderr("INFO", message, attrs)
}

type stderrLog struct {
	Timestamp  string         `json:"timestamp"`
	Severity   string         `json:"severity"`
	Message    string         `json:"message"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

func emitStderr(severity string, message string, attrs []log.KeyValue) {
	payload := stderrLog{
		Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
		Severity:   severity,
		Message:    message,
		Attributes: keyValuesToMap(attrs),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		stderrLogger.Printf("failed to marshal log payload: %v", err)
		return
	}

	stderrLogger.Print(string(data))
}

func keyValuesToMap(attrs []log.KeyValue) map[string]any {
	if len(attrs) == 0 {
		return nil
	}
	out := make(map[string]any, len(attrs))
	for _, attr := range attrs {
		out[attr.Key] = logValueToInterface(attr.Value)
	}
	return out
}

func logValueToInterface(value log.Value) any {
	switch value.Kind() {
	case log.KindBool:
		return value.AsBool()
	case log.KindInt64:
		return value.AsInt64()
	case log.KindFloat64:
		return value.AsFloat64()
	case log.KindString:
		return value.AsString()
	case log.KindBytes:
		bytes := value.AsBytes()
		if len(bytes) == 0 {
			return ""
		}
		return base64.StdEncoding.EncodeToString(bytes)
	case log.KindSlice:
		values := value.AsSlice()
		out := make([]any, 0, len(values))
		for _, item := range values {
			out = append(out, logValueToInterface(item))
		}
		return out
	case log.KindMap:
		return keyValuesToMap(value.AsMap())
	default:
		return nil
	}
}
